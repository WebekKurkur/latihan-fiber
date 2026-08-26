# agents.md

Catatan penggunaan AI untuk pengerjaan tugas pada repo ini.

## Alat yang Dipakai

- GitHub Copilot (mode Agent) di VS Code

## Bagian yang Dibantu AI

- Pembuatan `postman_collection.json` — susunan 19 request untuk menguji setiap status HTTP dan perilaku endpoint
- Pembuatan `tugas2/README.md` — kontrak API, tabel status, dan penjelasan query string sesuai modul 2
- Penulisan middleware `requireJSON` (penolakan `Content-Type` non-JSON dengan status 415)

## Bagian yang Saya Kerjakan Sendiri

- Inisialisasi proyek, `go.mod`, dan setup `main()` dasar
- Susunan struct di `model.go` (`Student`, request untuk POST/PUT/PATCH, `WebResponse`, `Meta`, `ListQuery`) — mengikuti pola pada modul 2
- Helper di `helper.go` (`ok`, `okList`, `created`, `noContent`, `fail`, `failValidation`, `failConflict`, `parseListQuery`) — mengikuti langkah pada modul 2
- Logika handler di `handler.go` (pencarian, validasi NIM, sort, paginasi, alur 409 untuk NIM duplikat) — mengikuti pola pada modul 2
- Menjalankan server dan validasi tiap endpoint lewat Postman
- Penulisan laporan PDF

## Sumber Referensi

- Modul 2 Praktikum Backend Lanjut (REST API & HTTP Deep Dive) — Universitas Airlangga
- Dokumentasi Fiber v2 — docs.gofiber.io

## Verifikasi

Seluruh endpoint diuji lewat koleksi Postman dan dicocokkan dengan status yang diharapkan pada modul.
