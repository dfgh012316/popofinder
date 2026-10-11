# Task：下架機制（寄信申請＋`takedown_personnel()`）

## 背景

倫理鐵律 4 要求要有主動下架機制，OD-7 也規定下架機制要在第一批爬蟲資料上線前完成。目前 bot 沒有任何下架管道，OD-8 的下架程序也只是三條手打的 SQL。

使用者 2026-10-12 決定：
- 醫師本人或院所**寄信**申請（附本人或院所證明），由維護者人工判斷。
- 不在卡片放下架按鈕，避免一般使用者誤觸。
- 不建 DB 收件匣，申請就在信箱裡。
- 確認後在 psql 執行一個 SQL function 完成下架。

**前置條件**：PR #37（migration 0005 歷史表、OD-7、OD-8）已 merge 進 `develop`。如果 `internal/database/history_integration_test.go` 或 `migrations/0005_add_history_and_checked_date.up.sql` 不存在，依 task-executor Phase 1 規則停止。

### 涉及檔案

- `migrations/0006_add_takedown_function.up.sql` — 新增：SQL function `takedown_personnel(INTEGER)`
- `migrations/0006_add_takedown_function.down.sql` — 新增：移除該 function
- `internal/database/history_integration_test.go` — 改寫 `TestHistoryAllowsTakedownPurge` 改為呼叫 function；新增 `TestTakedownPersonnelRejectsUnknownID`
- `internal/linebot/template/help.go` — 說明卡新增「下架與更正」段
- `internal/linebot/template/help_test.go` — 新增 `TestHelpFlexMessageTakedownContact`
- `internal/linebot/template/doctor_test.go` — 新增 `TestDoctorBubbleHasNoTakedownButton`
- `docs/open-decisions.md` — 追加 OD-9；OD-8 補一行指向 OD-9
- `CLAUDE.md` — 「架構」段的 DB 描述加上 `takedown_personnel`
- `docs/tasks/takedown-mechanism.md` — 本 task 文件

### 本 task 不做（依據）

- **爬蟲的排除清單、整院下架**：爬蟲還不存在。被下架的官網醫師要避免重爬加回，這件事要跟爬蟲在同一個 task 實作（OD-9 決策第 5 點）。
- **DB 收件匣表、bot 內下架按鈕或指令**：使用者 2026-10-12 決定改用寄信（避免誤觸、需要本人或院所證明），見 OD-9。
- **清除 `search_log` 裡含有該醫師姓名的查詢字串**：`search_log` 是內部用量資料，不對外顯示，也不關聯到 `medical_personnel` 的列。鐵律 4 針對的是對外公開的資料，所以不在下架範圍。
- **申請人身分的驗證方式細節**：由維護者人工判斷，不寫成程式。

### Schema Impact

- **新 function**：`takedown_personnel(p_id INTEGER) RETURNS void`。沒有新表、沒有新欄位。
- **下游消費者**：沒有 Go 程式呼叫它，只有維護者在 psql 手動執行，以及 `history_integration_test.go`。
- **Migration**：是，`migrations/0006_add_takedown_function.{up,down}.sql`。`makefile` 的 `test-integration` 已經會自動驗證最新一支 down，所以不用改 makefile。

### State Enumeration（`takedown_personnel(p_id)`）

| State | 條件 | 結果 |
|---|---|---|
| id 存在 | `medical_personnel` 有 `id = p_id` | 主表那一列被刪除；`medical_personnel_history` 中 `personnel_id = p_id` 的列全部刪除，只剩一筆 `op='DELETE'`、`old_row`/`new_row` 為 NULL、`reason='takedown'`；其他醫師的歷史不受影響 |
| id 不存在 | 沒有 `id = p_id` 的列 | `RAISE EXCEPTION`，訊息含 `not found`；整個呼叫回滾，歷史表不新增任何列 |

---

## Part 1：`takedown_personnel()` function 與整合測試

### 現況

- `migrations/` 有 0001–0005。0005 建了 `medical_personnel_history` 和 trigger `record_medical_personnel_history()`：主表 DELETE 時，trigger 會寫一筆帶 `old_row` 的 DELETE 歷史。
- `internal/database/history_integration_test.go` 的 `TestHistoryAllowsTakedownPurge` 用 `tx.MustExec` 手打三條 SQL 執行 OD-8 的下架程序，再斷言沒有含內容的歷史留下。
- `makefile` 的 `test-integration` 最後會執行 `$$(ls migrations/*.down.sql | sort | tail -n 1)`。

### 目標

- `SELECT takedown_personnel(<id>);` 一次完成 OD-8 的三個步驟。
- id 不存在時報錯，不會讓維護者打錯 id 還以為已經下架。
- 整合測試透過這個 function 驗證下架程序（取代手打的三條 SQL）。

### 不可動的範圍

- `migrations/0001`–`0005` 的所有檔案，包括 0005 的 `record_medical_personnel_history()` trigger function。
- `history_integration_test.go` 中 `historyRow`、`historyRows`、`valid`，以及 `TestHistoryAllowsTakedownPurge` 以外的測試函式。
- `makefile`。

### 實作步驟

1. 建立 `migrations/0006_add_takedown_function.up.sql`，內容完全如下：

   ```sql
   -- 下架程序(docs/open-decisions.md OD-8、OD-9):同一交易內刪主表 → 清除該醫師的全部歷史 → 寫一筆不含內容的紀錄。
   -- 用法(psql):SELECT takedown_personnel(<id>);
   -- 找不到該 id 時報錯並整個回滾,避免打錯 id 卻以為已經下架。
   CREATE FUNCTION takedown_personnel(p_id INTEGER) RETURNS void
   LANGUAGE plpgsql AS $$
   BEGIN
       DELETE FROM medical_personnel WHERE id = p_id;
       IF NOT FOUND THEN
           RAISE EXCEPTION 'medical_personnel id % not found', p_id;
       END IF;
       -- 上面的 DELETE 已由 trigger 寫入一筆含 old_row 的歷史,這裡連同它一起清除。
       DELETE FROM medical_personnel_history WHERE personnel_id = p_id;
       INSERT INTO medical_personnel_history (personnel_id, op, reason) VALUES (p_id, 'DELETE', 'takedown');
   END;
   $$;
   ```

2. 建立 `migrations/0006_add_takedown_function.down.sql`，內容完全如下：

   ```sql
   DROP FUNCTION IF EXISTS takedown_personnel(INTEGER);
   ```

3. 在 `internal/database/history_integration_test.go` 的 `TestHistoryAllowsTakedownPurge`：
   - 把註解 `// OD-8 takedown: delete the row, purge its history, keep one content-free DELETE record.` 與其下的四行（`tx := r.db.MustBegin()`、三個 `tx.MustExec(...)`、`require.NoError(t, tx.Commit())`）整段換成：
     ```go
     	// OD-8/OD-9 takedown: takedown_personnel deletes the row, purges its history and keeps one content-free DELETE record.
     	r.db.MustExec("SELECT takedown_personnel(1)")
     ```
   - 在 `require.Zero(t, leaked)` 之後新增：
     ```go
     	var remaining int
     	require.NoError(t, r.db.Get(&remaining, "SELECT count(*) FROM medical_personnel WHERE id = 1"))
     	require.Zero(t, remaining)
     ```
   - 其餘斷言（`leaked` 為 0、`historyRows` 等於兩列）保持不變。

4. 在同一個檔案末尾新增 `TestTakedownPersonnelRejectsUnknownID`：
   - `r := openIntegrationDB(t)`，`seedPersonnel` 一筆 `{"台北", "甲醫院", "內科", "王一"}`。
   - 執行 `_, err := r.db.Exec("SELECT takedown_personnel(999)")`，斷言 `require.Error(t, err)` 與 `require.Contains(t, err.Error(), "not found")`。
   - 斷言 `historyRows(t, r)` 等於 `[]historyRow{{PersonnelID: 1, Op: "INSERT", NewHospital: valid("甲醫院")}}`，也就是沒有寫入任何 takedown 紀錄。
   - 斷言 `SELECT count(*) FROM medical_personnel` 為 1。

5. 執行 `make test-integration`，確認全部整合測試通過，最後的 0006 down 也成功。

---

## Part 2：說明卡的「下架與更正」段

> 與 Part 1 互不依賴。

### 現況

- `internal/linebot/template/help.go` 的 `helpContents`：body 裡的第三個元素是一個 `"margin": "lg"` 的 vertical box，依序包含「支援格式」標題、四種查詢格式（各以 separator 分隔），最後一個元素是「支援城市」box。
- 說明卡由 `HelpFlexMessage()` 產生，加好友與「幫助」共用；Quick Reply 由 `helpQuickReply` 提供三個範例。
- 醫師卡片（`doctor.go` 的 `doctorBubble`）沒有任何下架相關文字或按鈕。

### 目標

- 說明卡最後多一段「下架與更正」，寫明信箱 `jerrychuang@jerrytech.me` 與證明方式。
- 醫師卡片維持沒有下架按鈕，由測試釘住。

### 視覺規格

| 元素 | 屬性 | 參考來源 |
|---|---|---|
| 段落前的分隔線 | `{"type": "separator", "margin": "lg"}` | 同檔各格式之間的 separator |
| 外層 box | `"type": "box"`、`"layout": "vertical"`、`"margin": "lg"` | 同檔「支援城市」box |
| 標題「下架與更正」 | `"type": "text"`、`"weight": "bold"`、`"size": "lg"` | 同檔「支援城市」標題 |
| 說明文字 | `"type": "text"`、`"size": "sm"`、`"color": "#666666"`、`"wrap": true`、`"margin": "sm"` | 同檔說明文字的 size／color；加 `wrap` 是因為句子較長 |

### 不可動的範圍

- `helpContents` 既有的所有元素與順序（新段落只能加在最後）。
- `helpQuickReply` 與 `HelpFlexMessage` 的簽名與內容。
- `doctor.go`（本 Part 只加測試，不改卡片）。

### 實作步驟

1. 在 `internal/linebot/template/help.go` 的 `helpContents`，找到外層 `"margin": "lg"` box 的 `contents` 陣列。在最後一個元素（「支援城市」box，以 `// Supported cities` 註解開頭）之後，依序追加：

   ```go
   					map[string]any{"type": "separator", "margin": "lg"},
   					// Takedown and correction contact (OD-9): by email with proof, no in-bot button.
   					map[string]any{
   						"type":   "box",
   						"layout": "vertical",
   						"margin": "lg",
   						"contents": []any{
   							map[string]any{"type": "text", "text": "下架與更正", "weight": "bold", "size": "lg"},
   							map[string]any{"type": "text", "text": "醫師本人或院所要求下架或更正資料，請寄信至 jerrychuang@jerrytech.me，並以院所官方信箱來信，或附上可證明身分的資料。", "size": "sm", "color": "#666666", "wrap": true, "margin": "sm"},
   						},
   					},
   ```

2. 在 `internal/linebot/template/help_test.go` 新增 `TestHelpFlexMessageTakedownContact`：呼叫 `HelpFlexMessage()`，對字串斷言 `require.Contains` 以下三個字串：`下架與更正`、`jerrychuang@jerrytech.me`、`可證明身分的資料`。
3. 在 `internal/linebot/template/doctor_test.go` 新增 `TestDoctorBubbleHasNoTakedownButton`：
   - 用 `DoctorsFlexMessage` 產生一張卡片，欄位為 `Name: "測試醫師"`、`City: "台北市"`、`Hospital: "測試醫院"`、`Source: blog`、`SourceURL: "https://example.com/doc"`。
   - 斷言 JSON 不含 `下架`，也不含 `"type":"postback"`（醫師卡片不得有 postback 按鈕）。
4. 執行 `go test ./internal/linebot/template/`。

---

## Part 3：OD-9、OD-8 補註、CLAUDE.md

> 與 Part 1、2 互不依賴。

### 現況

- `docs/open-decisions.md` 最後一條是 OD-8。OD-8 決策第 3 點寫著「下架機制本身(收件、流程)尚未實作」。
- `CLAUDE.md`「架構」段的 DB 描述寫「`medical_personnel_history`(由 trigger 記錄前者每一次異動,批次修改前先 `SET LOCAL popo.change_reason`;下架時連同歷史清除,見 open-decisions OD-8)」。

### 目標

OD-9 記錄下架管道與處理方式；OD-8 加一行補註指向 OD-9（不改 OD-8 原文）；CLAUDE.md 寫出 `takedown_personnel`。

### 不可動的範圍

- `docs/open-decisions.md` 中 OD-1–OD-8 的既有文字（OD-8 只能**新增**一行補註）。
- `CLAUDE.md` 除了「架構」段 DB 那一行以外的內容。

### 實作步驟

1. 在 `docs/open-decisions.md` 的 OD-8 中，找到以 `  - TRUNCATE 不會觸發 row trigger` 開頭的那一行，在它的下一行插入（逐字）：

   ```
     - 2026-10-12 補註:收件管道與處理方式(`SELECT takedown_personnel(<id>);`)見 OD-9。
   ```

2. 在 `docs/open-decisions.md` 檔尾追加以下內容，逐字照抄：

   ```markdown

   ## OD-9 — 下架管道:寄信+本人或院所證明,不在 bot 放下架按鈕

   - **日期**:2026-10-12
   - **狀態**:✅ 採納
   - **決策**:
     - 醫師本人或院所要求下架或更正資料,寄信到 `jerrychuang@jerrytech.me`。說明卡(加好友與「幫助」共用)寫明信箱與證明方式。
     - 申請要附本人或院所證明(例如以院所官方信箱來信,或附上可證明是醫師本人的資料),由維護者人工判斷是否照辦。
     - **不在醫師卡片放「要求下架」按鈕,也不做 bot 內的下架指令**;不建 DB 收件匣,申請就在信箱裡,維護者定期看信。
     - 確認後在 psql 執行 `SELECT takedown_personnel(<id>);`:同一交易內刪主表 → 清除該醫師的全部歷史 → 寫一筆不含內容的紀錄(OD-8);id 不存在時報錯並回滾。
     - 爬蟲上線時(OD-7),被下架的官網醫師要加入排除清單,避免重爬又加回來;排除清單與「整院下架」在爬蟲 task 一起實作。
     - `search_log` 裡的查詢字串不在下架範圍:它是內部用量資料,不對外顯示,也不關聯到醫師資料列。
   - **理由**:
     - 卡片上的按鈕會讓一般使用者誤觸,而且收到申請也無從判斷該不該下架;寄信可以附上證明,維護者能判斷是不是本人或院所。
     - 倫理鐵律 4 要求從第一天就要有下架機制;OD-7 要求在第一批爬蟲資料上線前完成。
     - 用 SQL function 而不是手打三條 SQL:一次做完 OD-8 的三個步驟,不會漏掉清除歷史,也有整合測試(`TestHistoryAllowsTakedownPurge`、`TestTakedownPersonnelRejectsUnknownID`)。
   ```

3. 在 `CLAUDE.md` 中，把 `下架時連同歷史清除,見 open-decisions OD-8)` 換成 `下架用 \`SELECT takedown_personnel(<id>);\` 連同歷史清除,見 open-decisions OD-8、OD-9)`（反引號是 Markdown 程式碼標記，照字面寫入），其餘字元不變。

---

## 驗證計畫

| Part | 驗證方式 | 預期斷言 | 為什麼這個斷言能證明目標達成 |
|------|---------|---------|------------------------------|
| Part 1 | `internal/database/history_integration_test.go:TestHistoryAllowsTakedownPurge` | 呼叫 `takedown_personnel(1)` 後，主表沒有 id 1；id 1 沒有任何含內容的歷史；歷史只剩 id 2 的 INSERT 和 id 1 的 takedown 紀錄 | 直接對應 State Enumeration「id 存在」那一格：三個步驟都做到，也沒有誤刪其他醫師的歷史 |
| Part 1 | `internal/database/history_integration_test.go:TestTakedownPersonnelRejectsUnknownID` | id 999 報錯（含 `not found`），歷史表沒有新增 takedown 紀錄，主表仍有 1 筆 | 對應「id 不存在」那一格：打錯 id 不會靜默成功，也不會留下假的下架紀錄 |
| Part 1 | `make test-integration` | exit 0（含最後的 0006 down） | 證明 0006 up 能在全新 DB 套用、整合測試全過，down 也能乾淨移除 |
| Part 2 | `internal/linebot/template/help_test.go:TestHelpFlexMessageTakedownContact` | 說明卡含「下架與更正」、信箱、「可證明身分的資料」 | 對應 OD-9「說明卡寫明信箱與證明方式」；說明卡在加好友時就會出現，申請人看得到 |
| Part 2 | `internal/linebot/template/doctor_test.go:TestDoctorBubbleHasNoTakedownButton` | 醫師卡片不含「下架」，也沒有 postback | 釘住 OD-9「不在卡片放下架按鈕」，之後有人加按鈕時測試會失敗 |
| Part 3 | `grep -q '^## OD-9 — ' docs/open-decisions.md && grep -q '收件管道與處理方式' docs/open-decisions.md` | exit 0 | OD-9 已追加，OD-8 也有補註指向 OD-9 |
| Part 3 | `grep -q 'takedown_personnel' CLAUDE.md` | exit 0 | CLAUDE.md 寫出下架的實際操作方式，下一個 session 讀得到 |

---

## 驗收

- [ ] `test -f migrations/0005_add_history_and_checked_date.up.sql`
- [ ] `go build ./...`
- [ ] `go vet ./...`
- [ ] `go test ./...`
- [ ] `test -z "$(gofmt -l $(find . -name '*.go' -not -path './internal/reconcile/fetch.go'))"`
- [ ] `test -f migrations/0006_add_takedown_function.up.sql && test -f migrations/0006_add_takedown_function.down.sql`
- [ ] `make test-integration`
- [ ] `grep -q '^func TestTakedownPersonnelRejectsUnknownID(' internal/database/history_integration_test.go`
- [ ] `grep -q 'SELECT takedown_personnel(1)' internal/database/history_integration_test.go && ! grep -q 'tx.MustExec("DELETE FROM medical_personnel_history' internal/database/history_integration_test.go`
- [ ] `test "$(grep -c -E '^func (TestHelpFlexMessageTakedownContact|TestDoctorBubbleHasNoTakedownButton)\(' internal/linebot/template/help_test.go internal/linebot/template/doctor_test.go | awk -F: '{s+=$2} END {print s}')" = 2`
- [ ] `go test ./internal/linebot/template -run 'TestHelpFlexMessageTakedownContact|TestDoctorBubbleHasNoTakedownButton' -count=1 -v`
- [ ] `grep -q '^## OD-9 — ' docs/open-decisions.md && grep -q '收件管道與處理方式' docs/open-decisions.md`
- [ ] `grep -q 'takedown_personnel' CLAUDE.md`

## 執行報告

### 摘要
| Part | 狀態 | 說明 |
|------|------|------|
| Part 1：`takedown_personnel()` function 與整合測試 | ✅ 完成 | 0006 up/down 照抄 task 的 SQL；`TestHistoryAllowsTakedownPurge` 改為呼叫 function，並多斷言主表已沒有該列；新增 `TestTakedownPersonnelRejectsUnknownID`；`make test-integration` exit 0（最後驗證 0006 down） |
| Part 2：說明卡的「下架與更正」段 | ✅ 完成 | 「支援城市」之後加 separator＋「下架與更正」box（樣式照視覺規格）；新增說明卡與「醫師卡片沒有下架按鈕」兩個測試 |
| Part 3：OD-9、OD-8 補註、CLAUDE.md | ✅ 完成 | OD-9 與 OD-8 補註用程式從 task 文件抽出原文寫入；CLAUDE.md 的 DB 描述改為 `takedown_personnel` |

### ❌ 未通過的驗收項目
無

### 🔍 需人工確認的驗收項目
無（task 沒有 `【人工確認】` 項目）。部署後要確認 production 的 migrate init container 輸出 `6/u add_takedown_function`。

### 🤔 執行中的判斷決策
- **Phase 1**：repo 沒有 `docs/architecture/definition-of-done.md` 和 `docs/decisions/OPEN-DECISIONS.md`。決策：改讀 `docs/open-decisions.md`，OD-1～OD-8 都已定案，沒有衝突。前置條件（PR #37 已 merge、0005 存在）成立。
- **Part 2, Step 3**：在 `TestDoctorBubbleHasNoTakedownButton` 的斷言前加了一行中文註解說明理由（OD-9），沿用同檔 `TestDoctorBubbleShowsSourceAndURL` 已有的中文註解寫法。

### ⚠️ 注意事項
- **新的 DB function** `takedown_personnel(p_id INTEGER)`（migration 0006）：同一交易內刪主表 → 清除該 `personnel_id` 的全部歷史（包括 DELETE 時 trigger 剛寫入、含 `old_row` 的那一筆）→ 寫一筆不含內容的 `reason='takedown'` 紀錄；id 不存在時 `RAISE EXCEPTION` 並整個回滾。維護者在 psql 執行 `SELECT takedown_personnel(<id>);`，沒有 Go 程式呼叫它。
- **下架管道**（OD-9）：寄信到說明卡上的信箱、附本人或院所證明、人工判斷；不在 bot 放按鈕，也不建 DB 收件匣。爬蟲的排除清單與整院下架留給爬蟲 task。
- 沒有新增 go.mod 依賴，也沒有改 makefile（它已經會驗證最新一支 down）。
