ALTER TABLE companies
    DROP COLUMN IF EXISTS office_latitude,
    DROP COLUMN IF EXISTS office_longitude,
    DROP COLUMN IF EXISTS office_radius_meters;
