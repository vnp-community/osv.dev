/**
 * useWebhookDeliveries — React Query hooks cho webhook delivery logs & hourly stats.
 * TASK-P4-03: thay thế DELIVERY_HISTORY, ACTIVITY_CHART constants trong WebhookEvents.tsx
 */
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { apiClient } from '@/shared/api/client';
import { ENDPOINTS } from '@/shared/api/endpoints';

// ─── Types ────────────────────────────────────────────────────────────────────

export type DeliveryStatus = 'success' | 'failed' | 'retried';

export interface WebhookDelivery {
  id: string;
  webhook_id: string;
  event: string;
  endpoint: string;
  status: DeliveryStatus;
  response_time: number;
  status_code: number;
  time: string;  // ISO
  request_body?: string;
  response_body?: string;
}

export interface WebhookDeliveriesResponse {
  deliveries: WebhookDelivery[];
  total: number;
}

export interface WebhookHourlyStats {
  h: string;
  success: number;
  failed: number;
}

// ─── Query Keys ──────────────────────────────────────────────────────────────

export const deliveryKeys = {
  all: ['webhooks', 'deliveries'] as const,
  list: (webhookId?: string, params?: Record<string, unknown>) =>
    [...deliveryKeys.all, webhookId, params] as const,
  hourly: ['webhooks', 'stats', 'hourly'] as const,
};

// ─── GET /api/v1/webhooks/deliveries ─────────────────────────────────────────

export function useWebhookDeliveries(webhookId?: string, params?: { page?: number }) {
  return useQuery<WebhookDeliveriesResponse>({
    queryKey: deliveryKeys.list(webhookId, params),
    queryFn: async () => {
      const { data } = await apiClient.get<WebhookDeliveriesResponse>(
        ENDPOINTS.webhooks.deliveries,
        { params: { webhook_id: webhookId, ...params } }
      );
      return data;
    },
    staleTime: 30_000,
    refetchInterval: 30_000,
  });
}

// ─── GET /api/v1/webhooks/stats/hourly ───────────────────────────────────────

export function useWebhookHourlyStats() {
  return useQuery<WebhookHourlyStats[]>({
    queryKey: deliveryKeys.hourly,
    queryFn: async () => {
      const { data } = await apiClient.get<WebhookHourlyStats[]>(
        ENDPOINTS.webhooks.deliveryStats
      );
      return data;
    },
    staleTime: 5 * 60_000,
  });
}

// ─── POST /api/v1/webhooks/deliveries/:id/retry ───────────────────────────────

export function useRetryDelivery() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (deliveryId: string) =>
      apiClient.post(ENDPOINTS.webhooks.retryDelivery(deliveryId)),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: deliveryKeys.all });
    },
  });
}
