# API Seed Change Requests

> **Cập nhật:** 2026-06-18  
> **Mục đích:** Tổng hợp các Change Requests cho phép client seed dữ liệu vào hệ thống thông qua API Gateway.  
> **Nguồn phân tích:** `specs/models/` (data models) + `specs/backend_api_specs.md` (API hiện tại)

---

## Vấn đề hiện tại

Hệ thống hiện tại **không hỗ trợ seeding dữ liệu từ client** vì:

1. **Thiếu bulk endpoints** — Hầu hết domain chỉ có single-item create, không có batch/bulk.
2. **Thiếu import endpoints** — Không có API nào cho phép upload file JSON/CSV để import hàng loạt.
3. **Thiếu write endpoints** cho một số domain — Ví dụ: assets chỉ được tạo từ scan results, không thể tạo thủ công.
4. **Gateway routing gaps** — Một số endpoints tồn tại ở service level nhưng chưa được expose qua gateway.
5. **Thiếu admin-level creation** — Admin không thể tạo users trực tiếp hoặc tạo API keys cho user khác.

---

## Danh sách Change Requests

| CR | Domain | Priority | Mô tả |
|----|--------|----------|-------|
| [SEED-001](./SEED-001-identity-bootstrap.md) | identity-service | 🔴 CRITICAL | Seed users, bulk create, API keys, role assignments |
| [SEED-002](./SEED-002-products-hierarchy-seed.md) | finding-service / product-service | 🔴 CRITICAL | Seed ProductTypes, Products, Engagements, Tests — bulk & import |
| [SEED-003](./SEED-003-findings-seed.md) | finding-service | 🔴 CRITICAL | Bulk create findings, import từ JSON/CSV, bulk update |
| [SEED-004](./SEED-004-cve-data-seed.md) | data-service / ranking-service | 🟠 HIGH | Custom CVE records, bulk triage, CPE ranking bulk |
| [SEED-005](./SEED-005-assets-scan-seed.md) | scan-service / asset-service | 🟠 HIGH | Tạo assets thủ công, bulk create, register agents, scheduled scans |
| [SEED-006](./SEED-006-config-seed.md) | notification / sla / jira | 🟡 MEDIUM | Bulk create SLA configs, notification rules, subscriptions, JIRA configs |

---

## Thứ tự triển khai (Dependency Order)

```
SEED-001 (Identity)
  │
  ├─→ SEED-002 (Products Hierarchy)
  │     │
  │     └─→ SEED-003 (Findings)
  │
  ├─→ SEED-004 (CVE Data)  [độc lập với products]
  │
  ├─→ SEED-005 (Assets & Scans)  [độc lập với products]
  │
  └─→ SEED-006 (Configs)
        │
        └─ Depends on SEED-001 (users) + SEED-002 (products)
```

**Recommended seed order cho full demo dataset:**
1. SEED-001: Tạo admin + regular users
2. SEED-004: Import CVE data (background, có thể parallel)
3. SEED-002: Tạo ProductTypes → Products → Engagements → Tests
4. SEED-005: Tạo assets (bulk từ CMDB export)
5. SEED-006: Cấu hình SLA, notification rules, JIRA
6. SEED-003: Import findings (bulk hoặc từ file)

---

## Tổng hợp endpoints mới cần thêm

### Gateway routes cần thêm

| Method | Path | Target Service | Auth |
|--------|------|---------------|------|
| `POST` | `/api/v1/admin/users` | identity-service | Admin |
| `POST` | `/api/v1/admin/users/bulk` | identity-service | Admin |
| `GET` | `/api/v1/admin/users/{id}` | identity-service | Admin |
| `POST` | `/api/v1/admin/users/{id}/api-keys` | identity-service | Admin |
| `POST` | `/api/v1/admin/users/{id}/roles` | identity-service | Admin |
| `POST` | `/api/v2/product-types/bulk` | finding-service | Maintainer+ |
| `POST` | `/api/v2/products/bulk` | finding-service | Maintainer+ |
| `POST` | `/api/v2/products/{id}/seed` | finding-service | Writer+ |
| `POST` | `/api/v2/products/import` | finding-service | Admin |
| `POST` | `/api/v2/findings/bulk-create` | finding-service | Writer+ |
| `POST` | `/api/v2/findings/import` | finding-service | API Importer+ |
| `POST` | `/api/v2/cve/custom` | data-service | `ingest:admin` |
| `PUT` | `/api/v2/cve/{id}/triage` | data-service | Maintainer+ |
| `POST` | `/api/v2/cve/bulk-triage` | data-service | Maintainer+ |
| `POST` | `/api/v2/cve/import` | data-service | Admin |
| `POST` | `/api/v1/ranking/bulk` | ranking-service | Admin |
| `POST` | `/api/v1/assets` | asset-service | Writer+ |
| `POST` | `/api/v1/assets/bulk` | asset-service | Writer+ |
| `POST` | `/api/v1/assets/import` | asset-service | Writer+ |
| `DELETE` | `/api/v1/assets/{id}` | asset-service | Maintainer+ |
| `POST` | `/api/v1/assets/{id}/vulnerabilities` | asset-service | Writer+ |
| `POST` | `/api/v1/agents` | scan-service | Admin |
| `GET` | `/api/v1/agents` | scan-service | Authenticated |
| `GET` | `/api/v1/agents/{id}` | scan-service | Authenticated |
| `POST` | `/api/v1/agents/{id}/reports` | scan-service | `scan:execute` |
| `GET` | `/api/v1/scans/scheduled` | scan-service | Authenticated |
| `POST` | `/api/v1/scans/scheduled` | scan-service | Authenticated |
| `GET` | `/api/v1/scans/scheduled/{id}` | scan-service | Authenticated |
| `PUT` | `/api/v1/scans/scheduled/{id}` | scan-service | Authenticated |
| `DELETE` | `/api/v1/scans/scheduled/{id}` | scan-service | Authenticated |
| `POST` | `/api/v2/sla-configurations/bulk` | sla-service | Admin |
| `POST` | `/api/v2/sla-configurations/assign-bulk` | sla-service | Admin |
| `POST` | `/api/v2/notification-rules/bulk` | notification-service | Authenticated |
| `POST` | `/api/v2/subscriptions/bulk` | notification-service | Authenticated |
| `POST` | `/api/v2/jira-configurations/bulk` | jira-service | Admin |
| `POST` | `/api/v2/webhooks/bulk` | notification-service | Authenticated |

---

## Design Patterns chung

Tất cả bulk/import endpoints tuân theo conventions sau:

### Bulk Create Response — `207 Multi-Status`

```json
{
  "created_count": 8,
  "failed_count": 2,
  "results": [
    { "status": "created", "id": "uuid" },
    { "status": "error",   "message": "Validation error: name required" }
  ]
}
```

### Import from File

- `Content-Type: multipart/form-data`
- File formats hỗ trợ: `json`, `csv`
- File size limits: 10MB (findings, assets), 50MB (CVE import)
- Response: `200 OK` với summary counts

### Authentication cho Seed APIs

| Scope | Cơ chế |
|-------|--------|
| Admin create/bulk | Role `admin` trong JWT |
| Service account | API key với scope `ingest:admin` hoặc `scan:execute` |
| Writer | Role ≥ `Writer` trong product scope |
| API Importer | Role ≥ `API Importer` |

---

## Ghi chú triển khai

> [!IMPORTANT]
> **Idempotency**: Các bulk endpoints nên hỗ trợ idempotency key (header `Idempotency-Key: uuid`) để seed script có thể retry an toàn.

> [!WARNING]
> **Rate limiting**: Gateway cần tăng rate limit cho bulk/import endpoints (ví dụ: 5 req/min thay vì 60 req/min cho single-item endpoints).

> [!NOTE]
> **Audit logging**: Mọi bulk create/import action phải được audit service ghi lại với `actor_type: "service"` nếu thực hiện bằng API key.
