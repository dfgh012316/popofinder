-- 官網醫師頁上的專長與經歷(docs/open-decisions.md OD-18),和學歷一樣以「；」串接每一行。
-- 部落格、民眾回報與沒有這兩欄的官網是 NULL。
ALTER TABLE medical_personnel
    ADD COLUMN specialty VARCHAR(500),
    ADD COLUMN experience VARCHAR(500);
