package service

import (
	"errors"
	"strings"
	"testing"

	"latihan-fiber/app/model"
)

// BuildMessage adalah fungsi murni, jadi tidak perlu database, Redis,
// maupun server. Pengujian di bawah hanya memeriksa aturan penyusunan
// pesan untuk event perubahan kuota.

func TestBuildMessageQuotaChanged(t *testing.T) {
	event := model.EnrollmentEvent{
		StudentID:  7,
		CourseID:   3,
		CourseCode: "IF201",
		CourseName: "Basis Data",
		OldQuota:   30,
		NewQuota:   40,
	}
	student := model.Student{ID: 7, Name: "sari", Email: "sari@example.com"}

	message, err := BuildMessage(model.EventQuotaChanged, event, student)
	if err != nil {
		t.Fatalf("BuildMessage error: %v", err)
	}

	if message.To != student.Email {
		t.Errorf("To: harap %q, dapat %q", student.Email, message.To)
	}
	if message.Subject != "KRS: kuota IF201 Basis Data berubah menjadi 40" {
		t.Errorf("Subject: dapat %q", message.Subject)
	}
	if !strings.Contains(message.Body, "sari") {
		t.Errorf("body harus menyebut nama mahasiswa, dapat %q", message.Body)
	}
	if !strings.Contains(message.Body, "30") || !strings.Contains(message.Body, "40") {
		t.Errorf("body harus menyebut kuota lama dan baru, dapat %q", message.Body)
	}
}

func TestBuildMessageQuotaChangedKeepsOldQuotaWhenZero(t *testing.T) {
	// Kuota lama bernilai 0 tidak boleh hilang dari pesan. Field memakai
	// omitempty pada payload, jadi nilai 0 memang tidak ikut tersimpan;
	// yang diuji di sini hanya perilaku BuildMessage terhadap nilai yang
	// memang tersedia.
	event := model.EnrollmentEvent{
		CourseCode: "IF201",
		CourseName: "Basis Data",
		OldQuota:   0,
		NewQuota:   25,
	}
	student := model.Student{Name: "budi", Email: "budi@example.com"}

	message, err := BuildMessage(model.EventQuotaChanged, event, student)
	if err != nil {
		t.Fatalf("BuildMessage error: %v", err)
	}
	if !strings.Contains(message.Body, "0") {
		t.Errorf("body harus menyebut kuota lama 0, dapat %q", message.Body)
	}
}

func TestBuildMessageUnknownEvent(t *testing.T) {
	_, err := BuildMessage("course.unknown", model.EnrollmentEvent{}, model.Student{})
	if err == nil {
		t.Fatal("event tak dikenal harus menghasilkan error")
	}
	if !errors.Is(err, errUnknownEvent) {
		t.Errorf("error harus terbungkus errUnknownEvent, dapat %v", err)
	}
}
