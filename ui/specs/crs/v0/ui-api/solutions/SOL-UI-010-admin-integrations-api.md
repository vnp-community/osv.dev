# SOL-UI-010 — Frontend Solution: Administration & Integrations API

**CR nguồn:** [CR-UI-010](../../../../../specs/crs/v0/ui-api/CR-UI-010-admin-integrations-api.md)  
**Ngày tạo:** 2026-06-16  
**Trạng thái:** Proposed  
**Ưu tiên:** P0 — Critical  
**Phạm vi:** Frontend React SPA (`ui/src/features/admin/`, `ui/src/features/integrations/`)

---

## 1. Tóm tắt giải pháp

CR-UI-010 bao phủ Admin (5 screens) + Integrations (3 screens). Frontend cần:

1. `adminApi.ts` — users, roles, health, settings
2. `integrationApi.ts` — API keys, JIRA config
3. `profileApi.ts` — user profile, change password
4. Hooks + components cho mỗi screen
5. RBAC guard: tất cả admin screens yêu cầu `user:manage` hoặc `system:configure`
6. System Health real-time refresh

---

## 2. File Structure

```
ui/src/
├── features/admin/
│   ├── api/
│   │   └── adminApi.ts
│   ├── hooks/
│   │   ├── useAdminUsers.ts
│   │   ├── useSystemHealth.ts
│   │   └── useSystemSettings.ts
│   ├── components/
│   │   ├── UserManagement.tsx       # /admin/users
│   │   ├── UserInviteDialog.tsx
│   │   ├── RBACManagement.tsx       # /admin/roles
│   │   ├── AuditLogs.tsx            # /admin/audit
│   │   ├── SystemHealth.tsx         # /admin/health
│   │   └── SystemSettings.tsx       # /admin/settings
│   └── types.ts
│
├── features/integrations/
│   ├── api/
│   │   └── integrationApi.ts
│   ├── hooks/
│   │   ├── useAPIKeys.ts
│   │   └── useJIRA.ts
│   ├── components/
│   │   ├── APIKeyManagement.tsx     # /integrations/api-keys
│   │   ├── APIKeyCreateDialog.tsx   # Shows plaintext key ONCE
│   │   └── JiraConfig.tsx           # /integrations/jira
│   └── types.ts
│
├── features/profile/
│   ├── api/
│   │   └── profileApi.ts
│   └── components/
│       └── UserProfile.tsx          # /profile
│
└── mocks/handlers/
    ├── admin.handlers.ts
    └── integration.handlers.ts
```

---

## 3. Implementation Chi Tiết

### 3.1 `features/admin/api/adminApi.ts`

```typescript
import apiClient from '@/shared/api/client';
import type {
  AdminUser, UserListResponse, RBACResponse,
  SystemHealthResponse, SystemSettings, AuditLogResponse
} from '../types';

export const adminApi = {
  // User Management
  listUsers: async (params: {
    q?: string;
    role?: string;
    is_active?: boolean;
    page?: number;
    page_size?: number;
  } = {}): Promise<UserListResponse> => {
    const { data } = await apiClient.get<UserListResponse>('/api/v1/admin/users', { params });
    return data;
  },

  inviteUser: async (payload: {
    email: string;
    name: string;
    role: string;
  }): Promise<AdminUser> => {
    const { data } = await apiClient.post<AdminUser>('/api/v1/admin/users/invite', payload);
    return data;
  },

  updateUser: async (id: string, payload: {
    role?: string;
    is_active?: boolean;
  }): Promise<AdminUser> => {
    const { data } = await apiClient.patch<AdminUser>(`/api/v1/admin/users/${id}`, payload);
    return data;
  },

  unlockUser: async (id: string): Promise<{ success: boolean }> => {
    const { data } = await apiClient.post(`/api/v1/admin/users/${id}/unlock`);
    return data;
  },

  resetPassword: async (id: string): Promise<{ success: boolean }> => {
    const { data } = await apiClient.post(`/api/v1/admin/users/${id}/reset-password`);
    return data;
  },

  // RBAC
  getRoles: async (): Promise<RBACResponse> => {
    const { data } = await apiClient.get<RBACResponse>('/api/v1/admin/roles');
    return data;
  },

  // System Health
  getHealth: async (): Promise<SystemHealthResponse> => {
    const { data } = await apiClient.get<SystemHealthResponse>('/api/v1/admin/health');
    return data;
  },

  // Settings
  getSettings: async (): Promise<SystemSettings> => {
    const { data } = await apiClient.get<SystemSettings>('/api/v1/admin/settings');
    return data;
  },

  updateSettings: async (payload: Partial<SystemSettings>): Promise<SystemSettings> => {
    const { data } = await apiClient.patch<SystemSettings>('/api/v1/admin/settings', payload);
    return data;
  },

  // Audit Log
  getAuditLog: async (params: {
    user_id?: string;
    action?: string;
    entity_type?: string;
    date_from?: string;
    date_to?: string;
    page?: number;
    page_size?: number;
  } = {}): Promise<AuditLogResponse> => {
    const { data } = await apiClient.get<AuditLogResponse>('/api/v1/audit-log', { params });
    return data;
  },
};
```

### 3.2 `features/integrations/api/integrationApi.ts`

```typescript
import apiClient from '@/shared/api/client';
import type { APIKey, APIKeyCreatedResponse, JIRAConfig } from '../types';

export const integrationApi = {
  // API Keys
  listAPIKeys: async (): Promise<{ api_keys: APIKey[]; total: number }> => {
    const { data } = await apiClient.get('/api/v1/api-keys');
    return data;
  },

  createAPIKey: async (payload: {
    name: string;
    permissions: string[];
    expires_at?: string | null;
  }): Promise<APIKeyCreatedResponse> => {
    const { data } = await apiClient.post<APIKeyCreatedResponse>('/api/v1/api-keys', payload);
    return data;
  },

  revokeAPIKey: async (id: string): Promise<void> => {
    await apiClient.delete(`/api/v1/api-keys/${id}`);
  },

  // JIRA
  getJIRAConfig: async (): Promise<JIRAConfig | null> => {
    try {
      const { data } = await apiClient.get<JIRAConfig>('/api/v1/jira/config');
      return data;
    } catch (e: any) {
      if (e.response?.status === 404) return null;
      throw e;
    }
  },

  saveJIRAConfig: async (payload: {
    jira_url: string;
    project_key: string;
    username: string;
    api_token: string;
  }): Promise<JIRAConfig> => {
    const { data } = await apiClient.post<JIRAConfig>('/api/v1/jira/config', payload);
    return data;
  },

  testJIRAConfig: async (): Promise<{
    success: boolean;
    jira_version: string;
    project_found: boolean;
    response_time_ms: number;
  }> => {
    const { data } = await apiClient.post('/api/v1/jira/config/test');
    return data;
  },
};
```

### 3.3 `features/admin/hooks/useSystemHealth.ts`

```typescript
import { useQuery } from '@tanstack/react-query';
import { adminApi } from '../api/adminApi';

export const adminKeys = {
  users: (params: object) => ['admin', 'users', params] as const,
  roles: () => ['admin', 'roles'] as const,
  health: () => ['admin', 'health'] as const,
  settings: () => ['admin', 'settings'] as const,
  auditLog: (params: object) => ['admin', 'audit-log', params] as const,
};

export function useSystemHealth() {
  return useQuery({
    queryKey: adminKeys.health(),
    queryFn: () => adminApi.getHealth(),
    staleTime: 30_000,
    refetchInterval: 60_000,  // Auto-refresh health mỗi 60s
  });
}
```

### 3.4 System Health Component

```tsx
// features/admin/components/SystemHealth.tsx
const STATUS_COLORS = {
  healthy:  { color: '#10B981', bg: '#10B98120', dot: 'animate-pulse' },
  degraded: { color: '#F59E0B', bg: '#F59E0B20', dot: 'animate-bounce' },
  down:     { color: '#EF4444', bg: '#EF444420', dot: '' },
};

export function SystemHealth() {
  const { data, isLoading, dataUpdatedAt, refetch } = useSystemHealth();

  if (isLoading) return <SystemHealthSkeleton />;

  const overallStyle = STATUS_COLORS[data?.overall_status ?? 'healthy'];

  return (
    <div className="space-y-6">
      {/* Overall Status Banner */}
      <div
        className="flex items-center gap-3 p-4 rounded-lg border"
        style={{ backgroundColor: overallStyle.bg, borderColor: `${overallStyle.color}40` }}
      >
        <div
          className={`w-3 h-3 rounded-full ${overallStyle.dot}`}
          style={{ backgroundColor: overallStyle.color }}
        />
        <span className="font-semibold" style={{ color: overallStyle.color }}>
          System {data?.overall_status?.toUpperCase()}
        </span>
        <span className="text-xs text-[var(--text-muted)] ml-auto">
          Last checked: {formatDate(data?.checked_at)} ·{' '}
          <button onClick={() => refetch()} className="underline">Refresh</button>
        </span>
      </div>

      {/* Services Grid */}
      <div className="grid grid-cols-2 gap-4">
        {data?.services.map(service => {
          const style = STATUS_COLORS[service.status as keyof typeof STATUS_COLORS];
          return (
            <div
              key={service.name}
              className="bg-[var(--bg-elevated)] border border-[var(--border-base)] rounded-lg p-4"
            >
              <div className="flex items-center gap-2">
                <div
                  className={`w-2 h-2 rounded-full ${style.dot}`}
                  style={{ backgroundColor: style.color }}
                />
                <span className="font-medium text-[var(--text-primary)]">{service.name}</span>
                <span className="text-xs text-[var(--text-muted)] ml-auto">v{service.version}</span>
              </div>
              <div className="mt-2 flex items-center gap-2 text-sm text-[var(--text-secondary)]">
                <span>{service.response_time_ms}ms</span>
                {service.details && (
                  <span className="text-[var(--status-warning)]">• {service.details}</span>
                )}
              </div>
            </div>
          );
        })}
      </div>

      {/* Infrastructure */}
      <div className="grid grid-cols-4 gap-4">
        <InfraCard label="NATS" data={data?.nats} />
        <InfraCard label="PostgreSQL" data={data?.postgres} />
        <InfraCard label="Redis" data={data?.redis} />
        <InfraCard label="OpenSearch" data={data?.opensearch} />
      </div>
    </div>
  );
}
```

### 3.5 API Key Management — Show Once Pattern

```tsx
// features/integrations/components/APIKeyManagement.tsx
export function APIKeyManagement() {
  const { data: keysData } = useQuery({
    queryKey: ['api-keys'],
    queryFn: () => integrationApi.listAPIKeys(),
  });
  const [newKeySecret, setNewKeySecret] = useState<string | null>(null);
  const queryClient = useQueryClient();

  const createKey = useMutation({
    mutationFn: integrationApi.createAPIKey,
    onSuccess: (data) => {
      // IMPORTANT: Show plaintext_key ONCE in dialog
      setNewKeySecret(data.plaintext_key);
      queryClient.invalidateQueries({ queryKey: ['api-keys'] });
    },
  });

  return (
    <div>
      <DataTable
        data={keysData?.api_keys ?? []}    // ← từ server, không hardcode
        columns={[
          { key: 'name', label: 'Name' },
          { key: 'prefix', label: 'Key Prefix', render: (v) => <code>{v}</code> },
          { key: 'permissions', label: 'Permissions', render: (v) => v.join(', ') },
          { key: 'last_used_at', label: 'Last Used', render: (v) => v ? formatDate(v) : 'Never' },
          { key: 'actions', render: (_, row) => (
            <Button size="sm" variant="destructive"
              onClick={() => revokeKey.mutate(row.id)}>
              Revoke
            </Button>
          )},
        ]}
      />

      {/* Show plaintext key ONCE in dialog */}
      {newKeySecret && (
        <Dialog open onOpenChange={() => setNewKeySecret(null)}>
          <DialogContent>
            <DialogHeader>
              <AlertTriangleIcon className="text-yellow-500" />
              Save Your API Key
            </DialogHeader>
            <p className="text-sm text-[var(--text-secondary)]">
              This key will only be shown <strong>once</strong>. Copy it now.
            </p>
            <CodeBlock code={newKeySecret} showCopyButton />
            <DialogFooter>
              <Button onClick={() => setNewKeySecret(null)}>I've saved the key</Button>
            </DialogFooter>
          </DialogContent>
        </Dialog>
      )}
    </div>
  );
}
```

---

## 4. MSW Handlers

```typescript
// mocks/handlers/admin.handlers.ts
import { http, HttpResponse } from 'msw';

const usersFixture = [
  { id: 'usr_admin001', email: 'admin@company.com', name: 'Admin User',
    role: 'admin', is_active: true, mfa_enabled: true,
    last_login_at: '2026-06-16T10:00:00Z', created_at: '2026-01-01T00:00:00Z',
    login_attempts: 0, is_locked: false },
  { id: 'usr_bob123', email: 'bob@company.com', name: 'Bob Smith',
    role: 'user', is_active: true, mfa_enabled: false,
    last_login_at: '2026-06-16T09:00:00Z', created_at: '2026-01-15T08:00:00Z',
    login_attempts: 0, is_locked: false },
];

export const adminHandlers = [
  http.get('/api/v1/admin/users', () => {
    return HttpResponse.json({ users: usersFixture, total: usersFixture.length, page: 1, page_size: 20 });
  }),

  http.post('/api/v1/admin/users/invite', async ({ request }) => {
    const body = await request.json() as any;
    return HttpResponse.json({
      id: 'usr_' + Date.now(), email: body.email, name: body.name,
      role: body.role, status: 'invited', invite_sent_at: new Date().toISOString(),
    }, { status: 201 });
  }),

  http.patch('/api/v1/admin/users/:id', async ({ params, request }) => {
    const body = await request.json() as any;
    const user = usersFixture.find(u => u.id === params.id);
    return HttpResponse.json({ ...(user ?? usersFixture[0]), ...body });
  }),

  http.post('/api/v1/admin/users/:id/unlock', () => {
    return HttpResponse.json({ success: true, user_id: 'usr_bob123', is_locked: false });
  }),

  http.get('/api/v1/admin/roles', () => {
    return HttpResponse.json({
      roles: ['admin', 'user', 'readonly', 'agent'],
      permissions: [
        { permission: 'scan:create', description: 'Create and start scans',
          roles: { admin: true, user: true, readonly: false, agent: false } },
        { permission: 'finding:write', description: 'Update finding status',
          roles: { admin: true, user: true, readonly: false, agent: false } },
        { permission: 'user:manage', description: 'Manage users and roles',
          roles: { admin: true, user: false, readonly: false, agent: false } },
        { permission: 'system:configure', description: 'Configure system settings',
          roles: { admin: true, user: false, readonly: false, agent: false } },
        { permission: 'report:download', description: 'Download security reports',
          roles: { admin: true, user: true, readonly: true, agent: false } },
        { permission: 'agent:report', description: 'Submit agent scan reports',
          roles: { admin: false, user: false, readonly: false, agent: true } },
      ],
    });
  }),

  http.get('/api/v1/admin/health', () => {
    return HttpResponse.json({
      services: [
        { name: 'identity-service', status: 'healthy', response_time_ms: 12,
          last_checked_at: new Date().toISOString(), version: '2.2.0', details: null },
        { name: 'data-service', status: 'healthy', response_time_ms: 18,
          last_checked_at: new Date().toISOString(), version: '2.2.0', details: null },
        { name: 'search-service', status: 'degraded', response_time_ms: 850,
          last_checked_at: new Date().toISOString(), version: '2.2.0', details: 'OpenSearch high latency' },
        { name: 'finding-service', status: 'healthy', response_time_ms: 15,
          last_checked_at: new Date().toISOString(), version: '2.1.0', details: null },
      ],
      nats: { status: 'healthy', pending_messages: 12, consumer_lag: 0 },
      postgres: { status: 'healthy', active_connections: 45, max_connections: 200 },
      redis: { status: 'healthy', used_memory_mb: 128, max_memory_mb: 512 },
      opensearch: { status: 'degraded', indexed_docs: 312450 },
      overall_status: 'degraded',
      checked_at: new Date().toISOString(),
    });
  }),

  http.get('/api/v1/admin/settings', () => {
    return HttpResponse.json({
      general: { platform_name: 'OSV Platform', platform_url: 'https://osv.company.com' },
      security: { session_timeout_minutes: 480, max_login_attempts: 5,
        lockout_duration_minutes: 15, mfa_required: false },
      ai: { ollama_url: 'http://ollama:11434', openai_api_key_preview: 'sk-...abc',
        default_provider: 'ollama', triage_enabled: true },
      notifications: { smtp_host: 'smtp.company.com', smtp_port: 587,
        smtp_from: 'security@company.com', slack_webhook_url: null, teams_webhook_url: null },
    });
  }),

  http.patch('/api/v1/admin/settings', async ({ request }) => {
    const body = await request.json() as any;
    return HttpResponse.json(body);
  }),

  http.get('/api/v1/audit-log', () => {
    return HttpResponse.json({
      events: [
        { id: 'aud_001', user_id: 'usr_bob123', user_name: 'Bob Smith',
          action: 'finding.status_changed', entity_type: 'finding', entity_id: 'F-2847',
          ip_address: '10.0.0.1', user_agent: 'Mozilla/5.0...',
          result: 'success', metadata: { from: 'active', to: 'mitigated' },
          timestamp: '2026-06-16T11:00:00Z' },
      ],
      total: 1, page: 1, page_size: 50,
    });
  }),
];

// mocks/handlers/integration.handlers.ts
export const integrationHandlers = [
  http.get('/api/v1/api-keys', () => {
    return HttpResponse.json({
      api_keys: [
        { id: 'key_001', name: 'CI/CD Pipeline', prefix: 'ovs_live_a1b2c3',
          permissions: ['scan:read', 'finding:read'],
          created_at: '2026-03-01T00:00:00Z', last_used_at: '2026-06-16T08:00:00Z',
          expires_at: null, is_active: true },
      ],
      total: 1,
    });
  }),

  http.post('/api/v1/api-keys', async ({ request }) => {
    const body = await request.json() as any;
    return HttpResponse.json({
      id: 'key_' + Date.now(), name: body.name,
      prefix: 'ovs_live_' + Math.random().toString(36).slice(2, 8),
      plaintext_key: 'ovs_live_' + Math.random().toString(36).slice(2) + '_' + Math.random().toString(36).slice(2),
      permissions: body.permissions,
      created_at: new Date().toISOString(), expires_at: body.expires_at ?? null,
      is_active: true,
    }, { status: 201 });
  }),

  http.delete('/api/v1/api-keys/:id', () => {
    return HttpResponse.json({ success: true, key_id: 'key_001', revoked_at: new Date().toISOString() });
  }),

  http.get('/api/v1/jira/config', () => {
    return HttpResponse.json({
      id: 'jira_001', jira_url: 'https://company.atlassian.net',
      project_key: 'SEC', username: 'security-bot@company.com',
      api_token_preview: 'ATATT...xyz', is_active: true,
      webhook_url: 'https://osv.company.com/api/v1/jira/webhook',
      created_at: '2026-03-01T00:00:00Z', last_sync_at: '2026-06-16T10:00:00Z',
    });
  }),

  http.post('/api/v1/jira/config/test', () => {
    return HttpResponse.json({
      success: true, jira_version: '9.4.0', project_found: true, response_time_ms: 456,
    });
  }),
];
```

---

## 5. RBAC Guard cho Admin Routes

```typescript
// Trong router.tsx — guard admin routes
const AdminRoute = ({ children, requiredPermission }) => {
  const { canManageUsers, canConfigureSystem } = usePermissions();

  const hasAccess = requiredPermission === 'user:manage'
    ? canManageUsers
    : requiredPermission === 'system:configure'
    ? canConfigureSystem
    : canManageUsers || canConfigureSystem;

  if (!hasAccess) return <Navigate to="/dashboard" replace />;
  return children;
};

// Usage trong router:
{ path: 'admin/users', element: <AdminRoute requiredPermission="user:manage"><UserManagement /></AdminRoute> },
{ path: 'admin/health', element: <AdminRoute requiredPermission="system:configure"><SystemHealth /></AdminRoute> },
```

---

## 6. Profile API

```typescript
// features/profile/api/profileApi.ts
export const profileApi = {
  get: async () => {
    const { data } = await apiClient.get('/api/v1/profile');
    return data;
  },
  patch: async (payload: { name?: string; avatar_url?: string }) => {
    const { data } = await apiClient.patch('/api/v1/profile', payload);
    return data;
  },
  changePassword: async (payload: { current_password: string; new_password: string }) => {
    const { data } = await apiClient.post('/api/v1/profile/change-password', payload);
    return data;
  },
};
```

---

## 7. Acceptance Criteria (Frontend)

### User Management
- [ ] User list từ `GET /api/v1/admin/users` — không hardcode
- [ ] Invite dialog → POST → email sent toast
- [ ] Role change → PATCH → table update
- [ ] Unlock user → POST unlock → `is_locked: false`
- [ ] Admin route guard redirect non-admin về `/dashboard`

### RBAC
- [ ] Permission matrix từ `GET /api/v1/admin/roles` — 8 permissions × 4 roles
- [ ] Checkmarks hiển thị đúng theo `roles` object

### System Health
- [ ] Health grid từ `GET /api/v1/admin/health` — không hardcode
- [ ] Service status màu đúng: green/yellow/red
- [ ] `overall_status: degraded` → banner vàng
- [ ] Auto-refresh mỗi 60s

### API Keys
- [ ] List từ `GET /api/v1/api-keys` — KHÔNG hiển thị `plaintext_key`
- [ ] Create → show `plaintext_key` trong dialog một lần duy nhất
- [ ] Sau khi đóng dialog → key không thể xem lại
- [ ] Revoke → key xóa khỏi list ngay

### JIRA
- [ ] Config load từ `GET /api/v1/jira/config`
- [ ] Test connection → hiển thị result
- [ ] Save → encrypt trên server (UI không xử lý)
