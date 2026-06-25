# F08 — Scan Import Pipeline

> **Spec Folder:** `specs/features/f08-scan-import/`  
> **Feature Doc:** [`docs/features/F08-scan-import.md`](../../../docs/features/F08-scan-import.md)  
> **SRS Refs:** FR-04-06  
> **Status:** ✅ v2.1 Implemented

---

## Sub-documents

| File | Nội dung |
|------|---------|
| [business-logic.md](./business-logic.md) | 12-step pipeline, parser factory, dedup algorithms |
| [dataflow.md](./dataflow.md) | Import flow, dedup flow, NATS events |

---

## Services

| Service | Port | Role |
|---------|------|------|
| `scan-service` | internal | Parse scan files, run dedup, create findings |
| `finding-service` | 8085 | Receive parsed findings, apply state machine |
| `audit-service` | 8090 | Log import events |

---

## Supported Tool Parsers (21+)

| Category | Tools |
|----------|-------|
| Network | Nmap XML, OpenVAS |
| Web | OWASP ZAP JSON/XML, Burp Suite |
| SAST | Bandit, Semgrep, Checkmarx, SonarQube |
| SCA | Trivy, Snyk, OWASP Dependency Check, Grype |
| Container | Trivy (container), Anchore |
| Cloud | Scout Suite, Prowler |
| Other | Generic CSV, JSON |

---

## Import Pipeline Steps

```
1.  Validate file format
2.  Detect tool type (auto-detect or explicit)
3.  Select parser (Parser Factory)
4.  Parse → raw findings list
5.  Normalize (common schema)
6.  Filter (remove info/noise if configured)
7.  Enrich with CVE data (từ data-service)
8.  Compute hash fingerprint per finding
9.  Dedup check (hash match in product)
10. Apply SLA deadline
11. Create findings in finding-service
12. Generate import summary
```

---

## Quick Reference: API Endpoints

| Method | Endpoint | Mô tả |
|--------|----------|-------|
| POST | `/api/v2/import-scan` | Import scan file (multipart/form-data) |
| GET | `/api/v2/import-scans` | List past imports |
| GET | `/api/v2/import-scans/{id}` | Import job status/result |

---

## Database Schema (`osv_scan`)

| Table | Key Fields | Mô tả |
|-------|-----------|-------|
| `import_jobs` | id, test_id, tool, status, file_name, created_count, dup_count, error_count | Import job tracking |
| `import_errors` | import_job_id, row, message | Per-row parse errors |
