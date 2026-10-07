package database

import (
	"context"
	"fmt"
	"net/url"
	"os"

	"github.com/jackc/pgx/v5/pgxpool"

	"siakad-uts/config"
)

func NewPool(ctx context.Context) (*pgxpool.Pool, error) {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		dsn = fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=%s",
			url.QueryEscape(config.GetEnv("DB_USER", "postgres")),
			url.QueryEscape(config.GetEnv("DB_PASSWORD", "")),
			config.GetEnv("DB_HOST", "localhost"),
			config.GetEnv("DB_PORT", "5432"),
			url.PathEscape(config.GetEnv("DB_NAME", "siakad_uts")),
			config.GetEnv("DB_SSLMODE", "disable"),
		)
	}
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, fmt.Errorf("konfigurasi database tidak valid: %w", err)
	}
	cfg.MaxConns = int32(config.GetEnvInt("DB_MAX_CONNS", 10))
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("gagal membuat pool: %w", err)
	}
	if err = pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("gagal terhubung ke database: %w", err)
	}
	return pool, nil
}
