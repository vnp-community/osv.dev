import { apiClient } from '@/shared/api/client';
import { ENDPOINTS } from '@/shared/api/endpoints';

export interface APIKey {
  id: string;
  name: string;
  prefix: string;
  scopes: string[];
  createdAt: string;
  lastUsedAt: string | null;
  expiresAt: string | null;
}

export interface CreateAPIKeyPayload {
  name: string;
  scopes: string[];
  expiresAt?: string;
}

export interface CreateAPIKeyResponse {
  apiKey: APIKey;
  secret: string; // shown once only
}

export interface Webhook {
  id: string;
  name: string;
  url: string;
  events: string[];
  secret: string;
  active: boolean;
  createdAt: string;
  lastTriggeredAt: string | null;
}

export interface CreateWebhookPayload {
  name: string;
  url: string;
  events: string[];
  secret?: string;
}

export const integrationApi = {
  // ─── API Keys ────────────────────────────────────────────────────────────
  listAPIKeys: async (): Promise<APIKey[]> => {
    const { data } = await apiClient.get(ENDPOINTS.apiKeys.list);
    return data as APIKey[];
  },

  createAPIKey: async (payload: CreateAPIKeyPayload): Promise<CreateAPIKeyResponse> => {
    const { data } = await apiClient.post(ENDPOINTS.apiKeys.create, payload);
    return data as CreateAPIKeyResponse;
  },

  revokeAPIKey: async (id: string): Promise<void> => {
    await apiClient.delete(ENDPOINTS.apiKeys.revoke(id));
  },

  // ─── Webhooks ────────────────────────────────────────────────────────────
  listWebhooks: async (): Promise<Webhook[]> => {
    const { data } = await apiClient.get(ENDPOINTS.webhooks.list);
    return data as Webhook[];
  },

  createWebhook: async (payload: CreateWebhookPayload): Promise<Webhook> => {
    const { data } = await apiClient.post<Webhook>(ENDPOINTS.webhooks.create, payload);
    return data;
  },

  deleteWebhook: async (id: string): Promise<void> => {
    await apiClient.delete(ENDPOINTS.webhooks.delete(id));
  },

  testWebhook: async (id: string): Promise<{ success: boolean; statusCode: number }> => {
    const { data } = await apiClient.post(ENDPOINTS.webhooks.test(id));
    return data as { success: boolean; statusCode: number };
  },
};
