# F09 — Reporting

> **Spec Folder:** `specs/features/f09-reporting/`  
> **Feature Doc:** [`docs/features/F09-reporting.md`](../../../docs/features/F09-reporting.md)  
> **Status:** ✅ v2.1 Implemented

---

## Sub-documents

| File | Nội dung |
|------|---------|
| [business-logic.md](./business-logic.md) | Report generation logic, formats, CI/CD integration |
| [dataflow.md](./dataflow.md) | Generate flow, storage, download flow |

---

## Services

| Service | Port | Role |
|---------|------|------|
| `finding-service` | 8085 | Report generation orchestration |
| MinIO/S3 | internal | Report artifact storage |
| `notification-service` | 8087 | Notify when report ready |

---

## Report Types

| Format | Content | Use Case |
|--------|---------|---------|
| PDF | Full vuln report, charts, executive summary | Management, audit |
| HTML | Interactive report | Developer review |
| CSV | Flat findings list | Data analysis, Excel |
| Excel (XLSX) | Multi-sheet: findings + stats | Security team |
| JSON | Machine-readable | CI/CD pipeline, integration |

---

## Quick Reference: API Endpoints

| Method | Endpoint | Mô tả |
|--------|----------|-------|
| POST | `/api/v2/reports/generate` | Request report generation |
| GET | `/api/v2/reports` | List reports |
| GET | `/api/v2/reports/{id}` | Report status |
| GET | `/api/v2/reports/{id}/download` | Download report file |

---

## CI/CD Integration

Report generation có thể trả về **exit code** dựa trên findings:

| Condition | Exit Code |
|-----------|----------|
| No Critical/High findings | 0 (success) |
| Has High findings | 1 (warning) |
| Has Critical findings | 2 (fail) |
