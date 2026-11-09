-- Migration 008: kolom email pada tabel students (Modul 11 - Background Job)
--   psql -U postgres -d praktikum_backend -f migrations/008_students_email.sql
--
-- Modul 11 membutuhkan alamat email untuk setiap mahasiswa, karena notifikasi
-- KRS dikirim ke email. Pada repo ini tabel yang berperan sebagai "users"
-- adalah students, jadi email ditambahkan di sini.
--
-- Nullable dulu karena data lama belum punya email. Nilainya diisi dengan
-- alamat turunan pada langkah backfill di bawah ini.

ALTER TABLE students
    ADD COLUMN IF NOT EXISTS email VARCHAR(255);

-- Backfill: alamat turunan yang PASTI unik.
--
-- Nama mahasiswa dijamin unik oleh unique index LOWER("name"), sehingga
-- alamat turunan dari nama pun unik. Id ditambahkan sebagai pengaman,
-- supaya alamat turunan ini tidak pernah bentrok dengan alamat yang
-- diisi manual di kemudian hari.
UPDATE students
SET email = regexp_replace(lower("name"), '[^a-z0-9]+', '.', 'g')
             || '-' || id || '@student.unair.ac.id'
WHERE email IS NULL OR email = '';

-- Unik secara case-insensitive. Partial index dipakai supaya baris dengan
-- email kosong tidak ikut diindeks (ada student yang memang belum punya
-- email).
CREATE UNIQUE INDEX IF NOT EXISTS students_email_unique
    ON students (LOWER(email))
    WHERE email IS NOT NULL AND email <> '';
