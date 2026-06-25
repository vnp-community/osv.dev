# Tasks — Backend Implementation (CR-008 → CR-014)

> **Từ solutions:** [`../solutions/`](../solutions/)  
> **Tổng:** 8 tasks độc lập, thực thi theo thứ tự ưu tiên  
> **Tạo:** 2026-06-19  

---

## Danh sách Tasks theo Priority

| Task | CR | Service | Priority | Phụ thuộc | Status |
|---|---|---|---|---|---|
| [TASK-CR012-P0-01](./TASK-CR012-P0-01_apikey-backend-generate.md) | CR-012 | identity-service | 🔴 P0 CRITICAL | Không có | ✅ DONE |
| [TASK-CR012-P1-02](./TASK-CR012-P1-02_system-settings-typed-schema.md) | CR-012 | identity-service | 🔴 P1 | Không có | ✅ DONE |
| [TASK-CR008-P1-03](./TASK-CR008-P1-03_scan-stats-weekly.md) | CR-008 | scan-service | 🔴 P1 | Không có | ✅ DONE |
| [TASK-CR011-P1-04](./TASK-CR011-P1-04_admin-rbac-user-lock.md) | CR-011 | identity-service | 🔴 P1 | Không có | ✅ DONE |
| [TASK-CR014-P1-05](./TASK-CR014-P1-05_ai-triage-queue-schema.md) | CR-014 | ai-service | 🔴 P1 | Không có | ✅ DONE |
| [TASK-CR009-P1-06](./TASK-CR009-P1-06_webhook-deliveries-endpoints.md) | CR-009 | notification-service | 🔴 P1 | Không có | ✅ DONE |
| [TASK-CR010-P2-07](./TASK-CR010-P2-07_reports-templates-schema.md) | CR-010 | finding-service | 🟡 P2 | MinIO phải chạy | ✅ DONE |
| [TASK-CR013-P2-08](./TASK-CR013-P2-08_public-stats-audit-filters.md) | CR-013 | gateway BFF + audit | 🟡 P2 | TASK-CR008-P1-03 | ✅ DONE |

---

## Thứ tự thực thi khuyến nghị

```
PHASE 1 — CRITICAL (chạy ngay):
  ① TASK-CR012-P0-01  ← API Key security (Math.random() → crypto/rand)
  ② TASK-CR012-P1-02  ← System Settings typed schema

PHASE 2 — Blocking (sprint hiện tại, song song OK):
  ③ TASK-CR008-P1-03  ← Scan Stats + Weekly chart
  ④ TASK-CR011-P1-04  ← RBAC Matrix + User auto-lock
  ⑤ TASK-CR014-P1-05  ← AI Triage Queue + Human Decision
  ⑥ TASK-CR009-P1-06  ← Webhook Deliveries + Hourly Stats

PHASE 3 — Degraded UX (sprint sau):
  ⑦ TASK-CR010-P2-07  ← Reports Templates + Schema + MinIO
  ⑧ TASK-CR013-P2-08  ← Public Stats BFF + Audit Log Filters
```

---

## Cấu trúc mỗi Task

Mỗi task file có cấu trúc chuẩn:
1. **Header** — CR, solution ref, priority, dependency, status
2. **Điều tra trước khi code** — shell commands để xác minh state hiện tại
3. **Bước thực thi** — code + file path đầy đủ
4. **Acceptance Criteria** — checklist để verify
5. **Verification** — curl commands để test

---

## DB Migrations Tổng hợp

| Migration file | Service | Task |
|---|---|---|
| `20260619_001_apikeys_security.sql` | identity-service | TASK-CR012-P0-01 |
| `20260619_002_system_settings.sql` | identity-service | TASK-CR012-P1-02 |
| `20260619_002_users_lock.sql` | identity-service | TASK-CR011-P1-04 |
| `20260619_001_triage_queue_human_decision.sql` | ai-service | TASK-CR014-P1-05 |
| `20260619_001_webhook_deliveries.sql` | notification-service | TASK-CR009-P1-06 |
| `20260619_001_reports_schema.sql` | finding-service | TASK-CR010-P2-07 |

> **Lưu ý:** Trong `identity-service` có 2 migrations đánh số `_001` và `_002` — cần đảm bảo thứ tự chạy đúng:
> 1. `20260619_001_apikeys_security.sql` (api_keys table)
> 2. `20260619_002_system_settings.sql` (system_settings table)
> 3. `20260619_002_users_lock.sql` (users + roles table) ← đánh lại số nếu cần

---

## Gateway Routes Cần Thêm

```go
// Phase 1 — Verify existing routes
// /api/v1/api-keys (GET, POST, DELETE) → identity-service:8081
// /api/v1/admin/settings (GET, PUT) → identity-service:8081

// Phase 2 — NEW routes
// /api/v1/scans/stats/weekly    → scan-service:8084      (TRƯỚC /scans/stats)
// /api/v1/scans/stats           → scan-service:8084
// /api/v1/admin/users/{id}/unlock → identity-service:8081
// /api/v1/ai/triage/queue       → ai-service:9103        (TRƯỚC /{findingId})
// /api/v1/ai/triage/{findingId}/review → ai-service:9103
// /api/v1/webhooks/deliveries   → notification-service:8087  (TRƯỚC /{id})
// /api/v1/webhooks/deliveries/{id}/retry → notification-service:8087
// /api/v1/webhooks/stats/hourly → notification-service:8087

// Phase 3 — NEW routes
// /api/v1/reports/templates     → finding-service:8085   (TRƯỚC /reports/{id})
// /api/v2/public/stats          → gateway BFF (NO auth)
```

> ⚠️ **Route Ordering Critical:** Luôn đăng ký literal paths TRƯỚC wildcard `/{id}` patterns.

---

## Common Patterns (Tham khảo khi code)

### Parse int với default
```go
func parseIntDefault(s string, def int) int {
    if s == "" { return def }
    n, err := strconv.Atoi(s)
    if err != nil || n <= 0 { return def }
    return n
}
```

### Respond JSON error
```go
func respondError(w http.ResponseWriter, status int, msg string) {
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(status)
    json.NewEncoder(w).Encode(map[string]string{"error": msg})
}
```

### Extract reviewer từ JWT headers (injected bởi gateway)
```go
reviewer := r.Header.Get("X-User-Email")
if reviewer == "" {
    reviewer = r.Header.Get("X-User-ID")
}
```

### Path variable (Go 1.22+ ServeMux)
```go
id := r.PathValue("id")
// Hoặc chi router:
id := chi.URLParam(r, "id")
```
