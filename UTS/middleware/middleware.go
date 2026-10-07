package middleware

import (
	"errors"
	"log/slog"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/helmet"
	"github.com/gofiber/fiber/v2/middleware/recover"
	"github.com/gofiber/fiber/v2/middleware/requestid"

	"siakad-uts/helper"
)

func Register(app *fiber.App, logger *slog.Logger, allowedOrigins string) {
	app.Use(requestid.New())
	app.Use(recover.New())
	app.Use(helmet.New())
	app.Use(corsPolicy(allowedOrigins))
	app.Use(RequestLogger(logger))
}

func corsPolicy(allowedOrigins string) fiber.Handler {
	if strings.TrimSpace(allowedOrigins) == "" {
		allowedOrigins = "http://localhost:5173"
	}
	return cors.New(cors.Config{
		AllowOrigins: allowedOrigins,
		AllowMethods: "GET,POST,PUT,DELETE,OPTIONS",
		AllowHeaders: "Origin,Content-Type,Accept,Authorization",
	})
}

func RequestLogger(logger *slog.Logger) fiber.Handler {
	return func(c *fiber.Ctx) error {
		start := time.Now()
		err := c.Next()
		status := c.Response().StatusCode()
		if err != nil {
			var appErr *helper.APIError
			if errors.As(err, &appErr) {
				status = appErr.Status
			} else {
				status = fiber.StatusInternalServerError
			}
		}
		attrs := []slog.Attr{
			slog.String("request_id", RequestID(c)),
			slog.String("method", c.Method()),
			slog.String("path", c.Path()),
			slog.Int("status", status),
			slog.Duration("duration", time.Since(start)),
			slog.String("ip", c.IP()),
		}
		identity := Identity(c)
		if identity.UserID > 0 {
			attrs = append(attrs, slog.Int("user_id", identity.UserID), slog.String("role", identity.Role))
		}
		logger.LogAttrs(c.UserContext(), slog.LevelInfo, "http_request", attrs...)
		return err
	}
}

func RequestID(c *fiber.Ctx) string {
	if id, ok := c.Locals("requestid").(string); ok {
		return id
	}
	return c.GetRespHeader(fiber.HeaderXRequestID)
}
