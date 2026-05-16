-- Migration: 000003_create_sessions
-- Creates the sessions table representing active/completed parking sessions

CREATE TABLE IF NOT EXISTS sessions (
    id             UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    reservation_id UUID        NOT NULL REFERENCES reservations (id),
    driver_id      UUID        NOT NULL,
    spot_id        UUID        NOT NULL REFERENCES spots (id),
    status         SMALLINT    NOT NULL DEFAULT 1 CHECK (status IN (1, 2)),
    -- 1=ACTIVE, 2=COMPLETED
    checked_in_at  TIMESTAMPTZ NOT NULL,
    checked_out_at TIMESTAMPTZ
);

CREATE INDEX idx_sessions_reservation_id ON sessions (reservation_id);
CREATE INDEX idx_sessions_driver_id      ON sessions (driver_id);
CREATE INDEX idx_sessions_spot_id        ON sessions (spot_id);
CREATE INDEX idx_sessions_status         ON sessions (status);
