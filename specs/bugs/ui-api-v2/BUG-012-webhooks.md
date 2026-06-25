# BUG-012 — Integrations: Webhook Events (API Contract Violation)

**Nguồn gốc**: [`ui-crash/BUG-integrations-webhooks.md`](../../../ui/specs/bugs/ui-crash/BUG-integrations-webhooks.md)  
**Loại**: 🟡 API Contract Violation (partial) — Backend trả sai shape  
**Priority**: P1

**Status**: `✅ Fixed` — TASK-005 / SOL-011 (fixed 2026-06-20)

---

## Thông tin

| Field | Value |
|-------|-------|
| **Route** | `/integrations/webhooks` |
| **Manifestation** | JS Crash — `TypeError: Cannot read properties of undefined (reading 'find')` |
| **Root Cause** | Data từ API không có field array mà component dùng `.find()` |
| **Component bị crash** | `WebhookEvents-8Y0JYtM0.js` |

---

## Root Cause (Backend)

Frontend component `WebhookEvents` gọi `.find()` trên một field trong API response, nhưng field đó là `undefined`. Nguyên nhân là backend trả về response không đúng schema — thiếu field expected.

**Error log**:
```
TypeError: Cannot read properties of undefined (reading 'find')
  at children (WebhookEvents-8Y0JYtM0.js:1:2380)
  at f (QueryBoundary-D1KgYtbP.js:1:2415)
```

---

## Phân tích

### Tại sao là backend bug?

`.find()` được gọi trên array data từ API response. Khác với lỗi timing/loading thông thường, `QueryBoundary` wrapper (`QueryBoundary-D1KgYtbP.js`) đảm bảo component chỉ render khi data đã load xong. Do đó lỗi xảy ra **sau khi data đã nhận** — tức là backend trả về data nhưng thiếu field.

### Kịch bản likely

```javascript
// Component expect:
const webhook = webhooks.find(w => w.id === selectedId)
//                        ^^^^^^^^ "webhooks" là undefined vì API không trả field này
```

---

## Backend Fix Required

| Item | Detail |
|------|--------|
| **Endpoint** | `GET /api/v1/webhooks` hoặc endpoint tương ứng |
| **Action** | Đảm bảo response có đúng schema với tất cả array fields |

### Expected Response Schema

```json
{
  "data": [
    {
      "id": "wh-001",
      "name": "Slack Notifications",
      "url": "https://hooks.slack.com/services/...",
      "events": ["finding.created", "scan.completed"],
      "status": "active",
      "secret": "***",
      "last_delivery": {
        "status": "success",
        "timestamp": "2026-06-19T10:00:00Z"
      }
    }
  ],
  "event_types": [           // ← field này phải luôn có
    "finding.created",
    "finding.updated",
    "scan.completed",
    "scan.failed"
  ]
}

// ❌ Sai — event_types bị thiếu → .find() crash
{
  "data": [...]
  // event_types field bị omit
}
```
