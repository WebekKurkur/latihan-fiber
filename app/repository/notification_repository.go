package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"

	"latihan-fiber/app/model"
)

type NotificationRepository interface {
	WithTx(tx DBTX) NotificationRepository

	// Claim mencatat bahwa notifikasi untuk sebuah event akan dikirim.
	// Mengembalikan false bila event itu sudah pernah tercatat.
	Claim(ctx context.Context, n model.Notification) (bool, error)

	FindByStudent(ctx context.Context, studentID, limit int) ([]model.Notification, error)
	Count(ctx context.Context) (int, error)
}

type notificationPostgresRepository struct {
	db DBTX
}

func NewNotificationRepository(db DBTX) NotificationRepository {
	return &notificationPostgresRepository{db: db}
}

func (r *notificationPostgresRepository) WithTx(tx DBTX) NotificationRepository {
	return &notificationPostgresRepository{db: tx}
}

// Claim memakai ON CONFLICT DO NOTHING, bukan SELECT lalu INSERT bila belum
// ada. UNIQUE constraint menangani dua worker yang mengklaim event bersamaan.
//
// Idempotency key-nya adalah pasangan (outbox_id, student_id), bukan
// outbox_id saja. Event broadcast (perubahan kuota) memakai satu baris
// outbox untuk satu peserta, sehingga outbox_id yang sama dapat muncul pada
// beberapa peserta yang berbeda. Pasangan ini tetap menolak pengiriman
// ganda untuk peserta yang sama.
func (r *notificationPostgresRepository) Claim(
	ctx context.Context,
	n model.Notification,
) (bool, error) {
	var id int64
	err := r.db.QueryRow(ctx, `
		INSERT INTO notifications (
			outbox_id, student_id, channel, subject, body
		)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (outbox_id, student_id) DO NOTHING
		RETURNING id`,
		n.OutboxID, n.StudentID, n.Channel, n.Subject, n.Body,
	).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("mencatat notifikasi: %w", err)
	}
	return true, nil
}

// FindByStudent mengurutkan berdasarkan outbox_id, bukan id notifikasi.
func (r *notificationPostgresRepository) FindByStudent(
	ctx context.Context,
	studentID, limit int,
) ([]model.Notification, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, outbox_id, student_id, channel, subject, body, sent_at
		FROM notifications
		WHERE student_id = $1
		ORDER BY outbox_id DESC
		LIMIT $2`, studentID, limit)
	if err != nil {
		return nil, fmt.Errorf("mengambil notifikasi: %w", err)
	}
	defer rows.Close()

	result := []model.Notification{}
	for rows.Next() {
		var n model.Notification
		if err := rows.Scan(
			&n.ID, &n.OutboxID, &n.StudentID, &n.Channel,
			&n.Subject, &n.Body, &n.SentAt,
		); err != nil {
			return nil, fmt.Errorf("membaca row notifikasi: %w", err)
		}
		result = append(result, n)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("membaca notifikasi: %w", err)
	}
	return result, nil
}

func (r *notificationPostgresRepository) Count(ctx context.Context) (int, error) {
	var total int
	if err := r.db.QueryRow(ctx,
		`SELECT COUNT(*) FROM notifications`,
	).Scan(&total); err != nil {
		return 0, fmt.Errorf("menghitung notifikasi: %w", err)
	}
	return total, nil
}
