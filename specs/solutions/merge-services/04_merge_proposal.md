# Đề xuất Merge — ≤ 10 Core Services

> **Mục tiêu**: Từ 17 active services → 8 core services + 1 shared layer.
> Nguyên tắc: mỗi service có 1 bounded context rõ ràng, Clean Architecture.

---

## Mapping: 17 → 8 Services

| # | Core Service (mới) | Merge từ (active) | Gộp thêm từ (archive) |
|---|-------------------|-------------------|----------------------|
| 1 | **identity-service** | auth-service | identity, admin |
| 2 | **data-service** | vulnerability-service, ingestion-service | cve-service, ingestion, source-sync, kev-service, taxonomy-service, cve-sync-service, converter, alias-relations, version-index |
| 3 | **search-service** | search-service, query-service, dd-search | cve-search-service, search, query-service-old, vulnerability-query, browse-service |
| 4 | **scan-service** | scan-service, schedule-service | scan-orchestrator, scanner, agent-service, asset-service, sbomvex, scan-service-old, schedule-service(archive) |
| 5 | **finding-service** | finding-service | finding-management, sla, audit |
| 6 | **ai-service** | ai-service | ai-enrichment, ai, ranking-service |
| 7 | **notification-service** | notification-service, integration-service | notification, notification-service-old, dd-notification, jira |
| 8 | **gateway-service** | unified-gateway | api-gateway, dd-api-gateway, web-bff, info-service |
| - | *(tách riêng)* | report-service, product-service | report, product-management |

> **Lưu ý**: `report-service` và `product-service` có thể merge vào `finding-service` hoặc giữ riêng tùy độ phức tạp. Tổng sẽ là **8-10 services**.

---

## Chi tiết từng Core Service

---

### 1. identity-service

**Bounded Context**: Identity & Access Management (IAM)
**Merge từ**: `auth-service`

#### Chức năng
- Xác thực: login, logout, register
- JWT access/refresh tokens
- OAuth2 (Google, GitHub, SSO)
- 2FA / TOTP
- API Key management
- RBAC (roles & permissions)

#### Clean Architecture Layout
```
identity-service/
├── cmd/server/
├── internal/
│   ├── domain/
│   │   ├── user/           # User aggregate
│   │   ├── token/          # Token value objects
│   │   ├── apikey/         # APIKey aggregate
│   │   ├── role/           # Role & permission
│   │   └── repository/     # Interfaces
│   ├── usecase/
│   │   ├── register/
│   │   ├── login/
│   │   ├── logout/
│   │   ├── oauth/
│   │   ├── refresh_token/
│   │   ├── validate_token/
│   │   └── manage_api_key/
│   ├── delivery/
│   │   ├── grpc/           # gRPC handlers
│   │   └── http/           # HTTP handlers
│   └── infra/
│       ├── postgres/       # User, token storage
│       ├── redis/          # Session/token cache
│       └── mongo/          # (optional)
├── migrations/
├── go.mod
└── Dockerfile
```

#### APIs
- `POST /auth/register`
- `POST /auth/login`
- `POST /auth/logout`
- `POST /auth/refresh`
- `GET  /auth/oauth/{provider}`
- `POST /auth/validate` (internal gRPC)
- `CRUD /auth/api-keys`

---

### 2. data-service

**Bounded Context**: Vulnerability Data Management
**Merge từ**: `vulnerability-service` + `ingestion-service`

#### Chức năng
- **Store**: Lưu trữ và quản lý CVE database (CRUD)
- **Ingest**: Thu thập từ NVD, OSV, GHSA, GitHub Advisory
- **Enrich**: KEV status, CWE taxonomy, alias resolution
- **Sync**: Incremental + full sync từ upstream sources
- **Publish**: Emit events khi có CVE mới/update

#### Clean Architecture Layout
```
data-service/
├── cmd/server/
├── internal/
│   ├── domain/
│   │   ├── cve/            # CVE aggregate root
│   │   │   ├── entity.go
│   │   │   ├── repository.go
│   │   │   └── events.go
│   │   ├── kev/            # CISA KEV domain
│   │   ├── taxonomy/       # CWE/CPE taxonomy
│   │   ├── alias/          # CVE aliases (CWE, GHSA, etc.)
│   │   ├── source/         # Data source definitions
│   │   └── errors/
│   ├── usecase/
│   │   ├── ingest/         # Run ingestion jobs
│   │   ├── sync/           # Sync from upstream
│   │   ├── update_cve/     # Update CVE data
│   │   ├── resolve_alias/  # Resolve CVE aliases
│   │   └── manage_kev/     # KEV CRUD
│   ├── delivery/
│   │   ├── grpc/           # gRPC CVE API
│   │   └── http/           # REST API
│   ├── infra/
│   │   ├── postgres/       # Primary storage
│   │   ├── mongo/          # Document store
│   │   ├── firestore/      # Raw data cache
│   │   ├── gcs/            # Large dataset storage
│   │   └── nats/           # Event publisher
│   └── fetcher/            # Source-specific HTTP fetchers
│       ├── nvd/
│       ├── osv/
│       ├── ghsa/
│       └── github/
├── migrations/
├── go.mod
└── Dockerfile
```

#### APIs
- `GET  /cve/{id}` — Get CVE details
- `POST /cve/query` — Batch query CVEs
- `GET  /cve/{id}/kev` — KEV status
- `GET  /cve/{id}/aliases` — Get aliases
- `POST /admin/sync` — Trigger sync job
- gRPC: `CVEService`, `DataSyncService`

---

### 3. search-service

**Bounded Context**: Search & Discovery
**Merge từ**: `search-service` + `query-service` + `dd-search`

#### Chức năng
- Full-text search CVE descriptions
- Faceted filtering (severity, ecosystem, date, CVSS)
- Aggregations & statistics
- Autocomplete suggestions
- Advanced query DSL

#### Clean Architecture Layout
```
search-service/
├── cmd/server/
├── internal/
│   ├── domain/
│   │   ├── query/          # Search query entities
│   │   ├── result/         # Search result entities
│   │   ├── filter/         # Filter value objects
│   │   └── repository/     # Search engine interfaces
│   ├── usecase/
│   │   ├── search_cve/
│   │   ├── aggregate/      # Statistics & aggregations
│   │   └── suggest/        # Autocomplete
│   ├── delivery/
│   │   ├── grpc/
│   │   └── http/
│   └── infra/
│       ├── elasticsearch/  # Primary search engine
│       ├── postgres/       # Fallback / relational queries
│       └── redis/          # Query result cache
├── go.mod
└── Dockerfile
```

#### APIs
- `POST /search` — Full-text search
- `POST /search/filter` — Filtered query
- `GET  /search/suggest` — Autocomplete
- `POST /search/aggregate` — Statistics
- gRPC: `SearchService`

---

### 4. scan-service

**Bounded Context**: Vulnerability Scanning Orchestration
**Merge từ**: `scan-service` + `schedule-service`

#### Chức năng
- **Asset management**: Quản lý software assets, containers, hosts
- **Scan jobs**: Tạo và điều phối scan jobs
- **Agent management**: Đăng ký, heartbeat, task assignment cho scanner agents
- **SBOM processing**: Phân tích Software Bill of Materials
- **Schedule**: Cron-based recurring scans

#### Clean Architecture Layout
```
scan-service/
├── cmd/server/
├── internal/
│   ├── domain/
│   │   ├── asset/          # Asset aggregate
│   │   ├── scan/           # Scan job aggregate
│   │   ├── agent/          # Scanner agent entity
│   │   ├── schedule/       # Scan schedule aggregate
│   │   └── sbom/           # SBOM entities
│   ├── usecase/
│   │   ├── register_asset/
│   │   ├── initiate_scan/
│   │   ├── assign_to_agent/
│   │   ├── update_scan_status/
│   │   ├── process_sbom/
│   │   ├── create_schedule/
│   │   └── trigger_scheduled_scan/
│   ├── delivery/
│   │   ├── grpc/           # Scanner agent gRPC
│   │   └── http/           # REST API
│   ├── infra/
│   │   ├── postgres/
│   │   ├── redis/          # Job queue, agent state
│   │   └── nats/           # Scan events
│   └── scheduler/          # Cron runner
├── migrations/
├── go.mod
└── Dockerfile
```

#### APIs
- `CRUD /assets` — Asset management
- `POST /scans` — Initiate scan
- `GET  /scans/{id}` — Scan status
- `CRUD /schedules` — Recurring schedules
- gRPC: `ScanService`, `ScannerAgentService`

---

### 5. finding-service

**Bounded Context**: Vulnerability Findings & Remediation
**Merge từ**: `finding-service` + `report-service` + `product-service`

#### Chức năng
- **Finding**: Track vulnerability findings (CVE + Asset correlation)
- **Product management**: Products, engagements, test sessions
- **SLA**: Policy và SLA tracking cho remediation
- **Audit**: Full audit trail của mọi thay đổi
- **Reports**: Generate vulnerability reports (PDF/JSON/Excel)

#### Clean Architecture Layout
```
finding-service/
├── cmd/server/
├── internal/
│   ├── domain/
│   │   ├── finding/        # Finding aggregate
│   │   ├── product/        # Product aggregate
│   │   ├── engagement/     # Engagement entity
│   │   ├── sla/            # SLA policy entity
│   │   ├── audit/          # Audit log entity
│   │   └── report/         # Report entity
│   ├── usecase/
│   │   ├── create_finding/
│   │   ├── update_finding/
│   │   ├── resolve_finding/
│   │   ├── track_sla/
│   │   ├── audit_action/
│   │   ├── manage_product/
│   │   └── generate_report/
│   ├── delivery/
│   │   ├── grpc/
│   │   └── http/
│   ├── infra/
│   │   ├── postgres/
│   │   ├── mongo/
│   │   └── nats/
│   └── formatters/         # Report formatters (PDF, Excel, JSON)
├── migrations/
├── go.mod
└── Dockerfile
```

#### APIs
- `CRUD /findings` — Finding management
- `CRUD /products` — Product management
- `GET  /findings/{id}/audit` — Audit trail
- `GET  /sla/status` — SLA dashboard
- `POST /reports/generate` — Generate report
- gRPC: `FindingService`, `ProductService`, `ReportService`

---

### 6. ai-service

**Bounded Context**: AI/ML Enrichment
**Merge từ**: `ai-service` (đã đầy đủ)

#### Chức năng
- CVE AI enrichment (description, impact, remediation)
- EPSS score calculation
- MITRE ATT&CK tagging
- Severity classification (ML-based)
- Exploit detection
- Threat intelligence correlation
- Vector embeddings cho semantic search

#### Clean Architecture Layout
```
ai-service/
├── cmd/server/
├── internal/
│   ├── domain/
│   │   ├── enrichment/
│   │   │   ├── embedding_service.go
│   │   │   ├── severity_classifier.go
│   │   │   ├── provider_chain.go
│   │   │   ├── exploit/
│   │   │   ├── mitretagger/
│   │   │   ├── threatintel/
│   │   │   └── port/       # AI provider interfaces
│   │   └── triage/         # AI triage logic
│   ├── usecase/
│   │   ├── enrich_cve/
│   │   ├── epss/
│   │   └── triage_finding/
│   ├── delivery/
│   │   └── grpc/
│   └── infra/
│       ├── firestore/
│       ├── redis/
│       └── providers/      # AI provider implementations
├── go.mod
└── Dockerfile
```

#### APIs
- gRPC: `AIEnrichmentService`
- `POST /enrich/{cve_id}` — Enrich CVE
- `GET  /epss/{cve_id}` — Get EPSS score
- `POST /triage/finding` — AI triage suggestion

---

### 7. notification-service

**Bounded Context**: Notifications & Integrations
**Merge từ**: `notification-service` + `integration-service`

#### Chức năng
- **Rules**: Rule engine — when to send notifications
- **Alerts**: Create & track alerts
- **Subscriptions**: User topic subscriptions
- **Delivery**: Email, webhook, Slack, Teams
- **Integrations**: Jira ticket creation, sync
- **Webhooks**: Outbound webhook management

#### Clean Architecture Layout
```
notification-service/
├── cmd/server/
├── internal/
│   ├── domain/
│   │   ├── rule/           # Notification rule entity
│   │   ├── alert/          # Alert entity
│   │   ├── subscription/   # User subscription
│   │   ├── webhook/        # Webhook config
│   │   ├── delivery/       # Delivery channel types
│   │   └── integration/    # Integration entities (Jira, etc.)
│   ├── usecase/
│   │   ├── evaluate_rules/
│   │   ├── send_alert/
│   │   ├── manage_subscription/
│   │   ├── manage_webhook/
│   │   └── jira_sync/
│   ├── delivery/
│   │   ├── grpc/
│   │   └── http/
│   ├── infra/
│   │   ├── postgres/
│   │   ├── nats/           # Subscribe to events
│   │   └── adapters/       # Email, Slack, webhook senders
│   └── integrations/
│       └── jira/           # Jira API client
├── migrations/
├── go.mod
└── Dockerfile
```

#### APIs
- `CRUD /rules` — Notification rules
- `CRUD /subscriptions` — User subscriptions
- `CRUD /webhooks` — Webhook configs
- `CRUD /integrations/jira` — Jira configuration
- gRPC: `NotificationService`

---

### 8. gateway-service

**Bounded Context**: API Gateway & BFF
**Merge từ**: `unified-gateway`

#### Chức năng
- Routing tất cả external requests đến đúng service
- Auth validation (validate JWT với identity-service)
- Rate limiting
- BFF (Backend for Frontend) aggregation
- Health checks

#### Clean Architecture Layout
```
gateway-service/
├── cmd/server/
├── internal/
│   ├── domain/
│   │   ├── auth/           # Auth validation entities
│   │   ├── policy/         # Access control policies
│   │   └── entity/         # Gateway entities
│   ├── usecase/
│   │   ├── authenticate/
│   │   ├── authorize/
│   │   └── aggregate_bff/  # BFF data aggregation
│   ├── delivery/
│   │   └── http/           # Route definitions
│   ├── proxy/              # Reverse proxy
│   ├── ratelimit/          # Rate limiter (Redis-backed)
│   └── health/             # Health endpoints
├── config/                 # Route configuration
├── go.mod
└── Dockerfile
```

#### Route Mapping
```
/api/auth/*     → identity-service
/api/cve/*      → data-service
/api/search/*   → search-service
/api/scan/*     → scan-service
/api/findings/* → finding-service
/api/ai/*       → ai-service
/api/alerts/*   → notification-service
/api/reports/*  → finding-service
```

---

## Tổng kết: 17 → 8 Services

```
services/ (before)          services/ (after)
─────────────────           ─────────────────
auth-service            ──► identity-service
vulnerability-service   ┐
ingestion-service       ├─► data-service
                        ┘
search-service          ┐
query-service           ├─► search-service
dd-search               ┘
scan-service            ┐
schedule-service        ├─► scan-service
                        ┘
finding-service         ┐
product-service         ├─► finding-service
report-service          ┘
ai-service              ──► ai-service
notification-service    ┐
integration-service     ├─► notification-service
                        ┘
unified-gateway         ──► gateway-service

shared/                 ──► shared/ (unchanged)
```

---

## Service Communication Matrix

```
                    ┌──────────────────────────────────────────────────┐
                    │              gateway-service                     │
                    │    (single entry point for all external calls)   │
                    └──────┬──────┬──────┬──────┬──────┬──────┬───────┘
                           │      │      │      │      │      │
              ┌────────────▼┐  ┌──▼──┐  ┌▼──┐  ┌▼──┐  ┌▼──┐  ┌▼──────┐
              │  identity-  │  │data-│  │sea│  │sca│  │fin│  │ai-│  │notif-│
              │  service    │  │serv │  │rch│  │n- │  │din│  │ser│  │icati-│
              │             │  │ice  │  │-sv│  │svc│  │g- │  │vic│  │on-sv │
              └─────────────┘  └─────┘  └───┘  └───┘  └───┘  └───┘  └──────┘
                                  │        │      │      │              │
                                  └────────┴──────┴──────┘              │
                                           │ NATS events                │
                                           └────────────────────────────┘
```

---

## Dependencies giữa services

| Service | Phụ thuộc vào |
|---------|--------------|
| identity-service | (standalone) |
| data-service | (standalone, publishes events) |
| search-service | data-service (via events/gRPC) |
| scan-service | identity-service, data-service |
| finding-service | scan-service, data-service, identity-service |
| ai-service | data-service (subscriptions) |
| notification-service | finding-service, scan-service (events) |
| gateway-service | identity-service (token validation), all services |

---

## Databases per Service

| Service | PostgreSQL | MongoDB | Redis | Firestore | NATS |
|---------|-----------|---------|-------|-----------|------|
| identity-service | ✅ users, tokens | ✅ | ✅ session | - | - |
| data-service | ✅ cve, kev | ✅ raw docs | - | ✅ cache | ✅ pub |
| search-service | - | - | ✅ cache | - | ✅ sub |
| scan-service | ✅ scans, assets | - | ✅ queue | - | ✅ pub/sub |
| finding-service | ✅ findings, sla | ✅ | - | - | ✅ sub |
| ai-service | - | - | ✅ cache | ✅ results | ✅ sub |
| notification-service | ✅ rules, subs | - | - | - | ✅ sub |
| gateway-service | - | - | ✅ rate-limit | - | - |
