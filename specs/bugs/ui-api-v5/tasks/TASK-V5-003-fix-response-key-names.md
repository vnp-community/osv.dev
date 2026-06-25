# TASK-V5-003: Sửa `scans/scheduled` và `risk-acceptances` Key Names

## Mô tả
Hai endpoints trả về JSON với key names sai so với OpenAPI spec.

## Bug 1: `GET /api/v1/scans/scheduled`
### Hiện tại
```json
{ "scheduled": [], "total": 0 }
```
### Spec yêu cầu
```json
{ "scheduled_scans": [], "total": 0 }
```
### Sửa tại
`services/scan-service/internal/delivery/http/router.go` — hàm `ListScheduled()`:
```go
// Đổi key "scheduled" → "scheduled_scans"
writeScanJSON(w, http.StatusOK, map[string]interface{}{
    "scheduled_scans": []interface{}{},
    "total":           0,
})
```

## Bug 2: `GET /api/v1/risk-acceptances`
### Hiện tại
Response thiếu wrapper key `risk_acceptances`

### Spec yêu cầu
```json
{ "risk_acceptances": [], "total": 0 }
```
### Sửa tại
Handler risk-acceptances trong `services/finding-service/`

## Acceptance Criteria
- [ ] `GET /api/v1/scans/scheduled` → `{ scheduled_scans: [], total: 0 }`
- [ ] `GET /api/v1/risk-acceptances` → `{ risk_acceptances: [...], total: N }`
- [ ] Tests `scans_scheduled_schema`, `scans_scheduled_list_is_array`, `risk_acceptances_schema` → PASS

## Status: TODO
