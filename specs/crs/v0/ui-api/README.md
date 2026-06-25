# UI-API Change Requests — Index

**Series:** UI-API v2  
**Ngày tạo:** 2026-06-16  
**Cập nhật:** 2026-06-17  
**Trạng thái:** 🟡 UI Layer Done / Backend Integration Pending (~72 endpoints spec'd)  
**Tác giả:** Phân tích từ PRD v3.0, SRS v3.0, URD v3.0, UI Architecture v1.1, TDD v1.0

---

## Tổng quan

Bộ **10 Change Requests** này định nghĩa toàn bộ API mà backend (Go Microservices) phải cung cấp để frontend (React SPA) có thể hoạt động hoàn chỉnh theo nguyên tắc **API-First** — không hardcode data.

**Tổng số endpoints:** ~70 REST + 2 SSE streams

---

## Danh sách Change Requests

| CR | Tên | Ưu tiên | Services | Trạng thái UI | Trạng thái Backend |
|----|-----|---------|----------|---------------|-----------------|
| [CR-UI-001](./CR-UI-001-auth-api.md) | Authentication & User API | P0 Critical | identity-service | ✅ Done | ⚠️ Pending (auth endpoints mới) |
| [CR-UI-002](./CR-UI-002-dashboard-api.md) | Dashboard & KPI API | P0 Critical | finding-service, sla-service, data-service | ✅ Done | ⚠️ Pending (BFF aggregate endpoint) |
| [CR-UI-003](./CR-UI-003-cve-intel-api.md) | CVE Intelligence API | P0 Critical | data-service, search-service | ✅ Done | ⚠️ Schema update needed |
| [CR-UI-004](./CR-UI-004-scanning-api.md) | Active Scanning API | P1 High | scan-service (ext), asset-service | ✅ Done | 🔵 v3.0 Planned (CR-OVS-001) |
| [CR-UI-005](./CR-UI-005-finding-api.md) | Finding Management API | P0 Critical | finding-service, sla-service, audit-service | ✅ Done | ⚠️ Schema + new endpoints pending |
| [CR-UI-006](./CR-UI-006-asset-api.md) | Asset Management API | P1 High | asset-service (new) | ✅ Done | 🔵 v3.0 Planned (CR-OVS-007) |
| [CR-UI-007](./CR-UI-007-product-api.md) | Product Security API | P0 Critical | finding-service | ✅ Done | ⚠️ Schema + grades endpoint pending |
| [CR-UI-008](./CR-UI-008-ai-center-api.md) | AI Center API | P1 High | ai-service | ✅ Done | 🔵 v3.0 Planned (CR-OVS-005) |
| [CR-UI-009](./CR-UI-009-reports-notifications-api.md) | Reports & Notifications API | P0 Critical | finding-service, notification-service | ✅ Done | ⚠️ New endpoints pending |
| [CR-UI-010](./CR-UI-010-admin-integrations-api.md) | Administration & Integrations API | P0 Critical | identity-service, jira-service, audit-service | ✅ Done | ⚠️ New endpoints pending |

## Tiến độ triển khai (Cập nhật 2026-06-17)

| Layer | Trạng thái | Chi tiết |
|-------|-----------|-----------|
| **UI Components** | ✅ 100% (39 screens) | Tất cả screens đã build trong `ui/src/app/components/` |
| **Mock Handlers (MSW)** | ✅ 100% | `auth`, `cve`, `dashboard`, `finding`, `scan`, `asset`, `kev` handlers |
| **Backend v2.2 (existing endpoints)** | ⚠️ ~60% | Các endpoint cũ cần schema update (6 endpoints) |
| **Backend v2.2 (new endpoints)** | ⛔ ~0% | ~26 endpoints mới chưa implement |
| **Backend v3.0 (planned)** | 🔵 Planned | Scanning SSE, Asset auto-reg, AI triage, MFA/OAuth2 |

**Legend:** ✅ Done │ ⚠️ Partial/Schema update needed │ ⛔ Not started │ 🔵 v3.0 Planned

---

## Trạng thái Backend vs. UI Requirement

### ✅ Đã Implemented (cần schema/endpoint adjustment)

| Endpoint | CR | Vấn đề |
|----------|----|----|
| `POST /api/v2/cves/search` | CR-UI-003 | Thiếu fields: `is_kev`, `has_exploit`, `epss_percentile`, `sources[]`, `aggregations` trong response |
| `GET /api/v2/cves/{id}` | CR-UI-003 | Thiếu: `affected_products[]`, `kev_detail`, `references[]` |
| `GET /api/v2/kev` | CR-UI-003 | Thiếu `stats.unmitigated_in_platform` (cần join finding-service) |
| `GET /api/v1/findings` | CR-UI-005 | Thiếu fields: `is_kev`, `epss_score`, `jira_*`, `sla_days_left`; thiếu `by_severity`, `by_status`, `sla_stats` trong response |
| `PATCH /api/v1/findings/{id}` | CR-UI-005 | Cần validate state machine transitions, return proper 409 |
| `GET /api/v1/products` | CR-UI-007 | Thiếu `grade`, `score`, `finding_summary` per product |

### ❌ Thiếu hoàn toàn (New Endpoints)

| Endpoint | CR | Service cần thêm |
|----------|----|----|
| `GET /api/v1/dashboard` | CR-UI-002 | Gateway BFF aggregate |
| `GET /api/v1/dashboard/sla` | CR-UI-002 | sla-service |
| `GET /api/v1/notifications/stream` (SSE) | CR-UI-002 | notification-service |
| `POST /api/v1/auth/login` | CR-UI-001 | identity-service |
| `POST /api/v1/auth/refresh` | CR-UI-001 | identity-service |
| `GET /api/v1/auth/me` | CR-UI-001 | identity-service |
| `POST /api/v1/auth/logout` | CR-UI-001 | identity-service |
| `GET /api/v2/vendors` (autocomplete) | CR-UI-003 | data-service |
| `GET /api/v2/epss/top` | CR-UI-003 | data-service |
| `GET /api/v2/epss/distribution` | CR-UI-003 | data-service |
| `GET /api/v2/cwe` (list) | CR-UI-003 | data-service |
| `POST /api/v1/findings/bulk/reopen` | CR-UI-005 | finding-service |
| `POST /api/v1/findings/bulk/assign` | CR-UI-005 | finding-service |
| `POST /api/v1/findings/{id}/notes` | CR-UI-005 | finding-service |
| `GET /api/v1/findings/stats` | CR-UI-005 | finding-service |
| `GET /api/v1/products/grades` | CR-UI-007 | finding-service |
| `GET /api/v1/reports/{id}/download` | CR-UI-009 | finding-service |
| `GET /api/v1/notifications` | CR-UI-009 | notification-service |
| `PATCH /api/v1/notifications/{id}/read` | CR-UI-009 | notification-service |
| `POST /api/v1/notifications/mark-all-read` | CR-UI-009 | notification-service |
| `POST /api/v1/webhooks/{id}/test` | CR-UI-009 | notification-service |
| `GET /api/v1/admin/users` | CR-UI-010 | identity-service |
| `POST /api/v1/admin/users/invite` | CR-UI-010 | identity-service |
| `PATCH /api/v1/admin/users/{id}` | CR-UI-010 | identity-service |
| `GET /api/v1/admin/health` | CR-UI-010 | gateway (fan-out) |
| `GET /api/v1/admin/settings` | CR-UI-010 | gateway |
| `PATCH /api/v1/admin/settings` | CR-UI-010 | gateway |

### 🔵 Planned v3.0 (phụ thuộc CRs khác)

| Endpoint | CR-UI | Phụ thuộc |
|----------|-------|-----------|
| `POST /api/v1/scans` (Nmap/ZAP) | CR-UI-004 | CR-OVS-001 |
| `GET /api/v1/scans/{id}/stream` (SSE) | CR-UI-004 | CR-OVS-001 |
| `GET /api/v1/assets` | CR-UI-006 | CR-OVS-007 |
| `POST /api/v1/ai/triage/{findingId}` | CR-UI-008 | CR-OVS-005 |
| `GET /api/v1/auth/mfa/setup` | CR-UI-001 | CR-OVS-003 |
| OAuth2 endpoints | CR-UI-001 | CR-OVS-003 |

---

## API Endpoint Map tổng hợp

### v1 Endpoints (Unified Gateway → Backend Services)

```
Authentication (identity-service)
  POST   /api/v1/auth/login
  POST   /api/v1/auth/refresh
  GET    /api/v1/auth/me
  POST   /api/v1/auth/logout
  GET    /api/v1/auth/mfa/setup          [v3.0]
  POST   /api/v1/auth/mfa/confirm        [v3.0]
  GET    /api/v1/auth/oauth/google       [v3.0]
  GET    /api/v1/auth/oauth/github       [v3.0]
  GET    /api/v1/auth/callback           [v3.0]

Dashboard (BFF aggregate)
  GET    /api/v1/dashboard
  GET    /api/v1/dashboard/sla
  GET    /api/v1/notifications/stream    [SSE]

Scans (scan-service)
  GET    /api/v1/scans
  POST   /api/v1/scans
  GET    /api/v1/scans/{id}
  GET    /api/v1/scans/{id}/stream       [SSE, v3.0]
  POST   /api/v1/scans/{id}/cancel
  GET    /api/v1/scans/{id}/results/nmap [v3.0]
  GET    /api/v1/scans/{id}/results/zap  [v3.0]
  GET    /api/v1/scans/scheduled
  POST   /api/v1/scans/import

Findings (finding-service)
  GET    /api/v1/findings
  GET    /api/v1/findings/stats
  GET    /api/v1/findings/{id}
  PATCH  /api/v1/findings/{id}
  POST   /api/v1/findings/{id}/notes
  GET    /api/v1/findings/{id}/audit
  POST   /api/v1/findings/bulk/close
  POST   /api/v1/findings/bulk/reopen
  POST   /api/v1/findings/bulk/assign
  GET    /api/v1/risk-acceptances
  POST   /api/v1/risk-acceptances
  DELETE /api/v1/risk-acceptances/{id}

SLA (sla-service)
  GET    /api/v1/sla/config
  PUT    /api/v1/sla/config

Assets (asset-service) [v3.0]
  GET    /api/v1/assets
  GET    /api/v1/assets/{id}
  GET    /api/v1/assets/{id}/findings
  PATCH  /api/v1/assets/{id}
  GET    /api/v1/assets/tags

Products (finding-service)
  GET    /api/v1/products
  POST   /api/v1/products
  GET    /api/v1/products/{id}
  PATCH  /api/v1/products/{id}
  GET    /api/v1/products/{id}/engagements
  POST   /api/v1/products/{id}/engagements
  GET    /api/v1/products/grades
  GET    /api/v1/products/types
  GET    /api/v1/engagements/{id}/tests

AI (ai-service) [v3.0]
  POST   /api/v1/ai/triage/{findingId}
  POST   /api/v1/ai/triage/{findingId}/review
  GET    /api/v1/ai/triage/queue
  GET    /api/v1/ai/enrichment
  POST   /api/v1/ai/enrichment/trigger
  GET    /api/v1/ai/enrichment/{cveId}

Reports (finding-service)
  GET    /api/v1/reports
  POST   /api/v1/reports
  GET    /api/v1/reports/{id}
  GET    /api/v1/reports/{id}/download
  DELETE /api/v1/reports/{id}

Notifications (notification-service)
  GET    /api/v1/notifications
  PATCH  /api/v1/notifications/{id}/read
  POST   /api/v1/notifications/mark-all-read
  GET    /api/v1/notifications/unread-count
  GET    /api/v1/webhooks
  POST   /api/v1/webhooks
  DELETE /api/v1/webhooks/{id}
  POST   /api/v1/webhooks/{id}/test

API Keys (identity-service)
  GET    /api/v1/api-keys
  POST   /api/v1/api-keys
  DELETE /api/v1/api-keys/{id}

JIRA (jira-service)
  GET    /api/v1/jira/config
  POST   /api/v1/jira/config
  POST   /api/v1/jira/config/test

Profile (identity-service)
  GET    /api/v1/profile
  PATCH  /api/v1/profile
  POST   /api/v1/profile/change-password

Admin (identity-service + gateway)
  GET    /api/v1/admin/users
  POST   /api/v1/admin/users/invite
  PATCH  /api/v1/admin/users/{id}
  POST   /api/v1/admin/users/{id}/unlock
  POST   /api/v1/admin/users/{id}/reset-password
  GET    /api/v1/admin/roles
  GET    /api/v1/admin/health
  GET    /api/v1/admin/settings
  PATCH  /api/v1/admin/settings

Audit (audit-service)
  GET    /api/v1/audit-log
```

### v2 Endpoints (Existing — need schema updates)

```
CVE Intelligence (data-service + search-service)
  POST   /api/v2/cves/search             [schema update needed]
  POST   /api/v2/cves/search/semantic
  GET    /api/v2/cves/{id}              [schema update needed]
  GET    /api/v2/cves/aggregations
  GET    /api/v2/cves/export
  GET    /api/v2/kev                    [add unmitigated_in_platform]
  GET    /api/v2/kev/stats
  GET    /api/v2/kev/ransomware
  GET    /api/v2/epss/{cve_id}
  GET    /api/v2/epss/top               [new]
  GET    /api/v2/epss/distribution      [new]
  GET    /api/v2/browse
  GET    /api/v2/browse/{vendor}
  GET    /api/v2/browse/{vendor}/{product}
  GET    /api/v2/cwe                    [new — list]
  GET    /api/v2/cwe/{id}
  GET    /api/v2/capec/{id}
  GET    /api/v2/vendors                [new — autocomplete]
  GET    /api/v2/dbinfo
```

---

## Chuẩn lỗi Response (Standard)

Mọi API error **PHẢI** theo format:
```json
{
  "error": "MACHINE_READABLE_CODE",
  "message": "Human readable message in English",
  "details": {},
  "trace_id": "abc123"
}
```

| HTTP Status | Error Code | Tình huống |
|-------------|-----------|------------|
| 400 | `VALIDATION_ERROR` | Input validation failed |
| 401 | `UNAUTHORIZED` | Missing/invalid auth |
| 401 | `INVALID_CREDENTIALS` | Wrong email/password |
| 401 | `TOKEN_EXPIRED` | JWT hết hạn |
| 403 | `FORBIDDEN` | Thiếu permission |
| 404 | `NOT_FOUND` | Entity không tồn tại |
| 409 | `CONFLICT` | State conflict (e.g., invalid transition) |
| 409 | `INVALID_TRANSITION` | Finding state machine violation |
| 423 | `ACCOUNT_LOCKED` | Too many login failures |
| 429 | `RATE_LIMIT_EXCEEDED` | Rate limit hit |
| 500 | `INTERNAL_ERROR` | Server error |
| 503 | `SERVICE_UNAVAILABLE` | Dependency unavailable |

---

## Non-Functional Requirements cho APIs

| NFR | Yêu cầu |
|-----|---------|
| Response time | CVE lookup < 100ms P95; Search < 500ms P95; Dashboard < 500ms P95 |
| Auth middleware | < 5ms per request |
| SSE latency | < 2 giây từ event đến browser |
| Rate limiting | 100 req/min per user (default); 5 req/min login endpoint |
| CORS | Chỉ allow whitelisted origins |
| TLS | TLS 1.3 minimum |
| Pagination | Server-side; max page_size = 200 |
| API versioning | `/api/v1/` và `/api/v2/` — backward compatible |

---

## Thứ tự Implementation (Priority)

### Sprint 1 — Foundation (P0 blocking)
1. **CR-UI-001** §2.1–2.4: login/refresh/me/logout
2. **CR-UI-002** §2.1: `GET /api/v1/dashboard` aggregate
3. **CR-UI-003**: Schema updates cho CVE search response (`is_kev`, `has_exploit`, `aggregations`)
4. **CR-UI-005**: Schema updates cho findings response + bulk/reopen + bulk/assign

### Sprint 2 — Core Features (P0)
5. **CR-UI-007**: Product grades endpoint + finding_summary
6. **CR-UI-009** §2.1–2.5: Reports CRUD + download
7. **CR-UI-009** §3.1–3.4: Notifications list/read
8. **CR-UI-010** §5.1–5.3: API Keys management
9. **CR-UI-010** §2.1–2.6: Admin User Management

### Sprint 3 — Intelligence (P1)
10. **CR-UI-003** new endpoints: vendors autocomplete, epss/top, epss/distribution, cwe list
11. **CR-UI-002** §2.3: Notifications SSE stream
12. **CR-UI-010** §3.1: System Health

### Sprint 4 — v3.0 Features (P1, planned)
13. **CR-UI-004**: Active Scanning API (CR-OVS-001)
14. **CR-UI-006**: Asset Management API (CR-OVS-007)
15. **CR-UI-008**: AI Center API (CR-OVS-005)
16. **CR-UI-001** §2.5–2.9: MFA + OAuth2 (CR-OVS-003)
