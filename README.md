# latihan-fiber - database dan repositories

API sederhana untuk mengelola data mahasiswa menggunakan Go + Fiber dan PostgreSQL.
Dokumen ini berisi cara menyiapkan basis data dari nol dan daftar variabel
environment yang diperlukan agar aplikasi dapat dijalankan.

---

## Cara Menyiapkan Basis Data dari Nol

Bagian ini mengasumsikan PostgreSQL sudah terpasang di mesin pembaca (atau
berjalan di Docker / layanan lain). Nama database yang dipakai pada contoh di
bawah adalah `praktikum_backend` — namanya bisa diganti, asal konsisten dengan
isi variabel `DB_NAME` pada `.env`.

### 1. Buat database

Masuk sebagai user PostgreSQL yang punya hak membuat database (contoh: `postgres`),
lalu jalankan salah satu perintah berikut sesuai situasi.

Menggunakan `createdb`:

```bash
createdb -U postgres praktikum_backend
```

Atau menggunakan `psql`:

```bash
psql -U postgres -c "CREATE DATABASE praktikum_backend;"
```

Verifikasi database sudah ada:

```bash
psql -U postgres -l
```

### 2. Jalankan migrasi

Skrip migrasi ada di `migrations/001_create_students.sql`. Skrip ini akan:

- Membuat tabel `students`.
- Membuat indeks unik `student_name_lower_key` pada `LOWER("name")` agar
  pencarian dan pengecekan keunikan nama tidak membedakan huruf besar/kecil.

Jalankan migrasi terhadap database yang baru dibuat:

```bash
psql -U postgres -d praktikum_backend -f migrations/001_create_students.sql
```

Perintah di atas dapat dijalankan berulang kali tanpa efek samping karena
`CREATE TABLE` dan `CREATE INDEX` menggunakan `IF NOT EXISTS`.

### 3. Verifikasi tabel

Pastikan tabel dan indeks sudah terbentuk:

```bash
psql -U postgres -d praktikum_backend -c "\d students"
```

Yang diharapkan muncul:

- Kolom-kolom: `id`, `name`, `NIM`, `Grade`, `is_active`, `created_at`.
- Index `student_name_lower_key` di bawah bagian "Indexes".

### 4. (Opsional) Cek data kosong

```bash
psql -U postgres -d praktikum_backend -c "SELECT COUNT(*) FROM students;"
```

Hasil yang diharapkan: `0` (tabel masih kosong).

---

## Daftar Variabel Environment

Salin file `.env.example` menjadi `.env`, lalu isi nilainya:

```bash
cp .env.example .env
```

Aplikasi membaca variabel-variabel berikut lewat pustaka `godotenv`. Jika
variabel tidak diisi, aplikasi menggunakan nilai bawaan yang tertera pada
kolom **Default** di bawah.

| Variabel       | Default              | Keterangan                                                                                  |
|----------------|----------------------|---------------------------------------------------------------------------------------------|
| `APP_PORT`     | `3000`               | Port HTTP yang dipakai server Fiber untuk listen.                                           |
| `DB_HOST`      | `localhost`          | Host PostgreSQL.                                                                            |
| `DB_PORT`      | `5432`               | Port PostgreSQL.                                                                            |
| `DB_USER`      | `postgres`           | User PostgreSQL yang dipakai untuk koneksi.                                                 |
| `DB_PASSWORD`  | `""` (kosong)        | Kata sandi user PostgreSQL. Wajib diisi jika server PostgreSQL mengaktifkan autentikasi.    |
| `DB_NAME`      | `praktikum_backend`  | Nama database yang dipakai aplikasi (lihat langkah 1 pada bagian "Cara Menyiapkan Basis Data"). |
| `DB_SSLMODE`   | `disable`            | Mode SSL untuk koneksi. Gunakan `disable` untuk pengembangan lokal, `require` untuk produksi.|
| `DB_MAX_CONNS` | `10`                 | Batas atas jumlah koneksi di pool pgxpool.                                                  |

Berkas `.env` tidak boleh di-commit ke repositori — pastikan sudah masuk
`.gitignore`.
