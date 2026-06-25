# Solutions Index — OSV Platform UI Crash Bugs

**Ngày tạo**: 2026-06-20  
**Scan method**: Playwright headless Chrome, client-side navigation  
**Tổng bugs**: 16 | **JS Crash**: 6 | **API 404**: 7 | **API 500**: 1 | **API 503**: 2

---

## Phân loại và solution files

### 🔴 Nhóm A — JS Runtime Crash (TypeError trong component)

| Bug | Route | Lỗi | Solution |
|-----|-------|-----|---------|
| BUG-001 | `/cve/cwe` | `undefined.map` trong CWELibrary | [SOL-001-cwe-library.md](SOL-001-cwe-library.md) |
| BUG-005 | `/assets` | `null.filter` trong AssetInventory | [SOL-005-assets.md](SOL-005-assets.md) |
| BUG-006 | `/products` | `undefined.flatMap` trong ProductSecurity | [SOL-006-products.md](SOL-006-products.md) |
| BUG-012 | `/integrations/webhooks` | `undefined.find` trong WebhookEvents | [SOL-012-webhooks.md](SOL-012-webhooks.md) |
| BUG-013 | `/admin/roles` | React Error #31 (object rendered as child) | [SOL-013-rbac.md](SOL-013-rbac.md) |
| BUG-015 | `/admin/health` | `undefined.includes` trong SystemHealth | [SOL-015-system-health.md](SOL-015-system-health.md) |

### 🟠 Nhóm B — API 404 (Backend endpoint chưa implement)

| Bug | Route | Missing endpoint | Solution |
|-----|-------|-----------------|---------|
| BUG-002 | `/scans` | `GET /api/v1/scans/stats/weekly` | [SOL-B-api-404.md](SOL-B-api-404.md) |
| BUG-004 | `/findings/risk-acceptance` | `GET /api/v1/findings/risk-acceptances` | [SOL-B-api-404.md](SOL-B-api-404.md) |
| BUG-009 | `/reports` | `GET /api/v1/reports`, `/api/v1/reports/templates` | [SOL-B-api-404.md](SOL-B-api-404.md) |
| BUG-010 | `/notifications` | `GET /api/v1/notifications` | [SOL-B-api-404.md](SOL-B-api-404.md) |
| BUG-011 | `/integrations/api-keys` | `GET /api/v1/api-keys` | [SOL-B-api-404.md](SOL-B-api-404.md) |
| BUG-014 | `/admin/audit` | `GET /api/v1/admin/audit-logs` | [SOL-B-api-404.md](SOL-B-api-404.md) |
| BUG-016 | `/admin/settings` | `GET /api/v1/admin/settings` | [SOL-B-api-404.md](SOL-B-api-404.md) |

### 🟡 Nhóm C — API 500/503 (Server error / Service down)

| Bug | Route | Lỗi | Solution |
|-----|-------|-----|---------|
| BUG-003 | `/findings` | `GET /api/v1/findings` → 500 | [SOL-C-server-errors.md](SOL-C-server-errors.md) |
| BUG-007 | `/ai/triage` | AI service 503 | [SOL-C-server-errors.md](SOL-C-server-errors.md) |
| BUG-008 | `/ai/enrichment` | AI service 503 | [SOL-C-server-errors.md](SOL-C-server-errors.md) |

---

## Ưu tiên sửa

```
P0 (Block release)  → BUG-013, BUG-003, BUG-005
P1 (Major UX bug)   → BUG-001, BUG-006, BUG-012, BUG-015
P2 (Service missing) → BUG-007, BUG-008
P3 (API stub needed) → BUG-002, BUG-004, BUG-009–011, BUG-014, BUG-016
```
