# ARCHITECTURE

## Recent Architectural Decisions

### 2026-10-12 — 全欄位查詢、`search_log` 查詢紀錄、DB 整合測試慣例
- `search.TypeName` 移除，改為 `TypeAll`：沒加 `@醫院`/`@科別` 的輸入同時比對 `name`/`hospital`/`department`。排序以 `matchRankExpr`（`CASE WHEN name ILIKE $1 THEN 0 WHEN hospital ILIKE $1 THEN 1 ELSE 2 END`）為主、`id` 為次；`@` 類型依 `id`。`Search` 從此一律有 `ORDER BY`，offset 分頁穩定。count 查詢用 `COUNT(*) FILTER` 一次取總數與三組筆數（`SearchStats.MatchCounts`，只在 `TypeAll` 填），摘要據此分組顯示。理由與「不保留姓名專查」的取捨見 open-decisions OD-6。
- 新表 `search_log`（migration `0004_add_search_log`）：每次文字訊息觸發的查詢寫一列（類型、城市、查詢字串、結果筆數），翻頁、說明、加好友不寫；寫入失敗只 `slog.Error`、不影響回覆。**刻意不存使用者識別**（OD-5），由 `TestSearchLogHasNoUserColumn` 釘住欄位集合。
- Quick Reply 以 `template.WithQuickReply` 疊加在既有訊息 JSON 上（LINE 只顯示一次回覆中最後一則的 QR）；說明卡（加好友與「幫助」共用）附範例、0 筆時附「擴大搜尋／使用說明」。範例不用真實醫師全名（OD-6）。
- DB 整合測試慣例：`//go:build integration` + `TEST_DATABASE_URL`，由 `make test-integration` 起本機 Docker `postgres:18.6-bookworm`、套全部 up migration、跑 `internal/database` 測試、再驗證 0004 down。預設 `go test ./...` 與 CI 仍不需 DB。

### 2026-07-18 — health / readiness / version 端點三分（統一 workspace 標準）
- 新增 `GET /readyz`（readiness，查 DB，失敗回 `503` + `{"status":"unhealthy","checks":{"db":"error"}}`）與 `GET /version`（回 `{"version":"<APP_VERSION>"}`）；`GET /health` 維持 liveness 靜態 `200`。對齊 jerry-wiki `concepts/health-checks.md` 拍板的標準：liveness 永不查相依、readiness 只查 inbound 必需的 DB（不查 LINE API 等 outbound）、只有 readiness 能回非 200。
- readiness 依賴以 `server.readinessChecker` 介面注入（`*database.Repository` 透過新增的 `Ping(ctx)` 滿足之，編譯期斷言 `var _ readinessChecker = (*database.Repository)(nil)`）；使 handler 能用 stub 白箱測試、延續「測試不連 DB」慣例。`server.New` 因此新增 `version` 參數（唯一 caller `cmd/server/main.go`）。
- `Config.AppVersion` 讀 `APP_VERSION`（caarlos0/env `envDefault:"unknown"`），由 deploy chart 注入（= image tag = commit short-SHA），免動 dockerfile/CI。
- 跨 repo 後續：`jerrytech-deploy/charts/app` 的 readiness/startup probe 改打 `/readyz`、新增 startupProbe，須在本 image 部署上線後才 rollout（見該 repo `docs/health-probe-standardization.md`）。

### 2026-06-09 — blog 重爬對帳工具(`cmd/reconcile` + `internal/reconcile`)
- 新增 package `internal/reconcile`(純函式核心 `ParsePost`/`BuildPlan` + I/O 外殼 `FetchPlaintextPost`)與 binary `cmd/reconcile`,把 `source`/`source_url` 從全 NULL 回填:抓 `popolist999.blogspot.com` 的「純文字版」post(Blogger feed JSON `?alt=json`,內容為 5 欄 HTML `<table>`),以 `normalizeKey(姓名+醫院)`(`strings.Fields` 去全空白)對帳。命中且未分類的既有列標 `source='blog'`、blog 有 DB 無者新增(皆 `verification_status='unverified'`,不宣稱查證);**已分類列與 DB 有 blog 無的列一律不動**(守「不臆測回填」)。`BuildPlan` 冪等可重跑。
- **無新增 go.mod 依賴**(HTML 用 stdlib `regexp`+`html.UnescapeString`)。工具讀 `DATABASE_URL` env(沿用 migration runner 慣例,非 server 的 `DB_*`),支援 `-dry-run` 預覽。CI 無 DB/網路,故核心以 fixture 單元測試、I/O 外殼僅 `go build`/`go vet`;實際 production 回填為操作者手動執行。

### 2026-06-09 — 對齊 0001 baseline 與 production 的 medical_personnel 表結構
- `0001_init.up.sql` 原以 `TEXT` 宣告六欄、缺 `created_at`/`updated_at` 與 `city` 索引,與樹莓派 production 手動建立的表(`VARCHAR(50/200/100/100/500)` + 兩個 `TIMESTAMPTZ` + `ix_medical_personnel_city`)不一致。改寫 baseline 使**全新/空 DB** 跑 migration 能重現 production 結構;六欄改對應長度 `VARCHAR`、新增 `created_at TIMESTAMPTZ DEFAULT now()`/`updated_at TIMESTAMPTZ`、加 `ix_medical_personnel_city` 索引。
- 保留 `CREATE TABLE IF NOT EXISTS`(production 已有表故為 no-op,`schema_migrations` 已在 version 2 不重跑 0001);`source`/`verification_status`/`source_url` 仍屬 0002 不併入 baseline。純對齊,Go 程式讀取欄位集合不變。

### 2026-06-09 — 導入 golang-migrate migration runner(baseline 策略 + image 內建)
- 比照 papiin 導入 golang-migrate 作為正式 migration 機制,取代先前「`migrations/` 為手動 DDL 慣例、無 runner」的作法。`migrate` binary(v4.18.1, `postgres` tag)與 `migrations/` 一起打包進 image,由 jerrytech-deploy chart 的 init container(`/migrate -path /migrations -database $DATABASE_URL up`)在 app 啟動前自動套用。
- 既有 DB 有資料且無 `schema_migrations` 表,故採 **baseline 策略**:`0001_init` 用 `CREATE TABLE IF NOT EXISTS` 描述現行 schema(在既有 DB 上為 no-op,無需 `force`);`0002_add_source_verification` 為來源/驗證欄位變更(PR #29 DDL 改名拆 up/down)。命名遵循 golang-migrate `NNNN_name.up.sql`/`.down.sql` 慣例,每個變更須 up/down 成對。
- 上線前提:0002 DDL 勿事先手動套用,交給 runner;Pi secret 已加 `DATABASE_URL`(僅供 migrate init container,app 仍讀 split `DB_*`)。

### 2026-06-09 — medical_personnel 加上來源/驗證/佐證欄位
- `medical_personnel` 新增三欄:`source TEXT`(nullable, CHECK `IN ('blog','public_report')`)、`verification_status TEXT NOT NULL DEFAULT 'unverified'`(CHECK `IN ('unverified','self_reported','verified')`)、`source_url TEXT`(nullable)。現有列 `source`/`source_url` 刻意留 NULL,不臆測回填;`verification_status` 由 DEFAULT 回填 `unverified`。
- 新增 `migrations/` 目錄作為手動 DDL 慣例(repo 無 migration runner,DDL 手動套用於樹莓派 Postgres,且須先於程式部署套用)。
- `internal/database/repository.go` 抽出 package-level 常數 `selectColumns`,使 SELECT 欄位列表可在無 DB 下被單元測試。
- LINE 卡片以中性文案誠實標示來源/驗證程度(`sourceLabel`/`verificationLabel` 對照函式),落實 `docs/product-direction.md` §5 鐵律 3。
