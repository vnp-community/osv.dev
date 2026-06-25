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
