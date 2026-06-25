# TASK-API-004 — Dashboard Module: API + Hooks + MSW + SSE Notifications

| Field | Value |
|-------|-------|
| **Task ID** | TASK-API-004 |
| **Module** | `ui/src/features/dashboard/`, `ui/src/features/notifications/` |
| **Solution Ref** | [SOL-UI-002](../solutions/SOL-UI-002-dashboard-api.md) |
| **Priority** | 🔴 P0 |
| **Depends On** | TASK-API-003 |
| **Estimated** | 2h |

---

## Context

Dashboard là màn hình đầu tiên sau login — cần load nhanh (<500ms) và auto-refresh. Hiện tại Dashboard.tsx có hardcode data (7 arrays). Task này:
1. Mở rộng `features/dashboard/` đã tạo ở init phase với types mới theo API response
2. Thêm `useSLADashboard` hook
3. Tạo SSE notifications module (`features/notifications/`)
4. Cập nhật MSW handlers

---

## Goal

- Dashboard API + hooks hoàn chỉnh theo CR-UI-002 response schema
- SSE notification hook kết nối `GET /api/v1/notifications/stream`
- MSW handlers cho dashboard + SLA + SSE stream

---

## Target Files

| Action | File Path |
|--------|-----------|
| MODIFY | `ui/src/features/dashboard/types.ts` (cập nhật theo API schema) |
| MODIFY | `ui/src/features/dashboard/api/dashboardApi.ts` |
| MODIFY | `ui/src/features/dashboard/hooks/useDashboardMetrics.ts` |
| CREATE | `ui/src/features/dashboard/hooks/useSLADashboard.ts` |
| CREATE | `ui/src/features/notifications/store/notificationStore.ts` |
| CREATE | `ui/src/features/notifications/hooks/useNotificationsSSE.ts` |
| CREATE | `ui/src/mocks/fixtures/dashboard.fixture.ts` |
| CREATE | `ui/src/mocks/handlers/dashboard.handlers.ts` |

---

## Implementation

### File 1: `ui/src/features/dashboard/types.ts` (MODIFY — cập nhật field names theo API)

```typescript
// API trả về snake_case — map đúng vào TypeScript
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

export interface RecentScan {
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
  severity_distribution: SeverityDistribution;
  product_grades: ProductGradeItem[];
  kev_alerts: KEVAlert[];
  recent_scans: RecentScan[];
  sla_breaches: SLABreach[];
}

export interface SLASummary {
  total_active_findings: number;
  compliance_percent: number;
  breached: number;
  at_risk: number;
  ok: number;
}

export interface SLADashboardData {
  summary: SLASummary;
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

### File 2: `ui/src/features/dashboard/api/dashboardApi.ts` (MODIFY)

```typescript
import { apiClient } from '@/shared/api/client';
import { ENDPOINTS } from '@/shared/api/endpoints';
import type { DashboardData, SLADashboardData } from '../types';

export const dashboardApi = {
  // GET /api/v1/dashboard?period=30d|90d|1y
  getMetrics: async (period: '30d' | '90d' | '1y' = '30d'): Promise<DashboardData> => {
    const { data } = await apiClient.get<DashboardData>(ENDPOINTS.dashboard.metrics, {
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
    const { data } = await apiClient.get<SLADashboardData>(ENDPOINTS.dashboard.sla, { params });
    return data;
  },
};
```

### File 3: `ui/src/features/dashboard/hooks/useDashboardMetrics.ts` (MODIFY)

```typescript
import { useQuery } from '@tanstack/react-query';
import { dashboardApi } from '../api/dashboardApi';
import type { DashboardData } from '../types';

export const dashboardKeys = {
  all:     ['dashboard'] as const,
  metrics: (period: string) => ['dashboard', 'metrics', period] as const,
  sla:     (params: object) => ['dashboard', 'sla', params] as const,
};

export function useDashboardMetrics(period: '30d' | '90d' | '1y' = '30d') {
  return useQuery<DashboardData>({
    queryKey: dashboardKeys.metrics(period),
    queryFn:  () => dashboardApi.getMetrics(period),
    staleTime:       60_000,
    refetchInterval: 60_000,   // Auto-refresh mỗi 60s
    retry: 1,
  });
}
```

### File 4: `ui/src/features/dashboard/hooks/useSLADashboard.ts` (CREATE)

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
    page:       params.page ?? 1,
    page_size:  params.pageSize ?? 20,
  };

  return useQuery({
    queryKey: dashboardKeys.sla(queryParams),
    queryFn:  () => dashboardApi.getSLADetail(queryParams),
    staleTime: 30_000,
  });
}
```

### File 5: `ui/src/features/notifications/store/notificationStore.ts` (CREATE)

```typescript
import { create } from 'zustand';

export interface NotificationEvent {
  type: string;
  title: string;
  severity?: string;
  entity_id?: string;
  timestamp: string;
}

interface NotificationState {
  notifications: NotificationEvent[];
  unreadCount: number;
  addNotification: (n: NotificationEvent) => void;
  markAllRead:     () => void;
  clearAll:        () => void;
}

export const useNotificationStore = create<NotificationState>((set) => ({
  notifications: [],
  unreadCount:   0,

  addNotification: (notification) =>
    set((state) => ({
      notifications: [notification, ...state.notifications].slice(0, 50),
      unreadCount:   state.unreadCount + 1,
    })),

  markAllRead: () => set({ unreadCount: 0 }),
  clearAll:    () => set({ notifications: [], unreadCount: 0 }),
}));
```

### File 6: `ui/src/features/notifications/hooks/useNotificationsSSE.ts` (CREATE)

```typescript
import { useEffect, useRef } from 'react';
import { useAuthStore } from '@/features/auth/store/authStore';
import { useNotificationStore } from '../store/notificationStore';
import toast from 'react-hot-toast';
import type { NotificationEvent } from '../store/notificationStore';

export type SSEStatus = 'connecting' | 'open' | 'closed' | 'error';

export function useNotificationsSSE() {
  const { accessToken, isAuthenticated } = useAuthStore();
  const { addNotification } = useNotificationStore();
  const sourceRef = useRef<EventSource | null>(null);
  const statusRef = useRef<SSEStatus>('closed');

  useEffect(() => {
    if (!isAuthenticated || !accessToken) return;

    // SSE không hỗ trợ Authorization header → dùng ?token= query param
    const url = `/api/v1/notifications/stream?token=${encodeURIComponent(accessToken)}`;
    const source = new EventSource(url, { withCredentials: true });
    sourceRef.current = source;
    statusRef.current = 'connecting';

    source.onopen = () => { statusRef.current = 'open'; };

    source.addEventListener('notification', (e: MessageEvent) => {
      const event = JSON.parse(e.data) as NotificationEvent;
      addNotification(event);

      // Toast cho events quan trọng
      if (event.type === 'finding.sla.breached' || event.type === 'kev.new') {
        toast.error(event.title, { duration: 8000, icon: '🚨' });
      } else if (event.type === 'scan.completed') {
        toast.success(event.title, { duration: 5000, icon: '✅' });
      }
    });

    source.addEventListener('ping', () => { /* keep-alive — no-op */ });

    source.onerror = () => {
      statusRef.current = 'error';
      source.close();
    };

    return () => {
      source.close();
      statusRef.current = 'closed';
    };
  }, [isAuthenticated, accessToken, addNotification]);

  return {
    isConnected: statusRef.current === 'open',
  };
}
```

### File 7: `ui/src/mocks/fixtures/dashboard.fixture.ts` (CREATE)

```typescript
import type { DashboardData, SLADashboardData } from '@/features/dashboard/types';

// Fixtures — CHỈ dùng trong MSW handlers, KHÔNG import vào components
export const dashboardFixture: DashboardData = {
  kpis: {
    critical_findings: 12, high_findings: 47, total_assets: 284,
    high_risk_assets: 18, active_scans: 2, queued_scans: 1,
    security_grade: 'C', security_score: 58,
    sla_compliance: 82.5, sla_at_risk: 8, sla_breached: 3,
  },
  risk_trend: [
    { month: 'Jan', critical: 5,  high: 28, medium: 64,  low: 112 },
    { month: 'Feb', critical: 8,  high: 35, medium: 71,  low: 98  },
    { month: 'Mar', critical: 10, high: 41, medium: 68,  low: 105 },
    { month: 'Apr', critical: 7,  high: 38, medium: 72,  low: 118 },
    { month: 'May', critical: 9,  high: 44, medium: 75,  low: 123 },
    { month: 'Jun', critical: 12, high: 47, medium: 80,  low: 130 },
  ],
  severity_distribution: { critical: 12, high: 47, medium: 80, low: 130, total: 269 },
  product_grades: [
    { id: 'prod_1', name: 'Banking Portal', grade: 'D', score: 42, critical_count: 2, high_count: 8 },
    { id: 'prod_2', name: 'Mobile App',     grade: 'B', score: 76, critical_count: 0, high_count: 4 },
    { id: 'prod_3', name: 'Internal API',   grade: 'C', score: 61, critical_count: 1, high_count: 6 },
  ],
  kev_alerts: [
    { cve_id: 'CVE-2026-12345', vendor: 'Apache', product: 'Struts',
      date_added: '2026-06-10', is_ransomware: true },
  ],
  recent_scans: [
    { id: 'sc_001', name: 'Weekly Network Scan', type: 'nmap_full',
      status: 'completed', targets: ['10.0.0.0/24'], finding_count: 23,
      started_at: '2026-06-16T08:00:00Z', completed_at: '2026-06-16T08:04:32Z',
      duration_ms: 272000, created_by: 'bob@company.com' },
  ],
  sla_breaches: [
    { finding_id: 'F-2801', title: 'Apache Struts RCE', cve_id: 'CVE-2026-12345',
      severity: 'Critical', product_name: 'Banking Portal',
      sla_expiration_date: '2026-06-09', days_overdue: 7 },
  ],
};

export const slaFixture: SLADashboardData = {
  summary: {
    total_active_findings: 269, compliance_percent: 82.5,
    breached: 3, at_risk: 8, ok: 258,
  },
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
```

### File 8: `ui/src/mocks/handlers/dashboard.handlers.ts` (CREATE)

```typescript
import { http, HttpResponse } from 'msw';
import { ENDPOINTS } from '@/shared/api/endpoints';
import { dashboardFixture, slaFixture } from '../fixtures/dashboard.fixture';

export const dashboardHandlers = [
  // GET /api/v1/dashboard
  http.get(ENDPOINTS.dashboard.metrics, () => {
    return HttpResponse.json(dashboardFixture);
  }),

  // GET /api/v1/dashboard/sla
  http.get(ENDPOINTS.dashboard.sla, () => {
    return HttpResponse.json(slaFixture);
  }),

  // GET /api/v1/notifications/stream — SSE
  http.get(ENDPOINTS.notifications.stream, () => {
    const encoder = new TextEncoder();

    const stream = new ReadableStream({
      async start(controller) {
        // Initial ping
        controller.enqueue(encoder.encode(
          `event: ping\ndata: {"ts":"${new Date().toISOString()}"}\n\n`
        ));

        // Mock KEV alert sau 2s
        await new Promise(r => setTimeout(r, 2000));
        const notif = JSON.stringify({
          type: 'kev.new',
          title: 'New KEV: Apache Struts RCE (CVE-2026-12345)',
          entity_id: 'CVE-2026-12345',
          timestamp: new Date().toISOString(),
        });
        controller.enqueue(encoder.encode(`event: notification\ndata: ${notif}\n\n`));

        // Keep-alive ping mỗi 30s
        const interval = setInterval(() => {
          controller.enqueue(encoder.encode(
            `event: ping\ndata: {"ts":"${new Date().toISOString()}"}\n\n`
          ));
        }, 30_000);

        // Cleanup khi client disconnect
        return () => clearInterval(interval);
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

## Verification

```bash
cd ui/
VITE_ENABLE_MSW=true pnpm dev

# 1. Truy cập /dashboard
# 2. KPI cards phải hiển thị: Critical: 12, High: 47, Total Assets: 284
# 3. Security Grade: C (score 58)
# 4. Risk Trend chart có 6 tháng dữ liệu
# 5. Sau 2s: toast notification "New KEV: Apache Struts RCE"
# 6. Auto-refresh sau 60s (kiểm tra Network tab)

npx tsc --noEmit
# Expected: no errors

# Verify không hardcode KPI values trong Dashboard.tsx
grep -n "critical_findings\|12.*47\|245\|284" src/app/components/Dashboard.tsx
# Expected: chỉ thấy prop names, không có literal numbers
```

---

## Checklist

- [ ] `features/dashboard/types.ts` — snake_case fields khớp API response
- [ ] `features/dashboard/api/dashboardApi.ts` — `getMetrics()` + `getSLADetail()`
- [ ] `features/dashboard/hooks/useDashboardMetrics.ts` — `refetchInterval: 60_000`
- [ ] `features/dashboard/hooks/useSLADashboard.ts` — created
- [ ] `features/notifications/store/notificationStore.ts` — Zustand store (không persist)
- [ ] `features/notifications/hooks/useNotificationsSSE.ts` — SSE với `?token=` auth
- [ ] SSE hook: `kev.new` + `finding.sla.breached` → `toast.error`; `scan.completed` → `toast.success`
- [ ] `mocks/fixtures/dashboard.fixture.ts` — realistic data
- [ ] `mocks/handlers/dashboard.handlers.ts` — 3 handlers dùng `ENDPOINTS.dashboard.*`
- [ ] SSE handler mock: ping sau 0s, notification sau 2s, ping mỗi 30s
- [ ] `npx tsc --noEmit` không lỗi
