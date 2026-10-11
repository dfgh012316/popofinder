DROP FUNCTION IF EXISTS takedown_source(TEXT);

CREATE OR REPLACE FUNCTION takedown_personnel(p_id INTEGER) RETURNS void
LANGUAGE plpgsql AS $$
BEGIN
    DELETE FROM medical_personnel WHERE id = p_id;
    IF NOT FOUND THEN
        RAISE EXCEPTION 'medical_personnel id % not found', p_id;
    END IF;
    -- 上面的 DELETE 已由 trigger 寫入一筆含 old_row 的歷史,這裡連同它一起清除。
    DELETE FROM medical_personnel_history WHERE personnel_id = p_id;
    INSERT INTO medical_personnel_history (personnel_id, op, reason) VALUES (p_id, 'DELETE', 'takedown');
END;
$$;

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

DROP TABLE IF EXISTS takedown_suppressions;
DROP INDEX IF EXISTS ux_medical_personnel_source_key;
ALTER TABLE medical_personnel DROP COLUMN IF EXISTS source_key;
