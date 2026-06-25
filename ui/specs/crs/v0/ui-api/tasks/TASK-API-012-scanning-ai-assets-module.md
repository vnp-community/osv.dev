# TASK-API-012 — Scanning + AI Center + Assets Detail: APIs + SSE + Hooks + MSW

| Field | Value |
|-------|-------|
| **Task ID** | TASK-API-012 |
| **Module** | `ui/src/features/scanning/`, `ui/src/features/ai-center/` |
| **Solution Ref** | [SOL-UI-004](../solutions/SOL-UI-004-scanning-api.md), [SOL-UI-008](../solutions/SOL-UI-008-ai-center-api.md), [SOL-UI-006](../solutions/SOL-UI-006-asset-api.md) |
| **Priority** | 🟡 P1 (Backend v3.0 needed) |
| **Depends On** | TASK-API-003 |
| **Estimated** | 4h |

---

## Context

- **Scanning:** Sử dụng SSE (`/api/v1/scans/{id}/stream`) cho real-time progress. ScanWizard là multi-step form 4 bước với Zod validation.
- **AI Center:** Async triage với 200 (sync result) hoặc 202 (processing — cần poll/refetch). Human review flow.
- **Assets Detail:** Đã có asset list (TASK-API-009), task này bổ sung thêm asset detail page với tabs.

> [!IMPORTANT]
> Vì backend v3.0 chưa sẵn sàng, MSW handlers của task này phải extra realistic: SSE streaming có progress %, triage có random 200/202.

---

## Goal

Tạo Scanning + AI Center modules với API, SSE hooks, ScanWizard Zod schema, và MSW.

---

## Target Files

| Action | File Path |
|--------|-----------|
| CREATE | `ui/src/features/scanning/types.ts` |
| CREATE | `ui/src/features/scanning/api/scanApi.ts` |
| CREATE | `ui/src/features/scanning/hooks/useScans.ts` |
| CREATE | `ui/src/features/scanning/hooks/useScanSSE.ts` |
| CREATE | `ui/src/features/scanning/schemas/scanWizard.schema.ts` |
| CREATE | `ui/src/features/ai-center/types.ts` |
| CREATE | `ui/src/features/ai-center/api/aiApi.ts` |
| CREATE | `ui/src/features/ai-center/hooks/useAITriage.ts` |
| CREATE | `ui/src/features/ai-center/hooks/useAIEnrichment.ts` |
| CREATE | `ui/src/mocks/handlers/scan.handlers.ts` |
| CREATE | `ui/src/mocks/handlers/ai.handlers.ts` |

---

## Implementation

### File 1: `ui/src/features/scanning/types.ts`

```typescript
export type ScanType = 'nmap_quick' | 'nmap_full' | 'nmap_udp' | 'zap_baseline' | 'zap_full' | 'zap_api' | 'grype' | 'trivy' | 'import';
export type ScanStatus = 'queued' | 'running' | 'completed' | 'failed' | 'cancelled';

export interface ScanTarget {
  type: 'ip' | 'cidr' | 'hostname' | 'url';
  value: string;
}

export interface ScheduleConfig {
  enabled: boolean;
  cron_expression?: string;
  timezone?: string;
}

export interface Scan {
  id: string;
  name: string;
  type: ScanType;
  status: ScanStatus;
  targets: string[];
  options: Record<string, any>;
  asset_id: string | null;
  product_id: string | null;
  engagement_id: string | null;
  schedule: ScheduleConfig | null;
  progress: number;       // 0-100
  current_target: string | null;
  finding_count: number | null;
  started_at: string | null;
  completed_at: string | null;
  duration_ms: number | null;
  error_message: string | null;
  created_by: string;
  created_at: string;
}

export interface ScanListResponse {
  scans: Scan[];
  total: number;
  page: number;
  page_size: number;
}

export interface CreateScanPayload {
  name: string;
  type: ScanType;
  targets: string[];
  options?: Record<string, any>;
  asset_id?: string;
  product_id?: string;
  engagement_id?: string;
  schedule?: ScheduleConfig;
}

// SSE Event types
export type SSEEventType = 'progress' | 'target_complete' | 'finding_found' | 'completed' | 'error';

export interface SSEProgressEvent {
  event: SSEEventType;
  scan_id: string;
  progress: number;
  current_target?: string;
  findings_so_far?: number;
  message?: string;
  error?: string;
  timestamp: string;
}
```

### File 2: `ui/src/features/scanning/api/scanApi.ts`

```typescript
import { apiClient } from '@/shared/api/client';
import { ENDPOINTS } from '@/shared/api/endpoints';
import type { Scan, ScanListResponse, CreateScanPayload } from '../types';

export const scanApi = {
  list: async (params: {
    status?: string; type?: string; product_id?: string; page?: number;
  } = {}): Promise<ScanListResponse> => {
    const { data } = await apiClient.get<ScanListResponse>(ENDPOINTS.scans.list, { params });
    return data;
  },

  create: async (payload: CreateScanPayload): Promise<Scan> => {
    const { data } = await apiClient.post<Scan>(ENDPOINTS.scans.create, payload);
    return data;
  },

  getById: async (id: string): Promise<Scan> => {
    const { data } = await apiClient.get<Scan>(ENDPOINTS.scans.detail(id));
    return data;
  },

  cancel: async (id: string): Promise<void> => {
    await apiClient.post(ENDPOINTS.scans.cancel(id));
  },

  // SSE stream URL — không dùng apiClient (EventSource không support headers)
  getStreamUrl: (id: string, token: string): string => {
    const base = import.meta.env.VITE_API_BASE_URL || '';
    return `${base}${ENDPOINTS.scans.stream(id)}?token=${encodeURIComponent(token)}`;
  },
};
```

### File 3: `ui/src/features/scanning/hooks/useScans.ts`

```typescript
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { scanApi } from '../api/scanApi';
import toast from 'react-hot-toast';

export const scanKeys = {
  all:    ['scans'] as const,
  list:   (params: object) => ['scans', 'list', params] as const,
  detail: (id: string) => ['scans', 'detail', id] as const,
};

export function useScans(params: { status?: string; page?: number } = {}) {
  const queryParams = { status: params.status, page: params.page ?? 1 };
  return useQuery({
    queryKey: scanKeys.list(queryParams),
    queryFn:  () => scanApi.list(queryParams),
    staleTime: 15_000,
    // Poll mỗi 5s khi có scan đang chạy
    refetchInterval: (data) => {
      const hasActive = data?.state?.data?.scans.some(
        s => s.status === 'running' || s.status === 'queued'
      );
      return hasActive ? 5_000 : false;
    },
  });
}

export function useScanDetail(id: string | null) {
  return useQuery({
    queryKey: scanKeys.detail(id ?? ''),
    queryFn:  () => scanApi.getById(id!),
    enabled:  !!id,
    staleTime: 10_000,
  });
}

export function useCreateScan() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: scanApi.create,
    onSuccess: (data) => {
      qc.invalidateQueries({ queryKey: scanKeys.all });
      toast.success(`Scan "${data.name}" started`);
    },
  });
}

export function useCancelScan() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: scanApi.cancel,
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: scanKeys.all });
      toast.success('Scan cancelled');
    },
  });
}
```

### File 4: `ui/src/features/scanning/hooks/useScanSSE.ts`

```typescript
import { useEffect, useRef, useState } from 'react';
import { useQueryClient } from '@tanstack/react-query';
import { useAuthStore } from '@/features/auth/store/authStore';
import { scanApi } from '../api/scanApi';
import { scanKeys } from './useScans';
import type { SSEProgressEvent } from '../types';

export interface ScanSSEState {
  progress: number;
  currentTarget: string | null;
  findingsSoFar: number;
  message: string | null;
  status: 'idle' | 'connecting' | 'streaming' | 'completed' | 'error';
  error: string | null;
}

export function useScanSSE(scanId: string | null) {
  const { accessToken } = useAuthStore();
  const queryClient = useQueryClient();
  const sourceRef = useRef<EventSource | null>(null);

  const [state, setState] = useState<ScanSSEState>({
    progress: 0, currentTarget: null, findingsSoFar: 0,
    message: null, status: 'idle', error: null,
  });

  useEffect(() => {
    if (!scanId || !accessToken) return;

    const url = scanApi.getStreamUrl(scanId, accessToken);
    const source = new EventSource(url);
    sourceRef.current = source;
    setState(s => ({ ...s, status: 'connecting' }));

    source.onopen = () => setState(s => ({ ...s, status: 'streaming' }));

    source.onmessage = (e) => {
      const event = JSON.parse(e.data) as SSEProgressEvent;

      setState(s => ({
        ...s,
        progress:      event.progress,
        currentTarget: event.current_target ?? s.currentTarget,
        findingsSoFar: event.findings_so_far ?? s.findingsSoFar,
        message:       event.message ?? null,
      }));

      if (event.event === 'completed') {
        setState(s => ({ ...s, status: 'completed', progress: 100 }));
        source.close();
        // Refresh scan detail to get final results
        queryClient.invalidateQueries({ queryKey: scanKeys.detail(scanId) });
        queryClient.invalidateQueries({ queryKey: scanKeys.all });
      } else if (event.event === 'error') {
        setState(s => ({ ...s, status: 'error', error: event.error ?? 'Scan failed' }));
        source.close();
      }
    };

    source.onerror = () => {
      setState(s => ({ ...s, status: 'error', error: 'Connection lost' }));
      source.close();
    };

    return () => {
      source.close();
    };
  }, [scanId, accessToken, queryClient]);

  const cancel = () => {
    sourceRef.current?.close();
    setState(s => ({ ...s, status: 'idle' }));
  };

  return { ...state, cancel };
}
```

### File 5: `ui/src/features/scanning/schemas/scanWizard.schema.ts`

```typescript
import { z } from 'zod';

// Step 1: Scan Type
export const scanTypeSchema = z.object({
  type: z.enum(['nmap_quick', 'nmap_full', 'nmap_udp', 'zap_baseline', 'zap_full', 'zap_api', 'grype', 'trivy']),
  name: z.string().min(3, 'Name must be at least 3 characters').max(100),
});

// Step 2: Targets
export const targetsSchema = z.object({
  targets: z.array(z.string().min(1))
    .min(1, 'At least one target required')
    .max(50, 'Maximum 50 targets'),
  asset_id: z.string().optional(),
  product_id: z.string().optional(),
  engagement_id: z.string().optional(),
});

// Step 3: Options (per scan type)
export const nmapOptionsSchema = z.object({
  port_range: z.string().default('1-65535'),
  timing_template: z.number().min(0).max(5).default(3),
  service_detection: z.boolean().default(true),
  os_detection: z.boolean().default(false),
});

export const zapOptionsSchema = z.object({
  ajax_spider: z.boolean().default(false),
  active_scan: z.boolean().default(true),
  passive_scan: z.boolean().default(true),
  auth_url: z.string().url().optional().or(z.literal('')),
  auth_method: z.enum(['form', 'http_basic', 'bearer']).optional(),
});

// Step 4: Schedule
export const scheduleSchema = z.object({
  schedule_enabled: z.boolean().default(false),
  cron_expression: z.string().optional(),
  timezone: z.string().default('UTC'),
});

// Full wizard schema
export const createScanSchema = scanTypeSchema
  .merge(targetsSchema)
  .merge(scheduleSchema)
  .extend({
    options: z.record(z.any()).optional(),
  });

export type CreateScanFormValues = z.infer<typeof createScanSchema>;
```

### File 6: `ui/src/features/ai-center/types.ts`

```typescript
export type AIRemarks = 'Confirmed' | 'FalsePositive' | 'NotAffected' | 'Unexplored';

export interface AITriageResult {
  remarks: AIRemarks;
  confidence: number;       // 0.0 - 1.0
  justification: string;
  actions: string[];
  generated_at: string;
  ai_provider: string;
}

export interface AITriageQueueItem {
  finding_id: string;
  finding_title: string;
  cve_id: string | null;
  severity: string;
  ai_result: AITriageResult | null;
  human_decision: 'accepted' | 'overridden' | 'rejected' | null;
  human_note: string | null;
  reviewed_by: string | null;
  reviewed_at: string | null;
}

export interface AITriageQueueResponse {
  queue: AITriageQueueItem[];
  total: number;
  stats: {
    pending: number;
    confirmed: number;
    false_positive: number;
    not_affected: number;
    time_saved_hours: number;
  };
  page: number;
  page_size: number;
}

export interface CVEEnrichmentStatus {
  cve_id: string;
  has_embedding: boolean;
  embedding_dims: number | null;
  is_cached: boolean;
  ai_severity: string | null;
  ai_provider: string | null;
  enriched_at: string | null;
}

export interface AIEnrichmentStats {
  stats: {
    total_cves: number;
    with_embedding: number;
    embedding_coverage_pct: number;
    last_enrichment_run: string;
    semantic_search_accuracy: number;
  };
  recent_enrichments: CVEEnrichmentStatus[];
  total: number;
}
```

### File 7: `ui/src/features/ai-center/api/aiApi.ts`

```typescript
import { apiClient } from '@/shared/api/client';
import { ENDPOINTS } from '@/shared/api/endpoints';
import type { AITriageResult, AITriageQueueResponse, AIEnrichmentStats, CVEEnrichmentStatus } from '../types';

export const aiApi = {
  requestTriage: async (findingId: string): Promise<AITriageResult | { status: 'processing'; estimated_ms: number }> => {
    const { data } = await apiClient.post(ENDPOINTS.ai.triage(findingId));
    return data as any;
  },

  getTriageQueue: async (params: { status?: string; page?: number } = {}): Promise<AITriageQueueResponse> => {
    const { data } = await apiClient.get<AITriageQueueResponse>(ENDPOINTS.ai.triageQueue, { params });
    return data;
  },

  reviewTriage: async (findingId: string, payload: {
    decision: 'accepted' | 'overridden' | 'rejected'; note?: string;
  }): Promise<any> => {
    const { data } = await apiClient.post(ENDPOINTS.ai.triageReview(findingId), payload);
    return data;
  },

  getEnrichmentStats: async (): Promise<AIEnrichmentStats> => {
    const { data } = await apiClient.get<AIEnrichmentStats>(ENDPOINTS.ai.enrichment);
    return data;
  },

  getCVEEnrichment: async (cveId: string): Promise<CVEEnrichmentStatus> => {
    const { data } = await apiClient.get<CVEEnrichmentStatus>(ENDPOINTS.ai.enrichByCve(cveId));
    return data;
  },

  triggerEnrichment: async (payload: {
    cve_ids?: string[]; force_refresh?: boolean;
  }): Promise<{ job_id: string; status: string; cve_count: number }> => {
    const { data } = await apiClient.post(ENDPOINTS.ai.enrichTrigger, payload);
    return data as any;
  },
};
```

### File 8: `ui/src/features/ai-center/hooks/useAITriage.ts`

```typescript
import { useState } from 'react';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { aiApi } from '../api/aiApi';
import { findingKeys } from '@/features/findings/hooks/useFindings';
import toast from 'react-hot-toast';

export const aiKeys = {
  all:        ['ai'] as const,
  queue:      (params: object) => ['ai', 'queue', params] as const,
  enrichment: () => ['ai', 'enrichment'] as const,
};

export function useAITriageQueue(params: { status?: string; page?: number } = {}) {
  return useQuery({
    queryKey:        aiKeys.queue(params),
    queryFn:         () => aiApi.getTriageQueue({ status: params.status ?? 'pending', page: params.page }),
    staleTime:       30_000,
    refetchInterval: 60_000,
  });
}

export function useRequestTriage() {
  const [processingIds, setProcessingIds] = useState<Set<string>>(new Set());
  const qc = useQueryClient();

  const mutation = useMutation({
    mutationFn: async (findingId: string) => {
      setProcessingIds(prev => new Set([...prev, findingId]));
      const result = await aiApi.requestTriage(findingId);
      setProcessingIds(prev => { const n = new Set(prev); n.delete(findingId); return n; });
      return { findingId, result };
    },
    onSuccess: ({ result }) => {
      if ('status' in result && result.status === 'processing') {
        toast('AI triage in progress...', { icon: '🤖', duration: 3000 });
        // Refetch queue sau 5s
        setTimeout(() => qc.invalidateQueries({ queryKey: aiKeys.all }), 5000);
      } else {
        toast.success('AI triage complete');
        qc.invalidateQueries({ queryKey: aiKeys.all });
      }
    },
  });

  return { requestTriage: mutation.mutate, isProcessing: (id: string) => processingIds.has(id) };
}

export function useReviewTriage() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({ findingId, payload }: {
      findingId: string;
      payload: { decision: 'accepted' | 'overridden' | 'rejected'; note?: string };
    }) => aiApi.reviewTriage(findingId, payload),
    onSuccess: (_, { payload }) => {
      qc.invalidateQueries({ queryKey: aiKeys.all });
      const msg = { accepted: 'AI suggestion accepted', overridden: 'Overridden with human decision', rejected: 'AI suggestion rejected' }[payload.decision];
      toast.success(msg);
    },
  });
}
```

### File 9: `ui/src/features/ai-center/hooks/useAIEnrichment.ts`

```typescript
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { aiApi } from '../api/aiApi';
import { aiKeys } from './useAITriage';
import toast from 'react-hot-toast';

export function useAIEnrichmentStats() {
  return useQuery({
    queryKey: aiKeys.enrichment(),
    queryFn:  () => aiApi.getEnrichmentStats(),
    staleTime: 5 * 60_000,
  });
}

export function useTriggerEnrichment() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: aiApi.triggerEnrichment,
    onSuccess: (data) => {
      toast.success(`Enrichment job queued: ${data.cve_count} CVEs`);
      setTimeout(() => qc.invalidateQueries({ queryKey: aiKeys.enrichment() }), 10_000);
    },
  });
}
```

### File 10: `ui/src/mocks/handlers/scan.handlers.ts`

```typescript
import { http, HttpResponse } from 'msw';
import { ENDPOINTS } from '@/shared/api/endpoints';
import type { Scan } from '@/features/scanning/types';

let scans: Scan[] = [
  { id: 'sc_001', name: 'Weekly Network Scan', type: 'nmap_full', status: 'completed', targets: ['10.0.0.0/24'], options: { port_range: '1-65535', timing_template: 3 }, asset_id: null, product_id: 'prod_1', engagement_id: 'eng_001', schedule: null, progress: 100, current_target: null, finding_count: 23, started_at: '2026-06-16T08:00:00Z', completed_at: '2026-06-16T08:04:32Z', duration_ms: 272000, error_message: null, created_by: 'bob@company.com', created_at: '2026-06-16T07:59:00Z' },
  { id: 'sc_002', name: 'ZAP API Scan', type: 'zap_api', status: 'running', targets: ['https://api.company.com'], options: { active_scan: true }, asset_id: 'ast_001', product_id: 'prod_3', engagement_id: null, schedule: null, progress: 45, current_target: 'https://api.company.com/users', finding_count: 5, started_at: '2026-06-16T12:30:00Z', completed_at: null, duration_ms: null, error_message: null, created_by: 'bob@company.com', created_at: '2026-06-16T12:29:00Z' },
];

export const scanHandlers = [
  http.get(ENDPOINTS.scans.list, () => {
    return HttpResponse.json({ scans, total: scans.length, page: 1, page_size: 20 });
  }),
  http.post(ENDPOINTS.scans.create, async ({ request }) => {
    const body = await request.json() as any;
    const newScan: Scan = { id: 'sc_' + Date.now(), name: body.name, type: body.type, status: 'queued', targets: body.targets, options: body.options ?? {}, asset_id: body.asset_id ?? null, product_id: body.product_id ?? null, engagement_id: body.engagement_id ?? null, schedule: body.schedule ?? null, progress: 0, current_target: null, finding_count: null, started_at: null, completed_at: null, duration_ms: null, error_message: null, created_by: 'bob@company.com', created_at: new Date().toISOString() };
    scans = [newScan, ...scans];
    return HttpResponse.json(newScan, { status: 201 });
  }),
  http.get('/api/v1/scans/:id', ({ params }) => {
    const scan = scans.find(s => s.id === params.id);
    if (!scan) return HttpResponse.json({ error: 'NOT_FOUND' }, { status: 404 });
    return HttpResponse.json(scan);
  }),
  http.post('/api/v1/scans/:id/cancel', ({ params }) => {
    const idx = scans.findIndex(s => s.id === params.id);
    if (idx >= 0) scans[idx] = { ...scans[idx], status: 'cancelled' };
    return HttpResponse.json({ success: true });
  }),
  // SSE stream
  http.get('/api/v1/scans/:id/stream', () => {
    const encoder = new TextEncoder();
    const stream = new ReadableStream({
      async start(controller) {
        for (let i = 10; i <= 100; i += 10) {
          await new Promise(r => setTimeout(r, 500));
          const event = JSON.stringify({ event: i < 100 ? 'progress' : 'completed', scan_id: 'sc_test', progress: i, current_target: `192.168.1.${i}`, findings_so_far: Math.floor(i / 10), message: `Scanning ${i}%...`, timestamp: new Date().toISOString() });
          controller.enqueue(encoder.encode(`data: ${event}\n\n`));
        }
        controller.close();
      },
    });
    return new HttpResponse(stream, { headers: { 'Content-Type': 'text/event-stream', 'Cache-Control': 'no-cache' } });
  }),
];
```

### File 11: `ui/src/mocks/handlers/ai.handlers.ts`

```typescript
import { http, HttpResponse } from 'msw';
import { ENDPOINTS } from '@/shared/api/endpoints';

const mockTriageResult = { remarks: 'Confirmed', confidence: 0.94, justification: 'CVE has CVSS 10.0, is in CISA KEV, and actively exploited. Component within affected version range.', actions: ['Update to patched version', 'Apply WAF rule', 'Audit all usages'], generated_at: new Date().toISOString(), ai_provider: 'ollama' };

export const aiHandlers = [
  http.post('/api/v1/ai/triage/:findingId', async () => {
    await new Promise(r => setTimeout(r, 300));
    if (Math.random() < 0.5) {
      return HttpResponse.json({ ...mockTriageResult, finding_id: 'F-mock' });
    }
    return HttpResponse.json({ finding_id: 'F-mock', status: 'processing', estimated_ms: 3000 }, { status: 202 });
  }),
  http.get(ENDPOINTS.ai.triageQueue, () => {
    return HttpResponse.json({ queue: [{ finding_id: 'F-2847', finding_title: 'Apache Log4j2 JNDI RCE', cve_id: 'CVE-2021-44228', severity: 'Critical', ai_result: mockTriageResult, human_decision: null, human_note: null, reviewed_by: null, reviewed_at: null }], total: 1, stats: { pending: 15, confirmed: 48, false_positive: 12, not_affected: 8, time_saved_hours: 24.5 }, page: 1, page_size: 20 });
  }),
  http.post('/api/v1/ai/triage/:findingId/review', async ({ params, request }) => {
    const body = await request.json() as any;
    return HttpResponse.json({ finding_id: params.findingId, decision: body.decision, note: body.note, reviewed_by: 'bob@company.com', reviewed_at: new Date().toISOString() });
  }),
  http.get(ENDPOINTS.ai.enrichment, () => {
    return HttpResponse.json({ stats: { total_cves: 312450, with_embedding: 298000, embedding_coverage_pct: 95.4, last_enrichment_run: '2026-06-16T06:00:00Z', semantic_search_accuracy: 0.82 }, recent_enrichments: [{ cve_id: 'CVE-2026-12345', has_embedding: true, embedding_dims: 1536, is_cached: true, ai_severity: 'Critical', ai_provider: 'ollama', enriched_at: '2026-06-16T06:00:00Z' }], total: 298000 });
  }),
  http.post(ENDPOINTS.ai.enrichTrigger, async ({ request }) => {
    const body = await request.json() as any;
    return HttpResponse.json({ job_id: 'enrich_job_' + Date.now(), status: 'queued', cve_count: body.cve_ids?.length || 14450 }, { status: 202 });
  }),
];
```

---

## Verification

```bash
cd ui/
VITE_ENABLE_MSW=true pnpm dev

# Scanning:
# 1. /scans → list 2 scans (1 completed, 1 running)
# 2. Create scan → ScanWizard 4 steps
# 3. Zod validation: Step 1 phải có name ≥ 3 chars
# 4. Submit → POST → scan in list với status=queued
# 5. SSE: progress bar tăng từ 0 đến 100 với 10% steps

# AI Center:
# 6. /ai/triage → queue với 1 item (F-2847)
# 7. Click "Run AI Triage" → 50% chance 200 (result), 50% chance 202 (spinner)
# 8. Accept/Override/Reject → review recorded
# 9. /ai/enrichment → 95.4% coverage stats

npx tsc --noEmit
# Expected: no errors
```

---

## Checklist

### Scanning
- [ ] `features/scanning/types.ts` — Scan, ScanType, ScanStatus, SSEProgressEvent, CreateScanPayload
- [ ] `features/scanning/api/scanApi.ts` — list, create, getById, cancel, `getStreamUrl(scanId, token)`
- [ ] `useScans` — `refetchInterval` khi có `running/queued`
- [ ] `useScanSSE` — EventSource với `?token=`, handle progress/completed/error events
- [ ] `scanWizard.schema.ts` — 4 Zod schemas: scanType, targets, options (nmap/zap), schedule
- [ ] `scan.handlers.ts` — list, create, detail, cancel, SSE stream (10 steps × 500ms)

### AI Center
- [ ] `features/ai-center/types.ts` — AITriageResult, AIRemarks, AITriageQueueItem, AIEnrichmentStats
- [ ] `features/ai-center/api/aiApi.ts` — 6 methods dùng `ENDPOINTS.ai.*`
- [ ] `useAITriageQueue` — refetchInterval 60s
- [ ] `useRequestTriage` — processingIds Set, handle 200 vs 202
- [ ] 202 response → 5s setTimeout → invalidate queue
- [ ] `useReviewTriage` — 3 decision types với toast
- [ ] `useAIEnrichmentStats` + `useTriggerEnrichment`
- [ ] `ai.handlers.ts` — POST triage: random 200/202; GET queue; POST review; GET enrichment; POST trigger

### General
- [ ] Tất cả hooks dùng `ENDPOINTS.*` (không string literal)
- [ ] `npx tsc --noEmit` không lỗi
