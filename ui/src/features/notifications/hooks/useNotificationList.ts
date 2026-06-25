/**
 * useNotificationList — React Query hook để lấy danh sách notifications từ REST API.
 * Khác với useNotifications (SSE-based), hook này dùng polling + mutations.
 *
 * @see TASK-P3-04 — thay thế const hardcode trong NotificationCenter.tsx
 */
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { apiClient } from '@/shared/api/client';
import { ENDPOINTS } from '@/shared/api/endpoints';

// ─── Types ────────────────────────────────────────────────────────────────────

export type NotificationType = 'critical' | 'sla' | 'kev' | 'scan' | 'info';

export interface AppNotification {
  id: string;
  type: NotificationType;
  title: string;
  description: string;
  product: string;
  read: boolean;
  created_at: string;
  time_ago?: string;
}

export interface NotificationsListResponse {
  items: AppNotification[];
  total: number;
  unread_count: number;
}

// ─── Query Keys ──────────────────────────────────────────────────────────────

const notifListKeys = {
  all: ['notifications', 'list'] as const,
  filtered: (params?: Record<string, unknown>) =>
    [...notifListKeys.all, params] as const,
};

// ─── GET /api/v1/notifications ───────────────────────────────────────────────

export function useNotificationList(params?: { type?: string; unread_only?: boolean }) {
  return useQuery<NotificationsListResponse>({
    queryKey: notifListKeys.filtered(params),
    queryFn: async () => {
      const { data } = await apiClient.get<NotificationsListResponse>(
        ENDPOINTS.notifications.list,
        { params }
      );
      return data;
    },
    staleTime: 15_000,
    refetchInterval: 60_000, // Poll every 60s while tab visible
  });
}

// ─── POST /api/v1/notifications/:id/read ─────────────────────────────────────

export function useMarkNotificationRead() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (id: string) =>
      apiClient.post(ENDPOINTS.notifications.markRead(id)),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: notifListKeys.all });
    },
  });
}

// ─── POST /api/v1/notifications/mark-all-read ────────────────────────────────

export function useMarkAllNotificationsRead() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: () =>
      apiClient.post(ENDPOINTS.notifications.markAllRead),
    onMutate: async () => {
      // Optimistic update — mark all read locally immediately
      await queryClient.cancelQueries({ queryKey: notifListKeys.all });
      const prev = queryClient.getQueriesData({ queryKey: notifListKeys.all });
      queryClient.setQueriesData(
        { queryKey: notifListKeys.all },
        (old: NotificationsListResponse | undefined) =>
          old
            ? {
                ...old,
                unread_count: 0,
                items: old.items.map((n) => ({ ...n, read: true })),
              }
            : old
      );
      return { prev };
    },
    onError: (_err, _vars, ctx) => {
      // Roll back on error
      if (ctx?.prev) {
        for (const [queryKey, data] of ctx.prev) {
          queryClient.setQueryData(queryKey, data);
        }
      }
    },
    onSettled: () => {
      queryClient.invalidateQueries({ queryKey: notifListKeys.all });
    },
  });
}
