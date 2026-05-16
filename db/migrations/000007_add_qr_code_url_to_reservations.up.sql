-- Migration: 000007_add_qr_code_url_to_reservations
-- Adds qr_code_url column to reservations table for idempotency replay

ALTER TABLE reservations
    ADD COLUMN qr_code_url TEXT NOT NULL DEFAULT '';
