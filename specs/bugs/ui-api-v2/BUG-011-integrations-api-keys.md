# BUG-011 — Integrations: API Key Management (API 404)

**Nguồn gốc**: [`ui-crash/BUG-integrations-api-keys.md`](../../../ui/specs/bugs/ui-crash/BUG-integrations-api-keys.md)  
**Loại**: 🔴 API 404 — Endpoint chưa deploy  
**Priority**: P2

**Status**: `✅ Fixed` — TASK-012 / SOL-010 (fixed 2026-06-20)

---

## Thông tin

| Field | Value |
|-------|-------|
| **Route** | `/integrations/api-keys` |
| **HTTP Status** | `404 Not Found` |
| **Endpoint bị lỗi** | `GET /api/v1/api-keys` |
| **Số lần xuất hiện** | 3 lần |

---

## Root Cause (Backend)

Endpoint `GET /api/v1/api-keys` chưa được implement. Tính năng quản lý API keys cho phép users tạo và quản lý access tokens cho third-party integrations.

**Error log**:
```
Failed to load resource: the server responded with a status of 404 ()
URL: https://c12.openledger.vn/api/v1/api-keys
```

---

## Backend Fix Required

### Cần implement

```
GET    /api/v1/api-keys              # List user's API keys
POST   /api/v1/api-keys              # Create new API key
DELETE /api/v1/api-keys/:id          # Revoke API key
PUT    /api/v1/api-keys/:id          # Update API key (name, permissions)
```

### Expected Response

```json
{
  "data": [
    {
      "id": "key-001",
      "name": "CI/CD Pipeline Key",
      "prefix": "sk-abc123",
      "permissions": ["findings:read", "scans:write"],
      "last_used_at": "2026-06-18T14:00:00Z",
      "created_at": "2026-01-15T00:00:00Z",
      "expires_at": null
    }
  ]
}
```

### Security Notes

- API key value chỉ được trả về **một lần** khi tạo (`POST`)
- `GET /api/v1/api-keys` chỉ trả về metadata (prefix, permissions), không trả key value
