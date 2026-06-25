# Solution 01 — UserManagement.tsx

## Vấn đề
`const users = [...]` — 7 users hardcode, mọi thao tác Edit/Disable không có tác dụng.

## API Endpoint
```
GET  /api/v1/admin/users          → Danh sách users (có filter, search, pagination)
POST /api/v1/admin/users/invite   → Mời user mới
PATCH /api/v1/admin/users/:id     → Cập nhật role/status
DELETE /api/v1/admin/users/:id    → Disable user
```

## TypeScript Types (từ TDD.md Section 11.2)

```typescript
// features/admin/types.ts
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

## Hook mới: `useAdminUsers`

```typescript
// features/admin/hooks/useAdminUsers.ts
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
    staleTime: 60_000,  // 1 phút — user list ít thay đổi
  });
}

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

export function useUpdateUser() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({ id, ...body }: { id: string; role?: string; isActive?: boolean }) =>
      apiClient.patch(ENDPOINTS.admin.userDetail(id), body),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: adminUserKeys.all });
    },
  });
}
```

## Component sau khi fix

```typescript
// features/admin/components/UserManagement.tsx
import { useState } from 'react';
import { Users, UserPlus, AlertTriangle, CheckCircle, Search } from 'lucide-react';
import { QueryBoundary } from '@/shared/components/feedback/QueryBoundary';
import { useAdminUsers, useInviteUser, useUpdateUser } from '../hooks/useAdminUsers';

// ── UI constants (không phải data) ─────────────────────────────────────────
const ROLE_STYLES: Record<string, { bg: string; color: string }> = {
  admin:    { bg: 'rgba(239,68,68,0.1)',   color: '#EF4444' },
  user:     { bg: 'rgba(79,140,255,0.1)',  color: '#4F8CFF' },
  readonly: { bg: 'rgba(107,114,128,0.1)', color: '#9CA3AF' },
  agent:    { bg: 'rgba(16,185,129,0.1)',  color: '#10B981' },
};

function UserManagementSkeleton() {
  return (
    <div className="flex-1 overflow-y-auto px-6 py-5 animate-pulse" style={{ background: '#0B1020' }}>
      <div className="h-8 rounded mb-6" style={{ background: '#151B2F', width: 200 }} />
      <div className="grid grid-cols-4 gap-4 mb-5">
        {Array.from({ length: 4 }).map((_, i) => (
          <div key={i} className="rounded-xl h-16" style={{ background: '#151B2F' }} />
        ))}
      </div>
      <div className="rounded-2xl h-64" style={{ background: '#151B2F' }} />
    </div>
  );
}

export function UserManagement() {
  const [search, setSearch] = useState('');
  const [showInvite, setShowInvite] = useState(false);
  const [inviteEmail, setInviteEmail] = useState('');
  const [inviteName, setInviteName] = useState('');
  const [inviteRole, setInviteRole] = useState('user');

  // ✅ Server data — không hardcode
  const usersQuery = useAdminUsers({ search });
  const inviteUser = useInviteUser();
  const updateUser = useUpdateUser();

  const handleInvite = async () => {
    await inviteUser.mutateAsync({ email: inviteEmail, name: inviteName, role: inviteRole as any });
    setShowInvite(false);
    setInviteEmail('');
    setInviteName('');
  };

  return (
    <QueryBoundary query={usersQuery} skeleton={<UserManagementSkeleton />}>
      {({ users, total }) => {
        const noMfaCount = users.filter(u => !u.mfaEnabled).length;

        return (
          <div className="flex-1 overflow-y-auto px-6 py-5" style={{ background: '#0B1020' }}>
            {/* Header */}
            <div className="flex items-center justify-between mb-6">
              <div>
                <h2 style={{ color: '#E5E7EB', fontSize: 18, fontWeight: 700 }}>User Management</h2>
                <p style={{ color: '#6B7280', fontSize: 12 }}>
                  {total} users · {noMfaCount} without MFA
                </p>
              </div>
              <button
                onClick={() => setShowInvite(true)}
                className="flex items-center gap-2 px-4 py-2 rounded-xl"
                style={{ background: 'linear-gradient(135deg, #4F8CFF, #3B6FCC)', color: 'white', border: 'none', fontSize: 13, cursor: 'pointer' }}
              >
                <UserPlus size={14} />Invite User
              </button>
            </div>

            {/* Role stats — computed từ server data */}
            <div className="grid grid-cols-4 gap-4 mb-5">
              {Object.entries(ROLE_STYLES).map(([role, style]) => (
                <div key={role} className="rounded-xl px-4 py-3 flex items-center gap-3"
                  style={{ background: style.bg, border: `1px solid ${style.color}30` }}>
                  <div style={{ color: style.color, fontSize: 22, fontWeight: 700 }}>
                    {users.filter(u => u.role === role).length}
                  </div>
                  <div style={{ color: '#9CA3AF', fontSize: 12, textTransform: 'capitalize' }}>{role}</div>
                </div>
              ))}
            </div>

            {/* Search */}
            <div className="relative mb-4 max-w-sm">
              <Search size={13} color="#4B5563" style={{ position: 'absolute', left: 10, top: '50%', transform: 'translateY(-50%)' }} />
              <input
                value={search}
                onChange={e => setSearch(e.target.value)}
                placeholder="Search users..."
                className="w-full rounded-xl pl-8 pr-4 py-2.5 outline-none"
                style={{ background: '#151B2F', border: '1px solid rgba(255,255,255,0.08)', color: '#E5E7EB', fontSize: 13 }}
              />
            </div>

            {/* Table — data từ server */}
            <div className="rounded-2xl" style={{ background: '#151B2F', border: '1px solid rgba(255,255,255,0.07)' }}>
              <table className="w-full">
                <thead>
                  <tr style={{ borderBottom: '1px solid rgba(255,255,255,0.06)' }}>
                    {['User', 'Email', 'Role', 'MFA', 'Last Login', 'Status', 'Actions'].map(h => (
                      <th key={h} className="px-5 py-3 text-left" style={{ color: '#6B7280', fontSize: 11, fontWeight: 600, letterSpacing: 0.5 }}>{h}</th>
                    ))}
                  </tr>
                </thead>
                <tbody>
                  {users.map((u, i) => (
                    <tr key={u.id} className="transition-all"
                      style={{ borderBottom: i < users.length - 1 ? '1px solid rgba(255,255,255,0.04)' : 'none' }}
                      onMouseEnter={e => (e.currentTarget.style.background = 'rgba(255,255,255,0.02)')}
                      onMouseLeave={e => (e.currentTarget.style.background = 'transparent')}
                    >
                      <td className="px-5 py-3">
                        <div className="flex items-center gap-3">
                          <div className="w-8 h-8 rounded-full flex items-center justify-center text-white"
                            style={{ background: !u.isActive ? '#374151' : 'linear-gradient(135deg, #4F8CFF, #7C3AED)', fontSize: 11, fontWeight: 700 }}>
                            {u.name.split(' ').map(n => n[0]).join('').slice(0, 2).toUpperCase()}
                          </div>
                          <span style={{ color: !u.isActive ? '#6B7280' : '#E5E7EB', fontSize: 13 }}>{u.name}</span>
                        </div>
                      </td>
                      <td className="px-5 py-3"><span style={{ color: '#6B7280', fontSize: 12 }}>{u.email}</span></td>
                      <td className="px-5 py-3">
                        <span className="px-2 py-0.5 rounded" style={{ ...ROLE_STYLES[u.role], fontSize: 11 }}>{u.role}</span>
                      </td>
                      <td className="px-5 py-3">
                        {u.mfaEnabled
                          ? <div className="flex items-center gap-1.5"><CheckCircle size={13} color="#10B981" /><span style={{ color: '#10B981', fontSize: 12 }}>Enabled</span></div>
                          : <div className="flex items-center gap-1.5"><AlertTriangle size={13} color="#F59E0B" /><span style={{ color: '#F59E0B', fontSize: 12 }}>Disabled</span></div>
                        }
                      </td>
                      <td className="px-5 py-3">
                        <span style={{ color: '#6B7280', fontSize: 12 }}>
                          {u.lastLoginAt ? new Date(u.lastLoginAt).toLocaleString() : 'Never'}
                        </span>
                      </td>
                      <td className="px-5 py-3">
                        <span className="px-2 py-0.5 rounded" style={{
                          background: u.isActive ? 'rgba(16,185,129,0.1)' : 'rgba(107,114,128,0.1)',
                          color: u.isActive ? '#10B981' : '#6B7280', fontSize: 11
                        }}>
                          {u.isActive ? 'active' : 'disabled'}
                        </span>
                      </td>
                      <td className="px-5 py-3">
                        <div className="flex items-center gap-2">
                          <button className="px-2.5 py-1 rounded-lg" style={{ background: 'rgba(255,255,255,0.05)', color: '#9CA3AF', border: 'none', cursor: 'pointer', fontSize: 11 }}>
                            Edit
                          </button>
                          <button
                            onClick={() => updateUser.mutate({ id: u.id, isActive: !u.isActive })}
                            className="px-2.5 py-1 rounded-lg"
                            style={{ background: 'rgba(239,68,68,0.08)', color: '#EF4444', border: 'none', cursor: 'pointer', fontSize: 11 }}>
                            {u.isActive ? 'Disable' : 'Enable'}
                          </button>
                        </div>
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>

            {/* Invite modal */}
            {showInvite && (
              <div className="fixed inset-0 z-50 flex items-center justify-center" style={{ background: 'rgba(0,0,0,0.7)', backdropFilter: 'blur(4px)' }}>
                <div className="w-full max-w-md rounded-2xl p-6" style={{ background: '#151B2F', border: '1px solid rgba(255,255,255,0.1)' }}>
                  <div className="flex items-center justify-between mb-5">
                    <h3 style={{ color: '#E5E7EB', fontSize: 16, fontWeight: 600 }}>Invite User</h3>
                    <button onClick={() => setShowInvite(false)} style={{ color: '#6B7280', background: 'none', border: 'none', cursor: 'pointer' }}>✕</button>
                  </div>
                  <div className="mb-4">
                    <label style={{ color: '#9CA3AF', fontSize: 13, display: 'block', marginBottom: 6 }}>Email Address</label>
                    <input type="email" value={inviteEmail} onChange={e => setInviteEmail(e.target.value)}
                      placeholder="user@company.com"
                      className="w-full rounded-xl px-4 py-3 outline-none"
                      style={{ background: '#0F1629', border: '1px solid rgba(255,255,255,0.09)', color: '#E5E7EB', fontSize: 13 }} />
                  </div>
                  <div className="mb-4">
                    <label style={{ color: '#9CA3AF', fontSize: 13, display: 'block', marginBottom: 6 }}>Full Name</label>
                    <input type="text" value={inviteName} onChange={e => setInviteName(e.target.value)}
                      placeholder="Jane Smith"
                      className="w-full rounded-xl px-4 py-3 outline-none"
                      style={{ background: '#0F1629', border: '1px solid rgba(255,255,255,0.09)', color: '#E5E7EB', fontSize: 13 }} />
                  </div>
                  <div className="mb-5">
                    <label style={{ color: '#9CA3AF', fontSize: 13, display: 'block', marginBottom: 6 }}>Role</label>
                    <select value={inviteRole} onChange={e => setInviteRole(e.target.value)}
                      className="w-full rounded-xl px-4 py-3 outline-none"
                      style={{ background: '#0F1629', border: '1px solid rgba(255,255,255,0.09)', color: '#E5E7EB', fontSize: 13 }}>
                      <option value="user">User</option>
                      <option value="admin">Admin</option>
                      <option value="readonly">Readonly</option>
                      <option value="agent">Agent</option>
                    </select>
                  </div>
                  <div className="flex gap-3">
                    <button onClick={() => setShowInvite(false)} className="flex-1 py-2.5 rounded-xl"
                      style={{ background: 'rgba(255,255,255,0.07)', color: '#9CA3AF', border: 'none', cursor: 'pointer' }}>
                      Cancel
                    </button>
                    <button onClick={handleInvite} disabled={inviteUser.isPending || !inviteEmail}
                      className="flex-1 py-2.5 rounded-xl"
                      style={{ background: 'linear-gradient(135deg, #4F8CFF, #3B6FCC)', color: 'white', border: 'none', cursor: 'pointer' }}>
                      {inviteUser.isPending ? 'Sending...' : 'Send Invite'}
                    </button>
                  </div>
                </div>
              </div>
            )}
          </div>
        );
      }}
    </QueryBoundary>
  );
}
```

## MSW Handler

```typescript
// src/mocks/handlers/admin.handlers.ts
import { http, HttpResponse } from 'msw';
import type { AdminUsersResponse } from '@/features/admin/types';

const usersFixture = [
  { id: 'u-1', name: 'Carol Anderson', email: 'carol@company.com', role: 'admin', isActive: true, mfaEnabled: true, lastLoginAt: new Date(Date.now() - 5 * 60000).toISOString(), createdAt: '2026-01-01T00:00:00Z', loginAttempts: 0, isLocked: false },
  { id: 'u-2', name: 'Bob Chen',       email: 'bob.chen@company.com', role: 'user', isActive: true, mfaEnabled: true, lastLoginAt: new Date(Date.now() - 3600000).toISOString(), createdAt: '2026-01-05T00:00:00Z', loginAttempts: 0, isLocked: false },
  { id: 'u-3', name: 'Alice Wu',       email: 'alice.wu@company.com', role: 'user', isActive: true, mfaEnabled: true, lastLoginAt: new Date(Date.now() - 7200000).toISOString(), createdAt: '2026-01-10T00:00:00Z', loginAttempts: 0, isLocked: false },
  { id: 'u-4', name: 'Dave Kim',       email: 'dave.kim@company.com', role: 'user', isActive: true, mfaEnabled: false, lastLoginAt: new Date(Date.now() - 86400000).toISOString(), createdAt: '2026-02-01T00:00:00Z', loginAttempts: 0, isLocked: false },
  { id: 'u-5', name: 'Eve Martinez',   email: 'eve.m@company.com', role: 'readonly', isActive: true, mfaEnabled: true, lastLoginAt: new Date(Date.now() - 3 * 86400000).toISOString(), createdAt: '2026-02-15T00:00:00Z', loginAttempts: 0, isLocked: false },
  { id: 'u-6', name: 'Frank Liu',      email: 'frank.l@company.com', role: 'agent', isActive: false, mfaEnabled: false, lastLoginAt: undefined, createdAt: '2026-03-01T00:00:00Z', loginAttempts: 0, isLocked: false },
];

export const adminUserHandlers = [
  http.get('/api/v1/admin/users', ({ request }) => {
    const url = new URL(request.url);
    const search = url.searchParams.get('search')?.toLowerCase() ?? '';
    const role = url.searchParams.get('role');

    let filtered = usersFixture;
    if (search) {
      filtered = filtered.filter(u =>
        u.name.toLowerCase().includes(search) ||
        u.email.toLowerCase().includes(search)
      );
    }
    if (role) filtered = filtered.filter(u => u.role === role);

    return HttpResponse.json({
      users: filtered,
      total: filtered.length,
      page: 1,
      pageSize: 50,
    } satisfies AdminUsersResponse);
  }),

  http.post('/api/v1/admin/users/invite', async ({ request }) => {
    const body = await request.json() as any;
    const newUser = {
      id: `u-${Date.now()}`,
      ...body,
      isActive: true,
      mfaEnabled: false,
      lastLoginAt: undefined,
      createdAt: new Date().toISOString(),
      loginAttempts: 0,
      isLocked: false,
    };
    usersFixture.push(newUser);
    return HttpResponse.json(newUser, { status: 201 });
  }),

  http.patch('/api/v1/admin/users/:id', async ({ params, request }) => {
    const body = await request.json() as any;
    const user = usersFixture.find(u => u.id === params.id);
    if (!user) return new HttpResponse(null, { status: 404 });
    Object.assign(user, body);
    return HttpResponse.json(user);
  }),
];
```
