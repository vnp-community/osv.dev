# TASK-API-001 — Cập nhật ENDPOINTS + MSW Handler Registration

| Field | Value |
|-------|-------|
| **Task ID** | TASK-API-001 |
| **Module** | `ui/src/shared/api/`, `ui/src/mocks/` |
| **Solution Ref** | [SOL-UI-001](../solutions/SOL-UI-001-auth-api.md), [SOL-UI-002](../solutions/SOL-UI-002-dashboard-api.md), [Solutions README](../solutions/README.md) |
| **Priority** | 🔴 P0 |
| **Depends On** | — (first task) |
| **Estimated** | 1h |

---

## Context

`src/shared/api/endpoints.ts` hiện có ~70% endpoints đã khai báo (từ init phase). Cần bổ sung các endpoints mới từ UI-API v2 chưa có: auth endpoints, dashboard SLA, notifications stream, findings bulk/reopen/assign/notes/stats, products grades, reports download, webhooks test, admin health/settings/roles.

`src/mocks/browser.ts` cũng cần import và đăng ký tất cả handlers mới từ các feature modules.

---

## Goal

1. Bổ sung toàn bộ endpoints còn thiếu vào `ENDPOINTS` constant
2. Cập nhật `src/mocks/browser.ts` để đăng ký tất cả 12 handler sets
3. Tạo `src/mocks/server.ts` cho Vitest (nếu chưa có)

---

## Target Files

| Action | File Path |
|--------|-----------|
| MODIFY | `ui/src/shared/api/endpoints.ts` |
| MODIFY | `ui/src/mocks/browser.ts` |
| CREATE | `ui/src/mocks/server.ts` |

---

## Implementation

### File 1: `ui/src/shared/api/endpoints.ts` (MODIFY — bổ sung thiếu)

Thêm các endpoint chưa có vào đúng section:

```typescript
export const ENDPOINTS = {
  // ─── Auth ──────────────────────────────────────────────────────────────
  auth: {
    login:        '/api/v1/auth/login',
    refresh:      '/api/v1/auth/refresh',
    logout:       '/api/v1/auth/logout',
    me:           '/api/v1/auth/me',
    mfaSetup:     '/api/v1/auth/mfa/setup',
    mfaConfirm:   '/api/v1/auth/mfa/confirm',
    oauthGoogle:  '/api/v1/auth/oauth/google',
    oauthGitHub:  '/api/v1/auth/oauth/github',
    oauthCallback:'/api/v1/auth/callback',
  },

  // ─── Dashboard ─────────────────────────────────────────────────────────
  dashboard: {
    metrics:  '/api/v1/dashboard',
    sla:      '/api/v1/dashboard/sla',
  },

  // ─── Notifications ─────────────────────────────────────────────────────
  notifications: {
    stream:       '/api/v1/notifications/stream',
    list:         '/api/v1/notifications',
    unreadCount:  '/api/v1/notifications/unread-count',
    markRead:     (id: string) => `/api/v1/notifications/${id}/read`,
    markAllRead:  '/api/v1/notifications/mark-all-read',
  },

  // ─── CVE Intelligence (v2) ─────────────────────────────────────────────
  cve: {
    search:       '/api/v2/cves/search',
    semantic:     '/api/v2/cves/search/semantic',
    detail:       (id: string) => `/api/v2/cves/${id}`,
    aggregations: '/api/v2/cves/aggregations',
    export:       '/api/v2/cves/export',
  },
  kev: {
    list:         '/api/v2/kev',
    stats:        '/api/v2/kev/stats',
    ransomware:   '/api/v2/kev/ransomware',
  },
  epss: {
    byCve:        (cveId: string) => `/api/v2/epss/${cveId}`,
    top:          '/api/v2/epss/top',
    distribution: '/api/v2/epss/distribution',
  },
  cwe: {
    list:         '/api/v2/cwe',
    detail:       (id: string) => `/api/v2/cwe/${id}`,
  },
  capec: {
    detail:       (id: string) => `/api/v2/capec/${id}`,
  },
  vendors:        '/api/v2/vendors',
  browse: {
    root:         '/api/v2/browse',
    byVendor:     (vendor: string) => `/api/v2/browse/${encodeURIComponent(vendor)}`,
    byProduct:    (vendor: string, product: string) =>
                    `/api/v2/browse/${encodeURIComponent(vendor)}/${encodeURIComponent(product)}`,
  },
  dbinfo:         '/api/v2/dbinfo',

  // ─── Scans (v1) ────────────────────────────────────────────────────────
  scans: {
    list:         '/api/v1/scans',
    create:       '/api/v1/scans',
    detail:       (id: string) => `/api/v1/scans/${id}`,
    stream:       (id: string) => `/api/v1/scans/${id}/stream`,
    cancel:       (id: string) => `/api/v1/scans/${id}/cancel`,
    nmap:         (id: string) => `/api/v1/scans/${id}/results/nmap`,
    zap:          (id: string) => `/api/v1/scans/${id}/results/zap`,
    scheduled:    '/api/v1/scans/scheduled',
    import:       '/api/v1/scans/import',
  },

  // ─── Findings (v1) ─────────────────────────────────────────────────────
  findings: {
    list:         '/api/v1/findings',
    stats:        '/api/v1/findings/stats',
    detail:       (id: string) => `/api/v1/findings/${id}`,
    patch:        (id: string) => `/api/v1/findings/${id}`,
    notes:        (id: string) => `/api/v1/findings/${id}/notes`,
    audit:        (id: string) => `/api/v1/findings/${id}/audit`,
    bulkClose:    '/api/v1/findings/bulk/close',
    bulkReopen:   '/api/v1/findings/bulk/reopen',
    bulkAssign:   '/api/v1/findings/bulk/assign',
  },
  riskAcceptances: {
    list:         '/api/v1/risk-acceptances',
    create:       '/api/v1/risk-acceptances',
    delete:       (id: string) => `/api/v1/risk-acceptances/${id}`,
  },
  sla: {
    config:       '/api/v1/sla/config',
  },

  // ─── Assets (v1) ───────────────────────────────────────────────────────
  assets: {
    list:         '/api/v1/assets',
    detail:       (id: string) => `/api/v1/assets/${id}`,
    findings:     (id: string) => `/api/v1/assets/${id}/findings`,
    patch:        (id: string) => `/api/v1/assets/${id}`,
    tags:         '/api/v1/assets/tags',
  },

  // ─── Products (v1) ─────────────────────────────────────────────────────
  products: {
    list:         '/api/v1/products',
    create:       '/api/v1/products',
    detail:       (id: string) => `/api/v1/products/${id}`,
    patch:        (id: string) => `/api/v1/products/${id}`,
    engagements:  (id: string) => `/api/v1/products/${id}/engagements`,
    grades:       '/api/v1/products/grades',
    types:        '/api/v1/products/types',
  },
  engagements: {
    tests:        (engId: string) => `/api/v1/engagements/${engId}/tests`,
  },

  // ─── AI (v1) ───────────────────────────────────────────────────────────
  ai: {
    triage:         (findingId: string) => `/api/v1/ai/triage/${findingId}`,
    triageReview:   (findingId: string) => `/api/v1/ai/triage/${findingId}/review`,
    triageQueue:    '/api/v1/ai/triage/queue',
    enrichment:     '/api/v1/ai/enrichment',
    enrichTrigger:  '/api/v1/ai/enrichment/trigger',
    enrichByCve:    (cveId: string) => `/api/v1/ai/enrichment/${cveId}`,
  },

  // ─── Reports (v1) ──────────────────────────────────────────────────────
  reports: {
    list:         '/api/v1/reports',
    create:       '/api/v1/reports',
    detail:       (id: string) => `/api/v1/reports/${id}`,
    download:     (id: string) => `/api/v1/reports/${id}/download`,
    delete:       (id: string) => `/api/v1/reports/${id}`,
  },

  // ─── Webhooks (v1) ─────────────────────────────────────────────────────
  webhooks: {
    list:         '/api/v1/webhooks',
    create:       '/api/v1/webhooks',
    delete:       (id: string) => `/api/v1/webhooks/${id}`,
    test:         (id: string) => `/api/v1/webhooks/${id}/test`,
  },

  // ─── API Keys (v1) ─────────────────────────────────────────────────────
  apiKeys: {
    list:         '/api/v1/api-keys',
    create:       '/api/v1/api-keys',
    revoke:       (id: string) => `/api/v1/api-keys/${id}`,
  },

  // ─── JIRA (v1) ─────────────────────────────────────────────────────────
  jira: {
    config:       '/api/v1/jira/config',
    test:         '/api/v1/jira/config/test',
  },

  // ─── Profile (v1) ──────────────────────────────────────────────────────
  profile: {
    get:            '/api/v1/profile',
    patch:          '/api/v1/profile',
    changePassword: '/api/v1/profile/change-password',
  },

  // ─── Admin (v1) ────────────────────────────────────────────────────────
  admin: {
    users:          '/api/v1/admin/users',
    userDetail:     (id: string) => `/api/v1/admin/users/${id}`,
    userInvite:     '/api/v1/admin/users/invite',
    userUnlock:     (id: string) => `/api/v1/admin/users/${id}/unlock`,
    userReset:      (id: string) => `/api/v1/admin/users/${id}/reset-password`,
    roles:          '/api/v1/admin/roles',
    health:         '/api/v1/admin/health',
    settings:       '/api/v1/admin/settings',
  },

  // ─── Audit (v1) ────────────────────────────────────────────────────────
  audit: {
    log:            '/api/v1/audit-log',
  },
} as const;
```

### File 2: `ui/src/mocks/browser.ts` (MODIFY — import tất cả handlers)

```typescript
import { setupWorker } from 'msw/browser';

// Import all handler sets (sẽ được tạo trong các tasks sau)
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

> [!NOTE]
> Nếu handler file chưa tồn tại khi chạy, import sẽ fail. Tạm thời comment out handlers chưa có và uncomment dần khi từng task hoàn thành. Hoặc tạo placeholder file rỗng export `[]`.

### File 3: `ui/src/mocks/server.ts` (CREATE — cho Vitest)

```typescript
import { setupServer } from 'msw/node';
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

export const server = setupServer(
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

### Placeholder handlers (tạm thời cho các handlers chưa có)

Tạo các file rỗng sau để không bị import error (sẽ được điền code trong các tasks sau):

```bash
# Tạo tất cả placeholder handler files
for name in auth dashboard cve scan finding asset product ai report notification admin integration; do
  echo "export const ${name}Handlers = [];" > ui/src/mocks/handlers/${name}.handlers.ts
done
```

---

## Verification

```bash
cd ui/

# Check TypeScript
npx tsc --noEmit
# Expected: no errors

# Verify ENDPOINTS có đủ keys
grep -c "'/api/" src/shared/api/endpoints.ts
# Expected: >= 50 endpoint strings

# Verify browser.ts import tất cả handlers
grep "import.*Handlers" src/mocks/browser.ts | wc -l
# Expected: 12

# Verify server.ts tồn tại
ls src/mocks/server.ts
# Expected: file exists
```

---

## Checklist

- [ ] `endpoints.ts`: auth section đầy đủ (9 endpoints)
- [ ] `endpoints.ts`: dashboard `metrics` + `sla` (tên object, không phải string đơn)
- [ ] `endpoints.ts`: notifications `stream`, `list`, `unreadCount`, `markRead`, `markAllRead`
- [ ] `endpoints.ts`: findings `stats`, `notes`, `bulkReopen`, `bulkAssign` (4 endpoints mới)
- [ ] `endpoints.ts`: products `grades`, `types` (2 endpoints mới)
- [ ] `endpoints.ts`: reports `download`, `delete` (2 endpoints mới)
- [ ] `endpoints.ts`: webhooks section đầy đủ
- [ ] `endpoints.ts`: ai section đầy đủ (6 endpoints)
- [ ] `endpoints.ts`: admin section đầy đủ (health, settings, roles, invite, unlock, reset)
- [ ] `browser.ts` import 12 handler sets
- [ ] `server.ts` tạo mới với setupServer (Vitest)
- [ ] Placeholder handler files tạo xong (12 files)
- [ ] `npx tsc --noEmit` không lỗi
