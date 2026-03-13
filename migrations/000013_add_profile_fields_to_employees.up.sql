ALTER TABLE employees
    ADD COLUMN first_name      VARCHAR(100) NULL,
    ADD COLUMN last_name       VARCHAR(100) NULL,
    ADD COLUMN profile_picture TEXT NOT NULL DEFAULT 'https://res.cloudinary.com/dmvot15pm/image/upload/v1773207988/attendance/selfies/public_id_12345.png';
