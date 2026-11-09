-- Migration 007: 60 akun mahasiswa untuk uji beban (Modul 9).
-- HANYA untuk database praktikum. Jangan dijalankan di server sungguhan.
--
-- Kolom password diisi '!' — bukan hash bcrypt yang sah — sehingga akun
-- ini TIDAK MUNGKIN dipakai login. Alat uji beban membuat token sendiri.
--
-- Catatan adaptasi: modul memakai tabel users(username, email). Repo ini
-- memakai tabel students("name", "NIM"), jadi identitas uji dipetakan ke
-- situ. NIM diberi awalan LOAD agar mudah dikenali sebagai data uji.
--   psql -U postgres -d praktikum_backend -f migrations/007_seed_loadtest.sql
INSERT INTO students ("name", "NIM", "Grade", password, role)
SELECT 'mhs' || LPAD(g::TEXT, 3, '0'),
       'LOAD' || LPAD(g::TEXT, 3, '0'),
       3.0,
       '!',
       'user'
FROM generate_series(1, 60) AS g
ON CONFLICT DO NOTHING;
