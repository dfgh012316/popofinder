# Task：官網爬蟲框架＋亞東紀念醫院

## 背景

OD-7 決定主動爬醫院／診所官網的醫師介紹頁，國內外畢業的醫師都收。第一批是亞東、雙和、長庚、馬偕、國泰。使用者 2026-10-12 決定先做**爬蟲框架＋亞東一家**，上線跑過一輪正式資料後，再加另外 4 家。

本 task 建好以下幾樣：
- **比對**：每位醫師有一個穩定鍵 `source_key`，重爬時靠它找到同一位醫師。
- **增刪改的規劃**：純函式算出要新增、更新、刪除的醫師，並帶刪除保護。
- **下架排除清單**：被下架的醫師，爬蟲之後不會再加回來。
- **抓取器**：可辨識的 UA、遵守 robots.txt、每次請求有間隔。
- **亞東 parser**。
- **`cmd/crawl` 與 image**。

其他使用者決定：
- 學歷寫「中醫學系」的中醫師也收。
- 每月重爬時更新「查到日期」，但只改日期的更新不記歷史。

亞東的頁面結構見 `docs/sources/medical-center-inventory.md` 第 2 節。2026-10-12 實測的結構如下：
- 總索引頁 `/section/section1.aspx` 有 54 個科別連結 `sectionDetail.aspx?CID=<碼>`。
- 科別頁的每位醫師是一張卡片，連到 `sectionDetail2.aspx?CID=<碼>&&DoctorID=<id>`（單引號屬性、雙 `&&`）。
- 個人頁的 `<title>` 是「姓名醫師｜科別｜亞東紀念醫院」，學歷在 `<div class="h5 color-g name">學歷</div><ul><li>…</li></ul>`。

### 涉及檔案

- `go.mod`、`go.sum` — 新增依賴 `github.com/temoto/robotstxt v1.1.2`
- `migrations/0007_add_crawl_keys.up.sql`、`migrations/0007_add_crawl_keys.down.sql` — `source_key`、`takedown_suppressions`、trigger 與 `takedown_personnel` 改版
- `internal/database/repository_integration_test.go` — `openIntegrationDB` 同時清空 `takedown_suppressions`
- `internal/database/history_integration_test.go` — 新增 3 個測試（只改日期不記歷史、下架記排除清單、`source_key` 唯一）
- `internal/crawl/crawl.go` — 套件說明、`Doctor`、`Source`
- `internal/crawl/fetcher.go`、`internal/crawl/fetcher_test.go` — 抓取器
- `internal/crawl/education.go`、`internal/crawl/education_test.go` — 學歷規則
- `internal/crawl/plan.go`、`internal/crawl/plan_test.go` — `BuildSyncPlan`
- `internal/crawl/femh.go`、`internal/crawl/femh_test.go`、`internal/crawl/femh_live_test.go` — 亞東 parser
- `internal/crawl/testdata/femh/index.html`、`dept_0401.html`、`dept_0203.html`、`profile_10001.html`、`profile_10002.html`、`profile_10003.html` — 合成 fixture（虛構姓名）
- `internal/database/crawl.go`、`internal/database/crawl_integration_test.go` — `LoadCrawlState`、`ApplyCrawlPlan`
- `cmd/crawl/main.go` — CLI
- `dockerfile` — image 加入 `/crawl`
- `docs/open-decisions.md` — 追加 OD-10；OD-8、OD-9 各補一行
- `CLAUDE.md` — 架構與指令段
- `docs/tasks/crawler-framework-femh.md` — 本 task 文件

### 本 task 不做（依據）

- **雙和、長庚、馬偕、國泰**：使用者 2026-10-12 決定框架＋亞東先上線，另開 task。
- **jerrytech-deploy 的 CronJob**：跨 repo、而且要先用 `-dry-run` 看過正式數字，所以上線時另外處理，不在本 task。
- **把真實頁面存成 fixture**：真實頁面含醫師姓名，repo 是 public，git 歷史無法下架（OD-7）。fixture 只用虛構姓名的合成 HTML，結構正確性由 `-tags live` 測試對真實網站驗證。
- **Server 端顯示 `source_key`**：卡片不需要它，`selectColumns` 不改。

### Schema Impact

- **新欄位**：`medical_personnel.source_key TEXT NULL`，加 partial unique index `ux_medical_personnel_source_key`（`WHERE source_key IS NOT NULL`）。既有部落格資料保持 NULL。
- **新表**：`takedown_suppressions(source_key TEXT PRIMARY KEY, created_at TIMESTAMPTZ NOT NULL DEFAULT now())`。只存鍵，不存姓名。
- **改 trigger function** `record_medical_personnel_history()`：UPDATE 時，如果去掉 `source_checked_at`、`updated_at` 之後前後相同，就不記歷史。這涵蓋原本「內容沒變不記」的情況，`TestHistorySkipsNoopUpdate` 仍然要通過。
- **改** `takedown_personnel(p_id)`：被刪的列有 `source_key` 時，寫入 `takedown_suppressions`（`ON CONFLICT DO NOTHING`）。
- **下游消費者**：
  - Server：`database.Search` 與 `selectColumns` 不讀新欄位，不受影響。
  - `cmd/reconcile`：不寫 `source_key`（維持 NULL），CronJob 已停用。
  - 寫入 `source_key` 的只有新的 `Repository.ApplyCrawlPlan`。
  - trigger 改版影響所有 UPDATE 路徑（手動 SQL、`ApplyCrawlPlan`），規則對所有路徑一致。
- **Migration**：是，0007 up/down。down 要把兩個 function 還原成 0005／0006 的原文。`makefile` 已經會驗證最新一支 down，不必改。

### State Enumeration（`BuildSyncPlan`，每個來源各算一次）

| 狀況 | 條件 | 結果 |
|---|---|---|
| 新醫師 | 有醫學學位、不在排除清單、DB 沒有這個 `source_key` | 加入 `Inserts` |
| 內容有變 | DB 有這個 key，姓名／縣市／醫院／科別／學歷／連結任一不同 | 加入 `Updates`（全欄位＋查到日期），會記歷史 |
| 內容沒變 | DB 有這個 key，上述欄位都相同 | 加入 `TouchIDs`（只更新查到日期），不記歷史 |
| 官網上消失 | DB 有，這次沒抓到 | 加入 `DeleteIDs`（歷史保留 `old_row`） |
| 不再寫醫學學位 | DB 有，這次抓到但沒有醫學學位 | 加入 `DeleteIDs` |
| 在排除清單 | key 在 `takedown_suppressions` | 不加入任何清單，`SkippedSuppressed`+1 |
| 沒有醫學學位的新醫師 | DB 沒有、沒有醫學學位 | 不加入任何清單，`SkippedNoDegree`+1（不存任何資料） |
| 數量驟降 | DB 現有數 > 0，而且「符合條件數 × 2 < DB 現有數」 | `DeleteIDs` 清空、`DeleteBlocked = true`；其他清單照算；`cmd/crawl` 結束時 exit 1 |
| 抓取失敗 | `Source.Crawl` 回傳 error（任何一頁失敗、索引為空、個人頁沒有姓名） | 這個來源完全不寫入；`cmd/crawl` 結束時 exit 1 |

---

## Part 1：migration 0007 與整合測試

### 現況

- `migrations/` 有 0001–0006。0005 建了 `record_medical_personnel_history()`（UPDATE 時 `IF OLD IS NOT DISTINCT FROM NEW THEN RETURN NULL`）；0006 建了 `takedown_personnel(p_id INTEGER)`。
- `internal/database/repository_integration_test.go` 的 `openIntegrationDB` 執行 `TRUNCATE medical_personnel, medical_personnel_history RESTART IDENTITY`。

### 目標

`source_key` 欄位、唯一索引、`takedown_suppressions` 表都存在；只改查到日期的 UPDATE 不記歷史；下架官網醫師時會留下排除鍵；0007 down 能完整還原。

### 不可動的範圍

- `migrations/0001`–`0006` 的所有檔案。
- 既有整合測試函式的內容（只改 `openIntegrationDB` 的 TRUNCATE 與註解）。

### 實作步驟

1. 建立 `migrations/0007_add_crawl_keys.up.sql`，內容完全如下：

   ```sql
   -- 官網爬蟲(docs/open-decisions.md OD-7、OD-10):每位醫師在各來源的穩定鍵,以及下架排除清單。
   ALTER TABLE medical_personnel
       ADD COLUMN source_key TEXT;

   CREATE UNIQUE INDEX ux_medical_personnel_source_key
       ON medical_personnel (source_key) WHERE source_key IS NOT NULL;

   -- 被下架的官網醫師:爬蟲重爬時跳過,避免又加回來(OD-9、OD-10)。只存來源鍵,不存姓名等內容。
   CREATE TABLE takedown_suppressions (
       source_key TEXT PRIMARY KEY,
       created_at TIMESTAMPTZ NOT NULL DEFAULT now()
   );

   -- 只有 source_checked_at / updated_at 改變的 UPDATE(每月重爬更新查到日期)不記歷史(OD-10)。
   CREATE OR REPLACE FUNCTION record_medical_personnel_history() RETURNS trigger
   LANGUAGE plpgsql AS $$
   DECLARE
       -- SET LOCAL 的交易結束後,同一條連線讀到的是空字串而不是 NULL,所以要 NULLIF。
       change_reason TEXT := NULLIF(current_setting('popo.change_reason', true), '');
   BEGIN
       IF TG_OP = 'INSERT' THEN
           INSERT INTO medical_personnel_history (personnel_id, op, new_row, reason)
           VALUES (NEW.id, TG_OP, to_jsonb(NEW), change_reason);
       ELSIF TG_OP = 'UPDATE' THEN
           IF (to_jsonb(OLD) - 'source_checked_at' - 'updated_at') = (to_jsonb(NEW) - 'source_checked_at' - 'updated_at') THEN
               RETURN NULL;
           END IF;
           INSERT INTO medical_personnel_history (personnel_id, op, old_row, new_row, reason)
           VALUES (NEW.id, TG_OP, to_jsonb(OLD), to_jsonb(NEW), change_reason);
       ELSE
           INSERT INTO medical_personnel_history (personnel_id, op, old_row, reason)
           VALUES (OLD.id, TG_OP, to_jsonb(OLD), change_reason);
       END IF;
       RETURN NULL;
   END;
   $$;

   -- 下架官網醫師時,把它的 source_key 記進排除清單(OD-10)。
   CREATE OR REPLACE FUNCTION takedown_personnel(p_id INTEGER) RETURNS void
   LANGUAGE plpgsql AS $$
   DECLARE
       v_key TEXT;
   BEGIN
       DELETE FROM medical_personnel WHERE id = p_id RETURNING source_key INTO v_key;
       IF NOT FOUND THEN
           RAISE EXCEPTION 'medical_personnel id % not found', p_id;
       END IF;
       IF v_key IS NOT NULL THEN
           INSERT INTO takedown_suppressions (source_key) VALUES (v_key) ON CONFLICT DO NOTHING;
       END IF;
       -- 上面的 DELETE 已由 trigger 寫入一筆含 old_row 的歷史,這裡連同它一起清除。
       DELETE FROM medical_personnel_history WHERE personnel_id = p_id;
       INSERT INTO medical_personnel_history (personnel_id, op, reason) VALUES (p_id, 'DELETE', 'takedown');
   END;
   $$;
   ```

2. 建立 `migrations/0007_add_crawl_keys.down.sql`。內容依序為：
   - (a) 以 `CREATE OR REPLACE FUNCTION` 還原 `takedown_personnel`：函式本體逐字複製 `migrations/0006_add_takedown_function.up.sql` 中從 `CREATE FUNCTION takedown_personnel` 到 `$$;` 的段落，只把開頭的 `CREATE FUNCTION` 改成 `CREATE OR REPLACE FUNCTION`。
   - (b) 以同樣方式還原 `record_medical_personnel_history`，來源是 `migrations/0005_add_history_and_checked_date.up.sql` 中從 `CREATE FUNCTION record_medical_personnel_history` 到 `$$;` 的段落。
   - (c) 依序執行以下三行：
     ```sql
     DROP TABLE IF EXISTS takedown_suppressions;
     DROP INDEX IF EXISTS ux_medical_personnel_source_key;
     ALTER TABLE medical_personnel DROP COLUMN IF EXISTS source_key;
     ```

3. 在 `internal/database/repository_integration_test.go` 的 `openIntegrationDB`：
   - 把 `TRUNCATE medical_personnel, medical_personnel_history RESTART IDENTITY` 改成 `TRUNCATE medical_personnel, medical_personnel_history, takedown_suppressions RESTART IDENTITY`。
   - 把註解 `// and empties medical_personnel and its history so each test starts from known ids.` 改成 `// and empties medical_personnel, its history and takedown suppressions so each test starts from known ids.`。

4. 在 `internal/database/history_integration_test.go` 末尾新增三個測試，沿用既有的 `historyRows`、`valid`、`seedPersonnel`，不要另寫 helper：
   - `TestHistorySkipsCheckedDateOnlyUpdate`：
     - seed 一筆 `{"台北", "甲醫院", "內科", "王一"}`。
     - 執行 `UPDATE medical_personnel SET source_checked_at = '2026-10-12', updated_at = now() WHERE id = 1`，斷言 `historyRows` 只有 1 列。
     - 再執行 `UPDATE medical_personnel SET source_checked_at = '2026-11-12', hospital = '乙醫院' WHERE id = 1`，斷言共 2 列，第 2 列為 `{PersonnelID: 1, Op: "UPDATE", OldHospital: valid("甲醫院"), NewHospital: valid("乙醫院")}`。
   - `TestTakedownPersonnelRecordsSuppression`：
     - 依序 INSERT 兩筆：`INSERT INTO medical_personnel (city, hospital, name, source_key) VALUES ('新北', '亞東紀念醫院', '王一', 'femh:1')`（id 1）與 `INSERT INTO medical_personnel (city, hospital, name) VALUES ('台北', '甲醫院', '王二')`（id 2）。
     - 執行 `SELECT takedown_personnel(1)` 與 `SELECT takedown_personnel(2)`。
     - 斷言 `SELECT source_key FROM takedown_suppressions ORDER BY source_key` 恰好等於 `[]string{"femh:1"}`。
   - `TestSourceKeyIsUnique`：
     - INSERT 兩筆 `source_key` 都是 `'femh:1'` 的列，第二筆要 `require.Error`。
     - 另 INSERT 兩筆 `source_key` 為 NULL 的列，都要 `require.NoError`。

5. 執行 `make test-integration`，確認全部通過（含最後的 0007 down）。

---

## Part 2：`internal/crawl` 共用部分（抓取器、學歷規則、`BuildSyncPlan`）

> 不依賴 Part 1，可以先做。

### 現況

- `internal/crawl` 不存在。
- HTML 解析的慣例見 `internal/reconcile/parse.go`：用 stdlib `regexp` 加 `html.UnescapeString`，不引入 HTML 解析套件。

### 目標

- `Fetcher.Get`：
  - 送出 UA `popofinder-crawler/1.0 (+mailto:jerrychuang@jerrytech.me)`。
  - 每個 host 先讀一次 robots.txt，路徑被禁止時回傳 error。
  - 同一 host 兩次請求間隔至少 `max(minInterval, Crawl-delay)`。
  - 非 200 回傳 error。
- 學歷規則依 OD-10。
- `BuildSyncPlan` 依上方 State Enumeration 算出結果，而且輸出順序固定。

### 不可動的範圍

- `internal/reconcile/` 全部。
- 不引入 `golang.org/x/net/html` 或其他 HTML 解析套件；唯一新依賴是 `github.com/temoto/robotstxt v1.1.2`。

### 實作步驟

1. 執行 `go get github.com/temoto/robotstxt@v1.1.2`，等 Part 2–4 的程式都完成後再執行 `go mod tidy`。`go.mod`、`go.sum` 的變動都屬於本 task。

2. 建立 `internal/crawl/crawl.go`：
   ```go
   // Package crawl fetches doctor profile pages from hospital websites and plans
   // how to sync them into medical_personnel (docs/open-decisions.md OD-7, OD-10).
   package crawl

   import "context"

   // Doctor is one doctor as published on a hospital website.
   type Doctor struct {
   	Key         string   // "<source>:<site doctor id>", e.g. "femh:42522"
   	Name        string
   	City        string
   	Hospital    string
   	Departments []string // in site order; BuildSyncPlan dedupes and joins them
   	Education   []string // one entry per line of the site's 學歷 field
   	URL         string   // profile page
   }

   // Source crawls one hospital website. Crawl returns an error if any page fails,
   // so callers never sync a partial crawl.
   type Source interface {
   	Name() string
   	Crawl(ctx context.Context) ([]Doctor, error)
   }
   ```

3. 建立 `internal/crawl/fetcher.go`：
   - 常數：`userAgent = "popofinder-crawler/1.0 (+mailto:jerrychuang@jerrytech.me)"`、`robotsAgent = "popofinder-crawler"`、`maxBodyBytes = 5 << 20`。
   - `type Fetcher struct`，未匯出欄位：`client *http.Client`（`Timeout: 30 * time.Second`）、`minInterval time.Duration`、`sleep func(time.Duration)`（預設 `time.Sleep`，測試可替換）、`robots map[string]*robotstxt.RobotsData`（key 為 `scheme://host`）、`last map[string]time.Time`（key 同上）。在型別註解寫明「不支援並行呼叫」。
   - `func NewFetcher(minInterval time.Duration) *Fetcher`。
   - `func (f *Fetcher) Get(ctx context.Context, rawURL string) (string, error)` 依序做：
     1. 用 `url.Parse` 取得 `scheme://host`。
     2. 取得 robots 資料：快取沒有時，GET `scheme://host/robots.txt`（帶同一個 UA）。網路錯誤就回傳 error。否則用 `robotstxt.FromStatusAndBytes(status, body)` 解析（4xx 視為全部允許、5xx 視為全部禁止），並快取 `*robotstxt.RobotsData`。
     3. 用 `data.TestAgent(u.RequestURI(), robotsAgent)` 判斷是否允許，**不可**用 `FindGroup(...).Test(...)`：robots.txt 回 5xx 時，函式庫的 `FindGroup` 會回傳空 group，`Test` 因此會判定為允許；只有 `TestAgent` 會正確處理 5xx 全部禁止（2026-10-12 讀 v1.1.2 原始碼確認）。不允許時回傳 `fmt.Errorf("crawl: %s disallowed by robots.txt", rawURL)`。
     4. `delay := max(f.minInterval, data.FindGroup(robotsAgent).CrawlDelay)`。這個 host 有上次請求時間、而且 `w := delay - time.Since(last) > 0` 時，呼叫 `f.sleep(w)`。
     5. 發出帶 UA 的 GET，記錄 `f.last[host] = time.Now()`。status 不是 200 時回傳 `fmt.Errorf("crawl: GET %s: status %d", rawURL, status)`。
     6. 用 `io.LimitReader(resp.Body, maxBodyBytes)` 讀出 body，回傳字串。

4. 建立 `internal/crawl/fetcher_test.go`（`package crawl`），每個測試用 `httptest.NewServer` 自建 handler：
   - `TestFetcherSendsUserAgent`：`/ok` 回 200 `hello` 並記下 `User-Agent`。斷言 body 是 `hello`，UA 等於 `userAgent`。
   - `TestFetcherRespectsRobotsDisallow`：`/robots.txt` 回 `User-agent: *\nDisallow: /private\n`。`Get(/private)` 要回傳含 `robots.txt` 的 error，而且 server 沒收到 `/private` 請求。
   - `TestFetcherAllowsWhenRobotsMissing`：`/robots.txt` 回 404，`Get(/ok)` 成功。
   - `TestFetcherTreatsRobots5xxAsDisallow`：`/robots.txt` 回 503，`Get(/ok)` 要回傳含 `robots.txt` 的 error，而且 server 沒收到 `/ok` 請求。
   - `TestFetcherErrorsOnNon200`：`/fail` 回 500，error 訊息含 `status 500`。
   - `TestFetcherWaitsCrawlDelay`：`/robots.txt` 回 `User-agent: *\nCrawl-delay: 5\n`。`NewFetcher(0)` 後把 `f.sleep` 換成記錄參數的函式，連續 `Get(/ok)` 兩次。斷言記錄到的最大等待 `> 4*time.Second`（實際不會睡）。

5. 建立 `internal/crawl/education.go`：
   - `func hasMedicalDegree(lines []string) bool`：任一行符合 `isMedicalDegreeLine` 就回傳 true。
   - `func isMedicalDegreeLine(line string) bool`：
     - 先排除：符合 `excludeRe` 就回傳 false。`excludeRe` = `碩士|博士|研究所|(?i:ph\.?\s?d|master|\bmph\b|\bmba\b|\bm\.?sc?\b)`。
     - 再依序判斷，任一成立就回傳 true：
       - (1) 含「醫學系」或「醫學士」（涵蓋牙醫學系、中醫學系、學士後醫學系、牙醫學士）。
       - (2) 不含「學系」，而且含「醫學院」或「醫學大學」。
       - (3) 符合 `englishRe` = `(?i:\bm\.?d\b|\bd\.?d\.?s\b|\bdmd\b|\bmbbs\b|doctor of medicine|bachelor of medicine|medicine|dental surgery)`。
     - 都不成立就回傳 false。
   - `func formatEducation(lines []string) string`：每行 `strings.TrimSpace`，去掉空行，用 `；` 串接，超過 500 個 rune 時截到 500。

6. 建立 `internal/crawl/education_test.go`，`TestIsMedicalDegreeLine` 用 table-driven，至少包含以下案例（輸入 → 預期）：

   | 輸入 | 預期 |
   |---|---|
   | `國立台灣大學醫學系` | true |
   | `緬甸仰光第一醫學院醫學系學士` | true |
   | `西班牙埃雷拉大學牙醫學士` | true |
   | `中國醫藥大學中醫學系` | true |
   | `高雄醫學大學學士後醫學系` | true |
   | `波蘭盧布林醫學大學` | true |
   | `國立陽明大學醫學院` | true |
   | `Doctor of Medicine, University of the Philippines` | true |
   | `M.D., Universidad Central del Este` | true |
   | `輔仁大學公共衛生學系碩士` | false |
   | `台北醫學大學醫學研究所博士` | false |
   | `高雄醫學大學藥學系` | false |
   | `國立陽明大學護理學系` | false |
   | `Master of Public Health, Johns Hopkins University` | false |

   另寫 `TestHasMedicalDegree`（`[碩士行, 醫學系行]` → true；`[碩士行]` → false；`nil` → false）與 `TestFormatEducation`（兩行加一個空白行 → `A；B`；600 個「醫」字 → 長度 500 rune）。

7. 建立 `internal/crawl/plan.go`：
   - 型別：
     ```go
     // ExistingRow is a crawled row already in medical_personnel (source_key IS NOT NULL).
     type ExistingRow struct {
     	ID         int            `db:"id"`
     	Key        string         `db:"source_key"`
     	Name       string         `db:"name"`
     	City       string         `db:"city"`
     	Hospital   string         `db:"hospital"`
     	Department sql.NullString `db:"department"`
     	Education  sql.NullString `db:"education"`
     	URL        sql.NullString `db:"source_url"`
     }

     // Row is the normalized value written for one doctor; Department "" means NULL.
     type Row struct {
     	Key, Name, City, Hospital, Department, Education, URL string
     }

     type RowUpdate struct {
     	ID  int
     	Row Row
     }

     // SyncPlan is the outcome of comparing one source's crawl with its existing rows.
     type SyncPlan struct {
     	Inserts           []Row
     	Updates           []RowUpdate
     	TouchIDs          []int
     	DeleteIDs         []int
     	SkippedSuppressed int
     	SkippedNoDegree   int
     	DeleteBlocked     bool
     }
     ```
   - `func rowFromDoctor(d Doctor) Row`：
     - `Name` = `strings.Join(strings.Fields(d.Name), "")`，最多 100 rune。
     - `City`、`Hospital`、`URL` 做 `TrimSpace`；`Hospital` 最多 200 rune。
     - `Department` = `Departments` 去空白、去重（保留第一次出現的順序）後，用 `、` 串接。超過 100 rune 時，從尾端整個拿掉科別直到不超過；只剩一個科別仍超過時，截到 100 rune。
     - `Education` = `formatEducation(d.Education)`。
   - `func BuildSyncPlan(crawled []Doctor, existing []ExistingRow, suppressed map[string]bool) SyncPlan`，完全依上方 State Enumeration：
     - `crawled` 中重複的 `Key` 只取第一個。
     - 符合條件 = `!suppressed[key] && hasMedicalDegree(d.Education)`。
     - 比較時把 `sql.NullString` 的 NULL 視為 `""`。
     - 刪除保護：`len(existing) > 0 && eligible*2 < len(existing)` 時，`DeleteIDs = nil`、`DeleteBlocked = true`。
     - 輸出排序：`Inserts` 依 `Key`、`Updates` 依 `ID`、`TouchIDs` 與 `DeleteIDs` 由小到大。

8. 建立 `internal/crawl/plan_test.go`：
   - `TestBuildSyncPlanInsertsNewDoctors`：existing 為空，兩位有醫學學位的醫師 → 2 筆 `Inserts`。`Row.Department` 是去重後的串接（輸入 `[]string{"內科", "內科", "家醫科"}` → `內科、家醫科`），`Row.Education` 用 `；` 串接，`Name` 去掉內部空白（`"王 一"` → `王一`）。
   - `TestBuildSyncPlanUpdatesTouchesAndDeletes`：existing 有 A（內容相同）、B（學歷不同）、C（這次沒抓到）；crawled 有 A、B'、D（新）。斷言：
     - `Inserts` 只有 D。
     - `Updates` 只有 B 的 ID，而且 `Row.Education` 是新值。
     - `TouchIDs` 只有 A 的 ID。
     - `DeleteIDs` 只有 C 的 ID。
     - `DeleteBlocked` 為 false。
   - `TestBuildSyncPlanSkipsSuppressedAndNoDegree`：
     - crawled 有 E（在 `suppressed`）、F（只寫碩士）、G（existing 有、這次只寫碩士）。
     - 斷言：不新增 E、F；`SkippedSuppressed == 1`；`SkippedNoDegree == 1`（F 算；G 因為已經存在，進 `DeleteIDs`）；`DeleteIDs` 只有 G。
     - existing 只有 G 一筆而且符合條件數為 0，會觸發保護。所以本測試的 existing 另外放 H、I、J 三筆，crawled 也包含 H、I、J 而且內容相同，讓符合條件數 3 × 2 ≥ 4。
   - `TestBuildSyncPlanBlocksMassDelete`：existing 10 筆，crawled 只有其中 4 筆（內容相同）。斷言 `DeleteBlocked == true`、`DeleteIDs` 為空、`TouchIDs` 有 4 筆。
   - `TestBuildSyncPlanFirstRunNeverBlocks`：existing 為空、crawled 為空。斷言 `DeleteBlocked == false`。
   - `TestRowFromDoctorLimits`：科別各 30 個字的 5 個科別 → 結果 ≤ 100 rune，而且是前幾個完整科別；單一 150 字的科別 → 100 rune。

9. 執行 `go test ./internal/crawl/`。

---

## Part 3：亞東 parser

> 依賴 Part 2（`Doctor`、`Fetcher`）。

### 現況

無亞東相關程式。真實頁面結構見本文件〈背景〉。

### 目標

`NewFemh(baseURL, fetcher).Crawl(ctx)` 回傳亞東所有醫師（同一位醫師跨科別只出現一次，科別依索引順序）。合成 fixture 測試與 live 測試都通過。

### 不可動的範圍

- Part 2 的匯出型別與函式簽名。
- fixture 只能用虛構姓名：王小明、李小華、張小美。

### 實作步驟

1. 建立 `internal/crawl/femh.go`：
   - 常數：`femhHospital = "亞東紀念醫院"`、`femhCity = "新北"`。
   - `femhExcludedDept = regexp.MustCompile("加護病房|呼吸照護|健康管理")`：索引中名稱符合的科別不爬，因為這些單位的醫師也列在各自的科別頁。
   - `type Femh struct { baseURL string; fetcher *Fetcher }`、`func NewFemh(baseURL string, f *Fetcher) *Femh`、`func (s *Femh) Name() string { return "femh" }`。
   - `type femhDept struct { CID, Name string }`。
   - `func parseFemhIndex(page string) []femhDept`：
     - 用 `(?s)<a[^>]*href=["'][^"']*sectionDetail\.aspx\?CID=(\w+)["'][^>]*>(.*?)</a>` 取出 CID 與連結文字。
     - 連結文字去掉標籤、`html.UnescapeString`、`TrimSpace` 後當科別名稱。
     - 依 CID 去重（保留第一次出現），去掉符合 `femhExcludedDept` 的科別，保持原順序。
   - `func parseFemhDept(page string) []string`：用 `sectionDetail2\.aspx\?CID=\w+(?:&amp;|&)+DoctorID=(\w+)` 取出 DoctorID，去重並保持順序。
   - `func parseFemhProfile(page string) (name string, education []string, err error)`：
     - 姓名：從 `(?s)<title>(.*?)</title>` 取出標題，`html.UnescapeString` 後取「醫師｜」之前的文字；沒有「醫師｜」時取第一個「｜」之前的文字。再用 `strings.Join(strings.Fields(...), "")` 去掉空白。結果為空時回傳 `fmt.Errorf("crawl: femh profile without name")`。
     - 學歷：用 `(?s)class=["']h5 color-g name["']>\s*學歷\s*</div>\s*<ul>(.*?)</ul>` 取出區塊，每個 `(?s)<li>(.*?)</li>` 去標籤、unescape、`TrimSpace`、略過空字串。沒有學歷區塊時回傳 `nil`，不是錯誤。
   - `func (s *Femh) Crawl(ctx context.Context) ([]Doctor, error)`：
     1. GET `baseURL + "/section/section1.aspx"`，用 `parseFemhIndex` 解析；結果為空時回傳 error。
     2. 依索引順序 GET 每個 `baseURL + "/section/sectionDetail.aspx?CID=" + cid`，用 `parseFemhDept` 取出 DoctorID。依第一次出現的順序記下每位醫師的第一個 CID 與科別名稱清單。
     3. 醫師總數為 0 時回傳 error。
     4. 依上述順序 GET 每位醫師的 `baseURL + "/section/sectionDetail2.aspx?CID=" + firstCID + "&&DoctorID=" + id`，用 `parseFemhProfile` 解析，組成 `Doctor{Key: "femh:" + id, Name, City: femhCity, Hospital: femhHospital, Departments, Education, URL: 該個人頁網址}`。
     5. 任何 GET 或 `parseFemhProfile` 失敗時，立即回傳包住原錯誤的 error，不回傳部分結果。

2. 建立 `internal/crawl/testdata/femh/index.html`，內容完全如下：

   ```html
   <!DOCTYPE html>
   <html lang="zh-Hant">
   <head><meta charset="UTF-8" /><title>醫療團隊｜亞東紀念醫院</title></head>
   <body>
   <ul class="section-list">
       <li><a href="sectionDetail.aspx?CID=0401" title="心臟血管內科">心臟血管內科</a></li>
       <li><a href="sectionDetail.aspx?CID=0203" title="新陳代謝科">新陳代謝科</a></li>
       <li><a href="sectionDetail.aspx?CID=0534" title="心血管加護病房">心血管加護病房</a></li>
   </ul>
   <ul class="footer-list">
       <li><a href="sectionDetail.aspx?CID=0203">新陳代謝科</a></li>
   </ul>
   </body>
   </html>
   ```

3. 建立 `internal/crawl/testdata/femh/dept_0401.html`，內容完全如下：

   ```html
   <!DOCTYPE html>
   <html lang="zh-Hant">
   <head><meta charset="UTF-8" /><title>心臟血管內科｜亞東紀念醫院</title></head>
   <body>
   <div class='row'>
   <div class='col-6 col-md-3'><a href='sectionDetail2.aspx?CID=0401&&DoctorID=10002' class='card-doctor'><div class='img'><div class='cover' Style ='background-image: url(https://example.invalid/doctorphoto/Doctor10002.jpg);'></div></div><div class='text'><div class='cwss1 arWebFont'>李小華     醫師</div><div class='doctor-brief'>專長：心臟衰竭</div></div></a> </div> <div class='col-6 col-md-3'><a href='sectionDetail2.aspx?CID=0401&&DoctorID=10003' class='card-doctor'><div class='img'><div class='cover' Style ='background-image: url(https://example.invalid/doctorphoto/Doctor10003.jpg);'></div></div><div class='text'><div class='cwss1 arWebFont'>張小美     醫師</div><div class='doctor-brief'>專長：高血壓</div></div></a> </div>
   </div>
   </body>
   </html>
   ```

4. 建立 `internal/crawl/testdata/femh/dept_0203.html`：結構同上，`<title>` 改為 `新陳代謝科｜亞東紀念醫院`，兩張卡片依序為：
   - `sectionDetail2.aspx?CID=0203&&DoctorID=10001`，姓名 `王小明     醫師`，專長 `糖尿病`。
   - `sectionDetail2.aspx?CID=0203&&DoctorID=10002`，姓名 `李小華     醫師`，專長 `甲狀腺疾病`。

5. 建立三個個人頁 fixture，結構完全依下面這份 `profile_10001.html`：

   ```html
   <!DOCTYPE html>
   <html lang="zh-Hant">
   <head><meta charset="UTF-8" /><title>王小明醫師｜新陳代謝科｜亞東紀念醫院</title></head>
   <body>
   <h3 class="col-12 col-sm-6 arWebFont">王小明 醫師</h3>
   <div class="col-12 col-sm-6">
       <div class="h5 color-g name">主治項目</div>
       <ul>
            <li>糖尿病 </li>
       </ul>
       <div class="h5 color-g name">學歷</div>
       <ul>
           <li>輔仁大學公共衛生學系碩士</li><li>國立台灣大學醫學系學士</li>
       </ul>
   </div>
   <div class="col-12 col-sm-6">
       <div class="h5 color-g name">經歷</div>
       <ul>
          <li>亞東紀念醫院新陳代謝科總醫師</li><li>輔仁大學醫學系兼任講師</li>
       </ul>
   </div>
   </body>
   </html>
   ```

   - `profile_10002.html`：
     - `<title>` 改為 `李小華醫師｜心臟血管內科｜亞東紀念醫院`，`<h3>` 改為 `李小華 醫師`。
     - 學歷 `<ul>` 改為 `<li>西班牙埃雷拉大學牙醫學士</li><li>波蘭盧布林醫學大學醫學系</li>`。
   - `profile_10003.html`：
     - `<title>` 改為 `張小美醫師｜心臟血管內科｜亞東紀念醫院`，`<h3>` 改為 `張小美 醫師`。
     - **刪除整個「學歷」標題與其 `<ul>`**（測試沒有學歷區塊的情況）。

6. 建立 `internal/crawl/femh_test.go`：
   - `TestParseFemhIndex`：讀 `testdata/femh/index.html`。斷言結果恰為 `[]femhDept{{"0401", "心臟血管內科"}, {"0203", "新陳代謝科"}}`（加護病房被排除、重複的 0203 去掉）。
   - `TestParseFemhDept`：讀 `dept_0203.html`，斷言 `[]string{"10001", "10002"}`。
   - `TestParseFemhProfile`：
     - `profile_10001.html` → 姓名 `王小明`、學歷 `[]string{"輔仁大學公共衛生學系碩士", "國立台灣大學醫學系學士"}`。
     - `profile_10003.html` → 姓名 `張小美`、學歷 `nil`、無錯誤。
     - 字串 `<html><title>｜｜</title></html>` → error。
   - `TestFemhCrawlAggregatesDepartments`：
     - `httptest.NewServer` 依路徑與 query 回傳 fixture：`/section/section1.aspx` → index；`/section/sectionDetail.aspx` 依 `CID` → `dept_<CID>.html`，檔案不存在時回 404；`/section/sectionDetail2.aspx` 依 `DoctorID` → `profile_<DoctorID>.html`；`/robots.txt` → 404。
     - 用 `NewFetcher(0)` 與 server URL 建 `NewFemh`，呼叫 `Crawl`。
     - 斷言回傳 3 位，順序為 `femh:10002`、`femh:10003`、`femh:10001`。
     - 第一位的 `Departments == []string{"心臟血管內科", "新陳代謝科"}`、`City == "新北"`、`Hospital == "亞東紀念醫院"`、`URL` 以 `/section/sectionDetail2.aspx?CID=0401&&DoctorID=10002` 結尾。
     - 加護病房 0534 沒有 fixture；若被抓取會回 404 讓 Crawl 失敗，所以這個測試同時證明它被排除。
   - `TestFemhCrawlFailsOnMissingDeptPage`：同一個 server，但讓 `CID=0203` 回 500。斷言 `Crawl` 回傳 error、結果為 nil。

7. 建立 `internal/crawl/femh_live_test.go`，第一行 `//go:build live`：
   - `TestFemhLive` 用 `NewFetcher(time.Second)` 對 `https://www.femh.org.tw` 依序：
     - 抓索引 → 斷言 `parseFemhIndex` 至少 40 個科別。
     - 抓第一個科別頁 → 斷言 `parseFemhDept` 至少 1 位。
     - 抓該科第一位醫師的個人頁 → 斷言姓名非空、學歷至少 1 行。
   - 只用 `t.Logf` 印出數量，不印姓名與學歷內容。總共 4 個請求（含 robots.txt）。

8. 執行 `go test ./internal/crawl/` 與 `go test -tags live ./internal/crawl -run TestFemhLive -count=1 -v`。

---

## Part 4：寫入 DB、`cmd/crawl`、image

> 依賴 Part 1（schema）、Part 2（`SyncPlan`）、Part 3（`NewFemh`）。

### 現況

- `internal/database/repository.go` 有 `Repository`（`Search`、`LogSearch`、`Ping`）。
- `cmd/reconcile/main.go` 讀 `DATABASE_URL`、用 `database.Connect`。
- `dockerfile` 會建 `popofinder`、`reconcile`、`migrate` 三個 binary。

### 目標

- `Repository.LoadCrawlState` 讀出某來源已寫入的列與排除鍵。
- `Repository.ApplyCrawlPlan` 在一個交易內寫入計畫：歷史 reason 為 `crawl:<來源>`，`source_checked_at` 為台北時區的日期。
- `cmd/crawl` 依 State Enumeration 執行，log 只記數量。
- image 內有 `/crawl`。

### 不可動的範圍

- `Repository.Search`、`LogSearch`、`Ping`、`selectColumns`。
- `cmd/server`、`cmd/reconcile`。
- `dockerfile` 既有的三個 build 指令與 COPY 行。

### 實作步驟

1. 建立 `internal/database/crawl.go`：
   - `func (r *Repository) LoadCrawlState(ctx context.Context, source string) ([]crawl.ExistingRow, map[string]bool, error)`：
     - `pattern := source + ":%"`。
     - 執行 `SELECT id, source_key, name, city, hospital, department, education, source_url FROM medical_personnel WHERE source_key LIKE $1 ORDER BY id`。
     - 執行 `SELECT source_key FROM takedown_suppressions WHERE source_key LIKE $1`，結果放進 map（值為 true）。
     - 錯誤用 `fmt.Errorf("repository: load crawl state: %w", err)` 包起來。
   - `func (r *Repository) ApplyCrawlPlan(ctx context.Context, source string, plan crawl.SyncPlan, checkedOn time.Time) error`：
     - `date := checkedOn.Format("2006-01-02")`，以字串傳給 DATE 欄位。不可直接傳 `time.Time`，否則會依連線時區轉換，讓台北凌晨變成前一天。
     - `tx, err := r.db.BeginTxx(ctx, nil)`，先執行 `SELECT set_config('popo.change_reason', $1, true)`，參數為 `"crawl:" + source`。
     - 逐筆 INSERT：`INSERT INTO medical_personnel (city, hospital, department, name, education, source, source_url, source_key, source_checked_at) VALUES ($1, $2, $3, $4, $5, 'hospital_directory', $6, $7, $8)`。`Department` 為 `""` 時傳 `sql.NullString{}`。
     - 逐筆 UPDATE：`UPDATE medical_personnel SET name = $1, city = $2, hospital = $3, department = $4, education = $5, source_url = $6, source_checked_at = $7, updated_at = now() WHERE id = $8`。
     - 逐筆 touch：`UPDATE medical_personnel SET source_checked_at = $1, updated_at = now() WHERE id = $2`。
     - 逐筆刪除：`DELETE FROM medical_personnel WHERE id = $1`。
     - 任何錯誤都要 `tx.Rollback()`，並回傳 `fmt.Errorf("repository: apply crawl plan: %w", err)`；全部成功才 `tx.Commit()`。

2. 建立 `internal/database/crawl_integration_test.go`（第一行 `//go:build integration`，`package database`），用 `crawl.BuildSyncPlan` 產生計畫，不要手刻 `SyncPlan`：
   - `TestApplyCrawlPlanRoundTrip`：
     - **第一輪**：`checkedOn = time.Date(2026, 10, 12, 0, 30, 0, 0, time.FixedZone("Asia/Taipei", 8*60*60))`。crawled 為 A、B、D 三位（`Key` 為 `femh:1`、`femh:2`、`femh:4`，學歷都含 `國立台灣大學醫學系`）。執行 LoadCrawlState → BuildSyncPlan → ApplyCrawlPlan。斷言：
       - 3 列的 `source` 為 `hospital_directory`、`verification_status` 為 `unverified`。
       - `source_checked_at::text` 都是 `2026-10-12`（證明台北凌晨沒有變成前一天）。
       - 歷史有 3 筆 INSERT，`reason` 都是 `crawl:femh`。
     - **第二輪**：`checkedOn` 為 2026-11-12 00:30（同時區）。crawled 為 A（不變）、B'（學歷改成 `國立陽明大學醫學系`）、C（新，`femh:3`）。斷言：
       - A 的 `source_checked_at` 是 `2026-11-12`，而且 A 沒有新增歷史列。
       - B 的學歷已更新，並有一筆 `op='UPDATE'`、`reason='crawl:femh'` 的歷史。
       - C 已新增。
       - D 已刪除，並有一筆 `op='DELETE'`、`reason='crawl:femh'`、`old_row` 不為 NULL 的歷史。
   - `TestLoadCrawlStateScopesBySource`：
     - 直接 INSERT 三列：部落格列（無 `source_key`）、`source_key='cgmh:1'`、`source_key='femh:1'`。
     - 在 `takedown_suppressions` 放 `femh:9`、`cgmh:9`。
     - 斷言 `LoadCrawlState(ctx, "femh")` 只回 `femh:1` 一列，排除 map 恰為 `{"femh:9": true}`。
   - `TestCrawlDoesNotReAddTakenDownDoctor`：
     - 第一輪寫入 A（`femh:1`），再執行 `SELECT takedown_personnel(<A 的 id>)`。
     - 第二輪 crawled 仍含 A：斷言計畫 `SkippedSuppressed == 1` 而且 `Inserts` 為空；套用後 `medical_personnel` 為 0 列。

3. 建立 `cmd/crawl/main.go`：
   - flag：`-sources`（字串，必填，逗號分隔；說明文字 `要爬的來源,逗號分隔(目前支援:femh)`）、`-dry-run`（`只印出計畫,不寫入資料庫`）。
   - `fetcher := crawl.NewFetcher(time.Second)`；`registry := map[string]crawl.Source{"femh": crawl.NewFemh("https://www.femh.org.tw", fetcher)}`。
   - **連 DB 之前**先驗證每個來源名稱：
     - 空字串：`slog.Error("no sources")` 後 `os.Exit(2)`。
     - 不在 registry：`slog.Error(fmt.Sprintf("unknown source %q", name))` 後 `os.Exit(2)`。
   - `db, err := database.Connect(os.Getenv("DATABASE_URL"))`，`repo := database.NewRepository(db)`。
   - `checkedOn := time.Now().In(time.FixedZone("Asia/Taipei", 8*60*60))`。distroless image 沒有 tzdata，不可用 `time.LoadLocation`。
   - 對每個來源依序執行：
     1. `Crawl`，失敗時 `slog.Error("crawl", "source", name, "err", err)`，記 `failed = true`，繼續下一個來源。
     2. `LoadCrawlState`。
     3. `BuildSyncPlan`。
     4. `slog.Info("plan", "source", name, "crawled", len(doctors), "insert", ..., "update", ..., "touch", ..., "delete", ..., "skipped_suppressed", ..., "skipped_no_degree", ..., "delete_blocked", ...)`。
     5. `plan.DeleteBlocked` 時 `slog.Error("delete blocked", "source", name)` 並記 `failed = true`。
     6. `-dry-run` 時跳過寫入。
     7. 否則 `ApplyCrawlPlan`，失敗時記 `failed = true`。
   - log **不可**包含姓名、學歷或網址。
   - 結束時 `failed` 為真就 `os.Exit(1)`。

4. 在 `dockerfile`：
   - 在 `RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o reconcile ./cmd/reconcile` 之後新增一行 `RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o crawl ./cmd/crawl`。
   - 在 `COPY --from=builder /build/reconcile /reconcile` 之後新增 `COPY --from=builder /build/crawl /crawl`。

5. 執行 `go mod tidy`、`go build ./...`、`go vet ./...`、`go test ./...`、`make test-integration`、`docker build -q -f dockerfile -t popofinder:crawl-check .`。

---

## Part 5：決策與文件

> 不依賴其他 Part。

### 現況

- `docs/open-decisions.md` 最後一條是 OD-9。
  - OD-8 已有一行補註「2026-10-12 補註:收件管道與處理方式…見 OD-9。」
  - OD-9 有一行「爬蟲上線時(OD-7),被下架的官網醫師要加入排除清單…在爬蟲 task 一起實作。」
- `CLAUDE.md`「架構」段寫「三張表」，沒有提到爬蟲；「常用指令」段沒有爬蟲指令。

### 目標

OD-10 記錄本 task 的資料規則；OD-8、OD-9 各補一行指向 OD-10；CLAUDE.md 與 schema、指令一致。

### 不可動的範圍

- OD-1–OD-9 的既有文字（只能新增補註行）。
- `CLAUDE.md` 中下列指定行以外的內容。

### 實作步驟

1. 在 `docs/open-decisions.md` 中以 `  - 2026-10-12 補註:收件管道與處理方式` 開頭的那一行之後，插入一行（逐字）：

   ```
     - 2026-10-12 補註:只改 `source_checked_at`/`updated_at` 的 UPDATE 也不記歷史(每月重爬更新查到日期),見 OD-10。
   ```

2. 在 `docs/open-decisions.md` 中以 `  - 爬蟲上線時(OD-7),被下架的官網醫師要加入排除清單` 開頭的那一行之後，插入一行（逐字）：

   ```
     - 2026-10-12 補註:排除清單(`takedown_suppressions`)與整院下架的做法見 OD-10。
   ```

3. 在 `docs/open-decisions.md` 檔尾追加以下內容，逐字照抄：

   ```markdown

   ## OD-10 — 官網爬蟲的資料規則(先做亞東)

   - **日期**:2026-10-12
   - **狀態**:✅ 採納
   - **決策**:
     - 先做爬蟲框架+亞東一家,上線跑過一輪正式資料後,再加雙和、長庚、馬偕、國泰(OD-7 第一批)。
     - 每位醫師以「來源:該站醫師 ID」為穩定鍵(`source_key`,例如 `femh:42522`);同一位醫師出現在多個科別只存一筆,科別以「、」串接(最多 100 字)。
     - 學歷存該頁學歷欄的每一行(以「；」串接,最多 500 字)。至少一行是醫學學士等級才收錄:含「醫學系」「醫學士」(涵蓋牙醫學系、中醫學系、學士後醫學系),或不含其他「學系」而含「醫學院」「醫學大學」,或英文 MD/DDS/DMD/MBBS/Medicine;寫了碩士、博士、研究所的那一行不算。**中醫師也收。**
     - 每月重爬:內容有變就更新並記歷史(reason `crawl:<來源>`);沒變只更新 `source_checked_at`,不記歷史(trigger 忽略只改 `source_checked_at`/`updated_at` 的 UPDATE)。官網上消失、或不再寫醫學學位的醫師刪除(記歷史)。
     - **刪除保護**:某來源這次任何一頁抓取失敗,整個來源不寫入;抓到符合條件的人數不到 DB 現有的一半時,不做任何刪除並讓 job 失敗。
     - **下架排除**:`takedown_personnel()` 下架官網醫師時,把它的 `source_key` 記進 `takedown_suppressions`(只存鍵,不存內容),爬蟲之後跳過。**整院下架**:從 CronJob 的 `-sources` 拿掉該院,再執行 `SELECT takedown_personnel(id) FROM medical_personnel WHERE source_key LIKE '<來源>:%';`。
     - 爬蟲使用 UA `popofinder-crawler/1.0 (+mailto:jerrychuang@jerrytech.me)`,執行時讀 robots.txt(被擋就停、遵守 Crawl-delay),同一 host 至少間隔 1 秒;log 只記數量,不記姓名。
     - parser 測試只用自製的合成 HTML(仿真實結構、虛構姓名),真實頁面不進 repo;另有 `-tags live` 的測試直接打官網,確認頁面結構沒變。
   - **理由**:
     - 先上一家:比對、刪除、排除清單、歷史這些框架邏輯有問題時只影響一家,修好再擴。
     - 用站內醫師 ID 而不是「姓名+醫院」當鍵:同名醫師常見,醫師換科別也不會被當成另一個人。
     - 查到日期每月更新,卡片上的「查到」才會保持新;若每次都記歷史,5 家每年約多 4 萬筆只改日期的紀錄,真正的變動會被淹沒。
     - 中醫學系也是醫師學位,納入不需要例外規則;卡片照實顯示學歷與科別,判斷留給使用者(倫理鐵律 1)。
     - 刪除保護:網站改版最常見的症狀是「突然抓到 0 筆」,沒有保護就會把整家醫院的資料刪光。
   ```

4. 在 `CLAUDE.md`：
   - 把 `` `cmd/server/main.go` 進入點;`internal/` 下分 config / database / server / linebot(client·handler·search·session·template·webhook)。 `` 整行換成 `` `cmd/server/main.go` 進入點;`internal/` 下分 config / database / server / linebot(client·handler·search·session·template·webhook)。`cmd/crawl` + `internal/crawl` 是醫院官網爬蟲(open-decisions OD-7、OD-10)。 ``。
   - 把以 `- DB:Postgres,透過 sqlx + pgx;三張表:` 開頭的那一行整行換成：
     ```
     - DB:Postgres,透過 sqlx + pgx;四張表:`medical_personnel`(id, city, hospital, department, name, education, source, verification_status, source_url, source_checked_at, source_key)、`medical_personnel_history`(由 trigger 記錄前者每一次異動,只改查到日期不記;批次修改前先 `SET LOCAL popo.change_reason`;下架用 `SELECT takedown_personnel(<id>);` 連同歷史清除,見 open-decisions OD-8、OD-9)、`takedown_suppressions`(被下架的官網醫師,爬蟲跳過,見 OD-10)與 `search_log`(查詢紀錄,刻意不存使用者,見 open-decisions OD-5)。
     ```
   - 在「常用指令」段以 `- \`make test-integration\`` 開頭的那一行之後，新增兩行：
     ```
     - `go run ./cmd/crawl -sources femh -dry-run` — 醫院官網爬蟲(讀 `DATABASE_URL`;`-dry-run` 只印計畫數量,不寫入)。
     - `go test -tags live ./internal/crawl` — 直接打醫院官網,確認頁面結構沒變(需要網路,CI 不跑)。
     ```

---

## 驗證計畫

| Part | 驗證方式 | 預期斷言 | 為什麼這個斷言能證明目標達成 |
|------|---------|---------|------------------------------|
| Part 1 | `internal/database/history_integration_test.go:TestHistorySkipsCheckedDateOnlyUpdate` | 只改日期不新增歷史；日期加醫院一起改會新增 UPDATE 歷史 | 對應「每月更新日期但不記歷史」，同時證明真正的內容變動仍會記錄 |
| Part 1 | `internal/database/history_integration_test.go:TestTakedownPersonnelRecordsSuppression` | 下架官網醫師後排除清單有它的 key；下架部落格資料不新增排除鍵 | 對應 OD-10「下架排除」：爬蟲之後能知道要跳過誰 |
| Part 1 | `internal/database/history_integration_test.go:TestSourceKeyIsUnique` | 同一個 `source_key` 第二筆失敗；NULL 可以重複 | 穩定鍵在 DB 層唯一，重爬不會意外產生重複醫師；部落格資料不受影響 |
| Part 1 | `make test-integration` | exit 0（含最後的 0007 down） | 0007 能在全新 DB 套用，down 能還原兩個 function、移除欄位與表 |
| Part 2 | `internal/crawl/fetcher_test.go:TestFetcherRespectsRobotsDisallow` | 被 robots 禁止的路徑回 error，server 沒收到請求 | 證明 OD-10「被擋就停」，而且是在發出請求之前就停 |
| Part 2 | `internal/crawl/fetcher_test.go:TestFetcherTreatsRobots5xxAsDisallow` | robots.txt 回 503 時不發出請求 | 釘住函式庫 `FindGroup` 在 5xx 時誤判為允許的坑；改用 `FindGroup().Test()` 時這個測試會失敗 |
| Part 2 | `internal/crawl/fetcher_test.go:TestFetcherWaitsCrawlDelay` | Crawl-delay 5 秒時第二次請求前等待超過 4 秒 | 證明遵守 robots 的 Crawl-delay，不會打太快 |
| Part 2 | `internal/crawl/fetcher_test.go:TestFetcherSendsUserAgent` | 請求帶 `popofinder-crawler/1.0 (+mailto:…)` | 院方看得到是誰在抓、怎麼聯絡 |
| Part 2 | `internal/crawl/education_test.go:TestIsMedicalDegreeLine` | 醫學系、牙醫學士、中醫學系、外國醫學大學、MD 為 true；碩士、博士、藥學系、護理學系為 false | 直接對應 OD-10 的收錄規則，包含容易誤判的藥學系 |
| Part 2 | `internal/crawl/plan_test.go:TestBuildSyncPlanUpdatesTouchesAndDeletes` | 新增 D、更新 B、只更新 A 的日期、刪除 C | 涵蓋 State Enumeration 前四格 |
| Part 2 | `internal/crawl/plan_test.go:TestBuildSyncPlanSkipsSuppressedAndNoDegree` | 排除清單與無學位的新醫師不新增；不再寫學位的既有醫師被刪除 | 涵蓋「在排除清單」「沒有醫學學位」「不再寫醫學學位」三格 |
| Part 2 | `internal/crawl/plan_test.go:TestBuildSyncPlanBlocksMassDelete` | 只抓到 4/10 時不刪除、`DeleteBlocked` 為 true | 「數量驟降」那一格：網站改版時不會清光整家醫院 |
| Part 3 | `internal/crawl/femh_test.go:TestFemhCrawlAggregatesDepartments` | 3 位醫師、跨科別的醫師只出現一次且科別依序串起、加護病房沒被抓取 | 證明亞東的索引→科別→個人頁流程與去重規則 |
| Part 3 | `internal/crawl/femh_test.go:TestFemhCrawlFailsOnMissingDeptPage` | 任一科別頁 500 時 Crawl 回 error、不回部分結果 | 「抓取失敗」那一格：部分結果不會被拿去同步，否則沒抓到的醫師會被刪除 |
| Part 3 | `internal/crawl/femh_test.go:TestParseFemhProfile` | 從 title 取出姓名、從學歷區塊取出各行；沒有學歷區塊回 nil | 個人頁解析的兩種情況都有覆蓋 |
| Part 3 | `go test -tags live ./internal/crawl -run TestFemhLive -count=1 -v` | 真實網站至少 40 個科別、科別頁至少 1 位、個人頁有姓名與學歷 | 合成 fixture 只仿結構；這個測試證明真實網站目前就是這個結構 |
| Part 4 | `internal/database/crawl_integration_test.go:TestApplyCrawlPlanRoundTrip` | 兩輪同步後新增、更新、只改日期、刪除都正確；日期是台北日期；歷史 reason 為 `crawl:femh` 而且只改日期的沒有歷史 | 端到端證明 DB 寫入與 trigger、日期時區的組合正確 |
| Part 4 | `internal/database/crawl_integration_test.go:TestLoadCrawlStateScopesBySource` | 只讀到 `femh:` 的列與排除鍵 | 爬一家醫院不會刪到另一家或部落格資料 |
| Part 4 | `internal/database/crawl_integration_test.go:TestCrawlDoesNotReAddTakenDownDoctor` | 下架後重爬，醫師不會被加回 | 鐵律 4 與 OD-9／OD-10 的排除清單端到端成立 |
| Part 4 | `go run ./cmd/crawl -sources nosuch 2>&1 \| grep -q 'unknown source .*nosuch'` | exit 0（grep 找到錯誤訊息） | 來源名稱在連 DB 前就驗證，打錯不會誤跑 |
| Part 4 | `docker build -q -f dockerfile -t popofinder:crawl-check .` | exit 0 | image 能建出 `/crawl`（含新依賴的下載） |
| Part 5 | `grep -q '^## OD-10 — ' docs/open-decisions.md && test "$(grep -c '見 OD-10' docs/open-decisions.md)" -ge 2` | exit 0 | OD-10 已追加，OD-8、OD-9 都有補註指向它 |
| Part 5 | `grep -q 'takedown_suppressions' CLAUDE.md && grep -q 'go run ./cmd/crawl' CLAUDE.md && ! grep -q '三張表' CLAUDE.md` | exit 0 | CLAUDE.md 的 schema 與指令和實作一致，沒有殘留舊描述 |

---

## 驗收

- [ ] `go build ./...`
- [ ] `go vet ./...`
- [ ] `go test ./...`
- [ ] `test -z "$(gofmt -l $(find . -name '*.go' -not -path './internal/reconcile/fetch.go'))"`
- [ ] `go list -m github.com/temoto/robotstxt | grep -q 'v1.1.2'`
- [ ] `test -f migrations/0007_add_crawl_keys.up.sql && test -f migrations/0007_add_crawl_keys.down.sql`
- [ ] `make test-integration`
- [ ] `test "$(grep -c -E '^func (TestHistorySkipsCheckedDateOnlyUpdate|TestTakedownPersonnelRecordsSuppression|TestSourceKeyIsUnique)\(' internal/database/history_integration_test.go)" = 3`
- [ ] `test "$(grep -c -E '^func (TestApplyCrawlPlanRoundTrip|TestLoadCrawlStateScopesBySource|TestCrawlDoesNotReAddTakenDownDoctor)\(' internal/database/crawl_integration_test.go)" = 3`
- [ ] `grep -q '^//go:build integration' internal/database/crawl_integration_test.go`
- [ ] `test "$(cat internal/crawl/fetcher_test.go internal/crawl/education_test.go internal/crawl/plan_test.go internal/crawl/femh_test.go | grep -c -E '^func (TestFetcherSendsUserAgent|TestFetcherRespectsRobotsDisallow|TestFetcherAllowsWhenRobotsMissing|TestFetcherTreatsRobots5xxAsDisallow|TestFetcherErrorsOnNon200|TestFetcherWaitsCrawlDelay|TestIsMedicalDegreeLine|TestHasMedicalDegree|TestFormatEducation|TestBuildSyncPlanInsertsNewDoctors|TestBuildSyncPlanUpdatesTouchesAndDeletes|TestBuildSyncPlanSkipsSuppressedAndNoDegree|TestBuildSyncPlanBlocksMassDelete|TestBuildSyncPlanFirstRunNeverBlocks|TestRowFromDoctorLimits|TestParseFemhIndex|TestParseFemhDept|TestParseFemhProfile|TestFemhCrawlAggregatesDepartments|TestFemhCrawlFailsOnMissingDeptPage)\(')" = 20`
- [ ] `go test ./internal/crawl -count=1 -v`
- [ ] `head -1 internal/crawl/femh_live_test.go | grep -q '^//go:build live'`
- [ ] `go test -tags live ./internal/crawl -run TestFemhLive -count=1 -v`
- [ ] `test "$(grep -ho '<title>[^｜<]*醫師｜' internal/crawl/testdata/femh/profile_*.html | sed 's/<title>//; s/醫師｜//' | LC_ALL=C sort -u | tr '\n' ' ')" = "張小美 李小華 王小明 "`
- [ ] `go run ./cmd/crawl -sources nosuch 2>&1 | grep -q 'unknown source .*nosuch'`
- [ ] `grep -q 'go build -ldflags="-s -w" -o crawl ./cmd/crawl' dockerfile && grep -q 'COPY --from=builder /build/crawl /crawl' dockerfile`
- [ ] `docker build -q -f dockerfile -t popofinder:crawl-check .`
- [ ] `grep -q '^## OD-10 — ' docs/open-decisions.md && test "$(grep -c '見 OD-10' docs/open-decisions.md)" -ge 2`
- [ ] `grep -q 'takedown_suppressions' CLAUDE.md && grep -q 'go run ./cmd/crawl' CLAUDE.md && ! grep -q '三張表' CLAUDE.md`

## 執行報告

### 摘要
| Part | 狀態 | 說明 |
|------|------|------|
| Part 1：migration 0007 與整合測試 | ✅ 完成 | `source_key`＋partial unique index、`takedown_suppressions`、trigger 忽略只改日期的 UPDATE、`takedown_personnel` 記排除鍵；down 還原 0005／0006 原文；3 個整合測試通過 |
| Part 2：`internal/crawl` 共用部分 | ✅ 完成 | `Fetcher`（UA、robots.txt 用 `TestAgent`、Crawl-delay）、學歷規則、`BuildSyncPlan`；15 個單元測試通過 |
| Part 3：亞東 parser | ✅ 完成 | 索引→科別→個人頁、6 個合成 fixture（只有王小明／李小華／張小美）、httptest 測試；live 測試通過（亞東 48 個科別、首科 28 位、個人頁有姓名與學歷） |
| Part 4：寫入 DB、`cmd/crawl`、image | ✅ 完成 | `LoadCrawlState`／`ApplyCrawlPlan`＋3 個整合測試、`cmd/crawl`（連 DB 前驗證來源）、image 內 `/crawl -sources nosuch` 回 exit 2 |
| Part 5：決策與文件 | ✅ 完成 | OD-10、OD-8／OD-9 補註、CLAUDE.md |

### ❌ 未通過的驗收項目
無

### 🔍 需人工確認的驗收項目
無

### 🤔 執行中的判斷決策
- **Part 2, Step 3**：robots.txt 與頁面的 GET 共用一個未匯出的 `do` helper（帶 UA、`maxBodyBytes` 上限），不寫兩份請求程式。`FromStatusAndBytes` 回錯誤時（例如 3xx 或解析失敗）也回傳 error。
- **Part 2, Step 5**：新增 `truncateRunes`，`formatEducation` 與 `rowFromDoctor`（姓名、醫院、科別）共用。
- **Part 2, Step 7**：「科別去空白」解讀為 `TrimSpace` 並去掉空字串。DB 已有、但 key 又在排除清單裡的列（正常流程不會發生，因為 `takedown_personnel` 會同時刪列）不算保留，會進 `DeleteIDs`。
- **Part 2, Step 8**：`TestRowFromDoctorLimits` 中，5 個 30 字科別取前 3 個（30×3＋2 個分隔＝92 字）；第 4 個會變成 123 字。
- **Part 2**：task 給的 `Doctor` 型別經 gofmt 調整了註解對齊。
- **Part 3, Step 1**：`internal/reconcile` 不可動，無法匯出它的 `tagRe`，所以 crawl 另有 `femhTagRe`（一行 regexp）。另外新增 `indexURL`／`deptURL`／`profileURL`，讓 `Crawl` 與 live 測試共用網址格式，不在測試裡抄一份。
- **Part 3／4**：`Fetcher` 的錯誤訊息依 task 格式含網址；`cmd/crawl` 的 `crawl` 錯誤 log 因此可能含個人頁網址（只有站內醫師 ID，沒有姓名或學歷）。計畫 log 只記數量。
- **Part 4, Step 1**：task 寫「直接傳 `time.Time` 會依連線時區轉換，讓台北凌晨變成前一天」。讀 pgx v5.7.4 `pgtype/date.go` 後確認：DATE 參數是用 `time.Time` 本身的年月日編碼，不會偏移。仍依 task 傳字串（不依賴 driver 行為），但程式註解改成符合實情的說法。`TestApplyCrawlPlanRoundTrip` 仍釘住「台北 00:30 存成當天」。UPDATE 的科別也用 `nullIfEmpty`；交易內的寫入放在 `applyCrawlPlan` helper，讓 rollback 只有一個出口。
- **Part 4, Step 3**：`LoadCrawlState` 失敗時（task 沒寫）記 log、`failed = true`，然後繼續下一個來源。`-sources femh,`（含空元素）視為 `no sources`，exit 2。

### ⚠️ 注意事項
- **新依賴**：`github.com/temoto/robotstxt v1.1.2`（direct）。注意：robots.txt 回 5xx 時，`FindGroup().Test()` 會判定為允許，必須用 `TestAgent`（`TestFetcherTreatsRobots5xxAsDisallow` 釘住）。
- **Schema（0007）**：`medical_personnel.source_key`（partial unique）、`takedown_suppressions`；trigger 只改 `source_checked_at`／`updated_at` 的 UPDATE 不記歷史；`takedown_personnel` 下架有 `source_key` 的列時寫入排除清單。
- **新模式**：`internal/crawl.Source` 介面；新增醫院＝新的 `Source` 實作，加進 `cmd/crawl` 的 `registry`，同步規則由 `BuildSyncPlan`（純函式）統一處理。image 多一個 `/crawl` binary。
- **上線**（本 task 外）：merge 後 production migrate 應輸出 `7/u`；jerrytech-deploy 的 CronJob 要改成 `/crawl -sources femh -dry-run`，看過數字再改成正式寫入。
