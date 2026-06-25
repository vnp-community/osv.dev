# Solution 02 — AuditLogs.tsx

## Vấn đề
`const auditLogs = [...]` — 10 audit events hardcode, không phân trang, filter chỉ local.

## API Endpoint
```
GET /api/v1/audit-log?severity=Critical&search=&page=1&pageSize=50
```
_(ENDPOINTS.audit.log đã có sẵn trong endpoints.ts)_

## TypeScript Types (từ TDD.md Section 11.2)

```typescript
// features/admin/types.ts — thêm vào file đã có
export interface AuditEvent {
  id: string;
  userId: string;
  userName: string;
  action: string;
  entityType: string;
  entityId: string;
  ipAddress: string;
  userAgent?: string;
  result: 'success' | 'failure';
  metadata?: Record<string, unknown>;
  timestamp: string;
  // Additional UI fields mapped from backend
  resource?: string;    // "Finding / F-2847"
  severity?: 'Info' | 'Warning' | 'Critical';
  before?: string;      // JSON string
  after?: string;       // JSON string
}

export interface AuditLogsResponse {
  events: AuditEvent[];
  total: number;
  page: number;
  pageSize: number;
}
```

## Hook mới: `useAuditLogs`

```typescript
// features/admin/hooks/useAuditLogs.ts
import { useQuery } from '@tanstack/react-query';
import { apiClient } from '@/shared/api/client';
import { ENDPOINTS } from '@/shared/api/endpoints';
import type { AuditLogsResponse } from '../types';

const auditKeys = {
  all: ['audit'] as const,
  list: (params?: Record<string, unknown>) => [...auditKeys.all, 'list', params] as const,
};

export interface AuditLogsParams {
  search?: string;
  severity?: 'Info' | 'Warning' | 'Critical';
  action?: string;
  userId?: string;
  dateFrom?: string;
  dateTo?: string;
  page?: number;
  pageSize?: number;
}

export function useAuditLogs(params?: AuditLogsParams) {
  return useQuery<AuditLogsResponse>({
    queryKey: auditKeys.list(params),
    queryFn: async () => {
      const { data } = await apiClient.get<AuditLogsResponse>(ENDPOINTS.audit.log, { params });
      return data;
    },
    staleTime: 30_000,  // 30s — audit log có thể thêm liên tục
  });
}
```

## Component sau khi fix

```typescript
// features/admin/components/AuditLogs.tsx
import { useState } from 'react';
import { Search, Download, Shield, User, Settings, AlertTriangle } from 'lucide-react';
import { useQuery } from '@tanstack/react-query';
import { apiClient } from '@/shared/api/client';
import { ENDPOINTS } from '@/shared/api/endpoints';
import { QueryBoundary } from '@/shared/components/feedback/QueryBoundary';
import type { AuditLogsResponse, AuditEvent } from '../types';

// ── UI constants (không phải data) ─────────────────────────────────────────
const SEVERITY_STYLES: Record<string, { bg: string; color: string }> = {
  Info:     { bg: 'rgba(79,140,255,0.1)',  color: '#4F8CFF' },
  Warning:  { bg: 'rgba(245,158,11,0.1)', color: '#F59E0B' },
  Critical: { bg: 'rgba(239,68,68,0.1)',  color: '#EF4444' },
};

const ACTION_ICON_MAP: Record<string, React.ElementType> = {
  CREATE: Settings, UPDATE: AlertTriangle, ASSIGN: User,
  CREATE_API_KEY: Shield, ACCEPT: Shield,
  DISABLE: User, SCAN: Settings, MARK: AlertTriangle,
};

function getActionIcon(action: string): React.ElementType {
  const key = Object.keys(ACTION_ICON_MAP).find(k => action.includes(k)) ?? 'CREATE';
  return ACTION_ICON_MAP[key] ?? Settings;
}

function AuditLogsSkeleton() {
  return (
    <div className="flex-1 overflow-y-auto px-6 py-5 animate-pulse" style={{ background: '#0B1020' }}>
      <div className="rounded-2xl h-64" style={{ background: '#151B2F' }} />
    </div>
  );
}

export function AuditLogs() {
  const [search, setSearch] = useState('');
  const [filterSeverity, setFilterSeverity] = useState('All');
  const [expanded, setExpanded] = useState<string | null>(null);
  const [page, setPage] = useState(1);

  // ✅ Server data — không hardcode
  const logsQuery = useQuery<AuditLogsResponse>({
    queryKey: ['audit', 'list', { search, filterSeverity, page }],
    queryFn: async () => {
      const { data } = await apiClient.get<AuditLogsResponse>(ENDPOINTS.audit.log, {
        params: {
          search: search || undefined,
          severity: filterSeverity !== 'All' ? filterSeverity : undefined,
          page,
          pageSize: 50,
        },
      });
      return data;
    },
    staleTime: 30_000,
  });

  return (
    <div className="flex-1 overflow-y-auto px-6 py-5" style={{ background: '#0B1020' }}>
      {/* Header */}
      <div className="flex items-center justify-between mb-5">
        <div>
          <h2 style={{ color: '#E5E7EB', fontSize: 18, fontWeight: 700 }}>Audit Logs</h2>
          <p style={{ color: '#6B7280', fontSize: 12 }}>Immutable audit trail · All user and system actions</p>
        </div>
        <button className="flex items-center gap-2 px-4 py-2 rounded-xl"
          style={{ background: 'rgba(255,255,255,0.05)', border: '1px solid rgba(255,255,255,0.09)', color: '#9CA3AF', fontSize: 13, cursor: 'pointer' }}>
          <Download size={14} />Export CSV
        </button>
      </div>

      {/* Filters */}
      <div className="flex items-center gap-3 mb-5">
        <div className="relative">
          <Search size={13} color="#4B5563" style={{ position: 'absolute', left: 10, top: '50%', transform: 'translateY(-50%)' }} />
          <input value={search} onChange={e => { setSearch(e.target.value); setPage(1); }}
            placeholder="Search actions, users..."
            className="rounded-xl pl-8 pr-4 py-2 outline-none"
            style={{ background: '#151B2F', border: '1px solid rgba(255,255,255,0.08)', color: '#E5E7EB', fontSize: 12, width: 240 }} />
        </div>
        {['All', 'Info', 'Warning', 'Critical'].map(s => (
          <button key={s} onClick={() => { setFilterSeverity(s); setPage(1); }}
            className="px-3 py-2 rounded-lg"
            style={{
              background: filterSeverity === s
                ? (s === 'All' ? 'rgba(79,140,255,0.12)' : SEVERITY_STYLES[s]?.bg)
                : 'rgba(255,255,255,0.05)',
              color: filterSeverity === s
                ? (s === 'All' ? '#4F8CFF' : SEVERITY_STYLES[s]?.color)
                : '#6B7280',
              fontSize: 12, border: 'none', cursor: 'pointer',
            }}>
            {s}
          </button>
        ))}
        {logsQuery.data && (
          <span style={{ color: '#6B7280', fontSize: 12, marginLeft: 'auto' }}>
            {logsQuery.data.total} events
          </span>
        )}
      </div>

      <QueryBoundary query={logsQuery} skeleton={<AuditLogsSkeleton />}>
        {({ events }) => (
          <div className="rounded-2xl overflow-hidden" style={{ background: '#151B2F', border: '1px solid rgba(255,255,255,0.07)' }}>
            <table className="w-full">
              <thead>
                <tr style={{ borderBottom: '1px solid rgba(255,255,255,0.06)' }}>
                  {['Timestamp', 'User', 'Action', 'Resource', 'Severity', ''].map(h => (
                    <th key={h} className="px-4 py-3 text-left" style={{ color: '#6B7280', fontSize: 11, fontWeight: 600, letterSpacing: 0.5 }}>{h}</th>
                  ))}
                </tr>
              </thead>
              <tbody>
                {events.map((log) => {
                  const Icon = getActionIcon(log.action);
                  const isExpanded = expanded === log.id;
                  const severityStyle = SEVERITY_STYLES[log.severity ?? 'Info'];
                  const resource = log.resource ?? `${log.entityType} / ${log.entityId}`;

                  return (
                    <>
                      <tr key={log.id} className="cursor-pointer transition-all"
                        style={{ borderBottom: '1px solid rgba(255,255,255,0.04)', background: isExpanded ? 'rgba(79,140,255,0.04)' : 'transparent' }}
                        onClick={() => setExpanded(isExpanded ? null : log.id)}
                        onMouseEnter={e => { if (!isExpanded) e.currentTarget.style.background = 'rgba(255,255,255,0.02)'; }}
                        onMouseLeave={e => { if (!isExpanded) e.currentTarget.style.background = 'transparent'; }}
                      >
                        <td className="px-4 py-3"><span style={{ color: '#6B7280', fontSize: 11, fontFamily: 'monospace' }}>{log.timestamp}</span></td>
                        <td className="px-4 py-3">
                          <div className="flex items-center gap-2">
                            <div className="w-5 h-5 rounded-full flex items-center justify-center"
                              style={{ background: log.userName === 'system' ? '#374151' : 'rgba(79,140,255,0.3)', fontSize: 9, color: 'white', fontWeight: 700 }}>
                              {log.userName === 'system' ? 'SYS' : log.userName.split('@')[0].slice(0, 2).toUpperCase()}
                            </div>
                            <span style={{ color: '#9CA3AF', fontSize: 12 }}>{log.userName}</span>
                          </div>
                        </td>
                        <td className="px-4 py-3">
                          <div className="flex items-center gap-2">
                            <Icon size={12} color="#6B7280" />
                            <span style={{ color: '#E5E7EB', fontSize: 12, fontFamily: 'monospace' }}>{log.action}</span>
                          </div>
                        </td>
                        <td className="px-4 py-3"><span style={{ color: '#6B7280', fontSize: 12 }}>{resource}</span></td>
                        <td className="px-4 py-3">
                          <span className="px-2 py-0.5 rounded" style={{ ...severityStyle, fontSize: 11 }}>
                            {log.severity ?? 'Info'}
                          </span>
                        </td>
                        <td className="px-4 py-3"><span style={{ color: '#4B5563', fontSize: 11 }}>{isExpanded ? '▲' : '▼'}</span></td>
                      </tr>
                      {isExpanded && log.metadata && (
                        <tr key={`${log.id}-detail`} style={{ borderBottom: '1px solid rgba(255,255,255,0.04)' }}>
                          <td colSpan={6} className="px-4 py-3">
                            <div className="grid grid-cols-2 gap-4">
                              {log.before && (
                                <div className="rounded-xl p-3" style={{ background: 'rgba(239,68,68,0.06)', border: '1px solid rgba(239,68,68,0.15)' }}>
                                  <div style={{ color: '#EF4444', fontSize: 10, fontWeight: 600, marginBottom: 6 }}>BEFORE</div>
                                  <pre style={{ color: '#FCA5A5', fontSize: 11, fontFamily: 'monospace' }}>
                                    {JSON.stringify(JSON.parse(log.before), null, 2)}
                                  </pre>
                                </div>
                              )}
                              {log.after && (
                                <div className="rounded-xl p-3" style={{ background: 'rgba(16,185,129,0.06)', border: '1px solid rgba(16,185,129,0.15)' }}>
                                  <div style={{ color: '#10B981', fontSize: 10, fontWeight: 600, marginBottom: 6 }}>AFTER</div>
                                  <pre style={{ color: '#A7F3D0', fontSize: 11, fontFamily: 'monospace' }}>
                                    {JSON.stringify(JSON.parse(log.after), null, 2)}
                                  </pre>
                                </div>
                              )}
                              {!log.before && !log.after && (
                                <div className="col-span-2 rounded-xl p-3" style={{ background: 'rgba(255,255,255,0.04)' }}>
                                  <pre style={{ color: '#9CA3AF', fontSize: 11 }}>
                                    {JSON.stringify(log.metadata, null, 2)}
                                  </pre>
                                </div>
                              )}
                            </div>
                          </td>
                        </tr>
                      )}
                    </>
                  );
                })}
              </tbody>
            </table>
          </div>
        )}
      </QueryBoundary>
    </div>
  );
}
```

## MSW Handler

```typescript
// src/mocks/handlers/audit.handlers.ts
import { http, HttpResponse } from 'msw';

const auditFixture = [
  { id: 'AL-1001', timestamp: '2026-06-14T09:42:15Z', userId: 'u-1', userName: 'carol@company.com',
    action: 'CREATE_SCAN', entityType: 'Scan', entityId: 'SC-0047',
    resource: 'Scan / SC-0047', severity: 'Info', ipAddress: '10.0.0.1', result: 'success',
    after: '{ "type": "NMAP", "target": "10.0.0.0/16" }' },
  { id: 'AL-1002', timestamp: '2026-06-14T09:35:01Z', userId: 'u-2', userName: 'bob.chen@company.com',
    action: 'UPDATE_FINDING', entityType: 'Finding', entityId: 'F-2847',
    resource: 'Finding / F-2847', severity: 'Warning', ipAddress: '10.0.0.2', result: 'success',
    before: '{ "status": "New" }', after: '{ "status": "Active" }' },
  // ... more fixtures
];

export const auditHandlers = [
  http.get('/api/v1/audit-log', ({ request }) => {
    const url = new URL(request.url);
    const search = url.searchParams.get('search')?.toLowerCase() ?? '';
    const severity = url.searchParams.get('severity');
    const page = parseInt(url.searchParams.get('page') ?? '1');
    const pageSize = parseInt(url.searchParams.get('pageSize') ?? '50');

    let filtered = auditFixture;
    if (search) {
      filtered = filtered.filter(l =>
        l.action.toLowerCase().includes(search) ||
        l.userName.toLowerCase().includes(search) ||
        l.resource?.toLowerCase().includes(search)
      );
    }
    if (severity) filtered = filtered.filter(l => l.severity === severity);

    const start = (page - 1) * pageSize;
    return HttpResponse.json({
      events: filtered.slice(start, start + pageSize),
      total: filtered.length,
      page,
      pageSize,
    });
  }),
];
```
