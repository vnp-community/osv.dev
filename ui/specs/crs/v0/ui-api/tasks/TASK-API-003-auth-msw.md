# TASK-API-003 — Auth MSW Handlers + Session Restore + AuthGuard + OAuthCallback

| Field | Value |
|-------|-------|
| **Task ID** | TASK-API-003 |
| **Module** | `ui/src/mocks/handlers/`, `ui/src/features/auth/components/`, `ui/src/app/` |
| **Solution Ref** | [SOL-UI-001 §3.7, §4, §5, §6](../solutions/SOL-UI-001-auth-api.md) |
| **Priority** | 🔴 P0 |
| **Depends On** | TASK-API-002 |
| **Estimated** | 1h |

---

## Context

Sau khi có authStore + authApi, cần:
1. MSW handlers để mock auth endpoints khi `VITE_ENABLE_MSW=true`
2. Session restore logic trong App root (gọi `/me` khi startup)
3. `AuthGuard` component để protect routes
4. `OAuthCallback` component cho OAuth2 redirect flow

---

## Goal

Hoàn thiện auth flow end-to-end: login → token → session restore → guard → logout.

---

## Target Files

| Action | File Path |
|--------|-----------|
| CREATE | `ui/src/mocks/handlers/auth.handlers.ts` |
| CREATE | `ui/src/mocks/fixtures/auth.fixture.ts` |
| CREATE | `ui/src/features/auth/components/OAuthCallback.tsx` |
| CREATE | `ui/src/app/components/AuthGuard.tsx` |
| MODIFY | `ui/src/app/App.tsx` (thêm session restore + AuthGuard) |

---

## Implementation

### File 1: `ui/src/mocks/fixtures/auth.fixture.ts`

```typescript
import type { User } from '@/features/auth/types';

// Fixtures — dùng trong MSW handlers. KHÔNG import vào components.
export const userFixtures: Record<string, User> = {
  admin: {
    id: 'usr_admin001',
    email: 'admin@company.com',
    name: 'Admin User',
    role: 'admin',
    permissions: [
      'scan:create', 'scan:read',
      'asset:write', 'asset:read',
      'finding:write', 'finding:read',
      'report:download',
      'user:manage',
      'system:configure',
    ],
    mfa_enabled: true,
    avatar_url: null,
    created_at: '2026-01-01T00:00:00Z',
  },
  bob: {
    id: 'usr_bob123',
    email: 'bob@company.com',
    name: 'Bob Smith',
    role: 'user',
    permissions: [
      'scan:create', 'scan:read',
      'asset:write', 'asset:read',
      'finding:write', 'finding:read',
      'report:download',
    ],
    mfa_enabled: false,
    avatar_url: null,
    created_at: '2026-01-15T08:00:00Z',
  },
  readonly: {
    id: 'usr_carol456',
    email: 'carol@company.com',
    name: 'Carol Jones',
    role: 'readonly',
    permissions: ['scan:read', 'asset:read', 'finding:read', 'report:download'],
    mfa_enabled: false,
    avatar_url: null,
    created_at: '2026-02-01T00:00:00Z',
  },
};

// Map email → fixture key
export function getUserByEmail(email: string): User | null {
  const entry = Object.values(userFixtures).find(u => u.email === email);
  return entry ?? null;
}
```

### File 2: `ui/src/mocks/handlers/auth.handlers.ts`

```typescript
import { http, HttpResponse } from 'msw';
import { ENDPOINTS } from '@/shared/api/endpoints';
import { getUserByEmail, userFixtures } from '../fixtures/auth.fixture';
import type { LoginRequest } from '@/features/auth/types';

// Session state (in-memory — reset mỗi khi MSW worker restart)
let currentUser = userFixtures.bob;
let fakeToken = 'mock-access-token-init';

export const authHandlers = [
  // POST /api/v1/auth/login
  http.post(ENDPOINTS.auth.login, async ({ request }) => {
    const body = await request.json() as LoginRequest;

    const user = getUserByEmail(body.email);
    if (!user) {
      return HttpResponse.json(
        {
          error: 'INVALID_CREDENTIALS',
          message: 'Invalid email or password',
          details: {},
          trace_id: 'mock-auth-001',
        },
        { status: 401 }
      );
    }

    currentUser = user;
    fakeToken = `mock-token-${user.role}-${Date.now()}`;

    return HttpResponse.json({
      access_token: fakeToken,
      expires_in: 900,
      user: currentUser,
      mfa_required: false,
    });
  }),

  // POST /api/v1/auth/refresh
  http.post(ENDPOINTS.auth.refresh, () => {
    fakeToken = `mock-refreshed-${Date.now()}`;
    return HttpResponse.json({
      access_token: fakeToken,
      expires_in: 900,
    });
  }),

  // GET /api/v1/auth/me
  http.get(ENDPOINTS.auth.me, () => {
    return HttpResponse.json({ user: currentUser });
  }),

  // POST /api/v1/auth/logout
  http.post(ENDPOINTS.auth.logout, () => {
    return HttpResponse.json({ success: true });
  }),

  // GET /api/v1/auth/mfa/setup
  http.get(ENDPOINTS.auth.mfaSetup, () => {
    return HttpResponse.json({
      secret: 'JBSWY3DPEHPK3PXP',
      qr_url: `otpauth://totp/OSV:${currentUser.email}?secret=JBSWY3DPEHPK3PXP&issuer=OSV%20Platform`,
      backup_codes: ['a1b2-c3d4', 'e5f6-g7h8', 'i9j0-k1l2', 'm3n4-o5p6'],
    });
  }),

  // POST /api/v1/auth/mfa/confirm
  http.post(ENDPOINTS.auth.mfaConfirm, async ({ request }) => {
    const body = await request.json() as { code: string };
    if (body.code === '123456') {
      return HttpResponse.json({ success: true, mfa_enabled: true });
    }
    return HttpResponse.json(
      { error: 'INVALID_MFA_CODE', message: 'TOTP code is invalid', details: {}, trace_id: 'mock-mfa-001' },
      { status: 400 }
    );
  }),
];
```

### File 3: `ui/src/app/components/AuthGuard.tsx`

```typescript
import { Navigate, useLocation } from 'react-router-dom';
import { useAuthStore } from '@/features/auth/store/authStore';
import { FullPageSpinner } from '@/shared/components/feedback/LoadingSpinner';

interface AuthGuardProps {
  children: React.ReactNode;
  /** Optional: require specific permission */
  requirePermission?: string;
}

export function AuthGuard({ children }: AuthGuardProps) {
  const { isAuthenticated, isLoading } = useAuthStore();
  const location = useLocation();

  if (isLoading) {
    return <FullPageSpinner label="Loading session..." />;
  }

  if (!isAuthenticated) {
    return <Navigate to="/login" state={{ from: location }} replace />;
  }

  return <>{children}</>;
}
```

### File 4: `ui/src/features/auth/components/OAuthCallback.tsx`

```typescript
import { useEffect } from 'react';
import { useNavigate } from 'react-router-dom';
import { useAuthStore } from '../store/authStore';
import { authApi } from '../api/authApi';
import { FullPageSpinner } from '@/shared/components/feedback/LoadingSpinner';

export function OAuthCallback() {
  const navigate = useNavigate();
  const { setAccessToken, setUser } = useAuthStore();

  useEffect(() => {
    const params = new URLSearchParams(window.location.search);
    const token = params.get('access_token');
    const error = params.get('error');

    if (error || !token) {
      navigate('/login?error=oauth_failed', { replace: true });
      return;
    }

    setAccessToken(token);
    authApi.me()
      .then(({ user }) => {
        setUser(user);
        navigate('/dashboard', { replace: true });
      })
      .catch(() => {
        navigate('/login?error=oauth_failed', { replace: true });
      });
  }, []);

  return <FullPageSpinner label="Completing login..." />;
}
```

### File 5: `ui/src/app/App.tsx` (MODIFY — thêm session restore)

Tìm App component root và thêm session restore:

```typescript
// Thêm vào App.tsx
import { useEffect } from 'react';
import { useAuth } from '@/features/auth/hooks/useAuth';

// Bên trong App component (phải là child của Router + QueryClientProvider):
function SessionRestorer() {
  const { restoreSession } = useAuth();

  useEffect(() => {
    // Gọi /api/v1/auth/me khi app khởi động
    // Interceptor sẽ tự động thử refresh nếu 401
    restoreSession();
  }, []); // empty deps — chỉ chạy 1 lần khi mount

  return null;
}

// Render <SessionRestorer /> ngay bên trong App, trước bất kỳ route nào:
// <Router>
//   <SessionRestorer />
//   <Routes>...</Routes>
// </Router>
```

---

## Verification

```bash
cd ui/
VITE_ENABLE_MSW=true pnpm dev

# Test login flow:
# 1. Mở http://localhost:3000
# 2. Redirect đến /login (AuthGuard hoạt động)
# 3. Login với bob@company.com / any-password → redirect /dashboard
# 4. F5 → /dashboard vẫn giữ (session restore qua /me)
# 5. Logout → redirect /login

# TypeScript check
npx tsc --noEmit

# Verify không hardcode credentials trong components
grep -rn "password\|secret\|token" src/features/auth/components/
# Expected: chỉ prop names, không có giá trị cứng
```

---

## Checklist

- [ ] `mocks/fixtures/auth.fixture.ts` — 3 users (admin, bob/user, carol/readonly), `getUserByEmail()`
- [ ] `mocks/handlers/auth.handlers.ts` — 6 handlers dùng `ENDPOINTS.auth.*`
- [ ] Handler login: email không tìm thấy → 401 JSON theo chuẩn error format
- [ ] Handler refresh: trả về token mới mỗi lần
- [ ] Handler me: trả về `currentUser` (stateful mock)
- [ ] `AuthGuard.tsx` — redirect `/login` khi `!isAuthenticated`, spinner khi `isLoading`
- [ ] `OAuthCallback.tsx` — parse `?access_token=` từ URL, set store, fetch `/me`, redirect
- [ ] `App.tsx` — `SessionRestorer` component gọi `restoreSession()` khi mount
- [ ] `npx tsc --noEmit` không lỗi
- [ ] Manual test: login → F5 → vẫn authenticated (session restore)
