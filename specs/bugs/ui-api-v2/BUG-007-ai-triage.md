# BUG-007 — AI Center: AI Triage Queue (API 503)

**Nguồn gốc**: [`ui-crash/BUG-ai-triage.md`](../../../ui/specs/bugs/ui-crash/BUG-ai-triage.md)  
**Loại**: 🔵 API 503 — Service Unavailable  
**Priority**: P1

**Status**: `✅ Fixed` — TASK-007 / SOL-007 (fixed 2026-06-20)

---

## Thông tin

| Field | Value |
|-------|-------|
| **Route** | `/ai/triage` |
| **HTTP Status** | `503 Service Unavailable` |
| **Endpoint bị lỗi** | `GET /api/v1/ai/triage/queue` |
| **Số lần xuất hiện** | 3 lần |

---

## Root Cause (Backend)

AI Triage service đang down hoặc chưa được khởi động. HTTP 503 chỉ ra service không khả dụng (service not running, resource exhausted, hoặc chưa deploy).

**Error log**:
```
Failed to load resource: the server responded with a status of 503 ()
URL: https://c12.openledger.vn/api/v1/ai/triage/queue
```

---

## Backend Fix Required

| Item | Detail |
|------|--------|
| **Endpoint** | `GET /api/v1/ai/triage/queue` |
| **Action** | Khởi động AI Triage service / kiểm tra deployment |

### Checklist

- [ ] Kiểm tra AI Triage service có đang chạy không
- [ ] Xem logs service: `docker logs ai-triage-service` hoặc `systemctl status ai-triage`
- [ ] Kiểm tra resource (memory/CPU) có đủ không
- [ ] Kiểm tra config (API keys, model endpoints) có đúng không
- [ ] Nếu chưa deploy: deploy AI Triage service

### Expected Response

```json
{
  "data": [
    {
      "id": "triage-001",
      "finding_id": "finding-456",
      "ai_score": 8.5,
      "ai_recommendation": "HIGH PRIORITY - SQL Injection vulnerability",
      "status": "pending_review"
    }
  ],
  "queue_size": 12
}
```
