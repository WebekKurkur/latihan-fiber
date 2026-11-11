package service

import (
	"errors"
	"fmt"
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

// List godoc
//
// @Summary      Daftar seluruh student
// @Description  Cursor pagination: kirim cursor.next_cursor sebagai parameter cursor untuk halaman berikutnya. Format CSV tersedia bila header Accept: text/csv.
// @Tags         student
// @Produce      json
// @Param        limit      query  int     false  "Jumlah per halaman, 1 sampai 100"  default(10)
// @Param        search     query  string  false  "Saring berdasarkan kata kunci"
// @Param        is_active  query  bool    false  "Saring berdasarkan status aktif"
// @Param        cursor     query  string  false  "Cursor: halaman sebelumnya"
// @Success      200  {object}  model.WebResponse{data=[]model.Student,cursor=model.CursorMeta}
// @Failure      400  {object}  model.ErrorResponse  "Cursor tidak sah"
// @Failure      403  {object}  model.ErrorResponse
// @Failure      406  {object}  model.ErrorResponse  "Format yang diminta tidak tersedia"
// @Security     BearerAuth
// @Router       /student [get]
func (s *StudentService) List(c *fiber.Ctx) error {
	ctx, cancel := helper.ReqCtx(c)
	defer cancel()

	// Format dipilih SEBELUM query dijalankan. Bila client meminta format
	// yang tidak dapat kita hasilkan, tidak ada gunanya membebani database
	// untuk hasil yang akan dibuang.
	format, err := helper.Negotiate(c, helper.FormatJSON, helper.FormatCSV)
	if err != nil {
		return err
	}

	q, err := helper.ParseCursorQuery(c)
	if err != nil {
		return helper.BadRequest("cursor tidak sah")
	}

	rows, err := s.repo.FindAfterCursor(ctx, q)
	if err != nil {
		return helper.Internal(err)
	}

	// Baris tambahan hasil limit+1 dipotong di sini. Ia hanya penanda bahwa
	// masih ada halaman berikutnya, bukan bagian dari halaman ini.
	hasMore := len(rows) > q.Limit
	if hasMore {
		rows = rows[:q.Limit]
	}
	meta := &model.CursorMeta{Limit: q.Limit, HasMore: hasMore}
	if hasMore && len(rows) > 0 {
		last := rows[len(rows)-1]
		meta.NextCursor = helper.EncodeCursor(last.CreatedAt, last.ID)
	}

	if format == helper.FormatCSV {
		return helper.WriteUsersCSV(c, rows)
	}
	return helper.SuccessCursor(c, "daftar student berhasil diambil", rows, meta)
}

// get/students/:id tugas 6

// Get godoc
//
// @Summary      Detail satu student
// @Tags         student
// @Produce      json
// @Param        id   path  int  true  "ID student"
// @Success      200  {object}  model.WebResponse{data=model.Student}
// @Failure      400  {object}  model.ErrorResponse  "ID bukan angka positif"
// @Failure      403  {object}  model.ErrorResponse
// @Failure      404  {object}  model.ErrorResponse
// @Security     BearerAuth
// @Router       /student/{id} [get]
func (s *StudentService) Get(c *fiber.Ctx) error {
	ctx, cancel := helper.ReqCtx(c)
	defer cancel()

	current, ok := helper.CurrentUser(c)
	fmt.Println("current", current, "ok", ok)
	if !ok {
		return helper.Unauthorized("belum terautentikasi")
	}

	id, valid := helper.ParamID(c)
	if !valid {
		return helper.BadRequest(
			"id harus berupa angka positif")
	}

	// Pemeriksaan hak akses dilakukan SEBELUM data lengkap diambil.
	// Bila dibalik, penyerang dapat membedakan id yang ada dari yang
	// tidak ada berdasarkan perbedaan waktu tanggap 403 vs 404.
	// ownerID, err := s.repo.FindOwnerID(ctx, id)
	// if err != nil {
	// 	return translateError(err, "student")
	// }
	if !CanAccessStudent(current, &id, s.perms, "student:read:any") {
		return helper.Forbidden(
			"tidak berhak mengakses data student lain")
	}

	student, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return translateError(err, "student")
	}

	return helper.Ok(c, "student ditemukan", student)
}

// Create godoc
//
// @Summary      Membuat data student baru
// @Description  Owner_id (student yang membuat) SELALU diisi dari identitas pemanggil, bukan dari body, sehingga tidak ada celah mass assignment.
// @Tags         student
// @Accept       json
// @Produce      json
// @Param        body  body      model.CreateStudentRequest  true  "Data student"
// @Success      201  {object}  model.WebResponse{data=model.Student}
// @Failure      400  {object}  model.ErrorResponse  "Body bukan JSON yang valid"
// @Failure      403  {object}  model.ErrorResponse
// @Failure      422  {object}  model.ErrorResponse  "Validasi gagal; detail per field ada di fields"
// @Security     BearerAuth
// @Router       /student [post]
func (s *StudentService) Create(c *fiber.Ctx) error {
	ctx, cancel := helper.ReqCtx(c)
	defer cancel()

	current, ok := helper.CurrentUser(c)
	if !ok {
		return helper.Unauthorized("belum terautentikasi")
	}

	var req model.CreateStudentRequest
	if err := c.BodyParser(&req); err != nil {
		return helper.BadRequest(
			"body harus berupa JSON yang valid")
	}

	req.Name = strings.TrimSpace(req.Name)
	req.NIM = strings.TrimSpace(req.NIM)

	if errs := helper.ValidateStruct(req); errs != nil {
		return helper.Validation(errs)
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
		// Email diturunkan dari nama agar student yang dibuat admin
		// tetap punya alamat tujuan notifikasi KRS (Modul 11).
		Email: DeriveEmail(req.Name),
	})
	if err != nil {
		return translateError(err, "student")
	}

	return helper.Created(
		c,
		"student berhasil dibuat",
		student,
		"/api/v1/students/"+strconv.Itoa(student.ID),
	)
}

// replace/students/:id tugas 6

// Replace godoc
//
// @Summary      Mengganti seluruh data student
// @Description  Semua field wajib diisi; field yang tidak dikirim akan ditimpa dengan nilai default.
// @Tags         student
// @Accept       json
// @Produce      json
// @Param        id path  int                          true  "ID student"
// @Param        body  body  model.ReplaceStudentRequest  true  "Data student pengganti"
// @Success      200   {object}  model.WebResponse{data=model.Student}
// @Failure      400   {object}  model.ErrorResponse  "Body bukan JSON yang valid"
// @Failure      403   {object}  model.ErrorResponse
// @Failure      404   {object}  model.ErrorResponse
// @Failure      422   {object}  model.ErrorResponse  "Validasi gagal"
// @Security     BearerAuth
// @Router       /student/{id} [put]
func (s *StudentService) Replace(c *fiber.Ctx) error {
	ctx, cancel := helper.ReqCtx(c)
	defer cancel()

	current, ok := helper.CurrentUser(c)
	if !ok {
		return helper.Unauthorized("belum terautentikasi")
	}

	id, valid := helper.ParamID(c)
	if !valid {
		return helper.BadRequest(
			"id harus berupa angka positif")
	}

	// Pemeriksaan hak dilakukan sebelum query Update untuk mencegah
	// timing attack membedakan id yang ada dari yang tidak ada.
	/* ownerID, err := s.repo.FindOwnerID(ctx, id)
	if err != nil {
		return translateError(err, "student")
	} */
	if !CanAccessStudent(current, &id, s.perms, "student:update:any") {
		return helper.Forbidden(
			"tidak berhak mengubah data student lain")
	}

	var req model.ReplaceStudentRequest
	if err := c.BodyParser(&req); err != nil {
		return helper.BadRequest(
			"body harus berupa JSON yang valid")
	}

	if errs := helper.ValidateStruct(req); errs != nil {
		return helper.Validation(errs)
	}

	student, err := s.repo.Update(ctx, model.Student{
		ID:       id,
		Name:     req.Name,
		NIM:      req.NIM,
		Grade:    req.Grade,
		IsActive: req.IsActive,
	})
	if err != nil {
		return translateError(err, "student")
	}

	return helper.Ok(c, "student berhasil diganti seluruhnya", student)
}

// patch/students/:id tugas 6

// Patch godoc
//
// @Summary      Memperbarui sebagian data student
// @Description  Hanya field yang dikirim yang diubah. Field yang tidak dikirim tidak berubah.
// @Tags         student
// @Accept       json
// @Produce      json
// @Param        id    path  int                        true  "ID student"
// @Param        body  body  model.PatchStudentRequest  true  "Field yang diubah"
// @Success      200   {object}  model.WebResponse{data=model.Student}
// @Failure      400   {object}  model.ErrorResponse  "Body bukan JSON yang valid atau tidak ada field yang diubah"
// @Failure      403   {object}  model.ErrorResponse
// @Failure      404   {object}  model.ErrorResponse
// @Failure      422   {object}  model.ErrorResponse  "Validasi gagal"
// @Security     BearerAuth
// @Router       /student/{id} [patch]
func (s *StudentService) Patch(c *fiber.Ctx) error {
	ctx, cancel := helper.ReqCtx(c)
	defer cancel()

	current, ok := helper.CurrentUser(c)
	if !ok {
		return helper.Unauthorized("belum terautentikasi")
	}

	id, valid := helper.ParamID(c)
	if !valid {
		return helper.BadRequest(
			"id harus berupa angka positif")
	}

	// Pemeriksaan hak dilakukan sebelum query Update untuk mencegah
	// timing attack membedakan id yang ada dari yang tidak ada.
	/* ownerID, err := s.repo.FindOwnerID(ctx, id)
	if err != nil {
		return translateError(err, "student")
	} */
	if !CanAccessStudent(current, &id, s.perms, "student:update:any") {
		return helper.Forbidden(
			"tidak berhak mengubah data student lain")
	}

	var req model.PatchStudentRequest
	if err := c.BodyParser(&req); err != nil {
		return helper.BadRequest(
			"body harus berupa JSON yang valid")
	}

	if IsEmptyPatch(req) {
		return helper.BadRequest(
			"tidak ada field yang diubah")
	}

	existing, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return translateError(err, "student")
	}

	if errs := helper.ValidateStruct(req); errs != nil {
		return helper.Validation(errs)
	}

	student := ApplyPatch(existing, req)

	student, err = s.repo.Update(ctx, student)
	if err != nil {
		return translateError(err, "student")
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

// AssignRole godoc
//
// @Summary      Mengubah role seorang student
// @Description  Memerlukan permission student:role:assign. Pemanggil tidak boleh mengubah role-nya sendiri.
// @Tags         student
// @Accept       json
// @Produce      json
// @Param        id    path  int                       true  "ID student"
// @Param        body  body  model.AssignRoleRequest  true  "Role baru"
// @Success      200   {object}  model.WebResponse{data=model.Student}
// @Failure      400   {object}  model.ErrorResponse  "Body bukan JSON yang valid"
// @Failure      404   {object}  model.ErrorResponse
// @Failure      422   {object}  model.ErrorResponse  "Role tidak dikenal atau mengubah role sendiri"
// @Security     BearerAuth
// @Router       /student/{id}/role [patch]
func (s *StudentService) AssignRole(c *fiber.Ctx) error {
	ctx, cancel := helper.ReqCtx(c)
	defer cancel()

	current, ok := helper.CurrentUser(c)
	if !ok {
		return helper.Unauthorized("belum terautentikasi")
	}

	id, valid := helper.ParamID(c)
	if !valid {
		return helper.BadRequest(
			"id harus berupa angka positif")
	}

	var req model.AssignRoleRequest
	if err := c.BodyParser(&req); err != nil {
		return helper.BadRequest(
			"body harus berupa JSON yang valid")
	}

	if errs := ValidateAssignRole(current, id, req, s.perms); len(errs) > 0 {
		return helper.Validation(errs)
	}

	student, err := s.repo.UpdateRole(ctx, id, strings.TrimSpace(req.Role))
	if err != nil {
		return translateError(err, "student")
	}

	return helper.Ok(c, "role student berhasil diubah", student)
}

// Delete godoc
//
// @Summary      Menghapus data student
// @Description  Student yang terkait dengan akun pemanggil tidak boleh dihapus.
// @Tags         student
// @Produce      json
// @Param        id   path  int  true  "ID student"
// @Success      204  "Berhasil dihapus; tanpa body"
// @Failure      403  {object}  model.ErrorResponse
// @Failure      404  {object}  model.ErrorResponse
// @Security     BearerAuth
// @Router       /student/{id} [delete]
func (s *StudentService) Delete(c *fiber.Ctx) error {
	ctx, cancel := helper.ReqCtx(c)
	defer cancel()

	current, ok := helper.CurrentUser(c)
	if !ok {
		return helper.Unauthorized("belum terautentikasi")
	}

	id, valid := helper.ParamID(c)
	if !valid {
		return helper.BadRequest(
			"id harus berupa angka positif")
	}

	// Memiliki permission student:delete TIDAK otomatis boleh menghapus
	// student yang terkait dengan akun pemanggil. Aturan ini mencegah
	// seorang admin menghapus student yang sedang menjadi identitasnya.
	if current.StudentID == id {
		return helper.Forbidden(
			"tidak boleh menghapus data student yang terkait dengan akun Anda")
	}

	if err := s.repo.Delete(ctx, id); err != nil {
		return translateError(err, "student")
	}

	return helper.NoContent(c)
}

// translateError mengubah error milik repository menjadi AppError.
//
// Perhatikan tanda tangannya: tidak ada fiber.Ctx. Fungsi ini hanya
// menerjemahkan satu jenis error menjadi jenis lain, dan tidak tahu
// apa pun tentang HTTP. Yang tidak dikenali menjadi Internal — fail
// closed: lebih baik membalas 500 daripada menebak-nebak status.
func translateError(err error, entity string) error {
	switch {
	case errors.Is(err, repository.ErrNotFound):
		return helper.NotFound(entity + " tidak ditemukan")
	case errors.Is(err, repository.ErrDuplicate):
		return helper.Conflict("nim sudah dipakai")
	case errors.Is(err, repository.ErrEmailDuplicate):
		return helper.Conflict("email sudah dipakai")
	default:
		// Tidak dikenali. Modul Bagian A menjamin "fail closed" — bila
		// ragu, jawab 500 dengan pesan seragam. Pesan asli tersimpan
		// pada cause dan hanya masuk ke log melalui ErrorHandler.
		return helper.Internal(err)
	}
}
