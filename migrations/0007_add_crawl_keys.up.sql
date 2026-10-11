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

-- 下架(OD-8、OD-9、OD-10):刪主表 → 清除這位醫師的全部歷史 → 每個受影響的 id 留一筆不含內容的紀錄。
-- 官網醫師(有 source_key)另外:記進排除清單;同一個 source_key 的所有 id 一起清除
-- (被爬蟲刪除後又加回會拿到新 id,舊 id 的歷史仍有內容)。
-- 主表那一列已不存在(例如已被爬蟲刪除)、但歷史還有內容時,用歷史裡的 id 一樣可以下架;
-- 主表與歷史都找不到內容時報錯並整個回滾,避免打錯 id 卻以為已經下架。
CREATE OR REPLACE FUNCTION takedown_personnel(p_id INTEGER) RETURNS void
LANGUAGE plpgsql AS $$
DECLARE
    v_key TEXT;
BEGIN
    DELETE FROM medical_personnel WHERE id = p_id RETURNING source_key INTO v_key;
    IF NOT FOUND THEN
        SELECT COALESCE(old_row, new_row)->>'source_key' INTO v_key
        FROM medical_personnel_history
        WHERE personnel_id = p_id AND (old_row IS NOT NULL OR new_row IS NOT NULL)
        ORDER BY id DESC
        LIMIT 1;
        IF NOT FOUND THEN
            RAISE EXCEPTION 'medical_personnel id % not found', p_id;
        END IF;
    END IF;
    IF v_key IS NOT NULL THEN
        INSERT INTO takedown_suppressions (source_key) VALUES (v_key) ON CONFLICT DO NOTHING;
        DELETE FROM medical_personnel WHERE source_key = v_key;
    END IF;
    -- 上面的 DELETE 已由 trigger 寫入含 old_row 的歷史,這裡連同它們一起清除。
    WITH purged AS (
        DELETE FROM medical_personnel_history
        WHERE personnel_id = p_id
           OR (v_key IS NOT NULL AND (old_row->>'source_key' = v_key OR new_row->>'source_key' = v_key))
        RETURNING personnel_id
    )
    INSERT INTO medical_personnel_history (personnel_id, op, reason)
    SELECT DISTINCT personnel_id, 'DELETE', 'takedown' FROM purged;
END;
$$;

-- 整院下架(OD-10):對某來源的每一位醫師執行 takedown_personnel,包括已被爬蟲刪除、只剩歷史的醫師。
-- 回傳下架的醫師數(以 source_key 計)。用法:SELECT takedown_source('femh');
CREATE FUNCTION takedown_source(p_source TEXT) RETURNS INTEGER
LANGUAGE plpgsql AS $$
DECLARE
    v_id INTEGER;
    v_count INTEGER := 0;
BEGIN
    FOR v_id IN
        SELECT min(personnel_id)
        FROM medical_personnel_history
        WHERE COALESCE(old_row, new_row)->>'source_key' LIKE p_source || ':%'
        GROUP BY COALESCE(old_row, new_row)->>'source_key'
    LOOP
        PERFORM takedown_personnel(v_id);
        v_count := v_count + 1;
    END LOOP;
    RETURN v_count;
END;
$$;
