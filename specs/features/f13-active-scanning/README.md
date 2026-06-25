# F13 — Active Vulnerability Scanning

> **Spec Folder:** `specs/features/f13-active-scanning/`  
> **Feature Doc:** [`docs/features/F13-active-scanning.md`](../../../docs/features/F13-active-scanning.md)  
> **SRS Refs:** FR-03-01 → FR-03-07  
> **Status:** 🔵 v3.0 Planned (OpenVulnScan)

---

## Sub-documents

| File | Nội dung |
|------|---------|
| [business-logic.md](./business-logic.md) | Scan state machine, Nmap/ZAP logic, agent report, scheduling |
| [dataflow.md](./dataflow.md) | Scan lifecycle flow, SSE progress stream, agent ingestion |

---

## Services (Planned)

| Service | Port | Role |
|---------|------|------|
| `scan-service-ovs` | 8058 | Scan orchestration, Nmap/ZAP execution, SSE, scheduler |
| `finding-service-ovs` | 8060 | Store scan findings với 6-state lifecycle |
| `asset-service` | 8068 | Auto-register assets sau scan |

---

## Scan State Machine

```
pending → queued → running → completed
                           → failed
                           → cancelled
```

---

## Scan Types

| Type | Tool | Output |
|------|------|--------|
| Network scan | Nmap + vulners script | CVE IDs, open ports, OS fingerprint |
| Web app scan | OWASP ZAP | Alerts: High/Medium/Low/Info |
| Agent report | External agent | SBOM-style findings list |
| Scheduled | Any of above | Periodic automated scan |

---

## Quick Reference: API Endpoints (Planned)

| Method | Endpoint | Mô tả |
|--------|----------|-------|
| POST | `/api/v1/scans` | Create and queue scan |
| GET | `/api/v1/scans/{id}` | Scan status |
| DELETE | `/api/v1/scans/{id}` | Cancel scan |
| GET | `/api/v1/scans/{id}/stream` | SSE progress stream |
| GET | `/api/v1/scans/{id}/findings` | Scan findings |
| POST | `/api/v1/agents/report` | Agent report ingestion |
| GET/POST | `/api/v1/scheduled-scans` | Scheduled scan management |

---

## Database Schema (`osv_scan_ovs`)

| Table | Key Fields | Mô tả |
|-------|-----------|-------|
| `scans` | id, product_id, type, targets[], status, started_at, completed_at | Scan job |
| `scan_findings` | id, scan_id, cve_id, title, severity, state, sha256_hash | 6-state findings |
| `scheduled_scans` | id, product_id, cron_expr, scan_type, targets[], last_run, next_run | Schedule |
| `scan_agents` | id, agent_id, api_key_id, last_seen | Registered agents |
