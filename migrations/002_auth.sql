-- Role disiapkan sekarang, tetapi baru dipakai untuk mengatur hak akses -- pada pertemuan 6. 
ALTER TABLE students 
    ADD COLUMN IF NOT EXISTS role VARCHAR(20) NOT NULL DEFAULT 'user'; 
  -- Refresh token disimpan sebagai HASH, bukan nilai aslinya. -- Alasannya sama seperti password: bila isi table ini bocor, penyerang -- tetap tidak memiliki token yang dapat dipakai. 
CREATE TABLE IF NOT EXISTS refresh_tokens ( 
    id            BIGSERIAL   PRIMARY KEY, 
    student_id    INTEGER     NOT NULL REFERENCES students(ID) ON DELETE CASCADE, 
    token_hash TEXT        NOT NULL UNIQUE, 
    expires_at TIMESTAMPTZ NOT NULL, 
    revoked_at TIMESTAMPTZ, 
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW() 
); 
  
CREATE INDEX IF NOT EXISTS refresh_tokens_user_id_idx 
    ON refresh_tokens (student_id); 