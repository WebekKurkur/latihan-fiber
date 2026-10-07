# SIAKAD Mini — UTS PBE

## 1. Sistem yang dibuat

SIAKAD Mini adalah RESTful API backend untuk kebutuhan akademik sederhana. Sistem ini mengelola:

- akun pengguna dengan role `admin` dan `mahasiswa`;
- data mahasiswa;
- data mata kuliah;
- Kartu Rencana Studi (KRS) melalui enrollment.

Role `admin` mengelola data mahasiswa, sedangkan role `mahasiswa` dapat melihat profilnya sendiri, melihat mata kuliah, mengambil mata kuliah, dan membatalkan KRS miliknya.

Aplikasi UTS ini merupakan **Go module mandiri** di folder `latihan-fiber/UTS`. Jalankan perintah Go dari folder tersebut; aplikasi tidak bergantung pada runtime aplikasi parent `latihan-fiber`.

## 2. Stack

| Komponen | Teknologi |
|---|---|
| Bahasa | Go 1.26.5 |
| Web framework | Fiber v2 |
| Database | PostgreSQL |
| Driver database | pgx/v5 dan pgxpool |
| Authentication | JWT dengan signing method HS256 |
| Password hashing | bcrypt melalui `golang.org/x/crypto` |
| Request validation | `validator/v10` |
| Environment | `godotenv` dan environment variables |
| Logging | `log/slog` + `lumberjack` |
| Security middleware | Fiber Recover, Helmet, CORS, Request ID |
| API testing | Postman collection `json_collection_UTS.json` |

## 3. Struktur proyek

```text
UTS/
├── main.go
├── go.mod
├── go.sum
├── .env.example
├── .gitignore
├── README.md
├── UTSPraktikum.md
├── json_collection_UTS.json
├── config/
│   ├── app.go
│   ├── env.go
│   └── logger.go
├── database/
│   └── postgres.go
├── app/
│   ├── model/
│   │   ├── user.go
│   │   ├── student.go
│   │   ├── course.go
│   │   ├── enrollment.go
│   │   ├── request.go
│   │   └── response.go
│   ├── repository/
│   │   ├── user_repository.go
│   │   ├── student_repository.go
│   │   ├── course_repository.go
│   │   └── enrollment_repository.go
│   └── service/
│       ├── auth_service.go
│       ├── student_service.go
│       ├── course_service.go
│       ├── enrollment_service.go
│       └── *_test.go
├── helper/
│   ├── jwt.go
│   ├── password.go
│   ├── response.go
│   ├── validation.go
│   └── validation_test.go
├── middleware/
│   ├── auth.go
│   ├── limiter.go
│   └── middleware.go
├── route/
│   └── route.go
└── migrations/
    ├── 001_create_siakad.sql
    └── 002_seed_siakad.sql
```

**Notes folder:**

- `config/` — konfigurasi environment, logger, Fiber app, dan error handler terpusat.
- `database/` — pembuatan connection pool PostgreSQL.
- `app/model/` — entity domain, request DTO, response DTO, dan identity.
- `app/repository/` — interface repository dan implementasi query PostgreSQL.
- `app/service/` — use case dan aturan bisnis autentikasi, mahasiswa, mata kuliah, dan KRS.
- `helper/` — JWT, bcrypt, response JSON, error, dan validasi request.
- `middleware/` — autentikasi Bearer token, role guard, rate limiter, Request ID, CORS, Helmet, Recover, dan request logger.
- `route/` — registrasi seluruh route HTTP.
- `migrations/` — schema database dan data awal.

## 4. Tabel database

Seluruh tabel UTS berada di schema PostgreSQL `siakad_uts`.

| Tabel | Kolom utama | Relasi dan fungsi |
|---|---|---|
| `users` | `id`, `email`, `password`, `role` | Akun login. Role hanya `admin` atau `mahasiswa`. Email unik tanpa membedakan huruf besar/kecil. |
| `students` | `id`, `user_id`, `nim`, `nama`, `prodi`, `angkatan`, `ipk_terakhir`, `deleted_at` | Profil mahasiswa. Terhubung 1–1 dengan `users` dan 1–N dengan `enrollments`. `nim` unik. |
| `courses` | `id`, `kode_mk`, `nama_mk`, `sks`, `semester`, `kuota` | Data mata kuliah. `kode_mk` unik. |
| `enrollments` | `id`, `student_id`, `course_id`, `tahun_akademik`, `created_at` | Data KRS. Memiliki unique constraint pada `(student_id, course_id, tahun_akademik)`. |

Migration dan seeder menyediakan minimal 1 admin, 20 mahasiswa, dan 10 mata kuliah. Password seed disimpan sebagai bcrypt hash menggunakan PostgreSQL `pgcrypto`.

## 5. Cara menjalankan proyek

### Persiapan environment

Dari folder `latihan-fiber/UTS`:

```powershell
Copy-Item .env.example .env
```

Isi `.env`:

```env
APP_NAME=siakad-uts
APP_PORT=3000
DB_HOST=localhost
DB_PORT=5432
DB_USER=postgres
DB_PASSWORD=[]
DB_NAME=[]
DB_SSLMODE=disable
DB_MAX_CONNS=10
JWT_SECRET=[]
JWT_ACCESS_TTL_MINUTES=15
ALLOWED_ORIGINS=http://localhost:5173
LOG_LEVEL=info
```

`.env` dimuat otomatis oleh aplikasi dan di-ignore oleh Git.

### Database

Buat database jika belum tersedia:

```powershell
createdb -U postgres siakad_uts
```

Jalankan migration dan seeder:

```powershell
psql -U postgres -d siakad_uts -v ON_ERROR_STOP=1 -f migrations/001_create_siakad.sql
psql -U postgres -d siakad_uts -v ON_ERROR_STOP=1 -f migrations/002_seed_siakad.sql
```

### Menjalankan server

```powershell
go mod tidy
go run .
```

Server berjalan pada `http://localhost:3000` atau nilai `APP_PORT` yang dikonfigurasi.

Akun seed development:

- Admin: `admin@siakad.test` / `Admin12345!`
- Mahasiswa 1: `mahasiswa1@siakad.test` / `187221000001`
- Mahasiswa 2: `mahasiswa2@siakad.test` / `187221000002`
- Mahasiswa 1–20 mengikuti pola password berupa NIM.

### Verifikasi kode

```powershell
go test ./...
go vet ./...
go build .
```

## 6. Endpoint API

Prefix seluruh endpoint adalah `/api/v1`.

| Method | Path | Auth | Role | Status sukses | Status error utama |
|---|---|---|---|---:|---|
| `POST` | `/auth/login` | Tidak | Publik | `200` | `401`, `422`, `429` |
| `GET` | `/auth/me` | Bearer token | Admin/mahasiswa | `200` | `401` |
| `GET` | `/students` | Bearer token | Admin | `200` | `401`, `403`, `422` |
| `POST` | `/students` | Bearer token | Admin | `201` | `401`, `403`, `422` |
| `GET` | `/students/:id` | Bearer token | Admin/mahasiswa sendiri | `200` | `401`, `403`, `404` |
| `PUT` | `/students/:id` | Bearer token | Admin | `200` | `401`, `403`, `404`, `422` |
| `DELETE` | `/students/:id` | Bearer token | Admin | `204` | `401`, `403`, `404` |
| `GET` | `/courses` | Bearer token | Admin/mahasiswa | `200` | `401`, `422` |
| `POST` | `/enrollments` | Bearer token | Mahasiswa | `201` | `401`, `403`, `409`, `422` |
| `DELETE` | `/enrollments/:id` | Bearer token | Mahasiswa pemilik | `204` | `401`, `403`, `404` |

Detail query dan body:

- `GET /students`: `page`, `per_page` maksimal 50, `prodi`, `angkatan`, `search`, `sort=nama|-ipk_terakhir`.
- `POST /students`: `nim`, `nama`, `email`, `prodi`, `angkatan`, `ipk_terakhir`.
- `PUT /students/:id`: `nama`, `prodi`, `angkatan`, `ipk_terakhir`. NIM tidak dapat diubah.
- `GET /courses`: `semester`, `search`, `available=true`.
- `POST /enrollments`: `course_id` dan `tahun_akademik`, contoh `2026/2027-Ganjil`.

Koleksi pengujian Postman tersedia di `json_collection_UTS.json`. Jalankan request login terlebih dahulu agar token tersimpan pada collection variables.

## 7. Aturan bisnis

Aturan berikut diambil dari `UTSPraktikum.md`:

1. Batas SKS berdasarkan IPK terakhir:
   - IPK `>= 3,00`: maksimal `24 SKS`.
   - IPK `2,50–2,99`: maksimal `21 SKS`.
   - IPK `< 2,50`: maksimal `18 SKS`.
2. Mahasiswa tidak dapat mengambil mata kuliah yang sama dua kali pada tahun akademik yang sama.
3. Mata kuliah yang kuotanya penuh tidak dapat diambil.
4. Mahasiswa hanya dapat mengakses detail mahasiswa dan mengubah KRS miliknya sendiri.
5. Pembuatan mahasiswa membuat record `users` dan `students` dalam satu transaction.
6. Password awal mahasiswa adalah NIM dan disimpan dalam bentuk hash.
7. Pengambilan KRS memakai transaction dan row locking pada data mahasiswa serta mata kuliah.
8. Mahasiswa yang dihapus menggunakan soft delete, tidak muncul pada daftar, dan tidak dapat login.
9. Penghapusan enrollment milik sendiri mengembalikan sisa kuota mata kuliah.

## 8. Format response

### Response sukses

```json
{
  "success": true,
  "message": "Data mahasiswa berhasil diambil",
  "data": [],
  "meta": {
    "current_page": 1,
    "per_page": 10,
    "total": 20,
    "last_page": 2
  }
}
```

`meta` digunakan pada endpoint list mahasiswa. Endpoint lain mengembalikan `data` sesuai kebutuhannya.

### Response error

```json
{
  "success": false,
  "message": "Validasi gagal",
  "errors": {
    "nim": ["NIM harus 12 digit"],
    "email": ["Format email tidak valid"]
  },
  "request_id": "52b26ad5-fa3f-43b5-bcde-c43f52c6f671"
}
```

Semua error memakai JSON seragam. `request_id` membantu menghubungkan response client dengan log server. Error internal menggunakan pesan umum dan tidak mengirim stack trace.

Status code yang digunakan sistem mencakup `200`, `201`, `204`, `401`, `403`, `404`, `409`, `422`, `429`, dan `500`.

## 9. Keamanan

- Semua endpoint kecuali login memerlukan `Authorization: Bearer <token>`.
- JWT menggunakan HS256, issuer, expiry, dan secret minimal 32 karakter.
- Algoritma JWT divalidasi agar token dengan algoritma yang tidak diizinkan ditolak.
- Password dan password awal NIM disimpan menggunakan bcrypt hash.
- Login dibatasi maksimal 5 kegagalan per menit per alamat IP dan mengembalikan `429`.
- Akun mahasiswa soft-deleted tidak dapat login dan tokennya tidak lagi diterima pada endpoint terlindungi.
- Role guard memisahkan akses admin dan mahasiswa.
- Ownership dicek dari identity token, bukan dari nilai yang dikirim pada request body.
- Unique constraint mencegah duplikasi email, NIM, kode mata kuliah, dan enrollment.
- Query menggunakan parameter binding. Nilai sort dibatasi dengan whitelist.
- Transaction dan row locking mencegah race condition pada kuota dan batas SKS.
- CORS memakai allowlist dari `ALLOWED_ORIGINS`, bukan membuka semua origin secara default.
- Helmet menambahkan security headers dan Recover mencegah panic mematikan server.
- Request ID dan structured logging membantu audit tanpa mencatat password atau token.
- Log dirotasi menggunakan `lumberjack`; file `.env`, log, dan binary diabaikan Git.

