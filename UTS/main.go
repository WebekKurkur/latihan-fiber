package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"siakad-uts/app/repository"
	"siakad-uts/app/service"
	"siakad-uts/config"
	"siakad-uts/database"
	"siakad-uts/helper"
	"siakad-uts/route"
)

const minSecretLength = 32

func main() {
	config.LoadEnv()
	logger := config.NewLogger()
	secret := config.GetEnv("JWT_SECRET", "")
	if len(secret) < minSecretLength {
		logger.Error("JWT_SECRET wajib minimal 32 karakter")
		os.Exit(1)
	}

	ctx := context.Background()
	pool, err := database.NewPool(ctx)
	if err != nil {
		logger.Error("gagal terhubung ke database", slog.String("error", err.Error()))
		os.Exit(1)
	}
	defer pool.Close()

	repo := repository.New(pool)
	jwt := helper.NewJWT(secret, time.Duration(config.GetEnvInt("JWT_ACCESS_TTL_MINUTES", 15))*time.Minute)
	deps := route.Dependencies{
		Auth:        service.AuthService{Users: repo, JWT: jwt},
		Students:    service.StudentService{Students: repo, Enrollments: repo},
		Courses:     service.CourseService{Courses: repo},
		Enrollments: service.EnrollmentService{Enrollments: repo, Students: repo},
		Users:       repo,
		JWT:         jwt,
	}
	app := config.NewApp(logger, deps)
	port := config.GetEnv("APP_PORT", "3001")

	go func() {
		if err := app.Listen(":" + port); err != nil {
			logger.Error("server berhenti", slog.String("error", err.Error()))
		}
	}()
	logger.Info("server berjalan", slog.String("port", port))

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := app.ShutdownWithContext(shutdownCtx); err != nil {
		logger.Error("gagal menutup server", slog.String("error", err.Error()))
	}
	logger.Info("server berhenti dengan rapi")
}
