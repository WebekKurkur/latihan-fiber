package route

import (
	"context"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5/pgxpool"

	"latihan-fiber/app/service"
	"latihan-fiber/helper"
	"latihan-fiber/middleware"
)

type Dependencies struct {
	Pool           *pgxpool.Pool
	JWT            *helper.JWTManager
	Permissions    *helper.PermissionSet
	StudentService *service.StudentService
	AuthService    *service.AuthService
}

func Register(app *fiber.App, deps Dependencies) {
	api := app.Group("/api/v1")

	// --- publik ---
	api.Get("/health", healthCheck(deps.Pool))

	// --- autentikasi ---
	auth := api.Group("/auth", middleware.RequireJSON)
	auth.Post("/register", deps.AuthService.Register)
	auth.Post("/login", middleware.LoginRateLimiter(), deps.AuthService.Login)
	auth.Post("/refresh", deps.AuthService.Refresh)
	auth.Post("/logout", deps.AuthService.Logout)
	auth.Get("/me", middleware.RequireAuth(deps.JWT), deps.AuthService.Me)

	// --- wajib login, hak akses diperiksa per endpoint ---
	student := api.Group("/student",
		middleware.RequireJSON, middleware.RequireAuth(deps.JWT))

	perms := deps.Permissions

	// Hak dapat diputuskan tanpa melihat data -> middleware.
	student.Get("/",
		middleware.RequirePermission(perms, "student:list"),
		deps.StudentService.List)
	student.Post("/",
		middleware.RequirePermission(perms, "student:create"),
		deps.StudentService.Create)
	student.Patch("/:id/role",
		middleware.RequirePermission(perms, "student:role:assign"),
		deps.StudentService.AssignRole)
	student.Delete("/:id",
		middleware.RequirePermission(perms, "student:delete"),
		deps.StudentService.Delete)

	// Hak bergantung pada kepemilikan data -> diperiksa di service.
	student.Get("/:id", deps.StudentService.Get)
	student.Put("/:id", deps.StudentService.Replace)
	student.Patch("/:id", deps.StudentService.Patch)
}

func healthCheck(pool *pgxpool.Pool) fiber.Handler {
	return func(c *fiber.Ctx) error {
		ctx, cancel := context.WithTimeout(c.UserContext(), 2*time.Second)
		defer cancel()

		if err := pool.Ping(ctx); err != nil {
			return helper.Fail(
				c,
				fiber.StatusServiceUnavailable,
				"database tidak dapat dihubungi",
			)
		}

		return helper.Ok(c, "server dan database berjalan", fiber.Map{
			"timestamp": time.Now(),
		})
	}
}
