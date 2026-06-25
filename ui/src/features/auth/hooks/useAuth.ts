import { useCallback } from 'react';
import { useNavigate, useLocation } from 'react-router';
import { useMutation, useQueryClient } from '@tanstack/react-query';
import { useAuthStore } from '../store/authStore';
import { authApi } from '../api/authApi';
import type { LoginRequest } from '../types';

export function useAuth() {
  const {
    user, accessToken, isAuthenticated, isLoading,
    setUser, setAccessToken, setLoading, logout: storeLogout,
  } = useAuthStore();
  const navigate = useNavigate();
  const location = useLocation();
  const queryClient = useQueryClient();

  // ─── Login ─────────────────────────────────────────────────────────────
  const loginMutation = useMutation({
    mutationFn: authApi.login,
    onSuccess: (data) => {
      if (data.mfa_required) {
        navigate('/login/mfa', { state: { mfaRequired: true } });
        return;
      }
      if (data.access_token && data.user) {
        setAccessToken(data.access_token);
        setUser(data.user);
        const from = (location.state as { from?: Location })?.from?.pathname ?? '/dashboard';
        navigate(from, { replace: true });
      }
    },
  });

  // ─── Logout ────────────────────────────────────────────────────────────
  const logoutMutation = useMutation({
    mutationFn: authApi.logout,
    onSettled: () => {
      storeLogout();
      queryClient.clear();
      navigate('/login', { replace: true });
    },
  });

  // ─── Session Restore ───────────────────────────────────────────────────
  // Gọi khi app khởi động để restore session.
  // - Nếu accessToken đã có trong memory (login trong cùng tab) → chỉ set isLoading=false
  // - Nếu accessToken là null (page reload) → POST /refresh cookie → GET /me
  // - Nếu refresh thất bại → logout
  const restoreSession = useCallback(async () => {
    // Nếu đã có token trong memory → không cần restore lại
    if (accessToken) {
      setLoading(false);
      return;
    }

    setLoading(true);
    try {
      // Bước 1: Dùng refresh cookie để lấy access_token mới
      const refreshResp = await authApi.refresh();
      if (refreshResp.access_token) {
        setAccessToken(refreshResp.access_token);
      }
      // Bước 2: Lấy thông tin user với token mới
      const { user } = await authApi.me();
      setUser(user);
    } catch {
      // Refresh thất bại → user phải login lại
      storeLogout();
    } finally {
      setLoading(false);
    }
  }, [accessToken, setUser, setAccessToken, storeLogout, setLoading]);

  return {
    user,
    accessToken,
    isAuthenticated,
    isLoading,
    login:       (payload: LoginRequest) => loginMutation.mutate(payload),
    logout:      () => logoutMutation.mutate(),
    restoreSession,
    isLoggingIn: loginMutation.isPending,
    loginError:  loginMutation.error,
  };
}
