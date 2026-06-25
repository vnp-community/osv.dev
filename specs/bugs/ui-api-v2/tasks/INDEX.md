# Tasks — Backend Bug Fixes (ui-api-v2)

**Nguồn**: Solutions từ [`specs/bugs/ui-api-v2/solutions/`](../solutions/)  
**Tổng tasks**: 14  
**Thứ tự thực thi**: theo Priority (P0 → P1 → P2)

---

## Nhóm A — P0: Nghẽn hoàn toàn (fix ngay)

| Task | Service | Bug | Thay đổi | Effort | Status |
|------|---------|-----|----------|--------|--------|
| [TASK-001](TASK-001-findings-500-join.md) | `finding-service` | BUG-003: Findings API 500 | `JOIN` → `LEFT JOIN` + nil guard | ~15 min | ✅ Done |
| [TASK-002](TASK-002-assets-null-slice.md) | `asset-service` | BUG-005: Assets `null.filter` | `var` → `make([]T, 0)` | ~10 min | ✅ Done |

## Nhóm B — P1: Page crash (fix trong sprint)

| Task | Service | Bug | Thay đổi | Effort | Status |
|------|---------|-----|----------|--------|--------|
| [TASK-003](TASK-003-cwe-null-slice.md) | `data-service` | BUG-001: CWE `undefined.map` | `var` → `make([]T, 0)` | ~10 min | ✅ Done |
| [TASK-004](TASK-004-products-response-shape.md) | `finding-service` | BUG-006: Products thiếu `finding_counts` | Tags nil guard + make[] | ~20 min | ✅ Done |
| [TASK-005](TASK-005-webhooks-null-event-types.md) | `notification-service` | BUG-012: Webhooks `event_types: null` | nil → `[]string{}` | ~10 min | ✅ Done |
| [TASK-006](TASK-006-admin-health-route-guard.md) | `apps/osv` BFF | BUG-015: Health route bị skip | Remove nil guard, nil-safe NATS+Settings | ~10 min | ✅ Done |
| [TASK-007](TASK-007-ai-service-startup.md) | `ai-service` + docker | BUG-007/008: AI 503 | Fatal→Warn startup + P2-01 degraded | ~20 min | ✅ Done |

## Nhóm C — P2: Endpoint 404 (implement handlers)

| Task | Service | Bug | Thay đổi | Effort | Status |
|------|---------|-----|----------|--------|--------|
| [TASK-008](TASK-008-scans-stats-handler.md) | `scan-service` | BUG-002: Scan Stats 404 | Implement `StatsHandler` + SQL | ~30 min | ✅ Done |
| [TASK-009](TASK-009-risk-acceptance-list.md) | `finding-service` | BUG-004: Risk Acceptances 404 | Implement `List` + register v1 route | ~20 min | ✅ Done |
| [TASK-010](TASK-010-reports-v1-routes.md) | `finding-service` | BUG-009: Reports 404 | Thêm `/api/v1/reports/*` routes | ~15 min | ✅ Done |
| [TASK-011](TASK-011-notifications-list.md) | `notification-service` | BUG-010: Notifications 404 | Implement `List` handler | ~20 min | ✅ Done |
| [TASK-012](TASK-012-api-keys-rewrite.md) | `identity-service` | BUG-011: API Keys 404 | Verify ForwardRewrite + implement handler | ~20 min | ✅ Done |
| [TASK-013](TASK-013-audit-log-handler.md) | `audit-service` | BUG-014: Audit Log 404 | Implement `List` handler + SQL | ~25 min | ✅ Done |
| [TASK-014](TASK-014-admin-settings-migration.md) | `apps/osv` BFF | BUG-016: Settings 404 | DB migration + SettingsRepo | ~30 min | ✅ Done |

---

## Thứ tự thực thi

```
P0 — Fix ngay (unblocks UI):
  1. TASK-001 → finding-service: LEFT JOIN fix
  2. TASK-002 → asset-service: nil → []

P1 — Sprint hiện tại:
  3. TASK-006 → apps/osv: health route guard
  4. TASK-003 → data-service: CWE nil slice
  5. TASK-004 → finding-service: products shape
  6. TASK-005 → notification-service: webhooks nil
  7. TASK-007 → ai-service: startup + defensive

P2 — Sprint tiếp:
  8.  TASK-008 → scan-service: stats handler
  9.  TASK-009 → finding-service: risk-acceptance list
  10. TASK-010 → finding-service: reports v1 routes
  11. TASK-011 → notification-service: notifications list
  12. TASK-012 → identity-service: api-keys
  13. TASK-013 → audit-service: audit log handler
  14. TASK-014 → apps/osv: settings migration
```

---

## Kết quả tổng kết

**✅ 14/14 tasks DONE** — Build verified 2026-06-22

| Service | Build | Tasks |
|---------|-------|-------|
| `finding-service` | ✅ | TASK-001, 004, 009, 010 |
| `asset-service` | ✅ | TASK-002 |
| `data-service` | ✅ | TASK-003 |
| `notification-service` | ✅ | TASK-005, 011 |
| `apps/osv` | ✅ | TASK-006, 014 |
| `ai-service` (cmd) | ✅ | TASK-007 |
| `scan-service` | ✅ | TASK-008 |
| `identity-service` | ✅ (pre-existing) | TASK-012 |
| `audit-service` (http) | ✅ | TASK-013 |
