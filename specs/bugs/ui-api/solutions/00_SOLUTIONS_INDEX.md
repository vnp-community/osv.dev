# OSV Platform — Bug Solutions Index

> **Phiên bản**: 1.0  
> **Tạo**: 2026-06-19  
> **Tham chiếu kiến trúc**: [01-architecture.md](../../01-architecture.md), [02-technical-design.md](../../02-technical-design.md)  
> **Bug reports**: [../INDEX.md](../INDEX.md)

---

## Nguyên Tắc Thiết Kế

Tất cả giải pháp tuân thủ:
1. **Clean Architecture**: Fix chỉ ở **Adapter layer** (HTTP handlers) — không chạm domain/usecase
2. **Dependency Rule**: Outer → Inner (Infra → Adapter → UseCase → Domain)
3. **Không sửa `apps/osv/internal/gateway/router.go`** trừ khi thực sự cần — routes đã đúng
4. **Graceful Degradation**: Khi service unavailable, trả 200 với empty data thay vì 503

---

## Danh Sách Solutions

| Solution | Bugs giải quyết | Service | Priority | Effort |
|---|---|---|---|---|
| [SOL-001](SOL-001_fix-cve-search-500.md) | BUG-BE-001 | search-service + data-service | P0 | 2–4h |
| [SOL-002](SOL-002_fix-scans-404.md) | BUG-BE-002 | scan-service | P0 | 4–8h |
| [SOL-003](SOL-003_fix-findings-sla-500.md) | BUG-BE-003, BUG-BE-004 | finding-service + sla-service + gateway BFF | P0 | 4–6h |
| [SOL-004](SOL-004_fix-schema-mismatches.md) | BUG-BE-005, 006, 007, 008, 009, 012 | identity/data/finding/asset/notification services | P1 | 6–10h |
| [SOL-005](SOL-005_implement-missing-endpoints.md) | BUG-BE-010 | data-service | P1 | 16–24h |
| [SOL-006](SOL-006_fix-ai-service-503.md) | BUG-BE-011 | ai-service | P2 | 1–2h config |

---

## Phân Tích Tác Động

### ✅ Gateway Router (`apps/osv/internal/gateway/router.go`)

Tất cả 150+ routes **đã được mount đúng** (đã xác nhận từ code). Gateway **KHÔNG cần sửa** ngoại trừ:
- `SOL-003`: Cập nhật `NewDashboardBFF()` để thêm `slaSvcAddr` parameter

### 🔧 Services cần fix

| Service | Files cần sửa | Loại fix |
|---|---|---|
| `data-service` | `kev_handler.go`, router | Schema + Route registration |
| `identity-service` | `handlers.go` | Thêm `/auth/me`, fix wrapper |
| `finding-service` | product/finding handlers, internal SLA endpoint | Schema + Internal endpoint |
| `sla-service` | SLA config handler, `/api/v2/sla-dashboard` | Schema |
| `asset-service` | asset handler | Schema wrapper |
| `notification-service` | webhook handler | Schema |
| `scan-service` | route registration | Route + handler |
| `ai-service` | config + graceful degradation | Config + Handler |
| `apps/osv/bff/dashboard.go` | `HandleDashboardSLA` | BFF target |

---

## Roadmap Thực Hiện

```
SPRINT 1 (P0 — Unblock core features) — ~2 ngày
├── SOL-001: Set NVD_API_KEY + OpenSearch index + search fallback
├── SOL-002: Implement scan-service list/create routes  
└── SOL-003: Fix findings 500 + SLA dashboard + BFF target

SPRINT 2 (P1 — Schema alignment) — ~3 ngày
├── SOL-004 FIX-1: auth/me wrapper (identity-service)
├── SOL-004 FIX-3: KEV response schema (data-service)
├── SOL-004 FIX-4/5: Assets/Products wrapper (asset-service, finding-service)
├── SOL-004 FIX-6/7/8: Admin handlers (gateway BFF)
├── SOL-004 FIX-9: Webhooks array (notification-service)
└── SOL-005: Register CWE/EPSS/Vendor routes trong data-service router

SPRINT 3 (P1 — Implement missing) — ~5 ngày
├── SOL-005: EPSSHandler implementation
├── SOL-005: VendorHandler implementation
└── SOL-005: Taxonomy (CWE/CAPEC) handler registration

SPRINT 4 (P2 — AI features) — ~2 ngày
└── SOL-006: Ollama setup + graceful degradation
```

---

## Key Findings (Tóm Tắt)

| # | Phát Hiện Quan Trọng | Hành Động |
|---|---|---|
| 1 | Gateway router.go **đầy đủ** — tất cả 150+ routes đã mount | KHÔNG sửa gateway |
| 2 | 404 errors là do **service handler chưa register route** (không phải gateway thiếu) | Fix trong từng service |
| 3 | 500 errors: CVE search (ES empty) + Findings (SQL) + Dashboard SLA (wrong internal URL) | Fix upstream services |
| 4 | Schema mismatch chỉ ở **Adapter layer** — domain logic đúng | Fix HTTP response format |
| 5 | `dashboard.go` gọi `finding-service:8085/internal/sla-dashboard` — endpoint này chưa có | Thêm hoặc đổi target sang sla-service |
| 6 | AI service thất bại do thiếu config, không phải code bug | Set OPENAI_API_KEY hoặc cấu hình Ollama |
| 7 | `kev_handler.go` dùng `entries`/`limit` thay vì `data`/`page_size` như spec | Đổi JSON tags |
| 8 | `handlers.go` (identity) `Logout` đã return 204 — có thể middleware override | Kiểm tra middleware chain |
