# F11 — JIRA Integration

> **Spec Folder:** `specs/features/f11-jira-integration/`  
> **Feature Doc:** [`docs/features/F11-jira-integration.md`](../../../docs/features/F11-jira-integration.md)  
> **SRS Refs:** FR-08 series  
> **Status:** ✅ v2.1 Implemented

---

## Sub-documents

| File | Nội dung |
|------|---------|
| [business-logic.md](./business-logic.md) | Credential management, sync logic, webhook verification, bidirectional rules |
| [dataflow.md](./dataflow.md) | OSV→JIRA create, JIRA→OSV update, webhook flow |

---

## Services

| Service | Port | Role |
|---------|------|------|
| `jira-service` | 8088 | JIRA API client, credential store, bidirectional sync |
| `finding-service` | 8085 | Provides finding data, receives state updates |
| `audit-service` | 8090 | Log sync events |

---

## Key Features

- **AES-256-GCM encrypted** JIRA credentials at rest
- **Bidirectional sync:** OSV finding → JIRA issue, JIRA status → OSV finding state
- **HMAC-SHA256 webhook** verification cho JIRA incoming webhooks
- **Configurable field mapping** per project

---

## Quick Reference: API Endpoints

| Method | Endpoint | Mô tả |
|--------|----------|-------|
| GET/POST | `/api/v2/jira-configs` | JIRA server config management |
| POST | `/api/v2/jira-configs/{id}/test` | Test JIRA connectivity |
| POST | `/api/v2/findings/{id}/push-to-jira` | Push finding to JIRA |
| GET | `/api/v2/findings/{id}/jira-link` | Get linked JIRA issue |
| POST | `/api/v2/jira/webhook` | Receive JIRA webhook |

---

## Field Mapping (OSV → JIRA)

| OSV Field | JIRA Field |
|-----------|-----------|
| `finding.title` | `summary` |
| `finding.description` + CVE details | `description` |
| `finding.severity` | `priority` (Critical→P1, High→P2, ...) |
| `finding.cve_id` | Custom field `CVE-ID` |
| `finding.sla_expiration_date` | `duedate` |
| `finding.component_name` | Custom field `Component` |

---

## Status Mapping (JIRA → OSV)

| JIRA Status | OSV State |
|------------|----------|
| `Done` / `Resolved` | Mitigated |
| `Won't Fix` | RiskAccepted |
| `In Progress` / `Open` | Active (no change) |
| `False Positive` | FalsePositive |

---

## Database Schema (`osv_jira`)

| Table | Key Fields | Mô tả |
|-------|-----------|-------|
| `jira_configs` | id, product_id, server_url, encrypted_creds, project_key | JIRA server config |
| `jira_issues` | id, finding_id, jira_issue_key, jira_issue_url, synced_at | Issue links |
| `jira_sync_log` | id, finding_id, direction, status, error | Sync audit |
