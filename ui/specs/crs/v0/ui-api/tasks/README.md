# Tasks — UI-API v2 Implementation

**Nguồn Solutions:** [solutions/](../solutions/)  
**Kiến trúc:** [architecture.md](../../../../ui/specs/architecture.md)  
**TDD:** [TDD.md](../../../../ui/specs/TDD.md)  
**CR Index:** [CR Index](../README.md)

---

## Mục tiêu

Bộ tasks này chuyển đổi các **Solution documents** thành các **tác vụ cụ thể để AI thực thi** — mỗi task có đủ context, code mẫu, target files, verification steps để AI có thể thực hiện độc lập mà không cần hỏi lại.

**Nguyên tắc bất biến:**
- Mọi dữ liệu hiển thị trên UI **PHẢI** từ `useQuery`/`useMutation` → KHÔNG hardcode
- Mock data chỉ được phép trong `src/mocks/` (MSW fixtures)
- Mỗi task phải verify bằng `npx tsc --noEmit` + grep không-hardcode

---

## Danh sách Tasks

### Sprint 1 — P0 Foundation (Blocking)

| Task ID | Tên | Priority | Phụ thuộc | Est. |
|---------|-----|----------|-----------|------|
| [TASK-API-001](./TASK-API-001-endpoints-update.md) | Cập nhật ENDPOINTS + MSW handler registration | 🔴 P0 | — | 1h |
| [TASK-API-002](./TASK-API-002-auth-module.md) | Auth module: authApi + authStore + useAuth + usePermissions | 🔴 P0 | TASK-API-001 | 2h |
| [TASK-API-003](./TASK-API-003-auth-msw.md) | Auth MSW handlers + session restore + AuthGuard | 🔴 P0 | TASK-API-002 | 1h |
| [TASK-API-004](./TASK-API-004-dashboard-module.md) | Dashboard module: dashboardApi + hooks + MSW + SSE notifications | 🔴 P0 | TASK-API-003 | 2h |
| [TASK-API-005](./TASK-API-005-cve-intel-module.md) | CVE Intel module: 4 API clients + hooks + URL state sync | 🔴 P0 | TASK-API-001 | 3h |
| [TASK-API-006](./TASK-API-006-cve-intel-msw.md) | CVE Intel MSW handlers + fixtures + vendor autocomplete | 🔴 P0 | TASK-API-005 | 2h |

### Sprint 2 — P0 Core Features

| Task ID | Tên | Priority | Phụ thuộc | Est. |
|---------|-----|----------|-----------|------|
| [TASK-API-007](./TASK-API-007-findings-module.md) | Findings module: findingApi + state machine + hooks + bulk ops | 🔴 P0 | TASK-API-003 | 3h |
| [TASK-API-008](./TASK-API-008-findings-msw.md) | Findings MSW handlers + risk-acceptance handlers | 🔴 P0 | TASK-API-007 | 1h |
| [TASK-API-009](./TASK-API-009-product-module.md) | Product module: productApi + grade formula + hooks + MSW | 🔴 P0 | TASK-API-003 | 2h |
| [TASK-API-010](./TASK-API-010-reports-notifications-module.md) | Reports + Notifications module: APIs + hooks + MSW + bell | 🔴 P0 | TASK-API-003 | 3h |
| [TASK-API-011](./TASK-API-011-admin-integrations-module.md) | Admin + Integrations module: APIs + hooks + MSW + health | 🔴 P0 | TASK-API-003 | 3h |

### Sprint 3–4 — P1 Advanced Features

| Task ID | Tên | Priority | Phụ thuộc | Est. |
|---------|-----|----------|-----------|------|
| [TASK-API-012](./TASK-API-012-scanning-ai-assets-module.md) | Scanning + AI Center + Assets: APIs + SSE + hooks + MSW | 🟡 P1 | TASK-API-003 | 4h |

---

## Thứ tự thực thi (Dependency Graph)

```
TASK-API-001 (Endpoints + MSW registration)
    │
    ├── TASK-API-002 (Auth module)
    │       └── TASK-API-003 (Auth MSW + session + guard)
    │               ├── TASK-API-004 (Dashboard)
    │               ├── TASK-API-007 (Findings)
    │               ├── TASK-API-009 (Products)
    │               ├── TASK-API-010 (Reports + Notifications)
    │               ├── TASK-API-011 (Admin + Integrations)
    │               └── TASK-API-012 (Scanning + AI + Assets)
    │
    └── TASK-API-005 (CVE Intel module)
            └── TASK-API-006 (CVE Intel MSW)
```

---

## Solution → Task Mapping

| Solution | Tasks |
|---------|-------|
| [SOL-UI-001](../solutions/SOL-UI-001-auth-api.md) Auth | TASK-API-002, TASK-API-003 |
| [SOL-UI-002](../solutions/SOL-UI-002-dashboard-api.md) Dashboard | TASK-API-004 |
| [SOL-UI-003](../solutions/SOL-UI-003-cve-intel-api.md) CVE Intel | TASK-API-005, TASK-API-006 |
| [SOL-UI-004](../solutions/SOL-UI-004-scanning-api.md) Scanning | TASK-API-012 |
| [SOL-UI-005](../solutions/SOL-UI-005-finding-api.md) Findings | TASK-API-007, TASK-API-008 |
| [SOL-UI-006](../solutions/SOL-UI-006-asset-api.md) Assets | TASK-API-012 |
| [SOL-UI-007](../solutions/SOL-UI-007-product-api.md) Products | TASK-API-009 |
| [SOL-UI-008](../solutions/SOL-UI-008-ai-center-api.md) AI Center | TASK-API-012 |
| [SOL-UI-009](../solutions/SOL-UI-009-reports-notifications-api.md) Reports+Notifs | TASK-API-010 |
| [SOL-UI-010](../solutions/SOL-UI-010-admin-integrations-api.md) Admin+Integrations | TASK-API-011 |

---

## Quy ước Task File

```markdown
# TASK-API-XXX — Tên Task

| Field | Value |
|-------|-------|
| Task ID | TASK-API-XXX |
| Module | ui/src/features/... |
| Solution Ref | SOL-UI-00X §N |
| Priority | 🔴/🟡 P0/P1 |
| Depends On | TASK-API-YYY |
| Estimated | Xh |

## Context     — Tại sao task này cần làm
## Goal        — Mục tiêu cụ thể
## Target Files — CREATE/MODIFY file list
## Implementation — Code đầy đủ cho từng file
## Verification  — Lệnh check + expected output
## Checklist    — Checkbox items AI phải hoàn thành
```

---

## Quy tắc Bắt buộc cho AI

1. **KHÔNG hardcode business data** trong components hoặc hooks
2. **Fixtures chỉ được đặt trong** `src/mocks/fixtures/*.ts`
3. **Mọi API call** phải đi qua `apiClient` từ `@/shared/api/client`
4. **Mọi endpoint URL** phải lấy từ `ENDPOINTS` constant
5. **Mọi query key** phải dùng key factory (`fooKeys.list(...)`, không string thô)
6. **Sau mỗi task:** chạy `npx tsc --noEmit` và verify grep không-hardcode
