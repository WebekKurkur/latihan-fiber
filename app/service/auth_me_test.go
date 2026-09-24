package service

import (
	"context"
	"encoding/json"
	"io"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"

	"latihan-fiber/app/model"
	"latihan-fiber/helper"
)

// studentRepoStub menjawab FindByID dengan student statis dan menolak
// semua panggilan lain (panic). Test ini tidak akan pernah menyentuh
// metode lain karena handler Me hanya memanggil FindByID.
type studentRepoStub struct {
	student model.Student
}

func (s studentRepoStub) FindByID(_ context.Context, id int) (model.Student, error) {
	s.student.ID = id
	return s.student, nil
}

func (studentRepoStub) FindAll(context.Context, model.ListQuery) ([]model.Student, int, error) {
	panic("tidak dipakai di test ini")
}
func (studentRepoStub) FindOwnerID(context.Context, int) (*int, error) {
	panic("tidak dipakai di test ini")
}
func (studentRepoStub) FindByUsername(context.Context, string) (model.Student, error) {
	panic("tidak dipakai di test ini")
}
func (studentRepoStub) Create(context.Context, model.Student) (model.Student, error) {
	panic("tidak dipakai di test ini")
}
func (studentRepoStub) Update(context.Context, model.Student) (model.Student, error) {
	panic("tidak dipakai di test ini")
}
func (studentRepoStub) UpdateRole(context.Context, int, string) (model.Student, error) {
	panic("tidak dipakai di test ini")
}
func (studentRepoStub) Delete(context.Context, int) error {
	panic("tidak dipakai di test ini")
}

// tokenRepoStub mengimplementasi repository.TokenRepository dengan
// metode yang semuanya panic. Handler Me tidak pernah memanggil
// satupun metode ini; stub ini ada hanya supaya kompiler menerima
// struct sebagai TokenRepository dan test kompilasi bersih.
type tokenRepoStub struct{}

func (tokenRepoStub) Save(context.Context, model.RefreshToken) error {
	panic("tidak dipakai di test ini")
}
func (tokenRepoStub) FindActive(context.Context, string) (model.RefreshToken, error) {
	panic("tidak dipakai di test ini")
}
func (tokenRepoStub) Revoke(context.Context, string) error {
	panic("tidak dipakai di test ini")
}
func (tokenRepoStub) RevokeAllForStudents(context.Context, int) error {
	panic("tidak dipakai di test ini")
}

func TestPermissionsOfReturnsSortedList(t *testing.T) {
	perms := helper.NewPermissionSet(map[string][]string{
		"admin": {"student:delete", "student:create", "student:list"},
		"user":  {"student:list"},
	})

	got := perms.PermissionsOf("admin")

	want := []string{"student:create", "student:delete", "student:list"}
	if len(got) != len(want) {
		t.Fatalf("admin: harap %d permission, dapat %d (%v)",
			len(want), len(got), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("admin[%d]: harap %q, dapat %q", i, want[i], got[i])
		}
	}

	// Role yang tidak dikenal → kembalikan slice kosong, bukan nil error.
	if got := perms.PermissionsOf("ghost"); len(got) != 0 {
		t.Errorf("role ghost harusnya kosong, dapat %v", got)
	}
}

func TestMeReturnsProfileWithPermissions(t *testing.T) {
	perms := helper.NewPermissionSet(map[string][]string{
		"admin": {
			"student:list", "student:read:any",
			"student:create", "student:update:any",
			"student:delete", "student:role:assign",
		},
	})

	svc := &AuthService{
		student: studentRepoStub{student: model.Student{
			Name: "sari", NIM: "250501001",
			IsActive: true, Role: "admin",
		}},
		tokens: tokenRepoStub{},
		perms:  perms,
	}

	app := fiber.New()
	app.Get("/auth/me",
		func(c *fiber.Ctx) error {
			c.Locals(helper.LocalsAuthStudents, model.AuthStudents{
				StudentID: 7, Name: "sari", Role: "admin",
			})
			return svc.Me(c)
		},
	)

	req := httptest.NewRequest("GET", "/auth/me", nil)
	resp, err := app.Test(req, 1500)
	if err != nil {
		t.Fatalf("app.Test gagal: %v", err)
	}
	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("status: harap 200, dapat %d", resp.StatusCode)
	}

	body, _ := io.ReadAll(resp.Body)
	var env struct {
		Success bool                  `json:"success"`
		Data    model.ProfileResponse `json:"data"`
	}
	if err := json.Unmarshal(body, &env); err != nil {
		t.Fatalf("respons bukan JSON: %v\n%s", err, string(body))
	}
	if !env.Success {
		t.Fatalf("success=false; body=%s", string(body))
	}

	got := env.Data
	if got.Role != "admin" {
		t.Errorf("role: harap admin, dapat %q", got.Role)
	}
	if !got.IsActive {
		t.Error("is_active seharusnya true")
	}

	wantPerms := []string{
		"student:create", "student:delete",
		"student:list", "student:read:any",
		"student:role:assign", "student:update:any",
	}
	if len(got.Permissions) != len(wantPerms) {
		t.Fatalf("permission: harap %d, dapat %d (%v)",
			len(wantPerms), len(got.Permissions), got.Permissions)
	}
	for i := range wantPerms {
		if got.Permissions[i] != wantPerms[i] {
			t.Errorf("permission[%d]: harap %q, dapat %q",
				i, wantPerms[i], got.Permissions[i])
		}
	}

	// GeneratedAt diisi saat respons dibuat — bukan zero value.
	if got.GeneratedAt.IsZero() {
		t.Error("generated_at seharusnya diisi waktu respons dibuat")
	}
}

func TestMeReflectsAuthStudentsRoleNotStudentTableRole(t *testing.T) {
	// Skenario: admin baru saja menurunkan rolenya sendiri di tabel
	// (melalui PATCH /:id/role) TAPI token lama masih memuat role=admin.
	// Handler Me HARUS memakai role dari token, bukan dari tabel.
	// Konsisten dengan komentar di handler Me.
	perms := helper.NewPermissionSet(map[string][]string{
		"admin": {"student:delete"},
		"user":  {},
	})

	svc := &AuthService{
		student: studentRepoStub{student: model.Student{
			Name: "sari", NIM: "x", IsActive: true, Role: "user",
		}},
		tokens: tokenRepoStub{},
		perms:  perms,
	}

	app := fiber.New()
	app.Get("/auth/me",
		func(c *fiber.Ctx) error {
			c.Locals(helper.LocalsAuthStudents, model.AuthStudents{
				StudentID: 9, Name: "sari", Role: "admin",
			})
			return svc.Me(c)
		},
	)

	resp, _ := app.Test(httptest.NewRequest("GET", "/auth/me", nil), 1500)
	body, _ := io.ReadAll(resp.Body)
	var env struct {
		Data model.ProfileResponse `json:"data"`
	}
	_ = json.Unmarshal(body, &env)

	if env.Data.Role != "admin" {
		t.Errorf("Me harus pakai role dari token (admin), dapat %q",
			env.Data.Role)
	}
	if len(env.Data.Permissions) != 1 ||
		env.Data.Permissions[0] != "student:delete" {
		t.Errorf("permission harusnya berisi student:delete, dapat %v",
			env.Data.Permissions)
	}
}
