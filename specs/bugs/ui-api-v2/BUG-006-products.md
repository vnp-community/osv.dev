# BUG-006 — Product Security (API Contract Violation)

**Nguồn gốc**: [`ui-crash/BUG-products.md`](../../../ui/specs/bugs/ui-crash/BUG-products.md)  
**Loại**: 🟠 API Contract Violation — Backend trả sai data shape  
**Priority**: P1

**Status**: `✅ Fixed` — TASK-004 / SOL-006 (fixed 2026-06-20)

---

## Thông tin

| Field | Value |
|-------|-------|
| **Route** | `/products` |
| **Manifestation** | JS Crash — `TypeError: Cannot read properties of undefined (reading 'flatMap')` |
| **Root Cause** | Data từ API không match expected shape |
| **Component bị crash** | `ProductSecurity-DLEnEWyR.js` |

---

## Root Cause (Backend)

Backend trả về data structure không khớp với contract mà frontend component `ProductSecurity` mong đợi. Component gọi `.flatMap()` trên một field trong response nhưng field đó là `undefined`.

**Error log**:
```
TypeError: Cannot read properties of undefined (reading 'flatMap')
  at children (ProductSecurity-DLEnEWyR.js:1:1697)
  at f (QueryBoundary-D1KgYtbP.js:1:2415)
```

---

## Backend Fix Required

| Item | Detail |
|------|--------|
| **Endpoint** | `GET /api/v1/products` |
| **Current behavior** | Trả về object thiếu field có thể `.flatMap()` |
| **Expected behavior** | Response phải có field array ở đúng path mà component expect |

### Vấn đề

`.flatMap()` được dùng để flatten nested arrays — ví dụ: lấy tất cả vulnerabilities từ nhiều products. Backend phải đảm bảo response có structure đúng:

```json
// ✅ Đúng — field có thể flatMap
{
  "data": [
    {
      "id": "prod-001",
      "name": "My Application",
      "components": [          // ← field này phải là array
        { "name": "lodash", "version": "4.17.11", "cves": [...] }
      ]
    }
  ]
}

// ❌ Sai — components missing hoặc undefined
{
  "data": [
    {
      "id": "prod-001",
      "name": "My Application"
      // components field bị thiếu → .flatMap() crash
    }
  ]
}
```

### Backend action

- Xác định field nào đang bị thiếu trong response của `GET /api/v1/products`
- Đảm bảo tất cả array fields được trả về (kể cả empty array `[]`) thay vì bị omit
