# Software Requirements Specification (SRS) — OSV Platform

**Version:** 3.0  
**Ngày cập nhật:** 2026-06-16  
**Trạng thái:** v2.2 Active — v3.0 Target  

---

## 1. Introduction

### 1.1 Mục đích

Tài liệu SRS này mô tả đầy đủ kiến trúc hệ thống, functional requirements, và non-functional requirements của **OSV Platform**.

- **v2.2 (Implemented)**: 30 CRs từ cve-search, DefectDojo, GlobalCVE — đã hoàn thành.
- **v3.0 (Planned)**: 7 CRs từ OpenVulnScan — active scanning, AI triage, JWT RS256/MFA/OAuth2.

### 1.2 Phạm vi

OSV Platform là hệ thống **Go Microservices** với **Clean Architecture** và **Event-Driven** design, bao gồm:
- CVE data aggregation và enrichment (v2.0 - v2.2)
- Finding lifecycle management với DefectDojo-style hierarchy (v2.1)
- Multi-channel notifications, JIRA integration, Audit trail (v2.1)
- OpenSearch FTS, pgvector semantic search, observability (v2.2)
- **[Planned v3.0]** Active vulnerability scanning (Nmap, OWASP ZAP, Agent)
- **[Planned v3.0]** AI-powered triage và semantic search
- **[Planned v3.0]** Enterprise auth (JWT RS256, MFA, OAuth2)
- **[Planned v3.1]** UI-API contracts — ~70 REST endpoints + 2 SSE streams cho React SPA frontend

### 1.3 Tài liệu tham chiếu

| Tài liệu | Mô tả |
|---------|-------|
| `docs/PRD.md` | Product Requirements Document |
| `docs/URD.md` | User Requirements Document |
| `specs/crs/v1/cve-search/` | 9 CRs từ cve-search ✅ |
| `specs/crs/v1/DefectDojo/` | 11 CRs từ DefectDojo ✅ |
| `specs/crs/v1/globalcve/` | 10 CRs từ GlobalCVE ✅ |
| `specs/crs/v1/OpenVulnScan/` | 7 CRs từ OpenVulnScan 🔵 Planned (v3.0) |
| `specs/crs/v0/ui-api/` | 10 CRs UI-API contracts 🔵 Planned (v3.1) |

---

## 2. System Architecture

### 2.1 Architecture Overview (v2.2 Implemented)

```
┌──────────────────────────────────────────────────────────────────────────────────┐
│                              CLIENT LAYER                                         │
│   Web UI  |  CLI (cvectl)  |  CI/CD Pipeline  |  Third-party APIs               │
└──────────────────────────────────────────────────────────────────────────────────┘
                                     │ HTTPS
                                     ▼
┌──────────────────────────────────────────────────────────────────────────────────┐
│                    UNIFIED GATEWAY  apps/osv  :8080                               │
│  Dual Auth: JWT + API Key  |  Redis Rate Limiting  |  Reverse Proxy              │
│  Route Dispatch (100+ routes)  |  UserHeader Injection  |  OpenAPI aggregation   │
└──────┬──────┬──────┬──────┬──────┬──────┬──────┬──────┬──────┬────────────────┘
       │      │      │      │      │      │      │      │      │
       ▼      ▼      ▼      ▼      ▼      ▼      ▼      ▼      ▼
    ┌──────┐┌──────┐┌──────┐┌────┐┌────┐┌────┐┌────┐┌────┐┌────────────┐
    │ident-││data- ││search││rank││find││scan││sla-││notif││jira/audit  │
    │:8081 ││:8082 ││-svc  ││:   ││:   ││:   ││:   ││:   ││-service    │
    └──────┘└──────┘└──────┘└────┘└────┘└────┘└────┘└────┘└────────────┘

═══════════════════════ INFRASTRUCTURE LAYER ═══════════════════════════════════
  PostgreSQL 16 (pgvector)    Redis 7          NATS JetStream
  OpenSearch 2 (BM25)         MinIO/S3         OpenTelemetry → Jaeger
  Prometheus + Grafana
```

### 2.2 Microservices Map (v2.2 Implemented)

| Service | HTTP Port | Nguồn CR | Mô tả |
|---------|:---------:|---------|-------|
| `apps/osv` (gateway) | 8080 | CR-DD-011, CR-GCV-008 | HTTP Gateway, dual auth JWT+APIKey, rate-limit, 100+ routes |
| `identity-service` | 8081 | CR-007 (cve-search) | LDAP, API key scopes, local auth |
| `data-service` | 8082 | CR-001~009 (cve-search), CR-GCV-001,002,003,005,007 | CVE query, taxonomy, feeds, stats, fetchers, EPSS, KEV |
| `search-service` | — | CR-GCV-004, CR-002,004 (cve-search) | OpenSearch FTS, pgvector semantic, browse, CPE export |
| `ranking-service` | 8084 | CR-004 (cve-search) | CPE ranking, vendor popularity |
| `finding-service` | 8085 | CR-DD-001,004,005,009 | Product/Engagement/Test hierarchy, findings, state machine, reports, grading |
| `scan-service` | — | CR-DD-002,003 | 21+ parsers, 12-step import pipeline, 3-algorithm dedup |
| `sla-service` | 8086 | CR-DD-006 | SLA config, breach detection, daily cron |
| `notification-service` | 8087 | CR-GCV-006, CR-DD-007 | Email/Slack/Teams/Webhook/In-app, 14 event types |
| `jira-service` | 8088 | CR-DD-008 | AES-256-GCM creds, HMAC webhook, bidirectional sync |
| `audit-service` | 8090 | CR-DD-010 | Append-only RLS, HMAC-SHA256, 40+ NATS subs |
| `ai-service` | — | CR-GCV-004 (embeddings) | CVE embeddings, LLM severity classification |

### 2.3 [Planned v3.0] Additional Services (OpenVulnScan)

| Service | HTTP | gRPC | Nguồn CR | Mô tả |
|---------|:----:|:----:|---------|-------|
| `auth-service` | 8051 | 50051 | CR-OVS-003 | JWT RS256, MFA TOTP, OAuth2 Google/GitHub, Argon2id, API keys `ovs_` |
| `scan-service-ovs` | 8058 | 50058 | CR-OVS-001 | Nmap/ZAP/Agent active scanning, SSE progress, state machine, scheduler |
| `finding-service-ovs` | 8060 | 50060 | CR-OVS-002 | 6-state lifecycle, SHA-256 dedup, SLA, audit trail |
| `product-service` | 8061 | 50061 | CR-OVS-004 | ProductType/Product/Engagement/Test hierarchy, CI/CD orchestrator |
| `ai-service-ovs` | 8052 | 50052 | CR-OVS-005 | Embedding, LLM chain (Ollama→OpenAI→Azure), EPSS, triage |
| `report-service` | 8065 | 50065 | CR-OVS-006 | PDF/HTML/CSV/Excel, MinIO, CI/CD exit code |
| `asset-service` | 8068 | 50068 | CR-OVS-007 | Asset registry, tagging, risk scoring, cron scheduler |

### 2.4 Infrastructure Stack

| Component | Version | Mô tả |
|-----------|---------|-------|
| Language | Go 1.22+ | All microservices |
| Database | PostgreSQL 16 + pgvector | Primary storage, schema-per-service |
| Cache | Redis 7 | JWT blacklist, EPSS cache, rate-limit |
| Search | OpenSearch 2 | Full-text BM25 search |
| Vector | pgvector (PostgreSQL extension) | 1536-dim semantic search |
| Message Queue | NATS JetStream | Event-driven communication |
| Object Storage | MinIO / AWS S3 | Report artifacts |
| API Design | REST + gRPC (protobuf) | External + internal |
| Observability | OpenTelemetry → Jaeger | Distributed tracing |
| Metrics | Prometheus + Grafana | All services |
| Logging | zerolog (structured JSON) | All services |
| Container | Docker + Docker Compose | Development |
| Orchestration | Kubernetes (Helm charts) | Production |

---

## 3. Functional Requirements

### 3.1 CVE Data Aggregation (FR-01 series) ✅ Implemented

#### FR-01-01: Multi-Source CVE Ingestion
*Nguồn: CR-GCV-001*

Hệ thống PHẢI thu thập CVE data từ các nguồn sau theo lịch định kỳ:

| Source | Schedule | Protocol |
|--------|----------|---------| 
| NVD CVE API | Every 2 hours | JSON REST |
| JVN RSS | Every 1 hour | RSS XML |
| CIRCL CVE Search | Every 6 hours | REST |
| ExploitDB | Every 24 hours | CSV stream |
| CVE.org deltaLog | Every 12 hours | GitHub API |
| CISA KEV | Every 6 hours | JSON |
| CNNVD | Every 12 hours | HTML scrape |
| Android Bulletins | Every 24 hours | HTML |
| Vendor advisories (Cisco, Red Hat, Ubuntu, Oracle, VMware) | Every 24 hours | Vendor APIs |

**Fetcher Registry pattern**: Mỗi fetcher tự đăng ký với registry. Fetcher PHẢI implement interface:
```go
type CVEFetcher interface {
    FetchSince(ctx context.Context, since time.Time) (<-chan CVERecord, error)
    Source() string
}
```

#### FR-01-02: EPSS Daily Sync
*Nguồn: CR-GCV-002*

- Hệ thống PHẢI sync EPSS scores từ FIRST.org mỗi ngày lúc 3:00 AM UTC.
- Format: CSV.GZ, download trực tiếp.
- Sau sync: `cves.epss_score` và `cves.epss_percentile` được cập nhật.
- CVE search API PHẢI hỗ trợ filter `?min_epss=0.7` và sort `?sort=epss_desc`.

#### FR-01-03: MITRE CAPEC/CWE Sync
*Nguồn: CR-GCV-003, CR-003*

- CAPEC: sync XML weekly (Sunday 5:00 AM) → `capec_patterns` table.
- CWE: sync XML.ZIP weekly (Sunday 5:00 AM) → `cwe_weaknesses` table.
- CVE-CWE link: extracted từ NVD data → `cve_cwes` junction table.
- CWE-CAPEC link: parsed từ CAPEC XML → `capec_cwes` table.
- API: `GET /api/v2/cwe/{id}`, `GET /api/v2/capec/{id}`.

#### FR-01-04: NVD CPE Dictionary
*Nguồn: CR-GCV-005*

- Sync NVD CPE dictionary weekly (Sunday 4:00 AM).
- Store: ~1M CPE entries in `cpe_dictionary` table.
- API: `GET /api/v2/vendors` (list), `GET /api/v2/vendors/{vendor}/products`.
- Filter: `?vendor=apache&product=log4j` → CVE search.
- Cache: Redis, TTL 24h.

#### FR-01-05: KEV Advanced
*Nguồn: CR-GCV-007*

- Sync CISA KEV v3 format (includes `knownRansomware` flag).
- Fields: `known_ransomware_campaign_use`, `required_action`, `short_description`.
- Diff detection: compare current → previous → publish `kev.new` NATS event.
- Advanced stats: `GET /api/v2/kev/stats` → top vendors, by-month, avg_days_to_patch.
- `GET /api/v2/kev/ransomware` → filter KEV by ransomware flag.

---

### 3.2 CVE Search & Discovery (FR-02 series) ✅ Implemented

#### FR-02-01: Full-Text Search (OpenSearch)
*Nguồn: CR-GCV-004*

- Index: `cves` index trong OpenSearch (BM25).
- Fields: `cve_id`, `description`, `vendors`, `products`, `cwe_ids`.
- Endpoint: `POST /api/v2/cves/search` với query object.
- Fallback: nếu OpenSearch unavailable → PostgreSQL GIN index.

#### FR-02-02: Semantic Search (pgvector)
*Nguồn: CR-GCV-004*

- Endpoint: `POST /api/v2/cves/search/semantic` với `query` string.
- AI pipeline: query → embedding (Ollama/OpenAI) → pgvector cosine similarity.
- Index: IVFFlat, dimensions = 1536.
- Response: top-K results với similarity score.

#### FR-02-03: CVE Aggregations
*Nguồn: CR-GCV-004*

- `GET /api/v2/cves/aggregations`: severity distribution, top vendors, by year, EPSS distribution.
- Backed by OpenSearch aggregations (terms, histogram).

#### FR-02-04: Browse Vendor/Product
*Nguồn: CR-002 (cve-search)*

- `GET /api/v2/browse`: paginated list of all vendors (sorted by CVE count).
- `GET /api/v2/browse/{vendor}`: products for vendor.
- `GET /api/v2/browse/{vendor}/{product}`: CVE list for product.
- Backend: PostgreSQL + Redis cache 1h.

#### FR-02-05: CPE Search (Lax & Strict)
*Nguồn: CR-001 (cve-search)*

```
POST /api/v2/cves/search/cpe
{
  "cpe": "cpe:2.3:a:apache:log4j:2.14.1:*:*:*:*:*:*:*",
  "mode": "lax"   // "strict" hoặc "lax"
}
```

- **Strict**: match exact CPE string.
- **Lax**: match version range theo CPE spec, includes affected versions.
- Kết quả enriched với CAPEC patterns.

#### FR-02-06: CVE Taxonomy Lookup
*Nguồn: CR-003 (cve-search)*

- `GET /api/v2/cwe/{id}` → CWE detail + linked CAPECs + related CVEs.
- `GET /api/v2/capec/{id}` → CAPEC detail + Likelihood + Mitigations.

#### FR-02-07: CVE Feeds
*Nguồn: CR-008 (cve-search)*

- `GET /api/v2/cves/recent` → CVEs trong 24h gần nhất.
- `GET /api/v2/cves/last/{n}` → last N CVEs.
- `GET /api/v2/cves/feed/atom` → Atom XML feed.
- `GET /api/v2/cves/feed/rss` → RSS 2.0 XML feed.

#### FR-02-08: CVE Export & Attribution
*Nguồn: CR-GCV-010*

- `GET /api/v2/cves/export?format=json&since=2026-01-01` → bulk JSON download.
- `GET /api/v2/cves/export?format=csv` → CSV download.
- Response includes `data_source`, `source_url`, `last_modified` per CVE.

#### FR-02-09: DB Statistics
*Nguồn: CR-005 (cve-search)*

- `GET /api/v2/dbinfo` → total CVEs per source, last sync timestamps, lag.

#### FR-02-10: CPE Ranking
*Nguồn: CR-004 (cve-search)*

- `ranking-service` cung cấp CPE popularity ranking.
- Ranking dựa trên: CVE count, EPSS average, KEV presence.
- `GET /api/v2/cpe/ranking?vendor=apache&limit=20`.

---

### 3.3 [Planned v3.0] Active Vulnerability Scanning (FR-03 series)

> **Status**: Chưa implement. Planned cho OpenVulnScan CRs v2.

#### FR-03-01: Scan Lifecycle Management *(Planned)*
*Nguồn: CR-OVS-001*

Scan PHẢI có state machine: `pending → queued → running → completed | failed | cancelled`

#### FR-03-02: Nmap Full Scan *(Planned)*
*Nguồn: CR-OVS-001*

- Command: `nmap -sV -O --script=vulners -oX - --open -T4 {targets}`
- CVE IDs extracted via regex từ vulners script output.

#### FR-03-03: OWASP ZAP Web Scan *(Planned)*
*Nguồn: CR-OVS-001*

- Spider target URL → Active Scan → Get alerts.
- Risk levels: High / Medium / Low / Informational.

#### FR-03-04: Agent Report Ingestion *(Planned)*
*Nguồn: CR-OVS-001*

- Endpoint: `POST /api/v1/agents/report` (API key auth, `agent:report` permission).
- Payload: `{agent_id, target, packages[], findings[]}` (SBOM-style).

#### FR-03-05: Scan Scheduling *(Planned)*
*Nguồn: CR-OVS-007*

- `ScheduledScan` entity với `cron_expr`, `frequency` (daily/weekly/custom).
- Scheduler checks every 1 minute.

#### FR-03-06: SSE Progress Stream *(Planned)*
*Nguồn: CR-OVS-001*

- `GET /api/v1/scans/{id}/stream` → `Content-Type: text/event-stream`.

#### FR-03-07: Asset Management *(Planned)*
*Nguồn: CR-OVS-007*

- Sau mỗi scan completed → auto upsert asset (key: IP address).

---

### 3.4 Finding Management (FR-04 series) ✅ Implemented

#### FR-04-01: Finding State Machine
*Nguồn: CR-DD-004*

6 states với priority ordering:

```
Duplicate > FalsePositive > OutOfScope > RiskAccepted > Mitigated > Active
```

Transitions:
- Active → Mitigated (close), FalsePositive, RiskAccepted, OutOfScope
- Any (except Duplicate) → Active (reopen)
- Duplicate → No manual transitions

#### FR-04-02: SLA Enforcement
*Nguồn: CR-DD-006*

Default SLA deadlines:

| Severity | Days |
|---------|------|
| Critical | 7 |
| High | 30 |
| Medium | 90 |
| Low | 180 |
| Info | — (no SLA) |

- `sla_expiration_date = date + sla_days`.
- Per-product SLA override có thể configure.
- Cron job (daily): check breaches → publish `finding.sla.breached`.

#### FR-04-03: Hash-based Deduplication
*Nguồn: CR-DD-003*

```go
HashCode = SHA-256(title + component_name + component_version + cve_id)
```

- Khi create finding: check existing hash trong cùng product.
- Nếu duplicate: `duplicate=true`, `duplicate_finding_id` = original ID, `active=false`.

#### FR-04-04: Audit Trail
*Nguồn: CR-DD-010*

- Mọi state change PHẢI tạo `audit_event` record: `{action, before, after, user_id, timestamp}`.
- Audit log PHẢI immutable (append-only, Row-Level Security).
- HMAC-SHA256 signature per event.
- `GET /api/v2/audit-log` → ordered audit trail.

#### FR-04-05: Product/Engagement/Test Hierarchy
*Nguồn: CR-DD-001*

```
ProductType → Product → Engagement → Test → Finding
```

- Finding PHẢI có `test_id`, `engagement_id`, `product_id` foreign keys.
- Product members: role-based access control per product.

#### FR-04-06: Scan Import Pipeline
*Nguồn: CR-DD-002*

- `scan-service` hỗ trợ 21+ tool parsers (Nmap XML, ZAP JSON, Bandit, Trivy, Snyk...).
- Parser Factory pattern: `ParserFactory.GetParser(toolName)`.
- Import endpoint: `POST /api/v2/import-scan` với file upload.
- 12-step import pipeline: validate → parse → normalize → dedup → create findings.

#### FR-04-07: Bulk Finding Operations
*Nguồn: CR-DD-004*

- Bulk close/reopen/tag: `POST /api/v2/findings/bulk`

#### FR-04-08: Risk Acceptance
*Nguồn: CR-DD-005*

- Risk Acceptance entity với: `{product_id, findings[], expiration_date, reason, retest_date}`.
- Hết expiry → NATS event → auto-reopen linked findings.
- `GET /api/v2/risk-acceptances` → list with expiry status.

#### FR-04-09: Product Grading
*Nguồn: CR-DD-009*

| Grade | Condition |
|-------|-----------|
| A | 0 Critical, 0 High |
| B | 0 Critical, ≤ 5 High |
| C | 0 Critical, > 5 High |
| D | 1-2 Critical |
| F | 3+ Critical or > 20 findings |

---

### 3.5 Authentication & Authorization (FR-05 series)

#### FR-05-01: API Keys ✅ Implemented
*Nguồn: CR-007 (cve-search)*

- Format: prefix + base58 random bytes.
- Storage: SHA-256 hash only (plain key shown ONCE at creation).
- Scoped permissions per key.
- Gateway validate on every request.

#### FR-05-02: LDAP Authentication ✅ Implemented
*Nguồn: CR-007 (cve-search)*

- LDAP provider integration cho enterprise users.
- Auth chain: local → LDAP (configurable order).
- LDAP groups → OSV roles mapping.

#### FR-05-03: RBAC ✅ Implemented
*Nguồn: CR-007 (cve-search)*

| Role | Permissions |
|------|------------|
| admin | Tất cả operations |
| user | scan:read, finding:write, finding:read, report:download |
| readonly | scan:read, finding:read, report:download |

#### FR-05-04: [Planned v3.0] JWT RS256 Auth
*Nguồn: CR-OVS-003*

- Algorithm: RS256 (asymmetric).
- Access token TTL: 15 minutes.
- Refresh token rotation với reuse-attack detection.
- Account lockout: 5 consecutive failures.

#### FR-05-05: [Planned v3.0] MFA (TOTP)
*Nguồn: CR-OVS-003*

- RFC 6238, 30-second window, ±1 period tolerance.
- 8 backup codes generated at setup.

#### FR-05-06: [Planned v3.0] OAuth2
*Nguồn: CR-OVS-003*

- Providers: Google, GitHub.
- Flow: redirect → callback → upsert user → return JWT tokens.

---

### 3.6 [Planned v3.0] AI & Enrichment (FR-06 series)

> **Status**: Partial — CVE embeddings cho pgvector implemented trong search-service. LLM triage planned.

#### FR-06-01: CVE Embedding Generation *(Partial — search-service)*
- Input: `{cve_id, summary, details}`.
- Output: `[]float32` (1536 dims).
- Cache: Redis key `osv:embed:{cve_id}`, TTL 7 days.
- Stored in: pgvector `cves.embedding` column.

#### FR-06-02: [Planned] Finding Triage
*Nguồn: CR-OVS-005*

- Input: `{finding_id, title, description, cve, severity, context}`.
- LLM prompt → JSON `{remarks, confidence, justification, actions[]}`.
- Remarks: `"Confirmed"` | `"FalsePositive"` | `"NotAffected"`.

#### FR-06-03: [Planned] LLM Provider Chain
*Nguồn: CR-OVS-005*

- Ordered failover: `Ollama → OpenAI → Azure OpenAI`.

---

### 3.7 Notifications & Integrations (FR-07 series) ✅ Implemented

#### FR-07-01: Notification Channels
*Nguồn: CR-DD-007, CR-GCV-006*

| Channel | Config |
|---------|--------|
| Email | SMTP server + from address |
| Slack | Webhook URL |
| Microsoft Teams | Webhook URL |
| In-app | NATS → stored alerts |
| Webhook | URL + HMAC secret |

#### FR-07-02: Webhook Delivery
*Nguồn: CR-GCV-006*

- HMAC-SHA256 signature header: `X-OSV-Signature: sha256={hex}`.
- Retry: 3 attempts, exponential backoff.
- Timeout: 10s per attempt.
- SSRF protection: block private IP ranges.
- Deduplication: 1h window per `{alert_type, cve_id}`.

#### FR-07-03: Alert Triggers (14 Event Types)
*Nguồn: CR-GCV-006, CR-DD-007*

| Event | Channel |
|-------|---------|
| CVE added to CISA KEV | Webhook + Slack + Email |
| New CRITICAL CVE for subscribed vendor | Webhook + Email |
| EPSS score spike (> 0.9) | Webhook |
| Finding SLA breach | Email + Slack |
| Finding status changed | In-app |
| Risk acceptance expired | In-app + Email |
| JIRA issue sync | In-app |

#### FR-07-04: JIRA Integration
*Nguồn: CR-DD-008*

- AES-256-GCM encrypted credentials.
- Create JIRA ticket khi finding active và severity >= High.
- Webhook: JIRA `issue.resolved` → finding `IsMitigated=true`.
- HMAC-SHA256 webhook verification.

---

### 3.8 Reporting (FR-08 series) ✅ Implemented

#### FR-08-01: Report Formats
*Nguồn: CR-DD-009, CR-GCV-010*

| Format | Mô tả |
|--------|-------|
| `html` | Bootstrap 5.3, light/dark theme, Chart.js charts |
| `pdf` | Executive summary + CVE table + distribution chart |
| `csv` | All fields including EPSS |
| `excel` | DefectDojo XLSX import format |
| `json` | Structured JSON with source attribution |

#### FR-08-02: Product Grading ✅
*Nguồn: CR-DD-009*

Grading A–F based on finding severity distribution (see FR-04-09).

#### FR-08-03: Artifact Storage
*Nguồn: CR-DD-009*

- Reports stored in MinIO/S3.
- `GET /api/v2/reports/{id}/download` → download with `Content-Disposition`.

---

### 3.9 Observability (FR-09 series) ✅ Implemented

*Nguồn: CR-GCV-009*

#### FR-09-01: Structured Logging

All services PHẢI dùng `zerolog` với fields:
- `level`, `ts` (RFC3339), `service`, `trace_id`, `span_id`
- Request: `method`, `path`, `status`, `latency_ms`, `user_id`

#### FR-09-02: Prometheus Metrics

Core metrics mỗi service PHẢI export:
```
http_requests_total{method, path, status}
http_request_duration_seconds{method, path}   // Histogram
db_query_duration_seconds{query}              // Histogram
cache_hits_total{cache_name}
nats_messages_published_total{subject}
nats_messages_consumed_total{subject}
```

#### FR-09-03: Distributed Tracing

- OpenTelemetry SDK integrated vào tất cả services.
- Trace propagation qua NATS headers và HTTP headers.
- Export to Jaeger.

---

## 4. Non-Functional Requirements

### 4.1 Performance

| ID | Requirement | Target |
|----|------------|--------|
| NFR-01 | API P95 response (CVE lookup) | < 100ms |
| NFR-02 | API P95 response (search) | < 500ms |
| NFR-03 | Gateway auth middleware | < 5ms |
| NFR-05 | Report generation (1000 findings) | < 30 seconds |
| NFR-06 | Embedding cache hit | < 10ms |
| NFR-07 | NVD sync lag | < 2 hours |
| NFR-08 | NATS message delivery | < 100ms P99 |

### 4.2 Scalability

| ID | Requirement |
|----|------------|
| NFR-10 | API Gateway: stateless, horizontal scalable |
| NFR-11 | data-service fetchers: stateless, horizontal scale |
| NFR-12 | finding-service: read replicas cho high-query workload |
| NFR-13 | NATS JetStream: durable consumers, at-least-once delivery |

### 4.3 Availability

| ID | Requirement |
|----|------------|
| NFR-20 | Core API uptime: 99.9% |
| NFR-21 | Health endpoints: `GET /health` + `GET /ready` per service |
| NFR-22 | Graceful shutdown: drain in-flight requests (30s timeout) |
| NFR-23 | Circuit breaker trên all external API calls |

### 4.4 Security

| ID | Requirement |
|----|------------|
| NFR-30 | All HTTP: TLS 1.3 minimum |
| NFR-31 | API Keys: SHA-256 stored (không plaintext) |
| NFR-32 | Webhook: HMAC-SHA256 signature |
| NFR-33 | SSRF protection trên webhook delivery |
| NFR-34 | SQL: parameterized queries only (no string concat) |
| NFR-35 | Secrets: environment variables (không hardcode) |
| NFR-36 | Rate limiting: per-IP Redis token bucket |
| NFR-37 | Audit: HMAC-SHA256 per event, append-only |

### 4.5 Maintainability

| ID | Requirement |
|----|------------|
| NFR-40 | All Go code: `golangci-lint` pass |
| NFR-41 | Test coverage: ≥ 80% unit tests cho domain + usecase layers |
| NFR-42 | Integration tests: mỗi service có Docker Compose test env |
| NFR-43 | API versioning: `/api/v1/`, `/api/v2/` — backward compatible |
| NFR-44 | Database migrations: versioned với up/down scripts |
| NFR-45 | Clean Architecture: domain, usecase, adapter, infrastructure layers |

---

## 5. NATS JetStream Event Catalog (Implemented)

| Event Subject | Publisher | Subscribers | Payload |
|--------------|-----------|------------|---------|
| `ingestion.cve.synced` | data-service | search-service, ai-service | `{cve_id, action: created\|updated}` |
| `kev.new` | data-service | notification-service | `{cve_ids[], date_added}` |
| `finding.created` | finding-service | notification-service, sla-service | `{finding_id, cve, severity, product_id}` |
| `finding.batch_created` | scan-service | notification-service, sla-service, audit-service | `{scan_id, finding_ids[]}` |
| `finding.status.changed` | finding-service | notification-service, jira-service, audit-service, sla-service | `{finding_id, from_state, to_state, user_id}` |
| `finding.sla.breached` | sla-service | notification-service, audit-service | `{finding_id, severity, expires_at}` |
| `risk_acceptance.expired` | finding-service | notification-service, audit-service | `{acceptance_id, finding_ids[]}` |
| `jira.issue.created` | jira-service | notification-service, audit-service | `{finding_id, jira_key}` |
| `ai.cve.enriched` | ai-service | search-service | `{cve_id, embedding_dims}` |

---

## 6. Database Schema Overview

Schema-per-service pattern — mỗi service có database schema riêng trên PostgreSQL 16.

| Service | Schema | Key Tables |
|---------|-------|-----------| 
| `identity-service` | `osv_identity` | users, api_keys, ldap_configs, sessions |
| `data-service` | `osv_cves` | cves, sync_jobs, fetcher_runs, kev_entries, capec_patterns, cwe_weaknesses, cpe_dictionary |
| `search-service` | `osv_cves` (shared) | cve_embeddings, search_aggregations |
| `ranking-service` | `osv_ranking` | cpe_rankings, vendor_stats |
| `finding-service` | `osv_finding` | product_types, products, engagements, tests, findings, finding_groups, finding_notes, risk_acceptances, report_runs |
| `scan-service` | `osv_scan` | test_imports, import_findings |
| `sla-service` | `osv_sla` | sla_configs, sla_product_assignments, sla_breaches |
| `notification-service` | `osv_notif` | webhooks, notification_rules, alerts, notification_log |
| `jira-service` | `osv_jira` | jira_configs, jira_issues |
| `audit-service` | `osv_audit` | audit_events (append-only, partitioned by month) |

---

## 7. API Versioning

| Version | Services | Status |
|---------|----------|--------|
| `/api/v1/` | Legacy data-service CVE APIs | Active (backward compat) |
| `/api/v2/` | All new services: findings, products, SLA, notifications, JIRA, audit, search | Active (v2.2) |

Backward compatibility: v1 endpoints PHẢI stable. Breaking changes → new version.

---

## 8. UI-API Contracts (v3.1 — Planned)

> **Nguồn**: 10 CRs từ `specs/crs/v0/ui-api/` — xác định API-first contracts cho React SPA frontend.

### 8.1 CR Series Overview

| CR ID | Tên | ưu tiên | Services | Trạng thái |
|-------|-----|---------|----------|------------|
| CR-UI-001 | Authentication & User API | P0 | identity-service | ⚠️ Cần mở rộng |
| CR-UI-002 | Dashboard & KPI API (BFF aggregate) | P0 | gateway fan-out, finding-service, sla-service, data-service | ❌ Thiếu hoàn toàn |
| CR-UI-003 | CVE Intelligence API | P0 | data-service, search-service | ⚠️ Schema update |
| CR-UI-004 | Active Scanning API | P1 | scan-service-ovs (CR-OVS-001) | ❌ Planned v3.0 |
| CR-UI-005 | Finding Management API | P0 | finding-service, sla-service, audit-service | ⚠️ Schema update |
| CR-UI-006 | Asset Management API | P1 | asset-service (CR-OVS-007) | ❌ Planned v3.0 |
| CR-UI-007 | Product Security API | P0 | finding-service | ⚠️ Cần thêm endpoints |
| CR-UI-008 | AI Center API | P1 | ai-service-ovs (CR-OVS-005) | ❌ Planned v3.0 |
| CR-UI-009 | Reports & Notifications API | P0 | finding-service, notification-service | ⚠️ Cần thêm endpoints |
| CR-UI-010 | Administration & Integrations API | P0 | identity-service, jira-service, audit-service, gateway | ⚠️ Cần thêm endpoints |

### 8.2 Endpoint Gap Analysis

#### ⚠️ Implemented — cần schema/field update

| Endpoint | CR-UI | Thiếu fields |
|----------|-------|---------------|
| `POST /api/v2/cves/search` | CR-UI-003 | `is_kev`, `has_exploit`, `epss_percentile`, `sources[]`, `aggregations` |
| `GET /api/v2/cves/{id}` | CR-UI-003 | `affected_products[]`, `kev_detail`, `references[]` |
| `GET /api/v2/kev` | CR-UI-003 | `stats.unmitigated_in_platform` (join finding-service) |
| `GET /api/v1/findings` | CR-UI-005 | `is_kev`, `epss_score`, `jira_*`, `sla_days_left`, `by_severity`, `sla_stats` |
| `PATCH /api/v1/findings/{id}` | CR-UI-005 | State machine 409 `INVALID_TRANSITION` |
| `GET /api/v1/products` | CR-UI-007 | `grade`, `score`, `finding_summary` |

#### ❌ New endpoints needed (P0 — blocking UI)

| Endpoint | CR-UI | Service |
|----------|-------|---------|
| `POST /api/v1/auth/login` | CR-UI-001 | identity-service |
| `POST /api/v1/auth/refresh` | CR-UI-001 | identity-service |
| `GET /api/v1/auth/me` | CR-UI-001 | identity-service |
| `POST /api/v1/auth/logout` | CR-UI-001 | identity-service |
| `GET /api/v1/dashboard` | CR-UI-002 | Gateway BFF (fan-out < 500ms) |
| `GET /api/v1/dashboard/sla` | CR-UI-002 | sla-service |
| `GET /api/v1/notifications/stream` | CR-UI-002 | notification-service (SSE) |
| `GET /api/v2/vendors` | CR-UI-003 | data-service (autocomplete) |
| `GET /api/v2/epss/top` | CR-UI-003 | data-service |
| `GET /api/v2/epss/distribution` | CR-UI-003 | data-service |
| `GET /api/v2/cwe` | CR-UI-003 | data-service (list) |
| `POST /api/v1/findings/bulk/reopen` | CR-UI-005 | finding-service |
| `POST /api/v1/findings/bulk/assign` | CR-UI-005 | finding-service |
| `POST /api/v1/findings/{id}/notes` | CR-UI-005 | finding-service |
| `GET /api/v1/findings/stats` | CR-UI-005 | finding-service |
| `GET /api/v1/products/grades` | CR-UI-007 | finding-service |
| `GET /api/v1/reports/{id}/download` | CR-UI-009 | finding-service |
| `GET /api/v1/notifications` | CR-UI-009 | notification-service |
| `PATCH /api/v1/notifications/{id}/read` | CR-UI-009 | notification-service |
| `POST /api/v1/notifications/mark-all-read` | CR-UI-009 | notification-service |
| `POST /api/v1/webhooks/{id}/test` | CR-UI-009 | notification-service |
| `GET /api/v1/admin/users` | CR-UI-010 | identity-service |
| `POST /api/v1/admin/users/invite` | CR-UI-010 | identity-service |
| `PATCH /api/v1/admin/users/{id}` | CR-UI-010 | identity-service |
| `GET /api/v1/admin/health` | CR-UI-010 | gateway (fan-out all services) |
| `GET /api/v1/admin/settings` | CR-UI-010 | gateway |

#### 🔵 Planned v3.0 endpoints (phụ thuộc OVS CRs)

| Endpoint | CR-UI | Phụ thuộc |
|----------|-------|----------|
| `POST /api/v1/scans` (Nmap/ZAP) | CR-UI-004 | CR-OVS-001 |
| `GET /api/v1/scans/{id}/stream` (SSE) | CR-UI-004 | CR-OVS-001 |
| `GET /api/v1/assets` | CR-UI-006 | CR-OVS-007 |
| `POST /api/v1/ai/triage/{findingId}` | CR-UI-008 | CR-OVS-005 |
| `GET /api/v1/auth/mfa/setup` | CR-UI-001 | CR-OVS-003 |
| OAuth2 endpoints (google/github/callback) | CR-UI-001 | CR-OVS-003 |

### 8.3 Standard Error Format (UI-API)

Mọi API error PHẢI trả về:
```json
{
  "error": "MACHINE_READABLE_CODE",
  "message": "Human readable message in English",
  "details": {},
  "trace_id": "abc123"
}
```

| HTTP | Error Code | Tình huống |
|------|-----------|------------|
| 400 | `VALIDATION_ERROR` | Input validation failed |
| 401 | `INVALID_CREDENTIALS` | Sai email/password |
| 401 | `TOKEN_EXPIRED` | JWT hết hạn |
| 401 | `REFRESH_TOKEN_REUSED` | Replay attack detected |
| 401 | `MFA_REQUIRED` | Cần TOTP code |
| 403 | `FORBIDDEN` | Thiếu permission |
| 404 | `NOT_FOUND` | Entity không tồn tại |
| 409 | `INVALID_TRANSITION` | State machine violation |
| 423 | `ACCOUNT_LOCKED` | > 5 login failures |
| 429 | `RATE_LIMIT_EXCEEDED` | Rate limit hit |

### 8.4 Non-Functional Requirements (UI-API)

| NFR | Yêu cầu |
|-----|----------|
| Dashboard BFF | < 500ms (parallel fan-out + Redis 60s cache) |
| Auth middleware | < 5ms per request |
| SSE latency | < 2 giây từ NATS event đến browser |
| Login rate limit | 5 req/min per IP |
| CORS | Whitelist origins only |
| TLS | TLS 1.3 minimum |
| Pagination | max page_size = 200 |
| Account lockout | 5 consecutive failures → 15min lockout |
| Refresh cookie | httpOnly, Secure, SameSite=Strict |
| Access token | Không lưu localStorage — Zustand memory store |

### 8.5 UI-API Implementation Sprint Order

| Sprint | CRs | Deliverable |
|--------|-----|------------|
| **Sprint 1** | CR-UI-001 (2.1-2.4), CR-UI-002, CR-UI-003 (schema), CR-UI-005 (schema) | Login flow, Dashboard BFF, CVE search update, Finding filters |
| **Sprint 2** | CR-UI-007, CR-UI-009, CR-UI-010 | Product grades, Reports download, Notifications, Admin users |
| **Sprint 3** | CR-UI-003 (new endpoints), CR-UI-002 SSE, CR-UI-010 health | Vendors autocomplete, EPSS top, System health |
| **Sprint 4** | CR-UI-004, CR-UI-006, CR-UI-008, CR-UI-001 (MFA+OAuth) | Active scan API, Asset API, AI triage, MFA setup |

> Sprint 4 phụ thuộc v3.0 OVS services: auth-service (MFA/OAuth2), scan-service-ovs, asset-service, ai-service-ovs.
