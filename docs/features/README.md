# OSV Platform — Feature Documentation Index

**Version:** 3.0  
**Cập nhật:** 2026-06-18  
**Nguồn:** PRD.md, SRS.md, URD.md + source code analysis

---

## Tổng quan

OSV Platform (Open Source Vulnerability Platform) là hệ thống **Go Microservices** tích hợp toàn diện: CVE database, finding management, AI enrichment, và reporting — được thiết kế theo **Clean Architecture** và **Event-Driven** với NATS JetStream.

## Danh sách Tính năng

| # | File | Tính năng | Trạng thái | Personas |
|---|------|-----------|-----------|----------|
| 01 | [F01-auth.md](./F01-auth.md) | Authentication & Authorization | ✅ v2.0 + 🔵 v3.0 | All |
| 02 | [F02-cve-data-aggregation.md](./F02-cve-data-aggregation.md) | CVE Data Aggregation (Multi-source) | ✅ v2.2 | Dave, Eve |
| 03 | [F03-cve-search.md](./F03-cve-search.md) | CVE Search & Discovery | ✅ v2.2 | Alice, Bob, Eve |
| 04 | [F04-cve-intelligence.md](./F04-cve-intelligence.md) | CVE Intelligence (EPSS, KEV, CWE) | ✅ v2.2 | Bob, Eve, Carol |
| 05 | [F05-finding-management.md](./F05-finding-management.md) | Finding Lifecycle Management | ✅ v2.1 | Bob, Carol |
| 06 | [F06-product-security.md](./F06-product-security.md) | Product & Engagement Hierarchy | ✅ v2.1 | Alice, Carol |
| 07 | [F07-sla-management.md](./F07-sla-management.md) | SLA Enforcement & Tracking | ✅ v2.1 | Carol |
| 08 | [F08-scan-import.md](./F08-scan-import.md) | Scan Import Pipeline (21+ parsers) | ✅ v2.1 | Alice, Bob |
| 09 | [F09-reporting.md](./F09-reporting.md) | Reporting (PDF/HTML/CSV/Excel/JSON) | ✅ v2.1 | Carol, Dave |
| 10 | [F10-notifications.md](./F10-notifications.md) | Notifications & Webhooks | ✅ v2.1-v2.2 | All |
| 11 | [F11-jira-integration.md](./F11-jira-integration.md) | JIRA Bidirectional Integration | ✅ v2.1 | Alice, Dave |
| 12 | [F12-audit-trail.md](./F12-audit-trail.md) | Audit Trail & Compliance | ✅ v2.1 | Carol, Admin |
| 13 | [F13-active-scanning.md](./F13-active-scanning.md) | Active Vulnerability Scanning (Nmap/ZAP) | 🔵 v3.0 | Bob |
| 14 | [F14-asset-management.md](./F14-asset-management.md) | Asset Inventory & Management | 🔵 v3.0 | Bob, Carol |
| 15 | [F15-ai-enrichment.md](./F15-ai-enrichment.md) | AI Enrichment & Triage | 🔵 v3.0 | Bob |
| 16 | [F16-dashboard.md](./F16-dashboard.md) | Executive Dashboard & KPIs | 🔶 v3.1 | Carol, Frank |
| 17 | [F17-admin-management.md](./F17-admin-management.md) | Administration & System Management | 🔶 v3.1 | Admin |
| 18 | [F18-observability.md](./F18-observability.md) | Observability (Metrics/Tracing/Logs) | ✅ v2.2 | DevOps |

---

## Ký hiệu

- ✅ **Implemented** — Đã hoàn chỉnh trong v2.x
- 🔵 **Planned v3.0** — OpenVulnScan CRs (Q1 2027)
- 🔶 **Planned v3.1** — UI-API CRs (Q2 2027)

## Kiến trúc Microservices

```
Gateway (8080)
├── identity-service (8081)     → F01 Auth
├── data-service (8082)         → F02 CVE Aggregation, F04 Intelligence
├── search-service              → F03 CVE Search
├── ranking-service (8084)      → F03 CVE Search
├── finding-service (8085)      → F05 Findings, F06 Products, F09 Reports
├── scan-service                → F08 Scan Import
├── sla-service (8086)          → F07 SLA
├── notification-service (8087) → F10 Notifications
├── jira-service (8088)         → F11 JIRA
├── audit-service (8090)        → F12 Audit
├── ai-service                  → F15 AI Enrichment [v3.0]
├── asset-service (8068)        → F14 Asset [v3.0]
├── scan-service-ovs (8058)     → F13 Active Scanning [v3.0]
└── report-service (8065)       → F09 Reports [v3.0]
```
