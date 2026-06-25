# F14 — Asset Management

> **Spec Folder:** `specs/features/f14-asset-management/`  
> **Feature Doc:** [`docs/features/F14-asset-management.md`](../../../docs/features/F14-asset-management.md)  
> **SRS Refs:** FR-03-07  
> **Status:** 🔵 v3.0 Planned (OpenVulnScan)

---

## Sub-documents

| File | Nội dung |
|------|---------|
| [business-logic.md](./business-logic.md) | Asset auto-register, tagging, risk scoring, cron re-scan |
| [dataflow.md](./dataflow.md) | Asset discovery flow, risk score calc, scheduled scan flow |

---

## Services (Planned)

| Service | Port | Role |
|---------|------|------|
| `asset-service` | 8068 | Asset registry, tagging, risk scoring, scheduled re-scan |
| `scan-service-ovs` | 8058 | Trigger scans on assets |
| `finding-service-ovs` | 8060 | Provide findings data for risk scoring |

---

## Asset Types

| Type | Primary Key | Ví dụ |
|------|-------------|-------|
| Host/IP | IP address | `10.0.0.1` |
| Domain | Hostname | `api.company.com` |
| URL | Full URL | `https://app.company.com` |
| Container | Image digest | `nginx:sha256:abc123` |

---

## Quick Reference: API Endpoints (Planned)

| Method | Endpoint | Mô tả |
|--------|----------|-------|
| GET | `/api/v1/assets` | List assets (filtered) |
| GET | `/api/v1/assets/{id}` | Asset detail + risk score |
| POST | `/api/v1/assets` | Manual asset registration |
| PATCH | `/api/v1/assets/{id}/tags` | Update tags |
| GET | `/api/v1/assets/{id}/findings` | Findings for asset |
| POST | `/api/v1/assets/{id}/scan` | Trigger ad-hoc scan |

---

## Database Schema (`osv_assets`)

| Table | Key Fields | Mô tả |
|-------|-----------|-------|
| `assets` | id, product_id, type, identifier, tags[], risk_score, last_scanned_at | Asset registry |
| `asset_findings` | asset_id, finding_id | Link assets to findings |
