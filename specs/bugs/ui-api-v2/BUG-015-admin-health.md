# BUG-015 — Admin: System Health (API Contract Violation)

**Nguồn gốc**: [`ui-crash/BUG-admin-health.md`](../../../ui/specs/bugs/ui-crash/BUG-admin-health.md)  
**Loại**: 🟠 API Contract Violation — Health data shape không match  
**Priority**: P1

**Status**: `✅ Fixed` — TASK-006 / SOL-013 (fixed 2026-06-20)

---

## Thông tin

| Field | Value |
|-------|-------|
| **Route** | `/admin/health` |
| **Manifestation** | JS Crash — `TypeError: Cannot read properties of undefined (reading 'includes')` |
| **Root Cause** | Health check data shape không match — field bị `undefined` |
| **Component bị crash** | `SystemHealth-BF--TRmg.js` |

---

## Root Cause (Backend)

Backend Health endpoint trả về data shape không đúng contract. Component `SystemHealth` gọi `.includes()` trên một field (likely `status` field) để kiểm tra trạng thái service, nhưng field đó là `undefined`.

**Error log** (chi tiết hơn các bug khác — thấy rõ call chain):
```
TypeError: Cannot read properties of undefined (reading 'includes')
  at SystemHealth-BF--TRmg.js:1:994
  at Array.find (<anonymous>)
  at A (SystemHealth-BF--TRmg.js:1:984)
  at SystemHealth-BF--TRmg.js:1:4634
  at Array.map (<anonymous>)
  at children (SystemHealth-BF--TRmg.js:1:4618)
```

### Phân tích call chain

```javascript
// Logic trong SystemHealth component (reconstructed):
services.map(service => {
  const match = statusConfig.find(config => 
    service.status.includes(config.pattern)  // ← crash: service.status is undefined
  )
})
```

→ Backend trả về array `services` nhưng mỗi item thiếu field `status`.

---

## Backend Fix Required

| Item | Detail |
|------|--------|
| **Endpoint** | `GET /api/v1/health` hoặc `GET /api/v1/admin/health` |
| **Action** | Đảm bảo mỗi service item có field `status` là string |

### Expected Response Schema

```json
{
  "overall_status": "degraded",
  "services": [
    {
      "name": "database",
      "status": "healthy",        // ← field này PHẢI có, không được undefined
      "latency_ms": 12,
      "last_check": "2026-06-20T00:10:00Z"
    },
    {
      "name": "redis",
      "status": "healthy",
      "latency_ms": 2,
      "last_check": "2026-06-20T00:10:00Z"
    },
    {
      "name": "ai-service",
      "status": "degraded",       // ← "healthy" | "degraded" | "down"
      "error": "Connection timeout",
      "last_check": "2026-06-20T00:09:55Z"
    }
  ],
  "version": "1.2.3",
  "uptime_seconds": 86400
}
```

### Contract Rule

- Field `status` trong mỗi service object **PHẢI** là string
- Giá trị hợp lệ: `"healthy"`, `"degraded"`, `"down"`, `"unknown"`
- Không được omit field `status` hay trả `null`
