# SOL-003: Notification-Service — List, Unread Count, Mark All Read

> **Bugs giải quyết**: BUG-002  
> **Service**: `services/notification-service`  
> **Port**: 8087  
> **Architecture ref**: §3.8 Notification-Service  
> **Status**: `[x] DONE`

## Kết Quả Thực Thi

**Đã hoàn thành trong notification-service:**

| Fix | File | Trạng thái |
|---|---|---|
| `GET /api/v1/notifications` (list alerts) | `internal/delivery/http/router.go` | ✅ Thêm mới (TASK-003) |
| `GET /api/v1/notifications/unread-count` | `internal/delivery/http/router.go` | ✅ Thêm mới (TASK-003) |
| `POST /api/v1/notifications/mark-all-read` | `internal/delivery/http/router.go` | ✅ Thêm mới (TASK-003) |
| `AlertsHandler` không nil trong production wiring | `embedded.go` | ✅ Wired |
| `GET /api/v2/notifications` v2 alias | `internal/delivery/http/router.go` | ✅ Đã có |

**Build verify**: `go build ./...` ✅ notification-service


## Phân Tích

Architecture §3.8 mô tả notification-service với:
- In-app channel: SSE stream + **Store in alerts table** (polled by frontend)
- Schema `osv_notif`: `webhooks`, `notification_rules`, `alerts`

Table `alerts` đã được thiết kế là nơi lưu trữ in-app notifications. Endpoints thiếu là read API trên table này.

Hiện tại:
- ✅ `GET /api/v1/notifications/stream` — SSE stream hoạt động  
- ✅ `PATCH /api/v1/notifications/{id}/read` — hoạt động  
- ❌ `GET /api/v1/notifications` — chưa có  
- ❌ `GET /api/v1/notifications/unread-count` — chưa có  
- ❌ `POST /api/v1/notifications/mark-all-read` — chưa có  

## Schema Alerts Table

```sql
-- osv_notif schema (có thể đã tồn tại, verify và bổ sung nếu thiếu)
CREATE TABLE IF NOT EXISTS alerts (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id      UUID NOT NULL,
    title        VARCHAR(500) NOT NULL,
    message      TEXT,
    type         VARCHAR(50) NOT NULL,  -- "critical_finding", "kev_new", "sla_breach", ...
    severity     VARCHAR(20),           -- "critical", "high", "medium", "info"
    resource_type VARCHAR(50),          -- "finding", "scan", "cve"
    resource_id   UUID,
    read         BOOLEAN NOT NULL DEFAULT FALSE,
    read_at      TIMESTAMPTZ,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_alerts_user_unread
    ON alerts(user_id, read, created_at DESC);
```

## HTTP Handlers

```go
// services/notification-service/internal/delivery/http/notification_handler.go

type NotificationHandler struct {
    alertRepo AlertRepository
}

// GET /api/v1/notifications?page=1&limit=20&read=false&type=critical_finding
func (h *NotificationHandler) List(w http.ResponseWriter, r *http.Request) {
    userID := r.Header.Get("X-User-ID")
    
    filter := AlertFilter{
        UserID: userID,
        Page:   parseIntParam(r, "page", 1),
        Limit:  parseIntParam(r, "limit", 20),
    }
    
    // Optional filters
    if readStr := r.URL.Query().Get("read"); readStr != "" {
        read := readStr == "true"
        filter.Read = &read
    }
    if t := r.URL.Query().Get("type"); t != "" {
        filter.Type = t
    }
    
    alerts, total, err := h.alertRepo.List(r.Context(), filter)
    if err != nil {
        respondError(w, http.StatusInternalServerError, err)
        return
    }
    
    unreadCount, _ := h.alertRepo.CountUnread(r.Context(), userID)
    
    respondJSON(w, http.StatusOK, map[string]interface{}{
        "items":        alerts,
        "total":        total,
        "unread_count": unreadCount,
        "page":         filter.Page,
        "limit":        filter.Limit,
    })
}

// GET /api/v1/notifications/unread-count
func (h *NotificationHandler) UnreadCount(w http.ResponseWriter, r *http.Request) {
    userID := r.Header.Get("X-User-ID")
    
    count, err := h.alertRepo.CountUnread(r.Context(), userID)
    if err != nil {
        respondError(w, http.StatusInternalServerError, err)
        return
    }
    
    respondJSON(w, http.StatusOK, map[string]int{"count": count})
}

// POST /api/v1/notifications/mark-all-read
func (h *NotificationHandler) MarkAllRead(w http.ResponseWriter, r *http.Request) {
    userID := r.Header.Get("X-User-ID")
    
    marked, err := h.alertRepo.MarkAllRead(r.Context(), userID)
    if err != nil {
        respondError(w, http.StatusInternalServerError, err)
        return
    }
    
    respondJSON(w, http.StatusOK, map[string]int{"marked": marked})
}

// PATCH /api/v1/notifications/{id}/read (đã có, đảm bảo consistent)
func (h *NotificationHandler) MarkRead(w http.ResponseWriter, r *http.Request) {
    userID := r.Header.Get("X-User-ID")
    id     := r.PathValue("id")
    
    if err := h.alertRepo.MarkRead(r.Context(), userID, id); err != nil {
        respondError(w, http.StatusNotFound, err)
        return
    }
    
    w.WriteHeader(http.StatusNoContent)
}
```

## Repository Interface

```go
// services/notification-service/internal/domain/alert_repo.go

type AlertFilter struct {
    UserID string
    Read   *bool
    Type   string
    Page   int
    Limit  int
}

type AlertRepository interface {
    List(ctx context.Context, filter AlertFilter) ([]Alert, int, error)
    CountUnread(ctx context.Context, userID string) (int, error)
    MarkRead(ctx context.Context, userID, alertID string) error
    MarkAllRead(ctx context.Context, userID string) (int, error)
    Store(ctx context.Context, userID string, evt NotificationEvent) error
}
```

## SQL Queries

```go
// services/notification-service/internal/infra/postgres/alert_repo.go

func (r *PostgresAlertRepo) List(ctx context.Context, f AlertFilter) ([]Alert, int, error) {
    where := []string{"user_id = $1"}
    args  := []interface{}{f.UserID}
    idx   := 2
    
    if f.Read != nil {
        where = append(where, fmt.Sprintf("read = $%d", idx))
        args = append(args, *f.Read)
        idx++
    }
    if f.Type != "" {
        where = append(where, fmt.Sprintf("type = $%d", idx))
        args = append(args, f.Type)
        idx++
    }
    
    offset := (f.Page - 1) * f.Limit
    
    query := fmt.Sprintf(`
        SELECT id, title, message, type, severity, resource_type, resource_id,
               read, read_at, created_at,
               COUNT(*) OVER() AS total
        FROM alerts
        WHERE %s
        ORDER BY created_at DESC
        LIMIT $%d OFFSET $%d
    `, strings.Join(where, " AND "), idx, idx+1)
    
    args = append(args, f.Limit, offset)
    
    // ... scan rows
}

func (r *PostgresAlertRepo) CountUnread(ctx context.Context, userID string) (int, error) {
    var count int
    err := r.db.QueryRow(ctx,
        "SELECT COUNT(*) FROM alerts WHERE user_id = $1 AND read = false",
        userID).Scan(&count)
    return count, err
}

func (r *PostgresAlertRepo) MarkAllRead(ctx context.Context, userID string) (int, error) {
    result, err := r.db.Exec(ctx, `
        UPDATE alerts
        SET read = true, read_at = NOW()
        WHERE user_id = $1 AND read = false
    `, userID)
    return int(result.RowsAffected()), err
}
```

## Router Registration

```go
// services/notification-service/internal/delivery/http/router.go

notifHandler := NewNotificationHandler(alertRepo)

// THÊM các routes còn thiếu:
r.GET("/api/v1/notifications",           authMiddleware(notifHandler.List))
r.GET("/api/v1/notifications/unread-count", authMiddleware(notifHandler.UnreadCount))
r.POST("/api/v1/notifications/mark-all-read", authMiddleware(notifHandler.MarkAllRead))

// Đã có (verify):
r.PATCH("/api/v1/notifications/{id}/read", authMiddleware(notifHandler.MarkRead))
r.GET("/api/v1/notifications/stream",    sseAuth(notifHandler.Stream))
```

## Webhook Stats (BUG-012)

Cùng file `notification_handler.go` — thêm webhook stats handler:

```go
// GET /api/v1/webhooks/stats
func (h *NotificationHandler) WebhookStats(w http.ResponseWriter, r *http.Request) {
    // Tổng hợp thống kê delivery: thành công, thất bại, avg response time
    stats, err := h.deliveryRepo.GetStats(r.Context())
    if err != nil {
        respondError(w, http.StatusInternalServerError, err)
        return
    }
    respondJSON(w, http.StatusOK, stats)
}

// GET /api/v1/webhooks/stats/hourly
func (h *NotificationHandler) WebhookStatsHourly(w http.ResponseWriter, r *http.Request) {
    stats, err := h.deliveryRepo.GetHourlyStats(r.Context(), 24) // Last 24h
    if err != nil {
        respondError(w, http.StatusInternalServerError, err)
        return
    }
    respondJSON(w, http.StatusOK, stats)
}
```

```go
r.GET("/api/v1/webhooks/stats",        authMiddleware(notifHandler.WebhookStats))
r.GET("/api/v1/webhooks/stats/hourly", authMiddleware(notifHandler.WebhookStatsHourly))
```

## Response Schema

```go
type AlertResponse struct {
    ID           string  `json:"id"`
    Title        string  `json:"title"`
    Message      string  `json:"message,omitempty"`
    Type         string  `json:"type"`
    Severity     string  `json:"severity,omitempty"`
    ResourceType string  `json:"resource_type,omitempty"`
    ResourceID   string  `json:"resource_id,omitempty"`
    Read         bool    `json:"read"`
    ReadAt       *string `json:"read_at,omitempty"`
    CreatedAt    string  `json:"created_at"`
}
```
