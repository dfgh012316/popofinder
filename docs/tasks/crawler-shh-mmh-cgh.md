# Task：官網爬蟲第二批——雙和、馬偕、國泰

## 背景

OD-7 的第一批是亞東、雙和、長庚、馬偕、國泰。亞東（`femh`）已在 2026-10-12 正式寫入。本 task 加上 **雙和（`shh`）、馬偕（`mmh`）、國泰（`cgh`）** 三個 `Source`；長庚另開 task。

2026-10-12 對四家各做了一次結構調查。每次請求間隔 2 秒，只發 GET，回報不含醫師姓名。原始頁面只存在本機，不進 repo。使用者同日的決定：
- **多院區一人一筆**：同一位醫師列在同一醫院體系的多個院區時，只存一筆。院區照固定順序排，醫師第一次出現的院區就是他的醫院與縣市；科別只收那個院區的。`source_key` 不含院區。
- **馬偕先只做台北清單**：只在淡水看診的 64 位，要先用 session 把清單切到淡水才看得到，之後再補。
- **長庚另開 task**：長庚跑一輪要 3–4 小時，還有穩定回 500 的頁面；使用者已決定跳過職位為「醫師」的列，在長庚 task 處理。

另外依調查結果修正學歷規則：長庚有人寫「OO大學醫學學士」。這四個字是「醫、學、學、士」，不含「醫學士」這個子字串，所以現行規則收不到。

### 涉及檔案

- `internal/crawl/crawl.go` — 新增共用的 `tagRe`、`listItemRe`、`htmlText`、`doctorIndex`（從 `femh.go` 搬來，並一般化）
- `internal/crawl/crawl_test.go` — 新檔，`TestDoctorIndex`
- `internal/crawl/testutil_test.go` — 新檔，共用的 `readFixture`、`serveFixture`（從 `femh_test.go` 搬來，並一般化）
- `internal/crawl/femh.go` — 改用共用 helper，刪掉 `femhTagRe`、`femhItemRe`、`femhText` 與 `Crawl` 內的 `doctorDepts`
- `internal/crawl/femh_test.go` — 改用 `readFixture`、`serveFixture`，刪掉 `readFemhFixture`
- `internal/crawl/education.go`、`internal/crawl/education_test.go` — 加「醫學學士」
- `internal/crawl/shh.go`、`internal/crawl/shh_test.go`、`internal/crawl/shh_live_test.go` — 雙和
- `internal/crawl/testdata/shh/index.html`、`dept_AB.html`、`dept_BD.html`、`dept_01.html`、`profile_10001.html`、`profile_10002.html`、`profile_10003.html` — 合成 fixture
- `internal/crawl/mmh.go`、`internal/crawl/mmh_test.go`、`internal/crawl/mmh_live_test.go` — 馬偕
- `internal/crawl/testdata/mmh/list.html`、`profile_1001.html`、`profile_1002.html`、`profile_1003.html` — 合成 fixture
- `internal/crawl/cgh.go`、`internal/crawl/cgh_test.go`、`internal/crawl/cgh_live_test.go` — 國泰
- `internal/crawl/testdata/cgh/main_category_1553.html`、`main_list_13_1.html`、`main_list_13_2.html`、`main_list_7_1.html`、`main_profile_20001.html`、`main_profile_20002.html`、`main_profile_20003.html`、`sijhih_category_2.html`、`sijhih_category_58.html`、`sijhih_category_61.html`、`sijhih_list_85_1.html`、`sijhih_profile_30001.html`、`list_empty.html` — 合成 fixture
- `cmd/crawl/main.go` — 註冊三個來源、更新 `-sources` 說明
- `docs/open-decisions.md` — OD-10 補註
- `docs/sources/medical-center-inventory.md` — 更正調查結果、新增第 6 節
- `docs/tasks/crawler-shh-mmh-cgh.md` — 本 task 文件

### 本 task 不做（依據）

- **長庚**：使用者 2026-10-12 決定另開 task，理由見背景。
- **馬偕淡水院區**：只在淡水看診的 64 位醫師要靠 session cookie 切換清單，現有 `Fetcher` 沒有 cookie jar。使用者 2026-10-12 選擇「先只做台北」。台北清單已包含 258 位兩院都有門診的醫師。
- **馬偕兒童醫院**：`child/find_doctor_list.php` 的 `did` 是另一套編號，87 位中有 83 位也出現在主站清單。爬了會讓同一人出現兩筆。這是 2026-10-12 告知使用者的預設處理，使用者沒有異議。
- **雙和的病理、核醫、急診、重症、臨床病理 5 科**：這 5 科在 `team.aspx` 只連到 `Introduction.aspx`，團隊介紹是自由文字，沒有 `docCode`。`shhIndexLinkRe` 本來就不會抓到。
- **學歷規則的已知誤判**：「生物醫學系」「獸醫學系」含「醫學系」，「醫學大學附設醫院 住院醫師」會被「醫學大學」那條放行。四家抽樣都沒有出現，依 side project 的 review 範圍，列為接受的風險，不在本 task 處理。
- **jerrytech-deploy 的 CronJob**：merge 後先用手動 Job 跑 `-sources shh,mmh,cgh -dry-run` 看數字，再改 CronJob。這部分跨 repo，不在本 task。
- **把真實頁面存成 fixture**：真實頁面含醫師姓名，而 repo 是 public（OD-7）。fixture 一律依本文件手寫。結構是否正確，由 `-tags live` 測試對真實網站驗證。

### State Enumeration（三個來源的 `Crawl`）

| 情況 | 來源 | 結果 |
|---|---|---|
| 任一頁回非 200（重試後） | 全部 | `Crawl` 回 error、不回部分結果 |
| 同一位醫師出現在同一院區的多個科別 | shh、cgh | 一筆；科別依索引順序串接；個人頁用第一個科別的網址抓 |
| 同一位醫師出現在多個院區 | cgh | 一筆；院區順序（總院→汐止→新竹→內湖）裡第一個院區的醫院、縣市、科別、個人頁網址；之後的院區略過他 |
| 排除的科別（shh：營養、藥劑、心理諮商、健檢類；cgh 分院：護理、藥劑、檢驗、營養、人事、公關） | shh、cgh | 不抓那一頁 |
| 單一科別頁沒有醫師 | shh、cgh | 不算錯誤 |
| 整個來源（shh）或某個院區（cgh）一個醫師 ID 都沒有 | shh、cgh | `Crawl` 回 error（小院區被改版清空時，「不到一半」的刪除保護擋不住） |
| 分院科別內容頁沒有「醫師介紹」連結 | cgh | 略過該科，不算錯誤 |
| 分院索引的連結是 `http` 開頭的站外網址 | cgh | 略過（汐止有連到不存在網域的死連結） |
| 列表有「最後一頁」連結 `&ge;` | cgh | 依第 1 頁的連結抓第 2 到 N 頁；不抓超過 N 的頁（網站會回傳最後一頁，抓到空頁才停會無限循環） |
| 清單的院區標記不是 `TP` | mmh | `parseMmhList` 回 error（醫院與縣市會錯） |
| 清單寫的「共 N 名」≠ 解析到的筆數 | mmh | `parseMmhList` 回 error（版型變了，有人沒被抓到） |
| 清單項目沒有 `did`（沒有個人頁，或其實是門診項目） | mmh | 略過 |
| 同一個 `did` 在清單出現兩次 | mmh | 一筆 |
| 清單的科別 `<sub>` 是空的 | mmh | `Departments` 為 nil |
| 個人頁沒有學歷區塊 | 全部 | `Education` 為 nil（`BuildSyncPlan` 會當成沒有醫學學位而跳過） |
| 個人頁找不到姓名 | 全部 | `Crawl` 回 error |

---

## Part 1：共用 helper 與學歷規則

**依賴**：Part 2、3、4 都要用到本 Part 的 helper，所以本 Part 先做。

### 現況

- `internal/crawl/femh.go` 有幾個其實不限亞東的 helper：`femhTagRe`（`<[^>]*>`）、`femhItemRe`（`(?s)<li>(.*?)</li>`）、`femhText`（去 tag、unescape、TrimSpace）。`Femh.Crawl` 裡另有一段 inline 的 `type doctorDepts struct{ firstCID string; names []string }`，加上 `order`、`byID`，用來彙整「醫師 ID → 第一個科別、所有科別名稱」。
- `internal/crawl/femh_test.go` 有 `readFemhFixture`，`femhFixtureServer` 裡有一個 inline 的 `serve` closure（讀 fixture，讀不到就回 404）。
- `internal/crawl/education.go` 的 `isMedicalDegreeLine` 只認 `醫學系`、`醫學士`，不認 `醫學學士`。

### 目標

- 雙和、國泰（以及之後的長庚）共用同一份文字 helper 與科別彙整邏輯，不再各抄一份。
- 「醫學學士」算醫學學位。
- 亞東的行為完全不變，既有的 femh 測試全部照舊通過。

### 不可動的範圍

- `parseFemhIndex`、`parseFemhDept`、`parseFemhProfile` 的簽名與行為。
- `Fetcher`、`BuildSyncPlan`、`rowFromDoctor`、`formatEducation`。
- `internal/reconcile`（它有自己的 `tagRe`，不要匯出、不要合併）。
- `excludeRe`、`englishRe` 兩個 regex 的內容。

### 實作步驟

1. 在 `internal/crawl/crawl.go` 加入以下內容。import 改為 `context`、`html`、`regexp`、`strings`：
   ```go
   var (
   	tagRe      = regexp.MustCompile(`<[^>]*>`)
   	listItemRe = regexp.MustCompile(`(?s)<li>(.*?)</li>`)
   )

   // htmlText strips tags, unescapes entities and trims the result.
   func htmlText(raw string) string {
   	return strings.TrimSpace(html.UnescapeString(tagRe.ReplaceAllString(raw, "")))
   }

   // doctorIndex collects doctor IDs from department pages: each ID once, in the
   // order first seen, with the code of the first department listing it (the
   // profile is fetched under that department) and every department name it is
   // listed under, in department order.
   type doctorIndex struct {
   	order []string
   	first map[string]string
   	depts map[string][]string
   }

   func newDoctorIndex() *doctorIndex {
   	return &doctorIndex{first: map[string]string{}, depts: map[string][]string{}}
   }

   func (x *doctorIndex) add(id, deptCode, deptName string) {
   	if _, ok := x.first[id]; !ok {
   		x.first[id] = deptCode
   		x.order = append(x.order, id)
   	}
   	x.depts[id] = append(x.depts[id], deptName)
   }
   ```
2. 在 `internal/crawl/femh.go`：
   - 刪掉 `femhItemRe`、`femhTagRe`、`femhText`。
   - 把 `femhText(` 全部改成 `htmlText(`，把 `femhItemRe` 改成 `listItemRe`。
   - 在 `Femh.Crawl` 裡刪掉 `type doctorDepts`、`var order []string`、`byID := ...`，改用 `x := newDoctorIndex()`：
     - 科別迴圈內呼叫 `x.add(id, d.CID, d.Name)`。
     - `len(order) == 0` 改為 `len(x.order) == 0`。
     - 個人頁迴圈改成 `for _, id := range x.order`，原本的 `dd.firstCID` 改為 `x.first[id]`，`dd.names` 改為 `x.depts[id]`。
   - 刪掉 femh.go 因此不再用到的 import（`html` 已移到 crawl.go）。
3. 新增 `internal/crawl/testutil_test.go`（`package crawl`）：
   ```go
   // readFixture returns testdata/<source>/<name>.
   func readFixture(t *testing.T, source, name string) string {
   	t.Helper()
   	b, err := os.ReadFile(filepath.Join("testdata", source, name))
   	require.NoError(t, err)
   	return string(b)
   }

   // serveFixture writes testdata/<source>/<name>, or a 404 when there is no such fixture.
   func serveFixture(w http.ResponseWriter, source, name string) {
   	b, err := os.ReadFile(filepath.Join("testdata", source, name))
   	if err != nil {
   		w.WriteHeader(http.StatusNotFound)
   		return
   	}
   	w.Write(b)
   }
   ```
4. 在 `internal/crawl/femh_test.go`：
   - 刪掉 `readFemhFixture`，呼叫處改成 `readFixture(t, "femh", <name>)`。
   - 刪掉 `femhFixtureServer` 內的 `serve` closure，呼叫處改成 `serveFixture(w, "femh", <name>)`。
   - 刪掉因此不再用到的 import。
5. 新增 `internal/crawl/crawl_test.go`，寫入 `TestDoctorIndex`：
   - 依序呼叫 `add("b", "D1", "甲")`、`add("a", "D1", "甲")`、`add("b", "D2", "乙")`。
   - 斷言 `order == []string{"b", "a"}`、`first["b"] == "D1"`、`depts["b"] == []string{"甲", "乙"}`、`depts["a"] == []string{"甲"}`。
6. 在 `internal/crawl/education.go` 的 `isMedicalDegreeLine` 中：
   - 把 `if strings.Contains(line, "醫學系") || strings.Contains(line, "醫學士") {` 改為 `if strings.Contains(line, "醫學系") || strings.Contains(line, "醫學士") || strings.Contains(line, "醫學學士") {`。
   - 上方註解改為 `// Covers 牙醫學系, 中醫學系, 學士後醫學系, 牙醫學士 and 醫學學士.`。
7. 在 `internal/crawl/education_test.go` 的 `TestIsMedicalDegreeLine` cases 加一行 `{"長庚大學醫學學士", true},`，放在 `{"高雄醫學大學學士後醫學系", true},` 之後。

---

## Part 2：雙和（`shh`）

**依賴**：Part 1。

### 現況

沒有雙和的 parser。2026-10-12 調查到的結構（以下 HTML 都是合成的，姓名與 ID 皆為虛構）：
- **robots.txt**：`User-agent: *` / `Disallow:`，全站允許，沒有 Crawl-delay。`/` 會 302，所以直接從 `/page/team.aspx` 開始。
- **總索引** `https://shh.tmu.edu.tw/page/team.aspx`：門診科別連結形如 `<a href="ShhregTable.aspx?deptCode=AB"><i class="glyphicon glyphicon-chevron-right"></i>心臟內科</a>`，`ShhregTable` 前面有時還帶 `../page/` 或 `/page/`。一般外科分成多條，形如 `deptCode=BD&type=1`…`&type=6`。總共有 66 個不同的 `deptCode`。
- **科別頁** `/page/TeamDoc.aspx?deptCode=<碼>`：沒有分頁。每位醫師在頁面上有兩條 `TeamDocDetail.aspx?deptCode=<碼>&docCode=<id>` 連結（照片＋「醫師簡介」），另外還有一條 `Shhreg.aspx?…&docCode=<id>` 的掛號連結。沒有醫師的科別頁內容是 `<div class="row panel-body"></div>`。
- **個人頁** `/page/TeamDocDetail.aspx?deptCode=<碼>&docCode=<id>`：
  - `<title>` 固定是院名，不能拿來取姓名。姓名在 `<div class="text-center text-primary">` 下的 `<h2>`，格式是「姓名 職稱」。
  - 學歷是 `<p class="title">學歷：</p>` 後面的一串 `<p><span class="point">•　</span>…</p>`，中間用 `<p>&nbsp;</p>` 分隔。
  - 同一個 `docCode` 在不同科別的個人頁內容各存一份，所以要用第一個科別去抓。
- **規模**：約 350–450 位，跑一輪約 410–510 個請求、8–11 分鐘。全站 UTF-8。

### 目標

`crawl.NewShh(baseURL, fetcher)` 回傳一個 `Source`，`Name()` 為 `"shh"`。`Crawl` 依「總索引 → 科別頁 → 個人頁」的順序抓，每位醫師回傳一筆 `Doctor`：
- `Key`：`shh:<docCode>`
- `City`：`新北`；`Hospital`：`雙和醫院`
- `Departments`：依索引順序列出的科別名稱
- `URL`：用第一個科別組出的個人頁網址

### 不可動的範圍

Part 1 的共用 helper 不改簽名；`femh.go` 不動。

### 實作步驟

1. 新增 `internal/crawl/shh.go`（`package crawl`）：
   ```go
   const (
   	shhHospital = "雙和醫院"
   	shhCity     = "新北"
   )

   var (
   	// 營養、藥劑、心理諮商 are not doctors; the 健檢 and 代謝及體重 centers list
   	// doctors again under a second docCode.
   	shhExcludedDept = regexp.MustCompile("營養|藥劑|心理諮商|健檢|健康檢查|代謝及體重")

   	shhIndexLinkRe = regexp.MustCompile(`(?s)<a[^>]*href=["'](?:\.\./page/|/page/)?ShhregTable\.aspx\?deptCode=(\w+)(?:&(?:amp;)?type=\d+)?["'][^>]*>(.*?)</a>`)
   	shhDoctorIDRe  = regexp.MustCompile(`TeamDocDetail\.aspx\?deptCode=\w+(?:&amp;|&)docCode=(\w+)`)
   	shhNameRe      = regexp.MustCompile(`(?s)<div class="text-center text-primary">\s*<h2[^>]*>(.*?)</h2>`)
   	shhEducationRe = regexp.MustCompile(`(?s)<p class="title">\s*學歷\s*[：:]\s*</p>(.*?)(?:<p class="title">|</div>)`)
   	shhBulletRe    = regexp.MustCompile(`(?s)<span class="point">.*?</span>`)
   	shhParagraphRe = regexp.MustCompile(`(?s)<p>(.*?)</p>`)
   )
   ```
2. 在同一檔加入以下型別與方法：
   - `type Shh struct { baseURL string; fetcher *Fetcher }`、`func NewShh(baseURL string, f *Fetcher) *Shh`、`func (s *Shh) Name() string { return "shh" }`。
   - `type shhDept struct { Code, Name string }`。
   - `indexURL()` 回傳 `baseURL + "/page/team.aspx"`。
   - `deptURL(code)` 回傳 `baseURL + "/page/TeamDoc.aspx?deptCode=" + code`。
   - `profileURL(code, docCode)` 回傳 `baseURL + "/page/TeamDocDetail.aspx?deptCode=" + code + "&docCode=" + docCode`。
3. 寫 `parseShhIndex(page string) []shhDept`：
   - 依 `shhIndexLinkRe` 的順序取 `(code, htmlText(name))`。
   - 以 code 去重，同一個 code 保留第一個名稱。
   - 名稱符合 `shhExcludedDept` 的跳過。
4. 寫 `parseShhDept(page string) []string`：依 `shhDoctorIDRe` 的順序取 `docCode`，去重。
5. 寫 `parseShhProfile(page string) (name string, education []string, err error)`：
   - **姓名**：取 `strings.Fields(htmlText(<shhNameRe 第 1 組>))` 的第一個元素。找不到或是空字串時，回 `fmt.Errorf("crawl: shh profile without name")`。
   - **學歷**：`shhEducationRe` 有比對到時：
     1. 先用 `shhBulletRe.ReplaceAllString(block, "")` 刪掉項目符號。
     2. 再對每個 `shhParagraphRe` 比對結果取 `htmlText`，非空字串的才 append。
     
     沒比對到學歷區塊時，`education` 為 nil。
6. 寫 `func (s *Shh) Crawl(ctx context.Context) ([]Doctor, error)`，結構照 `Femh.Crawl`，用 `newDoctorIndex()` 彙整：
   - 抓總索引。失敗時回 `fmt.Errorf("crawl: shh index: %w", err)`；`parseShhIndex` 回空時，回 `fmt.Errorf("crawl: shh index has no departments")`。
   - 依序抓每個科別頁。失敗時回 `fmt.Errorf("crawl: shh department %s: %w", code, err)`；成功時對每個 ID 呼叫 `x.add(id, d.Code, d.Name)`。
   - 全部科別抓完後 `len(x.order) == 0` 時，回 `fmt.Errorf("crawl: shh found no doctors")`。
   - 依 `x.order` 抓 `profileURL(x.first[id], id)`。抓取或解析失敗時回 `fmt.Errorf("crawl: shh doctor %s: %w", id, err)`。
   - 組出 `Doctor{Key: "shh:" + id, Name, City: shhCity, Hospital: shhHospital, Departments: x.depts[id], Education, URL}`。
7. 建立 fixture `internal/crawl/testdata/shh/`。每個檔案都是 `<!DOCTYPE html><html lang="zh-Hant"><head><meta charset="utf-8" /><title>衛生福利部雙和醫院</title></head><body>…</body></html>`，`<body>` 內容如下：
   - `index.html`：一個 `<div class="tab-pane active" id="one"><div class="row department">`，裡面依序 7 個 `<div class="col-xs-12 col-sm-6 col-md-3">`，各包一條連結。每條連結內容都是 `<i class="glyphicon glyphicon-chevron-right"></i>`，加上換行、名稱、換行：
     1. `href="ShhregTable.aspx?deptCode=AB"` 心臟內科
     2. `href="ShhregTable.aspx?deptCode=BD&type=1"` 一般外科
     3. `href="ShhregTable.aspx?deptCode=BD&type=2"` 乳房外科
     4. `href="ShhregTable.aspx?deptCode=15"` 營養室
     5. `href="Introduction.aspx?IntroType=A&deptCode=3800"` 急診醫學科
     6. `href="javascript:alert('請洽服務台');"` 質子治療
     7. `href="../page/ShhregTable.aspx?deptCode=01" onclick="return comfirmbox('01');"` 家庭醫學科
   - `dept_AB.html`：`<div class="row panel-body">` 內有兩個醫師區塊（10001 王小明、10002 李小華）。每個區塊的格式如下，ID、姓名照換：
     ```html
     <div class="media"><div class="media-left"><a href="/page/TeamDocDetail.aspx?deptCode=AB&docCode=10001"><img class="media-object thumbnail" src="../upload_files/doc/10001.jpg"style="width: 150px;" /></a></div><div class="media-body"><h3 class="media-heading">王小明 專任主治醫師</h3>主治專長：<br /><ul><li>高血壓</li></ul><br /><div><p style="text-align: right;"><a class="btn btn-theme" href="Shhreg.aspx?deptCode=AB&docCode=10001">我要掛號</a> <a class="btn btn-default" href="TeamDocDetail.aspx?deptCode=AB&docCode=10001">醫師簡介</a></p></div></div></div><div class="dots"></div>
     ```
   - `dept_BD.html`：同上格式，`deptCode=BD`，醫師依序是 10002 李小華、10003 張小美。
   - `dept_01.html`：只有 `<div class="row panel-body"></div>`。
   - `profile_10001.html`：
     ```html
     <aside class="left-sidebar">
     <h3>心臟內科          </h3>
     </aside>
     <div class="col-md-9">
     <div class="text-center text-primary">
     <h2 class="title">王小明 專任主治醫師</h2>
     </div>
     <p class="title">主治專長：</p>
     <p><span class="point">•　</span>高血壓</p>
     <p>&nbsp;</p>
     <p class="title">學歷：</p>
     <p><span class="point">•　</span>國立陽明大學醫學系</p><p><span class="point">•　</span>國立台灣大學臨床醫學研究所碩士</p>
     <p>&nbsp;</p>
     <p class="title">經歷：</p>
     <p><span class="point">•　</span>雙和醫院心臟內科總醫師</p>
     </div>
     ```
   - `profile_10002.html`：同 `profile_10001.html` 的格式，h2 為 `李小華 主治醫師`，學歷只有一行 `臺北醫學大學醫學系`。
   - `profile_10003.html`：同上格式，h2 為 `張小美 兼任主治醫師`，**沒有**「學歷：」那一段（只有主治專長與經歷）。
8. 新增 `internal/crawl/shh_test.go`。fixture server `shhFixtureServer(t, failCode string)` 的路由規則如下，其他路徑一律 404：

   | 路徑 | 回應 |
   |---|---|
   | `/page/team.aspx` | `index.html` |
   | `/page/TeamDoc.aspx` | `dept_<deptCode>.html`；`deptCode == failCode` 時回 500 |
   | `/page/TeamDocDetail.aspx` | `profile_<docCode>.html` |

   都用 `serveFixture(w, "shh", …)` 回應。寫以下 5 個測試：
   - `TestParseShhIndex`：斷言 `[]shhDept{{"AB", "心臟內科"}, {"BD", "一般外科"}, {"01", "家庭醫學科"}}`。
   - `TestParseShhDept`：
     - `dept_AB.html` 回 `[]string{"10001", "10002"}`。
     - `dept_01.html` 回 nil。
   - `TestParseShhProfile`：
     - `profile_10001.html` 回 `王小明`、`[]string{"國立陽明大學醫學系", "國立台灣大學臨床醫學研究所碩士"}`。
     - `profile_10003.html` 回 `張小美`、nil。
     - `"<html></html>"` 回 error。
   - `TestShhCrawlAggregatesDepartments`：
     - 用 `NewShh(srv.URL, NewFetcher(0))` 跑 `Crawl`。
     - 斷言回 3 筆，Key 依序為 `shh:10001`、`shh:10002`、`shh:10003`。
     - 第 2 筆的 `Departments == []string{"心臟內科", "一般外科"}`、`Name == "李小華"`、`City == "新北"`、`Hospital == "雙和醫院"`，`URL` 符合 `/page/TeamDocDetail\.aspx\?deptCode=AB&docCode=10002$`。
     - 第 3 筆的 `Education` 為 nil。
     - 營養室（15）沒有 fixture，被抓的話會 404 而失敗，所以 Crawl 成功就證明它沒被抓。
   - `TestShhCrawlFailsOnMissingDeptPage`：
     - `failCode = "BD"`，fetcher 的 `sleep` 設成 no-op。
     - 斷言 `Crawl` 回 error 且結果為 nil。
9. 新增 `internal/crawl/shh_live_test.go`，第一行 `//go:build live`。`TestShhLive` 用 `NewShh("https://shh.tmu.edu.tw", NewFetcher(time.Second))`，共 4 個請求，log 只記數量：
   - `parseShhIndex` 至少 50 個科別。
   - 第一個科別頁至少 1 個 ID。
   - 該 ID 的個人頁有姓名，學歷至少 1 行。

---

## Part 3：馬偕（`mmh`，台北清單）

**依賴**：Part 1。

### 現況

沒有馬偕的 parser。2026-10-12 調查到的結構（以下 HTML 都是合成的）：
- **robots.txt**：只有 `User-agent: Googlebot` / `allow: /`，沒有 `*` 群組，對本爬蟲等於全部允許。前面有 F5 WAF；2 秒間隔沒有被擋，1 秒間隔沒有測過。
- **清單** `https://www.mmh.org.tw/find_doctor_list.php`：
  - 不帶 cookie 時是台北院區清單：一整頁、沒有分頁，約 460 KB。頁面寫 `<h6>共 607 名</h6>`，正好等於 `<li class='list'>` 的數量。
  - 頁首急診連結的 `DMZWEBEMRA651/Default.aspx?a=TP` 標示這是台北清單（淡水是 `a=TS`）。
  - 每一筆形如 `<li class='list'>  <span class='name'><div><a href='doctor_view.php?depid=2&did=1234' target='_new'>姓名</a></div> <div>職稱<sub>科別</sub></div></span>…`。
  - 沒有個人頁的項目，`depid` 與 `did` 都是空的。同一個 `did` 偶爾會出現兩次。
  - 清單上的姓名可能含私用區字元，所以姓名以個人頁為準。
- **個人頁** `/doctor_view.php?depid=<depid>&did=<did>`：
  - 內容與 `depid` 無關。頁首 `<h1>` 會跟著網址的 `depid` 變，不能當科別。
  - 姓名在 `<h2 class="event-hed-title">`；`<title>` 固定是院名。
  - 「主要學歷」在 `<h2 class="event-ch-title">主要學歷</h2><ul class="info-list">…</ul>`，之後緊接下一個 `<h2 class="event-ch-title">`。區塊內是自由文字，`<br>`、`<p>`、`<ul><li>` 都有，所以區塊內可能再包一層 `</ul>`。
- **規模**：台北清單 504 位有個人頁，跑一輪約 506 個請求、10 分鐘。全站 UTF-8。

### 目標

`crawl.NewMmh(baseURL, fetcher)` 回傳一個 `Source`，`Name()` 為 `"mmh"`。`Crawl` 先抓清單，再抓每位有 `did` 的醫師的個人頁，回傳的 `Doctor` 如下：
- `Key`：`mmh:<did>`
- `City`：`台北`；`Hospital`：`馬偕紀念醫院`
- `Departments`：清單的 `<sub>`，空的就是 nil
- `Name`：取自個人頁
- `URL`：個人頁網址

### 不可動的範圍

Part 1 的共用 helper 不改簽名。不要為馬偕改 `Fetcher`，也不要加 cookie jar（淡水不在本 task）。

### 實作步驟

1. 新增 `internal/crawl/mmh.go`：
   ```go
   const (
   	mmhHospital = "馬偕紀念醫院"
   	mmhCity     = "台北"
   )

   var (
   	mmhListItemRe  = regexp.MustCompile(`(?s)<li class='list'>\s*<span class='name'><div><a href='doctor_view\.php\?depid=(\w*)(?:&amp;|&)did=(\d*)' target='_new'>(.*?)</a></div>\s*<div>(.*?)<sub>(.*?)</sub></div></span>`)
   	mmhCountRe     = regexp.MustCompile(`<h6>共 (\d+) 名</h6>`)
   	mmhAreaRe      = regexp.MustCompile(`DMZWEBEMRA651/Default\.aspx\?a=(TP|TS)`)
   	mmhNameRe      = regexp.MustCompile(`(?s)<h2 class="event-hed-title">(.*?)</h2>`)
   	mmhEducationRe = regexp.MustCompile(`(?s)<h2 class="event-ch-title">主要學歷</h2>\s*<ul class="info-list">(.*?)</ul>\s*<h2 class="event-ch-title">`)
   	mmhBreakRe     = regexp.MustCompile(`(?i)<br\s*/?>|</p>|</li>|</div>`)
   )
   ```
2. 在同一檔加入以下型別與方法：
   - `type Mmh struct { baseURL string; fetcher *Fetcher }`、`func NewMmh(baseURL string, f *Fetcher) *Mmh`、`func (s *Mmh) Name() string { return "mmh" }`。
   - `type mmhEntry struct { DepID, DID, Dept string }`。
   - `listURL()` 回傳 `baseURL + "/find_doctor_list.php"`。
   - `profileURL(depID, did)` 回傳 `baseURL + "/doctor_view.php?depid=" + depID + "&did=" + did`。
3. 寫 `parseMmhList(page string) ([]mmhEntry, error)`，依序檢查：
   1. `mmhAreaRe` 沒比對到，或第 1 組不是 `TP` 時，回 `fmt.Errorf("crawl: mmh list is not the 台北 list")`。
   2. 取 `matches := mmhListItemRe.FindAllStringSubmatch(page, -1)`。`mmhCountRe` 沒比對到，或其數字不等於 `len(matches)` 時，回 `fmt.Errorf("crawl: mmh list says %s doctors, parsed %d", <數字或空字串>, len(matches))`。
   3. 依序處理每筆：`did`（第 2 組）是空的就跳過；`did` 已出現過也跳過。其餘 append `mmhEntry{DepID: 第 1 組, DID: 第 2 組, Dept: htmlText(第 5 組)}`。
4. 寫 `parseMmhProfile(page string) (name string, education []string, err error)`：
   - **姓名**：`strings.Join(strings.Fields(htmlText(<mmhNameRe 第 1 組>)), "")`。空字串時回 `fmt.Errorf("crawl: mmh profile without name")`。
   - **學歷**：`mmhEducationRe` 有比對到時：
     1. 先把區塊內的 `mmhBreakRe` 都換成 `"\n"`，再取 `htmlText`。
     2. 以 `"\n"` 切開，每行 `strings.TrimSpace` 之後，非空的才 append。
     
     沒比對到時，`education` 為 nil。
5. 寫 `func (s *Mmh) Crawl(ctx context.Context) ([]Doctor, error)`：
   - 抓清單。抓取失敗時回 `fmt.Errorf("crawl: mmh list: %w", err)`；`parseMmhList` 失敗時直接回它的 error。
   - 沒有任何 entry 時，回 `fmt.Errorf("crawl: mmh list has no doctors")`。
   - 依序抓每個 entry 的 `profileURL(e.DepID, e.DID)`。抓取或解析失敗時回 `fmt.Errorf("crawl: mmh doctor %s: %w", e.DID, err)`。
   - `Departments` 在 `e.Dept != ""` 時為 `[]string{e.Dept}`，否則為 nil。
   - 組出 `Doctor{Key: "mmh:" + e.DID, Name, City: mmhCity, Hospital: mmhHospital, Departments, Education, URL}`。
6. 建立 fixture `internal/crawl/testdata/mmh/`。每個檔案都是 `<!DOCTYPE html><html lang="zh-Hant"><head><meta charset="utf-8"><title>…</title></head><body>…</body></html>`：
   - **`list.html`**：title 為 `網路掛號-依醫師：馬偕紀念醫院`，body 依序是：
     - `<div class="top"><a href="https://example.invalid/DMZWEBEMRA651/Default.aspx?a=TP">急診</a></div>`
     - `<div class="searchout"><h6>共 5 名</h6></div>`
     - 一個 `<ul>`，裡面 5 個項目**寫在同一行**、以 `</li><li class='list'>` 相接。每個項目的格式是 `<li class='list'>  <span class='name'><div><a href='doctor_view.php?depid=<depid>&did=<did>' target='_new'><姓名></a></div> <div><職稱><sub><科別></sub></div></span>  <span class='major'>高血壓</span>  <span class='link'>門診 : <br><a href='register_single_doctor.php?depid=14&did=<did>&area=tp' class='staff-link staff-linkask' target='_blank'>台北門診掛號</a></span></li>`。5 個項目依序為：

       | # | depid | did | 姓名 | 職稱 | 科別 |
       |---|---|---|---|---|---|
       | 1 | `2` | `1001` | `王&#xE000;明` | 主治醫師 | 心臟血管內科系 |
       | 2 | （空） | （空） | 麻醉門診 | （空） | （空） |
       | 3 | `5` | `1002` | 李小華 | 主治醫師 | 胃腸肝膽科 |
       | 4 | `247` | `1003` | 張小美 | （空） | （空） |
       | 5 | `2` | `1001` | 王小明 | 主治醫師 | 心臟血管內科系 |

   - **個人頁共同格式**：title 為 `馬偕紀念醫院`，body 依序是：
     ```html
     <div class="sub-nav-name"><h1>某科</h1></div>
     <h2 class="event-hed-title">姓名</h2>
     <h2 class="event-ch-title">主要學歷</h2>
     <ul class="info-list">（學歷區塊）</ul>
     <h2 class="event-ch-title">主要經歷</h2>
     <ul class="info-list"><p>馬偕紀念醫院總醫師</p></ul>
     ```
   - **`profile_1001.html`**：姓名 `王小明`，學歷區塊為：
     ```html
     <p>國立陽明大學醫學系<br />
     國立陽明大學臨床醫學研究所碩士</p>
     ```
   - **`profile_1002.html`**：姓名 `李小華`。學歷區塊為下列內容，裡面包了一層 `<ul>…</ul>`，而且在內層 `</ul>` **之後**還有一行：
     ```html
     <ul>
     	<li><p>臺北醫學大學醫學系 醫學士</p></li>
     </ul>

     <p>&nbsp;</p>
     <p>國立台灣大學公共衛生研究所碩士</p><br />
     ```
   - **`profile_1003.html`**：姓名 `張小美`，學歷區塊只有換行與空白。
7. 新增 `internal/crawl/mmh_test.go`。fixture server `mmhFixtureServer(t, failDID string)` 的路由規則如下，其他路徑一律 404：

   | 路徑 | 回應 |
   |---|---|
   | `/find_doctor_list.php` | `list.html` |
   | `/doctor_view.php` | `profile_<did>.html`；`did == failDID` 時回 500 |

   寫以下 4 個測試：
   - `TestParseMmhList`：
     - `list.html` 回 `[]mmhEntry{{"2", "1001", "心臟血管內科系"}, {"5", "1002", "胃腸肝膽科"}, {"247", "1003", ""}}`、無 error。
     - 把 `a=TP` 換成 `a=TS` 後回 error。
     - 把 `共 5 名` 換成 `共 6 名` 後回 error。
   - `TestParseMmhProfile`：
     - `profile_1001.html` 回 `王小明`、`[]string{"國立陽明大學醫學系", "國立陽明大學臨床醫學研究所碩士"}`。
     - `profile_1002.html` 回 `李小華`、`[]string{"臺北醫學大學醫學系 醫學士", "國立台灣大學公共衛生研究所碩士"}`。第二行在內層 `</ul>` 之後，區塊如果在內層 `</ul>` 就結束，這行會不見。
     - `profile_1003.html` 回 `張小美`、nil。
     - `"<html></html>"` 回 error。
   - `TestMmhCrawl`：
     - 用 `NewMmh(srv.URL, NewFetcher(0))` 跑 `Crawl`，斷言回 3 筆，Key 依序為 `mmh:1001`、`mmh:1002`、`mmh:1003`。
     - 第 1 筆 `Name == "王小明"`（取自個人頁，不是清單上的 `王&#xE000;明`）、`City == "台北"`、`Hospital == "馬偕紀念醫院"`、`Departments == []string{"心臟血管內科系"}`，`URL` 符合 `/doctor_view\.php\?depid=2&did=1001$`。
     - 第 3 筆 `Departments` 為 nil。
   - `TestMmhCrawlFailsOnMissingProfile`：
     - `failDID = "1002"`，fetcher 的 `sleep` 設成 no-op。
     - 斷言 `Crawl` 回 error 且結果為 nil。
8. 新增 `internal/crawl/mmh_live_test.go`，第一行 `//go:build live`。`TestMmhLive` 用 `NewMmh("https://www.mmh.org.tw", NewFetcher(time.Second))`，log 只記數量：
   - `parseMmhList` 無 error，且至少 400 筆。
   - 依序抓前 3 筆的個人頁，每頁都要有姓名；遇到第一個學歷至少 1 行的就停止。
   - 斷言有找到這樣的頁面。最多 5 個請求。

---

## Part 4：國泰（`cgh`，四個院區）

**依賴**：Part 1。

### 現況

沒有國泰的 parser。2026-10-12 調查到的結構如下（HTML 都是合成的）。

**網域與規範**
- 四個子網域在同一台伺服器上，robots.txt 都是 `User-agent: *` / `Allow: /`，沒有 Crawl-delay。
- `cgh.org.tw`（不帶 www）連不上，一律用 `www.cgh.org.tw`。全站 UTF-8，沒有 3xx。
- 頁面有 `<base href>`，相對連結都相對於 `/ec99/<partner>/`。

**院區**

| 院區 | 網址 | 醫院 | 縣市 |
|---|---|---|---|
| 總院 | `https://www.cgh.org.tw/ec99/rwd1320/` | 國泰綜合醫院 | 台北 |
| 汐止 | `https://sijhih.cgh.org.tw/ec99/rwd102/` | 汐止國泰綜合醫院 | 新北 |
| 新竹 | `https://hsinchu.cgh.org.tw/ec99/rwd103/` | 新竹國泰綜合醫院 | 新竹 |
| 內湖 | `https://neihu.cgh.org.tw/ec99/rwd1309A/` | 內湖國泰診所 | 台北 |

**總院索引** `category.asp?category_id=1553`
- 有 40 個醫師列表連結，形如 `<a class="col-md-12 col-xs-12 btn_division" href="category_doct.asp?mode=listView&amp;category_id=13" target="_blank" title="(另開視窗)">消化內科</a>`。
- 有一條連結被 HTML 註解掉了，而且用的是 `&` 而不是 `&amp;`。
- 有幾個名稱共用同一個 id。

**分院**
- 分院多一層：專科介紹頁（汐止、新竹是 `category.asp?category_id=2`，內湖是 `category.asp?category_id=45`）→ 科別內容頁 `category.asp?category_id=N` → 頁內「醫師介紹」連結 `category_doct.asp?mode=listView&amp;category_id=M`（N ≠ M）。
- 專科介紹頁的連結有兩種寫法：
  - `<a class="col-md-12 col-xs-12 btn_division" href="category.asp?category_id=58" …>`
  - `<div class="btn_division"><a href="/ec99/rwd1309A/category.asp?category_id=96" …>`
- 汐止有兩條連到不存在網域的死連結（`https://h2.cgh.org.tw/…`）。
- 分院有護理、藥劑、檢驗、營養、人事、公關等非醫師單位。

**醫師列表** `category_doct.asp?mode=listView&category_id=<id>&pageno=<n>`
- 一頁 12 人。每位醫師有 4 條 `product_doct.asp?prodid=<id>&category_id=<id>` 連結（其中一條多帶 `&doct=N`）。
- 只有 1 頁的列表沒有分頁區塊。多頁時，「最後一頁」連結是 `<a href='…pageno=N…'>&ge;</a>`。
- 頁碼超過最後一頁時，網站會回傳最後一頁。
- `og:url` 的 meta 裡也有 `pageno=1`。

**個人頁** `product_doct.asp?prodid=<id>&category_id=<任一列表>`
- 內容與 `category_id` 無關。
- 姓名在 `<span id="pd_name">主治醫師：王小明(消化內科主任)</span>`，格式是「職稱＋全形冒號＋姓名＋可有可無的括號附註」；`<title>` 固定是「醫師介紹」。
- 學歷在 `<h2 class="cgh2_title">學歷</h2>` 後的 `<ul><li>…</li></ul>`。
- **`prodid` 四個院區共用**（同一位醫師在不同院區是同一個 ID，學歷內容相同），有前導 0，要當字串處理。
- 每抓一次個人頁，頁面的人氣計數會加 1，是無害的副作用。

**規模**：合計約 1,000 個請求、17–20 分鐘。

### 目標

`crawl.NewCgh(mainHost, sijhihHost, hsinchuHost, neihuHost, fetcher)` 回傳一個 `Source`，`Name()` 為 `"cgh"`。`Crawl` 依院區順序（總院→汐止→新竹→內湖）**一個院區抓完再換下一個**，讓連續的請求都打同一個 host、維持每秒 1 個。

每位醫師一筆 `Doctor`，以第一個列出他的院區為準：
- `Key`：`cgh:<prodid>`
- `City`、`Hospital`：該院區的值
- `Departments`：只收該院區的科別
- `URL`：用該院區的網址組出

後面的院區再列出同一個 `prodid` 時跳過。

### 不可動的範圍

Part 1 的共用 helper 不改簽名。不要並行抓多個院區。

### 實作步驟

1. 新增 `internal/crawl/cgh.go`：
   ```go
   var (
   	// Non-doctor units on the branch indexes.
   	cghExcludedDept = regexp.MustCompile("護理|藥劑|檢驗|營養|人事|公關")

   	cghCommentRe    = regexp.MustCompile(`(?s)<!--.*?-->`)
   	cghListLinkRe   = regexp.MustCompile(`(?s)<a[^>]*href="category_doct\.asp\?mode=listView&(?:amp;)?category_id=(\d+)"[^>]*>(.*?)</a>`)
   	cghBranchDeptRe = regexp.MustCompile(`(?s)(?:<a class="[^"]*btn_division[^"]*"|<div class="btn_division">\s*<a) href="([^"]*?category\.asp\?category_id=(\d+))"[^>]*>(.*?)</a>`)
   	cghDoctorIDRe   = regexp.MustCompile(`product_doct\.asp\?prodid=(\d+)&(?:amp;)?category_id=\d+`)
   	cghLastPageRe   = regexp.MustCompile(`<a href='[^']*[?&]pageno=(\d+)[^']*'>&ge;</a>`)
   	cghNameRe       = regexp.MustCompile(`(?s)<span id="pd_name">(.*?)</span>`)
   	cghEducationRe  = regexp.MustCompile(`(?s)<h2 class="cgh2_title">\s*學歷\s*</h2>\s*<ul>(.*?)</ul>`)
   )

   type cghCampus struct {
   	host     string // scheme://host
   	base     string // path prefix, e.g. "/ec99/rwd1320/"
   	index    string // index page, relative to base
   	branch   bool   // the index links to department pages, which link to the doctor lists
   	hospital string
   	city     string
   }

   func (c cghCampus) url(rel string) string { return c.host + c.base + rel }

   func (c cghCampus) listURL(listID string, page int) string {
   	return c.url(fmt.Sprintf("category_doct.asp?mode=listView&category_id=%s&pageno=%d", listID, page))
   }

   func (c cghCampus) profileURL(prodID, listID string) string {
   	return c.url("product_doct.asp?prodid=" + prodID + "&category_id=" + listID)
   }

   // Cgh crawls 國泰 (www/sijhih/hsinchu/neihu.cgh.org.tw, one CMS). A doctor listed
   // at several campuses shares one prodid and is kept once, under the first campus.
   type Cgh struct {
   	campuses []cghCampus
   	fetcher  *Fetcher
   }

   func NewCgh(mainHost, sijhihHost, hsinchuHost, neihuHost string, f *Fetcher) *Cgh {
   	return &Cgh{fetcher: f, campuses: []cghCampus{
   		{mainHost, "/ec99/rwd1320/", "category.asp?category_id=1553", false, "國泰綜合醫院", "台北"},
   		{sijhihHost, "/ec99/rwd102/", "category.asp?category_id=2", true, "汐止國泰綜合醫院", "新北"},
   		{hsinchuHost, "/ec99/rwd103/", "category.asp?category_id=2", true, "新竹國泰綜合醫院", "新竹"},
   		{neihuHost, "/ec99/rwd1309A/", "category.asp?category_id=45", true, "內湖國泰診所", "台北"},
   	}}
   }

   func (s *Cgh) Name() string { return "cgh" }

   type cghDept struct {
   	ID, Name string
   }
   ```
2. 寫以下 5 個解析函式：
   - `parseCghIndex(page string) []cghDept`：總院用。
     1. 先 `cghCommentRe.ReplaceAllString(page, "")`。
     2. 依 `cghListLinkRe` 的順序取 `(id, htmlText(name))`，以 id 去重，保留第一個名稱。
   - `parseCghBranchIndex(page string) []cghDept`：
     1. 先刪掉註解。
     2. 依 `cghBranchDeptRe` 的順序處理：第 1 組 href 以 `http` 開頭的跳過；名稱 `htmlText(第 3 組)` 符合 `cghExcludedDept` 的跳過；以第 2 組 id 去重。
   - `parseCghListIDs(page string) []string`：分院科別內容頁用。先刪掉註解，再依 `cghListLinkRe` 取第 1 組，去重。
   - `parseCghListPage(page string) (ids []string, lastPage int)`：
     - `ids`：依 `cghDoctorIDRe` 取第 1 組，去重。
     - `lastPage`：`cghLastPageRe` 第 1 組的數字；沒有比對到或不是數字時為 1。
   - `parseCghProfile(page string) (name string, education []string, err error)`：
     - **姓名**：
       1. `n := htmlText(<cghNameRe 第 1 組>)`。
       2. 有 `：` 時，取它之後的部分（`strings.Cut`）。
       3. 依序在 `(` 和 `（` 處切斷，取前半。
       4. 最後 `strings.Join(strings.Fields(n), "")`。
       
       空字串時回 `fmt.Errorf("crawl: cgh profile without name")`。
     - **學歷**：`cghEducationRe` 有比對到時，對每個 `listItemRe` 比對結果取 `htmlText`，非空的才 append。
3. 寫 `func (s *Cgh) Crawl(ctx context.Context) ([]Doctor, error)`：
   - 用 `taken := map[string]bool{}` 記錄已經屬於前面院區的 `prodid`。
   - 依序對每個院區呼叫 `s.crawlCampus(ctx, c, taken)`，把回傳的 `[]Doctor` 接起來。任一院區回 error 時，`Crawl` 回 nil 與該 error。
4. 寫 `func (s *Cgh) crawlCampus(ctx context.Context, c cghCampus, taken map[string]bool) ([]Doctor, error)`，依序做下面四件事。

   **(1) 抓院區索引**：抓 `c.url(c.index)`，失敗時回 `fmt.Errorf("crawl: cgh %s index: %w", c.host, err)`。

   **(2) 整理這個院區的醫師列表 `lists []cghDept`**（以列表 id 去重，保留第一個科別名稱）：
   - 總院（`!c.branch`）：`lists = parseCghIndex(index)`。
   - 分院：
     - 先 `depts := parseCghBranchIndex(index)`。
     - 對每個科別抓 `c.url("category.asp?category_id=" + d.ID)`，失敗時回 `fmt.Errorf("crawl: cgh %s department %s: %w", c.host, d.ID, err)`。
     - 對該頁 `parseCghListIDs` 的每個列表 id，append `cghDept{ID: <列表 id>, Name: d.Name}`。
   - 總院的 `lists` 為空，或分院的 `depts` 為空時，回 `fmt.Errorf("crawl: cgh %s index has no departments", c.host)`。

   **(3) 抓醫師列表**：建立 `x := newDoctorIndex()` 與 `found := 0`。對每個列表 `l`：
   - 從 `p := 1` 開始抓 `c.listURL(l.ID, p)`，`last` 取第 1 頁 `parseCghListPage` 回傳的 `lastPage`，一直抓到 `p == last`。
   - 抓取失敗時回 `fmt.Errorf("crawl: cgh %s list %s page %d: %w", c.host, l.ID, p, err)`。
   - 每個 ID：`found++`；`taken[id]` 為 false 時呼叫 `x.add(id, l.ID, l.Name)`。
   - 全部列表抓完後 `found == 0` 時，回 `fmt.Errorf("crawl: cgh %s found no doctors", c.host)`。

   **(4) 抓個人頁**：依 `x.order` 抓 `c.profileURL(id, x.first[id])`。
   - 抓取或解析失敗時回 `fmt.Errorf("crawl: cgh doctor %s: %w", id, err)`。
   - 成功時 append `Doctor{Key: "cgh:" + id, Name, City: c.city, Hospital: c.hospital, Departments: x.depts[id], Education, URL}`，並設 `taken[id] = true`。
5. 建立 fixture `internal/crawl/testdata/cgh/`。每個檔案都是 `<!DOCTYPE html><html lang="zh-Hant"><head><meta charset="utf-8"><title>…</title></head><body>…</body></html>`。

   **5-1 總院索引與列表**
   - **`main_category_1553.html`**：title 為 `專科介紹`。body 依序 4 條，**第 2 條整個包在 `<!-- -->` 裡、用 `&` 而不是 `&amp;`**。每條是 `<div class="col-md-3 col-sm-6 col-xs-12"><a class="col-md-12 col-xs-12 btn_division" href="category_doct.asp?mode=listView&amp;category_id=<id>" target="_blank" title="(另開視窗)"><名稱></a></div>`：
     1. 13 消化內科
     2. `<!--` 26 心臟血管外科 `-->`
     3. 7 心血管中心
     4. 13 胃腸肝膽科
   - **列表頁醫師卡片格式**：每位醫師是下面 4 條連結，`<id>`、`<list>`、職稱、姓名照換：
     ```html
     <a href="product_doct.asp?prodid=<id>&category_id=<list>&doct=27" onclick="" class=""><img src="/rwd101/Store/bImages/b_<id>.jpg"></a>
     <a href="product_doct.asp?prodid=<id>&category_id=<list>" class="info">細節<!--細節--></a>
     <a href="product_doct.asp?prodid=<id>&category_id=<list>"  onclick="" class=""><h2>職稱：姓名</h2></a>
     <a href="product_doct.asp?prodid=<id>&category_id=<list>" type="button" class="btn btn-default ">詳細介紹<!--詳細介紹--></a>
     ```
     每個列表頁的 head 都要加 `<meta property="og:url" content="https://www.cgh.org.tw/ec99/rwd1320/category_doct.asp?mode=listView&category_id=<list>&pageno=1">`。
   - **`main_list_13_1.html`**：
     - 醫師：20001（`主治醫師：王小明(消化內科主任)`）、20002（`兼任主治醫師：李小華`），`<list>` 都是 13。
     - 分頁區塊（2 頁中的第 1 頁）：
       ```html
       <ul class="pagination ">
       <li><a href='/ec99/rwd1320/category_doct.asp?pageno=1&mode=listView&category_id=13'>&le;</a></li><li class='disabled'><a>&lt;</a></li>
       <li class="active"><a href="/ec99/rwd1320/category_doct.asp?pageno=1&mode=listView&category_id=13">1</a></li>
       <li class=""><a href="/ec99/rwd1320/category_doct.asp?pageno=2&mode=listView&category_id=13">2</a></li>
       <li><a href='/ec99/rwd1320/category_doct.asp?pageno=2&mode=listView&category_id=13'>&gt;</a></li><li><a href='/ec99/rwd1320/category_doct.asp?pageno=2&mode=listView&category_id=13'>&ge;</a></li>
       </ul>
       ```
   - **`main_list_13_2.html`**：
     - 醫師：20003（`主治醫師：張小美（兼任）`），`<list>` 13。
     - 分頁區塊與第 1 頁相同，只差兩處：`active` 改在第 2 頁那個 `<li>`；「下一頁」改為 `<li class='disabled'><a>&gt;</a></li>`。`&ge;` 連結仍指向 `pageno=2`。
   - **`main_list_7_1.html`**：醫師 20002（`兼任主治醫師：李小華`），`<list>` 7，沒有分頁區塊。

   **5-2 總院個人頁**
   - **共同格式**：title 為 `醫師介紹`，body 為：
     ```html
     <h1><span id="pd_name">職稱：姓名</span></h1>
     <input type=hidden name="prodName" value='職稱：姓名'>
     <div class="tab-content">
     <div class="tab-pane dpficon active" id="tab1" role="tabpanel">
     <div class="demo">
     <div class="demo-style">
     （學歷段）
     <h2 class="cgh2_title">經歷</h2>
     <ul>
     	<li>國泰綜合醫院總醫師</li>
     </ul>
     </div></div></div></div>
     ```
     學歷段的格式是 `<h2 class="cgh2_title">學歷</h2>`，加一行空行，再接 `<ul>`，每行一個 `\t<li>…</li>`，最後 `</ul>`。
   - **`main_profile_20001.html`**：`主治醫師：王小明(消化內科主任)`，學歷兩行：`國立台灣大學醫學系`、`國立台灣大學臨床醫學研究所碩士`。
   - **`main_profile_20002.html`**：`兼任主治醫師：李小華`，學歷一行：`臺北醫學大學醫學系`。
   - **`main_profile_20003.html`**：`主治醫師：張小美（兼任）`，**沒有學歷段**。

   **5-3 汐止**
   - **`sijhih_category_2.html`**：title 為 `專科介紹`，body 依序 4 條：
     1. `<a class="col-md-12 col-xs-12 btn_division" href="category.asp?category_id=58" title="消化內科(原胃腸肝膽科)">消化內科(原胃腸肝膽科)</a>`
     2. `<div class="btn_division"><a href="/ec99/rwd102/category.asp?category_id=61" title="前往精神科">精神科</a></div>`
     3. `<a class="col-md-12 col-xs-12 btn_division" href="https://h2.example.invalid/ec99/rwd102/category.asp?category_id=92" title="核子醫學科">核子醫學科</a>`
     4. `<a class="col-md-12 col-xs-12 btn_division" href="category.asp?category_id=60" title="營養組">營養組</a>`
   - **`sijhih_category_58.html`**：body 為 `<h2>消化內科</h2><a class="col-md-12 col-xs-12 btn_division" href="category_doct.asp?mode=listView&amp;category_id=85" target="_blank" title="(另開視窗)">醫師介紹</a>`。
   - **`sijhih_category_61.html`**：body 為 `<h2>精神科</h2><p>門診時間請見掛號系統。</p>`，沒有任何 `category_doct` 連結。
   - **`sijhih_list_85_1.html`**：醫師 20001（`主治醫師：王小明`）與 30001（`主治醫師：張小美`），`<list>` 85，沒有分頁區塊。
   - **`sijhih_profile_30001.html`**：同 5-2 的共同格式，`主治醫師：張小美`，學歷一行：`中山醫學大學醫學系`。

   **5-4 其他**
   - **`list_empty.html`**：列表頁，沒有醫師卡片、沒有分頁區塊（body 為 `<p>目前沒有資料</p>`）。
6. 新增 `internal/crawl/cgh_test.go`。fixture server `cghFixtureServer(t *testing.T, replace map[string]string) *httptest.Server` 依下面規則把請求對應到 fixture 檔名：
   1. 用 `path.Split(r.URL.Path)` 取得目錄與檔名。
   2. 目錄對應院區：`/ec99/rwd1320/` → `main`、`/ec99/rwd102/` → `sijhih`、`/ec99/rwd103/` → `hsinchu`、`/ec99/rwd1309A/` → `neihu`。其他目錄回 404（包括 `/robots.txt`；fetcher 對 404 的 robots 會當成全部允許）。
   3. 檔名對應 fixture：

      | 檔名 | fixture |
      |---|---|
      | `category.asp` | `<院區>_category_<category_id>.html` |
      | `category_doct.asp` | `<院區>_list_<category_id>_<pageno>.html` |
      | `product_doct.asp` | `<院區>_profile_<prodid>.html` |
      | 其他 | 404 |

   4. 組出的檔名如果是 `replace` 的 key，就換成對應的值。
   5. 最後用 `serveFixture(w, "cgh", name)` 回應。

   測試只跑前兩個院區：`s := NewCgh(srv.URL, srv.URL, srv.URL, srv.URL, f); s.campuses = s.campuses[:2]`。寫以下 8 個測試：
   - `TestParseCghIndex`：`main_category_1553.html` 回 `[]cghDept{{"13", "消化內科"}, {"7", "心血管中心"}}`。被註解掉的 26 不在結果裡。
   - `TestParseCghBranchIndex`：`sijhih_category_2.html` 回 `[]cghDept{{"58", "消化內科(原胃腸肝膽科)"}, {"61", "精神科"}}`。
   - `TestParseCghListIDs`：
     - `sijhih_category_58.html` 回 `[]string{"85"}`。
     - `sijhih_category_61.html` 回 nil。
   - `TestParseCghListPage`：
     - `main_list_13_1.html` 回 `([]string{"20001", "20002"}, 2)`。
     - `main_list_7_1.html` 回 `([]string{"20002"}, 1)`。
   - `TestParseCghProfile`：
     - `main_profile_20001.html` 回 `王小明`、`[]string{"國立台灣大學醫學系", "國立台灣大學臨床醫學研究所碩士"}`。
     - `main_profile_20003.html` 回 `張小美`、nil。
     - `"<html></html>"` 回 error。
   - `TestCghCrawlFirstCampusWins`：`replace` 為 nil，`Crawl` 成功。斷言：
     - 回 4 筆，Key 依序為 `cgh:20001`、`cgh:20002`、`cgh:20003`、`cgh:30001`。
     - 20001：`Hospital == "國泰綜合醫院"`、`City == "台北"`、`Departments == []string{"消化內科"}`（不含汐止的科別）、`Name == "王小明"`。
     - 20002：`Departments == []string{"消化內科", "心血管中心"}`。
     - 20003：`Name == "張小美"`、`Education` 為 nil。
     - 30001：`Hospital == "汐止國泰綜合醫院"`、`City == "新北"`、`Departments == []string{"消化內科(原胃腸肝膽科)"}`，`URL` 符合 `/ec99/rwd102/product_doct\.asp\?prodid=30001&category_id=85$`。
     - 以下頁面都沒有 fixture，被抓到會 404 而失敗，所以 Crawl 成功就證明它們沒被抓：第 3 頁 `main_list_13_3.html`、死連結 92、營養組 60、汐止的 20001 個人頁。
   - `TestCghCrawlFailsWhenCampusHasNoDoctors`：
     - `replace` 為 `{"sijhih_list_85_1.html": "list_empty.html"}`，fetcher 的 `sleep` 設成 no-op。
     - 斷言 `Crawl` 回 error 且結果為 nil。
   - `TestCghCrawlFailsOnMissingPage`：
     - `replace` 為 `{"main_list_13_2.html": "missing.html"}`，`missing.html` 不存在，所以會回 404。fetcher 的 `sleep` 設成 no-op。
     - 斷言 `Crawl` 回 error 且結果為 nil。
7. 新增 `internal/crawl/cgh_live_test.go`，第一行 `//go:build live`。`TestCghLive` 用 `NewCgh("https://www.cgh.org.tw", "https://sijhih.cgh.org.tw", "https://hsinchu.cgh.org.tw", "https://neihu.cgh.org.tw", NewFetcher(time.Second))`，最多 15 個請求，log 只記數量：
   - 總院：`parseCghIndex` 至少 35 個。第一個列表的第 1 頁至少 1 個 ID，`lastPage` 至少 1。第一個 ID 的個人頁（`profileURL(ids[0], lists[0].ID)`）有姓名，學歷至少 1 行。
   - 汐止、新竹、內湖：各自的 `parseCghBranchIndex` 至少 10 個。
   - 汐止：依序抓前 3 個科別的內容頁，遇到第一個 `parseCghListIDs` 非空的就停止。斷言有找到。

---

## Part 5：註冊來源與文件

**依賴**：Part 2、3、4。

### 現況

- `cmd/crawl/main.go` 的 `registry` 只有 `"femh"`，`-sources` 的說明寫 `(目前支援:femh)`。
- `docs/open-decisions.md` 的 OD-10 最後一則補註是 `2026-10-12 補註:網路錯誤與 5xx…`。
- `docs/sources/medical-center-inventory.md` 有幾格已經過時：雙和寫「總索引還沒找到」、長庚寫「沒有靜態的科別總表」、馬偕寫「一頁列出全部」、國泰寫「41 個科別」和「各院區是否共用未驗證」；第 4 節列出的法定聲明也漏了長庚。

### 目標

- `go run ./cmd/crawl -sources shh,mmh,cgh -dry-run` 可以跑。
- OD-10 記下本 task 的規則延伸。
- 盤點文件反映這次調查的結果。

### 不可動的範圍

- OD-10 既有的文字一字不改，只在最後一則補註之後追加；其他 OD 不動。
- 盤點文件只改下面列出的字串，再追加第 6 節。

### 實作步驟

1. 在 `cmd/crawl/main.go` 的 `registry` 加入以下三項（gofmt 會對齊冒號）：
   ```go
   "shh":  crawl.NewShh("https://shh.tmu.edu.tw", fetcher),
   "mmh":  crawl.NewMmh("https://www.mmh.org.tw", fetcher),
   "cgh":  crawl.NewCgh("https://www.cgh.org.tw", "https://sijhih.cgh.org.tw", "https://hsinchu.cgh.org.tw", "https://neihu.cgh.org.tw", fetcher),
   ```
   並把 flag 說明的 `(目前支援:femh)` 改為 `(目前支援:femh,shh,mmh,cgh)`。
2. 在 `docs/open-decisions.md` 的 OD-10 中，緊接在 `  - 2026-10-12 補註:網路錯誤與 5xx` 那一行之後，插入以下 5 行（兩個空格縮排、半形標點，與上下文一致）：
   ```
     - 2026-10-12 補註(第二批:雙和 `shh`、馬偕 `mmh`、國泰 `cgh`):同一位醫師列在同一醫院體系的多個院區時,一人一筆:依固定院區順序(國泰:總院→汐止→新竹→內湖),第一次出現的院區當醫院與縣市,科別只收那個院區的,`source_key` 不含院區。各院區網址不同但醫師 ID 共用,存多筆會讓同一人出現多張卡、下架也要逐筆做。
     - 2026-10-12 補註:學歷規則加「醫學學士」。長庚有醫師寫「OO大學醫學學士」,字元是醫、學、學、士,不含「醫學士」子字串。「生物醫學系」「獸醫學系」與「醫學大學附設醫院 住院醫師」這類可能誤判的寫法,四家抽樣都沒有出現,暫不處理。
     - 2026-10-12 補註:馬偕先只收 `find_doctor_list.php` 不帶 cookie 時列出的台北院區清單(含兩院都有門診的醫師,記為台北、馬偕紀念醫院);只在淡水看診的醫師要用 session 切換清單,之後再補。馬偕兒童醫院的醫師編號另成一套、大多也在主站清單,不收。
     - 2026-10-12 補註:雙和排除營養、藥劑、心理諮商與健檢類單位(健檢類中心的醫師用另一組 ID 重複列出);病理、核醫、急診、重症、臨床病理 5 科沒有醫師個人頁,不收。
     - 2026-10-12 補註:某個院區(國泰)或整個來源(雙和、馬偕)解析出 0 位醫師時整個來源失敗,不寫入:小院區被改版清空時,「不到一半」的刪除保護擋不住。馬偕另外檢查清單寫的「共 N 名」等於解析到的筆數、院區標記是台北。
   ```
3. 在 `docs/sources/medical-center-inventory.md` 做以下字串替換（每個舊字串都只出現一次）：

   | 舊字串 | 新字串 |
   |---|---|
   | `` `shh.tmu.edu.tw/page/TeamDoc.aspx?deptCode=<科別>`;**臨床科別的總索引還沒找到** `` | `` `shh.tmu.edu.tw/page/team.aspx`(門診科別總索引,66 個 `deptCode`)→ `page/TeamDoc.aspx?deptCode=<科別>` `` |
   | `` `cgmh.org.tw/tw/Services/DeptInfo/<院區>/<部>/<科>`(林口=3、高雄=8);**沒有靜態的科別總表**,下拉選單走 POST `GetDept`(不用),科別代碼要從舊站 `www1.cgmh.org.tw`、掛號站 `register.cgmh.org.tw` 或搜尋結果收集 `` | `` 各院區科別總表 `cgmh.org.tw/tw/Services/DeptList/<院區>`(林口=3、高雄=8,11 個院區有科別)→ `tw/Services/DeptInfo/<院區>/<部>[/<科>[/<子科>]]` `` |
   | `(一頁列出全部,約 465 KB,寫「共 607 名」)` | `(不帶 cookie 時只列台北院區,約 460 KB,寫「共 607 名」;只在淡水看診的醫師要用 session 切到淡水才列出)` |
   | ``「主要學歷」h2 區塊,`<br>` 分隔的自由文字,偶爾混入住院醫師訓練要過濾`` | ``「主要學歷」h2 區塊,自由文字,`<br>`、`<p>`、`<ul><li>` 都有,區塊內可能再包一層 `</ul>` `` |
   | `\| 607 \| 只寫 Googlebot allow \|` | `\| 台北清單 504 位有個人頁(加淡水共 568) \| 只寫 Googlebot allow(沒有 `*` 群組,等於全部允許) \|` |
   | `(41 個科別列表)` | `(40 個科別;分院多一層:專科介紹頁 → 科別內容頁 → 醫師列表)` |
   | `` `prodid` 5 位數字;各院區是否共用未驗證 `` | `` `prodid` 5 位數字(有前導 0),四個院區共用 `` |
   | `台大、新竹台大、花蓮慈濟的頁尾看得到` | `台大、新竹台大、花蓮慈濟、長庚的頁尾看得到` |

   （表中的 `\|` 是表格跳脫，實際字串是 `|`。）
4. 在 `docs/sources/medical-center-inventory.md` 檔尾追加：
   ```markdown

   ## 6. 第二批 spike(2026-10-12)

   雙和、長庚、馬偕、國泰各做一次結構調查,共約 180 次請求(每次間隔 2 秒、只發 GET、不送表單),回報不含醫師姓名。

   - **雙和**:總索引 `page/team.aspx` 有 66 個門診科別(`ShhregTable.aspx?deptCode=<碼>`,一般外科分 6 條同碼連結)。營養室、藥劑部、自費心理諮商門診不是醫師;代謝及體重管理中心、健檢中心、成人預防保健健康檢查專區的醫師用另一組 `docCode` 重複列出;都排除。病理、核醫、急診、重症、臨床病理 5 科只有自由文字的團隊介紹、沒有 `docCode`。個人頁 `<title>` 固定是院名,姓名在 `<h2>`;同一個 `docCode` 在不同科別的個人頁各存一份。估計 350–450 位。
   - **長庚**:每個院區有靜態科別清單 `tw/Services/DeptList/<院區>`(3 林口、1 台北、2 基隆、5 桃園、6 嘉義、M 雲林、8 高雄、V 土城、T 鳳山、W 大同、B 長庚診所),共約 491 個科別頁,科別頁沒有分頁。個人頁 `tw/Services/DoctorInfo/<id>` 與院區無關,同一人多院區同一個 id;科別頁列的是所屬院區。抽樣 22 頁中有 4 頁穩定回 500,都是職位「醫師」的字母 id。網站回應慢(個人頁中位數約 2 秒),估計約 4,000 位、一輪 3–4 小時。有醫師寫「醫學學士」。
   - **馬偕**:`find_doctor_list.php` 不帶 cookie 時只列台北院區(607 筆,504 位有個人頁);只在淡水看診的 64 位要先 GET `inc/ajax/setarea.php?area=2` 把 session 切到淡水。個人頁內容與網址的 `depid` 無關。兒童醫院 `child/find_doctor_list.php` 的 `did` 是另一套編號,87 位中 83 位也出現在主站。前面有 F5 WAF,2 秒間隔沒被擋。
   - **國泰**:總院索引 40 個科別(有一條被 HTML 註解掉的連結);分院多一層「專科介紹頁 → 科別內容頁 → 醫師列表」;汐止有兩條連到不存在網域 `h2.cgh.org.tw` 的死連結(含核子醫學科)。`prodid` 四個院區共用;列表頁一頁 12 人,頁碼超過最後一頁時回傳最後一頁,要用「最後一頁」連結決定頁數。`cgh.org.tw`(不帶 www)連不上,要用 `www.cgh.org.tw`。每抓一次個人頁,頁面的人氣計數會加 1。
   ```

---

## 驗證計畫

| Part | 驗證方式 | 預期斷言 | 為什麼這個斷言能證明目標達成 |
|------|---------|---------|------------------------------|
| Part 1 | `internal/crawl/crawl_test.go:TestDoctorIndex` | 順序依第一次出現；第一個科別碼保留；科別名稱依序累積 | 這是 femh、shh、cgh 共用的彙整規則，直接釘住「個人頁用第一個科別抓、科別依序串」 |
| Part 1 | `internal/crawl/femh_test.go:TestFemhCrawlAggregatesDepartments` | 與搬移前相同（3 位、跨科去重、加護病房沒被抓） | femh 改用共用 helper 後行為不變；這個測試在搬移前就存在 |
| Part 1 | `internal/crawl/education_test.go:TestIsMedicalDegreeLine` | `長庚大學醫學學士` 為 true，其他既有 case 不變 | 直接對應「醫學學士」規則漏洞；既有的藥學系、碩士等反例仍為 false |
| Part 1 | `! grep -n -E 'femhText\|femhTagRe\|femhItemRe\|readFemhFixture\|doctorDepts' internal/crawl/*.go` | exit 0 | 搬移後沒有留下第二份舊 helper |
| Part 2 | `internal/crawl/shh_test.go:TestParseShhIndex` | 只回 AB、BD、01；BD 用第一個名稱；營養室、Introduction、javascript 連結都不在 | 釘住索引的去重、排除與 `../page/` 前綴三種情況 |
| Part 2 | `internal/crawl/shh_test.go:TestParseShhProfile` | 姓名取 h2 的第一段；項目符號不留在學歷裡；沒有學歷區塊回 nil；沒有姓名回 error | 雙和的 title 不能用、學歷有 `<span class="point">`，這兩個坑都有覆蓋 |
| Part 2 | `internal/crawl/shh_test.go:TestShhCrawlAggregatesDepartments` | 3 位；跨科醫師一筆、科別依序、個人頁用第一個科別；空科別頁不算錯 | 端到端證明雙和的流程與 State Enumeration 的「同院區多科」「單一科別沒有醫師」兩格 |
| Part 2 | `internal/crawl/shh_test.go:TestShhCrawlFailsOnMissingDeptPage` | 科別頁 500 時回 error、結果為 nil | 「任一頁失敗」那一格：部分結果不會被拿去同步 |
| Part 3 | `internal/crawl/mmh_test.go:TestParseMmhList` | 3 筆（跳過沒有 did 的、去重）；院區不是 TP 回 error；筆數不符回 error | 覆蓋 State Enumeration 中 mmh 的五格；兩個 error 是版型變化的保護 |
| Part 3 | `internal/crawl/mmh_test.go:TestParseMmhProfile` | 內層 `</ul>` 之後的那行也有抓到；空區塊回 nil | 學歷區塊結尾必須錨在下一個 h2，否則會漏掉後面的行 |
| Part 3 | `internal/crawl/mmh_test.go:TestMmhCrawl` | 姓名取自個人頁而不是清單；科別來自 `<sub>`；空 `<sub>` 為 nil | 清單姓名可能含私用區字元，這個測試證明不會被存進 DB |
| Part 3 | `internal/crawl/mmh_test.go:TestMmhCrawlFailsOnMissingProfile` | 個人頁 500 時回 error、結果為 nil | 「任一頁失敗」那一格 |
| Part 4 | `internal/crawl/cgh_test.go:TestParseCghIndex` | 只回 13、7；被註解掉的 26 不在 | 註解掉的連結用 `&`，沒刪註解就會被抓進來 |
| Part 4 | `internal/crawl/cgh_test.go:TestParseCghBranchIndex` | 回 58、61；死連結與營養組不在；兩種連結寫法都抓到 | 覆蓋分院索引的兩種版型、站外死連結、排除單位 |
| Part 4 | `internal/crawl/cgh_test.go:TestParseCghListPage` | 第 1 頁回 2 個 ID 與最後一頁 2；沒有分頁的回 1 | 頁數取自 `&ge;` 連結，不靠「抓到空頁」 |
| Part 4 | `internal/crawl/cgh_test.go:TestCghCrawlFirstCampusWins` | 4 位；20001 只有總院的醫院、縣市與科別；30001 是汐止的醫院、縣市與網址；沒有抓第 3 頁、死連結、營養組 | 直接對應使用者決定的「多院區一人一筆、依院區順序取第一個」，同時證明分頁不會越界 |
| Part 4 | `internal/crawl/cgh_test.go:TestCghCrawlFailsWhenCampusHasNoDoctors` | 汐止列表為空時回 error | 「某院區 0 位」那一格：小院區被清空時不會把它的醫師刪掉 |
| Part 4 | `internal/crawl/cgh_test.go:TestCghCrawlFailsOnMissingPage` | 第 2 頁 404 時回 error、結果為 nil | 「任一頁失敗」那一格 |
| Part 2–4 | `go test -tags live ./internal/crawl -run 'TestFemhLive\|TestShhLive\|TestMmhLive\|TestCghLive' -count=1 -v` | 四個測試都通過 | 合成 fixture 只仿結構；這證明真實網站目前就是這個結構，四個國泰院區的索引網址都正確，femh 搬移後也還能用 |
| Part 2–4 | `test -z "$(find internal/crawl/testdata -name '*.html' -size +8k)"` | exit 0 | 真實頁面都在 16 KB 以上；這個檢查擋住誤把真實頁面（含醫師姓名）放進 public repo |
| Part 5 | `go run ./cmd/crawl -sources shh,mmh,cgh,nosuch 2>&1 \| grep -q 'unknown source .*nosuch'` | exit 0 | 三個新來源都已註冊（否則會先在 `shh` 報錯），而且仍在連 DB 前驗證 |
| Part 5 | `test "$(grep -c '2026-10-12 補註' docs/open-decisions.md)" -ge 9 && grep -q '學歷規則加「醫學學士」' docs/open-decisions.md && grep -q '一人一筆:依固定院區順序' docs/open-decisions.md` | exit 0 | OD-10 已記下本 task 的規則延伸（原本全文有 4 則「2026-10-12 補註」，加上 5 則；另兩個字串只出現在新補註裡） |
| Part 5 | `! grep -q -E '總索引還沒找到\|沒有靜態的科別總表\|一頁列出全部\|各院區是否共用未驗證' docs/sources/medical-center-inventory.md && grep -q '^## 6. 第二批 spike' docs/sources/medical-center-inventory.md` | exit 0 | 過時的敘述都已更正，調查結果有記錄 |

---

## 驗收

- [ ] `go build ./...`
- [ ] `go vet ./...`
- [ ] `go test ./...`
- [ ] `test -z "$(gofmt -l $(find . -name '*.go' -not -path './internal/reconcile/fetch.go'))"`
- [ ] `go test ./internal/crawl -count=1 -v`
- [ ] `test "$(cat internal/crawl/crawl_test.go internal/crawl/education_test.go internal/crawl/femh_test.go internal/crawl/shh_test.go internal/crawl/mmh_test.go internal/crawl/cgh_test.go | grep -c -E '^func (TestDoctorIndex|TestIsMedicalDegreeLine|TestFemhCrawlAggregatesDepartments|TestParseShhIndex|TestParseShhDept|TestParseShhProfile|TestShhCrawlAggregatesDepartments|TestShhCrawlFailsOnMissingDeptPage|TestParseMmhList|TestParseMmhProfile|TestMmhCrawl|TestMmhCrawlFailsOnMissingProfile|TestParseCghIndex|TestParseCghBranchIndex|TestParseCghListIDs|TestParseCghListPage|TestParseCghProfile|TestCghCrawlFirstCampusWins|TestCghCrawlFailsWhenCampusHasNoDoctors|TestCghCrawlFailsOnMissingPage)\(')" = 20`
- [ ] `grep -q '長庚大學醫學學士' internal/crawl/education_test.go && grep -q '"醫學學士"' internal/crawl/education.go`
- [ ] `! grep -n -E 'femhText|femhTagRe|femhItemRe|readFemhFixture|doctorDepts' internal/crawl/*.go`
- [ ] `test "$(grep -l -E '^func htmlText\(' internal/crawl/*.go)" = "internal/crawl/crawl.go" && test "$(grep -l -E '^func readFixture\(' internal/crawl/*.go)" = "internal/crawl/testutil_test.go"`
- [ ] `for f in shh mmh cgh; do head -1 internal/crawl/${f}_live_test.go | grep -q '^//go:build live' || exit 1; done`
- [ ] `test "$(grep -c -E '^func (TestShhLive|TestMmhLive|TestCghLive)\(' internal/crawl/shh_live_test.go internal/crawl/mmh_live_test.go internal/crawl/cgh_live_test.go | awk -F: '{s+=$2} END {print s}')" = 3`
- [ ] `go test -tags live ./internal/crawl -run 'TestFemhLive|TestShhLive|TestMmhLive|TestCghLive' -count=1 -v`
- [ ] `test -z "$(find internal/crawl/testdata -name '*.html' -size +8k)"`
- [ ] `go run ./cmd/crawl -sources shh,mmh,cgh,nosuch 2>&1 | grep -q 'unknown source .*nosuch'`
- [ ] `grep -q '目前支援:femh,shh,mmh,cgh' cmd/crawl/main.go`
- [ ] `test "$(grep -c '2026-10-12 補註' docs/open-decisions.md)" -ge 9 && grep -q '學歷規則加「醫學學士」' docs/open-decisions.md && grep -q '一人一筆:依固定院區順序' docs/open-decisions.md`
- [ ] `! grep -q -E '總索引還沒找到|沒有靜態的科別總表|一頁列出全部|各院區是否共用未驗證' docs/sources/medical-center-inventory.md && grep -q '^## 6. 第二批 spike' docs/sources/medical-center-inventory.md`

---

## 執行報告

### 摘要
| Part | 狀態 | 說明 |
|------|------|------|
| Part 1：共用 helper 與學歷規則 | ✅ 完成 | `tagRe`、`listItemRe`、`htmlText`、`doctorIndex` 搬到 `crawl.go`；`readFixture`、`serveFixture` 搬到 `testutil_test.go`；femh 改用共用版、測試照舊通過；加「醫學學士」 |
| Part 2：雙和（`shh`） | ✅ 完成 | `NewShh`、3 個 parser、`Crawl`、7 個合成 fixture、5 個單元測試、`TestShhLive` |
| Part 3：馬偕（`mmh`，台北清單） | ✅ 完成 | `NewMmh`、清單的院區與筆數檢查、學歷區塊錨在下一個 h2、4 個合成 fixture、4 個單元測試、`TestMmhLive` |
| Part 4：國泰（`cgh`，四個院區） | ✅ 完成 | `NewCgh`、5 個 parser、依院區順序一人一筆（`taken`）、頁數取自 `&ge;`、13 個合成 fixture、8 個單元測試、`TestCghLive` |
| Part 5：註冊來源與文件 | ✅ 完成 | `cmd/crawl` 註冊三個來源；OD-10 追加 5 則補註；盤點文件替換 8 個字串並追加第 6 節 |

### ❌ 未通過的驗收項目
無

### 🔍 需人工確認的驗收項目
無

### 🤔 執行中的判斷決策
- **Part 1, Step 2**：步驟寫「刪掉 femh.go 因此不再用到的 import（`html` 已移到 crawl.go）」，但 `parseFemhProfile` 仍用 `html.UnescapeString` 解 `<title>`、也仍用 `strings`。決策：兩個 import 都保留，只刪真正不再用到的（沒有）。
- **Part 2–4, fixture**：步驟把頁面外框寫成一行（`<!DOCTYPE html><html …><head>…</head><body>…</body></html>`）。決策：照既有 `testdata/femh/` 的寫法分行，標籤、屬性與內容不變；parser 都不依賴外框的換行。
- **Part 3, Step 3**：「共 N 名」與解析筆數的比較，以十進位字串比對（`count != fmt.Sprint(len(matches))`），沒比對到時 `count` 為空字串，error 訊息照步驟格式。`\d+` 不會有前導 0，與數值比較等價。
- **Part 4, Step 6**：三個 `Crawl` 測試都要「只跑前兩個院區」。決策：寫一個 `newTestCgh(srv, f)`（3 個呼叫處）包住 `NewCgh(srv.URL, …); s.campuses = s.campuses[:2]`，不在三處各抄一次。
- **Part 4, Step 4 (2)**：分院的列表以列表 id 去重（同一列表被兩個科別連到時保留第一個科別名稱），用 `crawlCampus` 內的 `seen` map 實作；總院由 `parseCghIndex` 去重。

### ⚠️ 注意事項
- **新的共用 helper**：之後的來源（長庚）用 `crawl.go` 的 `htmlText`、`listItemRe`、`doctorIndex`，測試用 `testutil_test.go` 的 `readFixture`、`serveFixture`，不要再各抄一份。`internal/reconcile` 的 `tagRe` 是另一個 package 的，刻意不合併。
- **多院區一人一筆**（OD-10 補註）：國泰依總院→汐止→新竹→內湖，第一個列出醫師的院區當醫院、縣市與科別；`source_key` 不含院區。長庚 task 沿用同一規則。
- **某院區 0 位就整個來源失敗**：`crawlCampus` 計算的 `found` 含已被前面院區收走的 ID，所以「全被收走」不算 0 位。
- **live 測試 2026-10-12 結果**：femh 48 科；shh 60 科（66 減 6 個排除單位，與調查一致）；mmh 台北清單 504 位有個人頁（與調查一致）；cgh 總院 40 個列表、汐止 31／新竹 30／內湖 20 個科別。
- **merge 後（不在本 task）**：手動 Job 跑 `/crawl -sources shh,mmh,cgh -dry-run` 看數字，再到 jerrytech-deploy 把 CronJob 改成 `-sources femh,shh,mmh,cgh`。
