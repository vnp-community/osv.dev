# TASK-API-010 — Reports + Notifications Module: APIs + Hooks + MSW + Bell Component

| Field | Value |
|-------|-------|
| **Task ID** | TASK-API-010 |
| **Module** | `ui/src/features/reports/`, `ui/src/features/notifications/` |
| **Solution Ref** | [SOL-UI-009](../solutions/SOL-UI-009-reports-notifications-api.md) |
| **Priority** | 🔴 P0 |
| **Depends On** | TASK-API-003 |
| **Estimated** | 3h |

---

## Context

- **Reports:** Polling tự động khi có report `pending/generating`. Download có 2 flow: presigned URL redirect hoặc blob stream.
- **Notifications:** `useNotificationsSSE` (TASK-API-004) xử lý real-time. Task này xử lý REST API (list, mark-read) + Webhook management + `NotificationBell` component.

---

## Goal

Tạo Reports module + Notifications REST module + Webhooks module + NotificationBell.

---

## Target Files

| Action | File Path |
|--------|-----------|
| CREATE | `ui/src/features/reports/types.ts` |
| CREATE | `ui/src/features/reports/api/reportApi.ts` |
| CREATE | `ui/src/features/reports/hooks/useReports.ts` |
| CREATE | `ui/src/features/notifications/types.ts` |
| CREATE | `ui/src/features/notifications/api/notificationApi.ts` |
| CREATE | `ui/src/features/notifications/hooks/useNotifications.ts` |
| CREATE | `ui/src/features/notifications/hooks/useWebhooks.ts` |
| CREATE | `ui/src/features/notifications/components/NotificationBell.tsx` |
| CREATE | `ui/src/mocks/fixtures/report.fixture.ts` |
| CREATE | `ui/src/mocks/handlers/report.handlers.ts` |
| CREATE | `ui/src/mocks/handlers/notification.handlers.ts` |

---

## Implementation

### File 1: `ui/src/features/reports/types.ts`

```typescript
export type ReportFormat = 'pdf' | 'html' | 'csv' | 'excel' | 'json';
export type ReportStatus = 'pending' | 'generating' | 'completed' | 'failed';

export interface ReportRun {
  id: string;
  product_id: string | null;
  product_name: string | null;
  engagement_id: string | null;
  format: ReportFormat;
  status: ReportStatus;
  exit_code: 0 | 1 | null;  // 0 = no findings, 1 = findings found (CI/CD gate)
  min_severity: string | null;
  min_score: number | null;
  finding_count: number | null;
  generated_at: string | null;
  artifact_url: string | null;
  expires_at: string | null;
  created_at: string;
  created_by: string;
}

export interface ReportListResponse {
  reports: ReportRun[];
  total: number;
  page: number;
  page_size: number;
}

export interface GenerateReportRequest {
  product_id?: string;
  engagement_id?: string;
  format: ReportFormat;
  min_severity?: 'Critical' | 'High' | 'Medium' | 'Low';
  min_score?: number;
  date_from?: string;
  date_to?: string;
}
```

### File 2: `ui/src/features/reports/api/reportApi.ts`

```typescript
import { apiClient } from '@/shared/api/client';
import { ENDPOINTS } from '@/shared/api/endpoints';
import type { ReportRun, ReportListResponse, GenerateReportRequest } from '../types';

export const reportApi = {
  list: async (params: {
    product_id?: string; format?: string; status?: string;
    page?: number; page_size?: number;
  } = {}): Promise<ReportListResponse> => {
    const { data } = await apiClient.get<ReportListResponse>(ENDPOINTS.reports.list, { params });
    return data;
  },

  generate: async (payload: GenerateReportRequest): Promise<ReportRun> => {
    const { data } = await apiClient.post<ReportRun>(ENDPOINTS.reports.create, payload);
    return data;
  },

  getById: async (id: string): Promise<ReportRun> => {
    const { data } = await apiClient.get<ReportRun>(ENDPOINTS.reports.detail(id));
    return data;
  },

  download: async (id: string, filename: string): Promise<void> => {
    // Thử presigned URL redirect trước
    try {
      const response = await apiClient.get(ENDPOINTS.reports.download(id), {
        maxRedirects: 0,
        validateStatus: (s) => s === 302 || (s >= 200 && s < 300),
      });

      if (response.status === 302) {
        window.open(response.headers.location, '_blank');
        return;
      }
    } catch { /* Fallthrough */ }

    // Direct binary download
    const blobResponse = await apiClient.get(ENDPOINTS.reports.download(id), {
      responseType: 'blob',
    });
    const url = URL.createObjectURL(blobResponse.data as Blob);
    const a = document.createElement('a');
    a.href = url;
    a.download = filename;
    a.click();
    URL.revokeObjectURL(url);
  },

  delete: async (id: string): Promise<void> => {
    await apiClient.delete(ENDPOINTS.reports.delete(id));
  },
};
```

### File 3: `ui/src/features/reports/hooks/useReports.ts`

```typescript
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { reportApi } from '../api/reportApi';
import toast from 'react-hot-toast';

export const reportKeys = {
  all:  ['reports'] as const,
  list: (params: object) => ['reports', 'list', params] as const,
};

export function useReports(params: { productId?: string; status?: string } = {}) {
  const queryParams = { product_id: params.productId, status: params.status };
  return useQuery({
    queryKey: reportKeys.list(queryParams),
    queryFn:  () => reportApi.list(queryParams),
    staleTime: 30_000,
    // Auto-poll mỗi 5s khi có report pending/generating
    refetchInterval: (data) => {
      const hasPending = data?.state?.data?.reports.some(
        r => r.status === 'generating' || r.status === 'pending'
      );
      return hasPending ? 5_000 : false;
    },
  });
}

export function useGenerateReport() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: reportApi.generate,
    onSuccess: () => {
      toast.success('Report generation started');
      qc.invalidateQueries({ queryKey: reportKeys.all });
    },
  });
}

export function useDeleteReport() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: reportApi.delete,
    onSuccess: () => {
      toast.success('Report deleted');
      qc.invalidateQueries({ queryKey: reportKeys.all });
    },
  });
}
```

### File 4: `ui/src/features/notifications/types.ts`

```typescript
export type NotificationType =
  | 'kev.new' | 'finding.sla.breached' | 'finding.sla.at_risk'
  | 'scan.completed' | 'scan.failed' | 'finding.status_changed'
  | 'risk_acceptance.expired' | 'report.completed';

export interface Notification {
  id: string;
  type: NotificationType;
  title: string;
  message: string;
  severity: 'Critical' | 'High' | 'Medium' | 'Low' | 'Info' | null;
  entity_type: 'cve' | 'finding' | 'scan' | 'report' | null;
  entity_id: string | null;
  is_read: boolean;
  created_at: string;
}

export interface NotificationListResponse {
  notifications: Notification[];
  total: number;
  unread_count: number;
  page: number;
  page_size: number;
}

export interface Webhook {
  id: string;
  url: string;
  events: string[];
  is_active: boolean;
  secret_preview: string;
  created_at: string;
  last_delivery_at: string | null;
  last_delivery_status: 'success' | 'failed' | null;
}
```

### File 5: `ui/src/features/notifications/api/notificationApi.ts`

```typescript
import { apiClient } from '@/shared/api/client';
import { ENDPOINTS } from '@/shared/api/endpoints';
import type { NotificationListResponse, Webhook } from '../types';

export const notificationApi = {
  list: async (params: { is_read?: boolean; page?: number; page_size?: number } = {}): Promise<NotificationListResponse> => {
    const { data } = await apiClient.get<NotificationListResponse>(ENDPOINTS.notifications.list, { params });
    return data;
  },

  getUnreadCount: async (): Promise<{ unread_count: number }> => {
    const { data } = await apiClient.get(ENDPOINTS.notifications.unreadCount);
    return data as { unread_count: number };
  },

  markRead: async (id: string): Promise<void> => {
    await apiClient.patch(ENDPOINTS.notifications.markRead(id));
  },

  markAllRead: async (): Promise<{ marked_count: number }> => {
    const { data } = await apiClient.post(ENDPOINTS.notifications.markAllRead);
    return data as { marked_count: number };
  },

  // Webhooks
  listWebhooks: async (): Promise<{ webhooks: Webhook[]; total: number }> => {
    const { data } = await apiClient.get(ENDPOINTS.webhooks.list);
    return data as { webhooks: Webhook[]; total: number };
  },

  createWebhook: async (payload: {
    url: string; events: string[]; secret?: string;
  }): Promise<Webhook & { hmac_secret: string }> => {
    const { data } = await apiClient.post(ENDPOINTS.webhooks.create, payload);
    return data as Webhook & { hmac_secret: string };
  },

  deleteWebhook: async (id: string): Promise<void> => {
    await apiClient.delete(ENDPOINTS.webhooks.delete(id));
  },

  testWebhook: async (id: string): Promise<{
    delivery_id: string; status: string; response_code: number; response_time_ms: number;
  }> => {
    const { data } = await apiClient.post(ENDPOINTS.webhooks.test(id));
    return data as any;
  },
};
```

### File 6: `ui/src/features/notifications/hooks/useNotifications.ts`

```typescript
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { notificationApi } from '../api/notificationApi';
import { useNotificationStore } from '../store/notificationStore';
import toast from 'react-hot-toast';

export const notifKeys = {
  all:         ['notifications'] as const,
  list:        (params: object) => ['notifications', 'list', params] as const,
  unreadCount: () => ['notifications', 'unread-count'] as const,
};

export function useUnreadCount() {
  return useQuery({
    queryKey:        notifKeys.unreadCount(),
    queryFn:         () => notificationApi.getUnreadCount(),
    staleTime:       30_000,
    refetchInterval: 60_000,
    select:          (data) => data.unread_count,
  });
}

export function useNotifications(params: { isRead?: boolean; page?: number } = {}) {
  const queryParams = { is_read: params.isRead, page: params.page ?? 1 };
  return useQuery({
    queryKey: notifKeys.list(queryParams),
    queryFn:  () => notificationApi.list(queryParams),
    staleTime: 30_000,
  });
}

export function useMarkRead() {
  const qc = useQueryClient();
  const { markAllRead: storeMarkAll } = useNotificationStore();

  const markOne = useMutation({
    mutationFn: notificationApi.markRead,
    onSuccess: () => qc.invalidateQueries({ queryKey: notifKeys.all }),
  });

  const markAll = useMutation({
    mutationFn: notificationApi.markAllRead,
    onSuccess: () => {
      storeMarkAll();
      qc.invalidateQueries({ queryKey: notifKeys.all });
      toast.success('All notifications marked as read');
    },
  });

  return { markOne, markAll };
}
```

### File 7: `ui/src/features/notifications/hooks/useWebhooks.ts`

```typescript
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { notificationApi } from '../api/notificationApi';
import toast from 'react-hot-toast';

export const webhookKeys = {
  list: () => ['webhooks'] as const,
};

export function useWebhooks() {
  return useQuery({
    queryKey: webhookKeys.list(),
    queryFn:  () => notificationApi.listWebhooks(),
    staleTime: 60_000,
  });
}

export function useCreateWebhook() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: notificationApi.createWebhook,
    onSuccess: () => qc.invalidateQueries({ queryKey: webhookKeys.list() }),
  });
}

export function useDeleteWebhook() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: notificationApi.deleteWebhook,
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: webhookKeys.list() });
      toast.success('Webhook deleted');
    },
  });
}

export function useTestWebhook() {
  return useMutation({
    mutationFn: notificationApi.testWebhook,
    onSuccess: (data) => {
      const msg = data.status === 'success'
        ? `Webhook OK (${data.response_code}, ${data.response_time_ms}ms)`
        : `Webhook failed (${data.response_code})`;
      data.status === 'success' ? toast.success(msg) : toast.error(msg);
    },
  });
}
```

### File 8: `ui/src/features/notifications/components/NotificationBell.tsx`

```tsx
import { useState } from 'react';
import { useNotificationStore } from '../store/notificationStore';
import { useUnreadCount } from '../hooks/useNotifications';
import { useNotifications, useMarkRead } from '../hooks/useNotifications';

export function NotificationBell() {
  const [open, setOpen] = useState(false);
  const { unreadCount: sseCount }    = useNotificationStore();
  const { data: polledCount = 0 }    = useUnreadCount();
  const totalUnread = Math.max(sseCount, polledCount);

  const { data: notifData } = useNotifications({ isRead: false });
  const { markOne, markAll } = useMarkRead();

  return (
    <div style={{ position: 'relative' }}>
      {/* Bell Button */}
      <button
        id="notification-bell-btn"
        onClick={() => setOpen(v => !v)}
        style={{
          position: 'relative', padding: '8px',
          background: 'transparent', border: 'none', cursor: 'pointer',
          borderRadius: '8px',
        }}
        aria-label={`Notifications${totalUnread > 0 ? ` (${totalUnread} unread)` : ''}`}
      >
        <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="#94A3B8" strokeWidth="2">
          <path d="M18 8A6 6 0 0 0 6 8c0 7-3 9-3 9h18s-3-2-3-9" />
          <path d="M13.73 21a2 2 0 0 1-3.46 0" />
        </svg>
        {totalUnread > 0 && (
          <span style={{
            position: 'absolute', top: '4px', right: '4px',
            minWidth: '18px', height: '18px', padding: '0 4px',
            borderRadius: '9px', background: '#EF4444',
            color: '#fff', fontSize: '10px', fontWeight: 700,
            display: 'flex', alignItems: 'center', justifyContent: 'center',
          }}>
            {totalUnread > 99 ? '99+' : totalUnread}
          </span>
        )}
      </button>

      {/* Dropdown */}
      {open && (
        <div style={{
          position: 'absolute', right: 0, top: '100%', width: '360px',
          background: '#1A2035', border: '1px solid rgba(255,255,255,0.1)',
          borderRadius: '12px', boxShadow: '0 20px 40px rgba(0,0,0,0.4)',
          zIndex: 1000, overflow: 'hidden',
        }}>
          {/* Header */}
          <div style={{ padding: '12px 16px', borderBottom: '1px solid rgba(255,255,255,0.08)', display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
            <span style={{ fontWeight: 600, color: '#E2E8F0' }}>Notifications</span>
            {totalUnread > 0 && (
              <button
                onClick={() => markAll.mutate()}
                style={{ fontSize: '12px', color: '#4F8CFF', background: 'none', border: 'none', cursor: 'pointer' }}
              >
                Mark all read
              </button>
            )}
          </div>

          {/* Notification List */}
          <div style={{ maxHeight: '400px', overflowY: 'auto' }}>
            {(notifData?.notifications ?? []).length === 0 ? (
              <div style={{ padding: '32px', textAlign: 'center', color: '#64748B' }}>
                No unread notifications
              </div>
            ) : (
              notifData?.notifications.map(n => (
                <div
                  key={n.id}
                  style={{
                    padding: '12px 16px', borderBottom: '1px solid rgba(255,255,255,0.05)',
                    cursor: 'pointer', background: n.is_read ? 'transparent' : 'rgba(79,140,255,0.06)',
                  }}
                  onClick={() => !n.is_read && markOne.mutate(n.id)}
                >
                  <div style={{ fontWeight: 500, fontSize: '13px', color: '#E2E8F0' }}>{n.title}</div>
                  <div style={{ fontSize: '12px', color: '#64748B', marginTop: '2px' }}>{n.message}</div>
                </div>
              ))
            )}
          </div>
        </div>
      )}
    </div>
  );
}
```

### File 9: `ui/src/mocks/fixtures/report.fixture.ts`

```typescript
import type { ReportRun } from '@/features/reports/types';

export const reportsFixture: ReportRun[] = [
  { id: 'rpt_001', product_id: 'prod_1', product_name: 'Banking Portal', engagement_id: null,
    format: 'pdf', status: 'completed', exit_code: 1, min_severity: 'High', min_score: 7.0,
    finding_count: 10, generated_at: '2026-06-16T11:00:00Z',
    artifact_url: 'https://storage.example.com/reports/rpt_001.pdf',
    expires_at: '2026-07-16T11:00:00Z', created_at: '2026-06-16T10:58:00Z',
    created_by: 'carol@company.com' },
  { id: 'rpt_002', product_id: 'prod_2', product_name: 'Mobile App', engagement_id: null,
    format: 'json', status: 'pending', exit_code: null, min_severity: 'Critical', min_score: null,
    finding_count: null, generated_at: null, artifact_url: null, expires_at: null,
    created_at: '2026-06-16T12:30:00Z', created_by: 'bob@company.com' },
];
```

### File 10: `ui/src/mocks/handlers/report.handlers.ts`

```typescript
import { http, HttpResponse } from 'msw';
import { ENDPOINTS } from '@/shared/api/endpoints';
import { reportsFixture } from '../fixtures/report.fixture';

let reports = [...reportsFixture];

export const reportHandlers = [
  http.get(ENDPOINTS.reports.list, () => {
    return HttpResponse.json({ reports, total: reports.length, page: 1, page_size: 20 });
  }),
  http.post(ENDPOINTS.reports.create, async ({ request }) => {
    const body = await request.json() as any;
    const newReport = {
      id: 'rpt_' + Date.now(), product_id: body.product_id ?? null,
      product_name: null, engagement_id: body.engagement_id ?? null,
      format: body.format, status: 'pending', exit_code: null,
      min_severity: body.min_severity ?? null, min_score: body.min_score ?? null,
      finding_count: null, generated_at: null, artifact_url: null, expires_at: null,
      created_at: new Date().toISOString(), created_by: 'bob@company.com',
    };
    reports = [newReport as any, ...reports];
    // Simulate completion after 5s
    setTimeout(() => {
      const idx = reports.findIndex(r => r.id === newReport.id);
      if (idx >= 0) reports[idx] = { ...reports[idx] as any, status: 'completed', exit_code: 0, finding_count: 5, generated_at: new Date().toISOString() };
    }, 5000);
    return HttpResponse.json(newReport, { status: 202 });
  }),
  http.get('/api/v1/reports/:id/download', () => {
    return new HttpResponse(null, { status: 302, headers: { Location: 'https://storage.example.com/reports/mock-report.pdf' } });
  }),
  http.delete('/api/v1/reports/:id', ({ params }) => {
    reports = reports.filter(r => r.id !== params.id);
    return HttpResponse.json({ success: true });
  }),
];
```

### File 11: `ui/src/mocks/handlers/notification.handlers.ts`

```typescript
import { http, HttpResponse } from 'msw';
import { ENDPOINTS } from '@/shared/api/endpoints';
import type { Notification } from '@/features/notifications/types';

let notifs: Notification[] = [
  { id: 'notif_001', type: 'finding.sla.breached',
    title: 'SLA Breached: CVE-2021-44228 (Banking Portal)',
    message: 'Finding F-2847 has exceeded the Critical SLA deadline by 7 days.',
    severity: 'Critical', entity_type: 'finding', entity_id: 'F-2847',
    is_read: false, created_at: '2026-06-16T08:00:00Z' },
  { id: 'notif_002', type: 'kev.new',
    title: 'New KEV: Apache Struts RCE (CVE-2026-12345)',
    message: 'CVE-2026-12345 was added to CISA KEV. Known ransomware campaign.',
    severity: 'Critical', entity_type: 'cve', entity_id: 'CVE-2026-12345',
    is_read: false, created_at: '2026-06-16T06:00:00Z' },
  { id: 'notif_003', type: 'scan.completed',
    title: 'Scan Completed: Weekly Network Scan',
    message: 'Found 23 findings (2 Critical, 8 High). View results.',
    severity: null, entity_type: 'scan', entity_id: 'sc_001',
    is_read: true, created_at: '2026-06-16T08:05:00Z' },
];

export const notificationHandlers = [
  http.get(ENDPOINTS.notifications.list, () => {
    return HttpResponse.json({ notifications: notifs, total: notifs.length, unread_count: notifs.filter(n => !n.is_read).length, page: 1, page_size: 20 });
  }),
  http.get(ENDPOINTS.notifications.unreadCount, () => {
    return HttpResponse.json({ unread_count: notifs.filter(n => !n.is_read).length });
  }),
  http.patch('/api/v1/notifications/:id/read', ({ params }) => {
    notifs = notifs.map(n => n.id === params.id ? { ...n, is_read: true } : n);
    return HttpResponse.json({ id: params.id, is_read: true });
  }),
  http.post(ENDPOINTS.notifications.markAllRead, () => {
    const count = notifs.filter(n => !n.is_read).length;
    notifs = notifs.map(n => ({ ...n, is_read: true }));
    return HttpResponse.json({ marked_count: count });
  }),
  http.get(ENDPOINTS.webhooks.list, () => {
    return HttpResponse.json({
      webhooks: [
        { id: 'wh_001', url: 'https://hooks.slack.com/services/xxx', events: ['kev.new', 'finding.sla.breached'],
          is_active: true, secret_preview: 'sha256:a1b2c3...', created_at: '2026-06-01T00:00:00Z',
          last_delivery_at: '2026-06-16T08:00:00Z', last_delivery_status: 'success' },
      ],
      total: 1,
    });
  }),
  http.post(ENDPOINTS.webhooks.create, async ({ request }) => {
    const body = await request.json() as any;
    return HttpResponse.json({ id: 'wh_' + Date.now(), url: body.url, events: body.events, is_active: true, hmac_secret: 'ovs_secret_' + Math.random().toString(36).slice(2), secret_preview: 'sha256:new...', created_at: new Date().toISOString(), last_delivery_at: null, last_delivery_status: null }, { status: 201 });
  }),
  http.delete('/api/v1/webhooks/:id', () => {
    return HttpResponse.json({ success: true });
  }),
  http.post('/api/v1/webhooks/:id/test', () => {
    return HttpResponse.json({ delivery_id: 'dlv_test_' + Date.now(), status: 'success', response_code: 200, response_time_ms: 245 });
  }),
];
```

---

## Verification

```bash
cd ui/
VITE_ENABLE_MSW=true pnpm dev

# Reports:
# 1. /reports → list 2 reports (1 completed, 1 pending)
# 2. Generate → status pending, auto-poll → sau 5s status = completed
# 3. Download completed report → presigned URL redirect (mock 302)

# Notifications:
# 4. Bell icon badge = 2 (2 unread)
# 5. Click bell → dropdown 2 unread
# 6. Click "Mark all read" → badge = 0
# 7. /notifications/webhooks → 1 webhook

npx tsc --noEmit
# Expected: no errors
```

---

## Checklist

- [ ] `features/reports/types.ts` — ReportRun, ReportFormat, ReportStatus, GenerateReportRequest
- [ ] `features/reports/api/reportApi.ts` — list, generate, getById, download (presigned + blob), delete
- [ ] `useReports` — `refetchInterval` active chỉ khi có `pending/generating` reports
- [ ] `features/notifications/types.ts` — Notification, NotificationListResponse, Webhook
- [ ] `features/notifications/api/notificationApi.ts` — 8 methods dùng `ENDPOINTS.*`
- [ ] `useUnreadCount` — poll 60s (backup cho SSE)
- [ ] `useMarkRead` — markOne + markAll + store sync
- [ ] `useWebhooks`, `useCreateWebhook`, `useDeleteWebhook`, `useTestWebhook`
- [ ] `NotificationBell.tsx` — badge count = max(sseCount, polledCount), dropdown, mark-read
- [ ] Report fixtures: 1 completed + 1 pending
- [ ] Report handlers: list, create (202 + setTimeout completion), download (302 redirect), delete
- [ ] Notification handlers: list, unread-count, markRead, markAllRead, webhooks CRUD
- [ ] `npx tsc --noEmit` không lỗi
