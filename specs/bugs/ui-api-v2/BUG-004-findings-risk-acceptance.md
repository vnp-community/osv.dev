# BUG-004 — Findings: Risk Acceptance Center (API 404)

**Nguồn gốc**: [`ui-crash/BUG-findings-risk-acceptance.md`](../../../ui/specs/bugs/ui-crash/BUG-findings-risk-acceptance.md)  
**Loại**: 🔴 API 404 — Endpoint chưa implement  
**Priority**: P2

**Status**: `✅ Fixed` — TASK-009 / SOL-004 (fixed 2026-06-20)

---

## Thông tin

| Field | Value |
|-------|-------|
| **Route** | `/findings/risk-acceptance` |
| **HTTP Status** | `404 Not Found` |
| **Endpoint bị lỗi** | `GET /api/v1/risk-acceptances` |
| **Số lần xuất hiện** | 3 lần |

---

## Root Cause (Backend)

Endpoint `GET /api/v1/risk-acceptances` chưa được implement. Risk Acceptance là tính năng cho phép security team đánh dấu một finding là "chấp nhận rủi ro" — backend chưa có API này.

**Error log**:
```
Failed to load resource: the server responded with a status of 404 ()
URL: https://c12.openledger.vn/api/v1/risk-acceptances
```

---

## Backend Fix Required

| Item | Detail |
|------|--------|
| **Endpoint** | `GET /api/v1/risk-acceptances` |
| **Action** | Implement endpoint, model, và database table |

### Cần implement

```
GET    /api/v1/risk-acceptances          # List all risk acceptances
POST   /api/v1/risk-acceptances          # Create new risk acceptance
GET    /api/v1/risk-acceptances/:id      # Get single
PUT    /api/v1/risk-acceptances/:id      # Update
DELETE /api/v1/risk-acceptances/:id      # Delete / revoke
```

### Expected Response Schema (GET list)

```json
{
  "data": [
    {
      "id": "ra-001",
      "finding_id": "finding-123",
      "reason": "Business decision - low exploitability",
      "accepted_by": "user@example.com",
      "expires_at": "2026-12-31T00:00:00Z",
      "status": "active"
    }
  ],
  "pagination": { "page": 1, "pageSize": 20, "total": 5 }
}
```
