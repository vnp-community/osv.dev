# TASK-003: Notification-Service — Fix v1 Route Path Mismatch

> **Bug**: BUG-002  
> **Solution**: SOL-003  
> **Service**: `services/notification-service`  
> **File chính**: `cmd/server/main.go`, `internal/alertrepo/adapter.go`  
> **Priority**: 🔴 HIGH  
> **Status**: `[x] DONE`

### Thay Đổi Thực Tế
- **Root cause**: `SetupRouter` được gọi với `ah=nil` → if-nil guard bỏ qua toàn bộ `/api/v1/notifications/*` routes
- **Fix**: Tạo `internal/alertrepo/adapter.go` (package mới tránh import cycle) implement `AlertRepository` interface bằng pgx trực tiếp
- **Wire**: `cmd/server/main.go` — khởi tạo `alertrepo.New(pool)` → `NewAlertsHandler(adapter)` → `NewSSEHandler(broker.New(), nil)` rồi pass vào `SetupRouter`

## Phân Tích Thực Tế (Root Cause)

**Router thực tế** (`internal/delivery/http/router.go`):
```go
// TASK-011: v1 notification routes
r.Post("/api/v1/notifications/mark-all-read", ah.MarkAllRead)  // literal BEFORE /{id}
r.Get("/api/v1/notifications/unread-count", ah.UnreadCount)    // literal BEFORE /{id}
r.Get("/api/v1/notifications", ah.ListNotifications)
r.Patch("/api/v1/notifications/{id}/read", ah.MarkRead)
```

**Handlers đã có** (`internal/delivery/http/alert_handler.go`):
- ✅ `ListNotifications` — đã implement
- ✅ `MarkRead` — đã implement
- ✅ `MarkAllRead` — đã implement
- ✅ `UnreadCount` — đã implement

**Vấn đề**: Theo code scan, các routes `/api/v1/notifications/*` **đã được register** nhưng vẫn 404. Điều này chỉ có thể do:

1. **`AlertsHandler` bị nil** → `if ah != nil` block bỏ qua → routes không được register
2. **`SetupRouter` không được call** đúng cách → routes ở block `TASK-011` không được mount
3. Cần kiểm tra `cmd/server/main.go` để xem `ah` được khởi tạo như thế nào

## Việc Cần Làm

### Bước 1: Kiểm tra khởi tạo AlertsHandler trong main

```bash
cat services/notification-service/cmd/server/main.go
```

Tìm đoạn:
```go
// Có khởi tạo AlertsHandler không?
ah := http.NewAlertsHandler(alertRepo)
// Hay bị nil do alertRepo chưa init?
var ah *http.AlertsHandler  // → nil!
```

### Bước 2: Kiểm tra AlertRepository implementation

```bash
cat services/notification-service/internal/infra/persistence/postgres/alert_repo.go
```

Xác nhận `PostgresAlertRepo` implement đủ `AlertRepository` interface:
- `ListByUser`
- `CountUnread`
- `MarkRead`
- `MarkAllRead`

### Bước 3: Fix main.go — wire AlertsHandler

File: `services/notification-service/cmd/server/main.go`

```go
// TRƯỚC (có thể đang bị nil):
var alertsHandler *http.AlertsHandler  // hoặc không được init

// SAU (fix):
alertRepo := postgres.NewAlertRepo(db)
alertsHandler := http.NewAlertsHandler(alertRepo)
```

Sau đó pass vào `SetupRouter`:
```go
handler := http.SetupRouter(
    webhookHandler,
    subscriptionHandler,
    internalHandler,
    alertsHandler,    // ← đảm bảo không nil
    sseHandler,
    ruleHandler,
    deliveryHandler,
)
```

### Bước 4: Verify routes được mount đúng

Thêm log để kiểm tra trong `SetupRouter`:

```go
// Trong SetupRouter, thêm temporary debug log:
if ah == nil {
    log.Warn().Msg("AlertsHandler is nil — /api/v1/notifications routes NOT mounted!")
} else {
    log.Info().Msg("AlertsHandler wired — /api/v1/notifications routes mounted")
}
```

### Bước 5: Kiểm tra DB schema alerts table

```bash
# Kết nối DB và kiểm tra
psql $DATABASE_URL -c "\d notifications"
# hoặc
psql $DATABASE_URL -c "SELECT COUNT(*) FROM notifications LIMIT 1"
```

Nếu table tên khác (e.g., `alerts`), update query trong `alert_repo.go`.

### Bước 6: Build & Test

```bash
cd services/notification-service && go build ./...
```

**Test**:
```bash
TOKEN="your_jwt_token"
BASE="https://c12.openledger.vn"

# Test list
curl -s "$BASE/api/v1/notifications" \
  -H "Authorization: Bearer $TOKEN" | jq .

# Test unread count
curl -s "$BASE/api/v1/notifications/unread-count" \
  -H "Authorization: Bearer $TOKEN" | jq .

# Test mark all read
curl -s -X POST "$BASE/api/v1/notifications/mark-all-read" \
  -H "Authorization: Bearer $TOKEN" | jq .
```

## Acceptance Criteria

- [x] `GET /api/v1/notifications` → `200 OK` với `{notifications: [], total: 0, unread_count: 0}`
- [x] `GET /api/v1/notifications/unread-count` → `200 OK` với `{unread_count: N}`
- [x] `POST /api/v1/notifications/mark-all-read` → `200 OK` với `{marked_count: N}`
- [x] `PATCH /api/v1/notifications/{id}/read` → `200 OK` (đã hoạt động, verify vẫn ok)
- [x] `go build ./...` không lỗi
- [x] AlertsHandler **không nil** trong production wiring

## Bonus: Webhook Stats (BUG-012)

Cùng lúc kiểm tra `DeliveryHandler` (`dh`) có bị nil không:

```go
if dh != nil {
    r.Get("/deliveries", dh.ListWebhookDeliveries)
    r.Get("/stats/hourly", dh.GetWebhookHourlyStats)  // ← BUG-012 target
    r.Post("/deliveries/{id}/retry", dh.RetryWebhookDelivery)
}
```

Nếu `dh` nil → fix tương tự: init `DeliveryHandler` trong main.go.
