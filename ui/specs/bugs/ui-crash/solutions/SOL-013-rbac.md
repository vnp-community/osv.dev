# SOL-013 — RBAC Management: React Error #31 (Object rendered as React child) — CRITICAL

**Bug file**: [BUG-admin-roles.md](../BUG-admin-roles.md)  
**Route**: `/admin/roles`  
**Component**: [`RBACManagement.tsx`](file:///Users/binhnt/Lab/sec/cve/osv.dev/ui/src/features/admin/components/RBACManagement.tsx)  
**Hook**: [`useRBACMatrix.ts`](file:///Users/binhnt/Lab/sec/cve/osv.dev/ui/src/features/admin/hooks/useRBACMatrix.ts)  
**Priority**: 🔴 P0 — **26 errors, trang crash hoàn toàn**

---

## Root Cause

React Error #31 với `args[]= object with keys {description, label, value}` có nghĩa là **một object JavaScript đang được render trực tiếp vào JSX** thay vì một giá trị primitive (string/number).

**Decoded error**: `Objects are not valid as a React child (found: object with keys {description, label, value})`

Phân tích [`RBACManagement.tsx`](file:///Users/binhnt/Lab/sec/cve/osv.dev/ui/src/features/admin/components/RBACManagement.tsx):

```typescript
// RBACMatrixResponse.roles được dùng làm:
// 1. Key trong ROLE_COLORS/ROLE_DISPLAY lookup — OK nếu là string
// 2. Render trực tiếp: {ROLE_DISPLAY[r] ?? r}  — OK nếu r là string

const roles = data?.roles ?? [];   // type: string[]
```

**Possible causes:**

**Cause A** — Backend trả `roles` là array of objects thay vì strings:
```json
// Backend trả về (SAI):
{ "roles": [{"name": "admin", "label": "Admin", "description": "..."}, ...] }
// Component expect (ĐÚNG):
{ "roles": ["admin", "user", "readonly", "agent"] }
```

**Cause B** — Backend trả `permissions[].roles` là object có shape khác:
```json
// Backend trả về:
{ "permission": "scan.create", "roles": { "admin": { "value": true, "label": "..." } } }
// Component expect:
{ "permission": "scan.create", "roles": { "admin": true } }
```

→ Khi `perm.roles[role]` là object `{ value: true, label: "..." }` thay vì `boolean`,  
component render `{perm.roles[role] ? <Check/> : <X/>}` sẽ gặp truthy object và hiển thị `<Check/>` đúng,  
NHƯNG nếu có chỗ nào đó render `{perm.roles[role]}` trực tiếp sẽ crash.

---

## Fix

### Fix 1 — Normalize response trong `useRBACMatrix` hook (Recommended)

```typescript
// ui/src/features/admin/hooks/useRBACMatrix.ts
import { useQuery } from '@tanstack/react-query';
import { apiClient } from '@/shared/api/client';
import { ENDPOINTS } from '@/shared/api/endpoints';
import type { RBACMatrixResponse, RBACPermission } from '../types';

export function useRBACMatrix() {
  return useQuery<RBACMatrixResponse>({
    queryKey: ['admin', 'rbac', 'matrix'],
    queryFn: async () => {
      const { data } = await apiClient.get<unknown>(ENDPOINTS.admin.roles);
      return normalizeRBACMatrix(data);
    },
    staleTime: 5 * 60_000,
  });
}

function normalizeRBACMatrix(raw: unknown): RBACMatrixResponse {
  const d = raw as Record<string, unknown>;

  // Normalize roles: handle both string[] and {name: string}[]
  const rawRoles = Array.isArray(d?.roles) ? d.roles : [];
  const roles: string[] = rawRoles.map((r: unknown) =>
    typeof r === 'string' ? r : (r as Record<string, string>)?.name ?? String(r)
  );

  // Normalize permissions
  const rawPerms = Array.isArray(d?.permissions) ? d.permissions : [];
  const permissions: RBACPermission[] = rawPerms.map((p: unknown) => {
    const perm = p as Record<string, unknown>;
    // Normalize roles map: { admin: true } OR { admin: { value: true } }
    const rawRolesMap = (perm?.roles ?? {}) as Record<string, unknown>;
    const normalizedRoles: Record<string, boolean> = {};
    for (const [role, val] of Object.entries(rawRolesMap)) {
      if (typeof val === 'boolean') {
        normalizedRoles[role] = val;
      } else if (typeof val === 'object' && val !== null) {
        normalizedRoles[role] = Boolean((val as Record<string, unknown>)?.value ?? false);
      } else {
        normalizedRoles[role] = Boolean(val);
      }
    }
    return {
      permission:  String(perm?.permission ?? ''),
      description: String(perm?.description ?? ''),
      roles:       normalizedRoles,
    };
  });

  return { roles, permissions };
}
```

### Fix 2 — Tạm thời: Defensive render trong component

```typescript
// RBACManagement.tsx:86 — safe render role
{roles.map((role) => {
  // Đảm bảo role là string trước khi render
  const roleStr = typeof role === 'string' ? role : (role as Record<string,string>)?.name ?? '';
  const color = ROLE_COLORS[roleStr] ?? "#6B7280";
  // ...
})}
```

---

## Kiểm tra nhanh với curl

```bash
# Lấy response thực tế từ backend
curl -s -H "Authorization: Bearer $TOKEN" \
  https://c12.openledger.vn/api/v1/admin/roles | jq '.roles[0]'

# Nếu output là string "admin" → roles OK
# Nếu output là {"name":"admin",...} → Cause A confirmed
```

---

## Files cần sửa

| File | Thay đổi |
|------|---------|
| [`useRBACMatrix.ts`](file:///Users/binhnt/Lab/sec/cve/osv.dev/ui/src/features/admin/hooks/useRBACMatrix.ts) | Thêm `normalizeRBACMatrix()` function, type raw response là `unknown` |
| [`RBACManagement.tsx`](file:///Users/binhnt/Lab/sec/cve/osv.dev/ui/src/features/admin/components/RBACManagement.tsx) | Optional defensive guard |
