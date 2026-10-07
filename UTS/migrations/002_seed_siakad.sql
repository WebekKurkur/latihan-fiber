-- Jalankan sesudah 001. pgcrypto crypt(..., gen_salt('bf')) menghasilkan bcrypt, bukan plaintext.
CREATE EXTENSION IF NOT EXISTS pgcrypto;
INSERT INTO siakad_uts.users(email,password,role)
VALUES ('admin@siakad.test', crypt('Admin12345!',gen_salt('bf')), 'admin')
ON CONFLICT DO NOTHING;
WITH sample AS (
 SELECT n, '187221'||lpad(n::text,6,'0') AS nim, 'mahasiswa'||n||'@siakad.test' AS email
 FROM generate_series(1,20) AS n
)
INSERT INTO siakad_uts.users(email,password,role)
SELECT email,crypt(nim,gen_salt('bf')),'mahasiswa' FROM sample
WHERE NOT EXISTS (SELECT 1 FROM siakad_uts.users u WHERE lower(u.email)=lower(sample.email));
WITH sample AS (
 SELECT n, '187221'||lpad(n::text,6,'0') AS nim, 'mahasiswa'||n||'@siakad.test' AS email
 FROM generate_series(1,20) AS n
)
INSERT INTO siakad_uts.students(user_id,nim,nama,prodi,angkatan,ipk_terakhir)
SELECT u.id,s.nim,'Mahasiswa '||s.n,'Sistem Informasi',2022,CASE WHEN s.n%3=0 THEN 2.40 WHEN s.n%3=1 THEN 3.45 ELSE 2.75 END
FROM sample s JOIN siakad_uts.users u ON lower(u.email)=lower(s.email)
WHERE NOT EXISTS (SELECT 1 FROM siakad_uts.students t WHERE t.user_id=u.id OR t.nim=s.nim);
INSERT INTO siakad_uts.courses(kode_mk,nama_mk,sks,semester,kuota)
SELECT 'MK'||lpad(n::text,3,'0'),'Mata Kuliah '||n, CASE WHEN n%2=0 THEN 2 ELSE 3 END, ((n-1)%8)+1,30
FROM generate_series(1,10) n ON CONFLICT (kode_mk) DO NOTHING;
