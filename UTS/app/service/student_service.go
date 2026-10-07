package service

import (
	"context"
	"errors"
	"siakad-uts/app/model"
	"siakad-uts/app/repository"
	"siakad-uts/helper"
	"strings"
)

type StudentService struct {
	Students    repository.StudentRepository
	Enrollments repository.EnrollmentRepository
}

func SKSLimit(ipk *float64) int {
	if ipk != nil && *ipk >= 3 {
		return 24
	}
	if ipk != nil && *ipk >= 2.5 {
		return 21
	}
	return 18
}
func (s StudentService) List(ctx context.Context, q model.PageQuery) ([]model.Student, *model.Meta, error) {
	list, total, e := s.Students.ListStudents(ctx, q)
	if e != nil {
		return nil, nil, helper.Internal(e)
	}
	return list, &model.Meta{CurrentPage: q.Page, PerPage: q.PerPage, Total: total, LastPage: (total + q.PerPage - 1) / q.PerPage}, nil
}
func (s StudentService) Detail(ctx context.Context, id int, identity model.Identity) (model.StudentDetail, error) {
	if identity.Role != "admin" && (identity.StudentID == nil || *identity.StudentID != id) {
		return model.StudentDetail{}, helper.Error(403, "Tidak berhak mengakses data mahasiswa lain")
	}
	student, e := s.Students.StudentByID(ctx, id)
	if e != nil {
		return model.StudentDetail{}, mapError(e)
	}
	items, total, e := s.Enrollments.StudentEnrollments(ctx, id)
	if e != nil {
		return model.StudentDetail{}, helper.Internal(e)
	}
	return model.StudentDetail{Student: student, MataKuliah: items, TotalSKS: total, BatasSKS: SKSLimit(student.IPK)}, nil
}
func (s StudentService) Create(ctx context.Context, r model.CreateStudentRequest) (model.Student, error) {
	r.NIM = strings.TrimSpace(r.NIM)
	r.Nama = strings.TrimSpace(r.Nama)
	r.Email = strings.TrimSpace(r.Email)
	r.Prodi = strings.TrimSpace(r.Prodi)
	if e := helper.ValidateStruct(r); e != nil {
		return model.Student{}, e
	}
	hash, e := helper.HashPassword(r.NIM)
	if e != nil {
		return model.Student{}, helper.Internal(e)
	}
	v, e := s.Students.CreateStudent(ctx, r, hash)
	if errors.Is(e, repository.ErrDuplicate) {
		return v, helper.Validation(map[string][]string{"nim": {"NIM atau email sudah terdaftar"}, "email": {"NIM atau email sudah terdaftar"}})
	}
	if e != nil {
		return v, helper.Internal(e)
	}
	return v, nil
}
func (s StudentService) Update(ctx context.Context, id int, r model.UpdateStudentRequest) (model.Student, error) {
	r.Nama = strings.TrimSpace(r.Nama)
	r.Prodi = strings.TrimSpace(r.Prodi)
	if e := helper.ValidateStruct(r); e != nil {
		return model.Student{}, e
	}
	v, e := s.Students.UpdateStudent(ctx, id, r)
	if e != nil {
		return v, mapError(e)
	}
	return v, nil
}
func (s StudentService) Delete(ctx context.Context, id int) error {
	return mapError(s.Students.DeleteStudent(ctx, id))
}
func mapError(e error) error {
	if e == nil {
		return nil
	}
	if errors.Is(e, repository.ErrNotFound) {
		return helper.Error(404, "Data tidak ditemukan")
	}
	return helper.Internal(e)
}
