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
	repo  repository.StudentRepository
	perms *helper.PermissionSet
}

// NewUserService menerima INTERFACE, bukan struct konkret.
func NewStudentService(
	repo repository.StudentRepository,
	perms *helper.PermissionSet,
) *StudentService {
	return &StudentService{repo: repo, perms: perms}
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

// get/students/:id tugas 6
func (s *StudentService) Get(c *fiber.Ctx) error {
	ctx, cancel := helper.ReqCtx(c)
	defer cancel()

	current, ok := helper.CurrentUser(c)
	if !ok {
		return helper.Fail(c, fiber.StatusUnauthorized, "belum terautentikasi")
	}

	id, valid := helper.ParamID(c)
	if !valid {
		return helper.Fail(c, fiber.StatusBadRequest,
			"id harus berupa angka positif")
	}

	// Pemeriksaan hak akses dilakukan SEBELUM data lengkap diambil.
	// Bila dibalik, penyerang dapat membedakan id yang ada dari yang
	// tidak ada berdasarkan perbedaan waktu tanggap 403 vs 404.
	ownerID, err := s.repo.FindOwnerID(ctx, id)
	if err != nil {
		return translateError(c, err, "gagal mengambil data student")
	}
	if !CanAccessStudent(current, ownerID, s.perms, "student:read:any") {
		return helper.Fail(c, fiber.StatusForbidden,
			"tidak berhak mengakses data student lain")
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

	current, ok := helper.CurrentUser(c)
	if !ok {
		return helper.Fail(c, fiber.StatusUnauthorized, "belum terautentikasi")
	}

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

	// owner_id SELALU diisi dari identitas pemanggil, bukan dari body.
	// Dengan begitu tidak ada celah mass assignment pada kolom pemilik.
	owner := current.StudentID
	student, err := s.repo.Create(ctx, model.Student{
		Name:     req.Name,
		NIM:      req.NIM,
		Grade:    req.Grade,
		IsActive: true,
		OwnerID:  &owner,
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

// replace/students/:id tugas 6
func (s *StudentService) Replace(c *fiber.Ctx) error {
	ctx, cancel := helper.ReqCtx(c)
	defer cancel()

	current, ok := helper.CurrentUser(c)
	if !ok {
		return helper.Fail(c, fiber.StatusUnauthorized, "belum terautentikasi")
	}

	id, valid := helper.ParamID(c)
	if !valid {
		return helper.Fail(c, fiber.StatusBadRequest,
			"id harus berupa angka positif")
	}

	// Pemeriksaan hak dilakukan sebelum query Update untuk mencegah
	// timing attack membedakan id yang ada dari yang tidak ada.
	ownerID, err := s.repo.FindOwnerID(ctx, id)
	if err != nil {
		return translateError(c, err, "gagal mengambil data student")
	}
	if !CanAccessStudent(current, ownerID, s.perms, "student:update:any") {
		return helper.Fail(c, fiber.StatusForbidden,
			"tidak berhak mengubah data student lain")
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

// patch/students/:id tugas 6
func (s *StudentService) Patch(c *fiber.Ctx) error {
	ctx, cancel := helper.ReqCtx(c)
	defer cancel()

	current, ok := helper.CurrentUser(c)
	if !ok {
		return helper.Fail(c, fiber.StatusUnauthorized, "belum terautentikasi")
	}

	id, valid := helper.ParamID(c)
	if !valid {
		return helper.Fail(c, fiber.StatusBadRequest,
			"id harus berupa angka positif")
	}

	// Pemeriksaan hak dilakukan sebelum query Update untuk mencegah
	// timing attack membedakan id yang ada dari yang tidak ada.
	ownerID, err := s.repo.FindOwnerID(ctx, id)
	if err != nil {
		return translateError(c, err, "gagal mengambil data student")
	}
	if !CanAccessStudent(current, ownerID, s.perms, "student:update:any") {
		return helper.Fail(c, fiber.StatusForbidden,
			"tidak berhak mengubah data student lain")
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

	existing, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return translateError(c, err, "gagal mengambil data student")
	}

	student, errs := ApplyPatch(existing, req)
	if len(errs) > 0 {
		return helper.FailValidation(c, errs)
	}

	student, err = s.repo.Update(ctx, student)
	if err != nil {
		return translateError(c, err, "gagal memperbarui student")
	}

	return helper.Ok(c, "student berhasil diperbarui sebagian", student)
}

// AssignRole melayani PATCH /students/:id/role.
//
// Endpoint ini tidak melakukan pemeriksaan kepemilikan: akses sudah dijaga
// oleh middleware student:role:assign, sehingga hanya role dengan permission
// tersebut (admin pada entity students) yang sampai ke sini. Pemeriksaan
// tambahan yang dilakukan di sini murni untuk INVARIAN, bukan otorisasi:
//
//  1. role yang dikirim harus role yang dikenal aplikasi (whitelist);
//  2. pemanggil tidak boleh mengubah rolenya sendiri.
//
// Keduanya ditangkap oleh ValidateAssignRole dan dikembalikan sebagai
// 422 supaya klien tahu bahwa permintaan ditolak oleh aturan domain,
// bukan oleh middleware.
func (s *StudentService) AssignRole(c *fiber.Ctx) error {
	ctx, cancel := helper.ReqCtx(c)
	defer cancel()

	current, ok := helper.CurrentUser(c)
	if !ok {
		return helper.Fail(c, fiber.StatusUnauthorized, "belum terautentikasi")
	}

	id, valid := helper.ParamID(c)
	if !valid {
		return helper.Fail(c, fiber.StatusBadRequest,
			"id harus berupa angka positif")
	}

	var req model.AssignRoleRequest
	if err := c.BodyParser(&req); err != nil {
		return helper.Fail(c, fiber.StatusBadRequest,
			"body harus berupa JSON yang valid")
	}

	if errs := ValidateAssignRole(current, id, req, s.perms); len(errs) > 0 {
		return helper.FailValidation(c, errs)
	}

	student, err := s.repo.UpdateRole(ctx, id, strings.TrimSpace(req.Role))
	if err != nil {
		return translateError(c, err, "gagal mengubah role student")
	}

	return helper.Ok(c, "role student berhasil diubah", student)
}

func (s *StudentService) Delete(c *fiber.Ctx) error {
	ctx, cancel := helper.ReqCtx(c)
	defer cancel()

	current, ok := helper.CurrentUser(c)
	if !ok {
		return helper.Fail(c, fiber.StatusUnauthorized, "belum terautentikasi")
	}

	id, valid := helper.ParamID(c)
	if !valid {
		return helper.Fail(c, fiber.StatusBadRequest,
			"id harus berupa angka positif")
	}

	// Memiliki permission student:delete TIDAK otomatis boleh menghapus
	// student yang terkait dengan akun pemanggil. Aturan ini mencegah
	// seorang admin menghapus student yang sedang menjadi identitasnya.
	if current.StudentID == id {
		return helper.Fail(c, fiber.StatusForbidden,
			"tidak boleh menghapus data student yang terkait dengan akun Anda")
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
