# TASK-P4-03 — Fix `WebhookEvents.tsx` → `useWebhookDeliveries()`

**Phase:** 4 — Scan & Report  
**Nguồn giải pháp:** [`solutions/10_11_12_webhook_report_notification.md` — Solution 10](../solutions/10_11_12_webhook_report_notification.md)  
**Ưu tiên:** 🔵 Scan & Report  
**Phụ thuộc:** TASK-P1-03, TASK-P3-03 (dùng chung integrations.handlers.ts)

---

## Vấn đề hiện tại

```typescript
// ❌ HIỆN TẠI — features/integrations/components/WebhookEvents.tsx
const DELIVERY_HISTORY = [
  { id: 'DEL-0441', event: 'finding.created', status: 'success', responseTime: 124, ... },
  // delivery log hardcode
];
const ACTIVITY_CHART = [
  { h: '06:00', success: 42, failed: 1 },
  // chart data tĩnh hardcode luôn
];
```

---

## API Endpoints mới

```
GET  /api/v1/webhooks/deliveries          → Delivery log
GET  /api/v1/webhooks/stats/hourly        → Hourly activity chart
POST /api/v1/webhooks/deliveries/:id/retry → Retry failed delivery
```

---

## Danh sách files cần tạo/sửa

### [NEW] `src/features/integrations/hooks/useWebhookDeliveries.ts`

```typescript
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
  h: string;
  success: number;
  failed: number;
}

export function useWebhookDeliveries(webhookId?: string, params?: { page?: number }) {
  return useQuery<WebhookDeliveriesResponse>({
    queryKey: ['webhooks', 'deliveries', webhookId, params],
    queryFn: async () => {
      const { data } = await apiClient.get<WebhookDeliveriesResponse>(
        '/api/v1/webhooks/deliveries', { params: { webhookId, ...params } }
      );
      return data;
    },
    staleTime: 30_000,
    refetchInterval: 30_000,
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

### [MODIFY] `src/features/integrations/components/WebhookEvents.tsx`

**Xóa:**
```typescript
const DELIVERY_HISTORY = [...]
const ACTIVITY_CHART = [...]
```

**Thêm:**
```typescript
import { useWebhookDeliveries, useWebhookHourlyStats, useRetryDelivery } from '../hooks/useWebhookDeliveries';

const deliveriesQuery = useWebhookDeliveries(selected?.id);
const hourlyStatsQuery = useWebhookHourlyStats();
const retryDelivery = useRetryDelivery();

// Chart: data={hourlyStatsQuery.data ?? []}
// Table: {deliveriesQuery.data?.deliveries.map(d => ...)}
// Retry button: onClick={() => retryDelivery.mutate(d.id)}
```

### [MODIFY] `src/shared/api/endpoints.ts` — thêm webhooks delivery endpoints

```typescript
webhooks: {
  // ... existing
  deliveries:  '/api/v1/webhooks/deliveries',
  statsHourly: '/api/v1/webhooks/stats/hourly',
  retry:       (id: string) => `/api/v1/webhooks/deliveries/${id}/retry`,
},
```

### [MODIFY] `src/mocks/handlers/integrations.handlers.ts` — thêm webhook delivery handlers

```typescript
http.get('/api/v1/webhooks/deliveries', () => {
  return HttpResponse.json({
    deliveries: [
      { id: 'DEL-0441', webhookId: 'wh-1', event: 'finding.created',
        endpoint: 'siem.company.com', status: 'success', responseTime: 124,
        statusCode: 200, time: new Date(Date.now() - 120000).toISOString() },
      { id: 'DEL-0439', webhookId: 'wh-2', event: 'sla.breached',
        endpoint: 'jira.company.com', status: 'failed', responseTime: 5001,
        statusCode: 503, time: new Date(Date.now() - 10800000).toISOString() },
    ],
    total: 2,
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

## Tiêu chí hoàn thành

- [x] `features/integrations/hooks/useWebhookDeliveries.ts` tạo xong (3 hooks)
- [x] `WebhookEvents.tsx` không còn `DELIVERY_HISTORY`, `ACTIVITY_CHART` constants
- [x] Delivery table render từ `deliveriesQuery.data?.deliveries` (API)
- [x] Hourly chart render từ `hourlyStatsQuery.data` (stable, không thay đổi)
- [x] "Retry" button gọi `retryDelivery.mutate(d.id)` + invalidate
- [x] formatTimeAgo() từ ISO string (không hardcode "2h ago")
- [x] MSW handlers: deliveries + hourly stats + retry (trong integration.handlers.ts)
- [x] TypeScript 0 lỗi mới

---

## ✅ Đã hoàn thành — 2026-06-19

**Files đã tạo/sửa:**
- [`features/integrations/hooks/useWebhookDeliveries.ts`](../../../../ui/src/features/integrations/hooks/useWebhookDeliveries.ts) — [NEW] 3 hooks
- [`features/integrations/components/WebhookEvents.tsx`](../../../../ui/src/features/integrations/components/WebhookEvents.tsx) — [MODIFY] Refactored
- [`mocks/handlers/integration.handlers.ts`](../../../../ui/src/mocks/handlers/integration.handlers.ts) — [MODIFY] +deliveries, +hourly, +retry

---

## Kiểm tra

```bash
# Mở http://localhost:3000 → Integrations → Webhook Events
# 1. Delivery log hiển thị từ MSW (DEL-0441 success, DEL-0439 failed)
# 2. Hourly chart hiển thị stable (không thay đổi khi re-render)
# 3. Click "Retry" trên DEL-0439 (failed) → POST được gửi
```
