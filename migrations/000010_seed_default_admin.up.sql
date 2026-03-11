CREATE EXTENSION IF NOT EXISTS "pgcrypto";

DO $$
DECLARE
    v_company_id   UUID;
    v_employee_uuid UUID;
    v_counter      INT;
    v_employee_id  TEXT;
    v_year         INT := EXTRACT(YEAR FROM NOW())::INT;
BEGIN
    -- Resolve GOTO company
    SELECT id INTO v_company_id FROM companies WHERE company_code = 'GOTO';

    IF v_company_id IS NULL THEN
        RAISE EXCEPTION 'Company GOTO not found. Run migration 000005 first.';
    END IF;

    -- Skip if default admin already exists
    IF EXISTS (SELECT 1 FROM employees WHERE email = 'admin@hris.com') THEN
        RETURN;
    END IF;

    -- Atomically increment the sequence counter (mirrors application logic)
    INSERT INTO company_employee_sequences (company_id, year, counter)
    VALUES (v_company_id, v_year, 1)
    ON CONFLICT (company_id, year)
    DO UPDATE SET counter = company_employee_sequences.counter + 1
    RETURNING counter INTO v_counter;

    v_employee_id  := 'GOTO-' || v_year || '-' || LPAD(v_counter::TEXT, 4, '0');
    v_employee_uuid := gen_random_uuid();

    -- Insert default admin user (password: Admin@1234)
    INSERT INTO employees (id, company_id, employee_id, email, phone_number, password, is_tnc_accepted, role, created_at, updated_at)
    VALUES (
        v_employee_uuid,
        v_company_id,
        v_employee_id,
        'admin@hris.com',
        '+628000000000',
        crypt('Admin@1234', gen_salt('bf', 10)),
        TRUE,
        'admin',
        NOW(),
        NOW()
    );

    -- Seed Mon–Fri 08:00–17:00 schedules for the new admin
    INSERT INTO employee_schedules (id, employee_id, company_id, day_of_week, clock_in_time, clock_out_time, is_active)
    SELECT
        gen_random_uuid(),
        v_employee_uuid,
        v_company_id,
        d.day,
        '08:00:00',
        '17:00:00',
        TRUE
    FROM (SELECT generate_series(1, 5) AS day) d
    ON CONFLICT (employee_id, day_of_week) DO NOTHING;
END $$;
