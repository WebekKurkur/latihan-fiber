package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// DBTX adalah operasi database yang dipakai repository, baik pool maupun
// transaction. Dengan kontrak ini repository dapat dipakai di dalam transaksi
// tanpa bergantung pada implementasi konkret.
type DBTX interface {
	Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// Kode SQLSTATE dari PostgreSQL yang perlu dikenali aplikasi.
// Daftar lengkapnya ada di lampiran dokumentasi PostgreSQL "Error Codes".
const (
	codeUniqueViolation      = "23505"
	codeSerializationFailure = "40001"
	codeDeadlockDetected     = "40P01"
)

// pgErrorCode mengembalikan SQLSTATE dari error PostgreSQL, atau string
// kosong bila err bukan error PostgreSQL.
func pgErrorCode(err error) string {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code
	}
	return ""
}

type TxManager struct {
	pool *pgxpool.Pool
}

func NewTxManager(pool *pgxpool.Pool) *TxManager {
	return &TxManager{pool: pool}
}

// WithTx menjalankan fn dalam transaction dan mengulang deadlock atau
// serialization failure. Fn harus aman untuk dijalankan ulang.
func (m *TxManager) WithTx(ctx context.Context, fn func(DBTX) error) error {
	const maxAttempts = 3
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		err := m.runOnce(ctx, fn)
		if err == nil || !retryableTransactionError(err) || attempt == maxAttempts {
			return err
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(time.Duration(attempt) * 50 * time.Millisecond):
		}
	}
	return nil
}

// WithTxOnce menjalankan fn tepat satu kali tanpa retry. Ini dipakai ketika fn
// mengandung side effect eksternal seperti pengiriman email.
func (m *TxManager) WithTxOnce(ctx context.Context, fn func(DBTX) error) error {
	return m.runOnce(ctx, fn)
}

func (m *TxManager) runOnce(ctx context.Context, fn func(DBTX) error) error {
	tx, err := m.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("memulai transaction: %w", err)
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	if err := fn(tx); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}
	return nil
}

func retryableTransactionError(err error) bool {
	code := pgErrorCode(err)
	return code == codeSerializationFailure || code == codeDeadlockDetected
}
