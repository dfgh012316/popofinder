-- 記錄每次使用者發起的查詢(不含翻頁),用來量測零結果率與覆蓋率缺口。
-- 刻意不存任何使用者識別(LINE userId 或其雜湊),見 docs/open-decisions.md OD-5。
CREATE TABLE search_log (
    id           BIGSERIAL PRIMARY KEY,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    search_type  TEXT NOT NULL CHECK (search_type IN ('all', 'hospital', 'department')),
    city         TEXT,
    search_term  TEXT NOT NULL,
    result_count INTEGER NOT NULL
);
