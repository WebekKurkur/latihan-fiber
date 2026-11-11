//go:build integration

package integration_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"latihan-fiber/app/model"
)

// swap mengirim POST /enrollments/swap atas nama student dan mengembalikan
// status code beserta field "code" pada response.
func (a api) swap(t *testing.T, student model.Student, fromCourseID, toCourseID int) (int, string) {
	t.Helper()

	token, err := a.jwt.GenerateAccess(student)
	if err != nil {
		t.Errorf("membuat token: %v", err)
		return 0, ""
	}

	payload, err := json.Marshal(model.SwapRequest{
		FromCourseID: fromCourseID,
		ToCourseID:   toCourseID,
	})
	if err != nil {
		t.Errorf("menyusun body swap: %v", err)
		return 0, ""
	}

	req := httptest.NewRequest(http.MethodPost, "/api/v1/enrollments/swap", bytes.NewReader(payload))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := a.app.Test(req, -1)
	if err != nil {
		t.Errorf("request swap: %v", err)
		return 0, ""
	}
	defer resp.Body.Close()

	var body struct {
		Code string `json:"code"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&body)
	return resp.StatusCode, body.Code
}

// drop mengirim DELETE /courses/:id/enroll atas nama student.
func (a api) drop(t *testing.T, student model.Student, courseID int) int {
	t.Helper()

	token, err := a.jwt.GenerateAccess(student)
	if err != nil {
		t.Errorf("membuat token: %v", err)
		return 0
	}

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/courses/"+strconv.Itoa(courseID)+"/enroll", nil)
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := a.app.Test(req, -1)
	if err != nil {
		t.Errorf("request drop: %v", err)
		return 0
	}
	defer resp.Body.Close()

	return resp.StatusCode
}

// courseEnrolledCount membaca jumlah peserta yang tercatat pada sebuah kelas.
func courseEnrolledCount(t *testing.T, courseID int) int {
	t.Helper()
	var n int
	if err := pool.QueryRow(context.Background(),
		`SELECT enrolled_count FROM courses WHERE id = $1`, courseID).Scan(&n); err != nil {
		t.Fatalf("membaca enrolled_count kelas %d: %v", courseID, err)
	}
	return n
}

// TestSwapEndpoint_TargetCourseFullKeepsStudentInSourceCourse menguji
// cases yang paling berbahaya dari endpoint pindah kelas: kelas tujuan penuh.
//
// Syarat yang harus dipenuhi SEMUA:
//  1. response 409 dengan code COURSE_FULL;
//  2. mahasiswa TETAP terdaftar di kelas asal (rollback transaction);
//  3. enrolled_count kedua kelas tidak berubah;
//  4. tidak ada event enrollment.swapped di outbox.
//
// Bila salah satu tidak terpenuhi, mahasiswa kehilangan kelas lamanya
// tanpa mendapat yang baru —ithmnamedua-duanya hilang.
func TestSwapEndpoint_TargetCourseFullKeepsStudentInSourceCourse(t *testing.T) {
	a := newAPI(t)
	student := seedStudent(t)

	fromCourse := seedCourse(t, 3, 10)
	toCourse := seedCourse(t, 3, 1)

	// Pengisi kelas tujuan: mahasiswa lain. Kuota 1 + 1 peserta = kelas penuh.
	other := seedStudent(t)
	if status, code := a.enroll(t, other, toCourse.ID); status != http.StatusCreated {
		t.Fatalf("seed peserta kelas tujuan: %d %s, seharusnya 201", status, code)
	}

	// Mahasiswa yang akan berpindah harus terdaftar di kelas asal.
	if status, code := a.enroll(t, student, fromCourse.ID); status != http.StatusCreated {
		t.Fatalf("pendaftaran kelas asal: %d %s, seharusnya 201", status, code)
	}

	fromBefore := courseEnrolledCount(t, fromCourse.ID)
	toBefore := courseEnrolledCount(t, toCourse.ID)

	status, code := a.swap(t, student, fromCourse.ID, toCourse.ID)
	if status != http.StatusConflict || code != "COURSE_FULL" {
		t.Fatalf("pindah ke kelas penuh: dapat %d %s, seharusnya 409 COURSE_FULL", status, code)
	}

	// (2) Mahasiswa tetap di kelas asal.
	if n := countRows(t,
		`SELECT COUNT(*) FROM enrollments WHERE course_id = $1 AND student_id = $2`,
		fromCourse.ID, student.ID); n != 1 {
		t.Errorf("pendaftaran di kelas asal = %d, seharusnya 1 (rollback harus mengembalikan kelas asal)", n)
	}

	// (3) enrolled_count kedua kelas tidak berubah.
	if got := courseEnrolledCount(t, fromCourse.ID); got != fromBefore {
		t.Errorf("enrolled_count kelas asal = %d, sebelumnya %d, seharusnya tetap", got, fromBefore)
	}
	if got := courseEnrolledCount(t, toCourse.ID); got != toBefore {
		t.Errorf("enrolled_count kelas tujuan = %d, sebelumnya %d, seharusnya tetap", got, fromBefore)
	}

	// (4) Tidak boleh ada jejak event di outbox.
	if n := countRows(t,
		`SELECT COUNT(*) FROM outbox WHERE (payload->>'student_id')::INT = $1 AND event_type = $2`,
		student.ID, model.EventEnrollmentSwapped); n != 0 {
		t.Errorf("event enrollment.swapped = %d, seharusnya 0 (perpindahan yang gagal tidak boleh meninggalkan jejak)", n)
	}
}

// TestDropEndpoint_RemovesEnrollmentAndWritesExactlyOneDroppedEvent
// menguji endpoint keluar dari kelas.
//
// Syarat:
//  1. response 204;
//  2. baris enrollment hilang;
//  3. enrolled_count berkurang satu;
//  4. outbox berisi TEPAT SATU event enrollment.dropped.
func TestDropEndpoint_RemovesEnrollmentAndWritesExactlyOneDroppedEvent(t *testing.T) {
	a := newAPI(t)
	student := seedStudent(t)
	course := seedCourse(t, 3, 10)

	if status, code := a.enroll(t, student, course.ID); status != http.StatusCreated {
		t.Fatalf("pendaftaran awal: %d %s, seharusnya 201", status, code)
	}
	before := courseEnrolledCount(t, course.ID)

	if status := a.drop(t, student, course.ID); status != http.StatusNoContent {
		t.Fatalf("keluar dari kelas: status = %d, seharusnya 204", status)
	}

	if n := countRows(t,
		`SELECT COUNT(*) FROM enrollments WHERE course_id = $1 AND student_id = $2`,
		course.ID, student.ID); n != 0 {
		t.Errorf("baris enrollments = %d, seharusnya 0", n)
	}

	if got := courseEnrolledCount(t, course.ID); got != before-1 {
		t.Errorf("enrolled_count = %d, sebelumnya %d, seharusnya berkurang satu", got, before)
	}

	dropped := countRows(t,
		`SELECT COUNT(*) FROM outbox WHERE (payload->>'student_id')::INT = $1 AND event_type = $2`,
		student.ID, model.EventEnrollmentDropped)
	if dropped != 1 {
		t.Errorf("event enrollment.dropped = %d, seharusnya tepat 1", dropped)
	}
}
