# SOL-UI-009 — Frontend Solution: Reports & Notifications API

**CR nguồn:** [CR-UI-009](../../../../../specs/crs/v0/ui-api/CR-UI-009-reports-notifications-api.md)  
**Ngày tạo:** 2026-06-16  
**Trạng thái:** Proposed  
**Ưu tiên:** P0 — Critical  
**Phạm vi:** Frontend React SPA (`ui/src/features/reports/`, `ui/src/features/notifications/`)

---

## 1. Tóm tắt giải pháp

CR-UI-009 bao phủ 2 modules:

**Reports:** Generate, list, download security reports (PDF/HTML/CSV/Excel/JSON).
**Notifications:** In-app notification center, mark-read, webhooks management.

Frontend cần:
1. `reportApi.ts` — CRUD reports + download trigger
2. `notificationApi.ts` — list, mark-read, webhooks CRUD
3. `useReports.ts` / `useNotifications.ts` hooks
4. Report download với blob handling + presigned URL redirect
5. Notification bell component (unread count badge)

---

## 2. File Structure

```
ui/src/
├── features/reports/
│   ├── api/
│   │   └── reportApi.ts
│   ├── hooks/
│   │   └── useReports.ts
│   ├── components/
│   │   ├── ReportCenter.tsx
│   │   ├── ReportList.tsx
│   │   ├── GenerateReportDialog.tsx
│   │   └── ReportStatusBadge.tsx
│   └── types.ts
│
├── features/notifications/
│   ├── api/
│   │   └── notificationApi.ts
│   ├── hooks/
│   │   ├── useNotifications.ts
│   │   └── useWebhooks.ts
│   ├── components/
│   │   ├── NotificationCenter.tsx    # /notifications
│   │   ├── NotificationBell.tsx      # Header bell icon
│   │   ├── WebhookList.tsx
│   │   └── WebhookForm.tsx
│   └── types.ts
│
└── mocks/handlers/
    ├── report.handlers.ts
    └── notification.handlers.ts
```

---

## 3. Implementation Chi Tiết

### 3.1 `features/reports/api/reportApi.ts`

```typescript
import apiClient from '@/shared/api/client';
import type { ReportRun, ReportListResponse, GenerateReportRequest } from '../types';

export const reportApi = {
  // GET /api/v1/reports
  list: async (params: {
    product_id?: string;
    format?: string;
    status?: string;
    page?: number;
    page_size?: number;
  } = {}): Promise<ReportListResponse> => {
    const { data } = await apiClient.get<ReportListResponse>('/api/v1/reports', { params });
    return data;
  },

  // POST /api/v1/reports
  generate: async (payload: GenerateReportRequest): Promise<ReportRun> => {
    const { data } = await apiClient.post<ReportRun>('/api/v1/reports', payload);
    return data;
  },

  // GET /api/v1/reports/{id}
  getById: async (id: string): Promise<ReportRun> => {
    const { data } = await apiClient.get<ReportRun>(`/api/v1/reports/${id}`);
    return data;
  },

  // GET /api/v1/reports/{id}/download
  download: async (id: string, filename: string): Promise<void> => {
    // Try presigned URL redirect first
    try {
      const response = await apiClient.get(`/api/v1/reports/${id}/download`, {
        maxRedirects: 0,
        validateStatus: (s) => s === 302 || (s >= 200 && s < 300),
      });

      if (response.status === 302) {
        // Presigned URL — open in new tab
        window.open(response.headers.location, '_blank');
        return;
      }

      // Direct binary stream
      const blobResponse = await apiClient.get(`/api/v1/reports/${id}/download`, {
        responseType: 'blob',
      });
      const url = URL.createObjectURL(blobResponse.data);
      const a = document.createElement('a');
      a.href = url;
      a.download = filename;
      a.click();
      URL.revokeObjectURL(url);

    } catch {
      // Fallback: direct download via window.location
      const token = (await import('@/features/auth/store/authStore')).authStore.getState().accessToken;
      window.location.href = `/api/v1/reports/${id}/download?token=${token}`;
    }
  },

  // DELETE /api/v1/reports/{id}
  delete: async (id: string): Promise<void> => {
    await apiClient.delete(`/api/v1/reports/${id}`);
  },
};
```

### 3.2 `features/reports/hooks/useReports.ts`

```typescript
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { reportApi } from '../api/reportApi';
import toast from 'react-hot-toast';

export const reportKeys = {
  all: ['reports'] as const,
  list: (params: object) => [...reportKeys.all, 'list', params] as const,
  detail: (id: string) => [...reportKeys.all, 'detail', id] as const,
};

export function useReports(params: { productId?: string; status?: string } = {}) {
  const queryParams = { product_id: params.productId, status: params.status };

  return useQuery({
    queryKey: reportKeys.list(queryParams),
    queryFn: () => reportApi.list(queryParams),
    staleTime: 30_000,
    // Poll mỗi 5s khi có report đang generating
    refetchInterval: (data) => {
      const hasPending = data?.state?.data?.reports.some(
        r => r.status === 'generating' || r.status === 'pending'
      );
      return hasPending ? 5_000 : false;
    },
  });
}

export function useGenerateReport() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: reportApi.generate,
    onSuccess: (data) => {
      toast.success('Report generation started');
      queryClient.invalidateQueries({ queryKey: reportKeys.all });
    },
  });
}

export function useDeleteReport() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: reportApi.delete,
    onSuccess: () => {
      toast.success('Report deleted');
      queryClient.invalidateQueries({ queryKey: reportKeys.all });
    },
  });
}
```

### 3.3 `features/notifications/api/notificationApi.ts`

```typescript
import apiClient from '@/shared/api/client';
import type { NotificationListResponse, Notification, Webhook } from '../types';

export const notificationApi = {
  // GET /api/v1/notifications
  list: async (params: {
    is_read?: boolean;
    type?: string[];
    page?: number;
    page_size?: number;
  } = {}): Promise<NotificationListResponse> => {
    const { data } = await apiClient.get<NotificationListResponse>('/api/v1/notifications', {
      params: { ...params, type: params.type?.join(',') },
    });
    return data;
  },

  // GET /api/v1/notifications/unread-count
  getUnreadCount: async (): Promise<{ unread_count: number }> => {
    const { data } = await apiClient.get('/api/v1/notifications/unread-count');
    return data;
  },

  // PATCH /api/v1/notifications/{id}/read
  markRead: async (id: string): Promise<void> => {
    await apiClient.patch(`/api/v1/notifications/${id}/read`);
  },

  // POST /api/v1/notifications/mark-all-read
  markAllRead: async (): Promise<{ marked_count: number }> => {
    const { data } = await apiClient.post('/api/v1/notifications/mark-all-read');
    return data;
  },

  // Webhooks
  listWebhooks: async (): Promise<{ webhooks: Webhook[]; total: number }> => {
    const { data } = await apiClient.get('/api/v1/webhooks');
    return data;
  },

  createWebhook: async (payload: {
    url: string;
    events: string[];
    secret?: string;
  }): Promise<Webhook & { hmac_secret: string }> => {
    const { data } = await apiClient.post('/api/v1/webhooks', payload);
    return data;
  },

  deleteWebhook: async (id: string): Promise<void> => {
    await apiClient.delete(`/api/v1/webhooks/${id}`);
  },

  testWebhook: async (id: string): Promise<{
    delivery_id: string;
    status: string;
    response_code: number;
    response_time_ms: number;
  }> => {
    const { data } = await apiClient.post(`/api/v1/webhooks/${id}/test`);
    return data;
  },
};
```

### 3.4 `features/notifications/hooks/useNotifications.ts`

```typescript
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { notificationApi } from '../api/notificationApi';
import { useNotificationStore } from '../store/notificationStore';
import toast from 'react-hot-toast';

export const notifKeys = {
  all: ['notifications'] as const,
  list: (params: object) => [...notifKeys.all, 'list', params] as const,
  unreadCount: () => [...notifKeys.all, 'unread-count'] as const,
  webhooks: () => [...notifKeys.all, 'webhooks'] as const,
};

// Unread count — polled mỗi 60s (backup cho SSE)
export function useUnreadCount() {
  return useQuery({
    queryKey: notifKeys.unreadCount(),
    queryFn: () => notificationApi.getUnreadCount(),
    staleTime: 30_000,
    refetchInterval: 60_000,
    select: (data) => data.unread_count,
  });
}

export function useNotifications(params: { isRead?: boolean; page?: number } = {}) {
  const queryParams = { is_read: params.isRead, page: params.page ?? 1 };

  return useQuery({
    queryKey: notifKeys.list(queryParams),
    queryFn: () => notificationApi.list(queryParams),
    staleTime: 30_000,
  });
}

export function useMarkRead() {
  const queryClient = useQueryClient();
  const { markAllRead: storeMarkAll } = useNotificationStore();

  const markOne = useMutation({
    mutationFn: notificationApi.markRead,
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: notifKeys.all });
    },
  });

  const markAll = useMutation({
    mutationFn: notificationApi.markAllRead,
    onSuccess: () => {
      storeMarkAll(); // Also clear Zustand store unread count
      queryClient.invalidateQueries({ queryKey: notifKeys.all });
    },
  });

  return { markOne, markAll };
}
```

### 3.5 NotificationBell Header Component

```tsx
// features/notifications/components/NotificationBell.tsx
export function NotificationBell() {
  const [open, setOpen] = useState(false);
  // Unread từ Zustand (real-time SSE) + React Query poll (backup)
  const { unreadCount: sseCount } = useNotificationStore();
  const { data: polledCount } = useUnreadCount();

  const totalUnread = Math.max(sseCount, polledCount ?? 0);

  return (
    <Popover open={open} onOpenChange={setOpen}>
      <PopoverTrigger>
        <button className="relative p-2 rounded-lg hover:bg-[var(--bg-overlay)] transition-colors">
          <BellIcon className="w-5 h-5 text-[var(--text-secondary)]" />
          {totalUnread > 0 && (
            <span className="absolute -top-0.5 -right-0.5 min-w-[18px] h-[18px] px-1 rounded-full bg-[var(--severity-critical)] text-white text-[10px] font-bold flex items-center justify-center">
              {totalUnread > 99 ? '99+' : totalUnread}
            </span>
          )}
        </button>
      </PopoverTrigger>

      <PopoverContent className="w-80" align="end">
        <NotificationDropdown onClose={() => setOpen(false)} />
      </PopoverContent>
    </Popover>
  );
}
```

### 3.6 Report Center Component

```tsx
// features/reports/components/ReportCenter.tsx
export function ReportCenter() {
  const { data, isLoading } = useReports();
  const generateReport = useGenerateReport();
  const deleteReport = useDeleteReport();
  const [showGenerate, setShowGenerate] = useState(false);
  const { canDownloadReports } = usePermissions();

  if (isLoading) return <ReportListSkeleton />;

  return (
    <div className="space-y-6">
      {/* Header */}
      <div className="flex items-center justify-between">
        <h1 className="text-2xl font-bold">Reports</h1>
        {canDownloadReports && (
          <Button onClick={() => setShowGenerate(true)}>
            + Generate Report
          </Button>
        )}
      </div>

      {/* Reports Table */}
      <DataTable
        columns={[
          { key: 'product_name', label: 'Product' },
          { key: 'format', label: 'Format', render: (v) => v.toUpperCase() },
          { key: 'status', render: (v) => <ReportStatusBadge status={v} /> },
          { key: 'finding_count', label: 'Findings' },
          { key: 'generated_at', label: 'Generated', render: (v) => formatDate(v) },
          {
            key: 'actions',
            render: (_, row) => (
              <div className="flex gap-2">
                <Button
                  size="sm"
                  disabled={row.status !== 'completed'}
                  onClick={() => reportApi.download(row.id, `${row.product_name}-report.${row.format}`)}
                >
                  ↓ Download
                </Button>
                <Button size="sm" variant="ghost" onClick={() => deleteReport.mutate(row.id)}>
                  Delete
                </Button>
              </div>
            ),
          },
        ]}
        data={data?.reports ?? []}  // ← từ server, không hardcode
        total={data?.total ?? 0}
      />

      {/* Generate Dialog */}
      {showGenerate && (
        <GenerateReportDialog
          onGenerate={(payload) => generateReport.mutate(payload)}
          onClose={() => setShowGenerate(false)}
          isLoading={generateReport.isPending}
        />
      )}
    </div>
  );
}
```

---

## 4. Generate Report Dialog (Form)

```tsx
// features/reports/components/GenerateReportDialog.tsx
import { useForm } from 'react-hook-form';
import { z } from 'zod';
import { zodResolver } from '@hookform/resolvers/zod';

const generateSchema = z.object({
  product_id: z.string().optional(),
  engagement_id: z.string().optional(),
  format: z.enum(['pdf', 'html', 'csv', 'excel', 'json']),
  min_severity: z.enum(['Critical', 'High', 'Medium', 'Low']).optional(),
  min_score: z.number().min(0).max(10).optional(),
  date_from: z.string().optional(),
  date_to: z.string().optional(),
}).refine(data => data.product_id || data.engagement_id, {
  message: 'Either Product or Engagement is required',
  path: ['product_id'],
});

export function GenerateReportDialog({ onGenerate, onClose, isLoading }) {
  const form = useForm({ resolver: zodResolver(generateSchema) });
  const { data: products } = useProducts();

  return (
    <Dialog open onOpenChange={onClose}>
      <DialogContent>
        <DialogHeader>Generate Report</DialogHeader>
        <form onSubmit={form.handleSubmit(onGenerate)}>
          <Select label="Product" {...form.register('product_id')}>
            {products?.products.map(p => (
              <option key={p.id} value={p.id}>{p.name}</option>
            ))}
          </Select>
          <Select label="Format" {...form.register('format')}>
            {['pdf', 'html', 'csv', 'excel', 'json'].map(f => (
              <option key={f} value={f}>{f.toUpperCase()}</option>
            ))}
          </Select>
          <Select label="Min Severity" {...form.register('min_severity')}>
            {['Critical', 'High', 'Medium', 'Low'].map(s => (
              <option key={s} value={s}>{s}</option>
            ))}
          </Select>
          <DialogFooter>
            <Button type="button" variant="ghost" onClick={onClose}>Cancel</Button>
            <Button type="submit" isLoading={isLoading}>Generate</Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}
```

---

## 5. MSW Handlers

```typescript
// mocks/handlers/report.handlers.ts
import { http, HttpResponse } from 'msw';

const reportsFixture = [
  {
    id: 'rpt_001', product_id: 'prod_1', product_name: 'Banking Portal',
    engagement_id: null, format: 'pdf', status: 'completed', exit_code: 1,
    min_severity: 'High', min_score: 7.0, finding_count: 10,
    generated_at: '2026-06-16T11:00:00Z',
    artifact_url: 'https://storage.example.com/reports/rpt_001.pdf',
    expires_at: '2026-07-16T11:00:00Z',
    created_at: '2026-06-16T10:58:00Z',
    created_by: 'carol@company.com',
  },
];

export const reportHandlers = [
  http.get('/api/v1/reports', () => {
    return HttpResponse.json({ reports: reportsFixture, total: reportsFixture.length, page: 1, page_size: 20 });
  }),

  http.post('/api/v1/reports', async ({ request }) => {
    const body = await request.json() as any;
    return HttpResponse.json({
      id: 'rpt_' + Date.now(), product_id: body.product_id,
      format: body.format, status: 'pending',
      created_at: new Date().toISOString(), created_by: 'carol@company.com',
    }, { status: 202 });
  }),

  http.get('/api/v1/reports/:id/download', () => {
    // Simulate presigned URL redirect
    return new HttpResponse(null, {
      status: 302,
      headers: { Location: 'https://storage.example.com/reports/mock-report.pdf' },
    });
  }),

  http.delete('/api/v1/reports/:id', () => {
    return HttpResponse.json({ success: true });
  }),
];

// mocks/handlers/notification.handlers.ts
export const notificationHandlers = [
  http.get('/api/v1/notifications', () => {
    return HttpResponse.json({
      notifications: [
        { id: 'notif_001', type: 'finding.sla.breached',
          title: 'SLA Breached: CVE-2025-44228 (Banking Portal)',
          message: 'Finding F-2847 has exceeded the Critical SLA deadline by 7 days.',
          severity: 'Critical', entity_type: 'finding', entity_id: 'F-2847',
          is_read: false, created_at: '2026-06-16T08:00:00Z' },
        { id: 'notif_002', type: 'kev.new',
          title: 'New KEV: Apache Struts RCE (CVE-2026-12345)',
          message: 'CVE-2026-12345 was added to CISA KEV. Known ransomware campaign usage.',
          severity: 'Critical', entity_type: 'cve', entity_id: 'CVE-2026-12345',
          is_read: true, created_at: '2026-06-16T06:00:00Z' },
      ],
      total: 2, unread_count: 1, page: 1, page_size: 20,
    });
  }),

  http.get('/api/v1/notifications/unread-count', () => {
    return HttpResponse.json({ unread_count: 1 });
  }),

  http.patch('/api/v1/notifications/:id/read', () => {
    return HttpResponse.json({ id: 'notif_001', is_read: true });
  }),

  http.post('/api/v1/notifications/mark-all-read', () => {
    return HttpResponse.json({ marked_count: 5 });
  }),

  http.get('/api/v1/webhooks', () => {
    return HttpResponse.json({
      webhooks: [
        { id: 'wh_001', url: 'https://hooks.slack.com/services/xxx',
          events: ['kev.new', 'finding.sla.breached'], is_active: true,
          secret_preview: 'sha256:a1b2c3...',
          created_at: '2026-06-01T00:00:00Z',
          last_delivery_at: '2026-06-16T08:00:00Z', last_delivery_status: 'success' },
      ],
      total: 1,
    });
  }),

  http.post('/api/v1/webhooks', async ({ request }) => {
    const body = await request.json() as any;
    return HttpResponse.json({
      id: 'wh_' + Date.now(), url: body.url, events: body.events,
      is_active: true,
      hmac_secret: 'ovs_secret_' + Math.random().toString(36).slice(2),
      created_at: new Date().toISOString(),
    }, { status: 201 });
  }),

  http.post('/api/v1/webhooks/:id/test', () => {
    return HttpResponse.json({
      delivery_id: 'dlv_test_' + Date.now(),
      status: 'success', response_code: 200, response_time_ms: 245,
    });
  }),

  http.delete('/api/v1/webhooks/:id', () => {
    return HttpResponse.json({ success: true });
  }),
];
```

---

## 6. Acceptance Criteria (Frontend)

### Reports
- [ ] Report list load từ `GET /api/v1/reports` — không hardcode
- [ ] Generate Report dialog → `POST /api/v1/reports` → 202 → status `pending`
- [ ] Auto-poll mỗi 5s khi có report `pending/generating`
- [ ] Download button: gọi `/reports/{id}/download` → binary hoặc presigned redirect
- [ ] `exit_code=1` badge (CI/CD) hiển thị khi report có findings
- [ ] Delete → xóa khỏi list ngay (optimistic update)

### Notifications
- [ ] Bell icon badge từ `unread_count` (SSE + poll backup)
- [ ] Notification Center từ `GET /api/v1/notifications`
- [ ] Mark one read → PATCH → badge count giảm
- [ ] Mark all read → POST → badge = 0
- [ ] Webhook list từ `GET /api/v1/webhooks`
- [ ] Create webhook → show `hmac_secret` một lần duy nhất (dialog)
- [ ] Test webhook → show delivery status
