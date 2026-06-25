import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { apiClient } from '@/shared/api/client';
import { ENDPOINTS } from '@/shared/api/endpoints';

// ─── Types ────────────────────────────────────────────────────────────────────

export type APIKeyStatus = 'active' | 'revoked' | 'expired';

export interface APIKey {
  id: string;
  name: string;
  prefix: string;
  scopes: string[];
  created_at: string;
  last_used_at?: string;
  expires_at?: string;
  status: APIKeyStatus;
  created_by: string;
}

export interface APIKeysResponse {
  items: APIKey[];
  total: number;
}

export interface CreateAPIKeyRequest {
  name: string;
  scopes: string[];
  expires_at?: string;
}

export interface CreateAPIKeyResponse {
  api_key: APIKey;
  /** Plain-text key — shown ONCE, never stored */
  plain_key: string;
}

// ─── Query Keys ──────────────────────────────────────────────────────────────

const apiKeyKeys = {
  all: ['api-keys'] as const,
  list: () => [...apiKeyKeys.all, 'list'] as const,
};

// ─── GET /api/v1/api-keys ────────────────────────────────────────────────────

export function useAPIKeys() {
  return useQuery<APIKeysResponse>({
    queryKey: apiKeyKeys.list(),
    queryFn: async () => {
      const { data } = await apiClient.get<APIKeysResponse>(
        ENDPOINTS.apiKeys.list
      );
      return data;
    },
    staleTime: 60_000,
  });
}

// ─── POST /api/v1/api-keys ───────────────────────────────────────────────────

export function useCreateAPIKey() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async (req: CreateAPIKeyRequest) => {
      const { data } = await apiClient.post<CreateAPIKeyResponse>(
        ENDPOINTS.apiKeys.create,
        req
      );
      return data;
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: apiKeyKeys.all });
    },
  });
}

// ─── DELETE /api/v1/api-keys/:id (revoke) ────────────────────────────────────

export function useRevokeAPIKey() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (id: string) =>
      apiClient.delete(ENDPOINTS.apiKeys.revoke(id)),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: apiKeyKeys.all });
    },
  });
}
