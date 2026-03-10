DELETE FROM company_employee_sequences 
WHERE company_id = (SELECT id FROM companies WHERE company_code = 'GOTO');

DELETE FROM companies WHERE company_code = 'GOTO';