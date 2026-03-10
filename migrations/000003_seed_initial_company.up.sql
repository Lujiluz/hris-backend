CREATE EXTENSION IF NOT EXISTS "pgcrypto";

INSERT INTO companies (id, company_code, name, created_at, updated_at)
VALUES (
    gen_random_uuid(), 
    'GOTO', 
    'Gojek Tokopedia', 
    NOW(), 
    NOW()
) ON CONFLICT (company_code) DO NOTHING;
