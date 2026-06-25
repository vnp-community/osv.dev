# TASK-API-002 — Auth Module: authApi + authStore + useAuth + usePermissions

| Field | Value |
|-------|-------|
| **Task ID** | TASK-API-002 |
| **Module** | `ui/src/features/auth/` |
| **Solution Ref** | [SOL-UI-001 §3](../solutions/SOL-UI-001-auth-api.md) |
| **Priority** | 🔴 P0 (blocking toàn bộ UI) |
| **Depends On** | TASK-API-001 |
| **Estimated** | 2h |

---

## Context

Auth là blocking task — không có auth thì không có gì chạy được. Cần triển khai:
- **Zustand store** lưu access token in-memory (KHÔNG localStorage)
- **Axios interceptor** đã có ở `shared/api/client.ts` — cần `injectAuthStore()` để inject store
- **authApi.ts** gọi tất cả auth endpoints
- **useAuth.ts** orchestrate login → token → user flow
- **usePermissions.ts** RBAC checks từ JWT claims

---

## Goal

Tạo feature module `src/features/auth/` hoàn chỉnh với types, API, store, hooks.

---

## Target Files

| Action | File Path |
|--------|-----------|
| CREATE | `ui/src/features/auth/types.ts` |
| CREATE | `ui/src/features/auth/api/authApi.ts` |
| CREATE | `ui/src/features/auth/store/authStore.ts` |
| CREATE | `ui/src/features/auth/hooks/useAuth.ts` |
| CREATE | `ui/src/features/auth/hooks/usePermissions.ts` |
| MODIFY | `ui/src/app/providers.tsx` (inject authStore vào apiClient) |

---

## Implementation

### File 1: `ui/src/features/auth/types.ts`

```typescript
export type UserRole = 'admin' | 'user' | 'readonly' | 'agent';

export type Permission =
  | 'scan:create' | 'scan:read'
  | 'asset:write' | 'asset:read'
  | 'finding:write' | 'finding:read'
  | 'report:download'
  | 'user:manage'
  | 'system:configure'
  | 'agent:report';

export interface User {
  id: string;
  email: string;
  name: string;
  role: UserRole;
  permissions: Permission[];
  mfa_enabled: boolean;
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

### File 2: `ui/src/features/auth/api/authApi.ts`

```typescript
import { apiClient } from '@/shared/api/client';
import { ENDPOINTS } from '@/shared/api/endpoints';
import type { LoginRequest, LoginResponse, RefreshResponse, MFASetupResponse, User } from '../types';

export const authApi = {
  login: async (payload: LoginRequest): Promise<LoginResponse> => {
    const { data } = await apiClient.post<LoginResponse>(ENDPOINTS.auth.login, payload);
    return data;
  },

  refresh: async (): Promise<RefreshResponse> => {
    const { data } = await apiClient.post<RefreshResponse>(ENDPOINTS.auth.refresh);
    return data;
  },

  me: async (): Promise<{ user: User }> => {
    const { data } = await apiClient.get<{ user: User }>(ENDPOINTS.auth.me);
    return data;
  },

  logout: async (): Promise<void> => {
    await apiClient.post(ENDPOINTS.auth.logout);
  },

  mfaSetup: async (): Promise<MFASetupResponse> => {
    const { data } = await apiClient.get<MFASetupResponse>(ENDPOINTS.auth.mfaSetup);
    return data;
  },

  mfaConfirm: async (code: string): Promise<{ success: boolean; mfa_enabled: boolean }> => {
    const { data } = await apiClient.post(ENDPOINTS.auth.mfaConfirm, { code });
    return data;
  },

  // OAuth2: redirect browser đến backend (không dùng apiClient)
  oauthGoogle: (): void => {
    window.location.href = ENDPOINTS.auth.oauthGoogle;
  },
  oauthGitHub: (): void => {
    window.location.href = ENDPOINTS.auth.oauthGitHub;
  },
};
```

### File 3: `ui/src/features/auth/store/authStore.ts`

```typescript
import { create } from 'zustand';
import { persist } from 'zustand/middleware';
import type { User, AuthState } from '../types';

export const useAuthStore = create<AuthState>()(
  persist(
    (set) => ({
      user: null,
      accessToken: null,       // In-memory ONLY — KHÔNG vào localStorage
      isAuthenticated: false,
      isLoading: true,

      setUser: (user: User) =>
        set({ user, isAuthenticated: true }),

      setAccessToken: (accessToken: string) =>
        set({ accessToken }),

      setLoading: (isLoading: boolean) =>
        set({ isLoading }),

      logout: () =>
        set({ user: null, accessToken: null, isAuthenticated: false }),
    }),
    {
      name: 'osv-auth',
      // Chỉ persist user object — TUYỆT ĐỐI KHÔNG persist accessToken
      partialize: (state) => ({ user: state.user }),
    }
  )
);

// Expose getState cho Axios interceptor (outside React tree)
export const authStoreApi = useAuthStore;
```

### File 4: `ui/src/features/auth/hooks/useAuth.ts`

```typescript
import { useCallback } from 'react';
import { useNavigate, useLocation } from 'react-router-dom';
import { useMutation, useQueryClient } from '@tanstack/react-query';
import { useAuthStore } from '../store/authStore';
import { authApi } from '../api/authApi';
import type { LoginRequest } from '../types';

export function useAuth() {
  const {
    user, accessToken, isAuthenticated, isLoading,
    setUser, setAccessToken, setLoading, logout: storeLogout,
  } = useAuthStore();
  const navigate = useNavigate();
  const location = useLocation();
  const queryClient = useQueryClient();

  // ─── Login ─────────────────────────────────────────────────────────────
  const loginMutation = useMutation({
    mutationFn: authApi.login,
    onSuccess: (data) => {
      if (data.mfa_required) {
        navigate('/login/mfa', { state: { mfaRequired: true } });
        return;
      }
      if (data.access_token && data.user) {
        setAccessToken(data.access_token);
        setUser(data.user);
        const from = (location.state as { from?: Location })?.from?.pathname ?? '/dashboard';
        navigate(from, { replace: true });
      }
    },
  });

  // ─── Logout ────────────────────────────────────────────────────────────
  const logoutMutation = useMutation({
    mutationFn: authApi.logout,
    onSettled: () => {
      storeLogout();
      queryClient.clear();
      navigate('/login', { replace: true });
    },
  });

  // ─── Session Restore ───────────────────────────────────────────────────
  // Gọi khi app khởi động để check refresh cookie còn hợp lệ không
  const restoreSession = useCallback(async () => {
    setLoading(true);
    try {
      const { user } = await authApi.me();
      setUser(user);
    } catch {
      storeLogout();
    } finally {
      setLoading(false);
    }
  }, [setUser, storeLogout, setLoading]);

  return {
    user,
    accessToken,
    isAuthenticated,
    isLoading,
    login:       (payload: LoginRequest) => loginMutation.mutate(payload),
    logout:      () => logoutMutation.mutate(),
    restoreSession,
    isLoggingIn: loginMutation.isPending,
    loginError:  loginMutation.error,
  };
}
```

### File 5: `ui/src/features/auth/hooks/usePermissions.ts`

```typescript
import { useAuthStore } from '../store/authStore';
import type { Permission } from '../types';

export function usePermissions() {
  const { user } = useAuthStore();

  const hasPermission = (permission: Permission): boolean =>
    user?.permissions.includes(permission) ?? false;

  return {
    // Scan
    canCreateScan:    hasPermission('scan:create'),
    canReadScan:      hasPermission('scan:read'),
    // Findings
    canWriteFindings: hasPermission('finding:write'),
    canReadFindings:  hasPermission('finding:read'),
    // Assets
    canWriteAssets:   hasPermission('asset:write'),
    canReadAssets:    hasPermission('asset:read'),
    // Reports
    canDownloadReports: hasPermission('report:download'),
    // Admin
    canManageUsers:      hasPermission('user:manage'),
    canConfigureSystem:  hasPermission('system:configure'),
    // Agent
    canReportAsAgent:    hasPermission('agent:report'),
    // Derived
    isAdmin:    user?.role === 'admin',
    isReadonly: user?.role === 'readonly',
    // Raw checker
    hasPermission,
  };
}
```

### File 6: `ui/src/app/providers.tsx` (MODIFY — inject authStore)

Tìm vị trí `<QueryClientProvider>` wrapper và thêm `useEffect` inject authStore:

```typescript
// Thêm vào providers.tsx — sau khi authStore available
import { useEffect } from 'react';
import { injectAuthStore } from '@/shared/api/client';
import { authStoreApi } from '@/features/auth/store/authStore';

function AuthStoreInjector() {
  useEffect(() => {
    injectAuthStore(
      () => authStoreApi.getState().accessToken,
      (token) => authStoreApi.getState().setAccessToken(token),
      () => authStoreApi.getState().logout(),
    );
  }, []);
  return null;
}

// Render <AuthStoreInjector /> bên trong <QueryClientProvider> nhưng ngoài Router
// để interceptor hoạt động từ đầu
```

---

## Verification

```bash
cd ui/

# TypeScript check
npx tsc --noEmit
# Expected: no errors

# Verify accessToken KHÔNG trong localStorage key
# (Kiểm tra partialize config)
grep "accessToken" src/features/auth/store/authStore.ts
# Expected: chỉ thấy trong state definition, KHÔNG trong partialize

# Verify không hardcode user data
grep -n "email.*@\|password.*=" src/features/auth/api/authApi.ts
# Expected: no output

# Verify exports
node -e "
  import('./src/features/auth/api/authApi.js').then(m => console.log('authApi:', Object.keys(m.authApi)));
"
```

---

## Checklist

- [ ] `features/auth/types.ts` — User, Permission, LoginRequest, LoginResponse, AuthState
- [ ] `features/auth/api/authApi.ts` — login, refresh, me, logout, mfaSetup, mfaConfirm, oauthGoogle, oauthGitHub
- [ ] `features/auth/api/authApi.ts` — dùng `ENDPOINTS.auth.*` (không string literal)
- [ ] `features/auth/store/authStore.ts` — Zustand persist, `partialize` chỉ persist `user`
- [ ] `features/auth/store/authStore.ts` — `accessToken` KHÔNG trong `partialize`
- [ ] `features/auth/hooks/useAuth.ts` — login, logout, restoreSession
- [ ] `features/auth/hooks/usePermissions.ts` — 10 permission getters
- [ ] `app/providers.tsx` — inject authStore vào apiClient interceptor
- [ ] `npx tsc --noEmit` không lỗi
