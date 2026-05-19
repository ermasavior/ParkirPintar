-- Rollback: 000009_add_qr_code_url_to_payments
ALTER TABLE payments
    DROP COLUMN IF EXISTS qr_code_url;
