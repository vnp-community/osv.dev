# UI-API Solutions — Index

**Series:** UI-API Solutions v0  
**Ngày tạo:** 2026-06-16  
**Trạng thái:** ✅ v2.2 Implemented (2026-06-17) | ⏳ v3.0 Pending OVS tasks  
**Nguồn kiến trúc:**
- [`specs/01-architecture.md`](../../01-architecture.md) — Go Microservices Architecture
- [`specs/02-technical-design.md`](../../02-technical-design.md) — Technical Design Document
- [`specs/crs/v0/ui-api/`](../) — 10 UI-API Change Requests

---

## Tổng quan

**5 tài liệu giải pháp** này ánh xạ 10 UI-API Change Requests thành các thay đổi cụ thể trong các services hiện có của OSV Platform Go Microservices.

Nguyên tắc thiết kế:
1. **Tái sử dụng tối đa** — extend existing services thay vì tạo mới
2. **Backward compatible** — không phá vỡ API v2.x đang dùng
3. **Phân tầng rõ ràng** — v2.2 extensions (blocking) trước, v3.0 features sau
4. **Gateway as BFF** — logic aggregate nhẹ đặt trong gateway, không tạo service mới

---

## Danh sách Solutions

| Solution | CRs Covered | Services | Phiên bản | Trạng thái |
|----------|-------------|----------|-----------|------------|
| [SOL-UI-001](./SOL-UI-001-auth-service-extension.md) | CR-UI-001 | identity-service:8081 | v2.2 ext | ✅ Implemented |
| [SOL-UI-002](./SOL-UI-002-dashboard-bff-sse.md) | CR-UI-002 | apps/osv (BFF) + notification-service:8087 | v2.2 ext | ✅ Implemented |
| [SOL-UI-003](./SOL-UI-003-cve-intel-schema-update.md) | CR-UI-003 | data-service:8082 + search-service | v2.2 ext | ✅ Implemented |
| [SOL-UI-004](./SOL-UI-004-finding-product-reports-admin.md) | CR-UI-005, 007, 009, 010 | finding-service:8085 + notification-service + audit-service + jira-service + gateway | v2.2 ext | ✅ Implemented |
| [SOL-UI-005](./SOL-UI-005-v30-scanning-assets-ai.md) | CR-UI-004, 006, 008 (+ CR-UI-001 §MFA/OAuth2) | scan-service:8058 + asset-service:8068 + ai-service:8052 + auth-service:8051 | v3.0 | ⏳ Pending OVS |

---

## Phân loại công việc

### Sprint 1 — P0 Blocking (phải có để UI load)

| Task | Solution | Service | Effort |
|------|----------|---------|--------|
| `POST /api/v1/auth/login` | SOL-UI-001 §2.3 | identity-service | ~4h |
| `POST /api/v1/auth/refresh` | SOL-UI-001 §2.4 | identity-service | ~3h |
| `GET /api/v1/auth/me` | SOL-UI-001 §2.5 | identity-service | ~2h |
| `POST /api/v1/auth/logout` | SOL-UI-001 §2.3 | identity-service | ~1h |
| Session table migration | SOL-UI-001 §3 | DB | ~1h |
| RBAC permissions[] vào JWT | SOL-UI-001 §2.5 | identity-service | ~2h |
| `GET /api/v1/dashboard` BFF | SOL-UI-002 §2.2 | apps/osv | ~6h |
| finding-service internal endpoints | SOL-UI-002 §2.4 | finding-service | ~4h |

**Sprint 1 total: ~23h**

---

### Sprint 2 — P0 Core Features

| Task | Solution | Service | Effort |
|------|----------|---------|--------|
| Finding response: `is_kev`, `epss_score`, `sla_days_left`, `jira_*` | SOL-UI-004 §1.1 | finding-service | ~4h |
| Finding list: `by_severity`, `by_status`, `sla_stats` aggregations | SOL-UI-004 §1.2 | finding-service | ~3h |
| `POST /api/v1/findings/bulk/reopen` | SOL-UI-004 §1.3 | finding-service | ~2h |
| `POST /api/v1/findings/bulk/assign` | SOL-UI-004 §1.3 | finding-service | ~2h |
| `POST /api/v1/findings/{id}/notes` | SOL-UI-004 §1.3 | finding-service | ~2h |
| `GET /api/v1/products/grades` | SOL-UI-004 §1.4 | finding-service | ~3h |
| Product response: `grade`, `finding_summary` | SOL-UI-004 §1.4 | finding-service | ~2h |
| `GET /api/v1/reports/{id}/download` (MinIO presigned) | SOL-UI-004 §1.5 | finding-service | ~2h |
| Notifications REST: list, mark read, unread-count | SOL-UI-002 §2.6 | notification-service | ~3h |

**Sprint 2 total: ~23h**

---

### Sprint 3 — P0 Admin + Intelligence

| Task | Solution | Service | Effort |
|------|----------|---------|--------|
| `GET /api/v1/admin/users` + CRUD | SOL-UI-001 §2.7 | identity-service | ~4h |
| `GET /api/v1/admin/roles` RBAC matrix | SOL-UI-001 §2.9 | identity-service | ~1h |
| `GET/POST/DELETE /api/v1/api-keys` | SOL-UI-001 §2.8 | identity-service | ~3h |
| `GET/PATCH /api/v1/profile` | SOL-UI-001 (profile handler) | identity-service | ~2h |
| `GET /api/v1/admin/health` (BFF fan-out) | SOL-UI-004 §5 | apps/osv | ~4h |
| `GET/PATCH /api/v1/admin/settings` | SOL-UI-004 §6 | apps/osv | ~3h |
| `POST /api/v1/jira/config` + test | SOL-UI-004 §4 | jira-service | ~3h |
| `GET /api/v1/audit-log` | SOL-UI-004 §3 | audit-service | ~2h |
| CVE search schema: `is_kev`, `has_exploit`, aggregations | SOL-UI-003 §2.2-2.3 | search-service | ~3h |
| `GET /api/v2/cves/{id}` full detail | SOL-UI-003 §2.2 | search-service | ~3h |
| KEV `unmitigated_in_platform` | SOL-UI-003 §2.4 | data-service | ~2h |

**Sprint 3 total: ~30h**

---

### Sprint 4 — P1 Intelligence + SSE

| Task | Solution | Service | Effort |
|------|----------|---------|--------|
| `GET /api/v1/notifications/stream` SSE | SOL-UI-002 §2.5 | notification-service | ~4h |
| SSE Event Broker + NATS bridge | SOL-UI-002 §2.5 | notification-service | ~4h |
| `POST /api/v1/webhooks/{id}/test` | SOL-UI-004 §2 | notification-service | ~2h |
| `GET /api/v2/epss/top` | SOL-UI-003 §2.5 | data-service | ~2h |
| `GET /api/v2/epss/distribution` | SOL-UI-003 §2.6 | data-service | ~2h |
| `GET /api/v2/cwe` list | SOL-UI-003 §2.7 | data-service | ~2h |
| `GET /api/v2/vendors` autocomplete | SOL-UI-003 §2.8 | data-service | ~2h |
| `GET /api/v1/dashboard/sla` detail | SOL-UI-002 §2.8 | sla-service | ~3h |

**Sprint 4 total: ~21h**

---

### Sprint 5-8 — v3.0 Features (phụ thuộc OVS CRs)

| Task | Solution | Phụ thuộc |
|------|----------|-----------|
| Active Scanning API + SSE | SOL-UI-005 §1 | CR-OVS-001 (scan-service v3.0) |
| Asset Management API | SOL-UI-005 §2 | CR-OVS-007 (asset-service) |
| AI Triage + Enrichment API | SOL-UI-005 §3 | CR-OVS-005 (ai-service) |
| MFA TOTP setup | SOL-UI-005 §4 | CR-OVS-003 (auth-service v3.0) |
| OAuth2 Google/GitHub | SOL-UI-005 §4 | CR-OVS-003 (auth-service v3.0) |

---

## Service Change Map

```
identity-service:8081
  ├── SOL-UI-001: +auth_handler.go, +admin_handler.go, +profile_handler.go, +apikey_handler.go
  ├── SOL-UI-001: +domain/rbac.go (permission matrix)
  ├── SOL-UI-001: +usecase/session_usecase.go (refresh token rotation)
  └── SOL-UI-001: +db/migrations/003_add_sessions.sql

apps/osv (gateway):8080
  ├── SOL-UI-002: +bff/dashboard.go (fan-out), +bff/clients.go
  ├── SOL-UI-002: +proxy.go ForwardSSE() method
  ├── SOL-UI-004: +bff/health.go (admin health fan-out)
  ├── SOL-UI-004: +bff/settings.go
  └── ALL: +router.go (50+ new route registrations)

finding-service:8085
  ├── SOL-UI-004: MODIFY dto.go (finding fields), MODIFY list handler (aggregations)
  ├── SOL-UI-004: +bulk_handler.go (reopen, assign)
  ├── SOL-UI-004: +note_handler.go (finding notes)
  ├── SOL-UI-004: MODIFY product_handler.go (grades endpoint)
  ├── SOL-UI-004: MODIFY report_handler.go (download endpoint)
  └── SOL-UI-002: +internal_handler.go (/internal/* BFF endpoints)

notification-service:8087
  ├── SOL-UI-002: +broker/event_broker.go (in-memory SSE fan-out)
  ├── SOL-UI-002: +adapter/http/sse_handler.go (SSE stream)
  ├── SOL-UI-002: MODIFY adapter/nats/subscriber.go (add broker.Push calls)
  ├── SOL-UI-002: +adapter/http/alerts_handler.go (notifications REST)
  └── SOL-UI-004: MODIFY webhook_handler.go (add test endpoint)

data-service:8082
  ├── SOL-UI-003: MODIFY dto.go (add fields to CVEResponseItem)
  ├── SOL-UI-003: MODIFY kev_handler.go (add unmitigated_in_platform)
  ├── SOL-UI-003: MODIFY epss_handler.go (add top, distribution)
  ├── SOL-UI-003: MODIFY cwe_handler.go (add list endpoint)
  └── SOL-UI-003: +vendor_handler.go (autocomplete)

search-service (internal)
  ├── SOL-UI-003: MODIFY dto.go (add 8 new response fields)
  └── SOL-UI-003: MODIFY opensearch/client.go (add aggregations query)

audit-service:8090
  └── SOL-UI-004: MODIFY handler.go (expose list endpoint via new route)

jira-service:8088
  └── SOL-UI-004: +config_handler.go (GET, POST, test endpoints)

sla-service:8086
  └── SOL-UI-002: +dashboard_handler.go (SLA aggregate for dashboard SLA page)
```

---

## Error Response Standard

Tất cả services **PHẢI** implement chuẩn lỗi này:

```json
{
  "error": "MACHINE_READABLE_CODE",
  "message": "Human readable description",
  "details": {},
  "trace_id": "abc123"
}
```

Shared error package: `services/shared/errors/errors.go` — đã có, tất cả service mới/extend phải import và sử dụng.

---

## Key Design Decisions

| Decision | Rationale |
|----------|-----------|
| BFF trong gateway (không tạo service riêng) | Gateway đã connect tất cả services; tránh thêm network hop; phù hợp với kiến trúc "gateway handles cross-cutting" |
| Refresh token = httpOnly cookie | Tránh XSS token theft; SameSite=Strict chống CSRF; không cần CSRF token bổ sung |
| SSE stream từ notification-service, không qua gateway | Tránh proxy phải stream; gateway forward SSE bằng ForwardSSE() không buffer |
| `epss_score` và `is_kev` copy vào findings table | Tránh cross-service query mỗi request; synced khi import scan hoặc daily job |
| Reports download = MinIO presigned URL (302 redirect) | Không stream large files qua gateway; MinIO URL trực tiếp đến client |
| RBAC check trong gateway middleware | Single enforcement point; services trust X-User-Role header từ internal network |
