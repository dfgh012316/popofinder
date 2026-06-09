-- 還原為原本兩種來源。
-- 注意:若已有 hospital_directory 資料,golang-migrate 會先跑 0004 down 移除資料,再跑此檔。
ALTER TABLE medical_personnel
    DROP CONSTRAINT medical_personnel_source_check;
ALTER TABLE medical_personnel
    ADD CONSTRAINT medical_personnel_source_check
        CHECK (source IN ('blog', 'public_report'));
