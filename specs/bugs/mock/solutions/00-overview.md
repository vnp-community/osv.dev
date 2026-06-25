# Mock Bug Solutions — Overview

> **Dựa trên**: `specs/01-architecture.md` (v3.0) và `specs/02-technical-design.md` (v2.0)  
> **Ngày tạo**: 2026-06-22  
> **Mục tiêu**: Giải pháp fix toàn bộ 15 bugs từ `mock-bugs-report.md`

---

## Nguyên tắc thiết kế (từ Architecture Spec)

Theo `01-architecture.md §1.2` và `02-technical-design.md §2`:

1. **Clean Architecture 4 layers**: domain → usecase → adapter → infra. Fix phải tuân theo dependency direction.
2. **Repository Pattern**: Mọi data access phải qua interface, không trực tiếp gọi DB.
3. **Event-Driven**: Tất cả state changes phải publish NATS event (at-least-once).
4. **Graceful Degradation**: Khi dependency không sẵn sàng, service vẫn phải hoạt động (với subset features), KHÔNG được panic.
5. **Env-based Config**: Tất cả credentials/URLs phải đọc từ environment variables.

---

## Danh sách file giải pháp

| File | Bugs | Mô tả |
|------|------|--------|
| [P0-crash-fixes.md](./P0-crash-fixes.md) | MOCK-002, MOCK-012, MOCK-014 | Crash Risk — fix ngay lập tức |
| [P1-data-correctness.md](./P1-data-correctness.md) | MOCK-007, MOCK-008, MOCK-011 | Data Correctness — fix ngắn hạn |
| [P2-feature-completion.md](./P2-feature-completion.md) | MOCK-001, MOCK-004, MOCK-006, MOCK-009, MOCK-010, MOCK-013 | Feature Completion — fix trung hạn |
| [P3-resilience.md](./P3-resilience.md) | MOCK-003, MOCK-005, MOCK-015 | Resilience — fix dài hạn |

---

## Roadmap tổng hợp

```
Tuần 1 (P0 — Crash Fixes):
  ├── MOCK-002: nil-check trong ReportHandler.Create()
  ├── MOCK-012: nil-check trong APIKeyValidator.Validate()
  └── MOCK-014: nil-check trong notification-service router

Tuần 2 (P1 — Data Correctness):
  ├── MOCK-007: Wire Agent PostgreSQL repository
  ├── MOCK-008: Wire AI embedding via ai-service gRPC
  └── MOCK-011: OAuth credentials từ env vars

Tuần 3-4 (P2 — Feature Completion):
  ├── MOCK-001: PostgreSQL ReportRepo + MinIO Storage wire
  ├── MOCK-004: Wire 7 nil handlers trong finding-service
  ├── MOCK-006: Wire ScanRepo, AgentRepo, StatsRepo
  ├── MOCK-009: Wire OpenSearch client
  ├── MOCK-010: Wire InternalHandler
  └── MOCK-013: SearchAddr trong EmbeddedConfig

Sprint 2 (P3 — Resilience):
  ├── MOCK-005: Outbox pattern cho NATS
  ├── MOCK-003: Wire NATS EventBus cho BulkHandler
  └── MOCK-015: Wire FindingClient gRPC + NATS cho asset-service
```

---

## Liên kết Architecture

- **Report Service**: `01-architecture.md §3.5` — embedded trong finding-service
- **Agent Management**: `01-architecture.md §3.6` — scan-service, active scanning
- **Semantic Search**: `01-architecture.md §3.3` + `02-technical-design.md §11.2`
- **OAuth**: `01-architecture.md §3.4` — identity-service
- **API Key Validation**: `02-technical-design.md §10.2` — prefix + SHA-256 pattern
- **NATS Events**: `01-architecture.md §2.1` — at-least-once, finding.status.changed v.v.
- **Notification**: `01-architecture.md §3.8` — 5-channel (email, slack, teams, SSE, webhook)
