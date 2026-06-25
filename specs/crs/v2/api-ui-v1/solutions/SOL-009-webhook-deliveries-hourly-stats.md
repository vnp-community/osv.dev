# SOL-009: Webhook Deliveries & Hourly Stats

> **CR:** [CR-009](../CR-009-webhook-deliveries-hourly-stats.md)  
> **Priority:** 🔴 HIGH (Phase 2)  
> **Service(s):** `notification-service` (`:8087`)  
> **Tạo:** 2026-06-19  
> **Cập nhật:** 2026-06-22  
> **Trạng thái:** ✅ **IMPLEMENTED** — 2026-06-22

---

## ✅ Implementation Status

| Endpoint | Trạng thái | File |
|---|---|---|
| `GET /api/v1/webhooks/deliveries` | ✅ Done | `gateway: handler_ui_api.go:WebhookDeliveryList` → `notification-service:8087/api/v1/webhooks/deliveries` |
| `POST /api/v1/webhooks/deliveries/{id}/retry` | ✅ Done | `gateway: handler_ui_api.go:WebhookDeliveryRetry` → `notification-service:8087` |
| `GET /api/v1/webhooks/stats/hourly` | ✅ Done | `gateway: handler_ui_api.go:WebhookHourlyStats` → `notification-service:8087/api/v1/webhooks/stats/hourly` |
| Notification-service `DeliveryHandler` | ✅ Done | `delivery_handler.go`: `ListWebhookDeliveries`, `RetryWebhookDelivery`, `GetWebhookHourlyStats` |
| Notification-service router registration | ✅ Done | `router.go:42-44` trong `/api/v1/webhooks` group |
| Gateway-service `RegisterUIAPIRoutes` | ✅ Done | `handler_ui_api.go:167-172` — literal paths trước `DELETE /{id}` wildcard |
| Filter `?webhook_id=`, `?status=` | ✅ Done | `delivery_handler.go:56-79` |
| Graceful degradation (table not exists) | ✅ Done | `delivery_handler.go:86-88` — return `{"deliveries":[],"total":0}` |
| 24h hourly buckets SQL | ✅ Done | `delivery_handler.go:222-231` — `date_trunc('hour', ...)` |

### Root Cause Analysis (405 bug)

**Vấn đề**: `GET /api/v1/webhooks/deliveries` trả **405 Method Not Allowed** thay vì 200.

**Chi router behavior**: 
- `gateway-service/RegisterUIAPIRoutes` có `r.Delete("/api/v1/webhooks/{id}", ...)` (wildcard)
- Thiếu `r.Get("/api/v1/webhooks/deliveries", ...)` (literal path)
- Chi 5: path `/api/v1/webhooks/deliveries` match wildcard `/{id}` với `id="deliveries"` → `DELETE` registered nhưng `GET` không có → **405**
- Chi KHÔNG fallback sang `r.NotFound` khi path match nhưng method không match

**Fix**: Thêm 3 literal routes vào `RegisterUIAPIRoutes` **trước** `DELETE /{id}`:
```go
r.Get("/api/v1/webhooks/deliveries", h.WebhookDeliveryList)
r.Post("/api/v1/webhooks/deliveries/{id}/retry", h.WebhookDeliveryRetry)
r.Get("/api/v1/webhooks/stats/hourly", h.WebhookHourlyStats)
```

---

## 1. Tóm tắt Giải pháp

> **Lưu ý về service:** CR-009 ghi "webhook-service:8089" nhưng theo architecture spec, không có service riêng `webhook-service`. Webhook management nằm trong `notification-service:8087`. Cần xác nhận với team — solution này implement trong `notification-service`.

| Thay đổi | Mô tả |
|---|---|
| **DB migration** | Tạo bảng `webhook_deliveries` |
| **Dispatcher logging** | Ghi delivery log sau mỗi lần gửi webhook |
| **3 endpoints mới** | GET deliveries, POST retry, GET hourly stats |
| **Gateway routing** | 3 routes mới trỏ vào notification-service |

---

## 2. DB Migration

### File: `services/notification-service/migrations/20260619_001_webhook_deliveries.sql`

```sql
-- Tạo bảng ghi log mỗi lần gửi webhook
CREATE TABLE IF NOT EXISTS webhook_deliveries (
    id               VARCHAR(36)  PRIMARY KEY DEFAULT gen_random_uuid()::text,
    webhook_id       VARCHAR(36)  NOT NULL REFERENCES webhooks(id) ON DELETE CASCADE,
    event            VARCHAR(100) NOT NULL,
    endpoint         VARCHAR(255) NOT NULL,      -- extracted hostname từ URL
    status           VARCHAR(20)  NOT NULL
        CHECK (status IN ('success', 'failed', 'retried')),
    response_time_ms INTEGER      NOT NULL,
    status_code      INTEGER      NOT NULL DEFAULT 0,
    request_body     TEXT,
    response_body    TEXT,
    time             TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

-- Indexes cho queries phổ biến
CREATE INDEX IF NOT EXISTS idx_webhook_deliveries_webhook_id
    ON webhook_deliveries(webhook_id);
CREATE INDEX IF NOT EXISTS idx_webhook_deliveries_time
    ON webhook_deliveries(time DESC);
CREATE INDEX IF NOT EXISTS idx_webhook_deliveries_status
    ON webhook_deliveries(status);

-- Partition theo tháng nếu volume lớn (optional, có thể thêm sau)
-- CREATE TABLE webhook_deliveries_2026_06 PARTITION OF webhook_deliveries
--     FOR VALUES FROM ('2026-06-01') TO ('2026-07-01');
```

---

## 3. Domain Model

### File: `services/notification-service/internal/domain/delivery.go`

```go
package domain

import "time"

// WebhookDelivery — 1 lần gửi webhook
type WebhookDelivery struct {
    ID             string    `json:"id"`
    WebhookID      string    `json:"webhook_id"`
    Event          string    `json:"event"`
    Endpoint       string    `json:"endpoint"`
    Status         string    `json:"status"`           // "success" | "failed" | "retried"
    ResponseTimeMs int       `json:"response_time_ms"`
    StatusCode     int       `json:"status_code"`
    RequestBody    string    `json:"request_body,omitempty"`
    ResponseBody   string    `json:"response_body,omitempty"`
    Time           time.Time `json:"time"`
}

// WebhookDeliveriesResponse — paginated list
type WebhookDeliveriesResponse struct {
    Deliveries []WebhookDelivery `json:"deliveries"`
    Total      int               `json:"total"`
}

// WebhookHourlyStats — 1 item trong hourly chart
type WebhookHourlyStats struct {
    H       string `json:"h"`       // "00:00", "01:00", ..., "23:00"
    Success int    `json:"success"`
    Failed  int    `json:"failed"`
}

// DeliveryFilter — query params cho list endpoint
type DeliveryFilter struct {
    WebhookID string
    Status    string // "success" | "failed" | "retried"
    Page      int
    PageSize  int
}
```

---

## 4. Dispatcher — Thêm Delivery Logging

### File: `services/notification-service/internal/delivery/webhook/dispatcher.go`

```go
package webhook

import (
    "bytes"
    "context"
    "crypto/hmac"
    "crypto/sha256"
    "encoding/hex"
    "encoding/json"
    "io"
    "net"
    "net/http"
    "net/url"
    "time"

    "github.com/google/uuid"
    "your-project/notification-service/internal/domain"
)

type Dispatcher struct {
    client      *http.Client
    deliveryRepo DeliveryRepository
}

// Dispatch — gửi webhook và log delivery
func (d *Dispatcher) Dispatch(ctx context.Context, wh domain.Webhook, payload []byte) {
    start := time.Now()

    // SSRF protection
    if isPrivateIP(wh.URL) {
        d.logDelivery(ctx, domain.WebhookDelivery{
            ID:             uuid.New().String(),
            WebhookID:      wh.ID,
            Event:          wh.Event,
            Endpoint:       extractHostname(wh.URL),
            Status:         "failed",
            StatusCode:     0,
            ResponseTimeMs: 0,
            RequestBody:    string(payload),
            ResponseBody:   "SSRF_BLOCKED",
            Time:           start,
        })
        return
    }

    // HMAC signature
    mac := hmac.New(sha256.New, []byte(wh.Secret))
    mac.Write(payload)
    signature := "sha256=" + hex.EncodeToString(mac.Sum(nil))

    delivery := domain.WebhookDelivery{
        ID:          uuid.New().String(),
        WebhookID:   wh.ID,
        Event:       wh.Event,
        Endpoint:    extractHostname(wh.URL),
        RequestBody: string(payload),
        Time:        start,
    }

    // Retry loop: 3 attempts, backoff 1s, 2s, 4s
    backoff := time.Second
    var lastErr error
    var lastStatusCode int
    var lastResponseBody string

    for attempt := 1; attempt <= 3; attempt++ {
        req, _ := http.NewRequestWithContext(ctx, "POST", wh.URL, bytes.NewReader(payload))
        req.Header.Set("Content-Type", "application/json")
        req.Header.Set("X-OSV-Signature", signature)
        req.Header.Set("X-OSV-Event", wh.Event)
        req.Header.Set("X-OSV-Delivery", delivery.ID)

        resp, err := d.client.Do(req)
        elapsed := int(time.Since(start).Milliseconds())

        if err != nil {
            lastErr = err
            lastStatusCode = 0
            if attempt < 3 {
                time.Sleep(backoff)
                backoff *= 2
            }
            continue
        }

        body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096)) // max 4KB response body
        resp.Body.Close()
        lastStatusCode = resp.StatusCode
        lastResponseBody = string(body)

        if resp.StatusCode < 400 {
            // Success
            delivery.Status         = "success"
            delivery.StatusCode     = resp.StatusCode
            delivery.ResponseTimeMs = elapsed
            delivery.ResponseBody   = lastResponseBody
            d.logDelivery(ctx, delivery)
            return
        }

        if attempt < 3 {
            time.Sleep(backoff)
            backoff *= 2
        }
        lastErr = fmt.Errorf("status %d", resp.StatusCode)
    }

    // All attempts failed
    _ = lastErr
    delivery.Status         = "failed"
    delivery.StatusCode     = lastStatusCode
    delivery.ResponseTimeMs = int(time.Since(start).Milliseconds())
    delivery.ResponseBody   = lastResponseBody
    d.logDelivery(ctx, delivery)
}

// logDelivery — async save, non-blocking
func (d *Dispatcher) logDelivery(ctx context.Context, del domain.WebhookDelivery) {
    go func() {
        if err := d.deliveryRepo.Save(context.Background(), del); err != nil {
            log.Warn().Err(err).Str("delivery_id", del.ID).Msg("failed to log webhook delivery")
        }
    }()
}

// Retry — gửi lại 1 delivery cụ thể
func (d *Dispatcher) Retry(ctx context.Context, original domain.WebhookDelivery) (*domain.WebhookDelivery, error) {
    wh, err := d.webhookRepo.GetByID(ctx, original.WebhookID)
    if err != nil {
        return nil, fmt.Errorf("webhook not found: %w", err)
    }

    newDelivery := domain.WebhookDelivery{
        ID:          uuid.New().String(),
        WebhookID:   original.WebhookID,
        Event:       original.Event,
        Endpoint:    original.Endpoint,
        RequestBody: original.RequestBody, // Same payload
        Status:      "retried",
        Time:        time.Now(),
    }

    // Gửi lại
    payload := []byte(original.RequestBody)
    mac := hmac.New(sha256.New, []byte(wh.Secret))
    mac.Write(payload)
    signature := "sha256=" + hex.EncodeToString(mac.Sum(nil))

    req, _ := http.NewRequestWithContext(ctx, "POST", wh.URL, bytes.NewReader(payload))
    req.Header.Set("Content-Type", "application/json")
    req.Header.Set("X-OSV-Signature", signature)
    req.Header.Set("X-OSV-Event", original.Event)
    req.Header.Set("X-OSV-Delivery", newDelivery.ID)

    start := time.Now()
    resp, err := d.client.Do(req)
    newDelivery.ResponseTimeMs = int(time.Since(start).Milliseconds())

    if err != nil {
        newDelivery.Status = "failed"
        newDelivery.StatusCode = 0
    } else {
        body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
        resp.Body.Close()
        newDelivery.StatusCode = resp.StatusCode
        newDelivery.ResponseBody = string(body)
        if resp.StatusCode < 400 {
            newDelivery.Status = "success"
        } else {
            newDelivery.Status = "failed"
        }
    }

    d.deliveryRepo.Save(context.Background(), newDelivery)
    return &newDelivery, nil
}

// Helper functions
func extractHostname(rawURL string) string {
    u, err := url.Parse(rawURL)
    if err != nil {
        return rawURL
    }
    return u.Hostname()
}

func isPrivateIP(rawURL string) bool {
    u, err := url.Parse(rawURL)
    if err != nil {
        return true
    }
    addrs, _ := net.LookupHost(u.Hostname())
    for _, addr := range addrs {
        ip := net.ParseIP(addr)
        if ip == nil {
            continue
        }
        if ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() {
            return true
        }
    }
    return false
}
```

---

## 5. Repository

### File: `services/notification-service/internal/infra/postgres/delivery_repo.go`

```go
package postgres

import (
    "context"
    "fmt"

    "your-project/notification-service/internal/domain"
)

type DeliveryRepository struct {
    db *pgxpool.Pool
}

func (r *DeliveryRepository) Save(ctx context.Context, d domain.WebhookDelivery) error {
    _, err := r.db.Exec(ctx, `
        INSERT INTO webhook_deliveries
            (id, webhook_id, event, endpoint, status, response_time_ms,
             status_code, request_body, response_body, time)
        VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
    `, d.ID, d.WebhookID, d.Event, d.Endpoint, d.Status,
        d.ResponseTimeMs, d.StatusCode, d.RequestBody, d.ResponseBody, d.Time)
    return err
}

func (r *DeliveryRepository) GetByID(ctx context.Context, id string) (*domain.WebhookDelivery, error) {
    var d domain.WebhookDelivery
    err := r.db.QueryRow(ctx, `
        SELECT id, webhook_id, event, endpoint, status, response_time_ms,
               status_code, request_body, response_body, time
        FROM webhook_deliveries WHERE id = $1
    `, id).Scan(
        &d.ID, &d.WebhookID, &d.Event, &d.Endpoint, &d.Status,
        &d.ResponseTimeMs, &d.StatusCode, &d.RequestBody, &d.ResponseBody, &d.Time,
    )
    if err != nil {
        return nil, err
    }
    return &d, nil
}

func (r *DeliveryRepository) List(ctx context.Context, f domain.DeliveryFilter) ([]domain.WebhookDelivery, int, error) {
    query := `
        SELECT id, webhook_id, event, endpoint, status, response_time_ms,
               status_code, request_body, response_body, time
        FROM webhook_deliveries WHERE 1=1
    `
    args := []interface{}{}
    idx := 1

    if f.WebhookID != "" {
        query += fmt.Sprintf(" AND webhook_id = $%d", idx)
        args = append(args, f.WebhookID)
        idx++
    }
    if f.Status != "" {
        query += fmt.Sprintf(" AND status = $%d", idx)
        args = append(args, f.Status)
        idx++
    }

    // Count
    countQuery := "SELECT COUNT(*) FROM webhook_deliveries WHERE 1=1" + extractWhere(query)
    var total int
    r.db.QueryRow(ctx, countQuery, args...).Scan(&total)

    pageSize := f.PageSize
    if pageSize <= 0 {
        pageSize = 50
    }
    page := f.Page
    if page <= 0 {
        page = 1
    }
    offset := (page - 1) * pageSize

    query += fmt.Sprintf(" ORDER BY time DESC LIMIT $%d OFFSET $%d", idx, idx+1)
    args = append(args, pageSize, offset)

    rows, err := r.db.Query(ctx, query, args...)
    if err != nil {
        return nil, 0, err
    }
    defer rows.Close()

    var deliveries []domain.WebhookDelivery
    for rows.Next() {
        var d domain.WebhookDelivery
        rows.Scan(&d.ID, &d.WebhookID, &d.Event, &d.Endpoint, &d.Status,
            &d.ResponseTimeMs, &d.StatusCode, &d.RequestBody, &d.ResponseBody, &d.Time)
        deliveries = append(deliveries, d)
    }
    return deliveries, total, nil
}

func (r *DeliveryRepository) GetHourlyStats(ctx context.Context) ([]domain.WebhookHourlyStats, error) {
    rows, err := r.db.Query(ctx, `
        SELECT
            to_char(date_trunc('hour', time AT TIME ZONE 'UTC'), 'HH24:00') AS h,
            COUNT(*) FILTER (WHERE status = 'success')                       AS success,
            COUNT(*) FILTER (WHERE status IN ('failed', 'retried'))          AS failed
        FROM webhook_deliveries
        WHERE time >= NOW() - INTERVAL '24 hours'
        GROUP BY date_trunc('hour', time AT TIME ZONE 'UTC')
        ORDER BY date_trunc('hour', time AT TIME ZONE 'UTC')
    `)
    if err != nil {
        return nil, err
    }
    defer rows.Close()

    var stats []domain.WebhookHourlyStats
    for rows.Next() {
        var s domain.WebhookHourlyStats
        rows.Scan(&s.H, &s.Success, &s.Failed)
        stats = append(stats, s)
    }
    return stats, nil
}
```

---

## 6. HTTP Handler

### File: `services/notification-service/internal/delivery/http/webhook_delivery_handler.go`

```go
package http

import (
    "encoding/json"
    "net/http"

    "your-project/notification-service/internal/domain"
)

type WebhookDeliveryHandler struct {
    deliveryRepo DeliveryRepository
    dispatcher   *webhook.Dispatcher
    cache        *redis.Client
}

// GET /api/v1/webhooks/deliveries
// Query: webhook_id, status, page, page_size
func (h *WebhookDeliveryHandler) ListDeliveries(w http.ResponseWriter, r *http.Request) {
    q := r.URL.Query()
    filter := domain.DeliveryFilter{
        WebhookID: q.Get("webhook_id"),
        Status:    q.Get("status"),
        Page:      parseIntDefault(q.Get("page"), 1),
        PageSize:  parseIntDefault(q.Get("page_size"), 50),
    }

    deliveries, total, err := h.deliveryRepo.List(r.Context(), filter)
    if err != nil {
        jsonError(w, 500, "INTERNAL", "Failed to fetch deliveries")
        return
    }

    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(domain.WebhookDeliveriesResponse{
        Deliveries: deliveries,
        Total:      total,
    })
}

// POST /api/v1/webhooks/deliveries/{deliveryId}/retry
func (h *WebhookDeliveryHandler) RetryDelivery(w http.ResponseWriter, r *http.Request) {
    deliveryID := r.PathValue("deliveryId")

    // 1. Get original delivery
    original, err := h.deliveryRepo.GetByID(r.Context(), deliveryID)
    if err != nil {
        jsonError(w, 404, "NOT_FOUND", "Delivery not found")
        return
    }

    // 2. Chỉ retry failed deliveries
    if original.Status == "success" {
        jsonError(w, 422, "CANNOT_RETRY", "Cannot retry a successful delivery")
        return
    }

    // 3. Retry
    newDelivery, err := h.dispatcher.Retry(r.Context(), *original)
    if err != nil {
        jsonError(w, 500, "INTERNAL", "Failed to retry delivery")
        return
    }

    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(newDelivery)
}

// GET /api/v1/webhooks/stats/hourly
// Cache: Redis TTL 5 phút
func (h *WebhookDeliveryHandler) GetHourlyStats(w http.ResponseWriter, r *http.Request) {
    ctx := r.Context()
    cacheKey := "webhook:stats:hourly"

    // Check cache
    if cached, err := h.cache.Get(ctx, cacheKey).Bytes(); err == nil {
        w.Header().Set("Content-Type", "application/json")
        w.Header().Set("X-Cache", "HIT")
        w.Write(cached)
        return
    }

    stats, err := h.deliveryRepo.GetHourlyStats(ctx)
    if err != nil {
        jsonError(w, 500, "INTERNAL", "Failed to fetch hourly stats")
        return
    }

    data, _ := json.Marshal(stats)
    h.cache.Set(ctx, cacheKey, data, 5*time.Minute)

    w.Header().Set("Content-Type", "application/json")
    w.Header().Set("X-Cache", "MISS")
    w.Write(data)
}
```

---

## 7. Gateway Routing

### File: `apps/osv/internal/gateway/router.go`

```go
// Webhook Delivery routes — thêm vào sau các webhook routes hiện có
// ⚠️ /deliveries phải trước /{webhookId} để tránh conflict

// Delivery History
mux.Handle("GET /api/v1/webhooks/deliveries",
    protected(proxy.Forward("notification-service:8087")))

// Retry Delivery
mux.Handle("POST /api/v1/webhooks/deliveries/{deliveryId}/retry",
    protected(proxy.Forward("notification-service:8087")))

// Hourly Stats
// ⚠️ /webhooks/stats/hourly phải trước /webhooks/{id}
mux.Handle("GET /api/v1/webhooks/stats/hourly",
    protected(proxy.Forward("notification-service:8087")))
```

> **Lưu ý service port:** CR-009 ghi `webhook-service:8089` nhưng webhook được xử lý bởi `notification-service:8087`. Nếu team quyết định tách ra webhook-service riêng sau này, chỉ cần update target port.

---

## 8. Tests

```go
func TestListDeliveries_ReturnsPaginatedResults(t *testing.T) {
    // Seed 5 deliveries
    resp := GET("/api/v1/webhooks/deliveries?page=1&page_size=2")
    var result domain.WebhookDeliveriesResponse
    json.Unmarshal(resp.Body, &result)

    assert.Len(t, result.Deliveries, 2)
    assert.Equal(t, 5, result.Total)
}

func TestListDeliveries_FilterByStatus(t *testing.T) {
    resp := GET("/api/v1/webhooks/deliveries?status=failed")
    var result domain.WebhookDeliveriesResponse
    json.Unmarshal(resp.Body, &result)
    for _, d := range result.Deliveries {
        assert.Equal(t, "failed", d.Status)
    }
}

func TestRetryDelivery_SuccessForFailedDelivery(t *testing.T) {
    // Seed failed delivery
    resp := POST("/api/v1/webhooks/deliveries/DEL-001/retry", nil)
    assert.Equal(t, 200, resp.StatusCode)
    var result domain.WebhookDelivery
    json.Unmarshal(resp.Body, &result)
    assert.NotEmpty(t, result.ID) // New delivery record
}

func TestRetryDelivery_422ForSuccessDelivery(t *testing.T) {
    resp := POST("/api/v1/webhooks/deliveries/DEL-SUCCESS/retry", nil)
    assert.Equal(t, 422, resp.StatusCode)
}

func TestGetHourlyStats_ReturnsStatsArray(t *testing.T) {
    resp := GET("/api/v1/webhooks/stats/hourly")
    var result []domain.WebhookHourlyStats
    json.Unmarshal(resp.Body, &result)
    // Có thể ít hơn 24 items nếu không có data
    assert.LessOrEqual(t, len(result), 24)
    for _, s := range result {
        assert.Regexp(t, `^\d{2}:00$`, s.H) // format "HH:00"
    }
}

func TestDispatcher_LogsDelivery(t *testing.T) {
    // Trigger webhook
    // Verify delivery record được tạo trong webhook_deliveries
    deliveries, _, _ := repo.List(ctx, domain.DeliveryFilter{WebhookID: wh.ID})
    assert.NotEmpty(t, deliveries)
    assert.Contains(t, []string{"success", "failed"}, deliveries[0].Status)
}
```

---

## 9. Acceptance Criteria Checklist

> **Kết quả API Test (2026-06-22):**
> - `GET /api/v1/webhooks/deliveries` → ❌ **405** (route chỉ đăng ký POST, chưa hỗ trợ GET)
> - `POST /api/v1/webhooks/deliveries/{id}/retry` → ✅ trả 404 đúng cho delivery không tồn tại
> - `GET /api/v1/webhooks/stats/hourly` → ❌ **404** (chưa implement)
> - **Bug cấn fix:** Gateway đang đăng ký `/webhooks/deliveries` chỉ có POST (hoặc GET route thiếu)

- [ ] `GET /api/v1/webhooks/deliveries` → `{ deliveries: [...], total: N }` — HTTP 200
- [ ] Filter `?webhook_id=wh-1` — chỉ deliveries của webhook đó
- [ ] Filter `?status=failed` — chỉ failed deliveries
- [ ] `POST /webhooks/deliveries/{id}/retry` với failed → HTTP 200, WebhookDelivery mới
- [ ] `POST /webhooks/deliveries/{id}/retry` với success → HTTP 422
- [ ] `GET /api/v1/webhooks/stats/hourly` → array (có thể < 24 items)
- [ ] Mỗi lần webhook gửi (trigger/test/retry) → tạo delivery record
- [ ] Delivery record có: `id`, `webhook_id`, `event`, `endpoint`, `status`, `response_time_ms`, `status_code`, `time`
