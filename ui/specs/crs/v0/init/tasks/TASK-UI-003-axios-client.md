# TASK-UI-003 — Axios Client + JWT Interceptors

| Field | Value |
|-------|-------|
| **Task ID** | TASK-UI-003 |
| **Module** | `ui/src/shared/api/` |
| **Solution Ref** | [SOL-002 §4](../solutions/SOL-002-phase1-foundation.md#4-step-3-shared-api-infrastructure) |
| **Priority** | 🔴 P0 |
| **Depends On** | TASK-UI-002 |
| **Estimated** | 1h |
| **Status** | ✅ Completed |
| **Completed** | 2026-06-17 |

---

## Context

Hiện tại không có HTTP client nào. Tất cả API call sẽ đi qua Axios instance dùng chung với: JWT Bearer injection, 401 → auto-refresh, error handling chuẩn hóa. `endpoints.ts` là single source of truth cho tất cả URL API.

---

## Goal

Tạo Axios client với interceptors + file endpoint constants.

---

## Target Files

| Action | File Path |
|--------|-----------|
| CREATE | `ui/src/shared/api/client.ts` |
| CREATE | `ui/src/shared/api/endpoints.ts` |

---

## Implementation

### File 1: `ui/src/shared/api/client.ts`

```typescript
import axios from 'axios';

// Import lazy để tránh circular dependency với authStore
// authStore sẽ import client.ts → không import authStore trực tiếp ở top level
let getAccessToken: () => string | null = () => null;
let setAccessToken: (token: string) => void = () => {};
let logoutFn: () => void = () => {};

/**
 * Inject auth store functions sau khi store được khởi tạo.
 * Gọi trong app/providers.tsx sau khi authStore ready.
 */
export function injectAuthStore(
  getToken: () => string | null,
  setToken: (token: string) => void,
  logout: () => void
) {
  getAccessToken = getToken;
  setAccessToken = setToken;
  logoutFn = logout;
}

export const apiClient = axios.create({
  baseURL: import.meta.env.VITE_API_BASE_URL || 'http://localhost:8080',
  timeout: 30_000,
  headers: {
    'Content-Type': 'application/json',
  },
  withCredentials: true, // Gửi httpOnly cookie (refresh token)
});

// ─── Request Interceptor: Inject JWT Bearer ────────────────────────────────
apiClient.interceptors.request.use(
  (config) => {
    const token = getAccessToken();
    if (token) {
      config.headers.Authorization = `Bearer ${token}`;
    }
    return config;
  },
  (error) => Promise.reject(error)
);

// ─── Response Interceptor: 401 → Refresh Token ────────────────────────────
apiClient.interceptors.response.use(
  (response) => response,
  async (error) => {
    const originalRequest = error.config;

    // Chỉ retry 1 lần (tránh vòng lặp vô tận)
    if (error.response?.status === 401 && !originalRequest._retry) {
      originalRequest._retry = true;
      try {
        // POST /api/v1/auth/refresh — refresh token từ httpOnly cookie
        const { data } = await axios.post(
          `${import.meta.env.VITE_API_BASE_URL || 'http://localhost:8080'}/api/v1/auth/refresh`,
          {},
          { withCredentials: true }
        );

        const newToken: string = data.access_token;
        setAccessToken(newToken);

        // Retry original request với token mới
        originalRequest.headers.Authorization = `Bearer ${newToken}`;
        return apiClient(originalRequest);
      } catch {
        // Refresh thất bại → logout và redirect
        logoutFn();
        window.location.href = '/login';
      }
    }

    return Promise.reject(error);
  }
);
```

### File 2: `ui/src/shared/api/endpoints.ts`

```typescript
/**
 * Single source of truth cho tất cả API endpoints.
 * Tất cả feature API modules import từ đây.
 */
export const ENDPOINTS = {
  // ─── Auth ──────────────────────────────────────────────────────────────
  auth: {
    login: '/api/v1/auth/login',
    refresh: '/api/v1/auth/refresh',
    logout: '/api/v1/auth/logout',
    me: '/api/v1/auth/me',
    mfaSetup: '/api/v1/auth/mfa/setup',
    mfaConfirm: '/api/v1/auth/mfa/confirm',
    oauthGoogle: '/api/v1/auth/oauth/google',
    oauthGitHub: '/api/v1/auth/oauth/github',
  },

  // ─── Dashboard ─────────────────────────────────────────────────────────
  dashboard: '/api/v1/dashboard',

  // ─── CVE Intelligence (v2) ─────────────────────────────────────────────
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
  browse: {
    vendors: '/api/v2/browse',
  },
  cwe: {
    detail: (id: string) => `/api/v2/cwe/${id}`,
    list: '/api/v2/cwe',
  },
  epss: {
    query: (cveId: string) => `/api/v2/epss/${cveId}`,
  },
  dbinfo: '/api/v2/dbinfo',

  // ─── Scans (v1) ────────────────────────────────────────────────────────
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
    scheduled: '/api/v1/scans/scheduled',
  },

  // ─── Findings (v1) ─────────────────────────────────────────────────────
  findings: {
    list: '/api/v1/findings',
    detail: (id: string) => `/api/v1/findings/${id}`,
    update: (id: string) => `/api/v1/findings/${id}`,
    bulkClose: '/api/v1/findings/bulk/close',
    audit: (id: string) => `/api/v1/findings/${id}/audit`,
  },

  // ─── Assets (v1) ───────────────────────────────────────────────────────
  assets: {
    list: '/api/v1/assets',
    detail: (id: string) => `/api/v1/assets/${id}`,
  },

  // ─── Products (v1) ─────────────────────────────────────────────────────
  products: {
    list: '/api/v1/products',
    detail: (id: string) => `/api/v1/products/${id}`,
    engagements: (productId: string) => `/api/v1/products/${productId}/engagements`,
  },

  // ─── Reports (v1) ──────────────────────────────────────────────────────
  reports: {
    list: '/api/v1/reports',
    create: '/api/v1/reports',
    detail: (id: string) => `/api/v1/reports/${id}`,
    download: (id: string, format: string) => `/api/v1/reports/${id}/download/${format}`,
  },

  // ─── AI (v1) ───────────────────────────────────────────────────────────
  ai: {
    triage: (findingId: string) => `/api/v1/ai/triage/${findingId}`,
    enrichment: '/api/v1/ai/enrichment',
  },

  // ─── Risk Acceptances (v1) ─────────────────────────────────────────────
  riskAcceptances: {
    list: '/api/v1/risk-acceptances',
    create: '/api/v1/risk-acceptances',
    detail: (id: string) => `/api/v1/risk-acceptances/${id}`,
  },

  // ─── Admin (v1) ────────────────────────────────────────────────────────
  admin: {
    users: '/api/v1/admin/users',
    user: (id: string) => `/api/v1/admin/users/${id}`,
    apiKeys: '/api/v1/api-keys',
    apiKey: (id: string) => `/api/v1/api-keys/${id}`,
    webhooks: '/api/v1/webhooks',
    webhook: (id: string) => `/api/v1/webhooks/${id}`,
    audit: '/api/v1/admin/audit',
    health: '/api/v1/admin/health',
    settings: '/api/v1/admin/settings',
    roles: '/api/v1/admin/roles',
  },

  // ─── Notifications ─────────────────────────────────────────────────────
  notifications: {
    stream: '/notifications/stream',
    list: '/api/v1/notifications',
    markRead: (id: string) => `/api/v1/notifications/${id}/read`,
  },
} as const;
```

---

## Verification

```bash
cd ui/

# Type check
npx tsc --noEmit

# Import test — không circular dependency
node -e "import('./src/shared/api/endpoints.js').then(m => console.log('OK:', Object.keys(m.ENDPOINTS)))"
```

**Expected:** `tsc --noEmit` không có lỗi, `ENDPOINTS` export đầy đủ.

---

## Checklist

- [x] `src/shared/api/client.ts` — Axios instance với `baseURL`, `timeout`, `withCredentials`
- [x] Request interceptor: inject `Authorization: Bearer {token}`
- [x] Response interceptor: `401` → POST `/api/v1/auth/refresh` → retry hoặc logout
- [x] `injectAuthStore()` function để tránh circular dependency
- [x] `src/shared/api/endpoints.ts` — tất cả endpoints của `auth`, `cve`, `kev`, `scans`, `findings`, `assets`, `products`, `reports`, `ai`, `admin`, `notifications`
- [x] `npx tsc --noEmit` không có lỗi
