# F07 — SLA Enforcement & Tracking

> **Spec Folder:** `specs/features/f07-sla-management/`  
> **Feature Doc:** [`docs/features/F07-sla-management.md`](../../../docs/features/F07-sla-management.md)  
> **SRS Refs:** FR-04-02  
> **Status:** ✅ v2.1 Implemented

---

## Sub-documents

| File | Nội dung |
|------|---------|
| [business-logic.md](./business-logic.md) | SLA calculation, breach detection, override rules |
| [dataflow.md](./dataflow.md) | SLA assignment flow, daily breach check, alert flow |

---

## Services

| Service | Port | Role |
|---------|------|------|
| `sla-service` | 8086 | SLA config, deadline calc, daily breach cron |
| `finding-service` | 8085 | Stores `sla_expiration_date` per finding |
| `notification-service` | 8087 | Breach alerts |
| `audit-service` | 8090 | Log breach events |

---

## Default SLA Table

| Severity | Days to Fix |
|---------|------------|
| Critical | **7 days** |
| High | **30 days** |
| Medium | **90 days** |
| Low | **180 days** |
| Info | No SLA |

Override: Per-product SLA config có thể thay đổi các giá trị mặc định.

---

## NATS Events

| Event | Trigger | Subscribers |
|-------|---------|------------|
| `finding.sla.breached` | Daily cron detects overdue | notification-service → Email/Slack |
| `finding.sla.approaching` | N days before expiry | notification-service → reminder |

---

## Quick Reference: API Endpoints

| Method | Endpoint | Mô tả |
|--------|----------|-------|
| GET | `/api/v2/sla-configs` | List SLA configurations |
| POST | `/api/v2/sla-configs` | Create/update SLA config for product |
| GET | `/api/v2/findings/sla-breached` | List breached findings |
| GET | `/api/v2/products/{id}/sla-status` | SLA status overview per product |

---

## Database Schema (`osv_sla`)

| Table | Key Fields | Mô tả |
|-------|-----------|-------|
| `sla_configs` | id, product_id (nullable), severity, days | SLA rules |
| `sla_breaches` | id, finding_id, breached_at, notified_at | Breach log |
