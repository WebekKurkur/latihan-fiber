package service

import (
	"testing"

	"latihan-fiber/app/model"
)

func TestValidateRegisterAcceptsValidStudent(t *testing.T) {
	req := model.RegisterRequest{
		Name:     "sari_modul5",
		NIM:      "250501001",
		Password: "rahasia123",
	}

	if errs := ValidateRegister(req); len(errs) != 0 {
		t.Fatalf("data valid seharusnya diterima, tetapi mendapat error: %v", errs)
	}
}

func TestValidateRegisterRejectsWeakPassword(t *testing.T) {
	req := model.RegisterRequest{
		Name:     "sari_modul5",
		NIM:      "250501001",
		Password: "password1",
	}

	errs := ValidateRegister(req)
	if got := errs["password"]; got != "password terlalu umum" {
		t.Fatalf("error password = %q, ingin %q", got, "password terlalu umum")
	}
}

func TestValidateRegisterRejectsInvalidNIM(t *testing.T) {
	req := model.RegisterRequest{
		Name:     "sari_modul5",
		NIM:      "nim-tidak-valid",
		Password: "rahasia123",
	}

	errs := ValidateRegister(req)
	if got := errs["NIM"]; got == "" {
		t.Fatal("NIM tidak valid seharusnya menghasilkan error")
	}
}

func TestValidateLoginRequiresNameAndPassword(t *testing.T) {
	errs := ValidateLogin(model.LoginRequest{})

	if _, ok := errs["Name"]; !ok {
		t.Fatal("name kosong seharusnya menghasilkan error")
	}
	if _, ok := errs["password"]; !ok {
		t.Fatal("password kosong seharusnya menghasilkan error")
	}
}
