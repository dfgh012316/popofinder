# Tasks INDEX

## Completed Tasks

- [2026-06-09] 為 medical_personnel 加上來源/驗證/佐證欄位並誠實標示於 LINE 卡片 — 新增 source/verification_status/source_url 三欄(migration + struct + 卡片中性顯示),為後續重爬對帳/眾包校對的前置地基;詳見 docs/ARCHITECTURE.md。
- [2026-06-09] 為 popofinder 補上 golang-migrate migration runner — 新增 0001 baseline(CREATE TABLE IF NOT EXISTS)+ 0002 來源/驗證變更(up/down 成對),dockerfile 內建 migrate v4.18.1 與 migrations/ 供 init container 自動套用;詳見 docs/ARCHITECTURE.md。
