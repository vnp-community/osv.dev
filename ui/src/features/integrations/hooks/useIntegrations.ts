import { useQuery, useMutation } from '@tanstack/react-query';
import { queryClient } from '@/shared/api/queryClient';
import { integrationApi, type CreateAPIKeyPayload, type CreateWebhookPayload } from '../api/integrationApi';
import { toast } from 'sonner';

const integrationKeys = {
  all: ['integrations'] as const,
  apiKeys: () => [...integrationKeys.all, 'api-keys'] as const,
  webhooks: () => [...integrationKeys.all, 'webhooks'] as const,
};

// ─── API Keys ─────────────────────────────────────────────────────────────────

export function useAPIKeys() {
  return useQuery({
    queryKey: integrationKeys.apiKeys(),
    queryFn: () => integrationApi.listAPIKeys(),
    staleTime: 60_000,
  });
}

export function useCreateAPIKey() {
  return useMutation({
    mutationFn: (payload: CreateAPIKeyPayload) => integrationApi.createAPIKey(payload),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: integrationKeys.apiKeys() });
      toast.success('API key created');
    },
  });
}

export function useRevokeAPIKey() {
  return useMutation({
    mutationFn: (id: string) => integrationApi.revokeAPIKey(id),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: integrationKeys.apiKeys() });
      toast.success('API key revoked');
    },
  });
}

// ─── Webhooks ─────────────────────────────────────────────────────────────────

export function useWebhooks() {
  return useQuery({
    queryKey: integrationKeys.webhooks(),
    queryFn: () => integrationApi.listWebhooks(),
    staleTime: 60_000,
  });
}

export function useCreateWebhook() {
  return useMutation({
    mutationFn: (payload: CreateWebhookPayload) => integrationApi.createWebhook(payload),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: integrationKeys.webhooks() });
      toast.success('Webhook created');
    },
  });
}

export function useDeleteWebhook() {
  return useMutation({
    mutationFn: (id: string) => integrationApi.deleteWebhook(id),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: integrationKeys.webhooks() });
      toast.success('Webhook deleted');
    },
  });
}

export function useTestWebhook() {
  return useMutation({
    mutationFn: (id: string) => integrationApi.testWebhook(id),
    onSuccess: (result) => {
      if (result.success) {
        toast.success(`Webhook test succeeded (${result.statusCode})`);
      } else {
        toast.error(`Webhook test failed (${result.statusCode})`);
      }
    },
  });
}
