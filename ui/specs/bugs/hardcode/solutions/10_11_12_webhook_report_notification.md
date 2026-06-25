# Solution 10 — WebhookEvents.tsx + 11 — ReportCenter.tsx + 12 — NotificationCenter.tsx

---

## Solution 10: WebhookEvents.tsx

### Vấn đề
- `DELIVERY_HISTORY = [...]` — delivery log hardcode
- `ACTIVITY_CHART = [...]` — chart data tĩnh

### Hook mở rộng: `useWebhookDeliveries`

```typescript
// features/integrations/hooks/useWebhookDeliveries.ts
import { useQuery, useMutation } from '@tanstack/react-query';
import { apiClient } from '@/shared/api/client';

export interface WebhookDelivery {
  id: string;
  webhookId: string;
  event: string;
  endpoint: string;
  status: 'success' | 'failed' | 'retried';
  responseTime: number;
  statusCode: number;
  time: string;
  requestBody?: string;
  responseBody?: string;
}

export interface WebhookDeliveriesResponse {
  deliveries: WebhookDelivery[];
  total: number;
}

export interface WebhookHourlyStats {
  h: string;   // "06:00", "09:00"
  success: number;
  failed: number;
}

export function useWebhookDeliveries(webhookId?: string, params?: { page?: number }) {
  return useQuery<WebhookDeliveriesResponse>({
    queryKey: ['webhooks', 'deliveries', webhookId, params],
    queryFn: async () => {
      const { data } = await apiClient.get<WebhookDeliveriesResponse>(
        '/api/v1/webhooks/deliveries',
        { params: { webhookId, ...params } }
      );
      return data;
    },
    staleTime: 30_000,
    refetchInterval: 30_000,  // Auto-refresh
  });
}

export function useWebhookHourlyStats() {
  return useQuery<WebhookHourlyStats[]>({
    queryKey: ['webhooks', 'stats', 'hourly'],
    queryFn: async () => {
      const { data } = await apiClient.get<WebhookHourlyStats[]>('/api/v1/webhooks/stats/hourly');
      return data;
    },
    staleTime: 5 * 60_000,
  });
}

export function useRetryDelivery() {
  return useMutation({
    mutationFn: (deliveryId: string) =>
      apiClient.post(`/api/v1/webhooks/deliveries/${deliveryId}/retry`),
  });
}
```

### Thay đổi trong WebhookEvents.tsx

```typescript
// Xóa:
// const DELIVERY_HISTORY = [...]
// const ACTIVITY_CHART = [...]

// Thêm imports:
import { useWebhookDeliveries, useWebhookHourlyStats, useRetryDelivery } from '../hooks/useWebhookDeliveries';

// Trong component:
const deliveriesQuery = useWebhookDeliveries(selected?.id);
const hourlyStatsQuery = useWebhookHourlyStats();
const retryDelivery = useRetryDelivery();

// Chart data từ server:
// data={hourlyStatsQuery.data ?? []}

// Table data từ server:
// {deliveriesQuery.data?.deliveries.map(d => ...)}
```

### Thêm endpoints.ts

```typescript
webhooks: {
  // ... existing
  deliveries:  '/api/v1/webhooks/deliveries',
  statsHourly: '/api/v1/webhooks/stats/hourly',
  retry:       (id: string) => `/api/v1/webhooks/deliveries/${id}/retry`,
},
```

### MSW Handler

```typescript
// Thêm vào integrations.handlers.ts
http.get('/api/v1/webhooks/deliveries', () => {
  return HttpResponse.json({
    deliveries: [
      { id: 'DEL-0441', webhookId: 'wh-1', event: 'finding.created', endpoint: 'siem.company.com', status: 'success', responseTime: 124, statusCode: 200, time: new Date(Date.now() - 120000).toISOString() },
      { id: 'DEL-0440', webhookId: 'wh-1', event: 'scan.completed', endpoint: 'siem.company.com', status: 'success', responseTime: 89, statusCode: 200, time: new Date(Date.now() - 7200000).toISOString() },
      { id: 'DEL-0439', webhookId: 'wh-2', event: 'sla.breached', endpoint: 'jira.company.com', status: 'failed', responseTime: 5001, statusCode: 503, time: new Date(Date.now() - 10800000).toISOString() },
      { id: 'DEL-0438', webhookId: 'wh-3', event: 'kev.alert', endpoint: 'slack.company.com', status: 'success', responseTime: 201, statusCode: 200, time: new Date(Date.now() - 14400000).toISOString() },
    ],
    total: 4,
  });
}),

http.get('/api/v1/webhooks/stats/hourly', () => {
  return HttpResponse.json([
    { h: '06:00', success: 42, failed: 1 },
    { h: '09:00', success: 87, failed: 2 },
    { h: '12:00', success: 65, failed: 0 },
    { h: '15:00', success: 93, failed: 3 },
    { h: '18:00', success: 78, failed: 1 },
    { h: '21:00', success: 45, failed: 0 },
  ]);
}),
```

---

## Solution 11: ReportCenter.tsx

### Vấn đề
- `const reports = [...]` — 5 báo cáo hardcode
- `const templates = [...]` — templates không load từ server
- Product filter options hardcode: `["All Products", "Banking App", ...]`
- Subtitle `"Last report 6h ago"` hardcode

### Hook mới: `useReports`

```typescript
// features/reports/hooks/useReports.ts
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { apiClient } from '@/shared/api/client';
import { ENDPOINTS } from '@/shared/api/endpoints';

export interface ReportRun {
  id: string;
  name?: string;
  type: 'Executive' | 'Technical' | 'Compliance';
  format: 'pdf' | 'html' | 'csv' | 'excel' | 'json';
  status: 'pending' | 'generating' | 'completed' | 'failed';
  findingCount?: number;
  fileSizeBytes?: number;
  generatedAt?: string;
  artifactUrl?: string;
  createdAt: string;
  createdBy: string;
}

export interface ReportsResponse {
  reports: ReportRun[];
  total: number;
  lastGeneratedAt?: string;  // ISO string của báo cáo gần nhất
}

export interface ReportTemplatesResponse {
  templates: Array<{
    id: string;
    name: string;
    description: string;
    type: 'Executive' | 'Technical' | 'Compliance';
  }>;
}

export function useReports() {
  return useQuery<ReportsResponse>({
    queryKey: ['reports', 'list'],
    queryFn: async () => {
      const { data } = await apiClient.get<ReportsResponse>(ENDPOINTS.reports.list);
      return data;
    },
    staleTime: 60_000,
  });
}

export function useReportTemplates() {
  return useQuery<ReportTemplatesResponse>({
    queryKey: ['reports', 'templates'],
    queryFn: async () => {
      const { data } = await apiClient.get<ReportTemplatesResponse>('/api/v1/reports/templates');
      return data;
    },
    staleTime: 10 * 60_000,  // 10 phút — templates ít thay đổi
  });
}

export function useCreateReport() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (req: { type: string; format: string; productId?: string; minSeverity?: string; dateRange?: string }) =>
      apiClient.post(ENDPOINTS.reports.create, req),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['reports'] });
    },
  });
}
```

### Thay đổi trong ReportCenter.tsx

```typescript
// Xóa:
// const templates = [...]
// const reports = [...]

// Sử dụng:
import { useReports, useReportTemplates, useCreateReport } from '../hooks/useReports';
import { useProducts } from '@/features/product-security/hooks/useProducts'; // Dùng lại hook sẵn có

// Trong component:
const reportsQuery = useReports();
const templatesQuery = useReportTemplates();
const productsQuery = useProducts();
const createReport = useCreateReport();

// Subtitle động:
// `${reportsQuery.data?.total} reports · Last report ${timeAgo(reportsQuery.data?.lastGeneratedAt)}`
```

### MSW Handler

```typescript
http.get('/api/v1/reports', () => {
  return HttpResponse.json({
    reports: [
      { id: 'R-047', name: 'Q2 2026 Executive Summary', type: 'Executive', format: 'pdf', status: 'completed', fileSizeBytes: 2516582, generatedAt: '2026-06-14T09:00:00Z', createdAt: '2026-06-14T09:00:00Z', createdBy: 'carol@company.com' },
      { id: 'R-046', name: 'Banking App Technical Report', type: 'Technical', format: 'pdf', status: 'completed', fileSizeBytes: 9123456, generatedAt: '2026-06-13T16:30:00Z', createdAt: '2026-06-13T16:30:00Z', createdBy: 'bob.chen@company.com' },
      { id: 'R-045', name: 'PCI DSS Compliance Q2', type: 'Compliance', format: 'pdf', status: 'completed', fileSizeBytes: 4300000, generatedAt: '2026-06-12T11:00:00Z', createdAt: '2026-06-12T11:00:00Z', createdBy: 'carol@company.com' },
    ],
    total: 3,
    lastGeneratedAt: '2026-06-14T09:00:00Z',
  });
}),

http.get('/api/v1/reports/templates', () => {
  return HttpResponse.json({
    templates: [
      { id: 'exec', name: 'Executive Summary', description: 'High-level overview for C-level presentations', type: 'Executive' },
      { id: 'tech', name: 'Technical Report', description: 'Detailed findings with CVE details and remediation', type: 'Technical' },
      { id: 'comp', name: 'Compliance Report', description: 'Mapped to PCI DSS, ISO 27001, SOC2, NIST', type: 'Compliance' },
    ],
  });
}),
```

---

## Solution 12: NotificationCenter.tsx

### Vấn đề
- `const notifications = [...]` — 10 notifications hardcode
- "Mark all read" button không có tác dụng
- Không real-time (không SSE)

### Hook mới: `useNotifications`

```typescript
// features/notifications/hooks/useNotifications.ts
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { useEffect, useRef } from 'react';
import { apiClient } from '@/shared/api/client';
import { ENDPOINTS } from '@/shared/api/endpoints';

export interface Notification {
  id: string;
  type: 'critical' | 'sla' | 'kev' | 'scan';
  title: string;
  description: string;
  product?: string;
  read: boolean;
  createdAt: string;  // ISO string
  timeAgo: string;    // Computed by server or client
  metadata?: Record<string, unknown>;
}

export interface NotificationsResponse {
  notifications: Notification[];
  total: number;
  unreadCount: number;
}

const notifKeys = {
  all: ['notifications'] as const,
  list: (params?: Record<string, unknown>) => [...notifKeys.all, 'list', params] as const,
};

export function useNotifications(params?: { type?: string; unreadOnly?: boolean }) {
  return useQuery<NotificationsResponse>({
    queryKey: notifKeys.list(params),
    queryFn: async () => {
      const { data } = await apiClient.get<NotificationsResponse>(
        ENDPOINTS.notifications.list,
        { params }
      );
      return data;
    },
    staleTime: 30_000,
    refetchInterval: 60_000,  // Poll mỗi 1 phút khi không có SSE
  });
}

export function useMarkAllRead() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: () => apiClient.post(ENDPOINTS.notifications.markAllRead),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: notifKeys.all });
    },
  });
}

export function useMarkRead() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (id: string) => apiClient.post(ENDPOINTS.notifications.markRead(id)),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: notifKeys.all });
    },
  });
}

// SSE real-time hook — notifications push từ server
export function useNotificationSSE() {
  const queryClient = useQueryClient();
  const sourceRef = useRef<EventSource | null>(null);

  useEffect(() => {
    if (sourceRef.current) sourceRef.current.close();

    const source = new EventSource(ENDPOINTS.notifications.stream, { withCredentials: true });

    source.onmessage = () => {
      // Invalidate để refetch notifications
      queryClient.invalidateQueries({ queryKey: notifKeys.all });
    };

    source.onerror = () => source.close();
    sourceRef.current = source;

    return () => source.close();
  }, [queryClient]);
}
```

### Thay đổi trong NotificationCenter.tsx

```typescript
// Xóa:
// const notifications = [...]
// const CATEGORIES = [...]   // Giữ lại — đây là UI config, không phải data

// Thêm:
import { useNotifications, useMarkAllRead, useMarkRead, useNotificationSSE } from '../hooks/useNotifications';

export function NotificationCenter() {
  const [filter, setFilter] = useState('all');
  const [showUnreadOnly, setShowUnreadOnly] = useState(false);

  // ✅ Server data + real-time
  const notifQuery = useNotifications({
    type: filter !== 'all' ? filter : undefined,
    unreadOnly: showUnreadOnly || undefined,
  });
  const markAllRead = useMarkAllRead();
  const markRead = useMarkRead();

  // Kết nối SSE cho real-time push
  useNotificationSSE();

  // ...render với QueryBoundary
  // unreadCount = notifQuery.data?.unreadCount
}
```

### MSW Handler

```typescript
// src/mocks/handlers/notifications.handlers.ts
import { http, HttpResponse } from 'msw';

const notificationsFixture = [
  { id: 'n-1', type: 'critical', title: 'Critical Finding Detected', description: 'CVE-2025-44228 found on webserver01.prod — CVSS 10.0, KEV active', product: 'Banking App', read: false, createdAt: new Date(Date.now() - 10 * 60000).toISOString(), timeAgo: '10 min ago' },
  { id: 'n-2', type: 'sla', title: 'SLA Breach Imminent', description: 'F-2842 (Cisco IOS XE) SLA expires in 24h — escalation required', product: 'Network Infra', read: false, createdAt: new Date(Date.now() - 25 * 60000).toISOString(), timeAgo: '25 min ago' },
  { id: 'n-3', type: 'kev', title: 'New KEV Added', description: 'CISA added CVE-2025-77001 (Microsoft Exchange) to KEV catalog', product: 'Global', read: false, createdAt: new Date(Date.now() - 3600000).toISOString(), timeAgo: '1h ago' },
  { id: 'n-4', type: 'scan', title: 'Scan Completed', description: 'Production Network Sweep (SC-0047) completed — 47 findings discovered', product: 'Production', read: true, createdAt: new Date(Date.now() - 7200000).toISOString(), timeAgo: '2h ago' },
  { id: 'n-5', type: 'critical', title: 'SLA Overdue', description: 'F-2846 (Spring Framework RCE) is 2 days overdue — immediate action required', product: 'API Gateway', read: true, createdAt: new Date(Date.now() - 10800000).toISOString(), timeAgo: '3h ago' },
];

export const notificationHandlers = [
  http.get('/api/v1/notifications', ({ request }) => {
    const url = new URL(request.url);
    const type = url.searchParams.get('type');
    const unreadOnly = url.searchParams.get('unreadOnly') === 'true';

    let filtered = notificationsFixture;
    if (type) filtered = filtered.filter(n => n.type === type);
    if (unreadOnly) filtered = filtered.filter(n => !n.read);

    return HttpResponse.json({
      notifications: filtered,
      total: filtered.length,
      unreadCount: notificationsFixture.filter(n => !n.read).length,
    });
  }),

  http.post('/api/v1/notifications/mark-all-read', () => {
    notificationsFixture.forEach(n => { n.read = true; });
    return HttpResponse.json({ success: true });
  }),

  http.post('/api/v1/notifications/:id/read', ({ params }) => {
    const notif = notificationsFixture.find(n => n.id === params.id);
    if (notif) notif.read = true;
    return HttpResponse.json({ success: true });
  }),
];
```
