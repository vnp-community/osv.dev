# Solution 08 — AITriage.tsx

## Vấn đề
`const queue = [...]` — 6 triage items hardcode. Accept/Reject button không có tác dụng.  
Metric `{ label: "Accepted Today", value: 8 }` hardcode.

## API Endpoints
```
GET  /api/v1/ai/triage/queue              → Queue với pagination
POST /api/v1/ai/triage/:findingId/review  → Submit human decision
```
_(ENDPOINTS.ai.triageQueue và ENDPOINTS.ai.triageReview đã có sẵn trong endpoints.ts)_

## TypeScript Types (từ TDD.md Section 9.2)

```typescript
// features/ai-center/types.ts
export type AITriageRemarks = 'Confirmed' | 'FalsePositive' | 'NotAffected' | 'Unexplored';
export type HumanDecision = 'accepted' | 'overridden' | 'rejected';

export interface AITriageQueueItem {
  findingId: string;
  findingTitle: string;
  cveId?: string;
  severity: 'Critical' | 'High' | 'Medium' | 'Low';
  aiResult: {
    remarks: AITriageRemarks;
    confidence: number;          // 0-1
    justification: string;
    actions: string[];
    generatedAt: string;
  };
  humanDecision?: HumanDecision;
  humanNote?: string;
  reviewedBy?: string;
  reviewedAt?: string;
}

export interface AITriageQueueResponse {
  items: AITriageQueueItem[];
  total: number;
  stats: {
    pending: number;
    acceptedToday: number;
    avgConfidence: number;   // 0-100
    falsePositiveRate: number; // percentage
  };
}

export interface ReviewTriageRequest {
  decision: HumanDecision;
  note?: string;
}
```

## Hook mới: `useAITriageQueue`

```typescript
// features/ai-center/hooks/useAITriageQueue.ts
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { apiClient } from '@/shared/api/client';
import { ENDPOINTS } from '@/shared/api/endpoints';
import type { AITriageQueueResponse, ReviewTriageRequest } from '../types';

const triageKeys = {
  all: ['ai', 'triage'] as const,
  queue: (params?: Record<string, unknown>) => [...triageKeys.all, 'queue', params] as const,
};

export function useAITriageQueue(params?: { status?: string; severity?: string; page?: number }) {
  return useQuery<AITriageQueueResponse>({
    queryKey: triageKeys.queue(params),
    queryFn: async () => {
      const { data } = await apiClient.get<AITriageQueueResponse>(
        ENDPOINTS.ai.triageQueue, { params }
      );
      return data;
    },
    staleTime: 30_000,
    refetchInterval: 30_000,  // Auto-refresh — queue thay đổi liên tục
  });
}

export function useReviewTriage() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({ findingId, ...body }: { findingId: string } & ReviewTriageRequest) =>
      apiClient.post(ENDPOINTS.ai.triageReview(findingId), body),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: triageKeys.all });
    },
  });
}
```

## Component sau khi fix

```typescript
// features/ai-center/components/AITriage.tsx
import { useState } from 'react';
import { Brain, CheckCircle, XCircle, AlertTriangle, Zap } from 'lucide-react';
import { QueryBoundary } from '@/shared/components/feedback/QueryBoundary';
import { useAITriageQueue, useReviewTriage } from '../hooks/useAITriageQueue';
import type { AITriageRemarks, HumanDecision } from '../types';

// ── UI constants ────────────────────────────────────────────────────────────
const VERDICT_CONFIG: Record<AITriageRemarks, { color: string; bg: string; label: string }> = {
  Confirmed:    { color: '#EF4444', bg: 'rgba(239,68,68,0.1)',   label: 'Confirmed Vulnerability' },
  FalsePositive:{ color: '#10B981', bg: 'rgba(16,185,129,0.1)',  label: 'False Positive' },
  NotAffected:  { color: '#6B7280', bg: 'rgba(107,114,128,0.1)', label: 'Not Affected' },
  Unexplored:   { color: '#F59E0B', bg: 'rgba(245,158,11,0.1)',  label: 'Pending Analysis' },
};

const SEVERITY_STYLES: Record<string, { bg: string; color: string }> = {
  Critical: { bg: 'rgba(239,68,68,0.1)',   color: '#EF4444' },
  High:     { bg: 'rgba(249,115,22,0.1)',  color: '#F97316' },
  Medium:   { bg: 'rgba(234,179,8,0.1)',   color: '#EAB308' },
  Low:      { bg: 'rgba(59,130,246,0.1)',  color: '#3B82F6' },
};

function AITriageSkeleton() {
  return (
    <div className="flex-1 overflow-y-auto px-6 py-5 animate-pulse" style={{ background: '#0B1020' }}>
      <div className="grid grid-cols-4 gap-4 mb-5">
        {Array.from({ length: 4 }).map((_, i) => (
          <div key={i} className="rounded-xl h-16" style={{ background: '#151B2F' }} />
        ))}
      </div>
      <div className="flex flex-col gap-3">
        {Array.from({ length: 4 }).map((_, i) => (
          <div key={i} className="rounded-2xl h-40" style={{ background: '#151B2F' }} />
        ))}
      </div>
    </div>
  );
}

export function AITriage() {
  const [statusFilter, setStatusFilter] = useState('pending');
  const [selected, setSelected] = useState<string | null>(null);

  // ✅ Server data — không hardcode
  const queueQuery = useAITriageQueue({ status: statusFilter === 'all' ? undefined : statusFilter });
  const reviewTriage = useReviewTriage();

  const handleDecision = async (findingId: string, decision: HumanDecision, note?: string) => {
    await reviewTriage.mutateAsync({ findingId, decision, note });
    setSelected(null);
  };

  return (
    <QueryBoundary query={queueQuery} skeleton={<AITriageSkeleton />}>
      {({ items, stats }) => (
        <div className="flex-1 overflow-y-auto px-6 py-5" style={{ background: '#0B1020' }}>
          {/* Header */}
          <div className="flex items-center gap-3 mb-5">
            <div className="w-9 h-9 rounded-xl flex items-center justify-center" style={{ background: 'rgba(167,139,250,0.1)' }}>
              <Brain size={18} color="#A78BFA" />
            </div>
            <div>
              <h2 style={{ color: '#E5E7EB', fontSize: 18, fontWeight: 700 }}>AI Triage Queue</h2>
              <p style={{ color: '#6B7280', fontSize: 12 }}>
                AI-powered vulnerability triage · {stats.pending} pending review
              </p>
            </div>
          </div>

          {/* Stats — computed từ server data */}
          <div className="grid grid-cols-4 gap-4 mb-5">
            {[
              { label: 'Pending Review', value: stats.pending, color: '#F59E0B' },
              { label: 'Accepted Today', value: stats.acceptedToday, color: '#10B981' },
              { label: 'Avg Confidence', value: `${stats.avgConfidence.toFixed(0)}%`, color: '#A78BFA' },
              { label: 'False Positive Rate', value: `${stats.falsePositiveRate.toFixed(1)}%`, color: '#4F8CFF' },
            ].map(stat => (
              <div key={stat.label} className="rounded-xl px-4 py-3"
                style={{ background: '#151B2F', border: '1px solid rgba(255,255,255,0.07)' }}>
                <div style={{ color: stat.color, fontSize: 20, fontWeight: 700 }}>{stat.value}</div>
                <div style={{ color: '#6B7280', fontSize: 11, marginTop: 2 }}>{stat.label}</div>
              </div>
            ))}
          </div>

          {/* Filter */}
          <div className="flex gap-2 mb-4">
            {['pending', 'accepted', 'overridden', 'all'].map(f => (
              <button key={f} onClick={() => setStatusFilter(f)}
                className="px-3 py-1.5 rounded-lg"
                style={{
                  background: statusFilter === f ? 'rgba(79,140,255,0.12)' : 'rgba(255,255,255,0.05)',
                  color: statusFilter === f ? '#4F8CFF' : '#6B7280',
                  fontSize: 12, border: 'none', cursor: 'pointer', textTransform: 'capitalize',
                }}>
                {f}
              </button>
            ))}
          </div>

          {/* Queue items — data từ server */}
          <div className="flex flex-col gap-3">
            {items.map(item => {
              const verdictConfig = VERDICT_CONFIG[item.aiResult.remarks];
              const sevStyle = SEVERITY_STYLES[item.severity] ?? SEVERITY_STYLES.Medium;
              const isSelected = selected === item.findingId;

              return (
                <div key={item.findingId} className="rounded-2xl p-5"
                  style={{
                    background: '#151B2F',
                    border: isSelected ? '1px solid rgba(79,140,255,0.3)' : '1px solid rgba(255,255,255,0.07)',
                  }}>
                  {/* Top row */}
                  <div className="flex items-center justify-between mb-3">
                    <div className="flex items-center gap-2">
                      <span style={{ color: '#6B7280', fontSize: 12 }}>{item.findingId}</span>
                      {item.cveId && <span style={{ color: '#4F8CFF', fontSize: 11 }}>· {item.cveId}</span>}
                      <span className="px-2 py-0.5 rounded" style={{ ...sevStyle, fontSize: 10 }}>{item.severity}</span>
                    </div>
                    <div className="flex items-center gap-2">
                      <span style={{ color: verdictConfig.color, fontSize: 12 }}>
                        {(item.aiResult.confidence * 100).toFixed(0)}% confidence
                      </span>
                      <span className="px-2 py-0.5 rounded" style={{ background: verdictConfig.bg, color: verdictConfig.color, fontSize: 11 }}>
                        {verdictConfig.label}
                      </span>
                    </div>
                  </div>

                  {/* Title */}
                  <div style={{ color: '#E5E7EB', fontSize: 14, fontWeight: 500, marginBottom: 8 }}>{item.findingTitle}</div>

                  {/* Confidence bar */}
                  <div className="mb-3">
                    <div className="flex justify-between mb-1">
                      <span style={{ color: '#9CA3AF', fontSize: 11 }}>AI Confidence</span>
                      <span style={{ color: verdictConfig.color, fontSize: 11, fontWeight: 600 }}>
                        {(item.aiResult.confidence * 100).toFixed(0)}%
                      </span>
                    </div>
                    <div className="h-1 rounded-full" style={{ background: 'rgba(255,255,255,0.08)' }}>
                      <div className="h-1 rounded-full transition-all"
                        style={{ width: `${item.aiResult.confidence * 100}%`, background: verdictConfig.color }} />
                    </div>
                  </div>

                  {/* AI Justification */}
                  <p style={{ color: '#9CA3AF', fontSize: 12, lineHeight: 1.6, marginBottom: 8 }}>
                    {item.aiResult.justification}
                  </p>

                  {/* Suggested fixes */}
                  {item.aiResult.actions.length > 0 && (
                    <div className="mb-4">
                      <div style={{ color: '#6B7280', fontSize: 11, fontWeight: 600, marginBottom: 6 }}>SUGGESTED ACTIONS</div>
                      <div className="flex flex-col gap-1.5">
                        {item.aiResult.actions.map((fix, idx) => (
                          <div key={idx} className="flex items-center gap-2">
                            <Zap size={11} color="#A78BFA" />
                            <span style={{ color: '#9CA3AF', fontSize: 12 }}>{fix}</span>
                          </div>
                        ))}
                      </div>
                    </div>
                  )}

                  {/* Human decision result or action buttons */}
                  {item.humanDecision ? (
                    <div className="flex items-center gap-2 px-3 py-2 rounded-xl"
                      style={{ background: 'rgba(255,255,255,0.04)', border: '1px solid rgba(255,255,255,0.07)' }}>
                      <CheckCircle size={13} color="#10B981" />
                      <span style={{ color: '#9CA3AF', fontSize: 12 }}>
                        {item.humanDecision === 'accepted' ? 'AI suggestion accepted' :
                          item.humanDecision === 'overridden' ? 'AI suggestion overridden' : 'Rejected'}
                        {item.reviewedBy && ` by ${item.reviewedBy}`}
                      </span>
                    </div>
                  ) : (
                    <div className="flex items-center gap-2 pt-2" style={{ borderTop: '1px solid rgba(255,255,255,0.06)' }}>
                      <button
                        onClick={() => handleDecision(item.findingId, 'accepted')}
                        disabled={reviewTriage.isPending}
                        className="flex items-center gap-1.5 px-3 py-1.5 rounded-lg"
                        style={{ background: 'rgba(16,185,129,0.12)', color: '#10B981', border: 'none', fontSize: 12, cursor: 'pointer' }}>
                        <CheckCircle size={12} />Accept AI
                      </button>
                      <button
                        onClick={() => handleDecision(item.findingId, 'overridden')}
                        disabled={reviewTriage.isPending}
                        className="flex items-center gap-1.5 px-3 py-1.5 rounded-lg"
                        style={{ background: 'rgba(245,158,11,0.12)', color: '#F59E0B', border: 'none', fontSize: 12, cursor: 'pointer' }}>
                        <AlertTriangle size={12} />Override
                      </button>
                      <button
                        onClick={() => handleDecision(item.findingId, 'rejected')}
                        disabled={reviewTriage.isPending}
                        className="flex items-center gap-1.5 px-3 py-1.5 rounded-lg"
                        style={{ background: 'rgba(239,68,68,0.08)', color: '#EF4444', border: 'none', fontSize: 12, cursor: 'pointer' }}>
                        <XCircle size={12} />Reject
                      </button>
                    </div>
                  )}
                </div>
              );
            })}
            {items.length === 0 && (
              <div className="text-center py-12" style={{ color: '#6B7280' }}>
                <Brain size={32} color="#374151" className="mx-auto mb-3" />
                <p>No items in triage queue</p>
              </div>
            )}
          </div>
        </div>
      )}
    </QueryBoundary>
  );
}
```

## MSW Handler

```typescript
// src/mocks/handlers/ai.handlers.ts
import { http, HttpResponse } from 'msw';

const triageQueueFixture = [
  {
    findingId: 'F-2847', findingTitle: 'Apache Log4j2 JNDI RCE', cveId: 'CVE-2025-44228', severity: 'Critical',
    aiResult: {
      remarks: 'Confirmed', confidence: 0.98,
      justification: 'CVSS 10.0, EPSS 98.2%, active CISA KEV. Host webserver01 runs Log4j2 2.14.0 (vulnerable). No compensating controls detected.',
      actions: ['Upgrade Log4j2 to 2.17.1+', 'Apply JVM flag: -Dlog4j2.formatMsgNoLookups=true', 'Enable WAF rule: CVE-2021-44228'],
      generatedAt: new Date().toISOString(),
    },
  },
  {
    findingId: 'F-2843', findingTitle: 'Information Disclosure - Stack Trace', severity: 'Medium',
    aiResult: {
      remarks: 'FalsePositive', confidence: 0.87,
      justification: 'Stack trace appears in non-production error page only. Header "X-Environment: dev" confirms staging instance. Not exposed to production traffic.',
      actions: ['Verify environment isolation', 'Disable debug mode in production config'],
      generatedAt: new Date().toISOString(),
    },
    humanDecision: 'accepted', reviewedBy: 'bob.chen@company.com', reviewedAt: new Date(Date.now() - 1800000).toISOString(),
  },
  // ... more items
];

export const aiHandlers = [
  http.get('/api/v1/ai/triage/queue', ({ request }) => {
    const url = new URL(request.url);
    const status = url.searchParams.get('status');

    let items = triageQueueFixture;
    if (status === 'pending') items = items.filter(i => !i.humanDecision);
    if (status === 'accepted') items = items.filter(i => i.humanDecision === 'accepted');

    const pending = triageQueueFixture.filter(i => !i.humanDecision).length;
    const acceptedToday = triageQueueFixture.filter(i => i.humanDecision === 'accepted').length;
    const avgConfidence = triageQueueFixture.reduce((s, i) => s + i.aiResult.confidence, 0) / triageQueueFixture.length * 100;

    return HttpResponse.json({
      items,
      total: items.length,
      stats: {
        pending,
        acceptedToday,
        avgConfidence: Math.round(avgConfidence),
        falsePositiveRate: 12.3,
      },
    });
  }),

  http.post('/api/v1/ai/triage/:findingId/review', async ({ params, request }) => {
    const body = await request.json() as any;
    const item = triageQueueFixture.find(i => i.findingId === params.findingId);
    if (item) {
      (item as any).humanDecision = body.decision;
      (item as any).reviewedAt = new Date().toISOString();
    }
    return HttpResponse.json({ success: true });
  }),
];
```
