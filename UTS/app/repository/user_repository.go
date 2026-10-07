package repository

import (
	"context"
	"errors"
	"siakad-uts/app/model"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrNotFound = errors.New("data tidak ditemukan")
var ErrDuplicate = errors.New("data duplikat")

type UserRepository interface {
	ByEmail(context.Context, string) (model.User, error)
	ByID(context.Context, int) (model.User, error)
}
type Postgres struct{ Pool *pgxpool.Pool }

func New(pool *pgxpool.Pool) *Postgres { return &Postgres{Pool: pool} }
func (r *Postgres) ByEmail(ctx context.Context, email string) (model.User, error) {
	var u model.User
	e := r.Pool.QueryRow(ctx, `SELECT u.id,
									  u.email,
									  u.password,
									  u.role,
									  s.id FROM siakad_uts.users u 
									  LEFT JOIN siakad_uts.students s ON s.user_id=u.id AND s.deleted_at IS NULL 
									  WHERE lower(u.email)=lower($1) AND (u.role='admin' OR s.id IS NOT NULL)`, email).Scan(&u.ID, &u.Email, &u.Password, &u.Role, &u.StudentID)
	if errors.Is(e, pgx.ErrNoRows) {
		return u, ErrNotFound
	}
	return u, e
}
func (r *Postgres) ByID(ctx context.Context, id int) (model.User, error) {
	var u model.User
	e := r.Pool.QueryRow(ctx, `SELECT u.id,
									  u.email,
									  u.password,
									  u.role,
									  s.id FROM siakad_uts.users u LEFT JOIN siakad_uts.students s ON s.user_id=u.id AND s.deleted_at IS NULL 
									  WHERE u.id=$1 AND (u.role='admin' OR s.id IS NOT NULL)`, id).Scan(&u.ID, &u.Email, &u.Password, &u.Role, &u.StudentID)
	if errors.Is(e, pgx.ErrNoRows) {
		return u, ErrNotFound
	}
	return u, e
}
