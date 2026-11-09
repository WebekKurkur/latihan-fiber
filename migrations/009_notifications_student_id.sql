-- Migration 009: penyelarasan tabel notifications ke student_id (Modul 11)
--   psql -U postgres -d praktikum_backend -f migrations/009_notifications_student_id.sql
--
-- Modul 11 memakai domain "users" (user_id). Repo ini memakai domain
-- "students" (student_id). Database di lingkungan ini masih menyimpan tabel
-- notifications dengan kolom user_id, sedangkan repository memakai student_id,
-- sehingga query notifikasi akan gagal dengan "column student_id does not
-- exist". Migration ini menamai ulang kolomnya, tanpa kehilangan data.
--
-- Script ini idempotent: menjalankannya berkali-kali aman.

DO $$
BEGIN
    IF EXISTS (
        SELECT 1
        FROM information_schema.columns
        WHERE table_name = 'notifications'
          AND column_name = 'user_id'
    ) THEN
        ALTER TABLE notifications RENAME COLUMN user_id TO student_id;
    END IF;
END
$$;

-- Nama constraint FK ikut mengikuti kolom yang sudah diganti nama.
DO $$
BEGIN
    IF EXISTS (
        SELECT 1
        FROM pg_constraint
        WHERE conname = 'notifications_user_id_fkey'
    ) THEN
        ALTER TABLE notifications
            RENAME CONSTRAINT notifications_user_id_fkey TO notifications_student_id_fkey;
    END IF;
END
$$;

-- Index lama memakai kolom user_id, sehingga tidak lagi bisa dipakai.
DROP INDEX IF EXISTS notifications_user_idx;

-- Index untuk query "notifikasi milik mahasiswa".
CREATE INDEX IF NOT EXISTS notifications_student_idx
    ON notifications (student_id, outbox_id DESC);
