# OSV Backend — Task Execution Index

> **Sprint mapping từ solutions**: [solutions/00_SOLUTIONS_INDEX.md](../solutions/00_SOLUTIONS_INDEX.md)  
> **Bug reports**: [../INDEX.md](../INDEX.md)  
> **Kiến trúc**: [specs/01-architecture.md](../../../01-architecture.md), [specs/02-technical-design.md](../../../02-technical-design.md)

---

## Phân Loại Tasks

### 🔴 SPRINT 1 — P0 (Unblock core, ~2 ngày)

| Task | File | Status | Bug | Mô tả |
|---|---|---|---|---|
| TASK-BE-P0-01 | [TASK-BE-P0-01_seed-opensearch.md](TASK-BE-P0-01_seed-opensearch.md) | ✅ DONE | BUG-BE-001 | Thêm `OPENSEARCH_ENABLED=true` vào docker-compose.server.yml; fallback Postgres GIN đã có sẵn trong usecase |
| TASK-BE-P0-02 | [TASK-BE-P0-02_scan-service-routes.md](TASK-BE-P0-02_scan-service-routes.md) | ✅ DONE | BUG-BE-002 | Thêm `ScanAPIHandler` + routes `/api/v1/scans` vào `delivery/http/router.go`; graceful empty response khi chưa wired DB |
| TASK-BE-P0-03 | [TASK-BE-P0-03_fix-findings-500.md](TASK-BE-P0-03_fix-findings-500.md) | ✅ DONE | BUG-BE-003 | Fix `FindingHandler.List` hỗ trợ `page`/`page_size` (spec) và `limit`/`offset` (legacy); fix div-by-zero; fix SLA OK clamp |
| TASK-BE-P0-04 | [TASK-BE-P0-04_fix-sla-dashboard.md](TASK-BE-P0-04_fix-sla-dashboard.md) | ✅ DONE | BUG-BE-004 | Fix `HandleDashboardSLA` trả 200 graceful empty state thay vì 503; `internal/sla-dashboard` đã có sẵn |

### 🟠 SPRINT 2 — P1 Schema Fixes (~3 ngày)

| Task | File | Status | Bug | Mô tả |
|---|---|---|---|---|
| TASK-BE-P1-01 | [TASK-BE-P1-01_auth-me-wrapper.md](TASK-BE-P1-01_auth-me-wrapper.md) | ✅ DONE | BUG-BE-005, 006 | Fix `Me` handler: wrap response trong `{user: {...}}` + thêm `name`, `email`, `mfa_enabled` fields; import `valueobject` |
| TASK-BE-P1-02 | [TASK-BE-P1-02_kev-schema-fix.md](TASK-BE-P1-02_kev-schema-fix.md) | ✅ DONE | BUG-BE-007 | Fix `ListKEV`: `entries`→`data`, `limit`→`page_size`; thêm `stats.added_last_30_days`, `stats.ransomware_related`; wrap `GetStats` |
| TASK-BE-P1-03 | [TASK-BE-P1-03_assets-products-schema.md](TASK-BE-P1-03_assets-products-schema.md) | ✅ DONE | BUG-BE-008 | Fix asset: `limit`→`page_size` + support `page_size` param; Fix product: `count`→`total`, `results`→`products`, add `page`/`page_size` |
| TASK-BE-P1-04 | [TASK-BE-P1-04_admin-schema-fix.md](TASK-BE-P1-04_admin-schema-fix.md) | ✅ DONE | BUG-BE-009 | Fix `HandleAdminHealth`: `services` là MAP (không phải array); Fix `UserDTO`: thêm `name` + `mfa_enabled` fields |
| TASK-BE-P1-05 | [TASK-BE-P1-05_webhooks-array.md](TASK-BE-P1-05_webhooks-array.md) | ✅ DONE | BUG-BE-012 | Fix `WebhookHandler.List`: trả JSON array trực tiếp (không wrap trong `{webhooks: [...]}`) |
| TASK-BE-P1-06 | [TASK-BE-P1-06_register-data-service-routes.md](TASK-BE-P1-06_register-data-service-routes.md) | ✅ DONE | BUG-BE-010 | Register EPSS/CWE/Vendor/Browse routes trong kev_router.go; init handlers trong embed/server.go |

### 🟠 SPRINT 3 — P1 Implement Missing (~5 ngày)

| Task | File | Status | Bug | Mô tả |
|---|---|---|---|---|
| TASK-BE-P1-07 | [TASK-BE-P1-07_epss-handler.md](TASK-BE-P1-07_epss-handler.md) | ✅ DONE | BUG-BE-010 | EPSSRepo (postgres/epss_repo.go): GetTopByEPSS + GetEPSSDistribution; router wired |
| TASK-BE-P1-08 | [TASK-BE-P1-08_vendor-browse-handler.md](TASK-BE-P1-08_vendor-browse-handler.md) | ✅ DONE | BUG-BE-010 | VendorRepo (postgres/vendor_repo.go): GetVendors/GetProductsByVendor/GetCVEsByVendorProduct; Browse handlers added |
| TASK-BE-P1-09 | [TASK-BE-P1-09_sla-config-schema.md](TASK-BE-P1-09_sla-config-schema.md) | ✅ DONE | BUG-BE-004 | Added GetConfig/UpdateConfig handlers; registered GET+PUT /api/v1/sla/config in sla-service main.go |

### 🟡 SPRINT 4 — P2 (~2 ngày)

| Task | File | Status | Bug | Mô tả |
|---|---|---|---|---|
| TASK-BE-P2-01 | [TASK-BE-P2-01_ai-service-config.md](TASK-BE-P2-01_ai-service-config.md) | ✅ DONE | BUG-BE-011 | deploy/.env: AI_BACKEND=ollama; docker-compose: thêm ollama service + ollama_data volume; embed.go: mount full AI router; chain.go: HasAvailableProvider(); handler.go: isReady() graceful degradation |

---

## Dependency Graph

```
TASK-BE-P0-01 (search-service seed)
    └── không phụ thuộc — standalone config

TASK-BE-P0-02 (scan-service routes)
    └── không phụ thuộc — standalone code

TASK-BE-P0-03 (findings-500)
    └── không phụ thuộc — finding-service fix

TASK-BE-P0-04 (sla-dashboard)
    └── TASK-BE-P0-03 (finding-service internal endpoint)

TASK-BE-P1-01 (auth-me)
    └── không phụ thuộc — identity-service

TASK-BE-P1-02 (kev-schema)
    └── không phụ thuộc — data-service

TASK-BE-P1-03 (assets/products)
    └── không phụ thuộc — asset-service + finding-service

TASK-BE-P1-04 (admin)
    └── không phụ thuộc — gateway BFF + identity-service

TASK-BE-P1-05 (webhooks)
    └── không phụ thuộc — notification-service

TASK-BE-P1-06 (register routes)
    └── TASK-BE-P1-07 (EPSS handler phải có trước)
    └── TASK-BE-P1-08 (Vendor handler phải có trước)

TASK-BE-P1-07 (EPSS handler)
    └── không phụ thuộc

TASK-BE-P1-08 (Vendor handler)
    └── không phụ thuộc

TASK-BE-P1-09 (sla-config)
    └── không phụ thuộc

TASK-BE-P2-01 (ai-service)
    └── không phụ thuộc
```

---

## Quy Ước

- Mỗi task là **atomic** — AI thực thi từng task độc lập
- Mỗi task có **Acceptance Criteria** rõ ràng để verify
- File path dùng relative từ repo root `/Users/binhnt/Lab/sec/cve/osv.dev/`
- Test bằng `curl` command có sẵn trong mỗi task
