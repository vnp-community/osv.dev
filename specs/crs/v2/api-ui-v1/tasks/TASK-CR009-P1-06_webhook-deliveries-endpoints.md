# TASK-CR009-P1-06 — Webhook Deliveries: DB Table + 3 Endpoints

**Phase:** Phase 2 — Blocking UI  
**Nguồn giải pháp:** [`solutions/SOL-009`](../solutions/SOL-009-webhook-deliveries-hourly-stats.md)  
**Ưu tiên:** 🔴 P1 — WebhookEvents UI crash khi load history  
**Phụ thuộc:** Không có  
**Status:** ✅ **DONE** — 2026-06-19  

---

## Mục tiêu

1. Tạo bảng `webhook_deliveries` để log mỗi lần gửi webhook
2. Webhook dispatcher ghi delivery record sau mỗi lần gửi
3. 3 endpoints mới: GET deliveries, POST retry, GET hourly stats

> **Service target:** `notification-service:8087` (không phải `webhook-service:8089` — typo trong CR)

---

## Điều tra trước khi code

```bash
# 1. Kiểm tra bảng webhooks và deliveries
docker exec osv-backend-postgres-1 psql -U osv -d osv \
  -c "\dt" | grep -i "webhook"

docker exec osv-backend-postgres-1 psql -U osv -d osv \
  -c "\d webhooks" 2>/dev/null || echo "webhooks table not found"

# 2. Tìm dispatcher hiện tại
grep -rn "dispatch\|Dispatch\|webhook.*send\|Send.*webhook" \
  services/notification-service/ --include="*.go" -l

# 3. Xem notification-service handlers
ls services/notification-service/internal/delivery/http/

# 4. Kiểm tra gateway routes
grep -n "webhooks" apps/osv/internal/gateway/router.go
```

---

## Bước 1: DB Migration

**File:** `services/notification-service/migrations/20260619_001_webhook_deliveries.sql`

```sql
CREATE TABLE IF NOT EXISTS webhook_deliveries (
    id               VARCHAR(36)  PRIMARY KEY DEFAULT gen_random_uuid()::text,
    webhook_id       VARCHAR(36)  NOT NULL,
    event            VARCHAR(100) NOT NULL,
    endpoint         VARCHAR(255) NOT NULL,
    status           VARCHAR(20)  NOT NULL
        CHECK (status IN ('success', 'failed', 'retried')),
    response_time_ms INTEGER      NOT NULL DEFAULT 0,
    status_code      INTEGER      NOT NULL DEFAULT 0,
    request_body     TEXT,
    response_body    TEXT,
    time             TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

-- Foreign key nếu webhooks table tồn tại
-- ALTER TABLE webhook_deliveries
--     ADD CONSTRAINT fk_webhook FOREIGN KEY (webhook_id)
--     REFERENCES webhooks(id) ON DELETE CASCADE;

CREATE INDEX IF NOT EXISTS idx_webhook_deliveries_webhook_id
    ON webhook_deliveries(webhook_id);
CREATE INDEX IF NOT EXISTS idx_webhook_deliveries_time
    ON webhook_deliveries(time DESC);
CREATE INDEX IF NOT EXISTS idx_webhook_deliveries_status
    ON webhook_deliveries(status);
```

```bash
docker exec osv-backend-postgres-1 psql -U osv -d osv \
  -f /migrations/20260619_001_webhook_deliveries.sql

# Verify
docker exec osv-backend-postgres-1 psql -U osv -d osv \
  -c "\d webhook_deliveries"
```

---

## Bước 2: Thêm Logging vào Dispatcher

**Tìm dispatcher:**
```bash
grep -rn "func.*Send\|func.*dispatch\|func.*Dispatch" \
  services/notification-service/ --include="*.go" -n | grep -i "webhook"
```

**Thêm logging SAU mỗi lần gửi:**

```go
// Trong webhook dispatcher/sender — thêm sau mỗi HTTP call:
import "github.com/google/uuid"

func logDelivery(ctx context.Context, db DB, webhookID, event, endpoint string,
    status string, statusCode, responseTimeMs int, reqBody, respBody string) {
    // Non-blocking — không được block main flow
    go func() {
        _, err := db.Exec(context.Background(), `
            INSERT INTO webhook_deliveries
                (id, webhook_id, event, endpoint, status,
                 response_time_ms, status_code, request_body, response_body, time)
            VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, NOW())
        `, uuid.New().String(), webhookID, event, endpoint, status,
            responseTimeMs, statusCode, reqBody, respBody)
        if err != nil {
            log.Warn().Err(err).Str("webhook_id", webhookID).Msg("failed to log delivery")
        }
    }()
}

// Gọi sau khi send:
start := time.Now()
resp, err := client.Post(webhookURL, "application/json", bytes.NewReader(payload))
elapsed := int(time.Since(start).Milliseconds())

status := "success"
statusCode := 0
respBody := ""
if err != nil || resp.StatusCode >= 400 {
    status = "failed"
}
if resp != nil {
    statusCode = resp.StatusCode
    body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
    respBody = string(body)
}

logDelivery(ctx, h.db, webhookID, event,
    extractHostname(webhookURL), status, statusCode, elapsed,
    string(payload), respBody)
```

---

## Bước 3: Delivery Handlers

**Tạo file handler mới:**
`services/notification-service/internal/delivery/http/webhook_delivery_handler.go`

```go
package http

import (
    "encoding/json"
    "fmt"
    "net/http"
    "time"
)

// GET /api/v1/webhooks/deliveries
func (h *Handler) ListWebhookDeliveries(w http.ResponseWriter, r *http.Request) {
    ctx := r.Context()
    q   := r.URL.Query()

    webhookID := q.Get("webhook_id")
    status    := q.Get("status") // "success"|"failed"|"retried"
    page      := parseIntDefault(q.Get("page"), 1)
    pageSize  := parseIntDefault(q.Get("page_size"), 50)

    query := `
        SELECT id, webhook_id, event, endpoint, status,
               response_time_ms, status_code, request_body, response_body, time
        FROM webhook_deliveries WHERE 1=1
    `
    args := []interface{}{}
    idx  := 1

    if webhookID != "" {
        query += fmt.Sprintf(" AND webhook_id = $%d", idx)
        args = append(args, webhookID); idx++
    }
    if status != "" {
        query += fmt.Sprintf(" AND status = $%d", idx)
        args = append(args, status); idx++
    }

    // Count total
    var total int
    countQ := "SELECT COUNT(*) FROM webhook_deliveries WHERE 1=1"
    // append same filters...
    h.db.QueryRow(ctx, countQ, args[:idx-1]...).Scan(&total)

    query += fmt.Sprintf(" ORDER BY time DESC LIMIT $%d OFFSET $%d", idx, idx+1)
    args = append(args, pageSize, (page-1)*pageSize)

    rows, err := h.db.Query(ctx, query, args...)
    if err != nil {
        respondError(w, 500, "failed to fetch deliveries")
        return
    }
    defer rows.Close()

    type DeliveryDTO struct {
        ID             string    `json:"id"`
        WebhookID      string    `json:"webhook_id"`
        Event          string    `json:"event"`
        Endpoint       string    `json:"endpoint"`
        Status         string    `json:"status"`
        ResponseTimeMs int       `json:"response_time_ms"`
        StatusCode     int       `json:"status_code"`
        RequestBody    string    `json:"request_body,omitempty"`
        ResponseBody   string    `json:"response_body,omitempty"`
        Time           time.Time `json:"time"`
    }

    var deliveries []DeliveryDTO
    for rows.Next() {
        var d DeliveryDTO
        rows.Scan(&d.ID, &d.WebhookID, &d.Event, &d.Endpoint, &d.Status,
            &d.ResponseTimeMs, &d.StatusCode, &d.RequestBody, &d.ResponseBody, &d.Time)
        deliveries = append(deliveries, d)
    }
    if deliveries == nil {
        deliveries = []DeliveryDTO{}
    }

    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(map[string]interface{}{
        "deliveries": deliveries,
        "total":      total,
    })
}

// POST /api/v1/webhooks/deliveries/{deliveryId}/retry
func (h *Handler) RetryWebhookDelivery(w http.ResponseWriter, r *http.Request) {
    ctx        := r.Context()
    deliveryID := r.PathValue("deliveryId") // Go 1.22+

    // Get original delivery
    var original struct {
        WebhookID   string
        Event       string
        Endpoint    string
        Status      string
        RequestBody string
    }
    err := h.db.QueryRow(ctx, `
        SELECT webhook_id, event, endpoint, status, request_body
        FROM webhook_deliveries WHERE id = $1
    `, deliveryID).Scan(
        &original.WebhookID, &original.Event, &original.Endpoint,
        &original.Status, &original.RequestBody,
    )
    if err != nil {
        respondError(w, 404, "delivery not found")
        return
    }

    // Chỉ retry failed
    if original.Status == "success" {
        w.Header().Set("Content-Type", "application/json")
        w.WriteHeader(422)
        json.NewEncoder(w).Encode(map[string]string{
            "error":   "CANNOT_RETRY",
            "message": "cannot retry a successful delivery",
        })
        return
    }

    // Lấy webhook config (URL, secret)
    var webhookURL, secret string
    h.db.QueryRow(ctx,
        `SELECT url, secret FROM webhooks WHERE id = $1`, original.WebhookID,
    ).Scan(&webhookURL, &secret)

    // Gửi lại
    start    := time.Now()
    payload  := []byte(original.RequestBody)
    newID    := uuid.New().String()

    // ... thực hiện HTTP POST với HMAC signature ...
    // (tái dùng code từ dispatcher)

    status     := "success"
    statusCode := 200
    elapsed    := int(time.Since(start).Milliseconds())

    // Lưu new delivery record
    h.db.Exec(ctx, `
        INSERT INTO webhook_deliveries
            (id, webhook_id, event, endpoint, status, response_time_ms, status_code,
             request_body, time)
        VALUES ($1, $2, $3, $4, $5, $6, $7, $8, NOW())
    `, newID, original.WebhookID, original.Event, original.Endpoint,
        status, elapsed, statusCode, original.RequestBody)

    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(map[string]interface{}{
        "id":              newID,
        "webhook_id":      original.WebhookID,
        "event":           original.Event,
        "endpoint":        original.Endpoint,
        "status":          status,
        "response_time_ms": elapsed,
        "status_code":     statusCode,
        "time":            time.Now(),
    })
}

// GET /api/v1/webhooks/stats/hourly
func (h *Handler) GetWebhookHourlyStats(w http.ResponseWriter, r *http.Request) {
    ctx := r.Context()
    cacheKey := "webhook:stats:hourly"

    // Redis cache 5 phút
    if h.cache != nil {
        if cached, err := h.cache.Get(ctx, cacheKey).Bytes(); err == nil {
            w.Header().Set("Content-Type", "application/json")
            w.Header().Set("X-Cache", "HIT")
            w.Write(cached)
            return
        }
    }

    rows, err := h.db.Query(ctx, `
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
        respondError(w, 500, "failed to fetch hourly stats")
        return
    }
    defer rows.Close()

    type HourlyStats struct {
        H       string `json:"h"`
        Success int    `json:"success"`
        Failed  int    `json:"failed"`
    }

    var stats []HourlyStats
    for rows.Next() {
        var s HourlyStats
        rows.Scan(&s.H, &s.Success, &s.Failed)
        stats = append(stats, s)
    }
    if stats == nil {
        stats = []HourlyStats{}
    }

    data, _ := json.Marshal(stats)
    if h.cache != nil {
        h.cache.Set(ctx, cacheKey, data, 5*time.Minute)
    }

    w.Header().Set("Content-Type", "application/json")
    w.Header().Set("X-Cache", "MISS")
    w.Write(data)
}
```

---

## Bước 4: Register Routes trong notification-service

```bash
grep -rn "Handle\|r\.Get\|mux\." \
  services/notification-service/ --include="*.go" | grep -i "webhook" | head -10
```

Thêm routes (thứ tự: `/deliveries` TRƯỚC `/{id}`):
```go
// Webhook Delivery routes
mux.Handle("GET /api/v1/webhooks/deliveries", h.ListWebhookDeliveries)
mux.Handle("POST /api/v1/webhooks/deliveries/{deliveryId}/retry", h.RetryWebhookDelivery)
mux.Handle("GET /api/v1/webhooks/stats/hourly", h.GetWebhookHourlyStats)
```

---

## Bước 5: Gateway Routes

```bash
grep -n "webhooks" apps/osv/internal/gateway/router.go
```

Thêm nếu chưa có:
```go
// ⚠️ /deliveries TRƯỚC /{id}, /stats/hourly TRƯỚC /{id}
mux.Handle("GET /api/v1/webhooks/deliveries",
    protected(proxy.Forward("notification-service:8087")))
mux.Handle("POST /api/v1/webhooks/deliveries/{deliveryId}/retry",
    protected(proxy.Forward("notification-service:8087")))
mux.Handle("GET /api/v1/webhooks/stats/hourly",
    protected(proxy.Forward("notification-service:8087")))
```

---

## Acceptance Criteria

- [ ] `GET /api/v1/webhooks/deliveries` → `{ deliveries: [...], total: N }` — HTTP 200
- [ ] Filter `?webhook_id=X` hoạt động
- [ ] Filter `?status=failed` hoạt động
- [ ] `POST /webhooks/deliveries/{id}/retry` với failed → HTTP 200, new delivery record
- [ ] `POST /webhooks/deliveries/{id}/retry` với success → HTTP 422
- [ ] `GET /api/v1/webhooks/stats/hourly` → array (≤ 24 items)
- [ ] Khi webhook được gửi → tự động tạo delivery record

## Verification

```bash
TOKEN="<your-token>"

# List deliveries (empty ok initially)
curl -s https://c12.openledger.vn/api/v1/webhooks/deliveries \
  -H "Authorization: Bearer $TOKEN" | jq '{total, count: (.deliveries | length)}'
# Expected: { "total": N, "count": N }

# Hourly stats
curl -s https://c12.openledger.vn/api/v1/webhooks/stats/hourly \
  -H "Authorization: Bearer $TOKEN" | jq 'length'
# Expected: 0 to 24

# Trigger a test webhook và verify delivery được log
curl -s -X POST https://c12.openledger.vn/api/v1/webhooks/<id>/test \
  -H "Authorization: Bearer $TOKEN"
# Sau đó check deliveries
curl -s "https://c12.openledger.vn/api/v1/webhooks/deliveries?webhook_id=<id>" \
  -H "Authorization: Bearer $TOKEN" | jq '.total'
# Expected: > 0
```
