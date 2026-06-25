# Tasks — GlobalCVE Implementation

## Cách sử dụng

Mỗi task file là một **đơn vị công việc độc lập** dành cho AI thực thi.  
Mỗi file bao gồm:
- **Context**: Mô tả bài toán, liên kết solution
- **Prerequisites**: Các task phải hoàn thành trước
- **Files to create/modify**: Danh sách file cụ thể với đường dẫn tuyệt đối
- **Implementation Spec**: Code mẫu chi tiết
## Task Index

> **Legend:** ✅ Done · 🔄 In Progress · ⏳ Pending · ❌ Blocked

### Phase 1 — Core Pipeline & Gateway (🔴 High)

| Task ID | Tên | Service | CR | Phụ thuộc | Status |
|---------|-----|---------|-----|-----------|--------|
| [TASK-GCV-001](./TASK-GCV-001-fetcher-interface-registry.md) | Fetcher Interface + Registry | `data-service` | CR-001 | — | ✅ Done |
| [TASK-GCV-002](./TASK-GCV-002-fetcher-circl-jvn.md) | CIRCL + JVN RSS Fetchers | `data-service` | CR-001 | TASK-GCV-001 | ✅ Done |
| [TASK-GCV-003](./TASK-GCV-003-fetcher-exploitdb-cveorg.md) | ExploitDB CSV + CVE.org GitHub Fetchers | `data-service` | CR-001 | TASK-GCV-001 | ✅ Done |
| [TASK-GCV-004](./TASK-GCV-004-fetcher-cnnvd-scheduler.md) | CNNVD Fetcher + Scheduler Update | `data-service` | CR-001 | TASK-GCV-001 | ✅ Done |
| [TASK-GCV-005](./TASK-GCV-005-cve-entity-source-migration.md) | CVE Entity Extension + DB Migration | `data-service` | CR-001 | — | ✅ Done |
| [TASK-GCV-006](./TASK-GCV-006-epss-filter-sort.md) | EPSS Filter + Sort in Search Service | `search-service` | CR-002 | — | ✅ Done |
| [TASK-GCV-007](./TASK-GCV-007-epss-stats-endpoint.md) | EPSS Stats Endpoint | `search-service` | CR-002 | TASK-GCV-006 | ✅ Done |
| [TASK-GCV-008](./TASK-GCV-008-apikey-domain-migration.md) | API Key Domain + DB Migration | `gateway-service` | CR-008 | — | ✅ Done |
| [TASK-GCV-009](./TASK-GCV-009-apikey-validator-middleware.md) | API Key Validator + Auth Middleware | `gateway-service` | CR-008 | TASK-GCV-008 | ✅ Done |
| [TASK-GCV-010](./TASK-GCV-010-gateway-health-aggregation.md) | Health Aggregation Endpoint | `gateway-service` | CR-008 | — | ✅ Done |
| [TASK-GCV-011](./TASK-GCV-011-gateway-response-cache.md) | Response Cache + Tiered Rate Limit | `gateway-service` | CR-008 | TASK-GCV-009 | ⏳ Pending |
| [TASK-GCV-012](./TASK-GCV-012-gateway-apikey-crud.md) | API Key CRUD Endpoints | `gateway-service` | CR-008 | TASK-GCV-009 | ⏳ Pending |
| [TASK-GCV-013](./TASK-GCV-013-gateway-route-table-v2.md) | Route Table v2 (all new routes) | `gateway-service` | CR-008 | TASK-GCV-011 | ⏳ Pending |

### Phase 2 — Enrichment & Observability (🟡 Medium)

| Task ID | Tên | Service | CR | Phụ thuộc | Status |
|---------|-----|---------|-----|-----------|--------|
| [TASK-GCV-014](./TASK-GCV-014-kev-entity-migration.md) | KEV Entity Extension + Migration | `data-service` | CR-007 | — | ✅ Done |
| [TASK-GCV-015](./TASK-GCV-015-kev-sync-diff-nats.md) | KEV Sync Diff Detection + NATS Publish | `data-service` | CR-007 | TASK-GCV-014 | ✅ Done |
| [TASK-GCV-016](./TASK-GCV-016-kev-ransomware-stats.md) | KEV Ransomware Endpoint + Advanced Stats | `data-service` | CR-007 | TASK-GCV-014 | ⏳ Pending |
| [TASK-GCV-017](./TASK-GCV-017-cwe-capec-taxonomy-endpoints.md) | CWE/CAPEC Taxonomy Endpoints | `search-service` | CR-003 | — | ⏳ Pending |
| [TASK-GCV-018](./TASK-GCV-018-cve-cwe-filter.md) | CVE Search CWE Filter | `search-service` | CR-003 | TASK-GCV-017 | ⏳ Pending |
| [TASK-GCV-019](./TASK-GCV-019-cpe-vendor-endpoints.md) | Vendor/Product Endpoints | `search-service` | CR-005 | — | ⏳ Pending |
| [TASK-GCV-020](./TASK-GCV-020-cve-vendor-product-filter.md) | CVE Search Vendor/Product Filter | `search-service` | CR-005 | TASK-GCV-019 | ⏳ Pending |
| [TASK-GCV-021](./TASK-GCV-021-observability-shared-pkg.md) | Observability Shared Package (zerolog + Prometheus + OTel) | `shared` | CR-009 | — | ⏳ Pending |
| [TASK-GCV-022](./TASK-GCV-022-observability-integrate-services.md) | Integrate Observability vào tất cả services | All services | CR-009 | TASK-GCV-021 | ⏳ Pending |

### Phase 3 — Advanced Search & Notifications (🟡 Medium)

| Task ID | Tên | Service | CR | Phụ thuộc | Status |
|---------|-----|---------|-----|-----------|--------|
| [TASK-GCV-023](./TASK-GCV-023-opensearch-client.md) | OpenSearch Client + Index Mapping | `search-service` | CR-004 | — | ⏳ Pending |
| [TASK-GCV-024](./TASK-GCV-024-opensearch-dual-backend.md) | Dual Backend Search (OpenSearch + PG fallback) | `search-service` | CR-004 | TASK-GCV-023 | ⏳ Pending |
| [TASK-GCV-025](./TASK-GCV-025-pgvector-semantic-search.md) | pgvector Semantic Search + AI Embedding | `search-service` | CR-004 | — | ⏳ Pending |
| [TASK-GCV-026](./TASK-GCV-026-search-new-endpoints.md) | POST /search, POST /search/semantic, GET /aggregations | `search-service` | CR-004 | TASK-GCV-024 | ⏳ Pending |
| [TASK-GCV-027](./TASK-GCV-027-notification-domain-db.md) | notification-service Domain + DB Schema | `notification-service` | CR-006 | — | ⏳ Pending |
| [TASK-GCV-028](./TASK-GCV-028-notification-webhook-usecase.md) | Webhook Registration + SSRF Protection Use Case | `notification-service` | CR-006 | TASK-GCV-027 | ⏳ Pending |
| [TASK-GCV-029](./TASK-GCV-029-notification-delivery-retry.md) | Webhook Delivery + HMAC + Retry Worker | `notification-service` | CR-006 | TASK-GCV-028 | ⏳ Pending |
| [TASK-GCV-030](./TASK-GCV-030-notification-alert-dispatch.md) | Alert Dispatch + Deduplication | `notification-service` | CR-006 | TASK-GCV-029 | ⏳ Pending |
| [TASK-GCV-031](./TASK-GCV-031-notification-http-api.md) | Notification HTTP API + Router | `notification-service` | CR-006 | TASK-GCV-030 | ⏳ Pending |
| [TASK-GCV-032](./TASK-GCV-032-data-service-notification-hook.md) | data-service Post-Sync Notification Hook | `data-service` | CR-006 | TASK-GCV-031 | ⏳ Pending |

### Phase 4 — Export & UI Support (🟢 Low)

| Task ID | Tên | Service | CR | Phụ thuộc | Status |
|---------|-----|---------|-----|-----------|--------|
| [TASK-GCV-033](./TASK-GCV-033-cve-export-csv-json.md) | CVE Export (CSV + JSON) Endpoint | `search-service` | CR-010 | — | ⏳ Pending |
| [TASK-GCV-034](./TASK-GCV-034-source-attribution.md) | Source Attribution in CVE Response | `search-service` | CR-010 | TASK-GCV-005 | ⏳ Pending |
| [TASK-GCV-035](./TASK-GCV-035-dashboard-stats-api.md) | Dashboard Stats API | `search-service` | CR-010 | — | ⏳ Pending |

---

## Dependency Graph

```
Phase 1 (Core Pipeline):
  TASK-GCV-001 → TASK-GCV-002
               → TASK-GCV-003
               → TASK-GCV-004
  TASK-GCV-005 (standalone: entity + migration)

  TASK-GCV-006 → TASK-GCV-007

  TASK-GCV-008 → TASK-GCV-009 → TASK-GCV-011 → TASK-GCV-013
               → TASK-GCV-012
  TASK-GCV-010 (standalone: health aggregation)

Phase 2 (Enrichment):
  TASK-GCV-014 → TASK-GCV-015
               → TASK-GCV-016

  TASK-GCV-017 → TASK-GCV-018
  TASK-GCV-019 → TASK-GCV-020

  TASK-GCV-021 → TASK-GCV-022

Phase 3 (Advanced Search & Notifications):
  TASK-GCV-023 → TASK-GCV-024 → TASK-GCV-026
  TASK-GCV-025 → TASK-GCV-026

  TASK-GCV-027 → TASK-GCV-028 → TASK-GCV-029 → TASK-GCV-030 → TASK-GCV-031 → TASK-GCV-032

Phase 4 (Export):
  TASK-GCV-033 (standalone)
  TASK-GCV-005 → TASK-GCV-034
  TASK-GCV-035 (standalone)
```

---

## Task Format

Mỗi task file tuân theo cấu trúc:

```markdown
# TASK-GCV-XXX — [Tên task]

## Metadata
| Field | Value |
|-------|-------|
| Task ID | TASK-GCV-XXX |
| Service | <service-name> |
| CR | CR-GCV-XXX |
| Phase | 1/2/3/4 |
| Priority | High/Medium/Low |
| Prerequisites | TASK-GCV-YYY hoặc — |

## Context
<Mô tả ngắn gọn bài toán>

## Reference
- Solution: [SOL-GCV-XXX](../solutions/SOL-GCV-XXX-....md)
- CR: [CR-GCV-XXX](../CR-GCV-XXX-....md)

## Files to Create/Modify
<Danh sách đường dẫn tuyệt đối>

## Implementation Spec
<Code mẫu hoặc spec chi tiết>

## Acceptance Criteria
<Danh sách điều kiện kiểm tra được>
```
