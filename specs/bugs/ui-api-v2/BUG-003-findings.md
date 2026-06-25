# BUG-003 — Findings: All Findings (API 500 — CRITICAL)

**Nguồn gốc**: [`ui-crash/BUG-findings.md`](../../../ui/specs/bugs/ui-crash/BUG-findings.md)  
**Loại**: 🔴 API 500 — Backend Server Error  
**Priority**: P0 — CRITICAL

**Status**: `✅ Fixed` — TASK-001 / SOL-003 (fixed 2026-06-20)

---

## Thông tin

| Field | Value |
|-------|-------|
| **Route** | `/findings` |
| **HTTP Status** | `500 Internal Server Error` |
| **Endpoint bị lỗi** | `GET /api/v1/findings?page=1&pageSize=50` |
| **Số lần xuất hiện** | 3 lần |

---

## Root Cause (Backend)

Backend crash khi xử lý query findings. Server trả về HTTP 500, nghĩa là có unhandled exception hoặc database error ở server side. Đây là bug nghiêm trọng nhất — dữ liệu findings hoàn toàn không truy cập được.

**Error log**:
```
Failed to load resource: the server responded with a status of 500 ()
URL: https://c12.openledger.vn/api/v1/findings?page=1&pageSize=50
```

---

## Backend Fix Required

| Item | Detail |
|------|--------|
| **Endpoint** | `GET /api/v1/findings` |
| **Action** | Debug server logs, fix unhandled exception |
| **Likely causes** | Database query error, nil pointer, unhandled data migration |

### Debugging Steps

1. **Kiểm tra server logs** tại thời điểm request:
   ```bash
   # Tìm 500 errors trong logs
   grep "500" /var/log/app/api.log | grep "findings"
   ```

2. **Kiểm tra database**:
   - Schema `findings` table có đúng không?
   - Có migration nào chưa chạy không?
   - Foreign key constraints có vi phạm không?

3. **Thử query trực tiếp** với curl để xem response body error

### Expected Behavior

```json
// ✅ Đúng — paginated response
{
  "data": [...],
  "pagination": {
    "page": 1,
    "pageSize": 50,
    "total": 120,
    "totalPages": 3
  }
}
```

---

## Impact

- Trang `/findings` hoàn toàn không sử dụng được
- Đây là trang core của ứng dụng — **P0**, cần fix ngay
