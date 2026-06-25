# SOL-V5-002: Sửa KEV Stats Response Schema

## Vấn đề
`GET /api/v2/kev/stats` trả về 200 nhưng thiếu 2 fields bắt buộc theo OpenAPI spec:
- `by_vendor`: Object map vendor → count
- `recent_additions`: Array các CVE được thêm vào KEV gần đây

Test pass riêng lẻ `by_vendor` và `recent_additions` là array, nhưng schema validation tổng thể fail vì field names khác.

## Root Cause
Handler trả về response với key names không đúng so với test expectations.
Test kiểm tra: `body["by_vendor"]` và `body["recent_additions"]`

## Files cần thay đổi
- `services/data-service/internal/delivery/http/kev_handler.go` (hoặc tương đương)
- Đảm bảo response JSON chứa `by_vendor` và `recent_additions` ở top-level

## Cách kiểm tra
- `GET /api/v2/kev/stats` → 200 với `{ by_vendor: {}, recent_additions: [], ... }`
