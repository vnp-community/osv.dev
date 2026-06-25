import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { apiClient } from '@/shared/api/client';
import { ENDPOINTS } from '@/shared/api/endpoints';
import type { SystemSettings } from '../types';

const settingsKeys = {
  all: ['admin', 'settings'] as const,
};

// ─── GET /api/v1/admin/settings ─────────────────────────────────────────────

export function useSystemSettings() {
  return useQuery<SystemSettings>({
    queryKey: settingsKeys.all,
    queryFn: async () => {
      const { data } = await apiClient.get<SystemSettings>(
        ENDPOINTS.admin.settings
      );
      return data;
    },
    staleTime: 5 * 60_000, // 5 minutes
  });
}

// ─── PUT /api/v1/admin/settings ─────────────────────────────────────────────

export function useUpdateSettings() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (settings: Partial<SystemSettings>) =>
      apiClient.put(ENDPOINTS.admin.settings, settings),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: settingsKeys.all });
    },
  });
}
