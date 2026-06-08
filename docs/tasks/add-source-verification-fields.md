# Task：為 medical_personnel 加上「資料來源 / 驗證狀態 / 學歷佐證連結」欄位並誠實標示於 LINE 卡片

## 背景

目前 `medical_personnel` 單表的每一筆資料看起來都一樣權威,但實際上來源**混合**:大部分來自「波波一覽表」blog,另有部分是維護者手動從 Google sheet(民眾回報)加入。**現有列無法逐筆得知其來源**,必須靠後續一個獨立的「重爬波波一覽表對帳」task 才能分類(在波波一覽表上找得到 → `blog`;找不到 → `public_report`)。這違反 `docs/product-direction.md` §5 鐵律 3「誠實標示驗證程度(爬來的 / 醫師自填 / 已驗證要分清楚)」。

本 task 為資料模型補上三個欄位並在 LINE 卡片誠實呈現:
- `source`(資料來源):**可為 NULL**;現有列一律留 NULL(代表「尚未分類」),不做任何臆測性回填。實際分類由後續對帳 task 填入。
- `verification_status`(驗證狀態):現有列回填 `unverified`(無論來自 blog 或民眾回報,目前都未經查證,此回填為真)。
- `source_url`(學歷佐證連結):**可為 NULL**;當某醫師學歷有公開在特定網站時,收錄該網址供病人自行查證。

這是後續所有方向(重爬對帳、眾包校對、擴展覆蓋率)的前置地基。

**非目標(不在本 task 範圍)**:
- 不重爬波波一覽表、不做來源分類(獨立 task,需待本 task 的 `source` 欄位就緒後才能對帳)。
- 不實作下架機制(`docs/product-direction.md` §5 鐵律 4,列為後續獨立 task)。
- 不新增資料寫入 / 民眾回報的 API 或介面(本 task 只動讀取路徑與顯示)。
- 不修改 `education` 欄位「未提供」的既有文案。

### 涉及檔案

- `migrations/0001_add_source_verification.sql` — 新增;本 task 唯一的 schema 變更 DDL(手動套用於 Postgres,repo 無 migration runner)。
- `internal/database/repository.go` — `MedicalPersonnel` struct 加三個欄位;SELECT 欄位列表加入新欄位。
- `internal/database/repository_test.go` — 新增;不依賴 DB,驗證 SELECT 欄位列表含新欄位。
- `internal/linebot/template/doctor.go` — 醫師卡片加入「資料來源」「驗證狀態」「學歷佐證」三列與對應 label 對照函式。
- `internal/linebot/template/doctor_test.go` — 新增;驗證卡片 JSON 含正確中性文案,且 `source_url` 為空時不顯示該列。

---

## Part 1：新增 schema migration DDL

### 現況

repo 內沒有 `migrations/` 目錄,也沒有任何 `.sql` 檔;`internal/database/db.go` 只負責連線,schema 由樹莓派上的 Postgres 外部維護。`medical_personnel` 現有欄位為:`id, city, hospital, department, name, education`。

### 目標

存在檔案 `migrations/0001_add_source_verification.sql`,內容對 `medical_personnel` 新增三欄:
- `source TEXT`(**可為 NULL,不設 DEFAULT**),且 `CHECK (source IN ('blog', 'public_report'))`(Postgres 的 CHECK 對 NULL 值通過,故現有列維持 NULL 合法)。
- `verification_status TEXT NOT NULL DEFAULT 'unverified'`,且 `CHECK (verification_status IN ('unverified', 'self_reported', 'verified'))`。
- `source_url TEXT`(**可為 NULL,不設 DEFAULT**)。

`verification_status` 的 `DEFAULT` 會使所有現有列回填為 `unverified`(正確:現有資料無論 blog 或民眾回報都未經查證)。`source` 與 `source_url` 維持 NULL,**刻意不臆測現有列來源**。

### 不可動的範圍

- 不得修改 `medical_personnel` 的既有欄位(`id, city, hospital, department, name, education`)。
- 不得對 `source` 設定 DEFAULT 或做任何 UPDATE 回填(必須維持 NULL)。
- 不得新增 migration runner、ORM migration 框架或 `db.go` 的連線邏輯。
- 不得撰寫資料寫入 / 重爬腳本。

### Schema Impact

- 變更:`medical_personnel` 新增 `source TEXT`(nullable, CHECK)、`verification_status TEXT NOT NULL DEFAULT 'unverified'`(CHECK)、`source_url TEXT`(nullable)。
- 上游影響:套用此 migration **必須先於** Part 2 的程式碼部署;否則 SELECT 取用不存在的欄位會在執行期失敗(此為運維順序,非程式邏輯)。
- 下游消費者:`internal/database/repository.go`(SELECT)、`internal/linebot/template/doctor.go`(顯示)。
- Migration 需求:是;檔案路徑 `migrations/0001_add_source_verification.sql`。

### 實作步驟

1. 建立目錄 `migrations/`。
2. 建立檔案 `migrations/0001_add_source_verification.sql`,寫入單一 `ALTER TABLE` 敘述,依序新增三欄,內容如下:

   ```sql
   ALTER TABLE medical_personnel
       ADD COLUMN source TEXT
           CHECK (source IN ('blog', 'public_report')),
       ADD COLUMN verification_status TEXT NOT NULL DEFAULT 'unverified'
           CHECK (verification_status IN ('unverified', 'self_reported', 'verified')),
       ADD COLUMN source_url TEXT;
   ```

---

## Part 2：repository struct 與 SELECT 加入新欄位

### 現況

`internal/database/repository.go` 的 `MedicalPersonnel` struct(第 12–19 行)欄位為 `ID, City, Hospital, Department, Name, Education`,其中 `Department` 與 `Education` 已使用 `sql.NullString`。`Search` 方法(第 76–79 行)以行內字串 `"SELECT id, city, hospital, department, name, education FROM medical_personnel %s LIMIT 10 OFFSET $%d"` 組出資料查詢。`MedicalPersonnel` 的消費者僅有 `Search`(產生)、`internal/linebot/template/doctor.go`、`internal/linebot/handler/message.go:70 buildSearchResponse`(原樣傳遞),且無任何 positional struct literal,新增欄位不會破壞既有建構。

### 目標

- `MedicalPersonnel` struct 多出三個欄位:`Source sql.NullString` 帶 `db:"source"`、`VerificationStatus string` 帶 `db:"verification_status"`、`SourceURL sql.NullString` 帶 `db:"source_url"`。
- SELECT 的欄位列表抽成 package-level 常數 `selectColumns`,值為 `"id, city, hospital, department, name, education, source, verification_status, source_url"`,並在 `Search` 的資料查詢使用該常數,使欄位列表可在無 DB 環境下被單元測試。

### 不可動的範圍

- 不得修改 `Search` 的 WHERE 組裝邏輯、分頁(`LIMIT 10 OFFSET`)、count 查詢或 `SearchStats` 計算。
- 不得修改 `handler/message.go` 與 `template/doctor.go` 以外對 `MedicalPersonnel` 的傳遞行為(本 Part 只改 struct 與 SELECT)。

### 實作步驟

1. 在 `MedicalPersonnel` struct 末尾(`Education` 欄位之後)新增三個欄位:
   ```go
   Source             sql.NullString `db:"source"`
   VerificationStatus string         `db:"verification_status"`
   SourceURL          sql.NullString `db:"source_url"`
   ```
2. 在 `repository.go` 的 import 區之後、`MedicalPersonnel` 定義之前,新增 package-level 常數:
   ```go
   const selectColumns = "id, city, hospital, department, name, education, source, verification_status, source_url"
   ```
3. 將 `Search` 中組出 `dataSQL` 的那行(第 76–79 行)改為使用 `selectColumns`:
   ```go
   dataSQL := fmt.Sprintf(
       "SELECT "+selectColumns+" FROM medical_personnel %s LIMIT 10 OFFSET $%d",
       where, argIdx,
   )
   ```
4. 建立 `internal/database/repository_test.go`,package 為 `database`,新增測試 `TestSelectColumnsIncludeNewFields`:分別斷言 `strings.Contains(selectColumns, "source")`、`strings.Contains(selectColumns, "verification_status")`、`strings.Contains(selectColumns, "source_url")` 為真,否則 `t.Fatalf`。此測試不連線 DB。

---

## Part 3：LINE 醫師卡片顯示「資料來源 / 驗證狀態 / 學歷佐證」

### 現況

`internal/linebot/template/doctor.go` 的 `doctorBubble`(第 25–87 行)用區域函式 `row(label, value, extra)` 產生水平列,body 的 `contents` 為 `[]any{cityRow, hospitalRow, deptRow, eduRow}`(第 83 行)。卡片目前不顯示任何資料來源、驗證程度或佐證連結資訊。`row` 產生的 label 節點為 `size:"sm" color:"#666666" flex:2`,value 節點為 `size:"sm" color:"#4A4A4A" flex:4`。

### 目標

醫師卡片在「學歷」列之後,新增「資料來源」「驗證狀態」兩列(一律顯示),並在有佐證連結時額外新增「學歷佐證」一列(`source_url` 為 NULL 或空字串時不顯示該列)。文字以中性、誠實的中文呈現,沿用既有 `row` 函式的視覺樣式。原始代碼值透過對照函式轉成顯示文字。

### 不可動的範圍

- 不得修改既有 `cityRow / hospitalRow / deptRow / eduRow` 的內容或樣式。
- 不得修改 `education` 的「未提供」文案(`strOrDefault` / `strVal` 不動)。
- 不得修改 `DoctorsFlexMessage`、`NextPageFlexMessage`、`buildFlexMessage` 的函式簽章與 carousel / no-result 行為。

### 視覺規格

| 元素 | 使用的 Token / 屬性 | 參考來源 |
|------|---------------------|---------|
| 「資料來源」「驗證狀態」「學歷佐證」label 節點 | `size:"sm"`、`color:"#666666"`、`flex:2` | `doctorBubble` 內 `row()` 的 labelNode |
| 對應 value 節點 | `size:"sm"`、`color:"#4A4A4A"`、`flex:4` | `doctorBubble` 內 `row()` 的 valueNode |
| 各新列與上一列的間距 | `margin:"md"` | 同 `deptRow` / `eduRow` 既有設定 |
| 「學歷佐證」value(URL)自動折行 | `wrap: true`(透過 `row` 的 `extra` 參數傳入) | 同 `eduRow` 的 `wrap` 用法 |

### State Enumeration

`source` 代碼 → 顯示文字(`sourceLabel`,參數型別 `sql.NullString`):

| `p.Source` | 顯示文字 |
|------------|---------|
| `Valid` 且 `String == "blog"` | `網路公開彙整` |
| `Valid` 且 `String == "public_report"` | `民眾回報` |
| 其他(NULL / 空 / 未知值) | `尚未分類` |

`verification_status` 代碼 → 顯示文字(`verificationLabel`,參數型別 `string`):

| `p.VerificationStatus` | 顯示文字 |
|------------------------|---------|
| `"verified"` | `已查證` |
| `"self_reported"` | `醫師本人提供` |
| `"unverified"` | `尚未查證` |
| 其他 / 空字串 | `尚未查證` |

「學歷佐證」列的條件渲染:

| 條件 | 渲染結果 |
|------|---------|
| `p.SourceURL.Valid && p.SourceURL.String != ""` | 顯示「學歷佐證」列,value 為該 URL 字串 |
| 否則 | **不**加入該列 |

### 實作步驟

1. 在 `doctor.go` 新增函式 `sourceLabel(ns sql.NullString) string`:以 `switch` 依上方 State Enumeration 表回傳對應中文字串;`ns.Valid` 為假或值不在表內時回傳 `"尚未分類"`。
2. 在 `doctor.go` 新增函式 `verificationLabel(s string) string`:以 `switch s` 依上方 State Enumeration 表回傳對應中文字串(含 `default` 回傳 `"尚未查證"`)。
3. 在 `doctorBubble` 內,於 `eduRow`(第 61 行)之後新增「資料來源」「驗證狀態」兩列:
   ```go
   sourceRow := row("資料來源", sourceLabel(p.Source), nil)
   sourceRow["margin"] = "md"

   verifyRow := row("驗證狀態", verificationLabel(p.VerificationStatus), nil)
   verifyRow["margin"] = "md"
   ```
4. 在 `doctorBubble` 內組出 body `contents` 之前,先以 `[]any{cityRow, hospitalRow, deptRow, eduRow, sourceRow, verifyRow}` 建立切片變數 `bodyContents`;接著當 `p.SourceURL.Valid && p.SourceURL.String != ""` 時,建立佐證列並 append:
   ```go
   if p.SourceURL.Valid && p.SourceURL.String != "" {
       urlRow := row("學歷佐證", p.SourceURL.String, map[string]any{"wrap": true})
       urlRow["margin"] = "md"
       bodyContents = append(bodyContents, urlRow)
   }
   ```
5. 將 body 的 `contents`(第 83 行)由 `[]any{cityRow, hospitalRow, deptRow, eduRow}` 改為 `bodyContents`。
6. 建立 `internal/linebot/template/doctor_test.go`,package 為 `template`,新增兩個測試:
   - `TestDoctorBubbleShowsSourceVerificationAndURL`:以 `DoctorsFlexMessage([]database.MedicalPersonnel{{Name: "測試醫師", City: "台北市", Hospital: "測試醫院", Source: sql.NullString{String: "public_report", Valid: true}, VerificationStatus: "verified", SourceURL: sql.NullString{String: "https://example.com/doc", Valid: true}}})` 取得 `json.RawMessage`,轉為 string,分別斷言含子字串 `"資料來源"`、`"民眾回報"`、`"驗證狀態"`、`"已查證"`、`"學歷佐證"`、`"https://example.com/doc"`;任一不含則 `t.Fatalf`。
   - `TestDoctorBubbleOmitsSourceURLWhenEmpty`:以 `DoctorsFlexMessage([]database.MedicalPersonnel{{Name: "測試醫師", City: "台北市", Hospital: "測試醫院", Source: sql.NullString{String: "blog", Valid: true}, VerificationStatus: "unverified"}})`(`SourceURL` 留零值)取得 string,斷言**不含**子字串 `"學歷佐證"`,且含 `"網路公開彙整"`、`"尚未查證"`;違反則 `t.Fatalf`。

---

## 驗證計畫

| Part | 驗證方式 | 預期斷言 | 為什麼這個斷言能證明目標達成 |
|------|---------|---------|------------------------------|
| Part 1 | `grep -Eq "ADD COLUMN source TEXT" migrations/0001_add_source_verification.sql` | exit code 0 | 證明 `source` 欄位被加入且(無 DEFAULT/NOT NULL)維持 nullable,符合「不臆測現有列來源」目標 |
| Part 1 | `grep -q "source IN ('blog', 'public_report')" migrations/0001_add_source_verification.sql` | exit code 0 | 證明 `source` 值受 CHECK 約束限制於合法集合 |
| Part 1 | `grep -Eq "ADD COLUMN verification_status TEXT NOT NULL DEFAULT 'unverified'" migrations/0001_add_source_verification.sql` | exit code 0 | 證明 `verification_status` 以 NOT NULL + 預設回填 `unverified`,正確反映現有資料未查證 |
| Part 1 | `grep -q "verification_status IN ('unverified', 'self_reported', 'verified')" migrations/0001_add_source_verification.sql` | exit code 0 | 證明 `verification_status` 值受 CHECK 約束限制於合法集合 |
| Part 1 | `grep -Eq "ADD COLUMN source_url TEXT" migrations/0001_add_source_verification.sql` | exit code 0 | 證明 `source_url` 佐證連結欄位被加入且 nullable |
| Part 1 | `! grep -Eq "DEFAULT 'blog'\|UPDATE +medical_personnel" migrations/0001_add_source_verification.sql` | exit code 0 | 反向證明**沒有**把現有列臆測回填為 blog,守住鐵律 3 |
| Part 2 | `go test ./internal/database/ -run TestSelectColumnsIncludeNewFields` | test passes | `selectColumns` 含三個新欄位,即 SELECT 會撈出新欄位(無需 DB) |
| Part 2 | `go build ./...` | exit code 0 | struct 新增欄位與常數抽取後全專案仍可編譯,證明型別/語法正確 |
| Part 3 | `go test ./internal/linebot/template/ -run TestDoctorBubbleShowsSourceVerificationAndURL` | test passes | 卡片 JSON 含「資料來源/民眾回報/驗證狀態/已查證/學歷佐證/URL」,對應 Part 3 顯示目標 |
| Part 3 | `go test ./internal/linebot/template/ -run TestDoctorBubbleOmitsSourceURLWhenEmpty` | test passes | 佐證連結為空時不出現「學歷佐證」列,直接驗證條件渲染分支 |
| 全域 | `go vet ./...` | exit code 0 | 全專案靜態檢查無誤,確保改動未引入可疑用法 |
| 全域 | `go test ./...` | exit code 0 | 既有測試(search/session/webhook)未被破壞 |

---

## 驗收

- [ ] `grep -Eq "ADD COLUMN source TEXT" migrations/0001_add_source_verification.sql`
- [ ] `grep -q "source IN ('blog', 'public_report')" migrations/0001_add_source_verification.sql`
- [ ] `grep -Eq "ADD COLUMN verification_status TEXT NOT NULL DEFAULT 'unverified'" migrations/0001_add_source_verification.sql`
- [ ] `grep -q "verification_status IN ('unverified', 'self_reported', 'verified')" migrations/0001_add_source_verification.sql`
- [ ] `grep -Eq "ADD COLUMN source_url TEXT" migrations/0001_add_source_verification.sql`
- [ ] `! grep -Eq "DEFAULT 'blog'|UPDATE +medical_personnel" migrations/0001_add_source_verification.sql`
- [ ] `go build ./...`
- [ ] `go vet ./...`
- [ ] `go test ./...`
- [ ] (Part 2) `go test ./internal/database/ -run TestSelectColumnsIncludeNewFields`
- [ ] (Part 3) `go test ./internal/linebot/template/ -run TestDoctorBubbleShowsSourceVerificationAndURL`
- [ ] (Part 3) `go test ./internal/linebot/template/ -run TestDoctorBubbleOmitsSourceURLWhenEmpty`

---

## 執行報告

### 摘要
| Part | 狀態 | 說明 |
|------|------|------|
| Part 1：新增 schema migration DDL | ✅ 完成 | 建立 `migrations/0001_add_source_verification.sql`,以單一 ALTER TABLE 新增 source(nullable+CHECK)、verification_status(NOT NULL DEFAULT 'unverified'+CHECK)、source_url(nullable)。 |
| Part 2：repository struct 與 SELECT 加入新欄位 | ✅ 完成 | struct 加三欄,SELECT 欄位列表抽為 `selectColumns` 常數並由 `Search` 使用;新增不連 DB 的 `repository_test.go`。 |
| Part 3：LINE 卡片顯示來源/驗證/佐證 | ✅ 完成 | 新增 `sourceLabel`/`verificationLabel` 對照函式與三列(佐證列條件渲染);新增 `doctor_test.go` 兩個測試。 |

### ❌ 未通過的驗收項目
無

### 🔍 需人工確認的驗收項目
無(無 【人工確認】 項目)

### 🤔 執行中的判斷決策
無

### ⚠️ 注意事項
- 運維順序:`migrations/0001_add_source_verification.sql` 必須先於 Part 2/3 程式碼部署套用(repo 無 migration runner,手動套用於樹莓派 Postgres);否則 SELECT 取用不存在欄位會在執行期失敗。
- `selectColumns` 為新的 package-level 常數,後續若再加欄位只需改此處。
