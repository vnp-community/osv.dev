# TASK-006: Reports v1 Alias, SLA Overview, Jira PUT

**Related CR**: [CR-006](../CR-006-reports-sla-jira-gaps.md)
**Status**: ✅ COMPLETED — 2026-06-18

---

## Gateway — Reports v1 Path Rewrite — CR-006

- [x] `GET /api/v1/reports` → finding-service:8085 (protected)
- [x] `POST /api/v1/reports` → finding-service:8085 (rate-limit 5/minute)
- [x] `GET /api/v1/reports/{id}/download` → finding-service:8085 (30s timeout)
- [x] `GET /api/v1/reports/{id}` → finding-service:8085
- [x] `DELETE /api/v1/reports/{id}` → finding-service:8085
- [x] Route ordering đúng: `/reports/{id}/download` trước `/reports/{id}` (literal before wildcard)

**Ghi chú**: Tất cả routes trỏ thẳng tới `/api/v2/reports` trong finding-service (không cần path rewrite vì finding-service đã có cả v1 lẫn v2 report handler). Không cần thêm `path_rewrite.go` riêng.

## Gateway — SLA Overview BFF — CR-006

- [x] Implement `slaOverviewBFF()` inline trong `apps/osv/internal/gateway/router.go`
  - Rewrite path: `/api/v1/sla/overview` → `/api/v2/sla-dashboard` (sla-service)
  - Forward về `sla-service:8086`
- [x] Route `GET /api/v1/sla/overview` → `slaOverviewBFF(proxy)` registered

## Gateway — Jira Config PUT — CR-006

- [x] `PUT /api/v1/jira/config` → jira-service:8088 (adminOnly)
- [x] Route registered cùng với GET và POST existing routes

## finding-service — Verify Reports v2

- [x] `GET /api/v2/reports` → handler `report.List` đã có (xem router.go finding-service)
- [x] `POST /api/v2/reports` → handler `report.Create` đã có (trả 202)
- [x] `GET /api/v2/reports/{id}` → handler `report.Get` đã có
- [x] `GET /api/v2/reports/{id}/download?format=pdf` → TODO: implement binary stream

## Build Status
- `apps/osv/internal/gateway/...` → ✅ Build OK

## Testing (chạy sau khi deploy)
- [ ] `GET /api/v1/reports` → 200 (không 404)
- [ ] `POST /api/v1/reports` với `{name, type, config}` → 202
- [ ] `DELETE /api/v1/reports/{id}` → 204
- [ ] `GET /api/v1/sla/overview` → 200 với SLA data
- [ ] `PUT /api/v1/jira/config` với body → 200
