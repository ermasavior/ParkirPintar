-- Rollback: 000008_add_qr_code_url_to_invoices
ALTER TABLE invoices
    DROP COLUMN IF EXISTS qr_code_url;
