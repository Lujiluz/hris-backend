DO $$
DECLARE
    v_employee_uuid UUID;
BEGIN
    SELECT id INTO v_employee_uuid FROM employees WHERE email = 'admin@hris.com';

    IF v_employee_uuid IS NOT NULL THEN
        DELETE FROM employee_schedules WHERE employee_id = v_employee_uuid;
        DELETE FROM employees WHERE id = v_employee_uuid;
    END IF;
END $$;
