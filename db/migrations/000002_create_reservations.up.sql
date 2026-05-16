-- Migration: 000002_create_reservations
-- Creates the reservations table

CREATE TABLE IF NOT EXISTS reservations (
    id               UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    idempotency_key  UUID        NOT NULL,
    driver_id        UUID        NOT NULL,
    spot_id          UUID        NOT NULL REFERENCES spots (id),
    vehicle_type     SMALLINT    NOT NULL CHECK (vehicle_type IN (1, 2)),
    -- 1=CAR, 2=MOTORCYCLE
    assignment_mode  SMALLINT    NOT NULL CHECK (assignment_mode IN (1, 2)),
    -- 1=SYSTEM_ASSIGNED, 2=USER_SELECTED
    status           SMALLINT    NOT NULL DEFAULT 1 CHECK (status IN (1, 2, 3, 4, 5, 6)),
    -- 1=PENDING_PAYMENT, 2=CONFIRMED, 3=EXPIRED, 4=CHECKED_IN, 5=COMPLETED, 6=CANCELLED
    confirmed_at     TIMESTAMPTZ,
    expires_at       TIMESTAMPTZ,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),

    CONSTRAINT reservations_idempotency_key_unique UNIQUE (idempotency_key)
);

CREATE INDEX idx_reservations_driver_id        ON reservations (driver_id);
CREATE INDEX idx_reservations_spot_id          ON reservations (spot_id);
CREATE INDEX idx_reservations_status           ON reservations (status);
CREATE INDEX idx_reservations_expires_at       ON reservations (expires_at) WHERE status = 2;
-- Partial index for expiry scheduler: only CONFIRMED reservations need expiry checks
