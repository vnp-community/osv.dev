# SOL-011 — Webhooks: Fix Response Schema (P1)

**Bug**: [BUG-012](../BUG-012-webhooks.md)  
**Service**: `notification-service`  
**Endpoint**: `GET /api/v1/webhooks`  
**Lỗi frontend**: Data shape sai — thiếu field `event_types` hoặc `deliveries`

**Status**: `✅ Implemented` — via [TASK-005](../../tasks/TASK-005-*.md)

---

## Root Cause

Frontend `Webhooks` component đọc `webhook.event_types` (array of strings) để hiển thị events. Backend trả `event_types` là:
- `null` thay vì `[]`
- Không có field này trong response

---

## Giải pháp

### Fix trong notification-service — Webhook Handler

```go
// services/notification-service/internal/delivery/http/webhook_handler.go

type WebhookResponse struct {
    ID          string   `json:"id"`
    Name        string   `json:"name"`
    URL         string   `json:"url"`
    EventTypes  []string `json:"event_types"`   // ← PHẢI là array, không null
    IsActive    bool     `json:"is_active"`
    Secret      string   `json:"secret,omitempty"`  // omit trong List
    CreatedAt   string   `json:"created_at"`
    UpdatedAt   string   `json:"updated_at"`
    // Stats (optional)
    TotalDeliveries    int `json:"total_deliveries,omitempty"`
    SuccessDeliveries  int `json:"success_deliveries,omitempty"`
    FailedDeliveries   int `json:"failed_deliveries,omitempty"`
}

func toWebhookResponse(w *Webhook) WebhookResponse {
    // FIX: defensive nil check for EventTypes
    eventTypes := w.EventTypes
    if eventTypes == nil {
        eventTypes = make([]string, 0)  // never null
    }
    
    return WebhookResponse{
        ID:         w.ID.String(),
        Name:       w.Name,
        URL:        w.URL,
        EventTypes: eventTypes,   // always array
        IsActive:   w.IsActive,
        CreatedAt:  w.CreatedAt.Format(time.RFC3339),
        UpdatedAt:  w.UpdatedAt.Format(time.RFC3339),
    }
}

// List — GET /api/v1/webhooks
func (h *WebhookHandler) List(w http.ResponseWriter, r *http.Request) {
    webhooks, err := h.repo.List(r.Context())
    if err != nil {
        respondError(w, http.StatusInternalServerError, "failed to list webhooks")
        return
    }
    
    // Defensive: never nil
    if webhooks == nil {
        webhooks = make([]*Webhook, 0)
    }
    
    responses := make([]WebhookResponse, 0, len(webhooks))
    for _, wh := range webhooks {
        responses = append(responses, toWebhookResponse(wh))
    }
    
    respondJSON(w, http.StatusOK, map[string]interface{}{
        "data":  responses,  // always array
        "total": len(responses),
    })
}
```

### Fix Repository — Scan EventTypes

```go
// Repository phải scan event_types column đúng cách
// PostgreSQL lưu dưới dạng TEXT[] hoặc JSONB

func (r *WebhookRepo) List(ctx context.Context) ([]*Webhook, error) {
    rows, err := r.pool.Query(ctx, `
        SELECT id, name, url, event_types, is_active, secret, created_at, updated_at
        FROM webhooks ORDER BY created_at DESC
    `)
    if err != nil {
        return nil, err
    }
    defer rows.Close()
    
    result := make([]*Webhook, 0)  // never nil
    for rows.Next() {
        wh := &Webhook{}
        var eventTypes []string    // pgx scan TEXT[] as []string
        
        if err := rows.Scan(
            &wh.ID, &wh.Name, &wh.URL,
            &eventTypes,    // ← pgx hỗ trợ scan TEXT[] → []string
            &wh.IsActive, &wh.Secret,
            &wh.CreatedAt, &wh.UpdatedAt,
        ); err != nil {
            return nil, err
        }
        
        // Defensive: ensure not nil
        if eventTypes == nil {
            eventTypes = make([]string, 0)
        }
        wh.EventTypes = eventTypes
        
        result = append(result, wh)
    }
    return result, rows.Err()
}
```

---

## Response Schema

```json
// GET /api/v1/webhooks
{
  "data": [
    {
      "id": "uuid",
      "name": "CI/CD Webhook",
      "url": "https://hooks.example.com/osv",
      "event_types": ["finding.created", "finding.sla.breached"],
      "is_active": true,
      "created_at": "2026-01-01T00:00:00Z",
      "updated_at": "2026-06-20T00:00:00Z"
    }
  ],
  "total": 1
}
```

**Không được trả:**
```json
{
  "data": [{"event_types": null}]
}
```

---

## Verification

```bash
curl -H "Authorization: Bearer <token>" \
  "https://c12.openledger.vn/api/v1/webhooks" | jq '.[0].event_types | type'
# Expected: "array" (kể cả [])
```
