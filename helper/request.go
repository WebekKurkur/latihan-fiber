package helper

import (
	"context"
	"strconv"
	"strings"
	"time"

	"latihan-fiber/app/model"

	"github.com/gofiber/fiber/v2"
)

var allowedSort = map[string]bool{
	"id": true,
}

// reqCtx memberi batas waktu untuk setiap operasi basis data.
// Tanpa batas waktu, satu query yang menggantung dapat menahan koneksi
// selamanya dan lama-lama menghabiskan seluruh isi pool.
func reqCtx(c *fiber.Ctx) (context.Context, context.CancelFunc) {
	return context.WithTimeout(c.UserContext(), 5*time.Second)
}

// paramID membaca :id dari URL dan memastikan nilainya angka positif.
// Mengembalikan (id, true) bila valid; (0, false) bila bukan angka
// atau bukan bilangan positif.
func paramID(c *fiber.Ctx) (int, bool) {
	raw := c.Params("id")
	id, err := strconv.Atoi(raw)
	if err != nil || id <= 0 {
		return 0, false
	}
	return id, true
}

// parseListQuery membaca query string dan memberi nilai bawaan yang aman.
// Aturan pentingnya: masukan dari klien tidak pernah dipercaya begitu saja.
func parseListQuery(c *fiber.Ctx) model.ListQuery {
	q := model.ListQuery{
		Page:   c.QueryInt("page", 1),
		Limit:  c.QueryInt("limit", 10),
		Search: strings.TrimSpace(c.Query("search")),
		Sort:   c.Query("sort", "id"),
		Order:  strings.ToLower(c.Query("order", "asc")),
	}

	if q.Page < 1 {
		q.Page = 1
	}
	if q.Limit < 1 {
		q.Limit = 10
	}
	if q.Limit > 100 { // batas atas wajib ada
		q.Limit = 100
	}
	if !allowedSort[q.Sort] { // daftar putih, bukan daftar hitam
		q.Sort = "id"
	}
	if q.Order != "desc" {
		q.Order = "asc"
	}

	if raw := c.Query("is_active"); raw != "" {
		if v, err := strconv.ParseBool(raw); err == nil {
			q.IsActive = &v
		}
	}

	return q
}
