# SOL-000 — Implementation Solutions Index

**Version:** 1.1  
**Ngày tạo:** 2026-06-16  
**Cập nhật:** 2026-06-17  
**Trạng thái:** 🔄 In Progress — Phase 1 ✅ Complete · Phase 2 API layers ✅ · Phase 3 Infra ✅ · Component migrations 🔄 Pending  

---

## Tổng quan

Thư mục này chứa các giải pháp kỹ thuật chi tiết để migrate OSV Platform UI từ MVP hiện tại sang kiến trúc mục tiêu định nghĩa trong [architecture.md](../../../architecture.md) và [TDD.md](../../../TDD.md).

---

## Danh sách Solutions

| File | Tiêu đề | Trạng thái |
|------|---------|------------|
| [SOL-001](./SOL-001-gap-analysis.md) | Gap Analysis | ✅ Completed |
| [SOL-002](./SOL-002-phase1-foundation.md) | Phase 1: Foundation | ✅ Completed |
| [SOL-003](./SOL-003-phase2-api-migration.md) | Phase 2: API Migration | 🔄 In Progress (API done / migrations pending) |
| [SOL-004](./SOL-004-phase3-polish-testing.md) | Phase 3: Polish & Testing | 🔄 In Progress (infra done / polish pending) |
| [SOL-005](./SOL-005-folder-structure.md) | Folder Structure | 🔄 In Progress (new files done / moves pending) |

---

## Lộ trình thực thi (Execution Roadmap)

```
Sprint 1-2  →  SOL-002 (Phase 1: Foundation)
Sprint 3-5  →  SOL-003 (Phase 2: API Migration)  
Sprint 6-7  →  SOL-004 (Phase 3: Polish & Testing)
Sprint 8+   →  Advanced Features (Notifications SSE, ⌘K, PWA)
```

---

## Vấn đề P0 cần giải quyết ngay

> [!CAUTION]
> **VI PHẠM KIẾN TRÚC**: Toàn bộ 37 components trong `src/app/components/` đang hardcode business data arrays trực tiếp trong component scope — đây là vi phạm nghiêm trọng nhất theo `architecture.md Section 5.5`.

**Ví dụ vi phạm trong Dashboard.tsx (dòng 11-119):**
```typescript
// ❌ VI PHẠM: 7 hardcoded arrays
const riskTrendData = [...];   // Line 11
const severityData = [...];    // Line 20
const productData = [...];     // Line 27
const recentFindings = [...];  // Line 35
const kevAlerts = [...];       // Line 103
const recentScans = [...];     // Line 109
const slaBreaches = [...];     // Line 115
```

**Giải pháp (từ SOL-003):**
```typescript
// ✅ ĐÚNG: React Query hook
export function Dashboard() {
  const metricsQuery = useDashboardMetrics('30d');
  return (
    <QueryBoundary query={metricsQuery} skeleton={<DashboardSkeleton />}>
      {(data) => <DashboardContent data={data} />}
    </QueryBoundary>
  );
}
```

---

## Checklist tổng thể

### Phase 1 — Foundation (Sprint 1-2) ✅ COMPLETED
- [x] `pnpm add zustand @tanstack/react-query axios msw zod @tanstack/react-virtual`
- [x] Shared types: `auth.ts`, `api.ts`, `cve.ts`, `finding.ts`, `scan.ts`
- [x] Axios client với JWT interceptors (`shared/api/client.ts`)
- [x] QueryClient với key factories (`shared/api/queryClient.ts`)
- [x] Zustand auth store — persist user, NOT token (`features/auth/store/authStore.ts`)
- [x] React Router v7 (`app/router.tsx`)
- [x] AuthGuard component (`app/components/AuthGuard.tsx`)
- [x] Providers wrapper (`app/providers.tsx`)
- [x] MSW setup: `mocks/browser.ts`, `mocks/server.ts`, `mocks/handlers/`, `mocks/fixtures/`
- [x] QueryBoundary component (`shared/components/feedback/QueryBoundary.tsx`)
- [x] Endpoints constants (`shared/api/endpoints.ts`)

### Phase 2 — API Migration (Sprint 3-5) 🔄 API DONE / MIGRATIONS PENDING
- [x] Dashboard: API layer + hook + fixture (`dashboard/api/`, `hooks/`, `types.ts`, `DashboardSkeleton`)
- [ ] Dashboard: Xóa 7 hardcoded arrays → React Query _(defer Phase 3)_
- [x] CVE Search: API layer + hook + MSW + fixtures (`cve-intel/api/cveApi.ts`, `useCVESearch.ts`, `CVETable.tsx` virtualized)
- [ ] CVE Search: Migrate component với URL-based filters _(defer Phase 3)_
- [x] KEV Catalog: API layer + hooks (`cve-intel/api/kevApi.ts`, `useKEVCatalog.ts`)
- [x] Findings: API layer + hooks + mutations (`findings/api/`, `hooks/`)
- [x] Scanning: API layer + SSE hook + Zod schema (`scanning/api/`, `hooks/useScanSSE.ts`, `schemas/`)
- [x] Assets: API layer + hook (`assets/api/`, `assets/hooks/`)
- [x] Product Security, AI Center, Reports, Admin, Notifications: API layers done
- [x] Shared utils: `severity.ts`, `sla.ts`, `productGrade.ts`, `findingStateMachine.ts`, `date.ts`, `cn.ts`
- [x] Shared components: `SeverityBadge`, `EPSSBar`, `KEVIndicator`, `SLABadge`, `StatusBadge`, `GradeCircle`

### Phase 3 — Polish (Sprint 6-7) 🔄 INFRA DONE / COMPONENT POLISH PENDING
- [x] CVETable: Virtualization với `@tanstack/react-virtual` ✅
- [ ] FindingsList: Virtualization _(pending)_
- [x] ScanWizard schema: Zod validation (`scanWizardSchema.ts`) ✅
- [ ] ScanWizard component: React Hook Form integration _(pending)_
- [x] DashboardSkeleton UI ✅
- [x] Vitest setup: `src/test/setup.ts`, `src/test/utils.tsx` ✅
- [x] Unit tests: `severity.test.ts`, `sla.test.ts`, `findingStateMachine.test.ts`, `useDashboardMetrics.test.tsx` ✅
- [x] ESLint custom rule: `eslint-rules/no-hardcode-mock-data.js` ✅
- [x] CI pipeline: `.github/workflows/ci.yml` với hardcode detection gate ✅
- [x] Playwright: `playwright.config.ts`, `e2e/auth.spec.ts`, `e2e/cve.spec.ts` ✅
- [ ] Component migrations từ `app/components/` → `features/*/components/` _(Phase 3 ongoing)_

### Phase 4 — Advanced (Sprint 8+)
- [x] In-app notifications via SSE (`useNotifications.ts` với `useSSE`) ✅ infrastructure
- [ ] Command Palette ⌘K (multi-entity global search) _(pending)_
- [ ] Offline PWA support (dashboard cache) _(pending)_
- [ ] Dark/Light theme toggle _(pending)_
- [ ] i18n preparation (react-i18next) _(pending)_

---

## Quyết định kiến trúc quan trọng

| # | Quyết định | Lý do |
|---|-----------|-------|
| 1 | Zustand cho auth, React Query cho server state | Tách biệt rõ ràng: global state vs cache |
| 2 | Access token trong Zustand memory (không localStorage) | Giảm XSS risk |
| 3 | Refresh token via httpOnly cookie (server-managed) | Browser không thể đọc qua JS |
| 4 | MSW cho dev/test, không có mock trong components | API-First principle, zero code change khi backend sẵn sàng |
| 5 | Feature-based folder structure | Scalability, tránh coupling giữa features |
| 6 | React Router v7 Data Router | URL-based deep links, browser history |
| 7 | Server-side pagination (pageSize=50) | Hỗ trợ 300K CVEs, không tải toàn bộ |
| 8 | Virtualization cho lists > 100 rows | Performance với datasets lớn |
| 9 | Key factory pattern cho React Query keys | Cache invalidation chính xác |
| 10 | Zod schema cho form validation | Type-safe, runtime validation |
