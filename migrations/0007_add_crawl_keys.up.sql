-- 官網爬蟲(docs/open-decisions.md OD-7、OD-10):每位醫師在各來源的穩定鍵,以及下架排除清單。
ALTER TABLE medical_personnel
    ADD COLUMN source_key TEXT;

CREATE UNIQUE INDEX ux_medical_personnel_source_key
    ON medical_personnel (source_key) WHERE source_key IS NOT NULL;

-- 被下架的官網醫師:爬蟲重爬時跳過,避免又加回來(OD-9、OD-10)。只存來源鍵,不存姓名等內容。
CREATE TABLE takedown_suppressions (
    source_key TEXT PRIMARY KEY,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- 只有 source_checked_at / updated_at 改變的 UPDATE(每月重爬更新查到日期)不記歷史(OD-10)。
CREATE OR REPLACE FUNCTION record_medical_personnel_history() RETURNS trigger
LANGUAGE plpgsql AS $$
DECLARE
    -- SET LOCAL 的交易結束後,同一條連線讀到的是空字串而不是 NULL,所以要 NULLIF。
    change_reason TEXT := NULLIF(current_setting('popo.change_reason', true), '');
BEGIN
    IF TG_OP = 'INSERT' THEN
        INSERT INTO medical_personnel_history (personnel_id, op, new_row, reason)
        VALUES (NEW.id, TG_OP, to_jsonb(NEW), change_reason);
    ELSIF TG_OP = 'UPDATE' THEN
        IF (to_jsonb(OLD) - 'source_checked_at' - 'updated_at') = (to_jsonb(NEW) - 'source_checked_at' - 'updated_at') THEN
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

-- 下架官網醫師時,把它的 source_key 記進排除清單(OD-10)。
CREATE OR REPLACE FUNCTION takedown_personnel(p_id INTEGER) RETURNS void
LANGUAGE plpgsql AS $$
DECLARE
    v_key TEXT;
BEGIN
    DELETE FROM medical_personnel WHERE id = p_id RETURNING source_key INTO v_key;
    IF NOT FOUND THEN
        RAISE EXCEPTION 'medical_personnel id % not found', p_id;
    END IF;
    IF v_key IS NOT NULL THEN
        INSERT INTO takedown_suppressions (source_key) VALUES (v_key) ON CONFLICT DO NOTHING;
    END IF;
    -- 上面的 DELETE 已由 trigger 寫入一筆含 old_row 的歷史,這裡連同它一起清除。
    DELETE FROM medical_personnel_history WHERE personnel_id = p_id;
    INSERT INTO medical_personnel_history (personnel_id, op, reason) VALUES (p_id, 'DELETE', 'takedown');
END;
$$;
