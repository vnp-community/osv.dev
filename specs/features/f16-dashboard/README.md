# F16 — Executive Dashboard & KPIs

> **Spec Folder:** `specs/features/f16-dashboard/`  
> **Feature Doc:** [`docs/features/F16-dashboard.md`](../../../docs/features/F16-dashboard.md)  
> **Status:** ✅ v2.2 Implemented

---

## Sub-documents

| File | Nội dung |
|------|---------|
| [business-logic.md](./business-logic.md) | KPI calculations, aggregation logic, caching strategy |
| [dataflow.md](./dataflow.md) | Dashboard data aggregation flow, real-time updates |

---

## Services

| Service | Port | Role |
|---------|------|------|
| `finding-service` | 8085 | Finding aggregations, product grades |
| `data-service` | 8082 | CVE stats, KEV stats |
| `sla-service` | 8086 | SLA compliance metrics |

---

## Dashboard Sections

| Section | Widgets |
|---------|---------|
| **Overview** | Total active findings, critical count, SLA compliance %, products at risk |
| **Risk Map** | Product list with grade + risk score heatmap |
| **CVE Intelligence** | New KEV this week, high EPSS CVEs, trending vulnerabilities |
| **SLA Status** | On-track vs breached by severity |
| **Trend** | Findings over time (line chart, 30/60/90 days) |
| **Top Vulnerabilities** | Most common CVEs across products |

---

## Quick Reference: API Endpoints

| Method | Endpoint | Mô tả |
|--------|----------|-------|
| GET | `/api/v2/dashboard/overview` | Global KPIs |
| GET | `/api/v2/dashboard/products` | Per-product risk summary |
| GET | `/api/v2/dashboard/sla` | SLA compliance breakdown |
| GET | `/api/v2/dashboard/trend` | Finding trend (30/60/90d) |
| GET | `/api/v2/dashboard/cve-intel` | CVE intelligence widgets |
| GET | `/api/v2/dashboard/top-vulnerabilities` | Most common CVEs |

---

## Caching Strategy

| Widget | Cache TTL | Rationale |
|--------|----------|-----------|
| Overview KPIs | 5 phút | Acceptable slight delay |
| Product grades | 5 phút | Updated on finding change |
| CVE intelligence | 1 giờ | KEV/EPSS update hourly |
| Trend data | 15 phút | Historical, changes slowly |
