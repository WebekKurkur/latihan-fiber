package service_test

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"testing"

	"latihan-fiber/app/model"
	"latihan-fiber/app/repository"
)

// File ini berisi TEST DOUBLE untuk lapisan repository.
//
// Catatan: file ini memakai package service_test (black-box). Semua test
// yang memakainya — termasuk test pada Langkah 3 — berada di package yang
// sama, sehingga yang terlihat dari luar package service hanyalah API yang
// diekspor. Itu persis seperti kode lain di luar package.
//
// Prinsip yang dipakai (modul 12 bagian A.4):
//  1. Fake MENYERTAI interface repository sebagai field anonim. Method yang
//     tidak diimplementasi fake menjadi nil dan akan panic bila dipanggil.
//     Ini disengaja: "method ini tidak seharusnya dipakai di sini" lebih
//     baik daripada diam-diam mengembalikan data palsu.
//  2. Assertion di waktu compile
//     var _ repository.XRepository = (*fakeX)(nil)
//     menangkap perubahan interface lebih awal, bukan saat test gagal.
var (
	_ repository.PortfolioRepository = (*fakePortfolioRepository)(nil)
	_ repository.StudentRepository   = (*fakeStudentRepository)(nil)
)

// fakePortfolioRepository menyimpan portofolio di memory, menggantikan
// MongoDB.
//
// Mutex diperlukan karena Fiber melayani request pada goroutine yang
// berbeda; tanpa mutex, `go test -race` akan melaporkan data race.
//
// CATATAN PENTING (kesalahan pada berita acara baris 4): fake ini TIDAK
// boleh dibuat sekali di level package lalu dipakai bersama seluruh test.
// Urutan test (lihat flag -shuffle=on) akan menentukan hasil, dan test pun
// saling merusak data. Selalu buat instance baru di dalam test.
type fakePortfolioRepository struct {
	repository.PortfolioRepository // method lain: panic bila terpanggil

	mu         sync.Mutex
	portfolios map[int]model.StudentPortfolio
	nextID     int
}

// newFakePortfolioRepository membuat instance fake yang kosong.
// Setiap test memanggil fungsi ini sendiri sehingga mendapat fake baru.
func newFakePortfolioRepository() *fakePortfolioRepository {
	return &fakePortfolioRepository{portfolios: map[int]model.StudentPortfolio{}}
}

// newID menghasilkan id proyek berbentuk hex 24 karakter, sama seperti
// ObjectID MongoDB. Bentuknya penting: repository produksi menolak id yang
// bukan hex melalui bson.ObjectIDFromHex, sehingga fake harus menghasilkan
// id dengan bentuk yang sama agar test AddProject lalu DeleteProject bisa
// saling berurutan.
func (f *fakePortfolioRepository) newID() string {
	f.nextID++
	return fmt.Sprintf("%024x", f.nextID)
}

// FindByStudentID meniru FindOne pada collection student_portfolios.
func (f *fakePortfolioRepository) FindByStudentID(_ context.Context, studentID int) (model.StudentPortfolio, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	portfolio, ok := f.portfolios[studentID]
	if !ok {
		return model.StudentPortfolio{}, repository.ErrPortfolioNotFound
	}
	return clonePortfolio(portfolio), nil
}

// Replace meniru ReplaceOne(..., upsert=true): satu portofolio per student
// (meniru unique index student_id_unique) dan proyek tanpa id diberi id baru.
func (f *fakePortfolioRepository) Replace(_ context.Context, portfolio model.StudentPortfolio) (model.StudentPortfolio, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	stored := clonePortfolio(portfolio)
	for i := range stored.Projects {
		if stored.Projects[i].ID == "" {
			stored.Projects[i].ID = f.newID()
		}
	}
	if stored.Projects == nil {
		stored.Projects = []model.PortfolioProject{}
	}
	if stored.Certificates == nil {
		stored.Certificates = []model.PortfolioCertificate{}
	}
	f.portfolios[stored.StudentID] = stored
	return clonePortfolio(stored), nil
}

// AddProject meniru FindOneAndUpdate dengan filter $expr (jumlah proyek
// masih di bawah MaxPortfolioProjects) plus $push.
func (f *fakePortfolioRepository) AddProject(_ context.Context, studentID int, project model.PortfolioProject) (model.StudentPortfolio, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	portfolio, ok := f.portfolios[studentID]
	if !ok {
		return model.StudentPortfolio{}, repository.ErrPortfolioNotFound
	}
	if len(portfolio.Projects) >= repository.MaxPortfolioProjects {
		return model.StudentPortfolio{}, repository.ErrPortfolioProjectLimit
	}
	if project.ID == "" {
		project.ID = f.newID()
	}
	portfolio.Projects = append(portfolio.Projects, project)
	f.portfolios[studentID] = portfolio
	return clonePortfolio(portfolio), nil
}

// DeleteProject meniru UpdateOne dengan $pull.
func (f *fakePortfolioRepository) DeleteProject(_ context.Context, studentID int, projectID string) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	portfolio, ok := f.portfolios[studentID]
	if !ok {
		return repository.ErrPortfolioNotFound
	}
	for i := range portfolio.Projects {
		if portfolio.Projects[i].ID == projectID {
			portfolio.Projects = append(portfolio.Projects[:i], portfolio.Projects[i+1:]...)
			f.portfolios[studentID] = portfolio
			return nil
		}
	}
	return repository.ErrPortfolioNotFound
}

// TopTechnologies meniru aggregation $unwind + $group + $sort + $limit.
func (f *fakePortfolioRepository) TopTechnologies(_ context.Context) ([]model.TechnologyStat, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	counts := map[string]int{}
	for _, portfolio := range f.portfolios {
		for _, project := range portfolio.Projects {
			for _, tech := range project.Technologies {
				counts[tech]++
			}
		}
	}
	stats := make([]model.TechnologyStat, 0, len(counts))
	for tech, count := range counts {
		stats = append(stats, model.TechnologyStat{Technology: tech, UsageCount: count})
	}
	sort.Slice(stats, func(i, j int) bool {
		if stats[i].UsageCount != stats[j].UsageCount {
			return stats[i].UsageCount > stats[j].UsageCount
		}
		return stats[i].Technology < stats[j].Technology
	})
	if len(stats) > 10 {
		stats = stats[:10]
	}
	return stats, nil
}

// EnsureIndexes tidak melakukan apa-apa pada fake: index hanya relevan
// untuk MongoDB. Method ini tetap diimplementasi agar fake memenuhi
// interface tanpa harus panik saat dipanggil.
func (f *fakePortfolioRepository) EnsureIndexes(context.Context) error {
	return nil
}

// clonePortfolio menyalin portofolio supaya data fake tidak bisa diubah
// lewat slice yang dikembalikan ke pemanggil. Tanpa ini, test bisa
// diam-diam mengubah isi fake dan membuat test lain gagal.
func clonePortfolio(p model.StudentPortfolio) model.StudentPortfolio {
	clone := p
	clone.Projects = make([]model.PortfolioProject, 0, len(p.Projects))
	for _, project := range p.Projects {
		project.Technologies = append([]string{}, project.Technologies...)
		clone.Projects = append(clone.Projects, project)
	}
	clone.Certificates = append([]model.PortfolioCertificate{}, p.Certificates...)
	return clone
}

// fakeStudentRepository menyimpan data student di memory, menggantikan
// PostgreSQL. Yang dipakai service portofolio hanya FindByID.
type fakeStudentRepository struct {
	repository.StudentRepository // method lain: panic bila terpanggil

	mu       sync.Mutex
	students map[int]model.Student
}

// newFakeStudentRepository membuat instance fake baru (dipakai per test).
func newFakeStudentRepository(students ...model.Student) *fakeStudentRepository {
	repo := &fakeStudentRepository{students: map[int]model.Student{}}
	for _, s := range students {
		repo.students[s.ID] = s
	}
	return repo
}

// FindByID meniru SELECT ... FROM students WHERE id = $1.
func (f *fakeStudentRepository) FindByID(_ context.Context, id int) (model.Student, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	student, ok := f.students[id]
	if !ok {
		return model.Student{}, repository.ErrNotFound
	}
	return student, nil
}

// Test di bawah ini menguji KEANDALAN fake itu sendiri, bukan service.
// Tujuannya: fake harus meniru perilaku repository MongoDB dan PostgreSQL
// sedekat mungkin, sehingga component test pada Langkah 3 memverifikasi
// service, bukan keanehan fake.

// TestFakePortfolioRepository_MirrorMongoBehaviour memeriksa empat aturan
// yang ditegakkan repository produksi:
//  1. portofolio yang belum ada menghasilkan ErrPortfolioNotFound;
//  2. Replace memberi id pada proyek yang belum punya id;
//  3. AddProject menolak ketika jumlah proyek mencapai batas 20;
//  4. DeleteProject pada proyek yang tidak ada menghasilkan ErrPortfolioNotFound.
func TestFakePortfolioRepository_MirrorMongoBehaviour(t *testing.T) {
	ctx := context.Background()
	repo := newFakePortfolioRepository()

	if _, err := repo.FindByStudentID(ctx, 1); !errors.Is(err, repository.ErrPortfolioNotFound) {
		t.Fatalf("FindByStudentID pada student tak dikenal: error = %v, seharusnya ErrPortfolioNotFound", err)
	}

	stored, err := repo.Replace(ctx, model.StudentPortfolio{
		StudentID: 1,
		Projects:  []model.PortfolioProject{{Title: "Sistem Jual Beli", Technologies: []string{"Go"}}},
	})
	if err != nil {
		t.Fatalf("Replace: %v", err)
	}
	if stored.Projects[0].ID == "" {
		t.Error("Replace seharusnya memberi id pada proyek yang belum punya id")
	}

	for i := 0; i < repository.MaxPortfolioProjects-1; i++ {
		if _, err := repo.AddProject(ctx, 1, model.PortfolioProject{Title: "Proyek"}); err != nil {
			t.Fatalf("AddProject ke-%d: %v", i, err)
		}
	}
	if _, err := repo.AddProject(ctx, 1, model.PortfolioProject{Title: "Proyek"}); !errors.Is(err, repository.ErrPortfolioProjectLimit) {
		t.Fatalf("AddProject melewati batas: error = %v, seharusnya ErrPortfolioProjectLimit", err)
	}

	if err := repo.DeleteProject(ctx, 1, stored.Projects[0].ID); err != nil {
		t.Fatalf("DeleteProject: %v", err)
	}
	if err := repo.DeleteProject(ctx, 1, stored.Projects[0].ID); !errors.Is(err, repository.ErrPortfolioNotFound) {
		t.Fatalf("DeleteProject untuk id yang sudah dihapus: error = %v, seharusnya ErrPortfolioNotFound", err)
	}
}

// TestFakePortfolioRepository_TopTechnologies memastikan agregasi fake
// menghitung pemakaian teknologi dengan benar dan mengurutkannya.
func TestFakePortfolioRepository_TopTechnologies(t *testing.T) {
	ctx := context.Background()
	repo := newFakePortfolioRepository()

	if _, err := repo.Replace(ctx, model.StudentPortfolio{
		StudentID: 1,
		Projects: []model.PortfolioProject{
			{Title: "A", Technologies: []string{"Go", "PostgreSQL"}},
			{Title: "B", Technologies: []string{"Go"}},
		},
	}); err != nil {
		t.Fatalf("Replace: %v", err)
	}

	stats, err := repo.TopTechnologies(ctx)
	if err != nil {
		t.Fatalf("TopTechnologies: %v", err)
	}
	if len(stats) != 2 {
		t.Fatalf("jumlah statistik = %d, seharusnya 2 (%v)", len(stats), stats)
	}
	if stats[0].Technology != "Go" || stats[0].UsageCount != 2 {
		t.Errorf("statistik teratas = %+v, seharusnya Go dengan 2 pemakaian", stats[0])
	}
	if stats[1].Technology != "PostgreSQL" || stats[1].UsageCount != 1 {
		t.Errorf("statistik kedua = %+v, seharusnya PostgreSQL dengan 1 pemakaian", stats[1])
	}
}

// TestFakeStudentRepository_NotFound memastikan student yang tidak ada
// menghasilkan ErrNotFound — sentinel yang dikenali service.
func TestFakeStudentRepository_NotFound(t *testing.T) {
	repo := newFakeStudentRepository(model.Student{ID: 1, Name: "sari"})

	if _, err := repo.FindByID(context.Background(), 1); err != nil {
		t.Fatalf("FindByID student yang ada: %v", err)
	}
	if _, err := repo.FindByID(context.Background(), 99); !errors.Is(err, repository.ErrNotFound) {
		t.Fatalf("FindByID student tak dikenal: error = %v, seharusnya ErrNotFound", err)
	}
}

// FindAllAfterCursor meniru keyset pagination pada collection
// student_portfolios: urutan (updated_at, student_id) DESC, filter pencarian
// pada headline/bio, dan filter keyset (updated_at, student_id) < cursor.
//
// Like repository produksi, hasilnya dipotong pada limit+1 supaya service
// bisa menentukan "masih ada halaman berikutnya?".
func (f *fakePortfolioRepository) FindAllAfterCursor(
	_ context.Context, q model.CursorQuery,
) ([]model.StudentPortfolio, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	rows := make([]model.StudentPortfolio, 0, len(f.portfolios))
	for _, portfolio := range f.portfolios {
		if q.Search != "" && !portfolioContains(portfolio, q.Search) {
			continue
		}
		if q.After != nil && !portfolioBefore(portfolio, *q.After) {
			continue
		}
		rows = append(rows, clonePortfolio(portfolio))
	}

	sort.Slice(rows, func(i, j int) bool {
		if !rows[i].UpdatedAt.Equal(rows[j].UpdatedAt) {
			return rows[i].UpdatedAt.After(rows[j].UpdatedAt)
		}
		return rows[i].StudentID > rows[j].StudentID
	})

	if len(rows) > q.Limit {
		rows = rows[:q.Limit+1]
	}
	return rows, nil
}

// portfolioContains meniru pencarian case-insensitive pada headline dan bio.
func portfolioContains(portfolio model.StudentPortfolio, search string) bool {
	needle := strings.ToLower(search)
	return strings.Contains(strings.ToLower(portfolio.Headline), needle) ||
		strings.Contains(strings.ToLower(portfolio.Bio), needle)
}

// portfolioBefore meniru filter keyset: baris dianggap "sebelum cursor"
// bila updated_at lebih kecil, atau sama-sama dan student_id lebih kecil.
func portfolioBefore(portfolio model.StudentPortfolio, after model.Cursor) bool {
	if portfolio.UpdatedAt.Before(after.CreatedAt) {
		return true
	}
	return portfolio.UpdatedAt.Equal(after.CreatedAt) && portfolio.StudentID < after.ID
}
