# BUG-007: Risk Acceptances — List và Create Chưa Implement

**ID**: BUG-007  
**Domain**: Risk Management  
**Mức độ**: 🔴 HIGH  
**Loại**: `404 Not Found`  
**Phát hiện**: 2026-06-23  
**Trạng thái**: OPEN  

## Endpoints Bị Lỗi

| Method | Endpoint | HTTP Status |
|---|---|---|
| `GET`  | `/api/v1/risk-acceptances` | **404** |
| `POST` | `/api/v1/risk-acceptances` | **404** |

## Endpoint Đang Hoạt Động (Bất Thường)

| Method | Endpoint | Status |
|---|---|---|
| `DELETE` | `/api/v1/risk-acceptances/{id}` | ✅ pass (404 vì test ID) |

> ⚠️ **Paradox**: DELETE/{id} route tồn tại nhưng GET list và POST create chưa implement. Không thể xóa thứ chưa tạo được.

## Mô Tả

`RiskAcceptanceCenter.tsx` không thể load danh sách các risk acceptance hoặc tạo mới. Tính năng Risk Acceptance — cho phép CISO chấp nhận một vulnerability không fix — hoàn toàn bị vô hiệu.

## Giải Pháp Đề Xuất

```
GET /api/v1/risk-acceptances?status=active&page=1&limit=20
→ 200 OK
{
  "items": [{
    "id": "...",
    "finding_id": "...",
    "cve_id": "CVE-...",
    "reason": "...",
    "accepted_by": { "name": "..." },
    "expires_at": "2026-12-31",
    "status": "active"
  }],
  "total": 12
}

POST /api/v1/risk-acceptances
Body: {
  "finding_id": "...",
  "reason": "Business justification...",
  "expires_at": "2026-12-31"
}
→ 201 Created { risk_acceptance }
```

## File UI Bị Ảnh Hưởng

- `src/features/findings/components/RiskAcceptanceCenter.tsx`
- `src/features/findings/hooks/useRiskAcceptances.ts`
