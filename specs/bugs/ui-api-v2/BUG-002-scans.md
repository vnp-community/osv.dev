# BUG-002 — Scanning: Scan Dashboard (API 404)

**Nguồn gốc**: [`ui-crash/BUG-scans.md`](../../../ui/specs/bugs/ui-crash/BUG-scans.md)  
**Loại**: 🔴 API 404 — Endpoint chưa được deploy  
**Priority**: P2

**Status**: `✅ Fixed` — TASK-008 / SOL-002 (fixed 2026-06-20)

---

## Thông tin

| Field | Value |
|-------|-------|
| **Route** | `/scans` |
| **HTTP Status** | `404 Not Found` |
| **Endpoint bị lỗi** | `GET /api/v1/scans/stats/weekly` |
| **Số lần xuất hiện** | 3 lần (repeated polling) |

---

## Root Cause (Backend)

Endpoint `GET /api/v1/scans/stats/weekly` chưa được implement hoặc route chưa đúng trên backend. Frontend Scan Dashboard component gọi endpoint này để hiển thị biểu đồ scan theo tuần.

**Error log**:
```
Failed to load resource: the server responded with a status of 404 ()
URL: https://c12.openledger.vn/api/v1/scans/stats/weekly
```

---

## Backend Fix Required

| Item | Detail |
|------|--------|
| **Endpoint** | `GET /api/v1/scans/stats/weekly` |
| **Action** | Implement endpoint và register route |
| **Expected Response** | Thống kê scan theo tuần (array of weekly stats) |

### Expected Response Schema

```json
{
  "data": [
    {
      "week": "2026-W24",
      "total": 45,
      "completed": 40,
      "failed": 3,
      "running": 2
    }
  ],
  "period": "last_12_weeks"
}
```

---

## Ghi chú

- Endpoint này bị gọi nhiều lần (3x) do component có retry/polling logic
- Cần implement cả route và business logic query thống kê scan theo tuần
