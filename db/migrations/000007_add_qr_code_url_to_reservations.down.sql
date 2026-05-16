-- Rollback: 000007_add_qr_code_url_to_reservations
ALTER TABLE reservations
    DROP COLUMN IF EXISTS qr_code_url;
