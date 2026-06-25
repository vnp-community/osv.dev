import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { apiClient } from '@/shared/api/client';
import { ENDPOINTS } from '@/shared/api/endpoints';
import type {
  AdminUsersResponse,
  InviteUserRequest,
  UserRole,
} from '../types';

const adminUserKeys = {
  all: ['admin', 'users'] as const,
  list: (params?: Record<string, unknown>) =>
    [...adminUserKeys.all, 'list', params] as const,
};

// ─── GET /api/v1/admin/users ────────────────────────────────────────────────

export function useAdminUsers(params?: { search?: string; role?: string; page?: number }) {
  return useQuery<AdminUsersResponse>({
    queryKey: adminUserKeys.list(params),
    queryFn: async () => {
      const { data } = await apiClient.get<AdminUsersResponse>(
        ENDPOINTS.admin.users,
        { params }
      );
      return data;
    },
    staleTime: 60_000,
  });
}

// ─── POST /api/v1/admin/users/invite ────────────────────────────────────────

export function useInviteUser() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (req: InviteUserRequest) =>
      apiClient.post(ENDPOINTS.admin.userInvite, req),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: adminUserKeys.all });
    },
  });
}

// ─── PATCH /api/v1/admin/users/:id ──────────────────────────────────────────

export function useUpdateUser() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({
      id,
      ...body
    }: {
      id: string;
      role?: UserRole;
      is_active?: boolean;
    }) => apiClient.patch(ENDPOINTS.admin.userDetail(id), body),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: adminUserKeys.all });
    },
  });
}

// ─── POST /api/v1/admin/users/:id/unlock ────────────────────────────────────

export function useUnlockUser() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (id: string) =>
      apiClient.post(ENDPOINTS.admin.userUnlock(id)),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: adminUserKeys.all });
    },
  });
}
