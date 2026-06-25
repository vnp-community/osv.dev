import { apiClient } from '@/shared/api/client';
import { ENDPOINTS } from '@/shared/api/endpoints';
import type { User } from '@/shared/types/auth';
import type { LoginRequest, LoginResponse, RefreshResponse, MFASetupResponse } from '../types';

export const authApi = {
  login: async (payload: LoginRequest): Promise<LoginResponse> => {
    const { data } = await apiClient.post<LoginResponse>(ENDPOINTS.auth.login, payload);
    return data;
  },

  refresh: async (): Promise<RefreshResponse> => {
    const { data } = await apiClient.post<RefreshResponse>(ENDPOINTS.auth.refresh);
    return data;
  },

  me: async (): Promise<{ user: User }> => {
    // Gateway /me returns { user: { id, email, name, role, permissions, mfa_enabled, avatar_url, created_at } }
    // Identity-service (via authMiddleware proxy) returns { user_id, role, permissions }
    // We normalize both to the User shape.
    const { data } = await apiClient.get<{
      user?: User;
      user_id?: string;
      role?: string;
      permissions?: string[];
    }>(ENDPOINTS.auth.me);

    if (data.user) {
      return { user: data.user };
    }

    // Fallback: identity-service raw response
    const user: User = {
      id: data.user_id ?? '',
      email: '',
      name: '',
      role: (data.role ?? 'user') as User['role'],
      permissions: (data.permissions ?? []) as User['permissions'],
      mfa_enabled: false,
      avatar_url: null,
      created_at: new Date().toISOString(),
    };
    return { user };
  },

  logout: async (): Promise<void> => {
    await apiClient.post(ENDPOINTS.auth.logout);
  },

  mfaSetup: async (): Promise<MFASetupResponse> => {
    const { data } = await apiClient.get<MFASetupResponse>(ENDPOINTS.auth.mfaSetup);
    return data;
  },

  mfaConfirm: async (code: string): Promise<{ success: boolean; mfa_enabled: boolean }> => {
    const { data } = await apiClient.post(ENDPOINTS.auth.mfaConfirm, { code });
    return data;
  },

  // OAuth2: redirect browser đến backend (không dùng apiClient)
  oauthGoogle: (): void => {
    window.location.href = ENDPOINTS.auth.oauthGoogle;
  },
  oauthGitHub: (): void => {
    window.location.href = ENDPOINTS.auth.oauthGitHub;
  },
};
