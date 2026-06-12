# Archive Services — Chi tiết

> Thư mục: `/archive/` — 45 services đã deprecated, được refactor hoặc split từ phiên bản cũ.

---

## Phân loại Archive

| Loại | Services | Ghi chú |
|------|----------|---------|
| **CVE/Vuln Data** | cve-service, cve-search-service, cve-sync-service, kev-service, taxonomy-service, version-index, vulnerability-query | Replaced bởi vulnerability-service + search-service |
| **Ingestion/Sync** | ingestion, ingest-service, source-sync, cve-sync-service, converter | Replaced bởi ingestion-service |
| **Scan** | scan-service-old, scan-orchestrator, scanner, agent-service, schedule-service | Replaced bởi scan-service + schedule-service |
| **Finding** | finding-management | Replaced bởi finding-service |
| **AI** | ai-enrichment, ai, ranking-service | Replaced bởi ai-service |
| **Auth/Identity** | identity, admin | Replaced bởi auth-service |
| **Asset** | asset-service, sbomvex | Absorbed vào scan-service |
| **Notification** | notification, notification-service-old, dd-notification | Replaced bởi notification-service |
| **Product** | product-management | Replaced bởi product-service |
| **Report** | report | Replaced bởi report-service |
| **Integration** | jira | Replaced bởi integration-service |
| **Gateway** | api-gateway, dd-api-gateway, web-bff, browse-service, info-service | Replaced bởi unified-gateway |
| **Impact** | impact-analysis, sla | Replaced bởi impact-service, finding-service |
| **Shared Libs** | dd-pkg, pkg, proto, dd-proto | Replaced bởi shared/ |
| **Query** | query-service-old, alias-relations | Replaced bởi query-service |
| **Other** | audit | Absorbed vào finding-service |

---

## Chi tiết từng Archive Service

---

### 1. cve-service
**Path**: `archive/cve-service/`
**Replaced by**: `vulnerability-service`

#### Domain
```
internal/domain/
├── entity/          # CVE entity
├── error/           # Domain errors
├── event/           # CVE events
├── repository/      # Repo interfaces
├── service/         # Domain services
└── valueobject/     # CVSS, severity, etc.
```
#### Layers: adapter, delivery, domain, infra, infrastructure, usecase
#### Notes: Có Dockerfile + pre-built binary `server` (16MB)

---

### 2. cve-search-service
**Path**: `archive/cve-search-service/`
**Replaced by**: `search-service`

#### Layers
```
internal/
├── adapter/         # Search engine adapter
├── delivery/        # HTTP handlers
├── domain/          # Search domain
├── subscriber/      # Event subscribers
└── usecase/         # Search use cases
```
#### Notes: Có Dockerfile, config/, migrations/

---

### 3. cve-sync-service
**Path**: `archive/cve-sync-service/`
**Replaced by**: `ingestion-service` (sync module)

#### Layers
```
internal/
├── adapter/
├── delivery/
├── domain/
└── usecase/
```

---

### 4. kev-service
**Path**: `archive/kev-service/`
**Replaced by**: `vulnerability-service` (kev domain)
**Chức năng**: CISA KEV (Known Exploited Vulnerabilities) ingestion.

#### Layers
```
internal/
├── adapter/
├── delivery/
├── domain/
└── usecase/
```
#### Notes: Có Dockerfile + config/

---

### 5. taxonomy-service
**Path**: `archive/taxonomy-service/`
**Replaced by**: `vulnerability-service` (taxonomy domain)
**Chức năng**: CWE/CPE taxonomy management.

#### Notes: Có pre-built binary (14.5MB) — không có source code visible

---

### 6. version-index
**Path**: `archive/version-index/`
**Replaced by**: `impact-service` (index domain)
**Chức năng**: Index versions của packages để match với CVE.

---

### 7. vulnerability-query
**Path**: `archive/vulnerability-query/`
**Replaced by**: `query-service`
**Chức năng**: Truy vấn vulnerability data.

---

### 8. ingestion
**Path**: `archive/ingestion/`
**Replaced by**: `ingestion-service`

#### Domain
```
internal/domain/
├── aggregate/       # Ingestion aggregate
├── entity/          # Job entities
├── event/           # Domain events
├── repository/      # Repo interfaces
└── valueobject/
```
#### Layers: application, domain, infra
#### Notes: Có Dockerfile + config/

---

### 9. ingest-service
**Path**: `archive/ingest-service/` (nếu tồn tại)
**Replaced by**: `ingestion-service`

---

### 10. source-sync
**Path**: `archive/source-sync/`
**Replaced by**: `ingestion-service` (sync module)
**Chức năng**: Sync từ external sources (GitHub Advisory, OSV.dev API).

#### Layers
```
internal/
├── application/
├── connectors/      # Source connectors
├── domain/
└── infra/
```
#### Notes: Có Dockerfile + config/ + go.sum với nhiều dependencies

---

### 11. converter
**Path**: `archive/converter/`
**Replaced by**: `ingestion-service` (converter module)
**Chức năng**: Convert giữa các CVE formats.

---

### 12. scan-service-old
**Path**: `archive/scan-service-old/`
**Replaced by**: `scan-service`

---

### 13. scan-orchestrator
**Path**: `archive/scan-orchestrator/`
**Replaced by**: `scan-service` (scheduler module)
**Chức năng**: Orchestrate scan jobs, assign to agents.

#### Layers
```
internal/
├── adapter/
├── domain/
├── infrastructure/
└── usecase/
```

---

### 14. scanner
**Path**: `archive/scanner/`
**Replaced by**: `scan-service` (scan agent domain)
**Chức năng**: Scanner agent — thực hiện scanning.

#### Domain
```
internal/domain/
├── entity/          # Scanner entities
├── repository/
└── service/
```
#### Layers: adapter, checkers, domain, infrastructure, parsers, usecase

---

### 15. agent-service
**Path**: `archive/agent-service/`
**Replaced by**: `scan-service` (agent domain)
**Chức năng**: Quản lý scanner agents.

---

### 16. schedule-service (archive)
**Path**: `archive/schedule-service/`
**Replaced by**: `schedule-service` (active)

---

### 17. finding-management
**Path**: `archive/finding-management/`
**Replaced by**: `finding-service`
**Chức năng**: Quản lý vulnerability findings.

#### Layers
```
internal/
├── domain/
└── usecase/
```

---

### 18. ai-enrichment
**Path**: `archive/ai-enrichment/`
**Replaced by**: `ai-service`
**Chức năng**: AI enrichment cho CVE data.

#### Layers
```
internal/
├── application/
├── domain/
└── infra/
```
#### Notes: Có interface/ directory cho AI provider interfaces

---

### 19. ai
**Path**: `archive/ai/`
**Replaced by**: `ai-service`

---

### 20. ranking-service
**Path**: `archive/ranking-service/`
**Replaced by**: `ai-service` (EPSS/severity scoring)
**Chức năng**: Vulnerability risk ranking.
#### Notes: Có pre-built binary (14.6MB)

---

### 21. identity
**Path**: `archive/identity/`
**Replaced by**: `auth-service`
**Chức năng**: Identity management — users, roles, permissions.

#### Domain
```
internal/domain/
├── entity/
├── error/
├── event/
├── repository/
└── valueobject/
```
#### Layers: domain, infra, infrastructure, usecase
#### Notes: Có Dockerfile + Makefile

---

### 22. admin
**Path**: `archive/admin/`
**Replaced by**: `auth-service` (admin endpoints)
**Chức năng**: Admin panel backend.

---

### 23. asset-service
**Path**: `archive/asset-service/`
**Replaced by**: `scan-service` (asset domain) + `product-service`
**Chức năng**: Quản lý assets (applications, containers, hosts).

#### Domain
```
internal/domain/
├── entity/
├── error/
├── event/
├── repository/
└── valueobject/
```
#### Layers: domain, infrastructure, usecase
#### Notes: Có Dockerfile + pre-built binary (16MB)

---

### 24. sbomvex
**Path**: `archive/sbomvex/`
**Replaced by**: `scan-service` (sbom module)
**Chức năng**: SBOM (Software Bill of Materials) và VEX processing.

---

### 25. notification
**Path**: `archive/notification/`
**Replaced by**: `notification-service`

#### Domain
```
internal/domain/
├── aggregate/
├── alert/
├── delivery/
└── rule/
```
#### Layers: application, domain, infra, infrastructure, usecase
#### Notes: Có Dockerfile + Makefile

---

### 26. notification-service-old
**Path**: `archive/notification-service-old/`
**Replaced by**: `notification-service`

---

### 27. dd-notification
**Path**: `archive/dd-notification/`
**Replaced by**: `notification-service`
**Chức năng**: DefectDojo-compatible notifications.

---

### 28. product-management
**Path**: `archive/product-management/`
**Replaced by**: `product-service`
**Chức năng**: Product/engagement management.

#### Layers
```
internal/
├── domain/
└── usecase/
```

---

### 29. report
**Path**: `archive/report/`
**Replaced by**: `report-service`

#### Layers
```
internal/
├── domain/
├── infrastructure/
└── usecase/
```

---

### 30. jira
**Path**: `archive/jira/`
**Replaced by**: `integration-service`
**Chức năng**: Jira ticket integration cho findings.

---

### 31. api-gateway
**Path**: `archive/api-gateway/`
**Replaced by**: `unified-gateway`
**Chức năng**: Old API gateway.

---

### 32. dd-api-gateway
**Path**: `archive/dd-api-gateway/`
**Replaced by**: `unified-gateway`
**Chức năng**: DefectDojo-compatible API gateway.

---

### 33. web-bff
**Path**: `archive/web-bff/`
**Replaced by**: `unified-gateway` (bff module)
**Chức năng**: Backend for Frontend aggregation.

---

### 34. browse-service
**Path**: `archive/browse-service/`
**Replaced by**: `unified-gateway` hoặc `search-service`
**Chức năng**: CVE browsing and discovery.

---

### 35. info-service
**Path**: `archive/info-service/`
**Replaced by**: `vulnerability-service` hoặc `unified-gateway`

---

### 36. impact-analysis
**Path**: `archive/impact-analysis/`
**Replaced by**: `impact-service`
**Chức năng**: Analyse impact của CVE trên portfolio.

---

### 37. sla
**Path**: `archive/sla/`
**Replaced by**: `finding-service` (sla domain)
**Chức năng**: SLA tracking cho vulnerability remediation.

---

### 38. audit
**Path**: `archive/audit/`
**Replaced by**: `finding-service` (audit domain)
**Chức năng**: Audit logs cho actions.

---

### 39. dd-pkg
**Path**: `archive/dd-pkg/`
**Replaced by**: `shared/pkg`
**Chức năng**: DefectDojo shared packages.

---

### 40. pkg
**Path**: `archive/pkg/`
**Replaced by**: `shared/pkg`
**Chức năng**: Common utility packages.

---

### 41. proto
**Path**: `archive/proto/`
**Replaced by**: `shared/proto`
**Chức năng**: Old proto definitions.

---

### 42. dd-proto
**Path**: `archive/dd-proto/`
**Replaced by**: `shared/proto`
**Chức năng**: DefectDojo proto definitions.

---

### 43. query-service-old
**Path**: `archive/query-service-old/`
**Replaced by**: `query-service`

---

### 44. alias-relations
**Path**: `archive/alias-relations/`
**Replaced by**: `vulnerability-service` (alias domain)
**Chức năng**: CVE alias và cross-reference management.

---

### 45. search
**Path**: `archive/search/`
**Replaced by**: `search-service`

---

## Tóm tắt Migration Trail

```
archive/identity         → services/auth-service
archive/admin            → services/auth-service

archive/cve-service      → services/vulnerability-service
archive/taxonomy-service → services/vulnerability-service
archive/kev-service      → services/vulnerability-service
archive/alias-relations  → services/vulnerability-service
archive/version-index    → services/impact-service

archive/ingestion        → services/ingestion-service
archive/source-sync      → services/ingestion-service
archive/converter        → services/ingestion-service
archive/cve-sync-service → services/ingestion-service

archive/scan-service-old   → services/scan-service
archive/scan-orchestrator  → services/scan-service
archive/scanner            → services/scan-service
archive/agent-service      → services/scan-service
archive/asset-service      → services/scan-service
archive/sbomvex            → services/scan-service
archive/schedule-service   → services/schedule-service (hoặc scan-service)

archive/finding-management → services/finding-service
archive/sla                → services/finding-service
archive/audit              → services/finding-service

archive/ai-enrichment    → services/ai-service
archive/ai               → services/ai-service
archive/ranking-service  → services/ai-service

archive/notification              → services/notification-service
archive/notification-service-old → services/notification-service
archive/dd-notification           → services/notification-service

archive/product-management → services/product-service
archive/report             → services/report-service
archive/jira               → services/integration-service

archive/api-gateway   → services/unified-gateway
archive/dd-api-gateway → services/unified-gateway
archive/web-bff        → services/unified-gateway
archive/browse-service → services/unified-gateway

archive/cve-search-service → services/search-service
archive/search             → services/search-service
archive/vulnerability-query → services/query-service
archive/query-service-old  → services/query-service

archive/dd-pkg  → services/shared/pkg
archive/pkg     → services/shared/pkg
archive/proto   → services/shared/proto
archive/dd-proto → services/shared/proto
```
