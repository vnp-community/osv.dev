# 04 — Development Roadmap: Hướng Phát Triển

> **Date:** 2026-06-03  
> **Status:** 🔄 In Execution — Xem Implementation Status ở dưới  
> **Horizon:** 12 tháng

---

## 1. Tầm Nhìn Phát Triển

### Từ: Python monolith + Go microservices song song
### Đến: Pure Go microservices platform + Python legacy isolate

```
Trạng thái HIỆN TẠI:
  ┌─────────────────────────────────────────────────────┐
  │  Python Stack (Legacy)    Go Stack (New)             │
  │  ─────────────────────    ─────────────────         │
  │  osv/ core library        services/pkg/             │
  │  gcp/workers/importer     services/ingestion/       │
  │  gcp/workers/worker       services/impact-analysis/ │
  │  gcp/api/ (gRPC)          services/api-gateway/     │
  │  gcp/website/ (Flask)     services/web-bff/         │
  │  gcp/indexer/             services/version-index/   │
  │  vulnfeeds/ (CLI)         (no equivalent yet)       │
  └─────────────────────────────────────────────────────┘

Trạng thái MỤC TIÊU (12 tháng):
  ┌─────────────────────────────────────────────────────┐
  │  Go Platform                                        │
  │  ─────────────────────────────────────────────────  │
  │  services/ (12 microservices)                       │
  │  apps/ (cli + server)                               │
  │  tools/cmd/ (admin tools)                           │
  │                                                     │
  │  Python Isolate (OSS-Fuzz only):                    │
  │  osv/ossfuzz/ — bisection, regress/fix tracking     │
  └─────────────────────────────────────────────────────┘
```

---

## 2. Roadmap Theo Quý

### Q3 2026 (Tháng 1-3): Foundation & Cleanup

**Mục tiêu:** Ổn định nền tảng Go, xóa code thừa

#### 2.1 Hoàn thiện `services/pkg/` Shared Library

| Task | Chi tiết | Priority |
|------|---------|----------|
| Ecosystem completeness audit | So sánh `osv/ecosystems/` vs `services/pkg/ecosystem/impl/` — identify gaps | P0 |
| Ecosystem test parity | Đảm bảo Go và Python cho cùng kết quả với cùng test cases | P0 |
| Add `services/pkg/converter/` | Port `vulnfeeds/conversion/versions.go` vào shared pkg | P1 |
| Add `services/pkg/clients/kev/` | CISA KEV API client | P1 |
| Add `services/pkg/classification/` | Severity scoring, auto-tagging logic | P1 |
| Merge `bindings/go/` → `services/pkg/clients/` | Xem 02-reorganization.md | P1 |

#### 2.2 Source Sync Enhancement

```go
// services/source-sync — cần develop thêm:

// 1. Webhook receiver — GitHub, GitLab webhooks
type WebhookHandler interface {
    HandleGitHubPush(ctx context.Context, event GitHubPushEvent) error
    HandleGitLabPush(ctx context.Context, event GitLabPushEvent) error
}

// 2. Credential manager — quản lý SSH keys, tokens per source
type CredentialManager interface {
    GetSSHKey(ctx context.Context, source string) ([]byte, error)
    GetToken(ctx context.Context, source string) (string, error)
    RotateCredential(ctx context.Context, source string) error
}

// 3. Admin API cho source management
type SourceAdmin interface {
    PauseSource(ctx context.Context, name string) error
    ResumeSource(ctx context.Context, name string) error
    TriggerSync(ctx context.Context, name string) error
    GetSourceStatus(ctx context.Context, name string) (*SourceStatus, error)
}
```

#### 2.3 Cleanup Actions (P0)

```bash
# Xóa các file không còn dùng
git rm tools/source-sync/source_sync.py
git rm -r tools/migrate/
git rm -r tools/datastore-remover/

# Tổ chức lại tools/
mkdir -p tools/cmd tools/scripts/ci tools/deprecated
git mv tools/aliaslookup tools/cmd/
git mv tools/smoke-test tools/cmd/
git mv tools/compare-responses tools/cmd/
git mv tools/apitester tools/cmd/
git mv tools/review_dependency_prs.py tools/scripts/ci/
git mv tools/datafix tools/deprecated/
```

---

### Q4 2026 (Tháng 4-6): Converter Service & AI Enhancement

#### 2.4 `services/converter/` — New Microservice

**Mục tiêu:** Chuyển `vulnfeeds/` từ batch CLI tool thành event-driven microservice.

```
Architecture:
  NVD API ─────────────────► converter-service ──► NATS "raw.cve.nvd"
  Alpine secdb ─────────────►                  ──► NATS "raw.cve.alpine"
  Debian tracker ───────────►                  ──► NATS "raw.cve.debian"
  Manual upload ─────────────►                 ──► NATS "raw.cve.custom"
         ▲                                              │
         │                                              ▼
  converter-service polls                      ingestion-service
  external sources                             (normalizes → OSV Schema)
```

**gRPC Interface:**
```protobuf
service ConverterService {
  // Convert a single CVE5 record to OSV format
  rpc ConvertCVE5(ConvertCVE5Request) returns (ConvertResponse);
  
  // Batch convert: stream input, stream output
  rpc BatchConvert(stream ConvertRequest) returns (stream ConvertResponse);
  
  // Get conversion stats for a source
  rpc GetStats(GetStatsRequest) returns (ConversionStats);
  
  // Trigger a full re-conversion for a source
  rpc TriggerFullConversion(TriggerRequest) returns (TriggerResponse);
}
```

**Core Logic từ vulnfeeds/ (port sang Go service):**
- `vulnfeeds/conversion/cve5/` → CVE5 format converter
- `vulnfeeds/conversion/nvd/` → NVD JSON v2 converter
- `vulnfeeds/conversion/versions.go` → Version detection từ CPE
- `vulnfeeds/git/` → Git utilities (dùng chung với impact-analysis)
- `vulnfeeds/triage/` → CVE triage logic

#### 2.5 `services/ai-enrichment/` — Enhancement

**Tính năng cần thêm:**
```
Hiện tại:
  ✅ LLM summarization (Ollama/Vertex)
  ✅ Vector embedding
  ✅ Basic tagging

Cần phát triển thêm:
  [ ] CISA KEV Integration
      - Tự động tag "kev" khi CVE xuất hiện trong KEV catalog
      - Poll KEV API hàng ngày
      - Alert khi KEV catalog có CVE mới từ nguồn đang track

  [ ] EPSS Score Integration
      - Fetch EPSS từ api.first.org/epss
      - Store EPSS score + percentile trong enriched record
      - Update hàng ngày (EPSS thay đổi theo thời gian)

  [ ] CWE Classification
      - Map CVSS vector → CWE IDs
      - Hierarchy-aware classification (CWE-89 → SQL Injection → Injection)
      - Store CWE tree trong search index

  [ ] Attack Pattern (CAPEC)
      - Map CWE → CAPEC attack patterns
      - Useful cho threat intelligence

  [ ] Exploitability Assessment
      - PoC availability check (GitHub, PacketStorm, ExploitDB)
      - Exploit maturity score
```

**Cấu trúc enrichment pipeline mới:**
```go
type EnrichmentPipeline struct {
    stages []EnrichmentStage
}

type EnrichmentStage interface {
    Name() string
    Enrich(ctx context.Context, vuln *osvschema.Vulnerability) error
    Priority() int  // 0 = highest priority (run first)
}

// Implementations:
// 1. CVSSEnrichment      — Parse/normalize CVSS vectors
// 2. KEVEnrichment       — Check against KEV catalog
// 3. EPSSEnrichment      — Add EPSS score
// 4. CWEEnrichment       — CWE classification
// 5. LLMSummarization    — AI-generated summary
// 6. VectorEmbedding     — Semantic search embedding
// 7. AutoTagging         — Auto-generated tags
```

---

### Q1 2027 (Tháng 7-9): Admin Service & Operations

#### 2.6 `services/admin/` — New Service

**Mục tiêu:** Admin API + dashboard backend cho operations team.

```
services/admin/
├── cmd/main.go
├── internal/
│   ├── application/
│   │   ├── source_management.go    # Manage sources
│   │   ├── data_quality.go         # Data quality monitoring
│   │   ├── import_findings.go      # Import error management
│   │   └── user_management.go      # Admin user management
│   └── infra/
│       └── handlers/               # REST API handlers
└── interface/
    └── openapi/                    # OpenAPI spec
```

**Admin API Endpoints:**
```
Sources:
  GET  /admin/v1/sources              — List all sources + health
  GET  /admin/v1/sources/{name}       — Source detail + recent sync log
  POST /admin/v1/sources/{name}/sync  — Trigger manual sync
  POST /admin/v1/sources/{name}/pause — Pause source
  POST /admin/v1/sources/{name}/resume— Resume source
  PUT  /admin/v1/sources              — Update source.yaml config

Data Quality:
  GET  /admin/v1/import-findings      — List recent import errors
  GET  /admin/v1/import-findings/{source} — Errors per source
  POST /admin/v1/import-findings/{id}/resolve — Mark resolved

Vulnerabilities:
  POST /admin/v1/vulns/{id}/withdraw  — Withdraw a vulnerability
  POST /admin/v1/vulns/{id}/reprocess — Reprocess a vulnerability
  GET  /admin/v1/vulns/stats          — Statistics dashboard data

System:
  GET  /admin/v1/health               — Overall system health
  GET  /admin/v1/metrics/summary      — Key metrics summary
```

---

### Q2 2027 (Tháng 10-12): Scale & Advanced Features

#### 2.7 Version Index Enhancement

```go
// services/version-index — cần develop thêm:

// 1. Incremental indexing (thay vì full re-index)
type IncrementalIndexer interface {
    IndexCommitRange(ctx context.Context, repo string, from, to string) error
    IndexTag(ctx context.Context, repo string, tag string) error
}

// 2. Index freshness tracking
type IndexFreshnessMonitor interface {
    GetStaleness(ctx context.Context, repo string) (time.Duration, error)
    ListStaleRepos(ctx context.Context, threshold time.Duration) ([]string, error)
}
```

#### 2.8 Search Enhancement

```
services/search — cần phát triển thêm:

1. Semantic Search (Vector Search)
   - Nhận embedding từ ai-enrichment
   - Store trong OpenSearch k-NN index
   - "CVE giống với CVE-2023-44487" (HTTP/2 rapid reset)

2. Faceted Search
   - Filter theo ecosystem, severity, year, has-fix, KEV, EPSS
   - Aggregations cho dashboard

3. Saved Searches / Alerts
   - User lưu query → webhook khi có CVE mới match
   - Integration với notification service
```

#### 2.9 Notification Enhancement

```go
// services/notification — cần phát triển thêm:

type NotificationChannel interface {
    Slack(ctx context.Context, msg SlackMessage) error
    Email(ctx context.Context, msg EmailMessage) error
    Webhook(ctx context.Context, url string, payload any) error
    PagerDuty(ctx context.Context, incident PagerDutyIncident) error
}

// Subscription system
type Subscription struct {
    ID        string
    Query     string           // saved search query
    Channels  []Channel        // where to notify
    MinSeverity Severity       // minimum severity to alert
    Ecosystems []string        // filter by ecosystem
    KEVOnly   bool             // only alert on KEV entries
}
```

---

## 3. Hướng Phát Triển Theo Module

### `apps/cli/` — Go CLI Tool
```
Hiện tại: Minimal implementation
Phát triển:
  [ ] cvectl sources list       — List tất cả nguồn CVE
  [ ] cvectl sources sync       — Trigger sync
  [ ] cvectl vuln get <id>      — Lấy thông tin CVE
  [ ] cvectl vuln search <q>    — Tìm kiếm
  [ ] cvectl admin import-findings — Xem lỗi import
  [ ] cvectl admin reprocess <id>  — Reprocess CVE
  [ ] cvectl version            — Show version info
```

### `apps/osv/` — Go OSV Server
```
Hiện tại: API server với /v1 endpoints
Phát triển:
  [ ] Rate limiting per API key (move từ api-gateway)
  [ ] API key management
  [ ] Quota tracking
  [ ] Webhook registration
  [ ] API versioning (v2 endpoints)
```

### `services/source-sync/` — Source Orchestrator
```
Hiện tại: Basic scheduler
Phát triển:
  [ ] Webhook receiver (GitHub/GitLab push events)
  [ ] Credential manager (SSH keys, tokens per source)
  [ ] Admin API (pause, resume, trigger per source)
  [ ] Dependency graph (nguồn phụ thuộc nhau: cve-osv → debian-cve)
  [ ] Smart scheduling (ưu tiên nguồn có thay đổi gần đây)
```

### `services/ingestion/` — Vulnerability Processor
```
Hiện tại: Basic OSV format validator + normalizer
Phát triển:
  [ ] Strict schema validation with per-source rules
  [ ] Auto-fix common schema issues (timestamps, missing fields)
  [ ] Deduplication pipeline
  [ ] Alias detection trong ingestion flow
  [ ] Import finding categorization (error types, trends)
```

---

## 4. Non-Functional Requirements

### Performance Targets
| Metric | Current (est.) | Target |
|--------|---------------|--------|
| CVE ingest latency (GHSA) | ~10 min | < 2 min |
| CVE ingest latency (REST) | ~2 hr | < 15 min |
| API query P95 | 5s | < 500ms |
| Search query P95 | 2s | < 200ms |
| Full database refresh | 4+ hr | < 1 hr |

### Reliability Targets
| Metric | Target |
|--------|--------|
| API availability | 99.9% |
| Data freshness SLA | < 30 min for Tier 1 sources |
| Import error rate | < 0.1% per source per day |

---

## 5. Tech Stack Changes

| Component | Current | Proposed |
|-----------|---------|---------|
| Python Core | google-cloud-ndb | ❌ Remove |
| Message Broker | Cloud Pub/Sub | ✅ Keep (prod) / NATS (dev) |
| Database | Cloud Firestore | ✅ Keep |
| Search | N/A | ✅ OpenSearch |
| Cache | Redis | ✅ Redis |
| AI/ML | Vertex AI + Ollama | ✅ Keep + EPSS API |
| Observability | Cloud Logging | ✅ OpenTelemetry |
| API Protocol | gRPC | ✅ gRPC + REST |

---

## 6. Implementation Status (2026-06-03)

> Cập nhật theo tiến độ thực hiện thực tế.

### 6.1 Q3 2026: Foundation & Cleanup ✅ DONE

| Mục | Proposal | Thực tế | Status |
|-----|----------|---------|--------|
| Ecosystem completeness audit | So sánh Python/Go | Parity test suite (6 cases) | ✅ |
| `pkg/clients/kev/` | CISA KEV client | 8/8 tests pass | ✅ |
| `pkg/clients/epss/` | EPSS scoring client | 4/4 tests pass | ✅ |
| `pkg/classification/` | Severity + auto-tagging | 12/12 tests pass | ✅ |
| `pkg/cwe/` | CWE database (mới) | 60+ CWEs, 12/12 tests pass | ✅ |
| `pkg/models/` | Bug + AliasGroup | 6/6 tests pass | ✅ |
| Merge `bindings/go/` → `pkg/clients/` | Merge + deprecate | Done | ✅ |
| Webhook receiver (GitHub + GitLab) | Source-sync webhook | `infra/webhook/` done | ✅ |
| Source NATS trigger | Publish sync events | `application/command/sync_source/` | ✅ |
| Cleanup `tools/deprecated/` | Archive legacy tools | `source_sync.py`, `migrate/` archived | ✅ |

### 6.2 Q4 2026: Converter & AI Enhancement ✅ MOSTLY DONE

| Mục | Proposal | Thực tế | Status |
|-----|----------|---------|--------|
| CVE5 converter | Port từ vulnfeeds/ | `converter/domain/cve5/` + ADP merge (8 tests) | ✅ |
| NVD JSON v2 converter | Port từ vulnfeeds/ | `converter/domain/nvd/converter.go` | ✅ |
| ADP Container merging | Merge CISA/NVD ADP | `cve5/adp.go` + dedup logic | ✅ |
| CPE → version detection | Port `versions.go` | 📋 TODO (TASK-04-03, 5d) | 📋 |
| gRPC interface | ConverterService proto | 📋 TODO (TASK-04-04, 2d) | 📋 |
| KEV Integration | Tag CVE là KEV | `threatintel/kev_stage.go` done | ✅ |
| EPSS Score | Fetch + store score | `threatintel/epss_stage.go` done | ✅ |
| CWE Classification | Map CVE → CWE | `threatintel/cwe_stage.go` done | ✅ |
| NATS Event Publisher | Publish raw.cve.* | `infra/publisher/nats_publisher.go` | ✅ |

### 6.3 Q1 2027: Admin Service & Operations ✅ DONE

| Mục | Proposal | Thực tế | Status |
|-----|----------|---------|--------|
| Source management handlers | List/Get/Sync/Pause/Resume | 5 handlers done | ✅ |
| Import findings handlers | List/Resolve errors | 2 handlers done | ✅ |
| Vuln admin operations | Withdraw/Reprocess/Stats | 3 handlers done | ✅ |
| System health endpoint | Check all components | `SystemHealth()` 6 components | ✅ |
| API key management | Create/List/Revoke | 3 handlers done | ✅ |
| Admin REST routes | 12 endpoints | All wired in `cmd/main.go` | ✅ |
| API v2 endpoints | Extended API | enrichment, related, timeline, batch | ✅ |
| `cvectl` CLI | Cobra/viper CLI | search, get, list, enrich commands | ✅ |
| OpenSearch index mapping | KEV+EPSS+tags+vector | `search/entity/entity.go` + mapping | ✅ |

### 6.4 Q2 2027: Scale & Advanced Features 🔄 IN PROGRESS

| Mục | Proposal | Thực tế | Status |
|-----|----------|---------|--------|
| Semantic vector search | k-NN OpenSearch | Index mapping done; query impl TODO | 🔄 |
| Bug struct Go port | Port `osv/models.py` | `pkg/models/vulnerability.go` | ✅ |
| AliasGroup port | Port entity | `pkg/models/alias_group.go` | ✅ |
| Ecosystem parity tests | Cross-lang tests | Parity test suite ready | ✅ |
| `osv/impact.py` port | Impact analysis Go | bisector partial | 🔄 |
| `osv/sources.py` port | Ingestion Go | Planned | 📋 |
| OSS-Fuzz isolation | Isolate Python | `osv/ossfuzz/README.md` done | ✅ |

### 6.5 Build Health (2026-06-03)

```
✅ ALL 8 SERVICES BUILD PASS (0 errors):
   admin | ai-enrichment | api-gateway | converter
   cvectl | impact-analysis | source-sync | pkg

✅ 50+ TESTS PASS:
   kev(8) | epss(4) | classification(12) | cwe(12)
   models(6) | converter/cve5(8)

Next Sprint Priorities:
   TASK-04-03: CPE → version detection (5d, high complexity)
   TASK-04-04: gRPC converter interface (2d)
   TASK-07-02: Semantic/vector search (3d)
   TASK-10-01: Port osv/impact.py (7d)
```
