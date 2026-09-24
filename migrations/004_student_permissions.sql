-- Permission khusus untuk entity students.
INSERT INTO permissions (name, description) VALUES
    ('student:list',         'Melihat daftar seluruh student'),
    ('student:read:any',     'Melihat data student mana pun'),
    ('student:create',       'Mendaftarkan student baru'),
    ('student:update:any',   'Mengubah data student mana pun'),
    ('student:delete',       'Menghapus student'),
    ('student:role:assign',  'Mengubah role milik student lain')
ON CONFLICT (name) DO NOTHING;

INSERT INTO role_permissions (role_name, permission_name) VALUES
    ('admin', 'student:list'),
    ('admin', 'student:read:any'),
    ('admin', 'student:create'),
    ('admin', 'student:update:any'),
    ('admin', 'student:delete'),
    ('admin', 'student:role:assign'),
    ('staff', 'student:list'),
    ('staff', 'student:read:any'),
    ('staff', 'student:create')
ON CONFLICT DO NOTHING;

-- Data lama belum memiliki informasi pemilik. NULL dipertahankan sampai
-- pemiliknya dapat ditentukan secara valid; data baru akan diisi aplikasi.
ALTER TABLE students
    ADD COLUMN IF NOT EXISTS owner_id INTEGER;

ALTER TABLE students
    DROP CONSTRAINT IF EXISTS students_owner_id_fkey;

ALTER TABLE students
    ADD CONSTRAINT students_owner_id_fkey
    FOREIGN KEY (owner_id) REFERENCES students(id) ON DELETE SET NULL;

CREATE INDEX IF NOT EXISTS students_owner_id_idx ON students (owner_id);
