-- 下架程序(docs/open-decisions.md OD-8、OD-9):同一交易內刪主表 → 清除該醫師的全部歷史 → 寫一筆不含內容的紀錄。
-- 用法(psql):SELECT takedown_personnel(<id>);
-- 找不到該 id 時報錯並整個回滾,避免打錯 id 卻以為已經下架。
CREATE FUNCTION takedown_personnel(p_id INTEGER) RETURNS void
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
