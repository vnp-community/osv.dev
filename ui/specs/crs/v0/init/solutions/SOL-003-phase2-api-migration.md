# SOL-003 — Phase 2: Remove Hardcode & Connect API (Priority Modules)

**Version:** 1.1  
**Ngày tạo:** 2026-06-16  
**Cập nhật:** 2026-06-17  
**Trạng thái:** 🔄 In Progress — API layers + hooks hoàn tất; component migration (remove hardcode) defer sang Phase 3  
**Phase:** Phase 2 (Sprint 3-5) — API Done / Component Migration Pending  
**Liên quan:** [SOL-002](./SOL-002-phase1-foundation.md)

---

## 1. Mục tiêu Phase 2

Xóa toàn bộ hardcoded data khỏi components và thay bằng MSW handlers + React Query hooks, theo thứ tự ưu tiên từ high-traffic screens trước.

---

## 2. Thứ tự Migration (Priority Order)

| Priority | Module | Component | Hardcode to Remove | MSW Handler |
|----------|--------|-----------|-------------------|-------------|
| **P0** | Dashboard | `Dashboard.tsx` | `riskTrendData`, `severityData`, `productData`, `recentFindings`, `kevAlerts`, `recentScans`, `slaBreaches` | `dashboard.handlers.ts` |
| **P0** | CVE Intel | `CVESearch.tsx` | `cveData = [...]` | `cve.handlers.ts` |
| **P1** | CVE Intel | `KEVCatalog.tsx` | KEV entries array | `kev.handlers.ts` |
| **P1** | Findings | `FindingsList.tsx` | findings array | `finding.handlers.ts` |
| **P2** | Scanning | `ScanDashboard.tsx` | scan list array + SSE | `scan.handlers.ts` |
| **P2** | Assets | `AssetInventory.tsx` | assets array | `asset.handlers.ts` |
| **P3** | Others | All remaining | various | respective handlers |

---

## 3. Module: Dashboard Migration

### 3.1 Feature-specific types

```typescript
// src/features/dashboard/types.ts
import type { Scan } from '@/shared/types/scan';

export interface DashboardKPIs {
  criticalFindings: number;
  highFindings: number;
  totalAssets: number;
  highRiskAssets: number;
  activeScans: number;
  queuedScans: number;
  securityGrade: 'A' | 'A-' | 'B+' | 'B' | 'B-' | 'C+' | 'C' | 'D' | 'F';
  securityScore: number;
  slaCompliance: number;
  slaAtRisk: number;
  slaBreached: number;
}

export interface RiskTrendPoint {
  month: string;
  critical: number;
  high: number;
  medium: number;
  low: number;
}

export interface SeverityDistribution {
  critical: number;
  high: number;
  medium: number;
  low: number;
  total: number;
}

export interface ProductGrade {
  id: string;
  name: string;
  grade: string;
  score: number;
  criticalCount: number;
  highCount: number;
}

export interface KEVAlert {
  cveId: string;
  vendor: string;
  product: string;
  dateAdded: string;
  isRansomware: boolean;
}

export interface SLABreach {
  findingId: string;
  title: string;
  dueIn: string;
  severity: string;
  isOverdue: boolean;
}

export interface DashboardData {
  kpis: DashboardKPIs;
  riskTrend: RiskTrendPoint[];
  severityDistribution: SeverityDistribution;
  productGrades: ProductGrade[];
  kevAlerts: KEVAlert[];
  recentScans: Scan[];
  slaBreaches: SLABreach[];
}
```

### 3.2 Dashboard API layer

```typescript
// src/features/dashboard/api/dashboardApi.ts
import { apiClient } from '@/shared/api/client';
import { ENDPOINTS } from '@/shared/api/endpoints';
import type { DashboardData } from '../types';

export const dashboardApi = {
  getMetrics: (period: '30d' | '90d' | '1y') =>
    apiClient
      .get<DashboardData>(ENDPOINTS.dashboard, { params: { period } })
      .then((r) => r.data),
};
```

### 3.3 Dashboard hook

```typescript
// src/features/dashboard/hooks/useDashboardMetrics.ts
import { useQuery } from '@tanstack/react-query';
import { dashboardKeys } from '@/shared/api/queryClient';
import { dashboardApi } from '../api/dashboardApi';

export function useDashboardMetrics(period: '30d' | '90d' | '1y' = '30d') {
  return useQuery({
    queryKey: dashboardKeys.metrics(period),
    queryFn: () => dashboardApi.getMetrics(period),
    staleTime: 60_000,
    refetchInterval: 60_000,
  });
}
```

---

## 4. Module: CVE Intelligence Migration

### 4.1 CVE API layer

```typescript
// src/features/cve-intel/api/cveApi.ts
import { apiClient } from '@/shared/api/client';
import { ENDPOINTS } from '@/shared/api/endpoints';
import type { CVE } from '@/shared/types/cve';

export interface CVESearchParams {
  query?: string;
  severity?: string[];
  vendors?: string[];
  cweIds?: string[];
  minCvss?: number;
  maxCvss?: number;
  minEpss?: number;
  maxEpss?: number;
  kevOnly?: boolean;
  hasExploit?: boolean;
  page?: number;
  pageSize?: number;
  sortBy?: 'cvss_desc' | 'epss_desc' | 'date_desc' | 'severity_desc';
}

export interface CVESearchResponse {
  data: CVE[];
  total: number;
  page: number;
  pageSize: number;
  aggregations: {
    bySeverity: Record<string, number>;
    topVendors: Array<{ vendor: string; count: number }>;
    byYear: Array<{ year: number; count: number }>;
  };
}

export const cveApi = {
  search: (params: CVESearchParams) =>
    apiClient
      .post<CVESearchResponse>(ENDPOINTS.cve.search, params)
      .then((r) => r.data),

  semantic: (query: string, limit = 20) =>
    apiClient
      .post<{ results: Array<CVE & { similarityScore: number }>; queryEmbeddingMs: number }>(
        ENDPOINTS.cve.semantic,
        { query, limit }
      )
      .then((r) => r.data),

  getById: (id: string) =>
    apiClient.get<CVE>(ENDPOINTS.cve.detail(id)).then((r) => r.data),

  export: (params: CVESearchParams) =>
    apiClient.get(ENDPOINTS.cve.export, { params, responseType: 'blob' }),
};
```

### 4.2 CVE hooks

```typescript
// src/features/cve-intel/hooks/useCVESearch.ts
import { useQuery } from '@tanstack/react-query';
import { cveKeys } from '@/shared/api/queryClient';
import { cveApi, type CVESearchParams } from '../api/cveApi';

export function useCVESearch(params: CVESearchParams) {
  return useQuery({
    queryKey: cveKeys.search(params),
    queryFn: () => cveApi.search(params),
    staleTime: 5 * 60_000,   // 5 min — CVE data không thay đổi thường xuyên
    placeholderData: (prev) => prev,  // Keep previous data while fetching
  });
}

// src/features/cve-intel/hooks/useCVEDetail.ts
export function useCVEDetail(id: string) {
  return useQuery({
    queryKey: cveKeys.detail(id),
    queryFn: () => cveApi.getById(id),
    staleTime: 5 * 60_000,
    enabled: !!id,
  });
}
```

### 4.3 CVE MSW handlers

```typescript
// src/mocks/handlers/cve.handlers.ts
import { http, HttpResponse } from 'msw';
import type { CVESearchParams } from '@/features/cve-intel/api/cveApi';
import { cvesFixture } from '../fixtures/cves.fixture';

export const cveHandlers = [
  // POST /api/v2/cves/search
  http.post('/api/v2/cves/search', async ({ request }) => {
    const body = (await request.json()) as CVESearchParams;

    let results = [...cvesFixture];

    // Apply filters (mimic backend logic)
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

    // Server-side pagination
    const page = body.page ?? 1;
    const pageSize = body.pageSize ?? 50;
    const start = (page - 1) * pageSize;
    const paginated = results.slice(start, start + pageSize);

    // Aggregations
    const bySeverity = results.reduce(
      (acc, c) => ({ ...acc, [c.severity]: (acc[c.severity] ?? 0) + 1 }),
      {} as Record<string, number>
    );

    return HttpResponse.json({
      data: paginated,
      total: results.length,
      page,
      pageSize,
      aggregations: {
        bySeverity,
        topVendors: [],
        byYear: [],
      },
    });
  }),

  // GET /api/v2/cves/:id
  http.get('/api/v2/cves/:id', ({ params }) => {
    const cve = cvesFixture.find((c) => c.id === params.id);
    if (!cve) return new HttpResponse(null, { status: 404 });
    return HttpResponse.json(cve);
  }),
];
```

### 4.4 CVE Search Component (After Migration)

```typescript
// src/features/cve-intel/components/CVESearch.tsx
import { useState } from 'react';
import { useSearchParams } from 'react-router';
import { useCVESearch } from '../hooks/useCVESearch';
import { CVETable } from './CVETable';
import { CVEFilterPanel } from './CVEFilterPanel';
import { CVEDetailDrawer } from './CVEDetailDrawer';
import { CVETableSkeleton } from './CVETableSkeleton';
import { QueryBoundary } from '@/shared/components/feedback/QueryBoundary';
import type { CVE } from '@/shared/types/cve';

export function CVESearch() {
  const [searchParams, setSearchParams] = useSearchParams();
  const [selectedCVE, setSelectedCVE] = useState<CVE | null>(null);

  // URL-based filter state (deep linking support)
  const filters = {
    query: searchParams.get('q') ?? undefined,
    severity: searchParams.getAll('severity'),
    kevOnly: searchParams.get('kev') === 'true',
    page: Number(searchParams.get('page') ?? '1'),
    pageSize: 50,
  };

  const cveQuery = useCVESearch(filters);

  const handleFilterChange = (newFilters: typeof filters) => {
    const params = new URLSearchParams();
    if (newFilters.query) params.set('q', newFilters.query);
    newFilters.severity.forEach((s) => params.append('severity', s));
    if (newFilters.kevOnly) params.set('kev', 'true');
    setSearchParams(params);
  };

  return (
    <div className="flex h-full overflow-hidden">
      {/* Left: Filter Panel */}
      <CVEFilterPanel filters={filters} onChange={handleFilterChange} />

      {/* Center: Table */}
      <div className="flex-1 overflow-hidden">
        <QueryBoundary
          query={cveQuery}
          skeleton={<CVETableSkeleton rows={10} />}
        >
          {(data) => (
            <CVETable
              cves={data.data}
              total={data.total}
              page={filters.page}
              pageSize={filters.pageSize}
              onPageChange={(page) =>
                setSearchParams((p) => { p.set('page', String(page)); return p; })
              }
              onRowClick={setSelectedCVE}
            />
          )}
        </QueryBoundary>
      </div>

      {/* Right: Detail Drawer */}
      {selectedCVE && (
        <CVEDetailDrawer
          cve={selectedCVE}
          onClose={() => setSelectedCVE(null)}
        />
      )}
    </div>
  );
}
```

---

## 5. Module: Findings Migration

### 5.1 Finding API layer

```typescript
// src/features/findings/api/findingApi.ts
import { apiClient } from '@/shared/api/client';
import { ENDPOINTS } from '@/shared/api/endpoints';
import type { Finding, FindingStatus } from '@/shared/types/finding';

export interface FindingsListParams {
  status?: FindingStatus[];
  severity?: string[];
  productId?: string;
  cveId?: string;
  slaStatus?: string;
  page?: number;
  pageSize?: number;
  sortBy?: string;
}

export interface FindingsListResponse {
  findings: Finding[];
  total: number;
  bySeverity: Record<string, number>;
  byStatus: Record<string, number>;
  slaStats: { breached: number; atRisk: number; ok: number };
}

export const findingApi = {
  list: (params: FindingsListParams) =>
    apiClient
      .get<FindingsListResponse>(ENDPOINTS.findings.list, { params })
      .then((r) => r.data),

  getById: (id: string) =>
    apiClient.get<Finding>(ENDPOINTS.findings.detail(id)).then((r) => r.data),

  update: (id: string, data: { status?: FindingStatus; comment?: string; assignedTo?: string }) =>
    apiClient
      .patch<Finding>(ENDPOINTS.findings.update(id), data)
      .then((r) => r.data),

  bulkClose: (findingIds: string[], comment?: string) =>
    apiClient
      .post(ENDPOINTS.findings.bulkClose, { findingIds, comment })
      .then((r) => r.data),

  getAudit: (id: string) =>
    apiClient
      .get(ENDPOINTS.findings.audit(id))
      .then((r) => r.data),
};
```

### 5.2 Finding hooks

```typescript
// src/features/findings/hooks/useFindings.ts
import { useQuery, useMutation } from '@tanstack/react-query';
import { findingKeys, queryClient } from '@/shared/api/queryClient';
import { findingApi, type FindingsListParams } from '../api/findingApi';
import type { FindingStatus } from '@/shared/types/finding';
import { toast } from 'sonner';

export function useFindings(params: FindingsListParams) {
  return useQuery({
    queryKey: findingKeys.list(params),
    queryFn: () => findingApi.list(params),
    staleTime: 30_000,
    placeholderData: (prev) => prev,
  });
}

export function useUpdateFinding() {
  return useMutation({
    mutationFn: ({
      id,
      status,
      comment,
    }: {
      id: string;
      status: FindingStatus;
      comment?: string;
    }) => findingApi.update(id, { status, comment }),

    onSuccess: (_, { id }) => {
      // Invalidate both the list and the specific finding
      queryClient.invalidateQueries({ queryKey: findingKeys.all });
      toast.success('Finding status updated');
    },
  });
}
```

---

## 6. Module: Scanning + SSE

### 6.1 Scan API

```typescript
// src/features/scanning/api/scanApi.ts
import { apiClient } from '@/shared/api/client';
import { ENDPOINTS } from '@/shared/api/endpoints';
import type { Scan, NmapHost, ZAPAlert } from '@/shared/types/scan';

export const scanApi = {
  list: (params?: { status?: string; page?: number }) =>
    apiClient
      .get<{ scans: Scan[]; total: number }>(ENDPOINTS.scans.list, { params })
      .then((r) => r.data),

  create: (data: {
    name: string;
    type: string;
    targets: string[];
    options?: object;
    engagementId?: string;
  }) =>
    apiClient.post<Scan>(ENDPOINTS.scans.create, data).then((r) => r.data),

  getById: (id: string) =>
    apiClient.get<Scan>(ENDPOINTS.scans.detail(id)).then((r) => r.data),

  cancel: (id: string) =>
    apiClient.post(ENDPOINTS.scans.cancel(id)).then((r) => r.data),

  getNmapResults: (id: string) =>
    apiClient
      .get<{ hosts: NmapHost[]; scanId: string }>(ENDPOINTS.scans.results.nmap(id))
      .then((r) => r.data),

  getZAPResults: (id: string) =>
    apiClient
      .get<{ alerts: ZAPAlert[]; scanId: string }>(ENDPOINTS.scans.results.zap(id))
      .then((r) => r.data),
};
```

### 6.2 SSE Hook

```typescript
// src/shared/hooks/useSSE.ts
import { useEffect, useRef, useState } from 'react';

export type SSEStatus = 'idle' | 'connecting' | 'streaming' | 'done' | 'error';

interface SSEOptions<T> {
  onMessage?: (data: T) => void;
  onDone?: () => void;
  onError?: () => void;
}

export function useSSE<T>(
  url: string,
  enabled: boolean,
  options: SSEOptions<T> = {}
): { status: SSEStatus } {
  const [status, setStatus] = useState<SSEStatus>('idle');
  const sourceRef = useRef<EventSource | null>(null);

  useEffect(() => {
    if (!enabled || !url) {
      setStatus('idle');
      return;
    }

    setStatus('connecting');
    const source = new EventSource(url, { withCredentials: true });
    sourceRef.current = source;

    source.onopen = () => setStatus('streaming');

    source.onmessage = (e) => {
      try {
        const data = JSON.parse(e.data) as T;
        options.onMessage?.(data);
      } catch (err) {
        console.error('[SSE] Parse error:', err);
      }
    };

    source.addEventListener('done', () => {
      setStatus('done');
      options.onDone?.();
      source.close();
    });

    source.onerror = () => {
      setStatus('error');
      options.onError?.();
      source.close();
    };

    return () => {
      source.close();
      sourceRef.current = null;
    };
  }, [url, enabled]);

  return { status };
}
```

### 6.3 Scan SSE Hook (feature-specific)

```typescript
// src/features/scanning/hooks/useScanSSE.ts
import { useState } from 'react';
import { useQueryClient } from '@tanstack/react-query';
import { useSSE } from '@/shared/hooks/useSSE';
import { scanKeys } from '@/shared/api/queryClient';
import { ENDPOINTS } from '@/shared/api/endpoints';
import type { ScanProgress } from '@/shared/types/scan';

export function useScanSSE(scanId: string, enabled: boolean) {
  const queryClient = useQueryClient();
  const [progress, setProgress] = useState<ScanProgress | null>(null);

  const { status } = useSSE<ScanProgress>(
    ENDPOINTS.scans.stream(scanId),
    enabled,
    {
      onMessage: (data) => {
        setProgress(data);
        // Optimistically update scan in cache
        queryClient.setQueryData(
          scanKeys.detail(scanId),
          (old: any) =>
            old
              ? { ...old, progress: data.progress, status: data.status }
              : old
        );
      },
      onDone: () => {
        // Fetch final scan state
        queryClient.invalidateQueries({ queryKey: scanKeys.detail(scanId) });
        queryClient.invalidateQueries({ queryKey: scanKeys.all });
      },
    }
  );

  return { progress, sseStatus: status };
}
```

### 6.4 MSW SSE Handler

```typescript
// src/mocks/handlers/scan.handlers.ts
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

  // SSE mock — simulate scan progress
  http.get('/api/v1/scans/:id/stream', ({ params }) => {
    const encoder = new TextEncoder();
    let progress = 0;

    const stream = new ReadableStream({
      async start(controller) {
        while (progress < 100) {
          await new Promise((r) => setTimeout(r, 500));
          progress = Math.min(progress + Math.floor(Math.random() * 15) + 5, 100);

          const data: ScanProgress = {
            scanId: params.id as string,
            status: progress < 100 ? 'running' : 'completed',
            progress,
            findingsFound: Math.floor(progress * 0.5),
            message: `Scanning... ${progress}%`,
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
        Connection: 'keep-alive',
      },
    });
  }),
];
```

---

## 7. Shared Utilities Migration

Các utility functions cần tách ra khỏi component files:

### 7.1 `src/shared/utils/severity.ts`

```typescript
// src/shared/utils/severity.ts
import type { Severity } from '@/shared/types/cve';

export const SEVERITY_COLORS: Record<Severity, string> = {
  Critical: '#EF4444',
  High: '#F97316',
  Medium: '#EAB308',
  Low: '#3B82F6',
  Info: '#6B7280',
};

export const SEVERITY_BG_COLORS: Record<Severity, string> = {
  Critical: 'rgba(239,68,68,0.15)',
  High: 'rgba(249,115,22,0.15)',
  Medium: 'rgba(234,179,8,0.15)',
  Low: 'rgba(59,130,246,0.15)',
  Info: 'rgba(107,114,128,0.15)',
};

export const SEVERITY_ORDER: Record<Severity, number> = {
  Critical: 0,
  High: 1,
  Medium: 2,
  Low: 3,
  Info: 4,
};

export function getSeverityColor(severity: Severity): string {
  return SEVERITY_COLORS[severity] ?? '#6B7280';
}
```

### 7.2 `src/shared/utils/sla.ts`

```typescript
// src/shared/utils/sla.ts
import type { SLAStatus } from '@/shared/types/finding';

export const SLA_CONFIG = {
  Critical: 7,    // days
  High: 30,
  Medium: 90,
  Low: 180,
} as const;

export function getSLAStatus(expirationDate: string): SLAStatus {
  const now = new Date();
  const exp = new Date(expirationDate);
  const daysLeft = Math.floor((exp.getTime() - now.getTime()) / (1000 * 60 * 60 * 24));

  if (daysLeft < 0) return 'breached';
  if (daysLeft <= 3) return 'at_risk';
  return 'ok';
}

export function getSLADaysLeft(expirationDate: string): number {
  const now = new Date();
  const exp = new Date(expirationDate);
  return Math.floor((exp.getTime() - now.getTime()) / (1000 * 60 * 60 * 24));
}
```

### 7.3 `src/shared/utils/productGrade.ts`

```typescript
// src/shared/utils/productGrade.ts
// Extracted from TDD.md Section 8.2

type ProductGrade = 'A' | 'B' | 'C' | 'D' | 'F';

export function calculateProductGrade(findings: {
  critical: number;
  high: number;
  total: number;
}): ProductGrade {
  const { critical, high, total } = findings;
  if (critical >= 3 || total > 20) return 'F';
  if (critical >= 1 && critical <= 2) return 'D';
  if (critical === 0 && high > 5) return 'C';
  if (critical === 0 && high <= 5) return 'B';
  if (critical === 0 && high === 0) return 'A';
  return 'F';
}
```

### 7.4 `src/shared/utils/findingStateMachine.ts`

```typescript
// src/shared/utils/findingStateMachine.ts
import type { FindingStatus } from '@/shared/types/finding';

export const VALID_TRANSITIONS: Record<FindingStatus, FindingStatus[]> = {
  active: ['mitigated', 'false_positive', 'risk_accepted', 'out_of_scope'],
  mitigated: ['active'],
  false_positive: ['active'],
  risk_accepted: ['active'],
  out_of_scope: ['active'],
  duplicate: [],
};

export function canTransition(from: FindingStatus, to: FindingStatus): boolean {
  return VALID_TRANSITIONS[from].includes(to);
}

export const STATUS_PRIORITY: Record<FindingStatus, number> = {
  duplicate: 6,
  false_positive: 5,
  out_of_scope: 4,
  risk_accepted: 3,
  mitigated: 2,
  active: 1,
};
```

---

## 8. Shared Components Extraction

### 8.1 `src/shared/components/data-display/SeverityBadge.tsx`

```typescript
// src/shared/components/data-display/SeverityBadge.tsx
import type { Severity } from '@/shared/types/cve';
import { SEVERITY_COLORS, SEVERITY_BG_COLORS } from '@/shared/utils/severity';

export function SeverityBadge({ severity }: { severity: Severity }) {
  return (
    <span
      style={{
        background: SEVERITY_BG_COLORS[severity],
        color: SEVERITY_COLORS[severity],
        padding: '2px 8px',
        borderRadius: 6,
        fontSize: 11,
        fontWeight: 600,
      }}
    >
      {severity}
    </span>
  );
}
```

### 8.2 `src/shared/components/data-display/DataTable.tsx`

```typescript
// src/shared/components/data-display/DataTable.tsx
interface Column<T> {
  key: keyof T | string;
  header: string;
  width?: string;
  sortable?: boolean;
  render?: (value: unknown, row: T) => React.ReactNode;
}

interface DataTableProps<T> {
  columns: Column<T>[];
  data: T[];
  total: number;
  page: number;
  pageSize: number;
  onPageChange: (page: number) => void;
  onSort?: (key: string, direction: 'asc' | 'desc') => void;
  onRowClick?: (row: T) => void;
  selectedRows?: Set<string>;
  onSelectRow?: (id: string) => void;
  getRowId: (row: T) => string;
  isLoading?: boolean;
  emptyState?: React.ReactNode;
}

export function DataTable<T>({
  columns,
  data,
  total,
  page,
  pageSize,
  onPageChange,
  onRowClick,
  getRowId,
  emptyState,
}: DataTableProps<T>) {
  const totalPages = Math.ceil(total / pageSize);

  if (data.length === 0 && emptyState) return <>{emptyState}</>;

  return (
    <div className="flex flex-col h-full">
      <div className="flex-1 overflow-auto">
        <table className="w-full">
          <thead>
            <tr style={{ borderBottom: '1px solid rgba(255,255,255,0.06)' }}>
              {columns.map((col) => (
                <th
                  key={String(col.key)}
                  className="px-4 py-3 text-left"
                  style={{
                    color: '#6B7280',
                    fontSize: 11,
                    fontWeight: 600,
                    letterSpacing: 0.5,
                    width: col.width,
                  }}
                >
                  {col.header.toUpperCase()}
                </th>
              ))}
            </tr>
          </thead>
          <tbody>
            {data.map((row) => (
              <tr
                key={getRowId(row)}
                onClick={() => onRowClick?.(row)}
                style={{
                  cursor: onRowClick ? 'pointer' : 'default',
                  borderBottom: '1px solid rgba(255,255,255,0.04)',
                }}
                onMouseEnter={(e) => {
                  if (onRowClick)
                    e.currentTarget.style.background = 'rgba(255,255,255,0.02)';
                }}
                onMouseLeave={(e) => {
                  e.currentTarget.style.background = 'transparent';
                }}
              >
                {columns.map((col) => (
                  <td key={String(col.key)} className="px-4 py-3">
                    {col.render
                      ? col.render((row as any)[col.key], row)
                      : String((row as any)[col.key] ?? '')}
                  </td>
                ))}
              </tr>
            ))}
          </tbody>
        </table>
      </div>

      {/* Pagination */}
      <div
        className="flex items-center justify-between px-4 py-3"
        style={{ borderTop: '1px solid rgba(255,255,255,0.06)' }}
      >
        <span style={{ color: '#6B7280', fontSize: 12 }}>
          {(page - 1) * pageSize + 1}–
          {Math.min(page * pageSize, total)} of {total.toLocaleString()}
        </span>
        <div className="flex items-center gap-2">
          <button
            onClick={() => onPageChange(page - 1)}
            disabled={page <= 1}
            style={{
              padding: '4px 12px',
              borderRadius: 8,
              background: 'rgba(255,255,255,0.05)',
              border: '1px solid rgba(255,255,255,0.08)',
              color: page <= 1 ? '#4B5563' : '#9CA3AF',
              fontSize: 12,
              cursor: page <= 1 ? 'not-allowed' : 'pointer',
            }}
          >
            ← Prev
          </button>
          <span style={{ color: '#6B7280', fontSize: 12 }}>
            Page {page} / {totalPages}
          </span>
          <button
            onClick={() => onPageChange(page + 1)}
            disabled={page >= totalPages}
            style={{
              padding: '4px 12px',
              borderRadius: 8,
              background: 'rgba(255,255,255,0.05)',
              border: '1px solid rgba(255,255,255,0.08)',
              color: page >= totalPages ? '#4B5563' : '#9CA3AF',
              fontSize: 12,
              cursor: page >= totalPages ? 'not-allowed' : 'pointer',
            }}
          >
            Next →
          </button>
        </div>
      </div>
    </div>
  );
}
```

---

## 9. Checklist Phase 2

### Dashboard
- [ ] Tạo `src/features/dashboard/types.ts`
- [ ] Tạo `src/features/dashboard/api/dashboardApi.ts`
- [ ] Tạo `src/features/dashboard/hooks/useDashboardMetrics.ts`
- [ ] Tạo `src/mocks/handlers/dashboard.handlers.ts`
- [ ] Tạo `src/mocks/fixtures/dashboard.fixture.ts`
- [ ] Migrate `Dashboard.tsx` → xóa 7 hardcoded arrays
- [ ] Verify Dashboard render với MSW

### CVE Intelligence
- [ ] Tạo `src/features/cve-intel/api/cveApi.ts`
- [ ] Tạo `src/features/cve-intel/hooks/useCVESearch.ts`
- [ ] Tạo `src/mocks/handlers/cve.handlers.ts`
- [ ] Tạo `src/mocks/fixtures/cves.fixture.ts` (50+ realistic CVEs)
- [ ] Migrate `CVESearch.tsx` → URL-based filters + React Query
- [ ] Migrate `KEVCatalog.tsx`
- [ ] Migrate `SemanticSearch.tsx`

### Findings
- [ ] Tạo `src/features/findings/api/findingApi.ts`
- [ ] Tạo `src/features/findings/hooks/useFindings.ts`
- [ ] Tạo `src/mocks/handlers/finding.handlers.ts`
- [ ] Migrate `FindingsList.tsx` → xóa hardcoded findings
- [ ] Migrate `FindingDetail.tsx` → real data + mutations

### Scanning
- [ ] Tạo `src/features/scanning/api/scanApi.ts`
- [ ] Tạo `src/features/scanning/hooks/useScanSSE.ts`
- [ ] Tạo `src/mocks/handlers/scan.handlers.ts` (bao gồm SSE)
- [ ] Migrate `ScanDashboard.tsx`
- [ ] Migrate `RunningScan.tsx` → SSE integration

### Shared
- [ ] Tạo `src/shared/utils/severity.ts`
- [ ] Tạo `src/shared/utils/sla.ts`
- [ ] Tạo `src/shared/utils/productGrade.ts`
- [ ] Tạo `src/shared/utils/findingStateMachine.ts`
- [ ] Tạo `src/shared/components/data-display/SeverityBadge.tsx`
- [ ] Tạo `src/shared/components/data-display/DataTable.tsx`
- [ ] Tạo `src/shared/components/data-display/KEVIndicator.tsx`
- [ ] Tạo `src/shared/components/data-display/EPSSBar.tsx`
