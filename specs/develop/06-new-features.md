# 06 — New Features: Tính Năng Mới Đề Xuất

> **Date:** 2026-06-03  
> **Status:** 🔄 In Execution — Xem Implementation Status ở dưới  
> **Horizon:** 12-18 tháng

---

## 1. Tổng Quan Tính Năng Mới

Dựa trên phân tích codebase và nhu cầu của platform CVE management, đây là các tính năng mới đề xuất phát triển, nhóm theo domain.

---

## 2. Source Management Features

### 2.1 Webhook-Based Source Sync

**Vấn đề hiện tại:** Source sync dùng polling theo lịch, latency từ 10 phút đến 12 giờ.

**Đề xuất:** Thêm webhook receiver vào `services/source-sync/`.

```
Implementation:
  GitHub Push → POST /webhooks/github → source-sync → NATS "source.changed"
  GitLab Push → POST /webhooks/gitlab → source-sync → NATS "source.changed"
  
Benefits:
  - Latency giảm từ ~10 phút → < 1 phút cho Git sources
  - Giảm tải polling (không cần kiểm tra liên tục)
  - Near-realtime CVE tracking
  
Effort: 2-3 sprints
Priority: HIGH — ảnh hưởng trực tiếp data freshness SLA
```

### 2.2 Source Health Dashboard

**Đề xuất:** Admin UI component cho source management.

```
Features:
  - Realtime status grid (30+ sources)
  - Last sync time + next scheduled sync
  - Error rate trend (7 ngày)
  - Circuit breaker state per source
  - Quick actions: pause/resume/trigger

Tech: Go backend (services/admin/) + React frontend
Effort: 3-4 sprints
Priority: MEDIUM
```

### 2.3 Source Credential Manager

**Đề xuất:** Centralized credential management thay vì hardcode secrets.

```go
// Interface đề xuất
type CredentialManager interface {
    // Lấy SSH key cho git source
    GetSSHKey(ctx context.Context, sourceName string) (*SSHCredential, error)
    
    // Lấy API token
    GetToken(ctx context.Context, sourceName string) (string, error)
    
    // Rotate credential (trigger manual hoặc theo schedule)
    RotateCredential(ctx context.Context, sourceName string) error
    
    // Kiểm tra expiry
    GetExpiry(ctx context.Context, sourceName string) (time.Time, error)
}

// Backend options:
// 1. GCP Secret Manager (production)
// 2. Kubernetes Secrets (staging)
// 3. .env file (dev)

Effort: 2 sprints
Priority: HIGH — security requirement
```

---

## 3. CVE Classification & Enrichment Features

### 3.1 CISA KEV Integration

**Vấn đề:** Hiện tại không có tracking CVE nào đang bị khai thác trong thực tế.

**CISA Known Exploited Vulnerabilities (KEV) Catalog:**
- URL: https://www.cisa.gov/sites/default/files/feeds/known_exploited_vulnerabilities.json
- Update: Hàng ngày
- Format: JSON với `cveID`, `vendorProject`, `product`, `vulnerabilityName`, `dateAdded`, `shortDescription`, `requiredAction`, `dueDate`

```go
// services/pkg/clients/kev/kev_client.go
type KEVClient interface {
    // Fetch toàn bộ KEV catalog
    FetchCatalog(ctx context.Context) (*KEVCatalog, error)
    
    // Kiểm tra một CVE có trong KEV không
    IsKEV(ctx context.Context, cveID string) (bool, *KEVEntry, error)
}

// ai-enrichment sẽ gọi KEV client và tag vulnerability
// Trigger: hàng ngày lúc 00:00 UTC + ngay khi ingest CVE mới

// Storage:
// - Thêm field kev_date_added vào enriched record
// - Thêm field kev_required_action
// - Index "kev" tag trong OpenSearch

Effort: 1-2 sprints
Priority: HIGH — critical for threat intel
```

### 3.2 EPSS Score Integration

**EPSS (Exploit Prediction Scoring System):**
- URL: https://api.first.org/data/v1/epss?cve=CVE-xxxx
- Update: Hàng ngày
- Range: 0.0 → 1.0 (probability of exploitation in next 30 days)
- Percentile: so sánh với tất cả CVE

```go
// services/pkg/clients/epss/epss_client.go
type EPSSClient interface {
    GetEPSS(ctx context.Context, cveID string) (*EPSSScore, error)
    GetBatchEPSS(ctx context.Context, cveIDs []string) (map[string]*EPSSScore, error)
}

type EPSSScore struct {
    CVE        string
    EPSS       float64   // 0.0 - 1.0
    Percentile float64   // 0.0 - 1.0
    Date       time.Time
}

// Enrichment flow:
// 1. Khi ingest CVE mới → fetch EPSS score
// 2. Hàng ngày: batch update EPSS cho tất cả CVE (EPSS thay đổi theo ngày)
// 3. Store trong enriched record + index trong OpenSearch

// Alert rule:
// EPSS_percentile > 0.95 → gửi high-priority alert

Effort: 1 sprint
Priority: HIGH — key metric cho prioritization
```

### 3.3 Auto CVE Tagging System

**Đề xuất:** Hệ thống tag tự động dựa trên rules + ML.

```go
// services/ai-enrichment/internal/domain/tagging/
type TaggerEngine struct {
    ruleTagger  RuleTagger   // Fast, deterministic
    mlTagger    MLTagger     // AI-based, slower
}

// Rule-based tags (fast, deterministic):
type RuleTagger interface {
    // Tag từ CVSS vector
    TagFromCVSS(vector string) []string
    // Tag từ keyword trong description
    TagFromKeywords(description string) []string
    // Tag từ CWE IDs
    TagFromCWE(cweIDs []string) []string
    // Tag từ affected packages
    TagFromPackages(packages []Package) []string
}

// Rule examples:
// AV:N → "network-based"
// C:H/I:H/A:H → "critical-impact"
// description contains "SQL" → "sqli"
// description contains "buffer overflow" → "memory-safety"
// cwe-89 → "sqli", "injection"
// package: kernel/* → "kernel"

// ML-based tags (via ai-enrichment LLM):
// - Attack technique tags (MITRE ATT&CK)
// - Product category tags
// - Exploitation complexity tags

Standard Tag Taxonomy:
  attack_vector: [network, adjacent, local, physical]
  attack_type: [rce, privesc, dos, info-disc, sqli, xss, ssrf, xxe, path-traversal, ...]
  impact: [confidentiality, integrity, availability]
  complexity: [low, high]
  auth_required: [none, low, high]
  scope: [unchanged, changed]
  phase: [exploitation, post-exploitation, initial-access]
  asset_type: [web, network, kernel, mobile, cloud, iot]
  status: [has-fix, no-fix, withdrawn, disputed, kev, wontfix]

Effort: 3-4 sprints
Priority: MEDIUM — enhances search and analytics
```

### 3.4 CWE & CAPEC Integration

```go
// services/pkg/cwe/
type CWEDatabase interface {
    // Lookup CWE by ID
    Get(cweID string) (*CWE, error)
    
    // Get all ancestors (e.g., CWE-89 → CWE-943 → CWE-77 → CWE-74)
    GetAncestors(cweID string) ([]*CWE, error)
    
    // Map to CAPEC attack patterns
    GetCAPEC(cweID string) ([]*CAPEC, error)
    
    // Map CVSS vector → likely CWE IDs
    SuggestFromCVSS(vector string) ([]*CWE, error)
}

// Data source: NVD CWE list (XML)
// Update: Hàng quý theo NVD release

Effort: 2 sprints
Priority: MEDIUM
```

---

## 4. Search & Discovery Features

### 4.1 Semantic/Vector Search

**Đề xuất:** Thêm vector search vào `services/search/`.

```
Architecture:
  ai-enrichment → generate embedding → store in OpenSearch k-NN index
  User query → generate embedding → k-NN search → rerank by text score
  
API Endpoint:
  POST /v1/search/semantic
  {
    "query": "HTTP/2 rapid reset attack",
    "k": 10,
    "filter": {"ecosystem": "Go", "min_severity": "HIGH"}
  }

Benefits:
  - Tìm CVE tương tự mà không cần biết chính xác tên
  - "CVE giống CVE-2023-44487" → tìm HTTP/2 related CVEs
  - Cross-language search (query tiếng Việt tìm CVE tiếng Anh)

Tech: OpenSearch k-NN plugin (đã có trong stack)
Effort: 2-3 sprints
Priority: MEDIUM
```

### 4.2 Faceted Search & Aggregations

```
Features:
  - Filter by: ecosystem, severity, year, source, has-fix, kev, epss-range
  - Sort by: published_date, modified_date, epss_score, cvss_score
  - Aggregations: CVE count by ecosystem, severity distribution, trend over time
  
API:
  POST /v1/search/faceted
  {
    "query": "deserialization",
    "filters": {
      "ecosystem": ["Java", "Python"],
      "severity": ["HIGH", "CRITICAL"],
      "kev": true,
      "has_fix": false
    },
    "sort": "epss_score:desc",
    "aggs": ["by_ecosystem", "by_year"]
  }

Effort: 2 sprints
Priority: MEDIUM
```

### 4.3 Saved Searches & Alerts

```go
// Cho phép users subscribe vào CVE queries
type SavedSearch struct {
    ID          string
    Name        string
    Query       SearchQuery
    Channels    []NotificationChannel   // slack, email, webhook
    MinSeverity Severity                // alert threshold
    KEVOnly     bool                    // only alert on KEV
    Ecosystems  []string                // filter
    CreatedAt   time.Time
    LastAlerted time.Time
}

// Trigger: Mỗi khi CVE mới được ingest → check tất cả saved searches
// Nếu match → gửi notification qua channels

Effort: 3 sprints
Priority: LOW (Phase 2)
```

---

## 5. API Enhancement Features

### 5.1 API v2 — Extended Query Capabilities

```protobuf
// Proposed v2 additions
service OSVv2 {
  // v1 endpoints kept for backward compat
  
  // NEW: Query with filters
  rpc QueryWithFilters(QueryWithFiltersRequest) returns (VulnerabilityList);
  
  // NEW: Get related vulnerabilities
  rpc GetRelated(GetRelatedRequest) returns (VulnerabilityList);
  
  // NEW: Get CVE timeline (history of changes)
  rpc GetTimeline(GetTimelineRequest) returns (VulnerabilityTimeline);
  
  // NEW: Batch get by multiple IDs
  rpc BatchGetById(BatchGetByIdRequest) returns (BatchVulnerabilityResponse);
  
  // NEW: Get enrichment data (EPSS, KEV, tags)
  rpc GetEnrichment(GetEnrichmentRequest) returns (EnrichmentData);
}
```

### 5.2 API Key Management

```go
// services/admin/ — API key management
type APIKeyService interface {
    CreateKey(ctx context.Context, req CreateKeyRequest) (*APIKey, error)
    RevokeKey(ctx context.Context, keyID string) error
    ListKeys(ctx context.Context, ownerID string) ([]*APIKey, error)
    ValidateKey(ctx context.Context, key string) (*APIKey, error)
    GetUsage(ctx context.Context, keyID string, period time.Duration) (*Usage, error)
}

type APIKey struct {
    ID          string
    Key         string      // hashed
    Name        string
    OwnerID     string
    RateLimit   int         // requests per minute
    Quota       int64       // total requests per month
    ExpiresAt   *time.Time
    Scopes      []string    // ["read:vulns", "read:search"]
    CreatedAt   time.Time
}

Effort: 3 sprints
Priority: MEDIUM — needed for production API management
```

---

## 6. Operations & Admin Features

### 6.1 Admin Dashboard (Web UI)

```
Tech Stack: Go backend (services/admin/) + React frontend

Pages:
  1. Overview
     - Total CVE count (today/week/month/all-time)
     - Source health grid
     - Pipeline throughput chart
     - Alert feed

  2. Sources
     - List all 30+ sources với status
     - Per-source detail: sync history, error log
     - Actions: pause/resume/trigger sync

  3. Import Findings
     - List recent import errors
     - Categorized by type (schema error, validation error, etc.)
     - Per-source error rate trend

  4. Vulnerabilities
     - Search + filter (admin view, includes withdrawn)
     - Actions: withdraw, reprocess, add note

  5. System
     - Service health (all microservices)
     - Queue depths (NATS)
     - Cache hit rates (Redis)
     - Trace/log viewer links

Effort: 6-8 sprints (significant UI work)
Priority: MEDIUM — ops team needs this
```

### 6.2 Data Quality Monitoring

```go
// Tự động phát hiện và báo cáo data quality issues
type DataQualityMonitor interface {
    // Check: CVE có CVSS score không?
    CheckMissingCVSS(ctx context.Context) ([]string, error)
    
    // Check: CVE có affected versions không?
    CheckMissingVersions(ctx context.Context) ([]string, error)
    
    // Check: CVE aliases đã resolved chưa?
    CheckUnresolvedAliases(ctx context.Context) ([]string, error)
    
    // Check: CVE từ nguồn X đã có bản đối chiếu từ nguồn Y?
    CheckCrossSourceCoverage(ctx context.Context, src1, src2 string) (*CoverageReport, error)
    
    // Scheduled: hàng ngày generate quality report
    GenerateDailyReport(ctx context.Context) (*QualityReport, error)
}

Effort: 2-3 sprints
Priority: MEDIUM
```

### 6.3 Audit Trail

```go
// Ghi lại tất cả admin operations
type AuditLog struct {
    ID         string
    Timestamp  time.Time
    Actor      string      // admin user hoặc service account
    Action     string      // "withdraw", "reprocess", "pause_source"
    Resource   string      // CVE ID hoặc source name
    Before     any         // state before
    After      any         // state after
    Reason     string      // optional justification
}

// Retention: 90 days
// Storage: Firestore collection "audit_logs"
// Export: CSV download for compliance

Effort: 1-2 sprints
Priority: LOW (compliance need)
```

---

## 7. Developer Experience Features

### 7.1 OSV CLI Enhancement (`apps/cli/`)

```bash
# Hiện tại: minimal implementation
# Đề xuất thêm:

# Source management
cvectl sources list
cvectl sources status ghsa
cvectl sources sync ghsa
cvectl sources pause redhat
cvectl sources resume redhat

# CVE lookup
cvectl vuln get CVE-2023-44487
cvectl vuln search "HTTP/2 rapid reset"
cvectl vuln related CVE-2023-44487
cvectl vuln history CVE-2023-44487

# Admin operations
cvectl admin import-findings list
cvectl admin import-findings resolve <id>
cvectl admin reprocess CVE-2023-44487
cvectl admin stats

# Export
cvectl export --ecosystem PyPI --format json > cves.json
cvectl export --kev --format csv > kev_cves.csv
```

### 7.2 Local Development Improvements

```yaml
# docker-compose additions
services:
  # Thêm Jaeger cho distributed tracing
  jaeger:
    image: jaegertracing/all-in-one:latest
    ports:
      - "16686:16686"  # UI
      - "14268:14268"  # HTTP collector

  # Thêm NATS UI
  nats-ui:
    image: sphqxe/natster:latest
    ports:
      - "4000:4000"

  # Thêm Swagger UI cho API docs
  swagger-ui:
    image: swaggerapi/swagger-ui
    ports:
      - "8888:8080"
    environment:
      SWAGGER_JSON_URL: http://api-gateway:8080/openapi.json
```

---

## 8. Feature Priority Matrix

| Feature | Business Value | Effort | Priority | Quarter |
|---------|---------------|--------|----------|---------|
| KEV Integration | 🔴 High | Low | **P0** | Q3 |
| EPSS Integration | 🔴 High | Low | **P0** | Q3 |
| Webhook-based sync | 🔴 High | Medium | **P0** | Q3 |
| Source Credential Manager | 🔴 High | Medium | **P0** | Q3 |
| Auto CVE Tagging | 🟠 Medium | High | **P1** | Q3-Q4 |
| Admin API (backend) | 🟠 Medium | Medium | **P1** | Q4 |
| API v2 | 🟠 Medium | High | **P1** | Q4 |
| Semantic Search | 🟠 Medium | Medium | **P1** | Q4 |
| Admin Dashboard (UI) | 🟡 Medium | Very High | **P2** | Q1 2027 |
| Saved Searches/Alerts | 🟡 Medium | High | **P2** | Q1 2027 |
| CWE/CAPEC Integration | 🟡 Low-Medium | Medium | **P2** | Q1 2027 |
| Data Quality Monitor | 🟡 Medium | Medium | **P2** | Q2 2027 |
| CLI Enhancement | 🟢 Low | Medium | **P3** | Q2 2027 |
| Audit Trail | 🟢 Low | Low | **P3** | Q2 2027 |

---

## 8. Implementation Status (2026-06-03)

> Cập nhật theo thực tế đã triển khai.

### 8.1 Source Management Features

| Tính năng | Proposal | Thực tế | Status |
|---------|----------|---------|--------|
| Webhook-Based Source Sync | GitHub + GitLab webhooks | `source-sync/infra/webhook/webhook.go` | ✅ DONE |
| Source Health (backend API) | Admin API list sources | `admin/handler.go` `ListSources()` | ✅ DONE |
| Source Admin API | Pause/Resume/Trigger | `admin/handler.go` 5 source endpoints | ✅ DONE |
| Source Credential Manager | GCP Secret Manager | 📋 TODO (TASK-03-02, 3d) | 📋 TODO |

### 8.2 CVE Classification & Enrichment

| Tính năng | Proposal | Thực tế | Status |
|---------|----------|---------|--------|
| CISA KEV Integration | KEV client + tagging | `pkg/clients/kev/` (8 tests) + `KEVStage` | ✅ DONE |
| EPSS Score Integration | Fetch + daily update | `pkg/clients/epss/` (4 tests) + `EPSSStage` | ✅ DONE |
| CWE Classification | Map CVE → CWE | `pkg/cwe/` (12 tests) + `CWEStage` | ✅ DONE |
| ADP Container Merging | CISA/NVD ADP enrichment | `converter/domain/cve5/adp.go` | ✅ DONE |
| Auto CVE Tagging (rule-based) | TagFromCVSS/Keywords/CWE | `pkg/classification/` (12 tests) | ✅ DONE |
| Auto CVE Tagging (ML) | LLM-based MITRE ATT&CK | 📋 TODO (TASK-05-04) | 📋 TODO |
| CAPEC Mapping | CWE → CAPEC patterns | 📋 TODO (follow-up) | 📋 TODO |
| Exploitability Check | PoC GitHub/ExploitDB | 📋 TODO (TASK-05-03) | 📋 TODO |
| High-Risk CVE Alerts | EPSS percentile > 0.95 | 📋 TODO (TASK-05-06) | 📋 TODO |

### 8.3 Admin Service & Operations

| Tính năng | Proposal | Thực tế | Status |
|---------|----------|---------|--------|
| Admin REST API (backend) | 12 endpoints | `admin/handler.go` — tất cả 12 done | ✅ DONE |
| Source management | List/Get/Sync/Pause/Resume | 5 handlers | ✅ DONE |
| Import findings | List/Resolve errors | 2 handlers | ✅ DONE |
| Vulnerability admin | Withdraw/Reprocess/Stats | 3 handlers | ✅ DONE |
| System health endpoint | 6 component health | `SystemHealth()` | ✅ DONE |
| API key management | Create/List/Revoke | 3 handlers | ✅ DONE |
| Data quality monitoring | Quality check jobs | 📋 TODO (TASK-06-04) | 📋 TODO |
| Audit trail | Log all write ops | 📋 TODO (TASK-06-05) | 📋 TODO |
| Admin Dashboard (UI) | React frontend | 📋 TODO (separate project) | 📋 TODO |

### 8.4 Search & Discovery

| Tính năng | Proposal | Thực tế | Status |
|---------|----------|---------|--------|
| OpenSearch index mapping | KEV+EPSS+tags+k-NN vector | `search/entity/entity.go` | ✅ DONE |
| Semantic/Vector Search | k-NN OpenSearch | Index mapping done; query impl TODO | 🔄 IN PROGRESS |
| Faceted Search | Aggregations + buckets | 📋 TODO (TASK-07-03) | 📋 TODO |
| Saved Searches & Alerts | Subscribe + notify | 📋 TODO (TASK-07-04) | 📋 TODO |

### 8.5 API & Developer Experience

| Tính năng | Proposal | Thực tế | Status |
|---------|----------|---------|--------|
| API v2 (enrichment, related, timeline) | Extended API | `api-gateway/infra/handlers/v2/` | ✅ DONE |
| Batch CVE lookup | POST /v2/batch | `BatchHandler` | ✅ DONE |
| `cvectl` CLI | cobra/viper CLI | search, get, list, enrich, diff | ✅ DONE |
| Rate limiting + quota | Per API key | 📋 TODO (TASK-08-03) | 📋 TODO |
| OpenAPI spec | Swagger docs | 📋 TODO | 📋 TODO |

### 8.6 Priority Matrix Cập Nhật

| Tính năng | Impact | Priority | Status | Scheduled |
|---------|--------|----------|--------|-----------|
| KEV Integration | 🔴 High | **P0** | ✅ DONE | Q3 2026 |
| EPSS Integration | 🔴 High | **P0** | ✅ DONE | Q3 2026 |
| CWE Classification | 🔴 High | **P0** | ✅ DONE | Q3 2026 |
| Webhook-based sync | 🔴 High | **P0** | ✅ DONE | Q3 2026 |
| ADP Container Merging | 🔴 High | **P0** | ✅ DONE | Q4 2026 |
| Auto CVE Tagging (rule) | 🟠 Medium | **P1** | ✅ DONE | Q3-Q4 |
| Admin API (backend) | 🟠 Medium | **P1** | ✅ DONE | Q4 2026 |
| API v2 | 🟠 Medium | **P1** | ✅ DONE | Q4 2026 |
| cvectl CLI | 🟠 Medium | **P1** | ✅ DONE | Q4 2026 |
| OpenSearch index mapping | 🟠 Medium | **P1** | ✅ DONE | Q4 2026 |
| Source Credential Manager | 🔴 High | **P0** | 📋 TODO | Q1 2027 |
| Semantic Search | 🟠 Medium | **P1** | 🔄 In Progress | Q1 2027 |
| Exploitability Check | 🟠 Medium | **P1** | 📋 TODO | Q2 2027 |
| Admin Dashboard (UI) | 🟡 Medium | **P2** | 📋 TODO | Q2 2027 |
| Saved Searches/Alerts | 🟡 Medium | **P2** | 📋 TODO | Q2 2027 |
| CAPEC Mapping | 🟡 Low | **P2** | 📋 TODO | Q2 2027 |
| Data Quality Monitor | 🟡 Medium | **P2** | 📋 TODO | Q3 2027 |
| Audit Trail | 🟢 Low | **P3** | 📋 TODO | Q3 2027 |
| Auto CVE Tagging (ML) | 🟢 Low | **P3** | 📋 TODO | Q3 2027 |
