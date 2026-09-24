package model

import "time"

type RegisterRequest struct {
	Name     string `json:"name"`
	NIM      string `json:"nim"`
	Password string `json:"password"`
	// Perhatikan: TIDAK ADA field Role di sini. Bila ada, siapa pun
	// dapat mendaftar sebagai admin (kerentanan mass assignment).
}

type LoginRequest struct {
	Name     string `json:"name"`
	Password string `json:"password"`
}

type RefreshRequest struct {
	RefreshToken string `json:"refresh_token"`
}

type TokenPair struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int    `json:"expires_in"` // detik
}

// RefreshToken adalah row pada table refresh_tokens.
// Perhatikan: yang disimpan TokenHash, bukan tokennya sendiri.
type RefreshToken struct {
	ID        int64
	StudentID int
	TokenHash string
	ExpiresAt time.Time
	RevokedAt *time.Time
	CreatedAt time.Time
}

// AuthStudents adalah identitas yang dibawa access token.
// Isinya sengaja minimal: hanya yang benar-benar diperlukan middleware.
type AuthStudents struct {
	StudentID int    `json:"students_id"`
	Name      string `json:"name"`
	Role      string `json:"role"`
}

// ProfileResponse dikembalikan oleh endpoint GET /auth/me.
//
// Bentuknya dipisah dari model.Student karena:
//  1. Kita ingin menyembunyikan kolom internal (password, owner_id)
//     tanpa menulis tag json:"-" di banyak tempat;
//  2. Daftar permission adalah TANGGUNG JAWAB endpoint /auth/me, bukan
//     entity student. Kalau besok ada endpoint lain yang juga butuh
//     profil, ia cukup membentuk responsnya sendiri dari sumber yang
//     sama tanpa ikut menyeret field permission.
type ProfileResponse struct {
	Student     Student   `json:"student"`
	Role        string    `json:"role"`
	Permissions []string  `json:"permissions"`
	IsActive    bool      `json:"is_active"`
	GeneratedAt time.Time `json:"generated_at"`
}
