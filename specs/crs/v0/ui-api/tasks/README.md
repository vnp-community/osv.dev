# Tasks — Backend UI-API Implementation

**Nguồn Solutions:** [`specs/crs/v0/ui-api/solutions/`](../solutions/)  
**Kiến trúc:** [`specs/01-architecture.md`](../../../01-architecture.md)  
**Technical Design:** [`specs/02-technical-design.md`](../../../02-technical-design.md)  
**CR Index:** [`specs/crs/v0/ui-api/README.md`](../README.md)

---

## Mục tiêu

Bộ tasks này chuyển đổi **5 Solution documents** thành các **tác vụ cụ thể để AI thực thi** trên Go Microservices — mỗi task có đủ context, code mẫu, target files, verification steps để AI thực hiện độc lập không cần hỏi lại.

**Nguyên tắc bất biến:**
- Mọi error response **PHẢI** theo format `{"error":"CODE","message":"...","trace_id":"..."}`
- Mọi handler **PHẢI** dùng `r.Header.Get("X-User-ID")` / `X-User-Role` từ gateway middleware — không tự validate JWT
- Mọi DB query **PHẢI** parameterized — không string concat
- Sau mỗi task: chạy `go build ./...` + `go test ./...` không có FAIL

---

## Danh sách Tasks

### Sprint 1 — P0 Blocking (auth + dashboard)

| Task ID | Tên | Priority | Solution Ref | Service | Est. |
|---------|-----|----------|-------------|---------|------|
| [TASK-BE-001](./TASK-BE-001-auth-http-handlers.md) | ✅ DONE identity-service: Auth HTTP Handlers (login/refresh/me/logout) | 🔴 P0 | SOL-UI-001 §2.3–2.4 | identity-service | 4h |
| [TASK-BE-002](./TASK-BE-002-session-repository.md) | ✅ DONE identity-service: Sessions table + Repository | 🔴 P0 | SOL-UI-001 §2.6, §3 | identity-service | 2h |
| [TASK-BE-003](./TASK-BE-003-rbac-userdata.md) | ✅ DONE identity-service: RBAC Matrix + UserDTO + Admin handlers | 🔴 P0 | SOL-UI-001 §2.5, §2.7, §2.9 | identity-service | 3h |
| [TASK-BE-004](./TASK-BE-004-apikey-profile-handlers.md) | ✅ DONE identity-service: API Keys CRUD + Profile handlers | 🔴 P0 | SOL-UI-001 §2.8 | identity-service | 3h |
| [TASK-BE-005](./TASK-BE-005-dashboard-bff.md) | ✅ DONE gateway: Dashboard BFF handler + internal clients | 🔴 P0 | SOL-UI-002 §2.2–2.4 | apps/osv | 5h |
| [TASK-BE-006](./TASK-BE-006-finding-internal-endpoints.md) | ✅ DONE finding-service: Internal aggregate endpoints cho BFF | 🔴 P0 | SOL-UI-002 §2.4 | finding-service | 3h |

### Sprint 2 — P0 Core Schema Extensions

| Task ID | Tên | Priority | Solution Ref | Service | Est. |
|---------|-----|----------|-------------|---------|------|
| [TASK-BE-007](./TASK-BE-007-finding-schema-extension.md) | ✅ DONE finding-service: Extended Finding Response DTO + Aggregations | 🔴 P0 | SOL-UI-004 §1.1–1.2 | finding-service | 4h |
| [TASK-BE-008](./TASK-BE-008-finding-bulk-notes.md) | ✅ DONE finding-service: Bulk Ops (reopen/assign) + Notes | 🔴 P0 | SOL-UI-004 §1.3 | finding-service | 3h |
| [TASK-BE-009](./TASK-BE-009-product-grades.md) | ✅ DONE finding-service: Product Grades endpoint + Product DTO extension | 🔴 P0 | SOL-UI-004 §1.4 | finding-service | 3h |
| [TASK-BE-010](./TASK-BE-010-reports-download.md) | ✅ DONE finding-service: Reports download via MinIO presigned URL | 🔴 P0 | SOL-UI-004 §1.5 | finding-service | 2h |
| [TASK-BE-011](./TASK-BE-011-notification-rest.md) | ✅ DONE notification-service: Notifications REST API (list/read/count) | 🔴 P0 | SOL-UI-002 §2.6 | notification-service | 3h |

### Sprint 3 — P0 Admin + CVE Intel

| Task ID | Tên | Priority | Solution Ref | Service | Est. |
|---------|-----|----------|-------------|---------|------|
| [TASK-BE-012](./TASK-BE-012-notification-sse.md) | ✅ DONE notification-service: SSE Event Broker + Stream Endpoint | 🟡 P1 | SOL-UI-002 §2.5 | notification-service | 4h |
| [TASK-BE-013](./TASK-BE-013-admin-health-settings.md) | ✅ DONE gateway: Admin Health Fan-out + Settings BFF | 🔴 P0 | SOL-UI-004 §5–6 | gateway | 4h |
| [TASK-BE-014](./TASK-BE-014-jira-config-http.md) | ✅ DONE jira-service: JIRA Config HTTP CRUD + test endpoint | 🔴 P0 | SOL-UI-004 §4 | jira-service | 3h |
| [TASK-BE-015](./TASK-BE-015-audit-log-http.md) | ✅ DONE audit-service: Audit log HTTP list endpoint | 🔴 P0 | SOL-UI-004 §3 | audit-service | 2h |
| [TASK-BE-016](./TASK-BE-016-cve-search-schema.md) | ✅ DONE search-service: CVE Search response schema update (7 new fields + aggregations) | 🔴 P0 | SOL-UI-003 §2.1–2.3 | search-service | 4h |

### Sprint 4 — P1 New Endpoints

| Task ID | Tên | Priority | Solution Ref | Service | Est. |
|---------|-----|----------|-------------|---------|------|
| [TASK-BE-017](./TASK-BE-017-kev-unmitigated.md) | ✅ DONE data-service: KEV unmitigated_in_platform cross-service count | 🟡 P1 | SOL-UI-003 §2.4 | data-service | 2h |
| [TASK-BE-018](./TASK-BE-018-epss-endpoints.md) | ✅ DONE data-service: EPSS top-N + distribution endpoints | 🟡 P1 | SOL-UI-003 §2.5–2.6 | data-service | 2h |
| [TASK-BE-019](./TASK-BE-019-cwe-vendors-endpoints.md) | ✅ DONE data-service: CWE list + Vendor autocomplete endpoints | 🟡 P1 | SOL-UI-003 §2.7–2.8 | data-service | 3h |
| [TASK-BE-020](./TASK-BE-020-webhook-test.md) | ✅ DONE notification-service: Webhook test delivery endpoint | 🟡 P1 | SOL-UI-004 §2 | notification-service | 2h |
| [TASK-BE-021](./TASK-BE-021-sla-dashboard.md) | ✅ DONE sla-service: SLA dashboard aggregate endpoint | 🟡 P1 | SOL-UI-002 §2.8 | sla-service | 3h |
| [TASK-BE-022](./TASK-BE-022-gateway-routes.md) | ✅ DONE gateway: Register tất cả routes mới (50+ routes) | 🔴 P0 | SOL-UI-001–004 (tất cả) | apps/osv | 2h |

---

## Thứ tự thực thi (Dependency Graph)

```
TASK-BE-002 (Session DB)
    └── TASK-BE-001 (Auth handlers — cần session repo)
            └── TASK-BE-003 (RBAC + Admin handlers)
            └── TASK-BE-004 (API Keys + Profile)

TASK-BE-006 (Finding internal endpoints)
    └── TASK-BE-005 (Dashboard BFF — cần internal endpoints)

TASK-BE-007 (Finding schema)
    └── TASK-BE-008 (Bulk + Notes — build on schema)
    └── TASK-BE-009 (Product grades — build on schema)

TASK-BE-010 (Reports download — standalone)
TASK-BE-011 (Notification REST — standalone)
TASK-BE-012 (SSE broker — needs TASK-BE-011 alert store)

TASK-BE-013 (Admin health — standalone BFF)
TASK-BE-014 (JIRA config — standalone)
TASK-BE-015 (Audit log — standalone)
TASK-BE-016 (CVE search schema — standalone)
TASK-BE-017 (KEV unmitigated — needs internal endpoint from finding-service)
TASK-BE-018 (EPSS endpoints — standalone)
TASK-BE-019 (CWE + vendors — standalone)
TASK-BE-020 (Webhook test — standalone)
TASK-BE-021 (SLA dashboard — standalone)

TASK-BE-022 (Gateway routes — phụ thuộc tất cả tasks trên)
```

---

## Solution → Task Mapping

| Solution | Tasks |
|---------|-------|
| [SOL-UI-001](../solutions/SOL-UI-001-auth-service-extension.md) Auth | TASK-BE-001, TASK-BE-002, TASK-BE-003, TASK-BE-004 |
| [SOL-UI-002](../solutions/SOL-UI-002-dashboard-bff-sse.md) Dashboard BFF + SSE | TASK-BE-005, TASK-BE-006, TASK-BE-011, TASK-BE-012, TASK-BE-021 |
| [SOL-UI-003](../solutions/SOL-UI-003-cve-intel-schema-update.md) CVE Intel | TASK-BE-016, TASK-BE-017, TASK-BE-018, TASK-BE-019 |
| [SOL-UI-004](../solutions/SOL-UI-004-finding-product-reports-admin.md) Finding+Product+Reports+Admin | TASK-BE-007, TASK-BE-008, TASK-BE-009, TASK-BE-010, TASK-BE-013, TASK-BE-014, TASK-BE-015, TASK-BE-020 |
| [SOL-UI-005](../solutions/SOL-UI-005-v30-scanning-assets-ai.md) v3.0 | Xem OVS tasks tại `specs/crs/v1/OpenVulnScan/tasks/` |
| Cross-cutting | TASK-BE-022 (gateway routes) |

---

## Quy ước Task File

```markdown
# TASK-BE-XXX — Tên Task

| Field | Value |
|-------|-------|
| Task ID | TASK-BE-XXX |
| Service | services/xxx-service |
| Solution Ref | SOL-UI-00X §N.N |
| Priority | 🔴/🟡 P0/P1 |
| Depends On | TASK-BE-YYY |
| Estimated | Xh |

## Context     — Tại sao task này cần, hiện trạng service
## Goal        — Mục tiêu cụ thể
## Target Files — CREATE/MODIFY file list với paths đầy đủ
## Implementation — Code Go đầy đủ cho từng file
## Verification  — go build + go test commands
## Checklist    — Checkbox items AI phải hoàn thành
```
