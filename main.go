package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5/pgxpool"

	"latihan-fiber/app/repository"
	"latihan-fiber/app/service"
	"latihan-fiber/config"
	"latihan-fiber/database"
	_ "latihan-fiber/docs" // hasil swag init; mendaftarkan spesifikasi ke swagger
	"latihan-fiber/helper"
	"latihan-fiber/route"
)

const minSecretLength = 32

// Nilai flag -role: bagian mana dari aplikasi yang dijalankan.
//
// "all"    - API dan worker dalam satu proses (paling mudah untuk praktikum).
// "api"    - hanya HTTP API; worker dijalankan sebagai proses terpisah.
// "worker" - hanya worker; API dijalankan sebagai proses terpisah.
//
// Pemisahan ini penting untuk modul ini: worker yang sedang sibuk tidak
// boleh membuat API ikut lambat, dan API yang sedang restart tidak boleh
// membuat notifikasi yang tertunda hilang.
const (
	roleAPI    = "api"
	roleWorker = "worker"
	roleAll    = "all"
)

// Blok komentar di bawah ini adalah informasi UMUM dokumentasi API. swag
// hanya membacanya dari file yang disebut pada flag -g (bawaannya main.go).
//
// @title                       API KRS Praktikum Backend Lanjut
// @version                     1.0
// @description                 API pendaftaran kelas (KRS), ulasan kelas, dan notifikasi.
// @description                 Seluruh endpoint selain /auth memerlukan access token.
// @BasePath                    /api/v1
// @securityDefinitions.apikey  BearerAuth
// @in                          header
// @name                        Authorization
// @description                 Isi dengan: Bearer <access_token>
func main() {
	config.LoadEnv()
	logger := config.NewLogger()

	role := flag.String("role", config.GetEnv("APP_ROLE", roleAll),
		"bagian yang dijalankan: api, worker, atau all")
	flag.Parse()

	runAPI, runWorker, err := splitRole(*role)
	if err != nil {
		logger.Error("flag -role tidak valid", slog.String("error", err.Error()))
		os.Exit(1)
	}

	jwtSecret := config.GetEnv("JWT_SECRET", "")
	if len(jwtSecret) < minSecretLength {
		logger.Error("JWT_SECRET tidak diisi atau terlalu pendek", slog.Int("minimal_karakter", minSecretLength))
		os.Exit(1)
	}

	// Satu context untuk seluruh bagian. Ketika sinyal berhenti diterima,
	// context ini dibatalkan: relay outbox yang sedang berjalan berhenti
	// sendiri, tanpa perlu sinyal tambahan.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	pool, err := database.NewPool(context.Background())
	if err != nil {
		logger.Error("gagal terhubung ke database", slog.String("error", err.Error()))
		os.Exit(1)
	}
	defer pool.Close()

	logger.Info("aplikasi dijalankan",
		slog.String("role", *role),
		slog.Bool("api", runAPI),
		slog.Bool("worker", runWorker),
	)

	// Worker dimulai lebih dulu supaya notifikasi yang tertunda segera
	// dikerjakan, termasuk yang dibuat oleh request pertama.
	var shutdownWorker func()
	if runWorker {
		shutdownWorker = startWorker(ctx, logger, pool)
		defer shutdownWorker()
	}

	if !runAPI {
		<-ctx.Done()
		logger.Info("sinyal berhenti diterima")
		return
	}

	app, cleanup, err := newAPI(logger, pool)
	if err != nil {
		logger.Error("gagal menyiapkan aplikasi", slog.String("error", err.Error()))
		os.Exit(1)
	}
	defer cleanup()

	port := config.GetEnv("APP_PORT", "3000")
	go func() {
		if err := app.Listen(":" + port); err != nil {
			logger.Error("server berhenti", slog.String("error", err.Error()))
		}
	}()
	logger.Info("server berjalan", slog.String("port", port))

	<-ctx.Done()
	logger.Info("sinyal berhenti diterima, menutup server")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := app.ShutdownWithContext(shutdownCtx); err != nil {
		logger.Error("gagal menutup server dengan rapi", slog.String("error", err.Error()))
	}
	logger.Info("server berhenti dengan rapi")
}

// newAPI merakit bagian HTTP: MongoDB, JWT, repository, service, dan route.
// Fungsi ini mengembalikan rutinitas cleanup untuk menutup koneksi MongoDB.
func newAPI(logger *slog.Logger, pool *pgxpool.Pool) (*fiber.App, func(), error) {
	mongoClient, err := database.NewMongoClient(context.Background())
	if err != nil {
		return nil, nil, fmt.Errorf("menghubungkan MongoDB: %w", err)
	}
	cleanup := func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := mongoClient.Disconnect(ctx); err != nil {
			logger.Error("gagal menutup MongoDB", slog.String("error", err.Error()))
		}
	}

	mongoDB := mongoClient.Database(config.GetEnv("MONGO_DATABASE", "praktikum_backend"))
	portfolioRepository := repository.NewPortfolioRepository(mongoDB)
	indexCtx, cancelIndexes := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancelIndexes()
	if err := portfolioRepository.EnsureIndexes(indexCtx); err != nil {
		cleanup()
		return nil, nil, fmt.Errorf("membuat index MongoDB: %w", err)
	}

	jwtManager := helper.NewJWTManager(
		jwtSecret(),
		config.GetEnv("JWT_ISSUER", "praktikum-backend"),
		time.Duration(config.GetEnvInt("JWT_ACCESS_TTL_MINUTES", 15))*time.Minute,
	)

	studentRepository := repository.NewStudentRepository(pool)
	tokenRepository := repository.NewTokenRepository(pool)
	roleRepository := repository.NewRoleRepository(pool)
	courseRepository := repository.NewCourseRepository(pool)
	outboxRepository := repository.NewOutboxRepository(pool)
	notificationRepository := repository.NewNotificationRepository(pool)
	txManager := repository.NewTxManager(pool)

	rawPermissions, err := roleRepository.LoadPermissions(context.Background())
	if err != nil {
		cleanup()
		return nil, nil, fmt.Errorf("memuat permission: %w", err)
	}
	permissions := helper.NewPermissionSet(rawPermissions)
	logger.Info("permission dimuat", slog.Any("roles", permissions.KnownRoles()))

	studentService := service.NewStudentService(studentRepository, permissions)
	authService := service.NewAuthService(
		studentRepository, tokenRepository, jwtManager,
		time.Duration(config.GetEnvInt("JWT_REFRESH_TTL_DAYS", 7))*24*time.Hour,
		permissions,
	)
	portfolioService := service.NewPortfolioService(studentRepository, portfolioRepository, permissions)

	// Outbox repository ikut masuk ke CourseService: perubahan KRS dan
	// pencatatan event KRS harus terjadi dalam SATU transaction.
	courseService := service.NewCourseService(courseRepository, outboxRepository, txManager)

	// Inspector hanya dipakai endpoint admin. Membacanya tidak membuka
	// koneksi Redis lebih awal, sehingga API tetap bisa jalan saat Redis mati.
	notificationService := service.NewNotificationService(
		notificationRepository, outboxRepository, asynq.NewInspector(config.RedisOpt()),
	)

	app := config.NewApp(logger, route.Dependencies{
		EnableSwagger: config.GetEnv("SWAGGER_ENABLED", "false") == "true",
		Pool:          pool, Mongo: mongoClient, JWT: jwtManager, Permissions: permissions,
		StudentService: studentService, AuthService: authService, PortfolioService: portfolioService,
		CourseService: courseService, NotificationService: notificationService,
	})
	return app, cleanup, nil
}

// jwtSecret membaca secret dari .env. Secret divalidasi di main().
func jwtSecret() string {
	return config.GetEnv("JWT_SECRET", "")
}

// splitRole menerjemahkan nilai flag -role menjadi pasangan booleans.
func splitRole(role string) (api bool, worker bool, err error) {
	switch strings.ToLower(strings.TrimSpace(role)) {
	case roleAPI:
		return true, false, nil
	case roleWorker:
		return false, true, nil
	case roleAll:
		return true, true, nil
	default:
		return false, false, fmt.Errorf("role %q tidak dikenal (pilihan: %s, %s, %s)",
			role, roleAPI, roleWorker, roleAll)
	}
}
