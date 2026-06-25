# Solutions Index — UI-API Change Requests

**Series:** UI-API v2  
**Ngày tạo:** 2026-06-16  
**Trạng thái:** Proposed  
**Phạm vi:** Frontend React SPA (React 18 + TypeScript + Vite)

---

## Mục tiêu

Bộ 10 Solution documents này định nghĩa **cách frontend implement** tương ứng với từng Change Request trong series UI-API v2. Mỗi solution tuân thủ nghiêm ngặt nguyên tắc **API-First / No-Hardcode Data** được định nghĩa trong [Architecture](../../../../ui/specs/architecture.md).

---

## Danh sách Solutions

| Solution | CR nguồn | Ưu tiên | Backend Status | Frontend Phase |
|----------|---------|---------|----------------|---------------|
| [SOL-UI-001](./SOL-UI-001-auth-api.md) | CR-UI-001 Auth API | P0 🔴 | ❌ New | Sprint 1 |
| [SOL-UI-002](./SOL-UI-002-dashboard-api.md) | CR-UI-002 Dashboard KPI | P0 🔴 | ❌ New | Sprint 1 |
| [SOL-UI-003](./SOL-UI-003-cve-intel-api.md) | CR-UI-003 CVE Intelligence | P0 🔴 | ⚠️ Schema Update | Sprint 1-3 |
| [SOL-UI-004](./SOL-UI-004-scanning-api.md) | CR-UI-004 Active Scanning | P1 🟡 | ❌ v3.0 | Sprint 4 |
| [SOL-UI-005](./SOL-UI-005-finding-api.md) | CR-UI-005 Finding Management | P0 🔴 | ⚠️ Schema Update | Sprint 1-2 |
| [SOL-UI-006](./SOL-UI-006-asset-api.md) | CR-UI-006 Asset Management | P1 🟡 | ❌ v3.0 | Sprint 4 |
| [SOL-UI-007](./SOL-UI-007-product-api.md) | CR-UI-007 Product Security | P0 🔴 | ⚠️ New Endpoints | Sprint 2 |
| [SOL-UI-008](./SOL-UI-008-ai-center-api.md) | CR-UI-008 AI Center | P1 🟡 | ❌ v3.0 | Sprint 4 |
| [SOL-UI-009](./SOL-UI-009-reports-notifications-api.md) | CR-UI-009 Reports & Notifications | P0 🔴 | ⚠️ New Endpoints | Sprint 2 |
| [SOL-UI-010](./SOL-UI-010-admin-integrations-api.md) | CR-UI-010 Admin & Integrations | P0 🔴 | ⚠️ New Endpoints | Sprint 2 |

---

## Architecture Patterns (Chung cho tất cả Solutions)

### API Call Pattern
```
Component
  └─ Custom Hook (useFeature*)
       └─ React Query (useQuery / useMutation)
            └─ API Module (*Api.ts)
                 └─ Axios Client (shared/api/client.ts)
                      └─ MSW Handler (dev) / Real API (prod)
```

### Key Conventions

| Aspect | Rule |
|--------|------|
| Data fetching | 100% qua React Query — không useState cho server data |
| Mutations | useMutation + onSuccess invalidateQueries |
| URL state | Filter/pagination → useSearchParams (React Router) |
| Loading state | Skeleton UI (không spinner toàn màn hình) |
| Error state | `<ErrorState onRetry={refetch} />` component |
| Empty state | `<EmptyState />` khi list rỗng |
| Auth token | Zustand in-memory ONLY — không localStorage |
| Hardcode data | ❌ TUYỆT ĐỐI KHÔNG — chỉ trong `src/mocks/` |
| SSE auth | Query param `?token=` (header không hỗ trợ) |

---

## MSW Handler Registration

Tất cả handlers đăng ký trong `src/mocks/browser.ts`:

```typescript
import { setupWorker } from 'msw/browser';
import { authHandlers } from './handlers/auth.handlers';
import { dashboardHandlers } from './handlers/dashboard.handlers';
import { cveHandlers } from './handlers/cve.handlers';
import { scanHandlers } from './handlers/scan.handlers';
import { findingHandlers } from './handlers/finding.handlers';
import { assetHandlers } from './handlers/asset.handlers';
import { productHandlers } from './handlers/product.handlers';
import { aiHandlers } from './handlers/ai.handlers';
import { reportHandlers } from './handlers/report.handlers';
import { notificationHandlers } from './handlers/notification.handlers';
import { adminHandlers } from './handlers/admin.handlers';
import { integrationHandlers } from './handlers/integration.handlers';

export const worker = setupWorker(
  ...authHandlers,
  ...dashboardHandlers,
  ...cveHandlers,
  ...scanHandlers,
  ...findingHandlers,
  ...assetHandlers,
  ...productHandlers,
  ...aiHandlers,
  ...reportHandlers,
  ...notificationHandlers,
  ...adminHandlers,
  ...integrationHandlers,
);
```

---

## React Query Key Factories

| Module | Root Key | Import |
|--------|---------|--------|
| Auth | `['auth']` | authStore (Zustand, no Query) |
| Dashboard | `['dashboard']` | `dashboardKeys` |
| CVE | `['cves']` | `cveKeys` |
| Scans | `['scans']` | `scanKeys` |
| Findings | `['findings']` | `findingKeys` |
| Assets | `['assets']` | `assetKeys` |
| Products | `['products']` | `productKeys` |
| AI | `['ai']` | `aiKeys` |
| Reports | `['reports']` | `reportKeys` |
| Notifications | `['notifications']` | `notifKeys` |
| Admin | `['admin', ...]` | `adminKeys` |

---

## Implementation Priority (Sprint Mapping)

### Sprint 1 — P0 Blocking

1. **SOL-UI-001** — Auth (login/refresh/me/logout + Axios interceptor)
2. **SOL-UI-002** §1 — Dashboard aggregate endpoint
3. **SOL-UI-003** — CVE Search schema fix (`is_kev`, `epss_percentile`, `aggregations`)
4. **SOL-UI-005** §1 — Findings schema fix + bulk/reopen + bulk/assign

### Sprint 2 — P0 Core

5. **SOL-UI-007** — Product grades + finding_summary
6. **SOL-UI-009** §1 — Reports CRUD + download
7. **SOL-UI-009** §2 — Notifications list/read
8. **SOL-UI-010** §1 — API Keys management
9. **SOL-UI-010** §2 — Admin User Management

### Sprint 3 — Intelligence

10. **SOL-UI-003** §2 — New endpoints: vendors, epss/top, epss/distribution, cwe list
11. **SOL-UI-002** §2 — Notifications SSE stream
12. **SOL-UI-010** §3 — System Health

### Sprint 4 — v3.0 Features

13. **SOL-UI-004** — Active Scanning API (CR-OVS-001)
14. **SOL-UI-006** — Asset Management API (CR-OVS-007)
15. **SOL-UI-008** — AI Center API (CR-OVS-005)
16. **SOL-UI-001** §MFA — MFA + OAuth2 (CR-OVS-003)

---

## Shared Components Referenced

| Component | Solution | Source |
|-----------|---------|--------|
| `GradeCircle` | SOL-UI-007 | `features/product-security/components/` |
| `SeverityBadge` | SOL-UI-003, 005 | `shared/components/` |
| `SLABadge` | SOL-UI-005, 002 | `shared/components/` |
| `DataTable` | All | `shared/components/` |
| `EmptyState` | All | `shared/components/` |
| `ErrorState` | All | `shared/components/` |
| `QueryBoundary` | All | `shared/components/` |
| `SSEProgressBar` | SOL-UI-004 | `features/scanning/components/` |
| `NotificationBell` | SOL-UI-009 | `features/notifications/components/` |
| `AuthGuard` | All routes | `app/components/` |

---

## Checklist Trước Khi Kết Nối Real API

- [ ] `VITE_ENABLE_MSW=false` trong `.env.development.local`
- [ ] `VITE_API_BASE_URL` trỏ đúng backend
- [ ] Xác nhận CORS header cho origin frontend
- [ ] Kiểm tra `withCredentials: true` cho refresh cookie
- [ ] Verify `/api/v1/notifications/stream?token=` SSE auth hoạt động
- [ ] Run ESLint `no-hardcode-mock-data` rule — không được có vi phạm
- [ ] Run `vitest` — tất cả API hooks tests phải pass với MSW server
