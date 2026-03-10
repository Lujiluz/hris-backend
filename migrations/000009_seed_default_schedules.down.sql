DELETE FROM employee_schedules
WHERE company_id = (SELECT id FROM companies WHERE company_code = 'GOTO');
