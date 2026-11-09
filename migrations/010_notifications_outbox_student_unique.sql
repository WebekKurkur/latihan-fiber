-- Migration 010: idempotency key notifikasi per penerima (Modul 11, D.2)
--   psql -U postgres -d praktikum_backend -f migrations/010_notifications_outbox_student_unique.sql
--
-- Migration 005 membuat UNIQUE (outbox_id) dengan asumsi satu event =
-- satu penerima. Model broadcast (D.2) melanggar asumsi itu: satu event
-- "kuota kelas berubah" harus sampai ke seluruh peserta kelas.
--
-- Solusinya bukan menghapus idempotency, melainkan membuatnya lebih halus:
-- key-nya menjadi pasangan (outbox_id, student_id). Dengan begitu:
--   - event KRS biasa tetap satu baris notifikasi per event;
--   - event broadcast boleh punya satu baris per peserta;
--   - task yang dijalankan dua kali untuk peserta yang sama tetap
--     menghasilkan satu notifikasi saja.
--
-- Script ini idempotent: menjalankannya berkali-kali aman.

DO $$
BEGIN
    -- Constraint lama harus dihapus lebih dulu, karena UNIQUE (outbox_id)
    -- akan menolak baris broadcast yang berbagi outbox_id.
    IF EXISTS (
        SELECT 1
        FROM pg_constraint
        WHERE conrelid = 'notifications'::regclass
          AND conname = 'notifications_outbox_unique'
    ) THEN
        ALTER TABLE notifications
            DROP CONSTRAINT notifications_outbox_unique;
    END IF;
END
$$;

-- Idempotency key yang baru. Nama constraint dibuat deterministik agar
-- migration ini aman dijalankan ulang.
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM pg_constraint
        WHERE conrelid = 'notifications'::regclass
          AND conname = 'notifications_outbox_student_unique'
    ) THEN
        ALTER TABLE notifications
            ADD CONSTRAINT notifications_outbox_student_unique
            UNIQUE (outbox_id, student_id);
    END IF;
END
$$;

-- Index lama student_id, outbox_id DESC masih dipakai untuk query
-- "notifikasi milik mahasiswa", jadi tidak perlu diubah.
CREATE INDEX IF NOT EXISTS notifications_student_idx
    ON notifications (student_id, outbox_id DESC);
