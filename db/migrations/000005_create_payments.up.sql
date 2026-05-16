-- Migration: 000005_create_payments
-- Creates the payments table for payment gateway transaction records

CREATE TABLE IF NOT EXISTS payments (
    id              UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    idempotency_key UUID         NOT NULL,
    reference_id    UUID         NOT NULL,
    -- points to reservations.id (BOOKING_FEE) or invoices.id (PARKING_FEE)
    payment_type    SMALLINT     NOT NULL CHECK (payment_type IN (1, 2)),
    -- 1=BOOKING_FEE, 2=PARKING_FEE
    amount_idr      BIGINT       NOT NULL CHECK (amount_idr > 0),
    method          SMALLINT     NOT NULL DEFAULT 1 CHECK (method IN (1)),
    -- 1=QRIS
    status          SMALLINT     NOT NULL DEFAULT 1 CHECK (status IN (1, 2, 3, 4)),
    -- 1=PENDING, 2=SUCCESS, 3=FAILED, 4=EXPIRED
    gateway_ref     VARCHAR(255),
    created_at      TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ  NOT NULL DEFAULT now(),

    CONSTRAINT payments_idempotency_key_unique UNIQUE (idempotency_key)
);

CREATE INDEX idx_payments_reference_id   ON payments (reference_id);
CREATE INDEX idx_payments_status         ON payments (status);
CREATE INDEX idx_payments_payment_type   ON payments (payment_type);
