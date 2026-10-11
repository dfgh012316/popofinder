DROP TRIGGER IF EXISTS medical_personnel_history_trigger ON medical_personnel;
DROP FUNCTION IF EXISTS record_medical_personnel_history();
DROP TABLE IF EXISTS medical_personnel_history;
ALTER TABLE medical_personnel DROP COLUMN IF EXISTS source_checked_at;
