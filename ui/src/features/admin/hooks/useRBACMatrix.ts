import { useQuery } from '@tanstack/react-query';
import { apiClient } from '@/shared/api/client';
import { ENDPOINTS } from '@/shared/api/endpoints';
import type { RBACMatrixResponse, RBACPermission } from '../types';

// ─── Normalize raw API response → typed RBACMatrixResponse ───────────────────
// Backend có thể trả roles là string[] HOẶC {name, label, description}[]
// Backend có thể trả permissions[].roles là Record<string,boolean> HOẶC Record<string,{value,label}>

function normalizeRBACMatrix(raw: unknown): RBACMatrixResponse {
  const d = (raw ?? {}) as Record<string, unknown>;

  // Normalize roles → luôn là string[]
  const rawRoles = Array.isArray(d.roles) ? d.roles : [];
  const roles: string[] = rawRoles.map((r: unknown) => {
    if (typeof r === 'string') return r;
    const obj = r as Record<string, unknown>;
    return String(obj?.name ?? obj?.id ?? r);
  });

  // Normalize permissions
  const rawPerms = Array.isArray(d.permissions) ? d.permissions : [];
  const permissions: RBACPermission[] = rawPerms.map((p: unknown) => {
    const perm = (p ?? {}) as Record<string, unknown>;

    // Normalize roles map: { admin: true } OR { admin: { value: true, label: "..." } }
    const rawRolesMap = (perm.roles ?? {}) as Record<string, unknown>;
    const normalizedRoles: Record<string, boolean> = {};
    for (const [role, val] of Object.entries(rawRolesMap)) {
      if (typeof val === 'boolean') {
        normalizedRoles[role] = val;
      } else if (val !== null && typeof val === 'object') {
        normalizedRoles[role] = Boolean((val as Record<string, unknown>).value ?? false);
      } else {
        normalizedRoles[role] = Boolean(val);
      }
    }

    return {
      permission:  String(perm.permission ?? ''),
      description: String(perm.description ?? ''),
      roles:       normalizedRoles,
    };
  });

  return { roles, permissions };
}

// ─── GET /api/v1/admin/roles ─────────────────────────────────────────────────

export function useRBACMatrix() {
  return useQuery<RBACMatrixResponse>({
    queryKey: ['admin', 'rbac', 'matrix'],
    queryFn: async () => {
      const { data } = await apiClient.get<unknown>(ENDPOINTS.admin.roles);
      return normalizeRBACMatrix(data);
    },
    staleTime: 5 * 60_000, // 5 minutes — RBAC matrix changes rarely
  });
}
