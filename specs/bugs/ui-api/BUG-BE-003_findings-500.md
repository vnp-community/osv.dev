# BUG-BE-003 — Findings Endpoints Trả 500 / 400

| Trường | Giá trị |
|---|---|
| **ID** | BUG-BE-003 |
| **Severity** | 🔴 Critical |
| **Priority** | P0 |
| **Component** | Backend / API Gateway / Finding Service |
| **Endpoints** | `GET /api/v1/findings`, `GET /api/v1/findings/stats` |
| **Phát hiện** | 2026-06-19 |
| **Server** | https://c12.openledger.vn |
| **Status** | Open |

---

## Mô tả

`GET /api/v1/findings` trả về HTTP 500 với message `"failed to list findings"`. `GET /api/v1/findings/stats` trả về HTTP 400. Chức năng Findings hoàn toàn không hoạt động.

## Tái hiện

```bash
# Liệt kê findings
curl -H "Authorization: Bearer <token>" \
  "https://c12.openledger.vn/api/v1/findings?page=1&pageSize=10"
# => HTTP 500
# {"error":"failed to list findings"}

# Filter theo status
curl -H "Authorization: Bearer <token>" \
  "https://c12.openledger.vn/api/v1/findings?status=active&pageSize=10"
# => HTTP 500

# Stats endpoint
curl -H "Authorization: Bearer <token>" \
  https://c12.openledger.vn/api/v1/findings/stats
# => HTTP 400
```

## Expected Response (theo spec)

```json
GET /api/v1/findings → HTTP 200
{
  "findings": [...],
  "total": 0,
  "page": 1,
  "page_size": 10,
  "by_severity": { "Critical": 0, "High": 0, "Medium": 0, "Low": 0 },
  "by_status": { "active": 0, "mitigated": 0, ... },
  "sla_stats": {
    "breached": 0,
    "at_risk": 0,
    "ok": 0
  }
}
```

## Root Cause Analysis

Lỗi `"failed to list findings"` gợi ý:
1. **DB query thất bại** — có thể do:
   - Table `findings` chưa được migrate
   - Foreign key join với `scans` hoặc `engagements` bị lỗi vì scan data rỗng
   - SQL syntax error trong query builder
2. **`/findings/stats` trả 400** — có thể do:
   - Required query param bị thiếu (endpoint yêu cầu `product_id` hoặc `engagement_id` bắt buộc)
   - Request validation reject

## Kiểm tra nhanh

```bash
# Kiểm tra migration đã chạy chưa
docker exec osv-backend-postgres-1 psql -U osv -d osv -c "\dt"
# Kiểm tra bảng findings có tồn tại không

# Xem logs finding handler
docker logs osv-backend-gateway-1 --tail 100 | grep -i "finding\|error"
```

## Fix

1. Chạy database migration đầy đủ nếu chưa
2. Debug SQL query trong findings list handler — xác nhận JOIN conditions
3. Với `/findings/stats`: kiểm tra xem endpoint có require param bắt buộc không — nếu có, cần document hoặc làm optional

## Ảnh hưởng

- Trang Findings không load
- Dashboard `sla_breaches` và `recent_findings` bị ảnh hưởng
- SLA tracking không hoạt động
