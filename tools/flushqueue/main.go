// Command flushqueue mengosongkan queue notifikasi di Redis.
//
// Digunakan sebelum menjalankan ulang uji Modul 11. Task lama yang masih
// tertinggal di Redis akan dikerjakan worker lagi dan membuat notifikasi
// baru yang tidak terhitung dalam harapan modul.
//
//	go run ./tools/flushqueue
package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/hibiken/asynq"

	"latihan-fiber/app/model"
	"latihan-fiber/config"
)

func main() {
	config.LoadEnv()

	inspector := asynq.NewInspector(config.RedisOpt())
	defer func() {
		if err := inspector.Close(); err != nil {
			fmt.Fprintf(os.Stderr, "gagal menutup inspector: %v\n", err)
		}
	}()

	queue := model.QueueNotifications
	deleted := 0

	steps := []struct {
		name string
		fn   func(string) (int, error)
	}{
		{"pending", inspector.DeleteAllPendingTasks},
		{"scheduled", inspector.DeleteAllScheduledTasks},
		{"retry", inspector.DeleteAllRetryTasks},
		{"archived", inspector.DeleteAllArchivedTasks},
		{"completed", inspector.DeleteAllCompletedTasks},
	}
	for _, step := range steps {
		n, err := step.fn(queue)
		if err != nil {
			// Queue yang belum pernah dipakai memang tidak ada di Redis.
			// Itu bukan kegagalan: tidak ada task yang perlu dihapus.
			if strings.Contains(err.Error(), "does not exist") {
				fmt.Printf("%s: 0 task dihapus (queue belum pernah dipakai)\n", step.name)
				continue
			}
			fmt.Fprintf(os.Stderr, "gagal mengosongkan %s: %v\n", step.name, err)
			os.Exit(1)
		}
		deleted += n
		fmt.Printf("%s: %d task dihapus\n", step.name, n)
	}

	fmt.Printf("total %d task dihapus dari queue %q\n", deleted, queue)
}
