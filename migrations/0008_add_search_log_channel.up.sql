-- 查詢紀錄分辨是從 LINE 還是網頁版來的(docs/open-decisions.md OD-17)。既有紀錄都來自 LINE。
ALTER TABLE search_log
    ADD COLUMN channel TEXT NOT NULL DEFAULT 'line'
        CHECK (channel IN ('line', 'web'));
