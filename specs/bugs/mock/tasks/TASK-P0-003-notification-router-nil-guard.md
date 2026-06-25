# TASK-P0-003 — Nil-guard trong notification-service router + Wire AlertsHandler

**Bug:** MOCK-014  
**Priority:** 🔴 P0 — Production Crash Risk  
**Effort:** ~45 phút  
**Service:** `notification-service`  
**Loại thay đổi:** Code fix (router) + DB migration + Wire embedded.go

---

## Mục tiêu

`notification-service/router.go` đăng ký route `ah.ListNotifications` và `sse.Stream` trực tiếp mà không kiểm tra nil. Khi `ah == nil` hoặc `sse == nil` (như trong embedded mode hiện tại), mọi request tới `/api/v2/notifications` sẽ panic ngay lập tức.

Fix gồm **2 phần song song**:
1. **Ngắn hạn (P0)**: Guard nil trong router — trả 503 thay vì panic
2. **Đầy đủ**: Wire AlertsHandler + SSEHandler thực sự trong embedded.go

---

## Preconditions

- [ ] Đọc `services/notification-service/internal/delivery/http/router.go`
- [ ] Đọc `services/notification-service/embedded.go`
- [ ] Xác định: tên struct `AlertsHandler`, `SSEHandler` (có thể khác tên)
- [ ] Xác định: helper `respondJSON`/`writeJSON`/`respond` đang dùng trong router

---

## Steps — Phần 1: Fix router.go (P0 — bắt buộc)

### Step 1 — Đọc router.go hiện tại

```
File: services/notification-service/internal/delivery/http/router.go
```

Tìm tất cả chỗ `ah.` và `sse.` được dùng trong route registration.

### Step 2 — Wrap notification routes trong nil-check

Tìm block route đăng ký alerts/notifications routes và bọc bằng `if ah != nil`:

```go
// FIX MOCK-014: guard nil AlertsHandler
if ah != nil {
    r.Route("/api/v2/notifications", func(r chi.Router) {
        r.Get("/", ah.ListNotifications)
        r.Patch("/{id}/read", ah.MarkRead)
        r.Post("/mark-all-read", ah.MarkAllRead)
        r.Get("/unread-count", ah.UnreadCount)
        if sse != nil {
            r.Get("/stream", sse.Stream)
        }
    })
    // v1 compat routes (nếu có)
    r.Get("/api/v1/notifications", ah.ListNotifications)
    r.Patch("/api/v1/notifications/{id}/read", ah.MarkRead)
} else {
    // Graceful stub — trả 503 thay vì 404
    unavailable := func(w http.ResponseWriter, r *http.Request) {
        w.Header().Set("Content-Type", "application/json")
        w.WriteHeader(http.StatusServiceUnavailable)
        w.Write([]byte(`{"error":"notification service not fully initialized"}`))
    }
    r.Get("/api/v2/notifications", unavailable)
    r.Get("/api/v2/notifications/stream", unavailable)
    r.Get("/api/v1/notifications", unavailable)
}
```

> **Quan trọng**: Giữ đúng cấu trúc route đang có, chỉ thêm nil-check. Không xóa/thêm routes.

---

## Steps — Phần 2: Wire AlertsHandler + SSEHandler trong embedded.go

### Step 3 — Đọc embedded.go hiện tại

```
File: services/notification-service/embedded.go
```

Xác định:
- Các repos đã được khởi tạo (webhook, subscription, rule...)
- Dòng gọi `SetupRouter(...)` với `nil, nil` cho ah và sse

### Step 4 — Tạo DB migration cho bảng `alerts`

**File mới**: `services/notification-service/internal/infra/postgres/migrations/004_add_alerts.sql`

```sql
CREATE TABLE IF NOT EXISTS alerts (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id     VARCHAR(36) NOT NULL,
    type        VARCHAR(50) NOT NULL,
    title       TEXT NOT NULL,
    message     TEXT,
    severity    VARCHAR(20) DEFAULT 'Info',
    is_read     BOOLEAN DEFAULT FALSE,
    entity_type VARCHAR(50),
    entity_id   VARCHAR(36),
    created_at  TIMESTAMPTZ DEFAULT NOW(),
    read_at     TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_alerts_user_id ON alerts(user_id);
CREATE INDEX IF NOT EXISTS idx_alerts_user_unread ON alerts(user_id, is_read) WHERE is_read = FALSE;
```

> **Lưu ý**: Đặt file migration theo đúng convention của project (kiểm tra thư mục migrations hiện có).

### Step 5 — Tạo AlertRepository (nếu chưa có)

Kiểm tra xem `AlertRepository` đã tồn tại chưa:
```bash
find services/notification-service -name "*alert*repo*" -o -name "*alert*repository*"
```

Nếu chưa có, tạo file mới:

**File mới**: `services/notification-service/internal/infra/postgres/alert_repo.go`

```go
package postgres

import (
    "context"
    "github.com/jackc/pgx/v5/pgxpool"
)

type AlertRepo struct {
    db *pgxpool.Pool
}

func NewAlertRepo(db *pgxpool.Pool) *AlertRepo {
    return &AlertRepo{db: db}
}

func (r *AlertRepo) Create(ctx context.Context, userID, alertType, title, message, severity, entityType, entityID string) error {
    _, err := r.db.Exec(ctx, `
        INSERT INTO alerts (user_id, type, title, message, severity, entity_type, entity_id)
        VALUES ($1, $2, $3, $4, $5, $6, $7)
    `, userID, alertType, title, message, severity, entityType, entityID)
    return err
}

func (r *AlertRepo) ListByUser(ctx context.Context, userID string, limit, offset int) ([]Alert, int, error) {
    var total int
    r.db.QueryRow(ctx, `SELECT COUNT(*) FROM alerts WHERE user_id = $1`, userID).Scan(&total)

    rows, err := r.db.Query(ctx, `
        SELECT id, user_id, type, title, message, severity, is_read,
               entity_type, entity_id, created_at, read_at
        FROM alerts WHERE user_id = $1
        ORDER BY created_at DESC LIMIT $2 OFFSET $3
    `, userID, limit, offset)
    if err != nil {
        return nil, 0, err
    }
    defer rows.Close()
    // scan rows into []Alert...
    return alerts, total, nil
}

func (r *AlertRepo) MarkRead(ctx context.Context, id, userID string) error {
    _, err := r.db.Exec(ctx, `
        UPDATE alerts SET is_read = TRUE, read_at = NOW()
        WHERE id = $1 AND user_id = $2
    `, id, userID)
    return err
}

func (r *AlertRepo) MarkAllRead(ctx context.Context, userID string) error {
    _, err := r.db.Exec(ctx, `
        UPDATE alerts SET is_read = TRUE, read_at = NOW()
        WHERE user_id = $1 AND is_read = FALSE
    `, userID)
    return err
}

func (r *AlertRepo) CountUnread(ctx context.Context, userID string) (int, error) {
    var count int
    err := r.db.QueryRow(ctx, `
        SELECT COUNT(*) FROM alerts WHERE user_id = $1 AND is_read = FALSE
    `, userID).Scan(&count)
    return count, err
}
```

### Step 6 — Wire AlertsHandler trong embedded.go

Kiểm tra xem `AlertsHandler` và `NewAlertsHandler` đã tồn tại chưa trong delivery layer:
```bash
find services/notification-service -name "*alert*handler*"
grep -r "AlertsHandler\|NewAlertsHandler" services/notification-service/
```

Trong `embedded.go`, sửa dòng gọi `SetupRouter(...)`:

```go
// FIX MOCK-014: Wire AlertsHandler thực sự
alertRepo := postgres.NewAlertRepo(pool)
ahHandler := httpdelivery.NewAlertsHandler(alertRepo)

// SSEHandler: wire event broker
// (kiểm tra xem SSEHandler + EventBroker đã có trong codebase chưa)
var sseHandler *httpdelivery.SSEHandler
// Nếu đã có SSEHandler implementation:
// jwtSecret := os.Getenv("JWT_SECRET")
// sseBroker := broker.NewEventBroker()
// go sseBroker.Run(ctx)
// sseHandler = httpdelivery.NewSSEHandler(sseBroker, jwt.NewValidator(jwtSecret))

r := httpdelivery.SetupRouter(whHandler, shHandler, ihHandler, ahHandler, sseHandler, rhHandler, dhHandler)
```

---

## Acceptance Criteria

### Phần 1 (P0 — bắt buộc):
- [ ] `GET /api/v2/notifications` khi `ah == nil` → trả `503 JSON`, không panic
- [ ] `GET /api/v2/notifications/stream` khi `sse == nil` → trả `503 JSON`, không panic
- [ ] Các routes khác (webhooks, subscriptions) không bị ảnh hưởng

### Phần 2 (Wire):
- [ ] `alerts` table tồn tại trong DB
- [ ] `GET /api/v2/notifications` trả danh sách alerts thực từ DB
- [ ] `PATCH /api/v2/notifications/{id}/read` cập nhật `is_read = TRUE` trong DB

---

## Test Commands

```bash
# Build check
cd /Users/binhnt/Lab/sec/cve/osv.dev
go build ./services/notification-service/...

# Verify nil-check added in router
grep -n "ah != nil\|sse != nil\|ServiceUnavailable" \
  services/notification-service/internal/delivery/http/router.go

# Run tests
go test ./services/notification-service/... -v
```
