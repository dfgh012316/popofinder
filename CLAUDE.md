# CLAUDE.md

POPO Finder — 讓使用者查詢台灣醫師學歷背景(是否為台灣醫學系畢業 / 波波)的服務。Go + Postgres,目前介面為 LINE bot,自架部署於樹莓派。

## ⚠️ 討論新功能前,先讀產品方向

**在規劃、提議或實作任何新功能之前,務必先閱讀 [`docs/product-direction.md`](docs/product-direction.md)。**

那份文件記錄了一次完整的方向定盤,核心是一道護欄:

> POPO Finder 是「**醫師學歷字典 / 參考工具**」,**不是**要長成網路效應的平台。
> 決策準則:**「如果全世界只有 3 個人用,它對那 3 個人還有價值嗎?」** 通不過的功能一律延後/不做。

已被**刻意否決**的方向(不要再提議,除非使用者主動推翻):評價系統、信號/unraveling 引擎、SEO 自動認領、主動行銷 email。擴展的唯一正當目的是「提高資料覆蓋率與正確性」;醫師認領要定位為「眾包校對」而非成長引擎。倫理鐵律見該文件第 5 節(中性呈現、絕不暗示「疑似波波」、誠實標示驗證程度、要有下架機制)。

**每當要做任何產品決策時,除了上面的方向文件,務必同時回顧 [`docs/open-decisions.md`](docs/open-decisions.md)** —— 那裡記錄了定盤之後陸續做過的個別決策與理由,用來避免前後矛盾、避免重複推導已經想過的事。做出新決策後,也要把它追加進那份文件(推翻舊決策時改狀態並註明原因,不要刪除歷史)。

## 架構

- Go 單體,監聽 `:8000`,使用 chi router。對外端點:`POST /api/linebot/callback`(LINE webhook)、`GET /health`。
- `cmd/server/main.go` 進入點;`internal/` 下分 config / database / server / linebot(client·handler·search·session·template·webhook)。
- DB:Postgres,透過 sqlx + pgx;目前單表 `medical_personnel`(id, city, hospital, department, name, education)。
- 設定由環境變數載入(`internal/config`),`.env` 可選;見 `.env.example`。

## 常用指令

- `make local-build` / `make local-run` — 本地 Docker build 與跑(:8000)。
- `make build` — buildx 多平台(arm64 + amd64,給樹莓派)推 image,需 `DOCKER_USER`。
- `go test ./...` — 跑測試(search / session / webhook 有單元測試)。
