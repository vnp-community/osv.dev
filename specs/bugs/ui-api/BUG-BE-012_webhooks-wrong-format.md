# BUG-BE-012 — GET /webhooks Trả Object Thay Vì Array

| Trường | Giá trị |
|---|---|
| **ID** | BUG-BE-012 |
| **Severity** | 🟡 Medium |
| **Priority** | P1 |
| **Component** | Backend / API Gateway / Integration Handlers |
| **Endpoint** | `GET /api/v1/webhooks` |
| **Phát hiện** | 2026-06-19 |
| **Server** | https://c12.openledger.vn |
| **Status** | Open |

---

## Mô tả

`GET /api/v1/webhooks` trả về một **object** `{ "webhooks": [...] }` thay vì **array trực tiếp** như spec định nghĩa. Frontend đang iterate `response` như array → crash vì nhận object.

## Tái hiện

```bash
curl -H "Authorization: Bearer <token>" \
  https://c12.openledger.vn/api/v1/webhooks
```

**Actual response:**
```json
HTTP/1.1 200 OK
{
  "webhooks": [
    {
      "id": "uuid",
      "name": "Slack Alerts",
      "url": "https://hooks.slack.com/...",
      "events": ["scan.completed"],
      "secret": "***",
      "active": true,
      "created_at": "..."
    }
  ]
}
```

**Expected response (theo spec):**
```json
HTTP/1.1 200 OK
[
  {
    "id": "uuid",
    "name": "Slack Alerts",
    "url": "https://hooks.slack.com/...",
    "events": ["scan.completed"],
    "secret": "***",
    "active": true,
    "created_at": "..."
  }
]
```

## Fix Options

**Option A — Sửa backend** (recommended): Trả array trực tiếp:
```go
c.JSON(200, webhooks) // thay vì gin.H{"webhooks": webhooks}
```

**Option B — Sửa spec**: Đổi spec thành `{ "webhooks": [...] }` — nhất quán với pattern của các endpoint khác.

> **Lưu ý**: Nên chọn Option A để nhất quán với spec hiện tại. Nếu chọn Option B thì phải update toàn bộ endpoint schema.

## Ảnh hưởng

- Integrations page: Webhook list không render (TypeError: `response.map is not a function`)
