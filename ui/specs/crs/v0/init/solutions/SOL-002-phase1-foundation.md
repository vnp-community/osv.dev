# SOL-002 — Phase 1: Foundation & No-Hardcode Enforcement

**Version:** 1.1  
**Ngày tạo:** 2026-06-16  
**Cập nhật:** 2026-06-17  
**Trạng thái:** ✅ Completed — Tất cả foundation items đã được implement  
**Phase:** Phase 1 (Sprint 1-2) — DONE  
**Liên quan:** [SOL-001](./SOL-001-gap-analysis.md), [architecture.md](../../../architecture.md)

---

## 1. Mục tiêu Phase 1

Thiết lập nền tảng kỹ thuật cho toàn bộ migration:
1. Cài đặt dependencies còn thiếu
2. Setup Zustand auth store
3. Setup React Query client
4. Setup Axios client với JWT interceptors
5. Tạo folder structure mới (feature-based)
6. Setup MSW mock layer (cơ bản)
7. Thêm TypeScript shared types
8. Migrate router từ `useState` sang React Router v7

---

## 2. Step 1: Cài đặt Dependencies

### 2.1 Lệnh cài đặt

```bash
cd ui/
pnpm add zustand @tanstack/react-query axios msw zod @tanstack/react-virtual
pnpm add -D vitest @testing-library/react @testing-library/user-event @vitejs/plugin-react jsdom
```

### 2.2 Dependencies breakdown

| Package | Purpose |
|---------|---------|
| `zustand` | Global state: auth, UI preferences |
| `@tanstack/react-query` | Server state: API data fetching, caching |
| `axios` | HTTP client với interceptors |
| `msw` | Mock Service Worker — intercept API calls in dev |
| `zod` | Runtime schema validation (forms) |
| `@tanstack/react-virtual` | Virtualized lists cho 300K CVEs |
| `vitest` | Test runner (co-located với Vite) |
| `@testing-library/react` | Component testing utilities |
| `jsdom` | DOM simulation cho Vitest |

---

## 3. Step 2: TypeScript Shared Types

Tạo file types trước khi viết bất kỳ component hay hook nào:

### 3.1 `src/shared/types/auth.ts`

```typescript
// src/shared/types/auth.ts
export type UserRole = 'admin' | 'user' | 'readonly' | 'agent';

export type Permission =
  | 'scan:create' | 'scan:read'
  | 'asset:write' | 'asset:read'
  | 'user:manage'
  | 'report:download'
  | 'system:configure'
  | 'finding:write' | 'finding:read'
  | 'agent:report';

export interface User {
  id: string;
  email: string;
  name: string;
  role: UserRole;
  permissions: Permission[];
  mfaEnabled: boolean;
  avatarUrl?: string;
  createdAt: string;
}

export interface AuthTokens {
  accessToken: string;
  expiresIn: number;
}

export interface AuthState {
  user: User | null;
  accessToken: string | null;
  isAuthenticated: boolean;
  isLoading: boolean;
  setUser: (user: User) => void;
  setAccessToken: (token: string) => void;
  logout: () => void;
}
```

### 3.2 `src/shared/types/api.ts`

```typescript
// src/shared/types/api.ts

export interface APIError {
  error: string;      // "NOT_FOUND", "UNAUTHORIZED"
  message: string;    // Human-readable
  details?: unknown;
  traceId?: string;
}

export interface PaginatedResponse<T> {
  data: T[];
  total: number;
  page: number;
  pageSize: number;
}
```

### 3.3 `src/shared/types/cve.ts`

```typescript
// src/shared/types/cve.ts
export type Severity = 'Critical' | 'High' | 'Medium' | 'Low' | 'Info';

export interface CVESource {
  name: string;
  url: string;
  lastModified: string;
}

export interface AISeverityResult {
  severity: Severity;
  confidence: number;
  reasoning: string;
  source: 'cvss_v3' | 'cvss_v2' | 'llm';
}

export interface CVE {
  id: string;
  severity: Severity;
  cvssV3?: number;
  cvssV2?: number;
  epssScore: number;
  epssPercentile: number;
  isKEV: boolean;
  vendor: string;
  product: string;
  cweIds: string[];
  capecIds: string[];
  description: string;
  publishedAt: string;
  updatedAt: string;
  sources: CVESource[];
  hasExploit: boolean;
  exploitDbUrl?: string;
  aiSeverity?: AISeverityResult;
  similarityScore?: number;
}

export interface KEVEntry {
  cveId: string;
  vendor: string;
  product: string;
  vulnerabilityName: string;
  dateAdded: string;
  shortDescription: string;
  requiredAction: string;
  dueDate?: string;
  knownRansomwareCampaignUse: boolean;
}

export interface EPSSData {
  cveId: string;
  epssScore: number;
  epssPercentile: number;
  date: string;
}

export interface CWEDetail {
  id: string;
  name: string;
  description: string;
  extendedDescription?: string;
  likelihood: string;
  mitigations: string[];
  relatedCVECount: number;
}
```

### 3.4 `src/shared/types/finding.ts`

```typescript
// src/shared/types/finding.ts
import type { Severity } from './cve';

export type FindingStatus =
  | 'active' | 'mitigated' | 'false_positive'
  | 'risk_accepted' | 'out_of_scope' | 'duplicate';

export type SLAStatus = 'ok' | 'at_risk' | 'breached';

export interface Finding {
  id: string;
  title: string;
  description: string;
  cveId?: string;
  severity: Severity;
  cvssV3?: number;
  epssScore?: number;
  isKEV: boolean;
  status: FindingStatus;
  isDuplicate: boolean;
  productId: string;
  productName: string;
  engagementId: string;
  testId: string;
  assetIp?: string;
  assetHostname?: string;
  slaExpirationDate?: string;
  slaStatus: SLAStatus;
  slaDaysLeft?: number;
  createdAt: string;
  updatedAt: string;
  createdBy: string;
  assignedTo?: string;
  jiraIssueKey?: string;
  jiraUrl?: string;
}

export interface AITriageResult {
  remarks: 'Confirmed' | 'FalsePositive' | 'NotAffected' | 'Unexplored';
  confidence: number;
  justification: string;
  actions: string[];
  generatedAt: string;
}
```

### 3.5 `src/shared/types/scan.ts`

```typescript
// src/shared/types/scan.ts
export type ScanType = 'nmap_full' | 'nmap_discovery' | 'zap' | 'agent' | 'import';
export type ScanStatus = 'pending' | 'queued' | 'running' | 'completed' | 'failed' | 'cancelled';

export interface Scan {
  id: string;
  name: string;
  type: ScanType;
  status: ScanStatus;
  targets: string[];
  progress: number;
  findingCount: number;
  startedAt?: string;
  completedAt?: string;
  durationMs?: number;
  createdBy: string;
  engagementId?: string;
  error?: string;
}

export interface ScanProgress {
  scanId: string;
  status: ScanStatus;
  progress: number;
  currentTarget?: string;
  message?: string;
  findingsFound: number;
}
```

---

## 4. Step 3: Shared API Infrastructure

### 4.1 `src/shared/api/client.ts`

```typescript
// src/shared/api/client.ts
import axios from 'axios';
import { useAuthStore } from '@/features/auth/store/authStore';

export const apiClient = axios.create({
  baseURL: import.meta.env.VITE_API_BASE_URL || 'http://localhost:8080',
  timeout: 30_000,
  headers: {
    'Content-Type': 'application/json',
  },
});

// Request interceptor: inject JWT Bearer token
apiClient.interceptors.request.use((config) => {
  const token = useAuthStore.getState().accessToken;
  if (token) {
    config.headers.Authorization = `Bearer ${token}`;
  }
  return config;
});

// Response interceptor: handle 401 → refresh token
apiClient.interceptors.response.use(
  (response) => response,
  async (error) => {
    const originalRequest = error.config;
    if (error.response?.status === 401 && !originalRequest._retry) {
      originalRequest._retry = true;
      try {
        // POST /api/v1/auth/refresh — cookie sent automatically
        const { data } = await axios.post(
          `${import.meta.env.VITE_API_BASE_URL}/api/v1/auth/refresh`,
          {},
          { withCredentials: true }
        );
        const newToken = data.access_token;
        useAuthStore.getState().setAccessToken(newToken);
        originalRequest.headers.Authorization = `Bearer ${newToken}`;
        return apiClient(originalRequest);
      } catch {
        useAuthStore.getState().logout();
        window.location.href = '/login';
      }
    }
    return Promise.reject(error);
  }
);
```

### 4.2 `src/shared/api/queryClient.ts`

```typescript
// src/shared/api/queryClient.ts
import { QueryClient } from '@tanstack/react-query';
import { toast } from 'sonner';
import type { APIError } from '@/shared/types/api';

export const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      staleTime: 30_000,          // 30s default
      gcTime: 5 * 60 * 1000,     // 5 min garbage collect
      retry: 2,
      refetchOnWindowFocus: false,
    },
    mutations: {
      onError: (error: unknown) => {
        const apiError = error as { response?: { data?: APIError } };
        const message = apiError?.response?.data?.message ?? 'An unexpected error occurred';
        toast.error(message);
      },
    },
  },
});

// ─── Query Key Factories ─────────────────────────────────────────────────────

export const cveKeys = {
  all: ['cves'] as const,
  search: (params: object) => [...cveKeys.all, 'search', params] as const,
  detail: (id: string) => [...cveKeys.all, 'detail', id] as const,
  kev: (params?: object) => [...cveKeys.all, 'kev', params] as const,
  epss: (cveId: string) => [...cveKeys.all, 'epss', cveId] as const,
};

export const findingKeys = {
  all: ['findings'] as const,
  list: (filters: object) => [...findingKeys.all, 'list', filters] as const,
  detail: (id: string) => [...findingKeys.all, 'detail', id] as const,
  audit: (id: string) => [...findingKeys.all, 'audit', id] as const,
};

export const scanKeys = {
  all: ['scans'] as const,
  list: (filters?: object) => [...scanKeys.all, 'list', filters] as const,
  detail: (id: string) => [...scanKeys.all, 'detail', id] as const,
  results: {
    nmap: (id: string) => [...scanKeys.all, 'results', 'nmap', id] as const,
    zap: (id: string) => [...scanKeys.all, 'results', 'zap', id] as const,
  },
};

export const dashboardKeys = {
  all: ['dashboard'] as const,
  metrics: (period: string) => [...dashboardKeys.all, 'metrics', period] as const,
};

export const assetKeys = {
  all: ['assets'] as const,
  list: (filters?: object) => [...assetKeys.all, 'list', filters] as const,
  detail: (id: string) => [...assetKeys.all, 'detail', id] as const,
};
```

### 4.3 `src/shared/api/endpoints.ts`

```typescript
// src/shared/api/endpoints.ts
// Single source of truth cho tất cả API endpoints

export const ENDPOINTS = {
  // Auth
  auth: {
    login: '/api/v1/auth/login',
    refresh: '/api/v1/auth/refresh',
    logout: '/api/v1/auth/logout',
    me: '/api/v1/auth/me',
    mfaSetup: '/api/v1/auth/mfa/setup',
    mfaConfirm: '/api/v1/auth/mfa/confirm',
    oauthGoogle: '/api/v1/auth/oauth/google',
  },

  // Dashboard
  dashboard: '/api/v1/dashboard',

  // CVE Intelligence (v2)
  cve: {
    search: '/api/v2/cves/search',
    semantic: '/api/v2/cves/search/semantic',
    detail: (id: string) => `/api/v2/cves/${id}`,
    export: '/api/v2/cves/export',
  },
  kev: {
    list: '/api/v2/kev',
    stats: '/api/v2/kev/stats',
  },
  browse: '/api/v2/browse',
  cwe: {
    detail: (id: string) => `/api/v2/cwe/${id}`,
  },
  epss: {
    query: (cveId: string) => `/api/v2/epss/${cveId}`,
  },
  dbinfo: '/api/v2/dbinfo',

  // Scans (v1)
  scans: {
    list: '/api/v1/scans',
    create: '/api/v1/scans',
    detail: (id: string) => `/api/v1/scans/${id}`,
    stream: (id: string) => `/api/v1/scans/${id}/stream`,
    cancel: (id: string) => `/api/v1/scans/${id}/cancel`,
    results: {
      nmap: (id: string) => `/api/v1/scans/${id}/results/nmap`,
      zap: (id: string) => `/api/v1/scans/${id}/results/zap`,
    },
  },

  // Findings (v1)
  findings: {
    list: '/api/v1/findings',
    detail: (id: string) => `/api/v1/findings/${id}`,
    update: (id: string) => `/api/v1/findings/${id}`,
    bulkClose: '/api/v1/findings/bulk/close',
    audit: (id: string) => `/api/v1/findings/${id}/audit`,
  },

  // Assets (v1)
  assets: {
    list: '/api/v1/assets',
    detail: (id: string) => `/api/v1/assets/${id}`,
  },

  // Reports (v1)
  reports: {
    list: '/api/v1/reports',
    create: '/api/v1/reports',
    download: (id: string, format: string) => `/api/v1/reports/${id}/download/${format}`,
  },

  // AI
  ai: {
    triage: (findingId: string) => `/api/v1/ai/triage/${findingId}`,
  },

  // Risk Acceptances
  riskAcceptances: '/api/v1/risk-acceptances',

  // Products
  products: '/api/v1/products',

  // Admin
  admin: {
    users: '/api/v1/admin/users',
    apiKeys: '/api/v1/api-keys',
    webhooks: '/api/v1/webhooks',
    audit: '/api/v1/admin/audit',
    health: '/api/v1/admin/health',
    settings: '/api/v1/admin/settings',
  },

  // Notifications
  notifications: {
    stream: '/notifications/stream',
  },
} as const;
```

---

## 5. Step 4: Zustand Auth Store

### 5.1 `src/features/auth/store/authStore.ts`

```typescript
// src/features/auth/store/authStore.ts
import { create } from 'zustand';
import { persist } from 'zustand/middleware';
import type { AuthState, User } from '@/shared/types/auth';

export const useAuthStore = create<AuthState>()(
  persist(
    (set) => ({
      user: null,
      accessToken: null,
      isAuthenticated: false,
      isLoading: false,
      setUser: (user: User) =>
        set({ user, isAuthenticated: true }),
      setAccessToken: (accessToken: string) =>
        set({ accessToken }),
      logout: () =>
        set({ user: null, accessToken: null, isAuthenticated: false }),
    }),
    {
      name: 'osv-auth',
      // QUAN TRỌNG: Chỉ persist user info, KHÔNG persist access token
      // Access token phải lấy lại qua refresh (httpOnly cookie)
      partialize: (state) => ({ user: state.user }),
    }
  )
);
```

---

## 6. Step 5: React Router v7 Setup

### 6.1 `src/app/router.tsx`

```typescript
// src/app/router.tsx
import { createBrowserRouter, Navigate } from 'react-router';
import { lazy, Suspense } from 'react';
import { AuthGuard } from './components/AuthGuard';
import { AppLayout } from './components/AppLayout';
import { LoadingSpinner } from '@/shared/components/feedback/LoadingSpinner';

// Route-level lazy loading — mỗi feature là một chunk riêng
const Dashboard = lazy(() =>
  import('@/features/dashboard/components/Dashboard')
);
const CVESearch = lazy(() =>
  import('@/features/cve-intel/components/CVESearch')
);
const KEVCatalog = lazy(() =>
  import('@/features/cve-intel/components/KEVCatalog')
);
const FindingsList = lazy(() =>
  import('@/features/findings/components/FindingsList')
);
const ScanDashboard = lazy(() =>
  import('@/features/scanning/components/ScanDashboard')
);
// ... (tất cả 37+ components)

const SuspenseWrapper = ({ children }: { children: React.ReactNode }) => (
  <Suspense fallback={<LoadingSpinner size="lg" label="Loading..." />}>
    {children}
  </Suspense>
);

export const router = createBrowserRouter([
  {
    path: '/',
    element: (
      <AuthGuard>
        <AppLayout />
      </AuthGuard>
    ),
    children: [
      { index: true, element: <Navigate to="/dashboard" replace /> },

      // Dashboard
      {
        path: 'dashboard',
        element: <SuspenseWrapper><Dashboard /></SuspenseWrapper>,
      },
      {
        path: 'dashboard/sla',
        element: <SuspenseWrapper><SLADashboard /></SuspenseWrapper>,
      },

      // CVE Intelligence
      { path: 'cve/search', element: <SuspenseWrapper><CVESearch /></SuspenseWrapper> },
      { path: 'cve/semantic', element: <SuspenseWrapper><SemanticSearch /></SuspenseWrapper> },
      { path: 'cve/kev', element: <SuspenseWrapper><KEVCatalog /></SuspenseWrapper> },
      { path: 'cve/epss', element: <SuspenseWrapper><EPSSAnalytics /></SuspenseWrapper> },
      { path: 'cve/vendors', element: <SuspenseWrapper><VendorCatalog /></SuspenseWrapper> },
      { path: 'cve/cwe', element: <SuspenseWrapper><CWELibrary /></SuspenseWrapper> },
      { path: 'cve/:id', element: <SuspenseWrapper><CVEDetail /></SuspenseWrapper> },

      // Scanning
      { path: 'scans', element: <SuspenseWrapper><ScanDashboard /></SuspenseWrapper> },
      { path: 'scans/new', element: <SuspenseWrapper><ScanWizard /></SuspenseWrapper> },
      { path: 'scans/:id', element: <SuspenseWrapper><ScanDetail /></SuspenseWrapper> },
      { path: 'scans/:id/results/nmap', element: <SuspenseWrapper><NmapResults /></SuspenseWrapper> },
      { path: 'scans/:id/results/zap', element: <SuspenseWrapper><ZAPResults /></SuspenseWrapper> },

      // Findings
      { path: 'findings', element: <SuspenseWrapper><FindingsList /></SuspenseWrapper> },
      { path: 'findings/:id', element: <SuspenseWrapper><FindingDetail /></SuspenseWrapper> },
      { path: 'findings/risk-acceptance', element: <SuspenseWrapper><RiskAcceptanceCenter /></SuspenseWrapper> },

      // Assets
      { path: 'assets', element: <SuspenseWrapper><AssetInventory /></SuspenseWrapper> },
      { path: 'assets/:id', element: <SuspenseWrapper><AssetDetail /></SuspenseWrapper> },

      // Product Security
      { path: 'products', element: <SuspenseWrapper><ProductSecurity /></SuspenseWrapper> },
      { path: 'products/:id', element: <SuspenseWrapper><ProductDetail /></SuspenseWrapper> },

      // AI Center
      { path: 'ai/triage', element: <SuspenseWrapper><AITriage /></SuspenseWrapper> },
      { path: 'ai/enrichment', element: <SuspenseWrapper><AIEnrichment /></SuspenseWrapper> },

      // Reports
      { path: 'reports', element: <SuspenseWrapper><ReportCenter /></SuspenseWrapper> },

      // Notifications
      { path: 'notifications', element: <SuspenseWrapper><NotificationCenter /></SuspenseWrapper> },

      // Integrations
      { path: 'integrations/api-keys', element: <SuspenseWrapper><APIKeyManagement /></SuspenseWrapper> },
      { path: 'integrations/webhooks', element: <SuspenseWrapper><WebhookEvents /></SuspenseWrapper> },

      // Admin
      { path: 'admin/users', element: <SuspenseWrapper><UserManagement /></SuspenseWrapper> },
      { path: 'admin/roles', element: <SuspenseWrapper><RBACManagement /></SuspenseWrapper> },
      { path: 'admin/audit', element: <SuspenseWrapper><AuditLogs /></SuspenseWrapper> },
      { path: 'admin/health', element: <SuspenseWrapper><SystemHealth /></SuspenseWrapper> },
      { path: 'admin/settings', element: <SuspenseWrapper><SystemSettings /></SuspenseWrapper> },

      // User
      { path: 'profile', element: <SuspenseWrapper><UserProfile /></SuspenseWrapper> },
      { path: 'onboarding', element: <SuspenseWrapper><OnboardingExperience /></SuspenseWrapper> },
    ],
  },
  { path: '/login', element: <LoginScreen /> },
  { path: '/auth/callback', element: <OAuthCallback /> },
]);
```

### 6.2 `src/app/components/AuthGuard.tsx`

```typescript
// src/app/components/AuthGuard.tsx
import { Navigate, useLocation } from 'react-router';
import { useAuthStore } from '@/features/auth/store/authStore';
import { FullPageSpinner } from '@/shared/components/feedback/LoadingSpinner';

export function AuthGuard({ children }: { children: React.ReactNode }) {
  const { isAuthenticated, isLoading } = useAuthStore();
  const location = useLocation();

  if (isLoading) return <FullPageSpinner />;

  if (!isAuthenticated) {
    return (
      <Navigate
        to="/login"
        state={{ from: location }}
        replace
      />
    );
  }

  return <>{children}</>;
}
```

### 6.3 `src/app/providers.tsx`

```typescript
// src/app/providers.tsx
import { QueryClientProvider } from '@tanstack/react-query';
import { ReactQueryDevtools } from '@tanstack/react-query-devtools';
import { Toaster } from 'sonner';
import { queryClient } from '@/shared/api/queryClient';

export function Providers({ children }: { children: React.ReactNode }) {
  return (
    <QueryClientProvider client={queryClient}>
      {children}
      <Toaster
        position="top-right"
        theme="dark"
        toastOptions={{
          style: {
            background: '#1E2A45',
            border: '1px solid rgba(255,255,255,0.1)',
            color: '#E5E7EB',
          },
        }}
      />
      {import.meta.env.DEV && <ReactQueryDevtools initialIsOpen={false} />}
    </QueryClientProvider>
  );
}
```

### 6.4 `src/main.tsx` (Updated)

```typescript
// src/main.tsx
import { StrictMode } from 'react';
import { createRoot } from 'react-dom/client';
import { RouterProvider } from 'react-router';
import { router } from './app/router';
import { Providers } from './app/providers';
import './styles/globals.css';

async function enableMocking() {
  if (import.meta.env.VITE_ENABLE_MSW !== 'true') return;
  const { worker } = await import('./mocks/browser');
  return worker.start({
    onUnhandledRequest: 'warn',
  });
}

enableMocking().then(() => {
  createRoot(document.getElementById('root')!).render(
    <StrictMode>
      <Providers>
        <RouterProvider router={router} />
      </Providers>
    </StrictMode>
  );
});
```

---

## 7. Step 6: MSW Setup (Development Mock Layer)

### 7.1 `src/mocks/browser.ts`

```typescript
// src/mocks/browser.ts
import { setupWorker } from 'msw/browser';
import { handlers } from './handlers';

export const worker = setupWorker(...handlers);
```

### 7.2 `src/mocks/handlers/index.ts`

```typescript
// src/mocks/handlers/index.ts
import { authHandlers } from './auth.handlers';
import { dashboardHandlers } from './dashboard.handlers';
import { cveHandlers } from './cve.handlers';
import { findingHandlers } from './finding.handlers';
import { scanHandlers } from './scan.handlers';
import { assetHandlers } from './asset.handlers';

export const handlers = [
  ...authHandlers,
  ...dashboardHandlers,
  ...cveHandlers,
  ...findingHandlers,
  ...scanHandlers,
  ...assetHandlers,
];
```

### 7.3 `src/mocks/handlers/dashboard.handlers.ts`

```typescript
// src/mocks/handlers/dashboard.handlers.ts
import { http, HttpResponse } from 'msw';
import { dashboardFixture } from '../fixtures/dashboard.fixture';

export const dashboardHandlers = [
  http.get('/api/v1/dashboard', ({ request }) => {
    const url = new URL(request.url);
    const period = url.searchParams.get('period') ?? '30d';
    return HttpResponse.json(dashboardFixture[period] ?? dashboardFixture['30d']);
  }),
];
```

### 7.4 `src/mocks/fixtures/dashboard.fixture.ts`

```typescript
// src/mocks/fixtures/dashboard.fixture.ts
// ⚠️ Chỉ dùng trong src/mocks/ — KHÔNG import vào components
import type { DashboardData } from '@/features/dashboard/types';

const base30d: DashboardData = {
  kpis: {
    criticalFindings: 245,
    highFindings: 395,
    totalAssets: 1247,
    highRiskAssets: 98,
    activeScans: 3,
    queuedScans: 2,
    securityGrade: 'B-',
    securityScore: 61,
    slaCompliance: 94.2,
    slaAtRisk: 22,
    slaBreached: 8,
  },
  riskTrend: [
    { month: 'Jan', critical: 320, high: 480, medium: 820, low: 1200 },
    { month: 'Feb', critical: 290, high: 450, medium: 780, low: 1150 },
    { month: 'Mar', critical: 310, high: 510, medium: 850, low: 1280 },
    { month: 'Apr', critical: 280, high: 420, medium: 740, low: 1100 },
    { month: 'May', critical: 245, high: 380, medium: 690, low: 980 },
    { month: 'Jun', critical: 245, high: 395, medium: 710, low: 1020 },
  ],
  severityDistribution: {
    critical: 245,
    high: 395,
    medium: 710,
    low: 1020,
    total: 2370,
  },
  productGrades: [
    { id: 'p1', name: 'Banking App', grade: 'B', score: 62, criticalCount: 8, highCount: 24 },
    { id: 'p2', name: 'Mobile App', grade: 'A-', score: 78, criticalCount: 2, highCount: 11 },
    { id: 'p3', name: 'API Gateway', grade: 'C+', score: 45, criticalCount: 14, highCount: 38 },
    { id: 'p4', name: 'Admin Portal', grade: 'B+', score: 71, criticalCount: 4, highCount: 16 },
    { id: 'p5', name: 'Data Pipeline', grade: 'C+', score: 55, criticalCount: 9, highCount: 22 },
  ],
  kevAlerts: [
    { cveId: 'CVE-2025-44228', vendor: 'Apache', product: 'Log4j2', dateAdded: '2026-06-12', isRansomware: false },
    { cveId: 'CVE-2025-22965', vendor: 'VMware', product: 'Spring', dateAdded: '2026-06-09', isRansomware: true },
    { cveId: 'CVE-2025-09876', vendor: 'Cisco', product: 'IOS XE', dateAdded: '2026-06-07', isRansomware: false },
  ],
  recentScans: [],
  slaBreaches: [],
};

export const dashboardFixture: Record<string, DashboardData> = {
  '30d': base30d,
  '90d': { ...base30d }, // Adjust for 90d period
  '1y': { ...base30d },  // Adjust for 1y period
};
```

---

## 8. Step 7: Shared QueryBoundary Component

```typescript
// src/shared/components/feedback/QueryBoundary.tsx
import type { UseQueryResult } from '@tanstack/react-query';

interface QueryBoundaryProps<T> {
  query: UseQueryResult<T>;
  skeleton: React.ReactNode;
  children: (data: T) => React.ReactNode;
}

export function QueryBoundary<T>({
  query,
  skeleton,
  children,
}: QueryBoundaryProps<T>) {
  if (query.isLoading) return <>{skeleton}</>;
  if (query.isError) {
    return (
      <ErrorState
        message={query.error?.message ?? 'An error occurred'}
        onRetry={() => query.refetch()}
      />
    );
  }
  if (!query.data) return <EmptyState />;
  return <>{children(query.data)}</>;
}
```

---

## 9. Feature Folder Structure mẫu

```
src/features/dashboard/
├── components/
│   ├── Dashboard.tsx          # Main component (no hardcode)
│   ├── KPICard.tsx            # Extracted shared UI
│   ├── RiskTrendChart.tsx     # Chart component
│   ├── SeverityDonut.tsx      # Chart component
│   └── DashboardSkeleton.tsx  # Loading skeleton
├── hooks/
│   └── useDashboardMetrics.ts # React Query hook
├── api/
│   └── dashboardApi.ts        # API calls
└── types.ts                   # Feature-specific types
```

```typescript
// src/features/dashboard/api/dashboardApi.ts
import { apiClient } from '@/shared/api/client';
import { ENDPOINTS } from '@/shared/api/endpoints';
import type { DashboardData } from '../types';

export const dashboardApi = {
  getMetrics: async (period: '30d' | '90d' | '1y'): Promise<DashboardData> => {
    const { data } = await apiClient.get<DashboardData>(ENDPOINTS.dashboard, {
      params: { period },
    });
    return data;
  },
};
```

```typescript
// src/features/dashboard/hooks/useDashboardMetrics.ts
import { useQuery } from '@tanstack/react-query';
import { dashboardKeys } from '@/shared/api/queryClient';
import { dashboardApi } from '../api/dashboardApi';

export function useDashboardMetrics(period: '30d' | '90d' | '1y' = '30d') {
  return useQuery({
    queryKey: dashboardKeys.metrics(period),
    queryFn: () => dashboardApi.getMetrics(period),
    staleTime: 60_000,           // 1 min
    refetchInterval: 60_000,     // Auto-refresh every minute
  });
}
```

```typescript
// src/features/dashboard/components/Dashboard.tsx
// ✅ ĐÚNG — Không có hardcode data
import { useState } from 'react';
import { useDashboardMetrics } from '../hooks/useDashboardMetrics';
import { QueryBoundary } from '@/shared/components/feedback/QueryBoundary';
import { DashboardSkeleton } from './DashboardSkeleton';
import { RiskTrendChart } from './RiskTrendChart';
import { SeverityDonut } from './SeverityDonut';
import { KPICard } from './KPICard';

export function Dashboard() {
  const [period, setPeriod] = useState<'30d' | '90d' | '1y'>('30d');
  const metricsQuery = useDashboardMetrics(period);

  return (
    <QueryBoundary
      query={metricsQuery}
      skeleton={<DashboardSkeleton />}
    >
      {(data) => (
        <div className="flex-1 overflow-y-auto p-6" style={{ background: '#0B1020' }}>
          {/* KPI Row — data từ server */}
          <div className="grid grid-cols-6 gap-4 mb-6">
            <KPICard
              label="Critical Findings"
              value={data.kpis.criticalFindings.toLocaleString()}
              // ...
            />
            {/* ... other KPI cards */}
          </div>

          {/* Charts — data từ server */}
          <RiskTrendChart data={data.riskTrend} />
          <SeverityDonut data={data.severityDistribution} />
        </div>
      )}
    </QueryBoundary>
  );
}
```

---

## 10. Environment Files

### 10.1 `.env.development`
```env
VITE_API_BASE_URL=http://localhost:8080
VITE_APP_ENV=development
VITE_ENABLE_MSW=true
VITE_SENTRY_DSN=
```

### 10.2 `.env.development.local` (gitignored)
```env
# Override khi backend đã sẵn sàng
VITE_ENABLE_MSW=false
VITE_API_BASE_URL=http://localhost:8080
```

### 10.3 `.env.production`
```env
VITE_API_BASE_URL=https://api.osv.internal
VITE_APP_ENV=production
VITE_ENABLE_MSW=false
```

---

## 11. vite.config.ts (Updated)

```typescript
// vite.config.ts
import { defineConfig } from 'vite';
import react from '@vitejs/plugin-react';
import path from 'path';

export default defineConfig({
  plugins: [react()],
  resolve: {
    alias: {
      '@': path.resolve(__dirname, './src'),
    },
  },
  server: {
    port: 3000,
    proxy: {
      '/api': {
        target: 'http://localhost:8080',
        changeOrigin: true,
      },
    },
  },
  test: {
    environment: 'jsdom',
    globals: true,
    setupFiles: './src/test/setup.ts',
  },
});
```

---

## 12. Checklist Phase 1

- [ ] `pnpm add zustand @tanstack/react-query axios msw zod`
- [ ] Tạo `src/shared/types/` (auth, api, cve, finding, scan)
- [ ] Tạo `src/shared/api/client.ts` (Axios + interceptors)
- [ ] Tạo `src/shared/api/queryClient.ts` (QueryClient + key factories)
- [ ] Tạo `src/shared/api/endpoints.ts`
- [ ] Tạo `src/features/auth/store/authStore.ts` (Zustand)
- [ ] Tạo `src/app/router.tsx` (React Router v7)
- [ ] Tạo `src/app/components/AuthGuard.tsx`
- [ ] Tạo `src/app/providers.tsx`
- [ ] Update `src/main.tsx`
- [ ] Setup MSW: `src/mocks/browser.ts`, `src/mocks/handlers/`
- [ ] Tạo `src/mocks/fixtures/dashboard.fixture.ts`
- [ ] Tạo `src/shared/components/feedback/QueryBoundary.tsx`
- [ ] Update `vite.config.ts` (alias `@`, test config)
- [ ] Tạo `.env.development`, `.env.production`
- [ ] Verify app chạy được với MSW enabled
