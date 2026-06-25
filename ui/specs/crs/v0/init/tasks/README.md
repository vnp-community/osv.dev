# Tasks — OSV Platform UI Init (v0)

**Nguồn:** [solutions/](../solutions/)  
**Kiến trúc:** [architecture.md](../../../architecture.md)  
**TDD:** [TDD.md](../../../TDD.md)

---

## Danh sách Tasks

### Phase 1 — Foundation (Sprint 1-2) ✅ COMPLETED

| Task ID | Tên | Ư u tiên | Phụ thuộc | Ước tính | Status |
|---------|-----|---------|-----------|---------|---------|
| [TASK-UI-001](./TASK-UI-001-install-deps.md) | Cài đặt dependencies | 🔴 P0 | — | 30m | ✅ Done |
| [TASK-UI-002](./TASK-UI-002-shared-types.md) | Tạo shared TypeScript types | 🔴 P0 | TASK-UI-001 | 1h | ✅ Done |
| [TASK-UI-003](./TASK-UI-003-axios-client.md) | Axios client + JWT interceptors | 🔴 P0 | TASK-UI-002 | 1h | ✅ Done |
| [TASK-UI-004](./TASK-UI-004-query-client.md) | React Query client + Auth Store + Providers + Router | 🔴 P0 | TASK-UI-003 | 2h | ✅ Done |
| [TASK-UI-005](./TASK-UI-005-msw-setup.md) | MSW mock layer setup | 🔴 P0 | TASK-UI-003 | 1.5h | ✅ Done |
| [TASK-UI-006](./TASK-UI-006-shared-utils-components.md) | Shared utils + QueryBoundary + Shared components | 🔴 P0 | TASK-UI-002 | 2h | ✅ Done |

### Phase 2 — API Migration (Sprint 3-5) 🔄 IN PROGRESS

| Task ID | Tên | Ư u tiên | Phụ thuộc | Ước tính | Status |
|---------|-----|---------|-----------|---------|---------|
| [TASK-UI-007](./TASK-UI-007-dashboard-module.md) | Dashboard module: API + hook + skeleton | 🔴 P0 | TASK-UI-004, 005, 006 | 3h | 🔄 In Progress |
| [TASK-UI-008](./TASK-UI-008-cve-intel-module.md) | CVE Intelligence module: API + hooks + CVETable | 🔴 P0 | TASK-UI-004, 005, 006 | 4h | 🔄 In Progress |
| [TASK-UI-009](./TASK-UI-009-findings-scanning-assets.md) | Findings + Scanning + Assets: API + hooks + SSE | 🟡 P1 | TASK-UI-004, 005, 006 | 4h | 🔄 In Progress |
| [TASK-UI-010](./TASK-UI-010-remaining-testing-ci.md) | Remaining modules + Testing + CI | 🟢 P2 | TASK-UI-007, 008, 009 | 6h | 🔄 In Progress |

> **Note:** Tất cả API layers + hooks + MSW handlers + test infra đã hoàn tất. Phần còn lại là **component migrations** (xóa hardcode từ `app/components/` và kết nối với React Query hooks) — được defer sang **Phase 3**.

### Phase 3 — Polish & Testing (Sprint 6-7) 📌 PENDING

| Task | Mô tả | Status |
|------|---------|--------|
| Component migrations | Xóa hardcode từ 37 components cũ | Pending |
| FindingsList virtualization | `@tanstack/react-virtual` | Pending |
| ScanWizard RHF integration | React Hook Form connection | Pending |
| E2E runtime verification | `pnpm playwright test` | Pending |
| Coverage check | `pnpm test --coverage` | Pending |

---

## Thứ tự thực thi (Dependency Order)

```
TASK-UI-001
    └── TASK-UI-002
            ├── TASK-UI-003
            │       └── TASK-UI-007
            │               ├── TASK-UI-011 (Dashboard)
            │               ├── TASK-UI-012 (CVE Intel)
            │               ├── TASK-UI-013 (Findings)
            │               ├── TASK-UI-014 (Scanning)
            │               ├── TASK-UI-015 (Assets)
            │               └── TASK-UI-016 (Remaining)
            ├── TASK-UI-004
            │       └── TASK-UI-008 ──► TASK-UI-011..016
            ├── TASK-UI-005
            │       └── TASK-UI-006
            └── TASK-UI-009
                    └── TASK-UI-010

[After Phase 2]:
    TASK-UI-012 ──► TASK-UI-017 (Virtualization)
    TASK-UI-014 ──► TASK-UI-018 (ScanWizard Form)
    TASK-UI-011..016 ──► TASK-UI-019 (Skeleton UI)
    TASK-UI-009..015 ──► TASK-UI-020 (Unit Tests)
    TASK-UI-011..016 ──► TASK-UI-021 (E2E Tests)
    TASK-UI-011..016 ──► TASK-UI-022 (CI Gate)
```

---

## Quy ước Task File

Mỗi task file tuân theo format:

```markdown
# TASK-UI-XXX — Tên Task

| Field | Value |
|-------|-------|
| Task ID | TASK-UI-XXX |
| Module | ui |
| Solution Ref | SOL-00X §N |
| Priority | 🔴/🟡/🟢 P0/P1/P2 |
| Depends On | TASK-UI-YYY |
| Estimated | Xh |

## Context
## Goal
## Target Files
## Implementation
## Verification
## Checklist
```
