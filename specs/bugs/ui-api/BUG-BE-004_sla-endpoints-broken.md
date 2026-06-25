# BUG-BE-004 — SLA Endpoints Không Hoạt Động (404 / 500)

| Trường | Giá trị |
|---|---|
| **ID** | BUG-BE-004 |
| **Severity** | 🔴 Critical |
| **Priority** | P0 |
| **Component** | Backend / API Gateway / SLA Service |
| **Endpoints** | `GET /api/v1/sla/overview`, `GET /api/v1/sla/config`, `GET /api/v1/dashboard/sla` |
| **Phát hiện** | 2026-06-19 |
| **Server** | https://c12.openledger.vn |
| **Status** | Open |

---

## Mô tả

Toàn bộ SLA-related endpoints đều bị lỗi:
- `GET /api/v1/sla/overview` → **404** (route chưa register)
- `GET /api/v1/sla/config` → **200 nhưng schema sai** (thiếu `global`, `product_overrides`)
- `GET /api/v1/dashboard/sla` → **500** `"Failed to parse SLA response"`

## Tái hiện

```bash
# 1. SLA Overview
curl -H "Authorization: Bearer <token>" \
  https://c12.openledger.vn/api/v1/sla/overview
# => HTTP 404 — "404 page not found"

# 2. SLA Config
curl -H "Authorization: Bearer <token>" \
  https://c12.openledger.vn/api/v1/sla/config
# => HTTP 200 nhưng response thiếu required fields

# 3. Dashboard SLA
curl -H "Authorization: Bearer <token>" \
  https://c12.openledger.vn/api/v1/dashboard/sla
# => HTTP 500
# {"error":"INTERNAL_ERROR","message":"Failed to parse SLA response"}
```

## Expected vs Actual

### `GET /api/v1/sla/config`

**Expected (spec):**
```json
{
  "global": {
    "critical_days": 7,
    "high_days": 30,
    "medium_days": 90,
    "low_days": 180
  },
  "product_overrides": []
}
```
**Actual:** Response không có `global` và `product_overrides` — có thể trả về flat object hoặc wrapper khác.

### `GET /api/v1/dashboard/sla`

**Expected (spec):**
```json
{
  "summary": {
    "total_active_findings": 0,
    "compliance_percent": 100.0,
    "breached": 0,
    "at_risk": 0,
    "ok": 0
  },
  "compliance_trend": [...],
  "breached_findings": [...],
  "at_risk_findings": [...],
  "by_product": [...],
  "total_breached": 0,
  "total_at_risk": 0,
  "page": 1,
  "page_size": 20
}
```
**Actual:** HTTP 500 — `"Failed to parse SLA response"` — gateway không parse được response từ SLA service.

## Root Cause Analysis

Lỗi `"Failed to parse SLA response"` trong `dashboard/sla` cho thấy:
1. Gateway gọi SLA service thành công nhưng **response format của SLA service không khớp** với struct gateway dùng để decode
2. Hoặc SLA service trả về lỗi và gateway không handle gracefully

Lỗi 404 `sla/overview` → route `/api/v1/sla/overview` chưa được mount trong gateway router.

## Fix

1. **`dashboard/sla`**: Debug gateway SLA handler — log raw response từ SLA service trước khi parse
2. **`sla/overview`**: Thêm route handler vào gateway router
3. **`sla/config`**: Đồng bộ response struct với spec — wrap trong `{ "global": {...}, "product_overrides": [...] }`

## Ảnh hưởng

- Trang Dashboard SLA (`/dashboard/sla`) không load
- SLA compliance tracking không hoạt động
- Risk management không có dữ liệu SLA
