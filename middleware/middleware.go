package middleware

import (
    "log/slog"
    "strings"
    "time"

    "github.com/gofiber/fiber/v2"
    "github.com/gofiber/fiber/v2/middleware/cors"
    "github.com/gofiber/fiber/v2/middleware/helmet"
    "github.com/gofiber/fiber/v2/middleware/recover"
    "github.com/gofiber/fiber/v2/middleware/requestid"

    "latihan-fiber/helper"
)

// Register memasang seluruh middleware yang berlaku untuk semua route. 
// URUTAN PENTING: middleware dieksekusi sesuai urutan pemasangan.
func Register(app *fiber.App, logger *slog.Logger, allowedOrigins string) { 
    app.Use(requestid.New()) 
    app.Use(recover.New()) 
    app.Use(helmet.New()) 
    app.Use(corsPolicy(allowedOrigins)) // BERUBAH: tidak lagi cors.New() 
    app.Use(RequestLogger(logger)) 
} 
  
// corsPolicy membatasi origin yang boleh memanggil API. 
// cors.New() tanpa konfigurasi mengizinkan SEMUA origin — cukup untuk 
// latihan pertemuan 2, tetapi tidak untuk API yang memakai token. 
func corsPolicy(allowedOrigins string) fiber.Handler { 
    if strings.TrimSpace(allowedOrigins) == "" { 
        allowedOrigins = "http://localhost:5173" 
    } 
  
    return cors.New(cors.Config{ 
        AllowOrigins: allowedOrigins, 
        AllowMethods: "GET,POST,PUT,PATCH,DELETE,OPTIONS", 
        AllowHeaders: "Origin,Content-Type,Accept,Authorization", 
    }) 
}

// RequestLogger mencatat setiap request ke log terstruktur.
// Perhatikan polanya: fungsi yang MENGEMBALIKAN fungsi (closure) —
// inilah cara middleware menerima dependensi dari luar.
//
// Jika RequireAuth sudah dipasang dan identitas tersimpan di Locals,
// user_id dan role akan ikut dicatat. Inilah inti Langkah 8: keputusan
// akses (403, 422 dari AssignRole) yang sebelumnya tak punya jejak
// sekarang dapat ditelusuri ke siapa yang meminta.
func RequestLogger(logger *slog.Logger) fiber.Handler {
    return func(c *fiber.Ctx) error {
        start := time.Now()

        err := c.Next()

        requestID, _ := c.Locals("requestid").(string)

        // Bentuk default: anonim (request publik, mis. /auth/login).
        attrs := []slog.Attr{
            slog.String("request_id", requestID),
            slog.String("method", c.Method()),
            slog.String("path", c.Path()),
            slog.Int("status", c.Response().StatusCode()),
            slog.Duration("duration", time.Since(start)),
            slog.String("ip", c.IP()),
        }

        // Hanya tambahkan identitas kalau RequireAuth sudah mengisi Locals.
        // Pemeriksaan dengan helper.CurrentUser (bukan type assertion mentah)
        // supaya kunci penyimpanan tetap satu sumber kebenaran.
        if user, ok := helper.CurrentUser(c); ok {
            attrs = append(attrs,
                slog.Int("user_id", user.StudentID),
                slog.String("role", user.Role),
            )
        }

        logger.LogAttrs(c.UserContext(), slog.LevelInfo,
            "http_request", attrs...)

        return err
    }
}

var methodsWithBody = map[string]bool{
    fiber.MethodPost:  true,
    fiber.MethodPut:   true,
    fiber.MethodPatch: true,
}

// RequireJSON menolak request berisi body yang Content-Type-nya bukan JSON. 
// Dipasang per grup route, bukan global.
func RequireJSON(c *fiber.Ctx) error {
    if methodsWithBody[c.Method()] {
        contentType := c.Get("Content-Type")

        if !strings.HasPrefix(contentType, fiber.MIMEApplicationJSON) {
            return helper.Fail(
                c,
                fiber.StatusUnsupportedMediaType,
                "Content-Type harus application/json",
            )
        }
    }

    return c.Next()
}