# SOL-V5-003: Sửa Các Response Schema Mismatch

## Vấn đề
Nhiều endpoints trả về response JSON với field names khác hoặc wrapper object không đúng:

### BUG-V5-005: `GET /api/v1/scans/scheduled`
- **Thực tế:** `{ "scheduled": [], "total": 0 }`
- **Spec:** `{ "scheduled_scans": [], "total": 0 }`
- **Sửa:** Đổi key `"scheduled"` → `"scheduled_scans"` trong `ScanAPIHandler.ListScheduled()`

### BUG-V5-006: `GET /api/v1/risk-acceptances`
- **Thực tế:** `{ "items": [], "total": 0 }` (hoặc thiếu wrapper)
- **Spec:** `{ "risk_acceptances": [], "total": 0 }`
- **Sửa:** Đổi key trong handler risk-acceptances của finding-service

### BUG-V5-007: `GET /api/v1/reports` → 500
- **Root Cause:** `ListByProduct()` query `WHERE product_id = $1` với empty string → SQL error (UUID parse)
- **Sửa:** Handler cần không truyền `product_id` filter khi không có giá trị; hoặc dùng `generated_by` = user_id khi không có `product_id`

### BUG-V5-008: `POST /api/v1/reports` — Thiếu fields
- **Thực tế:** Response thiếu `name`, `type`, `created_by`
- **Spec cần:** `{ name, type, status, created_by, ... }`
- **Sửa:** Map `title` → `name`, `format` → `type`, `generated_by` → `created_by` trong Create response handler

### BUG-V5-009: `GET /api/v1/api-keys` — Trả Dict thay vì Array
- **Thực tế:** `{ "keys": [], "total": 0 }`
- **Spec:** `[]` (direct array)
- **Sửa:** `ListAPIKeys()` trong `api_key_handler.go` → trả `items` array thay vì dict wrapper; hoặc test client cần extract `.keys`

## Files cần thay đổi
- `services/scan-service/internal/delivery/http/router.go` — `ListScheduled()` key name
- `services/finding-service/internal/delivery/http/risk_acceptance_handler.go` — key name
- `services/finding-service/internal/delivery/http/report_handler.go` — `List()` + `Create()` response fields
- `services/identity-service/adapter/handler/http/api_key_handler.go` — `ListAPIKeys()` response format
