# TASK-005 — Normalize `useRBACMatrix` để fix React Error #31 — CRITICAL

**Bug**: [BUG-admin-roles.md](../BUG-admin-roles.md)  
**Solution**: [SOL-013-rbac.md](../solutions/SOL-013-rbac.md)  
**Priority**: 🔴 P0 — 26 errors, trang crash hoàn toàn  
**Effort**: ~15 phút  
**Status**: `[x] DONE`

---

## Mô tả

Backend `/api/v1/admin/roles` trả về `roles` là **array of objects** `[{name, label, description}]` thay vì `string[]` như TypeScript type khai báo. Khi component render `{ROLE_DISPLAY[role] ?? role}` với `role` là object → React Error #31: "Objects are not valid as a React child".

---

## File cần sửa

**File**: [`ui/src/features/admin/hooks/useRBACMatrix.ts`](file:///Users/binhnt/Lab/sec/cve/osv.dev/ui/src/features/admin/hooks/useRBACMatrix.ts)

---

## Thay đổi chính xác

**Thay toàn bộ nội dung file** thành:

```typescript
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
```

---

## Kiểm tra response thực tế từ backend (chạy trước)

```bash
# Lấy token
TOKEN=$(curl -s -X POST https://c12.openledger.vn/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"email":"admin@openvulnscan.io","password":"Admin@123!ChangeMe"}' \
  | python3 -c "import sys,json; print(json.load(sys.stdin).get('access_token',''))")

# Xem shape của roles field
curl -s -H "Authorization: Bearer $TOKEN" \
  https://c12.openledger.vn/api/v1/admin/roles | python3 -c "
import sys, json
d = json.load(sys.stdin)
print('roles[0] type:', type(d.get('roles', [None])[0]).__name__ if d.get('roles') else 'empty')
print('roles[0] value:', d.get('roles', [None])[0])
print('perms[0].roles type:', type(list(d.get('permissions',[{}])[0].get('roles',{}).values())[0]).__name__ if d.get('permissions') else 'empty')
"
```

---

## Acceptance Criteria

- [ ] `useRBACMatrix` trả về `roles: string[]` dù backend trả object array
- [ ] Trang `/admin/roles` không còn React Error #31
- [ ] Permission matrix render đúng với ✓/✗ icons
- [ ] TypeScript không có lỗi mới (type `raw` là `unknown`)

---

## Verify

```bash
cd /Users/binhnt/Lab/sec/cve/osv.dev/ui
npx tsc --noEmit 2>&1 | grep -E "useRBACMatrix|RBACManagement"
```
