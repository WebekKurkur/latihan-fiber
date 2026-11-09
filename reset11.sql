-- reset11.sql — mengembalikan keadaan akun uji Modul 11
--   psql -U postgres -d praktikum_backend -f reset11.sql
--
-- HANYA menyentuh dua akun uji: mhsuji dan salahalamat. Data mahasiswa
-- lain, kelas, dan seluruh tabel tetap utuh.
--
-- CATATAN: sebelum mengulang uji, kosongkan juga queue Redis
-- (go run ./tools/flushqueue) dan kosongkan logs/mail.log. Task lama yang
-- masih tertinggal di Redis akan dikerjakan worker lagi dan membuat
-- notifikasi baru yang tidak terhitung dalam harapan modul.

-- 1. Riwayat notifikasi dan event milik akun uji.
DELETE FROM notifications
WHERE student_id IN (
    SELECT id FROM students WHERE name IN ('mhsuji', 'salahalamat')
);

-- Guard: hanya event KRS yang punya student_id numerik. Event lain
-- (misalnya event dengan bentuk payload berbeda) tidak boleh ikut terhapus.
DELETE FROM outbox
WHERE (payload->>'student_id') ~ '^[0-9]+$'
  AND (payload->>'student_id')::INT IN (
      SELECT id FROM students WHERE name IN ('mhsuji', 'salahalamat')
  );

-- 2. KRS milik akun uji.
DELETE FROM enrollments
WHERE student_id IN (
    SELECT id FROM students WHERE name IN ('mhsuji', 'salahalamat')
);

-- 3. Counter peserta dihitung ulang dari baris enrollments yang sebenarnya.
UPDATE courses c
SET enrolled_count = (SELECT COUNT(*) FROM enrollments e WHERE e.course_id = c.id);
