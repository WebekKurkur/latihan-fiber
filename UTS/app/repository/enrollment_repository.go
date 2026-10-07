package repository

import (
	"context"
	"errors"
	"siakad-uts/app/model"

	"github.com/jackc/pgx/v5"
)

type EnrollmentRepository interface {
	StudentEnrollments(context.Context, int) ([]model.Enrollment, int, error)
	Enroll(context.Context, int, model.CreateEnrollmentRequest, int) (model.Enrollment, error)
	Cancel(context.Context, int, int) error
}

var ErrFull = errors.New("kuota penuh")
var ErrSKS = errors.New("batas sks terlampaui")
var ErrForbidden = errors.New("bukan pemilik enrollment")

type LimitError struct{ Remaining int }

func (e *LimitError) Error() string { return "batas sks terlampaui" }
func (r *Postgres) StudentEnrollments(ctx context.Context, id int) ([]model.Enrollment, int, error) {
	rows, e := r.Pool.Query(ctx, `SELECT e.id,
										 e.student_id,
										 e.course_id,
										 e.tahun_akademik,
										 e.created_at,
										 c.id,
										 c.kode_mk,
										 c.nama_mk,
										 c.sks,
										 c.semester,
										 c.kuota
									FROM siakad_uts.enrollments e
									JOIN siakad_uts.courses c ON c.id=e.course_id
									WHERE e.student_id=$1
									ORDER BY e.created_at,e.id`, id)
	if e != nil {
		return nil, 0, e
	}
	defer rows.Close()
	out := []model.Enrollment{}
	total := 0
	for rows.Next() {
		var v model.Enrollment
		var c model.Course
		e = rows.Scan(&v.ID, &v.StudentID, &v.CourseID, &v.TahunAkademik, &v.CreatedAt, &c.ID, &c.KodeMK, &c.NamaMK, &c.SKS, &c.Semester, &c.Kuota)
		if e != nil {
			return nil, 0, e
		}
		v.Course = &c
		total += c.SKS
		out = append(out, v)
	}
	return out, total, rows.Err()
}
func (r *Postgres) Enroll(ctx context.Context, studentID int, req model.CreateEnrollmentRequest, limit int) (model.Enrollment, error) {
	tx, e := r.Pool.Begin(ctx)
	if e != nil {
		return model.Enrollment{}, e
	}
	defer tx.Rollback(ctx)
	var activeID int
	var ipk *float64
	e = tx.QueryRow(ctx, `SELECT id,ipk_terakhir 
							FROM siakad_uts.students 
							WHERE id=$1 AND deleted_at IS NULL FOR UPDATE`, studentID).Scan(&activeID, &ipk)
	if errors.Is(e, pgx.ErrNoRows) {
		return model.Enrollment{}, ErrNotFound
	}
	if e != nil {
		return model.Enrollment{}, e
	}
	var sks, kuota int
	e = tx.QueryRow(ctx, `SELECT sks,kuota 
							FROM siakad_uts.courses 
							WHERE id=$1 FOR UPDATE`, req.CourseID).Scan(&sks, &kuota)
	if errors.Is(e, pgx.ErrNoRows) {
		return model.Enrollment{}, ErrNotFound
	}
	if e != nil {
		return model.Enrollment{}, e
	}
	var exists bool
	e = tx.QueryRow(ctx, `SELECT EXISTS(
										SELECT 1 
										FROM siakad_uts.enrollments 
										WHERE student_id=$1 AND course_id=$2 AND tahun_akademik=$3
										)`, studentID, req.CourseID, req.TahunAkademik).Scan(&exists)
	if e != nil {
		return model.Enrollment{}, e
	}
	if exists {
		return model.Enrollment{}, ErrDuplicate
	}
	var filled int
	e = tx.QueryRow(ctx, `SELECT count(*) FROM siakad_uts.enrollments 
						   WHERE course_id=$1`, req.CourseID).Scan(&filled)
	if e != nil {
		return model.Enrollment{}, e
	}
	if filled >= kuota {
		return model.Enrollment{}, ErrFull
	}
	var used int
	e = tx.QueryRow(ctx, `SELECT coalesce(sum(c.sks),0)::int FROM siakad_uts.enrollments e 
							JOIN siakad_uts.courses c ON c.id=e.course_id 
							WHERE e.student_id=$1 AND e.tahun_akademik=$2`, studentID, req.TahunAkademik).Scan(&used)
	if e != nil {
		return model.Enrollment{}, e
	}
	if used+sks > sksLimit(ipk) {
		return model.Enrollment{}, &LimitError{Remaining: sksLimit(ipk) - used}
	}
	var v model.Enrollment
	e = tx.QueryRow(ctx, `INSERT INTO siakad_uts.enrollments(student_id,course_id,tahun_akademik) 
						  VALUES ($1,$2,$3) RETURNING id,student_id,course_id,tahun_akademik,created_at`, studentID, req.CourseID, req.TahunAkademik).Scan(&v.ID, &v.StudentID, &v.CourseID, &v.TahunAkademik, &v.CreatedAt)
	if uniqueViolation(e) {
		return model.Enrollment{}, ErrDuplicate
	}
	if e != nil {
		return model.Enrollment{}, e
	}
	return v, tx.Commit(ctx)
}
func (r *Postgres) Cancel(ctx context.Context, id, studentID int) error {
	tx, e := r.Pool.Begin(ctx)
	if e != nil {
		return e
	}
	defer tx.Rollback(ctx)
	var activeID int
	e = tx.QueryRow(ctx, `SELECT id FROM siakad_uts.students 
						  WHERE id=$1 AND deleted_at IS NULL FOR UPDATE`, studentID).Scan(&activeID)
	if errors.Is(e, pgx.ErrNoRows) {
		return ErrNotFound
	}
	if e != nil {
		return e
	}
	var owner, courseID int
	e = tx.QueryRow(ctx, `SELECT student_id,course_id FROM siakad_uts.enrollments 
							WHERE id=$1`, id).Scan(&owner, &courseID)
	if errors.Is(e, pgx.ErrNoRows) {
		return ErrNotFound
	}
	if e != nil {
		return e
	}
	if owner != studentID {
		return ErrForbidden
	}
	var locked int
	e = tx.QueryRow(ctx, `SELECT id FROM siakad_uts.courses 
							WHERE id=$1 FOR UPDATE`, courseID).Scan(&locked)
	if e != nil {
		return e
	}
	tag, e := tx.Exec(ctx, `DELETE FROM siakad_uts.enrollments 
							WHERE id=$1 AND student_id=$2`, id, studentID)
	if e != nil {
		return e
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return tx.Commit(ctx)
}

func sksLimit(ipk *float64) int {
	if ipk != nil && *ipk >= 3 {
		return 24
	}
	if ipk != nil && *ipk >= 2.5 {
		return 21
	}
	return 18
}
