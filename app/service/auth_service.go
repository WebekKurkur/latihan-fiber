package service

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"

	"latihan-fiber/app/model"
	"latihan-fiber/app/repository"
	"latihan-fiber/helper"
)

const refreshTokenBytes = 32

type AuthService struct {
	student    repository.StudentRepository
	tokens     repository.TokenRepository
	jwt        *helper.JWTManager
	refreshTTL time.Duration
	perms      *helper.PermissionSet
}

func NewAuthService(
	student repository.StudentRepository,
	tokens repository.TokenRepository,
	jwtManager *helper.JWTManager,
	refreshTTL time.Duration,
	perms *helper.PermissionSet,
) *AuthService {
	return &AuthService{
		student: student, tokens: tokens, jwt: jwtManager,
		refreshTTL: refreshTTL, perms: perms,
	}
}

func (s *AuthService) Register(c *fiber.Ctx) error {
	ctx, cancel := helper.ReqCtx(c)
	defer cancel()

	var req model.RegisterRequest
	if err := c.BodyParser(&req); err != nil {
		return helper.Fail(c, fiber.StatusBadRequest, "body harus berupa JSON yang valid")
	}

	req.Name = strings.TrimSpace(req.Name)
	req.NIM = strings.TrimSpace(req.NIM)

	if errs := ValidateRegister(req); len(errs) > 0 {
		return helper.FailValidation(c, errs)
	}

	// Password DI-HASH sebelum menyentuh database. Nilai aslinya tidak
	// pernah disimpan, tidak pernah di-log, dan tidak pernah dikirim balik.
	hashed, err := helper.HashPassword(req.Password)
	if err != nil {
		return helper.Fail(c, fiber.StatusInternalServerError, "gagal memproses password")
	}

	// Role selalu ditentukan server, tidak pernah diambil dari request.
	created, err := s.student.Create(ctx, model.Student{
		Name:     req.Name,
		NIM:      req.NIM,
		Password: hashed,
		Role:     "user",
		IsActive: true,
	})
	if err != nil {
		if errors.Is(err, repository.ErrDuplicate) {
			return helper.Fail(c, fiber.StatusConflict, "username sudah dipakai")
		}
		return helper.Fail(c, fiber.StatusInternalServerError, "gagal mendaftarkan user")
	}

	return helper.Created(c, "pendaftaran berhasil", created,
		"/api/v1/students/"+strconv.Itoa(created.ID))
}

func (s *AuthService) Login(c *fiber.Ctx) error {
	ctx, cancel := helper.ReqCtx(c)
	defer cancel()

	var req model.LoginRequest
	if err := c.BodyParser(&req); err != nil {
		return helper.Fail(c, fiber.StatusBadRequest, "body harus berupa JSON yang valid")
	}

	if errs := ValidateLogin(req); len(errs) > 0 {
		return helper.FailValidation(c, errs)
	}

	student, err := s.student.FindByUsername(ctx, strings.TrimSpace(req.Name))
	if err != nil {
		// Username tidak ditemukan. Tetap jalankan pemeriksaan palsu agar
		// waktu tanggapnya mirip dengan kasus password salah, lalu jawab
		// dengan pesan yang SAMA PERSIS. Membedakan keduanya sama saja
		// dengan memberi tahu penyerang username mana yang terdaftar.
		helper.VerifyDummyPassword(req.Password)
		return helper.Fail(c, fiber.StatusUnauthorized, "username atau password salah")
	}

	if !helper.VerifyPassword(student.Password, req.Password) {
		return helper.Fail(c, fiber.StatusUnauthorized, "username atau password salah")
	}

	if !student.IsActive {
		return helper.Fail(c, fiber.StatusForbidden, "akun dinonaktifkan")
	}

	pair, err := s.issueTokenPair(ctx, student)
	if err != nil {
		return helper.Fail(c, fiber.StatusInternalServerError, "gagal membuat token")
	}

	return helper.Ok(c, "login berhasil", pair)
}
func (s *AuthService) Refresh(c *fiber.Ctx) error {
	ctx, cancel := helper.ReqCtx(c)
	defer cancel()

	var req model.RefreshRequest
	if err := c.BodyParser(&req); err != nil {
		return helper.Fail(c, fiber.StatusBadRequest, "body harus berupa JSON yang valid")
	}
	if strings.TrimSpace(req.RefreshToken) == "" {
		return helper.Fail(c, fiber.StatusBadRequest, "refresh_token wajib diisi")
	}

	hash := helper.SHA256Hex(req.RefreshToken)

	stored, err := s.tokens.FindActive(ctx, hash)
	if err != nil {
		return helper.Fail(c, fiber.StatusUnauthorized,
			"refresh token tidak valid atau sudah kedaluwarsa")
	}

	student, err := s.student.FindByID(ctx, stored.StudentID)
	if err != nil || !student.IsActive {
		return helper.Fail(c, fiber.StatusUnauthorized, "akun tidak dapat dipakai")
	}

	// ROTASI: token lama langsung dicabut dan diganti yang baru.
	// Bila token lama sempat dicuri, ia hanya berguna sekali.
	if err := s.tokens.Revoke(ctx, hash); err != nil {
		return helper.Fail(c, fiber.StatusInternalServerError, "gagal memperbarui token")
	}

	pair, err := s.issueTokenPair(ctx, student)
	if err != nil {
		return helper.Fail(c, fiber.StatusInternalServerError, "gagal membuat token")
	}

	return helper.Ok(c, "token berhasil diperbarui", pair)
}

func (s *AuthService) Logout(c *fiber.Ctx) error {
	ctx, cancel := helper.ReqCtx(c)
	defer cancel()

	var req model.RefreshRequest
	if err := c.BodyParser(&req); err != nil {
		return helper.Fail(c, fiber.StatusBadRequest, "body harus berupa JSON yang valid")
	}

	if strings.TrimSpace(req.RefreshToken) != "" {
		// Kegagalan mencabut tidak dilaporkan sebagai error ke client:
		// dari sudut pandang pemakai, logout harus selalu berhasil.
		_ = s.tokens.Revoke(ctx, helper.SHA256Hex(req.RefreshToken))
	}

	return helper.Ok(c, "logout berhasil", nil)
}

func (s *AuthService) Me(c *fiber.Ctx) error {
	ctx, cancel := helper.ReqCtx(c)
	defer cancel()

	authUser, ok := helper.CurrentUser(c)
	if !ok {
		return helper.Fail(c, fiber.StatusUnauthorized, "belum terautentikasi")
	}

	student, err := s.student.FindByID(ctx, authUser.StudentID)
	if err != nil {
		return helper.Fail(c, fiber.StatusUnauthorized, "student tidak ditemukan")
	}

	// Daftar permission SELALU diambil dari PermissionSet yang sama
	// dengan yang dipakai middleware RequirePermission. Dengan begitu
	// apa yang klien lihat di /auth/me selalu konsisten dengan apa
	// yang akan diterimanya (atau tidak) saat memanggil endpoint lain.
	//
	// Sumbernya role pada AuthStudents (di JWT), bukan role pada
	// tabel students: bila admin mengganti role-nya sendiri tepat
	// sebelum request ini, server masih memakai role lama sampai
	// token baru diterbitkan. Itu disengaja.
	return helper.Ok(c, "profil berhasil diambil", model.ProfileResponse{
		Student:     student,
		Role:        authUser.Role,
		Permissions: s.perms.PermissionsOf(authUser.Role),
		IsActive:    student.IsActive,
		GeneratedAt: time.Now(),
	})
}

// issueTokenPair membuat access token dan refresh token sekaligus.
func (s *AuthService) issueTokenPair(
	ctx context.Context, user model.Student,
) (model.TokenPair, error) {
	accessToken, err := s.jwt.GenerateAccess(user)
	if err != nil {
		return model.TokenPair{}, err
	}

	refreshToken, err := helper.RandomToken(refreshTokenBytes)
	if err != nil {
		return model.TokenPair{}, err
	}

	// Yang disimpan hash-nya, bukan tokennya.
	err = s.tokens.Save(ctx, model.RefreshToken{
		StudentID: user.ID,
		TokenHash: helper.SHA256Hex(refreshToken),
		ExpiresAt: time.Now().Add(s.refreshTTL),
	})
	if err != nil {
		return model.TokenPair{}, err
	}

	return model.TokenPair{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		TokenType:    "Bearer",
		ExpiresIn:    int(s.jwt.AccessTTL().Seconds()),
	}, nil
}
