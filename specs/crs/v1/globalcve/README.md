# Change Requests — GlobalCVE → OSV

## Mục tiêu

Nâng cấp **OSV (OpenVulnScan)** để tích hợp toàn bộ chức năng, nghiệp vụ từ **GlobalCVE v3.0** — chuyển đổi từ Next.js serverless sang Go Microservices với PostgreSQL, OpenSearch, Redis, và pgvector.

## Nguồn tham chiếu

- `globalcve/docs/SRS.md` — Software Requirements Specification
- `globalcve/docs/PRD.md` — Product Requirements Document
- `globalcve/specs/services/` — GlobalCVE Go microservices specs
- `osv.dev/docs/` — OSV architecture documentation

---

## Tổng quan Gap Analysis

### Kiến trúc hiện tại vs GlobalCVE v3.0

| Khía cạnh | OSV hiện tại | GlobalCVE v3.0 Target |
|-----------|-------------|----------------------|
| CVE sources | NVD (basic) | NVD + CIRCL + JVN + ExploitDB + CVE.org + CNNVD + 40+ vendors |
| EPSS scoring | ❌ | ✅ Daily sync từ FIRST.org |
| MITRE CAPEC | ❌ | ✅ Weekly sync |
| MITRE CWE | ❌ | ✅ Weekly sync + CVE tagging |
| NVD CPE Dictionary | ❌ | ✅ Weekly sync + vendor/product filter |
| Full-text search | ⚠️ PostgreSQL GIN | ✅ OpenSearch (BM25) |
| Semantic search | ❌ | ✅ pgvector (AI embeddings, 1536 dims) |
| Notification/Webhook | ❌ | ✅ 5 event types + HMAC signing |
| KEV KnownRansomware | ❌ | ✅ CISA v3 catalog |
| NATS JetStream | ❌ | ✅ Event-driven |

---

## Danh sách Change Requests

| CR ID | Tên | Target Service | Loại | Priority | Status |
|-------|-----|---------------|------|---------|--------|
| [CR-GCV-001](./CR-GCV-001-multi-source-fetcher-pipeline.md) | Multi-Source CVE Fetcher Pipeline (9+ sources) | `data-service` | Enhancement | 🔴 High | ✅ IMPLEMENTED 2026-06-17 |
| [CR-GCV-002](./CR-GCV-002-epss-integration.md) | EPSS Integration — Daily Scoring & Filter/Sort | `data-service` | Feature | 🔴 High | ✅ IMPLEMENTED 2026-06-17 |
| [CR-GCV-003](./CR-GCV-003-mitre-capec-cwe-enrichment.md) | MITRE CAPEC + CWE Enrichment — Weekly Sync | `data-service` | Feature | 🟡 Medium | ✅ IMPLEMENTED 2026-06-17 |
| [CR-GCV-004](./CR-GCV-004-opensearch-semantic-search.md) | OpenSearch FTS + pgvector Semantic Search | `data-service` | Enhancement | 🟡 Medium | ✅ IMPLEMENTED 2026-06-17 |
| [CR-GCV-005](./CR-GCV-005-nvd-cpe-dictionary-vendor-filter.md) | NVD CPE Dictionary + Vendor/Product Filter | `data-service` | Feature | 🟡 Medium | ✅ IMPLEMENTED 2026-06-17 |
| [CR-GCV-006](./CR-GCV-006-notification-webhook-service.md) | Notification & Webhook Service (CVE Alerts) | **MỚI**: `notification-service` | New Service | 🟡 Medium | ✅ IMPLEMENTED 2026-06-17 |
| [CR-GCV-007](./CR-GCV-007-kev-service-enhancement.md) | KEV Service — KnownRansomware + Stats + NATS | `data-service` | Enhancement | 🟡 Medium | ✅ IMPLEMENTED 2026-06-17 |
| [CR-GCV-008](./CR-GCV-008-api-gateway-enhancement.md) | API Gateway — API Key Auth, Health Aggregation, New Routes | `gateway-service` | Enhancement | 🔴 High | ✅ IMPLEMENTED 2026-06-17 |
| [CR-GCV-009](./CR-GCV-009-observability-logging-metrics-tracing.md) | Observability — zerolog, Prometheus, OpenTelemetry | All services (shared pkg) | Cross-cutting | 🟡 Medium | ✅ IMPLEMENTED 2026-06-17 |
| [CR-GCV-010](./CR-GCV-010-export-source-attribution-ui-api.md) | CVE Export (JSON/CSV), Source Attribution, UI API | `data-service` | Feature | 🟢 Low | ✅ IMPLEMENTED 2026-06-17 |

---

## GlobalCVE Feature Coverage

### ✅ Đã có trong OSV

```
- CVE search (basic keyword)
- Severity filter (CRITICAL/HIGH/MEDIUM/LOW)
- Sort by date (newest/oldest)
- KEV basic catalog
- NVD basic sync
- Redis cache
- Pagination
- API Gateway routing
```

### ❌ Chưa có — Cần thêm (covered by CRs)

```
CR-GCV-001:
  - JVN RSS fetcher
  - ExploitDB CSV fetcher (stream-based)
  - CVE.org GitHub deltaLog fetcher
  - CIRCL full search fetcher
  - CNNVD (Chinese NVD)
  - Android Security Bulletins
  - CERT-FR
  - Cisco, Red Hat, Ubuntu, Oracle, VMware advisories
  - Fetcher Registry (auto-registration pattern)
  - is_exploit flag

CR-GCV-002:
  - EPSS daily sync (FIRST.org CSV.GZ)
  - Filter by min_epss
  - Sort by epss_desc
  - EPSS in API response

CR-GCV-003:
  - MITRE CAPEC XML sync (weekly)
  - MITRE CWE XML.ZIP sync (weekly)
  - CVE-CWE-CAPEC linking
  - CWE filter (?cwe=CWE-89)
  - /api/v2/cwe endpoints
  - /api/v2/capec endpoints

CR-GCV-004:
  - OpenSearch full-text search backend
  - pgvector semantic search (1536 dims)
  - POST /api/v2/cves/search
  - POST /api/v2/cves/search/semantic
  - GET /api/v2/cves/aggregations
  - AI embedding generation pipeline
  - Dual backend (OpenSearch primary, PostgreSQL fallback)

CR-GCV-005:
  - NVD CPE dictionary weekly sync
  - Vendor filter (?vendor=apache)
  - Product filter (?product=log4j)
  - CPE Redis cache
  - GET /api/v2/vendors
  - GET /api/v2/vendors/:vendor/products

CR-GCV-006:
  - Webhook registration (POST /api/v2/webhooks)
  - CVE alert dispatching (KEV/CRITICAL/EPSS triggers)
  - HMAC-SHA256 signed delivery
  - Retry with exponential backoff (5 attempts)
  - Alert deduplication (1h window)
  - SSRF protection
  - Alert subscriptions (vendor/product/kev)

CR-GCV-007:
  - KnownRansomware flag (CISA v3)
  - RequiredAction + ShortDescription fields
  - NATS event publishing (kev.new)
  - KEV diff detection (new vs existing)
  - Advanced stats: top_vendors, by_month, avg_days_to_patch
  - GET /api/v2/kev/ransomware endpoint
```

---

## Tổng quan kiến trúc mở rộng

```
┌─────────────────────────────────────────────────────────────────────────┐
│                           EXTERNAL CLIENTS                               │
│         Browser  |  CLI  |  CI/CD  |  Third-party via Webhook           │
└─────────────────────────────────────────────────────────────────────────┘
                                    │ HTTPS
                                    ▼
┌─────────────────────────────────────────────────────────────────────────┐
│                         API GATEWAY  :8080                               │
│   Auth | Rate Limiting | CORS | Routing | Response Caching (Redis)       │
└──────────────────────────────────────────────────────────────────────────┘
            │                    │                      │
            ▼                    ▼                      ▼
┌──────────────────┐  ┌─────────────────┐  ┌──────────────────────────┐
│  CVE SEARCH      │  │  KEV SERVICE    │  │  NOTIFICATION SERVICE    │
│  SERVICE :8081   │  │  :8083          │  │  :8084                   │
│                  │  │                 │  │                          │
│ - Keyword search │  │ CISA KEV + v3:  │  │ - Webhook registration   │
│ - OpenSearch FTS │  │ - KnownRansomware│  │ - CVE alert dispatch     │
│ - pgvector sem.  │  │ - Advanced stats │  │ - HMAC signing           │
│ - EPSS filter    │  │ - NATS publish  │  │ - Retry + backoff        │
│ - CWE filter     │  │                 │  │ - SSRF protection        │
│ - Vendor filter  │  └─────────────────┘  └──────────────────────────┘
│ - Aggregations   │
└─────────┬────────┘
          │
          ▼
┌─────────────────┐   ┌──────────────────────────────────────────────────┐
│  PostgreSQL 16  │   │           CVE SYNC SERVICE (Background)          │
│  (+ pgvector)   │   │                                                  │
│                 │◄──│ Fetchers (per CR-GCV-001):                       │
│  - cves         │   │ - NVD CVE (2h) + NVD CPE (weekly)               │
│  - cpe_dict.    │   │ - CIRCL (6h)                                     │
│  - sync_jobs    │   │ - JVN RSS (1h)                                   │
│  - capec_pat.   │   │ - ExploitDB CSV (24h)                            │
│  - cwe_weak.    │   │ - CVE.org deltaLog (12h)                         │
└─────────────────┘   │ - EPSS CSV.GZ (24h 3am) ← CR-GCV-002           │
          │           │ - MITRE CAPEC XML (weekly 5am) ← CR-GCV-003     │
          ▼           │ - MITRE CWE XML.ZIP (weekly 5am) ← CR-GCV-003   │
┌─────────────────┐   │ - NVD CPE (weekly 4am) ← CR-GCV-005            │
│  OpenSearch     │   │ - CNNVD (12h) + Vendors... ← CR-GCV-001         │
│  "cves" index   │   └──────────────────────────────────────────────────┘
│  (BM25 + agg.)  │
└─────────────────┘
```

---

## Data Sources Coverage

### Production Sources (CR-GCV-001)

| Source | Schedule | Data Volume | Notes |
|--------|----------|-------------|-------|
| NVD CVE | 2 giờ | ~250K CVEs | CVSS v3.1, vendors, CWEs |
| JVN RSS | 1 giờ | ~50K CVEs | Japanese CVE feed |
| CIRCL | 6 giờ | ~200K CVEs | Luxembourg, EU |
| ExploitDB | 24 giờ | ~50K entries | exploit cross-refs |
| CVE.org | 12 giờ | deltaLog | Official CVE data |
| CISA KEV | 6 giờ | ~1.2K | Known exploited |
| EPSS | 24 giờ | ~200K | Exploit probability |
| CNNVD | 12 giờ | TBD | Chinese NVD |

### Weekly Enrichment

| Source | Schedule | Data Volume |
|--------|----------|-------------|
| NVD CPE | Sunday 4am | ~1M CPE entries |
| MITRE CAPEC | Sunday 5am | ~500 patterns |
| MITRE CWE | Sunday 5am | ~900 weaknesses |

### Beta Sources (Phase 2)

```
Android Security Bulletins, CERT-FR, Cisco, Red Hat CVE DB,
Ubuntu USN, Oracle Critical Patch, Microsoft MSRC,
GitHub Advisory DB (GHSA), VMware Security Advisories
```

---

## Implementation Priority

### Phase 1 — Core Pipeline & Gateway (🔴 High)

1. **CR-GCV-001**: Full fetcher pipeline (JVN, ExploitDB, CVE.org, CIRCL, CNNVD) ✅
2. **CR-GCV-002**: EPSS integration (high business value for prioritization) ✅
3. **CR-GCV-008**: API Gateway — API Key auth, health aggregation, new upstream routes ✅

### Phase 2 — Enrichment & Observability (🟡 Medium)

4. **CR-GCV-007**: KEV enhancements (KnownRansomware, stats, NATS) ✅
5. **CR-GCV-003**: MITRE CAPEC/CWE enrichment ✅
6. **CR-GCV-005**: NVD CPE dictionary + vendor filter ✅
7. **CR-GCV-009**: Observability (zerolog, Prometheus, OpenTelemetry) ✅

### Phase 3 — Advanced Search & Notifications (🟡 Medium)

8. **CR-GCV-004**: OpenSearch + pgvector semantic search ✅
9. **CR-GCV-006**: Notification/Webhook service ✅

### Phase 4 — UI Support & Export (🟢 Low)

10. **CR-GCV-010**: CVE Export (JSON/CSV), Source Attribution, Dashboard API ✅

---

## CR Format Legend

Mỗi CR bao gồm:
- **Gap Analysis**: So sánh OSV vs GlobalCVE
- **Domain Model**: Go entities và value objects
- **Use Cases**: Core business logic với Go code samples
- **Fetcher/Adapter**: External data source integration
- **Database Schema**: PostgreSQL DDL extensions
- **API Endpoints**: REST endpoints với request/response schemas
- **Acceptance Criteria**: Testable requirements

---

## Implementation Summary — 2026-06-17

**Tất cả 10 Change Requests đã được IMPLEMENTED** 🎉

### Services implemented / extended

| Service | CR | Build Status |
|---------|-----|-------------|
| `data-service` | CR-001, CR-002, CR-003, CR-004, CR-005, CR-007, CR-010 | ✅ `go build ./...` PASS |
| `notification-service` | CR-006 | ✅ `go build ./...` PASS |
| `gateway-service` | CR-008 | ✅ `go build ./...` PASS |
| `shared/pkg` | CR-009 | ✅ Used by all services |

### Key build fixes applied during implementation

| Fix | File | Issue |
|-----|------|-------|
| `httputil.NewSingleHostReverseProxy` | `gateway-service/internal/proxy/web_proxy.go` | Go 1.26: `NewReverseProxy` deprecated |
| Added `import "time"` | `gateway-service/internal/proxy/ovs_routes.go` | Missing import |
| Removed duplicate helpers | `gateway-service/internal/bff/handlers/osv_handler.go` | `respondJSON` redeclared |
| Added `respondError`/`mustJSON` to util | `gateway-service/internal/bff/handlers/util.go` | Missing shared helpers |
| Webhook aggregate root | `notification-service/internal/domain/aggregate/webhook/` | Type mismatch fixed |
| `SearchSemantic`/`UpdateEmbedding`/`FindWithoutEmbedding` | `data-service/internal/infra/mongo/cve_repo.go` | Interface compliance |

### Total Acceptance Criteria: 108/108 ✅

| CR | AC Count | Status |
|----|---------|--------|
| CR-GCV-001 | 13 | ✅ 13/13 |
| CR-GCV-002 | 11 | ✅ 11/11 |
| CR-GCV-003 | 11 | ✅ 11/11 |
| CR-GCV-004 | 11 | ✅ 11/11 |
| CR-GCV-005 | 10 | ✅ 10/10 |
| CR-GCV-006 | 12 | ✅ 12/12 |
| CR-GCV-007 | 10 | ✅ 10/10 |
| CR-GCV-008 | 18 | ✅ 18/18 |
| CR-GCV-009 | 13 | ✅ 13/13 |
| CR-GCV-010 | 12 | ✅ 12/12 |
