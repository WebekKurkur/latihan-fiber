package helper

import (
	"fmt"
	"github.com/gofiber/fiber/v2"
	"siakad-uts/app/model"
)

type APIError struct {
	Status  int
	Message string
	Fields  map[string][]string
	Cause   error
}

func (e *APIError) Error() string            { return e.Message }
func Error(status int, message string) error { return &APIError{Status: status, Message: message} }
func Validation(fields map[string][]string) error {
	return &APIError{Status: 422, Message: "Validasi gagal", Fields: fields}
}
func Internal(err error) error {
	return &APIError{Status: 500, Message: "Terjadi kesalahan server", Cause: fmt.Errorf("%w", err)}
}
func OK(c *fiber.Ctx, message string, data any) error {
	return c.Status(200).JSON(model.Response{Success: true, Message: message, Data: data})
}
func Created(c *fiber.Ctx, message string, data any) error {
	return c.Status(201).JSON(model.Response{Success: true, Message: message, Data: data})
}
func NoContent(c *fiber.Ctx) error { return c.SendStatus(204) }
func List(c *fiber.Ctx, message string, data any, meta *model.Meta) error {
	return c.JSON(model.Response{Success: true, Message: message, Data: data, Meta: meta})
}
