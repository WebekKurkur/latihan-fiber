package service

import (
	"strings"
	"unicode"

	"latihan-fiber/app/model"
)

const minPasswordLength = 8

func ValidateRegister(req model.RegisterRequest) map[string]string {
	errs := map[string]string{}

	Name := strings.TrimSpace(req.Name)
	switch {
	case Name == "":
		errs["Name"] = "wajib diisi"
	case len(Name) < 3:
		errs["Name"] = "minimal 3 karakter"
	case !isValidName(Name):
		errs["Name"] = "hanya boleh huruf, angka, titik, dan garis bawah"
	}

	if !isValidNIM(req.NIM) {
		errs["NIM"] = "format NIM tidak valid"
	}

	if msg := checkPasswordStrength(req.Password); msg != "" {
		errs["password"] = msg
	}

	return errs
}

// ValidateLogin hanya memeriksa kelengkapan, BUKAN kekuatan password.
// Aturan kekuatan tidak diberlakukan di sini karena password lama
// mungkin dibuat sebelum aturannya berubah.
func ValidateLogin(req model.LoginRequest) map[string]string {
	errs := map[string]string{}

	if strings.TrimSpace(req.Name) == "" {
		errs["Name"] = "wajib diisi"
	}
	if req.Password == "" {
		errs["password"] = "wajib diisi"
	}

	return errs
}

func checkPasswordStrength(password string) string {
	if len(password) < minPasswordLength {
		return "minimal 8 karakter"
	}

	var hasLetter, hasDigit bool
	for _, r := range password {
		switch {
		case unicode.IsLetter(r):
			hasLetter = true
		case unicode.IsDigit(r):
			hasDigit = true
		}
	}

	if !hasLetter || !hasDigit {
		return "harus memuat huruf dan angka"
	}

	// Daftar ini sengaja sangat pendek. Sistem sungguhan memakai daftar
	// berisi jutaan password yang pernah bocor.
	weak := map[string]bool{
		"password1": true, "12345678": true, "qwerty123": true,
		"admin123": true, "password123": true,
	}
	if weak[strings.ToLower(password)] {
		return "password terlalu umum"
	}
	return ""
}

func isValidName(Name string) bool {
	for _, r := range Name {
		if !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '.' && r != '_' {
			return false
		}
	}
	return true
}

func isValidNIM(nim string) bool {
	nim = strings.TrimSpace(nim)
	if nim == "" {
		return false
	}

	for _, r := range nim {
		if !unicode.IsDigit(r) {
			return false
		}
	}
	return true
}
