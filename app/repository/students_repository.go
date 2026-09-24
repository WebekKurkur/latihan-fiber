package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"latihan-fiber/app/model"
)

// Sentinel error: error milik lapisan repository, bukan error milik pgx.
// Lapisan atas cukup mengenal dua ini dan tidak perlu tahu basis datanya apa.
var (
	ErrNotFound  = errors.New("data tidak ditemukan")
	ErrDuplicate = errors.New("data sudah ada")
)

// StudentRepository adalah KONTRAK penyimpanan data student.
// Perhatikan: tidak ada satu pun kata "SQL" atau "postgres" di sini.
type StudentRepository interface {
	FindAll(ctx context.Context, q model.ListQuery) ([]model.Student, int, error)
	FindAfterCursor(ctx context.Context, q model.CursorQuery) ([]model.Student, error)
	FindByID(ctx context.Context, id int) (model.Student, error)
	FindOwnerID(ctx context.Context, id int) (*int, error)
	FindByUsername(ctx context.Context, name string) (model.Student, error)
	Create(ctx context.Context, s model.Student) (model.Student, error)
	Update(ctx context.Context, s model.Student) (model.Student, error)
	UpdateRole(ctx context.Context, id int, role string) (model.Student, error)
	Delete(ctx context.Context, id int) error
}

// kolomUrut adalah daftar putih: pemetaan dari nilai yang boleh dikirim klien
// ke nama kolom yang sebenarnya. ORDER BY tidak dapat memakai parameter,
// sehingga nama kolom terpaksa disisipkan sebagai teks. Daftar putih inilah
// satu-satunya hal yang mencegah SQL injection di titik ini.
//
// Catatan: kolom NIM dan Grade ditulis dengan tanda kutip karena migrasi
// mendeklarasikannya sebagai "NIM" dan "Grade" (case-sensitive di PostgreSQL).
var kolomUrut = map[string]string{
	"id":         "id",
	"name":       "name",
	"nim":        `"NIM"`,
	"grade":      `"Grade"`,
	"created_at": "created_at",
}

// studentColumns adalah daftar eksplisit kolom yang dibaca dari tabel
// students. Dipakai oleh FindAfterCursor (cursor pagination).
var studentColumns = `id, name, "NIM", "Grade", role, is_active, created_at`

// scanStudent membaca satu baris menjadi model.Student.
func scanStudent(rows pgx.Row) (model.Student, error) {
	var s model.Student
	if err := rows.Scan(&s.ID, &s.Name, &s.NIM, &s.Grade, &s.Role,
		&s.IsActive, &s.CreatedAt); err != nil {
		return model.Student{}, err
	}
	return s, nil
}

type studentPostgresRepository struct {
	pool *pgxpool.Pool
}

// NewStudentRepository mengembalikan interface, bukan struct konkret.
func NewStudentRepository(pool *pgxpool.Pool) StudentRepository {
	return &studentPostgresRepository{pool: pool}
}

// buildFilter menyusun bagian WHERE beserta argumennya.
// Nilai dari klien SELALU menjadi argumen ($1, $2, ...), tidak pernah
// disambung langsung ke dalam teks SQL.
func buildFilter(q model.ListQuery) (string, []any) {
	where := " WHERE 1 = 1"
	args := []any{}

	if q.Search != "" {
		// Modul 3 bagian C: pencarian pada nama menggunakan ILIKE.
		where += fmt.Sprintf(" AND name ILIKE $%d", len(args)+1)
		args = append(args, "%"+q.Search+"%")
	}

	if q.IsActive != nil {
		where += fmt.Sprintf(" AND is_active = $%d", len(args)+1)
		args = append(args, *q.IsActive)
	}

	return where, args
}

func (r *studentPostgresRepository) FindAll(
	ctx context.Context, q model.ListQuery,
) ([]model.Student, int, error) {
	where, args := buildFilter(q)

	// 1) Hitung total sebelum dipenggal, untuk keperluan meta.
	var total int
	err := r.pool.QueryRow(ctx, "SELECT COUNT(*) FROM students"+where, args...).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("menghitung student: %w", err)
	}

	// 2) Ambil satu halaman saja. Penyaringan, pengurutan, dan pemenggalan
	//    dikerjakan basis data, bukan oleh Go.
	arah := "ASC"
	if q.Order == "desc" {
		arah = "DESC"
	}

	sqlText := fmt.Sprintf(
		`SELECT id, name, "NIM", "Grade", role, is_active, created_at
         FROM students%s
         ORDER BY %s %s
         LIMIT $%d OFFSET $%d`,
		where, kolomUrut[q.Sort], arah, len(args)+1, len(args)+2,
	)
	args = append(args, q.Limit, q.Offset())

	rows, err := r.pool.Query(ctx, sqlText, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("mengambil daftar student: %w", err)
	}
	defer rows.Close()

	hasil := []model.Student{}
	for rows.Next() {
		var s model.Student
		if err := rows.Scan(&s.ID, &s.Name, &s.NIM, &s.Grade, &s.Role,
			&s.IsActive, &s.CreatedAt); err != nil {
			return nil, 0, fmt.Errorf("membaca baris student: %w", err)
		}
		hasil = append(hasil, s)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("membaca hasil query: %w", err)
	}

	return hasil, total, nil
}

func (r *studentPostgresRepository) FindByID(
	ctx context.Context, id int,
) (model.Student, error) {
	var s model.Student

	err := r.pool.QueryRow(ctx,
		`SELECT id, name, "NIM", "Grade", password, role, is_active, created_at
         FROM students WHERE id = $1`, id,
	).Scan(&s.ID, &s.Name, &s.NIM, &s.Grade, &s.Password, &s.Role,
		&s.IsActive, &s.CreatedAt)

	if err != nil {
		// pgx.ErrNoRows diterjemahkan menjadi error milik kita sendiri.
		if errors.Is(err, pgx.ErrNoRows) {
			return model.Student{}, ErrNotFound
		}
		return model.Student{}, fmt.Errorf("mengambil student: %w", err)
	}

	return s, nil
}

func (r *studentPostgresRepository) Create(
	ctx context.Context, s model.Student,
) (model.Student, error) {
	// RETURNING membuat id dan created_at hasil buatan basis data
	// langsung ikut kembali, tanpa perlu query kedua.
	//
	// owner_id ikut disimpan di sini, namun nilai yang dipakai selalu
	// berasal dari service (diambil dari token). Nilai dari body request
	// tidak pernah menyentuh query ini.
	var ownerID any
	if s.OwnerID != nil {
		ownerID = *s.OwnerID
	}
	err := r.pool.QueryRow(ctx,
		`INSERT INTO students (name, "NIM", "Grade", password, role, is_active, owner_id)
		 VALUES ($1, $2, $3, $4, $5, $6, $7)
         RETURNING id, created_at`,
		s.Name, s.NIM, s.Grade, s.Password, s.Role, s.IsActive, ownerID,
	).Scan(&s.ID, &s.CreatedAt)

	if err != nil {
		if isUniqueViolation(err) {
			return model.Student{}, ErrDuplicate
		}
		return model.Student{}, fmt.Errorf("menyimpan student: %w", err)
	}

	return s, nil
}

// FindAfterCursor mengambil satu halaman memakai keyset pagination.
//
// id ikut dibandingkan karena created_at TIDAK dijamin unik. Bila dua
// baris dibuat pada mikrodetik yang sama dan hanya created_at yang
// dibandingkan, salah satu baris akan terlewat atau terkirim dua kali.
//
// Jumlah yang diminta sengaja limit+1. Baris tambahan itu tidak dikirim
// ke client; keberadaannya hanya dipakai untuk menjawab "masih ada
// halaman berikutnya?" tanpa perlu COUNT(*) atas seluruh tabel.
func (r *studentPostgresRepository) FindAfterCursor(
	ctx context.Context, q model.CursorQuery,
) ([]model.Student, error) {
	args := []any{}
	where := " WHERE 1 = 1"
	if q.Search != "" {
		args = append(args, "%"+q.Search+"%")
		where += fmt.Sprintf(" AND name ILIKE $%d", len(args))
	}
	if q.IsActive != nil {
		args = append(args, *q.IsActive)
		where += fmt.Sprintf(" AND is_active = $%d", len(args))
	}
	if q.After != nil {
		args = append(args, q.After.CreatedAt, q.After.ID)
		where += fmt.Sprintf(" AND (created_at, id) < ($%d, $%d)", len(args)-1, len(args))
	}
	args = append(args, q.Limit+1)
	query := fmt.Sprintf(
		"SELECT %s FROM students%s ORDER BY created_at ASC, id ASC LIMIT $%d",
		studentColumns, where, len(args),
	)
	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("mengambil daftar student: %w", err)
	}
	defer rows.Close()
	result := []model.Student{}
	for rows.Next() {
		s, err := scanStudent(rows)
		if err != nil {
			return nil, fmt.Errorf("membaca row student: %w", err)
		}
		result = append(result, s)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("membaca hasil query: %w", err)
	}
	return result, nil
}

// FindOwnerID hanya mengambil kolom owner_id. Dipakai oleh service
// untuk pemeriksaan kepemilikan SEBELUM data lengkap dibaca, sehingga
// serangan timing tidak dapat membedakan id yang ada dari yang tidak.
func (r *studentPostgresRepository) FindOwnerID(
	ctx context.Context, id int,
) (*int, error) {
	var ownerID *int
	err := r.pool.QueryRow(ctx,
		`SELECT owner_id FROM students WHERE id = $1`, id,
	).Scan(&ownerID)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("mengambil owner student: %w", err)
	}
	return ownerID, nil
}

func (r *studentPostgresRepository) Update(
	ctx context.Context, s model.Student,
) (model.Student, error) {
	// RETURNING mengembalikan baris hasil perubahan dalam satu perjalanan,
	// sehingga field yang tidak ikut diubah (created_at) tetap terisi benar.
	err := r.pool.QueryRow(ctx,
		`UPDATE students
         SET name = $1, "NIM" = $2, "Grade" = $3, is_active = $4
         WHERE id = $5
         RETURNING id, name, "NIM", "Grade", is_active, created_at`,
		s.Name, s.NIM, s.Grade, s.IsActive, s.ID,
	).Scan(&s.ID, &s.Name, &s.NIM, &s.Grade, &s.IsActive, &s.CreatedAt)

	if err != nil {
		// Tidak ada baris yang dikembalikan berarti id-nya memang tidak ada.
		if errors.Is(err, pgx.ErrNoRows) {
			return model.Student{}, ErrNotFound
		}
		if isUniqueViolation(err) {
			return model.Student{}, ErrDuplicate
		}
		return model.Student{}, fmt.Errorf("memperbarui student: %w", err)
	}

	return s, nil
}

// UpdateRole sengaja dipisah dari Update. Mengubah role adalah tindakan
// istimewa yang dijaga permission tersendiri, sehingga tidak boleh ikut
// terbawa oleh endpoint perubahan data biasa.
//
// Kolom password sengaja tidak dikembalikan: endpoint perubahan role
// tidak perlu dan tidak boleh membocorkan hash password.
func (r *studentPostgresRepository) UpdateRole(
	ctx context.Context, id int, role string,
) (model.Student, error) {
	var s model.Student
	err := r.pool.QueryRow(ctx,
		`UPDATE students
         SET role = $1
         WHERE id = $2
         RETURNING id, name, "NIM", "Grade", role, is_active, created_at`,
		role, id,
	).Scan(&s.ID, &s.Name, &s.NIM, &s.Grade, &s.Role,
		&s.IsActive, &s.CreatedAt)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return model.Student{}, ErrNotFound
		}
		return model.Student{}, fmt.Errorf("mengubah role student: %w", err)
	}
	return s, nil
}

func (r *studentPostgresRepository) Delete(ctx context.Context, id int) error {
	tag, err := r.pool.Exec(ctx, `DELETE FROM students WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("menghapus student: %w", err)
	}

	// Perintah berhasil dijalankan, tetapi tidak ada baris yang terkena.
	// Artinya id-nya memang tidak ada.
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}

	return nil
}

// isUniqueViolation memeriksa apakah error berasal dari pelanggaran
// batasan UNIQUE. Kode 23505 adalah kode resmi PostgreSQL untuk itu.
func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code == "23505"
	}
	return false
}

// FindByUsername dipakai saat login. Pencocokan tidak membedakan
// huruf besar dan kecil, sama seperti unique index-nya.
func (r *studentPostgresRepository) FindByUsername(
	ctx context.Context, username string,
) (model.Student, error) {
	var s model.Student

	err := r.pool.QueryRow(ctx,
		`SELECT id, name, "NIM", "Grade", password, role, is_active, created_at
         FROM students WHERE LOWER(name) = LOWER($1)`, username,
	).Scan(&s.ID, &s.Name, &s.NIM, &s.Grade, &s.Password, &s.Role,
		&s.IsActive, &s.CreatedAt)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return model.Student{}, ErrNotFound
		}
		return model.Student{}, fmt.Errorf("mengambil student: %w", err)
	}

	return s, nil
}
