package model

import "time"

// StudentPortfolio adalah dokumen portofolio yang disimpan di MongoDB.
// StudentID merujuk ke students.id di PostgreSQL; projects dan certificates
// di-embed karena ditampilkan bersama portofolionya.
type StudentPortfolio struct {
	StudentID    int                    `json:"student_id"`
	Headline     string                 `json:"headline"`
	Bio          string                 `json:"bio"`
	Projects     []PortfolioProject     `json:"projects"`
	Certificates []PortfolioCertificate `json:"certificates"`
	UpdatedAt    time.Time              `json:"updated_at"`
}

type PortfolioProject struct {
	ID           string   `json:"id"`
	Title        string   `json:"title"`
	Technologies []string `json:"technologies"`
	Link         string   `json:"link"`
	Year         int      `json:"year"`
}

type PortfolioCertificate struct {
	Name     string    `json:"name"`
	Issuer   string    `json:"issuer"`
	IssuedAt time.Time `json:"issued_at"`
}

type ReplacePortfolioRequest struct {
	Headline     string                      `json:"headline" validate:"required,min=3,max=100"`
	Bio          string                      `json:"bio" validate:"required,min=10,max=2000"`
	Projects     []PortfolioProjectInput     `json:"projects" validate:"max=20,dive"`
	Certificates []PortfolioCertificateInput `json:"certificates" validate:"max=50,dive"`
}

type PortfolioProjectInput struct {
	Title        string   `json:"title" validate:"required,min=2,max=120"`
	Technologies []string `json:"technologies" validate:"required,min=1,max=10,dive,required,min=1,max=40"`
	Link         string   `json:"link" validate:"omitempty,url,max=2048"`
	Year         int      `json:"year" validate:"required,min=1900,max=2100"`
}

type PortfolioCertificateInput struct {
	Name     string    `json:"name" validate:"required,min=2,max=160"`
	Issuer   string    `json:"issuer" validate:"required,min=2,max=160"`
	IssuedAt time.Time `json:"issued_at" validate:"required"`
}

type TechnologyStat struct {
	Technology string `json:"technology"`
	UsageCount int    `json:"usage_count"`
}

// PortfolioPage adalah response GET /portfolios.
//
// Sebelumnya response ini disusun dengan fiber.Map. Hasil JSON-nya sama
// persis, tetapi map tidak memiliki nama dan bentuk yang dapat dibaca swag,
// sehingga dokumentasi API tidak dapat menjelaskan isinya.
//
// Bentuknya punya struct sendiri karena field "meta" di sini berisi
// informasi cursor pagination — bukan nomor halaman seperti model.Meta.
type PortfolioPage struct {
	Success bool               `json:"success"`
	Message string             `json:"message"`
	Data    []StudentPortfolio `json:"data"`
	Meta    PortfolioPageMeta  `json:"meta"`
}

// PortfolioPageMeta adalah isi field "meta" pada PortfolioPage.
//
// Cursor dipegang sebagai "next_cursor": string yang dikirim kembali sebagai
// parameter query "cursor" untuk mengambil halaman berikutnya.
type PortfolioPageMeta struct {
	Limit      int    `json:"limit"`
	HasMore    bool   `json:"has_more"`
	NextCursor string `json:"next_cursor"`
}
