# API Seed Solutions

> **Cập nhật:** 2026-06-18  
> **Nguồn kiến trúc:** [01-architecture.md](../../../../../01-architecture.md)

Tổng hợp giải pháp kỹ thuật chi tiết để thực thi các Change Requests trong `api-seed/`.

---

## Danh sách Solutions

| Solution | CR | Domain | Files thay đổi |
|---------|-----|--------|----------------|
| [SOL-SEED-001](./SOL-SEED-001-identity-bootstrap.md) | SEED-001 | identity-service | 8 files + migration |
| [SOL-SEED-002](./SOL-SEED-002-products-hierarchy.md) | SEED-002 | finding-service | 9 files |
| [SOL-SEED-003](./SOL-SEED-003-findings-seed.md) | SEED-003 | finding-service | 9 files |
| [SOL-SEED-004](./SOL-SEED-004-cve-data.md) | SEED-004 | data-service + ranking-service | 10 files + migration |
| [SOL-SEED-005](./SOL-SEED-005-assets-scan.md) | SEED-005 | asset-service + scan-service | 8 files + migration |
| [SOL-SEED-006](./SOL-SEED-006-config-seed.md) | SEED-006 | sla + notification + jira | 6 files + shared helper |

---

## Kiến trúc nguyên tắc áp dụng

Tất cả solutions đều tuân theo **Clean Architecture 4 layers** (từ `01-architecture.md §6`):

```
Domain Layer        → Entity types, Repository interfaces mới
Use Case Layer      → UseCase mới hoặc mở rộng existing
Adapter Layer       → HTTP handlers + router registration
Infrastructure      → PostgreSQL repo implementations + migrations
Gateway Layer       → apps/osv/internal/gateway/router.go
```

---

## Shared Design Patterns

### 1. Bulk Response — 207 Multi-Status (RFC 4918)

Tất cả bulk endpoints đều trả về `207` với partial-failure support:

```json
{
  "created_count": 8,
  "failed_count":  2,
  "results": [
    { "key": "identifier", "status": "created", "id": "uuid" },
    { "key": "bad-item",   "status": "error",   "message": "validation error" }
  ]
}
```

**Implementation**: `services/shared/httputil/bulk_response.go` (SEED-006)

### 2. Route Ordering — Literal trước Wildcard

Trong cả `chi` router (services) và Go `net/http` ServeMux (gateway), **literal paths PHẢI đứng trước wildcard paths** để tránh shadowing:

```go
// ĐÚNG:
r.Post("/api/v2/findings/bulk-create", ...)  // literal
r.Post("/api/v2/findings/import",      ...)  // literal  
r.Post("/api/v2/findings/{id}/close",  ...)  // wildcard
```

### 3. Upsert Pattern cho Idempotency

Tất cả bulk create dùng `ON CONFLICT ... DO NOTHING` hoặc `DO UPDATE`:
- Asset: `ON CONFLICT (ip_address) DO UPDATE` (nếu `update_existing: true`)
- CVE: `ON CONFLICT (cve_id) DO UPDATE` (nếu `overwrite: true`)
- User: `ON CONFLICT (email) DO NOTHING` + report as "error"

### 4. File Import — Multipart

```go
// Max sizes:
// findings: 10MB
// CVE import: 50MB
// assets: 10MB

r.Body = http.MaxBytesReader(w, r.Body, maxBytes)
if err := r.ParseMultipartForm(maxBytes); err != nil {
    writeJSON(w, 413, errResp("payload_too_large", "..."))
}
```

### 5. NATS Events cho Audit Trail

Mọi bulk/import operation đều publish NATS event → `audit-service` ghi log:

```go
uc.eventPub.Publish("entity.batch_created", map[string]any{
    "count":    createdCount,
    "actor_id": actorID,
    "source":   "api_seed",  // phân biệt với automated fetchers
})
```

---

## Thứ tự triển khai (Dependencies)

```
SOL-SEED-001 (Identity — CRITICAL)
  │  Creates DB table role_assignments
  │
  ├─→ SOL-SEED-002 (Products Hierarchy — CRITICAL)
  │     │  Depends on: users exist (to set product members)
  │     │
  │     └─→ SOL-SEED-003 (Findings — CRITICAL)
  │           Depends on: product + test hierarchy exists
  │
  ├─→ SOL-SEED-004 (CVE Data — HIGH)
  │     │  Independent of products
  │     │  Creates DB table triage_entries
  │     │
  │     └─→ (ai-service auto-embeds new custom CVEs via NATS)
  │
  ├─→ SOL-SEED-005 (Assets & Scans — HIGH)
  │     Creates DB tables: assets, asset_vulnerabilities
  │
  └─→ SOL-SEED-006 (Config — MEDIUM)
        Depends on: products exist (SLA assign), users exist (notification rules)
```

---

## Tổng hợp Gateway Routes mới

Tổng cộng **~35 routes mới** được thêm vào `apps/osv/internal/gateway/router.go`:

### Identity (SEED-001) — 5 routes
```
POST /api/v1/admin/users                    → identity-service:8081  [adminOnly]
POST /api/v1/admin/users/bulk              → identity-service:8081  [adminOnly]
GET  /api/v1/admin/users/{id}              → identity-service:8081  [adminOnly]
POST /api/v1/admin/users/{id}/api-keys     → identity-service:8081  [adminOnly]
POST /api/v1/admin/users/{id}/roles        → identity-service:8081  [adminOnly]
```

### Products (SEED-002) — 4 routes
```
POST /api/v2/product-types/bulk            → finding-service:8085  [protected]
POST /api/v2/products/bulk                 → finding-service:8085  [protected]
POST /api/v2/products/import               → finding-service:8085  [adminOnly]
POST /api/v2/products/{id}/seed            → finding-service:8085  [protected]
```

### Findings (SEED-003) — 2 routes
```
POST /api/v2/findings/bulk-create          → finding-service:8085  [protected]
POST /api/v2/findings/import               → finding-service:8085  [protected]
```

### CVE Data (SEED-004) — 5 routes
```
POST /api/v2/cve/custom                    → data-service:8082    [adminOnly]
POST /api/v2/cve/bulk-triage               → data-service:8082    [protected]
POST /api/v2/cve/import                    → data-service:8082    [adminOnly]
PUT  /api/v2/cve/{id}/triage               → data-service:8082    [protected]
POST /api/v1/ranking/bulk                  → ranking-service      [adminOnly]
```

### Assets & Scans (SEED-005) — 14 routes
```
POST   /api/v1/assets                       → asset-service:8091   [protected]
POST   /api/v1/assets/bulk                  → asset-service:8091   [protected]
POST   /api/v1/assets/import                → asset-service:8091   [protected]
DELETE /api/v1/assets/{id}                  → asset-service:8091   [protected]
POST   /api/v1/assets/{id}/vulnerabilities  → asset-service:8091   [protected]
POST   /api/v1/agents                       → scan-service:8084    [adminOnly]
GET    /api/v1/agents                       → scan-service:8084    [protected]
GET    /api/v1/agents/{id}                  → scan-service:8084    [protected]
POST   /api/v1/agents/{id}/reports          → scan-service:8084    [protected]
POST   /api/v1/scans/scheduled              → scan-service:8084    [protected]
GET    /api/v1/scans/scheduled              → scan-service:8084    [protected]
GET    /api/v1/scans/scheduled/{id}         → scan-service:8084    [protected]
PUT    /api/v1/scans/scheduled/{id}         → scan-service:8084    [protected]
DELETE /api/v1/scans/scheduled/{id}         → scan-service:8084    [protected]
```

### Config (SEED-006) — 6 routes
```
POST /api/v2/sla-configurations/bulk         → sla-service:8086         [adminOnly]
POST /api/v2/sla-configurations/assign-bulk  → sla-service:8086         [adminOnly]
POST /api/v2/notification-rules/bulk         → notification-service:8087 [protected]
POST /api/v2/subscriptions/bulk              → notification-service:8087 [protected]
POST /api/v2/webhooks/bulk                   → notification-service:8087 [protected]
POST /api/v2/jira-configurations/bulk        → jira-service:8088        [adminOnly]
```

---

## Database Migrations cần chạy

| Migration file | Service | Nội dung |
|--------------|---------|---------|
| `migrations/identity/0010_admin_user_seed.sql` | identity-service | `role_assignments` table |
| `migrations/data/0015_triage_entries.sql` | data-service | `triage_entries` table |
| `migrations/asset/0001_init.sql` | asset-service | `assets`, `asset_vulnerabilities` tables + GIN index |
