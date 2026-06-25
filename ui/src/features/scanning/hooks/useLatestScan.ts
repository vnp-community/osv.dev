import { useQuery } from '@tanstack/react-query';
import { scanKeys } from '@/shared/api/queryClient';
import { scanApi } from '../api/scanApi';
import type { Scan } from '@/shared/types/scan';

/**
 * Fetches the most recently created scans.
 * Used by LatestNmapRedirect and LatestZAPRedirect to resolve
 * recent scans and let the user select if there are multiple.
 */
export function useRecentScans(toolType?: 'nmap' | 'zap') {
  // Map our UI tool type to the actual scan types used in the backend
  const apiType = toolType === 'nmap' ? 'nmap_full,nmap_discovery' : toolType === 'zap' ? 'zap' : undefined;

  return useQuery<Scan[]>({
    queryKey: [...scanKeys.list({ sort_by: '-created_at', page_size: 50, type: apiType }), 'recent'],
    queryFn: async () => {
      const result = await scanApi.list({ sort_by: '-created_at', page_size: 50, type: apiType });
      return result.scans || [];
    },
    staleTime: 30_000,
  });
}
