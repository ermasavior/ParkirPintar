-- Migration: 000001_create_spots
-- Creates the spots table representing physical parking spots

CREATE TABLE IF NOT EXISTS spots (
    id           UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    floor_number INT         NOT NULL CHECK (floor_number BETWEEN 1 AND 5),
    spot_code    VARCHAR(10) NOT NULL,
    vehicle_type SMALLINT    NOT NULL CHECK (vehicle_type IN (1, 2)),
    -- 1=CAR, 2=MOTORCYCLE
    status       SMALLINT    NOT NULL DEFAULT 1 CHECK (status IN (1, 2, 3)),
    -- 1=AVAILABLE, 2=LOCKED, 3=OCCUPIED

    CONSTRAINT spots_floor_code_unique UNIQUE (floor_number, spot_code)
);

CREATE INDEX IF NOT EXISTS idx_spots_status_vehicle_type ON spots (status, vehicle_type);
CREATE INDEX IF NOT EXISTS idx_spots_floor_vehicle_type  ON spots (floor_number, vehicle_type);
