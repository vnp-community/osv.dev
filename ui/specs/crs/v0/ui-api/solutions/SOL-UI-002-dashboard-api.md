# SOL-UI-002 — Frontend Solution: Dashboard & KPI API

**CR nguồn:** [CR-UI-002](../../../../../specs/crs/v0/ui-api/CR-UI-002-dashboard-api.md)  
**Ngày tạo:** 2026-06-16  
**Trạng thái:** Proposed  
**Ưu tiên:** P0 — Critical (màn hình đầu tiên sau login)  
**Phạm vi:** Frontend React SPA (`ui/src/features/dashboard/`)

---

## 1. Tóm tắt giải pháp

CR-UI-002 yêu cầu:
1. `GET /api/v1/dashboard` — BFF aggregate endpoint, response trong <500ms
2. `GET /api/v1/dashboard/sla` — SLA detail với pagination
3. `GET /api/v1/notifications/stream` — SSE cho in-app notifications (bell icon + toast)

Frontend cần:
- `dashboardApi.ts` — API calls
- `useDashboardMetrics()` hook — React Query với auto-refresh 60s
- `useSLADashboard()` hook
- `useNotificationsSSE()` hook — SSE connection cho notification bell
- MSW handlers với realistic fixture data

---

## 2. File Structure

```
ui/src/
├── features/dashboard/
│   ├── api/
│   │   └── dashboardApi.ts
│   ├── hooks/
│   │   ├── useDashboardMetrics.ts
│   │   └── useSLADashboard.ts
│   ├── components/
│   │   ├── Dashboard.tsx           # Main executive dashboard
│   │   ├── KPICard.tsx
│   │   ├── RiskTrendChart.tsx      # Recharts AreaChart
│   │   ├── SeverityDonut.tsx       # Recharts PieChart
│   │   ├── ProductGradesList.tsx
│   │   ├── KEVAlerts.tsx
│   │   ├── RecentScans.tsx
│   │   └── SLADashboard.tsx
│   └── types.ts
│
├── features/notifications/
│   ├── hooks/
│   │   └── useNotificationsSSE.ts  # SSE connection
│   └── store/
│       └── notificationStore.ts    # Zustand notifications state
│
└── mocks/handlers/
    └── dashboard.handlers.ts
```

---

## 3. Implementation Chi Tiết

### 3.1 `features/dashboard/api/dashboardApi.ts`

```typescript
import apiClient from '@/shared/api/client';
import type { DashboardData, SLADashboardData } from './types';

export const dashboardApi = {
  // GET /api/v1/dashboard?period=30d|90d|1y
  getMetrics: async (period: '30d' | '90d' | '1y' = '30d'): Promise<DashboardData> => {
    const { data } = await apiClient.get<DashboardData>('/api/v1/dashboard', {
      params: { period },
    });
    return data;
  },

  // GET /api/v1/dashboard/sla
  getSLADetail: async (params: {
    product_id?: string;
    page?: number;
    page_size?: number;
  } = {}): Promise<SLADashboardData> => {
    const { data } = await apiClient.get<SLADashboardData>('/api/v1/dashboard/sla', {
      params,
    });
    return data;
  },
};
```

### 3.2 `features/dashboard/hooks/useDashboardMetrics.ts`

```typescript
import { useQuery } from '@tanstack/react-query';
import { dashboardApi } from '../api/dashboardApi';
import type { DashboardData } from '../types';

// Query key factory
export const dashboardKeys = {
  all: ['dashboard'] as const,
  metrics: (period: string) => [...dashboardKeys.all, 'metrics', period] as const,
  sla: (params: object) => [...dashboardKeys.all, 'sla', params] as const,
};

export function useDashboardMetrics(period: '30d' | '90d' | '1y' = '30d') {
  return useQuery<DashboardData>({
    queryKey: dashboardKeys.metrics(period),
    queryFn: () => dashboardApi.getMetrics(period),
    staleTime: 60_000,          // 1 phút — dashboard data is relatively fresh
    refetchInterval: 60_000,    // Auto-refresh mỗi 60s (đồng bộ với cache TTL backend)
    retry: 1,
  });
}
```

### 3.3 `features/dashboard/hooks/useSLADashboard.ts`

```typescript
import { useQuery } from '@tanstack/react-query';
import { dashboardApi } from '../api/dashboardApi';
import { dashboardKeys } from './useDashboardMetrics';

export function useSLADashboard(params: {
  productId?: string;
  page?: number;
  pageSize?: number;
} = {}) {
  const queryParams = {
    product_id: params.productId,
    page: params.page ?? 1,
    page_size: params.pageSize ?? 20,
  };

  return useQuery({
    queryKey: dashboardKeys.sla(queryParams),
    queryFn: () => dashboardApi.getSLADetail(queryParams),
    staleTime: 30_000,
  });
}
```

### 3.4 `features/notifications/hooks/useNotificationsSSE.ts`

```typescript
import { useEffect, useRef } from 'react';
import { useAuthStore } from '@/features/auth/store/authStore';
import { useNotificationStore } from '../store/notificationStore';
import toast from 'react-hot-toast';

export type NotificationType =
  | 'finding.created'
  | 'finding.sla.breached'
  | 'finding.status.changed'
  | 'kev.new'
  | 'risk_acceptance.expired'
  | 'scan.completed'
  | 'ping';

export interface NotificationEvent {
  type: NotificationType;
  title: string;
  severity?: string;
  entity_id?: string;
  timestamp: string;
}

export function useNotificationsSSE() {
  const { accessToken, isAuthenticated } = useAuthStore();
  const { addNotification } = useNotificationStore();
  const sourceRef = useRef<EventSource | null>(null);

  useEffect(() => {
    if (!isAuthenticated || !accessToken) return;

    // SSE không hỗ trợ Authorization header → dùng query param ?token=
    // (Workaround đã được backend thiết kế hỗ trợ)
    const url = `/api/v1/notifications/stream?token=${encodeURIComponent(accessToken)}`;
    const source = new EventSource(url, { withCredentials: true });
    sourceRef.current = source;

    source.addEventListener('notification', (e) => {
      const event = JSON.parse(e.data) as NotificationEvent;
      addNotification(event);

      // Show toast cho events quan trọng
      if (event.type === 'finding.sla.breached' || event.type === 'kev.new') {
        toast.error(event.title, { duration: 8000, icon: '🚨' });
      } else if (event.type === 'scan.completed') {
        toast.success(event.title, { duration: 5000, icon: '✅' });
      }
    });

    source.addEventListener('ping', () => {
      // Keep-alive — no action needed
    });

    source.onerror = () => {
      source.close();
      // Reconnect sau 5 giây
      setTimeout(() => {
        sourceRef.current = null;
      }, 5000);
    };

    return () => {
      source.close();
      sourceRef.current = null;
    };
  }, [isAuthenticated, accessToken]);

  return {
    isConnected: sourceRef.current?.readyState === EventSource.OPEN,
  };
}
```

### 3.5 `features/notifications/store/notificationStore.ts`

```typescript
import { create } from 'zustand';
import type { NotificationEvent } from '../hooks/useNotificationsSSE';

interface NotificationState {
  notifications: NotificationEvent[];
  unreadCount: number;
  addNotification: (n: NotificationEvent) => void;
  markAllRead: () => void;
  clearAll: () => void;
}

export const useNotificationStore = create<NotificationState>((set) => ({
  notifications: [],
  unreadCount: 0,

  addNotification: (notification) => set((state) => ({
    notifications: [notification, ...state.notifications].slice(0, 50), // Giữ max 50
    unreadCount: state.unreadCount + 1,
  })),

  markAllRead: () => set({ unreadCount: 0 }),

  clearAll: () => set({ notifications: [], unreadCount: 0 }),
}));
```

### 3.6 `features/dashboard/types.ts`

```typescript
export type SecurityGrade = 'A' | 'A-' | 'B+' | 'B' | 'B-' | 'C+' | 'C' | 'D' | 'F';

export interface DashboardKPIs {
  critical_findings: number;
  high_findings: number;
  total_assets: number;
  high_risk_assets: number;
  active_scans: number;
  queued_scans: number;
  security_grade: SecurityGrade;
  security_score: number;       // 0-100
  sla_compliance: number;       // percentage
  sla_at_risk: number;
  sla_breached: number;
}

export interface RiskTrendPoint {
  month: string;
  critical: number;
  high: number;
  medium: number;
  low: number;
}

export interface ProductGrade {
  id: string;
  name: string;
  grade: string;
  score: number;
  critical_count: number;
  high_count: number;
}

export interface KEVAlert {
  cve_id: string;
  vendor: string;
  product: string;
  date_added: string;
  is_ransomware: boolean;
}

export interface SLABreach {
  finding_id: string;
  title: string;
  cve_id: string;
  severity: string;
  product_name: string;
  sla_expiration_date: string;
  days_overdue: number;
}

export interface DashboardData {
  kpis: DashboardKPIs;
  risk_trend: RiskTrendPoint[];
  severity_distribution: {
    critical: number;
    high: number;
    medium: number;
    low: number;
    total: number;
  };
  product_grades: ProductGrade[];
  kev_alerts: KEVAlert[];
  recent_scans: Array<{
    id: string;
    name: string;
    type: string;
    status: string;
    targets: string[];
    finding_count: number;
    started_at: string;
    completed_at: string | null;
    duration_ms: number | null;
    created_by: string;
  }>;
  sla_breaches: SLABreach[];
}

export interface SLADashboardData {
  summary: {
    total_active_findings: number;
    compliance_percent: number;
    breached: number;
    at_risk: number;
    ok: number;
  };
  compliance_trend: Array<{ month: string; compliance_percent: number }>;
  breached_findings: SLABreach[];
  at_risk_findings: Array<{
    finding_id: string;
    title: string;
    severity: string;
    product_name: string;
    sla_expiration_date: string;
    hours_remaining: number;
  }>;
  by_product: Array<{
    product_id: string;
    product_name: string;
    compliance_percent: number;
    breached: number;
    at_risk: number;
    ok: number;
  }>;
  total_breached: number;
  total_at_risk: number;
  page: number;
  page_size: number;
}
```

### 3.7 MSW Handler: `mocks/handlers/dashboard.handlers.ts`

```typescript
import { http, HttpResponse } from 'msw';
import type { DashboardData, SLADashboardData } from '@/features/dashboard/types';

const dashboardFixture: DashboardData = {
  kpis: {
    critical_findings: 12, high_findings: 47, total_assets: 284,
    high_risk_assets: 18, active_scans: 2, queued_scans: 1,
    security_grade: 'C', security_score: 58,
    sla_compliance: 82.5, sla_at_risk: 8, sla_breached: 3,
  },
  risk_trend: [
    { month: 'Jan', critical: 5, high: 28, medium: 64, low: 112 },
    { month: 'Feb', critical: 8, high: 35, medium: 71, low: 98 },
    { month: 'Mar', critical: 10, high: 41, medium: 68, low: 105 },
    { month: 'Apr', critical: 7, high: 38, medium: 72, low: 118 },
    { month: 'May', critical: 9, high: 44, medium: 75, low: 123 },
    { month: 'Jun', critical: 12, high: 47, medium: 80, low: 130 },
  ],
  severity_distribution: { critical: 12, high: 47, medium: 80, low: 130, total: 269 },
  product_grades: [
    { id: 'prod_1', name: 'Banking Portal', grade: 'D', score: 42, critical_count: 2, high_count: 8 },
    { id: 'prod_2', name: 'Mobile App', grade: 'B', score: 76, critical_count: 0, high_count: 4 },
  ],
  kev_alerts: [
    { cve_id: 'CVE-2026-12345', vendor: 'Apache', product: 'Struts', date_added: '2026-06-10', is_ransomware: true },
  ],
  recent_scans: [
    { id: 'sc_001', name: 'Weekly Network Scan', type: 'nmap_full', status: 'completed',
      targets: ['10.0.0.0/24'], finding_count: 23, started_at: '2026-06-16T08:00:00Z',
      completed_at: '2026-06-16T08:04:32Z', duration_ms: 272000, created_by: 'bob@company.com' },
  ],
  sla_breaches: [
    { finding_id: 'F-2801', title: 'Apache Struts RCE', cve_id: 'CVE-2026-12345',
      severity: 'Critical', product_name: 'Banking Portal',
      sla_expiration_date: '2026-06-09', days_overdue: 7 },
  ],
};

export const dashboardHandlers = [
  // GET /api/v1/dashboard
  http.get('/api/v1/dashboard', ({ request }) => {
    const url = new URL(request.url);
    const period = url.searchParams.get('period') || '30d';
    // Trả về fixture, có thể điều chỉnh theo period
    return HttpResponse.json(dashboardFixture);
  }),

  // GET /api/v1/dashboard/sla
  http.get('/api/v1/dashboard/sla', () => {
    const slaFixture: SLADashboardData = {
      summary: { total_active_findings: 269, compliance_percent: 82.5, breached: 3, at_risk: 8, ok: 258 },
      compliance_trend: [
        { month: 'Jan', compliance_percent: 91.0 },
        { month: 'Feb', compliance_percent: 88.5 },
        { month: 'Jun', compliance_percent: 82.5 },
      ],
      breached_findings: dashboardFixture.sla_breaches,
      at_risk_findings: [
        { finding_id: 'F-2845', title: 'Log4j JNDI Injection', severity: 'High',
          product_name: 'Mobile App', sla_expiration_date: '2026-06-17', hours_remaining: 18 },
      ],
      by_product: [
        { product_id: 'prod_1', product_name: 'Banking Portal',
          compliance_percent: 71.0, breached: 2, at_risk: 3, ok: 12 },
      ],
      total_breached: 3, total_at_risk: 8, page: 1, page_size: 20,
    };
    return HttpResponse.json(slaFixture);
  }),

  // GET /api/v1/notifications/stream — SSE mock
  http.get('/api/v1/notifications/stream', () => {
    const encoder = new TextEncoder();
    let count = 0;

    const stream = new ReadableStream({
      async start(controller) {
        // Gửi initial ping
        controller.enqueue(encoder.encode(`event: ping\ndata: {"ts":"${new Date().toISOString()}"}\n\n`));

        // Gửi 1 KEV alert sau 2s (mock)
        await new Promise(r => setTimeout(r, 2000));
        if (count < 1) {
          count++;
          const notif = JSON.stringify({
            type: 'kev.new',
            title: 'New KEV: Apache Struts RCE (CVE-2026-12345)',
            entity_id: 'CVE-2026-12345',
            timestamp: new Date().toISOString(),
          });
          controller.enqueue(encoder.encode(`event: notification\ndata: ${notif}\n\n`));
        }

        // Keep-alive ping mỗi 30s
        setInterval(() => {
          controller.enqueue(encoder.encode(`event: ping\ndata: {"ts":"${new Date().toISOString()}"}\n\n`));
        }, 30000);
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
];
```

---

## 4. Dashboard Component Pattern

```tsx
// features/dashboard/components/Dashboard.tsx
export function Dashboard() {
  const [period, setPeriod] = useState<'30d' | '90d' | '1y'>('30d');
  const { data, isLoading, isError, error, refetch } = useDashboardMetrics(period);

  if (isLoading) return <DashboardSkeleton />;
  if (isError) return <ErrorState message={error?.message} onRetry={refetch} />;
  if (!data) return <EmptyState title="No dashboard data" />;

  return (
    <div className="space-y-6">
      {/* Period Selector */}
      <PeriodSelector value={period} onChange={setPeriod} />

      {/* KPI Row */}
      <div className="grid grid-cols-6 gap-4">
        <KPICard label="Critical" value={data.kpis.critical_findings} trend="up" severity="critical" />
        <KPICard label="High" value={data.kpis.high_findings} trend="up" severity="high" />
        <KPICard label="Total Assets" value={data.kpis.total_assets} />
        <KPICard label="Active Scans" value={data.kpis.active_scans} />
        <GradeCircle grade={data.kpis.security_grade} score={data.kpis.security_score} />
        <KPICard label="SLA Compliance" value={`${data.kpis.sla_compliance}%`} />
      </div>

      {/* Charts Row */}
      <div className="grid grid-cols-3 gap-4">
        <RiskTrendChart data={data.risk_trend} className="col-span-2" />
        <SeverityDonut data={data.severity_distribution} />
      </div>

      {/* Bottom Row */}
      <div className="grid grid-cols-3 gap-4">
        <KEVAlerts alerts={data.kev_alerts} />
        <RecentScans scans={data.recent_scans} />
        <SLABreachesList breaches={data.sla_breaches} />
      </div>
    </div>
  );
}
```

---

## 5. React Query Cache Strategy

| Query | staleTime | refetchInterval | Lý do |
|-------|-----------|-----------------|-------|
| `useDashboardMetrics` | 60s | 60s | Dashboard auto-refresh |
| `useSLADashboard` | 30s | — | Sau mutation invalidate |

---

## 6. Acceptance Criteria (Frontend)

- [ ] Dashboard load lần đầu từ `GET /api/v1/dashboard` — không hardcode data
- [ ] Period selector (30d/90d/1y) thay đổi → refetch với `?period=` param
- [ ] Dashboard auto-refresh mỗi 60 giây (`refetchInterval: 60_000`)
- [ ] Skeleton hiển thị khi `isLoading=true`
- [ ] Error state với retry button khi `isError=true`
- [ ] `KPICard` chỉ hiển thị data từ `data.kpis.*` — không hardcode số
- [ ] `RiskTrendChart` dùng `data.risk_trend` từ API
- [ ] `useNotificationsSSE` kết nối SSE, show toast khi `kev.new` hoặc `sla.breached`
- [ ] Bell icon badge hiển thị `unreadCount` từ Zustand store
- [ ] MSW handler trả về fixture data đầy đủ cấu trúc

---

## 7. Performance Notes

- Dashboard response < 500ms (yêu cầu backend BFF fan-out)
- React Query cache giúp instant load khi navigate back to dashboard
- `staleTime: 60_000` tránh refetch không cần thiết khi switch tabs
- Skeleton UI tránh layout shift (CLS) khi loading
