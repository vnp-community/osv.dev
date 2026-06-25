# OpenVulnScan — Solutions Overview

> **Phiên bản**: v1  
> **Ngày tạo**: 2026-06-16  
> **Cập nhật**: 2026-06-17  
> **Trạng thái**: ✅ **IMPLEMENTED** (7/7 solutions, 40/40 tasks)  
> **Phạm vi**: CR-OVS-001 đến CR-OVS-007 (7 Change Requests)

---

## 1. Bản Đồ Change Requests → Solutions

| CR ID | Service | Port (gRPC/HTTP) | Solution File | Priority | Trạng thái |
|-------|---------|-----------------|---------------|---------|------------|
| [CR-OVS-001](../CR-OVS-001-scan-service-nmap-zap-agent.md) | `scan-service` | 50058 / 8058 | [SOL-OVS-001](SOL-OVS-001-scan-service.md) | 🔴 High | ✅ Implemented |
| [CR-OVS-002](../CR-OVS-002-finding-service-lifecycle-sla-dedup.md) | `finding-service` | 50060 / 8060 | [SOL-OVS-002](SOL-OVS-002-finding-service.md) | 🔴 High | ✅ Implemented |
| [CR-OVS-003](../CR-OVS-003-auth-service-jwt-rbac-mfa-apikey.md) | `auth-service` | 50051 / 8051 | [SOL-OVS-003](SOL-OVS-003-auth-service.md) | 🔴 High | ✅ Implemented |
| [CR-OVS-004](../CR-OVS-004-product-engagement-test-hierarchy.md) | `product-service` | 50061 / 8061 | [SOL-OVS-004](SOL-OVS-004-product-service.md) | 🟡 Medium | ✅ Implemented |
| [CR-OVS-005](../CR-OVS-005-ai-service-embedding-severity-epss-triage.md) | `ai-service` | 50052 / 8052 | [SOL-OVS-005](SOL-OVS-005-ai-service.md) | 🟡 Medium | ✅ Implemented |
| [CR-OVS-006](../CR-OVS-006-report-service-pdf-html-csv-excel.md) | `report-service` | 50065 / 8065 | [SOL-OVS-006](SOL-OVS-006-report-service.md) | 🟡 Medium | ✅ Implemented |
| [CR-OVS-007](../CR-OVS-007-asset-management-scheduled-scans.md) | `asset-service` + `scan-service` (ext) | 50068 / 8068 | [SOL-OVS-007](SOL-OVS-007-asset-management.md) | 🟡 Medium | ✅ Implemented |

---

## 2. Kiến Trúc Tổng Thể

```
                          ┌─────────────────────────────────────────────┐
                          │            unified-gateway (port 8080)       │
                          │    JWT Auth via auth-service gRPC            │
                          └──────────────────┬──────────────────────────┘
                                             │
              ┌──────────────────────────────┼────────────────────────────────┐
              │                              │                                 │
              ▼                              ▼                                 ▼
   ┌─────────────────┐           ┌────────────────────┐           ┌─────────────────┐
   │  auth-service   │           │   scan-service      │           │ product-service  │
   │  :50051/:8051   │           │   :50058/:8058      │           │  :50061/:8061    │
   │                 │           │  ┌─────────────┐   │           │  ┌────────────┐  │
   │  - JWT RS256    │           │  │ Nmap/ZAP    │   │           │  │ProductType │  │
   │  - Argon2id     │           │  │ Agent API   │   │           │  │Product     │  │
   │  - TOTP MFA     │           │  │ SSE stream  │   │           │  │Engagement  │  │
   │  - API Keys     │           │  │ Scheduler   │   │           │  │Test        │  │
   │  - OAuth2       │           │  └─────────────┘   │           │  │CI/CD Orch. │  │
   └────────┬────────┘           └────────┬───────────┘           └────────────────┘
            │ gRPC ValidateToken          │ NATS
            │ ValidateAPIKey              │ scan.scan.completed
            │                            ▼
            │                  ┌──────────────────────────────────────┐
            │                  │              NATS JetStream           │
            │                  │  Streams: SCAN_EVENTS, FINDING_EVENTS │
            │                  └───────┬────────────┬─────────────────┘
            │                          │            │
            │             ┌────────────┘            └─────────────┐
            │             ▼                                        ▼
            │  ┌────────────────────┐                  ┌──────────────────┐
            │  │  finding-service   │                  │   ai-service     │
            │  │  :50060/:8060      │                  │  :50052/:8052    │
            │  │                   │                   │                  │
            │  │  - SLA/Lifecycle  │                  │  - Embeddings    │
            │  │  - Dedup (hash)   │◀─gRPC────────────│  - EPSS          │
            │  │  - Audit trail    │                   │  - Triage LLM   │
            │  └────────┬──────────┘                   │  - pgvector     │
            │           │ gRPC                          └──────────────────┘
            │           ▼
            │  ┌────────────────────┐    ┌──────────────────┐
            │  │  report-service    │    │  asset-service   │
            │  │  :50065/:8065      │    │  :50068/:8068    │
            │  │                   │    │                   │
            │  │  - PDF/HTML/CSV   │    │  - Asset Registry │
            │  │  - Excel(DefectDojo│   │  - OS/Services   │
            │  │  - S3/MinIO       │    │  - Risk Scoring  │
            │  │  - CI/CD exitcode │    │  - Tags          │
            │  └────────────────────┘    └──────────────────┘
```

---

## 3. Data Flow Toàn Hệ Thống

### 3.1 Active Scan Flow (Nmap/ZAP)

```
1. Client → POST /api/v1/scans (via gateway)
2. gateway → auth-service: ValidateToken (gRPC, <1ms)
3. scan-service: Create scan entity (pending)
4. NATS: scan.scan.created published
5. scan-service worker: Nmap/ZAP execution
6. NATS: scan.scan.completed {scan_id, finding_count}
7. finding-service: Import findings (with dedup)
8. asset-service: Upsert assets from scan results
9. ai-service: Enrich CVEs (embeddings, severity, EPSS)
10. product-service: Update Test.finding_count, Product.grade
```

### 3.2 CI/CD Pipeline Flow

```
1. CI pipeline → POST /api/v1/orchestrate/cicd (product-service)
2. product-service: Find/Create Product + Engagement + Test
3. product-service → finding-service gRPC: CreateFinding (per result)
4. finding-service: Dedup check → create or mark duplicate
5. product-service: Close engagement
6. Response: {new: 18, duplicates: 7, product_id, engagement_id}
```

### 3.3 Report Generation Flow

```
1. Client → POST /api/v1/reports {scan_id, formats:[pdf,html,csv]}
2. report-service: Create report_run (async)
3. report-service → finding-service gRPC: GetFindingsByScan
4. Parallel: HTML + PDF + CSV generation
5. Upload to MinIO: reports/{run_id}/
6. Update report_run: status=completed, exit_code=0|1
7. Client polls: GET /api/v1/reports/{run_id}
8. Client downloads: presigned URL from MinIO
```

---

## 4. Technology Stack Summary

| Layer | Technology | Rationale |
|-------|-----------|-----------|
| **Language** | Go 1.22 | Performance, concurrency, existing OSV codebase |
| **HTTP Router** | `chi` | Lightweight, middleware-friendly |
| **gRPC** | `google.golang.org/grpc` | Inter-service communication |
| **Database** | PostgreSQL 15 | ACID, JSONB, pgvector (AI), INET type |
| **ORM/Query** | `sqlx` + raw SQL | Performance control, no magic |
| **Migrations** | `migrate` CLI | File-based migrations |
| **Messaging** | NATS JetStream | Event-driven, persistent, ack |
| **Cache** | Redis 7 | JTI blacklist, embedding cache, EPSS |
| **Vector DB** | pgvector (PostgreSQL ext) | Semantic CVE search |
| **AI** | Ollama + OpenAI | Local-first, cloud fallback |
| **Storage** | MinIO (S3-compat) | Report artifact storage |
| **Crypto** | Argon2id | Password hashing (OWASP 2024) |
| **JWT** | RS256 (asymmetric) | Services only need public key |
| **PDF** | `chromedp` (Chromium) | HTML→PDF, modern CSS support |
| **Excel** | `excelize` | Go-native XLSX library |
| **Monitoring** | Prometheus + Grafana | Metrics |
| **Logging** | `zerolog` | Structured JSON logging |

---

## 5. Database Per Service (Isolation)

| Service | Database Name | Key Extensions |
|---------|-------------|---------------|
| `auth-service` | `auth_service` | — |
| `scan-service` | `scan_service` | — |
| `finding-service` | `finding_service` | — |
| `product-service` | `product_service` | — |
| `ai-service` | `ai_service` | **pgvector** |
| `report-service` | `report_service` | — |
| `asset-service` | `asset_service` | — |

> **Note**: Mỗi service có database riêng biệt để đảm bảo **loose coupling**. Không có direct DB queries cross-service; chỉ qua gRPC/NATS.

---

## 6. NATS Event Registry (Toàn Hệ Thống)

| Topic | Publisher | Consumers | Payload |
|-------|-----------|----------|---------|
| `scan.scan.created` | scan-service | scan-service (worker) | `{scan_id, type, targets}` |
| `scan.scan.completed` | scan-service | finding-service, asset-service, ai-service | `{scan_id, finding_count, scan_type}` |
| `scan.scan.failed` | scan-service | notification-service | `{scan_id, error}` |
| `finding.created` | finding-service | ai-service (triage) | `{finding_id, cve, severity, product_id}` |
| `finding.status.changed` | finding-service | asset-service (risk), notification-service | `{finding_id, old_status, new_status}` |
| `finding.sla.breached` | finding-service (cron) | notification-service | `{finding_id, cve, days_overdue}` |
| `ai.cve.enriched` | ai-service | vulnerability-service (embedding store) | `{cve_id, embedding_dims, severity}` |
| `ai.triage.completed` | ai-service | finding-service (update remarks) | `{finding_id, remarks, confidence}` |
| `ingestion.cve.synced` | OSV core | ai-service (enrich) | `{cve_id, summary, cvss}` |
| `product.created` | product-service | — | `{product_id, criticality}` |

---

## 7. gRPC Service Registry

| Service | gRPC Port | Called By | Key Methods |
|---------|-----------|-----------|------------|
| `auth-service` | 50051 | unified-gateway | `ValidateToken`, `ValidateAPIKey` |
| `scan-service` | 50058 | finding-service, asset-service | `GetFindings`, `GetFindingsByScan` |
| `finding-service` | 50060 | report-service, product-service | `GetFindingsByProduct`, `CreateFinding`, `GetStats` |
| `product-service` | 50061 | finding-service | `GetProduct`, `GetEngagement`, `UpdateTestFindingCount` |
| `ai-service` | 50052 | (internal) | `EnrichCVE`, `GetEPSS`, `TriageFinding` |
| `report-service` | 50065 | (none, REST only) | — |
| `asset-service` | 50068 | (internal) | `GetAsset`, `GetAssetsByTag` |

---

## 8. Implementation Priority & Dependencies

### 8.1 Dependency Graph (Must Implement First)

```
auth-service [CR-OVS-003]          ← Foundation for all services
       │
       ├── scan-service [CR-OVS-001]    ← Core scanning capability
       │         │
       │         ├── finding-service [CR-OVS-002]    ← Finding lifecycle
       │         │         │
       │         │         ├── product-service [CR-OVS-004]
       │         │         ├── report-service [CR-OVS-006]
       │         │         └── ai-service [CR-OVS-005]
       │         │
       │         └── asset-service [CR-OVS-007]
       │
       └── (all services validate tokens via auth-service)
```

### 8.2 Recommended Sprint Order

| Sprint | Services | Deliverable |
|--------|----------|------------|
| **Sprint 1** | auth-service (core) | Register, Login, JWT, Redis JTI, ValidateToken gRPC |
| **Sprint 2** | scan-service (Phase 1) | Nmap scanning, NATS publish |
| **Sprint 3** | finding-service (Phase 1-2) | Finding CRUD, dedup, state machine |
| **Sprint 4** | auth-service (MFA+OAuth) + scan-service (ZAP, Agent) | Full auth, web scanning |
| **Sprint 5** | product-service | Hierarchy, CI/CD orchestrator |
| **Sprint 6** | report-service | PDF, HTML, Excel |
| **Sprint 7** | ai-service | Embeddings, EPSS, triage |
| **Sprint 8** | asset-service + scheduled scans | Asset registry, cron jobs |

---

## 9. Key Design Principles

### 9.1 Clean Architecture (All Services)
```
Domain → UseCase → Adapter → Delivery
```
- **Domain**: Entities, business rules, repository interfaces
- **UseCase**: Application logic (orchestrates domain)
- **Adapter**: DB, cache, messaging implementations
- **Delivery**: HTTP handlers, gRPC servers

### 9.2 Security-First Patterns

| Pattern | Where Used |
|---------|-----------|
| Argon2id (memory-hard hashing) | auth-service passwords |
| RS256 JWT (asymmetric) | Token signing/verification |
| JTI Redis blacklist | Logout + revocation |
| Token family tracking | Refresh token reuse detection |
| API key prefix `ovs_` | Human-readable identification |
| AES-256-GCM encryption | TOTP secrets, OAuth tokens |
| `subtle.ConstantTimeCompare` | Password + API key verification |
| Context timeout (30s) | All external calls (LLM, EPSS, Nmap) |

### 9.3 Observability Standards (All Services)

Each service exposes:
- `GET /health` → `{"status": "ok"}`
- `GET /metrics` (port 9090) → Prometheus format
- Structured JSON logs via `zerolog`
- Request ID propagation via HTTP header `X-Request-ID`

---

## 10. Files Created

| File | Description |
|------|-------------|
| `solutions/SOL-OVS-001-scan-service.md` | Scan Service solution (Nmap, ZAP, Agent) |
| `solutions/SOL-OVS-002-finding-service.md` | Finding Service solution (Lifecycle, SLA, Dedup) |
| `solutions/SOL-OVS-003-auth-service.md` | Auth Service solution (JWT RS256, RBAC, MFA) |
| `solutions/SOL-OVS-004-product-service.md` | Product Service solution (Hierarchy, CI/CD) |
| `solutions/SOL-OVS-005-ai-service.md` | AI Service solution (Embeddings, EPSS, Triage) |
| `solutions/SOL-OVS-006-report-service.md` | Report Service solution (PDF/HTML/CSV/Excel) |
| `solutions/SOL-OVS-007-asset-management.md` | Asset Management + Scheduled Scans solution |
| `solutions/README.md` | This file — overview và navigation |
