# Solutions Index — Backend Bugs (ui-api-v2)

**Nguồn bugs**: [`specs/bugs/ui-api-v2/`](../)  
**Codebase**: Go Microservices, Clean Architecture  
**Gateway**: `apps/osv` (port 8080, Go 1.22 stdlib ServeMux)  
**Ngày tạo**: 2026-06-20

---

## Tóm tắt giải pháp

| Bug ID | Page | Loại lỗi | Service cần sửa | Solution File | Priority | Status |
|--------|------|-----------|-----------------|---------------|----------|--------|
| [BUG-001](../BUG-001-cve-cwe.md) | CWE Library | API trả `undefined` | `data-service:8082` | [SOL-001](SOL-001-cve-cwe.md) | P1 | ✅ Done |
| [BUG-002](../BUG-002-scans.md) | Scan Dashboard | API 404 | `scan-service:8084` | [SOL-002](SOL-002-scans-stats.md) | P2 | ✅ Done |
| [BUG-003](../BUG-003-findings.md) | All Findings | API 500 | `finding-service:8085` | [SOL-003](SOL-003-findings-500.md) | P0 | ✅ Done |
| [BUG-004](../BUG-004-findings-risk-acceptance.md) | Risk Acceptance | API 404 | `finding-service:8085` | [SOL-004](SOL-004-risk-acceptances.md) | P2 | ✅ Done |
| [BUG-005](../BUG-005-assets.md) | Asset Inventory | API trả `null` | `asset-service:8091` | [SOL-005](SOL-005-assets-null.md) | P0 | ✅ Done |
| [BUG-006](../BUG-006-products.md) | Product Security | Data shape sai | `finding-service:8085` | [SOL-006](SOL-006-products-shape.md) | P1 | ✅ Done |
| [BUG-007](../BUG-007-ai-triage.md) | AI Triage Queue | API 503 | `ai-service:9103` | [SOL-007](SOL-007-ai-services.md) | P1 | ✅ Done |
| [BUG-008](../BUG-008-ai-enrichment.md) | AI Enrichment | API 503 | `ai-service:9103` | [SOL-007](SOL-007-ai-services.md) | P1 | ✅ Done |
| [BUG-009](../BUG-009-reports.md) | Report Center | API 404 | `finding-service:8085` | [SOL-008](SOL-008-reports.md) | P2 | ✅ Done |
| [BUG-010](../BUG-010-notifications.md) | Notifications | API 404 | `notification-service:8087` | [SOL-009](SOL-009-notifications.md) | P2 | ✅ Done |
| [BUG-011](../BUG-011-integrations-api-keys.md) | API Keys | API 404 | `identity-service:8081` | [SOL-010](SOL-010-api-keys.md) | P2 | ✅ Done |
| [BUG-012](../BUG-012-webhooks.md) | Webhooks | Data shape sai | `notification-service:8087` | [SOL-011](SOL-011-webhooks.md) | P1 | ✅ Done |
| [BUG-014](../BUG-014-admin-audit.md) | Audit Logs | API 404 | `audit-service:8090` | [SOL-012](SOL-012-audit-log.md) | P2 | ✅ Done |
| [BUG-015](../BUG-015-admin-health.md) | System Health | Data shape sai | `apps/osv` (BFF) | [SOL-013](SOL-013-admin-health.md) | P1 | ✅ Done |
| [BUG-016](../BUG-016-admin-settings.md) | System Settings | API 404 | `apps/osv` (BFF) | [SOL-014](SOL-014-admin-settings.md) | P2 | ✅ Done |

---

## Phân nhóm theo service

### `apps/osv` (Gateway BFF)
- [SOL-013](SOL-013-admin-health.md) — Admin Health: fix response schema cho `services` map
- [SOL-014](SOL-014-admin-settings.md) — Admin Settings: BFF + Settings repo đã exist, cần đảm bảo DB migration chạy

### `finding-service:8085`
- [SOL-003](SOL-003-findings-500.md) — Findings 500: debug DB query JOIN với jira_issues/jira_configs
- [SOL-004](SOL-004-risk-acceptances.md) — Risk Acceptances: endpoint đã route nhưng handler chưa có List
- [SOL-006](SOL-006-products-shape.md) — Products: response thiếu nested array field `vulnerabilities`/`findings`
- [SOL-008](SOL-008-reports.md) — Reports: endpoint đã route, kiểm tra handler `GetTemplates` và `List`

### `scan-service:8084`
- [SOL-002](SOL-002-scans-stats.md) — Scan Stats: thêm handler `GET /api/v1/scans/stats/weekly`

### `asset-service:8091`
- [SOL-005](SOL-005-assets-null.md) — Assets: fix repo trả `null` thay vì `[]`

### `data-service:8082`
- [SOL-001](SOL-001-cve-cwe.md) — CWE: fix response trả `undefined` thay vì `[]`

### `notification-service:8087`
- [SOL-009](SOL-009-notifications.md) — Notifications: thêm handler `GET /api/v1/notifications`
- [SOL-011](SOL-011-webhooks.md) — Webhooks: fix response schema thiếu `event_types` field

### `identity-service:8081`
- [SOL-010](SOL-010-api-keys.md) — API Keys: route đã đúng (`/api/v1/api-keys` → `/api/v1/auth/api-keys`), kiểm tra identity-service handler

### `ai-service:9103`
- [SOL-007](SOL-007-ai-services.md) — AI Services: deployment/startup fix

### `audit-service:8090`
- [SOL-012](SOL-012-audit-log.md) — Audit Log: endpoint đã route, kiểm tra handler response

---

## Thứ tự fix theo priority

```
P0 (immediate):
  1. SOL-003 — Findings 500 (data loss)
  2. SOL-005 — Assets null (page unusable)

P1 (this sprint):
  3. SOL-001 — CWE undefined
  4. SOL-006 — Products shape
  5. SOL-007 — AI Services 503
  6. SOL-011 — Webhooks shape
  7. SOL-013 — Admin Health shape

P2 (next sprint):
  8. SOL-002 — Scans stats
  9. SOL-004 — Risk Acceptances
  10. SOL-008 — Reports
  11. SOL-009 — Notifications
  12. SOL-010 — API Keys
  13. SOL-012 — Audit Log
  14. SOL-014 — Admin Settings
```
---

## Kết quả triển khai

**✅ 15/15 solutions IMPLEMENTED** — Build verified 2026-06-22

| Solution | Task | Thay đổi chính | Service |
|----------|------|----------------|---------|
| SOL-001 | TASK-003 | `nil slice → make([]CWEItem, 0)` in `cwe_handler.go` | `data-service` |
| SOL-002 | TASK-008 | `StatsHandler` + `/stats/weekly` route; `NewRouterFull()` | `scan-service` |
| SOL-003 | TASK-001 | `JOIN → LEFT JOIN`, `*string` nil scan, logging | `finding-service` |
| SOL-004 | TASK-009 | `List/Get/Delete` handlers + v1 compat routes | `finding-service` |
| SOL-005 | TASK-002 | `FindAll` nil → `make([]Asset, 0)`, ListAssets nil guard | `asset-service` |
| SOL-006 | TASK-004 | Tags nil guard + `make([]string, 0)` in `product_handler.go` | `finding-service` |
| SOL-007 | TASK-007 | `log.Fatal → log.Warn` on startup; P2-01 graceful degradation | `ai-service` |
| SOL-008 | TASK-010 | `/api/v1/reports/*` v1 compat routes added to router | `finding-service` |
| SOL-009 | TASK-011 | `/api/v1/notifications` routes reuse `AlertsHandler` | `notification-service` |
| SOL-010 | TASK-012 | `GET /api/v1/auth/api-keys` đã có sẵn, verified | `identity-service` |
| SOL-011 | TASK-005 | `events` nil → `make([]EventType, 0)` + append | `notification-service` |
| SOL-012 | TASK-013 | `/api/v1/audit-log` + `/api/v2/audit-log` routes in router | `audit-service` |
| SOL-013 | TASK-006 | Admin routes always registered; `natsConn` nil guard | `apps/osv` |
| SOL-014 | TASK-014 | `SettingsRepo.Get()` fallback defaults; Patch graceful no-op | `apps/osv` |
