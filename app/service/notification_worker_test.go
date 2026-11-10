package service

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"testing"

	"github.com/hibiken/asynq"

	"latihan-fiber/app/model"
	"latihan-fiber/app/repository"
)

// fakeStudents memenuhi repository.StudentRepository dengan cara embedding
// interface. Hanya FindByID yang benar-benar diisi; method lain berasal dari
// interface yang di-embed dan bernilai nil, sehingga test akan PANIC bila
// kode yang diuji memanggilnya. Itu disengaja: panic tersebut memberi tahu
// bahwa kode memakai dependency di luar yang kita perkirakan.
type fakeStudents struct {
	repository.StudentRepository
	students map[int]model.Student
}

func (f *fakeStudents) FindByID(_ context.Context, id int) (model.Student, error) {
	s, ok := f.students[id]
	if !ok {
		return model.Student{}, repository.ErrNotFound
	}
	return s, nil
}

func newTask(t *testing.T, eventType string, event any) *asynq.Task {
	t.Helper()
	payload, err := json.Marshal(event)
	if err != nil {
		t.Fatalf("menyusun payload event: %v", err)
	}
	job, err := json.Marshal(model.NotificationJob{OutboxID: 1, EventType: eventType, Payload: payload})
	if err != nil {
		t.Fatalf("menyusun job: %v", err)
	}
	return asynq.NewTask(model.TaskSendNotification, job)
}

// Seluruh skenario di bawah ini berhenti SEBELUM worker menyentuh
// transaction dan mailer. Karena itu tx dan mailer boleh nil: test ini
// murni unit test, tanpa database dan tanpa Redis.
func TestHandleSend_PermanentFailures(t *testing.T) {
	worker := NewNotificationWorker(
		&fakeStudents{students: map[int]model.Student{
			7: {ID: 7, Name: "mhsuji", Email: "mhsuji@student.unair.ac.id"},
			// Student 8 sengaja tidak punya email: notifikasi tidak akan
			// pernah sampai, sehingga ini kegagalan permanen.
			8: {ID: 8, Name: "mahasiswa_tanpa_email"},
		}},
		nil, nil, nil,
		slog.New(slog.NewTextHandler(io.Discard, nil)),
	)

	tests := []struct {
		name string
		task *asynq.Task
	}{
		{name: "payload task bukan JSON", task: asynq.NewTask(model.TaskSendNotification,
			[]byte("bukan json"))},
		{name: "student sudah dihapus", task: newTask(t, model.EventEnrollmentCreated,
			model.EnrollmentEvent{StudentID: 99})},
		{name: "jenis event tidak dikenal", task: newTask(t, "enrollment.unknown",
			model.EnrollmentEvent{StudentID: 7})},
		{name: "student tanpa alamat email", task: newTask(t, model.EventEnrollmentCreated,
			model.EnrollmentEvent{StudentID: 8})},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := worker.HandleSend(context.Background(), tt.task)
			// Asynq memeriksa SkipRetry dengan errors.Is, maka test juga
			// harus memeriksa dengan cara yang sama.
			if !errors.Is(err, asynq.SkipRetry) {
				t.Errorf("error = %v, seharusnya membungkus asynq.SkipRetry", err)
			}
		})
	}
}
