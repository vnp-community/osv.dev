# Change Request 009: Webhook Deliveries & Hourly Stats

**Tạo:** 2026-06-19  
**Status:** New — 3 endpoints thiếu trong webhook-service.  
**Nguồn:** openapi.yaml schemas `WebhookDelivery`, `WebhookDeliveriesResponse`, `WebhookHourlyStats`  
**Target directory:** `specs/crs/v2/api-ui-v1/`

---

## 1. Bối cảnh

Frontend `WebhookEvents.tsx` hiện hardcode delivery history và activity chart. Sau khi fix, component gọi 3 endpoints mới:

| Endpoint | Method | Path | Service | Trạng thái |
|---|---|---|---|---|
| Delivery history | `GET` | `/api/v1/webhooks/deliveries` | webhook-service | ❌ THIẾU |
| Retry delivery | `POST` | `/api/v1/webhooks/deliveries/{id}/retry` | webhook-service | ❌ THIẾU |
| Hourly stats | `GET` | `/api/v1/webhooks/stats/hourly` | webhook-service | ❌ THIẾU |

Backend hiện chỉ có CRUD cho webhooks (`/api/v1/webhooks`, `/api/v1/webhooks/{id}`, `/api/v1/webhooks/{id}/test`).

---

## 2. Chi tiết Thay đổi

### 2.1 [HIGH] `GET /api/v1/webhooks/deliveries` — Delivery History

**Gateway routing** — thêm vào `apps/osv/internal/gateway/router.go`:

```go
// Webhook Delivery History
mux.Handle("GET /api/v1/webhooks/deliveries",
    protected(proxy.Forward("webhook-service:8089")))
mux.Handle("POST /api/v1/webhooks/deliveries/{deliveryId}/retry",
    protected(proxy.Forward("webhook-service:8089")))
mux.Handle("GET /api/v1/webhooks/stats/hourly",
    protected(proxy.Forward("webhook-service:8089")))
```

**Query parameters được hỗ trợ:**
- `webhook_id` (optional) — filter theo webhook cụ thể
- `status` — filter: `success | failed | retried`
- `page` — default: 1
- `page_size` — default: 50

**Response schema `WebhookDeliveriesResponse`:**
```json
{
  "deliveries": [
    {
      "id": "DEL-0441",
      "webhook_id": "wh-1",
      "event": "finding.created",
      "endpoint": "siem.company.com",
      "status": "success",
      "response_time_ms": 124,
      "status_code": 200,
      "time": "2026-06-19T04:00:00Z",
      "request_body": "{\"event\":\"finding.created\",...}",
      "response_body": "{\"ok\":true}"
    }
  ],
  "total": 441
}
```

**DB schema mới cần thiết** (`webhook_deliveries` table):
```sql
CREATE TABLE webhook_deliveries (
    id              VARCHAR(36) PRIMARY KEY,
    webhook_id      VARCHAR(36) NOT NULL REFERENCES webhooks(id),
    event           VARCHAR(100) NOT NULL,
    endpoint        VARCHAR(255) NOT NULL,
    status          VARCHAR(20) NOT NULL CHECK (status IN ('success', 'failed', 'retried')),
    response_time_ms INTEGER NOT NULL,
    status_code     INTEGER NOT NULL,
    request_body    TEXT,
    response_body   TEXT,
    time            TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_webhook_deliveries_webhook_id ON webhook_deliveries(webhook_id);
CREATE INDEX idx_webhook_deliveries_time ON webhook_deliveries(time DESC);
```

Webhook-service cần **ghi log** mỗi delivery attempt vào bảng này.

---

### 2.2 [HIGH] `POST /api/v1/webhooks/deliveries/{id}/retry` — Retry

**Logic:**
1. Lấy delivery record từ DB
2. Kiểm tra `status == "failed"` (không retry `success`)
3. Lấy webhook config (URL, secret, events)
4. Gửi lại HTTP POST với cùng `request_body`
5. Tạo delivery record mới với status mới

**Response:**
- `200 OK` — `WebhookDelivery` record mới
- `404 Not Found` — delivery không tồn tại
- `422 Unprocessable Entity` — delivery đang `success` không cần retry

```go
// services/webhook-service/internal/handler/delivery.go
func (h *Handler) RetryDelivery(w http.ResponseWriter, r *http.Request) {
    deliveryId := r.PathValue("deliveryId")
    original, err := h.repo.GetDelivery(r.Context(), deliveryId)
    if err != nil {
        http.Error(w, "not found", 404)
        return
    }
    if original.Status == "success" {
        http.Error(w, "cannot retry successful delivery", 422)
        return
    }
    
    // Gửi lại và tạo record mới
    newDelivery, err := h.sender.Retry(r.Context(), original)
    if err != nil {
        http.Error(w, err.Error(), 500)
        return
    }
    
    json.NewEncoder(w).Encode(newDelivery)
}
```

---

### 2.3 [MEDIUM] `GET /api/v1/webhooks/stats/hourly` — Hourly Chart

**Response** — array WebhookHourlyStats (24 items, mỗi item = 1 giờ):
```json
[
  { "h": "00:00", "success": 12, "failed": 0 },
  { "h": "01:00", "success": 8,  "failed": 1 },
  ...
  { "h": "23:00", "success": 45, "failed": 2 }
]
```

**Query logic:**
```sql
SELECT 
    to_char(date_trunc('hour', time), 'HH24:00') AS h,
    COUNT(*) FILTER (WHERE status = 'success') AS success,
    COUNT(*) FILTER (WHERE status IN ('failed', 'retried')) AS failed
FROM webhook_deliveries
WHERE time >= NOW() - INTERVAL '24 hours'
GROUP BY date_trunc('hour', time)
ORDER BY date_trunc('hour', time);
```

**Cache khuyến nghị**: Redis TTL 5 phút.

---

### 2.4 Webhook Dispatcher — Logging yêu cầu

Webhook-service hiện có thể gửi webhooks nhưng **không log** delivery attempts. Cần thêm logging:

```go
// services/webhook-service/internal/dispatcher/dispatcher.go
func (d *Dispatcher) dispatch(ctx context.Context, wh Webhook, payload []byte) {
    start := time.Now()
    resp, err := d.client.Post(wh.URL, "application/json", bytes.NewReader(payload))
    
    delivery := Delivery{
        ID:             uuid.New().String(),
        WebhookID:      wh.ID,
        Event:          wh.Event,
        Endpoint:       extractHostname(wh.URL),
        RequestBody:    string(payload),
        Time:           start,
    }
    
    if err != nil || resp.StatusCode >= 400 {
        delivery.Status = "failed"
        delivery.StatusCode = resp.StatusCode
        delivery.ResponseTimeMs = int(time.Since(start).Milliseconds())
    } else {
        delivery.Status = "success"
        delivery.StatusCode = resp.StatusCode
        body, _ := io.ReadAll(resp.Body)
        delivery.ResponseBody = string(body)
        delivery.ResponseTimeMs = int(time.Since(start).Milliseconds())
    }
    
    d.repo.SaveDelivery(ctx, delivery) // async, non-blocking
}
```

---

## 3. Tiêu chí nghiệm thu (Acceptance Criteria)

1. `GET /api/v1/webhooks/deliveries` trả về `{ deliveries: [...], total: N }` — HTTP 200.
2. Filter `?webhook_id=wh-1` chỉ trả deliveries của webhook đó.
3. Filter `?status=failed` chỉ trả failed deliveries.
4. `POST /api/v1/webhooks/deliveries/{id}/retry` với failed delivery → HTTP 200, trả `WebhookDelivery`.
5. `POST /api/v1/webhooks/deliveries/{id}/retry` với success delivery → HTTP 422.
6. `GET /api/v1/webhooks/stats/hourly` trả về array (có thể ít hơn 24 items nếu không có data cho giờ đó).
7. Mỗi lần webhook được gửi (trigger, test, retry) đều tạo delivery record trong DB.
8. Delivery record có đầy đủ: `id`, `webhook_id`, `event`, `endpoint`, `status`, `response_time_ms`, `status_code`, `time`.
