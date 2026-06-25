# SOL-UI-005 — Frontend Solution: Finding Management API

**CR nguồn:** [CR-UI-005](../../../../../specs/crs/v0/ui-api/CR-UI-005-finding-api.md)  
**Ngày tạo:** 2026-06-16  
**Trạng thái:** Proposed  
**Ưu tiên:** P0 — Critical  
**Phạm vi:** Frontend React SPA (`ui/src/features/findings/`)

---

## 1. Tóm tắt giải pháp

CR-UI-005 bao phủ Finding Management với 4 screens. Frontend cần:

1. `findingApi.ts` — CRUD findings, bulk ops, audit, notes, risk acceptances
2. `useFindings.ts` — React Query với advanced URL-state filters
3. `useFindingDetail.ts` — Detail + audit trail
4. `useBulkOps.ts` — Bulk close/reopen/assign mutations
5. `useRiskAcceptances.ts` — Risk acceptance CRUD
6. Finding state machine guard — validate transitions trước khi call API

---

## 2. File Structure

```
ui/src/
├── features/findings/
│   ├── api/
│   │   ├── findingApi.ts
│   │   └── riskAcceptanceApi.ts
│   ├── hooks/
│   │   ├── useFindings.ts         # List với URL filters
│   │   ├── useFindingDetail.ts    # Single finding
│   │   ├── useFindingAudit.ts     # Audit trail
│   │   ├── useBulkOps.ts          # Bulk operations
│   │   └── useRiskAcceptances.ts
│   ├── components/
│   │   ├── FindingsList.tsx
│   │   ├── FindingDetail.tsx
│   │   ├── FindingFilters.tsx
│   │   ├── BulkActionsBar.tsx
│   │   ├── AuditTrail.tsx
│   │   ├── RiskAcceptanceCenter.tsx
│   │   └── SLABadge.tsx
│   ├── utils/
│   │   └── findingStateMachine.ts  # Valid transitions map
│   └── types.ts
│
└── mocks/handlers/
    └── finding.handlers.ts
```

---

## 3. Implementation Chi Tiết

### 3.1 `features/findings/api/findingApi.ts`

```typescript
import apiClient from '@/shared/api/client';
import type {
  FindingListResponse, Finding, PatchFindingRequest,
  BulkCloseRequest, BulkResponse, AuditEvent, Note
} from '../types';

export const findingApi = {
  // GET /api/v1/findings
  list: async (params: {
    status?: string[];
    severity?: string[];
    product_id?: string;
    engagement_id?: string;
    cve_id?: string;
    sla_status?: string;
    assigned_to?: string;
    is_kev?: boolean;
    date_from?: string;
    date_to?: string;
    page?: number;
    page_size?: number;
    sort_by?: string;
    q?: string;
  }): Promise<FindingListResponse> => {
    const { data } = await apiClient.get<FindingListResponse>('/api/v1/findings', {
      params: {
        ...params,
        status: params.status?.join(','),
        severity: params.severity?.join(','),
      },
    });
    return data;
  },

  // GET /api/v1/findings/stats
  getStats: async (productId?: string) => {
    const { data } = await apiClient.get('/api/v1/findings/stats', {
      params: { product_id: productId },
    });
    return data;
  },

  // GET /api/v1/findings/{id}
  getById: async (id: string): Promise<Finding> => {
    const { data } = await apiClient.get<Finding>(`/api/v1/findings/${id}`);
    return data;
  },

  // PATCH /api/v1/findings/{id}
  patch: async (id: string, payload: PatchFindingRequest): Promise<Finding> => {
    const { data } = await apiClient.patch<Finding>(`/api/v1/findings/${id}`, payload);
    return data;
  },

  // POST /api/v1/findings/{id}/notes
  addNote: async (id: string, content: string): Promise<Note> => {
    const { data } = await apiClient.post<Note>(`/api/v1/findings/${id}/notes`, { content });
    return data;
  },

  // GET /api/v1/findings/{id}/audit
  getAudit: async (id: string): Promise<{ audits: AuditEvent[] }> => {
    const { data } = await apiClient.get(`/api/v1/findings/${id}/audit`);
    return data;
  },

  // POST /api/v1/findings/bulk/close
  bulkClose: async (payload: BulkCloseRequest): Promise<BulkResponse> => {
    const { data } = await apiClient.post('/api/v1/findings/bulk/close', payload);
    return data;
  },

  // POST /api/v1/findings/bulk/reopen
  bulkReopen: async (payload: BulkCloseRequest): Promise<BulkResponse> => {
    const { data } = await apiClient.post('/api/v1/findings/bulk/reopen', payload);
    return data;
  },

  // POST /api/v1/findings/bulk/assign
  bulkAssign: async (payload: { finding_ids: string[]; assigned_to: string }): Promise<BulkResponse> => {
    const { data } = await apiClient.post('/api/v1/findings/bulk/assign', payload);
    return data;
  },
};
```

### 3.2 `features/findings/utils/findingStateMachine.ts`

```typescript
import type { FindingStatus } from '../types';

// Valid transitions theo CR-UI-005 §2.3
export const VALID_TRANSITIONS: Record<FindingStatus, FindingStatus[]> = {
  active: ['mitigated', 'false_positive', 'risk_accepted', 'out_of_scope', 'duplicate'],
  mitigated: ['active'],               // Reopen
  false_positive: ['active'],          // Reopen
  risk_accepted: ['active'],           // Reopen (RA expired)
  out_of_scope: ['active'],
  duplicate: [],                       // Terminal state
};

export function canTransition(from: FindingStatus, to: FindingStatus): boolean {
  return VALID_TRANSITIONS[from]?.includes(to) ?? false;
}

// Status action labels cho UI buttons
export const STATUS_ACTIONS: Partial<Record<FindingStatus, string>> = {
  mitigated: 'Mark as Mitigated',
  false_positive: 'Mark as False Positive',
  risk_accepted: 'Accept Risk',
  out_of_scope: 'Mark Out of Scope',
  active: 'Reopen',
};

export function getAvailableActions(currentStatus: FindingStatus): FindingStatus[] {
  return VALID_TRANSITIONS[currentStatus] ?? [];
}
```

### 3.3 `features/findings/hooks/useFindings.ts`

```typescript
import { useSearchParams } from 'react-router-dom';
import { useQuery } from '@tanstack/react-query';
import { findingApi } from '../api/findingApi';
import type { FindingListResponse } from '../types';

export const findingKeys = {
  all: ['findings'] as const,
  list: (params: object) => [...findingKeys.all, 'list', params] as const,
  detail: (id: string) => [...findingKeys.all, 'detail', id] as const,
  audit: (id: string) => [...findingKeys.all, 'audit', id] as const,
  stats: (productId?: string) => [...findingKeys.all, 'stats', productId] as const,
};

export function useFindings() {
  const [searchParams, setSearchParams] = useSearchParams();

  const params = {
    status: searchParams.getAll('status'),
    severity: searchParams.getAll('severity'),
    product_id: searchParams.get('product_id') || undefined,
    sla_status: searchParams.get('sla_status') || undefined,
    is_kev: searchParams.get('is_kev') === 'true' ? true : undefined,
    q: searchParams.get('q') || undefined,
    page: Number(searchParams.get('page') || '1'),
    page_size: Number(searchParams.get('page_size') || '50'),
    sort_by: searchParams.get('sort_by') || 'severity_desc',
  };

  const query = useQuery<FindingListResponse>({
    queryKey: findingKeys.list(params),
    queryFn: () => findingApi.list(params),
    staleTime: 30_000,
    placeholderData: (prev) => prev,
  });

  const setFilter = (key: string, value: string | string[] | undefined) => {
    setSearchParams((prev) => {
      const next = new URLSearchParams(prev);
      if (!value || (Array.isArray(value) && value.length === 0)) {
        next.delete(key);
      } else if (Array.isArray(value)) {
        next.delete(key);
        value.forEach(v => next.append(key, v));
      } else {
        next.set(key, value);
      }
      next.set('page', '1');
      return next;
    });
  };

  return { ...query, params, setFilter };
}
```

### 3.4 `features/findings/hooks/useBulkOps.ts`

```typescript
import { useMutation, useQueryClient } from '@tanstack/react-query';
import { findingApi } from '../api/findingApi';
import { findingKeys } from './useFindings';
import toast from 'react-hot-toast';

export function useBulkOps() {
  const queryClient = useQueryClient();

  const invalidateAll = () => {
    queryClient.invalidateQueries({ queryKey: findingKeys.all });
  };

  const bulkClose = useMutation({
    mutationFn: findingApi.bulkClose,
    onSuccess: (data) => {
      toast.success(`Closed ${data.success_count} findings`);
      invalidateAll();
    },
  });

  const bulkReopen = useMutation({
    mutationFn: findingApi.bulkReopen,
    onSuccess: (data) => {
      toast.success(`Reopened ${data.success_count} findings`);
      invalidateAll();
    },
  });

  const bulkAssign = useMutation({
    mutationFn: findingApi.bulkAssign,
    onSuccess: (data) => {
      toast.success(`Assigned ${data.success_count} findings`);
      invalidateAll();
    },
  });

  return { bulkClose, bulkReopen, bulkAssign };
}
```

### 3.5 Finding Detail — Status Action với State Machine

```tsx
// features/findings/components/FindingDetail.tsx (excerpt)
export function FindingDetail({ findingId }: { findingId: string }) {
  const { data: finding } = useQuery({
    queryKey: findingKeys.detail(findingId),
    queryFn: () => findingApi.getById(findingId),
    staleTime: 30_000,
  });
  const queryClient = useQueryClient();

  const patchFinding = useMutation({
    mutationFn: (payload: PatchFindingRequest) => findingApi.patch(findingId, payload),
    onSuccess: (updated) => {
      // Optimistic update cache
      queryClient.setQueryData(findingKeys.detail(findingId), updated);
      queryClient.invalidateQueries({ queryKey: findingKeys.list({}) });
      toast.success('Finding updated');
    },
    onError: (error: any) => {
      if (error.response?.data?.error === 'INVALID_TRANSITION') {
        toast.error(`Invalid transition: ${error.response.data.message}`);
      }
    },
  });

  const availableActions = finding ? getAvailableActions(finding.status) : [];

  // Status Actions Bar
  return (
    <div className="space-y-4">
      <div className="flex gap-2">
        {availableActions.map(targetStatus => (
          <Button
            key={targetStatus}
            variant={targetStatus === 'active' ? 'outline' : 'default'}
            onClick={() => patchFinding.mutate({
              status: targetStatus,
              comment: `Marked as ${targetStatus}`,
            })}
            isLoading={patchFinding.isPending}
          >
            {STATUS_ACTIONS[targetStatus]}
          </Button>
        ))}
      </div>
      {/* ... rest of detail */}
    </div>
  );
}
```

---

## 4. Findings List — Component Template

```tsx
// features/findings/components/FindingsList.tsx
export function FindingsList() {
  const [selectedIds, setSelectedIds] = useState<Set<string>>(new Set());
  const { data, isLoading, isError, error, refetch, params, setFilter } = useFindings();
  const { canWriteFindings } = usePermissions();

  if (isLoading) return <FindingsListSkeleton />;
  if (isError) return <ErrorState message={error?.message} onRetry={refetch} />;
  if (!data?.findings.length) return <EmptyState title="No findings found" description="Try adjusting your filters." />;

  return (
    <div className="flex flex-col h-full">
      {/* Stats bar */}
      <FindingsStatsBar stats={data.by_severity} slaStats={data.sla_stats} />

      {/* Filter tabs (status) */}
      <StatusTabs
        byStatus={data.by_status}
        activeStatuses={params.status}
        onChange={(statuses) => setFilter('status', statuses)}
      />

      {/* Filters */}
      <FindingFilters params={params} onFilterChange={setFilter} />

      {/* Bulk actions bar — only visible when rows selected */}
      {selectedIds.size > 0 && canWriteFindings && (
        <BulkActionsBar
          selectedIds={[...selectedIds]}
          onClear={() => setSelectedIds(new Set())}
        />
      )}

      {/* Table */}
      <DataTable
        columns={FINDINGS_COLUMNS}
        data={data.findings}    // ← Từ server, không hardcode
        total={data.total}
        page={params.page}
        pageSize={params.page_size}
        selectedIds={selectedIds}
        onSelectChange={setSelectedIds}
        onRowClick={(row) => navigate(`/findings/${row.id}`)}
      />
    </div>
  );
}
```

---

## 5. Risk Acceptance — `api/riskAcceptanceApi.ts`

```typescript
import apiClient from '@/shared/api/client';

export const riskAcceptanceApi = {
  list: async (params: { product_id?: string; is_expired?: boolean; page?: number }) => {
    const { data } = await apiClient.get('/api/v1/risk-acceptances', { params });
    return data;
  },

  create: async (payload: {
    product_id: string;
    finding_ids: string[];
    expiration_date: string;
    retest_date?: string;
    reason: string;
    approved_by: string;
  }) => {
    const { data } = await apiClient.post('/api/v1/risk-acceptances', payload);
    return data;
  },

  delete: async (id: string) => {
    const { data } = await apiClient.delete(`/api/v1/risk-acceptances/${id}`);
    return data;
  },
};
```

---

## 6. Acceptance Criteria (Frontend)

- [ ] Findings list gọi `GET /api/v1/findings` — không hardcode
- [ ] Status tab filter (Active/Mitigated/FP/RA/OOS/Duplicate) → URL params
- [ ] Filter by severity, product, SLA status → URL params → re-fetch
- [ ] `q=log4j` search → URL param → re-fetch
- [ ] Bulk select rows → BulkActionsBar hiển thị
- [ ] Bulk close → `POST /api/v1/findings/bulk/close` → invalidate list
- [ ] Finding detail: Status action buttons chỉ hiển thị valid transitions
- [ ] Invalid transition → 409 response → toast error (không crash)
- [ ] Add note → `POST /api/v1/findings/{id}/notes` → note xuất hiện ngay
- [ ] Audit trail hiển thị theo thứ tự mới nhất trước
- [ ] Risk Acceptance: create → findings liên quan chuyển `risk_accepted`
- [ ] SLA Badge thay đổi màu: đỏ (breached), vàng (at_risk), xanh (ok)
