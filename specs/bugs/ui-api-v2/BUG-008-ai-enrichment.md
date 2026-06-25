# BUG-008 — AI Center: AI Enrichment (API 503)

**Nguồn gốc**: [`ui-crash/BUG-ai-enrichment.md`](../../../ui/specs/bugs/ui-crash/BUG-ai-enrichment.md)  
**Loại**: 🔵 API 503 — Service Unavailable  
**Priority**: P1

**Status**: `✅ Fixed` — TASK-007 / SOL-007 (fixed 2026-06-20)

---

## Thông tin

| Field | Value |
|-------|-------|
| **Route** | `/ai/enrichment` |
| **HTTP Status** | `503 Service Unavailable` |
| **Endpoint bị lỗi** | `GET /api/v1/ai/enrichment` |
| **Số lần xuất hiện** | 3 lần |

---

## Root Cause (Backend)

AI Enrichment service đang down hoặc chưa được khởi động. Tương tự BUG-007, HTTP 503 chỉ ra backend service không khả dụng.

**Error log**:
```
Failed to load resource: the server responded with a status of 503 ()
URL: https://c12.openledger.vn/api/v1/ai/enrichment
```

---

## Backend Fix Required

| Item | Detail |
|------|--------|
| **Endpoint** | `GET /api/v1/ai/enrichment` |
| **Action** | Khởi động AI Enrichment service / kiểm tra deployment |

### Checklist

- [ ] Kiểm tra AI Enrichment service có đang chạy không
- [ ] Xem logs: `docker logs ai-enrichment-service`
- [ ] Kiểm tra dependencies: LLM API keys, embedding model endpoints
- [ ] Kiểm tra queue processor có chạy không
- [ ] Nếu chưa deploy: deploy AI Enrichment service

### Expected Response

```json
{
  "data": [
    {
      "cve_id": "CVE-2024-1234",
      "enrichment_status": "completed",
      "ai_summary": "Remote code execution vulnerability in...",
      "attack_vectors": ["network", "adjacent"],
      "remediation_steps": ["Update to version 2.1.0", "Apply patch KB12345"]
    }
  ]
}
```

---

## Ghi chú

- BUG-007 và BUG-008 có thể cùng do một AI service infrastructure bị down
- Nên kiểm tra và fix cả hai cùng lúc
