package middleware

import (
	"errors"
	"siakad-uts/app/model"
	"siakad-uts/app/repository"
	"siakad-uts/helper"
	"strings"

	"github.com/gofiber/fiber/v2"
)

const identityKey = "siakad-identity"

func Identity(c *fiber.Ctx) model.Identity { v, _ := c.Locals(identityKey).(model.Identity); return v }
func Auth(jwt *helper.JWT, users repository.UserRepository) fiber.Handler {
	return func(c *fiber.Ctx) error {
		parts := strings.Fields(c.Get("Authorization"))
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
			return helper.Error(401, "Token Bearer diperlukan")
		}
		id, e := jwt.Parse(parts[1])
		if e != nil {
			return helper.Error(401, "Token tidak valid atau kedaluwarsa")
		}
		u, e := users.ByID(c.UserContext(), id)
		if errors.Is(e, repository.ErrNotFound) {
			return helper.Error(401, "Akun tidak aktif")
		}
		if e != nil {
			return helper.Internal(e)
		}
		c.Locals(identityKey, model.Identity{UserID: u.ID, Role: u.Role, StudentID: u.StudentID})
		return c.Next()
	}
}
func Role(role string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		if Identity(c).Role != role {
			return helper.Error(403, "Akses ditolak")
		}
		return c.Next()
	}
}
