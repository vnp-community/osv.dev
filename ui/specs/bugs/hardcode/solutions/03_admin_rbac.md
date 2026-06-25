# Solution 03 — RBACManagement.tsx

## Vấn đề
`const ROLES`, `const PERMISSIONS`, `const MATRIX` — toàn bộ RBAC matrix hardcode 31 permissions × 4 roles.  
Số user theo role cũng hardcode: `{ users: 2 }`, `{ users: 4 }`.

## API Endpoints
```
GET /api/v1/admin/roles    → Danh sách roles + permission matrix + user counts
```

## TypeScript Types

```typescript
// features/admin/types.ts — thêm vào
export interface RBACRole {
  id: string;
  name: 'admin' | 'user' | 'readonly' | 'agent';
  displayName: string;
  description: string;
  userCount: number;
  color: string;
  permissions: string[];  // e.g. ['dashboard.view', 'scan:create', ...]
}

export interface RBACPermissionCategory {
  category: string;
  items: string[];  // permission keys
}

export interface RBACMatrixResponse {
  roles: RBACRole[];
  permissionCategories: RBACPermissionCategory[];
}
```

## Hook mới: `useRBACMatrix`

```typescript
// features/admin/hooks/useRBACMatrix.ts
import { useQuery } from '@tanstack/react-query';
import { apiClient } from '@/shared/api/client';
import { ENDPOINTS } from '@/shared/api/endpoints';
import type { RBACMatrixResponse } from '../types';

export function useRBACMatrix() {
  return useQuery<RBACMatrixResponse>({
    queryKey: ['admin', 'rbac', 'matrix'],
    queryFn: async () => {
      const { data } = await apiClient.get<RBACMatrixResponse>(ENDPOINTS.admin.roles);
      return data;
    },
    staleTime: 5 * 60_000,  // 5 phút — roles ít thay đổi
  });
}
```

## Component sau khi fix

```typescript
// features/admin/components/RBACManagement.tsx
import { useState } from 'react';
import { Shield, Check, X } from 'lucide-react';
import { QueryBoundary } from '@/shared/components/feedback/QueryBoundary';
import { useRBACMatrix } from '../hooks/useRBACMatrix';

// ── UI constants ────────────────────────────────────────────────────────────
const ROLE_COLOR_MAP: Record<string, string> = {
  admin:    '#EF4444',
  user:     '#4F8CFF',
  readonly: '#9CA3AF',
  agent:    '#10B981',
};

function RBACMatrixSkeleton() {
  return (
    <div className="flex-1 overflow-y-auto px-6 py-5 animate-pulse" style={{ background: '#0B1020' }}>
      <div className="grid grid-cols-4 gap-4 mb-5">
        {Array.from({ length: 4 }).map((_, i) => (
          <div key={i} className="rounded-2xl h-24" style={{ background: '#151B2F' }} />
        ))}
      </div>
      <div className="rounded-2xl h-96" style={{ background: '#151B2F' }} />
    </div>
  );
}

export function RBACManagement() {
  const rbacQuery = useRBACMatrix();
  const [activeCategory, setActiveCategory] = useState<string | null>(null);

  return (
    <QueryBoundary query={rbacQuery} skeleton={<RBACMatrixSkeleton />}>
      {({ roles, permissionCategories }) => {
        // Build permission → roles map từ server data
        const permissionMatrix: Record<string, Record<string, boolean>> = {};
        permissionCategories.forEach(cat => {
          cat.items.forEach(perm => {
            permissionMatrix[perm] = {};
            roles.forEach(role => {
              permissionMatrix[perm][role.name] = role.permissions.includes(perm);
            });
          });
        });

        const visibleCategories = activeCategory
          ? permissionCategories.filter(c => c.category === activeCategory)
          : permissionCategories;

        return (
          <div className="flex-1 overflow-y-auto px-6 py-5" style={{ background: '#0B1020' }}>
            {/* Header */}
            <div className="flex items-center gap-3 mb-5">
              <div className="w-9 h-9 rounded-xl flex items-center justify-center" style={{ background: 'rgba(167,139,250,0.1)' }}>
                <Shield size={18} color="#A78BFA" />
              </div>
              <div>
                <h2 style={{ color: '#E5E7EB', fontSize: 18, fontWeight: 700 }}>RBAC Management</h2>
                <p style={{ color: '#6B7280', fontSize: 12 }}>Role-based access control · Read-only matrix view</p>
              </div>
            </div>

            {/* Role cards — từ server data */}
            <div className="grid grid-cols-4 gap-4 mb-5">
              {roles.map(role => {
                const color = ROLE_COLOR_MAP[role.name] ?? '#9CA3AF';
                return (
                  <div key={role.id} className="rounded-2xl p-4"
                    style={{ background: '#151B2F', border: `1px solid ${color}25` }}>
                    <div className="flex items-center gap-2 mb-2">
                      <div className="w-3 h-3 rounded-full" style={{ background: color }} />
                      <span style={{ color: '#E5E7EB', fontSize: 14, fontWeight: 600 }}>{role.displayName}</span>
                    </div>
                    <div style={{ color: '#6B7280', fontSize: 12, marginBottom: 8 }}>{role.description}</div>
                    <div className="flex items-center justify-between">
                      <span style={{ color: color, fontSize: 20, fontWeight: 700 }}>{role.userCount}</span>
                      <span style={{ color: '#6B7280', fontSize: 11 }}>users</span>
                    </div>
                    <div style={{ color: '#4B5563', fontSize: 11, marginTop: 4 }}>
                      {role.permissions.length} permissions
                    </div>
                  </div>
                );
              })}
            </div>

            {/* Category filter */}
            <div className="flex gap-2 mb-4">
              <button
                onClick={() => setActiveCategory(null)}
                className="px-3 py-1.5 rounded-lg"
                style={{
                  background: !activeCategory ? 'rgba(79,140,255,0.12)' : 'rgba(255,255,255,0.05)',
                  color: !activeCategory ? '#4F8CFF' : '#6B7280',
                  fontSize: 12, border: 'none', cursor: 'pointer',
                }}>
                All
              </button>
              {permissionCategories.map(cat => (
                <button key={cat.category}
                  onClick={() => setActiveCategory(cat.category === activeCategory ? null : cat.category)}
                  className="px-3 py-1.5 rounded-lg"
                  style={{
                    background: activeCategory === cat.category ? 'rgba(79,140,255,0.12)' : 'rgba(255,255,255,0.05)',
                    color: activeCategory === cat.category ? '#4F8CFF' : '#6B7280',
                    fontSize: 12, border: 'none', cursor: 'pointer',
                  }}>
                  {cat.category}
                </button>
              ))}
            </div>

            {/* Permission matrix — computed từ server data */}
            <div className="rounded-2xl overflow-hidden" style={{ background: '#151B2F', border: '1px solid rgba(255,255,255,0.07)' }}>
              <table className="w-full">
                <thead>
                  <tr style={{ borderBottom: '1px solid rgba(255,255,255,0.06)' }}>
                    <th className="px-4 py-3 text-left" style={{ color: '#6B7280', fontSize: 11, fontWeight: 600, width: 240 }}>PERMISSION</th>
                    {roles.map(role => (
                      <th key={role.id} className="px-4 py-3 text-center" style={{ color: ROLE_COLOR_MAP[role.name] ?? '#9CA3AF', fontSize: 11, fontWeight: 700 }}>
                        {role.displayName.toUpperCase()}
                      </th>
                    ))}
                  </tr>
                </thead>
                <tbody>
                  {visibleCategories.map(cat => (
                    <>
                      <tr key={`cat-${cat.category}`} style={{ background: 'rgba(255,255,255,0.02)' }}>
                        <td colSpan={roles.length + 1} className="px-4 py-2">
                          <span style={{ color: '#6B7280', fontSize: 10, fontWeight: 700, letterSpacing: 1 }}>{cat.category.toUpperCase()}</span>
                        </td>
                      </tr>
                      {cat.items.map(perm => (
                        <tr key={perm} style={{ borderBottom: '1px solid rgba(255,255,255,0.03)' }}
                          onMouseEnter={e => (e.currentTarget.style.background = 'rgba(255,255,255,0.02)')}
                          onMouseLeave={e => (e.currentTarget.style.background = 'transparent')}>
                          <td className="px-4 py-2.5">
                            <span style={{ color: '#9CA3AF', fontSize: 12, fontFamily: 'monospace' }}>{perm}</span>
                          </td>
                          {roles.map(role => {
                            const allowed = permissionMatrix[perm]?.[role.name] ?? false;
                            return (
                              <td key={role.id} className="px-4 py-2.5 text-center">
                                {allowed
                                  ? <Check size={14} color="#10B981" className="mx-auto" />
                                  : <X size={14} color="#374151" className="mx-auto" />
                                }
                              </td>
                            );
                          })}
                        </tr>
                      ))}
                    </>
                  ))}
                </tbody>
              </table>
            </div>
          </div>
        );
      }}
    </QueryBoundary>
  );
}
```

## MSW Handler

```typescript
// src/mocks/handlers/admin.handlers.ts — thêm vào
http.get('/api/v1/admin/roles', () => {
  return HttpResponse.json({
    roles: [
      {
        id: 'role-admin', name: 'admin', displayName: 'Admin',
        description: 'Full system access', userCount: 2, color: '#EF4444',
        permissions: [
          'dashboard.view', 'dashboard.export',
          'scan:create', 'scan:read', 'scan:delete',
          'finding:read', 'finding:write', 'finding:delete',
          'asset:read', 'asset:write',
          'user:manage', 'report:download', 'system:configure',
          'ai:triage', 'ai:enrichment',
          'admin.users', 'admin.roles', 'admin.audit', 'admin.settings',
          'integration.api_keys', 'integration.webhooks',
        ],
      },
      {
        id: 'role-user', name: 'user', displayName: 'User',
        description: 'Standard analyst access', userCount: 4, color: '#4F8CFF',
        permissions: [
          'dashboard.view', 'scan:create', 'scan:read',
          'finding:read', 'finding:write',
          'asset:read', 'report:download', 'ai:triage',
        ],
      },
      {
        id: 'role-readonly', name: 'readonly', displayName: 'Readonly',
        description: 'View-only access', userCount: 3, color: '#9CA3AF',
        permissions: [
          'dashboard.view', 'scan:read', 'finding:read', 'asset:read',
        ],
      },
      {
        id: 'role-agent', name: 'agent', displayName: 'Agent',
        description: 'Automated scanning', userCount: 1, color: '#10B981',
        permissions: ['agent:report', 'scan:create'],
      },
    ],
    permissionCategories: [
      { category: 'Dashboard', items: ['dashboard.view', 'dashboard.export'] },
      { category: 'Scanning', items: ['scan:create', 'scan:read', 'scan:delete'] },
      { category: 'Findings', items: ['finding:read', 'finding:write', 'finding:delete'] },
      { category: 'Assets', items: ['asset:read', 'asset:write'] },
      { category: 'AI', items: ['ai:triage', 'ai:enrichment'] },
      { category: 'Reports', items: ['report:download'] },
      { category: 'Integrations', items: ['integration.api_keys', 'integration.webhooks'] },
      { category: 'Administration', items: ['user:manage', 'system:configure', 'admin.users', 'admin.roles', 'admin.audit', 'admin.settings'] },
      { category: 'Agent', items: ['agent:report'] },
    ],
  });
}),
```
