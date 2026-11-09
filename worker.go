package main

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5/pgxpool"

	"latihan-fiber/app/model"
	"latihan-fiber/app/repository"
	"latihan-fiber/app/service"
	"latihan-fiber/config"
	"latihan-fiber/helper"
)

// startWorker menyalakan dua komponen background:
//
//  1. OutboxRelay  — memindahkan event dari tabel outbox ke queue Redis.
//  2. Asynq worker — mengerjakan task "notification:send".
//
// Keduanya butuh PostgreSQL, tapi TIDAK butuh HTTP. Karena itu worker
// boleh dijalankan sebagai proses terpisah dari API (lihat flag -role),
// sehingga API tetap melayani request walaupun worker sedang mati.
//
// Fungsi yang dikembalikan adalah rutinitas shutdown. Ia dipanggil saat
// proses menerima sinyal berhenti.
func startWorker(ctx context.Context, logger *slog.Logger, pool *pgxpool.Pool) func() {
	mailer, err := helper.NewLogMailer(
		config.GetEnv("MAIL_LOG_PATH", filepath.Join("logs", "mail.log")),
		time.Duration(config.GetEnvInt("MAILER_DELAY_MS", 100))*time.Millisecond,
		config.GetEnvInt("MAILER_FAIL_EVERY", 0),
	)
	if err != nil {
		logger.Error("gagal menyiapkan mailer", slog.String("error", err.Error()))
		os.Exit(1)
	}

	outboxRepository := repository.NewOutboxRepository(pool)
	notificationRepository := repository.NewNotificationRepository(pool)
	studentRepository := repository.NewStudentRepository(pool)
	txManager := repository.NewTxManager(pool)

	// Client Asynq dipakai relay untuk memasukkan task ke queue.
	client := asynq.NewClient(config.RedisOpt())

	relay := service.NewOutboxRelay(
		outboxRepository, txManager, client, logger,
		time.Duration(config.GetEnvInt("RELAY_INTERVAL_MS", 1000))*time.Millisecond,
		config.GetEnvInt("RELAY_BATCH", 100),
	)
	go relay.Run(ctx)

	worker := service.NewNotificationWorker(
		studentRepository, notificationRepository, txManager, mailer, logger,
	)

	// ServeMux memetakan NAMA JENIS task ke handler-nya. Nama queue
	// ditentukan saat task dimasukkan (asynq.Queue(...)), bukan di sini.
	mux := asynq.NewServeMux()
	mux.HandleFunc(model.TaskSendNotification, worker.HandleSend)

	server := config.NewWorkerServer(logger)
	go func() {
		// Start mengembalikan error. Bila Redis mati, worker tetap hidup
		// dan menunggu; task yang masuk saat Redis hidup kembali akan
		// langsung dikerjakan.
		if err := server.Start(mux); err != nil {
			logger.Error("worker berhenti", slog.String("error", err.Error()))
		}
	}()
	logger.Info("worker berjalan", slog.String("queue", model.QueueNotifications))

	return func() {
		logger.Info("menutup worker")
		// Shutdown menunggu task yang sedang berjalan selesai (maksimal
		// ShutdownTimeout), lalu mengembalikan task yang belum selesai
		// ke queue supaya tidak hilang.
		server.Shutdown()
		if err := client.Close(); err != nil {
			logger.Error("gagal menutup klien queue", slog.String("error", err.Error()))
		}
	}
}
