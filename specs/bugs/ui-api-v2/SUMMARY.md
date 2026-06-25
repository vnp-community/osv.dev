# UI Bugs — Do Backend Gây Ra (API v1 → v2 Migration)

**Nguồn**: Phân loại từ scan report [`/ui/specs/bugs/ui-crash/SUMMARY.md`](../../../ui/specs/bugs/ui-crash/SUMMARY.md)  
**Phân loại ngày**: 2026-06-20  
**Cập nhật**: 2026-06-22 — **✅ 15/15 bugs FIXED**  
**Tiêu chí**: Chỉ lấy bugs có **root cause từ backend** — bao gồm:
- API trả về HTTP 4xx/5xx
- API trả về sai data shape (null/undefined thay vì array/object)
- API endpoint chưa được implement hoặc deploy
- Service backend đang down (503)

---

## Tổng quan

| Tổng bugs gốc | Bugs do Backend | Bugs thuần Frontend | Bugs đã fix |
|---------------|-----------------|---------------------|-------------|
| 16 | **15** | 1 (BUG-013 RBAC pure frontend) | **✅ 15/15** |

---

## Phân loại theo loại lỗi backend

### 🔴 API 500 — Backend Server Error (1 bug)

| Bug ID | Page | Route | Endpoint | Priority | Status |
|--------|------|-------|----------|----------|--------|
| [BUG-003](BUG-003-findings.md) | Findings — All Findings | `/findings` | `GET /api/v1/findings` | P0 | ✅ Fixed — TASK-001 |

### 🔴 API 404 — Endpoint Chưa Implement / Chưa Deploy (8 bugs)

| Bug ID | Page | Route | Endpoint(s) | Priority | Status |
|--------|------|-------|-------------|----------|--------|
| [BUG-002](BUG-002-scans.md) | Scanning — Scan Dashboard | `/scans` | `GET /api/v1/scans/stats/weekly` | P2 | ✅ Fixed — TASK-008 |
| [BUG-004](BUG-004-findings-risk-acceptance.md) | Findings — Risk Acceptance | `/findings/risk-acceptance` | `GET /api/v1/risk-acceptances` | P2 | ✅ Fixed — TASK-009 |
| [BUG-009](BUG-009-reports.md) | Reports — Report Center | `/reports` | `GET /api/v1/reports`, `GET /api/v1/reports/templates` | P2 | ✅ Fixed — TASK-010 |
| [BUG-010](BUG-010-notifications.md) | Notifications | `/notifications` | `GET /api/v1/notifications` | P2 | ✅ Fixed — TASK-011 |
| [BUG-011](BUG-011-integrations-api-keys.md) | Integrations — API Keys | `/integrations/api-keys` | `GET /api/v1/api-keys` | P2 | ✅ Fixed — TASK-012 |
| [BUG-014](BUG-014-admin-audit.md) | Admin — Audit Logs | `/admin/audit` | `GET /api/v1/audit-log` | P2 | ✅ Fixed — TASK-013 |
| [BUG-016](BUG-016-admin-settings.md) | Admin — System Settings | `/admin/settings` | `GET /api/v1/admin/settings` | P2 | ✅ Fixed — TASK-014 |

### 🔵 API 503 — Service Unavailable (2 bugs)

| Bug ID | Page | Route | Endpoint | Priority | Status |
|--------|------|-------|----------|----------|--------|
| [BUG-007](BUG-007-ai-triage.md) | AI Center — AI Triage Queue | `/ai/triage` | `GET /api/v1/ai/triage/queue` | P1 | ✅ Fixed — TASK-007 |
| [BUG-008](BUG-008-ai-enrichment.md) | AI Center — AI Enrichment | `/ai/enrichment` | `GET /api/v1/ai/enrichment` | P1 | ✅ Fixed — TASK-007 |

### 🟠 API Contract Violation — Sai Data Shape (3 bugs)

> API trả về `null`/`undefined`/wrong shape khiến frontend crash khi xử lý data.  
> Root cause là backend không tuân thủ API contract — backend cần sửa để trả đúng schema.

| Bug ID | Page | Route | API Endpoint | Lỗi Frontend | Priority | Status |
|--------|------|-------|--------------|--------------|----------|--------|
| [BUG-001](BUG-001-cve-cwe.md) | CVE Intel — CWE Library | `/cve/cwe` | `GET /api/v1/cwe` | `TypeError: undefined.map` | P1 | ✅ Fixed — TASK-003 |
| [BUG-005](BUG-005-assets.md) | Assets — Asset Inventory | `/assets` | `GET /api/v1/assets` | `TypeError: null.filter` | P0 | ✅ Fixed — TASK-002 |
| [BUG-006](BUG-006-products.md) | Product Security | `/products` | `GET /api/v1/products` | `TypeError: undefined.flatMap` | P1 | ✅ Fixed — TASK-004 |

### 🟡 API Contract Violation — Webhooks & Health (2 bugs)

| Bug ID | Page | Route | Lỗi | Priority | Status |
|--------|------|-------|-----|----------|--------|
| [BUG-012](BUG-012-webhooks.md) | Integrations — Webhooks | `/integrations/webhooks` | `TypeError: undefined.find` | P1 | ✅ Fixed — TASK-005 |
| [BUG-015](BUG-015-admin-health.md) | Admin — System Health | `/admin/health` | `undefined.includes` | P1 | ✅ Fixed — TASK-006 |

---

## Bugs KHÔNG thuộc phạm vi (pure frontend)

| Bug ID | Page | Lý do loại |
|--------|------|------------|
| BUG-013 RBAC Management | `/admin/roles` | React Error #31 — frontend render object trực tiếp vào JSX, cần sửa component mapping |

---

## Danh sách endpoints đã fix

```
✅ Endpoints bị 404 — đã implement:
GET  /api/v1/scans/stats/weekly        # BUG-002 → TASK-008
GET  /api/v1/risk-acceptances          # BUG-004 → TASK-009
GET  /api/v1/reports                   # BUG-009 → TASK-010
GET  /api/v1/reports/templates         # BUG-009 → TASK-010
GET  /api/v1/notifications             # BUG-010 → TASK-011
GET  /api/v1/api-keys                  # BUG-011 → TASK-012
GET  /api/v1/audit-log                 # BUG-014 → TASK-013
GET  /api/v1/admin/settings            # BUG-016 → TASK-014

✅ Endpoints bị 500 — đã fix:
GET  /api/v1/findings                  # BUG-003 → TASK-001 (LEFT JOIN + nil guard)

✅ Endpoints bị 503 — service đã startup gracefully:
GET  /api/v1/ai/triage/queue           # BUG-007 → TASK-007 (P2-01 degraded mode)
GET  /api/v1/ai/enrichment             # BUG-008 → TASK-007 (P2-01 degraded mode)

✅ Endpoints trả sai data shape — đã fix:
GET  /api/v2/cwe                       # BUG-001 → TASK-003 (nil → make([]CWEItem, 0))
GET  /api/v1/assets                    # BUG-005 → TASK-002 (nil → make([]Asset, 0))
GET  /api/v1/products                  # BUG-006 → TASK-004 (tags nil guard + make[])
GET  /api/v1/webhooks                  # BUG-012 → TASK-005 (events nil → []string{})
GET  /api/v1/admin/health              # BUG-015 → TASK-006 (nil-safe NATS + always register)
```

---

## Kết quả tổng kết — Fix hoàn thành 2026-06-22

| Priority | Bug ID | Action | Task | Build |
|----------|--------|--------|------|-------|
| 🔴 P0 | BUG-003 | `JOIN → LEFT JOIN` + `*string` nil scan | TASK-001 | ✅ |
| 🔴 P0 | BUG-005 | `FindAll` nil → `make([]Asset, 0)` | TASK-002 | ✅ |
| 🟠 P1 | BUG-001 | `cwe_handler` nil → `make([]CWEItem, 0)` | TASK-003 | ✅ |
| 🟠 P1 | BUG-006 | Tags nil guard + make[] | TASK-004 | ✅ |
| 🟠 P1 | BUG-012 | Events nil → `make([]string, 0)` + append | TASK-005 | ✅ |
| 🟠 P1 | BUG-015 | Admin routes always registered; NATS nil guard | TASK-006 | ✅ |
| 🟠 P1 | BUG-007 | `log.Fatal → log.Warn`; P2-01 graceful degradation | TASK-007 | ✅ |
| 🟠 P1 | BUG-008 | (same as BUG-007) | TASK-007 | ✅ |
| 🟡 P2 | BUG-002 | `StatsHandler` + `/stats/weekly` route | TASK-008 | ✅ |
| 🟡 P2 | BUG-004 | `List/Get/Delete` + v1 compat routes | TASK-009 | ✅ |
| 🟡 P2 | BUG-009 | `/api/v1/reports/*` routes added | TASK-010 | ✅ |
| 🟡 P2 | BUG-010 | `/api/v1/notifications` reuse `AlertsHandler` | TASK-011 | ✅ |
| 🟡 P2 | BUG-011 | `GET /api/v1/auth/api-keys` verified | TASK-012 | ✅ |
| 🟡 P2 | BUG-014 | `/api/v1/audit-log` + `/api/v2/audit-log` routes | TASK-013 | ✅ |
| 🟡 P2 | BUG-016 | `SettingsRepo` fallback defaults; graceful no-op | TASK-014 | ✅ |
