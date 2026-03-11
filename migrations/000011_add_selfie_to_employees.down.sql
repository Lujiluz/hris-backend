ALTER TABLE employees
    DROP COLUMN IF EXISTS selfie_registered_at,
    DROP COLUMN IF EXISTS selfie_url;
