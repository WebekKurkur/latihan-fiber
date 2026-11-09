package repository

import (
	"context"
	"errors"
	"fmt"

	"latihan-fiber/app/model"

	"github.com/jackc/pgx/v5"
)

var (
	ErrCourseFull       = errors.New("kuota kelas sudah penuh")
	ErrNotEnrolled      = errors.New("tidak terdaftar di kelas ini")
	ErrVersionConflict  = errors.New("data sudah diubah pihak lain")
	ErrQuotaBelowFilled = errors.New("kuota lebih kecil dari jumlah peserta")
)

type CourseRepository interface {
	// WithTx mengembalikan salinan repository yang menjalankan seluruh
	// query-nya di dalam transaction tx.
	WithTx(tx DBTX) CourseRepository
	FindAll(ctx context.Context) ([]model.Course, error)
	FindByID(ctx context.Context, id int) (model.Course, error)
	FindByStudent(ctx context.Context, studentID int) ([]model.Course, error)
	InsertEnrollment(ctx context.Context, courseID, studentID int) error
	UpdateQuota(ctx context.Context, id, quota, version int) (model.Course, error)
	// FindEnrolledStudentIDs mengembalikan daftar peserta sebuah kelas.
	// Dipakai untuk memberi tahu seluruh peserta ketika kuota berubah.
	FindEnrolledStudentIDs(ctx context.Context, courseID int) ([]int, error)
	IncrementEnrolled(ctx context.Context, courseID int) (model.Course, error)
	DecrementEnrolled(ctx context.Context, courseID int) (model.Course, error)
	DeleteEnrollment(ctx context.Context, courseID, studentID int) error
	LockStudent(ctx context.Context, studentID int) error
	SumCredits(ctx context.Context, studentID int) (int, error)
	LockCourses(ctx context.Context, ids ...int) error
	IsEnrolled(ctx context.Context, courseID, studentID int) (bool, error)
}

const courseColumns = `id, code, name, credits, quota, enrolled_count, version, created_at`

type coursePostgresRepository struct{ db DBTX }

// NewCourseRepository menerima DBTX. Pada main.go yang dikirim adalah
// pool, sehingga repository ini bekerja di luar transaction sampai
// WithTx dipanggil.
func NewCourseRepository(db DBTX) CourseRepository {
	return &coursePostgresRepository{db: db}
}
func (r *coursePostgresRepository) WithTx(tx DBTX) CourseRepository {
	return &coursePostgresRepository{db: tx}
}
func scanCourse(row pgx.Row) (model.Course, error) {
	var c model.Course
	err := row.Scan(&c.ID, &c.Code, &c.Name, &c.Credits, &c.Quota,
		&c.EnrolledCount, &c.Version, &c.CreatedAt)
	return c, err
}
func (r *coursePostgresRepository) collect(rows pgx.Rows, err error) ([]model.Course, error) {
	if err != nil {
		return nil, fmt.Errorf("mengambil daftar kelas: %w", err)
	}
	defer rows.Close()
	result := []model.Course{}
	for rows.Next() {
		c, err := scanCourse(rows)
		if err != nil {
			return nil, fmt.Errorf("membaca row kelas: %w", err)
		}
		result = append(result, c)
	}
	return result, rows.Err()
}
func (r *coursePostgresRepository) FindAll(ctx context.Context) ([]model.Course, error) {
	return r.collect(r.db.Query(ctx,
		"SELECT "+courseColumns+" FROM courses ORDER BY code"))
}
func (r *coursePostgresRepository) FindByID(ctx context.Context, id int) (model.Course, error) {
	c, err := scanCourse(r.db.QueryRow(ctx,
		"SELECT "+courseColumns+" FROM courses WHERE id = $1", id))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return model.Course{}, ErrNotFound
		}
		return model.Course{}, fmt.Errorf("mengambil kelas: %w", err)
	}
	return c, nil
}
func (r *coursePostgresRepository) FindByStudent(ctx context.Context, studentID int) ([]model.Course,
	error) {
	return r.collect(r.db.Query(ctx,
		`SELECT c.id, c.code, c.name, c.credits, c.quota, c.enrolled_count, c.version, c.created_at
 FROM courses c
 JOIN enrollments e ON e.course_id = c.id
 WHERE e.student_id = $1
 ORDER BY c.code`, studentID))
}

func (r *coursePostgresRepository) InsertEnrollment(ctx context.Context, courseID, studentID int) error {
	_, err := r.db.Exec(ctx,
		"INSERT INTO enrollments (course_id, student_id) VALUES ($1, $2)", courseID, studentID)
	if err != nil {
		if pgErrorCode(err) == codeUniqueViolation {
			return ErrDuplicate
		}
		return fmt.Errorf("mencatat pendaftaran: %w", err)
	}
	return nil
}

// LockStudent mengunci row milik satu mahasiswa sampai transaction selesai.
//
// Row students dipakai sebagai "gembok KRS": selama satu transaction memegang
// kunci ini, transaction lain milik mahasiswa YANG SAMA harus antre.
// Mahasiswa lain tidak terpengaruh, karena row yang dikunci berbeda.
//
// Tanpa FOR UPDATE, SELECT ini tidak mengunci apa pun dan pemeriksaan
// batas SKS sesudahnya dapat diakali dengan request yang serentak.
func (r *coursePostgresRepository) LockStudent(ctx context.Context, studentID int) error {
	var id int
	err := r.db.QueryRow(ctx,
		"SELECT id FROM students WHERE id = $1 FOR UPDATE", studentID).Scan(&id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		return fmt.Errorf("mengunci data mahasiswa: %w", err)
	}
	return nil
}
func (r *coursePostgresRepository) SumCredits(ctx context.Context, studentID int) (int, error) {
	var total int
	err := r.db.QueryRow(ctx,
		`SELECT COALESCE(SUM(c.credits), 0)
 FROM enrollments e
 JOIN courses c ON c.id = e.course_id
 WHERE e.student_id = $1`, studentID).Scan(&total)
	if err != nil {
		return 0, fmt.Errorf("menghitung SKS: %w", err)
	}
	return total, nil
}

// IncrementEnrolled mengambil satu kursi secara ATOMIK.
//
// Pemeriksaan kuota dan penambahan jumlah peserta terjadi dalam satu
// pernyataan UPDATE. PostgreSQL mengunci row tersebut selama UPDATE
// berjalan, sehingga dua request yang datang bersamaan tidak mungkin
// sama-sama melihat "masih ada satu kursi kosong".
func (r *coursePostgresRepository) IncrementEnrolled(ctx context.Context, courseID int) (model.Course, error) {
	c, err := scanCourse(r.db.QueryRow(ctx,
		`UPDATE courses SET enrolled_count = enrolled_count + 1
 WHERE id = $1 AND enrolled_count < quota
 RETURNING `+courseColumns, courseID))
	if err == nil {
		return c, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return model.Course{}, fmt.Errorf("menambah peserta: %w", err)
	}
	// Tidak ada row yang berubah. Dua kemungkinan: kelasnya tidak ada,
	// atau kelasnya ada tetapi sudah penuh. Bedakan keduanya.
	if _, findErr := r.FindByID(ctx, courseID); findErr != nil {
		return model.Course{}, findErr
	}
	return model.Course{}, ErrCourseFull
}
func (r *coursePostgresRepository) DecrementEnrolled(ctx context.Context, courseID int) (model.Course, error) {
	c, err := scanCourse(r.db.QueryRow(ctx,
		`UPDATE courses SET enrolled_count = enrolled_count - 1
 WHERE id = $1
 RETURNING `+courseColumns, courseID))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return model.Course{}, ErrNotFound
		}
		return model.Course{}, fmt.Errorf("mengurangi peserta: %w", err)
	}
	return c, nil
}

// DeleteEnrollment menghapus pendaftaran, lalu MEMERIKSA apakah memang
// ada yang terhapus. DELETE yang tidak menemukan row bukan error menurut
// PostgreSQL — ia sukses dengan nol row. Bila tidak diperiksa, kode
// sesudahnya akan mengurangi jumlah peserta untuk pendaftaran yang
// sebenarnya tidak pernah ada.
func (r *coursePostgresRepository) DeleteEnrollment(ctx context.Context, courseID, studentID int) error {
	ct, err := r.db.Exec(ctx,
		"DELETE FROM enrollments WHERE course_id = $1 AND student_id = $2", courseID, studentID)
	if err != nil {
		return fmt.Errorf("menghapus pendaftaran: %w", err)
	}
	if ct.RowsAffected() == 0 {
		return ErrNotEnrolled
	}
	return nil
}

// LockCourses mengunci beberapa kelas sekaligus, SELALU dalam urutan id
// menaik, apa pun urutan argumennya.
//
// Urutan yang seragam inilah yang mencegah deadlock: bila setiap
// transaction mengambil kunci dengan urutan yang sama, tidak mungkin ada
// dua transaction yang saling menunggu kunci milik satu sama lain.
func (r *coursePostgresRepository) LockCourses(ctx context.Context, ids ...int) error {
	rows, err := r.db.Query(ctx,
		"SELECT id FROM courses WHERE id = ANY($1) ORDER BY id FOR UPDATE", ids)
	if err != nil {
		return fmt.Errorf("mengunci kelas: %w", err)
	}
	defer rows.Close()
	found := 0
	for rows.Next() {
		found++
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("mengunci kelas: %w", err)
	}
	if found != len(ids) {
		return ErrNotFound
	}
	return nil
}

// UpdateQuota menerapkan optimistic locking.
//
// Tidak ada kunci yang dipegang selama admin mengisi form. Sebagai gantinya,
// UPDATE hanya berhasil bila version di database masih sama dengan version
// yang dikirim client. Bila sudah berubah, berarti ada orang lain yang
// menyimpan lebih dulu, dan perubahan ini ditolak alih-alih menimpanya.
//
// Syarat kuota tidak boleh lebih kecil dari jumlah peserta juga ditulis di
// dalam WHERE, agar pemeriksaan dan perubahan terjadi dalam satu langkah.
func (r *coursePostgresRepository) UpdateQuota(ctx context.Context, id, quota, version int) (model.Course, error) {
	c, err := scanCourse(r.db.QueryRow(ctx,
		`UPDATE courses SET quota = $1, version = version + 1
 WHERE id = $2 AND version = $3 AND $1 >= enrolled_count
 RETURNING `+courseColumns, quota, id, version))
	if err == nil {
		return c, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return model.Course{}, fmt.Errorf("mengubah kuota: %w", err)
	}
	current, findErr := r.FindByID(ctx, id)
	if findErr != nil {
		return model.Course{}, findErr
	}
	if current.Version != version {
		return model.Course{}, ErrVersionConflict
	}
	return model.Course{}, ErrQuotaBelowFilled
}

// FindEnrolledStudentIDs mengembalikan id seluruh peserta sebuah kelas.
//
// Urutan id menaik dipakai agar urutan pembuatan event outbox (dan dengan
// itu urutan pengiriman email) konsisten dan dapat diulang.
func (r *coursePostgresRepository) FindEnrolledStudentIDs(ctx context.Context, courseID int) ([]int, error) {
	rows, err := r.db.Query(ctx,
		`SELECT student_id FROM enrollments WHERE course_id = $1 ORDER BY student_id`, courseID)
	if err != nil {
		return nil, fmt.Errorf("mengambil peserta kelas: %w", err)
	}
	defer rows.Close()
	ids := []int{}
	for rows.Next() {
		var studentID int
		if err := rows.Scan(&studentID); err != nil {
			return nil, fmt.Errorf("membaca peserta kelas: %w", err)
		}
		ids = append(ids, studentID)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("mengambil peserta kelas: %w", err)
	}
	return ids, nil
}

// IsEnrolled dipakai layer lain — misalnya layanan ulasan di MongoDB — yang
// perlu tahu apakah seorang mahasiswa terdaftar di sebuah kelas.
func (r *coursePostgresRepository) IsEnrolled(ctx context.Context, courseID, studentID int) (bool,
	error) {
	var exists bool
	err := r.db.QueryRow(ctx,
		"SELECT EXISTS (SELECT 1 FROM enrollments WHERE course_id = $1 AND student_id = $2)",
		courseID, studentID).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("memeriksa pendaftaran: %w", err)
	}
	return exists, nil
}
