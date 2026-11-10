package service_test

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"

	"latihan-fiber/app/model"
	"latihan-fiber/app/service"
	"latihan-fiber/config"
	"latihan-fiber/helper"
	"latihan-fiber/route"
)

const (
	portfolioStudent = 7
	otherStudent     = 8
)

type portfolioTestEnv struct {
	app *fiber.App
	jwt *helper.JWTManager
}

func newPortfolioTestEnv(t *testing.T) portfolioTestEnv {
	t.Helper()

	students := newFakeStudentRepository(
		model.Student{ID: portfolioStudent, Name: "Mahasiswa Tujuh", NIM: "721", IsActive: true},
		model.Student{ID: otherStudent, Name: "Mahasiswa Delapan", NIM: "722", IsActive: true},
	)
	portfolios := newFakePortfolioRepository()
	perms := helper.NewPermissionSet(map[string][]string{"admin": {"student:read:any"}})
	jwtManager := helper.NewJWTManager("secret-khusus-test-minimal-32-karakter", "test", time.Minute)

	app := config.NewApp(slog.New(slog.NewTextHandler(io.Discard, nil)), route.Dependencies{
		JWT:              jwtManager,
		Permissions:      perms,
		PortfolioService: service.NewPortfolioService(students, portfolios, perms),
	})
	return portfolioTestEnv{app: app, jwt: jwtManager}
}

func (e portfolioTestEnv) do(t *testing.T, userID int, method, path, body string) (int, map[string]any) {
	t.Helper()

	token, err := e.jwt.GenerateAccess(model.Student{
		ID: userID, Name: "mhs" + strconv.Itoa(userID), Role: "user",
	})
	if err != nil {
		t.Fatalf("membuat token: %v", err)
	}

	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+token)
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := e.app.Test(req, -1)
	if err != nil {
		t.Fatalf("%s %s: %v", method, path, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNoContent {
		return resp.StatusCode, nil
	}
	var parsed map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		t.Fatalf("%s %s: response bukan JSON: %v", method, path, err)
	}
	return resp.StatusCode, parsed
}

const validPortfolio = `{
	"headline": "Backend Developer",
	"bio": "Mahasiswa yang fokus membangun layanan backend.",
	"projects": [{
		"title": "Student API",
		"technologies": ["Go", "go"],
		"link": "https://example.com/student-api",
		"year": 2026
	}],
	"certificates": []
}`

const emptyPortfolio = `{
	"headline": "Backend Developer",
	"bio": "Mahasiswa yang fokus membangun layanan backend.",
	"projects": [],
	"certificates": []
}`

func TestGetPortfolio_NotFound(t *testing.T) {
	env := newPortfolioTestEnv(t)

	status, body := env.do(t, portfolioStudent, http.MethodGet,
		"/api/v1/students/7/portfolio", "")
	if status != http.StatusNotFound || body["code"] != "NOT_FOUND" {
		t.Errorf("dapat %d %v, seharusnya 404 NOT_FOUND", status, body["code"])
	}
}

func TestReplacePortfolio_Success(t *testing.T) {
	env := newPortfolioTestEnv(t)

	status, body := env.do(t, portfolioStudent, http.MethodPut,
		"/api/v1/students/7/portfolio", validPortfolio)
	if status != http.StatusOK {
		t.Fatalf("status = %d, seharusnya 200 (body: %v)", status, body)
	}
	data := body["data"].(map[string]any)
	if data["student_id"] != float64(portfolioStudent) {
		t.Errorf("student_id = %v, seharusnya %d", data["student_id"], portfolioStudent)
	}
	projects := data["projects"].([]any)
	technologies := projects[0].(map[string]any)["technologies"].([]any)
	if len(technologies) != 1 || technologies[0] != "Go" {
		t.Errorf("technologies = %v, seharusnya [Go] setelah deduplikasi", technologies)
	}

	status, body = env.do(t, portfolioStudent, http.MethodGet,
		"/api/v1/students/7/portfolio", "")
	if status != http.StatusOK || len(body["data"].(map[string]any)["projects"].([]any)) != 1 {
		t.Errorf("portofolio tersimpan tidak dapat dibaca kembali: %d %v", status, body)
	}
}

func TestPortfolioProjectLifecycle(t *testing.T) {
	env := newPortfolioTestEnv(t)

	status, _ := env.do(t, portfolioStudent, http.MethodPut,
		"/api/v1/students/7/portfolio", emptyPortfolio)
	if status != http.StatusOK {
		t.Fatalf("menyiapkan portofolio: status = %d, seharusnya 200", status)
	}

	addBody := `{"title":"Queue Worker","technologies":["Go","Redis"],"year":2026}`
	status, body := env.do(t, portfolioStudent, http.MethodPost,
		"/api/v1/students/7/portfolio/projects", addBody)
	if status != http.StatusOK {
		t.Fatalf("menambah proyek: status = %d, seharusnya 200 (body: %v)", status, body)
	}
	projectID := body["data"].(map[string]any)["projects"].([]any)[0].(map[string]any)["id"].(string)

	status, _ = env.do(t, portfolioStudent, http.MethodDelete,
		"/api/v1/students/7/portfolio/projects/"+projectID, "")
	if status != http.StatusNoContent {
		t.Fatalf("menghapus proyek: status = %d, seharusnya 204", status)
	}
}

func TestPortfolioAccessAndValidation(t *testing.T) {
	tests := []struct {
		name       string
		userID     int
		path       string
		method     string
		body       string
		wantStatus int
		wantCode   string
	}{
		{"student lain", portfolioStudent, "/api/v1/students/8/portfolio", http.MethodGet, "", http.StatusForbidden, "FORBIDDEN"},
		{"student tidak ada", 99, "/api/v1/students/99/portfolio", http.MethodGet, "", http.StatusNotFound, "NOT_FOUND"},
		{"body bukan JSON", portfolioStudent, "/api/v1/students/7/portfolio", http.MethodPut, `{headline:`, http.StatusBadRequest, "BAD_REQUEST"},
		{"headline terlalu pendek", portfolioStudent, "/api/v1/students/7/portfolio", http.MethodPut, `{"headline":"x","bio":"Bio mahasiswa yang cukup panjang"}`, http.StatusUnprocessableEntity, "VALIDATION_ERROR"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			env := newPortfolioTestEnv(t)
			status, body := env.do(t, tt.userID, tt.method, tt.path, tt.body)
			if status != tt.wantStatus || body["code"] != tt.wantCode {
				t.Errorf("dapat %d %v, seharusnya %d %s", status, body["code"], tt.wantStatus, tt.wantCode)
			}
		})
	}
}
