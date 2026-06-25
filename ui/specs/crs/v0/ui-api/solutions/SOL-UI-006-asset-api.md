# SOL-UI-006 — Frontend Solution: Asset Management API

**CR nguồn:** [CR-UI-006](../../../../../specs/crs/v0/ui-api/CR-UI-006-asset-api.md)  
**Ngày tạo:** 2026-06-16  
**Trạng thái:** Proposed  
**Ưu tiên:** P1 — High (v3.0, phụ thuộc CR-OVS-007)  
**Phạm vi:** Frontend React SPA (`ui/src/features/assets/`)

---

## 1. Tóm tắt giải pháp

CR-UI-006 bao phủ Asset Management với 2 screens. Frontend cần:

1. `assetApi.ts` — List, detail, findings per asset, patch, tags
2. `useAssets.ts` — React Query với filter/search URL state
3. `useAssetDetail.ts` — Detail với 5 tabs
4. Risk score color coding utility
5. MSW handlers

> **Phụ thuộc:** Asset-service mới (CR-OVS-007). MSW handler dùng trong khi chờ backend.

---

## 2. File Structure

```
ui/src/
├── features/assets/
│   ├── api/
│   │   └── assetApi.ts
│   ├── hooks/
│   │   ├── useAssets.ts
│   │   └── useAssetDetail.ts
│   ├── components/
│   │   ├── AssetInventory.tsx     # /assets — table với filters
│   │   ├── AssetDetail.tsx        # /assets/:id — 5 tabs
│   │   ├── AssetRiskBadge.tsx
│   │   └── AssetTagEditor.tsx
│   ├── utils/
│   │   └── assetRisk.ts           # Risk score → color/label
│   └── types.ts
│
└── mocks/handlers/
    └── asset.handlers.ts
```

---

## 3. Implementation Chi Tiết

### 3.1 `features/assets/api/assetApi.ts`

```typescript
import apiClient from '@/shared/api/client';
import type { AssetListResponse, Asset } from '../types';
import type { FindingListResponse } from '@/features/findings/types';

export const assetApi = {
  // GET /api/v1/assets
  list: async (params: {
    tags?: string[];
    os?: string;
    min_risk_score?: number;
    max_risk_score?: number;
    last_seen_after?: string;
    q?: string;
    page?: number;
    page_size?: number;
    sort_by?: string;
  } = {}): Promise<AssetListResponse> => {
    const { data } = await apiClient.get<AssetListResponse>('/api/v1/assets', {
      params: {
        ...params,
        tags: params.tags?.join(','),
      },
    });
    return data;
  },

  // GET /api/v1/assets/{id}
  getById: async (id: string): Promise<Asset> => {
    const { data } = await apiClient.get<Asset>(`/api/v1/assets/${id}`);
    return data;
  },

  // GET /api/v1/assets/{id}/findings
  getFindings: async (id: string, params: {
    status?: string;
    page?: number;
    page_size?: number;
  } = {}): Promise<FindingListResponse> => {
    const { data } = await apiClient.get<FindingListResponse>(
      `/api/v1/assets/${id}/findings`,
      { params }
    );
    return data;
  },

  // PATCH /api/v1/assets/{id}
  patch: async (id: string, payload: {
    hostname?: string;
    tags?: string[];
  }): Promise<Asset> => {
    const { data } = await apiClient.patch<Asset>(`/api/v1/assets/${id}`, payload);
    return data;
  },

  // GET /api/v1/assets/tags
  getTags: async (): Promise<{ tags: string[] }> => {
    const { data } = await apiClient.get<{ tags: string[] }>('/api/v1/assets/tags');
    return data;
  },
};
```

### 3.2 `features/assets/utils/assetRisk.ts`

```typescript
// Risk Score → Color Coding (CR-UI-006 §3)
export const RISK_SCORE_COLORS = {
  critical: { color: '#EF4444', label: 'Critical', minScore: 8.0 },
  high:     { color: '#F97316', label: 'High',     minScore: 5.0 },
  medium:   { color: '#EAB308', label: 'Medium',   minScore: 3.0 },
  low:      { color: '#10B981', label: 'Low',      minScore: 0.0 },
} as const;

export function getRiskLevel(score: number): keyof typeof RISK_SCORE_COLORS {
  if (score >= 8.0) return 'critical';
  if (score >= 5.0) return 'high';
  if (score >= 3.0) return 'medium';
  return 'low';
}

export function getRiskColor(score: number): string {
  return RISK_SCORE_COLORS[getRiskLevel(score)].color;
}

export function getRiskLabel(score: number): string {
  return RISK_SCORE_COLORS[getRiskLevel(score)].label;
}
```

### 3.3 `features/assets/hooks/useAssets.ts`

```typescript
import { useSearchParams } from 'react-router-dom';
import { useQuery } from '@tanstack/react-query';
import { assetApi } from '../api/assetApi';

export const assetKeys = {
  all: ['assets'] as const,
  list: (params: object) => [...assetKeys.all, 'list', params] as const,
  detail: (id: string) => [...assetKeys.all, 'detail', id] as const,
  findings: (id: string, params: object) => [...assetKeys.all, 'findings', id, params] as const,
  tags: () => [...assetKeys.all, 'tags'] as const,
};

export function useAssets() {
  const [searchParams, setSearchParams] = useSearchParams();

  const params = {
    tags: searchParams.getAll('tag'),
    os: searchParams.get('os') || undefined,
    min_risk_score: searchParams.get('min_risk') ? Number(searchParams.get('min_risk')) : undefined,
    q: searchParams.get('q') || undefined,
    page: Number(searchParams.get('page') || '1'),
    page_size: Number(searchParams.get('page_size') || '50'),
    sort_by: searchParams.get('sort_by') || 'risk_score_desc',
  };

  const query = useQuery({
    queryKey: assetKeys.list(params),
    queryFn: () => assetApi.list(params),
    staleTime: 30_000,
    placeholderData: (prev) => prev,
  });

  const setFilter = (key: string, value: any) => {
    setSearchParams((prev) => {
      const next = new URLSearchParams(prev);
      if (!value || (Array.isArray(value) && value.length === 0)) {
        next.delete(key);
      } else if (Array.isArray(value)) {
        next.delete(key);
        value.forEach(v => next.append(key, v));
      } else {
        next.set(key, String(value));
      }
      next.set('page', '1');
      return next;
    });
  };

  return { ...query, params, setFilter };
}

export function useAssetTags() {
  return useQuery({
    queryKey: assetKeys.tags(),
    queryFn: () => assetApi.getTags(),
    staleTime: 5 * 60_000,  // Tags ít thay đổi
  });
}
```

### 3.4 `features/assets/hooks/useAssetDetail.ts`

```typescript
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { assetApi } from '../api/assetApi';
import { assetKeys } from './useAssets';

export function useAssetDetail(id: string) {
  return useQuery({
    queryKey: assetKeys.detail(id),
    queryFn: () => assetApi.getById(id),
    staleTime: 30_000,
    enabled: !!id,
  });
}

export function useAssetFindings(id: string, params: {
  status?: string;
  page?: number;
} = {}) {
  return useQuery({
    queryKey: assetKeys.findings(id, params),
    queryFn: () => assetApi.getFindings(id, params),
    staleTime: 30_000,
    enabled: !!id,
  });
}

export function usePatchAsset(id: string) {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (payload: { hostname?: string; tags?: string[] }) =>
      assetApi.patch(id, payload),
    onSuccess: (updated) => {
      queryClient.setQueryData(assetKeys.detail(id), updated);
      queryClient.invalidateQueries({ queryKey: assetKeys.list({}) });
    },
  });
}
```

### 3.5 Asset Detail — 5 Tabs Component

```tsx
// features/assets/components/AssetDetail.tsx
const TABS = ['Overview', 'Open Ports', 'Active Findings', 'Scan History', 'Tags'] as const;
type Tab = typeof TABS[number];

export function AssetDetail({ assetId }: { assetId: string }) {
  const [activeTab, setActiveTab] = useState<Tab>('Overview');
  const { data: asset, isLoading } = useAssetDetail(assetId);
  const { data: findings } = useAssetFindings(assetId, { status: 'active' });

  if (isLoading) return <AssetDetailSkeleton />;
  if (!asset) return <NotFound message="Asset not found" />;

  return (
    <div className="space-y-4">
      {/* Header */}
      <div className="flex items-center gap-4">
        <div>
          <h1 className="text-xl font-bold text-[var(--text-primary)]">
            {asset.hostname ?? asset.ip}
          </h1>
          <p className="text-sm text-[var(--text-secondary)]">{asset.ip} • {asset.os ?? 'Unknown OS'}</p>
        </div>
        <AssetRiskBadge score={asset.risk_score} />
      </div>

      {/* Tabs */}
      <Tabs value={activeTab} onValueChange={(v) => setActiveTab(v as Tab)}>
        <TabsList>
          {TABS.map(tab => <TabsTrigger key={tab} value={tab}>{tab}</TabsTrigger>)}
        </TabsList>

        <TabsContent value="Overview">
          <AssetOverview asset={asset} />
        </TabsContent>

        <TabsContent value="Open Ports">
          {/* Services table: port, protocol, service, version, CVEs */}
          <PortsTable services={asset.services} />
        </TabsContent>

        <TabsContent value="Active Findings">
          {/* Reuse FindingsList component với assetId filter */}
          <QueryBoundary query={{ data: findings, isLoading: false, isError: false }} skeleton={<Skeleton />}>
            {(d) => <FindingsMiniTable findings={d.findings} total={d.total} />}
          </QueryBoundary>
        </TabsContent>

        <TabsContent value="Scan History">
          <ScanHistoryTable scans={asset.scan_history ?? []} />
        </TabsContent>

        <TabsContent value="Tags">
          <AssetTagEditor assetId={assetId} currentTags={asset.tags} />
        </TabsContent>
      </Tabs>
    </div>
  );
}
```

### 3.6 AssetRiskBadge Component

```tsx
// features/assets/components/AssetRiskBadge.tsx
import { getRiskColor, getRiskLabel } from '../utils/assetRisk';

export function AssetRiskBadge({ score }: { score: number }) {
  const color = getRiskColor(score);
  const label = getRiskLabel(score);

  return (
    <span
      className="inline-flex items-center gap-1.5 px-2 py-1 rounded text-xs font-medium"
      style={{ backgroundColor: `${color}20`, color }}
    >
      <span className="w-1.5 h-1.5 rounded-full" style={{ backgroundColor: color }} />
      {label} Risk ({score.toFixed(1)})
    </span>
  );
}
```

---

## 4. MSW Handler — `mocks/handlers/asset.handlers.ts`

```typescript
import { http, HttpResponse } from 'msw';

const assetsFixture = [
  {
    id: 'asset_001',
    ip: '10.0.1.45',
    hostname: 'prod-web-01.internal',
    os: 'Linux 5.4.0 (Ubuntu 20.04)',
    services: [
      { port: 443, protocol: 'tcp', service: 'https', version: 'nginx 1.24.0',
        cve_ids: ['CVE-2025-44228'] },
      { port: 22, protocol: 'tcp', service: 'ssh', version: 'OpenSSH 8.9', cve_ids: [] },
    ],
    web_technologies: ['Nginx', 'React', 'Node.js'],
    tags: ['production', 'web', 'critical'],
    risk_score: 10.0,
    active_finding_count: 5,
    first_seen_at: '2026-01-15T08:00:00Z',
    last_seen_at: '2026-06-16T08:04:32Z',
    last_scan_id: 'sc_abc123',
  },
  {
    id: 'asset_002',
    ip: '10.0.1.46',
    hostname: 'prod-api-01.internal',
    os: 'Linux 5.4.0',
    services: [
      { port: 8080, protocol: 'tcp', service: 'http', version: 'Go 1.21', cve_ids: [] },
    ],
    web_technologies: ['Go'],
    tags: ['production', 'api'],
    risk_score: 4.2,
    active_finding_count: 2,
    first_seen_at: '2026-01-20T00:00:00Z',
    last_seen_at: '2026-06-16T08:04:32Z',
    last_scan_id: 'sc_abc123',
  },
];

export const assetHandlers = [
  http.get('/api/v1/assets', ({ request }) => {
    const url = new URL(request.url);
    const minRisk = url.searchParams.get('min_risk_score');
    const q = url.searchParams.get('q');

    let results = [...assetsFixture];
    if (minRisk) results = results.filter(a => a.risk_score >= Number(minRisk));
    if (q) results = results.filter(a =>
      a.ip.includes(q) || (a.hostname ?? '').includes(q)
    );

    return HttpResponse.json({
      assets: results,
      total: results.length,
      page: 1, page_size: 50,
      stats: {
        total: assetsFixture.length,
        high_risk: assetsFixture.filter(a => a.risk_score >= 8).length,
        by_os: [
          { os: 'Linux', count: 156 },
          { os: 'Windows', count: 98 },
        ],
      },
    });
  }),

  http.get('/api/v1/assets/tags', () => {
    return HttpResponse.json({
      tags: ['production', 'staging', 'web', 'api', 'database', 'critical', 'dmz'],
    });
  }),

  http.get('/api/v1/assets/:id', ({ params }) => {
    const asset = assetsFixture.find(a => a.id === params.id);
    if (!asset) return new HttpResponse(null, { status: 404 });
    return HttpResponse.json({
      ...asset,
      scan_history: [
        { scan_id: 'sc_abc123', type: 'nmap_full', status: 'completed',
          finding_count: 5, scanned_at: '2026-06-16T08:00:00Z' },
      ],
    });
  }),

  http.patch('/api/v1/assets/:id', async ({ params, request }) => {
    const body = await request.json() as any;
    const asset = assetsFixture.find(a => a.id === params.id);
    if (!asset) return new HttpResponse(null, { status: 404 });
    return HttpResponse.json({ ...asset, ...body });
  }),

  http.get('/api/v1/assets/:id/findings', () => {
    // Reuse findings fixture
    return HttpResponse.json({ findings: [], total: 0, page: 1, page_size: 20,
      by_severity: {}, by_status: {}, sla_stats: {} });
  }),
];
```

---

## 5. Acceptance Criteria (Frontend)

- [ ] Asset Inventory load từ `GET /api/v1/assets` — không hardcode
- [ ] Filter by tag → URL param `?tag=production` → re-fetch
- [ ] Filter by min_risk_score → chỉ hiển thị high-risk assets
- [ ] Search IP/hostname → `?q=10.0.1`
- [ ] Risk score badge đúng màu: Critical (đỏ), High (cam), Medium (vàng), Low (xanh)
- [ ] Asset Detail: 5 tabs load data từ API (`asset`, `asset.services`, `findings`, `scan_history`, `asset.tags`)
- [ ] Tag editor: PATCH `/api/v1/assets/:id` cập nhật tags
- [ ] `GET /api/v1/assets/tags` populate tag autocomplete filter

---

## 6. Phase Note

Assets là v3.0 feature. Trong khi chờ CR-OVS-007:
- `VITE_ENABLE_MSW=true` → MSW handlers cung cấp mock data
- Component hoàn chỉnh, sẵn sàng kết nối real API khi asset-service available
- Không có code change cần thiết khi tắt MSW
