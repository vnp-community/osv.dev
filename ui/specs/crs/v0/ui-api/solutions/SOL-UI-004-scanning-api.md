# SOL-UI-004 — Frontend Solution: Active Scanning API

**CR nguồn:** [CR-UI-004](../../../../../specs/crs/v0/ui-api/CR-UI-004-scanning-api.md)  
**Ngày tạo:** 2026-06-16  
**Trạng thái:** Proposed  
**Ưu tiên:** P1 — High (v3.0 feature, phụ thuộc CR-OVS-001)  
**Phạm vi:** Frontend React SPA (`ui/src/features/scanning/`)

---

## 1. Tóm tắt giải pháp

CR-UI-004 bao phủ 6 screens Scanning. Frontend cần:

1. `scanApi.ts` — CRUD scans, cancel, import, results (Nmap/ZAP)
2. `useScan.ts` / `useScanList.ts` — React Query hooks
3. `useScanSSE.ts` — SSE hook cho real-time progress
4. `ScanWizard` — 4-step form với React Hook Form + Zod validation
5. MSW handlers với SSE streaming mock

> **Lưu ý:** Các endpoints SSE và Nmap/ZAP kết quả (`/api/v1/scans/{id}/stream`, `/results/nmap`, `/results/zap`) là v3.0, phụ thuộc CR-OVS-001. Frontend sẽ implement với MSW trước.

---

## 2. File Structure

```
ui/src/
├── features/scanning/
│   ├── api/
│   │   └── scanApi.ts
│   ├── hooks/
│   │   ├── useScanList.ts         # List scans với stats
│   │   ├── useScanDetail.ts       # Single scan detail
│   │   ├── useScanSSE.ts          # SSE progress hook
│   │   └── useScanHistory.ts      # Scheduled + history
│   ├── components/
│   │   ├── ScanDashboard.tsx      # /scans — stats + tabs
│   │   ├── ScanWizard.tsx         # /scans/new — 4-step form
│   │   ├── RunningScan.tsx        # /scans/:id (running)
│   │   ├── ScanDetail.tsx         # /scans/:id (completed)
│   │   ├── NmapResults.tsx        # /scans/:id/results/nmap
│   │   ├── ZAPResults.tsx         # /scans/:id/results/zap
│   │   └── ScheduledScans.tsx
│   ├── schemas/
│   │   └── scanWizard.schema.ts   # Zod schema for ScanWizard
│   └── types.ts
│
└── mocks/handlers/
    └── scan.handlers.ts
```

---

## 3. Implementation Chi Tiết

### 3.1 `features/scanning/api/scanApi.ts`

```typescript
import apiClient from '@/shared/api/client';
import type {
  Scan, ScanListResponse, CreateScanRequest,
  NmapResultsResponse, ZAPResultsResponse,
  ScheduledScansResponse
} from '../types';

export const scanApi = {
  // GET /api/v1/scans
  list: async (params: {
    status?: string[];
    type?: string;
    page?: number;
    page_size?: number;
    sort_by?: string;
  } = {}): Promise<ScanListResponse> => {
    const { data } = await apiClient.get<ScanListResponse>('/api/v1/scans', {
      params: {
        ...params,
        status: params.status?.join(','),
      },
    });
    return data;
  },

  // POST /api/v1/scans
  create: async (payload: CreateScanRequest): Promise<Scan> => {
    const { data } = await apiClient.post<Scan>('/api/v1/scans', payload);
    return data;
  },

  // GET /api/v1/scans/{id}
  getById: async (id: string): Promise<Scan> => {
    const { data } = await apiClient.get<Scan>(`/api/v1/scans/${id}`);
    return data;
  },

  // POST /api/v1/scans/{id}/cancel
  cancel: async (id: string): Promise<{ success: boolean; scan_id: string; status: string }> => {
    const { data } = await apiClient.post(`/api/v1/scans/${id}/cancel`);
    return data;
  },

  // GET /api/v1/scans/{id}/results/nmap
  getNmapResults: async (id: string): Promise<NmapResultsResponse> => {
    const { data } = await apiClient.get<NmapResultsResponse>(`/api/v1/scans/${id}/results/nmap`);
    return data;
  },

  // GET /api/v1/scans/{id}/results/zap
  getZAPResults: async (id: string): Promise<ZAPResultsResponse> => {
    const { data } = await apiClient.get<ZAPResultsResponse>(`/api/v1/scans/${id}/results/zap`);
    return data;
  },

  // GET /api/v1/scans/scheduled
  getScheduled: async (): Promise<ScheduledScansResponse> => {
    const { data } = await apiClient.get<ScheduledScansResponse>('/api/v1/scans/scheduled');
    return data;
  },

  // POST /api/v1/scans/import
  importReport: async (formData: FormData): Promise<{ import_id: string; status: string }> => {
    const { data } = await apiClient.post('/api/v1/scans/import', formData, {
      headers: { 'Content-Type': 'multipart/form-data' },
    });
    return data;
  },
};
```

### 3.2 `features/scanning/hooks/useScanList.ts`

```typescript
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { scanApi } from '../api/scanApi';

export const scanKeys = {
  all: ['scans'] as const,
  list: (params: object) => [...scanKeys.all, 'list', params] as const,
  detail: (id: string) => [...scanKeys.all, 'detail', id] as const,
  nmap: (id: string) => [...scanKeys.all, 'nmap', id] as const,
  zap: (id: string) => [...scanKeys.all, 'zap', id] as const,
};

export function useScanList(params: {
  status?: string[];
  page?: number;
  pageSize?: number;
} = {}) {
  return useQuery({
    queryKey: scanKeys.list(params),
    queryFn: () => scanApi.list({
      status: params.status,
      page: params.page ?? 1,
      page_size: params.pageSize ?? 20,
    }),
    staleTime: 10_000,          // Scan list thay đổi thường
    refetchInterval: 15_000,    // Auto-refresh mỗi 15s (xem running scans)
  });
}

export function useCancelScan() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: scanApi.cancel,
    onSuccess: (_, scanId) => {
      queryClient.invalidateQueries({ queryKey: scanKeys.detail(scanId) });
      queryClient.invalidateQueries({ queryKey: scanKeys.all });
    },
  });
}

export function useCreateScan() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: scanApi.create,
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: scanKeys.all });
    },
  });
}
```

### 3.3 `features/scanning/hooks/useScanSSE.ts` — Real-time Progress

```typescript
import { useState, useEffect, useRef } from 'react';
import { useQueryClient } from '@tanstack/react-query';
import { useAuthStore } from '@/features/auth/store/authStore';
import { scanKeys } from './useScanList';
import type { ScanProgress } from '../types';

export type SSEStatus = 'connecting' | 'open' | 'closed' | 'error';

export function useScanSSE(scanId: string, enabled: boolean) {
  const [progress, setProgress] = useState<ScanProgress | null>(null);
  const [sseStatus, setSseStatus] = useState<SSEStatus>('closed');
  const queryClient = useQueryClient();
  const { accessToken } = useAuthStore();
  const sourceRef = useRef<EventSource | null>(null);

  useEffect(() => {
    if (!enabled || !scanId || !accessToken) {
      setSseStatus('closed');
      return;
    }

    // SSE với auth token qua query param
    const url = `/api/v1/scans/${scanId}/stream?token=${encodeURIComponent(accessToken)}`;
    const source = new EventSource(url, { withCredentials: true });
    sourceRef.current = source;
    setSseStatus('connecting');

    source.onopen = () => setSseStatus('open');

    source.onmessage = (e) => {
      const data = JSON.parse(e.data) as ScanProgress;
      setProgress(data);

      // Update scan trong React Query cache
      queryClient.setQueryData(scanKeys.detail(scanId), (old: any) =>
        old ? { ...old, progress: data.progress, status: data.status } : old
      );
    };

    source.addEventListener('done', (e) => {
      const data = JSON.parse((e as MessageEvent).data) as ScanProgress;
      setProgress(data);
      setSseStatus('closed');
      source.close();

      // Invalidate để fetch final state
      queryClient.invalidateQueries({ queryKey: scanKeys.detail(scanId) });
      queryClient.invalidateQueries({ queryKey: scanKeys.all });
    });

    source.addEventListener('ping', () => {
      // Keep-alive — no action
    });

    source.onerror = () => {
      setSseStatus('error');
      source.close();
    };

    return () => {
      source.close();
      setSseStatus('closed');
    };
  }, [scanId, enabled, accessToken, queryClient]);

  return {
    progress,
    sseStatus,
    isStreaming: sseStatus === 'open' || sseStatus === 'connecting',
  };
}
```

### 3.4 ScanWizard Schema (Zod) — `schemas/scanWizard.schema.ts`

```typescript
import { z } from 'zod';

const ipv4Regex = /^(\d{1,3}\.){3}\d{1,3}(\/\d{1,2})?$/;
const urlRegex = /^https?:\/\/.+/;

export const scanWizardSchema = z.discriminatedUnion('type', [
  // Nmap types
  z.object({
    type: z.enum(['nmap_full', 'nmap_discovery']),
    name: z.string().min(3, 'Name must be at least 3 characters'),
    targets: z.array(
      z.string().regex(ipv4Regex, 'Must be valid IP or CIDR (e.g. 10.0.0.0/24)')
    ).min(1, 'At least one target required'),
    options: z.object({
      scan_profile: z.enum(['discovery', 'full', 'custom']).optional(),
      port_range: z.string().regex(/^\d+-\d+$/, 'Format: 1-65535').optional(),
    }).optional(),
    engagement_id: z.string().optional(),
    schedule_frequency: z.enum(['once', 'daily', 'weekly', 'custom']).default('once'),
    schedule_cron_expr: z.string().optional(),
  }),

  // ZAP type
  z.object({
    type: z.literal('zap'),
    name: z.string().min(3),
    targets: z.array(
      z.string().regex(urlRegex, 'Must be valid URL (http/https)')
    ).min(1),
    options: z.object({
      max_depth: z.number().int().min(1).max(20).optional(),
      timeout: z.number().int().min(30).max(3600).optional(),
    }).optional(),
    engagement_id: z.string().optional(),
    schedule_frequency: z.enum(['once', 'daily', 'weekly', 'custom']).default('once'),
    schedule_cron_expr: z.string().optional(),
  }),
]);

export type ScanWizardForm = z.infer<typeof scanWizardSchema>;
```

### 3.5 ScanWizard Form — 4 Steps

```tsx
// features/scanning/components/ScanWizard.tsx
import { useForm } from 'react-hook-form';
import { zodResolver } from '@hookform/resolvers/zod';
import { scanWizardSchema, type ScanWizardForm } from '../schemas/scanWizard.schema';
import { useCreateScan } from '../hooks/useScanList';

const STEPS = ['Scan Type', 'Target', 'Schedule', 'Review & Launch'];

export function ScanWizard() {
  const [currentStep, setCurrentStep] = useState(0);
  const navigate = useNavigate();
  const createScan = useCreateScan();

  const form = useForm<ScanWizardForm>({
    resolver: zodResolver(scanWizardSchema),
    defaultValues: {
      type: 'nmap_full',
      name: '',
      targets: [],
      schedule_frequency: 'once',
    },
  });

  const onSubmit = async (values: ScanWizardForm) => {
    const result = await createScan.mutateAsync({
      name: values.name,
      type: values.type,
      targets: values.targets,
      options: values.options,
      engagement_id: values.engagement_id,
      schedule_frequency: values.schedule_frequency,
      schedule_cron_expr: values.schedule_cron_expr,
    });
    navigate(`/scans/${result.id}`);
  };

  return (
    <form onSubmit={form.handleSubmit(onSubmit)} className="max-w-2xl mx-auto">
      <StepIndicator steps={STEPS} current={currentStep} />

      {currentStep === 0 && <ScanTypeStep form={form} />}
      {currentStep === 1 && <TargetStep form={form} />}
      {currentStep === 2 && <ScheduleStep form={form} />}
      {currentStep === 3 && <ReviewStep form={form} onSubmit={form.handleSubmit(onSubmit)} />}

      <WizardNavigation
        current={currentStep}
        total={STEPS.length}
        onBack={() => setCurrentStep(s => s - 1)}
        onNext={() => setCurrentStep(s => s + 1)}
        isSubmitting={createScan.isPending}
      />
    </form>
  );
}
```

### 3.6 Running Scan — SSE Progress Bar

```tsx
// features/scanning/components/RunningScan.tsx
export function RunningScan({ scanId }: { scanId: string }) {
  const { data: scan } = useQuery({
    queryKey: scanKeys.detail(scanId),
    queryFn: () => scanApi.getById(scanId),
    refetchInterval: scan?.status === 'running' ? 5000 : false,
  });

  const { progress, sseStatus } = useScanSSE(
    scanId,
    scan?.status === 'running' || scan?.status === 'queued'
  );
  const cancelScan = useCancelScan();
  const { canCreateScan } = usePermissions();

  const displayProgress = progress?.progress ?? scan?.progress ?? 0;
  const currentStatus = progress?.status ?? scan?.status;

  return (
    <div className="space-y-6">
      {/* Scan Metadata */}
      <ScanMetadata scan={scan} sseStatus={sseStatus} />

      {/* Progress Bar */}
      <div className="space-y-2">
        <div className="flex justify-between text-sm">
          <span className="text-[var(--text-secondary)]">
            {progress?.message ?? `Scanning ${progress?.current_target ?? '...'}`}
          </span>
          <span className="font-mono text-[var(--brand-blue)]">{displayProgress}%</span>
        </div>
        <div className="h-2 rounded-full bg-[var(--bg-overlay)]">
          <div
            className="h-full rounded-full bg-gradient-to-r from-[var(--brand-blue)] to-[var(--brand-purple)] transition-all duration-500"
            style={{ width: `${displayProgress}%` }}
          />
        </div>
        <p className="text-xs text-[var(--text-muted)]">
          Findings found: {progress?.findings_found ?? 0}
        </p>
      </div>

      {/* Cancel Button */}
      {canCreateScan && currentStatus === 'running' && (
        <Button
          variant="destructive"
          onClick={() => cancelScan.mutate(scanId)}
          isLoading={cancelScan.isPending}
        >
          Cancel Scan
        </Button>
      )}
    </div>
  );
}
```

---

## 4. MSW Handler — SSE Scan Progress

```typescript
// mocks/handlers/scan.handlers.ts
import { http, HttpResponse } from 'msw';

export const scanHandlers = [
  // GET /api/v1/scans
  http.get('/api/v1/scans', () => {
    return HttpResponse.json({
      scans: [
        { id: 'sc_001', name: 'Weekly Network Scan', type: 'nmap_full',
          status: 'completed', targets: ['10.0.0.0/24'], progress: 100,
          finding_count: 23, started_at: '2026-06-16T08:00:00Z',
          completed_at: '2026-06-16T08:04:32Z', duration_ms: 272000,
          created_by: 'bob@company.com', engagement_id: 'eng_001', error: null },
      ],
      total: 1,
      page: 1, page_size: 20,
      stats: { active_scans: 0, completed_today: 1, total_findings_today: 23, scheduled_scans: 2 },
    });
  }),

  // POST /api/v1/scans
  http.post('/api/v1/scans', async ({ request }) => {
    const body = await request.json() as any;
    return HttpResponse.json({
      id: 'sc_new_' + Date.now(),
      name: body.name,
      type: body.type,
      status: 'queued',
      targets: body.targets,
      progress: 0, finding_count: 0,
      started_at: null, completed_at: null,
      created_by: 'bob@company.com',
      engagement_id: body.engagement_id ?? null,
    }, { status: 201 });
  }),

  // GET /api/v1/scans/:id/stream — SSE mock
  http.get('/api/v1/scans/:id/stream', ({ params }) => {
    const encoder = new TextEncoder();
    let progress = 0;

    const stream = new ReadableStream({
      async start(controller) {
        while (progress < 100) {
          await new Promise(r => setTimeout(r, 500));
          progress += Math.floor(Math.random() * 15) + 5;
          progress = Math.min(progress, 100);

          const data = JSON.stringify({
            scan_id: params.id,
            status: progress < 100 ? 'running' : 'completed',
            progress,
            current_target: `10.0.0.${Math.floor(progress * 2.54)}`,
            message: `Scanning port ${80 + progress}...`,
            findings_found: Math.floor(progress * 0.5),
          });
          controller.enqueue(encoder.encode(`event: message\ndata: ${data}\n\n`));
        }

        controller.enqueue(encoder.encode(
          `event: done\ndata: ${JSON.stringify({ scan_id: params.id, status: 'completed', progress: 100, findings_found: 50 })}\n\n`
        ));
        controller.close();
      },
    });

    return new HttpResponse(stream, {
      headers: { 'Content-Type': 'text/event-stream', 'Cache-Control': 'no-cache' },
    });
  }),

  // POST /api/v1/scans/:id/cancel
  http.post('/api/v1/scans/:id/cancel', () => {
    return HttpResponse.json({ success: true, scan_id: 'sc_001', status: 'cancelled' });
  }),

  // GET /api/v1/scans/:id/results/nmap
  http.get('/api/v1/scans/:id/results/nmap', ({ params }) => {
    return HttpResponse.json({
      scan_id: params.id,
      hosts: [
        { ip: '10.0.1.45', hostname: 'prod-web-01.internal', os: 'Linux 5.4.0',
          state: 'up',
          ports: [
            { port: 443, protocol: 'tcp', state: 'open', service: 'https',
              version: 'nginx 1.24.0', cve_ids: ['CVE-2025-44228'] },
            { port: 22, protocol: 'tcp', state: 'open', service: 'ssh',
              version: 'OpenSSH 8.9', cve_ids: [] },
          ],
          cve_ids: ['CVE-2025-44228'], risk_score: 10.0 },
      ],
      total_hosts: 254, hosts_up: 87, total_findings: 47,
    });
  }),

  // GET /api/v1/scans/:id/results/zap
  http.get('/api/v1/scans/:id/results/zap', ({ params }) => {
    return HttpResponse.json({
      scan_id: params.id,
      target_url: 'https://myapp.company.com',
      alerts: [
        { id: 'zap_001', name: 'SQL Injection', risk: 'High', confidence: 'High',
          url: 'https://myapp.company.com/api/users?id=1',
          description: 'SQL injection may be possible.',
          solution: 'Use parameterized queries.', evidence: 'select', cwe_id: 'CWE-89',
          references: ['https://owasp.org/www-community/attacks/SQL_Injection'] },
      ],
      total: 1,
      by_risk: { High: 1, Medium: 0, Low: 0, Informational: 0 },
    });
  }),
];
```

---

## 5. Acceptance Criteria (Frontend)

- [ ] Scan Dashboard hiển thị stats từ `response.stats.*` — không hardcode
- [ ] ScanWizard validates CIDR với Zod: invalid → form error
- [ ] ScanWizard validates URL với Zod cho ZAP type
- [ ] Submit wizard → `POST /api/v1/scans` → redirect `/scans/:id`
- [ ] RunningScan component nhận SSE events, cập nhật progress bar real-time
- [ ] `event: done` → invalidate queries → hiển thị completed state
- [ ] Cancel button gọi `POST /api/v1/scans/:id/cancel`, chỉ hiển thị khi `canCreateScan`
- [ ] Nmap Results table hiển thị hosts + ports + CVE IDs từ API
- [ ] ZAP Results table hiển thị alerts với severity distribution từ `by_risk`
- [ ] SSE kết nối với `?token=` auth (không dùng Authorization header)

---

## 6. Phase Implementation

| Feature | Phase | Phụ thuộc |
|---------|-------|---------|
| Scan list + stats | Phase 2 | Import parsers đã có |
| ScanWizard form (import) | Phase 2 | CR-DD-002 đã implement |
| SSE progress | Phase 3 | CR-OVS-001 (v3.0) |
| Nmap/ZAP active scan | Phase 4 | CR-OVS-001 (v3.0) |
| Nmap/ZAP results | Phase 4 | CR-OVS-001 (v3.0) |
| Scheduled scans | Phase 4 | CR-OVS-007 (v3.0) |
