package service

import (
	"context"
	"errors"
	"siakad-uts/app/model"
	"siakad-uts/app/repository"
	"testing"
)

func TestSKSLimit(t *testing.T) {
	for _, tc := range []struct {
		ipk  *float64
		want int
	}{{nil, 18}, {ptr(2.49), 18}, {ptr(2.50), 21}, {ptr(2.99), 21}, {ptr(3.0), 24}} {
		if got := SKSLimit(tc.ipk); got != tc.want {
			t.Errorf("IPK %v: got %d want %d", tc.ipk, got, tc.want)
		}
	}
}
func ptr(v float64) *float64 { return &v }

type fakeStudents struct{}

func (fakeStudents) ListStudents(context.Context, model.PageQuery) ([]model.Student, int, error) {
	return nil, 0, nil
}
func (fakeStudents) StudentByID(context.Context, int) (model.Student, error) {
	return model.Student{}, repository.ErrNotFound
}
func (fakeStudents) CreateStudent(context.Context, model.CreateStudentRequest, string) (model.Student, error) {
	return model.Student{}, nil
}
func (fakeStudents) UpdateStudent(context.Context, int, model.UpdateStudentRequest) (model.Student, error) {
	return model.Student{}, nil
}
func (fakeStudents) DeleteStudent(context.Context, int) error { return nil }

type fakeEnrollments struct{}

func (fakeEnrollments) StudentEnrollments(context.Context, int) ([]model.Enrollment, int, error) {
	return nil, 0, nil
}
func (fakeEnrollments) Enroll(context.Context, int, model.CreateEnrollmentRequest, int) (model.Enrollment, error) {
	return model.Enrollment{}, nil
}
func (fakeEnrollments) Cancel(context.Context, int, int) error { return nil }
func TestOwnershipCheckedBeforeLookup(t *testing.T) {
	own := 1
	s := StudentService{Students: fakeStudents{}, Enrollments: fakeEnrollments{}}
	_, err := s.Detail(context.Background(), 2, model.Identity{Role: "mahasiswa", StudentID: &own})
	if err == nil || err.Error() != "Tidak berhak mengakses data mahasiswa lain" {
		t.Fatalf("expected 403, got %v", err)
	}
	_, err = s.Detail(context.Background(), 1, model.Identity{Role: "mahasiswa", StudentID: &own})
	if err == nil || errors.Is(err, repository.ErrNotFound) {
		t.Fatalf("expected mapped 404, got %v", err)
	}
}
