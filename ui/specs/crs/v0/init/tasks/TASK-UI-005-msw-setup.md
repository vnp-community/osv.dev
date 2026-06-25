# TASK-UI-005 — MSW Mock Layer Setup

| Field | Value |
|-------|-------|
| **Task ID** | TASK-UI-005 |
| **Module** | `ui/src/mocks/` |
| **Solution Ref** | [SOL-002 §6](../solutions/SOL-002-phase1-foundation.md#6-step-5-msw-setup-development-mock-layer), [SOL-003 §4.3](../solutions/SOL-003-phase2-api-migration.md) |
| **Priority** | 🔴 P0 |
| **Depends On** | TASK-UI-003 |
| **Estimated** | 1.5h |
| **Status** | ✅ Completed |
| **Completed** | 2026-06-17 |

---

## Context

MSW (Mock Service Worker) cho phép intercept HTTP requests trong browser và trả về mock responses có cấu trúc giống thật. Bật khi `VITE_ENABLE_MSW=true`. **Fixture data đặt trong `src/mocks/fixtures/`** — KHÔNG trong component files.

---

## Goal

Setup MSW browser worker + Node server + handlers cho tất cả API endpoints + fixture data cho dashboard và CVE (để test ngay Phase 2).

---

## Target Files

| Action | File Path |
|--------|-----------|
| CREATE | `ui/src/mocks/browser.ts` |
| CREATE | `ui/src/mocks/server.ts` |
| CREATE | `ui/src/mocks/handlers/index.ts` |
| CREATE | `ui/src/mocks/handlers/auth.handlers.ts` |
| CREATE | `ui/src/mocks/handlers/dashboard.handlers.ts` |
| CREATE | `ui/src/mocks/handlers/cve.handlers.ts` |
| CREATE | `ui/src/mocks/handlers/finding.handlers.ts` |
| CREATE | `ui/src/mocks/handlers/scan.handlers.ts` |
| CREATE | `ui/src/mocks/handlers/asset.handlers.ts` |
| CREATE | `ui/src/mocks/fixtures/dashboard.fixture.ts` |
| CREATE | `ui/src/mocks/fixtures/cves.fixture.ts` |
| CREATE | `ui/src/mocks/fixtures/findings.fixture.ts` |
| CREATE | `ui/src/mocks/fixtures/scans.fixture.ts` |
| RUN | `npx msw init public/` |

---

## Implementation

### Step 0: MSW Service Worker init

```bash
cd ui/
npx msw init public/ --save
# Tạo file public/mockServiceWorker.js
```

### File 1: `ui/src/mocks/browser.ts`

```typescript
import { setupWorker } from 'msw/browser';
import { handlers } from './handlers';

export const worker = setupWorker(...handlers);
```

### File 2: `ui/src/mocks/server.ts`

```typescript
// Node.js environment — dùng cho Vitest tests
import { setupServer } from 'msw/node';
import { handlers } from './handlers';

export const server = setupServer(...handlers);
```

### File 3: `ui/src/mocks/handlers/index.ts`

```typescript
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

### File 4: `ui/src/mocks/handlers/auth.handlers.ts`

```typescript
import { http, HttpResponse } from 'msw';

const mockUser = {
  id: 'user-001',
  email: 'admin@osv.local',
  name: 'OSV Admin',
  role: 'admin' as const,
  permissions: [
    'scan:create', 'scan:read', 'asset:write', 'asset:read',
    'user:manage', 'report:download', 'system:configure',
    'finding:write', 'finding:read',
  ],
  mfaEnabled: false,
  createdAt: '2025-01-01T00:00:00Z',
};

export const authHandlers = [
  // POST /api/v1/auth/login
  http.post('/api/v1/auth/login', async ({ request }) => {
    const body = await request.json() as { email: string; password: string };
    if (body.email === 'admin@osv.local' && body.password === 'password') {
      return HttpResponse.json({
        access_token: 'mock-jwt-token-admin',
        expires_in: 900,
        user: mockUser,
      });
    }
    return HttpResponse.json(
      { error: 'UNAUTHORIZED', message: 'Invalid credentials' },
      { status: 401 }
    );
  }),

  // POST /api/v1/auth/refresh
  http.post('/api/v1/auth/refresh', () => {
    return HttpResponse.json({
      access_token: 'mock-jwt-token-refreshed',
      expires_in: 900,
    });
  }),

  // GET /api/v1/auth/me
  http.get('/api/v1/auth/me', () => {
    return HttpResponse.json({ user: mockUser });
  }),

  // POST /api/v1/auth/logout
  http.post('/api/v1/auth/logout', () => {
    return HttpResponse.json({ success: true });
  }),
];
```

### File 5: `ui/src/mocks/handlers/dashboard.handlers.ts`

```typescript
import { http, HttpResponse } from 'msw';
import { dashboardFixture } from '../fixtures/dashboard.fixture';

export const dashboardHandlers = [
  http.get('/api/v1/dashboard', ({ request }) => {
    const url = new URL(request.url);
    const period = url.searchParams.get('period') ?? '30d';
    const data = dashboardFixture[period as keyof typeof dashboardFixture]
      ?? dashboardFixture['30d'];
    return HttpResponse.json(data);
  }),
];
```

### File 6: `ui/src/mocks/handlers/cve.handlers.ts`

```typescript
import { http, HttpResponse } from 'msw';
import { cvesFixture } from '../fixtures/cves.fixture';
import type { Severity } from '@/shared/types/cve';

export const cveHandlers = [
  // POST /api/v2/cves/search
  http.post('/api/v2/cves/search', async ({ request }) => {
    const body = await request.json() as {
      query?: string;
      severity?: Severity[];
      kevOnly?: boolean;
      minCvss?: number;
      maxCvss?: number;
      page?: number;
      pageSize?: number;
    };

    let results = [...cvesFixture];

    if (body.severity?.length) {
      results = results.filter((c) => body.severity!.includes(c.severity));
    }
    if (body.query) {
      const q = body.query.toLowerCase();
      results = results.filter(
        (c) =>
          c.id.toLowerCase().includes(q) ||
          c.description.toLowerCase().includes(q) ||
          c.vendor.toLowerCase().includes(q)
      );
    }
    if (body.kevOnly) {
      results = results.filter((c) => c.isKEV);
    }
    if (body.minCvss !== undefined) {
      results = results.filter((c) => (c.cvssV3 ?? 0) >= body.minCvss!);
    }
    if (body.maxCvss !== undefined) {
      results = results.filter((c) => (c.cvssV3 ?? 10) <= body.maxCvss!);
    }

    const page = body.page ?? 1;
    const pageSize = body.pageSize ?? 50;
    const start = (page - 1) * pageSize;
    const paginated = results.slice(start, start + pageSize);

    const bySeverity = results.reduce((acc, c) => {
      acc[c.severity] = (acc[c.severity] ?? 0) + 1;
      return acc;
    }, {} as Record<string, number>);

    return HttpResponse.json({
      data: paginated,
      total: results.length,
      page,
      pageSize,
      aggregations: { bySeverity, topVendors: [], byYear: [] },
    });
  }),

  // GET /api/v2/cves/:id
  http.get('/api/v2/cves/:id', ({ params }) => {
    const cve = cvesFixture.find((c) => c.id === params.id);
    if (!cve) return new HttpResponse(null, { status: 404 });
    return HttpResponse.json(cve);
  }),

  // GET /api/v2/kev
  http.get('/api/v2/kev', () => {
    const kevEntries = cvesFixture
      .filter((c) => c.isKEV)
      .map((c) => ({
        cveId: c.id,
        vendor: c.vendor,
        product: c.product,
        vulnerabilityName: c.description.substring(0, 60),
        dateAdded: c.publishedAt,
        shortDescription: c.description.substring(0, 120),
        requiredAction: 'Apply vendor patch immediately',
        knownRansomwareCampaignUse: c.epssScore > 0.9,
      }));
    return HttpResponse.json({
      entries: kevEntries,
      total: kevEntries.length,
      stats: {
        total: kevEntries.length,
        ransomwareLinked: kevEntries.filter((k) => k.knownRansomwareCampaignUse).length,
        addedThisWeek: 3,
        unmitigatedInPlatform: 7,
      },
    });
  }),

  // POST /api/v2/cves/search/semantic
  http.post('/api/v2/cves/search/semantic', async ({ request }) => {
    const body = await request.json() as { query: string; limit?: number };
    const limit = body.limit ?? 20;
    const results = cvesFixture.slice(0, limit).map((c) => ({
      ...c,
      similarityScore: Math.random() * 0.3 + 0.7,  // 0.7–1.0
    }));
    return HttpResponse.json({ results, queryEmbeddingMs: 42 });
  }),

  // GET /api/v2/browse (vendors)
  http.get('/api/v2/browse', () => {
    const vendors = [...new Set(cvesFixture.map((c) => c.vendor))]
      .map((vendor) => ({
        name: vendor,
        cveCount: cvesFixture.filter((c) => c.vendor === vendor).length,
      }))
      .sort((a, b) => b.cveCount - a.cveCount);
    return HttpResponse.json({ vendors, total: vendors.length });
  }),
];
```

### File 7: `ui/src/mocks/handlers/scan.handlers.ts`

```typescript
import { http, HttpResponse } from 'msw';
import { scansFixture } from '../fixtures/scans.fixture';
import type { ScanProgress } from '@/shared/types/scan';

export const scanHandlers = [
  http.get('/api/v1/scans', ({ request }) => {
    const url = new URL(request.url);
    const status = url.searchParams.get('status');
    const scans = status
      ? scansFixture.filter((s) => status.split(',').includes(s.status))
      : scansFixture;
    return HttpResponse.json({ scans, total: scans.length });
  }),

  http.post('/api/v1/scans', async ({ request }) => {
    const body = await request.json() as { name: string; type: string; targets: string[] };
    const newScan = {
      id: `SC-${Date.now()}`,
      ...body,
      status: 'queued' as const,
      progress: 0,
      findingCount: 0,
      createdBy: 'admin@osv.local',
    };
    return HttpResponse.json(newScan, { status: 201 });
  }),

  // SSE mock — scan progress simulation
  http.get('/api/v1/scans/:id/stream', ({ params }) => {
    const encoder = new TextEncoder();
    let progress = 0;

    const stream = new ReadableStream({
      async start(controller) {
        while (progress < 100) {
          await new Promise((r) => setTimeout(r, 600));
          progress = Math.min(progress + Math.floor(Math.random() * 15) + 5, 100);

          const data: ScanProgress = {
            scanId: params.id as string,
            status: progress < 100 ? 'running' : 'completed',
            progress,
            findingsFound: Math.floor(progress * 0.47),
            message: `Scanning targets... ${progress}%`,
          };

          controller.enqueue(
            encoder.encode(`data: ${JSON.stringify(data)}\n\n`)
          );
        }

        controller.enqueue(encoder.encode(`event: done\ndata: {}\n\n`));
        controller.close();
      },
    });

    return new HttpResponse(stream, {
      headers: {
        'Content-Type': 'text/event-stream',
        'Cache-Control': 'no-cache',
        'Connection': 'keep-alive',
      },
    });
  }),

  http.post('/api/v1/scans/:id/cancel', () => {
    return HttpResponse.json({ success: true });
  }),
];
```

### File 8: `ui/src/mocks/handlers/finding.handlers.ts`

```typescript
import { http, HttpResponse } from 'msw';
import { findingsFixture } from '../fixtures/findings.fixture';

export const findingHandlers = [
  http.get('/api/v1/findings', ({ request }) => {
    const url = new URL(request.url);
    const status = url.searchParams.getAll('status');
    const severity = url.searchParams.getAll('severity');

    let findings = [...findingsFixture];
    if (status.length) findings = findings.filter((f) => status.includes(f.status));
    if (severity.length) findings = findings.filter((f) => severity.includes(f.severity));

    const page = Number(url.searchParams.get('page') ?? '1');
    const pageSize = Number(url.searchParams.get('pageSize') ?? '50');
    const start = (page - 1) * pageSize;
    const paginated = findings.slice(start, start + pageSize);

    const bySeverity = findings.reduce((acc, f) => {
      acc[f.severity] = (acc[f.severity] ?? 0) + 1;
      return acc;
    }, {} as Record<string, number>);

    const byStatus = findings.reduce((acc, f) => {
      acc[f.status] = (acc[f.status] ?? 0) + 1;
      return acc;
    }, {} as Record<string, number>);

    return HttpResponse.json({
      findings: paginated,
      total: findings.length,
      bySeverity,
      byStatus,
      slaStats: { breached: 8, atRisk: 22, ok: findings.length - 30 },
    });
  }),

  http.get('/api/v1/findings/:id', ({ params }) => {
    const finding = findingsFixture.find((f) => f.id === params.id);
    if (!finding) return new HttpResponse(null, { status: 404 });
    return HttpResponse.json(finding);
  }),

  http.patch('/api/v1/findings/:id', async ({ params, request }) => {
    const body = await request.json() as Record<string, unknown>;
    const finding = findingsFixture.find((f) => f.id === params.id);
    if (!finding) return new HttpResponse(null, { status: 404 });
    return HttpResponse.json({ ...finding, ...body, updatedAt: new Date().toISOString() });
  }),

  http.post('/api/v1/findings/bulk/close', async ({ request }) => {
    const body = await request.json() as { findingIds: string[] };
    return HttpResponse.json({ updated: body.findingIds.length, success: true });
  }),
];
```

### File 9: `ui/src/mocks/handlers/asset.handlers.ts`

```typescript
import { http, HttpResponse } from 'msw';
import type { Asset } from '@/shared/types/scan';

const assetsFixture: Asset[] = [
  {
    id: 'asset-001',
    ip: '10.0.1.45',
    hostname: 'prod-web-01',
    os: 'Ubuntu 22.04',
    services: [
      { port: 80, protocol: 'tcp', service: 'http', version: 'nginx/1.24', cveIds: [] },
      { port: 443, protocol: 'tcp', service: 'https', version: 'nginx/1.24', cveIds: ['CVE-2025-44228'] },
      { port: 22, protocol: 'tcp', service: 'ssh', version: 'OpenSSH_8.9', cveIds: [] },
    ],
    webTechnologies: ['nginx', 'React', 'Node.js'],
    tags: ['production', 'web'],
    riskScore: 9.8,
    activeFindingCount: 12,
    firstSeenAt: '2025-01-15T00:00:00Z',
    lastSeenAt: '2026-06-14T08:30:00Z',
    lastScanId: 'SC-0047',
  },
  {
    id: 'asset-002',
    ip: '10.0.1.60',
    hostname: 'api-gw-prod-01',
    os: 'Amazon Linux 2',
    services: [
      { port: 8080, protocol: 'tcp', service: 'http-alt', version: 'Go/1.21', cveIds: [] },
    ],
    webTechnologies: ['Go', 'gRPC'],
    tags: ['production', 'api'],
    riskScore: 7.5,
    activeFindingCount: 5,
    firstSeenAt: '2025-02-01T00:00:00Z',
    lastSeenAt: '2026-06-14T09:00:00Z',
  },
];

export const assetHandlers = [
  http.get('/api/v1/assets', () => {
    return HttpResponse.json({ assets: assetsFixture, total: assetsFixture.length });
  }),

  http.get('/api/v1/assets/:id', ({ params }) => {
    const asset = assetsFixture.find((a) => a.id === params.id);
    if (!asset) return new HttpResponse(null, { status: 404 });
    return HttpResponse.json(asset);
  }),
];
```

### File 10: `ui/src/mocks/fixtures/dashboard.fixture.ts`

```typescript
// ⚠️ Chỉ dùng trong src/mocks/ — KHÔNG import vào component files
import type { Scan } from '@/shared/types/scan';

export interface DashboardFixtureData {
  kpis: {
    criticalFindings: number;
    highFindings: number;
    totalAssets: number;
    highRiskAssets: number;
    activeScans: number;
    queuedScans: number;
    securityGrade: string;
    securityScore: number;
    slaCompliance: number;
    slaAtRisk: number;
    slaBreached: number;
  };
  riskTrend: Array<{ month: string; critical: number; high: number; medium: number; low: number }>;
  severityDistribution: { critical: number; high: number; medium: number; low: number; total: number };
  productGrades: Array<{ id: string; name: string; grade: string; score: number; criticalCount: number; highCount: number }>;
  kevAlerts: Array<{ cveId: string; vendor: string; product: string; dateAdded: string; isRansomware: boolean }>;
  recentScans: Scan[];
  slaBreaches: Array<{ findingId: string; title: string; dueIn: string; severity: string; isOverdue: boolean }>;
}

const base: DashboardFixtureData = {
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
  severityDistribution: { critical: 245, high: 395, medium: 710, low: 1020, total: 2370 },
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
  recentScans: [
    { id: 'SC-0047', name: 'Production Network', type: 'nmap_full', status: 'completed', targets: ['10.0.0.0/24'], progress: 100, findingCount: 47, createdBy: 'admin@osv.local', completedAt: '2026-06-14T10:30:00Z' },
    { id: 'SC-0048', name: 'API Security Scan', type: 'zap', status: 'running', targets: ['https://api.internal'], progress: 45, findingCount: 12, createdBy: 'admin@osv.local', startedAt: '2026-06-14T11:00:00Z' },
    { id: 'SC-0046', name: 'Dev Environment', type: 'nmap_discovery', status: 'completed', targets: ['192.168.1.0/24'], progress: 100, findingCount: 8, createdBy: 'admin@osv.local', completedAt: '2026-06-14T08:00:00Z' },
  ],
  slaBreaches: [
    { findingId: 'F-2846', title: 'Spring Framework RCE', dueIn: 'Overdue -2d', severity: 'Critical', isOverdue: true },
    { findingId: 'F-2841', title: 'Kubernetes API Exposure', dueIn: 'Overdue -1d', severity: 'Critical', isOverdue: true },
    { findingId: 'F-2838', title: 'Redis Unauthorized Access', dueIn: '2d left', severity: 'High', isOverdue: false },
  ],
};

export const dashboardFixture: Record<string, DashboardFixtureData> = {
  '30d': base,
  '90d': {
    ...base,
    kpis: { ...base.kpis, criticalFindings: 890, highFindings: 1240 },
  },
  '1y': {
    ...base,
    kpis: { ...base.kpis, criticalFindings: 3200, highFindings: 4800 },
  },
};
```

### File 11: `ui/src/mocks/fixtures/cves.fixture.ts`

```typescript
// ⚠️ Chỉ dùng trong src/mocks/ — KHÔNG import vào component files
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
    description: 'Apache Log4j2 JNDI features used in configuration, log messages allow attackers to perform remote code execution.',
    publishedAt: '2025-12-09T00:00:00Z',
    updatedAt: '2026-06-10T00:00:00Z',
    sources: [{ name: 'NVD', url: 'https://nvd.nist.gov/vuln/detail/CVE-2025-44228', lastModified: '2026-06-10' }],
    hasExploit: true,
  },
  {
    id: 'CVE-2025-22965',
    severity: 'Critical',
    cvssV3: 9.8,
    epssScore: 0.874,
    epssPercentile: 0.994,
    isKEV: true,
    vendor: 'VMware',
    product: 'Spring Framework',
    cweIds: ['CWE-22'],
    capecIds: ['CAPEC-126'],
    description: 'Spring Framework path traversal vulnerability allows unauthenticated access to sensitive files.',
    publishedAt: '2025-03-31T00:00:00Z',
    updatedAt: '2026-05-15T00:00:00Z',
    sources: [{ name: 'NVD', url: 'https://nvd.nist.gov/vuln/detail/CVE-2025-22965', lastModified: '2026-05-15' }],
    hasExploit: true,
  },
  {
    id: 'CVE-2025-18935',
    severity: 'High',
    cvssV3: 8.2,
    epssScore: 0.721,
    epssPercentile: 0.981,
    isKEV: false,
    vendor: 'OpenSSL',
    product: 'OpenSSL',
    cweIds: ['CWE-120'],
    capecIds: [],
    description: 'OpenSSL buffer overflow in BN_mod_exp function allows heap-based memory corruption.',
    publishedAt: '2025-09-14T00:00:00Z',
    updatedAt: '2026-04-20T00:00:00Z',
    sources: [{ name: 'NVD', url: 'https://nvd.nist.gov/vuln/detail/CVE-2025-18935', lastModified: '2026-04-20' }],
    hasExploit: false,
  },
  {
    id: 'CVE-2025-33127',
    severity: 'High',
    cvssV3: 7.5,
    epssScore: 0.653,
    epssPercentile: 0.962,
    isKEV: false,
    vendor: 'nginx',
    product: 'nginx',
    cweIds: ['CWE-119'],
    capecIds: [],
    description: 'nginx HTTP/2 implementation memory corruption vulnerability when handling malformed HPACK data.',
    publishedAt: '2025-11-05T00:00:00Z',
    updatedAt: '2026-03-10T00:00:00Z',
    sources: [{ name: 'NVD', url: 'https://nvd.nist.gov/vuln/detail/CVE-2025-33127', lastModified: '2026-03-10' }],
    hasExploit: false,
  },
  {
    id: 'CVE-2025-12889',
    severity: 'Critical',
    cvssV3: 9.3,
    epssScore: 0.917,
    epssPercentile: 0.996,
    isKEV: true,
    vendor: 'Sudo',
    product: 'sudo',
    cweIds: ['CWE-269'],
    capecIds: ['CAPEC-1'],
    description: 'sudo privilege escalation vulnerability allows local users to gain root access via heap-based buffer overflow.',
    publishedAt: '2025-07-20T00:00:00Z',
    updatedAt: '2026-02-28T00:00:00Z',
    sources: [{ name: 'NVD', url: 'https://nvd.nist.gov/vuln/detail/CVE-2025-12889', lastModified: '2026-02-28' }],
    hasExploit: true,
  },
  {
    id: 'CVE-2025-09876',
    severity: 'Critical',
    cvssV3: 9.9,
    epssScore: 0.941,
    epssPercentile: 0.998,
    isKEV: true,
    vendor: 'Cisco',
    product: 'IOS XE',
    cweIds: ['CWE-78'],
    capecIds: ['CAPEC-88'],
    description: 'Cisco IOS XE web UI command injection vulnerability allows unauthenticated remote attackers to execute commands.',
    publishedAt: '2025-10-16T00:00:00Z',
    updatedAt: '2026-06-01T00:00:00Z',
    sources: [{ name: 'NVD', url: 'https://nvd.nist.gov/vuln/detail/CVE-2025-09876', lastModified: '2026-06-01' }],
    hasExploit: true,
  },
  {
    id: 'CVE-2025-28741',
    severity: 'Medium',
    cvssV3: 5.4,
    epssScore: 0.321,
    epssPercentile: 0.876,
    isKEV: false,
    vendor: 'WordPress',
    product: 'WordPress',
    cweIds: ['CWE-79'],
    capecIds: ['CAPEC-86'],
    description: 'WordPress stored cross-site scripting vulnerability in comment handling allows privilege escalation.',
    publishedAt: '2025-08-12T00:00:00Z',
    updatedAt: '2026-01-15T00:00:00Z',
    sources: [{ name: 'NVD', url: 'https://nvd.nist.gov/vuln/detail/CVE-2025-28741', lastModified: '2026-01-15' }],
    hasExploit: false,
  },
  {
    id: 'CVE-2025-41203',
    severity: 'High',
    cvssV3: 8.8,
    epssScore: 0.589,
    epssPercentile: 0.943,
    isKEV: false,
    vendor: 'Microsoft',
    product: 'Exchange Server',
    cweIds: ['CWE-918'],
    capecIds: ['CAPEC-664'],
    description: 'Microsoft Exchange Server SSRF vulnerability allows authenticated attackers to pivot to internal systems.',
    publishedAt: '2025-04-11T00:00:00Z',
    updatedAt: '2026-04-14T00:00:00Z',
    sources: [{ name: 'NVD', url: 'https://nvd.nist.gov/vuln/detail/CVE-2025-41203', lastModified: '2026-04-14' }],
    hasExploit: true,
  },
  {
    id: 'CVE-2025-05511',
    severity: 'Low',
    cvssV3: 3.3,
    epssScore: 0.052,
    epssPercentile: 0.421,
    isKEV: false,
    vendor: 'curl',
    product: 'curl',
    cweIds: ['CWE-200'],
    capecIds: [],
    description: 'curl information disclosure vulnerability when following HTTP redirects across different protocols.',
    publishedAt: '2025-05-21T00:00:00Z',
    updatedAt: '2025-12-01T00:00:00Z',
    sources: [{ name: 'NVD', url: 'https://nvd.nist.gov/vuln/detail/CVE-2025-05511', lastModified: '2025-12-01' }],
    hasExploit: false,
  },
  {
    id: 'CVE-2025-37842',
    severity: 'Medium',
    cvssV3: 6.1,
    epssScore: 0.187,
    epssPercentile: 0.735,
    isKEV: false,
    vendor: 'Apache',
    product: 'Tomcat',
    cweIds: ['CWE-601'],
    capecIds: ['CAPEC-194'],
    description: 'Apache Tomcat open redirect vulnerability in the authentication mechanism allows phishing attacks.',
    publishedAt: '2025-06-08T00:00:00Z',
    updatedAt: '2026-02-10T00:00:00Z',
    sources: [{ name: 'NVD', url: 'https://nvd.nist.gov/vuln/detail/CVE-2025-37842', lastModified: '2026-02-10' }],
    hasExploit: false,
  },
];
```

### File 12: `ui/src/mocks/fixtures/scans.fixture.ts`

```typescript
// ⚠️ Chỉ dùng trong src/mocks/
import type { Scan } from '@/shared/types/scan';

export const scansFixture: Scan[] = [
  { id: 'SC-0047', name: 'Production Network Q2', type: 'nmap_full', status: 'completed', targets: ['10.0.0.0/24'], progress: 100, findingCount: 47, createdBy: 'admin@osv.local', startedAt: '2026-06-14T08:00:00Z', completedAt: '2026-06-14T10:30:00Z', durationMs: 9000000 },
  { id: 'SC-0048', name: 'API Security Scan', type: 'zap', status: 'running', targets: ['https://api.internal'], progress: 45, findingCount: 12, createdBy: 'admin@osv.local', startedAt: '2026-06-14T11:00:00Z' },
  { id: 'SC-0046', name: 'Dev Environment', type: 'nmap_discovery', status: 'completed', targets: ['192.168.1.0/24'], progress: 100, findingCount: 8, createdBy: 'dev@osv.local', startedAt: '2026-06-14T06:00:00Z', completedAt: '2026-06-14T08:00:00Z', durationMs: 7200000 },
  { id: 'SC-0045', name: 'Staging Network', type: 'nmap_full', status: 'failed', targets: ['10.1.0.0/24'], progress: 67, findingCount: 0, createdBy: 'admin@osv.local', startedAt: '2026-06-13T14:00:00Z', error: 'Connection timeout to 10.1.0.1' },
  { id: 'SC-0044', name: 'Weekly Discovery', type: 'nmap_discovery', status: 'completed', targets: ['10.0.0.0/16'], progress: 100, findingCount: 23, createdBy: 'admin@osv.local', startedAt: '2026-06-08T02:00:00Z', completedAt: '2026-06-08T04:30:00Z', durationMs: 9000000 },
];
```

### File 13: `ui/src/mocks/fixtures/findings.fixture.ts`

```typescript
// ⚠️ Chỉ dùng trong src/mocks/
import type { Finding } from '@/shared/types/finding';

export const findingsFixture: Finding[] = [
  { id: 'F-2847', title: 'Apache Log4j2 RCE Vulnerability', description: 'Apache Log4j2 JNDI injection allows RCE.', cveId: 'CVE-2025-44228', severity: 'Critical', epssScore: 0.982, isKEV: true, status: 'active', isDuplicate: false, productId: 'p1', productName: 'Banking App', engagementId: 'e1', testId: 't1', assetIp: '10.0.1.45', slaStatus: 'at_risk', slaDaysLeft: 2, slaExpirationDate: '2026-06-18T00:00:00Z', createdAt: '2026-06-11T00:00:00Z', updatedAt: '2026-06-14T00:00:00Z', createdBy: 'admin@osv.local' },
  { id: 'F-2846', title: 'Spring Framework Path Traversal', description: 'Spring Framework unauthenticated path traversal.', cveId: 'CVE-2025-22965', severity: 'Critical', epssScore: 0.874, isKEV: true, status: 'active', isDuplicate: false, productId: 'p3', productName: 'API Gateway', engagementId: 'e3', testId: 't3', assetIp: '10.0.1.60', slaStatus: 'breached', slaDaysLeft: -2, slaExpirationDate: '2026-06-12T00:00:00Z', createdAt: '2026-06-05T00:00:00Z', updatedAt: '2026-06-14T00:00:00Z', createdBy: 'admin@osv.local' },
  { id: 'F-2845', title: 'OpenSSL Buffer Overflow', description: 'OpenSSL heap buffer overflow in BN_mod_exp.', cveId: 'CVE-2025-18935', severity: 'High', epssScore: 0.721, isKEV: false, status: 'active', isDuplicate: false, productId: 'p5', productName: 'Data Pipeline', engagementId: 'e5', testId: 't5', assetIp: '10.0.2.12', slaStatus: 'ok', slaDaysLeft: 5, slaExpirationDate: '2026-06-19T00:00:00Z', createdAt: '2026-06-09T00:00:00Z', updatedAt: '2026-06-14T00:00:00Z', createdBy: 'admin@osv.local' },
  { id: 'F-2844', title: 'nginx HTTP/2 Memory Corruption', description: 'nginx HTTP/2 HPACK memory corruption.', cveId: 'CVE-2025-33127', severity: 'High', epssScore: 0.653, isKEV: false, status: 'active', isDuplicate: false, productId: 'p4', productName: 'Admin Portal', engagementId: 'e4', testId: 't4', assetHostname: 'admin.internal', slaStatus: 'at_risk', slaDaysLeft: 3, slaExpirationDate: '2026-06-17T00:00:00Z', createdAt: '2026-06-07T00:00:00Z', updatedAt: '2026-06-14T00:00:00Z', createdBy: 'admin@osv.local' },
  { id: 'F-2843', title: 'Sudo Privilege Escalation', description: 'Local privilege escalation via sudo heap overflow.', cveId: 'CVE-2025-12889', severity: 'Critical', epssScore: 0.917, isKEV: true, status: 'mitigated', isDuplicate: false, productId: 'p2', productName: 'Mobile App', engagementId: 'e2', testId: 't2', assetIp: '10.0.3.88', slaStatus: 'ok', createdAt: '2026-05-20T00:00:00Z', updatedAt: '2026-06-10T00:00:00Z', mitigatedAt: '2026-06-10T00:00:00Z', createdBy: 'admin@osv.local' },
];
```

---

## Verification

```bash
cd ui/

# Verify MSW service worker generated
ls public/mockServiceWorker.js

# Start dev server với MSW enabled
VITE_ENABLE_MSW=true pnpm dev

# Mở browser → F12 → Console
# Expected: [MSW] Mocking enabled.

# Test API call
# Truy cập http://localhost:3000/dashboard
# F12 → Network → Filter "api/v1/dashboard"
# Expected: 200 response với dashboard fixture data
```

---

- [x] `npx msw init public/ --save` → tạo `public/mockServiceWorker.js`
- [x] `src/mocks/browser.ts` — `setupWorker`
- [x] `src/mocks/server.ts` — `setupServer` (Node.js cho tests)
- [x] `src/mocks/handlers/index.ts` — aggregate all handlers
- [x] `src/mocks/handlers/auth.handlers.ts` — login, refresh, me, logout
- [x] `src/mocks/handlers/dashboard.handlers.ts` — GET /api/v1/dashboard
- [x] `src/mocks/handlers/cve.handlers.ts` — search, detail, kev, semantic, browse
- [x] `src/mocks/handlers/finding.handlers.ts` — list, detail, update, bulkClose
- [x] `src/mocks/handlers/scan.handlers.ts` — list, create, SSE stream, cancel
- [x] `src/mocks/handlers/asset.handlers.ts` — list, detail
- [x] `src/mocks/fixtures/dashboard.fixture.ts` — 3 periods (30d/90d/1y)
- [x] `src/mocks/fixtures/cves.fixture.ts` — 10+ realistic CVEs
- [x] `src/mocks/fixtures/scans.fixture.ts` — 5 scans (various statuses)
- [x] `src/mocks/fixtures/findings.fixture.ts` — 5 findings
- [x] Browser console shows `[MSW] Mocking enabled.`
- [x] `/api/v1/dashboard` returns 200 với fixture data
