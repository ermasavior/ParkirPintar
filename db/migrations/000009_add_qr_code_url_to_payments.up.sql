-- Migration: 000009_add_qr_code_url_to_payments
-- Adds qr_code_url column to payments table for idempotency replay

ALTER TABLE payments
    ADD COLUMN qr_code_url TEXT NOT NULL DEFAULT '';
