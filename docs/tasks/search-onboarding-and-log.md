# Task：查詢 onboarding（Quick Reply）＋全欄位查詢＋查詢紀錄

## 背景

第一次使用的人直接輸入醫院名稱（例如「三總」）會被當成醫師姓名查詢，結果是 0 筆，因為查醫院必須寫 `@醫院 三總`；說明卡也只是靜態的，沒有按鈕可以點。這個 task 做三件事：

- 沒有加 `@` 時同時比對姓名、醫院、科別，姓名符合的排在最前面。
- 用 LINE Quick Reply 做 onboarding（加好友、說明卡、查到 0 筆三個時機）。
- 新增 `search_log` 表，把每次查詢的結果筆數存進 DB，用來量零結果率與覆蓋率缺口。stdout log 會隨 pod 重啟消失，不能拿來累積。

2026-10-12 用部落格公開的 1476 筆資料模擬全欄位查詢：1033 個全名查詢中有 1032 個結果不變；會混在一起的主要是單一姓氏，例如「林」會多出林口長庚、員林、大林慈濟等醫院的 33 筆，所以用「姓名優先排序＋摘要列出各組筆數」處理。

**本刀刻意不做（使用者 2026-10-12 對話中決定）**：

- 臺／台正規化：另開資料清理 task。注意 `internal/reconcile/plan.go` 的 `normalizeKey` 是用原文比對，改寫 DB 資料會讓 reconcile 重複 INSERT，那份 task 要處理。
- stdout log 仍記錄 `userId` 與查詢原文：不動。
- 部落格中欄位填錯的資料：歸資料清理 task。
- 圖文選單（rich menu）：不做。

### 涉及檔案

- `internal/linebot/search/types.go` — 新增 `TypeAll`，移除不再產生的 `TypeName`
- `internal/linebot/search/parser.go` — 預設查詢類型改為 `TypeAll`，更新格式註解
- `internal/linebot/search/parser_test.go` — 更新既有案例並新增「只輸入醫院名」案例
- `internal/linebot/session/store_test.go` — 測試資料中的 `search.TypeName` 改為 `search.TypeAll`（因 `TypeName` 移除而必改）
- `internal/database/repository.go` — 全欄位 WHERE、`ORDER BY`、各組筆數 `MatchCounts`、`LogSearch`
- `internal/database/repository_integration_test.go` — 新檔，`//go:build integration`，對真 Postgres 驗證查詢與 `search_log`
- `migrations/0004_add_search_log.up.sql`、`migrations/0004_add_search_log.down.sql` — 新檔，建立／刪除 `search_log`
- `makefile` — 新增 `test-integration` target（起本機 Docker `postgres:18.6-bookworm`，套 migration，跑整合測試）
- `internal/linebot/handler/message.go` — `helpCommand` 常數、呼叫 `LogSearch`、摘要分組、查到 0 筆時的 Quick Reply
- `internal/linebot/handler/message_test.go` — 新檔，測摘要文字、0 筆時的 Quick Reply 組合
- `internal/linebot/template/quickreply.go` — 新檔，`QuickReplyAction` 與 `WithQuickReply`
- `internal/linebot/template/quickreply_test.go` — 新檔
- `internal/linebot/template/help.go` — 說明卡改成新用法並附範例 Quick Reply
- `internal/linebot/template/help_test.go` — 新檔
- `docs/open-decisions.md` — 追加 OD-5、OD-6
- `CLAUDE.md` — 更新「架構」的 DB 描述與「常用指令」

---

## Part 1：全欄位查詢與穩定排序

### 現況

- `search.ParseCriteria`（`internal/linebot/search/parser.go`）的預設類型是 `TypeName`；只有 `@醫院 `、`@科別 ` 前綴會改成 `TypeHospital`、`TypeDepartment`。
- `Repository.Search`（`internal/database/repository.go`）依類型只比對單一欄位（`name`／`hospital`／`department` 的 `ILIKE`），資料查詢**沒有 `ORDER BY`**；Postgres 不保證沒有排序條件時跨頁的順序，所以 offset 分頁會重複或漏列。
- `SearchStats` 只有 `TotalCount`、`CurrentPage`、`TotalPages`、`HasMore`。
- `formatSummary`（`internal/linebot/handler/message.go:103`）的 `typeText` map 有 `search.TypeName: "醫師"`。
- repo 的測試全部不需要 DB；沒有整合測試。

### 目標

- 沒有加 `@` 的輸入（含只加城市的 `台北 陳`）解析為 `TypeAll`，同時比對 `name`、`hospital`、`department`。
- `TypeAll` 的結果排序：姓名符合（rank 0）→ 醫院符合（rank 1）→ 只有科別符合（rank 2），同一組內依 `id` 升冪。`TypeHospital`、`TypeDepartment` 依 `id` 升冪排序。
- `TypeAll` 時 `SearchStats.MatchCounts` 填入三組筆數，三者總和等於 `TotalCount`；其他類型的 `MatchCounts` 為零值。
- `make test-integration` 會起本機 Postgres、套全部 up migration、跑 `-tags integration` 的測試，結束後清掉 container 並回傳測試結果。

### 不可動的範圍

- `@醫院 `、`@科別 ` 前綴語法與城市清單 `cities` 不變（不加「臺」）。
- `selectColumns`、`MedicalPersonnel`、`LIMIT 10` 分頁大小、`HasMore`／`TotalPages` 的計算方式不變。
- `internal/reconcile/**`、`internal/linebot/session/store.go`、`internal/linebot/handler/postback.go` 不動。
- 預設的 `go test ./...` 仍然不需要 DB（整合測試只在 `-tags integration` 時編譯）。

### 實作步驟

> 預期中的紅燈：步驟 1 刪掉 `TypeName` 之後，到步驟 4、5、6 改完 `store_test.go`、`message.go`、`repository.go` 之前，`go build ./...` 與 `go vet ./...` 會因為還有地方引用 `search.TypeName` 而失敗。這是預期的，照順序做完即可，不要停下來除錯。

1. 在 `internal/linebot/search/types.go`：刪除 `TypeName SearchType = "name"`，新增 `TypeAll SearchType = "all"`，放在 const 區塊第一個，並加註解 `// TypeAll matches name, hospital, or department (default when no @ prefix).`。
2. 在 `internal/linebot/search/parser.go`：把 `searchType := TypeName` 改為 `searchType := TypeAll`；把 doc comment 的第三行 `//	[城市] 醫師名稱        e.g. 台北 陳` 改為 `//	[城市] 關鍵字          e.g. 台北 陳 / 三總 / 牙科（同時比對姓名、醫院、科別）`。
3. 在 `internal/linebot/search/parser_test.go`：把 `name search no city`、`name search with city` 兩個案例的 `wantType` 改為 `TypeAll`，名稱分別改為 `keyword search no city`、`keyword search with city`；新增案例 `{name: "hospital name without prefix", input: "三總", wantType: TypeAll, wantTerm: "三總", wantCity: nil}`。
4. 在 `internal/linebot/session/store_test.go`：把三處 `search.TypeName` 全部改為 `search.TypeAll`。
5. 在 `internal/linebot/handler/message.go` 的 `formatSummary`：把 `typeText` map 的 `search.TypeName: "醫師",` 改為 `search.TypeAll: "",`（分組摘要在 Part 3 做）。
6. 在 `internal/database/repository.go`：
   1. 在 `selectColumns` 下方新增：
      ```go
      // matchRankExpr ranks a row for TypeAll ordering: 0 = name match, 1 = hospital match,
      // 2 = department-only match. It references $1, which Search always binds to the term.
      const matchRankExpr = "CASE WHEN name ILIKE $1 THEN 0 WHEN hospital ILIKE $1 THEN 1 ELSE 2 END"
      ```
   2. 新增型別：
      ```go
      // MatchCounts breaks a TypeAll result count down by which field matched first.
      type MatchCounts struct {
      	Name       int
      	Hospital   int
      	Department int
      }
      ```
      並在 `SearchStats` 結尾新增欄位 `MatchCounts MatchCounts // populated only for search.TypeAll`。
   3. 在 `Search` 的 `switch criteria.SearchType`：刪除 `case search.TypeName:` 分支，新增
      `case search.TypeAll: whereClauses = append(whereClauses, fmt.Sprintf("(name ILIKE $%d OR hospital ILIKE $%d OR department ILIKE $%d)", argIdx, argIdx, argIdx))`。
   4. 把 count 查詢換成單一查詢，同時取總數與三組筆數：
      ```go
      countSQL := fmt.Sprintf(
      	"SELECT COUNT(*) AS total, "+
      		"COUNT(*) FILTER (WHERE %[1]s = 0) AS name_count, "+
      		"COUNT(*) FILTER (WHERE %[1]s = 1) AS hospital_count, "+
      		"COUNT(*) FILTER (WHERE %[1]s = 2) AS department_count "+
      		"FROM medical_personnel %[2]s",
      	matchRankExpr, where,
      )
      var counts struct {
      	Total      int `db:"total"`
      	Name       int `db:"name_count"`
      	Hospital   int `db:"hospital_count"`
      	Department int `db:"department_count"`
      }
      if err := r.db.Get(&counts, countSQL, args...); err != nil {
      	return nil, SearchStats{}, fmt.Errorf("repository: count: %w", err)
      }
      totalCount := counts.Total
      ```
   5. 資料查詢加上排序：在組 `dataSQL` 前加
      ```go
      orderBy := "id"
      if criteria.SearchType == search.TypeAll {
      	orderBy = matchRankExpr + ", id"
      }
      ```
      `dataSQL` 改為 `fmt.Sprintf("SELECT "+selectColumns+" FROM medical_personnel %s ORDER BY %s LIMIT 10 OFFSET $%d", where, orderBy, argIdx)`。
   6. 組 `stats` 後，若 `criteria.SearchType == search.TypeAll`，設定 `stats.MatchCounts = MatchCounts{Name: counts.Name, Hospital: counts.Hospital, Department: counts.Department}`。
   7. 把 `Search` 的 doc comment 改為 `// Search queries medical_personnel with ILIKE filtering, deterministic ordering, and offset-based pagination.`。
7. 在 `makefile`：
   1. 在既有變數下方新增：
      ```make
      IT_CONTAINER := popofinder-it
      IT_PORT := 55432
      IT_DB_URL := postgres://postgres:it@127.0.0.1:$(IT_PORT)/postgres?sslmode=disable
      ```
   2. 在 `.PHONY` 行加入 `test-integration`。
   3. 在檔尾新增 target（recipe 行以 tab 縮排）：
      ```make
      test-integration:
      	docker rm -f $(IT_CONTAINER) >/dev/null 2>&1 || true
      	docker run -d --rm --name $(IT_CONTAINER) -e POSTGRES_PASSWORD=it -p 127.0.0.1:$(IT_PORT):5432 postgres:18.6-bookworm >/dev/null
      	@for i in $$(seq 1 60); do docker exec $(IT_CONTAINER) pg_isready -h 127.0.0.1 -U postgres -q && break; sleep 1; done
      	@set -e; for f in migrations/*.up.sql; do docker exec -i $(IT_CONTAINER) psql -q -U postgres -v ON_ERROR_STOP=1 < $$f; done
      	TEST_DATABASE_URL='$(IT_DB_URL)' go test -tags integration -count=1 -v ./internal/database/; status=$$?; \
      	docker rm -f $(IT_CONTAINER) >/dev/null; exit $$status
      ```
      （`pg_isready -h 127.0.0.1` 走 TCP，避免在 entrypoint 初始化期間只聽 unix socket 的暫時 server 上誤判 ready。Part 2 會在這個 recipe 裡加上 0004 down 的檢查。）
8. 新增 `internal/database/repository_integration_test.go`，第一行 `//go:build integration`，`package database`，內容：
   1. helper `openIntegrationDB(t *testing.T) *Repository`：讀 `os.Getenv("TEST_DATABASE_URL")`，空字串時 `t.Fatal("TEST_DATABASE_URL is required (run: make test-integration)")`；用 `Connect` 連線，`t.Cleanup` 關閉；執行 `TRUNCATE medical_personnel RESTART IDENTITY`；回傳 `NewRepository(db)`。
   2. helper `seedPersonnel(t *testing.T, r *Repository, rows [][4]any)`：依序以 `INSERT INTO medical_personnel (city, hospital, department, name) VALUES ($1, $2, $3, $4)` 寫入，`department` 傳 `nil` 時為 NULL。
   3. helper `names(ps []MedicalPersonnel) []string` 回傳 `Name` 切片。
   4. 共用種子（虛構資料，依此順序插入，id 為 1–6）：
      | id | city | hospital | department | name |
      |---|---|---|---|---|
      | 1 | 台北 | 林口長庚 | 內科 | 王小明 |
      | 2 | 台北 | 林新醫院 | 牙科 | 陳大文 |
      | 3 | 台中 | 中山醫院 | 新陳代謝科 | 李四 |
      | 4 | 台北 | 和平醫院 | 家醫科 | 林一 |
      | 5 | 高雄 | 林園診所 | NULL | 張三 |
      | 6 | 台北 | 測試診所 | 小兒科 | 林二 |
   5. `TestSearchAllFieldsOrdersNameFirst`：種子後
      - `Search(search.Criteria{SearchType: search.TypeAll, SearchTerm: "林"}, 0)` 的 `names` 等於 `["林一", "林二", "王小明", "陳大文", "張三"]`，`TotalCount == 5`，`MatchCounts == MatchCounts{Name: 2, Hospital: 3, Department: 0}`。
      - `SearchTerm: "陳"` 的 `names` 等於 `["陳大文", "李四"]`，`MatchCounts == MatchCounts{Name: 1, Hospital: 0, Department: 1}`。
   6. `TestSearchAllFieldsWithCity`：種子後 `Search(search.Criteria{SearchType: search.TypeAll, SearchTerm: "林", City: &taipei}, 0)`（`taipei := "台北"`）的 `names` 等於 `["林一", "林二", "王小明", "陳大文"]`，`MatchCounts == MatchCounts{Name: 2, Hospital: 2, Department: 0}`。
   7. `TestSearchHospitalTypeOrdersByID`：種子後 `Search(search.Criteria{SearchType: search.TypeHospital, SearchTerm: "林"}, 0)` 的 `names` 等於 `["王小明", "陳大文", "張三"]`，`TotalCount == 3`，`MatchCounts == MatchCounts{}`。
   8. `TestSearchPaginationStable`：不用共用種子，改成先插入 `{台北, 甲乙診所, 內科, 丙}`（id 1，只有醫院符合），再依序插入 12 筆 `{台北, 某醫院, 內科, 甲01}` … `{台北, 某醫院, 內科, 甲12}`。`Search(TypeAll, "甲", offset 0)` 的 `names` 等於 `甲01`…`甲10`、`HasMore == true`；`offset 10` 的 `names` 等於 `["甲11", "甲12", "丙"]`、`HasMore == false`；兩頁合併共 13 個且沒有重複。
   全部斷言用 `github.com/stretchr/testify/require`（已在 go.mod 的 testify 內）。

---

## Part 2：查詢紀錄 `search_log`

> 依賴 Part 1（`search.TypeAll`、`make test-integration`）。

### 現況

- 只有 `handler/message.go:33` 的 `slog.Info("[Message]", "userId", userID, "text", text)` 記錄查詢；這份 log 隨 pod 重啟消失，而且沒有結果筆數。
- migrations 目前到 `0003`。`0004_seed_tpech_directory` 在 commit `e1dc02b`（2026-06-10 00:16）加入、13 分鐘後由 `d3a7217` 移除，兩者都是直接 commit 到 develop，沒觸發 CI 部署；production pod 2026-09-26 起的 migrate init container 只帶 0001–0003 也成功啟動（golang-migrate 在 DB 版本找不到對應檔案時會失敗），所以 production 停在版本 3，新檔可以用 `0004`。

### 目標

- 新增 `search_log` 表：欄位剛好是 `id, created_at, search_type, city, search_term, result_count`，**沒有任何使用者識別欄位**；`search_type` 只接受 `all`、`hospital`、`department`。
- `Repository.LogSearch(ctx, criteria, resultCount)` 寫入一列。
- 使用者傳文字訊息觸發查詢時（含 0 筆）寫一列；說明、回報、翻頁 postback、加好友都不寫。寫入失敗時只記 `slog.Error("log search", "err", err)`，照常回覆使用者。
- `make test-integration` 額外驗證 0004 down 可以執行。

### 不可動的範圍

- `slog.Info("[Message]", ...)` 那行不動（本刀不改 stdout log）。
- `internal/linebot/handler/postback.go` 不呼叫 `LogSearch`。
- `medical_personnel` 的 schema 不動。

### 實作步驟

1. 新增 `migrations/0004_add_search_log.up.sql`：
   ```sql
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
   ```
2. 新增 `migrations/0004_add_search_log.down.sql`：
   ```sql
   DROP TABLE IF EXISTS search_log;
   ```
3. 在 `internal/database/repository.go` 檔尾新增：
   ```go
   // LogSearch records one user-initiated search for coverage analysis. It deliberately
   // stores no user identifier (docs/open-decisions.md OD-5).
   func (r *Repository) LogSearch(ctx context.Context, criteria search.Criteria, resultCount int) error {
   	_, err := r.db.ExecContext(ctx,
   		"INSERT INTO search_log (search_type, city, search_term, result_count) VALUES ($1, $2, $3, $4)",
   		string(criteria.SearchType), criteria.City, criteria.SearchTerm, resultCount,
   	)
   	if err != nil {
   		return fmt.Errorf("repository: log search: %w", err)
   	}
   	return nil
   }
   ```
4. 在 `internal/linebot/handler/message.go` 的 `Handle`：在 `personnel, stats, err := repo.Search(criteria, 0)` 的 `if err != nil { return err }` 之後、`buildSearchResponse` 之前插入：
   ```go
   if err := repo.LogSearch(ctx, criteria, stats.TotalCount); err != nil {
   	slog.Error("log search", "err", err)
   }
   ```
5. 在 `makefile` 的 `test-integration` recipe：把 `TEST_DATABASE_URL=...; status=$$?; \` 與 `docker rm -f ...` 兩行之間插入一行（tab 縮排）
   `docker exec -i $(IT_CONTAINER) psql -q -U postgres -v ON_ERROR_STOP=1 < migrations/0004_add_search_log.down.sql || status=1; \`
6. 在 `internal/database/repository_integration_test.go` 新增：
   1. `TestLogSearchInsertsRow`：`r := openIntegrationDB(t)`，執行 `TRUNCATE search_log RESTART IDENTITY`；呼叫 `LogSearch(ctx, search.Criteria{SearchType: search.TypeAll, SearchTerm: "林", City: &taipei}, 4)` 與 `LogSearch(ctx, search.Criteria{SearchType: search.TypeHospital, SearchTerm: "三總"}, 0)`；以 `SELECT search_type, city, search_term, result_count, created_at IS NOT NULL AS has_ts FROM search_log ORDER BY id` 讀回，斷言兩列依序為 `("all", "台北", "林", 4, true)`、`("hospital", NULL, "三總", 0, true)`（`city` 用 `sql.NullString` 掃描）。
   2. `TestSearchLogHasNoUserColumn`：`SELECT column_name FROM information_schema.columns WHERE table_name = 'search_log' ORDER BY ordinal_position` 的結果剛好等於 `["id", "created_at", "search_type", "city", "search_term", "result_count"]`。
   3. `TestSearchLogRejectsUnknownType`：`INSERT INTO search_log (search_type, search_term, result_count) VALUES ('name', 'x', 0)` 必須回傳 error。

### Schema Impact

- 新表 `search_log`（migration `0004_add_search_log`），由 production 的 migrate init container 在部署時自動套用。
- 寫入者只有 `Repository.LogSearch`，唯一呼叫點是 `MessageHandler.Handle`；目前沒有讀取者（分析時人工下 SQL）。
- `medical_personnel` 不變。

---

## Part 3：摘要分組與 Quick Reply onboarding

> 依賴 Part 1（`TypeAll`、`MatchCounts`）。

### 現況

- 加好友（`handler/follow.go`）與輸入「幫助」／`help`（`handler/message.go:37`）都回 `template.HelpFlexMessage()`：一張靜態說明卡，列出 `[城市]@醫院 醫院名稱`、`[城市]@科別 科別名稱`、`[城市] 醫師名稱` 三種格式與支援城市，沒有 Quick Reply。
- 查到 0 筆時回「摘要文字＋『找不到相關資料』卡片」，摘要會出現 `目前顯示第 1 - 0 筆`，也沒有任何下一步引導。
- `buildSearchResponse` 由 `MessageHandler` 與 `PostbackHandler` 共用。

### 目標

- 說明卡改成新用法，並附 3 個範例 Quick Reply（點了直接送出範例查詢）；加好友與「幫助」共用同一則訊息，所以 `follow.go` 不用改。
- `TypeAll` 摘要列出非零的組別筆數；0 筆時摘要不顯示「目前顯示第…筆」。
- 查到 0 筆時，最後一則訊息附 Quick Reply：條件有收窄（有城市或用了 `@`）時先給「擴大搜尋」，最後一律給「使用說明」。

### 不可動的範圍

- `handler/follow.go`、`handler/postback.go`、`handler/handler.go` 不改。
- `DoctorsFlexMessage`、`NextPageFlexMessage`、`doctorBubble`、`buildFlexMessage` 的簽名與輸出不變（Quick Reply 用 `WithQuickReply` 疊加）。
- 說明卡的「支援城市」區塊、標題「使用說明」、`@醫院`／`@科別` 兩個既有格式區塊的文字不變。
- 「回報」／`report` 仍是 no-op。
- 範例只能用姓氏、醫院簡稱、科別，**不可用真實醫師全名**（OD-6）。

### State Enumeration

| 情境 | 條件 | 回覆 |
|---|---|---|
| 加好友 | `follow` 事件 | 說明卡＋QR［範例：陳／範例：三總／範例：台北 牙科］ |
| 說明 | 文字為「幫助」或 `help`（不分大小寫） | 同上 |
| 有結果、沒有下一頁 | `TotalCount > 0 && !HasMore` | 摘要＋卡片 carousel，無 QR |
| 有結果、有下一頁 | `HasMore` | 摘要＋carousel＋下一頁按鈕，無 QR |
| 0 筆、全台、全欄位 | `TotalCount == 0 && City == nil && SearchType == TypeAll` | 0 筆摘要＋找不到卡片＋QR［使用說明］ |
| 0 筆、條件有收窄、關鍵字 1–300 字 | `TotalCount == 0 && (City != nil \|\| SearchType != TypeAll) && 1 ≤ runes(SearchTerm) ≤ 300` | 0 筆摘要＋找不到卡片＋QR［擴大搜尋（送出 `SearchTerm`）／使用說明］ |
| 0 筆、條件有收窄、關鍵字空白 | 同上但 `SearchTerm == ""` | QR［使用說明］ |
| 0 筆、關鍵字超過 300 字 | `runes(SearchTerm) > 300` | QR［使用說明］（LINE message action 的 text 上限 300） |
| 翻頁 postback 回 0 筆 | `PostbackHandler` 經 `buildSearchResponse` | 規則同上（共用函式） |

已知限制（接受，不處理）：「擴大搜尋」送出的是解析後的 `SearchTerm`。如果關鍵字本身以城市名開頭（例如 `@醫院 台北101`），重送時會再被解析成城市＋關鍵字。

### 實作步驟

1. 新增 `internal/linebot/template/quickreply.go`：
   ```go
   package template

   import "encoding/json"

   // QuickReplyAction is a message-type quick reply button: tapping it sends Text as the user.
   // LINE limits Label to 20 characters and Text to 300.
   type QuickReplyAction struct {
   	Label string
   	Text  string
   }

   // WithQuickReply returns msg with a quickReply attached. LINE only shows the quick reply
   // of the last message in a reply, so callers attach it to the final message.
   func WithQuickReply(msg json.RawMessage, actions []QuickReplyAction) (json.RawMessage, error) {
   	var m map[string]json.RawMessage
   	if err := json.Unmarshal(msg, &m); err != nil {
   		return nil, err
   	}
   	items := make([]any, len(actions))
   	for i, a := range actions {
   		items[i] = map[string]any{
   			"type":   "action",
   			"action": map[string]any{"type": "message", "label": a.Label, "text": a.Text},
   		}
   	}
   	qr, err := json.Marshal(map[string]any{"items": items})
   	if err != nil {
   		return nil, err
   	}
   	m["quickReply"] = qr
   	return json.Marshal(m)
   }
   ```
2. 在 `internal/linebot/template/help.go`：
   1. 刪除「支援格式」box 中的姓名格式區塊：👤 `[城市] 醫師名稱`／💡 `例如：台北 陳` 那個 vertical box，以及它後面緊接的 `{"type": "separator", "margin": "lg"}`（也就是「支援城市」box 前面那條）。刪完後「支援城市」box 直接接在科別區塊的 separator 之後。
   2. 在「支援格式」標題 text 之後、醫院區塊之前，插入兩個新區塊，每個區塊後各接一個 `map[string]any{"type": "separator", "margin": "lg"}`。結構與既有區塊相同（`box` vertical、`spacing: "sm"`，內含兩個 `layout: "baseline"` 的列；圖示 text `size: "sm"`、`color: "#666666"`、`flex: 1`；說明 text `size: "sm"`、`color: "#666666"`、`flex: 5`），新區塊的說明 text 另外加 `"wrap": true`：
      - 區塊 A：列 1 圖示 `🔍`、文字 `關鍵字（姓名、醫院或科別）`；列 2 圖示 `💡`、文字 `例如：陳、三總、牙科`
      - 區塊 B：列 1 圖示 `📍`、文字 `城市 關鍵字`；列 2 圖示 `💡`、文字 `例如：台北 牙科`
   3. 在 `helpContents` 下方新增：
      ```go
      // helpQuickReply offers tappable example searches. Examples use a surname, a hospital
      // short name, and a department only — never a real doctor's full name (OD-6).
      var helpQuickReply = []QuickReplyAction{
      	{Label: "範例：陳", Text: "陳"},
      	{Label: "範例：三總", Text: "三總"},
      	{Label: "範例：台北 牙科", Text: "台北 牙科"},
      }
      ```
   4. 把 `HelpFlexMessage` 改為：
      ```go
      // HelpFlexMessage returns the raw JSON for the help usage message with example quick replies.
      func HelpFlexMessage() (json.RawMessage, error) {
      	msg, err := buildFlexMessage("使用說明", helpContents)
      	if err != nil {
      		return nil, err
      	}
      	return WithQuickReply(msg, helpQuickReply)
      }
      ```
3. 在 `internal/linebot/handler/message.go`：
   1. 在 import 加入 `"unicode/utf8"`。
   2. 在 `MessageHandler` 型別上方新增：
      ```go
      // helpCommand is the text that shows the help card; quick replies send it too.
      const helpCommand = "幫助"

      // maxQuickReplyTextRunes is LINE's limit for a message action's text.
      const maxQuickReplyTextRunes = 300
      ```
   3. 把 `if lower == "幫助" || lower == "help" {` 改為 `if lower == helpCommand || lower == "help" {`。
   4. 新增函式：
      ```go
      // zeroResultActions suggests next steps when a search finds nothing: widen a narrowed
      // search (city or @ type) to an all-field nationwide search, and always offer help.
      func zeroResultActions(criteria search.Criteria) []template.QuickReplyAction {
      	var actions []template.QuickReplyAction
      	n := utf8.RuneCountInString(criteria.SearchTerm)
      	narrowed := criteria.City != nil || criteria.SearchType != search.TypeAll
      	if narrowed && n > 0 && n <= maxQuickReplyTextRunes {
      		actions = append(actions, template.QuickReplyAction{Label: "擴大搜尋", Text: criteria.SearchTerm})
      	}
      	return append(actions, template.QuickReplyAction{Label: "使用說明", Text: helpCommand})
      }
      ```
   5. 在 `buildSearchResponse`：在 `flexMsg, err := template.DoctorsFlexMessage(personnel)` 的錯誤檢查之後、`messages = append(messages, flexMsg)` 之前插入：
      ```go
      if stats.TotalCount == 0 {
      	flexMsg, err = template.WithQuickReply(flexMsg, zeroResultActions(criteria))
      	if err != nil {
      		return nil, err
      	}
      }
      ```
   6. 把 `formatSummary` 的 return 段（`start := ...` 到函式結尾）改為：
      ```go
      header := fmt.Sprintf("查詢%s%s「%s」", location, typeText, criteria.SearchTerm)
      if stats.TotalCount == 0 {
      	return header + "\n共有 0 筆符合的結果"
      }

      countLine := fmt.Sprintf("共有 %d 筆符合的結果", stats.TotalCount)
      if criteria.SearchType == search.TypeAll {
      	var groups []string
      	for _, g := range []struct {
      		label string
      		n     int
      	}{
      		{"姓名", stats.MatchCounts.Name},
      		{"醫院", stats.MatchCounts.Hospital},
      		{"科別", stats.MatchCounts.Department},
      	} {
      		if g.n > 0 {
      			groups = append(groups, fmt.Sprintf("%s %d", g.label, g.n))
      		}
      	}
      	countLine += "（" + strings.Join(groups, "、") + "）"
      }

      start := stats.CurrentPage*10 - 9
      end := stats.CurrentPage * 10
      if end > stats.TotalCount {
      	end = stats.TotalCount
      }
      return fmt.Sprintf("%s\n%s\n目前顯示第 %d - %d 筆", header, countLine, start, end)
      ```
4. 新增 `internal/linebot/template/quickreply_test.go`，`TestWithQuickReply`：對 `buildFlexMessage("alt", map[string]any{"type": "bubble"})` 套 `WithQuickReply(msg, []QuickReplyAction{{Label: "L", Text: "T"}})`，unmarshal 後斷言 `type == "flex"`、`altText == "alt"`、`contents.type == "bubble"` 都保留，且 `quickReply.items` 長度 1，`items[0].type == "action"`、`items[0].action == {"type": "message", "label": "L", "text": "T"}`。
5. 新增 `internal/linebot/template/help_test.go`：
   1. `TestHelpFlexMessageQuickReply`：unmarshal `HelpFlexMessage()`，斷言 `quickReply.items` 的 `action.text` 依序為 `["陳", "三總", "台北 牙科"]`，每個 `action.type == "message"`，每個 `action.label` 的 rune 數 ≤ 20。
   2. `TestHelpFlexMessageUsageText`：`HelpFlexMessage()` 的 JSON 字串包含 `關鍵字（姓名、醫院或科別）`、`例如：陳、三總、牙科`、`城市 關鍵字`、`例如：台北 牙科`、`[城市]@醫院 醫院名稱`、`[城市]@科別 科別名稱`、`支援城市`，並且**不包含** `[城市] 醫師名稱`。
6. 新增 `internal/linebot/handler/message_test.go`（`package handler`）：
   1. `TestFormatSummary`（table-driven，`taipei := "台北"`）：
      | criteria | stats | 期望字串 |
      |---|---|---|
      | `{TypeAll, "林", nil}` | `{TotalCount: 5, CurrentPage: 1, TotalPages: 1, MatchCounts: {2, 3, 0}}` | `查詢全台「林」\n共有 5 筆符合的結果（姓名 2、醫院 3）\n目前顯示第 1 - 5 筆` |
      | `{TypeAll, "陳", &taipei}` | `{TotalCount: 12, CurrentPage: 2, TotalPages: 2, MatchCounts: {10, 0, 2}}` | `查詢在台北「陳」\n共有 12 筆符合的結果（姓名 10、科別 2）\n目前顯示第 11 - 12 筆` |
      | `{TypeHospital, "三總", &taipei}` | `{TotalCount: 9, CurrentPage: 1, TotalPages: 1}` | `查詢在台北醫院「三總」\n共有 9 筆符合的結果\n目前顯示第 1 - 9 筆` |
      | `{TypeAll, "abc", nil}` | `{TotalCount: 0, CurrentPage: 1}` | `查詢全台「abc」\n共有 0 筆符合的結果` |
   2. `TestZeroResultActions`（table-driven），斷言回傳的 `[]template.QuickReplyAction`：
      | criteria | 期望 |
      |---|---|
      | `{TypeAll, "abc", nil}` | `[{使用說明, 幫助}]` |
      | `{TypeAll, "abc", &taipei}` | `[{擴大搜尋, abc}, {使用說明, 幫助}]` |
      | `{TypeHospital, "三總", nil}` | `[{擴大搜尋, 三總}, {使用說明, 幫助}]` |
      | `{TypeAll, "", &taipei}` | `[{使用說明, 幫助}]` |
      | `{TypeAll, strings.Repeat("字", 301), &taipei}` | `[{使用說明, 幫助}]` |
      | `{TypeAll, strings.Repeat("字", 300), &taipei}` | `[{擴大搜尋, <300 個字>}, {使用說明, 幫助}]` |
   3. `TestBuildSearchResponseQuickReply`：
      - 0 筆：`buildSearchResponse(search.Criteria{SearchType: search.TypeAll, SearchTerm: "abc", City: &taipei}, nil, database.SearchStats{TotalCount: 0, CurrentPage: 1})` 回 2 則訊息；第 1 則不含 `quickReply` 鍵；第 2 則的 `quickReply.items[*].action.text` 依序為 `["abc", "幫助"]`。
      - 有結果：`buildSearchResponse(search.Criteria{SearchType: search.TypeAll, SearchTerm: "林"}, []database.MedicalPersonnel{{Name: "測試", City: "台北", Hospital: "測試醫院"}}, database.SearchStats{TotalCount: 1, CurrentPage: 1, TotalPages: 1, MatchCounts: database.MatchCounts{Name: 1}})` 回 2 則訊息，兩則都不含 `quickReply` 鍵。

---

## Part 4：決策紀錄與文件指標

> 依賴 Part 1–3（文件描述的是做完的行為）。

### 現況

- `docs/open-decisions.md` 最後一條是 OD-4。
- `CLAUDE.md:22` 寫「目前單表 `medical_personnel`(...)」；`CLAUDE.md:29` 的測試指令只有 `go test ./...`。
- `docs/product-direction.md:92` 的「單一 `medical_personnel` 表」是定盤當下的現況紀錄（該文件開頭註明「寫於決策當下」），屬歷史紀錄，**不改**。

### 目標

- `docs/open-decisions.md` 追加 OD-5、OD-6；`CLAUDE.md` 的 DB 描述包含 `search_log`，常用指令包含 `make test-integration`。

### 不可動的範圍

- OD-1～OD-4 原文不動；`docs/product-direction.md` 不動。

### 實作步驟

1. 在 `docs/open-decisions.md` 檔尾追加（前面空一行）：
   ```markdown
   ## OD-5 — 查詢紀錄(search_log):存查詢內容,不存使用者

   - **日期**:2026-10-12
   - **狀態**:✅ 採納
   - **決策**:新增 `search_log` 表,記錄每次使用者發起的查詢(時間、查詢類型、城市、查詢字串、結果筆數),翻頁不記。**不存 LINE userId,也不存它的雜湊。**
   - **理由**:
     - 定盤說價值來自覆蓋率與正確性([`product-direction.md`](product-direction.md) 第 1 節),但目前量不到「查不到的比例」,也不知道「查不到的是誰」。stdout log 隨 pod 重啟消失,無法累積。
     - 存查詢字串才分得出是「語法打錯」還是「名單裡沒有」——後者就是覆蓋率缺口。
     - 不存使用者,避免留下「某個人查過哪些醫師」的關聯;代價是算不出回訪率。可以接受:回訪不是「3 個人也有價值」這把尺的成功定義。

   ## OD-6 — 預設全欄位查詢、姓名符合優先;onboarding 範例不用真實醫師全名

   - **日期**:2026-10-12
   - **狀態**:✅ 採納
   - **決策**:沒加 `@醫院`/`@科別` 時同時比對姓名、醫院、科別,排序為姓名符合 → 醫院符合 → 科別符合,摘要列出各組筆數;`@` 語法保留為精確篩選。Quick Reply 範例只用姓氏(陳)、醫院簡稱(三總)、科別(牙科),沿用原說明卡的範例。
   - **理由**:
     - 第一次使用的人直接輸入醫院名,會被當成姓名查而得到 0 筆。2026-10-12 用部落格 1476 筆資料模擬:1033 個全名查詢有 1032 個結果不變,會混在一起的主要是單一姓氏(例如「林」多出林口長庚等醫院 33 筆),用姓名優先排序加分組摘要就能讓使用者看懂。
     - 不另外保留「`[城市] 姓名` 只查姓名」的舊行為:姓名符合排最前面,舊結果完整留在前段,只是後面多接醫院/科別符合的列(單一姓氏:陳 +17、林 +33、王 +4、李 +2、張與黃 +0)。要完全一致只有兩條路:一是猜輸入是不是人名(「三總」和人名從字面分不出來);二是「有姓名符合就只顯示姓名」,但它在 1684 個簡短查詢中會把 6 個查詢的醫院/科別結果藏起來,最嚴重的是「牙醫」——一筆姓名欄被誤填成「牙醫」的資料會蓋掉 395 筆牙醫診所。
     - 範例按鈕若用真實醫師全名,等於在每個新使用者面前點名某位醫師,違反倫理鐵律 1、2(中性呈現、不暗示判斷)。
   ```
2. 在 `CLAUDE.md`：
   1. 把第 22 行改為 `- DB:Postgres,透過 sqlx + pgx;兩張表:`medical_personnel`(id, city, hospital, department, name, education, source, verification_status, source_url)與 `search_log`(查詢紀錄,刻意不存使用者,見 open-decisions OD-5)。`
   2. 在第 29 行（`go test ./...`）之後新增一行：`- `make test-integration` — 起本機 Docker `postgres:18.6-bookworm`、套 migrations,跑 `-tags integration` 的 DB 整合測試(`internal/database`)。`

---

## 驗證計畫

| Part | 驗證方式 | 預期斷言 | 為什麼這個斷言能證明目標達成 |
|------|---------|---------|------------------------------|
| Part 1 | `internal/linebot/search/parser_test.go:TestParseCriteria` | `陳`、`台北 陳`、`三總` 解析為 `TypeAll`，`@醫院`／`@科別` 案例不變 | 直接證明「沒加 @ 的輸入走全欄位」且舊語法沒壞 |
| Part 1 | `internal/database/repository_integration_test.go:TestSearchAllFieldsOrdersNameFirst`（經 `make test-integration`） | 「林」依序為姓名符合 2 筆、再醫院符合 3 筆；counts = {2,3,0}；「陳」的科別符合（新陳代謝科）排在姓名之後 | 種子刻意讓醫院符合的列 id 較小，排序結果只可能來自 rank 而不是 id；同時在真 Postgres 上驗證 FILTER／CASE SQL |
| Part 1 | `repository_integration_test.go:TestSearchAllFieldsWithCity` | 加 `台北` 後排除高雄的林園診所，counts = {2,2,0} | 證明城市條件與全欄位 OR 正確組合（OR 有括號） |
| Part 1 | `repository_integration_test.go:TestSearchHospitalTypeOrdersByID` | `@醫院 林` 只回 3 筆醫院符合，依 id 排序，`MatchCounts` 為零值 | 證明 `@醫院` 語意沒有被全欄位污染，且 MatchCounts 只在 TypeAll 填 |
| Part 1 | `repository_integration_test.go:TestSearchPaginationStable` | 兩頁 10＋3 筆、不重複，id 最小的醫院符合列出現在最後 | 證明新增的 ORDER BY 讓 offset 分頁穩定，且排序跨頁一致 |
| Part 2 | `repository_integration_test.go:TestLogSearchInsertsRow` | 兩列依序寫入，城市 nil 存成 NULL，created_at 有值 | 證明 `LogSearch` 與 migration 0004 在真 DB 上可寫入 |
| Part 2 | `repository_integration_test.go:TestSearchLogHasNoUserColumn` | 欄位剛好 6 個且沒有使用者欄位 | 把 OD-5「不存使用者」釘成 schema 層的斷言，之後有人加欄位會紅燈 |
| Part 2 | `repository_integration_test.go:TestSearchLogRejectsUnknownType` | `search_type='name'` 寫入失敗 | 證明 CHECK 存在，且與移除 `TypeName` 一致 |
| Part 2 | `make test-integration > /tmp/popofinder-it.log 2>&1` | exit 0（含 0004 down 成功） | recipe 在 down 失敗時把 status 設成 1，exit 0 代表 up/down 都能執行 |
| Part 2 | `grep -q 'repo.LogSearch(ctx, criteria, stats.TotalCount)' internal/linebot/handler/message.go` | exit 0 | 證明查詢路徑有記錄；`MessageHandler` 依賴具體 `*database.Repository`，沒有 DB 無法做行為測試，所以用結構檢查綁定呼叫點 |
| Part 2 | `! grep -q 'LogSearch' internal/linebot/handler/postback.go` | exit 0 | 證明翻頁不重複計數 |
| Part 3 | `internal/linebot/handler/message_test.go:TestFormatSummary` | 4 個情境字串完全相等 | 直接對應「TypeAll 分組、0 組不列、0 筆不顯示目前顯示、@醫院 格式不變」 |
| Part 3 | `message_test.go:TestZeroResultActions` | 6 個情境符合 State Enumeration | 涵蓋收窄／未收窄、空字串、300／301 字邊界 |
| Part 3 | `message_test.go:TestBuildSearchResponseQuickReply` | 0 筆時只有最後一則有 QR 且 text 為 `["abc","幫助"]`；有結果時沒有 QR | 證明 QR 掛在 LINE 會顯示的最後一則，且「使用說明」送出的字串就是 help 指令常數 |
| Part 3 | `internal/linebot/template/help_test.go:TestHelpFlexMessageQuickReply` | 3 個範例 text 依序正確、label ≤ 20 字 | 證明加好友與幫助都會帶範例按鈕（`follow.go` 呼叫同一函式），且符合 LINE label 上限 |
| Part 3 | `help_test.go:TestHelpFlexMessageUsageText` | 含新用法文字、不含 `[城市] 醫師名稱` | 證明說明卡已改成全欄位用法，舊的誤導文字已移除 |
| Part 3 | `internal/linebot/template/quickreply_test.go:TestWithQuickReply` | 原欄位保留、quickReply 結構符合 LINE 格式 | 證明疊加 QR 不會破壞原本的 flex 訊息 |
| Part 4 | `grep -q '^## OD-5 — 查詢紀錄(search_log)' docs/open-decisions.md && grep -q '^## OD-6 — 預設全欄位查詢' docs/open-decisions.md` | exit 0 | 證明決策已追加 |
| Part 4 | `grep -q 'search_log' CLAUDE.md && grep -q 'make test-integration' CLAUDE.md && ! grep -q '目前單表' CLAUDE.md` | exit 0 | 證明 CLAUDE.md 的過時指標已更新 |

---

## 驗收

- [ ] `go build ./...`
- [ ] `go vet ./...`
- [ ] `go vet -tags integration ./...`
- [ ] `test -z "$(gofmt -l . | grep -v '^internal/reconcile/fetch.go$')"`（`internal/reconcile/fetch.go` 自 `7a6d215` 起就有 gofmt 對齊問題，屬本刀不可動範圍，排除）
- [ ] `go test ./...`
- [ ] (Part 1) `go test ./internal/linebot/search/ -run '^TestParseCriteria$' -v 2>&1 | grep -q -- '--- PASS: TestParseCriteria '`
- [ ] (Part 1, 2) `make test-integration > /tmp/popofinder-it.log 2>&1`
- [ ] (Part 1) `grep -q -- '--- PASS: TestSearchAllFieldsOrdersNameFirst' /tmp/popofinder-it.log`
- [ ] (Part 1) `grep -q -- '--- PASS: TestSearchAllFieldsWithCity' /tmp/popofinder-it.log`
- [ ] (Part 1) `grep -q -- '--- PASS: TestSearchHospitalTypeOrdersByID' /tmp/popofinder-it.log`
- [ ] (Part 1) `grep -q -- '--- PASS: TestSearchPaginationStable' /tmp/popofinder-it.log`
- [ ] (Part 2) `grep -q -- '--- PASS: TestLogSearchInsertsRow' /tmp/popofinder-it.log`
- [ ] (Part 2) `grep -q -- '--- PASS: TestSearchLogHasNoUserColumn' /tmp/popofinder-it.log`
- [ ] (Part 2) `grep -q -- '--- PASS: TestSearchLogRejectsUnknownType' /tmp/popofinder-it.log`
- [ ] (Part 1, 2) `! grep -q -- '--- FAIL' /tmp/popofinder-it.log`
- [ ] (Part 2) `grep -q 'repo.LogSearch(ctx, criteria, stats.TotalCount)' internal/linebot/handler/message.go`
- [ ] (Part 2) `! grep -q 'LogSearch' internal/linebot/handler/postback.go`
- [ ] (Part 3) `go test ./internal/linebot/handler/ -run '^TestFormatSummary$' -v 2>&1 | grep -q -- '--- PASS: TestFormatSummary '`
- [ ] (Part 3) `go test ./internal/linebot/handler/ -run '^TestZeroResultActions$' -v 2>&1 | grep -q -- '--- PASS: TestZeroResultActions '`
- [ ] (Part 3) `go test ./internal/linebot/handler/ -run '^TestBuildSearchResponseQuickReply$' -v 2>&1 | grep -q -- '--- PASS: TestBuildSearchResponseQuickReply '`
- [ ] (Part 3) `go test ./internal/linebot/template/ -run '^TestHelpFlexMessageQuickReply$' -v 2>&1 | grep -q -- '--- PASS: TestHelpFlexMessageQuickReply '`
- [ ] (Part 3) `go test ./internal/linebot/template/ -run '^TestHelpFlexMessageUsageText$' -v 2>&1 | grep -q -- '--- PASS: TestHelpFlexMessageUsageText '`
- [ ] (Part 3) `go test ./internal/linebot/template/ -run '^TestWithQuickReply$' -v 2>&1 | grep -q -- '--- PASS: TestWithQuickReply '`
- [ ] (Part 4) `grep -q '^## OD-5 — 查詢紀錄(search_log)' docs/open-decisions.md && grep -q '^## OD-6 — 預設全欄位查詢' docs/open-decisions.md`
- [ ] (Part 4) `grep -q 'search_log' CLAUDE.md && grep -q 'make test-integration' CLAUDE.md && ! grep -q '目前單表' CLAUDE.md`

---

## 需後續人工確認的事項（不在自動化驗收範圍）

這些項目需要 production 存取或實機，executor 不做，部署後由使用者確認：

1. **`popo_app` 能不能寫入 `search_log`**：新表的 owner 是 migrate init container 用的 `DATABASE_URL` 那個 role。如果那個 role 不是 `popo_app`，app 會寫入失敗（只記 log，不影響回覆）。部署後先在 LINE 查一次，再執行：
   `sshpi 'sudo k3s kubectl exec -n shared postgres-0 -- psql -U postgres -d popo -c "SELECT tableowner FROM pg_tables WHERE tablename = '\''search_log'\''; SELECT search_type, result_count, created_at FROM search_log ORDER BY id DESC LIMIT 3;"'`
   如果 owner 不是 `popo_app`、或查不到剛才那筆，執行 `GRANT SELECT, INSERT ON search_log TO popo_app; GRANT USAGE ON SEQUENCE search_log_id_seq TO popo_app;`。
2. **Quick Reply 在手機上的呈現**：加好友（或封鎖後重新加入）看歡迎卡下方的 3 個範例按鈕；查一個不存在的字（例如 `台北 zzz`），確認出現「擴大搜尋／使用說明」。

---

## 執行報告

### 摘要
| Part | 狀態 | 說明 |
|------|------|------|
| Part 1：全欄位查詢與穩定排序 | ✅ 完成 | `TypeAll` 取代 `TypeName`；全欄位 OR、姓名→醫院→科別排序、`MatchCounts`；新增 `make test-integration` 與 4 個整合測試 |
| Part 2：查詢紀錄 `search_log` | ✅ 完成 | migration 0004、`Repository.LogSearch`、`MessageHandler` 呼叫點；3 個整合測試、0004 down 在 make target 驗證 |
| Part 3：摘要分組與 Quick Reply onboarding | ✅ 完成 | `template.WithQuickReply`、說明卡新用法＋範例 QR、0 筆時擴大搜尋／使用說明、摘要分組 |
| Part 4：決策紀錄與文件指標 | ✅ 完成 | OD-5、OD-6 追加（內容直接從本文件抽出）；`CLAUDE.md` 更新 |

### ❌ 未通過的驗收項目
無

### 🔍 需人工確認的驗收項目
- **部署後**：`popo_app` 能否寫入 `search_log`（table owner 取決於 migrate 用的 `DATABASE_URL` role；指令見上方「需後續人工確認的事項」第 1 點）
- **部署後**：手機上 Quick Reply 呈現（同上第 2 點）

### 🤔 執行中的判斷決策
- **Phase 1**：repo 沒有 `docs/architecture/definition-of-done.md` 與 `docs/decisions/OPEN-DECISIONS.md`；對應文件是 `docs/open-decisions.md`，已讀，無與本 task 衝突的未決事項。
- **驗收 gofmt**：`internal/reconcile/fetch.go` 自 `7a6d215` 起就有一行 struct 欄位對齊不符 gofmt，屬本 task「不可動的範圍」（`internal/reconcile/**`）。決策：不修該檔，把驗收的 gofmt 指令改為排除這一個既有檔案。理由：最小變動，且不擴大 diff 範圍。
- **Part 3, Step 3.4**：spec 沒指定 `zeroResultActions` 的位置。決策：放在 `formatSummary` 之後（同檔案、相關函式相鄰）。

### ⚠️ 注意事項
- 新增 DB 整合測試慣例：`//go:build integration` + `TEST_DATABASE_URL`，由 `make test-integration` 起本機 Docker `postgres:18.6-bookworm`（與 production PG18.6 同版）、套全部 up migration、跑測試、再驗證 0004 down。預設 `go test ./...` 與 CI 仍不需 DB。
- 新表 `search_log`（migration 0004），刻意不存使用者識別；schema 由 `TestSearchLogHasNoUserColumn` 釘住。
- `search.TypeName` 移除；沒加 `@` 的查詢一律為 `TypeAll`（全欄位，姓名優先）。`Search` 現在所有類型都有 `ORDER BY`（TypeAll：match rank, id；其他：id），offset 分頁穩定。
- `template.WithQuickReply` 是疊加 Quick Reply 的唯一入口（說明卡與 0 筆回覆共用）。
