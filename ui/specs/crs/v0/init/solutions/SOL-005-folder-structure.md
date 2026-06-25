# SOL-005 — Target Folder Structure & Migration Map

**Version:** 1.1  
**Ngày tạo:** 2026-06-16  
**Cập nhật:** 2026-06-17  
**Trạng thái:** 🔄 In Progress — Tất cả [NEW] files Phase 1+2 đã tạo; các MOVE/MIGRATE (app/components/ → features/) defer sang Phase 3  
**Liên quan:** [SOL-002](./SOL-002-phase1-foundation.md), [SOL-003](./SOL-003-phase2-api-migration.md)

---

## 1. Target Folder Structure (Complete)

```
ui/src/
│
├── main.tsx                         # Entry point — bootstrap MSW + ReactDOM
│
├── app/                             # Application shell
│   ├── router.tsx                   # React Router v7 — all routes defined here
│   ├── providers.tsx                # QueryClientProvider + Toaster
│   └── components/
│       ├── AppLayout.tsx            # Sidebar + Topbar wrapper (outlet)
│       ├── AuthGuard.tsx            # Redirect to /login if not authenticated
│       ├── Sidebar.tsx              # → MIGRATE from app/components/Sidebar.tsx
│       └── Topbar.tsx               # → MIGRATE from app/components/Topbar.tsx
│
├── features/                        # Feature modules (Domain-Driven)
│   │
│   ├── auth/
│   │   ├── components/
│   │   │   ├── LoginScreen.tsx      # → MIGRATE from app/components/LoginScreen.tsx
│   │   │   ├── MFASetup.tsx         # [NEW]
│   │   │   ├── MFAVerify.tsx        # [NEW]
│   │   │   └── OAuthCallback.tsx    # [NEW]
│   │   ├── hooks/
│   │   │   ├── useAuth.ts           # [NEW] — useAuthStore wrapper
│   │   │   └── usePermissions.ts    # [NEW] — RBAC hook
│   │   ├── api/
│   │   │   └── authApi.ts           # [NEW] — login, refresh, logout
│   │   ├── store/
│   │   │   └── authStore.ts         # [NEW] — Zustand persist store
│   │   └── types.ts                 # → Use shared/types/auth.ts
│   │
│   ├── dashboard/
│   │   ├── components/
│   │   │   ├── Dashboard.tsx        # → MIGRATE + remove hardcode
│   │   │   ├── KPICard.tsx          # → EXTRACT from Dashboard.tsx
│   │   │   ├── RiskTrendChart.tsx   # → EXTRACT from Dashboard.tsx
│   │   │   ├── SeverityDonut.tsx    # → EXTRACT from Dashboard.tsx
│   │   │   ├── ProductGradeList.tsx # → EXTRACT from Dashboard.tsx
│   │   │   ├── KEVAlertsFeed.tsx    # → EXTRACT from Dashboard.tsx
│   │   │   ├── RecentScansList.tsx  # → EXTRACT from Dashboard.tsx
│   │   │   ├── SLABreachesList.tsx  # → EXTRACT from Dashboard.tsx
│   │   │   └── DashboardSkeleton.tsx # [NEW]
│   │   ├── hooks/
│   │   │   └── useDashboardMetrics.ts # [NEW]
│   │   ├── api/
│   │   │   └── dashboardApi.ts      # [NEW]
│   │   └── types.ts                 # [NEW] — DashboardKPIs, RiskTrendPoint, etc.
│   │
│   ├── cve-intel/
│   │   ├── components/
│   │   │   ├── CVESearch.tsx        # → MIGRATE + URL-based filters
│   │   │   ├── CVETable.tsx         # → EXTRACT + virtualize
│   │   │   ├── CVEFilterPanel.tsx   # → EXTRACT from CVESearch.tsx
│   │   │   ├── CVEDetailDrawer.tsx  # → EXTRACT from CVESearch.tsx
│   │   │   ├── CVETableSkeleton.tsx # [NEW]
│   │   │   ├── KEVCatalog.tsx       # → MIGRATE
│   │   │   ├── SemanticSearch.tsx   # → MIGRATE
│   │   │   ├── EPSSAnalytics.tsx    # → MIGRATE
│   │   │   ├── VendorCatalog.tsx    # → MIGRATE
│   │   │   └── CWELibrary.tsx       # → MIGRATE
│   │   ├── hooks/
│   │   │   ├── useCVESearch.ts      # [NEW]
│   │   │   ├── useCVEDetail.ts      # [NEW]
│   │   │   ├── useKEVCatalog.ts     # [NEW]
│   │   │   ├── useSemanticSearch.ts # [NEW]
│   │   │   └── useEPSSData.ts       # [NEW]
│   │   ├── api/
│   │   │   ├── cveApi.ts            # [NEW] — search, semantic, detail, export
│   │   │   ├── kevApi.ts            # [NEW] — list, stats
│   │   │   └── taxonomyApi.ts       # [NEW] — browse, cwe
│   │   └── types.ts                 # → Use shared/types/cve.ts
│   │
│   ├── scanning/
│   │   ├── components/
│   │   │   ├── ScanDashboard.tsx    # → MIGRATE
│   │   │   ├── ScanWizard.tsx       # → MIGRATE + React Hook Form + Zod
│   │   │   ├── ScanDetail.tsx       # [NEW] — replaces RunningScan + ScanHistory
│   │   │   ├── RunningScan.tsx      # → MIGRATE + SSE
│   │   │   ├── ScanHistory.tsx      # → MIGRATE
│   │   │   ├── NmapResults.tsx      # → MIGRATE
│   │   │   └── ZAPResults.tsx       # → MIGRATE
│   │   ├── hooks/
│   │   │   ├── useScan.ts           # [NEW] — list, detail queries
│   │   │   ├── useScanSSE.ts        # [NEW] — SSE hook
│   │   │   └── useCreateScan.ts     # [NEW] — mutation
│   │   ├── api/
│   │   │   └── scanApi.ts           # [NEW]
│   │   ├── schemas/
│   │   │   └── scanWizardSchema.ts  # [NEW] — Zod schema
│   │   └── types.ts                 # → Use shared/types/scan.ts
│   │
│   ├── findings/
│   │   ├── components/
│   │   │   ├── FindingsList.tsx     # → MIGRATE + server-side pagination
│   │   │   ├── FindingDetail.tsx    # → MIGRATE + mutations
│   │   │   ├── FindingsFilter.tsx   # → EXTRACT from FindingsList.tsx
│   │   │   ├── FindingsTable.tsx    # → EXTRACT + virtualize
│   │   │   ├── BulkActionsBar.tsx   # → EXTRACT from FindingsList.tsx
│   │   │   ├── SLADashboard.tsx     # → MIGRATE
│   │   │   └── RiskAcceptanceCenter.tsx # → MIGRATE
│   │   ├── hooks/
│   │   │   ├── useFindings.ts       # [NEW]
│   │   │   ├── useFindingDetail.ts  # [NEW]
│   │   │   ├── useUpdateFinding.ts  # [NEW] — mutation
│   │   │   └── useBulkOps.ts        # [NEW] — bulk close mutation
│   │   ├── api/
│   │   │   ├── findingApi.ts        # [NEW]
│   │   │   └── riskAcceptanceApi.ts # [NEW]
│   │   └── types.ts                 # → Use shared/types/finding.ts
│   │
│   ├── assets/
│   │   ├── components/
│   │   │   ├── AssetInventory.tsx   # → MIGRATE
│   │   │   └── AssetDetail.tsx      # → MIGRATE
│   │   ├── hooks/
│   │   │   ├── useAssets.ts         # [NEW]
│   │   │   └── useAssetDetail.ts    # [NEW]
│   │   ├── api/
│   │   │   └── assetApi.ts          # [NEW]
│   │   └── types.ts                 # [NEW] — Asset, AssetService
│   │
│   ├── product-security/
│   │   ├── components/
│   │   │   ├── ProductSecurity.tsx  # → MIGRATE
│   │   │   └── ProductDetail.tsx    # [NEW]
│   │   ├── hooks/
│   │   │   ├── useProducts.ts       # [NEW]
│   │   │   └── useEngagements.ts    # [NEW]
│   │   ├── api/
│   │   │   └── productApi.ts        # [NEW]
│   │   └── types.ts                 # [NEW] — Product, Engagement, Test
│   │
│   ├── ai-center/
│   │   ├── components/
│   │   │   ├── AITriage.tsx         # → MIGRATE
│   │   │   └── AIEnrichment.tsx     # → MIGRATE
│   │   ├── hooks/
│   │   │   └── useAITriage.ts       # [NEW]
│   │   ├── api/
│   │   │   └── aiApi.ts             # [NEW]
│   │   └── types.ts                 # [NEW] — AITriageQueueItem, CVEEnrichmentStatus
│   │
│   ├── reports/
│   │   ├── components/
│   │   │   └── ReportCenter.tsx     # → MIGRATE
│   │   ├── hooks/
│   │   │   └── useReports.ts        # [NEW]
│   │   ├── api/
│   │   │   └── reportApi.ts         # [NEW]
│   │   └── types.ts                 # [NEW] — ReportRun, ReportFormat
│   │
│   ├── notifications/
│   │   ├── components/
│   │   │   └── NotificationCenter.tsx # → MIGRATE
│   │   ├── hooks/
│   │   │   └── useNotifications.ts  # [NEW] — SSE notifications
│   │   └── types.ts                 # [NEW]
│   │
│   ├── integrations/
│   │   ├── components/
│   │   │   ├── APIKeyManagement.tsx # → MIGRATE
│   │   │   └── WebhookEvents.tsx    # → MIGRATE
│   │   ├── hooks/
│   │   │   ├── useAPIKeys.ts        # [NEW]
│   │   │   └── useWebhooks.ts       # [NEW]
│   │   └── api/
│   │       └── integrationApi.ts    # [NEW]
│   │
│   └── admin/
│       ├── components/
│       │   ├── UserManagement.tsx   # → MIGRATE
│       │   ├── RBACManagement.tsx   # → MIGRATE
│       │   ├── AuditLogs.tsx        # → MIGRATE
│       │   ├── SystemHealth.tsx     # → MIGRATE
│       │   └── SystemSettings.tsx   # → MIGRATE
│       ├── hooks/
│       │   ├── useUsers.ts          # [NEW]
│       │   ├── useAuditLogs.ts      # [NEW]
│       │   └── useSystemHealth.ts   # [NEW]
│       ├── api/
│       │   └── adminApi.ts          # [NEW]
│       └── types.ts                 # [NEW] — AdminUser, AuditEvent, ServiceHealth
│
├── shared/                          # Cross-cutting concerns
│   ├── api/
│   │   ├── client.ts               # [NEW] — Axios instance + interceptors
│   │   ├── queryClient.ts          # [NEW] — QueryClient + key factories
│   │   └── endpoints.ts            # [NEW] — API endpoint constants
│   │
│   ├── hooks/
│   │   ├── useSSE.ts               # [NEW] — Generic SSE hook
│   │   ├── useDebounce.ts          # [NEW] — Debounce utility hook
│   │   ├── useLocalStorage.ts      # [NEW] — Persist user preferences
│   │   └── usePermissions.ts       # [NEW] — RBAC hook (wraps useAuthStore)
│   │
│   ├── utils/
│   │   ├── severity.ts             # [NEW] — SEVERITY_COLORS, helpers
│   │   ├── date.ts                 # [NEW] — Date formatting utilities
│   │   ├── sla.ts                  # [NEW] — SLA calculation
│   │   ├── productGrade.ts         # [NEW] — Grade calculation logic
│   │   ├── findingStateMachine.ts  # [NEW] — Valid status transitions
│   │   └── cn.ts                   # [NEW] — clsx + tailwind-merge helper
│   │
│   ├── types/
│   │   ├── api.ts                  # [NEW] — APIError, PaginatedResponse
│   │   ├── auth.ts                 # [NEW] — User, UserRole, Permission
│   │   ├── cve.ts                  # [NEW] — CVE, KEVEntry, CWEDetail
│   │   ├── finding.ts              # [NEW] — Finding, FindingStatus, SLAStatus
│   │   └── scan.ts                 # [NEW] — Scan, ScanProgress, NmapHost
│   │
│   └── components/
│       ├── data-display/
│       │   ├── DataTable.tsx        # [NEW] — Generic table with pagination
│       │   ├── SeverityBadge.tsx    # [NEW] — Extracted from Dashboard.tsx
│       │   ├── StatusBadge.tsx      # [NEW]
│       │   ├── SLABadge.tsx         # [NEW]
│       │   ├── EPSSBar.tsx          # [NEW]
│       │   ├── KEVIndicator.tsx     # [NEW]
│       │   └── GradeCircle.tsx      # [NEW] — Extracted from Dashboard.tsx
│       │
│       ├── feedback/
│       │   ├── QueryBoundary.tsx    # [NEW] — Wraps loading/error/empty
│       │   ├── LoadingSpinner.tsx   # [NEW]
│       │   ├── ErrorState.tsx       # [NEW]
│       │   ├── EmptyState.tsx       # [NEW]
│       │   └── FullPageSpinner.tsx  # [NEW]
│       │
│       ├── layout/
│       │   ├── PageHeader.tsx       # [NEW] — Title + actions + period picker
│       │   └── ContentPanel.tsx     # [NEW] — Scrollable content area
│       │
│       └── global/
│           ├── CommandPalette.tsx   # → MIGRATE from GlobalSearch.tsx
│           └── SSEProgressBar.tsx   # [NEW]
│
├── mocks/                           # MSW mock layer (DEV only)
│   ├── browser.ts                   # [NEW] — setupWorker
│   ├── server.ts                    # [NEW] — setupServer (for tests)
│   ├── handlers/
│   │   ├── index.ts                 # [NEW] — Export all handlers
│   │   ├── auth.handlers.ts         # [NEW]
│   │   ├── dashboard.handlers.ts    # [NEW]
│   │   ├── cve.handlers.ts          # [NEW]
│   │   ├── kev.handlers.ts          # [NEW]
│   │   ├── finding.handlers.ts      # [NEW]
│   │   ├── scan.handlers.ts         # [NEW] (bao gồm SSE mock)
│   │   ├── asset.handlers.ts        # [NEW]
│   │   ├── product.handlers.ts      # [NEW]
│   │   └── admin.handlers.ts        # [NEW]
│   └── fixtures/
│       ├── dashboard.fixture.ts     # [NEW]
│       ├── cves.fixture.ts          # [NEW] — 50+ realistic CVEs
│       ├── kev.fixture.ts           # [NEW]
│       ├── findings.fixture.ts      # [NEW]
│       ├── scans.fixture.ts         # [NEW]
│       ├── assets.fixture.ts        # [NEW]
│       └── users.fixture.ts         # [NEW]
│
├── styles/
│   ├── globals.css                  # → UPDATE — base styles + Tailwind
│   ├── theme.css                    # [NEW] — CSS design tokens (dark/light)
│   ├── fonts.css                    # [NEW] — Inter font import
│   └── animations.css              # [NEW] — Keyframe animations
│
└── test/
    ├── setup.ts                     # [NEW] — Vitest global setup
    └── utils.tsx                    # [NEW] — createTestProviders helper
```

---

## 2. Migration Map (Current → Target)

| Current File | Target Location | Action | Priority |
|-------------|----------------|--------|----------|
| `app/App.tsx` | `app/router.tsx` + `app/providers.tsx` | REPLACE | P0 |
| `app/components/LoginScreen.tsx` | `features/auth/components/LoginScreen.tsx` | MOVE + MIGRATE | P1 |
| `app/components/Sidebar.tsx` | `app/components/Sidebar.tsx` | KEEP (minor update) | P2 |
| `app/components/Topbar.tsx` | `app/components/Topbar.tsx` | KEEP | P2 |
| `app/components/Dashboard.tsx` | `features/dashboard/components/Dashboard.tsx` | MOVE + MIGRATE | P0 |
| `app/components/CVESearch.tsx` | `features/cve-intel/components/CVESearch.tsx` | MOVE + MIGRATE | P0 |
| `app/components/KEVCatalog.tsx` | `features/cve-intel/components/KEVCatalog.tsx` | MOVE + MIGRATE | P1 |
| `app/components/SemanticSearch.tsx` | `features/cve-intel/components/SemanticSearch.tsx` | MOVE + MIGRATE | P1 |
| `app/components/EPSSAnalytics.tsx` | `features/cve-intel/components/EPSSAnalytics.tsx` | MOVE + MIGRATE | P2 |
| `app/components/VendorCatalog.tsx` | `features/cve-intel/components/VendorCatalog.tsx` | MOVE + MIGRATE | P2 |
| `app/components/CWELibrary.tsx` | `features/cve-intel/components/CWELibrary.tsx` | MOVE + MIGRATE | P2 |
| `app/components/ScanDashboard.tsx` | `features/scanning/components/ScanDashboard.tsx` | MOVE + MIGRATE | P1 |
| `app/components/ScanWizard.tsx` | `features/scanning/components/ScanWizard.tsx` | MOVE + MIGRATE + RHF | P1 |
| `app/components/RunningScan.tsx` | `features/scanning/components/RunningScan.tsx` | MOVE + SSE | P1 |
| `app/components/ScanHistory.tsx` | `features/scanning/components/ScanHistory.tsx` | MOVE + MIGRATE | P2 |
| `app/components/NmapResults.tsx` | `features/scanning/components/NmapResults.tsx` | MOVE + MIGRATE | P2 |
| `app/components/ZAPResults.tsx` | `features/scanning/components/ZAPResults.tsx` | MOVE + MIGRATE | P2 |
| `app/components/FindingsList.tsx` | `features/findings/components/FindingsList.tsx` | MOVE + MIGRATE | P1 |
| `app/components/FindingDetail.tsx` | `features/findings/components/FindingDetail.tsx` | MOVE + MIGRATE | P1 |
| `app/components/SLADashboard.tsx` | `features/findings/components/SLADashboard.tsx` | MOVE + MIGRATE | P2 |
| `app/components/RiskAcceptanceCenter.tsx` | `features/findings/components/RiskAcceptanceCenter.tsx` | MOVE + MIGRATE | P2 |
| `app/components/AssetInventory.tsx` | `features/assets/components/AssetInventory.tsx` | MOVE + MIGRATE | P2 |
| `app/components/AssetDetail.tsx` | `features/assets/components/AssetDetail.tsx` | MOVE + MIGRATE | P2 |
| `app/components/ProductSecurity.tsx` | `features/product-security/components/ProductSecurity.tsx` | MOVE + MIGRATE | P2 |
| `app/components/AITriage.tsx` | `features/ai-center/components/AITriage.tsx` | MOVE + MIGRATE | P3 |
| `app/components/AIEnrichment.tsx` | `features/ai-center/components/AIEnrichment.tsx` | MOVE + MIGRATE | P3 |
| `app/components/ReportCenter.tsx` | `features/reports/components/ReportCenter.tsx` | MOVE + MIGRATE | P3 |
| `app/components/NotificationCenter.tsx` | `features/notifications/components/NotificationCenter.tsx` | MOVE + MIGRATE | P3 |
| `app/components/APIKeyManagement.tsx` | `features/integrations/components/APIKeyManagement.tsx` | MOVE + MIGRATE | P3 |
| `app/components/WebhookEvents.tsx` | `features/integrations/components/WebhookEvents.tsx` | MOVE + MIGRATE | P3 |
| `app/components/UserManagement.tsx` | `features/admin/components/UserManagement.tsx` | MOVE + MIGRATE | P3 |
| `app/components/RBACManagement.tsx` | `features/admin/components/RBACManagement.tsx` | MOVE + MIGRATE | P3 |
| `app/components/AuditLogs.tsx` | `features/admin/components/AuditLogs.tsx` | MOVE + MIGRATE | P3 |
| `app/components/SystemHealth.tsx` | `features/admin/components/SystemHealth.tsx` | MOVE + MIGRATE | P3 |
| `app/components/SystemSettings.tsx` | `features/admin/components/SystemSettings.tsx` | MOVE + MIGRATE | P3 |
| `app/components/GlobalSearch.tsx` | `shared/components/global/CommandPalette.tsx` | MOVE + UPGRADE | P3 |
| `app/components/UserProfile.tsx` | `features/auth/components/UserProfile.tsx` | MOVE + MIGRATE | P3 |
| `app/components/OnboardingExperience.tsx` | `features/auth/components/OnboardingExperience.tsx` | MOVE + MIGRATE | P3 |

**Legend:**
- **MOVE**: Chỉ di chuyển file, không thay đổi logic
- **MIGRATE**: Di chuyển + xóa hardcode + kết nối React Query
- **REPLACE**: Xóa hoàn toàn, viết lại từ đầu theo architecture
- **UPGRADE**: Di chuyển + nâng cấp significant tính năng

---

## 3. Migration Strategy: Incremental Approach

### 3.1 Chiến lược không break production

```
Bước 1: Giữ nguyên app/App.tsx
Bước 2: Thêm features/ folder song song với app/components/
Bước 3: Migrate từng module trong features/
Bước 4: Update imports trong App.tsx → router.tsx dần dần
Bước 5: Sau khi tất cả routes ổn định → xóa app/components/
```

### 3.2 Feature Flag cho migration

```typescript
// Trong giai đoạn migration, có thể dùng env var để bật/tắt router mới
// .env.development
VITE_USE_NEW_ROUTER=true

// app/App.tsx (temporary)
import { RouterProvider } from 'react-router';
import { router } from './router';
import LegacyApp from './LegacyApp';  // Old App.tsx renamed

export default function App() {
  if (import.meta.env.VITE_USE_NEW_ROUTER === 'true') {
    return <RouterProvider router={router} />;
  }
  return <LegacyApp />;
}
```

---

## 4. New Files to Create (Complete List)

### 4.1 Foundation (Phase 1 — P0)

| File | Description |
|------|-------------|
| `src/shared/types/api.ts` | APIError, PaginatedResponse |
| `src/shared/types/auth.ts` | User, UserRole, Permission |
| `src/shared/types/cve.ts` | CVE, KEVEntry, CWEDetail |
| `src/shared/types/finding.ts` | Finding, FindingStatus |
| `src/shared/types/scan.ts` | Scan, ScanProgress |
| `src/shared/api/client.ts` | Axios + interceptors |
| `src/shared/api/queryClient.ts` | QueryClient + key factories |
| `src/shared/api/endpoints.ts` | All API endpoints |
| `src/features/auth/store/authStore.ts` | Zustand persist store |
| `src/app/router.tsx` | React Router v7 config |
| `src/app/components/AuthGuard.tsx` | Auth guard wrapper |
| `src/app/providers.tsx` | App-level providers |
| `src/shared/components/feedback/QueryBoundary.tsx` | Query wrapper |
| `src/mocks/browser.ts` | MSW browser worker |
| `src/mocks/server.ts` | MSW Node server (tests) |
| `src/mocks/handlers/index.ts` | Handler aggregator |

### 4.2 API + Hooks (Phase 2 — P1/P2)

| File | Description |
|------|-------------|
| `src/features/dashboard/types.ts` | Dashboard-specific types |
| `src/features/dashboard/api/dashboardApi.ts` | Dashboard API calls |
| `src/features/dashboard/hooks/useDashboardMetrics.ts` | Dashboard query hook |
| `src/features/cve-intel/api/cveApi.ts` | CVE API (search, semantic, detail) |
| `src/features/cve-intel/api/kevApi.ts` | KEV API |
| `src/features/cve-intel/api/taxonomyApi.ts` | Browse, CWE, CAPEC |
| `src/features/cve-intel/hooks/useCVESearch.ts` | CVE search hook |
| `src/features/cve-intel/hooks/useCVEDetail.ts` | CVE detail hook |
| `src/features/cve-intel/hooks/useKEVCatalog.ts` | KEV list hook |
| `src/features/findings/api/findingApi.ts` | Finding CRUD |
| `src/features/findings/api/riskAcceptanceApi.ts` | Risk acceptance API |
| `src/features/findings/hooks/useFindings.ts` | Findings list + filter |
| `src/features/findings/hooks/useUpdateFinding.ts` | Status mutation |
| `src/features/scanning/api/scanApi.ts` | Scan CRUD |
| `src/features/scanning/hooks/useScanSSE.ts` | SSE scan progress |
| `src/features/scanning/schemas/scanWizardSchema.ts` | Zod form schema |
| `src/shared/hooks/useSSE.ts` | Generic SSE hook |
| `src/mocks/handlers/dashboard.handlers.ts` | Dashboard MSW |
| `src/mocks/handlers/cve.handlers.ts` | CVE MSW |
| `src/mocks/handlers/finding.handlers.ts` | Finding MSW |
| `src/mocks/handlers/scan.handlers.ts` | Scan + SSE MSW |
| `src/mocks/fixtures/dashboard.fixture.ts` | Dashboard fixture data |
| `src/mocks/fixtures/cves.fixture.ts` | 50+ CVE fixture |
| `src/mocks/fixtures/findings.fixture.ts` | Findings fixture |
| `src/mocks/fixtures/scans.fixture.ts` | Scans fixture |

### 4.3 Shared Components (Phase 2/3)

| File | Description |
|------|-------------|
| `src/shared/components/data-display/DataTable.tsx` | Generic table |
| `src/shared/components/data-display/SeverityBadge.tsx` | Severity chip |
| `src/shared/components/data-display/StatusBadge.tsx` | Finding status chip |
| `src/shared/components/data-display/SLABadge.tsx` | SLA countdown badge |
| `src/shared/components/data-display/EPSSBar.tsx` | EPSS score bar |
| `src/shared/components/data-display/KEVIndicator.tsx` | KEV chip/icon |
| `src/shared/components/data-display/GradeCircle.tsx` | Circular grade display |
| `src/shared/components/feedback/ErrorState.tsx` | Error display |
| `src/shared/components/feedback/EmptyState.tsx` | Empty state display |
| `src/shared/components/feedback/LoadingSpinner.tsx` | Spinner |
| `src/shared/utils/severity.ts` | SEVERITY_COLORS, helpers |
| `src/shared/utils/sla.ts` | SLA calculation |
| `src/shared/utils/productGrade.ts` | Grade calculation |
| `src/shared/utils/findingStateMachine.ts` | Status transitions |
| `src/shared/utils/date.ts` | Date formatting |
| `src/shared/utils/cn.ts` | clsx + tailwind-merge |

### 4.4 Testing (Phase 3)

| File | Description |
|------|-------------|
| `src/test/setup.ts` | Vitest global setup |
| `src/test/utils.tsx` | createTestProviders helper |
| `src/shared/utils/__tests__/severity.test.ts` | Severity utils tests |
| `src/shared/utils/__tests__/sla.test.ts` | SLA utils tests |
| `src/shared/utils/__tests__/findingStateMachine.test.ts` | State machine tests |
| `src/features/dashboard/hooks/__tests__/useDashboardMetrics.test.tsx` | Hook test |
| `src/features/cve-intel/hooks/__tests__/useCVESearch.test.tsx` | Hook test |
| `src/features/dashboard/components/__tests__/Dashboard.test.tsx` | Component test |
| `eslint-rules/no-hardcode-mock-data.js` | Custom ESLint rule |
| `e2e/auth.spec.ts` | Auth E2E flow |
| `e2e/cve.spec.ts` | CVE Intelligence E2E |
| `e2e/scanning.spec.ts` | Scanning E2E |
| `playwright.config.ts` | Playwright config |

---

## 5. Dependencies to Install

```bash
# Runtime (required)
pnpm add \
  zustand \
  @tanstack/react-query \
  @tanstack/react-query-devtools \
  axios \
  msw \
  zod \
  @hookform/resolvers \
  @tanstack/react-virtual

# Dev (testing)
pnpm add -D \
  vitest \
  @testing-library/react \
  @testing-library/user-event \
  @testing-library/jest-dom \
  @vitejs/plugin-react \
  jsdom \
  @playwright/test

# Already in package.json (no need to add):
# react-hook-form, react-router, recharts, lucide-react, sonner, @radix-ui/*
```

---

## 6. tsconfig.json Updates

```json
{
  "compilerOptions": {
    "strict": true,
    "baseUrl": ".",
    "paths": {
      "@/*": ["src/*"]
    },
    "types": ["vitest/globals", "@testing-library/jest-dom"]
  }
}
```
