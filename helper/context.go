package helper

import (
	"github.com/gofiber/fiber/v2"

	"latihan-fiber/app/model"
)

// LocalsAuthStudents adalah kunci penyimpanan identitas pemakai di dalam
// context request. Dibuat sebagai konstanta agar tidak ada salah ketik
// antara tempat menyimpan dan tempat membaca.
const LocalsAuthStudents = "AuthStudents"

func CurrentUser(c *fiber.Ctx) (model.AuthStudents, bool) {
	user, ok := c.Locals(LocalsAuthStudents).(model.AuthStudents)
	return user, ok
}
