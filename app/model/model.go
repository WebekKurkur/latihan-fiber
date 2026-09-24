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

// Mulai pertemuan ini, aturan validasi ditulis sebagai tag pada struct. 
// Aturan dan bentuk data berada pada baris yang sama, sehingga menambah 
// satu field tanpa aturannya menjadi kelalaian yang langsung terlihat. 
type CreateStudentRequest struct { 
    Name   string  `json:"username" validate:"required,min=3,max=30,alphanum"` 
    NIM    string  `json:"email"    validate:"required,email,max=120"` 
    Grade  float64 `json:"password" validate:"required,min=8,max=72,nospace"` 
} 

  
type ReplaceStudentRequest struct { 
    Name   string  `json:"name"  validate:"required,min=3,max=30,alphanum"` 
    NIM    string  `json:"nim"    validate:"required,email,max=120"` 
    Grade  float64 `json:"grade" validate:"required,min=0,max=100"` 
	IsActive bool    `json:"is_active"`
} 
  
// Pada PATCH, pointer membedakan "tidak dikirim" (nil) dari "dikirim 
// bernilai kosong". omitnil dipilih karena ia menyatakan maksud yang 
// sebenarnya: lewati hanya bila nil. 
type PatchStudentRequest struct { 
    Name     *string  `json:"name,omitempty"  validate:"omitnil,min=3,max=30,alphanum"` 
    NIM      *string  `json:"nim,omitempty"    validate:"omitnil,email,max=120"` 
    Grade    *float64 `json:"grade,omitempty" validate:"omitnil,min=0,max=100"` 
    IsActive *bool    `json:"is_active,omitempty"` 
} 

// AssignRoleRequest dipakai endpoint PATCH /students/:id/role.
type AssignRoleRequest struct {
	Role string `json:"role"`
}

// Amplop baku untuk semua respons sukses.
type WebResponse struct {
	Success bool        `json:"success"`
	Message string      `json:"message"`
	Data    any         `json:"data,omitempty"`
	Meta    *Meta       `json:"meta,omitempty"`
	Cursor  *CursorMeta `json:"cursor,omitempty"`
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

// Cursor adalah penanda posisi pada keyset pagination.
type Cursor struct {
	CreatedAt time.Time `json:"-"`
	ID        int       `json:"-"`
}

// CursorQuery menggantikan ListQuery pada endpoint yang memakai cursor.
type CursorQuery struct {
	Limit    int
	Search   string
	IsActive *bool
	After    *Cursor
}

// CursorMeta menggantikan Meta pada endpoint yang memakai cursor.
//
// Perhatikan tidak adanya Total dan TotalPages. Keduanya tidak dapat
// disediakan tanpa COUNT(*) atas seluruh tabel — persis biaya yang ingin
// dihindari oleh pagination berbasis cursor.
type CursorMeta struct {
	Limit      int    `json:"limit"`
	NextCursor string `json:"next_cursor,omitempty"`
	HasMore    bool   `json:"has_more"`
}
