# Product Requirements Document (PRD) — OSV Platform

**Version:** 3.0  
**Ngày cập nhật:** 2026-06-16  
**Trạng thái:** Active — v2.2 Implemented, v3.0 Target  

---

## 1. Product Overview

**Product Name:** OSV Platform (Open Source Vulnerability Platform)  
**Tagline:** *The complete vulnerability intelligence and finding management platform — from CVE discovery to security workflows.*

OSV Platform là hệ thống **Go Microservices** tích hợp toàn diện: CVE database, finding management, AI enrichment, và reporting — được thiết kế theo **Clean Architecture** và **Event-Driven** với NATS JetStream.

### 1.1 Phiên bản và lịch sử

| Version | Mô tả | Nguồn CR | Trạng thái |
|---------|-------|----------|-----------| 
| v1.0 | OSV gốc — CVE database trên GCP (Python + Datastore) | — | Legacy |
| v2.0 | Go Microservices migration — CVE search, KEV, API gateway | cve-search CRs (v1) | ✅ Implemented |
| v2.1 | DefectDojo integration — Finding lifecycle, Product hierarchy | DefectDojo CRs (v1) | ✅ Implemented |
| v2.2 | GlobalCVE integration — Multi-source fetcher, OpenSearch, pgvector, EPSS | GlobalCVE CRs (v1) | ✅ Implemented |
| **v3.0** | **Full platform** — Active scanning (Nmap/ZAP), Auth RS256, AI triage, Reports | OpenVulnScan CRs (v1) | 🔵 Planned |
| **v3.1** | **UI completeness** — React SPA API-first frontend, Dashboard BFF, SSE streams | UI-API CRs (v0) | 🔵 Planned |

> **Current State (v2.2):** 30 CRs đã được implement từ 3 nguồn (cve-search + DefectDojo + GlobalCVE). Kiến trúc microservices Go hoàn chỉnh với gateway, 13 services, PostgreSQL 16, OpenSearch 2, NATS JetStream. **UI-API (10 CRs)** xác định ~70 REST endpoints + 2 SSE streams cần thiết cho React SPA frontend.

---

## 2. Objectives

1. **Aggregation**: Thu thập CVE từ 15+ nguồn (NVD, CIRCL, JVN, ExploitDB, CISA KEV, CNNVD, vendor advisories).
2. **Intelligence**: Làm phong phú dữ liệu CVE bằng EPSS, MITRE CAPEC/CWE, AI embeddings, semantic search.
3. **Finding Management**: Quản lý vòng đời phát hiện lỗ hổng (active → mitigated/false_positive/risk_accepted) với SLA và deduplication.
4. **Reporting**: Xuất báo cáo đa định dạng (PDF, HTML, CSV, Excel, JSON) cho developers và auditors.
5. **Integration**: Kết nối CI/CD pipelines, JIRA, webhook notifications, và DefectDojo-style product hierarchy.
6. **[Planned v3.0] Active Scanning**: Chủ động quét hạ tầng bằng Nmap và OWASP ZAP; ingest báo cáo từ remote agents.

---

## 3. Target Audience & Personas

### 3.1 Developer / DevSecOps Engineer (Alice)
- Muốn kiểm tra dependencies có lỗ hổng không trong CI/CD pipeline.
- Cần exit code 0/1 để fail build khi có CVE nghiêm trọng.
- Dùng API hoặc CLI để tự động hóa.

### 3.2 Security Analyst (Bob)
- Cần tra cứu CVE theo CPE, vendor/product, CWE taxonomy.
- Quản lý findings qua vòng đời: active → triage → mitigate.
- Import scan results từ Nmap XML, Trivy, Bandit, Snyk qua parser factory.

### 3.3 Security Manager / CISO (Carol)
- Cần báo cáo tổng hợp (PDF/HTML) về tình trạng bảo mật của sản phẩm.
- Theo dõi SLA compliance (Critical phải fix trong 7 ngày).
- Muốn dashboard hiển thị risk score per product.

### 3.4 Tool Builder / Platform Integrator (Dave)
- Tích hợp OSV API vào package managers, IDEs, security scanners (osv-scanner, Trivy, Renovate).
- Cần bulk data dumps và webhook notifications khi CVE mới xuất hiện.
- Dùng gRPC hoặc REST API ổn định để build products.

### 3.5 Security Researcher / CERT Team (Eve)
- Cần tra cứu CVE theo CWE/CAPEC taxonomy.
- Muốn semantic search ("find CVEs similar to Log4Shell").
- Cần xuất dataset để nghiên cứu (JSON/CSV bulk export).

### 3.6 [Planned v3.0] Remote Security Agent (automated)
- Là Python agent chạy trên remote host, báo cáo installed packages + CVEs.
- Authenticate bằng API key.
- Đẩy SBOM-based findings về scan-service.

---

## 4. Key Features

### 4.1 CVE Data Platform ✅ Implemented (v2.0 — v2.2)

| Feature | Mô tả | CR tham chiếu |
|---------|-------|--------------| 
| Multi-source ingestion | NVD (2h), JVN (1h), CIRCL (6h), ExploitDB (24h), CVE.org (12h), CNNVD, vendors | CR-GCV-001 |
| EPSS scoring | Daily sync từ FIRST.org, filter/sort by exploit probability | CR-GCV-002 |
| MITRE CAPEC/CWE | Weekly sync, CVE-CWE-CAPEC linking | CR-GCV-003, CR-003 |
| OpenSearch FTS | BM25 full-text + aggregations | CR-GCV-004 |
| Semantic search | pgvector (1536 dims), cosine similarity | CR-GCV-004 |
| NVD CPE Dictionary | Weekly sync, vendor/product filter API | CR-GCV-005 |
| KEV tracking | CISA KEV catalog, KnownRansomware flag, diff detection | CR-GCV-007 |
| CVE export | JSON/CSV bulk download, source attribution | CR-GCV-010 |
| CVE browse | Browse by vendor/product catalog | CR-002 (cve-search) |
| CVE taxonomy | CWE/CAPEC lookup APIs | CR-003 (cve-search) |
| CVE feeds | Recent/last-N, Atom/RSS feed | CR-008 (cve-search) |
| CPE ranking | Vendor/product popularity ranking | CR-004 (cve-search) |
| DB statistics | Source counts, last sync times | CR-005 (cve-search) |

### 4.2 Finding Management ✅ Implemented (v2.1)

| Feature | Mô tả | CR tham chiếu |
|---------|-------|--------------| 
| Finding lifecycle | active → mitigated/false_positive/risk_accepted/out_of_scope/duplicate | CR-DD-004 |
| Hash deduplication | SHA-256(title+component+version+cve) | CR-DD-003 |
| SLA enforcement | Critical:7d, High:30d, Medium:90d, Low:180d | CR-DD-006 |
| Audit trail | Immutable event log per finding (HMAC-SHA256) | CR-DD-010 |
| Risk acceptance | Full + Simple risk acceptance with expiry | CR-DD-005 |
| Bulk operations | Bulk close/reopen/tag multiple findings | CR-DD-004 |
| Scan import | 21+ parser factory (Nmap XML, ZAP JSON, Bandit, Trivy...) | CR-DD-002 |

### 4.3 Product & Engagement Hierarchy ✅ Implemented (v2.1)

| Feature | Mô tả | CR tham chiếu |
|---------|-------|--------------| 
| ProductType | Category: Web App, API, Infrastructure, Mobile | CR-DD-001 |
| Product | Software under test, business criticality, lifecycle | CR-DD-001 |
| Engagement | Testing event: Interactive or CI/CD pipeline | CR-DD-001 |
| Test | Specific scan/assessment context | CR-DD-001 |
| Product grading | A–F grade based on finding severity distribution | CR-DD-009 |
| Report service | PDF/HTML/CSV/Excel reports per product | CR-DD-009 |

### 4.4 Authentication & Authorization ✅ Implemented (v2.0 + v2.1)

| Feature | Mô tả | CR tham chiếu |
|---------|-------|--------------| 
| JWT (HS256) | Access token, stateless auth | identity-service |
| RBAC | admin / user / readonly roles | CR-007 (cve-search) |
| API Keys | Scoped permissions, SHA-256 stored | CR-007 (cve-search) |
| LDAP | Enterprise LDAP authentication provider | CR-007 (cve-search) |
| Rate limiting | Per-IP Redis token bucket | CR-GCV-008 |
| Dual auth gateway | JWT + API Key in single middleware | CR-DD-011, CR-GCV-008 |

> **[Planned v3.0]** JWT RS256, Refresh token rotation, MFA (TOTP), OAuth2 Google/GitHub, Argon2id passwords, Account lockout.

### 4.5 Notifications & Integrations ✅ Implemented (v2.1 + v2.2)

| Feature | Mô tả | CR tham chiếu |
|---------|-------|--------------| 
| Webhook service | Registration, HMAC-SHA256 delivery, retry/backoff | CR-GCV-006, CR-DD-007 |
| Email/Slack/Teams | Multi-channel notification | CR-DD-007 |
| JIRA integration | Bidirectional sync (finding → ticket) | CR-DD-008 |
| SLA breach alerts | Notify when SLA deadline exceeded | CR-DD-006 |
| KEV new alerts | Notify when CVE added to CISA KEV | CR-GCV-007 |
| 14 event types | finding.created, sla.breached, kev.new, ... | CR-DD-007 |

### 4.6 Reporting ✅ Implemented (v2.1)

| Feature | Mô tả | CR tham chiếu |
|---------|-------|--------------| 
| HTML report | Bootstrap, light/dark theme, charts | CR-DD-009 |
| PDF report | Executive summary, CVE table, distribution chart | CR-DD-009 |
| CSV export | All CVE/finding data | CR-DD-009, CR-GCV-010 |
| Excel/XLSX | DefectDojo finding import format | CR-DD-009 |
| JSON export | Bulk CVE dataset with source attribution | CR-GCV-010 |
| Product grading | A-F letter grade per product | CR-DD-009 |

### 4.7 Observability ✅ Implemented (v2.2)

| Feature | Mô tả | CR tham chiếu |
|---------|-------|--------------| 
| Structured logging | zerolog JSON, trace_id propagation | CR-GCV-009 |
| Prometheus metrics | HTTP, gRPC, DB, cache, NATS metrics | CR-GCV-009 |
| OpenTelemetry | Distributed tracing → Jaeger | CR-GCV-009 |
| Health endpoints | GET /health + GET /ready per service | CR-GCV-009 |

### 4.8 [Planned v3.0] Active Vulnerability Scanning

| Feature | Mô tả | CR tham chiếu |
|---------|-------|--------------| 
| Nmap full scan | `-sV -O --script=vulners` → CVE detection per host | CR-OVS-001 |
| OWASP ZAP | Spider + Active Scan → XSS, SQLi, CSRF, ... | CR-OVS-001 |
| Agent scanning | Python agent → SBOM packages + CVE matching | CR-OVS-001 |
| Scan scheduling | Daily/weekly/custom cron via NATS | CR-OVS-007 |
| Asset registry | Auto-register hosts from scans, tagging, history | CR-OVS-007 |

### 4.9 [Planned v3.0] AI & Enrichment

| Feature | Mô tả | CR tham chiếu |
|---------|-------|--------------| 
| CVE embeddings | Generate 1536-dim vectors, Redis cache 7 days | CR-OVS-005 |
| LLM severity | CVSS-first, Ollama/OpenAI fallback | CR-OVS-005 |
| Finding triage | AI-assisted: Confirmed/FalsePositive/NotAffected | CR-OVS-005 |

### 4.10 [Planned v3.1] UI-API (Frontend API Contracts)

**Nguồn:** 10 CRs từ `specs/crs/v0/ui-api/` — xác định API-first contracts cho React SPA frontend (~70 endpoints + 2 SSE streams).

| CR | Feature Group | ưu tiên | Trạng thái |
|----|--------------|---------|------------|
| CR-UI-001 | **Auth & User API** — login, refresh, me, logout, MFA setup, OAuth2 (Google/GitHub) | P0 Critical | ⚠️ Cần mở rộng identity-service |
| CR-UI-002 | **Dashboard & KPI API** — BFF aggregate, SLA dashboard, SSE notification stream | P0 Critical | ❌ Thiếu hoàn toàn |
| CR-UI-003 | **CVE Intelligence API** — search schema update (is_kev, has_exploit, epss), vendors autocomplete, EPSS top/distribution, CWE list | P0 Critical | ⚠️ Cần schema update |
| CR-UI-004 | **Active Scanning API** — POST /scans (Nmap/ZAP), SSE stream, cancel, results | P1 High | ❌ Planned (CR-OVS-001) |
| CR-UI-005 | **Finding Management API** — list/filter/patch, bulk ops, notes, audit trail, SLA fields | P0 Critical | ⚠️ Cần schema update |
| CR-UI-006 | **Asset Management API** — list, get by IP/ID, patch tags, findings per asset | P1 High | ❌ Planned (CR-OVS-007) |
| CR-UI-007 | **Product Security API** — products/grades, engagement/test CRUD, finding_summary | P0 Critical | ⚠️ Cần thêm endpoints |
| CR-UI-008 | **AI Center API** — triage request/review/queue, CVE enrichment status | P1 High | ❌ Planned (CR-OVS-005) |
| CR-UI-009 | **Reports & Notifications API** — reports CRUD/download, notifications list/read, webhooks test | P0 Critical | ⚠️ Cần thêm endpoints |
| CR-UI-010 | **Admin & Integrations API** — user management, system health fan-out, JIRA config, audit log, settings | P0 Critical | ⚠️ Cần thêm endpoints |

**Endpoint gap summary:**
- ✅ **Implemented** (cần schema adjustment): search, findings CRUD, products, SLA config, JIRA config
- ❌ **New endpoints needed (P0)**: `GET /api/v1/dashboard`, `POST /api/v1/auth/login`, `GET /api/v1/auth/me`, `GET /api/v1/findings/stats`, `GET /api/v1/products/grades`, `GET /api/v1/notifications`, `GET /api/v1/admin/users`, `GET /api/v1/admin/health`
- 🔵 **Planned v3.0**: scan SSE stream, asset management, AI triage, MFA setup, OAuth2 callback

**Non-functional requirements (UI-API):**
- Dashboard BFF: < 500ms response (parallel fan-out + Redis 60s cache)
- Auth middleware: < 5ms per request
- SSE latency: < 2 giây từ NATS event đến browser
- Standard error format: `{error, message, details, trace_id}` cho mọi endpoint

---

## 5. Success Metrics

| Metric | Target | Status |
|--------|--------|--------|
| API P95 response time | < 100ms (CVE lookup), < 500ms (search) | ✅ |
| CVE data freshness | NVD sync lag < 2 hours | ✅ |
| CVE coverage | 300K+ CVEs across 15+ sources | ✅ |
| Semantic search accuracy | > 80% relevance @top-10 | ✅ |
| SLA breach notification | < 5 minutes after breach detected | ✅ |
| API availability | 99.9% uptime | Target |
| [v3.0] Nmap scan completion | < 5 minutes for /24 subnet | 🔵 Planned |
| [v3.0] Report generation | < 30 seconds for HTML/PDF | 🔵 Planned |

---

## 6. Constraints & Non-Goals

- **Non-Goal**: OSV không thay thế Shodan/Censys (internet-scale scanning).
- **Non-Goal**: OSV không tự exploit các lỗ hổng (không có exploit engine).
- **[v3.0 Constraint]**: Nmap và ZAP chỉ quét targets được authorized — không có internet-wide scanning.
- **[v3.0 Constraint]**: LLM triage là *gợi ý*, không phải quyết định cuối cùng — human review cần thiết.

---

## 7. Roadmap

### Phase 1 — Foundation ✅ DONE (Q3 2026) — v2.0
- CR-cve-search (9 CRs): Multi-source CPE search, taxonomy, EPSS, auth LDAP
- Microservices: data-service, search-service, ranking-service, identity-service, gateway

### Phase 2 — Security Workflows ✅ DONE (Q3 2026) — v2.1
- CR-DD (11 CRs): Product hierarchy, finding lifecycle, SLA, notifications, JIRA, audit
- Microservices: finding-service, scan-service, sla-service, notification-service, jira-service, audit-service

### Phase 2.2 — Intelligence ✅ DONE (Q4 2026) — v2.2
- CR-GCV (10 CRs): OpenSearch, pgvector, EPSS daily, KEV advanced, CPE dict, webhooks, observability

### Phase 3 — Active Scanning & AI 🔵 PLANNED (Q1 2027) — v3.0
- CR-OVS (7 CRs): Auth RS256/MFA/OAuth2, Scan (Nmap/ZAP), Asset management, AI triage, CI/CD orchestrator
- Sprint 1-8 theo dependency graph: auth-service → scan-service → finding-service-ovs → product-service → report-service → ai-service → asset-service

### Phase 3.1 — UI Completeness 🔵 PLANNED (Q2 2027) — v3.1
- CR-UI (10 CRs): Auth API, Dashboard BFF, CVE Intelligence schema updates, Active Scanning API, Finding Management API, Asset API, Product API, AI Center API, Reports & Notifications, Admin & Integrations
- Sprint 1-4: Foundation (P0 auth+dashboard) → Core Features → Intelligence endpoints → v3.0 features (scan/asset/AI)

---

## 8. Change Request Index

| CR Series | Nguồn | Số CRs | Thư mục | Trạng thái |
|-----------|-------|--------|---------|-----------|
| cve-search | cve-search Python monolith | 9 CRs (CR-001 → CR-009) | `specs/crs/v1/cve-search/` | ✅ Done |
| DefectDojo | Django DefectDojo | 11 CRs (CR-DD-001 → CR-DD-011) | `specs/crs/v1/DefectDojo/` | ✅ Done |
| GlobalCVE | GlobalCVE v3.0 (Next.js) | 10 CRs (CR-GCV-001 → CR-GCV-010) | `specs/crs/v1/globalcve/` | ✅ Done |
| OpenVulnScan | OpenVulnScan v3.0 (Go) | 7 CRs (CR-OVS-001 → CR-OVS-007) | `specs/crs/v1/OpenVulnScan/` | 🔵 Planned (v3.0) |
| **UI-API** | **React SPA Frontend API-First** | **10 CRs (CR-UI-001 → CR-UI-010)** | **`specs/crs/v0/ui-api/`** | **🔵 Planned (v3.1)** |
| **Tổng implemented** | | **30 CRs** | | ✅ |
| **Tổng planned** | | **47 CRs** | | |
