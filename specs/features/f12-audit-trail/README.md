# F12 — Audit Trail

> **Spec Folder:** `specs/features/f12-audit-trail/`  
> **Feature Doc:** [`docs/features/F12-audit-trail.md`](../../../docs/features/F12-audit-trail.md)  
> **SRS Refs:** FR-04-04  
> **Status:** ✅ v2.1 Implemented

---

## Sub-documents

| File | Nội dung |
|------|---------|
| [business-logic.md](./business-logic.md) | Append-only model, HMAC integrity, RLS, event types |
| [dataflow.md](./dataflow.md) | Event ingestion flow, query flow, verification flow |

---

## Services

| Service | Port | Role |
|---------|------|------|
| `audit-service` | 8090 | Subscribe 40+ NATS events, append audit log, integrity verification |

---

## Design Principles

- **Immutable:** Không có UPDATE hoặc DELETE trên `audit_events`
- **HMAC-SHA256 per event:** Mỗi record có signature để phát hiện tampering
- **Row-Level Security:** PostgreSQL RLS — chỉ `audit_writer` role được INSERT; `audit_reader` chỉ được SELECT
- **40+ event types:** Coverage toàn bộ platform

---

## Event Types (40+)

| Category | Events |
|----------|--------|
| Auth | `user.login`, `user.logout`, `user.locked`, `api_key.created`, `api_key.revoked` |
| Finding | `finding.created`, `finding.state_changed`, `finding.duplicate_detected` |
| SLA | `sla.breached`, `sla.approaching`, `sla_config.updated` |
| Product | `product.created`, `product.member_added`, `product.member_removed` |
| JIRA | `jira.issue_created`, `jira.sync_failed` |
| KEV | `kev.cve_added` |
| Import | `import.completed`, `import.failed` |
| System | `service.started`, `config.changed` |

---

## Quick Reference: API Endpoints

| Method | Endpoint | Mô tả |
|--------|----------|-------|
| GET | `/api/v2/audit-log` | Query audit events (filtered) |
| GET | `/api/v2/audit-log/{id}` | Single event detail |
| POST | `/api/v2/audit-log/verify` | Verify HMAC integrity of events |

---

## Database Schema (`osv_audit`)

| Table | Key Fields | Mô tả |
|-------|-----------|-------|
| `audit_events` | id, event_type, entity_type, entity_id, action, before (JSONB), after (JSONB), user_id, timestamp, hmac_sha256 | Immutable event log |
