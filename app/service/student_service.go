package service

import (
	"errors"
	"strconv"
	"strings"

	"github.com/gofiber/fiber/v2"

	"latihan-fiber/app/model"
	"latihan-fiber/app/repository"
	"latihan-fiber/helper"
)

// StudentService memegang dua tanggung jawab sekaligus pada struktur baku
// mata kuliah ini: menerima *fiber.Ctx (peran controller) dan menjalankan
// business rules (peran use case).
type StudentService struct {
	repo repository.StudentRepository
}

// NewUserService menerima INTERFACE, bukan struct konkret.
func NewStudentService(repo repository.StudentRepository) *StudentService {
	return &StudentService{repo: repo}
}

func (s *StudentService) List(c *fiber.Ctx) error {
	ctx, cancel := helper.ReqCtx(c)
	defer cancel()

	q := helper.ParseListQuery(c)

	students, total, err := s.repo.FindAll(ctx, q)
	if err != nil {
		return helper.Fail(c, fiber.StatusInternalServerError,
			"gagal mengambil data student")
	}

	return helper.OkList(c, "daftar student berhasil diambil", students, &model.Meta{
		Page:       q.Page,
		Limit:      q.Limit,
		Total:      total,
		TotalPages: CountTotalPages(total, q.Limit),
	})
}

func (s *StudentService) Get(c *fiber.Ctx) error {
	ctx, cancel := helper.ReqCtx(c)
	defer cancel()

	id, valid := helper.ParamID(c)
	if !valid {
		return helper.Fail(c, fiber.StatusBadRequest,
			"id harus berupa angka positif")
	}

	student, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return translateError(c, err, "gagal mengambil data student")
	}

	return helper.Ok(c, "student ditemukan", student)
}

func (s *StudentService) Create(c *fiber.Ctx) error {
	ctx, cancel := helper.ReqCtx(c)
	defer cancel()

	var req model.CreateStudentRequest
	if err := c.BodyParser(&req); err != nil {
		return helper.Fail(c, fiber.StatusBadRequest,
			"body harus berupa JSON yang valid")
	}

	req.Name = strings.TrimSpace(req.Name)
	req.NIM = strings.TrimSpace(req.NIM)

	if errs := ValidateCreate(req); len(errs) > 0 {
		return helper.FailValidation(c, errs)
	}

	student, err := s.repo.Create(ctx, model.Student{
		Name:     req.Name,
		NIM:      req.NIM,
		Grade:    req.Grade,
		IsActive: true,
	})
	if err != nil {
		return translateError(c, err, "gagal menyimpan student")
	}

	return helper.Created(
		c,
		"student berhasil dibuat",
		student,
		"/api/v1/students/"+strconv.Itoa(student.ID),
	)
}

func (s *StudentService) Replace(c *fiber.Ctx) error {
	ctx, cancel := helper.ReqCtx(c)
	defer cancel()

	id, valid := helper.ParamID(c)
	if !valid {
		return helper.Fail(c, fiber.StatusBadRequest,
			"id harus berupa angka positif")
	}

	var req model.ReplaceStudentRequest
	if err := c.BodyParser(&req); err != nil {
		return helper.Fail(c, fiber.StatusBadRequest,
			"body harus berupa JSON yang valid")
	}

	if errs := ValidateReplace(req); len(errs) > 0 {
		return helper.FailValidation(c, errs)
	}

	student, err := s.repo.Update(ctx, model.Student{
		ID:       id,
		Name:     req.Name,
		NIM:      req.NIM,
		Grade:    req.Grade,
		IsActive: req.IsActive,
	})
	if err != nil {
		return translateError(c, err, "gagal memperbarui student")
	}

	return helper.Ok(c, "student berhasil diganti seluruhnya", student)
}

func (s *StudentService) Patch(c *fiber.Ctx) error {
	ctx, cancel := helper.ReqCtx(c)
	defer cancel()

	id, valid := helper.ParamID(c)
	if !valid {
		return helper.Fail(c, fiber.StatusBadRequest,
			"id harus berupa angka positif")
	}

	var req model.PatchStudentRequest
	if err := c.BodyParser(&req); err != nil {
		return helper.Fail(c, fiber.StatusBadRequest,
			"body harus berupa JSON yang valid")
	}

	if IsEmptyPatch(req) {
		return helper.Fail(c, fiber.StatusBadRequest,
			"tidak ada field yang diubah")
	}

	current, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return translateError(c, err, "gagal mengambil data student")
	}

	student, errs := ApplyPatch(current, req)
	if len(errs) > 0 {
		return helper.FailValidation(c, errs)
	}

	student, err = s.repo.Update(ctx, student)
	if err != nil {
		return translateError(c, err, "gagal memperbarui student")
	}

	return helper.Ok(c, "student berhasil diperbarui sebagian", student)
}

func (s *StudentService) Delete(c *fiber.Ctx) error {
	ctx, cancel := helper.ReqCtx(c)
	defer cancel()

	id, valid := helper.ParamID(c)
	if !valid {
		return helper.Fail(c, fiber.StatusBadRequest,
			"id harus berupa angka positif")
	}

	if err := s.repo.Delete(ctx, id); err != nil {
		return translateError(c, err, "gagal menghapus student")
	}

	return helper.NoContent(c)
}

func translateError(c *fiber.Ctx, err error, generalMessage string) error {
	switch {
	case errors.Is(err, repository.ErrNotFound):
		return helper.Fail(c, fiber.StatusNotFound, "student tidak ditemukan")
	case errors.Is(err, repository.ErrDuplicate):
		return helper.Fail(c, fiber.StatusConflict, "nim sudah dipakai")
	default:
		return helper.Fail(c, fiber.StatusInternalServerError, generalMessage)
	}
}
