# OSV Platform — Architecture Document

> **Version:** 4.0  
> **Date:** 2026-06-23  
> **Status:** Active  
> **Project:** OSV Platform — Go Microservices  

---

## 1. Tổng Quan Hệ Thống

### 1.1 Giới Thiệu

**OSV Platform** là nền tảng quản lý lỗ hổng bảo mật toàn diện, được xây dựng theo kiến trúc **Go Microservices** với Clean Architecture và Event-Driven design.

Hệ thống cung cấp:
- **CVE Intelligence**: Thu thập, làm phong phú và tra cứu dữ liệu lỗ hổng từ 15+ nguồn
- **Finding Management**: Quản lý vòng đời phát hiện lỗ hổng với ProductType → Product → Engagement → Test → Finding hierarchy
- **Active Scanning**: Network scanning (Nmap), web scanning (ZAP), agent-based ingestion, SBOM parsing
- **AI Enrichment**: CVE embedding, severity prediction, automated triage với LLM provider failover chain
- **Security Workflows**: SLA tracking, audit trail, JIRA integration, 5-channel notifications, report generation
- **Asset Management**: Asset registry, tagging, risk scoring, scheduled scans

### 1.2 Mục Tiêu Kiến Trúc

| Mục tiêu | Mô tả |
|-----------|-------|
| **Scalability** | Mỗi service scale độc lập theo workload |
| **Data Freshness** | NVD sync < 2h, EPSS daily, CAPEC/CWE weekly |
| **Observability** | zerolog + Prometheus metrics trên tất cả services |
| **Reliability** | NATS at-least-once, graceful shutdown |
| **Maintainability** | Clean Architecture 4 layers (domain/usecase/adapter/infra) |

### 1.3 Phiên Bản Thực Hiện

| Version | CR Sources | Services | Trạng thái |
|---------|-----------|----------|-----------| 
| v2.0 | cve-search CRs (9) | data-service, search-service, ranking-service, identity-service, gateway | ✅ Done |
| v2.1 | DefectDojo CRs (11) | finding-service, scan-service, sla-service, notification-service, jira-service, audit-service | ✅ Done |
| v2.2 | GlobalCVE CRs (10) | Enhanced data-service (15+ fetchers), OpenSearch, pgvector, observability | ✅ Done |
| **v3.0** | **OpenVulnScan CRs (7)** | **ai-service, scan-service (active), product-service, asset-service, report-service, gateway-service (standalone)** | 🔵 In Progress |

---

## 2. Kiến Trúc Tổng Thể

### 2.1 High-Level Architecture

```
┌──────────────────────────────────────────────────────────────────────────────────────┐
│                              EXTERNAL DATA SOURCES                                    │
│  NVD CVE API  │  JVN RSS  │  CIRCL  │  ExploitDB  │  CVE.org  │  CISA KEV           │
│  CNNVD        │  FIRST EPSS │ MITRE CAPEC/CWE │ NVD CPE Dict │ Vendor Advisories    │
└─────────────────────────────────────┬────────────────────────────────────────────────┘
                                      │ HTTP/RSS/CSV pull (scheduled)
                                      ▼
┌─────────────────────────────────────────────────────────────────────────────────────┐
│                              DATA-SERVICE  :8082                                      │
│  Fetcher Registry (15+ fetchers)  │  EPSS daily sync  │  KEV diff detection         │
│  CAPEC/CWE weekly sync            │  CPE Dictionary   │  CVE enrichment pipeline     │
└──────────────────────────────────────────┬───────────────────────────────────────────┘
                                           │ PostgreSQL 16 writes
                                           │ NATS publish: ingestion.cve.synced
                         ┌─────────────────┴─────────────────┐
                         ▼                                     ▼
           ┌─────────────────────┐                ┌──────────────────────┐
           │  PostgreSQL 16      │                │   OpenSearch 2       │
           │  + pgvector         │                │   "cves" index       │
           │                     │◄── indexed ───►│   BM25 + aggregations│
           │  cves, kev_entries   │                └──────────────────────┘
           │  capec_patterns      │
           │  cwe_weaknesses      │
           │  cpe_dictionary      │
           └─────────────────────┘
                         │
                         ▼
┌──────────────────────────────────────────────────────────────────────────────────────┐
│                           SEARCH-SERVICE  :8083                                        │
│  OpenSearch FTS  │  pgvector semantic (1536-dim)  │  Browse vendor/product           │
│  EPSS filter/sort │  CVE export (JSON/CSV)         │  CPE search lax/strict          │
└──────────────────────────────────────────────────────────────────────────────────────┘

                            ┌─────────── NATS JetStream ───────────────────┐
                            │  finding.created, finding.status.changed       │
                            │  finding.sla.breached, kev.new, jira.*        │
                            │  scan.scan.created, scan.scan.completed        │
                            │  ai.cve.enriched, ai.triage.completed         │
                            └───────────────────────────────────────────────┘
                                  ▲           ▲           ▲           ▲
                                  │           │           │           │
               ┌──────────────────┐  ┌───────┐  ┌───────┐  ┌────────┐
               │  finding-service │  │  sla  │  │ notif │  │ audit  │
               │  :8085           │  │  :8086│  │ :8087 │  │ :8090  │
               │ ProductType      │  │ SLA   │  │ Email │  │ Immut. │
               │ Product          │  │ config│  │ Slack │  │ event  │
               │ Engagement       │  │ breach│  │ Teams │  │ log    │
               │ Test             │  │ cron  │  │ SSE   │  │ HMAC   │
               │ Finding          │  └───────┘  │ Webhook│ │ signed │
               │ RiskAcceptance   │             └───────┘  └────────┘
               │ Reports          │                 │
               └──────────────────┘       ┌─────────┴─────────┐
                         │                │   jira-service      │
                         │                │   :8088             │
                         │                │   AES creds         │
                         │                │   bidirectional sync│
                         ▼                └─────────────────────┘
               ┌──────────────────┐
               │  scan-service    │
               │  :8084           │
               │  Nmap/ZAP/Agent  │
               │  SCA parsers     │
               │  SBOM ingest     │
               │  Scheduler       │
               └──────────────────┘

┌─────────────────────────────────────────────────────────────────────────────────────┐
│  ai-service :9103          product-service :8089    asset-service :8091             │
│  Ollama→OpenAI chain       ProductType/Product      Asset registry                  │
│  CVE embedding             Engagement/Test          IP/hostname/OS tags             │
│  Severity prediction       CI/CD orchestrator       Risk scoring                    │
│  Finding triage            gRPC: GetProduct         Scheduled scan links            │
└─────────────────────────────────────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────────────────────────────────────┐
│          report-service (embedded in finding-service:8085)                           │
│  HTML / PDF (wkhtmltopdf) / CSV / Excel (excelize)                                  │
│  MinIO artifact storage                                                              │
└─────────────────────────────────────────────────────────────────────────────────────┘

┌──────────────────────────────────────────────────────────────────────────────────────┐
│                         UNIFIED GATEWAY  apps/osv  :8080                              │
│  Dual Auth (JWT + API Key)  │  Redis rate-limit  │  Reverse Proxy 150+ routes        │
│  UserHeader injection       │  OpenAPI /schema   │  BFF (Dashboard, Health, SLA)    │
│  SSE token auth (?token=)   │  Admin BFF         │  Transform middleware             │
└─────────────────────────────────────┬────────────────────────────────────────────────┘
                                      │ HTTPS
                                      ▼
                              CLIENT APPLICATIONS
                    Web UI  │  CLI  │  CI/CD  │  Third-party
```

### 2.2 Service Port Map

| Service | HTTP Port | gRPC Port | Vai trò |
|---------|:----------:|:---------:|---------| 
| `apps/osv` (gateway) | **8080** | — | API gateway, auth, rate-limit, 150+ routes, BFF |
| `identity-service` | **8081** | — | LDAP auth, TOTP MFA, API key management |
| `data-service` | **8082** | — | CVE fetching, enrichment, KEV/EPSS/CWE/CAPEC feeds |
| `search-service` | **8083** | — | OpenSearch FTS, pgvector semantic, browse, export |
| `scan-service` | **8084** | — | Nmap/ZAP active scanning, SCA parsers, SBOM, scheduler |
| `finding-service` | **8085** | — | Product hierarchy, findings, reports (formatters embedded) |
| `sla-service` | **8086** | — | SLA config, breach detection cron |
| `notification-service` | **8087** | — | Email/Slack/Teams/SSE/Webhook, NATS subscriber |
| `jira-service` | **8088** | — | Bidirectional JIRA sync, AES-256-GCM creds |
| `product-service` | **8089** | — | ProductType/Product/Engagement/Test, CI/CD orchestrator |
| `audit-service` | **8090** | — | Immutable event log, HMAC-signed partitions |
| `asset-service` | **8091** | — | Asset registry, tagging, risk scoring |
| `ai-service` | **9103** | — | CVE embeddings, severity prediction, finding triage |
| `ranking-service` | (internal) | — | CPE popularity ranking |
| `gateway-service` | — | **50051–50057** | Standalone gRPC gateway (v3.0 — parallel implementation) |

> **Note:** `gateway-service` (trong `services/gateway-service/`) là implementation gRPC gateway song song với `apps/osv`. `apps/osv` hiện là gateway chính đang hoạt động.

---

## 3. Các Thành Phần Kiến Trúc Chi Tiết

### 3.1 API Gateway (`apps/osv`)

**Vai trò:** HTTP reverse proxy, authentication, rate-limiting, routing, BFF  
**Implementation:** Go `net/http` stdlib ServeMux (Go 1.22+), no external router

#### Authentication Middleware

```
Request
  │
  ▼
Auth Middleware (auth.AuthMiddleware / auth.AuthenticateSSE)
  ├── Extract: "Authorization: Bearer {token}" hoặc "X-Api-Key: {key}"
  ├── JWT path: Parse HS256 claims, check exp
  └── API Key path: Lookup identity-service → validate SHA-256 hash
  │
  ▼
Transform Middleware (transform.InjectUserHeaders)
  └── Inject: X-User-ID, X-User-Role, X-User-Perms → upstream service
  │
  ▼
Rate Limiter (ratelimit.RateLimiter)
  └── Redis token bucket per IP (configurable per-route)
  │
  ▼
Reverse Proxy (ReverseProxy)
  └── Forward request to upstream service
```

#### Auth Chains

| Chain | Middleware | Dùng cho |
|-------|-----------|----------|
| `protected` | `Authenticate + InjectUserHeaders` | Hầu hết API endpoints |
| `adminOnly` | `Authenticate + InjectUserHeaders + RequireRole("Admin")` | Admin endpoints |
| `sseAuth` | `AuthenticateSSE + InjectUserHeaders` | SSE streams (accept `?token=<jwt>`) |
| Public | (none) | `/health`, `/readyz`, `/api/v2/schema`, legacy v1 |

#### Route Groups (Sprint-based)

| Sprint | Route Group | Upstream | Auth |
|--------|-------------|---------|------|
| 1 | `/api/v1/auth/*` (login, refresh) | identity-service:8081 | Public |
| 1 | `/api/v1/auth/*` (me, logout, TOTP, MFA) | identity-service:8081 | JWT/APIKey |
| 1 | `/api/v1/profile`, `/api/v1/api-keys` | identity-service:8081 | JWT/APIKey |
| 1 | `/api/v1/admin/users/*`, `/api/v1/admin/roles` | identity-service:8081 | Admin |
| 1 | `/api/v1/dashboard`, `/api/v1/dashboard/sla` | **In-gateway BFF** | JWT/APIKey |
| 2 | `/api/v1/findings/*` (CRUD, bulk, notes, audit) | finding-service:8085 | JWT/APIKey |
| 2 | `/api/v1/scans/*` (CRUD, cancel, stream, results) | scan-service:8084 | JWT/APIKey |
| 2 | `/api/v1/scans/{id}/stream` | scan-service:8084 | SSE Auth |
| 2 | `/api/v1/products/*`, `/api/v1/engagements/*` | finding-service:8085 | JWT/APIKey |
| 2 | `/api/v1/reports/*` (CRUD, download) | finding-service:8085 | JWT/APIKey |
| 2 | `/api/v1/notifications/stream` | notification-service:8087 | SSE Auth |
| 2 | `/api/v1/notifications/*`, `/api/v1/webhooks/*` | notification-service:8087 | JWT/APIKey |
| 3 | `/api/v1/admin/health`, `/api/v1/admin/settings` | **In-gateway BFF** | Admin |
| 3 | `/api/v1/jira/config` | jira-service:8088 | Admin |
| 3 | `/api/v1/audit-log` | audit-service:8090 | Admin |
| 4 | `/api/v2/epss/*`, `/api/v2/cwe/*`, `/api/v2/capec/*` | data-service:8082 | JWT/APIKey |
| 4 | `/api/v2/cves/search`, `/api/v2/cves/search/semantic` | search-service:8083 | JWT/APIKey |
| 4 | `/api/v2/cves/export`, `/api/v2/browse/*` | search-service:8083 | JWT/APIKey |
| 4 | `/api/v1/assets/*` (CRUD, tags, risk, history) | asset-service:8091 | JWT/APIKey |
| 4 | `/api/v1/assets/{id}/findings` | **BFF** → finding-service:8085 | JWT/APIKey |
| 4 | `/api/v1/sla/overview` | **BFF** → sla-service:8086 | JWT/APIKey |
| 4 | `/api/v1/sla/config` | sla-service:8086 | JWT/APIKey |
| 4 | `/api/v1/ai/triage`, `/api/v1/ai/enrich` (legacy) | ai-service:9103 | JWT/APIKey |
| 4 | `/api/v1/ai/triage/{findingId}`, `/api/v1/ai/triage/queue` | ai-service:9103 | JWT/APIKey |
| 4 | `/api/v1/ai/enrichment/*` | ai-service:9103 | JWT/APIKey |
| Legacy | `/api/v2/product-types`, `/api/v2/products`, `/api/v2/engagements`, `/api/v2/tests` | finding-service:8085 | JWT/APIKey |
| Legacy | `/api/v2/findings/*`, `/api/v2/finding-groups`, `/api/v2/risk-acceptances` | finding-service:8085 | JWT/APIKey |
| Legacy | `/api/v2/tool-configurations/*`, `/api/v2/metrics/*`, `/api/v2/product-grades/*` | finding-service:8085 | JWT/APIKey |
| Legacy | `/api/v2/reports/*` | finding-service:8085 | JWT/APIKey |
| Legacy | `/api/v2/import-scan`, `/api/v2/reimport-scan`, `/api/v2/parsers` | scan-service:8084 | JWT/APIKey |
| Legacy | `/api/v2/sla-configurations/*`, `/api/v2/sla-dashboard`, `/api/v2/sla-violations` | sla-service:8086 | JWT/APIKey |
| Legacy | `/api/v2/notification-rules/*`, `/api/v2/alerts/*` | notification-service:8087 | JWT/APIKey |
| Legacy | `/api/v2/jira-configurations/*`, `/api/v2/jira-issues/*` | jira-service:8088 | JWT/APIKey |
| Legacy | `/api/v2/audit-log/*` | audit-service:8090 | JWT/APIKey |
| Public | `/api/v1/kev/*`, `/api/v1/epss/*`, `/api/v1/cve/*`, `/api/v1/dbinfo` | data-service:8082 | None |
| Public | `/v1/`, `/api/v1/` (legacy OSV) | data-service:8082 | None |

#### Rate Limiting

```
Redis key: ratelimit:{ip}
Algorithm: Token bucket (per-route configurable)
Per-route limits:
  POST /api/v2/reports             → 5/minute
  POST /api/v2/import-scan         → 30/minute
  POST /api/v2/reimport-scan       → 30/minute
  POST /api/v1/reports             → 5/minute
  POST /api/v1/scans/import        → 10/minute
  GET /api/v2/audit-log/export     → 2/minute
  POST /api/v2/product-types       → 10/minute
  POST /api/v2/products            → 10/minute
  POST /api/v2/findings/bulk       → 10/minute
  POST /api/v2/jira-issues         → 20/minute
```

#### In-Gateway BFF Handlers

| BFF | Endpoint | Logic |
|-----|---------|-------|
| `DashboardBFF` | `GET /api/v1/dashboard` | Aggregate from finding-service + data-service, Redis cache |
| `DashboardBFF` | `GET /api/v1/dashboard/sla` | SLA summary aggregation |
| `HealthBFF` | `GET /api/v1/admin/health` | NATS ping + Postgres ping + Redis ping |
| `SettingsBFF` | `GET/PATCH /api/v1/admin/settings` | Settings from Postgres (settings repo) |
| `assetFindingsBFF` | `GET /api/v1/assets/{id}/findings` | Rewrite → `/api/v2/findings?asset_id={id}` |
| `slaOverviewBFF` | `GET /api/v1/sla/overview` | Rewrite → `/api/v2/sla-dashboard` |

---

### 3.2 Data-Service (`services/data-service`)

**Vai trò:** CVE data platform — ingestion, enrichment, serving  
**Pattern:** Clean Architecture (domain/usecase/adapter/infra)

#### Fetcher Registry Pattern

```go
type CVEFetcher interface {
    FetchSince(ctx context.Context, since time.Time) (<-chan CVERecord, error)
    Source() string
}

// Registry: tất cả fetchers tự đăng ký
var registry = map[string]CVEFetcher{
    "nvd":      &NVDFetcher{},      // Every 2h
    "jvn":      &JVNFetcher{},      // Every 1h (RSS)
    "circl":    &CIRCLFetcher{},    // Every 6h
    "exploitdb": &ExploitDBFetcher{}, // Every 24h (CSV stream)
    "cve_org":  &CVEOrgFetcher{},   // Every 12h (GitHub deltaLog)
    "cisa_kev": &CISAKEVFetcher{},  // Every 6h
    "epss":     &EPSSFetcher{},     // Daily 3am UTC (CSV.GZ)
    "capec":    &CAPECFetcher{},    // Weekly Sunday 5am
    "cwe":      &CWEFetcher{},      // Weekly Sunday 5am
    "nvd_cpe":  &NVDCPEFetcher{},   // Weekly Sunday 4am
    "cnnvd":    &CNNVDFetcher{},    // Every 12h
    // + vendor advisories...
}
```

#### CVE Processing Pipeline

```
Fetcher pulls data
      │
      ▼
Normalize to canonical CVERecord{
  cve_id, description, published_at, modified_at,
  severity_v3, severity_v2, epss_score, is_kev,
  is_exploit, cwe_ids[], vendor, product, cpe_list[],
  data_source, source_url
}
      │
      ▼
Upsert to PostgreSQL (ON CONFLICT DO UPDATE)
      │
      ├── Update OpenSearch index (sync)
      │
      └── Publish NATS: ingestion.cve.synced{cve_id, action}
             │
             ├── ai-service → generate embedding → update cves.embedding
             └── search-service → refresh aggregation cache
```

#### Key Tables (osv_cves schema)

```sql
CREATE TABLE cves (
    cve_id          VARCHAR(20) PRIMARY KEY,
    description     TEXT,
    severity_v3     VARCHAR(10), -- CRITICAL/HIGH/MEDIUM/LOW
    cvss_v3_score   NUMERIC(4,1),
    cvss_v3_vector  VARCHAR(100),
    epss_score      NUMERIC(6,5),
    epss_percentile NUMERIC(6,5),
    is_kev          BOOLEAN DEFAULT FALSE,
    is_exploit      BOOLEAN DEFAULT FALSE,
    known_ransomware BOOLEAN DEFAULT FALSE,
    data_source     VARCHAR(50),
    published_at    TIMESTAMPTZ,
    modified_at     TIMESTAMPTZ,
    embedding       vector(1536)  -- pgvector
);

CREATE TABLE kev_entries (
    cve_id VARCHAR(20) PRIMARY KEY REFERENCES cves(cve_id),
    known_ransomware_campaign_use BOOLEAN,
    required_action TEXT,
    short_description TEXT,
    date_added DATE
);
-- CWE, CAPEC, CPE tables...
```

---

### 3.3 Search-Service (`services/search-service`)

**Vai trò:** Full-text search, semantic search, browse, export

#### Dual Backend Architecture

```
Search Request
      │
      ▼
Search Strategy Selector
  ├── OpenSearch available? → BM25 full-text (primary)
  └── Fallback → PostgreSQL GIN index
      │
      ▼
Aggregate results + EPSS enrichment
      │
      ▼
Response with facets, aggregations, pagination
```

#### Semantic Search Flow (pgvector)

```
POST /api/v2/cves/search/semantic
  {"query": "buffer overflow in web server"}
      │
      ▼
Embedding generation (ai-service or OpenAI)
  → []float32 (1536 dims)
      │
      ▼
pgvector cosine similarity:
  SELECT cve_id, description, 1 - (embedding <=> $1) AS score
  FROM cves
  WHERE 1 - (embedding <=> $1) > 0.7
  ORDER BY score DESC LIMIT 20
      │
      ▼
Enrich with EPSS, KEV flags, CWE names
```

---

### 3.4 Identity-Service (`services/identity-service`)

**Vai trò:** Authentication, TOTP MFA, API key management  
**Structure:** `adapter/`, `internal/` (adapter, cache, crypto, delivery, domain, infra, infrastructure, metrics, provider, usecase)

#### Auth Chain

```
Login request (username + password)
    │
    ▼
Auth Chain (configurable order):
    ├── Local: lookup users table → bcrypt compare
    └── LDAP: bind to LDAP server → group mapping → OSV role
    │
    ▼
Issue JWT (HS256, configurable TTL)
    │
    ▼
Store session token hash
```

#### TOTP MFA

```
POST /api/v1/auth/totp/setup  → generate TOTP secret (AES-256-GCM encrypted)
POST /api/v1/auth/totp/verify → TOTP RFC 6238 (±1 period)
DELETE /api/v1/auth/totp      → disable MFA

MFA aliases (BFF path rewrite in gateway):
  GET  /api/v1/auth/mfa/setup   → /api/v1/auth/totp/setup
  POST /api/v1/auth/mfa/confirm → /api/v1/auth/totp/verify
```

#### API Key Management

```go
type APIKey struct {
    ID          uuid.UUID
    UserID      uuid.UUID
    Prefix      string    // First 12 chars for lookup
    HashSHA256  string    // SHA-256 of full key
    Scopes      []string  // ["cve:read", "finding:write", ...]
    ExpiresAt   *time.Time
    Revoked     bool
}

// Gateway validates:
// 1. Extract key from X-Api-Key header
// 2. Take first 12 chars → lookup by prefix
// 3. SHA-256(key) == stored hash (constant-time compare)
// 4. Check revoked + expiry + scope
```

---

### 3.5 Finding-Service (`services/finding-service`)

**Vai trò:** Product/Engagement/Test hierarchy + Finding lifecycle + Reports  
**Structure:** `internal/` (delivery, domain, formatters, infra, metrics, scheduler, usecase)

> **Note:** Report generation (HTML/PDF/CSV/Excel) và SLA scheduling đều được embed vào finding-service (không tách ra report-service riêng trong production hiện tại). `services/report-service` là implementation độc lập đang phát triển.

#### Domain Model

```
ProductType (Web App, API, Infrastructure, Mobile)
    └── Product (business_criticality, lifecycle, members[])
            └── Engagement (type: Interactive | CI/CD, status, dates)
                    └── Test (tool_config, scan_type)
                            └── Finding (CVE, severity, state, hash, SLA)
                                    ├── FindingGroup (related findings)
                                    ├── FindingNote (comments)
                                    └── RiskAcceptance (expiry)
```

#### Finding State Machine

```
                    ┌──────────────────────────────────────┐
                    ▼                                      │
              [Active] ──► [Mitigated]                    │ reopen
                 │ ──► [FalsePositive]  ─────────────────►┘
                 │ ──► [RiskAccepted]   ─────────────────►┘
                 │ ──► [OutOfScope]     ─────────────────►┘
                 │ ──► [Duplicate]      (no reopen)
                 
Priority: Duplicate > FalsePositive > OutOfScope > RiskAccepted > Mitigated > Active
```

#### Deduplication Algorithm

```go
func computeHash(f Finding) string {
    data := f.Title + f.ComponentName + f.ComponentVersion + f.CveID
    return hex.EncodeToString(sha256.Sum256([]byte(data))[:])
}

// On finding create:
// 1. Compute hash
// 2. SELECT existing WHERE hash = ? AND product_id = ? AND NOT duplicate
// 3. If found: mark new finding as duplicate, link to original
```

#### Product Grading

```go
func ComputeGrade(criticalCount, highCount, totalCount int) string {
    switch {
    case criticalCount >= 3 || totalCount > 20: return "F"
    case criticalCount >= 1 && criticalCount <= 2: return "D"
    case criticalCount == 0 && highCount > 5: return "C"
    case criticalCount == 0 && highCount <= 5: return "B"
    default: return "A"
    }
}
```

#### Report Formatters (embedded in finding-service)

```
internal/formatters/
  ├── html.go       → Go template → HTML report
  ├── pdf.go        → wkhtmltopdf (HTML → PDF)
  ├── csv_json_excel.go → CSV + JSON + Excel (excelize)
  └── interface.go  → ReportFormatter interface
```

---

### 3.6 Scan-Service (`services/scan-service`)

**Vai trò:** Active scanning (Nmap/ZAP), SCA parsers, SBOM ingestion, scheduled scans  
**Structure:** `internal/` (adapters, delivery/{grpc,http,sse}, domain, infra, infrastructure, metrics, parsers, sbom, scanner/{nmap,zap}, scheduler, usecase)

#### Active Scanners

| Scanner | Implementation | Output |
|---------|---------------|--------|
| Nmap | `scanner/nmap/scanner.go` | XML → parsed hosts/ports/CVEs |
| ZAP | `scanner/zap/` | ZAP REST API → web alerts |
| Agent | POST `/api/v1/agents/report` | Agent-submitted findings |

#### SCA Parsers (Language-specific)

```
internal/parsers/
  ├── golang.go   → go.sum / go.mod parsing
  ├── java.go     → Maven/Gradle dependency parsing
  ├── nodejs.go   → package-lock.json / yarn.lock
  ├── python.go   → requirements.txt / pipfile
  ├── rust.go     → Cargo.lock parsing
  └── checkers/   → vulnerability lookup per language
```

#### SBOM Support

```
internal/sbom/ → SBOM (Software Bill of Materials) ingestion
```

#### Scheduled Scans

```
internal/scheduler/
  ├── scheduler.go      → cron-based scan scheduler (1-min ticker)
  └── cron_worker.go    → NATS-triggered scan execution worker
```

#### SSE (Server-Sent Events) — Scan Progress

```
GET /api/v1/scans/{id}/stream → SSE stream (token auth via ?token=)
  → real-time scan progress events
  → delivery/sse/ implementation
```

#### Scan State Machine

```
pending → queued → running → completed
                           └→ failed
                 └→ cancelled
```

---

### 3.7 SLA-Service (`services/sla-service`)

**Vai trò:** SLA configuration, daily breach detection, per-product overrides

#### SLA Config Structure

```go
type SLAConfig struct {
    ID          uuid.UUID
    Name        string
    CriticalDays int  // Default: 7
    HighDays     int  // Default: 30
    MediumDays   int  // Default: 90
    LowDays      int  // Default: 180
    ProductID    *uuid.UUID  // nil = global default
}
```

#### Daily Breach Detection (Cron)

```
Daily cron @ 00:00 UTC:
  1. SELECT findings WHERE active = true AND sla_expiration_date <= NOW()
  2. For each breached finding:
     - Update finding: sla_breached = true
     - Publish NATS: finding.sla.breached{finding_id, severity, expires_at}
  3. notification-service subscribes → Email + Slack
```

---

### 3.8 Notification-Service (`services/notification-service`)

**Vai trò:** 5-channel notification dispatch, SSE in-app stream, webhook management

#### Channel Implementations

```
Email    → SMTP (net/smtp)
Slack    → Webhook HTTP POST JSON
Teams    → Webhook HTTP POST Adaptive Card
In-app   → SSE stream (GET /api/v1/notifications/stream + ?token= auth)
           + Store in alerts table (polled by frontend)
Webhook  → HTTP POST + HMAC-SHA256 signing
```

#### Webhook Delivery

```go
// HMAC signature
sig := hmac.New(sha256.New, []byte(secret))
sig.Write(payload)
header := "sha256=" + hex.EncodeToString(sig.Sum(nil))
req.Header.Set("X-OSV-Signature", header)

// SSRF protection
func isPrivateIP(ip net.IP) bool {
    // Block: 10.x, 172.16-31.x, 192.168.x, 127.x, ::1, fc00::/7
}

// Retry: 3 attempts, exponential backoff: 1s, 2s, 4s
```

#### NATS Events Subscribed

```
finding.created          → Email, Webhook, In-app
finding.status.changed   → In-app, Webhook
finding.sla.breached     → Email, Slack
finding.batch_created    → In-app summary
kev.new                  → Email, Webhook, Slack
risk_acceptance.expired  → Email, In-app
jira.issue.created       → In-app
scan.scan.completed      → In-app
scan.scan.failed         → Email, In-app
...and more
```

---

### 3.9 JIRA-Service (`services/jira-service`)

**Vai trò:** Bidirectional JIRA synchronization

#### AES-256-GCM Credential Encryption

```go
type JiraConfig struct {
    ServerURL      string
    EncryptedToken []byte  // AES-256-GCM encrypted
    ProjectKey     string
    ProductID      uuid.UUID
}
// Key from env: JIRA_ENCRYPTION_KEY (32 bytes)
```

#### Bidirectional Sync

```
Finding create/update → jira-service
  → Create/Update JIRA issue via REST API
  → Store jira_issue{finding_id, jira_key, status}

JIRA webhook POST /webhooks/jira/
  → Verify HMAC signature
  → If issue.resolved → publish finding status change
  → finding-service subscribes → close finding
```

---

### 3.10 Audit-Service (`services/audit-service`)

**Vai trò:** Immutable compliance event log

#### Append-Only Design

```sql
CREATE TABLE audit_events (
    id          UUID DEFAULT gen_random_uuid(),
    actor_id    UUID NOT NULL,
    actor_email VARCHAR(255),
    action      VARCHAR(100) NOT NULL,  -- "finding.close", "product.create", ...
    resource_type VARCHAR(50),
    resource_id UUID,
    before_json JSONB,
    after_json  JSONB,
    hmac_sig    VARCHAR(64),  -- HMAC-SHA256 of entire row
    created_at  TIMESTAMPTZ DEFAULT NOW()
) PARTITION BY RANGE (created_at);  -- Monthly partitions

ALTER TABLE audit_events ENABLE ROW LEVEL SECURITY;
CREATE POLICY audit_readonly ON audit_events FOR SELECT USING (true);
REVOKE UPDATE, DELETE ON audit_events FROM PUBLIC;
```

#### 40+ NATS Subscriptions

```
finding.status.changed → record state transition
finding.sla.breached   → record SLA breach
finding.batch_created  → record batch import
jira.issue.created     → record JIRA sync
risk_acceptance.*      → record acceptance lifecycle
scan.scan.completed    → record scan completion
... (all cross-service events)
```

---

### 3.11 AI-Service (`services/ai-service`)

**Vai trò:** CVE embedding generation, severity prediction, finding triage  
**Port:** 9103 (HTTP), gRPC handler trong `internal/delivery/grpc/`  
**Structure:** `internal/` (delivery/{grpc,http}, domain, infra, metrics, provider, usecase)

#### LLM Provider Chain

```
internal/provider/
  ├── chain.go    → Failover chain: try each provider in order
  ├── ollama/     → Ollama local LLM (primary, low-latency)
  └── openai/     → OpenAI API (fallback)
```

#### HTTP Endpoints

| Method | Path | Description |
|--------|------|-------------|
| `POST` | `/api/v1/ai/triage` | Legacy: triage finding (body: `{finding_id, cve_id}`) |
| `POST` | `/api/v1/ai/enrich` | Legacy: enrich CVE (body: `{cve_id}`) |
| `GET` | `/api/v1/ai/triage/queue` | Get triage queue status |
| `POST` | `/api/v1/ai/triage/{findingId}` | Async triage by finding ID (202 Accepted) |
| `POST` | `/api/v1/ai/triage/{findingId}/review` | Submit triage review feedback |
| `GET` | `/api/v1/ai/enrichment` | Enrichment pipeline status |
| `POST` | `/api/v1/ai/enrichment/trigger` | Trigger manual enrichment for CVE IDs |
| `GET` | `/api/v1/ai/enrichment/{cveId}` | Enrichment detail for specific CVE |

#### Use Cases

```
internal/usecase/
  ├── batch_enrich/     → Batch CVE enrichment (NATS-triggered)
  ├── enrich_cve/       → Single CVE enrichment (embedding + severity)
  ├── generate_embedding/ → Vector embedding generation (1536-dim)
  └── triage_finding/   → LLM-powered finding triage (async, 202 pattern)
```

---

### 3.12 Product-Service (`services/product-service`)

**Vai trò:** ProductType/Product/Engagement/Test hierarchy management, CI/CD orchestrator  
**Structure:** `internal/` (delivery/http, domain/entity, usecase)

#### CI/CD Orchestrator Flow

```
POST /api/v1/orchestrate/cicd
  {product_name, build_id, commit_hash, branch, scan_results[]}
      │
      ▼
Find/Create ProductType + Product (by name)
      │
      ▼
Create Engagement (type=CI/CD, build_id, commit_hash)
      │
      ▼
Create Test (linked to engagement)
      │
      ▼
CreateFinding per result (SHA-256 dedup)
      │
      ▼
Close engagement (status=Completed)
      │
      ▼
Response: {new: N, duplicates: M, product_id, engagement_id, exit_code: 0|1}
```

---

### 3.13 Asset-Service (`services/asset-service`)

**Vai trò:** Asset registry (IP, hostname, OS, services), tagging, risk scoring, scheduled scan linking  
**Port:** 8091  
**Structure:** `internal/` (delivery/http, usecase)

#### Asset Model

```
Asset {
  id, name, type (host/ip/web/container/cloud)
  address (INET type in Postgres)
  os, os_version, services[]
  tags[] (GIN index for fast tag queries)
  risk_score, last_seen_at
  linked_scan_ids[]
}
```

#### Key Routes

```
GET  /api/v1/assets/tags        → All tags (literal before wildcard)
GET  /api/v1/assets             → List assets (filter by tag, type)
POST /api/v1/assets             → Create/upsert asset
GET  /api/v1/assets/{id}        → Asset detail
PUT  /api/v1/assets/{id}        → Update asset
DELETE /api/v1/assets/{id}      → Delete asset
PUT  /api/v1/assets/{id}/tags   → Update tags
GET  /api/v1/assets/{id}/risk   → Risk summary
GET  /api/v1/assets/{id}/history → Scan history
GET  /api/v1/assets/{id}/findings → BFF → /api/v2/findings?asset_id={id}
```

---

### 3.14 Report-Service (`services/report-service`)

**Vai trò:** Standalone report generation service (v3.0)  
**Structure:** `internal/` (delivery, domain, formatters, storage, usecase), `templates/`

> **Note:** Trong finding-service hiện tại, report formatters đã được embed trực tiếp (`internal/formatters/`). `services/report-service` là implementation tách biệt đang phát triển cho v3.0.

#### Formatters

```
internal/formatters/
  ├── html.go          → Go template → HTML report
  ├── pdf.go           → wkhtmltopdf (HTML → PDF)
  ├── csv_json_excel.go → CSV + JSON + Excel (excelize)
  └── interface.go     → ReportFormatter interface
```

#### Storage

```
internal/storage/ → MinIO artifact upload
Bucket: osv-reports
Path: reports/{report_run_id}/{format}
```

---

### 3.15 Gateway-Service (`services/gateway-service`)

**Vai trò:** Standalone gRPC gateway (v3.0 parallel implementation)  
**Structure:** `internal/` (adapter, auth, bff, cache, delivery/{grpc,http}, domain, health, infra, metrics, proxy, ratelimit, usecase)

#### gRPC Upstreams

```yaml
upstreams:
  identity-service:     identity-service:50051
  data-service:         data-service:50052
  search-service:       search-service:50053
  scan-service:         scan-service:50054
  finding-service:      finding-service:50055
  ai-service:           ai-service:50056
  notification-service: notification-service:50057
```

#### BFF Components

```
internal/bff/
  ├── dashboard.go      → DashboardBFF aggregate
  ├── clients/          → gRPC clients to upstream services
  ├── graphql/          → GraphQL layer (planned)
  └── handlers/         → handler_auth.go, handler_dashboard.go,
                          handler_scan.go, handler_report.go,
                          handler_sbom.go, handler_cve_search.go,
                          handler_db.go, handler_ui_api.go,
                          handler_v1.go, osv_handler.go
```

---

## 4. Infrastructure Layer

### 4.1 PostgreSQL 16 + pgvector

**Role:** Primary data store, schema-per-service

| Schema | Owner Service | Purpose |
|--------|--------------|---------| 
| `osv_identity` | identity-service | users, api_keys, sessions, ldap_configs |
| `osv_cves` | data-service + search-service | cves, kev_entries, capec_patterns, cwe_weaknesses, cpe_dictionary |
| `osv_ranking` | ranking-service | cpe_rankings, vendor_stats |
| `osv_finding` | finding-service | product_types, products, engagements, tests, findings, reports |
| `osv_scan` | scan-service | scans, scan_findings, web_alerts, discovery_hosts, agent_findings, scheduled_scans |
| `osv_sla` | sla-service | sla_configs, sla_breaches |
| `osv_notif` | notification-service | webhooks, notification_rules, alerts |
| `osv_jira` | jira-service | jira_configs, jira_issues |
| `osv_audit` | audit-service | audit_events (partitioned monthly) |
| `osv_ai` | ai-service | cve_embeddings, severity_cache |
| `osv_asset` | asset-service | assets (INET type, GIN tags index) |
| `osv_product` | product-service | product_types, products, engagements, tests |

**pgvector**: Enabled on `osv_cves`, `cves.embedding vector(1536)`, IVFFlat index for ANN search.

### 4.2 OpenSearch 2

**Role:** Full-text search với BM25 scoring và aggregations

```json
{
  "mappings": {
    "properties": {
      "cve_id": {"type": "keyword"},
      "description": {"type": "text", "analyzer": "english"},
      "severity": {"type": "keyword"},
      "cvss_v3_score": {"type": "float"},
      "epss_score": {"type": "float"},
      "is_kev": {"type": "boolean"},
      "vendors": {"type": "keyword"},
      "products": {"type": "keyword"},
      "cwe_ids": {"type": "keyword"},
      "published_at": {"type": "date"}
    }
  }
}
```

### 4.3 Redis 7

**Role:** Rate-limiting, caching, JWT token store, SSE state

| Key Pattern | TTL | Purpose |
|------------|-----|---------| 
| `ratelimit:{ip}:{route}` | 60s | Rate-limit token bucket per route |
| `osv:embed:{cve_id}` | 7 days | CVE embedding cache |
| `osv:epss:{cve_id}` | 24h | EPSS score cache |
| `osv:cpe_dict` | 24h | CPE dictionary cache |
| `osv:jwt:revoked:{jti}` | token TTL | JWT revocation list |
| `ai:enrichment:status` | 5m | AI enrichment pipeline status |
| `ai:triage:queue` | 1h | Triage queue state |
| `dashboard:summary:{user}` | 5m | Dashboard BFF cache |

### 4.4 NATS JetStream

**Role:** Async event-driven communication between services

```
Streams:
  INGESTION  → ingestion.cve.synced
  FINDINGS   → finding.created, finding.status.changed, finding.batch_created
  SLA        → finding.sla.breached
  KEV        → kev.new
  JIRA       → jira.issue.created, jira.issue.resolved
  SCANS      → scan.scan.created, scan.scan.completed, scan.scan.failed
  AI         → ai.cve.enriched, ai.triage.completed
  AUDIT      → (all events, fan-out)
  PRODUCT    → product.created

Consumer groups (durable):
  notification-service → FINDINGS, SLA, KEV, SCANS
  jira-service         → FINDINGS.status_changed
  audit-service        → ALL streams
  sla-service          → FINDINGS.created, FINDINGS.batch_created
  ai-service           → INGESTION (trigger enrichment)
  finding-service      → AI.triage.completed (update remarks)
  asset-service        → SCANS.completed (upsert assets)
```

### 4.5 MinIO (Object Storage)

**Role:** Report artifacts storage

```
Bucket: osv-reports
Path:   reports/{report_run_id}/{id}.{format}
        reports/{run_id}/report.pdf
        reports/{run_id}/report.html
        reports/{run_id}/report.csv
        reports/{run_id}/report.xlsx
```

---

## 5. Data Flow Chi Tiết

### 5.1 CVE Ingestion Flow

```
External Source (NVD, JVN, CIRCL, ...)
        │ HTTP/RSS/CSV (scheduled by fetcher)
        ▼
Fetcher Registry (data-service)
        │
        ├── Normalize to CVERecord
        ├── Compute is_exploit (ExploitDB cross-ref)
        ├── Mark is_kev (CISA KEV lookup)
        │
        ▼
PostgreSQL: UPSERT cves (ON CONFLICT DO UPDATE)
        │
        ├── Update OpenSearch index (sync reindex)
        │
        └── NATS Publish: ingestion.cve.synced{cve_id, action}
               │
               ├── ai-service → generate embedding → update cves.embedding
               └── search-service → refresh aggregation cache
```

### 5.2 Active Scan Flow (Nmap/ZAP)

```
POST /api/v1/scans (gateway)
        │ auth check
        ▼
scan-service: Create Scan entity (status=pending)
        │
        ▼
NATS Publish: scan.scan.created
        │
        ▼
scan-service worker (NATS consumer): status → queued → running
        │
        ├── Nmap subprocess: -sV -O → parse XML → CVE lookup
        ├── ZAP: spider + active scan via ZAP REST API
        └── Agent: POST /api/v1/agents/report (API key)
        │
        ▼
SSE stream → client progress events (GET /api/v1/scans/{id}/stream)
        │
        ▼
NATS Publish: scan.scan.completed {scan_id, finding_count}
        │
        ├── finding-service: Import findings (SHA-256 dedup, SLA computation)
        ├── asset-service: Upsert assets (IP, hostname, OS, services)
        ├── ai-service: EnrichCVE parallel (embedding + severity + triage)
        └── notification-service: "Scan completed" in-app alert
```

### 5.3 Scan Import Flow (File Upload)

```
POST /api/v2/import-scan (multipart: file + metadata)
        │
        ▼
apps/osv gateway → auth + rate-limit (30/min) → scan-service:8084
        │
        ▼
SCA Parser Factory → detect language → parse (golang/java/nodejs/python/rust)
        │
        ▼
Parse → Normalize → Enrich (CVE lookup) → Dedup check
        │
        ├── New finding: INSERT, compute SLA deadline
        └── Duplicate: INSERT with duplicate_finding_id, active=false
        │
        ▼
NATS Publish: finding.batch_created{scan_id, finding_ids[]}
        │
        ├── notification-service → In-app alert: "Scan completed"
        ├── sla-service → Set SLA deadlines
        └── audit-service → Record batch import event
```

### 5.4 Finding State Change Flow

```
POST /api/v2/findings/{id}/close
        │
        ▼
apps/osv gateway → auth → finding-service:8085
        │
        ▼
State machine validation (Active → Mitigated)
        │
        ▼
Update finding: status, closed_at, closed_by
        │
        ▼
NATS Publish: finding.status.changed{finding_id, from, to, user_id}
        │
        ├── notification-service → In-app notification
        ├── jira-service → Update JIRA issue status
        └── audit-service → Record state transition + HMAC
```

### 5.5 SLA Breach Detection Flow

```
Daily cron @ 00:00 UTC (sla-service)
        │
        ▼
SELECT findings WHERE active=true AND sla_expiration_date <= NOW()
  AND sla_breached = false
        │
        ▼
For each finding:
  UPDATE findings SET sla_breached = true
  NATS Publish: finding.sla.breached{finding_id, severity, expires_at}
        │
        ├── notification-service → Email + Slack notification
        └── audit-service → Record breach event
```

### 5.6 AI Triage Flow (Async)

```
POST /api/v1/ai/triage/{findingId}
        │
        ▼
ai-service: 202 Accepted (fire-and-forget goroutine)
        │ (async)
        ▼
LLM Provider Chain: Ollama → OpenAI (failover)
  → Generate triage remarks + confidence score
        │
        ▼
Store result (Redis ai:triage:{findingId})
        │
        ▼
NATS Publish: ai.triage.completed{finding_id, remarks, confidence}
        │
        ▼
finding-service: Update finding.ai_remarks
```

### 5.7 Report Generation Flow

```
POST /api/v1/reports {scan_id, formats:[pdf,html,csv,excel]}
        │
        ▼
finding-service: Create report_run (status=generating)
        │
        ▼
GetFindingsByScan (internal query)
        │
        ▼
Parallel goroutines: HTML + PDF (wkhtmltopdf) + CSV + Excel (excelize)
        │
        ▼
Upload artifacts to MinIO: reports/{run_id}/{run_id}.{format}
        │
        ▼
report_run: status=completed, exit_code=0 (no CVEs) | 1 (CVEs found)
        │
        ▼
Client: GET /api/v1/reports/{run_id}/download → presigned URL
```

---

## 6. Clean Architecture Pattern

Mỗi service tuân theo 4-layer Clean Architecture:

```
┌─────────────────────────────────────────┐
│           INFRASTRUCTURE LAYER          │
│  PostgreSQL, Redis, OpenSearch, NATS    │
│  MinIO, SMTP, Nmap, ZAP REST, OpenAI   │
└─────────────────────┬───────────────────┘
                      │ implements
┌─────────────────────▼───────────────────┐
│              ADAPTER LAYER              │
│  HTTP handlers, gRPC handlers           │
│  Repository implementations             │
│  NATS publishers/subscribers            │
│  SSE writers                           │
└─────────────────────┬───────────────────┘
                      │ calls
┌─────────────────────▼───────────────────┐
│             USE CASE LAYER              │
│  Business logic orchestration           │
│  Input validation, error handling       │
│  Transaction coordination               │
└─────────────────────┬───────────────────┘
                      │ uses
┌─────────────────────▼───────────────────┐
│              DOMAIN LAYER               │
│  Entities (Finding, Product, CVE, ...)  │
│  Value Objects (Severity, State, ...)   │
│  Repository interfaces                  │
│  Domain events                          │
└─────────────────────────────────────────┘
```

---

## 7. Security Architecture

### 7.1 Authentication & Authorization

```
Client
  │ JWT (Authorization: Bearer) hoặc API Key (X-Api-Key)
  ▼
Gateway Auth Middleware (apps/osv)
  ├── JWT: parse HS256, check exp, extract claims
  └── API Key: SHA-256(key) → lookup by prefix → compare hash
  │
  ├── Inject X-User-ID, X-User-Role, X-User-Perms
  │
  └── Upstream service trusts injected headers (internal network only)

TOTP MFA (identity-service):
  └── RFC 6238, ±1 period tolerance, AES-256-GCM encrypted secret
```

### 7.2 Data Security

| Layer | Mechanism |
|-------|-----------|
| Transport | TLS 1.3 minimum |
| Passwords | bcrypt (local users) |
| API Keys | SHA-256 stored, constant-time compare, prefix lookup |
| TOTP secrets | AES-256-GCM encrypted in DB |
| JIRA creds | AES-256-GCM encrypted |
| Webhooks | HMAC-SHA256 per payload (`X-OSV-Signature`) |
| Audit events | HMAC-SHA256 per event row |
| SQL queries | Parameterized (no string concat) |
| SSRF protection | Webhook delivery blocks private IPs |
| Account lockout | 5 failed login attempts |

---

## 8. Observability

### 8.1 Logging (zerolog)

```go
log.Info().
    Str("service", "finding-service").
    Str("trace_id", traceID).
    Str("method", r.Method).
    Str("path", r.URL.Path).
    Int("status", w.status).
    Int64("latency_ms", latency.Milliseconds()).
    Str("user_id", userID).
    Msg("request handled")
```

### 8.2 Prometheus Metrics (per service)

```
http_requests_total{method, path, status}
http_request_duration_seconds{method, path}
db_query_duration_seconds{query_name}
cache_hits_total{cache}
cache_misses_total{cache}
nats_messages_published_total{subject}
nats_messages_consumed_total{subject}
nats_consumer_lag{consumer, stream}
```

> **Note:** Mỗi service có `internal/metrics/` package riêng.

---

## 9. Deployment Architecture

### 9.1 Development (Docker Compose)

```bash
# Infrastructure stack
docker-compose up -d postgres redis opensearch nats minio

# Core services
docker-compose up -d data-service search-service ranking-service identity-service

# Finding management
docker-compose up -d finding-service scan-service sla-service \
    notification-service jira-service audit-service

# v3.0 services
docker-compose up -d ai-service product-service asset-service

# Gateway
docker-compose up -d apps-osv

# Ports:
# Gateway:       http://localhost:8080
# Prometheus:    http://localhost:9091
# Grafana:       http://localhost:3000
# NATS Monitor:  http://localhost:8222
# OpenSearch:    http://localhost:9200
```

### 9.2 Repository Structure (Thực Tế)

```
osv.dev/
├── apps/
│   └── osv/                   # API Gateway (main, đang chạy)
│       ├── cmd/               # Main entry point
│       └── internal/
│           ├── gateway/
│           │   ├── router.go  # 150+ routes
│           │   ├── proxy.go   # ReverseProxy implementation
│           │   ├── auth/      # AuthMiddleware, SSE auth, APIKey validator
│           │   ├── bff/       # DashboardBFF, HealthBFF, SettingsBFF
│           │   ├── ratelimit/ # Redis token bucket
│           │   ├── transform/ # InjectUserHeaders, UserScopeFilter
│           │   ├── openapi/   # Schema aggregation
│           │   └── gwerrors/  # Gateway error types
│           ├── config/        # App config
│           ├── infra/         # Postgres pool, Redis client
│           ├── orchestrator/  # Service supervisor, health check
│           └── proxy/         # (additional proxy utilities)
│
├── services/
│   ├── data-service/          # CVE ingestion + enrichment
│   ├── search-service/        # OpenSearch + pgvector
│   ├── ranking-service/       # CPE ranking
│   ├── identity-service/      # Auth + TOTP MFA + API keys
│   │   ├── adapter/           # (top-level adapter)
│   │   └── internal/          # adapter, cache, crypto, delivery,
│   │                          # domain, infra, infrastructure, metrics,
│   │                          # provider, usecase
│   ├── finding-service/       # Products + findings + reports
│   │   └── internal/          # delivery, domain, formatters, infra,
│   │                          # metrics, scheduler, usecase
│   ├── scan-service/          # Active scanners + SCA parsers + SBOM
│   │   ├── agent/             # Agent SDK
│   │   └── internal/          # adapters, delivery/{grpc,http,sse},
│   │                          # domain, infra, infrastructure, metrics,
│   │                          # parsers, sbom, scanner/{nmap,zap},
│   │                          # scheduler, usecase
│   ├── sla-service/           # SLA config + breach detection
│   ├── notification-service/  # 5-channel notifications + SSE
│   ├── jira-service/          # JIRA sync
│   ├── audit-service/         # Immutable event log
│   ├── ai-service/            # Embeddings + triage (LLM chain)
│   │   └── internal/          # delivery/{grpc,http}, domain, infra,
│   │                          # metrics, provider/{ollama,openai}, usecase
│   ├── product-service/       # ProductType/Product/Engagement/Test
│   │   └── internal/          # delivery/http, domain/entity, usecase
│   ├── asset-service/         # Asset registry + risk scoring
│   │   └── internal/          # delivery/http, usecase
│   ├── report-service/        # Standalone report service (v3.0)
│   │   └── internal/          # delivery, domain, formatters, storage, usecase
│   ├── gateway-service/       # gRPC gateway (v3.0 parallel)
│   │   └── internal/          # adapter, auth, bff, cache,
│   │                          # delivery/{grpc,http}, domain, health,
│   │                          # infra, metrics, proxy, ratelimit, usecase
│   └── shared/                # Shared Go packages
│
├── specs/
│   ├── 01-architecture.md     # This document
│   ├── backend_api_specs.md   # API specifications
│   ├── models/                # Data model docs
│   └── crs/
│       ├── v1/                # Change Requests (implemented)
│       │   ├── cve-search/    # 9 CRs ✅
│       │   ├── DefectDojo/    # 11 CRs ✅
│       │   └── globalcve/     # 10 CRs ✅
│       └── v2/                # Change Requests (in progress)
│           └── OpenVulnScan/  # 7 CRs 🔵
│
└── docs/
    ├── PRD.md
    ├── SRS.md
    └── URD.md
```

---

## 10. NATS Event Registry (Đầy Đủ)

| Topic | Publisher | Consumers | Payload |
|-------|-----------|----------|---------| 
| `ingestion.cve.synced` | data-service | ai-service, search-service | `{cve_id, action, summary, cvss}` |
| `finding.created` | finding-service | ai-service (triage), notification-service, audit-service | `{finding_id, cve, severity, product_id}` |
| `finding.status.changed` | finding-service | notification-service, jira-service, audit-service | `{finding_id, from, to, user_id}` |
| `finding.batch_created` | scan-service | notification-service, sla-service, audit-service | `{scan_id, finding_ids[]}` |
| `finding.sla.breached` | sla-service (cron) | notification-service, audit-service | `{finding_id, severity, expires_at}` |
| `kev.new` | data-service | notification-service | `{cve_id, date_added}` |
| `scan.scan.created` | scan-service | scan-service worker | `{scan_id, user_id, type, targets}` |
| `scan.scan.completed` | scan-service | finding-service, asset-service, ai-service, notification-service | `{scan_id, finding_count, scan_type}` |
| `scan.scan.failed` | scan-service | notification-service | `{scan_id, error}` |
| `ai.cve.enriched` | ai-service | data-service (store embedding) | `{cve_id, embedding_dims, severity}` |
| `ai.triage.completed` | ai-service | finding-service (update remarks) | `{finding_id, remarks, confidence}` |
| `jira.issue.created` | jira-service | notification-service, audit-service | `{finding_id, jira_key}` |
| `jira.issue.resolved` | jira-service | finding-service | `{finding_id, jira_key}` |
| `risk_acceptance.*` | finding-service | notification-service, audit-service | `{acceptance_id, finding_id}` |
| `product.created` | product-service | audit-service | `{product_id, name, criticality}` |

---

## 11. Enterprise Architecture Standards (v4.0)

> **Bắt buộc áp dụng cho tất cả services từ phiên bản này.**  
> Mọi vi phạm đều bị coi là **bug** và phải được ghi vào Change Request tương ứng.

### 11.1 Nguyên Tắc Cốt Lõi

#### RULE-001: Zero Mock / Zero Hardcode trong Production Code

| Loại vi phạm | Ví dụ | Yêu cầu |
|--------------|-------|---------|
| Mock Handler | Handler trả static data thay vì gọi usecase | Phải implement usecase thật với repo thật |
| Mock UseCase | UseCase không làm gì (`return nil, nil`) | Phải implement đầy đủ business logic |
| Hardcode String | `"password_policy": "medium"` trong handler | Phải đọc từ DB qua settings repository |
| Hardcode Date | `"2026-06-22T00:00:00Z"` trong response | Phải dùng `time.Now().UTC()` |
| Static Config | Product types trong handler code | Phải lưu trong DB, quản lý qua admin API |
| Nil Handler | Handler = `nil` → panic/404 | Phải implement hoặc trả `501 Not Implemented` |

#### RULE-002: Mọi Data Phải Được Persist Đúng DB

| Loại data | Database phù hợp | Pattern |
|-----------|-----------------|---------|
| Transactional data (users, findings, scans) | PostgreSQL | Repository pattern + pgx |
| CVE/vulnerability data (read-heavy) | PostgreSQL + OpenSearch | Write PG, index OS |
| Vector embeddings | PostgreSQL + pgvector | extension `vector` |
| Session/cache/rate-limit | Redis | TTL-based keys |
| Event streaming | NATS JetStream | At-least-once delivery |
| Binary artifacts (PDF, reports) | MinIO/S3 | Pre-signed URLs |
| Document store (enrichment) | MongoDB | Flexible schema |

#### RULE-003: Clean Architecture Layer Contract

```
┌─────────────────────────────────────────────────────┐
│  DELIVERY LAYER (HTTP Handler / gRPC Server)         │
│  • Decode request → call UseCase → encode response   │
│  • NO business logic, NO DB queries, NO hardcode     │
│  • Returns 400/401/403/404/422/500 chính xác         │
└──────────────────────────┬──────────────────────────┘
                           │ calls
┌──────────────────────────▼──────────────────────────┐
│  USE CASE LAYER (Application Business Logic)         │
│  • Orchestrates: validate → repo → publish → return  │
│  • KHÔNG import infra trực tiếp — chỉ qua interface  │
│  • KHÔNG trả nil, nil khi chưa implement             │
└──────────────────────────┬──────────────────────────┘
                           │ calls interfaces
┌──────────────────────────▼──────────────────────────┐
│  DOMAIN LAYER (Entities + Interfaces)                │
│  • Pure Go — zero external dependencies              │
│  • Repository/Publisher interfaces định nghĩa ở đây │
│  • Domain events, value objects, business rules      │
└──────────────────────────┬──────────────────────────┘
                           │ implemented by
┌──────────────────────────▼──────────────────────────┐
│  INFRASTRUCTURE LAYER (Postgres / Redis / NATS / S3) │
│  • Implements domain interfaces                      │
│  • Connection pooling, retries, circuit breaker      │
│  • KHÔNG được gọi trực tiếp từ handler               │
└─────────────────────────────────────────────────────┘
```

#### RULE-004: Service Completion Contract

Mỗi service phải đạt các tiêu chí sau trước khi merge vào main:

- [ ] **Handler** không chứa `// TODO`, `// FIXME`, `// STUB`, `// MOCK`
- [ ] **UseCase** không trả `return nil, nil` hoặc `return errors.New("not implemented")`
- [ ] **Repository** có đầy đủ CRUD operations được implement thật
- [ ] **Migration SQL** đã chạy và schema tồn tại trên server
- [ ] **Integration test** qua `tests/client/` pass ≥ 80% cho service đó
- [ ] **Health endpoint** service trả 200 với status thật (không giả)

### 11.2 Chuẩn Thiết Kế Repository

#### Repository Interface Contract

```go
// ĐÚNG: Repository interface trong domain layer
type FindingRepository interface {
    Create(ctx context.Context, f *Finding) error
    FindByID(ctx context.Context, id uuid.UUID) (*Finding, error)
    List(ctx context.Context, filter Filter) ([]*Finding, int, error)
    Update(ctx context.Context, f *Finding) error
    Delete(ctx context.Context, id uuid.UUID) error
    Count(ctx context.Context, filter Filter) (int, error)
}

// SAI: Không được làm
func (uc *UseCase) Execute(ctx context.Context, id string) (*Result, error) {
    return nil, nil  // ← BUG: usecase rỗng
}

// SAI: Không được làm
func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
    writeJSON(w, 200, map[string]interface{}{
        "created_at": "2026-01-01T00:00:00Z",  // ← BUG: hardcode
    })
}
```

#### PostgreSQL Conventions

```sql
-- Mọi table PHẢI có:
CREATE TABLE service_entities (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    -- ... domain columns ...
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ  -- soft delete (nếu cần)
);

-- Index bắt buộc:
CREATE INDEX idx_{table}_{column} ON {table}({column});
CREATE INDEX idx_{table}_created ON {table}(created_at DESC);
```

### 11.3 Chuẩn Error Handling

#### HTTP Error Codes

| Scenario | HTTP Code | Response |
|----------|-----------|---------|
| Input validation fail | `422 Unprocessable Entity` | `{"error": "...", "field": "..."}` |
| Auth required | `401 Unauthorized` | `{"error": "authentication required"}` |
| Permission denied | `403 Forbidden` | `{"error": "insufficient permissions"}` |
| Resource not found | `404 Not Found` | `{"error": "not found", "id": "..."}` |
| Business rule violation | `409 Conflict` | `{"error": "..."}` |
| Feature not ready | `501 Not Implemented` | `{"error": "not implemented", "feature": "..."}` |
| Upstream unavailable | `503 Service Unavailable` | `{"error": "upstream unavailable", "service": "..."}` |
| Internal error | `500 Internal Server Error` | `{"error": "internal error"}` (no stack trace) |

> **TUYỆT ĐỐI KHÔNG**: trả `200 OK` cho error, panic trong handler, log stack trace ra client.

### 11.4 Chuẩn Health Check

Mỗi service PHẢI expose `/health` endpoint với response:

```json
{
  "status": "healthy",
  "version": "1.2.3",
  "service": "finding-service",
  "timestamp": "2026-06-23T14:00:00Z",
  "checks": {
    "postgresql": {"status": "healthy", "latency_ms": 3},
    "redis":      {"status": "healthy", "latency_ms": 1},
    "nats":       {"status": "healthy", "latency_ms": 2}
  }
}
```

Status levels:
- `healthy`: tất cả dependencies up, latency bình thường
- `degraded`: service hoạt động nhưng có dependency chậm/cảnh báo
- `unhealthy`: service không thể phục vụ request

### 11.5 Chuẩn Config (Environment Variables)

Mọi configuration phải đến từ environment variables — **KHÔNG hardcode trong code**:

```go
// ĐÚNG:
type Config struct {
    DatabaseURL     string        `env:"DATABASE_URL,required"`
    RedisAddr       string        `env:"REDIS_ADDR" envDefault:"localhost:6379"`
    AIServiceGRPC   string        `env:"AI_SERVICE_GRPC" envDefault:"localhost:50053"`
    MaxConcurrency  int           `env:"MAX_CONCURRENCY" envDefault:"10"`
    SessionTimeout  time.Duration `env:"SESSION_TIMEOUT" envDefault:"1h"`
}

// SAI:
const defaultTimeout = 3600  // ← hardcode
os.Getenv("HOST") + ":8080"  // ← concatenation without validation
```

### 11.6 Phân Loại Bug Priority

| Priority | Điều kiện | Phải fix trong |
|----------|-----------|---------------|
| 🔴 **P0 — Critical** | Production down, data loss, security breach | 24h |
| 🟠 **P1 — High** | Mock/stub trong production, hardcoded business data, feature không hoạt động | 1 sprint |
| 🟡 **P2 — Medium** | TODO trong usecase, unimplemented optional features | 2 sprints |
| 🟢 **P3 — Low** | Code quality, refactor, documentation gap | Backlog |

> **Mọi mock/hardcode** trong production code (non-test) = tối thiểu **P1 — High**.

### 11.7 AI Agent Implementation Rules

Khi AI agent thực hiện code changes, **bắt buộc tuân theo**:

1. **NEVER** viết `return nil, nil` trong UseCase khi chưa implement
2. **NEVER** hardcode timestamp, date, string literals trong handler response
3. **NEVER** để handler gọi trực tiếp SQL — phải qua repository
4. **NEVER** để nil repo/handler trong production routing — dùng `notImplemented()` handler
5. **ALWAYS** inject dependencies qua constructor (không `new()` trong handler)
6. **ALWAYS** persist data vào đúng database theo RULE-002
7. **ALWAYS** implement `Create`/`List`/`Update`/`Delete` trong repository trước khi wire handler
8. **ALWAYS** tạo migration SQL trước khi implement repository
9. **ALWAYS** kiểm tra `tool` output — nếu test fail phải fix, không skip
10. **ALWAYS** viết `// [FIX CR-HC-XXX]` comment khi fix hardcode/mock bug
