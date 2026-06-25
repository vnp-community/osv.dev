# Solution 05 — RiskAcceptanceCenter.tsx

## Vấn đề
`const acceptances = [...]` — 5 risk acceptance giả, approve/reject button không có tác dụng.

## API Endpoints
```
GET    /api/v1/risk-acceptances              → Danh sách (ENDPOINTS.riskAcceptances.list)
POST   /api/v1/risk-acceptances              → Tạo mới
DELETE /api/v1/risk-acceptances/:id          → Revoke
```

## TypeScript Types (từ TDD.md Section 6.2)

```typescript
// shared/types/finding.ts — đã có, supplement
export interface RiskAcceptance {
  id: string;
  productId: string;
  productName: string;
  findingIds: string[];
  findingTitle: string;   // Từ finding đầu tiên
  cveId?: string;
  severity: Severity;
  expirationDate: string;
  retestDate?: string;
  reason: string;
  approvedBy: string;
  approvedById: string;
  isExpired: boolean;
  daysLeft: number;   // Computed: negative = overdue
  createdAt: string;
}

export interface RiskAcceptancesResponse {
  acceptances: RiskAcceptance[];
  total: number;
}
```

## Hook mới: `useRiskAcceptances`

```typescript
// features/findings/hooks/useRiskAcceptances.ts
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { apiClient } from '@/shared/api/client';
import { ENDPOINTS } from '@/shared/api/endpoints';
import type { RiskAcceptancesResponse } from '@/shared/types/finding';

const riskAcceptanceKeys = {
  all: ['risk-acceptances'] as const,
  list: (params?: Record<string, unknown>) => [...riskAcceptanceKeys.all, 'list', params] as const,
};

export function useRiskAcceptances(params?: { productId?: string; status?: 'active' | 'expired' }) {
  return useQuery<RiskAcceptancesResponse>({
    queryKey: riskAcceptanceKeys.list(params),
    queryFn: async () => {
      const { data } = await apiClient.get<RiskAcceptancesResponse>(
        ENDPOINTS.riskAcceptances.list, { params }
      );
      return data;
    },
    staleTime: 60_000,
  });
}

export function useRevokeRiskAcceptance() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (id: string) =>
      apiClient.delete(ENDPOINTS.riskAcceptances.delete(id)),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: riskAcceptanceKeys.all });
    },
  });
}
```

## Component sau khi fix

```typescript
// features/findings/components/RiskAcceptanceCenter.tsx
import { useState } from 'react';
import { Shield, AlertTriangle, Clock, CheckCircle, XCircle } from 'lucide-react';
import { QueryBoundary } from '@/shared/components/feedback/QueryBoundary';
import { useRiskAcceptances, useRevokeRiskAcceptance } from '../hooks/useRiskAcceptances';

// ── UI constants ────────────────────────────────────────────────────────────
const SEVERITY_STYLES: Record<string, { bg: string; color: string }> = {
  Critical: { bg: 'rgba(239,68,68,0.1)',   color: '#EF4444' },
  High:     { bg: 'rgba(249,115,22,0.1)',  color: '#F97316' },
  Medium:   { bg: 'rgba(234,179,8,0.1)',   color: '#EAB308' },
  Low:      { bg: 'rgba(59,130,246,0.1)',  color: '#3B82F6' },
};

function getDaysLeftStyle(daysLeft: number) {
  if (daysLeft < 0) return { color: '#EF4444', label: `${Math.abs(daysLeft)}d overdue` };
  if (daysLeft <= 30) return { color: '#F59E0B', label: `${daysLeft}d left` };
  return { color: '#10B981', label: `${daysLeft}d left` };
}

function RiskAcceptanceSkeleton() {
  return (
    <div className="flex-1 overflow-y-auto px-6 py-5 animate-pulse" style={{ background: '#0B1020' }}>
      <div className="grid grid-cols-3 gap-4 mb-5">
        {Array.from({ length: 3 }).map((_, i) => (
          <div key={i} className="rounded-xl h-16" style={{ background: '#151B2F' }} />
        ))}
      </div>
      <div className="flex flex-col gap-3">
        {Array.from({ length: 4 }).map((_, i) => (
          <div key={i} className="rounded-2xl h-28" style={{ background: '#151B2F' }} />
        ))}
      </div>
    </div>
  );
}

export function RiskAcceptanceCenter() {
  const [filter, setFilter] = useState<'all' | 'active' | 'expired'>('all');

  // ✅ Server data — không hardcode
  const acceptancesQuery = useRiskAcceptances(filter !== 'all' ? { status: filter } : undefined);
  const revokeAcceptance = useRevokeRiskAcceptance();

  return (
    <QueryBoundary query={acceptancesQuery} skeleton={<RiskAcceptanceSkeleton />}>
      {({ acceptances, total }) => {
        const expiredCount = acceptances.filter(a => a.isExpired).length;
        const expiringCount = acceptances.filter(a => !a.isExpired && a.daysLeft <= 30).length;
        const activeCount = total - expiredCount;

        return (
          <div className="flex-1 overflow-y-auto px-6 py-5" style={{ background: '#0B1020' }}>
            {/* Header */}
            <div className="flex items-center gap-3 mb-5">
              <div className="w-9 h-9 rounded-xl flex items-center justify-center" style={{ background: 'rgba(167,139,250,0.1)' }}>
                <Shield size={18} color="#A78BFA" />
              </div>
              <div>
                <h2 style={{ color: '#E5E7EB', fontSize: 18, fontWeight: 700 }}>Risk Acceptance Center</h2>
                <p style={{ color: '#6B7280', fontSize: 12 }}>{activeCount} active · {expiredCount} expired · {expiringCount} expiring soon</p>
              </div>
            </div>

            {/* Stats — computed từ server data */}
            <div className="grid grid-cols-3 gap-4 mb-5">
              <div className="rounded-xl px-4 py-3 flex items-center gap-3" style={{ background: 'rgba(16,185,129,0.08)', border: '1px solid rgba(16,185,129,0.2)' }}>
                <CheckCircle size={18} color="#10B981" />
                <div>
                  <div style={{ color: '#10B981', fontSize: 20, fontWeight: 700 }}>{activeCount}</div>
                  <div style={{ color: '#9CA3AF', fontSize: 11 }}>Active Acceptances</div>
                </div>
              </div>
              <div className="rounded-xl px-4 py-3 flex items-center gap-3" style={{ background: 'rgba(245,158,11,0.08)', border: '1px solid rgba(245,158,11,0.2)' }}>
                <Clock size={18} color="#F59E0B" />
                <div>
                  <div style={{ color: '#F59E0B', fontSize: 20, fontWeight: 700 }}>{expiringCount}</div>
                  <div style={{ color: '#9CA3AF', fontSize: 11 }}>Expiring in 30 days</div>
                </div>
              </div>
              <div className="rounded-xl px-4 py-3 flex items-center gap-3" style={{ background: 'rgba(239,68,68,0.08)', border: '1px solid rgba(239,68,68,0.2)' }}>
                <XCircle size={18} color="#EF4444" />
                <div>
                  <div style={{ color: '#EF4444', fontSize: 20, fontWeight: 700 }}>{expiredCount}</div>
                  <div style={{ color: '#9CA3AF', fontSize: 11 }}>Expired</div>
                </div>
              </div>
            </div>

            {/* Filter */}
            <div className="flex gap-2 mb-4">
              {(['all', 'active', 'expired'] as const).map(f => (
                <button key={f} onClick={() => setFilter(f)}
                  className="px-3 py-1.5 rounded-lg"
                  style={{
                    background: filter === f ? 'rgba(79,140,255,0.12)' : 'rgba(255,255,255,0.05)',
                    color: filter === f ? '#4F8CFF' : '#6B7280',
                    fontSize: 12, border: 'none', cursor: 'pointer', textTransform: 'capitalize',
                  }}>
                  {f}
                </button>
              ))}
            </div>

            {/* List — data từ server */}
            <div className="flex flex-col gap-3">
              {acceptances.map(a => {
                const daysStyle = getDaysLeftStyle(a.daysLeft);
                const sevStyle = SEVERITY_STYLES[a.severity] ?? SEVERITY_STYLES.Low;

                return (
                  <div key={a.id} className="rounded-2xl p-5"
                    style={{
                      background: '#151B2F',
                      border: a.isExpired ? '1px solid rgba(239,68,68,0.2)' : '1px solid rgba(255,255,255,0.07)',
                    }}>
                    <div className="flex items-start justify-between mb-3">
                      <div className="flex-1">
                        <div className="flex items-center gap-2 mb-1">
                          <span style={{ color: '#6B7280', fontSize: 11 }}>{a.id}</span>
                          {a.cveId && <span style={{ color: '#4F8CFF', fontSize: 11 }}>· {a.cveId}</span>}
                          <span className="px-2 py-0.5 rounded ml-1" style={{ ...sevStyle, fontSize: 10 }}>{a.severity}</span>
                          {a.isExpired && (
                            <div className="flex items-center gap-1 ml-2">
                              <AlertTriangle size={11} color="#EF4444" />
                              <span style={{ color: '#EF4444', fontSize: 10, fontWeight: 600 }}>EXPIRED</span>
                            </div>
                          )}
                        </div>
                        <div style={{ color: '#E5E7EB', fontSize: 14, fontWeight: 500, marginBottom: 4 }}>{a.findingTitle}</div>
                        <div style={{ color: '#6B7280', fontSize: 12 }}>{a.productName}</div>
                      </div>
                      <div className="text-right ml-4">
                        <div style={{ color: daysStyle.color, fontSize: 14, fontWeight: 700 }}>{daysStyle.label}</div>
                        <div style={{ color: '#4B5563', fontSize: 11, marginTop: 2 }}>Expires {new Date(a.expirationDate).toLocaleDateString()}</div>
                      </div>
                    </div>

                    <div className="rounded-xl p-3 mb-3" style={{ background: 'rgba(255,255,255,0.03)' }}>
                      <div style={{ color: '#6B7280', fontSize: 11, fontWeight: 600, marginBottom: 4 }}>JUSTIFICATION</div>
                      <p style={{ color: '#9CA3AF', fontSize: 12, lineHeight: 1.6 }}>{a.reason}</p>
                    </div>

                    <div className="flex items-center justify-between">
                      <span style={{ color: '#6B7280', fontSize: 11 }}>Approved by <span style={{ color: '#9CA3AF' }}>{a.approvedBy}</span></span>
                      <div className="flex gap-2">
                        {!a.isExpired && (
                          <button
                            onClick={() => revokeAcceptance.mutate(a.id)}
                            disabled={revokeAcceptance.isPending}
                            className="px-3 py-1.5 rounded-lg"
                            style={{ background: 'rgba(239,68,68,0.08)', color: '#EF4444', border: 'none', fontSize: 12, cursor: 'pointer' }}>
                            Revoke
                          </button>
                        )}
                        <button className="px-3 py-1.5 rounded-lg"
                          style={{ background: 'rgba(255,255,255,0.05)', color: '#9CA3AF', border: 'none', fontSize: 12, cursor: 'pointer' }}>
                          View Finding
                        </button>
                      </div>
                    </div>
                  </div>
                );
              })}

              {acceptances.length === 0 && (
                <div className="rounded-2xl p-8 text-center" style={{ background: '#151B2F', border: '1px solid rgba(255,255,255,0.07)' }}>
                  <Shield size={32} color="#374151" className="mx-auto mb-3" />
                  <div style={{ color: '#6B7280', fontSize: 14 }}>No risk acceptances found</div>
                </div>
              )}
            </div>
          </div>
        );
      }}
    </QueryBoundary>
  );
}
```

## MSW Handler

```typescript
// src/mocks/handlers/findings.handlers.ts — thêm vào
import { http, HttpResponse } from 'msw';

const now = Date.now();
const riskAcceptancesFixture = [
  {
    id: 'RA-012', productId: 'p-3', productName: 'DevOps Platform',
    findingIds: ['F-2841'], findingTitle: 'Kubernetes API Server Exposure',
    cveId: 'CVE-2025-40001', severity: 'High',
    expirationDate: new Date(now + 92 * 86400000).toISOString(),
    reason: 'Network controls (VPN-only access) mitigate the risk. Tracked in SEC-2047.',
    approvedBy: 'Carol Anderson', approvedById: 'u-1',
    isExpired: false, daysLeft: 92,
    createdAt: new Date(now - 30 * 86400000).toISOString(),
  },
  {
    id: 'RA-011', productId: 'p-1', productName: 'Banking App',
    findingIds: ['F-2835'], findingTitle: 'SSL Certificate Weak Cipher Suite',
    severity: 'Medium',
    expirationDate: new Date(now + 15 * 86400000).toISOString(),
    reason: 'Awaiting maintenance window Q3. Client confirmed low exploitability.',
    approvedBy: 'Bob Chen', approvedById: 'u-2',
    isExpired: false, daysLeft: 15,
    createdAt: new Date(now - 75 * 86400000).toISOString(),
  },
  {
    id: 'RA-009', productId: 'p-2', productName: 'Mobile App',
    findingIds: ['F-2820'], findingTitle: 'Outdated Third-party Library',
    severity: 'Low',
    expirationDate: new Date(now - 5 * 86400000).toISOString(),
    reason: 'Library vendor has not released a patch. Monitoring ongoing.',
    approvedBy: 'Carol Anderson', approvedById: 'u-1',
    isExpired: true, daysLeft: -5,
    createdAt: new Date(now - 95 * 86400000).toISOString(),
  },
];

export const riskAcceptanceHandlers = [
  http.get('/api/v1/risk-acceptances', ({ request }) => {
    const url = new URL(request.url);
    const status = url.searchParams.get('status');

    let filtered = riskAcceptancesFixture;
    if (status === 'active') filtered = filtered.filter(a => !a.isExpired);
    if (status === 'expired') filtered = filtered.filter(a => a.isExpired);

    return HttpResponse.json({ acceptances: filtered, total: filtered.length });
  }),

  http.delete('/api/v1/risk-acceptances/:id', ({ params }) => {
    const idx = riskAcceptancesFixture.findIndex(a => a.id === params.id);
    if (idx === -1) return new HttpResponse(null, { status: 404 });
    riskAcceptancesFixture.splice(idx, 1);
    return HttpResponse.json({ success: true });
  }),
];
```
