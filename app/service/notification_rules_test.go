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

// TestBuildMessageKRS memakai table-driven test untuk tiga event KRS.
// Nama event dipakai sebagai nama subtest, sehingga bila satu event
// rusak kita langsung tahu event yang mana.
func TestBuildMessageKRS(t *testing.T) {
	student := model.Student{ID: 7, Name: "sari", NIM: "250501001", Email: "sari@student.unair.ac.id"}
	event := model.EnrollmentEvent{
		StudentID: 7, CourseID: 4,
		CourseCode: "IF302", CourseName: "Keamanan Informasi",
		FromCourseCode: "IF102", FromCourseName: "Basis Data Lanjut",
	}

	tests := []struct {
		eventType   string
		wantSubject string
	}{
		{model.EventEnrollmentCreated, "KRS: berhasil terdaftar di IF302 Keamanan Informasi"},
		{model.EventEnrollmentDropped, "KRS: keluar dari IF302 Keamanan Informasi"},
		{model.EventEnrollmentSwapped, "KRS: pindah dari IF102 Basis Data Lanjut ke IF302 Keamanan Informasi"},
	}

	for _, tt := range tests {
		t.Run(tt.eventType, func(t *testing.T) {
			msg, err := BuildMessage(tt.eventType, event, student)
			if err != nil {
				t.Fatalf("error tidak diharapkan: %v", err)
			}
			if msg.Subject != tt.wantSubject {
				t.Errorf("subject = %q, seharusnya %q", msg.Subject, tt.wantSubject)
			}
			if msg.To != student.Email {
				t.Errorf("to = %q, seharusnya %q", msg.To, student.Email)
			}
			if !strings.Contains(msg.Body, student.Name) {
				t.Errorf("body tidak menyebut nama: %q", msg.Body)
			}
		})
	}
}

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
