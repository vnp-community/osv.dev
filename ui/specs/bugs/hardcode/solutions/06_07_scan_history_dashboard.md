# Solution 06 — ScanHistory.tsx + 07 — ScanDashboard.tsx

## Solution 06: ScanHistory.tsx

### Vấn đề
`const history = [...]` — 8 scan records hardcode. Hook `useScans` **đã tồn tại** trong codebase nhưng chưa được dùng.

### API Endpoint
```
GET /api/v1/scans?status=completed,failed&page=1&pageSize=20
```
_(ENDPOINTS.scans.list đã có sẵn trong endpoints.ts)_

### Hook hiện có: `useScans`

Hook `useScans` đã được implement ở `features/scanning/hooks/useScans.ts`. Chỉ cần pass đúng params:

```typescript
// ✅ Đã có sẵn — chỉ cần sử dụng đúng
import { useScans } from '../hooks/useScans';

// Trong ScanHistory.tsx
const { data, isLoading } = useScans({ status: 'completed,failed', page });
```

### Component sau khi fix

```typescript
// features/scanning/components/ScanHistory.tsx
import { useState } from 'react';
import { Search, Filter } from 'lucide-react';
import { QueryBoundary } from '@/shared/components/feedback/QueryBoundary';
import { useScans } from '../hooks/useScans';

// ── UI constants (không phải data) ─────────────────────────────────────────
const STATUS_STYLES: Record<string, { bg: string; color: string }> = {
  completed:  { bg: 'rgba(16,185,129,0.1)',  color: '#10B981' },
  failed:     { bg: 'rgba(239,68,68,0.1)',   color: '#EF4444' },
  cancelled:  { bg: 'rgba(107,114,128,0.1)', color: '#9CA3AF' },
  running:    { bg: 'rgba(79,140,255,0.1)',  color: '#4F8CFF' },
};

const TYPE_LABELS: Record<string, string> = {
  nmap_full: 'NMAP', nmap_discovery: 'NMAP', zap: 'ZAP', agent: 'AGENT', import: 'IMPORT',
};

function ScanHistorySkeleton() {
  return (
    <div className="animate-pulse p-4">
      {Array.from({ length: 8 }).map((_, i) => (
        <div key={i} className="rounded-xl h-12 mb-2" style={{ background: '#151B2F' }} />
      ))}
    </div>
  );
}

export function ScanHistory() {
  const [search, setSearch] = useState('');
  const [typeFilter, setTypeFilter] = useState('all');
  const [page, setPage] = useState(1);

  // ✅ Dùng hook sẵn có — không hardcode
  const scansQuery = useScans({
    status: 'completed,failed,cancelled',
    search: search || undefined,
    type: typeFilter !== 'all' ? typeFilter : undefined,
    page,
    pageSize: 20,
  });

  return (
    <div className="flex-1 overflow-y-auto px-6 py-5" style={{ background: '#0B1020' }}>
      <div className="flex items-center justify-between mb-5">
        <h2 style={{ color: '#E5E7EB', fontSize: 18, fontWeight: 700 }}>Scan History</h2>
        <div className="flex items-center gap-3">
          {/* Search */}
          <div className="relative">
            <Search size={13} color="#4B5563" style={{ position: 'absolute', left: 10, top: '50%', transform: 'translateY(-50%)' }} />
            <input value={search} onChange={e => { setSearch(e.target.value); setPage(1); }}
              placeholder="Search scans..."
              className="rounded-xl pl-8 pr-4 py-2 outline-none"
              style={{ background: '#151B2F', border: '1px solid rgba(255,255,255,0.08)', color: '#E5E7EB', fontSize: 12, width: 200 }} />
          </div>
          {/* Type filter */}
          {['all', 'nmap_full', 'zap', 'agent'].map(t => (
            <button key={t} onClick={() => { setTypeFilter(t); setPage(1); }}
              className="px-3 py-1.5 rounded-lg"
              style={{
                background: typeFilter === t ? 'rgba(79,140,255,0.12)' : 'rgba(255,255,255,0.05)',
                color: typeFilter === t ? '#4F8CFF' : '#6B7280',
                fontSize: 12, border: 'none', cursor: 'pointer',
              }}>
              {t === 'all' ? 'All' : TYPE_LABELS[t]}
            </button>
          ))}
        </div>
      </div>

      <QueryBoundary query={scansQuery} skeleton={<ScanHistorySkeleton />}>
        {({ scans, total }) => (
          <>
            <div className="rounded-2xl overflow-hidden" style={{ background: '#151B2F', border: '1px solid rgba(255,255,255,0.07)' }}>
              <table className="w-full">
                <thead>
                  <tr style={{ borderBottom: '1px solid rgba(255,255,255,0.06)' }}>
                    {['ID', 'Name', 'Target', 'Type', 'Findings', 'Duration', 'Date', 'Status', ''].map(h => (
                      <th key={h} className="px-4 py-3 text-left" style={{ color: '#6B7280', fontSize: 11, fontWeight: 600, letterSpacing: 0.5 }}>{h}</th>
                    ))}
                  </tr>
                </thead>
                <tbody>
                  {scans.map((scan, i) => {
                    const statusStyle = STATUS_STYLES[scan.status] ?? STATUS_STYLES.completed;
                    const durationStr = scan.durationMs
                      ? new Date(scan.durationMs).toISOString().substr(11, 8)
                      : '—';
                    return (
                      <tr key={scan.id}
                        style={{ borderBottom: i < scans.length - 1 ? '1px solid rgba(255,255,255,0.04)' : 'none' }}
                        className="transition-all"
                        onMouseEnter={e => (e.currentTarget.style.background = 'rgba(255,255,255,0.02)')}
                        onMouseLeave={e => (e.currentTarget.style.background = 'transparent')}>
                        <td className="px-4 py-3"><span style={{ color: '#6B7280', fontSize: 12 }}>{scan.id}</span></td>
                        <td className="px-4 py-3"><span style={{ color: '#E5E7EB', fontSize: 13 }}>{scan.name}</span></td>
                        <td className="px-4 py-3"><span style={{ color: '#4F8CFF', fontSize: 11, fontFamily: 'monospace' }}>{scan.targets.join(', ')}</span></td>
                        <td className="px-4 py-3"><span className="px-2 py-0.5 rounded" style={{ background: 'rgba(255,255,255,0.06)', color: '#9CA3AF', fontSize: 10 }}>{TYPE_LABELS[scan.type] ?? scan.type}</span></td>
                        <td className="px-4 py-3"><span style={{ color: scan.findingCount > 0 ? '#F59E0B' : '#10B981', fontSize: 12, fontWeight: 600 }}>{scan.findingCount}</span></td>
                        <td className="px-4 py-3"><span style={{ color: '#6B7280', fontSize: 12, fontFamily: 'monospace' }}>{durationStr}</span></td>
                        <td className="px-4 py-3"><span style={{ color: '#6B7280', fontSize: 11 }}>{scan.completedAt ? new Date(scan.completedAt).toLocaleString() : '—'}</span></td>
                        <td className="px-4 py-3"><span className="px-2 py-0.5 rounded" style={{ ...statusStyle, fontSize: 11 }}>{scan.status}</span></td>
                        <td className="px-4 py-3">
                          {scan.status === 'completed' && (
                            <a href={`/scans/${scan.id}/results/${scan.type === 'zap' ? 'zap' : 'nmap'}`}
                              style={{ color: '#4F8CFF', fontSize: 11, textDecoration: 'none' }}>
                              View →
                            </a>
                          )}
                        </td>
                      </tr>
                    );
                  })}
                </tbody>
              </table>
            </div>

            {/* Pagination */}
            {total > 20 && (
              <div className="flex items-center justify-between mt-4">
                <span style={{ color: '#6B7280', fontSize: 12 }}>
                  Showing {(page - 1) * 20 + 1}–{Math.min(page * 20, total)} of {total}
                </span>
                <div className="flex gap-2">
                  <button disabled={page === 1} onClick={() => setPage(p => p - 1)}
                    className="px-3 py-1.5 rounded-lg"
                    style={{ background: 'rgba(255,255,255,0.05)', color: page === 1 ? '#374151' : '#9CA3AF', border: 'none', cursor: page === 1 ? 'not-allowed' : 'pointer', fontSize: 12 }}>
                    ← Prev
                  </button>
                  <button disabled={page * 20 >= total} onClick={() => setPage(p => p + 1)}
                    className="px-3 py-1.5 rounded-lg"
                    style={{ background: 'rgba(255,255,255,0.05)', color: page * 20 >= total ? '#374151' : '#9CA3AF', border: 'none', cursor: page * 20 >= total ? 'not-allowed' : 'pointer', fontSize: 12 }}>
                    Next →
                  </button>
                </div>
              </div>
            )}
          </>
        )}
      </QueryBoundary>
    </div>
  );
}
```

---

## Solution 07: ScanDashboard.tsx

### Vấn đề
`Math.random()` cho weekly activity chart — thay đổi mỗi render, không phản ánh thực tế.

### API Endpoint mới cần thêm
```
GET /api/v1/scans/stats          → KPI stats (running, completed_today, findings, scheduled)
GET /api/v1/scans/stats/weekly   → Weekly activity chart data
```

### Hook mới: `useScanStats`

```typescript
// features/scanning/hooks/useScanStats.ts
import { useQuery } from '@tanstack/react-query';
import { apiClient } from '@/shared/api/client';

export interface ScanStats {
  activeScans: number;
  completedToday: number;
  totalFindings: number;
  scheduledScans: number;
}

export interface WeeklyActivity {
  day: string;    // 'Mon', 'Tue', ...
  scans: number;
  findings: number;
}

export function useScanStats() {
  return useQuery<ScanStats>({
    queryKey: ['scans', 'stats'],
    queryFn: async () => {
      const { data } = await apiClient.get<ScanStats>('/api/v1/scans/stats');
      return data;
    },
    staleTime: 30_000,
    refetchInterval: 30_000,  // Auto-refresh mỗi 30s
  });
}

export function useWeeklyScanActivity() {
  return useQuery<WeeklyActivity[]>({
    queryKey: ['scans', 'stats', 'weekly'],
    queryFn: async () => {
      const { data } = await apiClient.get<WeeklyActivity[]>('/api/v1/scans/stats/weekly');
      return data;
    },
    staleTime: 5 * 60_000,   // 5 phút — weekly data ít thay đổi
  });
}
```

### Component sau khi fix (phần thay thế Math.random)

```typescript
// features/scanning/components/ScanDashboard.tsx
// Thay thế đoạn hardcode weeklyActivity bằng:

import { useScanStats, useWeeklyScanActivity } from '../hooks/useScanStats';
import { useScans } from '../hooks/useScans';

export function ScanDashboard() {
  const statsQuery = useScanStats();
  const weeklyQuery = useWeeklyScanActivity();
  const runningScansQuery = useScans({ status: 'running' });
  const scheduledScansQuery = useScans({ status: 'scheduled' }); // Nếu có

  // ... render với QueryBoundary
  // weeklyActivity = weeklyQuery.data ?? [] (stable, không random)
}
```

### MSW Handler

```typescript
// src/mocks/handlers/scan.handlers.ts — thêm vào
http.get('/api/v1/scans/stats', () => {
  return HttpResponse.json({
    activeScans: 2,
    completedToday: 8,
    totalFindings: 347,
    scheduledScans: 3,
  });
}),

http.get('/api/v1/scans/stats/weekly', () => {
  // Dữ liệu stable — không random
  return HttpResponse.json([
    { day: 'Mon', scans: 3, findings: 42 },
    { day: 'Tue', scans: 5, findings: 78 },
    { day: 'Wed', scans: 4, findings: 55 },
    { day: 'Thu', scans: 7, findings: 93 },
    { day: 'Fri', scans: 6, findings: 67 },
    { day: 'Sat', scans: 2, findings: 28 },
    { day: 'Sun', scans: 1, findings: 15 },
  ]);
}),
```

### Lưu ý: Thêm endpoint vào endpoints.ts

```typescript
// shared/api/endpoints.ts — thêm vào scans object
scans: {
  // ... existing endpoints
  stats:        '/api/v1/scans/stats',
  statsWeekly:  '/api/v1/scans/stats/weekly',
},
```
