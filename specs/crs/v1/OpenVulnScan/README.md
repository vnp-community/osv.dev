# Change Requests — OpenVulnScan → OSV

**Ngày tạo:** 2026-06-14  
**Ngày implement:** 2026-06-17  
**Trạng thái:** ✅ **IMPLEMENTED** — 7/7 CRs hoàn thành

## Mục tiêu

Nâng cấp **OSV (OpenVulnScan Go Microservices)** để tích hợp toàn bộ chức năng từ **OpenVulnScan v3.0** — hệ thống active vulnerability scanning với Nmap/ZAP, DefectDojo-style finding management, và AI-powered enrichment.

## Đặc điểm nổi bật của OpenVulnScan

OpenVulnScan **khác biệt hoàn toàn** với OSV (CVE database) và GlobalCVE (CVE search):

| Khía cạnh | OSV (CVE Database) | OpenVulnScan |
|-----------|-------------------|-------------|
| Mục đích | CVE data search/sync | **Active vulnerability scanning** |
| Nguồn data | NVD, CIRCL, JVN... | **Nmap + OWASP ZAP + Agent** |
| Đối tượng | CVE records | **Network hosts + Web apps + IT assets** |
| Workflow | Query CVEs | **Scan → Find → Triage → Report** |
| Auth | API key (basic) | **JWT RS256 + RBAC + MFA + OAuth2** |
| AI | Semantic search | **Severity classification + Finding triage** |

---

## Nguồn tham chiếu

- `OpenVulnScan/docs/PRD.md` — Product Requirements
- `OpenVulnScan/docs/SRS.md` — Software Requirements
- `OpenVulnScan/specs/services/` — Go microservices specs (12 files)

---

## Tổng quan Gap Analysis

### Services hoàn toàn mới cần tích hợp

| Service | Port | Mô tả |
|---------|:----:|-------|
| `auth-service` | 50051 | JWT RS256, RBAC, MFA, OAuth2 (Google/GitHub) |
| `scan-service` | 50058 | Nmap + ZAP + Agent scanning |
| `finding-service` | 50060 | Finding lifecycle, SLA, dedup, audit |
| `product-service` | 50061 | Product/Engagement/Test hierarchy |
| `ai-service` | 50052 | Embeddings, LLM severity, EPSS, triage |
| `report-service` | 50065 | PDF, HTML, CSV, Excel reports |
| `asset-service` | — | Asset registry, tagging, history |
| `impact-service` | 50053 | SBOM impact analysis, CPE matching |
| `integration-service` | 50054 | Jira + external ticketing |

---

## Danh sách Change Requests

| CR ID | Tên | Target | Loại | Priority | Status |
|-------|-----|--------|------|---------|--------|
| [CR-OVS-001](./CR-OVS-001-scan-service-nmap-zap-agent.md) | Scan Service — Nmap/ZAP/Agent Scanning, State Machine, SSE Progress | **MỚI**: `scan-service:50058` | New Service | 🔴 High | ✅ Implemented |
| [CR-OVS-002](./CR-OVS-002-finding-service-lifecycle-sla-dedup.md) | Finding Service — Lifecycle, SLA Policy, Hash Deduplication, Audit | **MỚI**: `finding-service:50060` | New Service | 🔴 High | ✅ Implemented |
| [CR-OVS-003](./CR-OVS-003-auth-service-jwt-rbac-mfa-apikey.md) | Auth Service — JWT RS256, RBAC, TOTP MFA, API Keys (ovs_), OAuth2 | **MỚI**: `auth-service:50051` | New Service | 🔴 High | ✅ Implemented |
| [CR-OVS-004](./CR-OVS-004-product-engagement-test-hierarchy.md) | Product Service — Product/Engagement/Test Hierarchy, CI/CD Orchestrator | **MỚI**: `product-service:50061` | New Service | 🟡 Medium | ✅ Implemented |
| [CR-OVS-005](./CR-OVS-005-ai-service-embedding-severity-epss-triage.md) | AI Service — Embeddings, LLM Severity, EPSS, Finding Triage | **MỚI**: `ai-service:50052` | New Service | 🟡 Medium | ✅ Implemented |
| [CR-OVS-006](./CR-OVS-006-report-service-pdf-html-csv-excel.md) | Report Service — PDF/HTML/CSV/Excel Reports, CI/CD Exit Code | **MỚI**: `report-service:50065` | New Service | 🟡 Medium | ✅ Implemented |
| [CR-OVS-007](./CR-OVS-007-asset-management-scheduled-scans.md) | Asset Management + Scheduled Scans (cron via NATS) | `scan-service` + **MỚI**: `asset-service` | Feature | 🟡 Medium | ✅ Implemented |

## Implementation Summary

| Service | Files Go | Implementation Date |
|---------|:--------:|---------------------|
| `scan-service` (CR-OVS-001) | 14 files | 2026-06-17 |
| `finding-service` (CR-OVS-002) | 18 files | 2026-06-17 |
| `identity-service` (CR-OVS-003) | 17 files | 2026-06-17 |
| `product-service` (CR-OVS-004) | 3 files | 2026-06-17 |
| `ai-service` (CR-OVS-005) | 25 files | 2026-06-17 |
| `report-service` (CR-OVS-006) | 9 files | 2026-06-17 |
| `asset-service` + sched (CR-OVS-007) | 14 files | 2026-06-17 |
| **Tổng** | **100 files** | 2026-06-17 |

---

---

## Feature Coverage

### ✅ Đã có trong OSV

```
- CVE database + search (NVD/CIRCL/JVN)
- KEV tracking
- EPSS scoring (CR-GCV-002)
- API Gateway (JWT auth basic)
- OpenSearch search
- Redis cache
- Prometheus metrics
- NATS JetStream
```

### ❌ Chưa có — OpenVulnScan bổ sung (covered by CRs above)

```
CR-OVS-001 — Scan Service:
  - Nmap full scan (-sV -O --script=vulners)
  - OWASP ZAP web scan (spider + active scan)
  - Host discovery scan (-sn -PE)
  - Agent-based package scanning (Python agent)
  - Scan state machine (pending→queued→running→completed)
  - SSE real-time progress streaming
  - Scan cancellation (terminate subprocess)
  - Parallel scan execution

CR-OVS-002 — Finding Service:
  - Finding lifecycle (active/mitigated/fp/risk_accepted/out_of_scope/duplicate)
  - SLA deadlines (Critical:7d, High:30d, Medium:90d, Low:180d)
  - Hash-based deduplication (SHA-256)
  - Finding audit trail (every state change)
  - Finding grouping and tagging
  - CVSSv4 tracking
  - VEX justification fields
  - Import findings from scan-service (NATS consumer)

CR-OVS-003 — Auth Service:
  - JWT RS256 (asymmetric, 15min TTL)
  - Refresh token rotation (reuse = revoke family)
  - TOTP MFA (RFC 6238, 30s window)
  - RBAC: admin/user/readonly/agent + fine-grained permissions
  - API keys with ovs_ prefix (SHA-256 stored)
  - Google OAuth2 callback
  - GitHub OAuth2 callback
  - Argon2id password hashing
  - Account lockout (5 failed attempts)
  - Audit log (all auth events)
  - gRPC ValidateToken (<1ms, Redis only)

CR-OVS-004 — Product Service:
  - ProductType hierarchy
  - Product (business criticality, lifecycle, platform)
  - Engagement (Interactive + CI/CD type)
  - Test context (linked to scan)
  - CI/CD Orchestrator (auto create product+engagement+test)
  - Deduplication per engagement
  - Product numeric grade

CR-OVS-005 — AI Service:
  - CVE embedding generation (pgvector, 1536 dims)
  - Redis embedding cache (7-day TTL)
  - LLM severity classification (CVSS-first, LLM fallback)
  - LLM provider chain (Ollama → OpenAI → Azure, failover)
  - EPSS per-CVE query (FIRST.org API, daily cache)
  - Finding triage recommendations (Confirmed/FP/NotAffected)
  - MITRE tagging (CVE → CAPEC)
  - Parallel enrichment (embedding + severity + exploit + MITRE)

CR-OVS-006 — Report Service:
  - HTML report (Bootstrap, light/dark theme, charts)
  - PDF report (executive summary + CVE table)
  - Excel/XLSX (DefectDojo finding import format)
  - CSV report (with VEX/EPSS fields)
  - Console output (ANSI colored terminal)
  - Severity + CVSS score filtering
  - S3/MinIO artifact storage
  - CI/CD exit code (0=clean, 1=CVEs found)
  - Parallel format generation

CR-OVS-007 — Asset Management + Scheduling:
  - Asset registry (IP, hostname, OS, services, web tech)
  - Auto-upsert assets after scan completion
  - Asset tagging (add/remove/set modes)
  - Asset risk score (derived from finding severity)
  - Asset finding/scan history
  - Scheduled scans (daily/weekly/custom cron)
  - Cron-based scheduler (NATS timer, every minute check)
  - Schedule enable/disable/manual trigger
  - Next run time computation
```

---

## Kiến trúc tổng quan

```
╔══════════════════════════════════════════════════════════════════════════╗
║                         CLIENT LAYER                                      ║
║  Web UI (Vue 3)  │  REST Client  │  Agent SDK (Python)  │  CLI (cvectl)  ║
╚══════════════════╦═══════════════════════════════════════════════════════╝
                   ║ HTTPS
╔══════════════════╩═══════════════════════════════════════════════════════╗
║                      UNIFIED GATEWAY :8080                                ║
║  JWT/API-key (→ auth-service gRPC)  |  Route Dispatch  |  BFF Layer      ║
╚══╦═══╦══════╦══════╦══════╦═════╦══════╦═══════╦════════╦══════════════╝
   ║   ║      ║      ║      ║     ║      ║       ║        ║
   ▼   ▼      ▼      ▼      ▼     ▼      ▼       ▼        ▼
┌──────┐ ┌──────┐ ┌──────┐ ┌────────┐ ┌──────┐ ┌──────┐ ┌──────┐ ┌──────┐
│ Auth │ │ Scan │ │Finding│ │Product │ │  AI  │ │Report│ │Asset │ │Vuln  │
│:50051│ │:50058│ │:50060 │ │:50061  │ │:50052│ │:50065│ │(new) │ │:50055│
└──────┘ └──┬───┘ └───┬───┘ └────────┘ └──────┘ └──────┘ └──────┘ └──────┘
            │          │
            ▼          ▼
         Nmap       Finding
          ZAP       Dedup
         Agent      SLA

════════════════ INFRASTRUCTURE LAYER ═══════════════════════════════════
  PostgreSQL 16 (pgvector)    Redis 7      NATS JetStream
  OpenSearch 2                Prometheus   OpenTelemetry → Jaeger
```

---

## NATS Event Flow

```
scan.scan.created        → ExecuteScan worker (scan-service)
scan.scan.completed      → finding-service (import findings)
                         → notification-service (alert users)
                         → ai-service (trigger enrichment)
scan.finding.discovered  → ai-service (per-finding enrichment)
ai.cve.enriched          → vulnerability-service (store embedding + severity)
ai.triage.completed      → finding-service (update triage recommendation)
finding.created          → notification-service (new finding alert)
finding.sla.breached     → notification-service (SLA breach alert)
finding.status.changed   → notification-service
ingestion.cve.synced     → ai-service (trigger embedding generation)
```

---

## Implementation Priority

### Phase 1 — Core Scanning (🔴 High)

1. **CR-OVS-003**: Auth Service (JWT/RBAC/MFA) — prerequisite cho tất cả
2. **CR-OVS-001**: Scan Service (Nmap/ZAP/Agent)
3. **CR-OVS-002**: Finding Service (lifecycle/SLA/dedup)

### Phase 2 — Management Layer (🟡 Medium)

4. **CR-OVS-004**: Product Service (hierarchy/CI-CD)
5. **CR-OVS-007**: Asset Management + Scheduling

### Phase 3 — AI & Reports (🟡 Medium)

6. **CR-OVS-005**: AI Service (embedding/triage/EPSS)
7. **CR-OVS-006**: Report Service (PDF/HTML/Excel)

---

## CR Format

Mỗi CR bao gồm:
- **Gap Analysis**: So sánh OSV vs OpenVulnScan feature-by-feature
- **Domain Model**: Go entities, value objects, state machines
- **Use Cases**: Core business logic với Go code samples
- **Database Schema**: PostgreSQL DDL (schema-per-service)
- **API Routes**: REST endpoints với request/response
- **NATS Events**: Published + Subscribed
- **Observability**: Prometheus metrics
- **Acceptance Criteria**: Testable requirements
