package helper

import (
	"github.com/gofiber/fiber/v2"

	"latihan-fiber/app/model"
)

// Bentuk-bentuk response berhasil dipusatkan di sini karena semuanya
// menghasilkan envelope yang sama (WebResponse). Kegagalan TIDAK dibuat
// di sini: handler cukup mengembalikan *helper.AppError dan ErrorHandler
// terpusat yang menulis bentuk kegagalannya.

func Ok(c *fiber.Ctx, message string, data any) error {
	return c.Status(fiber.StatusOK).JSON(model.WebResponse{
		Success: true, Message: message, Data: data,
	})
}

func OkList(c *fiber.Ctx, message string, data any, meta *model.Meta) error {
	return c.Status(fiber.StatusOK).JSON(model.WebResponse{
		Success: true, Message: message, Data: data, Meta: meta,
	})
}

func Created(c *fiber.Ctx, message string, data any, location string) error {
	c.Set("Location", location) // memberi tahu klien di mana sumber daya baru berada
	return c.Status(fiber.StatusCreated).JSON(model.WebResponse{
		Success: true, Message: message, Data: data,
	})
}

func NoContent(c *fiber.Ctx) error {
	return c.SendStatus(fiber.StatusNoContent) // 204: berhasil, tanpa body
}
