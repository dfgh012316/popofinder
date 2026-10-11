# Task：資料異動歷史表與官網校對的 schema 地基

## 背景

2026-10-12 抽樣發現，醫院／診所官網能確認名單上約 3 成醫師的學歷，而且名單的醫院欄大多過期。接下來要用 SQL／腳本做官網校對與資料清理，但目前改了 `medical_personnel` 就沒有紀錄可以事後比對。這個 task 先打地基：
- 用 trigger 記錄每一次異動的歷史表。
- 一個「查到日期」欄位，卡片上顯示官網來源與日期。
- 把兩條決策（OD-7、OD-8）和盤點結果寫進文件。

寫入校對資料、資料清理、下架機制都不在這個 task 裡（理由見「本 task 不做」）。

### 涉及檔案

- `migrations/0005_add_history_and_checked_date.up.sql` — 新增：`source_checked_at` 欄位、`medical_personnel_history` 表、trigger function 與 trigger
- `migrations/0005_add_history_and_checked_date.down.sql` — 新增：反向移除
- `makefile` — `test-integration` 最後改為驗證「最新一支」down migration（原本寫死 0004）
- `internal/database/history_integration_test.go` — 新增：trigger 的整合測試（`//go:build integration`）
- `internal/database/repository_integration_test.go` — `openIntegrationDB` 同時清空歷史表；新增 `TestSearchReturnsSourceCheckedAt`
- `internal/database/repository.go` — `selectColumns` 與 `MedicalPersonnel` 加 `source_checked_at`
- `internal/database/repository_test.go` — `TestSelectColumnsIncludeNewFields` 加檢查 `source_checked_at`
- `internal/linebot/template/doctor.go` — `sourceLabel` 對應 `hospital_directory`；來源連結文字附查到日期
- `internal/linebot/template/doctor_test.go` — 新增卡片測試
- `docs/open-decisions.md` — 追加 OD-7、OD-8
- `docs/sources/official-pages-inventory.md` — 新增：官網學歷盤點（只寫來源層級與數字）
- `CLAUDE.md` — 「架構」段的 DB 描述改為三張表
- `docs/tasks/personnel-history-and-official-source.md` — 本 task 文件

### 本 task 不做（依據）

- **寫入官網校對資料、資料清理（臺／台等）**：這些是部署後對 production DB 的操作，需要使用者放行，不在 repo 變更範圍內（2026-10-12 對話決定：先建地基，部署後再操作）。
- **下架機制本身**（收件、流程、bot 指令）：OD-8 只規定歷史表的設計不能讓下架做不到，由 `TestHistoryAllowsTakedownPurge` 釘住。
- **新增 `official_site` 來源值**：使用者 2026-10-12 決定沿用 0003 已有的 `hospital_directory`（production 0 筆、程式沒有引用），醫院與診所的官網都用它。所以 `source` 的 CHECK 不動。
- **reconcile**：CronJob 已由 jerrytech-deploy PR #23 停用，`cmd/reconcile` 與 `internal/reconcile` 不動。

### Schema Impact

- **新欄位**：`medical_personnel.source_checked_at DATE NULL`，表示「`source_url` 指向的頁面在哪一天確認過」。既有資料一律保持 NULL，不回填。
- **新表**：`medical_personnel_history`。由 AFTER INSERT/UPDATE/DELETE FOR EACH ROW trigger 寫入，用 JSONB 存整列。`personnel_id` 不設 FK，主表刪了歷史還在。
- **下游消費者**：
  - `database.MedicalPersonnel`：新增 `SourceCheckedAt sql.NullTime`。
  - `selectColumns`：唯一的消費者是 `Search` 的 data query。
  - `template.doctorBubble`：讀 `SourceCheckedAt`。
  - `cmd/reconcile` 的 INSERT／UPDATE 不受影響：新欄位 nullable，trigger 會把它的寫入記成 reason NULL。CronJob 已停用。
  - `internal/linebot/handler/message_test.go:124` 的 struct literal 用具名欄位，不受影響。
- **Migration**：是，`migrations/0005_add_history_and_checked_date.{up,down}.sql`。production 的 `medical_personnel` owner 是 `popo_app`，`popo_app` 也有 public schema 的 CREATE 權限（2026-10-12 查證），migrate init container 能建 trigger 與 function。

### State Enumeration（卡片「來源連結」列）

| State | 條件 | 渲染結果 |
|-------|------|---------|
| 沒有連結 | `!SourceURL.Valid \|\| SourceURL.String == ""` | 不顯示「來源連結」列（不論 `SourceCheckedAt`），整張卡不出現「查到」 |
| 有連結、沒有日期 | URL 有值、`!SourceCheckedAt.Valid` | 值文字為 `點此查看來源`（與現況相同） |
| 有連結、有日期 | URL 有值、`SourceCheckedAt.Valid` | 值文字為 `點此查看來源（2026-10-12 查到）`，日期格式 `2006-01-02`，括號為全形 |

`sourceLabel`：

| `source` | 顯示 |
|---|---|
| `blog` | 網路公開彙整（不變） |
| `public_report` | 民眾回報（不變） |
| `hospital_directory` | 醫療院所官網（新增） |
| NULL 或其他 | 尚未分類（不變） |

---

## Part 1：歷史表、trigger 與查到日期欄位（migration 0005）

### 現況

- `migrations/` 有 0001–0004。0003 已把 `source` CHECK 放寬為 `('blog', 'public_report', 'hospital_directory')`。
- server 從不寫入 `medical_personnel`。會寫的只有 `cmd/reconcile` 的 UPDATE／INSERT（已停用）和手動 SQL，所以異動完全沒有紀錄。
- `makefile` 的 `test-integration` 先套全部 up migration、跑 `go test -tags integration ./internal/database/`，最後寫死執行 `migrations/0004_add_search_log.down.sql`。
- `internal/database/repository_integration_test.go` 的 `openIntegrationDB` 只執行 `TRUNCATE medical_personnel RESTART IDENTITY`。

### 目標

- 每一次 INSERT／UPDATE／DELETE `medical_personnel`（含手動 SQL）都在 `medical_personnel_history` 留一列，內容是修改前後的整列 JSONB，以及交易內 `SET LOCAL popo.change_reason` 設定的原因。
- 內容完全沒變的 UPDATE 不記錄。
- 歷史表的設計允許 OD-8 的下架程序：刪除某位醫師的全部歷史後，寫一筆不含內容的 DELETE 紀錄。
- `make test-integration` 會驗證最新一支 down migration。

### 不可動的範圍

- `migrations/0001`–`0004` 的所有檔案（包括 0003 down 檔裡過時的「0004 down 移除資料」註解，那是既有問題，不在本 task 修）。
- `cmd/reconcile/`、`internal/reconcile/`。
- `search_log` 表與 `Repository.LogSearch`。
- `openIntegrationDB` 以外的既有整合測試函式內容。

### 實作步驟

1. 建立 `migrations/0005_add_history_and_checked_date.up.sql`，內容完全如下：

   ```sql
   -- 官網校對與資料異動歷史的 schema 地基,見 docs/open-decisions.md OD-7、OD-8。
   -- source_checked_at:source_url 指向的頁面在哪一天確認過;既有資料保持 NULL,不回填。
   ALTER TABLE medical_personnel
       ADD COLUMN source_checked_at DATE;

   -- 記錄 medical_personnel 的每一次異動(含手動 SQL),之後才能比對改了什麼、為什麼改。
   -- 批次修改前在交易內先 SET LOCAL popo.change_reason = '<原因>'(或 set_config('popo.change_reason', <原因>, true)),
   -- reason 欄會記下原因;未設定時為 NULL。personnel_id 刻意不設 FK,主表刪了歷史還在。
   -- 下架時要連同該 personnel_id 的歷史一起清除,只留一筆不含內容的紀錄,見 OD-8。
   CREATE TABLE medical_personnel_history (
       id           BIGSERIAL PRIMARY KEY,
       personnel_id INTEGER NOT NULL,
       op           TEXT NOT NULL CHECK (op IN ('INSERT', 'UPDATE', 'DELETE')),
       changed_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
       old_row      JSONB,
       new_row      JSONB,
       reason       TEXT
   );

   CREATE INDEX ix_medical_personnel_history_personnel_id ON medical_personnel_history (personnel_id);

   CREATE FUNCTION record_medical_personnel_history() RETURNS trigger
   LANGUAGE plpgsql AS $$
   DECLARE
       -- SET LOCAL 的交易結束後,同一條連線讀到的是空字串而不是 NULL,所以要 NULLIF。
       change_reason TEXT := NULLIF(current_setting('popo.change_reason', true), '');
   BEGIN
       IF TG_OP = 'INSERT' THEN
           INSERT INTO medical_personnel_history (personnel_id, op, new_row, reason)
           VALUES (NEW.id, TG_OP, to_jsonb(NEW), change_reason);
       ELSIF TG_OP = 'UPDATE' THEN
           IF OLD IS NOT DISTINCT FROM NEW THEN
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

   CREATE TRIGGER medical_personnel_history_trigger
       AFTER INSERT OR UPDATE OR DELETE ON medical_personnel
       FOR EACH ROW EXECUTE FUNCTION record_medical_personnel_history();
   ```

2. 建立 `migrations/0005_add_history_and_checked_date.down.sql`，內容完全如下：

   ```sql
   DROP TRIGGER IF EXISTS medical_personnel_history_trigger ON medical_personnel;
   DROP FUNCTION IF EXISTS record_medical_personnel_history();
   DROP TABLE IF EXISTS medical_personnel_history;
   ALTER TABLE medical_personnel DROP COLUMN IF EXISTS source_checked_at;
   ```

3. 在 `makefile` 的 `test-integration` target，把 `docker exec -i $(IT_CONTAINER) psql -q -U postgres -v ON_ERROR_STOP=1 < migrations/0004_add_search_log.down.sql || status=1; \` 這一行的 `migrations/0004_add_search_log.down.sql` 換成 `$$(ls migrations/*.down.sql | sort | tail -n 1)`，其餘字元不變。之後新增 migration 時，這裡不必再改。

4. 在 `internal/database/repository_integration_test.go` 的 `openIntegrationDB`：
   - 把 `db.MustExec("TRUNCATE medical_personnel RESTART IDENTITY")` 改成 `db.MustExec("TRUNCATE medical_personnel, medical_personnel_history RESTART IDENTITY")`。
   - 把函式上方註解的 `// and empties medical_personnel so each test starts from known ids.` 改成 `// and empties medical_personnel and its history so each test starts from known ids.`。

5. 建立 `internal/database/history_integration_test.go`：第一行 `//go:build integration`，`package database`，沿用同 package 的 `openIntegrationDB`、`seedPersonnel`，不要另寫連線或 seed helper。內容依序為：

   a. `historyRow` 型別與 `historyRows(t *testing.T, r *Repository) []historyRow` helper（供 b–f 共用）：
      - 執行 `SELECT personnel_id, op, old_row->>'hospital' AS old_hospital, new_row->>'hospital' AS new_hospital, reason FROM medical_personnel_history ORDER BY id`。
      - 欄位：`PersonnelID int \`db:"personnel_id"\``、`Op string \`db:"op"\``、`OldHospital sql.NullString \`db:"old_hospital"\``、`NewHospital sql.NullString \`db:"new_hospital"\``、`Reason sql.NullString \`db:"reason"\``。
      - helper 開頭呼叫 `t.Helper()`。
      - 以下 c–g 的斷言裡，`OldHospital: 甲醫院` 這類寫法是簡寫，代表 `sql.NullString{String: "甲醫院", Valid: true}`；沒寫的欄位就是零值（NULL）。

   b. `TestHistoryRecordsInsert`：`seedPersonnel` 一筆 `{"台北", "甲醫院", "內科", "王一"}`。斷言 `historyRows` 等於 `[]historyRow{{PersonnelID: 1, Op: "INSERT", NewHospital: sql.NullString{String: "甲醫院", Valid: true}}}`，也就是 `OldHospital`、`Reason` 都是零值（NULL）。

   c. `TestHistoryRecordsUpdate`：seed 同上一筆，再 `r.db.MustExec("UPDATE medical_personnel SET hospital = '乙醫院' WHERE id = 1")`。斷言 `historyRows` 的第 2 列為 `{PersonnelID: 1, Op: "UPDATE", OldHospital: 甲醫院, NewHospital: 乙醫院}`（`Reason` NULL），總列數為 2。

   d. `TestHistorySkipsNoopUpdate`：seed 同上一筆，再 `r.db.MustExec("UPDATE medical_personnel SET hospital = hospital WHERE id = 1")`。斷言 `historyRows` 只有 1 列，而且是 INSERT。

   e. `TestHistoryRecordsDelete`：seed 同上一筆，再 `r.db.MustExec("DELETE FROM medical_personnel WHERE id = 1")`。斷言第 2 列為 `{PersonnelID: 1, Op: "DELETE", OldHospital: 甲醫院}`（`NewHospital`、`Reason` NULL）。

   f. `TestHistoryReasonFromSessionSetting`：seed 同上一筆。用 `conn, err := r.db.Connx(ctx)` 固定同一條連線，並 `defer conn.Close()`，然後依序：
      1. `tx, err := conn.BeginTxx(ctx, nil)`，在 tx 內執行 `SET LOCAL popo.change_reason = '資料清理'` 與 `UPDATE medical_personnel SET hospital = '乙醫院' WHERE id = 1`，再 `tx.Commit()`。
      2. 在同一個 `conn` 上（交易外）執行 `UPDATE medical_personnel SET hospital = '丙醫院' WHERE id = 1`。

      斷言 `historyRows` 中 `Op == "UPDATE"` 的兩列，`Reason` 依序為 `sql.NullString{String: "資料清理", Valid: true}` 與 `sql.NullString{}`。

   g. `TestHistoryAllowsTakedownPurge`：`seedPersonnel` 兩筆 `{"台北", "甲醫院", "內科", "王一"}`、`{"台北", "甲醫院", "內科", "王二"}`，再把 id 1 的 hospital UPDATE 成 `'乙醫院'`。接著執行 OD-8 的下架程序：`tx := r.db.MustBegin()`，在 tx 內依序執行
      - `DELETE FROM medical_personnel WHERE id = 1`
      - `DELETE FROM medical_personnel_history WHERE personnel_id = 1`
      - `INSERT INTO medical_personnel_history (personnel_id, op, reason) VALUES (1, 'DELETE', 'takedown')`

      然後 `require.NoError(t, tx.Commit())`。斷言：
      - `SELECT count(*) FROM medical_personnel_history WHERE personnel_id = 1 AND (old_row IS NOT NULL OR new_row IS NOT NULL)` 為 0。
      - `historyRows` 等於 `[]historyRow{{PersonnelID: 2, Op: "INSERT", NewHospital: 甲醫院}, {PersonnelID: 1, Op: "DELETE", Reason: takedown}}`。

   所有斷言使用 `github.com/stretchr/testify/require`（與 `repository_integration_test.go` 相同）。

6. 執行 `make test-integration`，確認 b–g 六個測試與既有整合測試全部通過，最後的 down migration（0005）也成功。
7. 在 `## 執行報告` 的「⚠️ 注意事項」寫明：`test-integration` 改成驗證最新一支 down migration。`docs/ARCHITECTURE.md` 2026-10-12 那條「再驗證 0004 down」的描述因此過時，收尾寫 ARCHITECTURE 新條目時要註明已改成驗證最新一支。

---

## Part 2：讀取查到日期、卡片顯示官網來源

> 依賴 Part 1：步驟 5 的整合測試要用到 0005 加的欄位。

### 現況

- `internal/database/repository.go`：
  - `selectColumns` 是 `"id, city, hospital, department, name, education, source, verification_status, source_url"`。
  - `MedicalPersonnel` 沒有查到日期欄位。
- `internal/linebot/template/doctor.go`：
  - `sourceLabel` 只認 `blog`、`public_report`，`hospital_directory` 會顯示「尚未分類」。
  - 有 `SourceURL` 時，「來源連結」列的值固定是 `點此查看來源`。
- `internal/database/repository_test.go` 的 `TestSelectColumnsIncludeNewFields` 檢查 `source`、`verification_status`、`source_url`。

### 目標

- `Search` 回傳的每一筆都帶 `SourceCheckedAt`。
- 卡片依上方 State Enumeration 顯示：`hospital_directory` 顯示為「醫療院所官網」；有連結也有日期時，連結文字附上「（YYYY-MM-DD 查到）」。

### 不可動的範圍

- `Search` 的 WHERE、ORDER BY、count 查詢與 `matchRankExpr`。
- 卡片其他列（縣市、醫院、科別、學歷、資料來源）的結構與樣式。
- 不顯示 `verification_status`（OD-4）。既有 `TestDoctorBubbleShowsSourceAndURL` 的「驗證狀態」斷言不動。
- `DoctorsFlexMessage`、`NextPageFlexMessage`、`buildFlexMessage` 的簽名。

### 實作步驟

1. 在 `internal/database/repository.go`：
   - 把 `selectColumns` 改成 `"id, city, hospital, department, name, education, source, verification_status, source_url, source_checked_at"`。
   - 在 `MedicalPersonnel` 的 `SourceURL` 欄位下方新增 `SourceCheckedAt sql.NullTime \`db:"source_checked_at"\``。
2. 在 `internal/database/repository_test.go` 的 `TestSelectColumnsIncludeNewFields` 末尾，用與既有三段相同的寫法，新增檢查 `source_checked_at` 的一段。
3. 在 `internal/linebot/template/doctor.go`：
   - `sourceLabel` 的 `switch` 在 `case "public_report":` 之後新增 `case "hospital_directory": return "醫療院所官網"`。
   - 在 `if p.SourceURL.Valid && p.SourceURL.String != ""` 區塊開頭宣告 `urlText := "點此查看來源"`；`if p.SourceCheckedAt.Valid` 時改成 `fmt.Sprintf("點此查看來源（%s 查到）", p.SourceCheckedAt.Time.Format("2006-01-02"))`。
   - 把 `row("來源連結", "點此查看來源", ...)` 的第二個參數改成 `urlText`。
   - 不要在其他地方另寫一份來源標籤或日期格式。
4. 在 `internal/linebot/template/doctor_test.go` 新增兩個測試（需 import `time`）：
   - `TestDoctorBubbleShowsCheckedDate`：
     - 卡片 A：`Source` 為 `hospital_directory`、`SourceURL` 為 `https://example.com/a`、`SourceCheckedAt` 為 `sql.NullTime{Time: time.Date(2026, 10, 12, 0, 0, 0, 0, time.UTC), Valid: true}`。斷言 JSON 含 `醫療院所官網` 與 `點此查看來源（2026-10-12 查到）`。
     - 卡片 B：同樣的 URL，但沒有 `SourceCheckedAt`。斷言 JSON 含 `"text":"點此查看來源"`（含結尾引號，確認沒有附日期），而且不含 `查到`。
   - `TestDoctorBubbleCheckedDateNeedsURL`：有 `SourceCheckedAt`、沒有 `SourceURL`。斷言 JSON 不含 `來源連結`，也不含 `查到`。
5. 在 `internal/database/repository_integration_test.go` 末尾新增 `TestSearchReturnsSourceCheckedAt`：
   - `r := openIntegrationDB(t)` 後執行：
     `r.db.MustExec("INSERT INTO medical_personnel (city, hospital, department, name, source, source_url, source_checked_at) VALUES ('台北', '甲醫院', '內科', '王一', 'hospital_directory', 'https://example.com/a', '2026-10-12'), ('台北', '甲醫院', '內科', '王二', 'blog', NULL, NULL)")`
   - 呼叫 `r.Search(search.Criteria{SearchType: search.TypeAll, SearchTerm: "王"}, 0)`，斷言：
     - 回傳 2 筆。
     - `got[0].SourceCheckedAt.Valid` 為 true，`got[0].SourceCheckedAt.Time.Format("2006-01-02") == "2026-10-12"`。
     - `got[0].Source.String == "hospital_directory"`。
     - `got[1].SourceCheckedAt.Valid` 為 false。
6. 執行 `go test ./...` 與 `make test-integration`。

---

## Part 3：決策與盤點文件

### 現況

- `docs/open-decisions.md` 最後一條是 OD-6。
- `docs/sources/` 不存在。
- `CLAUDE.md`「架構」段寫「兩張表：`medical_personnel`(…source_url)與 `search_log`」。

### 目標

OD-7、OD-8 追加進 open-decisions；盤點文件存在且不含任何醫師姓名；CLAUDE.md 的 DB 描述與 schema 一致。

### 不可動的範圍

- `docs/open-decisions.md` 既有 OD-1–OD-6 的文字。
- `docs/product-direction.md`。
- `CLAUDE.md` 除了「架構」段 DB 那一行以外的內容。
- **任何文件都不得寫入抽樣醫師的姓名、個人頁網址或「某位醫師是什麼學歷」**（repo 是 public，見 OD-7）。

### 實作步驟

1. 在 `docs/open-decisions.md` 檔尾（OD-6 之後）追加以下內容，逐字照抄：

   ```markdown

   ## OD-7 — 官網公開資料:半自動校對,不做全自動爬蟲

   - **日期**:2026-10-12
   - **狀態**:✅ 採納
   - **決策**:
     - 以「醫師」為單位校對:找出醫師所屬醫院/診所官網上寫了學歷的介紹頁,**人工確認是同一人**後才寫入 DB。第三方醫師名錄、預約平台只當線索(找醫師現在在哪),不當來源。
     - 只存事實:學歷、官網連結(`source_url`)、查到日期(`source_checked_at`),`source` 記為 `hospital_directory`(沿用 0003 的值,醫院與診所官網都用它,不另增 `official_site`)。**不轉錄頁面文字。**
     - 醫師已換執業地點時,依官網更新 `hospital`;修改前後由 `medical_personnel_history` 記錄(OD-8)。
     - `verification_status` 不因官網校對而改變:「官網公開」由 `source` 表達,`self_reported`/`verified` 的語意保留給醫師認領(OD-4)。
     - 優先校對 `search_log` 中實際被查過的醫師。
     - 不做全自動定期爬蟲;補名單外(例如台灣醫學系畢業)醫師另行決策。
     - **具名的校對結果只存在 DB,不寫進 repo**:repo 是 public,git 歷史無法下架,違反倫理鐵律 4。盤點文件只寫來源層級與數字。
   - **理由**:
     - 2026-10-12 抽樣 14 位名單醫師(見 [`sources/official-pages-inventory.md`](sources/official-pages-inventory.md)):官網寫了學歷且與名單一致 4 位、矛盾 0 位;找到介紹頁但沒寫醫學院 4 位;找不到 6 位。名單上的醫院有 706 種寫法、前 100 種只涵蓋 51% 的資料,加上醫師常換地方、同名難分,每家醫院寫解析器的維護成本與 side project 不成比例。
     - 名單的醫院欄過期比學歷錯誤嚴重:14 位中只有 4 位確定還在名單上的單位。校對可以同時更新執業地點——「過期資料比沒資料更傷信任」([`product-direction.md`](product-direction.md) 第 7 節)。
     - 不先「請醫師主動公開」:2024-11 公開學歷的爭議中,醫師公會全聯會理事長稱公開學歷「如同獵巫」、衛福部表示不會要求公開學歷,醫界至少是分裂的;而且自願公開的多半是台灣畢業的醫師,沒公開的人依鐵律 1 不能做任何暗示。醫院/診所官網已自行公開的學歷是現成的公開資料,醫療法第 85 條也允許醫療廣告登載學歷。
     - 優先校對被查過的醫師,符合「3 個人也有價值」這把尺:先讓真的有人查的那幾筆變準。

   ## OD-8 — 資料異動歷史表;下架時連同歷史清除

   - **日期**:2026-10-12
   - **狀態**:✅ 採納
   - **決策**:
     - 新增 `medical_personnel_history`,由 DB trigger 記錄 `medical_personnel` 每一次 INSERT/UPDATE/DELETE(含手動 SQL):修改前後整列(JSONB)與原因;內容沒變的 UPDATE 不記。
     - 批次修改前,在交易內先 `SET LOCAL popo.change_reason = '<原因>'`,`reason` 欄記下原因(例如「資料清理:臺→台」「官網校對」)。
     - **下架時**,在同一個交易內:刪除主表那一列 → 刪除該 `personnel_id` 的全部歷史 → 寫一筆不含內容的紀錄(`op='DELETE'`、`old_row`/`new_row` 為 NULL、`reason='takedown'`)。下架機制本身(收件、流程)尚未實作;本條只規定歷史表的設計不能讓下架做不到(`personnel_id` 不設 FK、`old_row`/`new_row` 可為 NULL),由 `TestHistoryAllowsTakedownPurge` 釘住。
     - TRUNCATE 不會觸發 row trigger、不留紀錄;production 不對 `medical_personnel` 下 TRUNCATE。
   - **理由**:
     - 之後的資料清理與官網校對都用 SQL/腳本修改,server 本身不寫這張表,只有 DB trigger 抓得到所有修改路徑;沒有歷史就無法事後比對「改了什麼、為什麼改」。
     - 歷史表保留完整資料,若下架只刪主表,醫師的資料仍留在歷史表,等於沒有下架(倫理鐵律 4)。留一筆不含內容的紀錄,仍看得出「這裡曾下架一筆」。
     - 整列存 JSONB:之後主表加欄位,trigger 與歷史表都不用改。既有資料不做初始快照:每列第一次被修改時,`old_row` 就記下修改前的狀態。
   ```

2. 建立 `docs/sources/official-pages-inventory.md`，內容逐字照抄：

   ```markdown
   # 醫院/診所官網學歷盤點(2026-10-12)

   從部落格名單 1476 筆以固定 seed 隨機抽 15 位(醫院 11、牙醫診所 4);其中 1 筆是「在學中」且姓名遮罩,不查,實際查 14 位。每位查四件事:官網介紹頁有沒有寫學歷、和名單是否一致、醫師是否還在名單上的單位、同頁其他醫師有幾位寫了學歷。

   分 3 路調查,全部只發讀取請求:沒有登入、送出表單、解驗證碼,也沒有繞過 Cloudflare 或 TLS 憑證檢查;每個網域最多 10 次,總共約 108 次請求。**具名結果不寫進 repo**(repo 是 public,見 [`../open-decisions.md`](../open-decisions.md) OD-7);本文件只記來源層級的觀察與數字。樣本只有 14 位,比例只能當量級參考。

   ## 1. 結論

   - 官網能確認名單學歷的約 3 成(4/14),沒有發現矛盾(0/14)。
   - 過期的是醫院欄:確定還在名單上單位的只有 4 位。確認學歷的 4 位裡,有 2 位是在新的執業地點找到的。
   - 解析頁面不難,難在找到醫師現在在哪、確認同名的是不是同一人 → 採半自動校對(OD-7)。

   ## 2. 抽樣結果

   | 官網狀況 | 人數 |
   |---|---|
   | 寫了學歷,與名單一致 | 4 |
   | 找到介紹頁,但沒寫醫學院 | 4 |
   | 找不到官網介紹頁 | 6 |
   | 與名單矛盾 | 0 |

   「沒寫醫學院」的四種情況:學歷欄只寫研究所碩士;「學經歷」混寫欄只有受訓經歷;個人頁沒有學歷欄;只寫國外碩士與「歐盟認證牙醫師」,且無法確認是否同一人。**有學歷欄不等於寫了醫學院。**

   | 執業地點 | 人數 |
   |---|---|
   | 還在名單上的單位 | 4 |
   | 確定已換地方 | 3 |
   | 疑似換地方(可能是同名的另一人) | 1 |
   | 找不到 | 6 |

   ## 3. 同頁醫師寫學歷的比例(名單外覆蓋率的參考)

   只計數,不記姓名。數字是「寫了任何學校或學位」,不一定是醫學系。

   | 頁面 | 寫學歷/總數 |
   |---|---|
   | 大里仁愛醫院 乳房外科列表 | 4/4 |
   | 義大醫院 新陳代謝科(掛號網科別頁) | 11/16 |
   | 義大醫院 牙科部 | 17/18 |
   | 4 個牙醫診所/聯盟的團隊頁 | 10/12、4/4、3/3、7/9 |
   | 高雄榮總 心臟內科、中山附醫 肝膽腸胃科 | 列表不顯示學歷;各抽 3 個個人頁,都有學歷欄 |
   | 北市聯醫仁愛院區 口腔醫學科 | 列表 0/10 顯示;個人頁因 TLS 憑證問題讀不到 |
   | 某牙醫聯盟的集團站團隊頁 | 卡片 0/114 顯示;學歷只在個人頁 |

   ## 4. 各來源的格式與限制

   | 來源 | 格式 | 觀察 |
   |---|---|---|
   | 高雄榮總 `org.vghks.gov.tw` | 伺服器渲染(ASP.NET) | 個人頁有學歷欄;網址參數是不透明的 hash,改版後舊網址(`/cv/`)整個 404;頁尾「未經允許請勿任意轉載、複製或做商業用途」 |
   | 台中榮總 `vghtc.gov.tw` | 需要 JS | 單位頁、醫師頁不跑 JS 時正文是空的;掛號班表頁回 HTTP 500 |
   | 中山附醫 `csh.org.tw` | 伺服器渲染(ASP.NET 與 WordPress 兩站) | `EmpCode` 是穩定的鍵;列表只列專任/兼任主治醫師,受訓中的醫師可能不在官網;學歷欄可能只寫研究所 |
   | 長庚 `cgmh.org.tw` | 伺服器渲染 | 個人頁只有經歷、專長、認證,沒有學歷欄(看了 1 頁);數字 ID 穩定;頁尾「未經授權,禁止轉載」 |
   | 大里仁愛醫院 `jah.org.tw` | 靜態 HTML(無障礙版) | 科別列表直接寫學歷 |
   | 義大醫院 `webreg.edah.org.tw` | 伺服器渲染 | 沒有獨立學歷欄,「學經歷」是混寫的自由文字,有人不寫學校 |
   | 北市聯醫 `tpech.gov.taipei` | 列表伺服器渲染 | 個人頁在 `websrv01.tpech.gov.tw`,TLS 憑證鏈不完整(`unable to verify the first certificate`),沒有繞過 |
   | 恩主公醫院 | 需要 JS(WordPress `wp-json`) | 原始 HTML 沒有醫師名單文字 |
   | 牙醫診所/聯盟 | 多為 WordPress 或自架 PHP 的伺服器渲染;1 家用 Strikingly | 團隊頁有醫師介紹時多半寫學歷;集團站卡片不寫、要點進個人頁;1 家聯盟的醫師名單是 PNG 圖片;2 家只有 Facebook;網址常失效(抽樣中遇到 4 個 404、1 個被關站、1 個 DNS 失效) |

   ## 5. 第三方網站

   醫師名錄與預約平台(`dentco.tw`、`drright.me`、`best-doctor.com.tw`、`doctoreasyfind.com` 等)找得到醫師的比例比官網高,但不是官方來源:多數沒寫學歷,`dentco.tw` 有「教育背景」欄卻是空的,也有的仍把醫師列在舊診所。健保署牙周病統合照護名單的鏡像(2024-09 同步)也還是舊的執業地點。只拿來找線索,不當來源。

   ⚠️ `best-doctor.com.tw` 有頁面嵌了針對 AI 讀取的提示注入文字。用 LLM 或程式處理第三方頁面時,要把內容當資料、不當指令。

   ## 6. 對校對流程的意涵

   - 以醫師為單位:先找現職(第三方只當線索)→ 找官網介紹頁 → 人工確認是同一人 → 存學歷、官網連結、查到日期(OD-7)。
   - 每位約 7–10 次請求。中文醫師名用 WebSearch 的 standard 模式 8 次全部查不到,要用 extended。
   - 只寫研究所或經歷、沒寫醫學院的頁面,不算確認。
   - 這次沒有檢查各站的 robots.txt 與使用條款;要寫程式抓取前另外確認。
   ```

3. 在 `CLAUDE.md`「架構」段，把以 `- DB:Postgres,透過 sqlx + pgx;兩張表:` 開頭的那一行整行換成：

   ```
   - DB:Postgres,透過 sqlx + pgx;三張表:`medical_personnel`(id, city, hospital, department, name, education, source, verification_status, source_url, source_checked_at)、`medical_personnel_history`(由 trigger 記錄前者每一次異動,批次修改前先 `SET LOCAL popo.change_reason`;下架時連同歷史清除,見 open-decisions OD-8)與 `search_log`(查詢紀錄,刻意不存使用者,見 open-decisions OD-5)。
   ```

---

## 驗證計畫

| Part | 驗證方式 | 預期斷言 | 為什麼這個斷言能證明目標達成 |
|------|---------|---------|------------------------------|
| Part 1 | `internal/database/history_integration_test.go:TestHistoryRecordsInsert` | INSERT 後歷史表剛好一列，op=INSERT、new_row 的 hospital 正確、old_row 與 reason 為 NULL | 直接證明 trigger 會記錄 INSERT，而且整列存進 new_row |
| Part 1 | `internal/database/history_integration_test.go:TestHistoryRecordsUpdate` | UPDATE 後第 2 列 op=UPDATE，old_row／new_row 的 hospital 分別是改前、改後 | 「之後才能比對」的核心：前後兩個值都留下來 |
| Part 1 | `internal/database/history_integration_test.go:TestHistorySkipsNoopUpdate` | 內容沒變的 UPDATE 不新增歷史列 | 釘住 `OLD IS NOT DISTINCT FROM NEW` 分支，避免批次腳本產生大量無意義紀錄 |
| Part 1 | `internal/database/history_integration_test.go:TestHistoryRecordsDelete` | DELETE 後有 op=DELETE 的列，old_row 保留被刪的整列 | 證明主表刪了，歷史還在（personnel_id 沒有 FK） |
| Part 1 | `internal/database/history_integration_test.go:TestHistoryReasonFromSessionSetting` | 交易內 SET LOCAL 的 UPDATE 記下「資料清理」；同一連線在交易外的 UPDATE reason 是 NULL 而不是空字串 | 證明 reason 機制有效，也覆蓋 SET LOCAL 結束後 `current_setting` 回空字串這個坑（NULLIF） |
| Part 1 | `internal/database/history_integration_test.go:TestHistoryAllowsTakedownPurge` | 執行 OD-8 下架程序後，該 personnel_id 沒有任何含內容的歷史列，只剩一筆 reason=takedown 的 DELETE；其他醫師的歷史不受影響 | 釘住「歷史表的設計不能讓下架做不到」：若日後有人加 FK 或把 old_row 設成 NOT NULL，這個測試會失敗 |
| Part 1 | `make test-integration` | exit 0（含最後執行 0005 down） | 證明 0005 up 能在全新 DB 套用、所有整合測試通過，down 也能乾淨移除 |
| Part 2 | `internal/database/repository_integration_test.go:TestSearchReturnsSourceCheckedAt` | `Search` 回傳的第一筆 SourceCheckedAt 為 2026-10-12、source 為 hospital_directory；第二筆 SourceCheckedAt 無效 | 證明 selectColumns、struct tag、pgx 對 DATE 的掃描整條路通，而且 `hospital_directory` 能通過既有 CHECK |
| Part 2 | `internal/linebot/template/doctor_test.go:TestDoctorBubbleShowsCheckedDate` | 有日期時文字為「點此查看來源（2026-10-12 查到）」、來源顯示「醫療院所官網」；沒日期時文字恰為「點此查看來源」 | 對應 State Enumeration 的「有連結有日期」「有連結沒日期」兩格，以及 sourceLabel 新增的那一格 |
| Part 2 | `internal/linebot/template/doctor_test.go:TestDoctorBubbleCheckedDateNeedsURL` | 有日期但沒連結時，不出現「來源連結」與「查到」 | 對應「沒有連結」那一格：日期不能脫離來源單獨出現 |
| Part 2 | `go test ./internal/database -run TestSelectColumnsIncludeNewFields -count=1` | exit 0 | 不連 DB 也能釘住 selectColumns 包含新欄位 |
| Part 3 | `grep -q '^## OD-7 — ' docs/open-decisions.md && grep -q '^## OD-8 — ' docs/open-decisions.md` | exit 0 | 兩條決策已追加，下一次產品決策回顧時讀得到 |
| Part 3 | `test -f docs/sources/official-pages-inventory.md && grep -q '^## 4. 各來源的格式與限制' docs/sources/official-pages-inventory.md` | exit 0 | 盤點文件存在且含來源格式表，後續寫校對腳本時有依據 |
| Part 3 | `grep -q 'medical_personnel_history' CLAUDE.md && ! grep -q '兩張表' CLAUDE.md` | exit 0 | CLAUDE.md 的 DB 描述已改成三張表，沒有殘留舊描述 |

---

## 驗收

- [ ] `go build ./...`
- [ ] `go vet ./...`
- [ ] `go test ./...`
- [ ] `test -z "$(gofmt -l $(find . -name '*.go' -not -path './internal/reconcile/fetch.go'))"`
- [ ] `test -f migrations/0005_add_history_and_checked_date.up.sql && test -f migrations/0005_add_history_and_checked_date.down.sql`
- [ ] `make test-integration`
- [ ] `test "$(grep -c -E '^func (TestHistoryRecordsInsert|TestHistoryRecordsUpdate|TestHistorySkipsNoopUpdate|TestHistoryRecordsDelete|TestHistoryReasonFromSessionSetting|TestHistoryAllowsTakedownPurge)\(' internal/database/history_integration_test.go)" = 6`
- [ ] `grep -q '^//go:build integration' internal/database/history_integration_test.go`
- [ ] `grep -q '^func TestSearchReturnsSourceCheckedAt(' internal/database/repository_integration_test.go`
- [ ] `grep -q 'TRUNCATE medical_personnel, medical_personnel_history RESTART IDENTITY' internal/database/repository_integration_test.go`
- [ ] `! grep -q '0004_add_search_log.down.sql' makefile`
- [ ] `test "$(grep -c -E '^func (TestDoctorBubbleShowsCheckedDate|TestDoctorBubbleCheckedDateNeedsURL)\(' internal/linebot/template/doctor_test.go)" = 2`
- [ ] `go test ./internal/linebot/template -run 'TestDoctorBubbleShowsCheckedDate|TestDoctorBubbleCheckedDateNeedsURL' -count=1 -v`
- [ ] `go test ./internal/database -run TestSelectColumnsIncludeNewFields -count=1`
- [ ] `grep -q '^## OD-7 — ' docs/open-decisions.md && grep -q '^## OD-8 — ' docs/open-decisions.md`
- [ ] `test -f docs/sources/official-pages-inventory.md && grep -q '^## 4. 各來源的格式與限制' docs/sources/official-pages-inventory.md`
- [ ] `grep -q 'medical_personnel_history' CLAUDE.md && ! grep -q '兩張表' CLAUDE.md`

## 執行報告

### 摘要
| Part | 狀態 | 說明 |
|------|------|------|
| Part 1：歷史表、trigger 與查到日期欄位（migration 0005） | ✅ 完成 | 0005 up/down 照抄 task 的 SQL；6 個 trigger 整合測試通過；`make test-integration` 改為驗證最新一支 down（0005），exit 0 |
| Part 2：讀取查到日期、卡片顯示官網來源 | ✅ 完成 | `selectColumns`／`MedicalPersonnel` 加 `source_checked_at`；`hospital_directory` 顯示「醫療院所官網」；有連結也有日期時附「（YYYY-MM-DD 查到）」 |
| Part 3：決策與盤點文件 | ✅ 完成 | OD-7、OD-8、`docs/sources/official-pages-inventory.md`、CLAUDE.md 的 DB 描述，都用程式從 task 文件抽出原文寫入 |

### ❌ 未通過的驗收項目
無

### 🔍 需人工確認的驗收項目
無（task 沒有 `【人工確認】` 項目）。部署後要確認 production 的 migrate init container 輸出 `5/u add_history_and_checked_date`，`medical_personnel_history` 的 owner 為 `popo_app`。

### 🤔 執行中的判斷決策
- **Phase 1**：repo 沒有 `docs/architecture/definition-of-done.md` 和 `docs/decisions/OPEN-DECISIONS.md`。決策：改讀對應的 `docs/open-decisions.md`，OD-1～OD-6 都已定案，跟本 task 沒有衝突。
- **Part 1, Step 5**：task 用 `OldHospital: 甲醫院` 簡寫表示 `sql.NullString{String: "甲醫院", Valid: true}`。決策：在 `history_integration_test.go` 加一個測試用的小 helper `valid(s string) sql.NullString`，5 個測試共用；它不是連線或 seed helper，不違反「不要另寫連線或 seed helper」。
- **Part 3, Step 1–3**：決策：用 Python 從 task 文件 Part 3 的三個 fenced block 抽出內容（去掉清單縮排）再寫入，確保逐字照抄；寫入後用抽樣的姓名清單掃過三份文件，沒有出現任何醫師姓名。

### ⚠️ 注意事項
- **新的 schema 模式**：`medical_personnel` 的所有異動由 `record_medical_personnel_history()` trigger 記到 `medical_personnel_history`（整列 JSONB，內容沒變的 UPDATE 不記，`personnel_id` 不設 FK）。批次修改前在交易內 `SET LOCAL popo.change_reason`；交易結束後，同一條連線的 `current_setting` 會回空字串，所以 trigger 用 `NULLIF` 轉成 NULL（`TestHistoryReasonFromSessionSetting` 釘住）。TRUNCATE 不會觸發 row trigger。
- **下架**（OD-8）：同一交易內刪主表 → 刪該 `personnel_id` 的全部歷史 → 寫一筆不含內容的 DELETE（`reason='takedown'`），由 `TestHistoryAllowsTakedownPurge` 釘住。下架機制本身尚未實作。
- **新欄位** `medical_personnel.source_checked_at DATE`：既有資料為 NULL，卡片只在有 `source_url` 時才顯示日期。官網校對沿用 0003 的 `source='hospital_directory'`，CHECK 沒有改。
- **`make test-integration`** 改為驗證最新一支 down migration（`$$(ls migrations/*.down.sql | sort | tail -n 1)`），以後新增 migration 不用再改。`docs/ARCHITECTURE.md` 2026-10-12 那條「再驗證 0004 down」因此過時。
- 沒有新增 go.mod 依賴。
- 編輯期間 gopls 回報的 `p.SourceCheckedAt undefined`、`"time" imported and not used`、`zeroResultActions` 都是快取過期造成的；`go build ./...`、`go vet ./...`、`go test ./...` 全部通過。
