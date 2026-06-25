# Tasks — DefectDojo Implementation

## Cách sử dụng

Mỗi task file là một **đơn vị công việc độc lập** dành cho AI thực thi.  
Mỗi file bao gồm:
- **Context**: Mô tả bài toán, liên kết solution
- **Prerequisites**: Các task phải hoàn thành trước
- **Files to create/modify**: Danh sách file cụ thể
- **Implementation**: Code mẫu / spec chi tiết
- **Acceptance Criteria**: Điều kiện hoàn thành

---

## Task Index

### Phase 1 — Foundation

| Task ID | Tên | Service | CR | Phụ thuộc |
|---------|-----|---------|-----|-----------|
| [TASK-DD-001](./TASK-DD-001-finding-product-hierarchy-domain.md) | Product/Engagement/Test Domain Extensions | finding-service | CR-DD-001 | — |
| [TASK-DD-002](./TASK-DD-002-finding-product-hierarchy-usecases.md) | Product Member + Tool Config Use Cases | finding-service | CR-DD-001 | TASK-DD-001 |
| [TASK-DD-003](./TASK-DD-003-finding-product-hierarchy-api.md) | Product Hierarchy REST API + gRPC Extensions | finding-service | CR-DD-001 | TASK-DD-002 |
| [TASK-DD-004](./TASK-DD-004-finding-state-machine.md) | Finding State Machine (6 States) | finding-service | CR-DD-004 | TASK-DD-001 |
| [TASK-DD-005](./TASK-DD-005-finding-bulk-notes-groups.md) | Finding Bulk Ops, Notes, Groups, CVSS | finding-service | CR-DD-004 | TASK-DD-004 |
| [TASK-DD-006](./TASK-DD-006-finding-grpc-dedup-extensions.md) | Finding gRPC Extensions for Dedup & SLA | finding-service | CR-DD-003/006 | TASK-DD-004 |
| [TASK-DD-007](./TASK-DD-007-finding-migrations.md) | Finding Service DB Migrations | finding-service | CR-DD-001/004 | TASK-DD-001 |
| [TASK-DD-008](./TASK-DD-008-scan-import-pipeline.md) | Scan Import Pipeline (12 steps) | scan-service | CR-DD-002 | TASK-DD-006 |
| [TASK-DD-009](./TASK-DD-009-scan-parser-factory.md) | Security Parser Factory (20+ parsers) | scan-service | CR-DD-002 | — |
| [TASK-DD-010](./TASK-DD-010-scan-dedup-engine.md) | Deduplication Engine (3 algorithms) | scan-service | CR-DD-003 | TASK-DD-006 |
| [TASK-DD-011](./TASK-DD-011-scan-migrations.md) | Scan Service DB Migrations | scan-service | CR-DD-002 | — |
| [TASK-DD-012](./TASK-DD-012-gateway-auth-ratelimit.md) | Gateway Auth (JWT + API Key) + Rate Limit | apps/osv | CR-DD-011 | — |
| [TASK-DD-013](./TASK-DD-013-gateway-routes.md) | Gateway Route Rules (100+ routes) | apps/osv | CR-DD-011 | TASK-DD-012 |
| [TASK-DD-014](./TASK-DD-014-gateway-openapi.md) | OpenAPI Aggregation + Error Standardization | apps/osv | CR-DD-011 | TASK-DD-013 |

### Phase 2 — Security Management

| Task ID | Tên | Service | CR | Phụ thuộc |
|---------|-----|---------|-----|-----------|
| [TASK-DD-015](./TASK-DD-015-finding-risk-acceptance.md) | Risk Acceptance Domain + Use Cases | finding-service | CR-DD-005 | TASK-DD-003 |
| [TASK-DD-016](./TASK-DD-016-finding-risk-acceptance-scheduler.md) | Risk Acceptance Expiry Scheduler | finding-service | CR-DD-005 | TASK-DD-015 |
| [TASK-DD-017](./TASK-DD-017-sla-service-bootstrap.md) | SLA Service Bootstrap (new service) | sla-service | CR-DD-006 | — |
| [TASK-DD-018](./TASK-DD-018-sla-config-crud.md) | SLA Config CRUD + Assignment | sla-service | CR-DD-006 | TASK-DD-017 |
| [TASK-DD-019](./TASK-DD-019-sla-compute-breach.md) | SLA Compute Expiry + Breach Detection | sla-service | CR-DD-006 | TASK-DD-018 |
| [TASK-DD-020](./TASK-DD-020-sla-bulk-recompute.md) | SLA Bulk Recompute + Dashboard | sla-service | CR-DD-006 | TASK-DD-019 |
| [TASK-DD-021](./TASK-DD-021-notification-event-types.md) | Notification Event Types Extension | notification-service | CR-DD-007 | — |
| [TASK-DD-022](./TASK-DD-022-notification-dispatch.md) | Notification Dispatch + Retry Logic | notification-service | CR-DD-007 | TASK-DD-021 |
| [TASK-DD-023](./TASK-DD-023-notification-channels.md) | Notification Channels (Email/Slack/Teams/Webhook) | notification-service | CR-DD-007 | TASK-DD-022 |
| [TASK-DD-024](./TASK-DD-024-notification-inapp-ssrf.md) | In-app Alerts + SSRF Protection | notification-service | CR-DD-007 | TASK-DD-022 |
| [TASK-DD-025](./TASK-DD-025-audit-service-bootstrap.md) | Audit Service Bootstrap (new service) | audit-service | CR-DD-010 | — |
| [TASK-DD-026](./TASK-DD-026-audit-record-query.md) | Audit Record + Query + Export | audit-service | CR-DD-010 | TASK-DD-025 |

### Phase 3 — Integrations

| Task ID | Tên | Service | CR | Phụ thuộc |
|---------|-----|---------|-----|-----------|
| [TASK-DD-027](./TASK-DD-027-jira-service-bootstrap.md) | JIRA Service Bootstrap (new service) | jira-service | CR-DD-008 | — |
| [TASK-DD-028](./TASK-DD-028-jira-push-finding.md) | JIRA Push Finding + Credential Encryption | jira-service | CR-DD-008 | TASK-DD-027 |
| [TASK-DD-029](./TASK-DD-029-jira-pull-webhook.md) | JIRA Webhook Handler + Pull Status | jira-service | CR-DD-008 | TASK-DD-028 |
| [TASK-DD-030](./TASK-DD-030-finding-report-grading.md) | Finding Report Generation + Product Grading | finding-service | CR-DD-009 | TASK-DD-006 |

---

## Task Format

Mỗi task file tuân theo cấu trúc:

```markdown
# TASK-DD-XXX — [Tên task]

## Metadata
| Field | Value |
|-------|-------|
| Task ID | TASK-DD-XXX |
| Service | <service-name> |
| CR | CR-DD-XXX |
| Phase | 1/2/3 |
| Priority | High/Medium/Low |
| Prerequisites | TASK-DD-YYY |

## Context
<Mô tả ngắn gọn>

## Reference
<Links đến solution docs và CR docs>

## Files to Create/Modify
<Danh sách file cụ thể>

## Implementation Spec
<Code mẫu, chi tiết implementation>

## Acceptance Criteria
<Danh sách điều kiện hoàn thành>
```

---

## Dependency Graph

```
Phase 1:
TASK-DD-001 → TASK-DD-002 → TASK-DD-003
           ↘ TASK-DD-004 → TASK-DD-005
           ↘ TASK-DD-006 ← (cần cho scan)
           ↘ TASK-DD-007 (migrations)

TASK-DD-006 → TASK-DD-008 → (import pipeline)
TASK-DD-009 (parsers, độc lập)
TASK-DD-006 → TASK-DD-010 (dedup)
TASK-DD-011 (migrations, độc lập)

TASK-DD-012 → TASK-DD-013 → TASK-DD-014

Phase 2:
TASK-DD-003 → TASK-DD-015 → TASK-DD-016
TASK-DD-017 → TASK-DD-018 → TASK-DD-019 → TASK-DD-020
TASK-DD-021 → TASK-DD-022 → TASK-DD-023
                          ↘ TASK-DD-024
TASK-DD-025 → TASK-DD-026

Phase 3:
TASK-DD-027 → TASK-DD-028 → TASK-DD-029
TASK-DD-006 → TASK-DD-030
```
