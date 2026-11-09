// Alat uji beban Modul 9: membuktikan ada atau tidaknya race condition
// pada endpoint KRS. Folder tools/ bukan bagian aplikasi — ia alat bukti,
// bukan fitur — dan tidak ikut di-deploy.
//
// Server harus sedang berjalan di terminal lain:
//
//	go run ./tools/loadtest -mode crowd    # 60 mahasiswa berebut IF399 (kuota 5)
//	go run ./tools/loadtest -mode credits  # 10 mahasiswa x 12 kelas, batas 24 SKS
//	go run ./tools/loadtest -mode swap     # 20 pasang pindah kelas berlawanan arah
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"latihan-fiber/app/model"
	"latihan-fiber/app/service"
	"latihan-fiber/config"
	"latihan-fiber/database"
	"latihan-fiber/helper"
)

const (
	minSecretLength = 32

	crowdAttackers  = 60
	creditAttackers = 10
	swapPairs       = 20
)

// creditCourseCodes: 12 kelas 3 SKS — total 36 SKS bila semuanya lolos,
// jauh melewati batas 24 SKS. Setiap mahasiswa hanya boleh berhasil 8.
var creditCourseCodes = []string{
	"IF101", "IF102",
	"IF301", "IF302", "IF303", "IF304", "IF305", "IF306",
	"IF307", "IF308", "IF309", "IF310",
}

// attacker adalah satu akun mahasiswa uji beserta access token yang
// dicetak alat ini sendiri.
type attacker struct {
	id    int
	name  string
	token string
}

func main() {
	base := flag.String("base", "http://127.0.0.1:3000", "alamat dasar server")
	mode := flag.String("mode", "crowd", "mode uji: crowd | credits | swap")
	flag.Parse()

	config.LoadEnv()

	secret := config.GetEnv("JWT_SECRET", "")
	if len(secret) < minSecretLength {
		fatal("JWT_SECRET tidak diisi atau terlalu pendek (minimal %d karakter)", minSecretLength)
	}
	jwtManager := helper.NewJWTManager(
		secret,
		config.GetEnv("JWT_ISSUER", "praktikum-backend"),
		time.Duration(config.GetEnvInt("JWT_ACCESS_TTL_MINUTES", 15))*time.Minute,
	)

	pool, err := database.NewPool(context.Background())
	if err != nil {
		fatal("gagal terhubung ke database: %v", err)
	}
	defer pool.Close()

	ctx := context.Background()
	attackers, err := loadAttackers(ctx, pool, jwtManager)
	if err != nil {
		fatal("gagal memuat akun uji: %v\n(jalankan migrations/007_seed_loadtest.sql lebih dulu)", err)
	}
	courseIDs, err := loadCourseIDs(ctx, pool)
	if err != nil {
		fatal("gagal memuat kelas: %v\n(jalankan migrations/006_courses.sql lebih dulu)", err)
	}

	client := &http.Client{Timeout: 60 * time.Second}

	switch *mode {
	case "crowd":
		requireAttackers(attackers, crowdAttackers)
		runCrowd(client, *base, ctx, attackers[:crowdAttackers], courseIDs["IF399"], pool)
	case "credits":
		requireAttackers(attackers, creditAttackers)
		runCredits(client, *base, ctx, attackers[:creditAttackers], courseIDs, pool)
	case "swap":
		requireAttackers(attackers, 2*swapPairs)
		runSwap(client, *base, ctx, attackers[:2*swapPairs], courseIDs, pool)
	default:
		fatal("mode tidak dikenal: %s (pilihan: crowd | credits | swap)", *mode)
	}
}

func requireAttackers(attackers []attacker, need int) {
	if len(attackers) < need {
		fatal("butuh %d akun uji, hanya ada %d (jalankan migrations/007_seed_loadtest.sql)", need, len(attackers))
	}
}

// ---------- mode crowd ----------

func runCrowd(client *http.Client, base string, ctx context.Context, attackers []attacker, target int, pool *pgxpool.Pool) {
	fmt.Printf("crowd: %d mahasiswa berebut kelas IF399 (kuota 5) secara serentak...\n\n", len(attackers))

	start := time.Now()
	tally := fire(len(attackers), func(i int) string {
		return enroll(client, base, attackers[i], target)
	})

	reportTally(tally)
	fmt.Printf("waktu total : %dms\n\n", time.Since(start).Milliseconds())
	checkCourse(ctx, pool, "IF399")
}

// ---------- mode credits ----------

func runCredits(client *http.Client, base string, ctx context.Context, attackers []attacker, courseIDs map[string]int, pool *pgxpool.Pool) {
	courses := make([]int, 0, len(creditCourseCodes))
	for _, code := range creditCourseCodes {
		id, ok := courseIDs[code]
		if !ok {
			fatal("kelas %s tidak ditemukan di database", code)
		}
		courses = append(courses, id)
	}

	fmt.Printf("credits: %d mahasiswa masing-masing mendaftar ke %d kelas sekaligus (batas %d SKS)...\n\n",
		len(attackers), len(courses), service.MaxCreditsPerSemester)

	start := time.Now()
	tally := fire(len(attackers)*len(courses), func(i int) string {
		return enroll(client, base, attackers[i%len(attackers)], courses[i/len(attackers)])
	})

	reportTally(tally)
	fmt.Printf("waktu total : %dms\n\n", time.Since(start).Milliseconds())
	checkCredits(ctx, pool, attackers)
}

// ---------- mode swap ----------

func runSwap(client *http.Client, base string, ctx context.Context, attackers []attacker, courseIDs map[string]int, pool *pgxpool.Pool) {
	from, okFrom := courseIDs["IF101"]
	to, okTo := courseIDs["IF102"]
	if !okFrom || !okTo {
		fatal("IF101/IF102 tidak ditemukan di database")
	}

	// Persiapan: separuh mahasiswa terdaftar di IF101, separuh di IF102.
	// Pendaftaran dijalankan bergiliran supaya persiapan sendiri tidak
	// ikut memicu rebutan.
	setup := map[string]int{}
	for i := 0; i < swapPairs; i++ {
		setup[enroll(client, base, attackers[i], from)]++
	}
	for i := swapPairs; i < len(attackers); i++ {
		setup[enroll(client, base, attackers[i], to)]++
	}
	fmt.Println("persiapan (pendaftaran bergiliran):")
	reportTally(setup)
	fmt.Printf("\nswap: %d pasang mahasiswa pindah kelas berlawanan arah (IF101 <-> IF102) secara serentak...\n\n", swapPairs)

	start := time.Now()
	tally := fire(len(attackers), func(i int) string {
		if i < swapPairs {
			return swap(client, base, attackers[i], from, to)
		}
		return swap(client, base, attackers[i], to, from)
	})

	reportTally(tally)
	fmt.Printf("waktu total : %dms\n\n", time.Since(start).Milliseconds())

	// Pindah kelas menjawab 200 (pendaftaran menjawab 201), jadi
	// "berhasil" dihitung dari seluruh status 2xx.
	success := 0
	serverFailures := 0
	for label, count := range tally {
		if strings.HasPrefix(label, "2") {
			success += count
		}
		if strings.HasPrefix(label, "5") {
			serverFailures += count
		}
	}
	fmt.Printf("berhasil=%d  gagal-sisi-server (deadlock dkk.)=%d\n\n", success, serverFailures)

	checkCourse(ctx, pool, "IF101")
	checkCourse(ctx, pool, "IF102")
}

// ---------- kirim request ----------

func postJSON(client *http.Client, url, token string, payload any) (*http.Response, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	return client.Do(req)
}

// enroll mengirim satu request pendaftaran kelas.
func enroll(client *http.Client, base string, a attacker, courseID int) string {
	resp, err := postJSON(client, fmt.Sprintf("%s/api/v1/courses/%d/enroll", base, courseID), a.token, struct{}{})
	if err != nil {
		return "NETWORK_ERROR"
	}
	defer resp.Body.Close()
	return statusLabel(resp)
}

// swap mengirim satu request pindah kelas.
func swap(client *http.Client, base string, a attacker, fromCourseID, toCourseID int) string {
	resp, err := postJSON(client, base+"/api/v1/enrollments/swap", a.token,
		model.SwapRequest{FromCourseID: fromCourseID, ToCourseID: toCourseID})
	if err != nil {
		return "NETWORK_ERROR"
	}
	defer resp.Body.Close()
	return statusLabel(resp)
}

// statusLabel mengubah response menjadi label untuk dihitung: "201" bila
// berhasil, "409 COURSE_FULL" bila ditolak dengan kode yang spesifik.
func statusLabel(resp *http.Response) string {
	raw, _ := io.ReadAll(resp.Body)
	var envelope struct {
		Code string `json:"code"`
	}
	_ = json.Unmarshal(raw, &envelope)
	if envelope.Code == "" {
		return strconv.Itoa(resp.StatusCode)
	}
	return fmt.Sprintf("%d %s", resp.StatusCode, envelope.Code)
}

// ---------- serentak ----------

// fire menjalankan n pekerjaan secara serentak dan menghitung hasilnya.
//
// Setiap goroutine menunggu di depan "gerbang" start yang sama. Gerbang
// baru dibuka setelah SELURUH goroutine siap, sehingga request benar-benar
// berangkat hampir bersamaan — bukan bergiliran sesuai urutan dibuatnya.
func fire(n int, job func(i int) string) map[string]int {
	var wg sync.WaitGroup
	start := make(chan struct{})
	results := make(chan string, n)

	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int, out chan<- string) {
			defer wg.Done()
			<-start
			out <- job(i)
		}(i, results)
	}

	close(start) // buka gerbang
	wg.Wait()    // tunggu SEMUA goroutine selesai
	close(results)

	tally := map[string]int{}
	for r := range results {
		tally[r]++
	}
	return tally
}

func reportTally(tally map[string]int) {
	labels := make([]string, 0, len(tally))
	for label := range tally {
		labels = append(labels, label)
	}
	sort.Strings(labels)
	for _, label := range labels {
		fmt.Printf("  %-30s %d\n", label, tally[label])
	}
}

// ---------- periksa keadaan database ----------

// checkCourse mencocokkan kolom enrolled_count dengan jumlah baris
// enrollments yang sebenarnya. Penghitung adalah turunan; baris
// enrollments adalah sumber kebenarannya.
func checkCourse(ctx context.Context, pool *pgxpool.Pool, code string) {
	var quota, enrolled, rows int
	err := pool.QueryRow(ctx, `
		SELECT c.quota, c.enrolled_count,
		       (SELECT COUNT(*) FROM enrollments e WHERE e.course_id = c.id)
		FROM courses c WHERE c.code = $1`, code).Scan(&quota, &enrolled, &rows)
	if err != nil {
		fmt.Printf("%s gagal diperiksa: %v\n", code, err)
		return
	}
	status := "TIDAK KONSISTEN"
	if enrolled == rows {
		status = "KONSISTEN"
	}
	fmt.Printf("%s  kuota=%d  enrolled_count=%d  baris enrollments=%d  -> %s\n",
		code, quota, enrolled, rows, status)
}

// checkCredits memastikan tidak ada mahasiswa yang melewati batas SKS.
func checkCredits(ctx context.Context, pool *pgxpool.Pool, attackers []attacker) {
	maxTaken := 0
	exceeded := 0
	for _, a := range attackers {
		var taken int
		err := pool.QueryRow(ctx, `
			SELECT COALESCE(SUM(c.credits), 0)
			FROM enrollments e JOIN courses c ON c.id = e.course_id
			WHERE e.student_id = $1`, a.id).Scan(&taken)
		if err != nil {
			fmt.Printf("%s gagal diperiksa: %v\n", a.name, err)
			exceeded++
			continue
		}
		if taken > maxTaken {
			maxTaken = taken
		}
		if taken > service.MaxCreditsPerSemester {
			exceeded++
		}
	}

	status := "TIDAK KONSISTEN"
	if exceeded == 0 {
		status = "KONSISTEN"
	}
	fmt.Printf("SKS terbanyak satu mahasiswa=%d (batas %d)\n", maxTaken, service.MaxCreditsPerSemester)
	fmt.Printf("seluruh %d mahasiswa <= %d SKS -> %s\n", len(attackers), service.MaxCreditsPerSemester, status)
}

// ---------- muat data uji dari database ----------

func loadAttackers(ctx context.Context, pool *pgxpool.Pool, jwtManager *helper.JWTManager) ([]attacker, error) {
	rows, err := pool.Query(ctx,
		`SELECT id, "name" FROM students WHERE "name" ~ '^mhs[0-9]{3}$' ORDER BY "name"`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var attackers []attacker
	for rows.Next() {
		var a attacker
		if err := rows.Scan(&a.id, &a.name); err != nil {
			return nil, err
		}
		// Alat ini mencetak token sendiri karena ia memegang JWT_SECRET.
		// Itulah sebabnya JWT_SECRET tidak boleh bocor: siapa pun yang
		// memegangnya dapat mencetak token atas nama siapa saja.
		a.token, err = jwtManager.GenerateAccess(model.Student{ID: a.id, Name: a.name, Role: "user"})
		if err != nil {
			return nil, err
		}
		attackers = append(attackers, a)
	}
	return attackers, rows.Err()
}

func loadCourseIDs(ctx context.Context, pool *pgxpool.Pool) (map[string]int, error) {
	rows, err := pool.Query(ctx, `SELECT id, code FROM courses`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	courseIDs := map[string]int{}
	for rows.Next() {
		var id int
		var code string
		if err := rows.Scan(&id, &code); err != nil {
			return nil, err
		}
		courseIDs[code] = id
	}
	return courseIDs, rows.Err()
}

func fatal(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "loadtest: "+format+"\n", args...)
	os.Exit(1)
}
