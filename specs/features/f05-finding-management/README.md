# F05 — Finding Lifecycle Management

> **Spec Folder:** `specs/features/f05-finding-management/`  
> **Feature Doc:** [`docs/features/F05-finding-management.md`](../../../docs/features/F05-finding-management.md)  
> **SRS Refs:** FR-04-01 → FR-04-09  
> **Status:** ✅ v2.1 Implemented

---

## Sub-documents

| File | Nội dung |
|------|---------|
| [business-logic.md](./business-logic.md) | State machine, dedup, grading, risk acceptance, SLA |
| [dataflow.md](./dataflow.md) | Create/update/close finding flows, bulk ops, NATS events |

---

## Services

| Service | Port | Role |
|---------|------|------|
| `finding-service` | 8085 | CRUD findings, state machine, grading, risk acceptance |
| `sla-service` | 8086 | SLA deadline calculation, breach detection |
| `scan-service` | internal | Scan import → create findings |
| `audit-service` | 8090 | Record every state change |
| `notification-service` | 8087 | SLA breach alerts, state change alerts |

---

## Finding State Machine

```
                    ┌──────────────┐
                    │    Active    │◄────────────────────────┐
                    └──────┬───────┘                         │
                           │  transitions:                   │ reopen
          ┌────────────────┼────────────────────────┐        │
          ▼                ▼                         ▼        │
   ┌─────────────┐ ┌──────────────┐  ┌─────────────┐  ┌───────────┐
   │  Mitigated  │ │FalsePositive │  │ RiskAccepted│  │OutOfScope │
   └─────────────┘ └──────────────┘  └─────────────┘  └───────────┘
                                                               │
              ┌────────────────────────────────────────────────┘
              │  auto-detected on create
              ▼
        ┌───────────┐
        │ Duplicate │  (no manual transitions allowed)
        └───────────┘

Priority (for dedup/display): Duplicate > FalsePositive > OutOfScope > RiskAccepted > Mitigated > Active
```

---

## Quick Reference: API Endpoints

| Method | Endpoint | Mô tả |
|--------|----------|-------|
| GET | `/api/v2/findings` | List findings (filtered) |
| POST | `/api/v2/findings` | Create finding |
| GET | `/api/v2/findings/{id}` | Finding detail |
| PATCH | `/api/v2/findings/{id}` | Update finding |
| POST | `/api/v2/findings/{id}/close` | Close (Mitigate) |
| POST | `/api/v2/findings/{id}/reopen` | Reopen |
| POST | `/api/v2/findings/bulk` | Bulk operations |
| GET | `/api/v2/risk-acceptances` | List risk acceptances |
| POST | `/api/v2/risk-acceptances` | Create risk acceptance |
| GET | `/api/v2/products/{id}/grade` | Product security grade |

---

## NATS Events

| Event | Trigger | Subscribers |
|-------|---------|------------|
| `finding.state.changed` | Any state transition | audit-service, notification-service |
| `finding.sla.breached` | Daily cron detects breach | sla-service → notification-service |
| `finding.duplicate.detected` | Dedup on create | audit-service |
| `risk.acceptance.expired` | Cron detects expiry | finding-service (auto-reopen) |

---

## Database Schema (`osv_findings`)

| Table | Key Fields | Mô tả |
|-------|-----------|-------|
| `findings` | id, test_id, cve_id, title, severity, state, hash_code, sla_expiration | Core finding |
| `finding_notes` | id, finding_id, note, author, created_at | Notes log |
| `risk_acceptances` | id, product_id, expiration_date, reason | Risk acceptance records |
| `risk_acceptance_findings` | risk_acceptance_id, finding_id | Junction table |
