package repository

import (
	"context"
	"errors"
	"fmt"
	"siakad-uts/app/model"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

type StudentRepository interface {
	ListStudents(context.Context, model.PageQuery) ([]model.Student, int, error)
	StudentByID(context.Context, int) (model.Student, error)
	CreateStudent(context.Context, model.CreateStudentRequest, string) (model.Student, error)
	UpdateStudent(context.Context, int, model.UpdateStudentRequest) (model.Student, error)
	DeleteStudent(context.Context, int) error
}

const studentCols = `id,user_id,nim,nama,prodi,angkatan,ipk_terakhir`

func scanStudent(row pgx.Row) (model.Student, error) {
	var s model.Student
	e := row.Scan(&s.ID, &s.UserID, &s.NIM, &s.Nama, &s.Prodi, &s.Angkatan, &s.IPK)
	if errors.Is(e, pgx.ErrNoRows) {
		return s, ErrNotFound
	}
	return s, e
}
func uniqueViolation(e error) bool {
	var p *pgconn.PgError
	return errors.As(e, &p) && p.Code == "23505"
}
func (r *Postgres) ListStudents(ctx context.Context, q model.PageQuery) ([]model.Student, int, error) {
	where := ` WHERE deleted_at IS NULL AND ($1='' OR prodi=$1) AND ($2=0 OR angkatan=$2) AND ($3='' OR nim ILIKE '%'||$3||'%' OR nama ILIKE '%'||$3||'%')`
	args := []any{q.Prodi, q.Angkatan, q.Search}
	var total int
	if e := r.Pool.QueryRow(ctx, `SELECT count(*) FROM siakad_uts.students`+where, args...).Scan(&total); e != nil {
		return nil, 0, e
	}
	sort := `nama ASC,id ASC`
	if q.Sort == "-ipk_terakhir" {
		sort = `ipk_terakhir DESC NULLS LAST,id ASC`
	}
	rows, e := r.Pool.Query(ctx, fmt.Sprintf(`SELECT %s FROM siakad_uts.students%s 
											  ORDER BY %s LIMIT $4 OFFSET $5`, studentCols, where, sort),
		q.Prodi, q.Angkatan, q.Search, q.PerPage, (q.Page-1)*q.PerPage)
	if e != nil {
		return nil, 0, e
	}
	defer rows.Close()
	result := []model.Student{}
	for rows.Next() {
		s, e := scanStudent(rows)
		if e != nil {
			return nil, 0, e
		}
		result = append(result, s)
	}
	return result, total, rows.Err()
}
func (r *Postgres) StudentByID(ctx context.Context, id int) (model.Student, error) {
	return scanStudent(r.Pool.QueryRow(ctx, `SELECT `+studentCols+` FROM siakad_uts.students WHERE id=$1 AND deleted_at IS NULL`, id))
}
func (r *Postgres) CreateStudent(ctx context.Context, req model.CreateStudentRequest, hash string) (model.Student, error) {
	tx, e := r.Pool.Begin(ctx)
	if e != nil {
		return model.Student{}, e
	}
	defer tx.Rollback(ctx)
	var userID int
	e = tx.QueryRow(ctx, `INSERT INTO siakad_uts.users(email,password,role) 
						  VALUES (lower($1),$2,'mahasiswa') RETURNING id`, req.Email, hash).Scan(&userID)
	if uniqueViolation(e) {
		return model.Student{}, ErrDuplicate
	}
	if e != nil {
		return model.Student{}, e
	}
	s, e := scanStudent(tx.QueryRow(ctx, `INSERT INTO siakad_uts.students(user_id,nim,nama,prodi,angkatan,ipk_terakhir) 
										  VALUES ($1,$2,$3,$4,$5,$6) RETURNING `+studentCols, userID, req.NIM, req.Nama, req.Prodi, req.Angkatan, req.IPK))
	if uniqueViolation(e) {
		return model.Student{}, ErrDuplicate
	}
	if e != nil {
		return model.Student{}, e
	}
	return s, tx.Commit(ctx)
}
func (r *Postgres) UpdateStudent(ctx context.Context, id int, req model.UpdateStudentRequest) (model.Student, error) {
	return scanStudent(r.Pool.QueryRow(ctx, `UPDATE siakad_uts.students SET nama=$2,prodi=$3,angkatan=$4,ipk_terakhir=$5 
											 WHERE id=$1 AND deleted_at IS NULL RETURNING `+studentCols, id, req.Nama, req.Prodi, req.Angkatan, req.IPK))
}
func (r *Postgres) DeleteStudent(ctx context.Context, id int) error {
	tag, e := r.Pool.Exec(ctx, `UPDATE siakad_uts.students SET deleted_at=now() 
								WHERE id=$1 AND deleted_at IS NULL`, id)
	if e != nil {
		return e
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}
