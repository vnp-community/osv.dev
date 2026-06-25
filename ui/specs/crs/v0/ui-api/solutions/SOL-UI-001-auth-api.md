# SOL-UI-001 — Frontend Solution: Authentication & User API

**CR nguồn:** [CR-UI-001](../../../../../specs/crs/v0/ui-api/CR-UI-001-auth-api.md)  
**Ngày tạo:** 2026-06-16  
**Trạng thái:** Proposed  
**Ưu tiên:** P0 — Critical (blocking toàn bộ UI)  
**Phạm vi:** Frontend React SPA (`ui/src/features/auth/`)

---

## 1. Tóm tắt giải pháp

CR-UI-001 yêu cầu Auth API hoàn chỉnh từ backend. Phía frontend cần:
1. Implement `authApi.ts` — tất cả API calls (login/refresh/me/logout/MFA/OAuth)
2. Implement `authStore.ts` — Zustand store cho auth state (JWT in-memory, không localStorage)
3. Implement `useAuth.ts` hook — orchestrate login flow, token refresh, session restore
4. Setup Axios interceptor — auto-refresh token khi 401
5. MSW handlers — mock auth endpoints khi backend chưa sẵn sàng

---

## 2. File Structure

```
ui/src/
├── features/auth/
│   ├── api/
│   │   └── authApi.ts              # API calls cho tất cả auth endpoints
│   ├── hooks/
│   │   ├── useAuth.ts              # Main auth hook (login, logout, restore session)
│   │   └── usePermissions.ts       # RBAC permission checks
│   ├── store/
│   │   └── authStore.ts            # Zustand store (in-memory JWT)
│   ├── components/
│   │   ├── LoginScreen.tsx
│   │   ├── MFAVerify.tsx
│   │   ├── MFASetup.tsx
│   │   └── OAuthCallback.tsx
│   └── types.ts                    # Auth-specific types
│
├── shared/
│   └── api/
│       └── client.ts               # Axios instance + JWT interceptor
│
└── mocks/
    └── handlers/
        └── auth.handlers.ts        # MSW handlers cho auth endpoints
```

---

## 3. Implementation Chi Tiết

### 3.1 `features/auth/api/authApi.ts`

```typescript
import apiClient from '@/shared/api/client';
import type {
  LoginRequest, LoginResponse, RefreshResponse,
  MFASetupResponse, MFAConfirmRequest, User
} from '../types';

export const authApi = {
  // POST /api/v1/auth/login
  login: async (payload: LoginRequest): Promise<LoginResponse> => {
    const { data } = await apiClient.post<LoginResponse>('/api/v1/auth/login', payload);
    return data;
  },

  // POST /api/v1/auth/refresh — cookie tự động gửi kèm
  refresh: async (): Promise<RefreshResponse> => {
    const { data } = await apiClient.post<RefreshResponse>('/api/v1/auth/refresh');
    return data;
  },

  // GET /api/v1/auth/me
  me: async (): Promise<{ user: User }> => {
    const { data } = await apiClient.get<{ user: User }>('/api/v1/auth/me');
    return data;
  },

  // POST /api/v1/auth/logout
  logout: async (): Promise<void> => {
    await apiClient.post('/api/v1/auth/logout');
  },

  // GET /api/v1/auth/mfa/setup [v3.0]
  mfaSetup: async (): Promise<MFASetupResponse> => {
    const { data } = await apiClient.get<MFASetupResponse>('/api/v1/auth/mfa/setup');
    return data;
  },

  // POST /api/v1/auth/mfa/confirm [v3.0]
  mfaConfirm: async (payload: MFAConfirmRequest): Promise<{ success: boolean; mfa_enabled: boolean }> => {
    const { data } = await apiClient.post('/api/v1/auth/mfa/confirm', payload);
    return data;
  },

  // OAuth2 — navigate browser đến endpoint (không dùng apiClient)
  oauthGoogle: (): void => {
    window.location.href = '/api/v1/auth/oauth/google';
  },
  oauthGithub: (): void => {
    window.location.href = '/api/v1/auth/oauth/github';
  },
};
```

### 3.2 `features/auth/store/authStore.ts`

```typescript
import { create } from 'zustand';
import { persist } from 'zustand/middleware';
import type { User, AuthState } from '../types';

export const useAuthStore = create<AuthState>()(
  persist(
    (set) => ({
      user: null,
      accessToken: null,        // In-memory ONLY — KHÔNG vào localStorage
      isAuthenticated: false,
      isLoading: true,          // true khi đang khởi tạo (check session)

      setUser: (user: User) => set({ user, isAuthenticated: true }),
      setAccessToken: (accessToken: string) => set({ accessToken }),
      setLoading: (isLoading: boolean) => set({ isLoading }),

      logout: () => set({
        user: null,
        accessToken: null,
        isAuthenticated: false,
      }),
    }),
    {
      name: 'osv-auth',
      // Chỉ persist user object (tên, role) — KHÔNG persist access token
      partialize: (state) => ({ user: state.user }),
    }
  )
);

// Expose store getState cho Axios interceptor (outside React)
export const authStore = useAuthStore;
```

### 3.3 `shared/api/client.ts` — Axios + JWT Interceptor

```typescript
import axios from 'axios';
import { authStore } from '@/features/auth/store/authStore';

const apiClient = axios.create({
  baseURL: import.meta.env.VITE_API_BASE_URL || 'http://localhost:8080',
  timeout: 30_000,
  withCredentials: true,  // Quan trọng: tự động gửi httpOnly refresh cookie
});

// REQUEST INTERCEPTOR: inject Bearer token
apiClient.interceptors.request.use((config) => {
  const token = authStore.getState().accessToken;
  if (token) {
    config.headers.Authorization = `Bearer ${token}`;
  }
  return config;
});

// RESPONSE INTERCEPTOR: handle 401 → auto refresh
let isRefreshing = false;
let refreshQueue: Array<(token: string) => void> = [];

apiClient.interceptors.response.use(
  (response) => response,
  async (error) => {
    const originalRequest = error.config;

    if (error.response?.status === 401 && !originalRequest._retry) {
      originalRequest._retry = true;

      if (isRefreshing) {
        // Queue request, chờ refresh hoàn thành
        return new Promise((resolve) => {
          refreshQueue.push((token: string) => {
            originalRequest.headers.Authorization = `Bearer ${token}`;
            resolve(apiClient(originalRequest));
          });
        });
      }

      isRefreshing = true;
      try {
        const { data } = await axios.post('/api/v1/auth/refresh', {}, {
          baseURL: import.meta.env.VITE_API_BASE_URL || 'http://localhost:8080',
          withCredentials: true,
        });

        const newToken = data.access_token;
        authStore.getState().setAccessToken(newToken);

        // Drain queue
        refreshQueue.forEach((cb) => cb(newToken));
        refreshQueue = [];
        isRefreshing = false;

        originalRequest.headers.Authorization = `Bearer ${newToken}`;
        return apiClient(originalRequest);

      } catch (refreshError) {
        // Refresh failed → logout
        authStore.getState().logout();
        refreshQueue = [];
        isRefreshing = false;
        window.location.href = '/login';
        return Promise.reject(refreshError);
      }
    }

    return Promise.reject(error);
  }
);

export default apiClient;
```

### 3.4 `features/auth/hooks/useAuth.ts`

```typescript
import { useCallback } from 'react';
import { useNavigate, useLocation } from 'react-router-dom';
import { useMutation, useQueryClient } from '@tanstack/react-query';
import { useAuthStore } from '../store/authStore';
import { authApi } from '../api/authApi';
import type { LoginRequest } from '../types';

export function useAuth() {
  const { user, accessToken, isAuthenticated, isLoading, setUser, setAccessToken, logout: storeLogout } = useAuthStore();
  const navigate = useNavigate();
  const location = useLocation();
  const queryClient = useQueryClient();

  // Login mutation
  const loginMutation = useMutation({
    mutationFn: authApi.login,
    onSuccess: (data) => {
      if (data.mfa_required) {
        // Redirect to MFA verify
        navigate('/login/mfa', { state: { mfaRequired: true } });
        return;
      }
      if (data.access_token && data.user) {
        setAccessToken(data.access_token);
        setUser(data.user);
        // Navigate back to original page hoặc dashboard
        const from = (location.state as { from?: Location })?.from?.pathname || '/dashboard';
        navigate(from, { replace: true });
      }
    },
  });

  // Logout mutation
  const logoutMutation = useMutation({
    mutationFn: authApi.logout,
    onSettled: () => {
      // Clear tất cả dù có lỗi hay không
      storeLogout();
      queryClient.clear();
      navigate('/login', { replace: true });
    },
  });

  // Restore session: gọi /api/v1/auth/me khi app khởi động
  const restoreSession = useCallback(async () => {
    try {
      const { user } = await authApi.me();
      setUser(user);
    } catch {
      storeLogout();
    }
  }, [setUser, storeLogout]);

  const login = useCallback(
    (payload: LoginRequest) => loginMutation.mutate(payload),
    [loginMutation]
  );

  const logout = useCallback(
    () => logoutMutation.mutate(),
    [logoutMutation]
  );

  return {
    user,
    accessToken,
    isAuthenticated,
    isLoading,
    login,
    logout,
    restoreSession,
    isLoggingIn: loginMutation.isPending,
    loginError: loginMutation.error,
  };
}
```

### 3.5 `features/auth/hooks/usePermissions.ts`

```typescript
import { useAuthStore } from '../store/authStore';
import type { Permission } from '../types';

export function usePermissions() {
  const { user } = useAuthStore();

  const hasPermission = (permission: Permission): boolean =>
    user?.permissions.includes(permission) ?? false;

  return {
    // Scan
    canCreateScan: hasPermission('scan:create'),
    canReadScan: hasPermission('scan:read'),

    // Finding
    canWriteFindings: hasPermission('finding:write'),
    canReadFindings: hasPermission('finding:read'),

    // Asset
    canWriteAssets: hasPermission('asset:write'),
    canReadAssets: hasPermission('asset:read'),

    // Reports
    canDownloadReports: hasPermission('report:download'),

    // Admin
    canManageUsers: hasPermission('user:manage'),
    canConfigureSystem: hasPermission('system:configure'),

    // Agent
    canReportAsAgent: hasPermission('agent:report'),

    // Derived
    isAdmin: user?.role === 'admin',
    isReadonly: user?.role === 'readonly',

    // Raw
    hasPermission,
  };
}
```

### 3.6 `features/auth/types.ts`

```typescript
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
  mfa_enabled: boolean;        // snake_case: match API response
  avatar_url?: string | null;
  created_at: string;
}

export interface LoginRequest {
  email: string;
  password: string;
  mfa_code?: string;
}

export interface LoginResponse {
  access_token: string | null;
  expires_in: number;
  user: User | null;
  mfa_required?: boolean;
}

export interface RefreshResponse {
  access_token: string;
  expires_in: number;
}

export interface MFASetupResponse {
  secret: string;
  qr_url: string;
  backup_codes: string[];
}

export interface MFAConfirmRequest {
  code: string;
}

export interface AuthState {
  user: User | null;
  accessToken: string | null;
  isAuthenticated: boolean;
  isLoading: boolean;
  setUser: (user: User) => void;
  setAccessToken: (token: string) => void;
  setLoading: (loading: boolean) => void;
  logout: () => void;
}
```

### 3.7 MSW Handler: `mocks/handlers/auth.handlers.ts`

```typescript
import { http, HttpResponse } from 'msw';
import type { LoginRequest, LoginResponse, RefreshResponse } from '@/features/auth/types';

// Fixture users
const users = {
  admin: {
    id: 'usr_admin001',
    email: 'admin@company.com',
    name: 'Admin User',
    role: 'admin' as const,
    permissions: ['scan:create','scan:read','asset:write','asset:read',
      'finding:write','finding:read','report:download','user:manage','system:configure'],
    mfa_enabled: false,
    avatar_url: null,
    created_at: '2026-01-01T00:00:00Z',
  },
  bob: {
    id: 'usr_bob123',
    email: 'bob@company.com',
    name: 'Bob Smith',
    role: 'user' as const,
    permissions: ['scan:create','scan:read','asset:write','asset:read',
      'finding:write','finding:read','report:download'],
    mfa_enabled: false,
    avatar_url: null,
    created_at: '2026-01-15T08:00:00Z',
  },
};

let currentUser = users.bob;
let fakeToken = 'mock-access-token-' + Date.now();

export const authHandlers = [
  // POST /api/v1/auth/login
  http.post('/api/v1/auth/login', async ({ request }) => {
    const body = await request.json() as LoginRequest;

    if (body.email === 'admin@company.com') {
      currentUser = users.admin;
    } else if (body.email === 'bob@company.com') {
      currentUser = users.bob;
    } else {
      return HttpResponse.json(
        { error: 'INVALID_CREDENTIALS', message: 'Invalid email or password', details: {}, trace_id: 'mock-001' },
        { status: 401 }
      );
    }

    fakeToken = 'mock-access-token-' + Date.now();

    return HttpResponse.json({
      access_token: fakeToken,
      expires_in: 900,
      user: currentUser,
    } satisfies LoginResponse);
  }),

  // POST /api/v1/auth/refresh
  http.post('/api/v1/auth/refresh', () => {
    fakeToken = 'mock-refreshed-token-' + Date.now();
    return HttpResponse.json({
      access_token: fakeToken,
      expires_in: 900,
    } satisfies RefreshResponse);
  }),

  // GET /api/v1/auth/me
  http.get('/api/v1/auth/me', () => {
    return HttpResponse.json({ user: currentUser });
  }),

  // POST /api/v1/auth/logout
  http.post('/api/v1/auth/logout', () => {
    return HttpResponse.json({ success: true });
  }),

  // GET /api/v1/auth/mfa/setup
  http.get('/api/v1/auth/mfa/setup', () => {
    return HttpResponse.json({
      secret: 'JBSWY3DPEHPK3PXP',
      qr_url: 'otpauth://totp/OSV:bob@company.com?secret=JBSWY3DPEHPK3PXP&issuer=OSV%20Platform',
      backup_codes: ['a1b2-c3d4', 'e5f6-g7h8', 'i9j0-k1l2', 'm3n4-o5p6'],
    });
  }),

  // POST /api/v1/auth/mfa/confirm
  http.post('/api/v1/auth/mfa/confirm', async ({ request }) => {
    const body = await request.json() as { code: string };
    if (body.code === '123456') {
      return HttpResponse.json({ success: true, mfa_enabled: true });
    }
    return HttpResponse.json(
      { error: 'INVALID_MFA_CODE', message: 'TOTP code is invalid or expired', details: {}, trace_id: 'mock-002' },
      { status: 400 }
    );
  }),
];
```

---

## 4. Session Restore Strategy

```typescript
// app/App.tsx — khởi tạo session khi app load
function App() {
  const { restoreSession, isAuthenticated } = useAuth();
  const { setLoading } = useAuthStore();

  useEffect(() => {
    // Gọi /api/v1/auth/me để check xem refresh cookie còn hợp lệ không
    restoreSession().finally(() => setLoading(false));
  }, []);

  // ...
}
```

**Luồng restore:**
1. App khởi động → `isLoading = true`
2. Gọi `GET /api/v1/auth/me` (Axios sẽ gửi kèm httpOnly refresh cookie)
3. Nếu 200 → set user, `isAuthenticated = true`
4. Nếu 401 → Axios interceptor thử `POST /api/v1/auth/refresh` (cookie)
5. Nếu refresh thành công → retry `/me` → set user
6. Nếu cả hai fail → `logout()` → redirect `/login`
7. `isLoading = false`

---

## 5. OAuth2 Callback Handler

```typescript
// features/auth/components/OAuthCallback.tsx
export function OAuthCallback() {
  const navigate = useNavigate();
  const { setAccessToken, setUser } = useAuthStore();

  useEffect(() => {
    const params = new URLSearchParams(window.location.search);
    const token = params.get('access_token');

    if (token) {
      setAccessToken(token);
      // Fetch user info
      authApi.me().then(({ user }) => {
        setUser(user);
        navigate('/dashboard', { replace: true });
      });
    } else {
      navigate('/login?error=oauth_failed', { replace: true });
    }
  }, []);

  return <FullPageSpinner label="Completing OAuth login..." />;
}
```

---

## 6. AuthGuard Component

```typescript
// app/components/AuthGuard.tsx
import { Navigate, useLocation } from 'react-router-dom';
import { useAuthStore } from '@/features/auth/store/authStore';

export function AuthGuard({ children }: { children: React.ReactNode }) {
  const { isAuthenticated, isLoading } = useAuthStore();
  const location = useLocation();

  if (isLoading) return <FullPageSpinner />;

  if (!isAuthenticated) {
    return <Navigate to="/login" state={{ from: location }} replace />;
  }

  return <>{children}</>;
}
```

---

## 7. Acceptance Criteria (Frontend)

- [ ] Login form gọi `POST /api/v1/auth/login`, lưu `access_token` vào Zustand (memory only)
- [ ] Khi 401 → Axios interceptor tự động gọi `POST /api/v1/auth/refresh`, retry request
- [ ] Khi refresh thất bại → logout và redirect `/login`
- [ ] App khởi động → restore session qua `GET /api/v1/auth/me`
- [ ] `usePermissions()` trả về đúng quyền từ JWT claims/permissions array
- [ ] `AuthGuard` redirect về `/login` khi `isAuthenticated = false`
- [ ] MSW handlers chặn tất cả auth requests khi `VITE_ENABLE_MSW=true`
- [ ] OAuth callback parse `access_token` từ URL, set vào store
- [ ] `access_token` không bao giờ lưu vào `localStorage` hay `sessionStorage`

---

## 8. Dependencies Frontend

| Thư viện | Version | Mục đích |
|---------|---------|---------|
| `axios` | ^1.7 | HTTP client với interceptors |
| `zustand` | ^5.0 | Auth state (in-memory token) |
| `zustand/middleware` (persist) | built-in | Persist user object (tên, role) |
| `@tanstack/react-query` | ^5.0 | Login/logout mutations |
| `react-router-dom` | ^7.0 | Navigation sau login/logout |
| `msw` | ^2.0 | Mock auth API khi dev |

---

## 9. Phụ thuộc Backend (Blocking)

| Endpoint | Trạng thái | Phase |
|----------|-----------|-------|
| `POST /api/v1/auth/login` | ❌ Thiếu | Sprint 1 |
| `POST /api/v1/auth/refresh` | ❌ Thiếu | Sprint 1 |
| `GET /api/v1/auth/me` | ❌ Thiếu | Sprint 1 |
| `POST /api/v1/auth/logout` | ❌ Thiếu | Sprint 1 |
| `GET /api/v1/auth/mfa/setup` | ❌ v3.0 | Sprint 4 |
| OAuth2 endpoints | ❌ v3.0 | Sprint 4 |
