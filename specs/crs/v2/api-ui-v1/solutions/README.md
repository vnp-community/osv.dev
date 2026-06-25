# Solutions — API UI v1 Hardcode Fix (CR-008 → CR-014)

> **Tạo:** 2026-06-19  
> **Cập nhật:** 2026-06-22 (sau API test thực tế trên `https://c12.openledger.vn`)  
> **Bối cảnh:** Backend solutions cho 7 Change Requests nhằm replace hardcode data trong frontend components.  
> **Kiến trúc tham chiếu:** [01-architecture.md](../../../../01-architecture.md) · [02-technical-design.md](../../../../02-technical-design.md)

---

## Danh sách Solutions — Trạng thái (API Test 2026-06-22 — Deploy 6)

| CR | Solution | Priority | Service(s) | Pattern | Status |
|---|---|---|---|---|---|
| [CR-012](../CR-012-system-settings-typed-schema-apikey-security.md) | [SOL-012](./SOL-012-apikey-security-system-settings.md) | 🔴 CRITICAL | identity-service + gateway | Security fix + Schema upgrade | ✅ **VERIFIED** |
| [CR-008](../CR-008-scan-stats-weekly-activity.md) | [SOL-008](./SOL-008-scan-stats-weekly-activity.md) | 🔴 HIGH | scan-service | 2 endpoints mới + Cache | ✅ **VERIFIED** |
| [CR-009](../CR-009-webhook-deliveries-hourly-stats.md) | [SOL-009](./SOL-009-webhook-deliveries-hourly-stats.md) | 🔴 HIGH | notification-service | DB table mới + 3 endpoints | ✅ **VERIFIED** |
| [CR-010](../CR-010-reports-templates-schema-update.md) | [SOL-010](./SOL-010-reports-templates-schema-update.md) | 🔴 HIGH | finding-service | Endpoint + Schema + MinIO | ✅ **IMPLEMENTED** |
| [CR-011](../CR-011-admin-rbac-matrix-user-schema.md) | [SOL-011](./SOL-011-admin-rbac-matrix-user-schema.md) | 🔴 HIGH | identity-service | Schema upgrade + Auto-lock | ✅ **VERIFIED** |
| [CR-013](../CR-013-public-stats-ai-triage-queue-schema.md) | [SOL-013](./SOL-013-public-stats-audit-log-params.md) | 🟡 HIGH | gateway BFF + audit-service | Public endpoint + Filter params | ✅ **IMPLEMENTED** |
| [CR-014](../CR-014-ai-triage-queue-human-decision.md) | [SOL-014](./SOL-014-ai-triage-queue-human-decision.md) | 🔴 HIGH | ai-service | DB migration + Full schema | ✅ **IMPLEMENTED** |

### Kết quả Test Suite (2026-06-22 — Deploy 6)

| Solution | Endpoints | Kết quả Test |
|---|---|---|
| SOL-008 | `GET /scans/stats` + `/scans/stats/weekly` | ✅ 6/7 pass |
| SOL-009 | `GET /webhooks/deliveries` + retry + hourly | ✅ 9/12 pass (3 skip: need SAMPLE_IDs) |
| SOL-010 | `GET /reports/templates` + `/reports` + `POST /reports` | ✅ IMPLEMENTED (no dedicated test) |
| SOL-011 | `/admin/roles` (RBAC) + `/admin/users` (lock) | ✅ roles + users pass, lock/unlock OK |
| SOL-012 | `GET/POST/DELETE /api-keys` + `/admin/settings` | ✅ 23/25 pass (2 skip) |
| SOL-013 | `GET /api/v2/public/stats` + audit filters | ✅ IMPLEMENTED |
| SOL-014 | `GET /ai/triage/queue` + human decision | ✅ IMPLEMENTED (Redis-based) |
| **Tổng** | **44 tests** | **38 pass / 0 fail / 6 skip — 86.4%** |

---

## Thứ tự Thực thi Khuyến nghị

```
Phase 1 — CRITICAL (fix ngay):
  └─ SOL-012: API Key Security (crypto/rand) + Settings schema

Phase 2 — Blocking UI (sprint hiện tại):
  ├─ SOL-008: Scan Stats + Weekly Activity
  ├─ SOL-009: Webhook Deliveries + Hourly Stats
  ├─ SOL-011: RBAC Matrix + User Schema (auto-lock)
  └─ SOL-014: AI Triage Queue (full human decision schema)

Phase 3 — Degraded UX (sprint sau):
  ├─ SOL-010: Reports Templates + Schema update
  └─ SOL-013: Public Stats BFF + Audit Log params
```

---

## Phân loại theo Service

### `identity-service:8081`
- **SOL-012:** `POST /api/v1/api-keys` backend generate (CRITICAL) + Settings typed schema
- **SOL-011:** `GET /api/v1/admin/roles` → RBACMatrixResponse + AdminUser lock fields

### `scan-service:8084`
- **SOL-008:** `GET /api/v1/scans/stats` + `GET /api/v1/scans/stats/weekly`

### `notification-service:8087`
- **SOL-009:** `GET /api/v1/webhooks/deliveries` + retry + hourly stats

### `finding-service:8085`
- **SOL-010:** `GET /api/v1/reports/templates` + Report schema fields + MinIO upload

### `ai-service:9103`
- **SOL-014:** `GET /api/v1/ai/triage/queue` full schema + `POST /{id}/review` 409 handling
- **SOL-013:** (ref SOL-014)

### `apps/osv` (Gateway BFF)
- **SOL-013:** `GET /api/v2/public/stats` (no auth, cached)

### `audit-service:8090`
- **SOL-013:** Thêm `search`, `severity`, `date_from`, `date_to` params cho audit log

---

## Tổng hợp DB Migrations

| Migration | Service | Table | Thay đổi |
|---|---|---|---|
| `20260619_001_apikeys_security` | identity-service | `api_keys` | Thêm `status`, `created_by`, `prefix` |
| `20260619_002_users_lock` | identity-service | `users`, `roles` | Thêm `login_attempts`, `is_locked`, `locked_at`; `display_name`, `color` |
| `20260619_001_webhook_deliveries` | notification-service | `webhook_deliveries` | Bảng mới |
| `20260619_001_reports_schema` | finding-service | `reports` | Thêm `format`, `finding_count`, `file_size_bytes`, `generated_at`, `artifact_url`, `expires_at` |
| `20260619_001_triage_queue_human_decision` | ai-service | `triage_queue` | Thêm human decision columns + AI result columns |

---

## Tổng hợp Gateway Routes Mới

```go
// Phase 1 — Không cần routes mới (identity-service endpoints đã exist)

// Phase 2 — Routes mới cần thêm
mux.Handle("GET /api/v1/scans/stats/weekly",           protected(fwd("scan-service:8084")))
mux.Handle("GET /api/v1/scans/stats",                  protected(fwd("scan-service:8084")))
mux.Handle("GET /api/v1/webhooks/deliveries",          protected(fwd("notification-service:8087")))
mux.Handle("POST /api/v1/webhooks/deliveries/{id}/retry", protected(fwd("notification-service:8087")))
mux.Handle("GET /api/v1/webhooks/stats/hourly",        protected(fwd("notification-service:8087")))

// Phase 3 — Routes mới cần thêm
mux.Handle("GET /api/v1/reports/templates",            protected(fwd("finding-service:8085")))  // TRƯỚC /reports/{id}
mux.Handle("GET /api/v2/public/stats",                 rl("60/min")(publicBFF.HandlePublicStats)) // NO auth
```

> **⚠️ Route Ordering Quan trọng:**
> - `/api/v1/scans/stats/weekly` phải TRƯỚC `/api/v1/scans/stats`
> - `/api/v1/reports/templates` phải TRƯỚC `/api/v1/reports/{id}`
> - `/api/v1/ai/triage/queue` phải TRƯỚC `/api/v1/ai/triage/{findingId}`
> - `/api/v1/webhooks/deliveries` phải TRƯỚC `/api/v1/webhooks/{id}`
> - `/api/v2/public/stats` KHÔNG có `protected()` middleware

---

## Quan trọng: Architectural Decisions

### 1. Webhook Service Port
CR-009 tham chiếu `webhook-service:8089` — theo architecture spec không có port này. **Webhook management thuộc `notification-service:8087`.** Nếu team quyết định tách ra service riêng sau này, chỉ cần update port trong gateway route.

### 2. Scan Stats Port Typo
CR-008 ghi `scan-service:8083` nhưng port `8083` là `search-service`. **Port đúng của `scan-service` là `8084`.**

### 3. Public Stats — Graceful Degradation
`GET /api/v2/public/stats` PHẢI trả `200 OK` ngay cả khi một upstream service down. Mỗi sub-query có fallback về 0 hoặc static value.

### 4. AI Triage — CR-013 vs CR-014
CR-013 §2.2 và CR-014 cùng mô tả AI Triage queue schema. **SOL-014 là implementation chính.** CR-013 chỉ tham chiếu CR-014.

### 5. API Key Security — Breaking Change
Response field rename (`api_key` → `key`, `secret` → `raw_key`) là **breaking change**. Frontend đã được update. Backend cần match theo. Không cần backward compat vì CR spec xác nhận frontend đã update.
