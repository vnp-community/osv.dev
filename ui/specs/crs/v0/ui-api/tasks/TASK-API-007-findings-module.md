# TASK-API-007 — Findings Module: findingApi + State Machine + Hooks + Bulk Ops

| Field | Value |
|-------|-------|
| **Task ID** | TASK-API-007 |
| **Module** | `ui/src/features/findings/` |
| **Solution Ref** | [SOL-UI-005 §3](../solutions/SOL-UI-005-finding-api.md) |
| **Priority** | 🔴 P0 |
| **Depends On** | TASK-API-003 |
| **Estimated** | 3h |

---

## Context

Findings module có state machine phức tạp: một finding chỉ có thể transition sang states hợp lệ (e.g., `active → mitigated`, `mitigated → active`, không thể `duplicate → anything`). Frontend phải validate transition TRƯỚC KHI gọi API để tránh lỗi 409.

Ngoài ra, bulk operations (close/reopen/assign) phải cập nhật UI optimistically.

---

## Goal

Tạo Findings feature module với: API client, 5 hooks, bulk operations với optimistic update.

---

## Target Files

| Action | File Path |
|--------|-----------|
| CREATE | `ui/src/features/findings/types.ts` |
| CREATE | `ui/src/features/findings/api/findingApi.ts` |
| CREATE | `ui/src/features/findings/hooks/useFindings.ts` |
| CREATE | `ui/src/features/findings/hooks/useFindingDetail.ts` |
| CREATE | `ui/src/features/findings/hooks/useUpdateFinding.ts` |
| CREATE | `ui/src/features/findings/hooks/useBulkOps.ts` |
| CREATE | `ui/src/features/findings/hooks/useFindingNotes.ts` |

---

## Implementation

### File 1: `ui/src/features/findings/types.ts`

```typescript
import type { Severity } from '@/features/cve-intel/types';

export type FindingStatus =
  | 'active'
  | 'mitigated'
  | 'false_positive'
  | 'risk_accepted'
  | 'out_of_scope'
  | 'duplicate';

export type SLAStatus = 'ok' | 'at_risk' | 'breached';

export interface FindingNote {
  id: string;
  author_id: string;
  author_name: string;
  content: string;
  created_at: string;
  updated_at: string;
}

export interface AuditEvent {
  id: string;
  user_id: string;
  user_name: string;
  action: string;
  from_value: string | null;
  to_value: string | null;
  note: string | null;
  timestamp: string;
}

export interface RiskAcceptance {
  id: string;
  finding_id: string;
  accepted_by: string;
  accepted_by_name: string;
  reason: string;
  expires_at: string | null;
  created_at: string;
}

export interface Finding {
  id: string;
  title: string;
  description: string;
  severity: Severity;
  cvss_v3: number | null;
  cve_id: string | null;
  cwe_id: string | null;
  status: FindingStatus;
  asset_id: string;
  asset_name: string;
  product_id: string | null;
  product_name: string | null;
  engagement_id: string | null;
  scanner: string;
  endpoint: string | null;
  sla_expiration_date: string;
  sla_status: SLAStatus;
  days_overdue: number | null;
  duplicate_of: string | null;
  assigned_to: string | null;
  assigned_to_name: string | null;
  risk_acceptance: RiskAcceptance | null;
  is_kev: boolean;
  epss_score: number | null;
  created_at: string;
  updated_at: string;
}

export interface FindingListResponse {
  findings: Finding[];
  total: number;
  page: number;
  page_size: number;
}

export interface FindingStats {
  by_severity: { critical: number; high: number; medium: number; low: number };
  by_status:   { active: number; mitigated: number; false_positive: number; risk_accepted: number };
  sla_summary: { ok: number; at_risk: number; breached: number };
  sla_compliance_pct: number;
}

export interface FindingListParams {
  q?: string;
  severity?: Severity[];
  status?: FindingStatus[];
  asset_id?: string;
  product_id?: string;
  cve_id?: string;
  sla_status?: SLAStatus;
  is_kev?: boolean;
  assigned_to?: string;
  page?: number;
  page_size?: number;
  sort_by?: 'severity_desc' | 'cvss_desc' | 'epss_desc' | 'sla_asc' | 'created_desc';
}

export interface UpdateFindingPayload {
  status?: FindingStatus;
  note?: string;
  assigned_to?: string | null;
  cvss_v3?: number | null;
  severity?: Severity;
}

export interface BulkClosePayload {
  finding_ids: string[];
  status: 'mitigated' | 'false_positive' | 'risk_accepted' | 'out_of_scope';
  note?: string;
}

export interface BulkAssignPayload {
  finding_ids: string[];
  assigned_to: string;
}
```

### File 2: `ui/src/features/findings/api/findingApi.ts`

```typescript
import { apiClient } from '@/shared/api/client';
import { ENDPOINTS } from '@/shared/api/endpoints';
import type {
  Finding, FindingListResponse, FindingStats, FindingNote, AuditEvent,
  FindingListParams, UpdateFindingPayload, BulkClosePayload, BulkAssignPayload,
} from '../types';

export const findingApi = {
  // GET /api/v1/findings
  list: async (params: FindingListParams = {}): Promise<FindingListResponse> => {
    const { data } = await apiClient.get<FindingListResponse>(ENDPOINTS.findings.list, {
      params: {
        ...params,
        severity:    params.severity?.join(','),
        status:      params.status?.join(','),
        sla_status:  params.sla_status,
      },
    });
    return data;
  },

  // GET /api/v1/findings/stats
  getStats: async (params: { product_id?: string; asset_id?: string } = {}): Promise<FindingStats> => {
    const { data } = await apiClient.get<FindingStats>(ENDPOINTS.findings.stats, { params });
    return data;
  },

  // GET /api/v1/findings/{id}
  getById: async (id: string): Promise<Finding> => {
    const { data } = await apiClient.get<Finding>(ENDPOINTS.findings.detail(id));
    return data;
  },

  // PATCH /api/v1/findings/{id}
  update: async (id: string, payload: UpdateFindingPayload): Promise<Finding> => {
    const { data } = await apiClient.patch<Finding>(ENDPOINTS.findings.patch(id), payload);
    return data;
  },

  // GET /api/v1/findings/{id}/notes
  getNotes: async (id: string): Promise<{ notes: FindingNote[]; total: number }> => {
    const { data } = await apiClient.get(ENDPOINTS.findings.notes(id));
    return data as { notes: FindingNote[]; total: number };
  },

  // POST /api/v1/findings/{id}/notes
  addNote: async (id: string, content: string): Promise<FindingNote> => {
    const { data } = await apiClient.post<FindingNote>(ENDPOINTS.findings.notes(id), { content });
    return data;
  },

  // GET /api/v1/findings/{id}/audit
  getAudit: async (id: string): Promise<{ events: AuditEvent[]; total: number }> => {
    const { data } = await apiClient.get(ENDPOINTS.findings.audit(id));
    return data as { events: AuditEvent[]; total: number };
  },

  // POST /api/v1/findings/bulk/close
  bulkClose: async (payload: BulkClosePayload): Promise<{
    processed: number; failed: number; failed_ids: string[];
  }> => {
    const { data } = await apiClient.post(ENDPOINTS.findings.bulkClose, payload);
    return data as { processed: number; failed: number; failed_ids: string[] };
  },

  // POST /api/v1/findings/bulk/reopen
  bulkReopen: async (findingIds: string[]): Promise<{ processed: number; failed: number }> => {
    const { data } = await apiClient.post(ENDPOINTS.findings.bulkReopen, { finding_ids: findingIds });
    return data as { processed: number; failed: number };
  },

  // POST /api/v1/findings/bulk/assign
  bulkAssign: async (payload: BulkAssignPayload): Promise<{ processed: number }> => {
    const { data } = await apiClient.post(ENDPOINTS.findings.bulkAssign, payload);
    return data as { processed: number };
  },
};
```

### File 3: `ui/src/features/findings/hooks/useFindings.ts`

```typescript
import { useSearchParams } from 'react-router-dom';
import { useQuery } from '@tanstack/react-query';
import { findingApi } from '../api/findingApi';
import type { FindingListParams, Severity, FindingStatus, SLAStatus } from '../types';

export const findingKeys = {
  all:    ['findings'] as const,
  list:   (params: FindingListParams) => ['findings', 'list', params] as const,
  detail: (id: string) => ['findings', 'detail', id] as const,
  notes:  (id: string) => ['findings', 'notes', id] as const,
  audit:  (id: string) => ['findings', 'audit', id] as const,
  stats:  (params: object) => ['findings', 'stats', params] as const,
};

export function useFindings() {
  const [searchParams, setSearchParams] = useSearchParams();

  const params: FindingListParams = {
    q:          searchParams.get('q') || undefined,
    severity:   searchParams.getAll('severity') as Severity[],
    status:     searchParams.getAll('status') as FindingStatus[],
    product_id: searchParams.get('product_id') || undefined,
    asset_id:   searchParams.get('asset_id') || undefined,
    sla_status: (searchParams.get('sla_status') as SLAStatus) || undefined,
    is_kev:     searchParams.get('is_kev') === 'true' ? true : undefined,
    assigned_to:searchParams.get('assigned_to') || undefined,
    page:       Number(searchParams.get('page') || '1'),
    page_size:  Number(searchParams.get('page_size') || '20'),
    sort_by:    (searchParams.get('sort_by') as FindingListParams['sort_by']) || 'severity_desc',
  };

  const query = useQuery({
    queryKey:        findingKeys.list(params),
    queryFn:         () => findingApi.list(params),
    staleTime:       30_000,
    placeholderData: (prev) => prev,
  });

  const setFilter = <K extends keyof FindingListParams>(key: K, value: FindingListParams[K]) => {
    setSearchParams((prev) => {
      const next = new URLSearchParams(prev);
      if (!value || (Array.isArray(value) && value.length === 0)) {
        next.delete(key);
      } else if (Array.isArray(value)) {
        next.delete(key);
        (value as string[]).forEach(v => next.append(key, v));
      } else {
        next.set(key, String(value));
      }
      next.set('page', '1');
      return next;
    });
  };

  return { ...query, params, setFilter };
}

export function useFindingStats(params: { productId?: string } = {}) {
  const queryParams = { product_id: params.productId };
  return useQuery({
    queryKey: findingKeys.stats(queryParams),
    queryFn:  () => findingApi.getStats(queryParams),
    staleTime: 60_000,
  });
}
```

### File 4: `ui/src/features/findings/hooks/useFindingDetail.ts`

```typescript
import { useQuery } from '@tanstack/react-query';
import { findingApi } from '../api/findingApi';
import { findingKeys } from './useFindings';

export function useFindingDetail(id: string | null) {
  return useQuery({
    queryKey: findingKeys.detail(id ?? ''),
    queryFn:  () => findingApi.getById(id!),
    enabled:  !!id,
    staleTime: 30_000,
  });
}
```

### File 5: `ui/src/features/findings/hooks/useUpdateFinding.ts`

```typescript
import { useMutation, useQueryClient } from '@tanstack/react-query';
import { findingApi } from '../api/findingApi';
import { findingKeys } from './useFindings';
import { canTransition } from '@/shared/utils/findingStateMachine';
import toast from 'react-hot-toast';
import type { UpdateFindingPayload, FindingStatus } from '../types';

export function useUpdateFinding(findingId: string) {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (payload: UpdateFindingPayload) => findingApi.update(findingId, payload),

    // Optimistic update: cập nhật local state trước khi nhận response
    onMutate: async (payload) => {
      await queryClient.cancelQueries({ queryKey: findingKeys.detail(findingId) });
      const prev = queryClient.getQueryData(findingKeys.detail(findingId));
      queryClient.setQueryData(findingKeys.detail(findingId), (old: any) => ({
        ...old, ...payload,
      }));
      return { prev };
    },

    onError: (_err, _vars, context) => {
      // Rollback nếu server trả lỗi
      if (context?.prev) {
        queryClient.setQueryData(findingKeys.detail(findingId), context.prev);
      }
      toast.error('Failed to update finding');
    },

    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: findingKeys.detail(findingId) });
      queryClient.invalidateQueries({ queryKey: findingKeys.list({}) });
      queryClient.invalidateQueries({ queryKey: findingKeys.stats({}) });
    },
  });
}

// Guard: kiểm tra transition hợp lệ TRƯỚC KHI gọi mutation
export function validateTransition(currentStatus: FindingStatus, nextStatus: FindingStatus): boolean {
  return canTransition(currentStatus, nextStatus);
}
```

### File 6: `ui/src/features/findings/hooks/useBulkOps.ts`

```typescript
import { useState, useCallback } from 'react';
import { useMutation, useQueryClient } from '@tanstack/react-query';
import { findingApi } from '../api/findingApi';
import { findingKeys } from './useFindings';
import toast from 'react-hot-toast';
import type { BulkClosePayload, FindingStatus } from '../types';

export function useSelectionState() {
  const [selectedIds, setSelectedIds] = useState<Set<string>>(new Set());

  const toggleOne = useCallback((id: string) => {
    setSelectedIds(prev => {
      const next = new Set(prev);
      if (next.has(id)) next.delete(id);
      else next.add(id);
      return next;
    });
  }, []);

  const selectAll = useCallback((ids: string[]) => {
    setSelectedIds(new Set(ids));
  }, []);

  const clearAll = useCallback(() => {
    setSelectedIds(new Set());
  }, []);

  return {
    selectedIds,
    selectedCount: selectedIds.size,
    isSelected: (id: string) => selectedIds.has(id),
    toggleOne,
    selectAll,
    clearAll,
    asArray: () => Array.from(selectedIds),
  };
}

export function useBulkOps() {
  const queryClient = useQueryClient();
  const invalidate = () => {
    queryClient.invalidateQueries({ queryKey: findingKeys.all });
  };

  const bulkClose = useMutation({
    mutationFn: findingApi.bulkClose,
    onSuccess: (data) => {
      invalidate();
      const msg = data.failed > 0
        ? `Closed ${data.processed} findings (${data.failed} failed)`
        : `Closed ${data.processed} findings`;
      toast.success(msg);
    },
    onError: () => toast.error('Bulk close failed'),
  });

  const bulkReopen = useMutation({
    mutationFn: findingApi.bulkReopen,
    onSuccess: (data) => {
      invalidate();
      toast.success(`Reopened ${data.processed} findings`);
    },
    onError: () => toast.error('Bulk reopen failed'),
  });

  const bulkAssign = useMutation({
    mutationFn: findingApi.bulkAssign,
    onSuccess: (data) => {
      invalidate();
      toast.success(`Assigned ${data.processed} findings`);
    },
    onError: () => toast.error('Bulk assign failed'),
  });

  return { bulkClose, bulkReopen, bulkAssign };
}
```

### File 7: `ui/src/features/findings/hooks/useFindingNotes.ts`

```typescript
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { findingApi } from '../api/findingApi';
import { findingKeys } from './useFindings';
import toast from 'react-hot-toast';

export function useFindingNotes(findingId: string) {
  return useQuery({
    queryKey: findingKeys.notes(findingId),
    queryFn:  () => findingApi.getNotes(findingId),
    enabled:  !!findingId,
    staleTime: 30_000,
  });
}

export function useAddNote(findingId: string) {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (content: string) => findingApi.addNote(findingId, content),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: findingKeys.notes(findingId) });
      toast.success('Note added');
    },
  });
}

export function useFindingAudit(findingId: string) {
  return useQuery({
    queryKey: findingKeys.audit(findingId),
    queryFn:  () => findingApi.getAudit(findingId),
    enabled:  !!findingId,
    staleTime: 60_000,
  });
}
```

---

## Verification

```bash
cd ui/
npx tsc --noEmit
# Expected: no errors

# Verify state machine guard
node -e "
  import('./src/shared/utils/findingStateMachine.js').then(m => {
    console.log(m.canTransition('active', 'mitigated'));    // true
    console.log(m.canTransition('duplicate', 'active'));   // false
    console.log(m.canTransition('mitigated', 'active'));   // true
  });
"

# Verify không hardcode finding data
grep -rn "CVE-\|severity.*Critical\|F-[0-9]" src/features/findings/hooks/
# Expected: no output
```

---

## Checklist

- [ ] `features/findings/types.ts` — Finding, FindingNote, AuditEvent, RiskAcceptance, Params types
- [ ] `features/findings/api/findingApi.ts` — 10 API methods dùng `ENDPOINTS.findings.*`
- [ ] `findingKeys` factory — all, list, detail, notes, audit, stats
- [ ] `useFindings` — URL state sync, `setFilter`, `placeholderData`
- [ ] `useFindingStats` — aggregate counts per severity/status/SLA
- [ ] `useFindingDetail` — `enabled: !!id` (lazy)
- [ ] `useUpdateFinding` — optimistic update + rollback on error
- [ ] `validateTransition()` — guard using `canTransition()` từ `findingStateMachine`
- [ ] `useSelectionState` — toggle, selectAll, clearAll
- [ ] `useBulkOps` — bulkClose, bulkReopen, bulkAssign với toast
- [ ] `useFindingNotes` + `useAddNote` + `useFindingAudit`
- [ ] `npx tsc --noEmit` không lỗi
