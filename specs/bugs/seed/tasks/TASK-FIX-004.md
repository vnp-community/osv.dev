# TASK-FIX-004: Fix Notification & Asset Services — Port + Nil Handler Issues

**Trạng thái:** ⚠️ CÒN VẤN ĐỀ — Phát hiện 2 bugs mới từ deploy config  
**Liên kết bug:** [BUG-SEED-001 §4 & §5](../001-seed-script-missing-routes.md)  
**Liên kết giải pháp:** [SOL-BUG-SEED-001 §Bug4b & §Bug5](../solutions/SOL-BUG-SEED-001.md)  
**Priority:** P1  
**Cập nhật:** 2026-06-19 — Phát hiện thêm bugs sau khi phân tích `deploy/dev/`

---

## Bối cảnh — Phân tích mới từ deploy/dev/

### Notification Service — 2 vấn đề phát hiện thêm

**Vấn đề 1: Port mismatch (tương tự SLA)**

| Layer | Port |
|-------|------|
| `wire.go` dòng 98 | `notifSvc := adapters.NewEmbeddedService("notification-service", **8087**)` |
| `.env` dòng 67 | `NOTIFICATION_HTTP_PORT=**8086**` |
| `docker-compose.server.yml` dòng 196 | `NOTIFICATION_HTTP_PORT: "**8086**"` |
| `notification-service/cmd/server/main.go` dòng 108 | `httpPort := envOr("NOTIFICATION_HTTP_PORT", envOr("HTTP_PORT", "**8086**"))` |
| `gateway/router.go` | proxy đến `notification-service:**8087**` |

Trong **embedded mode** (`wire.go`): service listen trên `:8087` → gateway forward đến `:8087` → **OK**.  
Nhưng `.env` và docker-compose định nghĩa `NOTIFICATION_HTTP_PORT=8086` — nếu service nào đó đọc env này thay vì port hardcode trong `wire.go`, sẽ conflict.

**Kết luận:** Trong embedded mode (monolith), port được hardcode bởi `NewEmbeddedService` trong `wire.go`, KHÔNG đọc env. Nên `8087` trong `wire.go` và gateway là nhất quán → **KHÔNG có port mismatch trong embedded mode**. Env `NOTIFICATION_HTTP_PORT=8086` chỉ dùng cho standalone mode.

**Vấn đề 2: AlertHandler và SSEHandler là `nil` trong embedded.go** ⚠️ BUG THỰC SỰ

`services/notification-service/embedded.go` dòng 45:
```go
r := deliverhttp.SetupRouter(whHandler, shHandler, ihHandler, nil, nil, rhHandler)
//                                                              ^^^  ^^^
//                                                              ah   sse   ← KHÔNG được khởi tạo!
```

`SetupRouter` nhận `ah *AlertsHandler` và `sse *SSEHandler`. Cả hai được truyền `nil`.  
Trong `router.go`, các routes `/api/v2/notifications/*` và SSE stream **không có nil-guard** → panic hoặc không mount:
```go
r.Route("/api/v2/notifications", func(r chi.Router) {
    r.Get("/", ah.ListNotifications)      // nil pointer dereference nếu ah == nil!
    r.Get("/stream", sse.Stream)
    ...
})
```

Hậu quả: Routes `/api/v2/notifications` **không được mount** (hoặc gây panic) → 404/500 khi seed gọi notification routes.

### Asset Service — Không có vấn đề mới

`wire.go` dòng 112: `assetSvc := adapters.NewEmbeddedService("asset-service", 8091)`  
Gateway: `asset-service:8091` ✅ Nhất quán  
`embedded.go`: đầy đủ ✅

---

## Fix cần thực hiện

### Fix A: Khởi tạo AlertHandler và SSEHandler trong notification embedded.go

**File cần sửa:** `services/notification-service/embedded.go`

```go
// Hiện tại (SAI — nil handlers)
r := deliverhttp.SetupRouter(whHandler, shHandler, ihHandler, nil, nil, rhHandler)

// Cần sửa — khởi tạo alertRepo + AlertHandler + SSEHandler
alertRepo := infrapostgres.NewAlertRepository(pool)         // hoặc tên đúng
alertUC := usecase.NewAlertUseCase(alertRepo)               // tùy cách implement
ahHandler := deliverhttp.NewAlertsHandler(alertUC)
sseHandler := deliverhttp.NewSSEHandler(alertUC, redisClient) // tùy cấu trúc
r := deliverhttp.SetupRouter(whHandler, shHandler, ihHandler, ahHandler, sseHandler, rhHandler)
```

> **Lưu ý:** Tên constructor có thể khác — cần kiểm tra `services/notification-service/internal/delivery/http/` để xác định đúng.

**Bước thực hiện:**

1. Xác định constructor của `AlertsHandler` và `SSEHandler`:
   ```bash
   grep -n "func New" services/notification-service/internal/delivery/http/alert_handler.go
   grep -n "func New" services/notification-service/internal/delivery/http/sse_handler.go
   ```

2. Xác định repository và usecase cần thiết:
   ```bash
   grep -n "NewAlert" services/notification-service/internal/infra/
   grep -rn "NewAlert" services/notification-service/internal/usecase/
   ```

3. Cập nhật `services/notification-service/embedded.go` để khởi tạo đầy đủ và truyền vào `SetupRouter`.

4. Build verify:
   ```bash
   cd services/notification-service && go build ./...
   ```

---

## Verify sau khi fix

### Bước 1: Build notification-service

```bash
cd /Users/binhnt/Lab/sec/cve/osv.dev/services/notification-service
export PATH=$PATH:/usr/local/go/bin
go build ./...
# Kỳ vọng: không lỗi
```

### Bước 2: Deploy (sau khi sửa embedded.go)

```bash
cd /Users/binhnt/Lab/sec/cve/osv.dev
bash deploy/dev/deploy_backend.sh
```

### Bước 3: Test trên server

```bash
ssh ubuntu@172.20.2.48

TOKEN=$(curl -s -X POST http://localhost:8080/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"username":"admin","password":"Admin@123!ChangeMe"}' | jq -r '.token')

# Test notification-rules (rhHandler — đã có)
curl -s -w "\n[HTTP %{http_code}]\n" \
  -X POST http://localhost:8080/api/v2/notification-rules \
  -H "Authorization: Bearer $TOKEN" -H "Content-Type: application/json" \
  -d '{"name":"Test Rule","event_type":"finding.created","channel":"email","enabled":true}'
# Kỳ vọng: 201

# Test subscriptions (shHandler — đã có)
curl -s -w "\n[HTTP %{http_code}]\n" \
  -X POST http://localhost:8080/api/v2/subscriptions \
  -H "Authorization: Bearer $TOKEN" -H "Content-Type: application/json" \
  -d '{"event_type":"finding.sla.breached","channels":["email"]}'
# Kỳ vọng: 201

# Test webhooks (whHandler — đã có)
curl -s -w "\n[HTTP %{http_code}]\n" \
  -X POST http://localhost:8080/api/v1/webhooks \
  -H "Authorization: Bearer $TOKEN" -H "Content-Type: application/json" \
  -d '{"name":"Test Webhook","url":"https://httpbin.org/post","secret":"mysecret"}'
# Kỳ vọng: 201

# Test notifications list (ahHandler — fix bắt buộc)
curl -s -w "\n[HTTP %{http_code}]\n" \
  http://localhost:8080/api/v2/notifications \
  -H "Authorization: Bearer $TOKEN"
# Kỳ vọng: 200 — KHÔNG phải 404/panic

# Test asset POST
curl -s -w "\n[HTTP %{http_code}]\n" \
  -X POST http://localhost:8080/api/v1/assets \
  -H "Authorization: Bearer $TOKEN" -H "Content-Type: application/json" \
  -d '{"name":"Test Asset","asset_type":"hostname","value":"test.example.com"}'
# Kỳ vọng: 201
```

---

## Định nghĩa hoàn thành

### Notification
- [ ] `embedded.go`: `ah *AlertsHandler` và `sse *SSEHandler` không phải `nil`
- [ ] `go build ./...` trong `notification-service` — PASS
- [ ] `POST /api/v2/notification-rules` → 201
- [ ] `POST /api/v2/subscriptions` → 201
- [ ] `POST /api/v1/webhooks` → 201
- [ ] `GET /api/v2/notifications` → 200 (không phải 404)

### Asset
- [ ] `go build ./...` trong `asset-service` — PASS (đã PASS trong lần kiểm tra trước)
- [ ] `POST /api/v1/assets` → 201
