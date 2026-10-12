# Task：正確性與資料清理（臺／台、部落格時效標示、一次性清理腳本）

## 背景

官網資料上線後，剩下的錯誤多半出在部落格時期的資料：查詢時「臺」「台」不互通；部落格從 2025-03 起就不再更新，卡片卻沒有標示；資料裡還混有重複列，以及醫院欄寫著「在學」這類狀態的非執業者。本 task 處理其中能用規則解決、而且不含具名內容的部分。

不在本 task 內：
- **production 執行清理腳本**：PR merge 後由維護者執行。2026-10-12 在交易中試跑後 ROLLBACK，預期結果是非執業者刪 214 筆、未分類重複刪 43 筆、部落格重複 104 組刪 129 筆（77 筆內容有變），主表 3491 → 3105 筆，`hospital_directory` 1888 筆不動。
- **自由文字裡的私人評論**（約 20 筆，例如醫院欄寫借錢、家人職業）：逐筆人工清理。內容是具名資料，不進 repo，修改記在歷史表 `reason='cleanup:free-text'`。
- 長庚爬蟲（另開 task）。

### 涉及檔案

- `internal/linebot/search/parser.go` — `ParseCriteria` 先把「臺」換成「台」
- `internal/linebot/search/parser_test.go` — `TestParseCriteria` 加兩個案例
- `internal/database/repository.go` — `Search` 比對欄位時把「臺」換成「台」
- `internal/database/repository_integration_test.go` — 新增 `TestSearchMatchesBothTaiSpellings`
- `internal/linebot/template/doctor.go` — 部落格來源標示加上時間，資料來源列可換行
- `internal/linebot/template/doctor_test.go` — 新增 `TestDoctorBubbleBlogSourceShowsDate`
- `scripts/cleanup/2026-10-12-blog-cleanup.sql` — 新檔：一次性清理腳本
- `internal/database/cleanup_integration_test.go` — 新檔：`TestBlogCleanupScript`
- `docs/open-decisions.md` — 追加 OD-13
- `docs/ARCHITECTURE.md` — 追加一則架構決策

---

## Part 1：查詢時把「臺」當成「台」

### 現況

- `internal/linebot/search/parser.go` 的 `ParseCriteria` 用 `strings.HasPrefix(message, c)` 比對 `cities`（17 個縣市都寫「台」），查詢字串照使用者輸入保留。所以輸入「臺北 陳」時認不出縣市，整串「臺北 陳」都會被當成關鍵字。
- `internal/database/repository.go` 的 `Search` 用 `name ILIKE $n`、`hospital ILIKE $n`、`department ILIKE $n` 比對（`TypeAll` 三欄都比，`@醫院`／`@科別` 只比一欄）。`matchRankExpr` 也是直接用 `name ILIKE $1`、`hospital ILIKE $1`。
- 2026-10-12 的 production 資料：醫院欄有 36 筆含「臺」（部落格 34、未分類 2），9 家醫院兩種寫法並存，例如「臺北榮總」和「台北榮總」；`city` 欄全部是「台」。

### 目標

- `ParseCriteria` 解析前先把訊息中所有「臺」換成「台」：縣市認得出來，查詢字串也只會有「台」。
- `Search` 比對時把 `name`、`hospital`、`department` 經 `translate(<欄位>, '臺', '台')` 再做 `ILIKE`；`matchRankExpr` 用同一組欄位表示式。
- 資料本身不改，卡片照原文顯示。

### 不可動的範圍

- `tieBreakOrder`、`city = $n` 條件、`LogSearch`、`selectColumns` 不動。
- `internal/linebot/handler`、`internal/linebot/session` 不動。摘要顯示、`search_log` 和 session 存的都會是換成「台」之後的查詢字串，這是預期行為，記在 OD-13。
- 不加其他異體字的對應，只處理「臺」→「台」。

### 實作步驟

1. 在 `internal/linebot/search/parser.go` 的 `ParseCriteria` 函式最前面（`searchType := TypeAll` 之前）加入：
   ```go
   	// Data and users spell 台 and 臺 both ways; search with 台 only. Repository.Search
   	// reads 臺 as 台 in the columns it compares (docs/open-decisions.md OD-13).
   	message = strings.ReplaceAll(message, "臺", "台")
   ```
2. 在 `internal/database/repository.go` 的 `matchRankExpr` 宣告之前加入：
   ```go
   // Columns compared with 臺 read as 台: ParseCriteria already turned 臺 into 台 in the
   // search term, and stored data spells it both ways (docs/open-decisions.md OD-13).
   const (
   	nameMatchCol       = "translate(name, '臺', '台')"
   	hospitalMatchCol   = "translate(hospital, '臺', '台')"
   	departmentMatchCol = "translate(department, '臺', '台')"
   )
   ```
3. 把 `matchRankExpr` 的值改成 `"CASE WHEN " + nameMatchCol + " ILIKE $1 THEN 0 WHEN " + hospitalMatchCol + " ILIKE $1 THEN 1 ELSE 2 END"`，上方原有的註解保留不動。
4. 在 `Search` 的 `switch criteria.SearchType` 中：
   - `search.TypeAll` 改為 `fmt.Sprintf("(%[2]s ILIKE $%[1]d OR %[3]s ILIKE $%[1]d OR %[4]s ILIKE $%[1]d)", argIdx, nameMatchCol, hospitalMatchCol, departmentMatchCol)`
   - `search.TypeHospital` 改為 `fmt.Sprintf("%s ILIKE $%d", hospitalMatchCol, argIdx)`
   - `search.TypeDepartment` 改為 `fmt.Sprintf("%s ILIKE $%d", departmentMatchCol, argIdx)`
5. 在 `internal/linebot/search/parser_test.go` 的 `TestParseCriteria` 表格最後加兩個案例：
   - `name: "臺 city and hospital term"`，`input: "臺北@醫院 臺大醫院"`，`wantType: TypeHospital`，`wantTerm: "台大醫院"`，`wantCity: strPtr("台北")`
   - `name: "臺 city with keyword"`，`input: "臺中 陳"`，`wantType: TypeAll`，`wantTerm: "陳"`，`wantCity: strPtr("台中")`
6. 在 `internal/database/repository_integration_test.go` 的 `TestSearchOrdersByDepartmentThenName` 之後新增：
   ```go
   // 臺 and 台 match each other in every compared column, typed either way (OD-13).
   func TestSearchMatchesBothTaiSpellings(t *testing.T) {
   	r := openIntegrationDB(t)
   	seedPersonnel(t, r, [][4]any{
   		{"台北", "臺北榮民總醫院", "內科", "王小明"},
   		{"台北", "台北榮總", "心臟內科", "李小華"},
   		{"台中", "測試醫院", "臺式推拿科", "張小美"},
   		{"台中", "測試醫院", "內科", "張小臺"},
   	})

   	tests := []struct {
   		input string
   		want  []string
   	}{
   		{"臺北@醫院 臺北榮", []string{"王小明", "李小華"}},
   		{"@醫院 台北榮", []string{"王小明", "李小華"}},
   		{"@科別 臺式", []string{"張小美"}},
   		{"張小台", []string{"張小臺"}},
   	}
   	for _, tt := range tests {
   		got, _, err := r.Search(search.ParseCriteria(tt.input), 0)
   		require.NoError(t, err)
   		require.Equal(t, tt.want, names(got), "input %s", tt.input)
   	}
   }
   ```

---

## Part 2：部落格卡片標示資料時間

### 現況

- `internal/linebot/template/doctor.go` 的 `sourceLabel` 對 `source='blog'` 回傳「網路公開彙整」。`doctorBubble` 的資料來源列是 `row("資料來源", sourceLabel(p.Source), nil)`，值不換行。
- 上游部落格 2025-03-09 之後就沒再更新（Blogger feed `updated` 為 2025-03-10）。DB 裡的部落格資料全部來自這份凍結的內容，抽樣顯示醫院欄大多已過時。

### 目標

- `source='blog'` 的卡片「資料來源」顯示「網路公開彙整（2025 年 3 月前）」，值可以換行。
- `public_report`、`hospital_directory` 和 NULL 的標示不變。

### 視覺規格

| 元素 | 屬性 | 來源 |
|------|------|------|
| 資料來源的值 | `size: sm`、`color: #4A4A4A`、`flex: 4`（沿用 `row()`），另加 `wrap: true` | `doctorBubble` 內的 `row` 函式；醫院列、學歷列已經用 `wrap: true` |

### 不可動的範圍

- `row` 函式本身、其他列、`sourceLabel` 的其他分支不動。
- 說明卡（`help.go`）裡「網路公開彙整」的文字不動。

### 實作步驟

1. 在 `internal/linebot/template/doctor.go` 的 `sourceLabel` 中，把 `case "blog":` 的回傳值改為 `"網路公開彙整（2025 年 3 月前）"`，並在該行上方加註解 `// The blog stopped updating in 2025-03; its hospitals are often out of date (OD-13).`
2. 把 `sourceRow := row("資料來源", sourceLabel(p.Source), nil)` 改為 `sourceRow := row("資料來源", sourceLabel(p.Source), map[string]any{"wrap": true})`。
3. 在 `internal/linebot/template/doctor_test.go` 新增 `TestDoctorBubbleBlogSourceShowsDate`：
   - 以 `Source: sql.NullString{String: "blog", Valid: true}` 和 `Name: "測試醫師"`、`City: "台北市"`、`Hospital: "測試醫院"` 呼叫 `DoctorsFlexMessage`。
   - 把結果 unmarshal 成 `map[string]any`，沿頂層 `contents`（carousel）→ 其 `contents[0]`（bubble）→ `body` → `contents`（各列 box）找出 `contents[0].text` 為「資料來源」的那個 box。
   - 斷言它第二個子元素的 `text` 等於「網路公開彙整（2025 年 3 月前）」，且 `wrap` 等於 `true`。

---

## Part 3：一次性清理腳本與整合測試

### 現況

- 2026-10-12 的 production 資料（主表 3491 筆）：
  - 醫院欄是狀態而不是執業地點的有 214 筆（部落格 207、未分類 7），例如「在學」「就學中」「待實習」「國考尚未通過」「雙和醫院實習中」。
  - 128 筆未分類（`source IS NULL`）中，有 47 筆和某筆部落格資料的姓名、縣市相同，醫院、科別、學歷去掉標點和空白後也完全相同。成因是最早匯入時去掉了「/」等字元，reconcile 的 `normalizeKey` 只忽略空白，所以又新增了一次。
  - 部落格資料中，同名、同縣市、醫院去標點後相同的有 108 組 245 筆，全部是 2024-11-16 最早匯入的，是部落格本身重複收錄（不同人回報的同一位醫師），只有 19 組科別和學歷完全相同。
- repo 沒有 `scripts/` 目錄。歷史 trigger 會記錄每次 UPDATE／DELETE，`reason` 取自交易內的 `SET LOCAL popo.change_reason`；內容沒變的 UPDATE 不記。

### 目標

- 新增 `scripts/cleanup/2026-10-12-blog-cleanup.sql`，依序做三件事，每一步各自設定 `popo.change_reason`：
  1. 刪除非執業者（`cleanup:non-practitioner`）
  2. 刪除未分類重複列（`cleanup:unclassified-duplicate`）
  3. 合併部落格重複組（`cleanup:blog-merge`）
- 腳本不含 `BEGIN`／`COMMIT`，以 `psql -1 -v ON_ERROR_STOP=1 -f` 在單一交易中執行；重跑不會再有任何變動。
- `TestBlogCleanupScript` 用虛構資料驗證三條規則、官網資料完全不受影響，以及重跑不再變動。

### State Enumeration

| 列的狀況 | 腳本結果 |
|---------|---------|
| 非官網資料，醫院欄符合 `(就學\|在學\|實習\|畢業\|待業\|國考\|學生)` | 刪除（步驟 1） |
| 官網資料，醫院欄含「實習」 | 不動 |
| 未分類列，有部落格列的姓名、縣市相同，醫院、科別、學歷去標點空白後也相同 | 刪除（步驟 2），部落格列保留 |
| 未分類列，同姓名同醫院但學歷不同 | 不動 |
| 部落格列，同名同縣市同醫院（去標點空白）的組，組內有 2 筆以上 | 保留最小 id；科別依首次出現順序去重後以「、」串接；學歷去掉「被組內另一個學歷完整包含」的寫法，再依首次出現順序以「；」串接；其餘列刪除（步驟 3） |
| 部落格列，同名同醫院但縣市不同 | 各自成組，不合併 |
| 部落格列，組內只有自己 | 不動 |
| 官網資料，和部落格組同名同縣市同醫院 | 不動，不參與合併 |

### 不可動的範圍

- `source='hospital_directory'` 的列完全不碰。
- 不改 schema、不新增 migration。這個腳本不放進 `migrations/`，不會在部署時自動執行。
- 不修改 `cmd/reconcile`、`internal/reconcile`。

### 實作步驟

1. 建立 `scripts/cleanup/2026-10-12-blog-cleanup.sql`，內容完全如下：
   ```sql
   -- One-off cleanup of blog-era rows (docs/open-decisions.md OD-13). Run once, in one transaction:
   --   psql -1 -v ON_ERROR_STOP=1 -f scripts/cleanup/2026-10-12-blog-cleanup.sql
   -- Rows written by the hospital-directory crawler (source = 'hospital_directory') are never touched.
   -- Every change goes through the history trigger with its own popo.change_reason.

   -- Text compared without punctuation or spaces: the first import dropped "/" and similar
   -- characters that the blog keeps, so the same entry can differ only in those.
   CREATE OR REPLACE FUNCTION pg_temp.cleanup_norm(t text) RETURNS text
   LANGUAGE sql IMMUTABLE AS $$ SELECT regexp_replace(coalesce(t, ''), '[[:punct:][:space:]、，。／/．.]', '', 'g') $$;

   -- 1. Non-practitioners: the hospital field is a status (在學, 待實習, 國考尚未通過 …), not a workplace.
   SET LOCAL popo.change_reason = 'cleanup:non-practitioner';
   DELETE FROM medical_personnel
   WHERE source IS DISTINCT FROM 'hospital_directory'
     AND hospital ~ '(就學|在學|實習|畢業|待業|國考|學生)';

   -- 2. Unclassified rows that repeat a blog row except for punctuation and spaces; the blog row
   --    (with its source link) stays.
   SET LOCAL popo.change_reason = 'cleanup:unclassified-duplicate';
   DELETE FROM medical_personnel n
   WHERE n.source IS NULL
     AND EXISTS (
       SELECT 1 FROM medical_personnel b
       WHERE b.source = 'blog' AND b.name = n.name AND b.city = n.city
         AND pg_temp.cleanup_norm(b.hospital) = pg_temp.cleanup_norm(n.hospital)
         AND pg_temp.cleanup_norm(b.department) = pg_temp.cleanup_norm(n.department)
         AND pg_temp.cleanup_norm(b.education) = pg_temp.cleanup_norm(n.education));

   -- 3. Blog rows with the same name, city and hospital are one doctor listed several times.
   --    Keep the smallest id; departments are joined with 、 and educations with ；, both in
   --    first-seen order, dropping an education whose text is contained in another one.
   SET LOCAL popo.change_reason = 'cleanup:blog-merge';
   CREATE TEMP TABLE cleanup_blog_group ON COMMIT DROP AS
   SELECT * FROM (
     SELECT id,
            nullif(btrim(department), '') AS dept,
            nullif(btrim(education), '') AS edu,
            pg_temp.cleanup_norm(education) AS edu_norm,
            min(id) OVER w AS keep_id,
            count(*) OVER w AS group_size
     FROM medical_personnel
     WHERE source = 'blog'
     WINDOW w AS (PARTITION BY name, city, pg_temp.cleanup_norm(hospital))
   ) g
   WHERE group_size > 1;

   UPDATE medical_personnel m
   SET department = (
         SELECT string_agg(dept, '、' ORDER BY first_id)
         FROM (SELECT dept, min(id) AS first_id FROM cleanup_blog_group g
               WHERE g.keep_id = m.id AND dept IS NOT NULL GROUP BY dept) d),
       education = (
         SELECT string_agg(edu, '；' ORDER BY first_id)
         FROM (SELECT DISTINCT ON (edu_norm) edu, id AS first_id FROM cleanup_blog_group g
               WHERE g.keep_id = m.id AND g.edu_norm <> ''
                 AND NOT EXISTS (SELECT 1 FROM cleanup_blog_group o
                                 WHERE o.keep_id = g.keep_id AND o.edu_norm <> g.edu_norm
                                   AND strpos(o.edu_norm, g.edu_norm) > 0)
               ORDER BY edu_norm, id) e)
   WHERE m.id IN (SELECT keep_id FROM cleanup_blog_group);

   DELETE FROM medical_personnel
   WHERE id IN (SELECT id FROM cleanup_blog_group WHERE id <> keep_id);
   ```
2. 建立 `internal/database/cleanup_integration_test.go`，第一行 `//go:build integration`，`package database`，內容：
   - helper `runBlogCleanup(t *testing.T, r *Repository)`：`os.ReadFile("../../scripts/cleanup/2026-10-12-blog-cleanup.sql")`，`tx := r.db.MustBegin()`，`_, err := tx.Exec(string(script))`，`require.NoError(t, err)`，`require.NoError(t, tx.Commit())`。pgx 在沒有參數時走 simple protocol，可以一次執行多個語句。
   - `TestBlogCleanupScript`：
     1. `r := openIntegrationDB(t)`，用一個 `r.db.MustExec` 依序 INSERT 下表 13 列（欄位順序 `city, hospital, department, name, education, source`；`NULL` 代表 SQL NULL；id 依序是 1–13）：

        | id | city | hospital | department | name | education | source |
        |----|------|----------|-----------|------|-----------|--------|
        | 1 | 台北 | 在學中 | NULL | 王小明 | 波蘭甲醫學大學 | blog |
        | 2 | 台北 | 待實習 | 牙科 | 李小華 | 西班牙乙大學 | NULL |
        | 3 | 台北 | 丙診所/丁診所 | 牙科 | 張小美 | 西班牙乙大學牙醫系 | blog |
        | 4 | 台北 | 丙診所丁診所 | 牙科 | 張小美 | 西班牙乙大學牙醫系 | NULL |
        | 5 | 台北 | 丙診所丁診所 | 牙科 | 張小美 | 捷克丙大學 | NULL |
        | 6 | 新北 | 戊診所 | 牙醫 | 陳大文 | 大學 | blog |
        | 7 | 新北 | 戊 診所 | 牙科 | 陳大文 | 波蘭甲醫學大學 | blog |
        | 8 | 新北 | 戊診所. | 牙醫 | 陳大文 | 西班牙乙大學牙醫系 | blog |
        | 9 | 新北 | 戊診所 | 牙醫 | 陳大文 | 波蘭甲醫學大學 | blog |
        | 10 | 新北 | 戊診所 | 牙醫 | 林一 | 大學 | blog |
        | 11 | 新北 | 戊診所 | 牙醫 | 陳大文 | 國立甲大學醫學系 | hospital_directory |
        | 12 | 台北 | 甲實習醫院 | 內科 | 王小明 | 國立甲大學醫學系 | hospital_directory |
        | 13 | 台北 | 戊診所 | 牙醫 | 陳大文 | 大學 | blog |

     2. 定義 `type cleanedRow struct { ID int; Hospital, Department, Name, Education, Source string }`（db tag 分別為 `id`、`hospital`、`department`、`name`、`education`、`source`），以及讀取函式：`SELECT id, hospital, coalesce(department, '') AS department, name, coalesce(education, '') AS education, coalesce(source, '') AS source FROM medical_personnel ORDER BY id`。
     3. 定義 `type reasonCount struct { Reason, Op string; N int }`（db tag `reason`、`op`、`n`），以及讀取函式：`SELECT reason, op, count(*) AS n FROM medical_personnel_history WHERE reason LIKE 'cleanup:%' GROUP BY reason, op ORDER BY reason, op`。
     4. 呼叫 `runBlogCleanup`，斷言資料列等於：
        - `{3, "丙診所/丁診所", "牙科", "張小美", "西班牙乙大學牙醫系", "blog"}`
        - `{5, "丙診所丁診所", "牙科", "張小美", "捷克丙大學", ""}`
        - `{6, "戊診所", "牙醫、牙科", "陳大文", "波蘭甲醫學大學；西班牙乙大學牙醫系", "blog"}`
        - `{10, "戊診所", "牙醫", "林一", "大學", "blog"}`
        - `{11, "戊診所", "牙醫", "陳大文", "國立甲大學醫學系", "hospital_directory"}`
        - `{12, "甲實習醫院", "內科", "王小明", "國立甲大學醫學系", "hospital_directory"}`
        - `{13, "戊診所", "牙醫", "陳大文", "大學", "blog"}`
     5. 斷言歷史統計等於 `{cleanup:blog-merge, DELETE, 3}`、`{cleanup:blog-merge, UPDATE, 1}`、`{cleanup:non-practitioner, DELETE, 2}`、`{cleanup:unclassified-duplicate, DELETE, 1}`。
     6. 再呼叫一次 `runBlogCleanup`，斷言資料列與歷史統計都和第 4、5 步完全相同（重跑不再變動）。

---

## Part 4：OD-13 與 ARCHITECTURE.md

（依賴 Part 1–3：內容描述的是它們的結果。）

### 現況

- `docs/open-decisions.md` 的最後一條是 OD-12。
- `docs/ARCHITECTURE.md` 的 `## Recent Architectural Decisions` 下，第一則是「2026-10-12 — 查詢排序改為科別 → 姓名 → id；「醫院清單」指令」。

### 目標

- `docs/open-decisions.md` 結尾追加 OD-13，記錄本 task 的決策、刻意不做的項目和理由。
- `docs/ARCHITECTURE.md` 的 `## Recent Architectural Decisions` 最上方加一則。

### 不可動的範圍

- 已有的 OD-1 到 OD-12 一個字都不改（規則：只追加）。
- `docs/ARCHITECTURE.md` 的既有條目不改。

### 實作步驟

1. 在 `docs/open-decisions.md` 結尾（OD-12 之後空一行）追加以下內容：
   ```markdown
   ## OD-13 — 資料清理:臺/台查詢互通、部落格時效標示、刪除非執業者與重複列

   - **日期**:2026-10-12
   - **狀態**:✅ 採納
   - **決策**:
     - 查詢時「臺」當成「台」:`ParseCriteria` 先把輸入的「臺」換成「台」,`Search` 比對姓名、醫院、科別時也把欄位裡的「臺」當「台」。資料本身不改,卡片照原文顯示;摘要、`search_log` 和 session 存的是換過的查詢字串。
     - 部落格資料的卡片「資料來源」顯示「網路公開彙整（2025 年 3 月前）」。
     - 一次性清理(`scripts/cleanup/2026-10-12-blog-cleanup.sql`,維護者以 `psql -1` 執行,每步各有 `popo.change_reason`;官網資料不碰):
       - 刪除非執業者:醫院欄是「在學」「就學中」「待實習」「國考尚未通過」「○○醫院實習」等狀態,沒有執業地點的列。
       - 刪除未分類重複列:與某筆部落格列的姓名、縣市相同,醫院、科別、學歷去掉標點空白後也相同的未分類列;保留有來源連結的部落格列。
       - 合併部落格重複:同名、同縣市、醫院去標點空白後相同的部落格列合併成一筆(保留最小 id),科別去重後以「、」串接,學歷去掉被另一個寫法完整包含的寫法後以「；」串接。
     - 自由文字中的私人評論、家人資訊、地址與經歷段落,逐筆人工改成只剩院所名稱與學經歷事實,歷史表記 `reason='cleanup:free-text'`;修改內容含具名資料,不進 repo。
     - 不做:
       - 官網資料與部落格同名、同醫院體系的列不合併。
       - 雙和官網本身同一人兩個 ID 的 3 組不處理。
       - 醫院名稱寫法不統一。
     - 清理後不要再跑 `/reconcile`:被刪除與合併的列對不上部落格,會被當成新資料重新寫入(CronJob 已停用)。
   - **理由**:
     - 2026-10-12 資料:醫院欄有 36 筆含「臺」(部落格 34、未分類 2),9 家醫院兩種寫法並存,查「台北榮總」查不到「臺北榮總」;`city` 全部是「台」,但使用者輸入「臺北」時原本認不出縣市。只在查詢時比對,資料與之後爬蟲寫入的原文都不用改。
     - 部落格 2025-03-09 之後不再更新,抽樣 14 位名單醫師只有 4 位確定還在部落格寫的單位;卡片不標時間會讓人以為醫院是現況(倫理鐵律 3、第 7 節「資料時效」)。
     - 非執業者 214 筆(部落格 207、未分類 7):病人不會在任何院所遇到他們,對查詢沒有價值,卻曝光了還不是醫師的人的學歷與狀態。刪除的內容留在歷史表;日後執業了,官網爬蟲自然會收到。
     - 未分類重複:128 筆未分類中有 47 筆與部落格列只差標點空白(其中 4 筆同時是非執業者),是最早匯入時去掉「/」等字元、reconcile 只忽略空白而又新增一次造成的。其餘未分類列多半是民眾回報,不動(不臆測來源)。
     - 部落格重複:108 組 245 筆,全部來自 2024-11-16 的最早匯入,是部落格本身重複收錄同一位醫師(例如同一間診所的同一位醫師有 4 筆,學歷寫法各不相同);只有 19 組科別與學歷完全相同。合併時保留每一種學歷寫法(只去掉被完整包含的),不丟資訊。風險是同一間診所剛好有同名的兩位醫師,機率極低。扣掉非執業者後為 104 組,合併後學歷最長 66 字、科別最長 19 字,在欄位上限內。
     - 試跑(交易內執行後 ROLLBACK)的預期結果:非執業者刪 214 筆、未分類重複刪 43 筆、部落格重複 104 組刪 129 筆(77 筆內容有變),主表 3491 → 3105 筆,官網資料 1888 筆不變。
     - 官網與部落格同名同醫院體系的 25 組不合併:15 組部落格寫國外學校、11 組官網寫國內醫學院,很可能是同名的不同人或其中一邊有誤;兩筆都標了來源,照實並列比較誠實。
     - 雙和 3 組沒有一致規則(1 組科別與學歷都相同,1 組只有科別相同,1 組只有學歷相同),而且爬蟲照官網寫,每月會再寫回來。
     - 醫院名稱(官網以外 795 種寫法)要逐一人工對照,改動大,先不做。
   ```
2. 在 `docs/ARCHITECTURE.md` 的 `## Recent Architectural Decisions` 標題與空行之後、「### 2026-10-12 — 查詢排序改為科別 → 姓名 → id；「醫院清單」指令」之前，插入：
   ```markdown
   ### 2026-10-12 — 臺／台查詢互通、`scripts/cleanup/` 一次性資料清理
   - 「臺」當「台」：`ParseCriteria` 先把輸入的「臺」換成「台」；`Search` 比對時用常數 `nameMatchCol`／`hospitalMatchCol`／`departmentMatchCol`（`translate(<欄位>, '臺', '台')`），`matchRankExpr` 也用同一組。資料不改。`TestSearchMatchesBothTaiSpellings` 涵蓋三個欄位與縣市前綴。
   - 新目錄 `scripts/cleanup/`：一次性資料清理腳本，只寫規則、不含具名內容；不放進 `migrations/`（不會在部署時自動執行），由維護者以 `psql -1 -v ON_ERROR_STOP=1 -f` 執行，每步各設 `popo.change_reason`。第一支 `2026-10-12-blog-cleanup.sql`（OD-13），`TestBlogCleanupScript` 用虛構資料驗證規則與重跑不再變動。
   - 部落格卡片的資料來源顯示「網路公開彙整（2025 年 3 月前）」，資料來源列改為可換行。

   ```

---

## 驗證計畫

| Part | 驗證方式 | 預期斷言 | 為什麼這個斷言能證明目標達成 |
|------|---------|---------|------------------------------|
| Part 1 | `internal/linebot/search/parser_test.go:TestParseCriteria`（新增的 `臺 city and hospital term`、`臺 city with keyword`） | 「臺北@醫院 臺大醫院」解析成 台北／醫院／「台大醫院」；「臺中 陳」解析成 台中／全欄位／「陳」 | 直接對應「輸入的臺換成台、縣市認得出來」的目標；修改前縣市是 nil，查詢字串保留「臺」 |
| Part 1 | `internal/database/repository_integration_test.go:TestSearchMatchesBothTaiSpellings` | 四個查詢分別回傳指定的醫師 | 涵蓋醫院欄兩種寫法、科別欄「臺」、姓名欄「臺」，以及「臺北」縣市前綴，證明三個比對欄位與輸入端都互通；任何一欄沒有 translate 都會少回傳 |
| Part 1 | `! grep -nE '(^\|[ "(])(name\|hospital\|department) ILIKE' internal/database/repository.go` | exit 0（沒有直接用欄位做 ILIKE 的地方） | 證明 WHERE 條件和 `matchRankExpr` 全部改用 translate 欄位，沒有漏改的比對路徑 |
| Part 2 | `internal/linebot/template/doctor_test.go:TestDoctorBubbleBlogSourceShowsDate` | 資料來源的值是「網路公開彙整（2025 年 3 月前）」且 `wrap` 為 true | 直接驗證卡片上的文字與換行設定，對應目標兩點 |
| Part 2 | `internal/linebot/template/doctor_test.go:TestDoctorBubbleOmitsSourceURLWhenEmpty`（既有） | 仍通過 | 證明部落格卡片仍然不顯示來源連結，其餘行為不變 |
| Part 3 | `internal/database/cleanup_integration_test.go:TestBlogCleanupScript` | 剩下的 7 列與合併後內容完全符合；歷史表各 reason 的筆數符合；重跑後兩者不變 | 依 State Enumeration 每一列都有對應的種子列：三條規則各自生效、官網列（含醫院欄有「實習」的）不受影響、不同縣市不合併；歷史表筆數證明每步的 reason 都有設定；重跑不變證明可以安全重跑 |
| Part 3 | `test "$(grep -c "SET LOCAL popo.change_reason = 'cleanup:" scripts/cleanup/2026-10-12-blog-cleanup.sql)" = 3 && ! grep -nE '^\s*(BEGIN\|COMMIT)\s*;' scripts/cleanup/2026-10-12-blog-cleanup.sql` | exit 0 | 三步各有自己的 reason，而且腳本把交易交給 `psql -1` 處理，試跑時才能包在外層交易裡再 ROLLBACK |
| Part 4 | `grep -q '^## OD-13 ' docs/open-decisions.md && grep -q 'scripts/cleanup/2026-10-12-blog-cleanup.sql' docs/ARCHITECTURE.md` | exit 0 | 決策與架構文件都有記錄，對應 Part 4 目標 |

---

## 驗收

- [ ] `go build ./...`
- [ ] `go vet ./...`
- [ ] `go vet -tags integration ./internal/database`
- [ ] `go test ./... -count=1`
- [ ] `test -z "$(gofmt -l $(find . -name '*.go' -not -path './internal/reconcile/fetch.go'))"`
- [ ] (Part 1) `go test ./internal/linebot/search -run '^TestParseCriteria$' -count=1 -v 2>&1 | grep -q -- '--- PASS: TestParseCriteria/臺_city_and_hospital_term'`
- [ ] (Part 1) `go test ./internal/linebot/search -run '^TestParseCriteria$' -count=1 -v 2>&1 | grep -q -- '--- PASS: TestParseCriteria/臺_city_with_keyword'`
- [ ] (Part 1) `! grep -nE '(^|[ "(])(name|hospital|department) ILIKE' internal/database/repository.go`
- [ ] (Part 2) `go test ./internal/linebot/template -run '^TestDoctorBubbleBlogSourceShowsDate$' -count=1 -v 2>&1 | grep -q -- '--- PASS: TestDoctorBubbleBlogSourceShowsDate'`
- [ ] (Part 2) `go test ./internal/linebot/template -run '^TestDoctorBubbleOmitsSourceURLWhenEmpty$' -count=1 -v 2>&1 | grep -q -- '--- PASS: TestDoctorBubbleOmitsSourceURLWhenEmpty'`
- [ ] (Part 1、3) `make test-integration > /tmp/popo-it.log 2>&1`
- [ ] (Part 1) `grep -q -- '--- PASS: TestSearchMatchesBothTaiSpellings' /tmp/popo-it.log`
- [ ] (Part 3) `grep -q -- '--- PASS: TestBlogCleanupScript' /tmp/popo-it.log`
- [ ] (Part 3) `test "$(grep -c "SET LOCAL popo.change_reason = 'cleanup:" scripts/cleanup/2026-10-12-blog-cleanup.sql)" = 3 && ! grep -nE '^\s*(BEGIN|COMMIT)\s*;' scripts/cleanup/2026-10-12-blog-cleanup.sql`
- [ ] (Part 4) `grep -q '^## OD-13 ' docs/open-decisions.md && grep -q 'scripts/cleanup/2026-10-12-blog-cleanup.sql' docs/ARCHITECTURE.md`

---

## 執行報告

### 摘要
| Part | 狀態 | 說明 |
|------|------|------|
| Part 1：查詢時把「臺」當成「台」 | ✅ 完成 | `ParseCriteria` 先換字；`Search` 與 `matchRankExpr` 改用 `nameMatchCol`／`hospitalMatchCol`／`departmentMatchCol`；兩個解析案例與 `TestSearchMatchesBothTaiSpellings` 通過 |
| Part 2：部落格卡片標示資料時間 | ✅ 完成 | 標示改為「網路公開彙整（2025 年 3 月前）」，資料來源列 `wrap: true`；`TestDoctorBubbleBlogSourceShowsDate` 通過 |
| Part 3：一次性清理腳本與整合測試 | ✅ 完成 | 腳本照 task 原文建立（與 2026-10-12 在本機及 production 試跑過的草稿逐字相同）；`TestBlogCleanupScript` 通過，含重跑不變 |
| Part 4：OD-13 與 ARCHITECTURE.md | ✅ 完成 | 照 task 原文追加 |

### ❌ 未通過的驗收項目
無

### 🔍 需人工確認的驗收項目
無

### 🤔 執行中的判斷決策
- **Phase 1**: skill 指定的 `docs/architecture/definition-of-done.md`、`docs/decisions/OPEN-DECISIONS.md` 在本 repo 不存在。決策: 改讀 `docs/open-decisions.md`（本 repo 的決策紀錄），沒有與未決項目衝突。
- **Part 2, Step 3**: task 沒指定斷言寫法。決策: 沿用 `doctor_test.go` 既有的 `t.Fatalf` 風格，不引入 testify。
- **Part 4, Step 2（gate 修正）**: task 指定的 ARCHITECTURE 文字寫「`scripts/cleanup/`」與「第一支 `2026-10-12-blog-cleanup.sql`」，但驗收要 grep 完整路徑 `scripts/cleanup/2026-10-12-blog-cleanup.sql`，兩者不一致，gate 第一次跑失敗。決策: 把「第一支」後面改成完整路徑；驗收命令不動。

### ⚠️ 注意事項
- production 執行清理腳本不在本 PR：merge 後由維護者先在交易內試跑核對數字（非執業者 214、未分類重複 43、部落格合併刪 129），再以 `psql -1 -v ON_ERROR_STOP=1 -f` 正式執行。
- 清理後不要再跑 `/reconcile`（OD-13）。
