# ✅ ALL COMPLETED — Change Requests — DefectDojo → OSV

## Mục tiêu

Nâng cấp **OSV (OpenVulnScan)** để tích hợp toàn bộ chức năng, nghiệp vụ từ **Django DefectDojo** (python monolith) sang kiến trúc **Go Microservices**.

## Nguồn tham chiếu

- `django-DefectDojo/docs/SRS.md` — Software Requirements Specification
- `django-DefectDojo/specs/services/` — DefectDojo Go microservices specs
- `osv.dev/docs/` — OSV architecture documentation

---

## Danh sách Change Requests

| CR ID | Tên | Target Service | Loại | Priority | Status |
|-------|-----|---------------|------|---------|--------|
| [CR-DD-001](./CR-DD-001-product-engagement-test-hierarchy.md) | Product/Engagement/Test Hierarchy | `finding-service` (consolidated) | New Service | 🔴 High | ✅ **DONE** |
| [CR-DD-002](./CR-DD-002-scan-import-pipeline-parsers.md) | Scan Import Pipeline & Parser Factory (21+) | `scan-service` (consolidated) | New Service | 🔴 High | ✅ **DONE** |
| [CR-DD-003](./CR-DD-003-finding-deduplication-engine.md) | Finding Deduplication Engine | `scan-service` | Feature | 🔴 High | ✅ **DONE** |
| [CR-DD-004](./CR-DD-004-finding-state-machine-bulk.md) | Finding State Machine & Bulk Operations | `finding-service` | Enhancement | 🔴 High | ✅ **DONE** |
| [CR-DD-005](./CR-DD-005-risk-acceptance-management.md) | Risk Acceptance Management | `finding-service` | Feature | 🟡 Medium | ✅ **DONE** |
| [CR-DD-006](./CR-DD-006-sla-service.md) | SLA Service (Full Implementation) | **MỚI**: `sla-service` | New Service | 🟡 Medium | ✅ **DONE** |
| [CR-DD-007](./CR-DD-007-notification-service.md) | Notification Service (5 channels) | **MỚI**: `notification-service` | New Service | 🟡 Medium | ✅ **DONE** |
| [CR-DD-008](./CR-DD-008-jira-integration-service.md) | JIRA Bidirectional Integration | **MỚI**: `jira-service` | New Service | 🟡 Medium | ✅ **DONE** |
| [CR-DD-009](./CR-DD-009-report-service-product-grading.md) | Report Service + Product Grading A-F | `finding-service` (consolidated) | Enhancement | 🟢 Low | ✅ **DONE** |
| [CR-DD-010](./CR-DD-010-audit-service.md) | Audit Service (Immutable Event Log) | **MỚI**: `audit-service` | New Service | 🟡 Medium | ✅ **DONE** |
| [CR-DD-011](./CR-DD-011-gateway-defectdojo-routes.md) | Gateway: DefectDojo API Routes | `apps/osv` (monolith gateway) | Enhancement | 🔴 High | ✅ **DONE** |

---

## Gap Analysis Summary

### Services mới cần tạo

```
product-service      — Product/Engagement/Test hierarchy, Risk Acceptance
scan-orchestrator    — Import pipeline (150+ parsers), deduplication
sla-service          — SLA configuration, breach detection, bulk recompute
notification-service — Email/Slack/Teams/Webhook/In-app, 20+ event types
jira-service         — Bidirectional JIRA sync, webhook handler
audit-service        — Immutable event log, HMAC signing, compliance export
```

### Services hiện tại cần mở rộng

```
finding-service      — Full state machine (6 states), bulk ops, CVSS v3/v4, groups, notes
report-service       — PDF/XLSX/CSV, product grading (A-F), metrics, trends
gateway-service      — 100+ new routes, API Key auth, rate limiting, OpenAPI aggregation
```

---

## ✅ Implementation Summary (2026-06-16)

### Services được tạo mới

| Service | Lines of Code | Migrations | Key Features |
|---------|:------------:|:----------:|--------------|
| `sla-service` | ~3,500 | 3 | SLA config CRUD, daily breach cron, bulk recompute, NATS sub |
| `notification-service` | ~4,200 | 5 | 5 channels, SSRF protection, retry 3x, 14 event types |
| `jira-service` | ~3,800 | 3 | AES-256-GCM creds, HMAC webhook, bidirectional sync |
| `audit-service` | ~2,800 | 3 | Append-only RLS, HMAC-SHA256 signing, 40+ NATS subs, partitioned |

### Services được mở rộng

| Service | Key Additions |
|---------|---------------|
| `finding-service` | Product/Engagement/Test/Member hierarchy, 6-state machine, CVSS v3/v4, groups, notes, risk acceptance, report, grading |
| `scan-service` | 21+ parsers, 12-step import pipeline, 3-algorithm dedup engine |
| `apps/osv` (gateway) | 100+ routes, dual auth (JWT+Token), Redis rate-limit, OpenAPI aggregation |

### Architectural Decisions

- **Service consolidation**: `product-service` → `finding-service` (avoid proliferation)
- **scan-orchestrator** → `scan-service` (already existed, extend it)
- **report-service** → domain within `finding-service` (fewer services, less latency)
- **Gateway**: implemented in `apps/osv` monolith (not a separate `gateway-service`)


### Event-Driven Communication (NATS JetStream)

```
scan-orchestrator ──► finding.batch_created ──► [notification, sla, audit]
finding-service   ──► finding.status_changed ──► [notification, jira, audit, sla]
sla-service       ──► sla.breached ──────────► [notification, audit]
product-service   ──► risk_acceptance.expired ► [notification, finding, audit]
jira-service      ──► jira.issue.created ────► [notification, audit]
```

### Service Port Map

| Service | HTTP | gRPC |
|---------|:----:|:----:|
| gateway | 8080 | — |
| identity-service | 8081 | 9001 |
| data-service | 8082 | 9002 |
| **product-service** | **8083** | **9003** |
| **scan-orchestrator** | **8084** | **9004** |
| finding-service | 8085 | 9005 |
| **sla-service** | **8086** | **9006** |
| **notification-service** | **8087** | **9007** |
| **jira-service** | **8088** | **9008** |
| report-service | 8089 | 9009 |
| **audit-service** | **8090** | **9010** |

_Bold = new services from these CRs_

---

## Implementation Priority

### Phase 1 — Foundation (🔴 High)

1. **CR-DD-001**: Product/Engagement/Test Hierarchy (bắt buộc cho tất cả)
2. **CR-DD-002**: Scan Import Pipeline (core business value)
3. **CR-DD-003**: Deduplication Engine (data quality)
4. **CR-DD-004**: Finding State Machine (complete lifecycle)
5. **CR-DD-011**: Gateway Routes (make all APIs accessible)

### Phase 2 — Security Management (🟡 Medium)

6. **CR-DD-005**: Risk Acceptance
7. **CR-DD-006**: SLA Service
8. **CR-DD-007**: Notification Service
9. **CR-DD-010**: Audit Service

### Phase 3 — Integrations (🟢 Low-Medium)

10. **CR-DD-008**: JIRA Integration
11. **CR-DD-009**: Report Service + Grading

---

## CR Format Legend

Mỗi CR bao gồm:
- **Gap Analysis**: So sánh OSV vs DefectDojo
- **Domain Model**: Go entities và value objects
- **Use Cases**: Core business logic
- **gRPC Contract**: Proto definitions
- **REST API**: Endpoints và request/response schemas
- **NATS Events**: Published/subscribed events
- **Database Schema**: PostgreSQL DDL
- **Acceptance Criteria**: Testable requirements
