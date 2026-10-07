package helper

import (
	"testing"

	"siakad-uts/app/model"
)

func TestValidateStruct(t *testing.T) {
	if ValidateStruct(model.LoginRequest{Email: "not-email", Password: "short"}) == nil {
		t.Fatal("login invalid accepted")
	}
	if ValidateStruct(model.CreateStudentRequest{NIM: "123", Nama: "", Email: "x", Prodi: "", Angkatan: 3000}) == nil {
		t.Fatal("student invalid accepted")
	}
	if ValidateStruct(model.CreateEnrollmentRequest{CourseID: 1, TahunAkademik: "2026/2028-Ganjil"}) == nil {
		t.Fatal("year gap accepted")
	}
	if ValidateStruct(model.CreateEnrollmentRequest{CourseID: 1, TahunAkademik: "2026/2027-Ganjil"}) != nil {
		t.Fatal("valid enrollment rejected")
	}
}
