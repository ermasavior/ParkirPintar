-- Migration: 000006_seed_spots
-- Seeds the parking inventory: 5 floors × 30 car spots + 5 floors × 50 motorcycle spots
-- Total: 150 car spots + 250 motorcycle spots = 400 spots

DO $$
DECLARE
    floor     INT;
    spot_num  INT;
    code      VARCHAR(10);
BEGIN
    -- Car spots: floors 1-5, spots A1..A30 per floor (vehicle_type=1)
    FOR floor IN 1..5 LOOP
        FOR spot_num IN 1..30 LOOP
            code := 'A' || spot_num::TEXT;
            INSERT INTO spots (floor_number, spot_code, vehicle_type, status)
            VALUES (floor, code, 1, 1);
        END LOOP;
    END LOOP;

    -- Motorcycle spots: floors 1-5, spots B1..B50 per floor (vehicle_type=2)
    FOR floor IN 1..5 LOOP
        FOR spot_num IN 1..50 LOOP
            code := 'B' || spot_num::TEXT;
            INSERT INTO spots (floor_number, spot_code, vehicle_type, status)
            VALUES (floor, code, 2, 1);
        END LOOP;
    END LOOP;
END $$;
