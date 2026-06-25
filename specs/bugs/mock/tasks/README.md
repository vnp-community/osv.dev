# Mock Bug Fix Tasks

> **Nguồn giải pháp**: `specs/bugs/mock/solutions/`  
> **Ngày tạo**: 2026-06-22  
> **Mục đích**: Task list để AI agent thực thi fix code trực tiếp

---

## Danh sách Tasks

| Task ID | Bug | Priority | Service | Loại thay đổi | Status |
|---------|-----|----------|---------|---------------|--------|
| [TASK-P0-001](./TASK-P0-001-report-handler-nil-check.md) | MOCK-002 | 🔴 P0 | finding-service | Code fix | `[x] DONE` |
| [TASK-P0-002](./TASK-P0-002-apikey-validator-nil-check.md) | MOCK-012 | 🔴 P0 | gateway-service | Code fix | `[x] DONE` |
| [TASK-P0-003](./TASK-P0-003-notification-router-nil-guard.md) | MOCK-014 | 🔴 P0 | notification-service | Code fix + DB | `[x] DONE` |
| [TASK-P1-001](./TASK-P1-001-agent-postgres-repo.md) | MOCK-007 | 🔴 P1 | scan-service | New file + DB | `[x] DONE` |
| [TASK-P1-002](./TASK-P1-002-semantic-search-ai-embedder.md) | MOCK-008 | 🔴 P1 | search-service | New file + config | `[x] DONE` |
| [TASK-P1-003](./TASK-P1-003-oauth-env-credentials.md) | MOCK-011 | 🔴 P1 | identity-service | Code fix + config | `[x] DONE` |
| [TASK-P2-001](./TASK-P2-001-report-repo-minio.md) | MOCK-001 | 🟡 P2 | finding-service | New file + DB | `[x] DONE` |
| [TASK-P2-002](./TASK-P2-002-finding-service-nil-handlers.md) | MOCK-004 | 🟡 P2 | finding-service | Code fix | `[x] DONE` |
| [TASK-P2-003](./TASK-P2-003-scan-service-repos.md) | MOCK-006 | 🟡 P2 | scan-service | New file + DB | `[x] DONE` |
| [TASK-P2-004](./TASK-P2-004-opensearch-wire.md) | MOCK-009,010 | 🟡 P2 | search-service | Code fix + config | `[x] DONE` |
| [TASK-P2-005](./TASK-P2-005-gateway-search-addr.md) | MOCK-013 | 🟡 P2 | gateway-service | Code fix | `[x] DONE` |
| [TASK-P3-001](./TASK-P3-001-outbox-publisher.md) | MOCK-005 | 🟡 P3 | finding-service | New file + DB | `[x] DONE` |
| [TASK-P3-002](./TASK-P3-002-bulk-handler-eventbus.md) | MOCK-003 | 🟡 P3 | finding-service | Code fix | `[x] DONE` |
| [TASK-P3-003](./TASK-P3-003-asset-finding-client.md) | MOCK-015 | 🟡 P3 | asset-service | New file + config | `[x] DONE` |

---

## Quy ước Task Format

Mỗi task file chứa:
- **Context**: Bug ID, file cần thay đổi, dependencies
- **Preconditions**: Điều kiện cần có trước khi thực thi
- **Steps**: Danh sách bước thực thi cụ thể
- **Acceptance Criteria**: Tiêu chí xác nhận hoàn thành
- **Test Commands**: Câu lệnh kiểm tra sau khi fix
