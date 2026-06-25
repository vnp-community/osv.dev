import { useQuery } from '@tanstack/react-query';
import { apiClient } from '@/shared/api/client';
import { ENDPOINTS } from '@/shared/api/endpoints';
import type { AuditLogsResponse, AuditLogsParams } from '../types';

// ─── GET /api/v1/audit-log ───────────────────────────────────────────────────

export function useAuditLogs(params?: AuditLogsParams) {
  return useQuery<AuditLogsResponse>({
    queryKey: ['audit', 'list', params],
    queryFn: async () => {
      const { data } = await apiClient.get<AuditLogsResponse>(
        ENDPOINTS.audit.log,
        { params }
      );
      return data;
    },
    staleTime: 30_000,
  });
}
