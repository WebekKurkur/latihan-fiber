package service

import (
	"errors"
	"fmt"

	"latihan-fiber/app/model"
)

// errUnknownEvent menandai event yang jenisnya tidak dikenali worker.
// Event seperti ini tidak akan pernah berhasil dikirim, sehingga worker
// harus berhenti mencobanya (SkipRetry), bukan mengulanginya.
var errUnknownEvent = errors.New("jenis event tidak dikenal")

// BuildMessage menyusun pesan dari sebuah event. Fungsi murni: tidak
// menyentuh database, Redis, maupun email, sehingga mudah diuji.
//
// Nama dan kode kelas diambil dari EVENT, bukan dari tabel kelas saat ini.
// Notifikasi menggambarkan keadaan pada saat event terjadi.
func BuildMessage(
	eventType string,
	e model.EnrollmentEvent,
	student model.Student,
) (model.Message, error) {
	course := e.CourseCode + " " + e.CourseName

	var subject, body string
	switch eventType {
	case model.EventEnrollmentCreated:
		subject = "KRS: berhasil terdaftar di " + course
		body = fmt.Sprintf("Halo %s, Anda tercatat sebagai peserta kelas %s.",
			student.Name, course)
	case model.EventEnrollmentDropped:
		subject = "KRS: keluar dari " + course
		body = fmt.Sprintf("Halo %s, Anda sudah tidak terdaftar di kelas %s.",
			student.Name, course)
	case model.EventEnrollmentSwapped:
		from := e.FromCourseCode + " " + e.FromCourseName
		subject = "KRS: pindah dari " + from + " ke " + course
		body = fmt.Sprintf("Halo %s, Anda telah pindah dari kelas %s ke kelas %s.",
			student.Name, from, course)
	case model.EventQuotaChanged:
		// Event ini menyangkut seluruh peserta kelas. Pesannya menyebut
		// nilai kuota BARU karena itulah informasi yang dicari mahasiswa.
		subject = fmt.Sprintf("KRS: kuota %s berubah menjadi %d", course, e.NewQuota)
		body = fmt.Sprintf(
			"Halo %s, kuota kelas %s berubah dari %d menjadi %d.",
			student.Name, course, e.OldQuota, e.NewQuota,
		)
	default:
		return model.Message{}, fmt.Errorf("%w: %q", errUnknownEvent, eventType)
	}

	return model.Message{To: student.Email, Subject: subject, Body: body}, nil
}
