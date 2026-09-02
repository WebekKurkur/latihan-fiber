package main

import (
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
)

var students []Student
var nextID = 1

// findStudentIndex mencari student berdasarkan ID.
func findStudentIndex(id int) int {
	for i := range students {
		if students[i].ID == id {
			return i
		}
	}
	return -1
}

// findStudentByNIM mencari student berdasarkan NIM (identifier unik).
func findStudentByNIM(nim string) int {
	for i := range students {
		if students[i].NIM == nim {
			return i
		}
	}
	return -1
}

// cocokPencarian memeriksa apakah kata kunci muncul di name atau NIM.
func cocokPencarian(s Student, kata string) bool {
	kata = strings.ToLower(kata)
	return strings.Contains(strings.ToLower(s.Name), kata) ||
		strings.Contains(strings.ToLower(s.NIM), kata)
}

// validateNIM memvalidasi format NIM (minimal 8 digit angka).
func validateNIM(nim string) bool {
	if len(nim) < 8 {
		return false
	}
	for _, ch := range nim {
		if ch < '0' || ch > '9' {
			return false
		}
	}
	return true
}

func paramID(c *fiber.Ctx) (int, bool) {
	id, err := strconv.Atoi(c.Params("id"))
	if err != nil || id < 1 {
		return 0, false
	}
	return id, true
}

func listStudents(c *fiber.Ctx) error {
	q := parseListQuery(c)

	// 1) Saring
	hasil := []Student{}
	for _, s := range students {
		if q.IsActive != nil && s.IsActive != *q.IsActive {
			continue
		}
		if q.Search != "" && !cocokPencarian(s, q.Search) {
			continue
		}
		hasil = append(hasil, s)
	}

	// 2) Urutkan
	sort.SliceStable(hasil, func(i, j int) bool {
		var lebihKecil bool
		switch q.Sort {
		case "name":
			lebihKecil = hasil[i].Name < hasil[j].Name
		case "nim":
			lebihKecil = hasil[i].NIM < hasil[j].NIM
		case "grade":
			lebihKecil = hasil[i].Grade < hasil[j].Grade
		case "created_at":
			lebihKecil = hasil[i].CreatedAt.Before(hasil[j].CreatedAt)
		default:
			lebihKecil = hasil[i].ID < hasil[j].ID
		}
		if q.Order == "desc" {
			return !lebihKecil
		}
		return lebihKecil
	})

	// 3) Potong sesuai halaman
	total := len(hasil)
	totalPages := (total + q.Limit - 1) / q.Limit
	mulai := (q.Page - 1) * q.Limit
	if mulai > total {
		mulai = total
	}
	akhir := mulai + q.Limit
	if akhir > total {
		akhir = total
	}

	return okList(c, "daftar mahasiswa berhasil diambil", hasil[mulai:akhir], &Meta{
		Page: q.Page, Limit: q.Limit, Total: total, TotalPages: totalPages,
	})
}

func getStudent(c *fiber.Ctx) error {
	id, valid := paramID(c)
	if !valid {
		return fail(c, fiber.StatusBadRequest, "id harus berupa angka positif")
	}

	i := findStudentIndex(id)
	if i == -1 {
		return fail(c, fiber.StatusNotFound, "mahasiswa tidak ditemukan")
	}

	return ok(c, "mahasiswa ditemukan", students[i])
}

func createStudent(c *fiber.Ctx) error {
	var req CreateStudentRequest
	if err := c.BodyParser(&req); err != nil {
		return fail(c, fiber.StatusBadRequest, "body harus berupa JSON yang valid")
	}

	errs := map[string]string{}
	req.Name = strings.TrimSpace(req.Name)
	req.NIM = strings.TrimSpace(req.NIM)

	if req.Name == "" {
		errs["name"] = "wajib diisi"
	}
	if req.NIM == "" {
		errs["nim"] = "wajib diisi"
	} else if !validateNIM(req.NIM) {
		errs["nim"] = "format NIM tidak valid"
	} else if findStudentByNIM(req.NIM) != -1 {
		return failConflict(c, "NIM sudah terdaftar: "+req.NIM)
	}
	if req.Grade < 0 || req.Grade > 100 {
		errs["grade"] = "nilai harus antara 0-100"
	}
	if len(errs) > 0 {
		return failValidation(c, errs)
	}

	baru := Student{
		ID:        nextID,
		Name:      req.Name,
		NIM:       req.NIM,
		Grade:     req.Grade,
		IsActive:  true,
		CreatedAt: time.Now(),
	}
	students = append(students, baru)
	nextID++

	return created(c, "mahasiswa berhasil dibuat", baru,
		"/api/v1/students/"+strconv.Itoa(baru.ID))
}

// replaceStudent — PUT: ganti seluruh isi.
func replaceStudent(c *fiber.Ctx) error {
	id, valid := paramID(c)
	if !valid {
		return fail(c, fiber.StatusBadRequest, "id harus berupa angka positif")
	}

	i := findStudentIndex(id)
	if i == -1 {
		return fail(c, fiber.StatusNotFound, "mahasiswa tidak ditemukan")
	}

	var req ReplaceStudentRequest
	if err := c.BodyParser(&req); err != nil {
		return fail(c, fiber.StatusBadRequest, "body harus berupa JSON yang valid")
	}

	errs := map[string]string{}
	req.Name = strings.TrimSpace(req.Name)
	req.NIM = strings.TrimSpace(req.NIM)

	if req.Name == "" {
		errs["name"] = "wajib diisi pada PUT"
	}
	if req.NIM == "" {
		errs["nim"] = "wajib diisi pada PUT"
	} else if !validateNIM(req.NIM) {
		errs["nim"] = "format NIM tidak valid"
	} else if students[i].NIM != req.NIM && findStudentByNIM(req.NIM) != -1 {
		return failConflict(c, "NIM sudah terdaftar: "+req.NIM)
	}
	if req.Grade < 0 || req.Grade > 100 {
		errs["grade"] = "nilai harus antara 0-100"
	}
	if len(errs) > 0 {
		return failValidation(c, errs)
	}

	students[i].Name = req.Name
	students[i].NIM = req.NIM
	students[i].Grade = req.Grade
	students[i].IsActive = req.IsActive

	return ok(c, "mahasiswa berhasil diganti seluruhnya", students[i])
}

// patchStudent — PATCH: hanya ubah field yang dikirim.
func patchStudent(c *fiber.Ctx) error {
	id, valid := paramID(c)
	if !valid {
		return fail(c, fiber.StatusBadRequest, "id harus berupa angka positif")
	}

	i := findStudentIndex(id)
	if i == -1 {
		return fail(c, fiber.StatusNotFound, "mahasiswa tidak ditemukan")
	}

	var req PatchStudentRequest
	if err := c.BodyParser(&req); err != nil {
		return fail(c, fiber.StatusBadRequest, "body harus berupa JSON yang valid")
	}

	if req.Name == nil && req.NIM == nil && req.Grade == nil && req.IsActive == nil {
		return fail(c, fiber.StatusBadRequest, "tidak ada field yang diubah")
	}

	if req.Name != nil {
		if strings.TrimSpace(*req.Name) == "" {
			return failValidation(c, map[string]string{"name": "tidak boleh kosong"})
		}
		students[i].Name = *req.Name
	}
	if req.NIM != nil {
		if strings.TrimSpace(*req.NIM) == "" {
			return failValidation(c, map[string]string{"nim": "tidak boleh kosong"})
		}
		if !validateNIM(*req.NIM) {
			return failValidation(c, map[string]string{"nim": "format NIM tidak valid"})
		}
		if students[i].NIM != *req.NIM && findStudentByNIM(*req.NIM) != -1 {
			return failConflict(c, "NIM sudah terdaftar: "+*req.NIM)
		}
		students[i].NIM = *req.NIM
	}
	if req.Grade != nil {
		if *req.Grade < 0 || *req.Grade > 100 {
			return failValidation(c, map[string]string{"grade": "nilai harus antara 0-100"})
		}
		students[i].Grade = *req.Grade
	}
	if req.IsActive != nil {
		students[i].IsActive = *req.IsActive
	}

	return ok(c, "mahasiswa berhasil diperbarui sebagian", students[i])
}

func deleteStudent(c *fiber.Ctx) error {
	id, valid := paramID(c)
	if !valid {
		return fail(c, fiber.StatusBadRequest, "id harus berupa angka positif")
	}

	i := findStudentIndex(id)
	if i == -1 {
		return fail(c, fiber.StatusNotFound, "mahasiswa tidak ditemukan")
	}

	students = append(students[:i], students[i+1:]...)

	return noContent(c) // 204: berhasil, tanpa body
}
