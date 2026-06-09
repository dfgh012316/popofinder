# Task：對齊 0001 baseline migration 與樹莓派 production 的 medical_personnel 表結構

## 背景

`migrations/0001_init.up.sql`(golang-migrate baseline)目前以 `TEXT` 宣告六個欄位、且缺少 `created_at` / `updated_at` 兩欄與 `city` 索引。但樹莓派 production 上手動建立的 `medical_personnel` 實際是 `VARCHAR(長度)` 欄位、含 `created_at`(`DEFAULT now()`)/`updated_at` 兩個 timestamp 欄,並有 `ix_medical_personnel_city` 索引。在 production 上因 `CREATE TABLE IF NOT EXISTS` 為 no-op,此落差不影響現況;但**全新/空 DB 跑 migration 會建出與 production 不同的表結構**(型別、欄位、索引皆異)。本 task 改寫 baseline,使全新 DB 重現 production 的表結構。

production 實際結構(來自 `\d medical_personnel`,本 task 的對齊目標,其中 `source` / `verification_status` / `source_url` 屬 0002 不在 baseline 範圍):

| 欄位 | 型別 | Nullable | Default |
|------|------|----------|---------|
| id | integer (serial) | NOT NULL | nextval 序列 |
| city | character varying(50) | NOT NULL | — |
| hospital | character varying(200) | NOT NULL | — |
| department | character varying(100) | NULL | — |
| name | character varying(100) | NOT NULL | — |
| education | character varying(500) | NULL | — |
| created_at | timestamp with time zone | NULL | now() |
| updated_at | timestamp with time zone | NULL | — |

索引:`ix_medical_personnel_city` btree(city)。

### 涉及檔案

- `migrations/0001_init.up.sql` — 修改;改寫為與 production 一致的欄位型別/長度,並補上 `created_at` / `updated_at` 與 `city` 索引。

---

## Part 1：改寫 0001 baseline 使其等同 production 表結構

### 現況

`migrations/0001_init.up.sql` 內容為:

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

六欄皆 `TEXT`,無 `created_at` / `updated_at`,無任何索引。與 production 的 `VARCHAR(長度)` + 兩個 timestamp 欄 + `ix_medical_personnel_city` 不一致。

### 目標

`migrations/0001_init.up.sql` 建立的表(在全新 DB 上)與 production 的 `medical_personnel`(扣除 0002 的三欄)完全一致:六欄改為對應長度的 `VARCHAR`、`id` 維持 `SERIAL PRIMARY KEY`、新增 `created_at TIMESTAMPTZ DEFAULT now()` 與 `updated_at TIMESTAMPTZ`、新增 `ix_medical_personnel_city` 索引;保留 `CREATE TABLE IF NOT EXISTS`(在既有 DB 仍為 no-op)。

### 不可動的範圍

- 不得在 `0001_init.up.sql` 加入 `source` / `verification_status` / `source_url`(那是 0002 的職責)。
- 不得修改 `migrations/0001_init.down.sql`(`DROP TABLE IF EXISTS medical_personnel;` 仍正確)、`migrations/0002_add_source_verification.up.sql`、`migrations/0002_add_source_verification.down.sql`、`dockerfile`、`internal/` 下任何 Go 程式碼。
- 不得移除 `CREATE TABLE IF NOT EXISTS` 的 `IF NOT EXISTS`(維持對既有 DB 的 idempotency)。

### Schema Impact

- 變更:`0001_init.up.sql` 六欄型別由 `TEXT` 改為 production 的 `VARCHAR(50/200/100/100/500)`,新增 `created_at TIMESTAMPTZ DEFAULT now()`、`updated_at TIMESTAMPTZ`,新增 `ix_medical_personnel_city` 索引。
- 上游影響:無。production 已存在該表,`CREATE TABLE IF NOT EXISTS` 在 production 為 no-op;`schema_migrations` 已在 version 2,migrate 不會重跑 0001。僅影響全新/空 DB 的建表結果。
- 下游消費者:golang-migrate runner(全新 DB 的建表)。Go 程式碼(`internal/database/repository.go`)讀取的欄位集合不變,不受影響。
- Migration 需求:是;改寫既有 `migrations/0001_init.up.sql`(不新增檔案)。

### 實作步驟

1. 將 `migrations/0001_init.up.sql` 全檔內容覆寫為以下內容(逐字):

   ```sql
   CREATE TABLE IF NOT EXISTS medical_personnel (
       id SERIAL PRIMARY KEY,
       city VARCHAR(50) NOT NULL,
       hospital VARCHAR(200) NOT NULL,
       department VARCHAR(100),
       name VARCHAR(100) NOT NULL,
       education VARCHAR(500),
       created_at TIMESTAMPTZ DEFAULT now(),
       updated_at TIMESTAMPTZ
   );

   CREATE INDEX IF NOT EXISTS ix_medical_personnel_city ON medical_personnel (city);
   ```

---

## 驗證計畫

| Part | 驗證方式 | 預期斷言 | 為什麼這個斷言能證明目標達成 |
|------|---------|---------|------------------------------|
| Part 1 | `bash -c 'set -e; CID=$(docker run -d -e POSTGRES_PASSWORD=pw postgres:16-alpine); trap "docker rm -f $CID >/dev/null 2>&1" EXIT; for i in $(seq 1 30); do docker exec $CID pg_isready -U postgres >/dev/null 2>&1 && break; sleep 1; done; docker cp migrations/0001_init.up.sql $CID:/up.sql; docker exec $CID psql -U postgres -v ON_ERROR_STOP=1 -q -f /up.sql'` | exit code 0 | 把改寫後的 baseline 套進全新 Postgres 且 `ON_ERROR_STOP=1` 無誤,證明 DDL 合法、可在空 DB 完整建出表與索引(不只是文字正確) |
| Part 1 | `grep -Eq "city VARCHAR\(50\) NOT NULL" migrations/0001_init.up.sql` | exit code 0 | `city` 改為 `VARCHAR(50) NOT NULL`,對齊 production `character varying(50)` NOT NULL |
| Part 1 | `grep -Eq "hospital VARCHAR\(200\) NOT NULL" migrations/0001_init.up.sql` | exit code 0 | `hospital` 對齊 production `character varying(200)` NOT NULL |
| Part 1 | `grep -Eq "department VARCHAR\(100\)" migrations/0001_init.up.sql` | exit code 0 | `department` 對齊 production `character varying(100)`(nullable) |
| Part 1 | `grep -Eq "name VARCHAR\(100\) NOT NULL" migrations/0001_init.up.sql` | exit code 0 | `name` 對齊 production `character varying(100)` NOT NULL |
| Part 1 | `grep -Eq "education VARCHAR\(500\)" migrations/0001_init.up.sql` | exit code 0 | `education` 對齊 production `character varying(500)`(nullable) |
| Part 1 | `grep -Eq "created_at TIMESTAMPTZ DEFAULT now\(\)" migrations/0001_init.up.sql` | exit code 0 | 補上 production 缺漏的 `created_at`(`timestamp with time zone` DEFAULT now()) |
| Part 1 | `grep -Eq "updated_at TIMESTAMPTZ" migrations/0001_init.up.sql` | exit code 0 | 補上 production 缺漏的 `updated_at`(`timestamp with time zone`) |
| Part 1 | `grep -Eq "CREATE INDEX IF NOT EXISTS ix_medical_personnel_city ON medical_personnel \(city\)" migrations/0001_init.up.sql` | exit code 0 | 補上 production 的 `ix_medical_personnel_city` 索引,且 `IF NOT EXISTS` 對既有 DB 安全 |
| Part 1 | `grep -Eq "CREATE TABLE IF NOT EXISTS medical_personnel" migrations/0001_init.up.sql` | exit code 0 | 保留 `IF NOT EXISTS`,維持既有 DB 上的 idempotency(0001 no-op) |
| Part 1 | `bash -c '! grep -qiE "source|verification_status|source_url" migrations/0001_init.up.sql'` | exit code 0 | 反向證明 0002 的三欄未被誤併入 baseline,維持 0001/0002 職責分離 |
| 全域 | `go build ./...` | exit code 0 | 證明本 task 的 SQL-only 變更未波及 Go 程式碼,專案仍可編譯 |
| 全域 | `go test ./...` | exit code 0 | 既有單元測試(search/session/webhook/database/template)未被破壞 |

---

## 驗收

- [ ] (Part 1) `bash -c 'set -e; CID=$(docker run -d -e POSTGRES_PASSWORD=pw postgres:16-alpine); trap "docker rm -f $CID >/dev/null 2>&1" EXIT; for i in $(seq 1 30); do docker exec $CID pg_isready -U postgres >/dev/null 2>&1 && break; sleep 1; done; docker cp migrations/0001_init.up.sql $CID:/up.sql; docker exec $CID psql -U postgres -v ON_ERROR_STOP=1 -q -f /up.sql'`
- [ ] (Part 1) `grep -Eq "city VARCHAR\(50\) NOT NULL" migrations/0001_init.up.sql`
- [ ] (Part 1) `grep -Eq "hospital VARCHAR\(200\) NOT NULL" migrations/0001_init.up.sql`
- [ ] (Part 1) `grep -Eq "department VARCHAR\(100\)" migrations/0001_init.up.sql`
- [ ] (Part 1) `grep -Eq "name VARCHAR\(100\) NOT NULL" migrations/0001_init.up.sql`
- [ ] (Part 1) `grep -Eq "education VARCHAR\(500\)" migrations/0001_init.up.sql`
- [ ] (Part 1) `grep -Eq "created_at TIMESTAMPTZ DEFAULT now\(\)" migrations/0001_init.up.sql`
- [ ] (Part 1) `grep -Eq "updated_at TIMESTAMPTZ" migrations/0001_init.up.sql`
- [ ] (Part 1) `grep -Eq "CREATE INDEX IF NOT EXISTS ix_medical_personnel_city ON medical_personnel \(city\)" migrations/0001_init.up.sql`
- [ ] (Part 1) `grep -Eq "CREATE TABLE IF NOT EXISTS medical_personnel" migrations/0001_init.up.sql`
- [ ] (Part 1) `bash -c '! grep -qiE "source|verification_status|source_url" migrations/0001_init.up.sql'`
- [ ] `go build ./...`
- [ ] `go test ./...`

---

## 執行報告

### 摘要
| Part | 狀態 | 說明 |
|------|------|------|
| Part 1：改寫 0001 baseline 使其等同 production 表結構 | ✅ 完成 | 六欄改為對應長度 VARCHAR、新增 created_at/updated_at、補 ix_medical_personnel_city 索引,保留 CREATE TABLE IF NOT EXISTS。 |

### ❌ 未通過的驗收項目
無

### 🔍 需人工確認的驗收項目
無

### 🤔 執行中的判斷決策
無

### ⚠️ 注意事項
- SQL-only 變更,僅影響全新/空 DB 的建表結果;production 因 `CREATE TABLE IF NOT EXISTS` 為 no-op,不受影響。未動 Go 程式碼、down migration、0002 migration。
