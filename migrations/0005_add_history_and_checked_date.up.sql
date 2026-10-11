-- 官網校對與資料異動歷史的 schema 地基,見 docs/open-decisions.md OD-7、OD-8。
-- source_checked_at:source_url 指向的頁面在哪一天確認過;既有資料保持 NULL,不回填。
ALTER TABLE medical_personnel
    ADD COLUMN source_checked_at DATE;

-- 記錄 medical_personnel 的每一次異動(含手動 SQL),之後才能比對改了什麼、為什麼改。
-- 批次修改前在交易內先 SET LOCAL popo.change_reason = '<原因>'(或 set_config('popo.change_reason', <原因>, true)),
-- reason 欄會記下原因;未設定時為 NULL。personnel_id 刻意不設 FK,主表刪了歷史還在。
-- 下架時要連同該 personnel_id 的歷史一起清除,只留一筆不含內容的紀錄,見 OD-8。
CREATE TABLE medical_personnel_history (
    id           BIGSERIAL PRIMARY KEY,
    personnel_id INTEGER NOT NULL,
    op           TEXT NOT NULL CHECK (op IN ('INSERT', 'UPDATE', 'DELETE')),
    changed_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    old_row      JSONB,
    new_row      JSONB,
    reason       TEXT
);

CREATE INDEX ix_medical_personnel_history_personnel_id ON medical_personnel_history (personnel_id);

CREATE FUNCTION record_medical_personnel_history() RETURNS trigger
LANGUAGE plpgsql AS $$
DECLARE
    -- SET LOCAL 的交易結束後,同一條連線讀到的是空字串而不是 NULL,所以要 NULLIF。
    change_reason TEXT := NULLIF(current_setting('popo.change_reason', true), '');
BEGIN
    IF TG_OP = 'INSERT' THEN
        INSERT INTO medical_personnel_history (personnel_id, op, new_row, reason)
        VALUES (NEW.id, TG_OP, to_jsonb(NEW), change_reason);
    ELSIF TG_OP = 'UPDATE' THEN
        IF OLD IS NOT DISTINCT FROM NEW THEN
            RETURN NULL;
        END IF;
        INSERT INTO medical_personnel_history (personnel_id, op, old_row, new_row, reason)
        VALUES (NEW.id, TG_OP, to_jsonb(OLD), to_jsonb(NEW), change_reason);
    ELSE
        INSERT INTO medical_personnel_history (personnel_id, op, old_row, reason)
        VALUES (OLD.id, TG_OP, to_jsonb(OLD), change_reason);
    END IF;
    RETURN NULL;
END;
$$;

CREATE TRIGGER medical_personnel_history_trigger
    AFTER INSERT OR UPDATE OR DELETE ON medical_personnel
    FOR EACH ROW EXECUTE FUNCTION record_medical_personnel_history();
