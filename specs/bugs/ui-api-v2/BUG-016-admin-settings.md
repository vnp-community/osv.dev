# BUG-016 — Admin: System Settings (API 404)

**Nguồn gốc**: [`ui-crash/BUG-admin-settings.md`](../../../ui/specs/bugs/ui-crash/BUG-admin-settings.md)  
**Loại**: 🔴 API 404 — Endpoint chưa deploy  
**Priority**: P2

**Status**: `✅ Fixed` — TASK-014 / SOL-014 (fixed 2026-06-20)

---

## Thông tin

| Field | Value |
|-------|-------|
| **Route** | `/admin/settings` |
| **HTTP Status** | `404 Not Found` |
| **Endpoint bị lỗi** | `GET /api/v1/admin/settings` |
| **Số lần xuất hiện** | 3 lần |

---

## Root Cause (Backend)

Endpoint `GET /api/v1/admin/settings` chưa được implement. Tính năng System Settings cho phép admin cấu hình các thông số hệ thống toàn cục.

**Error log**:
```
Failed to load resource: the server responded with a status of 404 ()
URL: https://c12.openledger.vn/api/v1/admin/settings
```

---

## Backend Fix Required

### Cần implement

```
GET  /api/v1/admin/settings          # Get all system settings
PUT  /api/v1/admin/settings          # Update settings (bulk update)
PUT  /api/v1/admin/settings/:key     # Update single setting
```

### Expected Response

```json
{
  "data": {
    "general": {
      "platform_name": "OSV Security Platform",
      "timezone": "Asia/Ho_Chi_Minh",
      "language": "en"
    },
    "security": {
      "session_timeout_minutes": 60,
      "mfa_required": false,
      "allowed_ip_ranges": []
    },
    "notifications": {
      "email_enabled": true,
      "slack_webhook": null,
      "critical_alert_threshold": 9.0
    },
    "scanning": {
      "max_concurrent_scans": 5,
      "default_scan_timeout_minutes": 30
    }
  }
}
```

### Authorization

- Endpoint này chỉ accessible với role `admin`
- Backend phải verify role trước khi trả data
