# TASK-UI-004 — React Query Client + Auth Store + Providers + Router

| Field | Value |
|-------|-------|
| **Task ID** | TASK-UI-004 |
| **Module** | `ui/src/shared/api/`, `ui/src/features/auth/`, `ui/src/app/` |
| **Solution Ref** | [SOL-002 §4,5,6](../solutions/SOL-002-phase1-foundation.md) |
| **Priority** | 🔴 P0 |
| **Depends On** | TASK-UI-003 |
| **Estimated** | 2h |
| **Status** | ✅ Completed (Phase 3: Sidebar+LoginScreen migrated 2026-06-17) |
| **Completed** | 2026-06-17 |

---

## Context

Cần thiết lập 4 thành phần nền tảng cùng lúc vì chúng phụ thuộc nhau:
1. **QueryClient** — cấu hình React Query với cache strategy và key factories
2. **Auth Store** — Zustand store persist user info (không persist token)
3. **App Providers** — wrapper QueryClientProvider + Toaster
4. **React Router v7** — thay thế `useState` view switch trong App.tsx hiện tại

---

## Goal

Thiết lập đầy đủ state management + routing foundation.

---

## Target Files

| Action | File Path |
|--------|-----------|
| CREATE | `ui/src/shared/api/queryClient.ts` |
| CREATE | `ui/src/features/auth/store/authStore.ts` |
| CREATE | `ui/src/app/providers.tsx` |
| CREATE | `ui/src/app/router.tsx` |
| CREATE | `ui/src/app/components/AuthGuard.tsx` |
| CREATE | `ui/src/app/components/AppLayout.tsx` |
| MODIFY | `ui/src/main.tsx` |

---

## Implementation

### File 1: `ui/src/shared/api/queryClient.ts`

```typescript
import { QueryClient } from '@tanstack/react-query';
import { toast } from 'sonner';
import type { APIError } from '@/shared/types/api';

export const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      staleTime: 30_000,          // 30s — default cho hầu hết queries
      gcTime: 5 * 60 * 1000,     // 5 min garbage collect
      retry: 2,
      refetchOnWindowFocus: false,
    },
    mutations: {
      onError: (error: unknown) => {
        const apiError = error as { response?: { data?: APIError } };
        const message =
          apiError?.response?.data?.message ?? 'An unexpected error occurred';
        toast.error(message);
      },
    },
  },
});

// ─── Query Key Factories ─────────────────────────────────────────────────────
// Pattern: [...baseKey, 'operation', params]

export const authKeys = {
  all: ['auth'] as const,
  me: () => [...authKeys.all, 'me'] as const,
};

export const dashboardKeys = {
  all: ['dashboard'] as const,
  metrics: (period: string) => [...dashboardKeys.all, 'metrics', period] as const,
};

export const cveKeys = {
  all: ['cves'] as const,
  search: (params: object) => [...cveKeys.all, 'search', params] as const,
  detail: (id: string) => [...cveKeys.all, 'detail', id] as const,
  kev: (params?: object) => [...cveKeys.all, 'kev', params] as const,
  kevStats: () => [...cveKeys.all, 'kev-stats'] as const,
  epss: (cveId: string) => [...cveKeys.all, 'epss', cveId] as const,
  semantic: (query: object) => [...cveKeys.all, 'semantic', query] as const,
  vendors: (params?: object) => [...cveKeys.all, 'vendors', params] as const,
  cwe: (id: string) => [...cveKeys.all, 'cwe', id] as const,
};

export const findingKeys = {
  all: ['findings'] as const,
  list: (filters: object) => [...findingKeys.all, 'list', filters] as const,
  detail: (id: string) => [...findingKeys.all, 'detail', id] as const,
  audit: (id: string) => [...findingKeys.all, 'audit', id] as const,
  riskAcceptances: (filters?: object) => [...findingKeys.all, 'risk-acceptances', filters] as const,
};

export const scanKeys = {
  all: ['scans'] as const,
  list: (filters?: object) => [...scanKeys.all, 'list', filters] as const,
  detail: (id: string) => [...scanKeys.all, 'detail', id] as const,
  nmapResults: (id: string) => [...scanKeys.all, 'results', 'nmap', id] as const,
  zapResults: (id: string) => [...scanKeys.all, 'results', 'zap', id] as const,
};

export const assetKeys = {
  all: ['assets'] as const,
  list: (filters?: object) => [...assetKeys.all, 'list', filters] as const,
  detail: (id: string) => [...assetKeys.all, 'detail', id] as const,
};

export const productKeys = {
  all: ['products'] as const,
  list: (filters?: object) => [...productKeys.all, 'list', filters] as const,
  detail: (id: string) => [...productKeys.all, 'detail', id] as const,
};

export const reportKeys = {
  all: ['reports'] as const,
  list: () => [...reportKeys.all, 'list'] as const,
};

export const adminKeys = {
  users: ['admin', 'users'] as const,
  audit: (filters?: object) => ['admin', 'audit', filters] as const,
  health: ['admin', 'health'] as const,
  settings: ['admin', 'settings'] as const,
};
```

### File 2: `ui/src/features/auth/store/authStore.ts`

```typescript
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
        set({
          user: null,
          accessToken: null,
          isAuthenticated: false,
        }),
    }),
    {
      name: 'osv-auth',
      // QUAN TRỌNG: Chỉ persist user info
      // accessToken KHÔNG persist — phải lấy lại qua refresh cookie
      partialize: (state) => ({ user: state.user }),
    }
  )
);
```

### File 3: `ui/src/app/providers.tsx`

```typescript
import { QueryClientProvider } from '@tanstack/react-query';
import { ReactQueryDevtools } from '@tanstack/react-query-devtools';
import { Toaster } from 'sonner';
import { useEffect } from 'react';
import { queryClient } from '@/shared/api/queryClient';
import { injectAuthStore } from '@/shared/api/client';
import { useAuthStore } from '@/features/auth/store/authStore';

function AuthStoreInjector({ children }: { children: React.ReactNode }) {
  const { accessToken, setAccessToken, logout } = useAuthStore();

  useEffect(() => {
    // Inject auth store functions vào Axios client (tránh circular import)
    injectAuthStore(
      () => useAuthStore.getState().accessToken,
      setAccessToken,
      logout
    );
  }, [setAccessToken, logout]);

  return <>{children}</>;
}

export function Providers({ children }: { children: React.ReactNode }) {
  return (
    <QueryClientProvider client={queryClient}>
      <AuthStoreInjector>
        {children}
      </AuthStoreInjector>
      <Toaster
        position="top-right"
        theme="dark"
        toastOptions={{
          style: {
            background: '#1E2A45',
            border: '1px solid rgba(255,255,255,0.1)',
            color: '#E5E7EB',
            fontFamily: "'Inter', sans-serif",
          },
        }}
      />
      {import.meta.env.DEV && (
        <ReactQueryDevtools initialIsOpen={false} buttonPosition="bottom-right" />
      )}
    </QueryClientProvider>
  );
}
```

### File 4: `ui/src/app/components/AuthGuard.tsx`

```typescript
import { Navigate, useLocation } from 'react-router';
import { useAuthStore } from '@/features/auth/store/authStore';

export function FullPageSpinner() {
  return (
    <div
      className="w-full h-screen flex items-center justify-center"
      style={{ background: '#0B1020' }}
    >
      <div
        className="w-10 h-10 rounded-full border-2 border-t-transparent animate-spin"
        style={{ borderColor: '#4F8CFF', borderTopColor: 'transparent' }}
      />
    </div>
  );
}

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

### File 5: `ui/src/app/components/AppLayout.tsx`

```typescript
import { Outlet } from 'react-router';
import { Sidebar } from '@/app/components/Sidebar';
import { Topbar } from '@/app/components/Topbar';

export function AppLayout() {
  return (
    <div
      className="w-full h-screen flex flex-col overflow-hidden"
      style={{ background: '#0B1020', fontFamily: "'Inter', sans-serif" }}
    >
      <div className="flex flex-1 overflow-hidden">
        <Sidebar />
        <div className="flex flex-col flex-1 overflow-hidden">
          <Topbar />
          <main className="flex-1 overflow-hidden">
            <Outlet />
          </main>
        </div>
      </div>
    </div>
  );
}
```

### File 6: `ui/src/app/router.tsx`

```typescript
import { createBrowserRouter, Navigate } from 'react-router';
import { lazy, Suspense } from 'react';
import { AuthGuard, FullPageSpinner } from './components/AuthGuard';
import { AppLayout } from './components/AppLayout';

// ─── Lazy imports — mỗi feature là 1 chunk riêng ─────────────────────────
// Tạm thời import từ app/components/ cũ — sẽ migrate dần sang features/
const Dashboard = lazy(() =>
  import('@/app/components/Dashboard').then((m) => ({ default: m.Dashboard }))
);
const CVESearch = lazy(() =>
  import('@/app/components/CVESearch').then((m) => ({ default: m.CVESearch }))
);
const KEVCatalog = lazy(() =>
  import('@/app/components/KEVCatalog').then((m) => ({ default: m.KEVCatalog }))
);
const SemanticSearch = lazy(() =>
  import('@/app/components/SemanticSearch').then((m) => ({ default: m.SemanticSearch }))
);
const EPSSAnalytics = lazy(() =>
  import('@/app/components/EPSSAnalytics').then((m) => ({ default: m.EPSSAnalytics }))
);
const VendorCatalog = lazy(() =>
  import('@/app/components/VendorCatalog').then((m) => ({ default: m.VendorCatalog }))
);
const CWELibrary = lazy(() =>
  import('@/app/components/CWELibrary').then((m) => ({ default: m.CWELibrary }))
);
const ScanDashboard = lazy(() =>
  import('@/app/components/ScanDashboard').then((m) => ({ default: m.ScanDashboard }))
);
const ScanWizard = lazy(() =>
  import('@/app/components/ScanWizard').then((m) => ({ default: m.ScanWizard }))
);
const RunningScan = lazy(() =>
  import('@/app/components/RunningScan').then((m) => ({ default: m.RunningScan }))
);
const ScanHistory = lazy(() =>
  import('@/app/components/ScanHistory').then((m) => ({ default: m.ScanHistory }))
);
const NmapResults = lazy(() =>
  import('@/app/components/NmapResults').then((m) => ({ default: m.NmapResults }))
);
const ZAPResults = lazy(() =>
  import('@/app/components/ZAPResults').then((m) => ({ default: m.ZAPResults }))
);
const FindingsList = lazy(() =>
  import('@/app/components/FindingsList').then((m) => ({ default: m.FindingsList }))
);
const FindingDetail = lazy(() =>
  import('@/app/components/FindingDetail').then((m) => ({ default: m.FindingDetail }))
);
const SLADashboard = lazy(() =>
  import('@/app/components/SLADashboard').then((m) => ({ default: m.SLADashboard }))
);
const RiskAcceptanceCenter = lazy(() =>
  import('@/app/components/RiskAcceptanceCenter').then((m) => ({ default: m.RiskAcceptanceCenter }))
);
const AssetInventory = lazy(() =>
  import('@/app/components/AssetInventory').then((m) => ({ default: m.AssetInventory }))
);
const AssetDetail = lazy(() =>
  import('@/app/components/AssetDetail').then((m) => ({ default: m.AssetDetail }))
);
const ProductSecurity = lazy(() =>
  import('@/app/components/ProductSecurity').then((m) => ({ default: m.ProductSecurity }))
);
const AITriage = lazy(() =>
  import('@/app/components/AITriage').then((m) => ({ default: m.AITriage }))
);
const AIEnrichment = lazy(() =>
  import('@/app/components/AIEnrichment').then((m) => ({ default: m.AIEnrichment }))
);
const ReportCenter = lazy(() =>
  import('@/app/components/ReportCenter').then((m) => ({ default: m.ReportCenter }))
);
const NotificationCenter = lazy(() =>
  import('@/app/components/NotificationCenter').then((m) => ({ default: m.NotificationCenter }))
);
const APIKeyManagement = lazy(() =>
  import('@/app/components/APIKeyManagement').then((m) => ({ default: m.APIKeyManagement }))
);
const WebhookEvents = lazy(() =>
  import('@/app/components/WebhookEvents').then((m) => ({ default: m.WebhookEvents }))
);
const UserManagement = lazy(() =>
  import('@/app/components/UserManagement').then((m) => ({ default: m.UserManagement }))
);
const RBACManagement = lazy(() =>
  import('@/app/components/RBACManagement').then((m) => ({ default: m.RBACManagement }))
);
const AuditLogs = lazy(() =>
  import('@/app/components/AuditLogs').then((m) => ({ default: m.AuditLogs }))
);
const SystemHealth = lazy(() =>
  import('@/app/components/SystemHealth').then((m) => ({ default: m.SystemHealth }))
);
const SystemSettings = lazy(() =>
  import('@/app/components/SystemSettings').then((m) => ({ default: m.SystemSettings }))
);
const UserProfile = lazy(() =>
  import('@/app/components/UserProfile').then((m) => ({ default: m.UserProfile }))
);
const OnboardingExperience = lazy(() =>
  import('@/app/components/OnboardingExperience').then((m) => ({ default: m.OnboardingExperience }))
);
const LoginScreen = lazy(() =>
  import('@/app/components/LoginScreen').then((m) => ({ default: m.LoginScreen }))
);

const S = ({ children }: { children: React.ReactNode }) => (
  <Suspense fallback={<FullPageSpinner />}>{children}</Suspense>
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
      { path: 'dashboard', element: <S><Dashboard /></S> },
      { path: 'dashboard/sla', element: <S><SLADashboard /></S> },

      // CVE Intelligence
      { path: 'cve/search', element: <S><CVESearch /></S> },
      { path: 'cve/semantic', element: <S><SemanticSearch /></S> },
      { path: 'cve/kev', element: <S><KEVCatalog /></S> },
      { path: 'cve/epss', element: <S><EPSSAnalytics /></S> },
      { path: 'cve/vendors', element: <S><VendorCatalog /></S> },
      { path: 'cve/cwe', element: <S><CWELibrary /></S> },

      // Scanning
      { path: 'scans', element: <S><ScanDashboard /></S> },
      { path: 'scans/new', element: <S><ScanWizard /></S> },
      { path: 'scans/history', element: <S><ScanHistory /></S> },
      { path: 'scans/:id', element: <S><RunningScan /></S> },
      { path: 'scans/:id/results/nmap', element: <S><NmapResults /></S> },
      { path: 'scans/:id/results/zap', element: <S><ZAPResults /></S> },

      // Findings
      { path: 'findings', element: <S><FindingsList /></S> },
      { path: 'findings/risk-acceptance', element: <S><RiskAcceptanceCenter /></S> },
      { path: 'findings/:id', element: <S><FindingDetail /></S> },

      // Assets
      { path: 'assets', element: <S><AssetInventory /></S> },
      { path: 'assets/:id', element: <S><AssetDetail /></S> },

      // Product Security
      { path: 'products', element: <S><ProductSecurity /></S> },

      // AI Center
      { path: 'ai/triage', element: <S><AITriage /></S> },
      { path: 'ai/enrichment', element: <S><AIEnrichment /></S> },

      // Reports
      { path: 'reports', element: <S><ReportCenter /></S> },

      // Notifications
      { path: 'notifications', element: <S><NotificationCenter /></S> },

      // Integrations
      { path: 'integrations/api-keys', element: <S><APIKeyManagement /></S> },
      { path: 'integrations/webhooks', element: <S><WebhookEvents /></S> },

      // Admin
      { path: 'admin/users', element: <S><UserManagement /></S> },
      { path: 'admin/roles', element: <S><RBACManagement /></S> },
      { path: 'admin/audit', element: <S><AuditLogs /></S> },
      { path: 'admin/health', element: <S><SystemHealth /></S> },
      { path: 'admin/settings', element: <S><SystemSettings /></S> },

      // User
      { path: 'profile', element: <S><UserProfile /></S> },
      { path: 'onboarding', element: <S><OnboardingExperience /></S> },
    ],
  },
  { path: '/login', element: <S><LoginScreen /></S> },
]);
```

### File 7: `ui/src/main.tsx` (Replace)

```typescript
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

> [!NOTE]
> `Sidebar.tsx` và `Topbar.tsx` cần được update để dùng `useNavigate()` từ react-router thay vì nhận `onNavigate` prop. Xem phần compatibility notes.

---

## Compatibility Notes

`Sidebar.tsx` hiện nhận `activeSection` và `onNavigate` props. Cần thay bằng `useNavigate` + `useLocation`:

```typescript
// app/components/Sidebar.tsx — minimal changes
import { useNavigate, useLocation } from 'react-router';

export function Sidebar() {
  const navigate = useNavigate();
  const location = useLocation();
  const activeSection = location.pathname.split('/')[1]; // e.g. "dashboard"

  // Thay onNavigate(view) → navigate(`/${path}`)
  // ...
}
```

`LoginScreen.tsx` hiện nhận `onLogin` prop. Thay bằng `useNavigate`:

```typescript
// app/components/LoginScreen.tsx — minimal changes
import { useNavigate } from 'react-router';

export function LoginScreen() {
  const navigate = useNavigate();
  // Khi login thành công → navigate('/dashboard')
}
```

---

## Verification

```bash
cd ui/
pnpm dev
# Truy cập http://localhost:3000/login → redirect đến /login
# Login → redirect đến /dashboard
# Truy cập http://localhost:3000/cve/search → hiện CVESearch component
# Truy cập http://localhost:3000/findings → hiện FindingsList component
# Browser Back/Forward hoạt động
```

---

- [x] `src/shared/api/queryClient.ts` — QueryClient + 8 key factory objects
- [x] `src/features/auth/store/authStore.ts` — Zustand persist (user only, NOT token)
- [x] `src/app/providers.tsx` — QueryClientProvider + AuthStoreInjector + Toaster + DevTools
- [x] `src/app/components/AuthGuard.tsx` — redirect `/login` nếu chưa auth
- [x] `src/app/components/AppLayout.tsx` — Sidebar + Topbar + `<Outlet />` + breadcrumbs ✅ Done
- [x] `src/app/router.tsx` — tất cả routes với lazy loading (App.tsx wrapper + individual routes)
- [x] `src/main.tsx` — bootstrap MSW + RouterProvider + Providers
- [x] `Sidebar.tsx` update: `useNavigate` + `useLocation` thay `onNavigate` prop ✅ Done
- [x] `LoginScreen.tsx` update: `useNavigate` thay `onLogin` prop ✅ Done
- [x] `pnpm dev` khởi động, routing hoạt động đúng
- [x] URL `/dashboard` hiển thị Dashboard component
- [x] URL `/cve/search` hiển thị CVESearch component
- [x] Browser history (Back/Forward) hoạt động
