# TASK-005 — notification-service: Fix `event_types: null` → `[]`

**Bug**: [BUG-012](../BUG-012-webhooks.md)  
**Solution**: [SOL-011](../solutions/SOL-011-webhooks.md)  
**Priority**: 🟠 P1  
**Effort**: ~10 phút  
**Status**: `[x] DONE`

---

## Mô tả

`GET /api/v1/webhooks` trả `webhook.event_types: null`. Frontend `Webhooks` component gọi `.find()` trên `event_types` → crash.

---

## File cần sửa

Tìm webhook handler trong notification-service:

```bash
find /Users/binhnt/Lab/sec/cve/osv.dev/services/notification-service/internal -name "*.go" \
  | xargs grep -l "webhook\|Webhook" | head -5
```

---

## Thay đổi — Repository

**Tìm** phần scan `event_types` trong webhook repo:

```go
var eventTypes []string    // (có thể đang dùng pq.Array hoặc pgx scan)
if err := rows.Scan(/* ... */, &eventTypes /* ... */); err != nil {
```

**Sau khi scan**, thêm nil guard:

```go
if eventTypes == nil {
    eventTypes = make([]string, 0)   // never nil
}
wh.EventTypes = eventTypes
```

---

## Thay đổi — DTO / Response Builder

**Tìm** function chuyển entity sang response (tên có thể là `toWebhookResponse`, `mapWebhook`):

```go
func toWebhookResponse(wh *Webhook) WebhookResponse {
    return WebhookResponse{
        // ...
        EventTypes: wh.EventTypes,   // ← có thể nil nếu DB column NULL
    }
}
```

**Thay bằng**:

```go
func toWebhookResponse(wh *Webhook) WebhookResponse {
    eventTypes := wh.EventTypes
    if eventTypes == nil {
        eventTypes = make([]string, 0)  // never null in JSON
    }

    return WebhookResponse{
        ID:         wh.ID.String(),
        Name:       wh.Name,
        URL:        wh.URL,
        EventTypes: eventTypes,         // always []
        IsActive:   wh.IsActive,
        CreatedAt:  wh.CreatedAt.Format(time.RFC3339),
        UpdatedAt:  wh.UpdatedAt.Format(time.RFC3339),
    }
}
```

---

## Thay đổi — Handler List

**Thêm nil guard** trước respond:

```go
if webhooks == nil {
    webhooks = make([]*Webhook, 0)
}

responses := make([]WebhookResponse, 0, len(webhooks))
for _, wh := range webhooks {
    responses = append(responses, toWebhookResponse(wh))
}

respondJSON(w, http.StatusOK, map[string]interface{}{
    "data":  responses,   // never null
    "total": len(responses),
})
```

---

## Acceptance Criteria

- [ ] `GET /api/v1/webhooks` trả `event_types: []` (không phải `null`)
- [ ] Không còn crash `undefined.find` trên trang Webhooks
- [ ] `go build ./...` trong notification-service không có lỗi

---

## Verify

```bash
curl -s -H "Authorization: Bearer <token>" \
  "https://c12.openledger.vn/api/v1/webhooks" | jq '.data[0].event_types | type'
# Expected: "array"
```
