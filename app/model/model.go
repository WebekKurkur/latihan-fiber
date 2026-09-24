package model

import "time"

// Student adalah entity utama. NIM berfungsi sebagai penanda unik.
type Student struct {
	ID        int       `json:"id"`
	Name      string    `json:"name"`
	NIM       string    `json:"nim"`
	Grade     float64   `json:"grade"`
	Password  string    `json:"-"`
	Role      string    `json:"role"`
	IsActive  bool      `json:"is_active"`
	CreatedAt time.Time `json:"created_at"`
	// OwnerID menandai student yang mendaftarkan data ini.
	// Dikirim ke JSON hanya bila tersedia; data lama belum memiliki pemilik.
	OwnerID *int `json:"owner_id,omitempty"`
}

// POST — semua field wajib
type CreateStudentRequest struct {
	Name  string  `json:"name"`
	NIM   string  `json:"nim"`
	Grade float64 `json:"grade"`
}

// PUT — ganti seluruh isi, jadi field bertipe biasa dan semuanya wajib
type ReplaceStudentRequest struct {
	Name     string  `json:"name"`
	NIM      string  `json:"nim"`
	Grade    float64 `json:"grade"`
	IsActive bool    `json:"is_active"`
}

// PATCH — ubah sebagian, jadi field bertipe pointer supaya bisa dibedakan
// antara "tidak dikirim" (nil) dan "dikirim bernilai kosong"
type PatchStudentRequest struct {
	Name     *string  `json:"name,omitempty"`
	NIM      *string  `json:"nim,omitempty"`
	Grade    *float64 `json:"grade,omitempty"`
	IsActive *bool    `json:"is_active,omitempty"`
}

// AssignRoleRequest dipakai endpoint PATCH /students/:id/role.
type AssignRoleRequest struct {
	Role string `json:"role"`
}

// Amplop baku untuk semua respons sukses.
type WebResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
	Meta    *Meta  `json:"meta,omitempty"`
}

// ErrorResponse adalah bentuk baku untuk SETIAP kegagalan.
// Ia hanya dibentuk oleh ErrorHandler terpusat; service tidak menulis
// response kegagalan secara langsung.
//
// field "errors" pada versi lama diganti "fields" agar sesuai dengan
// Validator.ValidationErrors (satu pesan per field) dan tidak rancu
// dengan array of objects. Penambahan code + request_id adalah
// breaking change yang sengaja: code adalah kontrak, request_id
// adalah pengikat antara laporan klien dan log server.
type ErrorResponse struct {
	Success   bool              `json:"success"`
	Code      string            `json:"code"`
	Message   string            `json:"message"`
	Fields    map[string]string `json:"fields,omitempty"`
	RequestID string            `json:"request_id,omitempty"`
}

type Meta struct {
	Page       int `json:"page"`
	Limit      int `json:"limit"`
	Total      int `json:"total"`
	TotalPages int `json:"total_pages"`
}

type ListQuery struct {
	Page     int
	Limit    int
	Search   string
	Sort     string
	Order    string
	IsActive *bool
}

// Offset menghitung berapa baris yang dilewati untuk halaman ini.
// Perhitungan ini pindah ke sini karena kini dipakai langsung oleh SQL.
func (q ListQuery) Offset() int {
	return (q.Page - 1) * q.Limit
}
