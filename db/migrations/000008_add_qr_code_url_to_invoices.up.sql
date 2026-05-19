-- Migration: 000008_add_qr_code_url_to_invoices
-- Adds qr_code_url column to invoices table for idempotency replay

ALTER TABLE invoices
    ADD COLUMN qr_code_url TEXT NOT NULL DEFAULT '';
