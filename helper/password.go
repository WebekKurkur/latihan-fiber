package helper

import "strings"

// Fungsi passwordStrength dipindahkan apa adanya dari auth_rules.go milik
// Modul 5. Ia tetap berupa fungsi biasa, bukan sepenuhnya tag, karena
// sebuah tag hanya mampu menjawab benar atau salah — padahal
// "minimal 8 karakter" dan "password terlalu umum" menuntut tindakan
// yang berbeda dari pemakainya.
//
// Mengembalikan string kosong berarti password lolos semua aturan.
// Mengembalikan string bukan kosong berarti aturan itu yang menolak,
// dan string itulah yang akan ditampilkan ke pemakai sebagai pesan.
//
// Daftar "weak" sengaja sangat pendek. Sistem sungguhan memakai daftar
// berisi jutaan password yang pernah bocor.
func passwordStrength(password string) string {
	if len(password) < 8 {
		return "minimal 8 karakter"
	}

	var hasLetter, hasDigit bool
	for _, r := range password {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z':
			hasLetter = true
		case r >= '0' && r <= '9':
			hasDigit = true
		}
	}

	if !hasLetter || !hasDigit {
		return "harus memuat huruf dan angka"
	}

	weak := map[string]bool{
		"password1": true, "12345678": true, "qwerty123": true,
		"admin123": true, "password123": true,
	}
	if weak[strings.ToLower(password)] {
		return "password terlalu umum"
	}
	return ""
}
