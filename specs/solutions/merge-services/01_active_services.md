# Active Services — Chi tiết

> Thư mục: `/services/` | Go module prefix: `github.com/osv/<name>`

---

## 1. auth-service

**Module**: `github.com/osv/auth-service`
**Chức năng**: Xác thực người dùng, quản lý session và API key.

### Domain Layer
```
internal/domain/
├── entity/          # User, Token, APIKey, Session entities
├── error/           # Domain errors
├── event/           # Auth events
├── identity/        # Identity value objects
├── repository/      # Repository interfaces
└── valueobject/     # Email, Password, Role, etc.
```

### Use Cases
```
internal/usecase/
├── login/           # Đăng nhập username/password
├── logout/          # Huỷ session
├── manage_api_key/  # CRUD API key
├── oauth/           # OAuth2 flow (Google, GitHub, etc.)
├── refresh_token/   # Làm mới JWT access token
├── register/        # Đăng ký tài khoản
└── validate_token/  # Kiểm tra JWT token validity
```

### Infrastructure
```
internal/infra/          # PostgreSQL, MongoDB, Redis adapters
internal/infrastructure/ # HTTP server setup
internal/provider/       # OAuth2 providers
```

### Key Dependencies
- `github.com/golang-jwt/jwt/v5` — JWT tokens
- `golang.org/x/crypto` — Password hashing (bcrypt)
- `golang.org/x/oauth2` — OAuth2 flows
- `github.com/pquerna/otp` — TOTP/2FA
- `pgx/v5` — PostgreSQL
- `mongo-driver` — MongoDB
- `go-redis/v9` — Session/token cache

### Migrations
- Có thư mục `migrations/` (DB schema cho user, tokens, api_keys)

---

## 2. ai-service

**Module**: `github.com/osv/ai-service`
**Chức năng**: AI-powered enrichment — tự động phân tích CVE, đánh giá severity, gắn MITRE tags, tính EPSS score.

### Domain Layer
```
internal/domain/
├── enrichment/
│   ├── embedding_service.go     # Vector embedding cho CVE descriptions
│   ├── provider_chain.go        # Chain of AI providers (fallback logic)
│   ├── severity_classifier.go   # ML-based severity classification
│   ├── exploit/                 # Exploit detection & analysis
│   ├── mitretagger/             # MITRE ATT&CK tagging
│   ├── port/                    # Domain ports (interfaces)
│   └── threatintel/             # Threat intelligence enrichment
└── triage/                      # (empty — planned)
```

### Use Cases
```
internal/usecase/
├── enrich_cve/       # Enrich CVE with AI analysis
├── epss/             # Calculate EPSS (Exploit Prediction Scoring)
└── triage_finding/   # AI-assisted finding triage
```

### Infrastructure
```
internal/infra/   # Firestore, Redis, gRPC clients
```

### Key Dependencies
- `cloud.google.com/go/firestore` — Store enrichment results
- `go-redis/v9` — Caching enriched data
- `nats.go` — Event-driven triggers
- `google.golang.org/grpc` — gRPC API
- `golang.org/x/oauth2` — Google AI API auth

---

## 3. vulnerability-service

**Module**: `github.com/osv/vulnerability-service`
**Chức năng**: Core vulnerability database — quản lý toàn bộ dữ liệu CVE, KEV, taxonomy, alias.

### Domain Layer
```
internal/domain/
├── aggregate/       # CVE aggregate root
├── alias/           # CVE alias relationships (CWE, GHSA, etc.)
├── cve/             # CVE entity definitions
├── entity/          # Shared entities
├── errors/          # Domain errors
├── kev/             # CISA KEV (Known Exploited Vulnerabilities)
├── repository/      # Repository interfaces
├── service/         # Domain services
├── taxonomy/        # CWE, CPE taxonomy
└── valueobject/     # CVSS scores, severity, ecosystem
```

### Layers
```
internal/
├── adapter/         # gRPC/HTTP adapters
├── application/     # Application service orchestration
├── delivery/        # HTTP handlers
├── domain/          # (above)
├── infra/           # DB implementations
└── usecase/         # Business use cases
```

### Key Dependencies
- `pgx/v5` — PostgreSQL (primary store)
- `mongo-driver` — MongoDB (document store)
- `nats.go` — Event publishing
- `shared/proto` — gRPC contracts
- `robfig/cron` — Scheduled sync triggers
- `go-chi` — HTTP API

### Migrations
- Có thư mục `migrations/` cho schema CVE database

---

## 4. ingestion-service

**Module**: `github.com/osv/ingestion-service`
**Chức năng**: Data ingestion pipeline — thu thập CVE từ NVD, OSV, GHSA, GitHub và các nguồn khác.

### Domain Layer
```
internal/domain/
├── aggregate/       # Ingestion job aggregate
├── cve5/            # CVE 5.x format handling
├── entity/          # IngestionJob, DataSource entities
├── errors/          # Domain errors
├── errors.go        
├── event/           # Ingestion events (started, completed, failed)
├── repository/      # Repository interfaces
├── service/         # Domain services
└── valueobject/     # Source type, status, etc.
```

### Additional Layers
```
internal/
├── adapter/         # Source adapters (NVD, OSV, GitHub)
├── application/     # Pipeline orchestration
├── converter/       # Format converters (CVE JSON → domain)
├── delivery/        # HTTP/gRPC handlers
├── fetcher/         # HTTP fetchers for data sources
├── infra/           # PostgreSQL, Firestore, GCS implementations
├── pipeline/        # ETL pipeline stages
├── sync/            # Incremental/full sync logic
└── usecase/         # Ingest, sync, convert use cases
```

### Key Dependencies
- `cloud.google.com/go/firestore` — Raw data storage
- `cloud.google.com/go/storage` — GCS for large datasets
- `pgx/v5` — PostgreSQL (processed data)
- `mongo-driver` — MongoDB
- `nats.go` — Event streaming
- `shared/proto` — gRPC
- `robfig/cron` — Scheduled ingestion
- `ossf/osv-schema` — OSV JSON schema validation

---

## 5. finding-service

**Module**: `github.com/osv/finding-service`
**Chức năng**: Quản lý vulnerability findings — tracking, SLA, audit trail.

### Domain Layer
```
internal/domain/
├── audit/           # Audit log entries
├── finding/         # Finding entity (vulnerability + asset + status)
└── sla/             # SLA policies and tracking
```

### Layers
```
internal/
├── delivery/        # HTTP/gRPC handlers
├── domain/          # (above)
├── infra/           # DB implementations
└── usecase/         # Create/update/resolve findings, SLA tracking
```

### Key Dependencies
- `pgx/v5`, `mongo-driver` — Data stores
- `nats.go` — Event streaming
- `shared/proto` — gRPC contracts

### Migrations
- Có thư mục `migrations/`

---

## 6. scan-service

**Module**: `github.com/osv/scan-service`
**Chức năng**: Quản lý và điều phối scanning — agent management, asset tracking, scan jobs.

### Domain Layer
```
internal/domain/
├── agent/           # Scanner agent management
├── asset/           # Asset (software, container, host) entities
├── entity/          # Shared entities
├── repository/      # Repository interfaces
├── scan/            # Scan job entity
└── schedule/        # Scan schedule entity
```

### Additional Layers
```
internal/
├── adapters/        # External service adapters
├── delivery/        # HTTP/gRPC handlers
├── infra/           # PostgreSQL, Redis implementations
├── infrastructure/  # Server configuration
├── parsers/         # SBOM, CycloneDX, SPDX parsers
├── sbom/            # SBOM processing
├── scheduler/       # Cron-based scan triggering
└── usecase/         # Initiate scan, update status, manage agents
```

### Key Dependencies
- `pgx/v5` — PostgreSQL
- `go-redis/v9` — Job queue / state
- `nats.go` — Scan event streaming
- `shared/proto` — gRPC
- `go-chi` — HTTP API
- `robfig/cron` — Internal scheduling

---

## 7. schedule-service

**Module**: `github.com/osv/schedule-service`
**Chức năng**: Quản lý recurring scan schedules (cron expressions).

### Domain Layer
```
internal/domain/
└── schedule/
    └── entity.go    # Schedule aggregate (ID, CronExpr, Type, Status, TargetIDs)
```

### Schedule Entity
```go
type Schedule struct {
    ID          uuid.UUID
    Name        string
    CronExpr    string        // "0 2 * * *"
    Type        ScheduleType  // full_scan | incremental_scan | targeted_scan
    TargetIDs   []string      // product/asset IDs
    Status      ScheduleStatus // active | paused | disabled
    LastRunAt   *time.Time
    NextRunAt   *time.Time
}
```

### Layers
```
internal/
├── domain/usecase/   # CRUD schedules, pause/resume
```

### Key Dependency
- `github.com/google/uuid`

> **Note**: Service nhỏ, likely candidate để merge vào **scan-service**.

---

## 8. product-service

**Module**: `github.com/osv/product-service`
**Chức năng**: Quản lý products, engagements, tests (DefectDojo-style product management).

### Domain Layer
```
internal/domain/
├── engagement/      # Engagement (scanning campaign)
├── orchestrator/    # Orchestration logic
├── product/         # Product entity (application/system being scanned)
├── product_type/    # Product categorization
├── repository/      # Repository interfaces
└── test/            # Test (scan session) entities
```

### Layers
```
internal/
├── delivery/        # HTTP/gRPC handlers
├── domain/          # (above)
├── infra/           # DB implementations
└── usecase/         # CRUD products, engagements, tests
```

### Migrations
- Có thư mục `migrations/`

---

## 9. impact-service

**Module**: `github.com/osv/impact-service`
**Chức năng**: Phân tích impact của CVE đến assets — version matching, affected packages.

### Domain Layer
```
internal/domain/
├── impact/          # Impact analysis entities
└── index/           # Version index for fast lookup
```

### Layers
```
internal/
├── domain/
├── infra/           # DB implementations
└── usecase/         # Calculate impact, index versions
```

---

## 10. notification-service

**Module**: `github.com/osv/notification-service`
**Chức năng**: Alert và notification — webhook, email, rule engine, subscriptions.

### Domain Layer
```
internal/domain/
├── aggregate/       # Notification aggregate
├── alert/           # Alert entity (triggered notification)
├── delivery/        # Delivery channels (email, webhook, Slack)
├── errors/          # Domain errors
├── repository/      # Repository interfaces
├── rule/            # Rule engine (when to notify)
├── subscription/    # User subscriptions to topics
└── webhook/         # Webhook configuration
```

### Layers
```
internal/
├── adapter/         # External delivery adapters
├── delivery/        # HTTP handlers
├── domain/          # (above)
├── infra/           # DB implementations
└── usecase/         # Send alerts, manage subscriptions, webhooks
```

### Migrations
- Có thư mục `migrations/`

---

## 11. report-service

**Module**: `github.com/osv/report-service`
**Chức năng**: Tạo báo cáo vulnerability theo nhiều format.

### Domain Layer
```
internal/domain/
├── entity/          # Report entity
└── service/         # Report generation service
```

### Layers
```
internal/
├── adapter/         # Format adapters
├── domain/          # (above)
├── formatters/      # PDF, JSON, Excel, HTML formatters
├── infrastructure/  # File storage
└── usecase/         # Generate report, export, schedule report
```

### Key Notes
- Có file binary `server` (~8.98MB) — pre-built binary trong repo
- Có `Dockerfile` và `migrations/`

---

## 12. search-service

**Module**: `github.com/osv/search-service`
**Chức năng**: Full-text search và faceted search cho CVE/vulnerabilities.

### Domain Layer
```
internal/domain/
├── entity/          # SearchQuery, SearchResult entities
├── errors/          # Search errors
├── repository/      # Search repository interfaces
├── service/         # Search domain service
└── valueobject/     # Filter, sort, pagination
```

### Layers
```
internal/
├── application/     # Application orchestration
├── delivery/        # HTTP/gRPC handlers
├── domain/          # (above)
├── factory/         # Search engine factory
├── infra/           # Elasticsearch/OpenSearch adapter
└── usecase/         # Search CVE, filter, autocomplete
```

---

## 13. query-service

**Module**: `github.com/osv/query-service`
**Chức năng**: Complex query và aggregation cho vulnerability data.

### Domain Layer
```
internal/domain/
├── entity/          # Query entities
├── repository/      # Query repository interfaces
└── valueobject/     # Query parameters, aggregations
```

### Layers
```
internal/
├── delivery/        # HTTP/gRPC handlers
├── domain/          # (above)
├── infra/           # DB query implementations
└── usecase/         # Execute queries, aggregations, statistics
```

### Key Dependencies
- Heavy dependency list (`go.sum` 17KB) — many query/analytics libs

---

## 14. integration-service

**Module**: `github.com/osv/integration-service`
**Chức năng**: Tích hợp với external tools — hiện tại: Jira.

### Layers
```
internal/
├── delivery/        # HTTP handlers
└── jira/            # Jira integration logic
```

> **Note**: Service rất nhỏ, likely candidate để merge vào **notification-service** hoặc tạo dedicated integration layer.

---

## 15. unified-gateway

**Module**: `github.com/osv/unified-gateway`
**Chức năng**: API Gateway — routing, auth validation, rate limiting, BFF (Backend for Frontend).

### Layers
```
internal/
├── adapter/         # Service adapters
├── auth/            # Auth middleware/validation
├── bff/             # BFF aggregation handlers
├── delivery/        # Route definitions
├── domain/
│   ├── auth/        # Auth domain
│   ├── entity/      # Gateway entities
│   └── policy/      # Access policies
├── health/          # Health check endpoints
├── proxy/           # Reverse proxy logic
├── ratelimit/       # Rate limiting
└── usecase/         # Gateway use cases
```

---

## 16. dd-search

**Module**: `github.com/osv/dd-search`
**Chức năng**: DefectDojo search adapter.

### Layers
```
internal/
├── infrastructure/  # Search backend (ES/PG)
└── usecase/         # Search use cases
```

> **Note**: Likely candidate để merge vào **search-service**.

---

## 17. shared

**Shared Libraries** (không phải service, là internal packages).

### shared/pkg — Utility Packages
```
classification/   # Vulnerability classification
clients/          # HTTP/gRPC client utilities
config/           # Config loading helpers
cpe/              # CPE parsing & matching
cveid/            # CVE ID validation & parsing
cwe/              # CWE lookup
database/         # DB connection helpers
domain/           # Shared domain types
ecosystem/        # Package ecosystem types (npm, pypi, etc.)
errors/           # Shared error types
grpcutil/         # gRPC utilities
health/           # Health check interfaces
logger/           # Zerolog wrapper
middleware/       # HTTP/gRPC middleware
models/           # Shared data models
nats/             # NATS client wrapper
observability/    # OpenTelemetry setup
osvschema/        # OSV JSON schema types
osvutil/          # OSV utility functions
pagination/       # Cursor-based pagination
pgp/              # PGP signature utilities
purl/             # Package URL (purl) parsing
resilience/       # Retry, circuit breaker
search/           # Shared search types
semver/           # Semantic versioning
severity/         # CVSS severity helpers
testutil/         # Test utilities
version/          # Version comparison
```

### shared/proto — gRPC Definitions
```
asset/            # Asset service proto
auth/             # Auth service proto
cve/              # CVE service proto
cvedb/            # CVE database proto
datasync/         # Data sync proto
finding/          # Finding service proto
identity/         # Identity proto
product/          # Product service proto
reporter/         # Report service proto
sbomvex/          # SBOM/VEX proto
scan/             # Scan service proto
scanner/          # Scanner agent proto
gen/              # Generated Go code
```
