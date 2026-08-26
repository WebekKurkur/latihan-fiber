# API Students — Tugas 2

REST API sederhana untuk mengelola data mahasiswa, dibuat dengan Go + Fiber sebagai tugas Praktikum Backend Lanjut Pertemuan 2.

## Menjalankan

```bash
go run .
```

Server berjalan di `http://localhost:3000`.

## Struktur Berkas

| File | Isi |
| --- | --- |
| `main.go` | Setup aplikasi, middleware, dan route |
| `model.go` | Struct `Student` dan request/response |
| `helper.go` | Amplop respons, validasi query string |
| `handler.go` | Handler tiap endpoint |

## Kontrak API

Base URL: `http://localhost:3000/api/v1`

### Daftar Endpoint

| Metode | Endpoint | Parameter | Body | Status Berhasil | Keterangan |
| --- | --- | --- | --- | --- | --- |
| GET | `/students` | `page`, `limit`, `search`, `sort`, `order`, `is_active` | — | 200 | Daftar mahasiswa dengan paginasi, pencarian, filter, dan sort |
| GET | `/students/:id` | — | — | 200 | Ambil satu mahasiswa |
| POST | `/students` | — | `name`, `nim`, `grade` | 201 | Buat mahasiswa baru, mengembalikan header `Location` |
| PUT | `/students/:id` | — | `name`, `nim`, `grade`, `is_active` | 200 | Ganti seluruh isi; semua field wajib |
| PATCH | `/students/:id` | — | sebagian dari `name`, `nim`, `grade`, `is_active` | 200 | Ubah sebagian; hanya field yang dikirim |
| DELETE | `/students/:id` | — | — | 204 | Hapus, tanpa body |

### Query String (hanya untuk GET `/students`)

| Parameter | Tipe | Default | Keterangan |
| --- | --- | --- | --- |
| `page` | int | 1 | Halaman ke-N |
| `limit` | int | 10 | Jumlah per halaman, maks 100 |
| `search` | string | — | Pencarian di field `name` dan `nim`, case-insensitive |
| `sort` | string | `id` | Field sortir: `id`, `name`, `nim`, `grade`, `created_at` |
| `order` | string | `asc` | `asc` atau `desc` |
| `is_active` | bool | — | Filter status aktif |

Batas atas `limit` = **100**, alasannya: mencegah client mengirim `limit=9999999` yang bisa membuat server kehabisan memori atau melambat saat data membengkas.

### Status HTTP yang Digunakan

| Status | Situasi |
| --- | --- |
| 200 | Pengambilan atau perubahan berhasil |
| 201 | Penambahan berhasil, disertai header `Location` |
| 204 | Penghapusan berhasil |
| 400 | Body bukan JSON yang sah, atau `id` bukan angka |
| 404 | Mahasiswa dengan ID tersebut tidak ditemukan |
| 409 | NIM sudah terdaftar (konflik dengan data yang ada) |
| 415 | `Content-Type` bukan `application/json` |
| 422 | Validasi isi gagal (mis. `name` kosong, `nim` kurang dari 8 digit) |

## Contoh Permintaan dan Respons

### 1. POST — Buat Mahasiswa

**Permintaan:**
```http
POST /api/v1/students
Content-Type: application/json

{
  "name": "Sari Rahmadhani",
  "nim": "2024100101",
  "grade": 3.75
}
```

**Respons (201):**
```json
{
  "success": true,
  "message": "student berhasil dibuat",
  "data": {
    "id": 1,
    "name": "Sari Rahmadhani",
    "nim": "2024100101",
    "grade": 3.75,
    "is_active": true,
    "created_at": "2026-08-26T09:30:00Z"
  }
}
```
Header respons berisi `Location: /api/v1/students/1`.

### 2. GET — Daftar Mahasiswa

**Permintaan:**
```http
GET /api/v1/students?page=1&limit=5&sort=name&order=asc&is_active=true
```

**Respons (200):**
```json
{
  "success": true,
  "message": "daftar student berhasil diambil",
  "data": [ ... ],
  "meta": {
    "page": 1,
    "limit": 5,
    "total": 12,
    "total_pages": 3
  }
}
```

### 3. GET — Satu Mahasiswa

**Permintaan:**
```http
GET /api/v1/students/1
```

**Respons (200):**
```json
{
  "success": true,
  "message": "student ditemukan",
  "data": {
    "id": 1,
    "name": "Sari Rahmadhani",
    "nim": "2024100101",
    "grade": 3.75,
    "is_active": true,
    "created_at": "2026-08-26T09:30:00Z"
  }
}
```

### 4. PUT — Ganti Seluruh Isi

**Permintaan:**
```http
PUT /api/v1/students/1
Content-Type: application/json

{
  "name": "Sari R.",
  "nim": "2024100101",
  "grade": 3.80,
  "is_active": false
}
```

**Respons (200):**
```json
{
  "success": true,
  "message": "student berhasil diganti seluruhnya",
  "data": { ... }
}
```

### 5. PATCH — Ubah Sebagian

**Permintaan:**
```http
PATCH /api/v1/students/1
Content-Type: application/json

{
  "is_active": true
}
```

**Respons (200):**
```json
{
  "success": true,
  "message": "student berhasil diperbarui sebagian",
  "data": { ... }
}
```

### 6. DELETE — Hapus

**Permintaan:**
```http
DELETE /api/v1/students/1
```

**Respons (204):** tanpa body.

## Contoh Respons Gagal

### 422 — Validasi Gagal

```json
{
  "success": false,
  "message": "validasi gagal",
  "errors": {
    "name": "wajib diisi",
    "nim": "minimal 8 digit angka"
  }
}
```

### 409 — NIM Sudah Terdaftar

```json
{
  "success": false,
  "message": "NIM sudah terdaftar"
}
```

### 404 — Tidak Ditemukan

```json
{
  "success": false,
  "message": "student tidak ditemukan"
}
```

### 415 — Content-Type Salah

```json
{
  "success": false,
  "message": "Content-Type harus application/json"
}
```

## Middleware

| Middleware | Berlaku Untuk | Fungsi |
| --- | --- | --- |
| `requestid` | Global | Memberi setiap request ID unik, muncul di header `X-Request-Id` dan log |
| `logger` | Global | Mencatat method, path, status, dan durasi |
| `cors` | Global | Mengizinkan request dari origin lain |
| `requireJSON` | Grup `/students` | Menolak request POST/PUT/PATCH yang bukan `application/json` (status 415) |

## Penjelasan Tambahan (Requirement 3 & 4)

### Requirement 3 — Query String

Tujuan query string adalah memberi cara bagi klien untuk mengubah cara kumpulan data ditampilkan tanpa mengubah endpoint.

| Parameter | Tujuan | Default | Aturan Aman |
| --- | --- | --- | --- |
| `page` | Paginasi ke halaman ke-berapa | `1` | Klien bisa kirim `page=0` atau negatif → server paksa balik ke `1` |
| `limit` | Batas baris per halaman | `10` | Maks `100` (lihat catatan di bawah) |
| `search` | Pencarian substring pada `name` dan `nim` | kosong | Case-insensitive |
| `sort` | Pilih field urutan | `id` | Hanya field whitelist: `id`, `name`, `nim`, `grade`, `created_at`. Field lain diabaikan |
| `order` | Arah urutan | `asc` | Hanya `asc` atau `desc`. Selain itu dipaksa jadi `asc` |
| `is_active` | Filter status aktif | tidak menyaring | `true` / `false` |

**Kenapa `limit` dibatasi 100?**

Tanpa batas, klien nakal bisa kirim `?limit=99999999` dan membuat server meng-copy seluruh slice ke memori lalu loop pagination dalam satu kali request. Pembatasan ini pengaman standar, bukan aturan mati.

**Kenapa `sort` perlu whitelist?**

Nama field dari klien dipakai langsung untuk memilih kolom pengurutan. Kalau nama itu ditempel mentah ke query SQL, itu jalur masuknya SQL injection. Hari ini data masih di memori sehingga belum berbahaya, tetapi polanya dibentuk sekarang agar kebiasaan benar sudah ada saat basis data dipasang di Pertemuan 3.

### Requirement 4 — Status HTTP

Server wajib menjawab dengan status yang mencerminkan kondisi sebenarnya, bukan menyalahkan server untuk kesalahan klien.

| Status | Nama | Situasi di API ini | Contoh Konkret |
| --- | --- | --- | --- |
| 200 | OK | Pengambilan atau perubahan berhasil | `GET /students`, `PUT /students/1` |
| 201 | Created | Penambahan berhasil, ada header `Location` | `POST /students` baru |
| 204 | No Content | Penghapusan berhasil, body kosong | `DELETE /students/1` |
| 400 | Bad Request | Permintaan tidak bisa diproses | `PUT /students/abc` (id bukan angka) atau body bukan JSON |
| 404 | Not Found | Data dengan ID tersebut tidak ada | `GET /students/999` |
| 409 | Conflict | Bentrok dengan data yang sudah ada | `POST` dengan `nim` yang sudah terdaftar |
| 415 | Unsupported Media Type | `Content-Type` bukan `application/json` | `POST` tanpa header Content-Type |
| 422 | Unprocessable Entity | Validasi isi gagal, rincian per-field | `POST` dengan `name` kosong atau `nim` kurang dari 8 digit |

**Pembeda 400 vs 422:**

| Aspek | 400 | 422 |
| --- | --- | --- |
| Permintaan dipahami server? | Tidak | Ya |
| Contoh | JSON rusak | Format field benar tapi isinya tidak lolos validasi |
| Bentuk respons | `{success:false, message}` | `{success:false, message, errors: {field:alasan}}` |

**Pembeda 422 vs 409:**

| Aspek | 422 | 409 |
| --- | --- | --- |
| Permintaan valid menurut format? | Tidak | Ya |
| Yang bermasalah | Field tidak lolos validasi | Bentrok dengan state data yang sudah ada |
| Contoh | `nim` = `"abc"` (kurang dari 8 digit) | `nim` = `"210101001"` tapi sudah dipakai mahasiswa lain |

## Pengujian

Koleksi Postman siap pakai tersedia di `postman_collection.json` dengan total **19 request**. Cara import: buka Postman → klik **Import** → tarik file `postman_collection.json` ke dialog, atau klik tab **Raw text** lalu paste isinya.

### Daftar 19 Request

| # | Nama Request | Metode | Endpoint | Yang Diuji |
| --- | --- | --- | --- | --- |
| 1 | Buat student (johndoe) | POST | `/students` | Data valid → 201 + `Location` |
| 2 | Buat student kedua (anita) | POST | `/students` | Data valid → 201 |
| 3 | Buat student ketiga (budi) | POST | `/students` | Data valid → 201 |
| 4 | Ambil semua student | GET | `/students` | Daftar awal → 200 |
| 5 | Ambil student by ID | GET | `/students/1` | ID ada → 200 |
| 6 | Ambil student ID tidak ada | GET | `/students/999` | ID tidak ada → 404 |
| 7 | Pagination page 1 limit 2 | GET | `/students?page=1&limit=2` | `meta.total` + `total_pages` |
| 8 | Pagination page 2 limit 2 | GET | `/students?page=2&limit=2` | Pindah halaman |
| 9 | Sort by name asc | GET | `/students?sort=name&order=asc` | Whitelist sort |
| 10 | Sort by grade desc | GET | `/students?sort=grade&order=desc` | Whitelist sort + desc |
| 11 | Search "an" | GET | `/students?search=an` | Case-insensitive substring |
| 12 | Filter is_active true | GET | `/students?is_active=true` | Filter aktif/non-aktif |
| 13 | Update seluruh (PUT) | PUT | `/students/1` | Ganti seluruh field → 200 |
| 14 | PUT tanpa is_active | PUT | `/students/1` | Field hilang → 422 |
| 15 | Update sebagian (PATCH) | PATCH | `/students/1` | Ubah `grade` saja → 200 |
| 16 | Hapus student | DELETE | `/students/1` | Hapus → 204 tanpa body |
| 17 | NIM duplikat | POST | `/students` | Bentrok dengan data → 409 |
| 18 | Body bukan JSON | POST | `/students` | Body rusak → 400 |
| 19 | Tanpa Content-Type | POST | `/students` | Content-Type salah → 415 |

### Status yang Terbukti oleh Koleksi Ini

| Status | Nomor Request |
| --- | --- |
| 200 | 4, 5, 9, 10, 11, 12, 13, 15 |
| 201 | 1, 2, 3 |
| 204 | 16 |
| 400 | 18 |
| 404 | 6 |
| 409 | 17 |
| 415 | 19 |
| 422 | 14 |

Semua 8 status dari Requirement 4 tercakup dalam 19 request ini, dengan minimal dua contoh untuk status yang paling sering muncul (200 dan 201).

## Skenario Konkret PUT vs PATCH (Requirement 2)

Bukti perbedaan nyata PUT dan PATCH pada data yang sama:

**Data awal mahasiswa ID 1:**

```json
{ "id": 1, "name": "Sari Rahmadhani", "nim": "210101001", "grade": 85.5, "is_active": true }
```

**PUT** `/students/1` dengan body:

```json
{ "name": "Sari R.", "nim": "210101001", "grade": 90.0, "is_active": true }
```

Setelah PUT → seluruh field terganti. Field yang tidak dikirim dianggap dikosongkan. Coba PUT tanpa `is_active` → server menjawab 422.

**PATCH** `/students/1` dengan body:

```json
{ "grade": 88.0 }
```

Setelah PATCH → hanya `grade` yang berubah. `name`, `nim`, dan `is_active` tetap. Inilah alasan PATCH menggunakan tipe pointer di struct Go: `nil` artinya "tidak dikirim", bukan "bernilai kosong".

## Catatan

- Data disimpan di memori (`var students []Student`), akan hilang ketika server dimatikan. Penyimpanan permanen dengan basis data akan dibahas di pertemuan 3.
- Field `ID` di-generate otomatis oleh server, client tidak boleh mengirimnya.
- `nextID` di-reset ke `1` setiap kali server dimulai ulang, sehingga mahasiswa baru akan mendapat `id=1` lagi — bukan cacat, hanya sifat data yang belum persisten.