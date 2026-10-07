package config

import (
	"errors"
	"log/slog"

	"github.com/gofiber/fiber/v2"

	"siakad-uts/app/model"
	"siakad-uts/helper"
	"siakad-uts/middleware"
	"siakad-uts/route"
)

func NewApp(logger *slog.Logger, deps route.Dependencies) *fiber.App {
	app := fiber.New(fiber.Config{
		AppName:   GetEnv("APP_NAME", "siakad-uts"),
		BodyLimit: 1 << 20,
		ErrorHandler: func(c *fiber.Ctx, err error) error {
			var appErr *helper.APIError
			if !errors.As(err, &appErr) {
				var fiberErr *fiber.Error
				if errors.As(err, &fiberErr) && fiberErr.Code == fiber.StatusNotFound {
					appErr = &helper.APIError{Status: fiber.StatusNotFound, Message: "Endpoint tidak ditemukan"}
				} else {
					appErr = &helper.APIError{Status: fiber.StatusInternalServerError, Message: "Terjadi kesalahan server", Cause: err}
				}
			}
			if appErr.Status >= fiber.StatusInternalServerError {
				logger.Error("request_failed",
					slog.String("request_id", middleware.RequestID(c)),
					slog.String("path", c.Path()),
					slog.Int("status", appErr.Status),
					slog.Any("error", appErr.Cause),
				)
			}
			return c.Status(appErr.Status).JSON(model.Response{Success: false, Message: appErr.Message, Errors: appErr.Fields, RequestID: middleware.RequestID(c)})
		},
	})
	middleware.Register(app, logger, GetEnv("ALLOWED_ORIGINS", ""))
	route.Register(app, deps)
	return app
}
