# SOL-UI-008 — Frontend Solution: AI Center API

**CR nguồn:** [CR-UI-008](../../../../../specs/crs/v0/ui-api/CR-UI-008-ai-center-api.md)  
**Ngày tạo:** 2026-06-16  
**Trạng thái:** Proposed  
**Ưu tiên:** P1 — High (v3.0, phụ thuộc CR-OVS-005)  
**Phạm vi:** Frontend React SPA (`ui/src/features/ai-center/`)

---

## 1. Tóm tắt giải pháp

CR-UI-008 bao phủ AI Center với 2 screens. Frontend cần:

1. `aiApi.ts` — triage queue, review, enrichment status/trigger
2. `useAITriage.ts` / `useAIEnrichment.ts` hooks
3. Async triage flow: 200 (cached) hoặc 202 (polling/SSE)
4. MSW handlers với realistic AI responses

> **Phụ thuộc:** ai-service LLM triage (CR-OVS-005). MSW dùng trong khi chờ.

---

## 2. File Structure

```
ui/src/
├── features/ai-center/
│   ├── api/
│   │   └── aiApi.ts
│   ├── hooks/
│   │   ├── useAITriage.ts
│   │   └── useAIEnrichment.ts
│   ├── components/
│   │   ├── AITriage.tsx           # /ai/triage — queue + actions
│   │   ├── AIEnrichment.tsx       # /ai/enrichment — stats
│   │   ├── TriageCard.tsx
│   │   └── EnrichmentStats.tsx
│   └── types.ts
│
└── mocks/handlers/
    └── ai.handlers.ts
```

---

## 3. Implementation Chi Tiết

### 3.1 `features/ai-center/api/aiApi.ts`

```typescript
import apiClient from '@/shared/api/client';
import type {
  AITriageResult, AITriageQueueResponse,
  AIReviewRequest, AIReviewResponse,
  AIEnrichmentStats, CVEEnrichmentStatus
} from '../types';

export const aiApi = {
  // POST /api/v1/ai/triage/{findingId}
  // Returns 200 (sync result) hoặc 202 (async processing)
  requestTriage: async (findingId: string): Promise<AITriageResult | { status: 'processing'; estimated_ms: number }> => {
    const { data } = await apiClient.post(`/api/v1/ai/triage/${findingId}`);
    return data;
  },

  // GET /api/v1/ai/triage/queue
  getQueue: async (params: {
    status?: string;
    severity?: string[];
    page?: number;
    page_size?: number;
  } = {}): Promise<AITriageQueueResponse> => {
    const { data } = await apiClient.get<AITriageQueueResponse>('/api/v1/ai/triage/queue', {
      params: { ...params, severity: params.severity?.join(',') },
    });
    return data;
  },

  // POST /api/v1/ai/triage/{findingId}/review
  reviewTriage: async (findingId: string, payload: AIReviewRequest): Promise<AIReviewResponse> => {
    const { data } = await apiClient.post<AIReviewResponse>(
      `/api/v1/ai/triage/${findingId}/review`,
      payload
    );
    return data;
  },

  // GET /api/v1/ai/enrichment
  getEnrichmentStats: async (): Promise<AIEnrichmentStats> => {
    const { data } = await apiClient.get<AIEnrichmentStats>('/api/v1/ai/enrichment');
    return data;
  },

  // GET /api/v1/ai/enrichment/{cveId}
  getCVEEnrichment: async (cveId: string): Promise<CVEEnrichmentStatus> => {
    const { data } = await apiClient.get<CVEEnrichmentStatus>(`/api/v1/ai/enrichment/${cveId}`);
    return data;
  },

  // POST /api/v1/ai/enrichment/trigger
  triggerEnrichment: async (payload: {
    cve_ids?: string[];
    force_refresh?: boolean;
  }): Promise<{ job_id: string; status: string; cve_count: number }> => {
    const { data } = await apiClient.post('/api/v1/ai/enrichment/trigger', payload);
    return data;
  },
};
```

### 3.2 `features/ai-center/hooks/useAITriage.ts`

```typescript
import { useState } from 'react';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { aiApi } from '../api/aiApi';
import { findingKeys } from '@/features/findings/hooks/useFindings';
import toast from 'react-hot-toast';

export const aiKeys = {
  all: ['ai'] as const,
  queue: (params: object) => [...aiKeys.all, 'queue', params] as const,
  enrichment: () => [...aiKeys.all, 'enrichment'] as const,
};

export function useAITriageQueue(params: {
  status?: string;
  page?: number;
} = {}) {
  return useQuery({
    queryKey: aiKeys.queue(params),
    queryFn: () => aiApi.getQueue({ status: params.status || 'pending', page: params.page }),
    staleTime: 30_000,
    refetchInterval: 60_000, // Refresh mỗi phút
  });
}

export function useRequestTriage() {
  const [processingIds, setProcessingIds] = useState<Set<string>>(new Set());
  const queryClient = useQueryClient();

  const mutation = useMutation({
    mutationFn: async (findingId: string) => {
      setProcessingIds(prev => new Set(prev).add(findingId));
      const result = await aiApi.requestTriage(findingId);
      setProcessingIds(prev => {
        const next = new Set(prev);
        next.delete(findingId);
        return next;
      });
      return { findingId, result };
    },
    onSuccess: ({ result }) => {
      if ('status' in result && result.status === 'processing') {
        toast('AI triage in progress...', { icon: '🤖', duration: 3000 });
      } else {
        toast.success('AI triage complete');
        queryClient.invalidateQueries({ queryKey: aiKeys.all });
      }
    },
  });

  return {
    requestTriage: mutation.mutate,
    isProcessing: (id: string) => processingIds.has(id),
  };
}

export function useReviewTriage() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: ({ findingId, payload }: {
      findingId: string;
      payload: { decision: 'accepted' | 'overridden' | 'rejected'; note?: string };
    }) => aiApi.reviewTriage(findingId, payload),

    onSuccess: (data, { payload }) => {
      queryClient.invalidateQueries({ queryKey: aiKeys.queue({}) });
      // Nếu accepted + AI suggests FalsePositive → invalidate finding để UI prompt
      queryClient.invalidateQueries({ queryKey: findingKeys.detail(data.finding_id) });

      const msg = payload.decision === 'accepted'
        ? 'AI suggestion accepted'
        : payload.decision === 'overridden'
        ? 'Overridden with human decision'
        : 'AI suggestion rejected';
      toast.success(msg);
    },
  });
}
```

### 3.3 `features/ai-center/hooks/useAIEnrichment.ts`

```typescript
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { aiApi } from '../api/aiApi';
import { aiKeys } from './useAITriage';
import toast from 'react-hot-toast';

export function useAIEnrichmentStats() {
  return useQuery({
    queryKey: aiKeys.enrichment(),
    queryFn: () => aiApi.getEnrichmentStats(),
    staleTime: 5 * 60_000,
  });
}

export function useTriggerEnrichment() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: aiApi.triggerEnrichment,
    onSuccess: (data) => {
      toast.success(`Enrichment job queued: ${data.cve_count} CVEs`);
      // Refresh stats sau 10s
      setTimeout(() => {
        queryClient.invalidateQueries({ queryKey: aiKeys.enrichment() });
      }, 10_000);
    },
  });
}
```

### 3.4 AI Triage Queue Component

```tsx
// features/ai-center/components/AITriage.tsx
export function AITriage() {
  const [statusFilter, setStatusFilter] = useState('pending');
  const { data, isLoading } = useAITriageQueue({ status: statusFilter });
  const { requestTriage, isProcessing } = useRequestTriage();
  const reviewTriage = useReviewTriage();

  if (isLoading) return <AITriageSkeleton />;

  return (
    <div className="space-y-4">
      {/* Stats Bar */}
      <div className="grid grid-cols-5 gap-3">
        <KPICard label="Pending" value={data?.stats.pending} icon="🕐" />
        <KPICard label="Confirmed" value={data?.stats.confirmed} icon="✅" />
        <KPICard label="False Positives" value={data?.stats.false_positive} icon="🚫" />
        <KPICard label="Not Affected" value={data?.stats.not_affected} icon="❌" />
        <KPICard
          label="Time Saved"
          value={`${data?.stats.time_saved_hours}h`}
          icon="⏱️"
          tooltip="Estimated analyst time saved by AI pre-screening"
        />
      </div>

      {/* Status Filter */}
      <StatusTabs
        options={['pending', 'confirmed', 'false_positive', 'not_affected']}
        value={statusFilter}
        onChange={setStatusFilter}
      />

      {/* Triage Queue */}
      <div className="space-y-3">
        {data?.queue.map(item => (
          <TriageCard
            key={item.finding_id}
            item={item}
            isProcessing={isProcessing(item.finding_id)}
            onRequestTriage={() => requestTriage(item.finding_id)}
            onReview={(decision, note) =>
              reviewTriage.mutate({ findingId: item.finding_id, payload: { decision, note } })
            }
          />
        ))}
      </div>
    </div>
  );
}
```

### 3.5 TriageCard Component

```tsx
// features/ai-center/components/TriageCard.tsx
const REMARKS_STYLES = {
  Confirmed:    { color: '#EF4444', icon: '⚠️', label: 'Confirmed Vulnerability' },
  FalsePositive:{ color: '#10B981', icon: '✅', label: 'Likely False Positive' },
  NotAffected:  { color: '#6B7280', icon: '❌', label: 'Not Affected' },
  Unexplored:   { color: '#EAB308', icon: '🔍', label: 'Needs Investigation' },
};

export function TriageCard({ item, isProcessing, onRequestTriage, onReview }) {
  const hasResult = !!item.ai_result;
  const remarksStyle = hasResult ? REMARKS_STYLES[item.ai_result.remarks] : null;

  return (
    <div className="bg-[var(--bg-elevated)] rounded-lg border border-[var(--border-base)] p-4">
      {/* Header */}
      <div className="flex items-start justify-between">
        <div>
          <Link to={`/findings/${item.finding_id}`} className="font-medium hover:text-[var(--brand-blue)]">
            {item.finding_title}
          </Link>
          <div className="flex gap-2 mt-1">
            <SeverityBadge severity={item.severity} />
            {item.cve_id && <span className="text-xs text-[var(--text-muted)]">{item.cve_id}</span>}
          </div>
        </div>

        {/* AI Result or Request Button */}
        {!hasResult ? (
          <Button
            size="sm"
            onClick={onRequestTriage}
            isLoading={isProcessing}
          >
            🤖 Run AI Triage
          </Button>
        ) : (
          <div
            className="flex items-center gap-1.5 text-sm font-medium px-3 py-1 rounded-full"
            style={{ backgroundColor: `${remarksStyle!.color}20`, color: remarksStyle!.color }}
          >
            {remarksStyle!.icon} {remarksStyle!.label}
            <span className="text-xs opacity-70">
              ({(item.ai_result.confidence * 100).toFixed(0)}%)
            </span>
          </div>
        )}
      </div>

      {/* AI Justification */}
      {hasResult && (
        <div className="mt-3 space-y-2">
          <p className="text-sm text-[var(--text-secondary)]">{item.ai_result.justification}</p>

          {item.ai_result.actions.length > 0 && (
            <ul className="text-sm text-[var(--text-muted)] list-disc list-inside">
              {item.ai_result.actions.map((action, i) => (
                <li key={i}>{action}</li>
              ))}
            </ul>
          )}

          {/* Human Review — only if not yet reviewed */}
          {!item.human_decision && (
            <div className="flex gap-2 pt-2">
              <Button size="sm" variant="outline" onClick={() => onReview('accepted')}>
                ✓ Accept
              </Button>
              <Button size="sm" variant="outline" onClick={() => onReview('overridden')}>
                ✏️ Override
              </Button>
              <Button size="sm" variant="ghost" onClick={() => onReview('rejected')}>
                ✗ Reject
              </Button>
            </div>
          )}

          {/* Already reviewed badge */}
          {item.human_decision && (
            <div className="text-xs text-[var(--text-muted)]">
              Reviewed by {item.reviewed_by} — {item.human_decision}
            </div>
          )}
        </div>
      )}
    </div>
  );
}
```

---

## 4. MSW Handler — `mocks/handlers/ai.handlers.ts`

```typescript
import { http, HttpResponse } from 'msw';

const mockTriageResult = {
  remarks: 'Confirmed',
  confidence: 0.94,
  justification: 'CVE-2025-44228 has a CVSS score of 10.0, is in CISA KEV, and actively exploited. Component log4j-core 2.14.1 is within the affected version range.',
  actions: [
    'Update log4j-core to version 2.15.0 or later',
    'Apply WAF rule to block JNDI lookup patterns',
    'Audit all log4j usages in codebase',
  ],
  generated_at: new Date().toISOString(),
  ai_provider: 'ollama',
};

export const aiHandlers = [
  // POST /api/v1/ai/triage/:findingId
  http.post('/api/v1/ai/triage/:findingId', async () => {
    // Simulate 50% chance of async processing
    if (Math.random() < 0.5) {
      await new Promise(r => setTimeout(r, 500));
      return HttpResponse.json({ ...mockTriageResult, finding_id: 'F-mock' });
    }
    return HttpResponse.json({ finding_id: 'F-mock', status: 'processing', estimated_ms: 3000 }, { status: 202 });
  }),

  // GET /api/v1/ai/triage/queue
  http.get('/api/v1/ai/triage/queue', () => {
    return HttpResponse.json({
      queue: [
        {
          finding_id: 'F-2847',
          finding_title: 'Apache Log4j2 JNDI Remote Code Execution',
          cve_id: 'CVE-2025-44228',
          severity: 'Critical',
          ai_result: mockTriageResult,
          human_decision: null,
          human_note: null,
          reviewed_by: null,
          reviewed_at: null,
        },
        {
          finding_id: 'F-2848',
          finding_title: 'Nginx Path Traversal',
          cve_id: 'CVE-2024-56789',
          severity: 'Medium',
          ai_result: {
            ...mockTriageResult,
            remarks: 'FalsePositive',
            confidence: 0.82,
            justification: 'The version detected (nginx 1.24.0) is not in the affected range per NVD.',
          },
          human_decision: null,
          human_note: null,
          reviewed_by: null,
          reviewed_at: null,
        },
      ],
      total: 2,
      stats: { pending: 15, confirmed: 48, false_positive: 12, not_affected: 8, time_saved_hours: 24.5 },
      page: 1, page_size: 20,
    });
  }),

  // POST /api/v1/ai/triage/:findingId/review
  http.post('/api/v1/ai/triage/:findingId/review', async ({ params, request }) => {
    const body = await request.json() as any;
    return HttpResponse.json({
      finding_id: params.findingId,
      decision: body.decision,
      note: body.note,
      reviewed_by: 'bob@company.com',
      reviewed_at: new Date().toISOString(),
    });
  }),

  // GET /api/v1/ai/enrichment
  http.get('/api/v1/ai/enrichment', () => {
    return HttpResponse.json({
      stats: {
        total_cves: 312450,
        with_embedding: 298000,
        embedding_coverage_pct: 95.4,
        last_enrichment_run: '2026-06-16T06:00:00Z',
        semantic_search_accuracy: 0.82,
      },
      recent_enrichments: [
        { cve_id: 'CVE-2026-12345', has_embedding: true, embedding_dims: 1536,
          is_cached: true, ai_severity: 'Critical', ai_provider: 'ollama',
          enriched_at: '2026-06-16T06:00:00Z' },
      ],
      total: 298000,
    });
  }),

  // POST /api/v1/ai/enrichment/trigger
  http.post('/api/v1/ai/enrichment/trigger', async ({ request }) => {
    const body = await request.json() as any;
    return HttpResponse.json({
      job_id: 'enrich_job_' + Date.now(),
      status: 'queued',
      cve_count: body.cve_ids?.length || 14450,
    }, { status: 202 });
  }),
];
```

---

## 5. Async Triage Polling Strategy

Khi `POST /api/v1/ai/triage/{id}` trả về `202 processing`:

```typescript
// Polling approach cho async triage
async function pollTriageResult(findingId: string, maxWaitMs = 30_000): Promise<AITriageResult> {
  const start = Date.now();
  const delay = (ms: number) => new Promise(r => setTimeout(r, ms));

  while (Date.now() - start < maxWaitMs) {
    await delay(2000);
    // Refetch queue để kiểm tra triage result
    const queue = await aiApi.getQueue({ status: 'pending', page: 1 });
    const item = queue.queue.find(q => q.finding_id === findingId && q.ai_result);
    if (item?.ai_result) return item.ai_result;
  }

  throw new Error('Triage timeout');
}
```

Hoặc đơn giản hơn: React Query `refetchInterval: 3000` khi triage đang processing.

---

## 6. Acceptance Criteria (Frontend)

- [ ] AI Triage Queue load từ `GET /api/v1/ai/triage/queue` — không hardcode
- [ ] `Run AI Triage` button → `POST /api/v1/ai/triage/{id}` → hiển thị result hoặc spinner
- [ ] 202 response → spinner, sau đó poll/refetch khi result sẵn sàng
- [ ] Confidence % hiển thị bên cạnh Remarks badge
- [ ] Accept/Override/Reject buttons → `POST /api/v1/ai/triage/{id}/review`
- [ ] Time Saved KPI từ `stats.time_saved_hours`
- [ ] AI Enrichment Stats từ `GET /api/v1/ai/enrichment`
- [ ] Coverage % progress bar (e.g., 95.4%)
- [ ] Trigger Enrichment → `POST /api/v1/ai/enrichment/trigger` → 202 toast
