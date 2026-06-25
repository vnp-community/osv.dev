# TASK-UI-007 — Dashboard Module Migration

| Field | Value |
|-------|-------|
| **Task ID** | TASK-UI-007 |
| **Module** | `ui/src/features/dashboard/` |
| **Solution Ref** | [SOL-003 §3](../solutions/SOL-003-phase2-api-migration.md#3-module-dashboard-migration) |
| **Priority** | 🔴 P0 |
| **Depends On** | TASK-UI-004, TASK-UI-005, TASK-UI-006 |
| **Estimated** | 3h |
| **Status** | ✅ Completed — tất cả 11/11 items done (2026-06-17) |
| **Updated** | 2026-06-17 |

---

## Context

`src/app/components/Dashboard.tsx` hiện chứa **7 hardcoded data arrays** (vi phạm P0 theo `architecture.md §5.5`):

```typescript
// ❌ CẦN XÓA: 7 hardcoded arrays trong Dashboard.tsx
const riskTrendData = [...];   // Line ~11
const severityData = [...];    // Line ~20
const productData = [...];     // Line ~27
const recentFindings = [...];  // Line ~35
const kevAlerts = [...];       // Line ~103
const recentScans = [...];     // Line ~109
const slaBreaches = [...];     // Line ~115
```

---

## Goal

1. Tạo feature module `src/features/dashboard/` với API layer + React Query hook
2. Migrate `Dashboard.tsx` để dùng `useDashboardMetrics` hook thay vì hardcode
3. Tạo `DashboardSkeleton` component

---

## Target Files

| Action | File Path |
|--------|-----------|
| CREATE | `ui/src/features/dashboard/types.ts` |
| CREATE | `ui/src/features/dashboard/api/dashboardApi.ts` |
| CREATE | `ui/src/features/dashboard/hooks/useDashboardMetrics.ts` |
| CREATE | `ui/src/features/dashboard/components/DashboardSkeleton.tsx` |
| MODIFY | `ui/src/app/components/Dashboard.tsx` |

---

## Implementation

### File 1: `ui/src/features/dashboard/types.ts`

```typescript
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

export interface ProductGradeItem {
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
  productGrades: ProductGradeItem[];
  kevAlerts: KEVAlert[];
  recentScans: Scan[];
  slaBreaches: SLABreach[];
}
```

### File 2: `ui/src/features/dashboard/api/dashboardApi.ts`

```typescript
import { apiClient } from '@/shared/api/client';
import { ENDPOINTS } from '@/shared/api/endpoints';
import type { DashboardData } from '../types';

export const dashboardApi = {
  getMetrics: async (period: '30d' | '90d' | '1y'): Promise<DashboardData> => {
    const { data } = await apiClient.get<DashboardData>(ENDPOINTS.dashboard, {
      params: { period },
    });
    return data;
  },
};
```

### File 3: `ui/src/features/dashboard/hooks/useDashboardMetrics.ts`

```typescript
import { useQuery } from '@tanstack/react-query';
import { dashboardKeys } from '@/shared/api/queryClient';
import { dashboardApi } from '../api/dashboardApi';

export function useDashboardMetrics(period: '30d' | '90d' | '1y' = '30d') {
  return useQuery({
    queryKey: dashboardKeys.metrics(period),
    queryFn: () => dashboardApi.getMetrics(period),
    staleTime: 60_000,           // 1 min
    refetchInterval: 60_000,     // Auto-refresh
  });
}
```

### File 4: `ui/src/features/dashboard/components/DashboardSkeleton.tsx`

```typescript
function SkeletonBox({
  w, h, className = '',
}: {
  w?: string; h: string; className?: string;
}) {
  return (
    <div
      data-testid="skeleton-box"
      className={`rounded-xl animate-pulse ${className}`}
      style={{ width: w, height: h, background: 'rgba(255,255,255,0.06)' }}
    />
  );
}

export function DashboardSkeleton() {
  return (
    <div
      className="flex-1 overflow-y-auto p-6"
      data-testid="dashboard-skeleton"
      style={{ background: '#0B1020' }}
    >
      {/* Header */}
      <div className="flex items-center justify-between mb-6">
        <SkeletonBox h="28px" w="300px" />
        <SkeletonBox h="36px" w="200px" />
      </div>

      {/* KPI Row skeleton (6 cards) */}
      <div className="grid grid-cols-6 gap-4 mb-6">
        {Array.from({ length: 6 }).map((_, i) => (
          <div
            key={i}
            className="rounded-2xl p-5"
            style={{
              background: '#151B2F',
              border: '1px solid rgba(255,255,255,0.07)',
            }}
          >
            <SkeletonBox h="36px" w="36px" className="mb-4 rounded-xl" />
            <SkeletonBox h="24px" w="70px" className="mb-2" />
            <SkeletonBox h="12px" w="50px" />
          </div>
        ))}
      </div>

      {/* Charts row skeleton */}
      <div className="grid grid-cols-3 gap-4 mb-6">
        <SkeletonBox h="280px" className="col-span-2 rounded-2xl" />
        <SkeletonBox h="280px" className="rounded-2xl" />
      </div>

      {/* Bottom row skeleton */}
      <div className="grid grid-cols-2 gap-4">
        <SkeletonBox h="300px" className="rounded-2xl" />
        <SkeletonBox h="300px" className="rounded-2xl" />
      </div>
    </div>
  );
}
```

### File 5: `ui/src/app/components/Dashboard.tsx` (MODIFY)

Cần **xóa** tất cả 7 hardcoded data arrays và **thay** bằng `useDashboardMetrics` hook. Chỉ thay phần data, giữ nguyên UI/JSX:

```typescript
// ✅ Thêm imports
import { useState } from 'react';
import { useDashboardMetrics } from '@/features/dashboard/hooks/useDashboardMetrics';
import { QueryBoundary } from '@/shared/components/feedback/QueryBoundary';
import { DashboardSkeleton } from '@/features/dashboard/components/DashboardSkeleton';

// ❌ XÓA: 7 hardcoded arrays (khoảng dòng 11-130)
// const riskTrendData = [...]; → XÓA
// const severityData = [...];  → XÓA
// const productData = [...];   → XÓA
// const recentFindings = [...]; → XÓA
// const kevAlerts = [...];      → XÓA
// const recentScans = [...];    → XÓA
// const slaBreaches = [...];    → XÓA

export function Dashboard() {
  const [period, setPeriod] = useState<'30d' | '90d' | '1y'>('30d');
  const metricsQuery = useDashboardMetrics(period);

  return (
    <QueryBoundary
      query={metricsQuery}
      skeleton={<DashboardSkeleton />}
    >
      {(data) => (
        <DashboardContent
          data={data}
          period={period}
          onPeriodChange={setPeriod}
        />
      )}
    </QueryBoundary>
  );
}

// DashboardContent nhận data từ API — KHÔNG có hardcode
function DashboardContent({
  data,
  period,
  onPeriodChange,
}: {
  data: import('@/features/dashboard/types').DashboardData;
  period: '30d' | '90d' | '1y';
  onPeriodChange: (p: '30d' | '90d' | '1y') => void;
}) {
  // Giữ nguyên toàn bộ JSX/UI hiện tại
  // Chỉ thay tên biến cũ → data.xxx:
  //   riskTrendData     → data.riskTrend
  //   severityData      → data.severityDistribution (cần transform cho Recharts nếu cần)
  //   productData       → data.productGrades
  //   recentFindings    → data.slaBreaches (hoặc tạo endpoint riêng)
  //   kevAlerts         → data.kevAlerts
  //   recentScans       → data.recentScans
  //   slaBreaches       → data.slaBreaches
  //
  //   stats.critical    → data.kpis.criticalFindings
  //   stats.high        → data.kpis.highFindings
  //   stats.totalAssets → data.kpis.totalAssets
  //   ...etc

  return (
    <div
      className="flex-1 overflow-y-auto p-6"
      style={{ background: '#0B1020' }}
    >
      {/* Giữ nguyên toàn bộ JSX — chỉ thay data source */}
      {/* ... existing Dashboard JSX ... */}
    </div>
  );
}
```

> [!IMPORTANT]
> Khi migrate, đọc kỹ `Dashboard.tsx` để map chính xác từng biến cũ sang `data.xxx`. Cấu trúc fixture trong `dashboard.fixture.ts` phải khớp với cấu trúc `DashboardData` type.

---

## Verification

```bash
cd ui/

# Verify types
npx tsc --noEmit

# Start dev với MSW
VITE_ENABLE_MSW=true pnpm dev

# Truy cập http://localhost:3000/dashboard
# Kiểm tra:
# 1. Skeleton hiện trước khi data load
# 2. KPI cards hiện data từ MSW fixture (e.g., criticalFindings: 245)
# 3. Charts render đúng
# 4. Period picker (30d/90d/1y) trigger refetch

# Verify không còn hardcode
grep -n "const riskTrendData\|const severityData\|const productData\|const recentFindings\|const kevAlerts" \
  src/app/components/Dashboard.tsx
# Expected: No output (đã xóa hết)
```

---

- [x] `src/features/dashboard/types.ts` — đầy đủ 7 types
- [x] `src/features/dashboard/api/dashboardApi.ts` — `getMetrics(period)`
- [x] `src/features/dashboard/hooks/useDashboardMetrics.ts` — React Query hook, refetchInterval: 60s
- [x] `src/features/dashboard/components/DashboardSkeleton.tsx` — skeleton với `data-testid`
- [x] `Dashboard.tsx`: XÓA tất cả 7 hardcoded arrays ✅ Done (không còn hardcode)
- [x] `Dashboard.tsx`: import `useDashboardMetrics` + `QueryBoundary` + `DashboardSkeleton` ✅ Done
- [x] `Dashboard.tsx`: wrap với `QueryBoundary` ✅ Done
- [x] `Dashboard.tsx`: map tất cả UI elements sang `data.xxx` ✅ Done
- [x] Period picker (30d/90d/1y) hoạt động và trigger refetch ✅ Done
- [x] `grep` cho hardcode → no output ✅ Done
- [x] Dashboard render đúng với MSW fixture data ✅ Done
