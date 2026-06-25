# Tasks — Hardcode Bug Fix v1

> **Tạo từ**: [solutions/](../solutions/README.md)  
> **Tổng tasks**: 13  
> **Phương pháp**: Mỗi task là 1 unit of work độc lập, có thể thực thi bởi AI agent

---

## Thứ Tự Thực Thi

```
TASK-000 (prerequisite)
    └── TASK-001 → TASK-002 → TASK-003 → TASK-004 → TASK-005  [Phase 1: Production blockers]
    └── TASK-006 → TASK-007 → TASK-008 → TASK-009 → TASK-010  [Phase 2: Before release]
    └── TASK-011 → TASK-012                                    [Phase 3: Tech debt]
```

---

## Danh Sách Tasks

| Task | Bug | Priority | Service(s) | Mô tả |
|------|-----|----------|------------|-------|
| [TASK-000](./TASK-000-shared-config-helpers.md) | — | 🔵 Prerequisite | `shared` | Thêm helper functions vào `shared/pkg/config/loader.go` |
| [TASK-001](./TASK-001-fix-bug004-credentials-infra.md) | BUG-004 | 🔴 High | `notification`, `asset`, `search`, `finding` | Xóa dev credentials; thêm warning log cho localhost fallback |
| [TASK-002](./TASK-002-fix-bug001-gateway-search-addr.md) | BUG-001 | 🔴 High | `gateway-service` | Thêm `SearchAddr` vào `EmbeddedConfig`; fix hardcoded search-service URL |
| [TASK-003](./TASK-003-fix-bug003-osv-handler-timeout.md) | BUG-003 | 🔴 High | `gateway-service` | Thêm warning log + configurable timeout cho OSV handler |
| [TASK-004](./TASK-004-fix-bug010-report-repo-stub.md) | BUG-010 | 🔴 High | `finding-service` | Fix `nilReportRepo` inconsistency; config-driven MinIO wiring |
| [TASK-005](./TASK-005-fix-bug012-scan-zap-config.md) | BUG-012 | 🔴 High | `scan-service` | Tạo `ScanConfig` struct; externalize ZAP URL và timeouts |
| [TASK-006](./TASK-006-fix-bug009-grade-logic.md) | BUG-009 | 🟡 Medium | `finding-service` | Fix grade "A" unreachable; tách domain scoring package |
| [TASK-007](./TASK-007-fix-bug011-stats-partial.md) | BUG-011 | 🟡 Medium | `data-service` | Bỏ hardcoded empty fields KEV/EPSS; implement repo methods |
| [TASK-008](./TASK-008-fix-bug007-ai-config.md) | BUG-007 | 🟡 Medium | `ai-service` | Unify Ollama URL; tập trung AI config vào 1 nơi |
| [TASK-009](./TASK-009-fix-bug002-gateway-bff-proxy.md) | BUG-002 | 🟡 Medium | `gateway-service`, `product-service`, `identity-service` | Proxy ProductTypes và AdminRoles đến upstream service |
| [TASK-010](./TASK-010-fix-bug008-pagination-constants.md) | BUG-008 | 🟡 Medium | `finding-service` | Tạo pagination constants package; unify magic numbers |
| [TASK-011](./TASK-011-fix-bug005-metrics-ports.md) | BUG-005 | 🟢 Low | 4 services | Đọc `METRICS_PORT` từ env thay vì hardcode |
| [TASK-012](./TASK-012-fix-bug006-version-strings.md) | BUG-006 | 🟢 Low | 4 services + Makefile | Inject version qua ldflags; xóa hardcoded `"1.0.0"` |
