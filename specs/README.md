# OSV Platform — Specs Index

> **Project:** OSV Platform (Open Source Vulnerability Platform)  
> **Architecture:** Go Microservices — Clean Architecture  
> **Last Updated:** 2026-06-16  
> **Status:** v2.2 Active — v3.0 Planned  

---

## Danh Sách Tài Liệu

| # | Tài liệu | Mô tả |
|---|----------|-------|
| 01 | [Architecture Document](./01-architecture.md) | Kiến trúc Go Microservices: 13 services, gateway, data flows, NATS events, infrastructure |
| 02 | [Technical Design Document](./02-technical-design.md) | Chi tiết kỹ thuật: algorithms, schema, state machines, error handling, testing strategy |
| 03 | [Deployment Model](./03-deployment-model.md) | Mô hình triển khai, service ports, CVE source management, Kubernetes, rollout status |
| — | **[CRs v1/](./crs/v1/)** | **Change Requests đã implement (30 CRs từ cve-search + DefectDojo + GlobalCVE)** |
| — | **[CRs v2/](./crs/v2/)** | **Change Requests planned (7 CRs từ OpenVulnScan — v3.0 target)** |

---

## Tóm Tắt Hệ Thống

**OSV Platform** là nền tảng quản lý lỗ hổng bảo mật toàn diện được xây dựng lại thành **Go Microservices** từ 3 nguồn tham chiếu (cve-search, DefectDojo, GlobalCVE). Hệ thống:

- **Thu thập** CVE từ **15+ nguồn** (NVD, JVN, CIRCL, ExploitDB, CVE.org, CISA KEV, CNNVD, vendor advisories)
- **Làm phong phú** với EPSS, MITRE CAPEC/CWE, NVD CPE Dictionary, AI embeddings
- **Quản lý** findings với DefectDojo-style Product/Engagement/Test hierarchy và state machine
- **Tích hợp** JIRA bidirectional, 5-channel notifications, SLA enforcement, audit trail

---

## Services Chính (v2.2 — 13 services)

```
apps/osv/           → API Gateway (dual auth, rate-limit, 100+ routes)
services/
├── data-service/       → CVE ingestion (15+ fetchers), EPSS, KEV, taxonomy
├── search-service/     → OpenSearch BM25 + pgvector semantic
├── ranking-service/    → CPE popularity ranking
├── identity-service/   → LDAP auth, API key management
├── finding-service/    → Product hierarchy, findings, state machine, reports
├── scan-service/       → 21+ parsers, 12-step pipeline, dedup
├── sla-service/        → SLA config, daily breach detection
├── notification-service/ → Email/Slack/Teams/Webhook/In-app, 14 events
├── jira-service/       → AES creds, bidirectional sync
├── audit-service/      → Append-only HMAC event log
├── ai-service/         → CVE embeddings
└── shared/             → Shared Go packages
```

---

## Công Nghệ Sử Dụng

| Layer | Technology |
|-------|-----------|
| Languages | Go 1.22+ |
| Database | PostgreSQL 16 + pgvector |
| Cache | Redis 7 |
| Search | OpenSearch 2 (BM25 + aggregations) |
| Vector | pgvector (1536-dim, IVFFlat index) |
| Messaging | NATS JetStream (at-least-once) |
| Object Storage | MinIO / AWS S3 |
| API | REST + gRPC |
| Observability | zerolog + Prometheus + OpenTelemetry/Jaeger |
| Container | Docker + Docker Compose |
| Orchestration | Kubernetes (Helm charts) |

---

## Change Request Index

| CR Series | Nguồn | Số CRs | Thư mục | Trạng thái |
|-----------|-------|--------|---------|-----------|
| cve-search | cve-search Python monolith | 9 CRs | `crs/v1/cve-search/` | ✅ Done |
| DefectDojo | Django DefectDojo | 11 CRs | `crs/v1/DefectDojo/` | ✅ Done |
| GlobalCVE | GlobalCVE v3 (Next.js) | 10 CRs | `crs/v1/globalcve/` | ✅ Done |
| OpenVulnScan | OpenVulnScan v3 (Go) | 7 CRs | `crs/v2/OpenVulnScan/` | 🔵 Planned |
| **Tổng implemented** | | **30 CRs** | | ✅ |

---

## Tài Liệu Dự Án (docs/)

| Tài liệu | Mô tả |
|---------|-------|
| [docs/PRD.md](../docs/PRD.md) | Product Requirements Document — features, roadmap, personas |
| [docs/SRS.md](../docs/SRS.md) | Software Requirements Specification — functional + non-functional |
| [docs/URD.md](../docs/URD.md) | User Requirements Document — per-persona requirements + traceability |
