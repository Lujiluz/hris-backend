ALTER TABLE attendance_records
  ADD COLUMN is_offline_submission BOOLEAN NOT NULL DEFAULT FALSE;
