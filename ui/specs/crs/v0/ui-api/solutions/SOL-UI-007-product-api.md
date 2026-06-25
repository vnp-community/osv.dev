# SOL-UI-007 — Frontend Solution: Product Security API

**CR nguồn:** [CR-UI-007](../../../../../specs/crs/v0/ui-api/CR-UI-007-product-api.md)  
**Ngày tạo:** 2026-06-16  
**Trạng thái:** Proposed  
**Ưu tiên:** P0 — Critical  
**Phạm vi:** Frontend React SPA (`ui/src/features/product-security/`)

---

## 1. Tóm tắt giải pháp

CR-UI-007 bao phủ Product/Engagement/Test hierarchy. Frontend cần:

1. `productApi.ts` — CRUD products, engagements, tests, grades
2. `useProducts.ts` / `useProductDetail.ts` hooks
3. `GradeCircle` shared component (reusable)
4. Product grade calculation utility (server-side, UI phải match formula)

---

## 2. File Structure

```
ui/src/
├── features/product-security/
│   ├── api/
│   │   └── productApi.ts
│   ├── hooks/
│   │   ├── useProducts.ts
│   │   ├── useProductDetail.ts
│   │   ├── useProductGrades.ts
│   │   └── useEngagements.ts
│   ├── components/
│   │   ├── ProductSecurity.tsx    # /products — list với grades
│   │   ├── ProductDetail.tsx      # /products/:id — detail + engagements
│   │   ├── GradeCircle.tsx        # Shared grade component
│   │   ├── ScorecardView.tsx      # All products grades
│   │   └── EngagementList.tsx
│   ├── utils/
│   │   └── gradeCalculator.ts    # Client-side grade formula (validate against server)
│   └── types.ts
│
└── mocks/handlers/
    └── product.handlers.ts
```

---

## 3. Implementation Chi Tiết

### 3.1 `features/product-security/api/productApi.ts`

```typescript
import apiClient from '@/shared/api/client';
import type {
  ProductListResponse, Product, CreateProductRequest,
  EngagementListResponse, Engagement, CreateEngagementRequest,
  TestListResponse, ProductGradesResponse
} from '../types';

export const productApi = {
  // GET /api/v1/products
  list: async (params: {
    page?: number;
    page_size?: number;
    q?: string;
    product_type?: string;
    criticality?: string;
    lifecycle?: string;
  } = {}): Promise<ProductListResponse> => {
    const { data } = await apiClient.get<ProductListResponse>('/api/v1/products', { params });
    return data;
  },

  // POST /api/v1/products
  create: async (payload: CreateProductRequest): Promise<Product> => {
    const { data } = await apiClient.post<Product>('/api/v1/products', payload);
    return data;
  },

  // GET /api/v1/products/{id}
  getById: async (id: string): Promise<Product> => {
    const { data } = await apiClient.get<Product>(`/api/v1/products/${id}`);
    return data;
  },

  // PATCH /api/v1/products/{id}
  patch: async (id: string, payload: Partial<CreateProductRequest>): Promise<Product> => {
    const { data } = await apiClient.patch<Product>(`/api/v1/products/${id}`, payload);
    return data;
  },

  // GET /api/v1/products/{id}/engagements
  getEngagements: async (productId: string): Promise<EngagementListResponse> => {
    const { data } = await apiClient.get<EngagementListResponse>(
      `/api/v1/products/${productId}/engagements`
    );
    return data;
  },

  // POST /api/v1/products/{id}/engagements
  createEngagement: async (productId: string, payload: CreateEngagementRequest): Promise<Engagement> => {
    const { data } = await apiClient.post<Engagement>(
      `/api/v1/products/${productId}/engagements`,
      payload
    );
    return data;
  },

  // GET /api/v1/engagements/{id}/tests
  getTests: async (engagementId: string): Promise<TestListResponse> => {
    const { data } = await apiClient.get<TestListResponse>(
      `/api/v1/engagements/${engagementId}/tests`
    );
    return data;
  },

  // GET /api/v1/products/grades
  getGrades: async (): Promise<ProductGradesResponse> => {
    const { data } = await apiClient.get<ProductGradesResponse>('/api/v1/products/grades');
    return data;
  },

  // GET /api/v1/products/types
  getTypes: async () => {
    const { data } = await apiClient.get('/api/v1/products/types');
    return data as { types: Array<{ value: string; label: string }> };
  },
};
```

### 3.2 `features/product-security/utils/gradeCalculator.ts`

```typescript
// Grade calculation — phải match server-side formula (CR-UI-007 §2.9)
export type Grade = 'A' | 'B' | 'C' | 'D' | 'F';

export interface FindingSummary {
  critical: number;
  high: number;
  medium: number;
  low: number;
  total_active: number;
}

export function calculateGrade(summary: FindingSummary): Grade {
  const { critical, high, total_active } = summary;

  if (critical === 0 && high === 0) return 'A';
  if (critical === 0 && high <= 5) return 'B';
  if (critical === 0 && high > 5) return 'C';
  if (critical === 1 || critical === 2) return 'D';
  if (critical >= 3 || total_active > 20) return 'F';

  return 'C'; // fallback
}

// Grade → visual properties
export const GRADE_STYLES: Record<Grade, { color: string; bg: string; label: string }> = {
  'A': { color: '#10B981', bg: '#10B98120', label: 'Excellent' },
  'B': { color: '#3B82F6', bg: '#3B82F620', label: 'Good' },
  'C': { color: '#EAB308', bg: '#EAB30820', label: 'Fair' },
  'D': { color: '#F97316', bg: '#F9731620', label: 'Poor' },
  'F': { color: '#EF4444', bg: '#EF444420', label: 'Critical' },
};
```

### 3.3 `features/product-security/hooks/useProducts.ts`

```typescript
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { useSearchParams } from 'react-router-dom';
import { productApi } from '../api/productApi';

export const productKeys = {
  all: ['products'] as const,
  list: (params: object) => [...productKeys.all, 'list', params] as const,
  detail: (id: string) => [...productKeys.all, 'detail', id] as const,
  engagements: (id: string) => [...productKeys.all, 'engagements', id] as const,
  grades: () => [...productKeys.all, 'grades'] as const,
  types: () => [...productKeys.all, 'types'] as const,
};

export function useProducts() {
  const [searchParams, setSearchParams] = useSearchParams();

  const params = {
    q: searchParams.get('q') || undefined,
    product_type: searchParams.get('type') || undefined,
    criticality: searchParams.get('criticality') || undefined,
    page: Number(searchParams.get('page') || '1'),
    page_size: Number(searchParams.get('page_size') || '20'),
  };

  const query = useQuery({
    queryKey: productKeys.list(params),
    queryFn: () => productApi.list(params),
    staleTime: 60_000,
  });

  const setFilter = (key: string, value: string | undefined) => {
    setSearchParams((prev) => {
      const next = new URLSearchParams(prev);
      value ? next.set(key, value) : next.delete(key);
      next.set('page', '1');
      return next;
    });
  };

  return { ...query, params, setFilter };
}

export function useProductGrades() {
  return useQuery({
    queryKey: productKeys.grades(),
    queryFn: () => productApi.getGrades(),
    staleTime: 5 * 60_000,
  });
}

export function useCreateProduct() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: productApi.create,
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: productKeys.all });
    },
  });
}
```

### 3.4 `GradeCircle` Shared Component

```tsx
// shared/components/GradeCircle.tsx (hoặc features/product-security/components)
import { GRADE_STYLES, type Grade } from '../utils/gradeCalculator';

export function GradeCircle({
  grade,
  score,
  size = 'md',
  showScore = true,
}: {
  grade: string;
  score: number;
  size?: 'sm' | 'md' | 'lg';
  showScore?: boolean;
}) {
  const g = grade as Grade;
  const style = GRADE_STYLES[g] ?? GRADE_STYLES['F'];
  const sizes = { sm: 'w-10 h-10 text-lg', md: 'w-14 h-14 text-2xl', lg: 'w-20 h-20 text-4xl' };

  return (
    <div className="flex flex-col items-center gap-1">
      <div
        className={`${sizes[size]} rounded-full flex items-center justify-center font-bold border-2`}
        style={{
          backgroundColor: style.bg,
          color: style.color,
          borderColor: style.color,
        }}
      >
        {g}
      </div>
      {showScore && (
        <span className="text-xs text-[var(--text-secondary)]">{score}/100</span>
      )}
    </div>
  );
}
```

### 3.5 Product Security List Screen

```tsx
// features/product-security/components/ProductSecurity.tsx
export function ProductSecurity() {
  const { data, isLoading, isError, refetch, setFilter } = useProducts();
  const { canWriteFindings } = usePermissions();
  const [showCreateDialog, setShowCreateDialog] = useState(false);

  if (isLoading) return <ProductListSkeleton />;
  if (isError) return <ErrorState onRetry={refetch} />;

  return (
    <div className="space-y-6">
      {/* Header */}
      <div className="flex items-center justify-between">
        <h1 className="text-2xl font-bold">Product Security</h1>
        {canWriteFindings && (
          <Button onClick={() => setShowCreateDialog(true)}>+ New Product</Button>
        )}
      </div>

      {/* Scorecards Link */}
      <Link to="/products/grades" className="text-sm text-[var(--brand-blue)]">
        View All Scorecards →
      </Link>

      {/* Products Grid */}
      <div className="grid grid-cols-1 gap-4">
        {data?.products.map(product => (
          <ProductCard key={product.id} product={product} />
        ))}
      </div>

      {/* Pagination */}
      <Pagination
        total={data?.total ?? 0}
        page={1}
        pageSize={20}
      />

      {/* Create Dialog */}
      {showCreateDialog && (
        <CreateProductDialog onClose={() => setShowCreateDialog(false)} />
      )}
    </div>
  );
}
```

---

## 4. MSW Handler — `mocks/handlers/product.handlers.ts`

```typescript
import { http, HttpResponse } from 'msw';

const productsFixture = [
  {
    id: 'prod_1', name: 'Banking Portal',
    description: 'Customer-facing online banking application',
    type: 'web_app', criticality: 'critical', lifecycle: 'production',
    grade: 'D', score: 42,
    finding_summary: { critical: 2, high: 8, medium: 15, low: 20, total_active: 45 },
    sla_config: { product_id: 'prod_1', critical_days: 3, high_days: 14, medium_days: 60, low_days: 120 },
    tags: ['banking', 'pci-dss', 'production'],
    created_at: '2026-01-15T08:00:00Z',
  },
  {
    id: 'prod_2', name: 'Mobile App',
    description: 'iOS and Android mobile banking app',
    type: 'mobile', criticality: 'high', lifecycle: 'production',
    grade: 'B', score: 76,
    finding_summary: { critical: 0, high: 4, medium: 12, low: 18, total_active: 34 },
    sla_config: null,
    tags: ['mobile', 'ios', 'android'],
    created_at: '2026-02-10T08:00:00Z',
  },
];

export const productHandlers = [
  http.get('/api/v1/products', () => {
    return HttpResponse.json({
      products: productsFixture,
      total: productsFixture.length,
      page: 1, page_size: 20,
    });
  }),

  http.get('/api/v1/products/grades', () => {
    return HttpResponse.json({
      products: productsFixture.map(p => ({
        id: p.id, name: p.name, grade: p.grade, score: p.score,
        critical_count: p.finding_summary.critical,
        high_count: p.finding_summary.high,
        trend: p.grade === 'D' ? 'worsening' : 'improving',
      })),
      overall_grade: 'C',
      overall_score: 58,
    });
  }),

  http.get('/api/v1/products/types', () => {
    return HttpResponse.json({
      types: [
        { value: 'web_app', label: 'Web Application' },
        { value: 'api', label: 'API' },
        { value: 'infrastructure', label: 'Infrastructure' },
        { value: 'mobile', label: 'Mobile' },
      ],
    });
  }),

  http.get('/api/v1/products/:id', ({ params }) => {
    const product = productsFixture.find(p => p.id === params.id);
    if (!product) return new HttpResponse(null, { status: 404 });
    return HttpResponse.json({
      ...product,
      engagements: [
        { id: 'eng_001', product_id: product.id,
          name: 'Q2 2026 Security Assessment', type: 'interactive',
          start_date: '2026-04-01', end_date: '2026-04-30',
          status: 'completed', lead_id: 'usr_bob123', cicd_url: null },
      ],
    });
  }),

  http.post('/api/v1/products', async ({ request }) => {
    const body = await request.json() as any;
    return HttpResponse.json({
      id: 'prod_' + Date.now(),
      ...body,
      grade: 'A', score: 100,
      finding_summary: { critical: 0, high: 0, medium: 0, low: 0, total_active: 0 },
      sla_config: null, tags: body.tags || [],
      created_at: new Date().toISOString(),
    }, { status: 201 });
  }),

  http.get('/api/v1/products/:id/engagements', ({ params }) => {
    return HttpResponse.json({
      engagements: [
        { id: 'eng_001', product_id: params.id,
          name: 'Q2 2026 Security Assessment', type: 'interactive',
          start_date: '2026-04-01', end_date: '2026-04-30',
          status: 'completed', lead_id: 'usr_bob123', cicd_url: null,
          test_count: 3, finding_count: 15 },
      ],
      total: 1,
    });
  }),

  http.post('/api/v1/products/:id/engagements', async ({ params, request }) => {
    const body = await request.json() as any;
    return HttpResponse.json({
      id: 'eng_' + Date.now(), product_id: params.id, ...body,
      status: 'not_started',
    }, { status: 201 });
  }),
];
```

---

## 5. Acceptance Criteria (Frontend)

- [ ] Product list load từ `GET /api/v1/products` — không hardcode
- [ ] `GradeCircle` hiển thị đúng màu theo Grade formula (A=xanh, B=blue, C=vàng, D=cam, F=đỏ)
- [ ] Grade formula client-side match server-side (CR-UI-007 §2.9)
- [ ] Scorecards screen (`/products/grades`) từ `GET /api/v1/products/grades`
- [ ] `trend` field hiển thị improving (↑) / worsening (↓) icon
- [ ] Product Detail: engagements list từ API
- [ ] Create Product form: `grade = 'A'` khi mới tạo (no findings)
- [ ] Finding summary per product hiển thị từ `product.finding_summary.*`
