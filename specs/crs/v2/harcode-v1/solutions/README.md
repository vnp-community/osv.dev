# Solutions Index — Hardcode V1 Change Requests

**Phạm vi:** 15 Change Requests từ audit hardcode/mock trong `services/`  
**Kiến trúc tham chiếu:** `specs/01-architecture.md §11` + `specs/02-technical-design.md §11`  
**Ngày:** 2026-06-23  
**Cập nhật:** 2026-06-24 — Tất cả 15 solutions đã được IMPLEMENTED ✅

---

## Tổng quan

| Solution | CR | Service | Độ phức tạp | Sprint | Status |
|----------|----|---------|------------|--------|--------|
| [SOL-001](SOL-001-ai-generate-embedding.md) | CR-HC-001 | ai-service | High | S1 | ✅ DONE |
| [SOL-002](SOL-002-search-mock-embedder.md) | CR-HC-002 | search-service | Medium | S1 | ✅ DONE |
| [SOL-003](SOL-003-gateway-product-types.md) | CR-HC-003 | gateway + product-service | Medium | S2 | ✅ DONE |
| [SOL-004](SOL-004-gateway-admin-settings.md) | CR-HC-004 | gateway + identity-service | High | S2 | ✅ DONE |
| [SOL-005](SOL-005-finding-report-date.md) | CR-HC-005 | finding-service | Low | S1 | ✅ DONE |
| [SOL-006](SOL-006-identity-static-permissions.md) | CR-HC-006 | identity-service | Medium | S2 | ✅ DONE |
| [SOL-007](SOL-007-scan-nil-handlers.md) | CR-HC-007 | scan-service | High | S2 | ✅ DONE |
| [SOL-008](SOL-008-data-cwe-repo.md) | CR-HC-008 | data-service | Low | S1 | ✅ DONE |
| [SOL-009](SOL-009-data-avg-days-patch.md) | CR-HC-009 | data-service | Low | S1 | ✅ DONE |
| [SOL-010](SOL-010-jira-stub-issues.md) | CR-HC-010 | jira-service | High | S3 | ✅ DONE |
| [SOL-011](SOL-011-identity-invitation-email.md) | CR-HC-011 | identity-service | High | S3 | ✅ DONE |
| [SOL-012](SOL-012-search-history.md) | CR-HC-012 | search-service | Medium | S2 | ✅ DONE |
| [SOL-013](SOL-013-ai-batch-enrich.md) | CR-HC-013 | ai-service | High | S2 | ✅ DONE |
| [SOL-014](SOL-014-data-grpc-cvedb.md) | CR-HC-014 | data-service | High | S3 | ✅ DONE |
| [SOL-015](SOL-015-gateway-health-check.md) | CR-HC-015 | gateway-service | Medium | S1 | ✅ DONE |

---

## Build Verification

**Date:** 2026-06-24  
**Result:** ✅ All 7 services build with `go build ./...` — zero errors

| Service | Build |
|---------|-------|
| identity-service | ✅ OK |
| scan-service | ✅ OK |
| ai-service | ✅ OK |
| data-service | ✅ OK |
| search-service | ✅ OK |
| gateway-service | ✅ OK |
| finding-service | ✅ OK |

---

## Tóm tắt các thay đổi chính

### Sprint 1 — Quick Wins (COMPLETED)
1. **SOL-005** — Fix hardcoded timestamp: `time.Now()` thay `time.Date(2024,...)` trong report handler
2. **SOL-008** — Wire CWERepo: `postgres.NewCWERepo(pgPool)` → `httpdelivery.NewCWEHandler(cweRepo)` trong data-service embed
3. **SOL-009** — AvgDaysToPatch: SQL query real `AVG(patch_date - published_date)` thay 30.0 hardcode
4. **SOL-015** — Gateway health ready: ping tất cả upstream services thay trả `{"status":"ok"}` hardcoded
5. **SOL-002** — Remove MockEmbedder: search-service trả 503 khi AI unavailable
6. **SOL-001** — generate_embedding: gọi real Ollama/OpenAI HTTP endpoint

### Sprint 2 — Architecture (COMPLETED)
7. **SOL-012** — Search history: persist vào `search_history` PostgreSQL table
8. **SOL-003** — ProductTypes từ DB: `product_types` table, API endpoint `/api/v2/product-types`
9. **SOL-004** — Admin settings từ DB: `platform_settings` table, handler + usecase layer
10. **SOL-006** — RBAC Matrix từ DB: `rbac_roles_metadata` table, `RBACRepo` implementation
11. **SOL-007** — Scan nil handlers: wire real `ScheduleRepo` (PostgreSQL) trong scan-service embedded
12. **SOL-013** — AI batch_enrich: inject + call `enrich.UseCase` thay TODO stub

### Sprint 3 — Complex Features (COMPLETED)
13. **SOL-010** — Jira Issue CRUD: `IssueMappingRepo` (PostgreSQL), 4 real DB handlers (List/Create/Get/Delete)
14. **SOL-011** — User invitation email: `InviteUserUseCase` + SMTP sender + `user_invitations` table + `AcceptInvite` endpoint
15. **SOL-014** — gRPC CVEDB: Register `CVEDBHandler` với real PostgreSQL repositories (`CVEBinToolRepo`, `ExploitRepo`, `MetricRepo`, `PURL2CPERepo`, `DBAdminRepo`)

---

## Nguyên tắc áp dụng trong mọi solution

1. **Migration First**: SQL migration được tạo trước khi implement repository
2. **Interface in Domain**: Interface định nghĩa trong `domain/` package
3. **Constructor Injection**: Tất cả dependencies inject qua constructor
4. **No Return nil,nil**: Mọi UseCase có real implementation — không có TODO hoặc mock
5. **Nil-safe degradation**: Khi dependency optional (SMTP, AI) → log warning, không crash
6. **Compile-time verification**: `var _ repository.Interface = (*Impl)(nil)` trong mọi repo file
