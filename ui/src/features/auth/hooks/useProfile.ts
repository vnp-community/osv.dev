/**
 * useProfile.ts — React Query hooks cho /api/v1/profile/*
 * Phase 3 refactor: thay thế sessions & notifSettings hardcode trong UserProfile.tsx
 */
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { apiClient } from '@/shared/api/client';
import { ENDPOINTS } from '@/shared/api/endpoints';
import type { UserSession, NotifSetting } from '@/mocks/fixtures/profile.fixture';

export type { UserSession, NotifSetting };

export const profileKeys = {
  sessions:              ['profile', 'sessions'] as const,
  notificationSettings:  ['profile', 'notifications', 'settings'] as const,
};

// ─── useSessions ─────────────────────────────────────────────────────────────

export function useSessions() {
  return useQuery<{ items: UserSession[]; total: number }>({
    queryKey: profileKeys.sessions,
    queryFn: async () => {
      const { data } = await apiClient.get<{ items: UserSession[]; total: number }>(
        ENDPOINTS.profile.sessions
      );
      return {
        items: Array.isArray(data?.items) ? data.items : [],
        total: typeof data?.total === 'number' ? data.total : 0,
      };
    },
    staleTime: 60_000,
  });
}

// ─── useNotificationSettings ─────────────────────────────────────────────────

export function useNotificationSettings() {
  return useQuery<{ items: NotifSetting[] }>({
    queryKey: profileKeys.notificationSettings,
    queryFn: async () => {
      const { data } = await apiClient.get<{ items: NotifSetting[] }>(
        ENDPOINTS.profile.notificationSettings
      );
      return { items: Array.isArray(data?.items) ? data.items : [] };
    },
    staleTime: 60_000,
  });
}

// ─── useUpdateNotificationSettings ───────────────────────────────────────────

export function useUpdateNotificationSettings() {
  const queryClient = useQueryClient();
  return useMutation<{ items: NotifSetting[] }, Error, NotifSetting[]>({
    mutationFn: async (items) => {
      const { data } = await apiClient.put<{ items: NotifSetting[] }>(
        ENDPOINTS.profile.notificationSettings,
        { items }
      );
      return data;
    },
    onSuccess: (data) => {
      queryClient.setQueryData(profileKeys.notificationSettings, data);
    },
  });
}
