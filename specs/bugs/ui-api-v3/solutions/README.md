# Solutions Index — UI API v3 Bugs

> **Dựa trên**: `specs/01-architecture.md` (v3.0), `specs/02-technical-design.md` (v2.0)  
> **Ngày**: 2026-06-23

## Tổng Quan Phân Tích

Sau khi đọc kiến trúc, **tất cả 40 bug endpoints đều đã được thiết kế trong spec** — vấn đề là chúng **chưa được route trong `apps/osv` gateway** hoặc **chưa được implement trong upstream service**. Mọi giải pháp đều tuân theo pattern:

```
Gateway (apps/osv) → Route Registration → Upstream Service → DB/Cache
```

### Phân Loại Nguyên Nhân

| Nhóm | Nguyên nhân | Số bug |
|---|---|---|
| **A — Route Missing in Gateway** | Gateway chưa register route hoặc route bị sai path | BUG-001, 002, 005, 007, 010, 013, 014, 016, 017 |
| **B — Wrong HTTP Method in Upstream** | Service implement method khác (PUT vs PATCH) | BUG-006, 008, 009, 012, 015 |
| **C — Feature Not Implemented in Upstream** | Use case chưa có trong service | BUG-003, 004, 011 |

---

## Danh Sách Files Giải Pháp

| File | Bugs | Service Liên Quan | Status |
|---|---|---|---|
| [SOL-001-gateway-routes.md](./SOL-001-gateway-routes.md) | Nhiều bugs | `apps/osv` gateway | `[x] DONE` |
| [SOL-002-identity-service.md](./SOL-002-identity-service.md) | BUG-001, BUG-014 | `identity-service` | `[x] DONE` |
| [SOL-003-notification-service.md](./SOL-003-notification-service.md) | BUG-002 | `notification-service` | `[x] DONE` |
| [SOL-004-finding-service.md](./SOL-004-finding-service.md) | BUG-006, BUG-007, BUG-008, BUG-010 | `finding-service`, `sla-service` | `[x] DONE` |
| [SOL-005-scan-service.md](./SOL-005-scan-service.md) | BUG-005 | `scan-service` | `[x] DONE` |
| [SOL-006-ai-service.md](./SOL-006-ai-service.md) | BUG-011 | `ai-service` | `[x] DONE` |
| [SOL-007-jira-integration.md](./SOL-007-jira-integration.md) | BUG-013 | `jira-service` | `[x] DONE` |
| [SOL-008-asset-product-service.md](./SOL-008-asset-product-service.md) | BUG-009 | `asset-service`, `product-service` | `[x] DONE` |
| [SOL-009-search-audit.md](./SOL-009-search-audit.md) | BUG-003, BUG-004, BUG-016, BUG-017 | `search-service`, `audit-service` | `[x] DONE` |

---

## Nguyên Tắc Thiết Kế (từ Architecture)

1. **Gateway pattern**: Tất cả routes phải đi qua `apps/osv` (port 8080). Gateway thực hiện: Auth → InjectHeaders → RateLimit → ReverseProxy đến upstream.
2. **Auth middleware**: `protected` chain cho hầu hết endpoints; `adminOnly` cho admin routes.
3. **Clean Architecture**: Mỗi use case mới phải thêm vào đúng layer: domain → usecase → adapter/delivery → infra.
4. **NATS events**: Mọi mutation quan trọng phải publish event để audit-service ghi lại.
5. **Redis cache**: BFF endpoints (dashboard, triage queue, grades) dùng Redis TTL để tránh DB overload.
