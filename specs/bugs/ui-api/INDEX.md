# OSV Platform — Backend API Bug Index

> Phát hiện từ automated test suite chạy ngày **2026-06-19**  
> Server: **https://c12.openledger.vn**  
> Test script: `tests/client/run_all.py`  
> Kết quả: **37/105 passed (35.2%)**

---

## Tổng Quan

| Severity | Số lượng |
|---|---|
| 🔴 Critical (P0) | 4 |
| 🟠 High (P1) | 6 |
| 🟡 Medium (P2) | 2 |
| **Tổng** | **12** |

---

## Danh Sách Bug

| Bug ID | Severity | Endpoint bị ảnh hưởng | Vấn đề | File |
|---|---|---|---|---|
| [BUG-BE-001](BUG-BE-001_cve-search-500.md) | 🔴 Critical | `POST /api/v2/cves/search` | **500** — Elasticsearch index rỗng, search hoàn toàn không dùng được | [→](BUG-BE-001_cve-search-500.md) |
| [BUG-BE-002](BUG-BE-002_scans-404.md) | 🔴 Critical | `GET /api/v1/scans` | **404** — Route chưa register trong gateway | [→](BUG-BE-002_scans-404.md) |
| [BUG-BE-003](BUG-BE-003_findings-500.md) | 🔴 Critical | `GET /api/v1/findings` | **500** — DB query lỗi nội bộ | [→](BUG-BE-003_findings-500.md) |
| [BUG-BE-004](BUG-BE-004_sla-endpoints-broken.md) | 🔴 Critical | `GET /api/v1/sla/*`, `GET /api/v1/dashboard/sla` | **404/500** — SLA service parse error + route thiếu | [→](BUG-BE-004_sla-endpoints-broken.md) |
| [BUG-BE-005](BUG-BE-005_auth-me-missing-wrapper.md) | 🟠 High | `GET /api/v1/auth/me` | Schema sai: thiếu `{ "user": {...} }` wrapper | [→](BUG-BE-005_auth-me-missing-wrapper.md) |
| [BUG-BE-006](BUG-BE-006_logout-wrong-status-code.md) | 🟡 Medium | `POST /api/v1/auth/logout` | Trả **200** thay vì **204** | [→](BUG-BE-006_logout-wrong-status-code.md) |
| [BUG-BE-007](BUG-BE-007_kev-schema-mismatch.md) | 🟠 High | `GET /api/v2/kev`, `/kev/stats`, `/kev/ransomware` | Schema sai: `items`→`data`, thiếu `page_size`, `stats` | [→](BUG-BE-007_kev-schema-mismatch.md) |
| [BUG-BE-008](BUG-BE-008_assets-products-schema-mismatch.md) | 🟠 High | `GET /api/v1/assets`, `/products`, `/products/types` | Schema sai: thiếu wrapper object + types format sai | [→](BUG-BE-008_assets-products-schema-mismatch.md) |
| [BUG-BE-009](BUG-BE-009_admin-schema-mismatch.md) | 🟡 Medium | `GET /api/v1/admin/health`, `/admin/users`, `/admin/roles` | Schema sai: services array→map, thiếu `name`/`mfa_enabled`, field name sai | [→](BUG-BE-009_admin-schema-mismatch.md) |
| [BUG-BE-010](BUG-BE-010_missing-endpoints-404.md) | 🟠 High | 20+ endpoints | **404** — Chưa implement (vendors, CWE, CAPEC, EPSS, profile, reports...) | [→](BUG-BE-010_missing-endpoints-404.md) |
| [BUG-BE-011](BUG-BE-011_ai-service-503.md) | 🟡 Medium | `GET /api/v1/ai/*` | **503** — AI service không chạy (OPENAI_API_KEY trống) | [→](BUG-BE-011_ai-service-503.md) |
| [BUG-BE-012](BUG-BE-012_webhooks-wrong-format.md) | 🟡 Medium | `GET /api/v1/webhooks` | Response là object thay vì array | [→](BUG-BE-012_webhooks-wrong-format.md) |

---

## Phân Loại Theo Loại Lỗi

### 🔥 Service Down / 5xx Errors

| Endpoint | HTTP | Lỗi |
|---|---|---|
| `POST /api/v2/cves/search` | 500 | `{"error":"search failed"}` |
| `GET /api/v1/findings` | 500 | `{"error":"failed to list findings"}` |
| `GET /api/v1/dashboard/sla` | 500 | `{"error":"INTERNAL_ERROR","message":"Failed to parse SLA response"}` |
| `GET /api/v1/ai/triage/queue` | 503 | `{"error":"SERVICE_UNAVAILABLE"}` |
| `GET /api/v1/ai/enrichment` | 503 | AI service down |

### 🚧 Routes Chưa Register / 404

| Endpoint | Nhóm |
|---|---|
| `GET /api/v1/scans` | Scans |
| `GET /api/v1/sla/overview` | SLA |
| `GET /api/v1/findings/stats` | Findings |
| `GET /api/v2/vendors` | Taxonomy |
| `GET /api/v2/cwe` | Taxonomy |
| `GET /api/v2/capec/{id}` | Taxonomy |
| `GET /api/v2/epss/top` | EPSS |
| `GET /api/v1/profile` | Auth |
| `GET /api/v1/reports` | Reports |

### 📐 Schema Mismatch (HTTP 200 nhưng format sai)

| Endpoint | Vấn đề |
|---|---|
| `GET /auth/me` | Thiếu `{ "user": {...} }` wrapper |
| `POST /auth/logout` | 200 thay vì 204 |
| `GET /api/v2/kev` | `items`→`data`, thiếu `page_size`, `stats` |
| `GET /api/v1/assets` | Thiếu `{ "assets": [...], "total": N }` wrapper |
| `GET /api/v1/products` | Array trực tiếp thay vì wrapper |
| `GET /api/v1/products/types` | Object array thay vì string array |
| `GET /api/v1/admin/health` | `services` array thay vì object/map |
| `GET /api/v1/admin/users` | Thiếu `name`, `mfa_enabled` |
| `GET /api/v1/admin/roles` | Key sai: `role_name`→`name`, `perms`→`permissions` |
| `GET /api/v1/webhooks` | `{ "webhooks": [...] }` thay vì array trực tiếp |

---

## Roadmap Fix Ưu Tiên

```
P0 — Fix ngay (unblock core features)
  BUG-BE-001  Seed Elasticsearch index (set NVD_API_KEY + restart)
  BUG-BE-002  Register /scans routes trong gateway
  BUG-BE-003  Debug /findings 500 query
  BUG-BE-004  Fix /dashboard/sla 500 + add /sla/overview route

P1 — Sprint tiếp theo (schema alignment)
  BUG-BE-005  auth/me add user wrapper
  BUG-BE-007  KEV response schema: items→data, thêm page_size + stats
  BUG-BE-008  assets/products add wrapper objects
  BUG-BE-009  admin: health services, user fields, role fields
  BUG-BE-010  Implement missing endpoints (vendors, CWE, CAPEC, EPSS...)
  BUG-BE-012  webhooks return array directly

P2 — Backlog
  BUG-BE-006  logout 200→204
  BUG-BE-011  AI service (set OPENAI_API_KEY hoặc dùng Ollama)
```

---

## Nguồn

- Test output: [`tests/client/report/api_test_20260619_113635.log`](../../tests/client/report/api_test_20260619_113635.log)
- Crash report: [`tests/client/report/crash_report_20260619_113635.log`](../../tests/client/report/crash_report_20260619_113635.log)
- Full report: [`tests/client/report/api_report_20260619_113635.md`](../../tests/client/report/api_report_20260619_113635.md)
- UI bugs: [`ui/specs/bugs/ui-crash/crash.md`](../../ui/specs/bugs/ui-crash/crash.md)
