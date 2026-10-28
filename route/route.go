package route

import (
	"context"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.mongodb.org/mongo-driver/v2/mongo"

	"latihan-fiber/app/service"
	"latihan-fiber/helper"
	"latihan-fiber/middleware"
)

type Dependencies struct {
	Pool             *pgxpool.Pool
	Mongo            *mongo.Client
	JWT              *helper.JWTManager
	Permissions      *helper.PermissionSet
	StudentService   *service.StudentService
	AuthService      *service.AuthService
	PortfolioService *service.PortfolioService
}

func Register(app *fiber.App, deps Dependencies) {
	api := app.Group("/api/v1")
	api.Get("/health", healthCheck(deps.Pool, deps.Mongo))

	auth := api.Group("/auth", middleware.RequireJSON)
	auth.Post("/register", deps.AuthService.Register)
	auth.Post("/login", middleware.LoginRateLimiter(), deps.AuthService.Login)
	auth.Post("/refresh", deps.AuthService.Refresh)
	auth.Post("/logout", deps.AuthService.Logout)
	auth.Get("/me", middleware.RequireAuth(deps.JWT), deps.AuthService.Me)

	student := api.Group("/student", middleware.RequireJSON, middleware.RequireAuth(deps.JWT))
	perms := deps.Permissions
	student.Get("/", middleware.RequirePermission(perms, "student:list"), deps.StudentService.List)
	student.Post("/", middleware.RequirePermission(perms, "student:create"), deps.StudentService.Create)
	student.Patch("/:id/role", middleware.RequirePermission(perms, "student:role:assign"), deps.StudentService.AssignRole)
	student.Delete("/:id", middleware.RequirePermission(perms, "student:delete"), deps.StudentService.Delete)
	student.Get("/:id", deps.StudentService.Get)
	student.Put("/:id", deps.StudentService.Replace)
	student.Patch("/:id", deps.StudentService.Patch)

	// Modul 10: resource portofolio pada namespace plural /students.
	students := api.Group("/students", middleware.RequireAuth(deps.JWT))
	students.Get("/:id/portfolio", deps.PortfolioService.Get)
	students.Put("/:id/portfolio", middleware.RequireJSON, deps.PortfolioService.Replace)
	students.Post("/:id/portfolio/projects", middleware.RequireJSON, deps.PortfolioService.AddProject)
	students.Delete("/:id/portfolio/projects/:projectId", deps.PortfolioService.DeleteProject)

	portfolios := api.Group("/portfolios", middleware.RequireAuth(deps.JWT))
	portfolios.Get("/stats/technologies", deps.PortfolioService.TopTechnologies)
}

func healthCheck(pool *pgxpool.Pool, mongoClient *mongo.Client) fiber.Handler {
	return func(c *fiber.Ctx) error {
		ctx, cancel := context.WithTimeout(c.UserContext(), 2*time.Second)
		defer cancel()
		if err := pool.Ping(ctx); err != nil {
			return helper.ServiceUnavailable("PostgreSQL tidak dapat dihubungi")
		}
		if err := mongoClient.Ping(ctx, nil); err != nil {
			return helper.ServiceUnavailable("MongoDB tidak dapat dihubungi")
		}
		return helper.Ok(c, "server, PostgreSQL, dan MongoDB berjalan", fiber.Map{"timestamp": time.Now()})
	}
}
