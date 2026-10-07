package route

import (
	"errors"
	"siakad-uts/app/model"
	"siakad-uts/app/repository"
	"siakad-uts/app/service"
	"siakad-uts/helper"
	"siakad-uts/middleware"
	"strings"

	"github.com/gofiber/fiber/v2"
)

type Dependencies struct {
	Auth        service.AuthService
	Students    service.StudentService
	Courses     service.CourseService
	Enrollments service.EnrollmentService
	Users       repository.UserRepository
	JWT         *helper.JWT
}

func body(c *fiber.Ctx, dst any) error {
	if !strings.HasPrefix(strings.ToLower(c.Get("Content-Type")), "application/json") {
		return helper.Validation(map[string][]string{"body": {"Content-Type harus application/json"}})
	}
	if e := c.BodyParser(dst); e != nil {
		return helper.Validation(map[string][]string{"body": {"JSON tidak valid"}})
	}
	return nil
}
func Register(app *fiber.App, d Dependencies) {
	api := app.Group("/api/v1")
	limiter := middleware.NewLoginLimiter()
	api.Post("/auth/login", limiter.Guard, func(c *fiber.Ctx) error {
		var r model.LoginRequest
		if e := body(c, &r); e != nil {
			return e
		}
		data, e := d.Auth.Login(c.UserContext(), r)
		if e != nil {
			var apiErr *helper.APIError
			if errors.As(e, &apiErr) && apiErr.Status == 401 {
				limiter.Record(c.IP(), true)
			}
			return e
		}
		limiter.Record(c.IP(), false)
		return helper.OK(c, "Login berhasil", data)
	})
	api.Get("/auth/me", middleware.Auth(d.JWT, d.Users), func(c *fiber.Ctx) error {
		id := middleware.Identity(c).UserID
		data, e := d.Auth.Me(c.UserContext(), id)
		if e != nil {
			return e
		}
		identity := middleware.Identity(c)
		if identity.StudentID != nil {
			v, e := d.Students.Students.StudentByID(c.UserContext(), *identity.StudentID)
			if e != nil {
				return helper.Internal(e)
			}
			data = map[string]any{"user": data.(map[string]any)["user"], "student": map[string]any{"nim": v.NIM, "nama": v.Nama, "prodi": v.Prodi, "angkatan": v.Angkatan}}
		}
		return helper.OK(c, "Profil berhasil diambil", data)
	})
	secured := api.Group("", middleware.Auth(d.JWT, d.Users))
	students := secured.Group("/students")
	students.Get("", middleware.Role("admin"), func(c *fiber.Ctx) error {
		page, e := helper.PositiveQuery(c, "page", 1, 1000000)
		if e != nil {
			return e
		}
		per, e := helper.PositiveQuery(c, "per_page", 10, 50)
		if e != nil {
			return e
		}
		year := 0
		if c.Query("angkatan") != "" {
			year, e = helper.PositiveQuery(c, "angkatan", 0, 9999)
			if e != nil {
				return e
			}
		}
		sort := c.Query("sort", "nama")
		if sort != "nama" && sort != "-ipk_terakhir" {
			return helper.Validation(map[string][]string{"sort": {"Sort hanya nama atau -ipk_terakhir"}})
		}
		list, meta, e := d.Students.List(c.UserContext(), model.PageQuery{Page: page, PerPage: per, Prodi: c.Query("prodi"), Angkatan: year, Search: c.Query("search"), Sort: sort})
		if e != nil {
			return e
		}
		return helper.List(c, "Data mahasiswa berhasil diambil", list, meta)
	})
	students.Post("", middleware.Role("admin"), func(c *fiber.Ctx) error {
		var r model.CreateStudentRequest
		if e := body(c, &r); e != nil {
			return e
		}
		v, e := d.Students.Create(c.UserContext(), r)
		if e != nil {
			return e
		}
		return helper.Created(c, "Mahasiswa berhasil ditambahkan", v)
	})
	students.Get("/:id", func(c *fiber.Ctx) error {
		id, e := helper.ParamID(c)
		if e != nil {
			return e
		}
		v, e := d.Students.Detail(c.UserContext(), id, middleware.Identity(c))
		if e != nil {
			return e
		}
		return helper.OK(c, "Detail mahasiswa berhasil diambil", v)
	})
	students.Put("/:id", middleware.Role("admin"), func(c *fiber.Ctx) error {
		id, e := helper.ParamID(c)
		if e != nil {
			return e
		}
		var r model.UpdateStudentRequest
		if e = body(c, &r); e != nil {
			return e
		}
		v, e := d.Students.Update(c.UserContext(), id, r)
		if e != nil {
			return e
		}
		return helper.OK(c, "Mahasiswa berhasil diperbarui", v)
	})
	students.Delete("/:id", middleware.Role("admin"), func(c *fiber.Ctx) error {
		id, e := helper.ParamID(c)
		if e != nil {
			return e
		}
		if e = d.Students.Delete(c.UserContext(), id); e != nil {
			return e
		}
		return helper.NoContent(c)
	})
	secured.Get("/courses", func(c *fiber.Ctx) error {
		semester := 0
		var e error
		if c.Query("semester") != "" {
			semester, e = helper.PositiveQuery(c, "semester", 0, 14)
			if e != nil {
				return e
			}
		}
		available := c.Query("available")
		if available != "" && available != "true" && available != "false" {
			return helper.Validation(map[string][]string{"available": {"Gunakan true atau false"}})
		}
		v, e := d.Courses.List(c.UserContext(), model.CourseQuery{Semester: semester, Search: c.Query("search"), Available: available == "true"})
		if e != nil {
			return e
		}
		return helper.OK(c, "Mata kuliah berhasil diambil", v)
	})
	secured.Post("/enrollments", middleware.Role("mahasiswa"), func(c *fiber.Ctx) error {
		var r model.CreateEnrollmentRequest
		if e := body(c, &r); e != nil {
			return e
		}
		identity := middleware.Identity(c)
		if identity.StudentID == nil {
			return helper.Error(403, "Akses ditolak")
		}
		v, e := d.Enrollments.Enroll(c.UserContext(), *identity.StudentID, r)
		if e != nil {
			return e
		}
		return helper.Created(c, "Mata kuliah berhasil diambil", v)
	})
	secured.Delete("/enrollments/:id", middleware.Role("mahasiswa"), func(c *fiber.Ctx) error {
		id, e := helper.ParamID(c)
		if e != nil {
			return e
		}
		identity := middleware.Identity(c)
		if identity.StudentID == nil {
			return helper.Error(403, "Akses ditolak")
		}
		if e = d.Enrollments.Cancel(c.UserContext(), id, *identity.StudentID); e != nil {
			return e
		}
		return helper.NoContent(c)
	})
}
