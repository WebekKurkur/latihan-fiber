package helper

import (
	"github.com/gofiber/fiber/v2"

	"latihan-fiber/app/model"
)

// LocalsAuthStudents adalah kunci penyimpanan identitas pemakai di dalam
// context request. Dibuat sebagai konstanta agar tidak ada salah ketik
// antara tempat menyimpan dan tempat membaca.
const LocalsAuthStudents = "AuthStudents"

// LocalsRequestID adalah kunci penyimpanan request id yang dipasang oleh
// middleware requestid dari Fiber.
const LocalsRequestID = "requestid"

// RequestID membaca request id dari Locals. Mengembalikan string kosong
// bila belum dipasang — bukan error, karena ErrorHandler tetap dapat
// menulis response tanpa request id.
func RequestID(c *fiber.Ctx) string {
	id, _ := c.Locals(LocalsRequestID).(string)
	return id
}

func CurrentUser(c *fiber.Ctx) (model.AuthStudents, bool) {
	user, ok := c.Locals(LocalsAuthStudents).(model.AuthStudents)
	return user, ok
}
