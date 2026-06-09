# Task：為 popofinder 補上 golang-migrate migration runner（baseline 0001 + 既有變更 0002 + image 內建 migrate）

## 背景

popofinder 過去只有單一 `medical_personnel` 表且不再變動，因此 schema 由樹莓派上的 Postgres 外部手動維護、repo 無 migration runner。現在服務要擴張、schema 會持續演進，需要一套可重複、可追蹤版本的 migration 機制。本 task 比照 `papiin-dashboard/apps/api` 的作法：把 golang-migrate 的 `migrate` binary 與 `migrations/` 一起打包進 image，由 jerrytech-deploy chart 既有的 init container（`/migrate -path /migrations -database $(DATABASE_URL) up`）在 app 啟動前自動套用。

因為樹莓派上的 DB **已存在且有資料**（`medical_personnel` 為手動建立、無 `schema_migrations` 版本表），採「baseline」策略導入：`0001` 代表**現行 schema**（`CREATE TABLE IF NOT EXISTS`），`0002` 為來源/驗證欄位變更（即 PR #29 的 DDL，改名拆成 up/down）。

### ⚠️ 前置依賴（執行本 task 前必須先滿足）

本 task **建立在 PR #29（branch `task/add-source-verification-fields`）已合併進 `develop` 之上**。該 PR 帶入 `migrations/0001_add_source_verification.sql` 以及 Go struct / LINE 卡片變更。執行本 task 前，`develop` 必須已包含 `migrations/0001_add_source_verification.sql`（Part 2 會把它改名）。若該檔不存在，請先合併 PR #29 再執行本 task。

### 🚦 本 task 範圍外的上線程序（人工執行，不在自動化驗收內）

以下步驟**不屬於本 task 的 Parts／驗收**，由維護者在本 task 的 PR 合併、image build 完成後執行（jerrytech-deploy 由 ArgoCD 自動同步至 production，故不在此 repo 自動套用）：

1. ✅ **已完成（2026-06-09）**：popofinder 的 k8s secret（`popo` namespace，名稱 `popofinder`）已新增 `DATABASE_URL`，值為 `postgresql://postgres:<DB_PASSWORD>@postgres.postgres.svc:5432/popo`（server-side 由既有 `DB_PASSWORD` 組出，與 app 的 `cfg.DatabaseURL()` 同格式）。migrate init container 僅 `envFrom: secretRef`，故需此完整 URL；app 仍讀既有 split `DB_*`，多出的 `DATABASE_URL` 被 `caarlos0/env` 忽略，不影響 app。
2. **不需要 `migrate force`**：`0001_init.up.sql` 用 `CREATE TABLE IF NOT EXISTS`，既有 DB 上 `migrate up` 會把 0001 視為 no-op、再套用 0002，DB 資料不受影響。⚠️ 前提：**勿事先手動套用 0002 的 DDL**，交給 runner 套，否則 `ADD COLUMN` 會因欄位已存在而失敗。
3. 在 `jerrytech-deploy` 的 `apps/popofinder/values.yaml` 設定 `migrations.enabled: true`（觸發 ArgoCD 套用 init container；init container 跑 `migrate ... up`，依序套 0001 no-op + 0002）。

### 涉及檔案

- `migrations/0001_init.up.sql` — 新增；現行 schema 的 baseline（`CREATE TABLE IF NOT EXISTS medical_personnel`）。
- `migrations/0001_init.down.sql` — 新增；baseline 的反向（`DROP TABLE IF EXISTS medical_personnel`）。
- `migrations/0001_add_source_verification.sql` — 既有（來自 PR #29）；本 task 將其改名移除（內容移入 `0002_*.up.sql`）。
- `migrations/0002_add_source_verification.up.sql` — 新增（由上者改名而來）；來源/驗證欄位的 ALTER。
- `migrations/0002_add_source_verification.down.sql` — 新增；上者的反向（`DROP COLUMN`）。
- `dockerfile` — 修改；builder stage 安裝 golang-migrate 並複製 binary；runtime stage 複製 `/migrate` 與 `migrations/`。

---

## Part 1：新增 0001 baseline migration（現行 schema）

### 現況

`migrations/` 目前只有 PR #29 帶入的 `0001_add_source_verification.sql`（單一 ALTER，無 up/down 命名、無 baseline 建表）。沒有任何 migration 負責建立 `medical_personnel` 表本身；該表在樹莓派上是手動建立的。`medical_personnel` 現行欄位（見 `internal/database/repository.go` struct 與 `CLAUDE.md`）為：`id`、`city`、`hospital`、`department`(nullable)、`name`、`education`(nullable)。

### 目標

存在一對 golang-migrate baseline 檔，代表**來源/驗證欄位加入之前**的現行 schema：

- `migrations/0001_init.up.sql`：以 `CREATE TABLE IF NOT EXISTS medical_personnel (...)` 建立含六個既有欄位的表（`department`、`education` 為 nullable，其餘 NOT NULL，`id` 為 `SERIAL PRIMARY KEY`）。`IF NOT EXISTS` 使其在既有 DB 上即便被執行也不會出錯（實務上既有 DB 會以 `force 1` 跳過）。
- `migrations/0001_init.down.sql`：`DROP TABLE IF EXISTS medical_personnel;`（baseline 的標準反向）。

### 不可動的範圍

- 不得在 `0001_init.up.sql` 內加入 `source` / `verification_status` / `source_url` 任何欄位（那是 0002 的職責）。
- 不得修改 `0001_add_source_verification.sql`（Part 2 才處理）。
- 不得改動 `dockerfile`、Go 程式碼或任何 `internal/` 檔案。

### Schema Impact

- 變更：新增 baseline migration，描述既有 `medical_personnel`（`id SERIAL PK`、`city TEXT NOT NULL`、`hospital TEXT NOT NULL`、`department TEXT`、`name TEXT NOT NULL`、`education TEXT`）。
- 上游影響：無（baseline 僅描述已存在的表；`IF NOT EXISTS` 使既有 DB 上 `migrate up` 將 0001 視為 no-op，無需 `force`）。
- 下游消費者：golang-migrate runner（image 內 init container）；fresh/空 DB 會以此建表後再套 0002。
- Migration 需求：是；`migrations/0001_init.up.sql` 與 `migrations/0001_init.down.sql`。

### 實作步驟

1. 建立 `migrations/0001_init.up.sql`，內容為：

   ```sql
   CREATE TABLE IF NOT EXISTS medical_personnel (
       id SERIAL PRIMARY KEY,
       city TEXT NOT NULL,
       hospital TEXT NOT NULL,
       department TEXT,
       name TEXT NOT NULL,
       education TEXT
   );
   ```

2. 建立 `migrations/0001_init.down.sql`，內容為：

   ```sql
   DROP TABLE IF EXISTS medical_personnel;
   ```

---

## Part 2：把來源/驗證變更改名為 0002（up/down 成對）

### 現況

`migrations/0001_add_source_verification.sql`（PR #29）內含單一 `ALTER TABLE medical_personnel ADD COLUMN ...`，新增 `source`（nullable + CHECK `IN ('blog','public_report')`）、`verification_status`（NOT NULL DEFAULT `'unverified'` + CHECK `IN ('unverified','self_reported','verified')`）、`source_url`（nullable）。檔名不符 golang-migrate 的 `NNNN_name.up.sql` / `.down.sql` 慣例，且無 down。

### 目標

來源/驗證變更成為版本 0002，且 up/down 成對：

- `migrations/0002_add_source_verification.up.sql`：內容與原 `0001_add_source_verification.sql` 的 ALTER **完全相同**。
- `migrations/0002_add_source_verification.down.sql`：以單一 `ALTER TABLE ... DROP COLUMN` 移除三個欄位。
- 原 `migrations/0001_add_source_verification.sql` 不再存在（已改名）。

### 不可動的範圍

- 不得更動 ALTER 的欄位定義、CHECK 約束或 DEFAULT（必須與 PR #29 逐字相同）。
- 不得改動 `0001_init.*`（Part 1 的產物）或 `dockerfile`、Go 程式碼。

### Schema Impact

- 變更：將既有來源/驗證 ALTER 從 `0001_add_source_verification.sql` 移到 `0002_add_source_verification.up.sql`，並補上 `0002_add_source_verification.down.sql`。
- 上游影響：既有 DB 上 `migrate up` 會在 0001 no-op 後套用 0002（新增三欄）；前提是 0002 的 DDL 未被事先手動套用（見背景上線程序）。
- 下游消費者：`internal/database/repository.go`（SELECT 已含三欄）、`internal/linebot/template/doctor.go`（顯示）。
- Migration 需求：是；`migrations/0002_add_source_verification.up.sql` 與 `migrations/0002_add_source_verification.down.sql`。

### 實作步驟

1. 以 `git mv migrations/0001_add_source_verification.sql migrations/0002_add_source_verification.up.sql` 將原檔改名（內容保持不變）。
2. 建立 `migrations/0002_add_source_verification.down.sql`，內容為：

   ```sql
   ALTER TABLE medical_personnel
       DROP COLUMN source,
       DROP COLUMN verification_status,
       DROP COLUMN source_url;
   ```

---

## Part 3：dockerfile 內建 migrate binary 與 migrations

### 現況

`dockerfile`（lowercase）為兩段式 build：builder（`golang:1.23-alpine`）以 `CGO_ENABLED=0 GOOS=linux go build` 產出 `/build/popofinder`；runtime（`gcr.io/distroless/static-debian12`）只 `COPY --from=builder /build/popofinder /popofinder`，`USER 1000`、`EXPOSE 8000`、`ENTRYPOINT ["/popofinder"]`。image 內**沒有** `migrate` binary，也**沒有** `migrations/`。

### 目標

image 同時內建 app 與 migrate runner，供 chart 的 init container 使用：

- builder stage 在 app build 之後，以 `go install` 安裝 golang-migrate `v4.18.1`（`postgres` tag），並把 binary 複製到 `/build/migrate`。
- runtime stage 額外 `COPY --from=builder /build/migrate /migrate` 與 `COPY migrations /migrations`。
- 維持既有 `GOOS=linux go build`（buildx 多平台以模擬處理 GOARCH）、`USER 1000`、`EXPOSE 8000`、`ENTRYPOINT` 不變。

### 不可動的範圍

- 不得修改 app build 那行（`go build ... -o popofinder ./cmd/server`）、`USER`、`EXPOSE`、`ENTRYPOINT`。
- 不得改用 `TARGETPLATFORM`/`TARGETARCH` 交叉編譯結構（保留 popofinder 既有 GOOS=linux + buildx 模擬作法）。
- 不得改動 `makefile` 或 `.github/workflows/ci.yaml`。

### Manifest Verification

- 結構驗證：`grep` 斷言三行關鍵指令存在（見驗收）。
- build 可行性由 CI（`make ci` → `docker buildx build`）在合併後驗證；本 task 的 gate 不執行實際 image build。

### 實作步驟

1. 在 `dockerfile` 的 builder stage、app build 行（`RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o popofinder ./cmd/server`）之後，新增一行安裝並複製 migrate binary：

   ```dockerfile
   RUN CGO_ENABLED=0 GOOS=linux go install -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@v4.18.1 \
       && find /go/bin -name migrate -type f | head -1 | xargs -I{} cp {} /build/migrate
   ```

2. 在 runtime stage 的 `COPY --from=builder /build/popofinder /popofinder` 之後，新增兩行：

   ```dockerfile
   COPY --from=builder /build/migrate /migrate
   COPY migrations /migrations
   ```

---

## 驗證計畫

| Part | 驗證方式 | 預期斷言 | 為什麼這個斷言能證明目標達成 |
|------|---------|---------|------------------------------|
| Part 1 | `test -f migrations/0001_init.up.sql && test -f migrations/0001_init.down.sql` | exit code 0 | 證明 baseline 的 up/down 成對存在，符合 golang-migrate 命名慣例 |
| Part 1 | `grep -Eq "CREATE TABLE IF NOT EXISTS medical_personnel" migrations/0001_init.up.sql` | exit code 0 | 證明 0001 以 `IF NOT EXISTS` 建立現行表，既有 DB 不會因重跑而失敗 |
| Part 1 | `grep -Eq "department TEXT" migrations/0001_init.up.sql && grep -Eq "education TEXT" migrations/0001_init.up.sql` | exit code 0 | 證明 baseline 含 nullable 的 `department`/`education`，與現行 struct 一致 |
| Part 1 | `grep -Eq "DROP TABLE IF EXISTS medical_personnel" migrations/0001_init.down.sql` | exit code 0 | 證明 baseline 有對應的反向，rollback 語意完整 |
| Part 2 | `test -f migrations/0002_add_source_verification.up.sql && test -f migrations/0002_add_source_verification.down.sql` | exit code 0 | 證明來源/驗證變更已成為版本 0002 且 up/down 成對 |
| Part 2 | `test ! -f migrations/0001_add_source_verification.sql` | exit code 0 | 反向證明舊命名檔已移除，避免與 0002 重複套用同一變更 |
| Part 2 | `grep -Eq "ADD COLUMN source TEXT" migrations/0002_add_source_verification.up.sql` | exit code 0 | 證明 0002 up 保有 `source` 欄位新增（內容自 PR #29 沿用） |
| Part 2 | `grep -q "verification_status IN ('unverified', 'self_reported', 'verified')" migrations/0002_add_source_verification.up.sql` | exit code 0 | 證明 0002 up 保留 `verification_status` 的 CHECK 約束未被竄改 |
| Part 2 | `grep -Eq "DROP COLUMN source" migrations/0002_add_source_verification.down.sql && grep -Eq "DROP COLUMN verification_status" migrations/0002_add_source_verification.down.sql && grep -Eq "DROP COLUMN source_url" migrations/0002_add_source_verification.down.sql` | exit code 0 | 證明 0002 down 完整反向移除三個欄位 |
| Part 3 | `grep -q "go install -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@v4.18.1" dockerfile` | exit code 0 | 證明 image 在 build 階段安裝指定版本的 golang-migrate |
| Part 3 | `grep -Eq "COPY --from=builder /build/migrate /migrate" dockerfile` | exit code 0 | 證明 migrate binary 被複製進 runtime image，init container 才有 `/migrate` 可執行 |
| Part 3 | `grep -Eq "COPY migrations /migrations" dockerfile` | exit code 0 | 證明 migrations 檔被複製進 image，init container 的 `-path /migrations` 才有內容 |
| 全域 | `go build ./...` | exit code 0 | 證明本 task 的 migration/dockerfile 變更未波及 Go 程式碼，專案仍可編譯 |
| 全域 | `go vet ./...` | exit code 0 | 全專案靜態檢查無誤 |
| 全域 | `go test ./...` | exit code 0 | 既有單元測試（search/session/webhook/database/template）未被破壞 |

---

## 驗收

- [ ] (Part 1) `test -f migrations/0001_init.up.sql && test -f migrations/0001_init.down.sql`
- [ ] (Part 1) `grep -Eq "CREATE TABLE IF NOT EXISTS medical_personnel" migrations/0001_init.up.sql`
- [ ] (Part 1) `grep -Eq "department TEXT" migrations/0001_init.up.sql && grep -Eq "education TEXT" migrations/0001_init.up.sql`
- [ ] (Part 1) `grep -Eq "DROP TABLE IF EXISTS medical_personnel" migrations/0001_init.down.sql`
- [ ] (Part 2) `test -f migrations/0002_add_source_verification.up.sql && test -f migrations/0002_add_source_verification.down.sql`
- [ ] (Part 2) `test ! -f migrations/0001_add_source_verification.sql`
- [ ] (Part 2) `grep -Eq "ADD COLUMN source TEXT" migrations/0002_add_source_verification.up.sql`
- [ ] (Part 2) `grep -q "verification_status IN ('unverified', 'self_reported', 'verified')" migrations/0002_add_source_verification.up.sql`
- [ ] (Part 2) `grep -Eq "DROP COLUMN source" migrations/0002_add_source_verification.down.sql && grep -Eq "DROP COLUMN verification_status" migrations/0002_add_source_verification.down.sql && grep -Eq "DROP COLUMN source_url" migrations/0002_add_source_verification.down.sql`
- [ ] (Part 3) `grep -q "go install -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@v4.18.1" dockerfile`
- [ ] (Part 3) `grep -Eq "COPY --from=builder /build/migrate /migrate" dockerfile`
- [ ] (Part 3) `grep -Eq "COPY migrations /migrations" dockerfile`
- [ ] `go build ./...`
- [ ] `go vet ./...`
- [ ] `go test ./...`

---

## 執行報告

### 摘要
| Part | 狀態 | 說明 |
|------|------|------|
| Part 1：新增 0001 baseline migration | ✅ 完成 | 新增 `0001_init.up.sql`(CREATE TABLE IF NOT EXISTS，六欄)與 `0001_init.down.sql`(DROP TABLE IF EXISTS)。 |
| Part 2：來源/驗證變更改名為 0002 | ✅ 完成 | `git mv` 原 `0001_add_source_verification.sql` → `0002_add_source_verification.up.sql`(內容不變),新增 `0002_..._down.sql` DROP 三欄。 |
| Part 3：dockerfile 內建 migrate 與 migrations | ✅ 完成 | builder stage `go install` golang-migrate v4.18.1(postgres tag)並複製 binary;runtime stage `COPY /migrate` 與 `COPY migrations /migrations`。 |

### ❌ 未通過的驗收項目
無

### 🔍 需人工確認的驗收項目
無

### 🤔 執行中的判斷決策
無

### ⚠️ 注意事項
- image 自此同時內建 app 與 golang-migrate runner(`/migrate` + `/migrations`),供 jerrytech-deploy init container 使用;實際 image build 由 CI(`make ci` → buildx)在合併後驗證,本 task gate 不跑 image build。
- 既有 DB 採 baseline 策略:`0001_init` 用 `CREATE TABLE IF NOT EXISTS` 在既有 DB 上為 no-op,再套 0002;前提是 0002 DDL 未被事先手動套用(見背景上線程序)。
