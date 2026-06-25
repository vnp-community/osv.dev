# BUG-010 — Notifications: Notification Center (API 404)

**Nguồn gốc**: [`ui-crash/BUG-notifications.md`](../../../ui/specs/bugs/ui-crash/BUG-notifications.md)  
**Loại**: 🔴 API 404 — Endpoint chưa deploy  
**Priority**: P2

**Status**: `✅ Fixed` — TASK-011 / SOL-009 (fixed 2026-06-20)

---

## Thông tin

| Field | Value |
|-------|-------|
| **Route** | `/notifications` |
| **HTTP Status** | `404 Not Found` |
| **Endpoint bị lỗi** | `GET /api/v1/notifications` |
| **Số lần xuất hiện** | 3 lần |

---

## Root Cause (Backend)

Endpoint `GET /api/v1/notifications` chưa được implement hoặc route chưa đăng ký.

**Error log**:
```
Failed to load resource: the server responded with a status of 404 ()
URL: https://c12.openledger.vn/api/v1/notifications
```

---

## Backend Fix Required

### Cần implement

```
GET    /api/v1/notifications              # List user notifications
PUT    /api/v1/notifications/:id/read     # Mark as read
PUT    /api/v1/notifications/read-all     # Mark all as read
DELETE /api/v1/notifications/:id          # Delete notification
```

### Expected Response

```json
{
  "data": [
    {
      "id": "notif-001",
      "type": "finding_critical",
      "title": "New critical finding detected",
      "message": "CVE-2024-9999 found in production asset",
      "is_read": false,
      "created_at": "2026-06-19T10:00:00Z",
      "link": "/findings/finding-789"
    }
  ],
  "unread_count": 5,
  "pagination": { "page": 1, "pageSize": 20, "total": 12 }
}
```
