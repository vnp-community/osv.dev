/**
 * useScanResults.ts — React Query hooks cho Nmap & ZAP scan results
 * Phase 3 refactor: thay thế hardcoded arrays trong NmapResults.tsx & ZAPResults.tsx
 */
import { useQuery } from '@tanstack/react-query';
import { useParams } from 'react-router';
import { apiClient } from '@/shared/api/client';
import { ENDPOINTS } from '@/shared/api/endpoints';
import type { NmapHost, NmapResultsResponse } from '@/mocks/fixtures/nmap.fixture';
import type { ZapAlert, ZapResultsResponse } from '@/mocks/fixtures/zap.fixture';

export type { NmapHost, NmapResultsResponse, ZapAlert, ZapResultsResponse };

// ─── Query Keys ───────────────────────────────────────────────────────────────

export const scanResultKeys = {
  nmap: (scanId: string) => ['scans', scanId, 'results', 'nmap'] as const,
  zap:  (scanId: string) => ['scans', scanId, 'results', 'zap']  as const,
};

// ─── useNmapResults ───────────────────────────────────────────────────────────

export function useNmapResults(scanId: string | undefined) {
  return useQuery<NmapResultsResponse>({
    queryKey: scanResultKeys.nmap(scanId ?? ''),
    queryFn: async () => {
      const { data } = await apiClient.get<NmapResultsResponse>(
        ENDPOINTS.scans.nmap(scanId!)
      );
      return {
        hosts:  Array.isArray(data?.hosts) ? data.hosts : [],
        total:  typeof data?.total === 'number' ? data.total : (data?.hosts?.length ?? 0),
        scanId: data?.scanId ?? scanId ?? '',
      };
    },
    enabled: !!scanId,
    staleTime: 60_000,
  });
}

// ─── useZAPResults ────────────────────────────────────────────────────────────

export function useZAPResults(scanId: string | undefined) {
  return useQuery<ZapResultsResponse>({
    queryKey: scanResultKeys.zap(scanId ?? ''),
    queryFn: async () => {
      const { data } = await apiClient.get<ZapResultsResponse>(
        ENDPOINTS.scans.zap(scanId!)
      );
      const alerts = Array.isArray(data?.alerts) ? data.alerts : [];
      return {
        alerts,
        total:        typeof data?.total === 'number' ? data.total : alerts.length,
        scanId:       data?.scanId ?? scanId ?? '',
        riskBreakdown: data?.riskBreakdown ?? alerts.reduce((acc, a) => {
          acc[a.risk] = (acc[a.risk] ?? 0) + a.count;
          return acc;
        }, {} as Record<string, number>),
      };
    },
    enabled: !!scanId,
    staleTime: 60_000,
  });
}
