# ARCHITECTURE

## Recent Architectural Decisions

### 2026-10-12 — 官網爬蟲框架（`cmd/crawl` + `internal/crawl`）＋亞東
- 新 package `internal/crawl`：`Source` 介面（`Name`、`Crawl`，任一頁失敗就回 error、不回部分結果）、`Fetcher`（UA `popofinder-crawler/1.0 (+mailto:…)`、每 host 讀一次 robots.txt、同 host 間隔 `max(minInterval, Crawl-delay)`、非 200 回 error、不支援並行）、學歷規則 `hasMedicalDegree`（OD-10）、純函式 `BuildSyncPlan`（insert／update／只改日期的 touch／delete、排除清單、無醫學學位跳過、符合條件數 ×2 < 現有數時不刪並設 `DeleteBlocked`）。新增醫院＝新的 `Source` 實作，加進 `cmd/crawl` 的 `registry`。第一個實作 `Femh`（亞東：索引→科別→個人頁，以站內 DoctorID 為鍵）。
- **新依賴** `github.com/temoto/robotstxt v1.1.2`：robots.txt 回 5xx 時 `FindGroup().Test()` 會判定為允許，必須用 `TestAgent`（`TestFetcherTreatsRobots5xxAsDisallow` 釘住）。
- **Schema（migration `0007_add_crawl_keys`）**：`medical_personnel.source_key`（`<來源>:<站內 ID>`，partial unique index）、`takedown_suppressions`（只存鍵）。trigger 改為「去掉 `source_checked_at`／`updated_at` 後前後相同就不記」，取代 0005 條目的「內容沒變的 UPDATE 不記」；`takedown_personnel` 下架有 `source_key` 的列時寫入排除清單，完成 0006 條目留給爬蟲 task 的部分。
- `Repository.LoadCrawlState`／`ApplyCrawlPlan`：一個交易、`set_config('popo.change_reason', 'crawl:<來源>', true)`、`source='hospital_directory'`；`source_checked_at` 以 `"2006-01-02"` 字串傳入。`cmd/crawl`（`-sources`、`-dry-run`）連 DB 前驗證來源名稱（錯誤 exit 2），時區用 `time.FixedZone`（distroless 沒有 tzdata），log 只記數量；image 多一個 `/crawl`。
- parser 測試只用合成 fixture（虛構姓名）；`//go:build live` 的 `TestFemhLive` 直接打官網確認結構（4 個請求，只印數量），CI 不跑。

### 2026-10-12 — 下架機制：寄信申請＋`takedown_personnel()`
- 新 DB function `takedown_personnel(p_id INTEGER)`（migration `0006_add_takedown_function`）：同一交易內刪主表 → 清除該 `personnel_id` 的全部歷史（包括 DELETE 時 trigger 剛寫入、含 `old_row` 的那一筆）→ 寫一筆不含內容的 `reason='takedown'` 紀錄；id 不存在時 `RAISE EXCEPTION` 並回滾。維護者在 psql 執行，沒有 Go 呼叫端；`TestHistoryAllowsTakedownPurge`、`TestTakedownPersonnelRejectsUnknownID` 釘住。
- 下架管道（OD-9）：說明卡寫明信箱，醫師本人或院所寄信並附證明，人工判斷；不在 bot 放按鈕、不建 DB 收件匣（`TestDoctorBubbleHasNoTakedownButton` 釘住卡片沒有按鈕）。爬蟲的排除清單與整院下架留給爬蟲 task。

### 2026-10-12 — `medical_personnel_history`（trigger 記錄異動）與 `source_checked_at`
- 新表 `medical_personnel_history`（migration `0005_add_history_and_checked_date`）：由 AFTER INSERT/UPDATE/DELETE FOR EACH ROW trigger `record_medical_personnel_history()` 寫入，存整列 JSONB（`old_row`/`new_row`）與 `reason`；內容沒變的 UPDATE 不記；`personnel_id` 不設 FK。用 DB trigger 而不是在應用程式裡記，是因為 server 不寫這張表，所有修改都來自手動 SQL 或腳本（reconcile CronJob 已停用）。批次修改前在交易內 `SET LOCAL popo.change_reason`；SET LOCAL 的交易結束後，同一條連線的 `current_setting` 會回空字串，trigger 用 `NULLIF` 轉成 NULL。TRUNCATE 不會觸發，不留紀錄。
- 下架規則（OD-8）：同一交易內刪主表 → 刪該 id 的全部歷史 → 寫一筆不含內容的 DELETE（`reason='takedown'`）。`TestHistoryAllowsTakedownPurge` 釘住「不設 FK、`old_row`/`new_row` 可為 NULL」。
- `medical_personnel.source_checked_at DATE`：`source_url` 指向的頁面在哪一天確認過。卡片只在有連結時附「（YYYY-MM-DD 查到）」。官網校對沿用 0003 的 `source='hospital_directory'`（卡片顯示「醫療院所官網」），不新增來源值（OD-7）。
- `make test-integration` 改為驗證最新一支 down migration，取代下面 2026-10-12 那條寫的「0004 down」；之後新增 migration 不必改 makefile。

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
