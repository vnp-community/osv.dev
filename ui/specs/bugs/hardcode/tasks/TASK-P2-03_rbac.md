# TASK-P2-03 — Fix `RBACManagement.tsx` → `useRBACMatrix()`

**Phase:** 2 — Admin Module  
**Nguồn giải pháp:** [`solutions/03_admin_rbac.md`](../solutions/03_admin_rbac.md)  
**Ưu tiên:** 🟠 Admin — ưu tiên cao  
**Phụ thuộc:** TASK-P1-03, TASK-P2-01 (dùng chung `features/admin/types.ts`)

---

## Vấn đề hiện tại

```typescript
// ❌ HIỆN TẠI — features/admin/components/RBACManagement.tsx
const ROLES = [
  { name: 'admin', users: 2, ... },
  { name: 'user', users: 4, ... },
  // hardcode 4 roles
];
const PERMISSIONS = ['dashboard.view', 'scan:create', ...]; // 31 permissions hardcode
const MATRIX = {
  admin: { 'dashboard.view': true, ... },
  // toàn bộ matrix hardcode
};
```

---

## API Endpoint

```
GET /api/v1/admin/roles → Roles + permission matrix + user counts
```

---

## Danh sách files cần tạo/sửa

### [MODIFY] `src/features/admin/types.ts` — thêm RBAC types

```typescript
export interface RBACRole {
  id: string;
  name: 'admin' | 'user' | 'readonly' | 'agent';
  displayName: string;
  description: string;
  userCount: number;
  color: string;
  permissions: string[];
}

export interface RBACPermissionCategory {
  category: string;
  items: string[];
}

export interface RBACMatrixResponse {
  roles: RBACRole[];
  permissionCategories: RBACPermissionCategory[];
}
```

### [NEW] `src/features/admin/hooks/useRBACMatrix.ts`

```typescript
export function useRBACMatrix() {
  return useQuery<RBACMatrixResponse>({
    queryKey: ['admin', 'rbac', 'matrix'],
    queryFn: async () => {
      const { data } = await apiClient.get<RBACMatrixResponse>(ENDPOINTS.admin.roles);
      return data;
    },
    staleTime: 5 * 60_000,
  });
}
```

### [MODIFY] `src/features/admin/components/RBACManagement.tsx`

Xem code đầy đủ tại: [`solutions/03_admin_rbac.md`](../solutions/03_admin_rbac.md) — mục "Component sau khi fix"

**Thay đổi chính:**
- Xóa `ROLES`, `PERMISSIONS`, `MATRIX` constants
- Import `useRBACMatrix`
- Build permission matrix từ server data: `role.permissions.includes(perm)`
- Role cards hiển thị `role.userCount` từ server (không hardcode "2", "4")
- Category filter động từ `permissionCategories`

### [MODIFY] `src/mocks/handlers/admin.handlers.ts` — thêm roles handler

```typescript
http.get('/api/v1/admin/roles', () => {
  return HttpResponse.json({
    roles: [...],
    permissionCategories: [...],
  });
}),
```

Xem data fixture đầy đủ tại: [`solutions/03_admin_rbac.md`](../solutions/03_admin_rbac.md) — mục "MSW Handler"

---

## Tiêu chí hoàn thành

- [x] `features/admin/types.ts` có RBACPermission, RBACMatrixResponse
- [x] `features/admin/hooks/useRBACMatrix.ts` tạo xong
- [x] `RBACManagement.tsx` không còn `ROLES`, `PERMISSIONS`, `MATRIX` constants
- [x] Role cards hiển thị số permission từ server (không hardcode "2", "4")
- [x] Permission matrix render động từ server data, group theo prefix
- [x] MSW handler trả về đúng 4 roles và 34 permissions
- [x] Loading/error state
- [x] TypeScript 0 lỗi mới

---

## ✅ Đã hoàn thành — 2026-06-19

**Files đã tạo/sửa:**
- [`features/admin/types.ts`](../../../../ui/src/features/admin/types.ts) — RBACPermission, RBACMatrixResponse
- [`features/admin/hooks/useRBACMatrix.ts`](../../../../ui/src/features/admin/hooks/useRBACMatrix.ts) — [NEW]
- [`features/admin/components/RBACManagement.tsx`](../../../../ui/src/features/admin/components/RBACManagement.tsx) — [MODIFY] Dynamic grouping
- [`mocks/handlers/admin.handlers.ts`](../../../../ui/src/mocks/handlers/admin.handlers.ts) — 34 permissions, real RBAC matrix

---

## Kiểm tra

```bash
# Mở http://localhost:3000 → Admin → RBAC Management
# 1. Verify: 4 role cards với userCount từ server (2, 4, 3, 1)
# 2. Filter "AI" → chỉ hiện AI permissions
# 3. Verify: Admin có Check ở ai:triage, User cũng có Check, Readonly không có
```
