# Task：health / readiness / version 端點標準化

## 背景

統一 workspace 各服務的 health check 標準（決策見 jerry-wiki `concepts/health-checks.md`）。popofinder 目前只有單一靜態 `GET /health`，liveness 與 readiness 共用同一個不查相依的端點，等於沒有真正的 readiness。本 task 讓 popofinder 提供三分端點：`/health`（liveness，靜態，維持現狀）、`/readyz`（readiness，查 DB，失敗回 503）、`/version`（回傳注入的版本字串）。

popofinder 測試慣例為**不連 DB / 不連網路**（無 testcontainers、無 docker-compose）。因此 readiness 的 DB 檢查以一個可注入的 `Ping` 介面實作，測試用 stub 覆蓋成功/失敗兩路徑，不需真實 DB。

本 task **只涵蓋 popofinder 的 endpoint 與測試**。K8s probe 改指 `/readyz` 屬於 `jerrytech-deploy/charts/app` 的獨立變更，必須在本 task 的新 image 部署上線（`/readyz` 可用）之後才執行——不在本 task 範圍。

### 涉及檔案

- `internal/database/repository.go` — `Repository`（持有 `db *sqlx.DB`）；新增 `Ping` 方法
- `internal/server/server.go` — chi router 與 handler；新增 readiness 介面、`/readyz`、`/version`、Server 欄位、`New` 簽名
- `internal/config/config.go` — `Config`（caarlos0/env）；新增 `AppVersion` 欄位
- `cmd/server/main.go` — 組裝入口；更新 `server.New` 呼叫傳入版本
- `internal/server/server_test.go` — 新測試檔（白箱 `package server`，stub Pinger，不連 DB）

### Schema Impact

- 新增兩個 root-level（非 `/api` 前綴）端點：`GET /readyz`、`GET /version`（`GET /health` 已存在、行為不變）。
- `GET /readyz` 回應：成功 `{"status":"ok","checks":{"db":"ok"}}`（200）；失敗 `{"status":"unhealthy","checks":{"db":"error"}}`（503）。
- `GET /version` 回 `{"version":"<AppVersion>"}`，讀環境變數 `APP_VERSION`（由 deploy chart 注入，值 = image tag = commit short-SHA），本地未設時 `"unknown"`。
- `server.New` 簽名變更（新增 `version` 參數）；唯一 consumer 為 `cmd/server/main.go`（本 task 一併更新）。
- 下游消費者：`jerrytech-deploy/charts/app` 的 `readinessProbe`/`startupProbe` 將改打 `/readyz`（跨 repo，獨立 task）。

### State Enumeration

| State | 條件 | `/readyz` 結果 |
|-------|------|---------------|
| DB 可連 | `ready.Ping(ctx) == nil` | `200` + `{"status":"ok","checks":{"db":"ok"}}` |
| DB 不可連 | `ready.Ping(ctx) != nil` | `503` + `{"status":"unhealthy","checks":{"db":"error"}}` |

---

## Part 1：Repository 新增 `Ping` 方法

### 現況

`internal/database/repository.go:33-39` 的 `Repository` struct 持有 `db *sqlx.DB`。`*sqlx.DB` 內嵌 `*sql.DB`，提供 `PingContext(ctx)`。目前 `Repository` 無對外暴露連線健康檢查的方法。

### 目標

`Repository` 新增 `Ping(ctx context.Context) error`，回傳 `db.PingContext(ctx)` 的結果。

### 不可動的範圍

不得更動 `Repository` 既有的 `Search`/`SearchStats` 等查詢方法；不得更動 `NewRepository` 簽名。

### 實作步驟

1. 在 `internal/database/repository.go` 的 import block 加入 `"context"`。
2. 在 `NewRepository`（`:37-39`）之後新增：
   ```go
   // Ping verifies the database connection is usable (readiness check).
   func (r *Repository) Ping(ctx context.Context) error {
       return r.db.PingContext(ctx)
   }
   ```

---

## Part 2：Server 新增 readiness 介面與 `/readyz`

### 現況

`internal/server/server.go` 的 `Server` struct（`:18-24`）持有 `repo *database.Repository`。`routes()`（`:42-53`）註冊 `/health` 與 `/api/linebot/callback`。`handleHealth`（`:55-59`）用 `json.NewEncoder` + `WriteHeader(200)` 輸出靜態 `{"status":"ok"}`。Server 無 readiness 端點，且 handler 目前直接依賴具體 `*database.Repository`，不利於 stub 測試。

### 目標

定義 `readinessChecker` 介面（含 `Ping(ctx) error`），`Server` 新增 `ready readinessChecker` 欄位並在 `New` 中以傳入的 `repo` 賦值；新增 `GET /readyz`：以 2 秒 timeout 對 `s.ready.Ping` 做檢查，成功 `200` + `{"status":"ok","checks":{"db":"ok"}}`、失敗 `503` + `{"status":"unhealthy","checks":{"db":"error"}}`。以編譯期斷言確保 `*database.Repository` 滿足該介面。

### 不可動的範圍

不得更動 `handleHealth` 與 `handleCallback` 的行為；不得在 `/readyz` 檢查任何 outbound 依賴（例如 LINE API），只檢查 DB。

### 實作步驟

1. 在 `internal/server/server.go` 的 import block 加入 `"context"` 與 `"time"`。
2. 在 `Server` struct（`:18-24`）新增欄位 `ready readinessChecker`（放在 `repo` 之後）。
3. 在 `Server` struct 定義之後新增介面與編譯期斷言：
   ```go
   // readinessChecker is the dependency the /readyz handler probes. *database.Repository satisfies it.
   type readinessChecker interface {
       Ping(ctx context.Context) error
   }

   var _ readinessChecker = (*database.Repository)(nil)
   ```
4. 在 `routes()` 的 `s.router.Get("/health", s.handleHealth)`（`:46`）之後新增：
   ```go
   s.router.Get("/readyz", s.handleReady)
   ```
5. 在 `handleHealth`（`:59`）之後新增：
   ```go
   func (s *Server) handleReady(w http.ResponseWriter, r *http.Request) {
       ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
       defer cancel()
       w.Header().Set("Content-Type", "application/json")
       if err := s.ready.Ping(ctx); err != nil {
           w.WriteHeader(http.StatusServiceUnavailable)
           _ = json.NewEncoder(w).Encode(map[string]any{
               "status": "unhealthy",
               "checks": map[string]string{"db": "error"},
           })
           return
       }
       w.WriteHeader(http.StatusOK)
       _ = json.NewEncoder(w).Encode(map[string]any{
           "status": "ok",
           "checks": map[string]string{"db": "ok"},
       })
   }
   ```
6. （`New` 中對 `s.ready` 的賦值在 Part 3 步驟一併完成。）

---

## Part 3：新增 `/version`、config、New 簽名與 main 組裝

### 現況

`internal/config/config.go:11-20` 的 `Config` 全欄位為 `required`，無版本欄位。`server.New`（`server.go:26-36`）簽名為 `New(secret string, lineClient *client.Client, dispatcher *handler.Dispatcher, repo *database.Repository)`，`cmd/server/main.go` 是其唯一 caller（`Handler: server.New(cfg.LineMessageChannelSecret, lineClient, dispatcher, repo)`）。Server 無 `/version` 端點、無版本欄位。

### 目標

`Config` 新增可選 `AppVersion`（讀 `APP_VERSION`，預設 `unknown`）；`Server` 新增 `version string` 欄位；`New` 新增 `version string` 參數（置於 `secret` 之後）並賦值 `s.version` 與 `s.ready = repo`；新增 `GET /version` 回 `200` + `{"version":"<version>"}`；`main.go` 傳入 `cfg.AppVersion`。

### 不可動的範圍

不得把 `AppVersion` 標為 `required`（它可選、有預設）；不得更動 `Config` 其他欄位與 `DatabaseURL()`/`IsProduction()`。

### 實作步驟

1. 在 `internal/config/config.go` 的 `Config` struct 末尾（`DBName` 那行之後）新增：
   ```go
   AppVersion string `env:"APP_VERSION" envDefault:"unknown"`
   ```
2. 在 `internal/server/server.go` 的 `Server` struct 新增欄位 `version string`（放在 `ready` 之後）。
3. 將 `New` 簽名改為：
   ```go
   func New(secret, version string, lineClient *client.Client, dispatcher *handler.Dispatcher, repo *database.Repository) *Server {
   ```
   並在其 struct literal 中新增 `version: version,`、`ready: repo,`（與既有 `repo: repo,` 並列）。
4. 在 `routes()` 的 `s.router.Get("/readyz", s.handleReady)` 之後新增：
   ```go
   s.router.Get("/version", s.handleVersion)
   ```
5. 在 `handleReady` 之後新增：
   ```go
   func (s *Server) handleVersion(w http.ResponseWriter, r *http.Request) {
       w.Header().Set("Content-Type", "application/json")
       w.WriteHeader(http.StatusOK)
       _ = json.NewEncoder(w).Encode(map[string]string{"version": s.version})
   }
   ```
6. 在 `cmd/server/main.go` 中，將 `Handler: server.New(cfg.LineMessageChannelSecret, lineClient, dispatcher, repo),` 改為 `Handler: server.New(cfg.LineMessageChannelSecret, cfg.AppVersion, lineClient, dispatcher, repo),`。

---

## Part 4：新增 server 白箱測試（stub Pinger，不連 DB）

### 現況

`internal/server/` 目前無測試檔。popofinder 測試慣例為不連 DB/網路。`Server` 的 handler 可在白箱測試（`package server`）中直接以部分欄位建構並呼叫。

### 目標

新增 `internal/server/server_test.go`（`package server`），以 stub `readinessChecker` 覆蓋 `/readyz` 成功與失敗兩路徑、驗證 `/version` 與 `/health`，全程不連 DB。

### 不可動的範圍

僅新增測試檔，不得更動非測試程式碼。

### 實作步驟

1. 建立 `internal/server/server_test.go`，內容：
   ```go
   package server

   import (
       "context"
       "encoding/json"
       "errors"
       "net/http"
       "net/http/httptest"
       "testing"
   )

   type stubPinger struct{ err error }

   func (s stubPinger) Ping(ctx context.Context) error { return s.err }

   func decode(t *testing.T, rec *httptest.ResponseRecorder) map[string]any {
       t.Helper()
       var body map[string]any
       if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
           t.Fatalf("decode body: %v", err)
       }
       return body
   }

   func TestHandleHealth(t *testing.T) {
       s := &Server{}
       rec := httptest.NewRecorder()
       s.handleHealth(rec, httptest.NewRequest(http.MethodGet, "/health", nil))
       if rec.Code != http.StatusOK {
           t.Fatalf("health expected 200, got %d", rec.Code)
       }
       if body := decode(t, rec); body["status"] != "ok" {
           t.Fatalf("unexpected health body: %v", body)
       }
   }

   func TestHandleReadyOK(t *testing.T) {
       s := &Server{ready: stubPinger{err: nil}}
       rec := httptest.NewRecorder()
       s.handleReady(rec, httptest.NewRequest(http.MethodGet, "/readyz", nil))
       if rec.Code != http.StatusOK {
           t.Fatalf("readyz expected 200, got %d", rec.Code)
       }
       body := decode(t, rec)
       checks, _ := body["checks"].(map[string]any)
       if body["status"] != "ok" || checks["db"] != "ok" {
           t.Fatalf("unexpected readyz body: %v", body)
       }
   }

   func TestHandleReadyUnhealthy(t *testing.T) {
       s := &Server{ready: stubPinger{err: errors.New("db down")}}
       rec := httptest.NewRecorder()
       s.handleReady(rec, httptest.NewRequest(http.MethodGet, "/readyz", nil))
       if rec.Code != http.StatusServiceUnavailable {
           t.Fatalf("readyz on dead DB expected 503, got %d", rec.Code)
       }
       body := decode(t, rec)
       checks, _ := body["checks"].(map[string]any)
       if body["status"] != "unhealthy" || checks["db"] != "error" {
           t.Fatalf("unexpected unhealthy body: %v", body)
       }
   }

   func TestHandleVersion(t *testing.T) {
       s := &Server{version: "test-version"}
       rec := httptest.NewRecorder()
       s.handleVersion(rec, httptest.NewRequest(http.MethodGet, "/version", nil))
       if rec.Code != http.StatusOK {
           t.Fatalf("version expected 200, got %d", rec.Code)
       }
       if body := decode(t, rec); body["version"] != "test-version" {
           t.Fatalf("unexpected version body: %v", body)
       }
   }
   ```

---

## 驗證計畫

| Part | 驗證方式 | 預期斷言 | 為什麼這個斷言能證明目標達成 |
|------|---------|---------|------------------------------|
| Part 1 | `go build ./...`（含 `server.go` 的 `var _ readinessChecker = (*database.Repository)(nil)`） | exit code 0 | 編譯期斷言只有在 `*database.Repository` 具備符合簽名的 `Ping(ctx) error` 時才通過，直接證明 Part 1 的方法存在且正確 |
| Part 2 | `internal/server/server_test.go:TestHandleReadyOK` | HTTP 200、`status=="ok"`、`checks.db=="ok"` | Ping 成功時 `/readyz` 回 200 且回報 db:ok，證明 readiness 成功路徑正確 |
| Part 2 | `internal/server/server_test.go:TestHandleReadyUnhealthy` | HTTP 503、`status=="unhealthy"`、`checks.db=="error"` | Ping 失敗時 `/readyz` 回 503，證明 readiness 在相依故障時正確回報 not-ready（而非假 200） |
| Part 3 | `internal/server/server_test.go:TestHandleVersion` | HTTP 200、`version=="test-version"` | 以 `version` 欄位建構 Server，端點回傳該值，證明 `/version` 讀取注入的版本 |
| Part 3 | `go build ./...` | exit code 0 | `main.go` 以新 `New` 簽名（含 `cfg.AppVersion`）呼叫可編譯，證明 config 欄位與 New 參數 wiring 正確 |
| Part 4 | `go test ./internal/server/ -run 'TestHandleHealth\|TestHandleReady\|TestHandleVersion' -count=1` | 全部 pass | 四個 handler 行為在不連 DB 下被完整覆蓋 |

---

## 驗收

- [ ] `go build ./...`
- [ ] `go vet ./...`
- [ ] `go test ./internal/server/ -run 'TestHandleHealth|TestHandleReady|TestHandleVersion' -count=1`
- [ ] `go test ./...`

---

## 執行報告

### 摘要
| Part | 狀態 | 說明 |
|------|------|------|
| Part 1：Repository.Ping | ✅ 完成 | `repository.go` 加 `Ping(ctx) error` 包 `db.PingContext` |
| Part 2：/readyz + readiness 介面 | ✅ 完成 | `readinessChecker` 介面 + 編譯期斷言 + `handleReady`（200/503）+ route |
| Part 3：/version + config + New | ✅ 完成 | `Config.AppVersion`、`Server.version`、`New` 加 `version` 參數、`main.go` 同步、`handleVersion` + route |
| Part 4：server 白箱測試 | ✅ 完成 | `stubPinger` + 4 個 handler 測試（health/readyOK/readyUnhealthy/version），不連 DB |

### ❌ 未通過的驗收項目
無

### 🔍 需人工確認的驗收項目
無

### 🤔 執行中的判斷決策
無（spec 完整，未遇 ambiguity 或 spec-vs-code 衝突）

### ⚠️ 注意事項
- `server.New` 簽名新增 `version string` 參數（置於 `secret` 後）；唯一 caller `cmd/server/main.go` 已同步傳入 `cfg.AppVersion`。
- readiness 以 `readinessChecker` 介面注入，`*database.Repository` 滿足之（編譯期斷言 `var _ readinessChecker = (*database.Repository)(nil)`）；測試用 `stubPinger` 覆蓋成功/失敗兩路徑，不需 DB，符合 popofinder 測試不連 DB 慣例。
- `Config.AppVersion` 讀 `APP_VERSION`（caarlos0/env，`envDefault:"unknown"`），由 deploy chart 注入；可選欄位，不影響既有 required 檢查。
- 跨 repo 後續：`jerrytech-deploy/charts/app` 的 readiness/startup probe 改打 `/readyz` 為獨立變更，須在本 image 部署上線後才執行（見 `jerrytech-deploy/docs/health-probe-standardization.md`）。
