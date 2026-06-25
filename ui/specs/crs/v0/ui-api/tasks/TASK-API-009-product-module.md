# TASK-API-009 — Product Module: productApi + Grade Formula + Hooks + MSW

| Field | Value |
|-------|-------|
| **Task ID** | TASK-API-009 |
| **Module** | `ui/src/features/product-security/` |
| **Solution Ref** | [SOL-UI-007 §3](../solutions/SOL-UI-007-product-api.md) |
| **Priority** | 🔴 P0 |
| **Depends On** | TASK-API-003 |
| **Estimated** | 2h |

---

## Context

Product Security module quản lý Products + Engagements. Grade của product được tính từ `finding_summary` (critical×15 + high×5 + medium×1). Frontend nhận `grade` từ server — không tự tính. `calculateProductGrade()` chỉ dùng làm local utility khi server chưa trả grade.

---

## Goal

Tạo Product feature module với API, hooks, MSW. Bao gồm cả Asset module vì chúng liên quan chặt.

---

## Target Files

| Action | File Path |
|--------|-----------|
| CREATE | `ui/src/features/product-security/types.ts` |
| CREATE | `ui/src/features/product-security/api/productApi.ts` |
| CREATE | `ui/src/features/product-security/api/assetApi.ts` |
| CREATE | `ui/src/features/product-security/hooks/useProducts.ts` |
| CREATE | `ui/src/features/product-security/hooks/useAssets.ts` |
| CREATE | `ui/src/mocks/fixtures/product.fixture.ts` |
| CREATE | `ui/src/mocks/handlers/product.handlers.ts` |
| CREATE | `ui/src/mocks/fixtures/asset.fixture.ts` |
| CREATE | `ui/src/mocks/handlers/asset.handlers.ts` |

---

## Implementation

### File 1: `ui/src/features/product-security/types.ts`

```typescript
import type { Severity } from '@/features/cve-intel/types';
import type { Finding } from '@/features/findings/types';

export type ProductGrade = 'A' | 'A-' | 'B+' | 'B' | 'B-' | 'C+' | 'C' | 'D' | 'F';

export interface FindingSummary {
  critical: number;
  high: number;
  medium: number;
  low: number;
  total: number;
  active: number;
  mitigated: number;
  risk_accepted: number;
  false_positive: number;
}

export interface Product {
  id: string;
  name: string;
  description: string | null;
  product_type: string;
  team: string;
  business_criticality: 'Critical' | 'High' | 'Medium' | 'Low';
  lifecycle: 'Production' | 'Beta' | 'Development' | 'Maintenance' | 'Retired';
  revenue_impact: boolean;
  external_audience: boolean;
  internet_accessible: boolean;
  tags: string[];
  grade: ProductGrade | null;
  score: number | null;
  sla_compliance_pct: number;
  finding_summary: FindingSummary;
  engagement_count: number;
  last_scan_at: string | null;
  created_at: string;
  created_by: string;
}

export interface ProductListResponse {
  products: Product[];
  total: number;
  page: number;
  page_size: number;
}

export interface ProductGradeItem {
  product_id: string;
  product_name: string;
  grade: ProductGrade;
  score: number;
  finding_summary: FindingSummary;
  sla_compliance_pct: number;
  last_scan_at: string | null;
  trend: 'improving' | 'stable' | 'declining';
}

export interface Engagement {
  id: string;
  product_id: string;
  name: string;
  description: string | null;
  engagement_type: string;
  status: 'Not Started' | 'In Progress' | 'On Hold' | 'Completed' | 'Cancelled';
  start_date: string;
  end_date: string | null;
  lead_engineer: string;
  test_count: number;
  finding_count: number;
  created_at: string;
}

export interface ProductType {
  id: string;
  name: string;
}

// Asset types
export type RiskCategory = 'Critical' | 'High' | 'Medium' | 'Low';

export interface Asset {
  id: string;
  name: string;
  type: 'host' | 'web_application' | 'api' | 'database' | 'cloud_resource' | 'mobile_app' | 'container';
  environment: 'Production' | 'Staging' | 'Development' | 'QA';
  hostname: string | null;
  ip_address: string | null;
  mac_address: string | null;
  os: string | null;
  os_version: string | null;
  url: string | null;
  port: number | null;
  risk_score: number;       // 0-100
  risk_category: RiskCategory;
  finding_count: number;
  critical_finding_count: number;
  tags: string[];
  owner: string | null;
  product_id: string | null;
  product_name: string | null;
  last_scan_at: string | null;
  first_seen_at: string;
  created_at: string;
}

export interface AssetListResponse {
  assets: Asset[];
  total: number;
  page: number;
  page_size: number;
}
```

### File 2: `ui/src/features/product-security/api/productApi.ts`

```typescript
import { apiClient } from '@/shared/api/client';
import { ENDPOINTS } from '@/shared/api/endpoints';
import type { Product, ProductListResponse, ProductGradeItem, Engagement, ProductType } from '../types';

export const productApi = {
  list: async (params: {
    q?: string;
    product_type?: string;
    business_criticality?: string;
    page?: number;
    page_size?: number;
  } = {}): Promise<ProductListResponse> => {
    const { data } = await apiClient.get<ProductListResponse>(ENDPOINTS.products.list, { params });
    return data;
  },

  getById: async (id: string): Promise<Product> => {
    const { data } = await apiClient.get<Product>(ENDPOINTS.products.detail(id));
    return data;
  },

  create: async (payload: Partial<Product>): Promise<Product> => {
    const { data } = await apiClient.post<Product>(ENDPOINTS.products.create, payload);
    return data;
  },

  update: async (id: string, payload: Partial<Product>): Promise<Product> => {
    const { data } = await apiClient.patch<Product>(ENDPOINTS.products.patch(id), payload);
    return data;
  },

  getEngagements: async (productId: string): Promise<{ engagements: Engagement[]; total: number }> => {
    const { data } = await apiClient.get(ENDPOINTS.products.engagements(productId));
    return data as { engagements: Engagement[]; total: number };
  },

  getGrades: async (): Promise<{ products: ProductGradeItem[]; total: number }> => {
    const { data } = await apiClient.get(ENDPOINTS.products.grades);
    return data as { products: ProductGradeItem[]; total: number };
  },

  getTypes: async (): Promise<{ types: ProductType[] }> => {
    const { data } = await apiClient.get(ENDPOINTS.products.types);
    return data as { types: ProductType[] };
  },
};
```

### File 3: `ui/src/features/product-security/api/assetApi.ts`

```typescript
import { apiClient } from '@/shared/api/client';
import { ENDPOINTS } from '@/shared/api/endpoints';
import type { Asset, AssetListResponse } from '../types';

export const assetApi = {
  list: async (params: {
    q?: string;
    type?: string;
    environment?: string;
    risk_category?: string;
    product_id?: string;
    tags?: string[];
    page?: number;
    page_size?: number;
    sort_by?: string;
  } = {}): Promise<AssetListResponse> => {
    const { data } = await apiClient.get<AssetListResponse>(ENDPOINTS.assets.list, {
      params: { ...params, tags: params.tags?.join(',') },
    });
    return data;
  },

  getById: async (id: string): Promise<Asset> => {
    const { data } = await apiClient.get<Asset>(ENDPOINTS.assets.detail(id));
    return data;
  },

  update: async (id: string, payload: Partial<Asset>): Promise<Asset> => {
    const { data } = await apiClient.patch<Asset>(ENDPOINTS.assets.patch(id), payload);
    return data;
  },

  getFindings: async (id: string, params: { page?: number } = {}) => {
    const { data } = await apiClient.get(ENDPOINTS.assets.findings(id), { params });
    return data;
  },

  getAvailableTags: async (): Promise<{ tags: string[] }> => {
    const { data } = await apiClient.get(ENDPOINTS.assets.tags);
    return data as { tags: string[] };
  },
};
```

### File 4: `ui/src/features/product-security/hooks/useProducts.ts`

```typescript
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { productApi } from '../api/productApi';
import toast from 'react-hot-toast';

export const productKeys = {
  all:         ['products'] as const,
  list:        (params: object) => ['products', 'list', params] as const,
  detail:      (id: string) => ['products', 'detail', id] as const,
  engagements: (id: string) => ['products', 'engagements', id] as const,
  grades:      () => ['products', 'grades'] as const,
  types:       () => ['products', 'types'] as const,
};

export function useProducts(params: { q?: string; productType?: string; page?: number } = {}) {
  const queryParams = { q: params.q, product_type: params.productType, page: params.page ?? 1 };
  return useQuery({
    queryKey:  productKeys.list(queryParams),
    queryFn:   () => productApi.list(queryParams),
    staleTime: 5 * 60_000,
  });
}

export function useProductDetail(id: string | null) {
  return useQuery({
    queryKey: productKeys.detail(id ?? ''),
    queryFn:  () => productApi.getById(id!),
    enabled:  !!id,
    staleTime: 5 * 60_000,
  });
}

export function useProductGrades() {
  return useQuery({
    queryKey: productKeys.grades(),
    queryFn:  () => productApi.getGrades(),
    staleTime: 5 * 60_000,
    refetchInterval: 5 * 60_000,
  });
}

export function useProductEngagements(productId: string | null) {
  return useQuery({
    queryKey: productKeys.engagements(productId ?? ''),
    queryFn:  () => productApi.getEngagements(productId!),
    enabled:  !!productId,
    staleTime: 5 * 60_000,
  });
}

export function useCreateProduct() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: productApi.create,
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: productKeys.all });
      toast.success('Product created');
    },
  });
}

export function useProductTypes() {
  return useQuery({
    queryKey: productKeys.types(),
    queryFn:  () => productApi.getTypes(),
    staleTime: 24 * 60 * 60_000, // Product types rất ổn định
  });
}
```

### File 5: `ui/src/features/product-security/hooks/useAssets.ts`

```typescript
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { assetApi } from '../api/assetApi';
import toast from 'react-hot-toast';

export const assetKeys = {
  all:      ['assets'] as const,
  list:     (params: object) => ['assets', 'list', params] as const,
  detail:   (id: string) => ['assets', 'detail', id] as const,
  findings: (id: string) => ['assets', 'findings', id] as const,
  tags:     () => ['assets', 'tags'] as const,
};

export function useAssets(params: {
  q?: string;
  type?: string;
  environment?: string;
  riskCategory?: string;
  productId?: string;
  page?: number;
} = {}) {
  const queryParams = {
    q: params.q, type: params.type, environment: params.environment,
    risk_category: params.riskCategory, product_id: params.productId, page: params.page ?? 1,
  };
  return useQuery({
    queryKey:  assetKeys.list(queryParams),
    queryFn:   () => assetApi.list(queryParams),
    staleTime: 30_000,
  });
}

export function useAssetDetail(id: string | null) {
  return useQuery({
    queryKey: assetKeys.detail(id ?? ''),
    queryFn:  () => assetApi.getById(id!),
    enabled:  !!id,
    staleTime: 30_000,
  });
}

export function useAssetFindings(assetId: string | null) {
  return useQuery({
    queryKey: assetKeys.findings(assetId ?? ''),
    queryFn:  () => assetApi.getFindings(assetId!),
    enabled:  !!assetId,
    staleTime: 30_000,
  });
}

export function useAvailableTags() {
  return useQuery({
    queryKey: assetKeys.tags(),
    queryFn:  () => assetApi.getAvailableTags(),
    staleTime: 60 * 60_000,
  });
}

export function useUpdateAsset(assetId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (payload: Parameters<typeof assetApi.update>[1]) => assetApi.update(assetId, payload),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: assetKeys.detail(assetId) });
      qc.invalidateQueries({ queryKey: assetKeys.list({}) });
      toast.success('Asset updated');
    },
  });
}
```

### File 6: `ui/src/mocks/fixtures/product.fixture.ts`

```typescript
import type { Product, ProductGradeItem } from '@/features/product-security/types';

export const productsFixture: Product[] = [
  {
    id: 'prod_1', name: 'Banking Portal', description: 'Customer-facing online banking.',
    product_type: 'Web Application', team: 'Core Banking',
    business_criticality: 'Critical', lifecycle: 'Production',
    revenue_impact: true, external_audience: true, internet_accessible: true,
    tags: ['pci-dss', 'customer-facing'],
    grade: 'D', score: 42, sla_compliance_pct: 71.0,
    finding_summary: { critical: 2, high: 8, medium: 12, low: 5, total: 27, active: 20, mitigated: 5, risk_accepted: 1, false_positive: 1 },
    engagement_count: 3, last_scan_at: '2026-06-16T08:04:32Z',
    created_at: '2025-01-15T09:00:00Z', created_by: 'admin@company.com',
  },
  {
    id: 'prod_2', name: 'Mobile App', description: 'iOS and Android banking app.',
    product_type: 'Mobile Application', team: 'Mobile',
    business_criticality: 'High', lifecycle: 'Production',
    revenue_impact: true, external_audience: true, internet_accessible: false,
    tags: ['mobile', 'customer-facing'],
    grade: 'B', score: 76, sla_compliance_pct: 88.0,
    finding_summary: { critical: 0, high: 4, medium: 8, low: 12, total: 24, active: 12, mitigated: 10, risk_accepted: 2, false_positive: 0 },
    engagement_count: 2, last_scan_at: '2026-06-10T14:00:00Z',
    created_at: '2025-02-01T10:00:00Z', created_by: 'admin@company.com',
  },
  {
    id: 'prod_3', name: 'Internal API', description: 'Backend microservices API.',
    product_type: 'API', team: 'Platform',
    business_criticality: 'High', lifecycle: 'Production',
    revenue_impact: false, external_audience: false, internet_accessible: false,
    tags: ['internal', 'api'],
    grade: 'C', score: 61, sla_compliance_pct: 82.5,
    finding_summary: { critical: 1, high: 6, medium: 10, low: 8, total: 25, active: 15, mitigated: 7, risk_accepted: 2, false_positive: 1 },
    engagement_count: 4, last_scan_at: '2026-06-15T09:30:00Z',
    created_at: '2025-01-20T08:00:00Z', created_by: 'admin@company.com',
  },
];

export const productGradesFixture: ProductGradeItem[] = productsFixture.map(p => ({
  product_id: p.id, product_name: p.name,
  grade: p.grade!, score: p.score!,
  finding_summary: p.finding_summary,
  sla_compliance_pct: p.sla_compliance_pct,
  last_scan_at: p.last_scan_at,
  trend: p.score! >= 75 ? 'stable' : 'declining',
}));
```

### File 7: `ui/src/mocks/handlers/product.handlers.ts`

```typescript
import { http, HttpResponse } from 'msw';
import { ENDPOINTS } from '@/shared/api/endpoints';
import { productsFixture, productGradesFixture } from '../fixtures/product.fixture';
import { assetsFixture } from '../fixtures/asset.fixture';

export const productHandlers = [
  http.get(ENDPOINTS.products.list, () => {
    return HttpResponse.json({ products: productsFixture, total: productsFixture.length, page: 1, page_size: 20 });
  }),
  http.get('/api/v1/products/:id', ({ params }) => {
    const p = productsFixture.find(x => x.id === params.id);
    if (!p) return HttpResponse.json({ error: 'NOT_FOUND' }, { status: 404 });
    return HttpResponse.json(p);
  }),
  http.get('/api/v1/products/:id/engagements', ({ params }) => {
    return HttpResponse.json({
      engagements: [
        { id: 'eng_001', product_id: params.id, name: 'Q2 2026 Security Assessment',
          engagement_type: 'Penetration Test', status: 'In Progress',
          start_date: '2026-06-01', end_date: '2026-06-30', lead_engineer: 'bob@company.com',
          test_count: 3, finding_count: 15, created_at: '2026-05-20T00:00:00Z',
          description: null },
      ],
      total: 1,
    });
  }),
  http.get(ENDPOINTS.products.grades, () => {
    return HttpResponse.json({ products: productGradesFixture, total: productGradesFixture.length });
  }),
  http.get(ENDPOINTS.products.types, () => {
    return HttpResponse.json({
      types: [
        { id: 'web_app', name: 'Web Application' },
        { id: 'api', name: 'API' },
        { id: 'mobile', name: 'Mobile Application' },
        { id: 'network', name: 'Network Infrastructure' },
      ],
    });
  }),
  http.post(ENDPOINTS.products.create, async ({ request }) => {
    const body = await request.json() as any;
    return HttpResponse.json({ id: 'prod_new_' + Date.now(), ...body, grade: null, score: null, finding_summary: { critical: 0, high: 0, medium: 0, low: 0, total: 0, active: 0, mitigated: 0, risk_accepted: 0, false_positive: 0 }, sla_compliance_pct: 100, engagement_count: 0, last_scan_at: null, created_at: new Date().toISOString(), created_by: 'bob@company.com' }, { status: 201 });
  }),
];
```

### File 8: `ui/src/mocks/fixtures/asset.fixture.ts`

```typescript
import type { Asset } from '@/features/product-security/types';

export const assetsFixture: Asset[] = [
  { id: 'ast_001', name: 'app-server-01.internal', type: 'host', environment: 'Production',
    hostname: 'app-server-01.internal', ip_address: '10.0.1.10', mac_address: null,
    os: 'Ubuntu', os_version: '22.04', url: null, port: null,
    risk_score: 92, risk_category: 'Critical', finding_count: 18, critical_finding_count: 3,
    tags: ['backend', 'production'], owner: 'platform-team@company.com',
    product_id: 'prod_1', product_name: 'Banking Portal',
    last_scan_at: '2026-06-16T08:04:32Z', first_seen_at: '2025-01-10T00:00:00Z',
    created_at: '2025-01-10T00:00:00Z' },
  { id: 'ast_002', name: 'bastion-host.internal', type: 'host', environment: 'Production',
    hostname: 'bastion-host.internal', ip_address: '10.0.0.1', mac_address: null,
    os: 'CentOS', os_version: '9', url: null, port: 22,
    risk_score: 78, risk_category: 'High', finding_count: 7, critical_finding_count: 1,
    tags: ['bastion', 'ssh', 'production'], owner: null,
    product_id: null, product_name: null,
    last_scan_at: '2026-06-14T10:00:00Z', first_seen_at: '2025-01-05T00:00:00Z',
    created_at: '2025-01-05T00:00:00Z' },
  { id: 'ast_003', name: 'web-proxy-01.internal', type: 'host', environment: 'Production',
    hostname: 'web-proxy-01.internal', ip_address: '10.0.1.5', mac_address: null,
    os: 'Debian', os_version: '11', url: null, port: 80,
    risk_score: 45, risk_category: 'Medium', finding_count: 4, critical_finding_count: 0,
    tags: ['nginx', 'proxy', 'production'], owner: null,
    product_id: 'prod_2', product_name: 'Mobile App',
    last_scan_at: '2026-06-10T11:00:00Z', first_seen_at: '2025-02-01T00:00:00Z',
    created_at: '2025-02-01T00:00:00Z' },
];
```

### File 9: `ui/src/mocks/handlers/asset.handlers.ts`

```typescript
import { http, HttpResponse } from 'msw';
import { ENDPOINTS } from '@/shared/api/endpoints';
import { assetsFixture } from '../fixtures/asset.fixture';

export const assetHandlers = [
  http.get(ENDPOINTS.assets.list, () => {
    return HttpResponse.json({ assets: assetsFixture, total: assetsFixture.length, page: 1, page_size: 20 });
  }),
  http.get('/api/v1/assets/:id', ({ params }) => {
    const a = assetsFixture.find(x => x.id === params.id);
    if (!a) return HttpResponse.json({ error: 'NOT_FOUND' }, { status: 404 });
    return HttpResponse.json(a);
  }),
  http.patch('/api/v1/assets/:id', async ({ params, request }) => {
    const body = await request.json() as any;
    const a = assetsFixture.find(x => x.id === params.id);
    return HttpResponse.json({ ...(a ?? {}), ...body });
  }),
  http.get(ENDPOINTS.assets.tags, () => {
    return HttpResponse.json({ tags: ['production', 'backend', 'nginx', 'ssh', 'pci-dss', 'api', 'mobile', 'bastion'] });
  }),
  http.get('/api/v1/assets/:id/findings', () => {
    return HttpResponse.json({ findings: [], total: 0 });
  }),
];
```

---

## Verification

```bash
cd ui/
VITE_ENABLE_MSW=true pnpm dev

# 1. /products → Danh sách 3 products với grades D/B/C
# 2. /products/grades → Grade Scorecards (D, B, C)
# 3. Product Detail /products/prod_1 → Finding Summary: Critical 2, High 8
# 4. /assets → List 3 assets
# 5. Asset risk_score 92 → Critical badge màu đỏ

npx tsc --noEmit
# Expected: no errors
```

---

## Checklist

- [ ] `features/product-security/types.ts` — Product, Asset, Engagement, ProductGradeItem
- [ ] `productApi.ts` — 7 methods dùng `ENDPOINTS.products.*`
- [ ] `assetApi.ts` — 5 methods dùng `ENDPOINTS.assets.*`
- [ ] `useProducts` — staleTime 5m, `useProductGrades` — refetchInterval 5m
- [ ] `useAssets` — filter q, type, environment, riskCategory, productId
- [ ] `useUpdateAsset` — invalidate detail + list khi success
- [ ] 3 product fixtures với grades đa dạng (D, B, C)
- [ ] 3 asset fixtures với risk_category đa dạng (Critical, High, Medium)
- [ ] Product handlers: list, detail, engagements, grades, types, create
- [ ] Asset handlers: list, detail, patch, tags, findings
- [ ] `npx tsc --noEmit` không lỗi
