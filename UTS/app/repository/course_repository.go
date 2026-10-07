package repository

import (
	"context"
	"siakad-uts/app/model"

	"github.com/jackc/pgx/v5"
)

type CourseRepository interface {
	ListCourses(context.Context, model.CourseQuery) ([]model.Course, error)
}

func scanCourse(row pgx.Row) (model.Course, error) {
	var c model.Course
	e := row.Scan(&c.ID, &c.KodeMK, &c.NamaMK, &c.SKS, &c.Semester, &c.Kuota, &c.Terisi, &c.SisaKuota)
	return c, e
}
func (r *Postgres) ListCourses(ctx context.Context, q model.CourseQuery) ([]model.Course, error) {
	rows, e := r.Pool.Query(ctx, `SELECT c.id,
										c.kode_mk,
										c.nama_mk,
										c.sks,
										c.semester,
										c.kuota,
										count(e.id)::int AS terisi,
										(c.kuota-count(e.id))::int AS sisa_kuota
									FROM siakad_uts.courses c
									LEFT JOIN siakad_uts.enrollments e ON e.course_id=c.id
									WHERE ($1=0 OR c.semester=$1) AND ($2='' OR c.kode_mk ILIKE '%'||$2||'%' OR c.nama_mk ILIKE '%'||$2||'%')
									GROUP BY c.id
									HAVING (NOT $3::boolean OR count(e.id)<c.kuota)
									ORDER BY c.kode_mk`, q.Semester, q.Search, q.Available)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	result := []model.Course{}
	for rows.Next() {
		c, e := scanCourse(rows)
		if e != nil {
			return nil, e
		}
		result = append(result, c)
	}
	return result, rows.Err()
}
