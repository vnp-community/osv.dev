# BUG-014 — Admin: Audit Logs (API 404)

**Nguồn gốc**: [`ui-crash/BUG-admin-audit.md`](../../../ui/specs/bugs/ui-crash/BUG-admin-audit.md)  
**Loại**: 🔴 API 404 — Endpoint chưa deploy  
**Priority**: P2

**Status**: `✅ Fixed` — TASK-013 / SOL-012 (fixed 2026-06-20)

---

## Thông tin

| Field | Value |
|-------|-------|
| **Route** | `/admin/audit` |
| **HTTP Status** | `404 Not Found` |
| **Endpoint bị lỗi** | `GET /api/v1/audit-log` |
| **Số lần xuất hiện** | 3 lần |

---

## Root Cause (Backend)

Endpoint `GET /api/v1/audit-log` chưa được implement hoặc route chưa đăng ký. Audit Log là tính năng admin ghi lại mọi hành động của users trong hệ thống.

**Error log**:
```
Failed to load resource: the server responded with a status of 404 ()
URL: https://c12.openledger.vn/api/v1/audit-log
```

---

## Backend Fix Required

### Cần implement

```
GET /api/v1/audit-log                    # List audit log entries (paginated, filterable)
GET /api/v1/audit-log/export             # Export audit log (CSV/JSON)
```

### Expected Response

```json
{
  "data": [
    {
      "id": "audit-001",
      "timestamp": "2026-06-19T10:05:00Z",
      "actor": {
        "id": "user-123",
        "email": "admin@example.com",
        "role": "admin"
      },
      "action": "finding.risk_accepted",
      "resource": {
        "type": "finding",
        "id": "finding-456"
      },
      "ip_address": "192.168.1.100",
      "user_agent": "Mozilla/5.0...",
      "metadata": {}
    }
  ],
  "pagination": { "page": 1, "pageSize": 50, "total": 1234 }
}
```

### Query Parameters cần hỗ trợ

| Param | Type | Description |
|-------|------|-------------|
| `page` | int | Trang hiện tại |
| `pageSize` | int | Số items/trang |
| `actor` | string | Filter theo user |
| `action` | string | Filter theo loại action |
| `from` | datetime | Filter từ ngày |
| `to` | datetime | Filter đến ngày |
