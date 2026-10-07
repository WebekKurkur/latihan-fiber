package service

import (
	"context"
	"errors"
	"fmt"
	"siakad-uts/app/model"
	"siakad-uts/app/repository"
	"siakad-uts/helper"
)

type EnrollmentService struct {
	Enrollments repository.EnrollmentRepository
	Students    repository.StudentRepository
}

func (s EnrollmentService) Enroll(ctx context.Context, studentID int, r model.CreateEnrollmentRequest) (model.Enrollment, error) {
	if e := helper.ValidateStruct(r); e != nil {
		return model.Enrollment{}, e
	}
	student, e := s.Students.StudentByID(ctx, studentID)
	if e != nil {
		return model.Enrollment{}, mapError(e)
	}
	v, e := s.Enrollments.Enroll(ctx, studentID, r, SKSLimit(student.IPK))
	switch {
	case e == nil:
		return v, nil
	case errors.Is(e, repository.ErrDuplicate):
		return v, helper.Error(409, "Mata kuliah sudah diambil pada tahun akademik ini")
	case errors.Is(e, repository.ErrFull):
		return v, helper.Validation(map[string][]string{"course_id": {"Kuota mata kuliah penuh"}})
	case errors.Is(e, repository.ErrNotFound):
		return v, helper.Validation(map[string][]string{"course_id": {"Mata kuliah tidak ditemukan"}})
	}
	var limit *repository.LimitError
	if errors.As(e, &limit) {
		return v, helper.Validation(map[string][]string{"course_id": {fmt.Sprintf("Batas SKS terlampaui; sisa SKS %d", limit.Remaining)}})
	}
	return v, helper.Internal(e)
}
func (s EnrollmentService) Cancel(ctx context.Context, id, studentID int) error {
	e := s.Enrollments.Cancel(ctx, id, studentID)
	if errors.Is(e, repository.ErrForbidden) {
		return helper.Error(403, "Enrollment milik mahasiswa lain")
	}
	return mapError(e)
}
