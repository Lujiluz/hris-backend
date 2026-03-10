CREATE TABLE company_employee_sequences (
    company_id UUID NOT NULL REFERENCES companies(id) ON DELETE CASCADE,
    year INT NOT NULL,
    counter INT NOT NULL DEFAULT 0,
    PRIMARY KEY (company_id, year)
);