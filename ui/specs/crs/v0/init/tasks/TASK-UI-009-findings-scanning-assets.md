# TASK-UI-009 — Findings + Scanning + Assets Modules

| Field | Value |
|-------|-------|
| **Task ID** | TASK-UI-009 |
| **Module** | `ui/src/features/findings/`, `ui/src/features/scanning/`, `ui/src/features/assets/` |
| **Solution Ref** | [SOL-003 §5,6](../solutions/SOL-003-phase2-api-migration.md) |
| **Priority** | 🟡 P1 |
| **Depends On** | TASK-UI-004, TASK-UI-005, TASK-UI-006 |
| **Estimated** | 4h |
| **Status** | ✅ Completed — tất cả Findings/Scan/Asset components đã migrate (2026-06-17) |
| **Updated** | 2026-06-17 |

---

## Context

- **Findings**: `FindingsList.tsx`, `FindingDetail.tsx`, `SLADashboard.tsx`, `RiskAcceptanceCenter.tsx` đều hardcode findings data. `FindingDetail` cần mutations cho status update.
- **Scanning**: `ScanDashboard.tsx`, `ScanWizard.tsx`, `RunningScan.tsx` cần API + SSE (Server-Sent Events) cho scan progress.
- **Assets**: `AssetInventory.tsx`, `AssetDetail.tsx` cần asset API.

---

## Goal

Tạo 3 feature modules với API layers, hooks, mutations, và SSE support. Migrate tất cả components khỏi hardcode.

---

## Target Files

| Action | File Path |
|--------|-----------|
| CREATE | `ui/src/features/findings/api/findingApi.ts` |
| CREATE | `ui/src/features/findings/hooks/useFindings.ts` |
| CREATE | `ui/src/features/findings/hooks/useUpdateFinding.ts` |
| CREATE | `ui/src/features/scanning/api/scanApi.ts` |
| CREATE | `ui/src/features/scanning/hooks/useScans.ts` |
| CREATE | `ui/src/shared/hooks/useSSE.ts` |
| CREATE | `ui/src/features/scanning/hooks/useScanSSE.ts` |
| CREATE | `ui/src/features/scanning/schemas/scanWizardSchema.ts` |
| CREATE | `ui/src/features/assets/api/assetApi.ts` |
| CREATE | `ui/src/features/assets/hooks/useAssets.ts` |
| MODIFY | `ui/src/app/components/FindingsList.tsx` |
| MODIFY | `ui/src/app/components/FindingDetail.tsx` |
| MODIFY | `ui/src/app/components/ScanDashboard.tsx` |
| MODIFY | `ui/src/app/components/RunningScan.tsx` |
| MODIFY | `ui/src/app/components/AssetInventory.tsx` |

---

## Implementation

### ── FINDINGS MODULE ─────────────────────────────────────────────────────

### File 1: `ui/src/features/findings/api/findingApi.ts`

```typescript
import { apiClient } from '@/shared/api/client';
import { ENDPOINTS } from '@/shared/api/endpoints';
import type { Finding, FindingStatus } from '@/shared/types/finding';

export interface FindingsListParams {
  status?: FindingStatus[];
  severity?: string[];
  productId?: string;
  cveId?: string;
  slaStatus?: 'ok' | 'at_risk' | 'breached';
  assignedTo?: string;
  page?: number;
  pageSize?: number;
  sortBy?: string;
}

export interface FindingsListResponse {
  findings: Finding[];
  total: number;
  bySeverity: Record<string, number>;
  byStatus: Record<string, number>;
  slaStats: { breached: number; atRisk: number; ok: number };
}

export const findingApi = {
  list: async (params: FindingsListParams): Promise<FindingsListResponse> => {
    const { data } = await apiClient.get<FindingsListResponse>(
      ENDPOINTS.findings.list,
      { params }
    );
    return data;
  },

  getById: async (id: string): Promise<Finding> => {
    const { data } = await apiClient.get<Finding>(ENDPOINTS.findings.detail(id));
    return data;
  },

  update: async (
    id: string,
    payload: { status?: FindingStatus; comment?: string; assignedTo?: string }
  ): Promise<Finding> => {
    const { data } = await apiClient.patch<Finding>(
      ENDPOINTS.findings.update(id),
      payload
    );
    return data;
  },

  bulkClose: async (findingIds: string[], comment?: string): Promise<{ updated: number }> => {
    const { data } = await apiClient.post(ENDPOINTS.findings.bulkClose, {
      findingIds,
      comment,
    });
    return data as { updated: number };
  },

  getAudit: async (id: string) => {
    const { data } = await apiClient.get(ENDPOINTS.findings.audit(id));
    return data;
  },
};
```

### File 2: `ui/src/features/findings/hooks/useFindings.ts`

```typescript
import { useQuery } from '@tanstack/react-query';
import { findingKeys } from '@/shared/api/queryClient';
import { findingApi, type FindingsListParams } from '../api/findingApi';

export function useFindings(params: FindingsListParams) {
  return useQuery({
    queryKey: findingKeys.list(params),
    queryFn: () => findingApi.list(params),
    staleTime: 30_000,
    placeholderData: (prev) => prev,
  });
}

export function useFindingDetail(id: string | null) {
  return useQuery({
    queryKey: findingKeys.detail(id ?? ''),
    queryFn: () => findingApi.getById(id!),
    enabled: !!id,
    staleTime: 30_000,
  });
}
```

### File 3: `ui/src/features/findings/hooks/useUpdateFinding.ts`

```typescript
import { useMutation } from '@tanstack/react-query';
import { findingKeys, queryClient } from '@/shared/api/queryClient';
import { findingApi } from '../api/findingApi';
import type { FindingStatus } from '@/shared/types/finding';
import { toast } from 'sonner';

export function useUpdateFinding() {
  return useMutation({
    mutationFn: ({
      id,
      status,
      comment,
    }: {
      id: string;
      status: FindingStatus;
      comment?: string;
    }) => findingApi.update(id, { status, comment }),

    onSuccess: (updated) => {
      // Update specific finding in cache
      queryClient.setQueryData(findingKeys.detail(updated.id), updated);
      // Invalidate list queries
      queryClient.invalidateQueries({ queryKey: findingKeys.all });
      toast.success('Finding status updated');
    },
  });
}

export function useBulkCloseFinding() {
  return useMutation({
    mutationFn: ({
      findingIds,
      comment,
    }: {
      findingIds: string[];
      comment?: string;
    }) => findingApi.bulkClose(findingIds, comment),

    onSuccess: (result) => {
      queryClient.invalidateQueries({ queryKey: findingKeys.all });
      toast.success(`${result.updated} findings closed`);
    },
  });
}
```

### ── SCANNING MODULE ─────────────────────────────────────────────────────

### File 4: `ui/src/features/scanning/api/scanApi.ts`

```typescript
import { apiClient } from '@/shared/api/client';
import { ENDPOINTS } from '@/shared/api/endpoints';
import type { Scan, NmapHost, ZAPAlert } from '@/shared/types/scan';

export interface CreateScanPayload {
  name: string;
  type: string;
  targets: string[];
  options?: {
    scanProfile?: string;
    portRange?: string;
    maxDepth?: number;
    timeout?: number;
  };
  engagementId?: string;
}

export const scanApi = {
  list: async (params?: { status?: string; page?: number }): Promise<{ scans: Scan[]; total: number }> => {
    const { data } = await apiClient.get(ENDPOINTS.scans.list, { params });
    return data as { scans: Scan[]; total: number };
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

  getNmapResults: async (id: string): Promise<{ hosts: NmapHost[]; scanId: string }> => {
    const { data } = await apiClient.get(ENDPOINTS.scans.results.nmap(id));
    return data as { hosts: NmapHost[]; scanId: string };
  },

  getZAPResults: async (id: string): Promise<{ alerts: ZAPAlert[]; scanId: string }> => {
    const { data } = await apiClient.get(ENDPOINTS.scans.results.zap(id));
    return data as { alerts: ZAPAlert[]; scanId: string };
  },
};
```

### File 5: `ui/src/features/scanning/hooks/useScans.ts`

```typescript
import { useQuery, useMutation } from '@tanstack/react-query';
import { scanKeys, queryClient } from '@/shared/api/queryClient';
import { scanApi, type CreateScanPayload } from '../api/scanApi';
import { useNavigate } from 'react-router';
import { toast } from 'sonner';

export function useScans(params?: { status?: string }) {
  return useQuery({
    queryKey: scanKeys.list(params),
    queryFn: () => scanApi.list(params),
    staleTime: 15_000,
    refetchInterval: 15_000,  // Poll tích cực khi có scan đang chạy
  });
}

export function useScanDetail(id: string | null) {
  return useQuery({
    queryKey: scanKeys.detail(id ?? ''),
    queryFn: () => scanApi.getById(id!),
    enabled: !!id,
    staleTime: 10_000,
  });
}

export function useCreateScan() {
  const navigate = useNavigate();

  return useMutation({
    mutationFn: (payload: CreateScanPayload) => scanApi.create(payload),
    onSuccess: (scan) => {
      queryClient.invalidateQueries({ queryKey: scanKeys.all });
      toast.success(`Scan "${scan.name}" queued successfully`);
      navigate(`/scans/${scan.id}`);
    },
  });
}

export function useCancelScan() {
  return useMutation({
    mutationFn: (id: string) => scanApi.cancel(id),
    onSuccess: (_, id) => {
      queryClient.invalidateQueries({ queryKey: scanKeys.detail(id) });
      queryClient.invalidateQueries({ queryKey: scanKeys.list() });
      toast.success('Scan cancelled');
    },
  });
}
```

### File 6: `ui/src/shared/hooks/useSSE.ts`

```typescript
import { useEffect, useRef, useState } from 'react';

export type SSEStatus = 'idle' | 'connecting' | 'streaming' | 'done' | 'error';

interface SSEOptions<T> {
  onMessage?: (data: T) => void;
  onDone?: () => void;
  onError?: () => void;
}

export function useSSE<T>(
  url: string,
  enabled: boolean,
  options: SSEOptions<T> = {}
): { status: SSEStatus } {
  const [status, setStatus] = useState<SSEStatus>('idle');
  const sourceRef = useRef<EventSource | null>(null);
  const optionsRef = useRef(options);
  optionsRef.current = options;

  useEffect(() => {
    if (!enabled || !url) {
      setStatus('idle');
      return;
    }

    setStatus('connecting');
    const source = new EventSource(url, { withCredentials: true });
    sourceRef.current = source;

    source.onopen = () => setStatus('streaming');

    source.onmessage = (e) => {
      try {
        const data = JSON.parse(e.data) as T;
        optionsRef.current.onMessage?.(data);
      } catch (err) {
        console.error('[SSE] JSON parse error:', err);
      }
    };

    source.addEventListener('done', () => {
      setStatus('done');
      optionsRef.current.onDone?.();
      source.close();
    });

    source.onerror = () => {
      setStatus('error');
      optionsRef.current.onError?.();
      source.close();
    };

    return () => {
      source.close();
      sourceRef.current = null;
      setStatus('idle');
    };
  }, [url, enabled]);

  return { status };
}
```

### File 7: `ui/src/features/scanning/hooks/useScanSSE.ts`

```typescript
import { useState, useCallback } from 'react';
import { useQueryClient } from '@tanstack/react-query';
import { useSSE } from '@/shared/hooks/useSSE';
import { scanKeys } from '@/shared/api/queryClient';
import { ENDPOINTS } from '@/shared/api/endpoints';
import type { ScanProgress, ScanStatus } from '@/shared/types/scan';

export function useScanSSE(scanId: string, enabled: boolean) {
  const queryClient = useQueryClient();
  const [progress, setProgress] = useState<ScanProgress | null>(null);

  const handleMessage = useCallback(
    (data: ScanProgress) => {
      setProgress(data);
      // Optimistic cache update
      queryClient.setQueryData(
        scanKeys.detail(scanId),
        (old: { status: ScanStatus; progress: number } | undefined) =>
          old
            ? { ...old, progress: data.progress, status: data.status }
            : old
      );
    },
    [scanId, queryClient]
  );

  const handleDone = useCallback(() => {
    // Fetch final scan state after SSE closes
    queryClient.invalidateQueries({ queryKey: scanKeys.detail(scanId) });
    queryClient.invalidateQueries({ queryKey: scanKeys.list() });
  }, [scanId, queryClient]);

  const { status: sseStatus } = useSSE<ScanProgress>(
    `${import.meta.env.VITE_API_BASE_URL || ''}${ENDPOINTS.scans.stream(scanId)}`,
    enabled,
    {
      onMessage: handleMessage,
      onDone: handleDone,
    }
  );

  return { progress, sseStatus };
}
```

### File 8: `ui/src/features/scanning/schemas/scanWizardSchema.ts`

```typescript
import { z } from 'zod';

const ipOrHostname = z.string().regex(
  /^(\d{1,3}\.){3}\d{1,3}(\/\d{1,2})?$|^[a-zA-Z0-9.-]+$/,
  'Must be a valid IP, CIDR, or hostname'
);

export const scanWizardSchema = z.object({
  type: z.enum(['nmap_full', 'nmap_discovery', 'zap', 'import']),
  name: z.string().min(3, 'Scan name must be at least 3 characters').max(100),
  targetsRaw: z
    .string()
    .min(1, 'At least one target is required'),
  scanProfile: z.enum(['discovery', 'full', 'custom']).optional(),
  portRange: z.string().optional(),
  maxDepth: z.number().min(1).max(10).optional(),
  timeout: z.number().min(30).max(3600).optional(),
  frequency: z.enum(['once', 'daily', 'weekly', 'custom']).default('once'),
  cronExpr: z.string().optional(),
  engagementId: z.string().optional(),
});

export type ScanWizardFormData = z.infer<typeof scanWizardSchema>;

// Parse targetsRaw string → string[]
export function parseTargets(targetsRaw: string): string[] {
  return targetsRaw
    .split(/[\n,;]/)
    .map((t) => t.trim())
    .filter(Boolean);
}
```

### ── ASSETS MODULE ────────────────────────────────────────────────────────

### File 9: `ui/src/features/assets/api/assetApi.ts`

```typescript
import { apiClient } from '@/shared/api/client';
import { ENDPOINTS } from '@/shared/api/endpoints';
import type { Asset } from '@/shared/types/scan';

export interface AssetsListParams {
  riskLevel?: 'critical' | 'high' | 'medium' | 'low';
  tags?: string[];
  query?: string;
  page?: number;
  pageSize?: number;
}

export interface AssetsListResponse {
  assets: Asset[];
  total: number;
}

export const assetApi = {
  list: async (params?: AssetsListParams): Promise<AssetsListResponse> => {
    const { data } = await apiClient.get<AssetsListResponse>(
      ENDPOINTS.assets.list,
      { params }
    );
    return data;
  },

  getById: async (id: string): Promise<Asset> => {
    const { data } = await apiClient.get<Asset>(ENDPOINTS.assets.detail(id));
    return data;
  },
};
```

### File 10: `ui/src/features/assets/hooks/useAssets.ts`

```typescript
import { useQuery } from '@tanstack/react-query';
import { assetKeys } from '@/shared/api/queryClient';
import { assetApi, type AssetsListParams } from '../api/assetApi';

export function useAssets(params?: AssetsListParams) {
  return useQuery({
    queryKey: assetKeys.list(params),
    queryFn: () => assetApi.list(params),
    staleTime: 60_000,
    placeholderData: (prev) => prev,
  });
}

export function useAssetDetail(id: string | null) {
  return useQuery({
    queryKey: assetKeys.detail(id ?? ''),
    queryFn: () => assetApi.getById(id!),
    enabled: !!id,
    staleTime: 60_000,
  });
}
```

---

## Migration Pattern cho Components

### FindingsList.tsx — MODIFY

```typescript
// ❌ XÓA: hardcoded findings array
// ✅ THAY:
import { useFindings } from '@/features/findings/hooks/useFindings';
import { useSearchParams } from 'react-router';
import { QueryBoundary } from '@/shared/components/feedback/QueryBoundary';

export function FindingsList() {
  const [searchParams, setSearchParams] = useSearchParams();

  const filters = {
    status: searchParams.getAll('status') as FindingStatus[],
    severity: searchParams.getAll('severity'),
    slaStatus: (searchParams.get('sla') as SLAStatus) ?? undefined,
    page: Number(searchParams.get('page') ?? '1'),
    pageSize: 50,
  };

  const findingsQuery = useFindings(filters);

  return (
    <QueryBoundary query={findingsQuery}>
      {(data) => (
        // Dùng data.findings, data.total, data.bySeverity, data.slaStats
      )}
    </QueryBoundary>
  );
}
```

### RunningScan.tsx — MODIFY

```typescript
import { useParams } from 'react-router';
import { useScanDetail, useCancelScan } from '@/features/scanning/hooks/useScans';
import { useScanSSE } from '@/features/scanning/hooks/useScanSSE';

export function RunningScan() {
  const { id } = useParams<{ id: string }>();
  const scanQuery = useScanDetail(id ?? null);
  const cancelMutation = useCancelScan();

  const isActive = scanQuery.data?.status === 'running' ||
                   scanQuery.data?.status === 'queued';

  const { progress, sseStatus } = useScanSSE(id ?? '', isActive);

  // Dùng progress.progress (0-100) từ SSE
  // Dùng scanQuery.data cho scan metadata
  // cancelMutation.mutate(id) để cancel
}
```

### AssetInventory.tsx — MODIFY

```typescript
import { useAssets } from '@/features/assets/hooks/useAssets';
import { QueryBoundary } from '@/shared/components/feedback/QueryBoundary';

export function AssetInventory() {
  const assetsQuery = useAssets();

  return (
    <QueryBoundary query={assetsQuery}>
      {(data) => (
        // Dùng data.assets, data.total
        // XÓA hardcoded asset arrays
      )}
    </QueryBoundary>
  );
}
```

---

## Verification

```bash
cd ui/
npx tsc --noEmit

# Verify no hardcode in finding/scan/asset components
grep -rn "const findings\s*=\s*\[" \
     "const scans\s*=\s*\[" \
     "const assets\s*=\s*\[" \
  src/app/components/FindingsList.tsx \
  src/app/components/ScanDashboard.tsx \
  src/app/components/AssetInventory.tsx
# Expected: No output

VITE_ENABLE_MSW=true pnpm dev
# /findings → hiện 5 findings từ MSW fixture
# /scans → hiện 5 scans từ MSW fixture
# /scans/:id (running scan) → SSE progress bar hoạt động
# /assets → hiện assets từ MSW fixture
```

---

## Checklist

### Findings
- [x] `features/findings/api/findingApi.ts` — list, getById, update (PATCH), bulkClose
- [x] `features/findings/hooks/useFindings.ts` — useFindings + useFindingDetail
- [x] `features/findings/hooks/useUpdateFinding.ts` — mutation + cache invalidation
- [x] `FindingsList.tsx`: URL-based filters + `useFindings` ✅ Done
- [x] `FindingDetail.tsx`: `useFindingDetail` + `useUpdateFinding` mutation ✅ Done
- [x] `SLADashboard.tsx`: `useFindings({ slaStatus })` via useQuery + QueryBoundary ✅ Done

### Scanning
- [x] `features/scanning/api/scanApi.ts` — list, create, getById, cancel, nmap/zap results
- [x] `features/scanning/hooks/useScans.ts` — list, detail, create mutation, cancel mutation
- [x] `shared/hooks/useSSE.ts` — generic SSE hook
- [x] `features/scanning/hooks/useScanSSE.ts` — scan-specific SSE + cache update
- [x] `features/scanning/schemas/scanWizardSchema.ts` — Zod schema
- [x] `ScanDashboard.tsx`: `useScans` hook ✅ Done
- [x] `RunningScan.tsx`: `useScanSSE` → SSE progress bar ✅ Done
- [x] `ScanWizard.tsx`: `useCreateScan` mutation ✅ Done (migrated)

### Assets
- [x] `features/assets/api/assetApi.ts` — list, getById
- [x] `features/assets/hooks/useAssets.ts` — useAssets + useAssetDetail
- [x] `AssetInventory.tsx`: `useAssets` hook ✅ Done
- [x] `AssetDetail.tsx`: `useAssetDetail` hook ✅ Done (migrated)

### Common
- [x] `npx tsc --noEmit` không có lỗi (chỉ 1 pre-existing)
- [x] `grep` cho hardcoded arrays → no output ✅ Done
