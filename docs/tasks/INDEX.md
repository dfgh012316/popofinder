# Tasks INDEX

## Completed Tasks

- [2026-06-09] 為 medical_personnel 加上來源/驗證/佐證欄位並誠實標示於 LINE 卡片 — 新增 source/verification_status/source_url 三欄(migration + struct + 卡片中性顯示),為後續重爬對帳/眾包校對的前置地基;詳見 docs/ARCHITECTURE.md。
- [2026-06-09] 為 popofinder 補上 golang-migrate migration runner — 新增 0001 baseline(CREATE TABLE IF NOT EXISTS)+ 0002 來源/驗證變更(up/down 成對),dockerfile 內建 migrate v4.18.1 與 migrations/ 供 init container 自動套用;詳見 docs/ARCHITECTURE.md。
- [2026-06-09] 對齊 0001 baseline migration 與樹莓派 production 的 medical_personnel 表結構 — 將 baseline 六欄由 TEXT 改為對應長度 VARCHAR、補 created_at/updated_at 與 ix_medical_personnel_city 索引,使全新/空 DB 建表結果等同 production(0001 對既有 DB 仍 no-op);詳見 docs/ARCHITECTURE.md。
