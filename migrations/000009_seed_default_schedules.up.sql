-- Seed default Mon–Fri 08:00–17:00 schedules for all employees in the GOTO company.
-- Uses ON CONFLICT DO NOTHING so re-running is safe.
INSERT INTO employee_schedules (id, employee_id, company_id, day_of_week, clock_in_time, clock_out_time, is_active)
SELECT
    gen_random_uuid(),
    e.id        AS employee_id,
    e.company_id,
    d.day       AS day_of_week,
    '08:00:00'  AS clock_in_time,
    '17:00:00'  AS clock_out_time,
    true        AS is_active
FROM employees e
CROSS JOIN (
    SELECT generate_series(1, 5) AS day  -- 1=Mon … 5=Fri
) d
WHERE e.company_id = (SELECT id FROM companies WHERE company_code = 'GOTO')
  AND e.deleted_at IS NULL
ON CONFLICT (employee_id, day_of_week) DO NOTHING;
