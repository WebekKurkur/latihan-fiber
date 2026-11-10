package service

import (
	"errors"
	"fmt"
	"testing"

	"github.com/gofiber/fiber/v2"

	"latihan-fiber/app/repository"
	"latihan-fiber/helper"
)

// TestTranslatePortfolioError memverifikasi pemetaan error repository menjadi
// AppError (status HTTP + kode) secara white-box. Fungsi translatePortfolioError
// tidak diekspor, jadi test harus berada pada package service.
func TestTranslatePortfolioError(t *testing.T) {
	tests := []struct {
		name       string
		err        error
		wantStatus int
		wantCode   string
	}{
		{
			name:       "id proyek tidak valid menjadi 400",
			err:        repository.ErrPortfolioInvalidProjectID,
			wantStatus: fiber.StatusBadRequest,
			wantCode:   helper.CodeBadRequest,
		},
		{
			name:       "portofolio tidak ditemukan menjadi 404",
			err:        repository.ErrPortfolioNotFound,
			wantStatus: fiber.StatusNotFound,
			wantCode:   helper.CodeNotFound,
		},
		{
			name:       "batas proyek menjadi 409",
			err:        repository.ErrPortfolioProjectLimit,
			wantStatus: fiber.StatusConflict,
			wantCode:   helper.CodeConflict,
		},
		{
			name:       "student tidak ditemukan menjadi 404",
			err:        repository.ErrNotFound,
			wantStatus: fiber.StatusNotFound,
			wantCode:   helper.CodeNotFound,
		},
		{
			name:       "nim duplikat menjadi 409",
			err:        repository.ErrDuplicate,
			wantStatus: fiber.StatusConflict,
			wantCode:   helper.CodeConflict,
		},
		{
			name:       "email duplikat menjadi 409",
			err:        repository.ErrEmailDuplicate,
			wantStatus: fiber.StatusConflict,
			wantCode:   helper.CodeConflict,
		},
		{
			name:       "error terbungkus tetap dikenali",
			err:        fmt.Errorf("lapisan luar: %w", repository.ErrPortfolioProjectLimit),
			wantStatus: fiber.StatusConflict,
			wantCode:   helper.CodeConflict,
		},
		{
			name:       "error tak dikenal menjadi 500",
			err:        errors.New("koneksi MongoDB putus"),
			wantStatus: fiber.StatusInternalServerError,
			wantCode:   helper.CodeInternal,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := translatePortfolioError(tt.err)

			var appErr *helper.AppError
			if !errors.As(got, &appErr) {
				t.Fatalf("translatePortfolioError(%v) menghasilkan %T, ingin *helper.AppError", tt.err, got)
			}
			if appErr.Status != tt.wantStatus {
				t.Errorf("status = %d, ingin %d", appErr.Status, tt.wantStatus)
			}
			if appErr.Code != tt.wantCode {
				t.Errorf("code = %q, ingin %q", appErr.Code, tt.wantCode)
			}
		})
	}
}
