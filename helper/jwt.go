package helper

import (
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"latihan-fiber/app/model"
)

var (
	ErrInvalidToken = errors.New("token tidak valid")
	ErrExpiredToken = errors.New("token sudah kedaluwarsa")
)

// accessClaims adalah isi access token. Selain field bawaan JWT,
// ditambahkan username dan role agar middleware tidak perlu
// menanyakannya ke database pada setiap request.
type accessClaims struct {
	Name string `json:"name"`
	Role string `json:"role"`
	jwt.RegisteredClaims
}

type JWTManager struct {
	secret    []byte
	issuer    string
	accessTTL time.Duration
}

func NewJWTManager(secret, issuer string, accessTTL time.Duration) *JWTManager {
	return &JWTManager{secret: []byte(secret), issuer: issuer, accessTTL: accessTTL}
}

func (m *JWTManager) AccessTTL() time.Duration { return m.accessTTL }

// GenerateAccess membuat access token berumur pendek.
func (m *JWTManager) GenerateAccess(s model.Student) (string, error) {
	now := time.Now()

	claims := accessClaims{
		Name: s.Name,
		Role: s.Role,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   strconv.Itoa(s.ID),
			Issuer:    m.issuer,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(m.accessTTL)),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(m.secret)
}

// Parse memeriksa tanda tangan dan masa berlaku token,
// lalu mengembalikan identitas yang dibawanya.
func (m *JWTManager) Parse(tokenString string) (model.AuthStudents, error) {
	claims := &accessClaims{}

	token, err := jwt.ParseWithClaims(tokenString, claims,
		func(t *jwt.Token) (any, error) {
			// PEMERIKSAAN WAJIB: pastikan algoritmanya memang yang kita pilih.
			// Tanpa baris ini, penyerang dapat mengirim token ber-alg "none"
			// atau menukar algoritma, dan tanda tangannya akan lolos.
			if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, fmt.Errorf("algoritma tidak diharapkan: %v", t.Header["alg"])
			}
			return m.secret, nil
		},
		jwt.WithIssuer(m.issuer),
		jwt.WithExpirationRequired(),
	)

	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) {
			return model.AuthStudents{}, ErrExpiredToken
		}
		return model.AuthStudents{}, ErrInvalidToken
	}
	if !token.Valid {
		return model.AuthStudents{}, ErrInvalidToken
	}

	StudentID, err := strconv.Atoi(claims.Subject)
	if err != nil {
		return model.AuthStudents{}, ErrInvalidToken
	}

	return model.AuthStudents{
		StudentID: StudentID,
		Name:      claims.Name,
		Role:      claims.Role,
	}, nil
}
