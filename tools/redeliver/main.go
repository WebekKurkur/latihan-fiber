// Command redeliver mengulangi satu event outbox beberapa kali.
//
// Alat ini dibuat untuk menguji idempotency (Bagian C.5): satu event
// outbox dimasukkan ke queue beberapa kali dengan TaskID BERBEDA, sehingga
// worker mengerjakan task yang sama lebih dari satu kali.
//
// Hasil yang benar tetap: SATU baris di tabel notifications dan SATU baris
// di logs/mail.log. Kalau tidak begitu, handler worker tidak idempoten.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/hibiken/asynq"

	"latihan-fiber/app/model"
	"latihan-fiber/config"
	"latihan-fiber/database"
)

func main() {
	config.LoadEnv()

	outboxID := flag.Int64("outbox", 0, "id event outbox yang akan diulang")
	copies := flag.Int("copies", 5, "berapa banyak salinan task yang dibuat")
	flag.Parse()

	if *outboxID <= 0 {
		fmt.Fprintln(os.Stderr, "flag -outbox wajib diisi dengan id event yang valid")
		os.Exit(1)
	}
	if *copies <= 0 {
		fmt.Fprintln(os.Stderr, "flag -copies harus lebih besar dari nol")
		os.Exit(1)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	pool, err := database.NewPool(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "gagal terhubung ke database: %v\n", err)
		os.Exit(1)
	}
	defer pool.Close()

	var eventType string
	var payload []byte
	err = pool.QueryRow(ctx,
		`SELECT event_type, payload FROM outbox WHERE id = $1`, *outboxID,
	).Scan(&eventType, &payload)
	if err != nil {
		fmt.Fprintf(os.Stderr, "event outbox %d tidak ditemukan: %v\n", *outboxID, err)
		os.Exit(1)
	}

	job := model.NotificationJob{
		OutboxID:  *outboxID,
		EventType: eventType,
		Payload:   json.RawMessage(payload),
	}
	body, err := json.Marshal(job)
	if err != nil {
		fmt.Fprintf(os.Stderr, "menyusun payload task: %v\n", err)
		os.Exit(1)
	}

	client := asynq.NewClient(config.RedisOpt())
	defer func() {
		if err := client.Close(); err != nil {
			fmt.Fprintf(os.Stderr, "gagal menutup klien queue: %v\n", err)
		}
	}()

	for n := 1; n <= *copies; n++ {
		task := asynq.NewTask(model.TaskSendNotification, body,
			// TaskID harus BERBEDA untuk setiap salinan: Asynq menolak
			// TaskID yang sama. Justru perbedaan itulah yang membuat
			// salinan ini menjadi percobaan idempotency yang nyata.
			asynq.TaskID(fmt.Sprintf("redeliver-%d-%d", *outboxID, n)),
			asynq.Queue(model.QueueNotifications),
			asynq.MaxRetry(5),
			asynq.Timeout(30*time.Second),
			asynq.Retention(24*time.Hour),
		)
		if _, err := client.EnqueueContext(ctx, task); err != nil {
			fmt.Fprintf(os.Stderr, "gagal memasukkan salinan %d: %v\n", n, err)
			os.Exit(1)
		}
		fmt.Printf("salinan %d untuk outbox %d (%s) sudah masuk queue\n", n, *outboxID, eventType)
	}
}
