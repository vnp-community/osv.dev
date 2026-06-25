# Tasks Index — UI API v3 Bug Fixes

> **Nguồn giải pháp**: `specs/bugs/ui-api-v3/solutions/`  
> **Phân tích thực tế**: Dựa trên code scan của toàn bộ services  
> **Ngày tạo**: 2026-06-23  
> **Phạm vi**: Server-side implementation (Go microservices)

## Phát Hiện Quan Trọng Từ Code Scan

Gateway (`apps/osv/internal/gateway/router.go`) **đã có đủ tất cả routes**. Vấn đề hoàn toàn nằm ở **upstream service handlers**:

| Service | Vấn Đề Thực Tế |
|---|---|
| `identity-service` | Router `/adapter/handler/http/router.go` thiếu `profile/sessions`, `profile/notifications/settings`, và MFA setup dùng POST thay vì GET |
| `notification-service` | Router mount `/api/v2/notifications` nhưng gateway forward `/api/v1/notifications` — path mismatch |
| `finding-service` | Thiếu `PATCH /api/v1/findings/{id}`, `bulk/close`, `bulk_reopen` dùng sai path |
| `ai-service` | Handlers `GetTriageQueue`, `GetEnrichmentStatus`, `TriggerEnrichment` đã có nhưng router chưa mount |
| `jira-service` | Routes `/api/v1/integrations/jira` chưa có trong service router |

## Danh Sách Tasks

| Task | File | Service | Priority | Status |
|---|---|---|---|---|
| TASK-001 | [TASK-001](./TASK-001-identity-mfa-get.md) | identity-service | 🔴 HIGH | `[x] DONE` |
| TASK-002 | [TASK-002](./TASK-002-identity-profile-sessions.md) | identity-service | 🔴 HIGH | `[x] DONE` |
| TASK-003 | [TASK-003](./TASK-003-notification-v1-routes.md) | notification-service | 🔴 HIGH | `[x] DONE` |
| TASK-004 | [TASK-004](./TASK-004-finding-patch-bulk.md) | finding-service | 🔴 HIGH | `[x] DONE` |
| TASK-005 | [TASK-005](./TASK-005-finding-risk-acceptance.md) | finding-service | 🔴 HIGH | `[x] DONE` |
| TASK-006 | [TASK-006](./TASK-006-ai-service-router.md) | ai-service | 🟡 MEDIUM | `[x] DONE` |
| TASK-007 | [TASK-007](./TASK-007-jira-service-routes.md) | jira-service | 🟡 MEDIUM | `[x] DONE` |
| TASK-008 | [TASK-008](./TASK-008-asset-patch.md) | asset-service | 🟡 MEDIUM | `[x] DONE` |
| TASK-009 | [TASK-009](./TASK-009-scan-history.md) | scan-service | 🟡 MEDIUM | `[x] DONE` |
| TASK-010 | [TASK-010](./TASK-010-search-audit.md) | search/audit/data service | 🟢 LOW | `[x] DONE` |

## Thứ Tự Thực Hiện Đề Xuất

```
Sprint 1 (Critical):  TASK-001 → TASK-002 → TASK-003 → TASK-004 → TASK-005
Sprint 2 (Medium):    TASK-006 → TASK-007 → TASK-008
Sprint 3 (Low):       TASK-009 → TASK-010
```
