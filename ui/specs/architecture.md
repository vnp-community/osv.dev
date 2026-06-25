# Frontend Architecture — OSV Platform UI

**Version:** 1.1  
**Ngày tạo:** 2026-06-14  
**Cập nhật:** 2026-06-14  
**Trạng thái:** Active  
**Tác giả:** Based on analysis of PRD v3.0, SRS v3.0, URD v3.0

> [!IMPORTANT]
> **API-First Principle**: Mọi dữ liệu hiển thị trên UI **PHẢI** được lấy từ server thông qua API. **NGHIÊM CẤM** hardcode dữ liệu (`const data = [...]`) trong component. Xem **Section 5.5** để biết quy tắc và **Section 5.6** cho MSW mock layer khi phát triển.

---

## 1. Tổng quan

OSV Platform UI là **Single-Page Application (SPA)** phục vụ 6 nhóm người dùng (Alice/Bob/Carol/Dave/Eve/Agent) với ~60 màn hình, tích hợp với hệ thống backend Go Microservices thông qua **Unified Gateway** (:8080 REST).

### 1.1 Mục tiêu kiến trúc

| Mục tiêu | Yêu cầu |
|-----------|---------|
| **Performance** | LCP < 2s (NFR-U-04), Time-to-Interactive < 3s |
| **Real-time** | SSE scan progress < 2s latency (NFR-U-05) |
| **Scalability** | Hỗ trợ 60+ screens, ~300K CVEs, phân trang server-side |
| **Maintainability** | Feature-based module structure, clear separation of concerns |
| **Security** | JWT RS256 token management, RBAC-based UI rendering |
| **DX** | TypeScript strict mode, hot reload, component testing |
| **API-First** | **Mọi dữ liệu từ server** — không hardcode, MSW cho local dev |

---

## 2. Technology Stack

### 2.1 Core Stack (Hiện tại → Đề xuất)

| Layer | Hiện tại | Đề xuất | Lý do |
|-------|----------|---------|-------|
| Framework | React 18 + TypeScript | **React 18 + TypeScript** (giữ) | Stable, large ecosystem |
| Build tool | Vite | **Vite 5** | Fast HMR, native ESM |
| Styling | Tailwind CSS + inline styles | **Tailwind CSS + CSS Variables** | Consistent design tokens |
| UI Primitives | shadcn/ui (Radix) | **shadcn/ui (Radix)** (giữ) | Accessible, headless |
| State | `useState` (local) | **Zustand + React Query** | Global state + server state |
| Routing | Custom view switch | **React Router v7** | URL-based navigation, deep links |
| Charts | Recharts | **Recharts** (giữ) | Lightweight, composable |
| Icons | Lucide React | **Lucide React** (giữ) | Consistent icon set |
| HTTP Client | — (mock data) | **Axios + React Query** | Interceptors, cache, retry |
| Real-time | — | **EventSource (SSE)** | Scan progress streaming |
| Forms | Native `useState` | **React Hook Form + Zod** | Validation, performance |
| **Mock API** | Hardcoded arrays | **MSW (Mock Service Worker)** | API mock khi dev, không hardcode data |
| Testing | — | **Vitest + React Testing Library** | Fast, co-located |
| E2E | — | **Playwright** | Cross-browser automation |

### 2.2 Development Tools

```
Node.js: 22 LTS
Package manager: pnpm (workspace monorepo support)
Linter: ESLint + TypeScript ESLint
Formatter: Prettier
Pre-commit: Husky + lint-staged  (bao gồm check no-hardcode-data)
CI: GitHub Actions
```

### 2.3 API-First Development Workflow

```
┌─────────────────────────────────────────────────────────────┐
│  DEVELOPMENT                       PRODUCTION               │
│                                                             │
│  Browser ──►  MSW Service Worker  ──► Mock Response         │
│         (src/mocks/handlers/*.ts)                           │
│                                                             │
│  Browser ──►  Real API Gateway    ──► Go Microservices      │
│         (VITE_API_BASE_URL=...)                             │
└─────────────────────────────────────────────────────────────┘

Quy tắc:
1. Viết API handler trong MSW TRƯỚC khi viết component
2. Component chỉ nhận data qua React Query hook
3. Không có const mockData = [...] trong component files
4. Khi backend sẵn sàng → tắt MSW → zero code change cần thiết
```

---

## 3. Cấu trúc thư mục (Target)

```
ui/
├── src/
│   ├── main.tsx                    # Entry point
│   ├── app/
│   │   ├── App.tsx                 # Root component + providers
│   │   ├── router.tsx              # React Router configuration
│   │   └── providers.tsx           # QueryClient, Auth, Theme providers
│   │
│   ├── features/                   # Feature-based modules (Đổi từ flat components)
│   │   ├── auth/
│   │   │   ├── components/         # LoginScreen, MFASetup, OAuthCallback
│   │   │   ├── hooks/              # useAuth, useUser, usePermissions
│   │   │   ├── api/                # authApi.ts (login, refresh, logout)
│   │   │   ├── store/              # authStore.ts (Zustand)
│   │   │   └── types.ts
│   │   │
│   │   ├── dashboard/
│   │   │   ├── components/         # Dashboard, KPICard, RiskTrendChart
│   │   │   ├── hooks/              # useDashboardMetrics, useSLASummary
│   │   │   ├── api/                # dashboardApi.ts
│   │   │   └── types.ts
│   │   │
│   │   ├── cve-intel/              # CVE Intelligence module
│   │   │   ├── components/         # CVESearch, KEVCatalog, SemanticSearch,
│   │   │   │                       # VendorCatalog, CWELibrary, EPSSAnalytics
│   │   │   ├── hooks/              # useCVESearch, useKEVCatalog, useSemantic
│   │   │   ├── api/                # cveApi.ts, kevApi.ts, taxonomyApi.ts
│   │   │   └── types.ts
│   │   │
│   │   ├── scanning/               # Active Scanning module
│   │   │   ├── components/         # ScanDashboard, ScanWizard, RunningScan,
│   │   │   │                       # ScanHistory, NmapResults, ZAPResults
│   │   │   ├── hooks/              # useScan, useScanSSE, useScanHistory
│   │   │   ├── api/                # scanApi.ts
│   │   │   └── types.ts
│   │   │
│   │   ├── findings/               # Finding Management module
│   │   │   ├── components/         # FindingsList, FindingDetail,
│   │   │   │                       # RiskAcceptanceCenter, SLADashboard
│   │   │   ├── hooks/              # useFindings, useFindingDetail, useBulkOps
│   │   │   ├── api/                # findingApi.ts, riskAcceptanceApi.ts
│   │   │   └── types.ts
│   │   │
│   │   ├── assets/                 # Asset Management module
│   │   │   ├── components/         # AssetInventory, AssetDetail
│   │   │   ├── hooks/              # useAssets, useAssetDetail
│   │   │   ├── api/                # assetApi.ts
│   │   │   └── types.ts
│   │   │
│   │   ├── product-security/       # Product/Engagement/Test hierarchy
│   │   │   ├── components/         # ProductSecurity, ProductDetail,
│   │   │   │                       # EngagementList, Scorecards
│   │   │   ├── hooks/              # useProducts, useEngagements
│   │   │   ├── api/                # productApi.ts
│   │   │   └── types.ts
│   │   │
│   │   ├── ai-center/              # AI Triage & Enrichment module
│   │   ├── cve-intel/
│   │   ├── scanning/
│   │   ├── findings/
│   │   ├── assets/
│   │   ├── product-security/
│   │   ├── ai-center/
│   │   ├── reports/
│   │   ├── notifications/
│   │   ├── integrations/
│   │   └── admin/
│   │
│   ├── shared/                     # Shared across features
│   │   │   └── useLocalStorage.ts  # Persistent user preferences
│   │   │
│   │   ├── api/
│   │   │   ├── client.ts           # Axios instance + interceptors
│   │   │   ├── queryClient.ts      # React Query configuration
│   │   │   └── endpoints.ts        # API endpoint constants
│   │   │
│   │   ├── utils/
│   │   │   ├── severity.ts         # SEVERITY_COLORS, CVSS_COLOR helpers
│   │   │   ├── date.ts             # Date formatting utilities
│   │   │   ├── sla.ts              # SLA deadline calculation
│   │   │   └── cn.ts               # clsx/tailwind-merge helper
│   │   │
│   │   └── types/
│   │       ├── api.ts              # Base API response types
│   │       ├── cve.ts              # CVE, EPSS, KEV types
│   │       ├── finding.ts          # Finding, SLA, Audit types
│   │       ├── scan.ts             # Scan, Asset types
│   │       └── auth.ts             # User, Role, Permission types
│   │
│   └── styles/
│       ├── globals.css             # Base styles + Tailwind
│       ├── theme.css               # CSS design tokens (dark/light)
│       ├── fonts.css               # Inter font import
│       └── animations.css          # Keyframe animations
│
├── public/                         # Static assets
├── index.html
├── vite.config.ts
├── tailwind.config.ts
├── tsconfig.json
└── package.json
```

---

## 4. Architecture Layers

### 4.1 Layer Diagram

```
┌─────────────────────────────────────────────────────────────────┐
│                        PRESENTATION LAYER                        │
│  Pages/Views (feature components) + Layout (Sidebar, Topbar)    │
└────────────────────────────┬────────────────────────────────────┘
                             │ Props / Hooks
┌────────────────────────────▼────────────────────────────────────┐
│                       APPLICATION LAYER                          │
│  Custom Hooks (useFeature*) — orchestrate API + local state     │
│  React Query: server state, cache, mutations                     │
│  Zustand stores: auth, UI preferences, real-time updates         │
└────────────────────────────┬────────────────────────────────────┘
                             │ API calls
┌────────────────────────────▼────────────────────────────────────┐
│                      INFRASTRUCTURE LAYER                        │
│  Axios client (interceptors: auth headers, token refresh, retry) │
│  SSE EventSource (scan progress, notifications)                  │
│  React Query queryClient (cache strategy, stale-while-revalidate)│
└────────────────────────────┬────────────────────────────────────┘
                             │ HTTP/SSE
┌────────────────────────────▼────────────────────────────────────┐
│                    UNIFIED GATEWAY :8080                         │
│  JWT Auth + Rate Limiting + BFF aggregation                      │
│  REST /api/v1/ (scan, finding, auth, product, report, asset)     │
│  REST /api/v2/ (cve-search, kev, browse, taxonomy, webhooks)     │
│  SSE  /api/v1/scans/{id}/stream                                  │
└─────────────────────────────────────────────────────────────────┘
```

### 4.2 State Management Strategy

| State Type | Tool | Scope | Ví dụ |
|------------|------|-------|-------|
| **Server state** | React Query | Per-query cache | CVE list, findings, scans |
| **Auth state** | Zustand (persist) | Global | JWT token, user info, permissions |
| **UI state** | Zustand / useState | Local/Global | Sidebar expanded, active view, filters |
| **Form state** | React Hook Form | Local (form) | Scan wizard, finding form |
| **Real-time** | EventSource + Zustand | Global | Scan progress, notifications |
| **URL state** | React Router | URL params | Search query, pagination, filters |

### 4.3 Routing Architecture

```tsx
// router.tsx — React Router v7 (Data Router)
const router = createBrowserRouter([
  {
    path: "/",
    element: <AuthGuard><AppLayout /></AuthGuard>,
    children: [
      { index: true, element: <Navigate to="/dashboard" /> },
      
      // Dashboard
      { path: "dashboard", element: <Dashboard /> },
      { path: "dashboard/sla", element: <SLADashboard /> },
      
      // CVE Intelligence
      { path: "cve/search", element: <CVESearch /> },
      { path: "cve/semantic", element: <SemanticSearch /> },
      { path: "cve/kev", element: <KEVCatalog /> },
      { path: "cve/epss", element: <EPSSAnalytics /> },
      { path: "cve/vendors", element: <VendorCatalog /> },
      { path: "cve/cwe", element: <CWELibrary /> },
      { path: "cve/:id", element: <CVEDetail /> },          // Deep link CVE detail
      
      // Scanning
      { path: "scans", element: <ScanDashboard /> },
      { path: "scans/new", element: <ScanWizard /> },
      { path: "scans/:id", element: <ScanDetail /> },       // Deep link scan detail
      { path: "scans/:id/results/nmap", element: <NmapResults /> },
      { path: "scans/:id/results/zap", element: <ZAPResults /> },
      
      // Findings
      { path: "findings", element: <FindingsList /> },
      { path: "findings/:id", element: <FindingDetail /> },  // Deep link finding
      { path: "findings/risk-acceptance", element: <RiskAcceptanceCenter /> },
      
      // Assets
      { path: "assets", element: <AssetInventory /> },
      { path: "assets/:id", element: <AssetDetail /> },     // Deep link asset
      
      // Product Security
      { path: "products", element: <ProductSecurity /> },
      { path: "products/:id", element: <ProductDetail /> },
      
      // AI Center
      { path: "ai/triage", element: <AITriage /> },
      { path: "ai/enrichment", element: <AIEnrichment /> },
      
      // Reports
      { path: "reports", element: <ReportCenter /> },
      
      // Notifications
      { path: "notifications", element: <NotificationCenter /> },
      
      // Integrations
      { path: "integrations/api-keys", element: <APIKeyManagement /> },
      { path: "integrations/webhooks", element: <WebhookEvents /> },
      { path: "integrations/jira", element: <JiraConfig /> },
      
      // Admin
      { path: "admin/users", element: <UserManagement /> },
      { path: "admin/roles", element: <RBACManagement /> },
      { path: "admin/audit", element: <AuditLogs /> },
      { path: "admin/health", element: <SystemHealth /> },
      { path: "admin/settings", element: <SystemSettings /> },
      
      // User
      { path: "profile", element: <UserProfile /> },
      { path: "onboarding", element: <OnboardingExperience /> },
    ],
  },
  { path: "/login", element: <LoginScreen /> },
  { path: "/auth/callback", element: <OAuthCallback /> },   // OAuth2 Google/GitHub
]);
```

---

## 5. API Integration Architecture

### 5.1 Axios Client Configuration

```typescript
// shared/api/client.ts
const apiClient = axios.create({
  baseURL: import.meta.env.VITE_API_BASE_URL || 'http://localhost:8080',
  timeout: 30_000,
});

// Request interceptor: inject JWT
apiClient.interceptors.request.use((config) => {
  const token = authStore.getState().accessToken;
  if (token) config.headers.Authorization = `Bearer ${token}`;
  return config;
});

// Response interceptor: handle 401 → refresh token
apiClient.interceptors.response.use(
  (response) => response,
  async (error) => {
    if (error.response?.status === 401 && !error.config._retry) {
      error.config._retry = true;
      const newToken = await authApi.refreshToken();
      authStore.getState().setAccessToken(newToken);
      error.config.headers.Authorization = `Bearer ${newToken}`;
      return apiClient(error.config);
    }
    return Promise.reject(error);
  }
);
```

### 5.2 React Query Configuration

```typescript
// shared/api/queryClient.ts
const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      staleTime: 30_000,         // 30s — CVE data is fresh enough
      gcTime: 5 * 60 * 1000,    // 5 min cache
      retry: 2,
      refetchOnWindowFocus: false,
    },
    mutations: {
      onError: (error) => toast.error(error.message),
    },
  },
});

// Key factory pattern
export const cveKeys = {
  all: ['cves'] as const,
  search: (params: SearchParams) => [...cveKeys.all, 'search', params] as const,
  detail: (id: string) => [...cveKeys.all, 'detail', id] as const,
};
```

### 5.3 SSE (Server-Sent Events) Hook

```typescript
// shared/hooks/useSSE.ts
export function useSSE(url: string, enabled: boolean) {
  const [state, setState] = useState<SSEState>({ status: 'idle', data: null });

  useEffect(() => {
    if (!enabled) return;
    const source = new EventSource(url, {
      withCredentials: true,
    });

    source.onmessage = (e) => {
      setState({ status: 'streaming', data: JSON.parse(e.data) });
    };
    source.addEventListener('done', () => {
      setState(prev => ({ ...prev, status: 'done' }));
      source.close();
    });
    source.onerror = () => {
      setState({ status: 'error', data: null });
      source.close();
    };

    return () => source.close();
  }, [url, enabled]);

  return state;
}

// Usage in RunningScan.tsx
const { data: progress } = useSSE(
  `/api/v1/scans/${scanId}/stream`,
  scan?.status === 'running'
);
```

### 5.4 API Endpoint Map

| Feature | Endpoint (v1) | Endpoint (v2) | Method |
|---------|--------------|--------------|--------|
| CVE Search (FTS) | — | `POST /api/v2/cves/search` | POST |
| CVE Semantic | — | `POST /api/v2/cves/search/semantic` | POST |
| CVE Detail | — | `GET /api/v2/cves/{id}` | GET |
| CVE Export | — | `GET /api/v2/cves/export` | GET |
| KEV Catalog | — | `GET /api/v2/kev` | GET |
| KEV Stats | — | `GET /api/v2/kev/stats` | GET |
| Vendor Browse | — | `GET /api/v2/browse` | GET |
| CWE Lookup | — | `GET /api/v2/cwe/{id}` | GET |
| EPSS Query | — | `GET /api/v2/epss/{cve_id}` | GET |
| Scans List | `GET /api/v1/scans` | — | GET |
| Create Scan | `POST /api/v1/scans` | — | POST |
| Scan Progress | `GET /api/v1/scans/{id}/stream` | — | SSE |
| Cancel Scan | `POST /api/v1/scans/{id}/cancel` | — | POST |
| Findings List | `GET /api/v1/findings` | — | GET |
| Finding Detail | `GET /api/v1/findings/{id}` | — | GET |
| Update Finding | `PATCH /api/v1/findings/{id}` | — | PATCH |
| Bulk Close | `POST /api/v1/findings/bulk/close` | — | POST |
| Assets | `GET /api/v1/assets` | — | GET |
| Products | `GET /api/v1/products` | — | GET |
| Reports | `GET /api/v1/reports` | — | GET |
| Auth Login | `POST /api/v1/auth/login` | — | POST |
| Auth Refresh | `POST /api/v1/auth/refresh` | — | POST |
| API Keys | `GET /api/v1/api-keys` | — | GET |
| Webhooks | `GET /api/v1/webhooks` | — | GET |
| DB Stats | — | `GET /api/v2/dbinfo` | GET |

### 5.5 No-Hardcode Data Rule

> [!CAUTION]
> **VI PHẠM nghiêm trọng**: Bất kỳ `const data = [...]` hoặc `const mockData = {...}` nào đặt trực tiếp trong component files sẽ bị reject trong code review.

#### 5.5.1 Nguyên tắc

| ❌ KHÔNG làm | ✅ NÊN làm |
|-------------|----------|
| `const cveData = [{ id: 'CVE-2025-...' }]` trong component | `const { data } = useCVESearch(params)` |
| `const productData = [{ name: 'Banking', grade: 'B' }]` | `const { data } = useProducts()` |
| `const recentFindings = [{ id: 'F-2847', ... }]` | `const { data } = useFindings({ limit: 5 })` |
| Render static numbers như `<div>245</div>` cho metrics | `<div>{kpis?.criticalFindings ?? <Skeleton />}</div>` |
| `useState(mockScanData)` khởi tạo với data giả | `useQuery(...)` fetch từ API |

#### 5.5.2 Quy trình kiểm soát

```
1. ESLint custom rule: no-hardcode-mock-data
   → Cảnh báo khi phát hiện array literals > 2 phần tử trong component scope

2. Code Review Checklist:
   □ Component không có array/object literals lớn ở top-level
   □ Mọi data display đều thông qua useQuery/useMutation
   □ Loading state: hiển thị Skeleton khi isLoading=true
   □ Error state: hiển thị ErrorState khi isError=true
   □ Empty state: hiển thị EmptyState khi data=[] hoặc data=null

3. CI Gate:
   → grep -r 'const.*Data.*=.*\[' src/features/ --include='*.tsx'
   → Fail build nếu tìm thấy pattern này
```

#### 5.5.3 Component Template (Server Data)

```typescript
// ✅ ĐÚNG — Template chuẩn cho mọi feature component
export function FindingsList() {
  const { filters } = useFindingsFilters();          // URL state
  const { data, isLoading, isError, error } = useFindings(filters); // Server state

  if (isLoading) return <FindingsListSkeleton />;    // Skeleton UI
  if (isError)   return <ErrorState message={error.message} onRetry={refetch} />;
  if (!data?.findings.length) return <EmptyState title="No findings found" />;

  return (
    <DataTable
      columns={FINDINGS_COLUMNS}
      data={data.findings}       // ← Từ server, không hardcode
      total={data.total}         // ← Server-side pagination
      ...
    />
  );
}

// ❌ SAI — Hardcode data trong component
export function FindingsList() {
  const findings = [           // ← VI PHẠM: hardcode data
    { id: 'F-2847', cve: 'CVE-2025-44228', ... },
    { id: 'F-2846', ... },
  ];
  return <DataTable data={findings} />;
}
```

#### 5.5.4 Loading & Error State Pattern

```typescript
// shared/components/feedback/QueryBoundary.tsx
// Wrapper tự động handle loading/error cho mọi query
export function QueryBoundary<T>({
  query,
  skeleton,
  children,
}: {
  query: UseQueryResult<T>;
  skeleton: React.ReactNode;
  children: (data: T) => React.ReactNode;
}) {
  if (query.isLoading) return <>{skeleton}</>;
  if (query.isError)   return <ErrorState message={query.error.message} />;
  if (!query.data)     return <EmptyState />;
  return <>{children(query.data)}</>;
}

// Usage:
<QueryBoundary
  query={useCVESearch(params)}
  skeleton={<CVETableSkeleton rows={10} />}
>
  {(data) => <CVETable cves={data.cves} total={data.total} />}
</QueryBoundary>
```

---

### 5.6 MSW Mock Layer (Development)

Khi backend chưa sẵn sàng, sử dụng **MSW (Mock Service Worker)** để intercept HTTP requests và trả về mock responses **có cấu trúc giống thật**. Fixture data được đặt trong `src/mocks/fixtures/`, **không** trong component files.

#### 5.6.1 Setup

```typescript
// src/mocks/browser.ts
import { setupWorker } from 'msw/browser';
import { handlers } from './handlers';

export const worker = setupWorker(...handlers);

// src/main.tsx
async function enableMocking() {
  if (import.meta.env.VITE_ENABLE_MSW !== 'true') return;
  const { worker } = await import('./mocks/browser');
  return worker.start({
    onUnhandledRequest: 'warn',  // Cảnh báo khi có request không có handler
  });
}

enableMocking().then(() => {
  createRoot(document.getElementById('root')!).render(<App />);
});
```

#### 5.6.2 Handler Examples

```typescript
// src/mocks/handlers/cve.handlers.ts
import { http, HttpResponse } from 'msw';
import { cvesFixture } from '../fixtures/cves.fixture';

export const cveHandlers = [
  // POST /api/v2/cves/search
  http.post('/api/v2/cves/search', async ({ request }) => {
    const body = await request.json() as CVESearchRequest;
    
    let results = cvesFixture;
    
    // Apply same filters as real backend
    if (body.severity?.length) {
      results = results.filter(c => body.severity!.includes(c.severity));
    }
    if (body.query) {
      const q = body.query.toLowerCase();
      results = results.filter(c =>
        c.id.toLowerCase().includes(q) ||
        c.description.toLowerCase().includes(q) ||
        c.vendor.toLowerCase().includes(q)
      );
    }
    if (body.kevOnly) {
      results = results.filter(c => c.isKEV);
    }
    
    // Server-side pagination
    const page = body.page ?? 1;
    const pageSize = body.pageSize ?? 50;
    const start = (page - 1) * pageSize;
    const paginated = results.slice(start, start + pageSize);
    
    return HttpResponse.json({
      data: paginated,
      total: results.length,
      page,
      pageSize,
    } satisfies CVESearchResponse);
  }),

  // GET /api/v2/cves/:id
  http.get('/api/v2/cves/:id', ({ params }) => {
    const cve = cvesFixture.find(c => c.id === params.id);
    if (!cve) return new HttpResponse(null, { status: 404 });
    return HttpResponse.json(cve);
  }),

  // GET /api/v2/kev
  http.get('/api/v2/kev', () => {
    const { kevEntriesFixture } = require('../fixtures/kev.fixture');
    return HttpResponse.json({
      entries: kevEntriesFixture,
      total: kevEntriesFixture.length,
    });
  }),
];
```

```typescript
// src/mocks/handlers/scan.handlers.ts — SSE mock
http.get('/api/v1/scans/:id/stream', ({ params }) => {
  const encoder = new TextEncoder();
  let progress = 0;

  const stream = new ReadableStream({
    async start(controller) {
      while (progress < 100) {
        await new Promise(r => setTimeout(r, 500));  // 500ms interval
        progress += Math.floor(Math.random() * 15) + 5;
        progress = Math.min(progress, 100);
        
        const data = JSON.stringify({
          scanId: params.id,
          status: progress < 100 ? 'running' : 'completed',
          progress,
          findingsFound: Math.floor(progress * 0.5),
        } satisfies ScanProgress);
        
        controller.enqueue(encoder.encode(`data: ${data}\n\n`));
      }
      
      controller.enqueue(encoder.encode(`event: done\ndata: {}\n\n`));
      controller.close();
    },
  });

  return new HttpResponse(stream, {
    headers: {
      'Content-Type': 'text/event-stream',
      'Cache-Control': 'no-cache',
    },
  });
}),
```

#### 5.6.3 Fixture Structure

```typescript
// src/mocks/fixtures/cves.fixture.ts
// Fixture data — realistic, đủ đa dạng, KHÔNG dùng trong component
import type { CVE } from '@/shared/types/cve';

export const cvesFixture: CVE[] = [
  {
    id: 'CVE-2025-44228',
    severity: 'Critical',
    cvssV3: 10.0,
    epssScore: 0.982,
    epssPercentile: 0.999,
    isKEV: true,
    vendor: 'Apache',
    product: 'Log4j2',
    cweIds: ['CWE-917'],
    capecIds: ['CAPEC-242'],
    description: 'Apache Log4j2 JNDI features used in configuration...',
    publishedAt: '2025-12-09T00:00:00Z',
    updatedAt: '2026-06-10T00:00:00Z',
    sources: [{ name: 'NVD', url: 'https://nvd.nist.gov/...', lastModified: '2026-06-10' }],
    hasExploit: true,
  },
  // ... 49 more realistic CVEs
];
```

#### 5.6.4 Bật/Tắt MSW

```env
# .env.development — Bật MSW khi backend chưa có
VITE_ENABLE_MSW=true

# .env.development.local — Override khi backend đã có
VITE_ENABLE_MSW=false
VITE_API_BASE_URL=http://localhost:8080

# .env.production — LUÔN tắt MSW
VITE_ENABLE_MSW=false
```

---

## 6. Authentication & Authorization Architecture

### 6.1 Auth Flow

```
Login                          App
  │                              │
  ├── POST /api/v1/auth/login ──►│ 200 { access_token, refresh_token }
  │                              │
  ├── Store in Zustand (memory)  │
  ├── Refresh token → httpOnly cookie (server-managed)
  │                              │
  ├── All API requests: Bearer {access_token}
  │                              │
  ├── Token expires (15min) → interceptor triggers refresh
  │   POST /api/v1/auth/refresh (cookie sent automatically)
  │   ◄── 200 { access_token }
  │                              │
  └── RBAC check: usePermissions() hook reads role/permissions from JWT claims
```

### 6.2 RBAC-based UI Rendering

```typescript
// features/auth/hooks/usePermissions.ts
export function usePermissions() {
  const { user } = useAuth();
  
  return {
    canCreateScan: user?.permissions.includes('scan:create') ?? false,
    canManageUsers: user?.role === 'admin',
    canDownloadReports: user?.permissions.includes('report:download') ?? false,
    canWriteFindings: user?.permissions.includes('finding:write') ?? false,
    // ...
  };
}

// Usage
function ScanButton() {
  const { canCreateScan } = usePermissions();
  if (!canCreateScan) return null;
  return <Button onClick={...}>New Scan</Button>;
}
```

---

## 7. Performance Architecture

### 7.1 Code Splitting Strategy

```typescript
// Route-level lazy loading
const CVESearch = lazy(() => import('./features/cve-intel/components/CVESearch'));
const ScanWizard = lazy(() => import('./features/scanning/components/ScanWizard'));
const ReportCenter = lazy(() => import('./features/reports/components/ReportCenter'));

// Pre-fetch on hover (navigation intent)
function SidebarLink({ to, children }) {
  return (
    <Link
      to={to}
      onMouseEnter={() => {
        // Pre-load route chunk
        import(`./features/${routeToFeature(to)}`);
      }}
    >
      {children}
    </Link>
  );
}
```

### 7.2 React Query Cache Strategy

| Data Type | staleTime | gcTime | refetchInterval | Lý do |
|-----------|-----------|--------|-----------------|-------|
| CVE List | 5 min | 10 min | — | Không thay đổi thường xuyên |
| KEV Catalog | 30 min | 1 hour | — | Daily updates |
| Dashboard KPIs | 60s | 5 min | 60s (auto) | Real-time metrics |
| Scan Status | 2s | 5 min | 2s (khi running) | Active scan monitoring |
| Finding Detail | 30s | 5 min | — | After mutation invalidate |
| Auth User | Infinity | Infinity | — | Stable within session |
| DB Stats | 5 min | 30 min | — | Infrequent change |

### 7.3 Virtualized Lists

Cho các danh sách lớn (CVE search results, findings list):

```typescript
// Sử dụng @tanstack/react-virtual
import { useVirtualizer } from '@tanstack/react-virtual';

function CVETable({ cves }: { cves: CVE[] }) {
  const parentRef = useRef<HTMLDivElement>(null);
  const rowVirtualizer = useVirtualizer({
    count: cves.length,
    getScrollElement: () => parentRef.current,
    estimateSize: () => 52,  // row height in px
    overscan: 5,
  });
  // ...
}
```

---

## 8. Design System

### 8.1 Color Tokens (Dark Mode — Primary)

```css
:root {
  /* Background layers */
  --bg-base: #0B1020;        /* App background */
  --bg-surface: #0F1629;     /* Sidebar, panels */
  --bg-elevated: #151B2F;    /* Cards, drawers */
  --bg-overlay: #1E2A45;     /* Tooltips, dropdowns */

  /* Brand */
  --brand-blue: #4F8CFF;
  --brand-purple: #7C3AED;
  --brand-gradient: linear-gradient(135deg, #4F8CFF, #7C3AED);

  /* Severity */
  --severity-critical: #EF4444;
  --severity-high: #F97316;
  --severity-medium: #EAB308;
  --severity-low: #3B82F6;
  --severity-info: #6B7280;

  /* Text */
  --text-primary: #E5E7EB;
  --text-secondary: #9CA3AF;
  --text-muted: #6B7280;
  --text-disabled: #4B5563;

  /* Border */
  --border-base: rgba(255,255,255,0.06);
  --border-subtle: rgba(255,255,255,0.04);
  --border-emphasis: rgba(255,255,255,0.12);

  /* Status */
  --status-success: #10B981;
  --status-warning: #F59E0B;
  --status-error: #EF4444;
  --status-info: #4F8CFF;
  --status-ai: #A78BFA;
}
```

### 8.2 Shared Components Inventory

| Component | Props | Sử dụng |
|-----------|-------|---------|
| `SeverityBadge` | `severity: Severity` | CVE list, findings |
| `StatusBadge` | `status: FindingStatus` | Findings |
| `SLABadge` | `daysLeft: number` | SLA monitoring |
| `EPSSBar` | `score: number` | CVE detail |
| `KPICard` | `label, value, trend, icon` | Dashboard |
| `GradeCircle` | `grade, score` | Product security |
| `KEVIndicator` | `isKEV: boolean` | CVE list |
| `DataTable` | `columns, data, onSort, onFilter` | All list views |
| `EmptyState` | `title, description, action` | Empty lists |
| `LoadingSpinner` | `size, label` | Loading states |
| `ErrorBoundary` | `fallback` | Error recovery |
| `CommandPalette` | `onNavigate` | ⌘K global search |
| `SSEProgressBar` | `scanId` | Running scan |

---

## 9. Security Considerations

### 9.1 Token Storage

| Token | Storage | Lý do |
|-------|---------|-------|
| Access Token (JWT 15min) | **Zustand (in-memory)** | Không lưu localStorage → giảm XSS risk |
| Refresh Token | **httpOnly Cookie** | Server-managed, không accessible từ JS |
| API Key (hiển thị) | **Never stored** | Chỉ hiển thị 1 lần sau khi tạo |

### 9.2 Content Security Policy

```
Content-Security-Policy:
  default-src 'self';
  script-src 'self';
  style-src 'self' 'unsafe-inline' fonts.googleapis.com;
  font-src fonts.gstatic.com;
  connect-src 'self' [API_DOMAIN];
  img-src 'self' data:;
  frame-ancestors 'none';
```

### 9.3 Input Sanitization

- CVE descriptions từ external sources: render với `DOMPurify` trước khi inject HTML
- Search inputs: debounce + trim; API handles SQL injection prevention
- No dangerouslySetInnerHTML trừ khi DOMPurify-sanitized

---

## 10. Real-time Features

### 10.1 Scan Progress (SSE)

```
Browser                    Gateway              scan-service
   │                          │                     │
   ├─ GET /api/v1/scans/{id}/stream ────────────────►│
   │                          │     SSE stream       │
   │◄─ data: {progress: 45} ──────────────────────── │ (every 2s)
   │◄─ data: {progress: 78} ──────────────────────── │
   │◄─ event: done ─────────────────────────────────┤
   │                          │                     │
   ├─ Close EventSource       │                     │
   └─ Invalidate scan queries → refresh UI
```

### 10.2 In-app Notifications (NATS → WebSocket/SSE)

```
NATS JetStream               Gateway              Browser
    │                           │                    │
    ├─ finding.created ─────────►│                    │
    │                    SSE push│ GET /notifications/stream
    │                           ├───────────────────►│
    │                           │ data: {type, title}│
    │                           │◄────────────────── │
    │                           │                    ├─ Show toast
    │                           │                    └─ Update bell badge
```

---

## 11. Testing Architecture

### 11.1 Testing Pyramid

```
         ┌──────┐
         │  E2E │  ~20 critical flows (Playwright)
         └──────┘
       ┌──────────┐
       │Integration│  ~50 component + API mock tests
       └──────────┘
     ┌──────────────┐
     │  Unit Tests  │  ~200 utility + hook tests (Vitest)
     └──────────────┘
```

### 11.2 Test Coverage Targets

| Layer | Target | Tool |
|-------|--------|------|
| Shared utilities (severity, date, sla) | ≥ 90% | Vitest |
| API hooks (useFindings, useCVESearch) | ≥ 80% | Vitest + MSW |
| Feature components | ≥ 70% | React Testing Library |
| E2E critical flows | 20 flows | Playwright |

### 11.3 E2E Critical Flows

1. Login → Dashboard → Export PDF Report
2. CVE Search → Filter Critical + KEV → View CVE Detail
3. Create Nmap Scan → Monitor Progress (SSE) → View Results
4. Finding detail → Change status to Mitigated → Verify audit trail
5. Create API Key → Copy → Revoke
6. Admin: Create user → Assign role → Verify permissions

---

## 12. Environment Configuration

```env
# .env.development (commit vào git — defaults an toàn)
VITE_API_BASE_URL=http://localhost:8080
VITE_APP_ENV=development
VITE_ENABLE_MSW=true         # Bật MSW khi backend chưa sẵn sàng
VITE_SENTRY_DSN=

# .env.development.local (gitignored — override local)
VITE_ENABLE_MSW=false        # Tắt khi backend đã chạy
VITE_API_BASE_URL=http://localhost:8080

# .env.test (Vitest)
VITE_ENABLE_MSW=false        # MSW node server setup riêng trong Vitest

# .env.production
VITE_API_BASE_URL=https://api.osv.internal
VITE_APP_ENV=production
VITE_ENABLE_MSW=false        # PHẢI luôn false trong production
VITE_SENTRY_DSN=https://xxx@sentry.io/xxx
```

> [!WARNING]
> `VITE_ENABLE_MSW=true` **PHẢI** được tắt trước khi deploy production. CI/CD pipeline cần kiểm tra biến này.

---

## 13. Deployment

### 13.1 Build Output

```
dist/
├── index.html           # SPA entry
├── assets/
│   ├── index-[hash].js  # Main bundle (target: < 250KB gzipped)
│   ├── vendor-[hash].js # node_modules chunk
│   └── [feature]-[hash].js  # Lazy-loaded feature chunks
└── ...
```

### 13.2 Nginx Configuration (Production)

```nginx
location / {
  try_files $uri $uri/ /index.html;  # SPA routing
  add_header Cache-Control "no-cache, no-store";
}

location /assets/ {
  expires 1y;
  add_header Cache-Control "public, immutable";
}

location /api/ {
  proxy_pass http://gateway:8080;
  proxy_http_version 1.1;
  proxy_set_header Connection '';  # Keep-alive for SSE
}
```

---

## 14. Migration Path (Từ MVP → Target)

> [!IMPORTANT]
> Mỗi phase đều bắt đầu với việc **xóa hardcode data** và thay bằng MSW handler + React Query hook trước khi kết nối real API.

### Phase 1 — Foundation & No-Hardcode Enforcement (Sprint 1-2)
- [ ] Setup React Router v7, Zustand, React Query
- [ ] Tạo `shared/api/client.ts` với Axios interceptors + JWT refresh
- [ ] Setup MSW: `src/mocks/browser.ts`, `src/mocks/server.ts`
- [ ] Migrate auth flow: login → JWT → token refresh (xóa hardcode user)
- [ ] Feature-based folder restructure
- [ ] Thêm ESLint custom rule: `no-hardcode-mock-data`
- [ ] Setup `QueryBoundary` component cho loading/error/empty states

### Phase 2 — Remove Hardcode & Connect API (Sprint 3-5)

**Thứ tự ưu tiên (high-traffic screens trước):**

- [ ] **Dashboard**: Xóa `riskTrendData`, `productData`, `recentFindings` hardcode
  → Tạo `dashboard.handlers.ts` + `useDashboardMetrics()` hook
  → Kết nối `GET /api/v1/dashboard`

- [ ] **CVE Search**: Xóa `cveData = [...]` trong CVESearch.tsx
  → Tạo `cve.handlers.ts` với filter logic
  → Kết nối `POST /api/v2/cves/search` với pagination

- [ ] **KEV Catalog**: Xóa hardcode KEV entries
  → Kết nối `GET /api/v2/kev` với stats

- [ ] **Findings List**: Xóa hardcode findings array
  → Kết nối `GET /api/v1/findings` với server-side filter + pagination

- [ ] **Scan Dashboard**: Xóa hardcode scan list, implement SSE
  → Kết nối `GET /api/v1/scans` + SSE `/api/v1/scans/{id}/stream`

- [ ] **Assets**: Xóa hardcode assets, kết nối `GET /api/v1/assets`

- [ ] **Dashboard Sidebar stats** (Critical/Running/SLA): Kết nối real KPIs

- [ ] React Hook Form + Zod cho Scan Wizard

### Phase 3 — Polish & Performance (Sprint 6-7)
- [ ] Virtualized lists cho CVE table (>1000 rows)
- [ ] Code splitting + lazy loading
- [ ] Skeleton UI hoàn chỉnh cho mọi danh sách
- [ ] E2E tests (Playwright) với MSW server
- [ ] Error boundaries + graceful degradation
- [ ] CI gate: block build nếu phát hiện hardcode data pattern

### Phase 4 — Advanced Features (Sprint 8+)
- [ ] In-app notifications via SSE/WebSocket (NATS events)
- [ ] Command palette (⌘K) global search (multi-entity)
- [ ] Offline support (PWA) — cache dashboard metrics
- [ ] Dark/Light theme toggle

---

## 15. Data Flow Rules Summary

```
┌─────────────────────────────────────────────────────────────────────┐
│                    DATA FLOW — ALLOWED PATHS                         │
│                                                                      │
│  Server (API) ──► React Query Cache ──► Custom Hook ──► Component   │
│                                                                      │
│  Static Config (env vars, constants) ──► Component (OK)             │
│   e.g: SEVERITY_COLORS, SLA_CONFIG, ROUTE_DEFINITIONS               │
│                                                                      │
│  URL Params (filters, pagination) ──► Custom Hook ──► Component     │
│                                                                      │
│  User Interaction ──► Mutation ──► Invalidate Cache ──► Re-fetch     │
└─────────────────────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────────────────────┐
│                    DATA FLOW — FORBIDDEN PATHS                       │
│                                                                      │
│  ❌ Hardcode Array ──────────────────────────────────► Component     │
│  ❌ Hardcode Object ─────────────────────────────────► Component     │
│  ❌ faker/random ────────────────────────────────────► Component     │
│  ❌ Math.random() cho business data ────────────────► Component     │
└─────────────────────────────────────────────────────────────────────┘
```

### Các trường hợp được phép dùng literal data trong code

| Trường hợp | Ví dụ | Cho phép? |
|------------|-------|----------|
| UI constants (màu severity) | `SEVERITY_COLORS = { Critical: '#EF4444' }` | ✅ Có |
| Enum options (filter UI) | `SEVERITY_OPTIONS = ['All', 'Critical', 'High']` | ✅ Có |
| Config SLA defaults | `SLA_DAYS = { Critical: 7, High: 30 }` | ✅ Có |
| Route definitions | `ROUTES = { dashboard: '/dashboard' }` | ✅ Có |
| MSW fixtures | `cvesFixture: CVE[] = [...]` trong `src/mocks/` | ✅ Có |
| Business data trong component | `const findings = [{ id: 'F-2847'... }]` | ❌ Không |
| Chart data trong component | `const riskTrend = [{ month: 'Jan'... }]` | ❌ Không |
| KPI values trong component | `<div>245</div>` cho critical count | ❌ Không |
