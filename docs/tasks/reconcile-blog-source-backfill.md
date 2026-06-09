# Task：重爬 blog 對帳回填來源並補充新資料

## 背景

`medical_personnel` 的 `source` / `source_url` 目前所有列皆為 NULL、`verification_status` 皆為 DEFAULT `unverified`,三欄對使用者零資訊量(見 `docs/product-direction.md` §3「擴展資料覆蓋率與正確性」)。資料母體是 blog `popolist999.blogspot.com` 的「純文字版」post —— 一個從 Google Sheets 貼上的 HTML `<table>`,每列 5 欄(縣市/醫療院所/科別/姓名/學歷),共約 1480 列(見 memory `blog-source-structure`)。本 task 建立一個重爬對帳工具:抓該 post → 解析 → 與 DB 以 `(姓名+醫院)` 為鍵對帳 → 命中的既有列標 `source='blog'`、blog 有但 DB 無的列新增進來。

**驗證範圍界定**:本 repo 無 DB 測試 harness、CI 不連網(`internal/database/repository_test.go` 僅測字串常數)。因此「抓取 + 寫 DB」屬 **I/O 外殼,只能 `go build`/`go vet` 檢查**;可驗證的核心是**純函式**(HTML 解析、對帳計畫產生),以 fixture 單元測試。**實際對 production 回填是操作者手動執行 binary(支援 `-dry-run` 預覽),不在本 task 的自動化驗證範圍內。**

### 涉及檔案

- `internal/reconcile/parse.go` — 新檔:`Record` 型別與 `ParsePost(postHTML string) []Record`(HTML table → 記錄,純函式)
- `internal/reconcile/parse_test.go` — 新檔:`ParsePost` 的 fixture 單元測試
- `internal/reconcile/testdata/sample_post.html` — 新檔:真實結構的 HTML fixture
- `internal/reconcile/plan.go` — 新檔:`ExistingRow` / `Plan` 型別與 `BuildPlan(blog []Record, existing []ExistingRow) Plan`(對帳分類,純函式)
- `internal/reconcile/plan_test.go` — 新檔:`BuildPlan` 的單元測試
- `internal/reconcile/fetch.go` — 新檔:`FetchPlaintextPost(ctx, baseURL) (postHTML, postURL string, err error)`(HTTP I/O,僅 build/vet)
- `cmd/reconcile/main.go` — 新檔:CLI 進入點,串接 fetch→parse→plan→DB(I/O,僅 build/vet)

本 task **只新增檔案,不修改任何既有檔案**。

---

## Part 1：解析 blog 純文字 post 的 HTML table

### 現況

無任何 HTML 解析程式碼。blog post 內容是單一 `<table>`,每個 `<tr>` 含 5 個 `<td>`,欄序固定為 縣市、醫療院所、科別、姓名、學歷;第一列為表頭(`縣市/醫療院所/科別/姓名/學歷`);科別欄可能為空字串;學歷欄可能含拉丁字母(如 `Poznan`);儲存格內容偶有內層 tag 包裹與 HTML entity。table 前後另有 `<p>` 說明文字。

### 目標

`ParsePost(postHTML string) []Record` 接收 post 的 HTML 字串,回傳每筆資料列對應的 `Record`,並滿足:跳過表頭列、跳過儲存格數不等於 5 的列、跳過姓名為空的列、移除儲存格內層 tag、`html.UnescapeString` 還原 entity、每欄 `strings.TrimSpace`。空科別保留為空字串。

### 不可動的範圍

無特別限制,但不得破壞現有 build/test/lint,且不得新增 `go.mod` 依賴(只用 stdlib `regexp`、`strings`、`html`)。

### 實作步驟

1. 建立 `internal/reconcile/parse.go`,package 名 `reconcile`。定義:
   ```go
   type Record struct {
       City       string
       Hospital   string
       Department string
       Name       string
       Education  string
   }
   ```
2. 在 `parse.go` 定義套件層級變數:`var trRe = regexp.MustCompile(`(?s)<tr[^>]*>(.*?)</tr>`)`、`var tdRe = regexp.MustCompile(`(?s)<td[^>]*>(.*?)</td>`)`、`var tagRe = regexp.MustCompile(`<[^>]*>`)`。
3. 實作 `func cleanCell(raw string) string`:依序執行 `tagRe.ReplaceAllString(raw, "")` 移除內層 tag、`html.UnescapeString(...)`、`strings.TrimSpace(...)`,回傳結果。
4. 實作 `func ParsePost(postHTML string) []Record`:用 `trRe.FindAllStringSubmatch` 取出所有 `<tr>` 內容;對每個 tr 用 `tdRe.FindAllStringSubmatch` 取出 td;若 td 數不等於 5 則 `continue`;對 5 個 td 各跑 `cleanCell`;若清理後 `Name`(第 4 欄)為空字串則 `continue`;若 `City`(第 1 欄)等於 `"縣市"` 或 `Name` 等於 `"姓名"`(表頭)則 `continue`;其餘組成 `Record{City, Hospital, Department, Name, Education}`(依欄序 0..4)append 進結果。回傳 slice(初始化為 `[]Record{}`,非 nil)。
5. 建立 `internal/reconcile/testdata/sample_post.html`,內容**逐字**為:
   ```html
   <p><span style="color: red;">本網站資料僅供參考</span></p>
   <p>波蘭醫師一覽表(純文字版)</p>
   <table border="1"><tbody>
   <tr><td>縣市</td><td>醫療院所</td><td>科別</td><td>姓名</td><td>學歷</td></tr>
   <tr><td>台中</td><td>大里仁愛醫院</td><td></td><td>陳柏誠</td><td>波蘭</td></tr>
   <tr><td>台中</td><td>中山附醫</td><td>內科</td><td>薛崇亨</td><td>波蘭</td></tr>
   <tr><td>台中</td><td>中山醫</td><td>婦產科</td><td>黃允瑤</td><td>Poznan</td></tr>
   <tr><td>台北</td><td>某某診所</td><td>家醫科</td><td><span>林小明</span></td><td>波蘭&amp;捷克</td></tr>
   </tbody></table>
   ```
6. 建立 `internal/reconcile/parse_test.go`,實作 `TestParsePost`:用 `os.ReadFile("testdata/sample_post.html")` 讀入 fixture,呼叫 `ParsePost`,斷言回傳**恰好 4 筆**,且等於下列(用 `reflect.DeepEqual` 或逐欄比對):
   - `{City:"台中", Hospital:"大里仁愛醫院", Department:"", Name:"陳柏誠", Education:"波蘭"}`
   - `{City:"台中", Hospital:"中山附醫", Department:"內科", Name:"薛崇亨", Education:"波蘭"}`
   - `{City:"台中", Hospital:"中山醫", Department:"婦產科", Name:"黃允瑤", Education:"Poznan"}`
   - `{City:"台北", Hospital:"某某診所", Department:"家醫科", Name:"林小明", Education:"波蘭&捷克"}`

---

## Part 2:對帳分類產生回填計畫

### 現況

無對帳邏輯。DB 既有列有些來自 blog、有些是民眾手動回報(見 memory `data-mixed-source`),`source` 全為 NULL。需要一個純函式,輸入 blog 記錄與既有列摘要,輸出「要標記為 blog 的既有列 ID」與「要新增的 blog 記錄」。

### State Enumeration

對帳鍵 `key = normalize(name) + "\x00" + normalize(hospital)`,其中 `normalize(s) = strings.Join(strings.Fields(s), "")`(去除所有前後與內部空白,適配 CJK)。

| 情境 | 條件 | 處理 |
|------|------|------|
| 既有列匹配 blog 且未分類 | blog 含此 key 且該列 `HasSource == false` | 列入 `MarkBlogIDs`(設 `source='blog'`) |
| 既有列匹配 blog 但已分類 | blog 含此 key 且該列 `HasSource == true` | 不動(不覆寫既有來源判斷) |
| 既有列不在 blog | blog 不含此 key | 不動(留 NULL,可能是民眾回報) |
| blog 有、DB 無 | 既有列不含此 key | 列入 `Inserts`(新增 `source='blog'`) |
| blog 內重複 | 同 key 在 blog 出現多次且 DB 無 | `Inserts` 只保留一筆(去重) |

### 目標

`BuildPlan(blog []Record, existing []ExistingRow) Plan` 依上表分類,且**冪等**:把回傳的 `MarkBlogIDs` 對應列視為已 `HasSource=true`、把 `Inserts` 視為已存在後,再次呼叫 `BuildPlan` 應回傳空計畫。

### 不可動的範圍

`BuildPlan` 為純函式,不得連 DB、不得做 I/O。不得修改既有列的內容欄位(city/hospital/department/name/education)語意 —— 計畫只描述來源欄位的變更與新增。

### 實作步驟

1. 建立 `internal/reconcile/plan.go`,package `reconcile`。定義:
   ```go
   type ExistingRow struct {
       ID        int
       Name      string
       Hospital  string
       HasSource bool
   }
   type Plan struct {
       MarkBlogIDs []int
       Inserts     []Record
   }
   ```
2. 實作 `func normalizeKey(name, hospital string) string`:回傳 `strings.Join(strings.Fields(name), "") + "\x00" + strings.Join(strings.Fields(hospital), "")`。
3. 實作 `func BuildPlan(blog []Record, existing []ExistingRow) Plan`:
   a. 建 `blogKeys := map[string]bool`,遍歷 `blog` 填入 `normalizeKey(r.Name, r.Hospital)`。
   b. 建 `existingKeys := map[string]bool`,遍歷 `existing` 填入 `normalizeKey(e.Name, e.Hospital)`。
   c. `MarkBlogIDs`:遍歷 `existing`,若 `blogKeys[key]` 為真且 `e.HasSource == false`,append `e.ID`。
   d. `Inserts`:遍歷 `blog`,維護一個 `seen := map[string]bool`;對每筆 `r` 計算 key,若 `!existingKeys[key]` 且 `!seen[key]`,則 append `r` 到 `Inserts` 並設 `seen[key]=true`。
   e. 回傳 `Plan{MarkBlogIDs, Inserts}`,兩個 slice 皆初始化為非 nil 空 slice。
4. 建立 `internal/reconcile/plan_test.go`,實作下列各別 test,每個用最小輸入斷言:
   - `TestBuildPlan_MarksUnclassifiedMatch`:`existing` 一列 `{ID:1, Name:"陳柏誠", Hospital:"大里仁愛醫院", HasSource:false}`,`blog` 含同名同院一筆 → `MarkBlogIDs == [1]`。
   - `TestBuildPlan_SkipsAlreadyClassified`:同上但 `HasSource:true` → `len(MarkBlogIDs) == 0`。
   - `TestBuildPlan_InsertsBlogOnly`:`existing` 空,`blog` 一筆 → `len(Inserts) == 1` 且該筆等於 blog 記錄。
   - `TestBuildPlan_LeavesDbOnlyUntouched`:`existing` 一列 `{ID:9, Name:"王大明", Hospital:"某院", HasSource:false}`,`blog` 為**不含**該人的另一筆 → `len(MarkBlogIDs) == 0` 且該 blog 筆進 `Inserts`(長度 1),既有列不出現在任何輸出。
   - `TestBuildPlan_NormalizesWhitespace`:`existing` `{ID:5, Name:" 陳柏誠 ", Hospital:"大里仁愛醫院 ", HasSource:false}`,`blog` `{Name:"陳柏誠", Hospital:"大里仁愛醫院", ...}` → `MarkBlogIDs == [5]` 且 `len(Inserts) == 0`。
   - `TestBuildPlan_DedupsBlogInserts`:`existing` 空,`blog` 含兩筆同名同院記錄 → `len(Inserts) == 1`。
   - `TestBuildPlan_Idempotent`:任一含命中與新增的輸入產生 `p1`;依 `p1.MarkBlogIDs` 把對應 `existing` 列改成 `HasSource:true`、把 `p1.Inserts` 以 `{ID, Name, Hospital, HasSource:true}` append 進 `existing`,再呼叫 `BuildPlan` 得 `p2`,斷言 `len(p2.MarkBlogIDs) == 0` 且 `len(p2.Inserts) == 0`。

---

## Part 3:HTTP 抓取與 CLI 串接(I/O 外殼)

### 現況

`cmd/` 只有 `server`。無抓取工具。blog 透過 Blogger feed JSON 提供 post:`GET {baseURL}/feeds/posts/default?alt=json&max-results=150`,回傳 `feed.entry[]`,每筆有 `title.$t`、`content.$t`(post HTML)、`link[]`(其中 `rel=="alternate"` 的 `href` 為 post 永久連結)。純文字版 post 的 title 含「純文字版」,永久連結為 `https://popolist999.blogspot.com/2021/06/google.html`。DB 連線沿用 `internal/database.Connect`,讀環境變數 `DATABASE_URL`(與 migration runner 相同慣例,見 memory `migration-runner-plan`;pgx 驅動接受 `?sslmode=disable`)。

### 目標

提供 `FetchPlaintextPost` 與 `cmd/reconcile` binary:能 `go build`/`go vet` 通過。binary 支援 `-dry-run` 旗標,串接 fetch→parse→`BuildPlan`,非 dry-run 時於單一交易內套用計畫(既有列 `UPDATE` 來源、新列 `INSERT`),並印出計數摘要。此部分不寫單元測試(I/O 性質),正確性靠操作者執行驗證。

### Schema Impact

- 寫入欄位:既有列 `source`(→`'blog'`)、`source_url`(→ post 永久連結)、`updated_at`(→`now()`);新列另含 `verification_status`(→`'unverified'`)與五個內容欄位。
- 不變更 schema、不新增 migration、不改 API 契約。
- `verification_status` 一律 `'unverified'`:blog 為眾包通報非查證資料,不可宣稱已查證(`docs/product-direction.md` §5 鐵律 3)。

### 不可動的範圍

不得修改 `internal/database`、`internal/server`、`internal/linebot`、`cmd/server` 等既有套件。不得新增 `go.mod` 依賴(用 stdlib `net/http`、`encoding/json`、`context`、`flag`、`os`、`log/slog`,DB 用既有 `github.com/jmoiron/sqlx` 與 `internal/database`)。既有列只寫來源欄位,不得覆寫 city/hospital/department/name/education。

### 實作步驟

1. 建立 `internal/reconcile/fetch.go`,package `reconcile`。實作 `func FetchPlaintextPost(ctx context.Context, baseURL string) (postHTML string, postURL string, err error)`:
   - 用 `http.NewRequestWithContext` 對 `baseURL + "/feeds/posts/default?alt=json&max-results=150"` 發 GET,`http.Client{Timeout: 30 * time.Second}` 執行,結束 `defer resp.Body.Close()`,非 200 回傳 error。
   - 定義最小解碼結構,只取需要欄位:
     ```go
     type feedResp struct {
         Feed struct {
             Entry []struct {
                 Title   struct{ T string `json:"$t"` } `json:"title"`
                 Content struct{ T string `json:"$t"` } `json:"content"`
                 Link    []struct {
                     Rel  string `json:"rel"`
                     Href string `json:"href"`
                 } `json:"link"`
             } `json:"entry"`
         } `json:"feed"`
     }
     ```
   - `json.NewDecoder(resp.Body).Decode(&fr)`;遍歷 `fr.Feed.Entry`,找 `strings.Contains(e.Title.T, "純文字版")` 的第一筆;取其 `Content.T` 為 `postHTML`,並從其 `Link` 取 `Rel=="alternate"` 的 `Href` 為 `postURL`。找不到則回傳 error。
2. 建立 `cmd/reconcile/main.go`,package `main`。`func main()`:
   - 用 `flag.Bool("dry-run", false, "只印出計畫,不寫入資料庫")` 與 `flag.String("base-url", "https://popolist999.blogspot.com", "blog base URL")`,`flag.Parse()`。
   - `ctx := context.Background()`。呼叫 `reconcile.FetchPlaintextPost(ctx, *baseURL)`,err 非 nil 則 `slog.Error` 後 `os.Exit(1)`。
   - `records := reconcile.ParsePost(postHTML)`;`slog.Info("parsed", "count", len(records))`。
   - `db, err := database.Connect(os.Getenv("DATABASE_URL"))`,err 處理同上;`defer db.Close()`。
   - 查既有列:`db.Select(&existing, "SELECT id, name, hospital, (source IS NOT NULL) AS has_source FROM medical_personnel")`,其中 `existing` 為 `[]reconcile.ExistingRow`,並在 `ExistingRow` 欄位加上 `db` tag(`db:"id"`、`db:"name"`、`db:"hospital"`、`db:"has_source"`)。
   - `plan := reconcile.BuildPlan(records, existing)`;`slog.Info("plan", "mark_blog", len(plan.MarkBlogIDs), "inserts", len(plan.Inserts))`。
   - 若 `*dryRun` 為真:`slog.Info("dry-run, no writes")` 後 `return`。
   - 否則開交易 `tx := db.MustBegin()`(或 `db.Beginx()` 並處理 err):
     - 對每個 `id := range plan.MarkBlogIDs`:`tx.Exec("UPDATE medical_personnel SET source='blog', source_url=$1, updated_at=now() WHERE id=$2", postURL, id)`。
     - 對每筆 `r := range plan.Inserts`:把空科別轉 NULL —— `var dept any; if r.Department != "" { dept = r.Department }`;`tx.Exec("INSERT INTO medical_personnel (city, hospital, department, name, education, source, verification_status, source_url) VALUES ($1,$2,$3,$4,$5,'blog','unverified',$6)", r.City, r.Hospital, dept, r.Name, r.Education, postURL)`。
     - `tx.Commit()`,err 處理。
   - `slog.Info("done", "marked", len(plan.MarkBlogIDs), "inserted", len(plan.Inserts))`。
3. 在 `internal/reconcile/plan.go` 的 `ExistingRow` 補上前述 `db` struct tag(供 `sqlx.Select` 對應)。

---

## 驗證計畫

| Part | 驗證方式 | 預期斷言 | 為什麼這個斷言能證明目標達成 |
|------|---------|---------|------------------------------|
| Part 1 | `internal/reconcile/parse_test.go:TestParsePost` | 回傳恰好 4 筆且四筆欄位完全等於預期 | 直接驗證 HTML table 的每個 `<tr>` 5 個 `<td>` 正確映射到 Record,且表頭跳過、空科別保留、內層 tag 移除、`&amp;` 還原 —— 涵蓋 Part 1 全部解析規則 |
| Part 2 | `internal/reconcile/plan_test.go:TestBuildPlan_MarksUnclassifiedMatch` | `MarkBlogIDs == [1]` | 證明 blog 命中且未分類的既有列會被標為 blog(State Enumeration 第 1 列) |
| Part 2 | `internal/reconcile/plan_test.go:TestBuildPlan_SkipsAlreadyClassified` | `len(MarkBlogIDs) == 0` | 證明已分類列不被覆寫(第 2 列,守住「不臆測回填」) |
| Part 2 | `internal/reconcile/plan_test.go:TestBuildPlan_LeavesDbOnlyUntouched` | 既有列不出現在任何輸出 | 證明 DB 有 blog 無的列保持 NULL(第 3 列,可能是民眾回報) |
| Part 2 | `internal/reconcile/plan_test.go:TestBuildPlan_InsertsBlogOnly` | `len(Inserts) == 1` 且內容相符 | 證明 blog 有 DB 無的列會被新增(第 4 列,補充新資料) |
| Part 2 | `internal/reconcile/plan_test.go:TestBuildPlan_DedupsBlogInserts` | `len(Inserts) == 1` | 證明 blog 內重複 key 只新增一筆(第 5 列) |
| Part 2 | `internal/reconcile/plan_test.go:TestBuildPlan_NormalizesWhitespace` | `MarkBlogIDs == [5]` 且 `len(Inserts) == 0` | 證明 `(姓名+醫院)` 鍵的空白正規化讓 DB 與 blog 對得起來 |
| Part 2 | `internal/reconcile/plan_test.go:TestBuildPlan_Idempotent` | 第二次 `BuildPlan` 回傳空計畫 | 證明工具可重複執行不重複寫入(R7 冪等),操作安全 |
| Part 3 | `go build ./...` | exit code 0 | I/O 外殼(HTTP+DB+CLI)無法在無網路無 DB 的 headless 環境執行,build 成功是「串接型別正確、可產出 binary」的最低保證 |
| Part 3 | `go vet ./...` | exit code 0 | 驗證新增的 fetch/cmd 程式碼無 vet 等級問題(printf、struct tag、unreachable 等),補強無單元測試的 I/O 外殼品質 |

---

## 驗收

- [ ] `go build ./...`
- [ ] `go vet ./...`
- [ ] `go test ./...`
- [ ] (Part 1) `go test ./internal/reconcile/ -run TestParsePost`
- [ ] (Part 2) `go test ./internal/reconcile/ -run TestBuildPlan`

---

## 執行報告

### 摘要
| Part | 狀態 | 說明 |
|------|------|------|
| Part 1：解析 blog HTML table | ✅ 完成 | `ParsePost` + `cleanCell` + 真實結構 fixture,`TestParsePost` 斷言 4 筆完全相符 |
| Part 2：對帳分類產生計畫 | ✅ 完成 | `BuildPlan` 純函式 + `normalizeKey`,7 個 `TestBuildPlan_*` 涵蓋 State Enumeration 全部情境與冪等性 |
| Part 3：HTTP 抓取與 CLI 串接 | ✅ 完成 | `FetchPlaintextPost` + `cmd/reconcile`(`-dry-run`/`-base-url`),`go build`/`go vet` 通過 |

### ❌ 未通過的驗收項目
無

### 🔍 需人工確認的驗收項目
無(本 task 無 【人工確認】 項目)

### 🤔 執行中的判斷決策
- **Part 3, Step 2**: 步驟給「`db.MustBegin()` 或 `db.Beginx()` 並處理 err」兩選項。選 `db.Beginx()` 並在每個 statement 失敗時 `tx.Rollback()` 後 `os.Exit(1)`。理由:這是讓「單一交易」語意成立的最小正確實作(MustBegin 會 panic,Beginx + rollback 是慣用且可控的錯誤路徑)。

### ⚠️ 注意事項
- 新增 package `internal/reconcile`(純函式核心 parse/plan + I/O 外殼 fetch)與新 binary `cmd/reconcile`。**無新增 go.mod 依賴**,全用 stdlib + 既有 sqlx。
- `cmd/reconcile` 讀 `DATABASE_URL` env(沿用 migration runner 慣例),非 server 用的 `DB_*` 拆解變數;操作者執行時需自備 `DATABASE_URL`(pgx 接受 `?sslmode=disable`)。
- 實際 production 回填為操作者手動執行(建議先 `cmd/reconcile -dry-run` 預覽計數),不在自動化驗證範圍。
