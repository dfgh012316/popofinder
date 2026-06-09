-- 擴充 source 允許值,納入「醫院官網名冊」爬取來源。
-- 誠實標示資料來源,符合 docs/product-direction.md 倫理鐵律 3(爬來 / 自填 / 已驗證要分清楚)。
ALTER TABLE medical_personnel
    DROP CONSTRAINT medical_personnel_source_check;
ALTER TABLE medical_personnel
    ADD CONSTRAINT medical_personnel_source_check
        CHECK (source IN ('blog', 'public_report', 'hospital_directory'));
