# POPO Finder

查詢台灣醫師學歷的 LINE bot 與網頁版。收錄醫療院所官網、網路公開彙整與民眾回報的醫師學歷，台灣與國外畢業的都有；卡片照實顯示學歷與資料來源，不做任何判斷。

POPO Finder 的定位是「醫師學歷字典」：查一位醫師在哪裡受醫學教育，判斷留給你自己。它不是評價平台，也不會標示誰是不是「波波」。

## 開始使用

- 網頁版：https://popo.jerrytech.me
- LINE：[點擊我加入好友](https://lin.ee/h1GxF9s)，或掃描 QR code

![QRcode](./assets/join-qrcode.png)

## 怎麼查

網頁版和 LINE 的查詢方式相同。直接輸入關鍵字（姓名、醫院或科別），多個條件用空白分開，每個條件都要符合：

| 輸入 | 找什麼 |
|---|---|
| `陳` | 姓名、醫院或科別含「陳」 |
| `三總 牙科` | 三總的牙科醫師 |
| `台北 牙科` | 台北的牙科醫師（縣市可以放在任何位置） |
| `台北@醫院 三總` | 只比對醫院名稱 |
| `台北@科別 心臟` | 只比對科別，打部分字即可（心臟內科、心臟血管外科都會找到） |

- 常見簡稱也查得到，例如三總、北榮、台大、高醫、林長、彰基、部桃，完整清單在 [`internal/linebot/search/alias.go`](internal/linebot/search/alias.go)。
- 醫師卡片上的「同院同科醫師」會查同一家醫院、同科別的其他醫師。
- 輸入「醫院清單」看哪些醫院收錄了官網醫師名單、各有幾位；輸入「幫助」看使用說明。
- 聊天室下方的選單也有這些入口：

![圖文選單](./assets/richmenu/richmenu.png)

## 收錄範圍與資料來源

- **醫療院所官網**：亞東、雙和、馬偕、國泰、長庚、高醫、小港、蒔美牙醫等院所的醫師介紹頁，只收有寫醫學學歷的醫師。卡片照抄學歷原文，附官網連結與查到的日期。
- **網路公開彙整**：[波波一覽表](https://popolist999.blogspot.com/)。這個部落格 2025 年 3 月後就沒有更新，卡片會標示「網路公開彙整（2025 年 3 月前）」，醫院欄可能已經過時。
- **民眾回報**：使用者回報的資料。

截至 2026 年 11 月約收錄 6,100 位醫師。衛福部統計 114 年底全國醫療院所的執業醫師（含牙醫師、中醫師）有 81,539 人，所以大部分醫師還查不到。查得到不代表任何結論，請以卡片上的學歷為準；查不到也不代表什麼。

## 補充、更正與下架

在 bot 輸入「回報」，或點選單的「補充與更正」，可以看完整說明。都是寄信到 jerrychuang@jerrytech.me，由維護者人工處理：

- **補充學歷**：院所官網寫了醫師學歷的，附上網址就好，任何人都可以提供。官網沒寫的，請醫師本人來信，附上醫師證書字號與學歷證明；證明只用來核對，核對完就刪除。
- **更正或下架**：醫師本人或院所來信，並以院所官方信箱寄出，或附上可證明身分的資料。

## 原則

- 只呈現學歷事實，不對缺少的資料做任何暗示，也不判斷誰是「波波」。
- 誠實標示資料來源與時效。
- 醫師或院所要求下架就照辦，連同異動歷史一起清除。
- 查詢紀錄只存查詢內容，不存是誰查的。

方向與每一個決策的理由在 [`docs/product-direction.md`](docs/product-direction.md) 和 [`docs/open-decisions.md`](docs/open-decisions.md)。

## 開發

Go 單體服務（chi router），資料庫是 Postgres（sqlx + pgx），部署在樹莓派的 k3s 上。

- `cmd/server`：LINE bot 與網頁版，監聽 `:8000`；端點有 `POST /api/linebot/callback`（LINE webhook）、`GET /health`、`GET /readyz`、`GET /version`，以及網頁版的 `/`、`/search`、`/hospitals`、`/report`（`internal/web`）。網頁版的查詢結果不給搜尋引擎收錄，並限制每個 IP 的查詢頻率。
- `cmd/crawl`：醫院官網爬蟲。
- `cmd/reconcile`：重新比對部落格資料、回填來源。
- `migrations/`：資料表結構，部署時由 init container 套用。
- `assets/richmenu/`、`scripts/richmenu/`：LINE 圖文選單與套用腳本。

```sh
cp .env.example .env               # 填入 LINE channel 與 DB 設定
go run ./cmd/server                # 本機執行（:8000）
go test ./...                      # 單元測試，不需要 DB 與網路
make test-integration              # 起本機 Docker Postgres，跑 DB 整合測試
go run ./cmd/crawl -sources femh -dry-run   # 爬蟲，只印計畫、不寫入（讀 DATABASE_URL）
```

部署：PR merge 進 `develop` 後，CI 跑測試、build arm64 與 amd64 的 image 推到 Docker Hub，再通知部署 repo 更新 image tag。只改文件、README、`assets/`、`scripts/` 或測試的 PR 不會觸發 build。

架構的變更紀錄在 [`docs/ARCHITECTURE.md`](docs/ARCHITECTURE.md)。
