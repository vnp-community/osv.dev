# Change Requests — API UI v1 Hardcode Fix

**Tạo:** 2026-06-19  
**Nguồn:** [openapi.yaml](../../../ui/specs/openapi.yaml) — các thay đổi từ hardcode solutions  
**Bối cảnh:** Frontend đang có dữ liệu hardcode trong 14+ components. Sau khi fix, các components gọi API thực. Các CR này yêu cầu backend bổ sung endpoints/schemas còn thiếu.

---

## Danh sách Change Requests

| CR | Tiêu đề | Priority | Service(s) | Status |
|---|---|---|---|---|
| [CR-008](./CR-008-scan-stats-weekly-activity.md) | Scan Stats & Weekly Activity Endpoints | 🔴 HIGH | scan-service | New |
| [CR-009](./CR-009-webhook-deliveries-hourly-stats.md) | Webhook Deliveries & Hourly Stats | 🔴 HIGH | webhook-service | New |
| [CR-010](./CR-010-reports-templates-schema-update.md) | Reports Templates Endpoint & Schema Updates | 🔴 HIGH | finding-service | New |
| [CR-011](./CR-011-admin-rbac-matrix-user-schema.md) | Admin RBAC Matrix & User Schema Updates | 🔴 HIGH | identity-service | New |
| [CR-012](./CR-012-system-settings-typed-schema-apikey-security.md) | System Settings Typed Schema + API Key Security | 🔴 CRITICAL | identity-service | New |
| [CR-013](./CR-013-public-stats-ai-triage-queue-schema.md) | Public Stats Endpoint + AI Triage Queue Schema | 🟡 HIGH | gateway BFF, ai-service | New |
| [CR-014](./CR-014-ai-triage-queue-human-decision.md) | AI Triage Queue — Full Schema with Human Decision Fields | 🔴 HIGH | ai-service | New |

---

## Phân loại theo Service

### `scan-service`
- `GET /api/v1/scans/stats` — KPI stats (CR-008)
- `GET /api/v1/scans/stats/weekly` — weekly chart (CR-008)
- `ScansListResponse.stats` field (CR-008)

### `webhook-service`
- `GET /api/v1/webhooks/deliveries` + delivery logging (CR-009)
- `POST /api/v1/webhooks/deliveries/{id}/retry` (CR-009)
- `GET /api/v1/webhooks/stats/hourly` (CR-009)
- DB schema: `webhook_deliveries` table (CR-009)

### `finding-service`
- `GET /api/v1/reports/templates` (CR-010)
- `Report` schema: thêm `format`, `file_size_bytes`, `artifact_url`, ... (CR-010)
- `ReportsListResponse.last_generated_at` (CR-010)
- S3/MinIO artifact upload sau khi generate (CR-010)

### `identity-service`
- `GET /api/v1/admin/roles` → `RBACMatrixResponse` (CR-011)
- `AdminUser` schema: `login_attempts`, `is_locked` (CR-011)
- `GET /api/v1/admin/users` response wrap (CR-011)
- Auto-lock sau 5 failed logins (CR-011)
- `POST /api/v1/api-keys` — backend generate với `crypto/rand` (CR-012 **CRITICAL**)
- `APIKey` schema: `status`, `created_by` (CR-012)
- `CreateAPIKeyResponse`: `key` + `raw_key` (CR-012)
- `GET/PUT /api/v1/admin/settings` → `SystemSettings` typed (CR-012)

### `gateway BFF`
- `GET /api/v2/public/stats` — no auth, cached (CR-013)
- `GET /api/v1/audit-log?search=&severity=&date_from=&date_to=` params (CR-013)

### `ai-service`
- `GET /api/v1/ai/triage/queue` → `AITriageQueueResponse` full schema (CR-013, **CR-014**)
- `POST /api/v1/ai/triage/{id}/review` → thêm `note` field + 409 conflict handling (CR-014)
- DB migration: `triage_queue` — thêm `human_decision`, `human_note`, `reviewed_by`, `reviewed_at` (CR-014)
- Stats computation từ DB: `pending`, `accepted_today`, `avg_confidence`, `false_positive_rate` (CR-014)
- Edge cases: 409 Conflict khi re-review, `?force=true` override, 404 not found (CR-014)

---

## Phân loại theo Mức độ khẩn cấp

### 🔴 CRITICAL — Bảo mật (Fix ngay)
| Issue | CR | Mô tả |
|---|---|---|
| API Key frontend generation | CR-012 | `Math.random()` không phải CSPRNG — bất kỳ key nào đều có thể bị đoán |

### 🔴 HIGH — Blocking UI (gây crash hoặc hiển thị sai)
| Issue | CR | Mô tả |
|---|---|---|
| Scan Dashboard hardcode data | CR-008 | `Math.random()` trong chart, cần stats endpoint |
| Webhook delivery history | CR-009 | UI hiển thị empty state hoặc crash |
| Report templates | CR-010 | Dialog không load được templates |
| RBAC matrix | CR-011 | Permission matrix không hiển thị |

### 🟡 HIGH — Degraded UX
| Issue | CR | Mô tả |
|---|---|---|
| Public stats login page | CR-013 | Login page hiển thị "—" cho tất cả stats |
| AI triage human decision | CR-013 | Review button không có tác dụng |
| Settings untyped schema | CR-012 | Không có validation khi save settings |

---

## Implementation Order (khuyến nghị)

```
Phase 1 (ngay): CR-012 CRITICAL (API Key security)
Phase 2 (sprint hiện tại):
  - CR-008 (Scan Stats) — blocking UI crash
  - CR-009 (Webhook Deliveries) — blocking UI
  - CR-011 (RBAC Matrix) — blocking UI
  - CR-014 (AI Triage Queue) — blocking UI review buttons
Phase 3 (sprint sau):
  - CR-010 (Reports Templates)
  - CR-013 (Public Stats + minor AI Triage)
```

---

## Liên kết tới Solutions

| CR | Solution docs |
|---|---|
| CR-008 | [06_07_scan_history_dashboard.md](../../../ui/specs/bugs/hardcode/solutions/06_07_scan_history_dashboard.md) |
| CR-009 | [10_11_12_webhook_report_notification.md](../../../ui/specs/bugs/hardcode/solutions/10_11_12_webhook_report_notification.md) |
| CR-010 | [10_11_12_webhook_report_notification.md](../../../ui/specs/bugs/hardcode/solutions/10_11_12_webhook_report_notification.md) |
| CR-011 | [03_admin_rbac.md](../../../ui/specs/bugs/hardcode/solutions/03_admin_rbac.md) |
| CR-012 | [04_admin_settings.md](../../../ui/specs/bugs/hardcode/solutions/04_admin_settings.md) · [09_api_key_management.md](../../../ui/specs/bugs/hardcode/solutions/09_api_key_management.md) |
| CR-013 | [13_14_15_16_misc_and_setup.md](../../../ui/specs/bugs/hardcode/solutions/13_14_15_16_misc_and_setup.md) |
| CR-014 | [08_ai_triage.md](../../../ui/specs/bugs/hardcode/solutions/08_ai_triage.md) |
