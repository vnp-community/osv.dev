# BUG-005 — Assets: Asset Inventory (API Contract Violation — CRITICAL)

**Nguồn gốc**: [`ui-crash/BUG-assets.md`](../../../ui/specs/bugs/ui-crash/BUG-assets.md)  
**Loại**: 🟠 API Contract Violation — Backend trả `null` thay vì array  
**Priority**: P0 — CRITICAL

**Status**: `✅ Fixed` — TASK-002 / SOL-005 (fixed 2026-06-20)

---

## Thông tin

| Field | Value |
|-------|-------|
| **Route** | `/assets` |
| **Manifestation** | JS Crash — `TypeError: Cannot read properties of null (reading 'filter')` |
| **Root Cause** | API trả về `null` thay vì array |
| **Component bị crash** | `AssetInventory-DyFjB9ih.js` |

---

## Root Cause (Backend)

Backend API endpoint cho asset list trả về `null` trong response body thay vì empty array `[]`. Frontend component `AssetInventory` gọi `.filter()` trực tiếp trên data nhận về, gây crash khi data là `null`.

**Error log**:
```
TypeError: Cannot read properties of null (reading 'filter')
  at children (AssetInventory-DyFjB9ih.js:1:3266)
  at f (QueryBoundary-D1KgYtbP.js:1:2415)
```

---

## Backend Fix Required

| Item | Detail |
|------|--------|
| **Endpoint** | `GET /api/v1/assets` |
| **Current behavior** | Trả về `null` khi không có assets |
| **Expected behavior** | Trả về `[]` khi không có assets |
| **HTTP Status** | `200 OK` với empty array, không bao giờ `null` |

### Contract phải đảm bảo

```json
// ✅ Đúng — không có data
{
  "data": [],
  "pagination": { "page": 1, "pageSize": 50, "total": 0 }
}

// ✅ Đúng — có data
{
  "data": [
    {
      "id": "asset-001",
      "hostname": "server-01.internal",
      "ip": "10.0.0.1",
      "type": "server",
      "risk_score": 7.5
    }
  ],
  "pagination": { "page": 1, "pageSize": 50, "total": 1 }
}

// ❌ Sai — gây crash
null
{ "data": null }
```

---

## Impact

- Trang Asset Inventory hoàn toàn không dùng được
- **P0** — assets là core feature của security platform
