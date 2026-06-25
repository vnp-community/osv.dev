# TASK-P2-01 — Fix `UserManagement.tsx` → `useAdminUsers()`

**Phase:** 2 — Admin Module  
**Nguồn giải pháp:** [`solutions/01_admin_user_management.md`](../solutions/01_admin_user_management.md)  
**Ưu tiên:** 🟠 Admin — ưu tiên cao  
**Phụ thuộc:** TASK-P1-03, TASK-P1-04

---

## Vấn đề hiện tại

```typescript
// ❌ HIỆN TẠI — features/admin/components/UserManagement.tsx
const users = [
  { id: 'u-1', name: 'Admin User', email: 'admin@company.com', role: 'admin', ... },
  { id: 'u-2', name: 'Bob Chen', ... },
  // 7 users hardcode
];
// Edit/Disable button không có tác dụng
```

---

## API Endpoints cần implement

```
GET    /api/v1/admin/users        → Danh sách users (filter, search, pagination)
POST   /api/v1/admin/users/invite → Mời user mới
PATCH  /api/v1/admin/users/:id    → Cập nhật role/status
DELETE /api/v1/admin/users/:id    → Disable user
```

---

## Danh sách files cần tạo/sửa

### [NEW] `src/features/admin/types.ts` — thêm AdminUser types

```typescript
export interface AdminUser {
  id: string;
  email: string;
  name: string;
  role: 'admin' | 'user' | 'readonly' | 'agent';
  isActive: boolean;
  mfaEnabled: boolean;
  lastLoginAt?: string;
  createdAt: string;
  loginAttempts: number;
  isLocked: boolean;
}

export interface AdminUsersResponse {
  users: AdminUser[];
  total: number;
  page: number;
  pageSize: number;
}

export interface InviteUserRequest {
  email: string;
  name: string;
  role: AdminUser['role'];
}
```

### [NEW] `src/features/admin/hooks/useAdminUsers.ts`

```typescript
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { apiClient } from '@/shared/api/client';
import { ENDPOINTS } from '@/shared/api/endpoints';
import type { AdminUsersResponse, InviteUserRequest } from '../types';

const adminUserKeys = {
  all: ['admin', 'users'] as const,
  list: (params?: Record<string, unknown>) => [...adminUserKeys.all, 'list', params] as const,
};

export function useAdminUsers(params?: { search?: string; role?: string; page?: number }) {
  return useQuery<AdminUsersResponse>({
    queryKey: adminUserKeys.list(params),
    queryFn: async () => {
      const { data } = await apiClient.get<AdminUsersResponse>(ENDPOINTS.admin.users, { params });
      return data;
    },
    staleTime: 60_000,
  });
}

export function useInviteUser() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (req: InviteUserRequest) => apiClient.post(ENDPOINTS.admin.userInvite, req),
    onSuccess: () => { queryClient.invalidateQueries({ queryKey: adminUserKeys.all }); },
  });
}

export function useUpdateUser() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({ id, ...body }: { id: string; role?: string; isActive?: boolean }) =>
      apiClient.patch(ENDPOINTS.admin.userDetail(id), body),
    onSuccess: () => { queryClient.invalidateQueries({ queryKey: adminUserKeys.all }); },
  });
}
```

### [MODIFY] `src/features/admin/components/UserManagement.tsx`

Xem code đầy đủ tại: [`solutions/01_admin_user_management.md`](../solutions/01_admin_user_management.md) — mục "Component sau khi fix"

**Thay đổi chính:**
- Xóa `const users = [...]`
- Import `useAdminUsers`, `useInviteUser`, `useUpdateUser`
- Wrap với `<QueryBoundary>`
- Invite modal thực sự gọi `inviteUser.mutateAsync()`
- Enable/Disable button thực sự gọi `updateUser.mutate()`

### [MODIFY] `src/shared/api/endpoints.ts` — thêm admin endpoints

```typescript
admin: {
  // ... existing
  users:           '/api/v1/admin/users',
  userInvite:      '/api/v1/admin/users/invite',
  userDetail:      (id: string) => `/api/v1/admin/users/${id}`,
  roles:           '/api/v1/admin/roles',
  settings:        '/api/v1/admin/settings',
  audit:           '/api/v1/audit-log',
},
```

### [NEW] `src/mocks/handlers/admin.handlers.ts`

Xem code đầy đủ tại: [`solutions/01_admin_user_management.md`](../solutions/01_admin_user_management.md) — mục "MSW Handler"

Export: `adminUserHandlers`

### [MODIFY] `src/mocks/handlers/index.ts`

Uncomment dòng `adminUserHandlers`:
```typescript
import { adminUserHandlers } from './admin.handlers';
export const handlers = [...adminUserHandlers, ...];
```

---

## Tiêu chí hoàn thành

- [x] `features/admin/types.ts` có đầy đủ AdminUser, AdminUsersResponse, InviteUserRequest
- [x] `features/admin/hooks/useAdminUsers.ts` tạo xong với 4 hooks (get, invite, update, unlock)
- [x] `UserManagement.tsx` không còn `const users = [...]`
- [x] Search box filter client-side (dữ liệu từ API)
- [x] Invite button mở modal và POST lên `/api/v1/admin/users/invite`
- [x] Enable/Disable button gọi `PATCH /api/v1/admin/users/:id`
- [x] Unlock button gọi `POST /api/v1/admin/users/:id/unlock` khi account bị locked
- [x] Loading spinner khi fetch, error state với Retry button
- [x] MSW handler import từ admin-users.fixture.ts, mutable array cho mutations
- [x] TypeScript 0 lỗi mới trong admin feature

---

## ✅ Đã hoàn thành — 2026-06-19

**Files đã tạo/sửa:**
- [`features/admin/types.ts`](../../../../ui/src/features/admin/types.ts) — [NEW] Types cho tất cả P2 tasks
- [`features/admin/hooks/useAdminUsers.ts`](../../../../ui/src/features/admin/hooks/useAdminUsers.ts) — [NEW] 4 hooks
- [`features/admin/components/UserManagement.tsx`](../../../../ui/src/features/admin/components/UserManagement.tsx) — [MODIFY] Refactored
- [`mocks/handlers/admin.handlers.ts`](../../../../ui/src/mocks/handlers/admin.handlers.ts) — [MODIFY] Import fixtures + search/filter


---

## Kiểm tra

```bash
# Mở http://localhost:3000 → Admin → User Management
# 1. Verify: users hiển thị từ MSW (không phải hardcode)
# 2. Search "carol" → chỉ hiện Carol Anderson
# 3. Click "Disable" trên 1 user → status thay đổi
# 4. Click "Invite User" → điền form → Send Invite → user mới xuất hiện
```
