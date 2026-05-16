-- Migration: 000004_create_invoices
-- Creates the invoices table for parking fee billing records

CREATE TABLE IF NOT EXISTS invoices (
    id                UUID   PRIMARY KEY DEFAULT gen_random_uuid(),
    idempotency_key   UUID   NOT NULL,
    session_id        UUID   NOT NULL REFERENCES sessions (id),
    reservation_id    UUID   NOT NULL REFERENCES reservations (id),
    status            SMALLINT NOT NULL DEFAULT 1 CHECK (status IN (1, 2, 3)),
    -- 1=PENDING_PAYMENT, 2=PAID, 3=PAYMENT_FAILED
    booking_fee_idr   BIGINT NOT NULL DEFAULT 5000,
    parking_fee_idr   BIGINT NOT NULL CHECK (parking_fee_idr >= 0),
    overnight_fee_idr BIGINT NOT NULL DEFAULT 0 CHECK (overnight_fee_idr IN (0, 20000)),
    total_idr         BIGINT NOT NULL CHECK (total_idr >= 0),
    created_at        TIMESTAMPTZ NOT NULL DEFAULT now(),

    CONSTRAINT invoices_idempotency_key_unique UNIQUE (idempotency_key)
);

CREATE INDEX idx_invoices_session_id     ON invoices (session_id);
CREATE INDEX idx_invoices_reservation_id ON invoices (reservation_id);
CREATE INDEX idx_invoices_status         ON invoices (status);
