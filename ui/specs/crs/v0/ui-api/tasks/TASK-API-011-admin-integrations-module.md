# TASK-API-011 — Admin + Integrations Module: APIs + Hooks + MSW

| Field | Value |
|-------|-------|
| **Task ID** | TASK-API-011 |
| **Module** | `ui/src/features/admin/`, `ui/src/features/integrations/`, `ui/src/features/profile/` |
| **Solution Ref** | [SOL-UI-010](../solutions/SOL-UI-010-admin-integrations-api.md) |
| **Priority** | 🔴 P0 |
| **Depends On** | TASK-API-003 |
| **Estimated** | 3h |

---

## Context

Admin module bao gồm 5 screens yêu cầu `user:manage` hoặc `system:configure`. Integrations gồm API Keys ("show once" pattern) và JIRA. Profile là /profile của current user.

**"Show Once" pattern:** Khi tạo API Key, `plaintext_key` được trả về trong response — cần show trong modal, sau khi đóng modal không thể xem lại.

---

## Goal

Tạo Admin + Integrations + Profile modules với APIs, hooks, MSW fixtures.

---

## Target Files

| Action | File Path |
|--------|-----------|
| CREATE | `ui/src/features/admin/types.ts` |
| CREATE | `ui/src/features/admin/api/adminApi.ts` |
| CREATE | `ui/src/features/admin/hooks/useAdminUsers.ts` |
| CREATE | `ui/src/features/admin/hooks/useSystemHealth.ts` |
| CREATE | `ui/src/features/admin/hooks/useSystemSettings.ts` |
| CREATE | `ui/src/features/admin/hooks/useAuditLog.ts` |
| CREATE | `ui/src/features/integrations/types.ts` |
| CREATE | `ui/src/features/integrations/api/integrationApi.ts` |
| CREATE | `ui/src/features/integrations/hooks/useAPIKeys.ts` |
| CREATE | `ui/src/features/integrations/hooks/useJIRA.ts` |
| CREATE | `ui/src/features/profile/api/profileApi.ts` |
| CREATE | `ui/src/features/profile/hooks/useProfile.ts` |
| CREATE | `ui/src/mocks/handlers/admin.handlers.ts` |
| CREATE | `ui/src/mocks/handlers/integration.handlers.ts` |

---

## Implementation

### File 1: `ui/src/features/admin/types.ts`

```typescript
export interface AdminUser {
  id: string;
  email: string;
  name: string;
  role: 'admin' | 'user' | 'readonly' | 'agent';
  is_active: boolean;
  mfa_enabled: boolean;
  last_login_at: string | null;
  created_at: string;
  login_attempts: number;
  is_locked: boolean;
}

export interface UserListResponse {
  users: AdminUser[];
  total: number;
  page: number;
  page_size: number;
}

export interface PermissionMatrix {
  permission: string;
  description: string;
  roles: Record<string, boolean>;
}

export interface RBACResponse {
  roles: string[];
  permissions: PermissionMatrix[];
}

export interface ServiceHealth {
  name: string;
  status: 'healthy' | 'degraded' | 'down';
  response_time_ms: number;
  last_checked_at: string;
  version: string;
  details: string | null;
}

export interface InfraHealth {
  status: 'healthy' | 'degraded' | 'down';
  [key: string]: any;
}

export interface SystemHealthResponse {
  services: ServiceHealth[];
  nats: InfraHealth & { pending_messages: number; consumer_lag: number };
  postgres: InfraHealth & { active_connections: number; max_connections: number };
  redis: InfraHealth & { used_memory_mb: number; max_memory_mb: number };
  opensearch: InfraHealth & { indexed_docs: number };
  overall_status: 'healthy' | 'degraded' | 'down';
  checked_at: string;
}

export interface SystemSettings {
  general: { platform_name: string; platform_url: string };
  security: {
    session_timeout_minutes: number;
    max_login_attempts: number;
    lockout_duration_minutes: number;
    mfa_required: boolean;
  };
  ai: {
    ollama_url: string;
    openai_api_key_preview: string | null;
    default_provider: 'ollama' | 'openai';
    triage_enabled: boolean;
  };
  notifications: {
    smtp_host: string | null;
    smtp_port: number;
    smtp_from: string | null;
    slack_webhook_url: string | null;
    teams_webhook_url: string | null;
  };
}

export interface AuditEvent {
  id: string;
  user_id: string;
  user_name: string;
  action: string;
  entity_type: string | null;
  entity_id: string | null;
  ip_address: string;
  user_agent: string;
  result: 'success' | 'failure';
  metadata: Record<string, any>;
  timestamp: string;
}
```

### File 2: `ui/src/features/admin/api/adminApi.ts`

```typescript
import { apiClient } from '@/shared/api/client';
import { ENDPOINTS } from '@/shared/api/endpoints';
import type { AdminUser, UserListResponse, RBACResponse, SystemHealthResponse, SystemSettings, AuditEvent } from '../types';

export const adminApi = {
  listUsers: async (params: { q?: string; role?: string; is_active?: boolean; page?: number } = {}): Promise<UserListResponse> => {
    const { data } = await apiClient.get<UserListResponse>(ENDPOINTS.admin.users, { params });
    return data;
  },

  inviteUser: async (payload: { email: string; name: string; role: string }): Promise<AdminUser> => {
    const { data } = await apiClient.post<AdminUser>(ENDPOINTS.admin.userInvite, payload);
    return data;
  },

  updateUser: async (id: string, payload: { role?: string; is_active?: boolean }): Promise<AdminUser> => {
    const { data } = await apiClient.patch<AdminUser>(ENDPOINTS.admin.userDetail(id), payload);
    return data;
  },

  unlockUser: async (id: string): Promise<{ success: boolean; is_locked: boolean }> => {
    const { data } = await apiClient.post(ENDPOINTS.admin.userUnlock(id));
    return data as { success: boolean; is_locked: boolean };
  },

  resetPassword: async (id: string): Promise<{ success: boolean }> => {
    const { data } = await apiClient.post(ENDPOINTS.admin.userReset(id));
    return data as { success: boolean };
  },

  getRoles: async (): Promise<RBACResponse> => {
    const { data } = await apiClient.get<RBACResponse>(ENDPOINTS.admin.roles);
    return data;
  },

  getHealth: async (): Promise<SystemHealthResponse> => {
    const { data } = await apiClient.get<SystemHealthResponse>(ENDPOINTS.admin.health);
    return data;
  },

  getSettings: async (): Promise<SystemSettings> => {
    const { data } = await apiClient.get<SystemSettings>(ENDPOINTS.admin.settings);
    return data;
  },

  updateSettings: async (payload: Partial<SystemSettings>): Promise<SystemSettings> => {
    const { data } = await apiClient.patch<SystemSettings>(ENDPOINTS.admin.settings, payload);
    return data;
  },

  getAuditLog: async (params: {
    user_id?: string; action?: string; date_from?: string; page?: number; page_size?: number;
  } = {}): Promise<{ events: AuditEvent[]; total: number; page: number; page_size: number }> => {
    const { data } = await apiClient.get(ENDPOINTS.audit.log, { params });
    return data as any;
  },
};
```

### File 3: Admin Hooks

```typescript
// ui/src/features/admin/hooks/useAdminUsers.ts
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { adminApi } from '../api/adminApi';
import toast from 'react-hot-toast';

export const adminKeys = {
  users:    (params: object) => ['admin', 'users', params] as const,
  roles:    () => ['admin', 'roles'] as const,
  health:   () => ['admin', 'health'] as const,
  settings: () => ['admin', 'settings'] as const,
  audit:    (params: object) => ['admin', 'audit', params] as const,
};

export function useAdminUsers(params: { q?: string; role?: string; page?: number } = {}) {
  return useQuery({ queryKey: adminKeys.users(params), queryFn: () => adminApi.listUsers(params), staleTime: 30_000 });
}

export function useInviteUser() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: adminApi.inviteUser,
    onSuccess: () => { qc.invalidateQueries({ queryKey: ['admin', 'users'] }); toast.success('Invitation sent'); },
  });
}

export function useUpdateUser() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({ id, payload }: { id: string; payload: { role?: string; is_active?: boolean } }) => adminApi.updateUser(id, payload),
    onSuccess: () => { qc.invalidateQueries({ queryKey: ['admin', 'users'] }); toast.success('User updated'); },
  });
}

export function useUnlockUser() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: adminApi.unlockUser,
    onSuccess: () => { qc.invalidateQueries({ queryKey: ['admin', 'users'] }); toast.success('User unlocked'); },
  });
}

// ui/src/features/admin/hooks/useSystemHealth.ts
export function useSystemHealth() {
  const { useQuery } = require('@tanstack/react-query');
  return useQuery({
    queryKey: adminKeys.health(),
    queryFn:  adminApi.getHealth,
    staleTime:       30_000,
    refetchInterval: 60_000,  // Auto-refresh mỗi 60s
  });
}

// ui/src/features/admin/hooks/useSystemSettings.ts
export function useSystemSettings() {
  const { useQuery, useMutation, useQueryClient } = require('@tanstack/react-query');
  const query = useQuery({ queryKey: adminKeys.settings(), queryFn: adminApi.getSettings, staleTime: 5 * 60_000 });
  const qc = useQueryClient();
  const update = useMutation({
    mutationFn: adminApi.updateSettings,
    onSuccess: () => { qc.invalidateQueries({ queryKey: adminKeys.settings() }); toast.success('Settings saved'); },
  });
  return { query, update };
}

// ui/src/features/admin/hooks/useAuditLog.ts
export function useAuditLog(params: { userId?: string; action?: string; page?: number } = {}) {
  const { useQuery } = require('@tanstack/react-query');
  const queryParams = { user_id: params.userId, action: params.action, page: params.page ?? 1 };
  return useQuery({ queryKey: adminKeys.audit(queryParams), queryFn: () => adminApi.getAuditLog(queryParams), staleTime: 30_000 });
}
```

> [!NOTE]
> Tách useSystemHealth, useSystemSettings, useAuditLog thành các file riêng biệt và sử dụng proper imports thay vì require().

### File 4: `ui/src/features/integrations/types.ts`

```typescript
export interface APIKey {
  id: string;
  name: string;
  prefix: string;   // e.g., "ovs_live_a1b2c3" — KHÔNG phải full key
  permissions: string[];
  created_at: string;
  last_used_at: string | null;
  expires_at: string | null;
  is_active: boolean;
}

export interface APIKeyCreatedResponse extends APIKey {
  plaintext_key: string;  // Chỉ trả về lúc tạo — "show once"
}

export interface JIRAConfig {
  id: string;
  jira_url: string;
  project_key: string;
  username: string;
  api_token_preview: string;  // Masked
  is_active: boolean;
  webhook_url: string;
  created_at: string;
  last_sync_at: string | null;
}
```

### File 5: `ui/src/features/integrations/api/integrationApi.ts`

```typescript
import { apiClient } from '@/shared/api/client';
import { ENDPOINTS } from '@/shared/api/endpoints';
import type { APIKey, APIKeyCreatedResponse, JIRAConfig } from '../types';

export const integrationApi = {
  listAPIKeys: async (): Promise<{ api_keys: APIKey[]; total: number }> => {
    const { data } = await apiClient.get(ENDPOINTS.apiKeys.list);
    return data as { api_keys: APIKey[]; total: number };
  },

  createAPIKey: async (payload: {
    name: string; permissions: string[]; expires_at?: string | null;
  }): Promise<APIKeyCreatedResponse> => {
    const { data } = await apiClient.post<APIKeyCreatedResponse>(ENDPOINTS.apiKeys.create, payload);
    return data;
  },

  revokeAPIKey: async (id: string): Promise<void> => {
    await apiClient.delete(ENDPOINTS.apiKeys.revoke(id));
  },

  getJIRAConfig: async (): Promise<JIRAConfig | null> => {
    try {
      const { data } = await apiClient.get<JIRAConfig>(ENDPOINTS.jira.config);
      return data;
    } catch (e: any) {
      if (e.response?.status === 404) return null;
      throw e;
    }
  },

  saveJIRAConfig: async (payload: {
    jira_url: string; project_key: string; username: string; api_token: string;
  }): Promise<JIRAConfig> => {
    const { data } = await apiClient.post<JIRAConfig>(ENDPOINTS.jira.config, payload);
    return data;
  },

  testJIRAConfig: async (): Promise<{
    success: boolean; jira_version: string; project_found: boolean; response_time_ms: number;
  }> => {
    const { data } = await apiClient.post(ENDPOINTS.jira.test);
    return data as any;
  },
};
```

### File 6: Integration Hooks

```typescript
// ui/src/features/integrations/hooks/useAPIKeys.ts
import { useState } from 'react';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { integrationApi } from '../api/integrationApi';
import toast from 'react-hot-toast';

export const apiKeyKeys = {
  list: () => ['api-keys'] as const,
};

export function useAPIKeys() {
  return useQuery({
    queryKey: apiKeyKeys.list(),
    queryFn:  () => integrationApi.listAPIKeys(),
    staleTime: 60_000,
  });
}

// "Show Once" pattern: expose plaintext_key từ mutation response
export function useCreateAPIKey() {
  const [newKeySecret, setNewKeySecret] = useState<string | null>(null);
  const qc = useQueryClient();

  const mutation = useMutation({
    mutationFn: integrationApi.createAPIKey,
    onSuccess: (data) => {
      // Lưu plaintext_key trong local state — không persist, không cache
      setNewKeySecret(data.plaintext_key);
      qc.invalidateQueries({ queryKey: apiKeyKeys.list() });
    },
  });

  const clearSecret = () => setNewKeySecret(null);

  return { mutation, newKeySecret, clearSecret };
}

export function useRevokeAPIKey() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: integrationApi.revokeAPIKey,
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: apiKeyKeys.list() });
      toast.success('API key revoked');
    },
  });
}

// ui/src/features/integrations/hooks/useJIRA.ts
export function useJIRAConfig() {
  const { useQuery, useMutation, useQueryClient } = require('@tanstack/react-query');
  const qc = useQueryClient();

  const configQuery = useQuery({
    queryKey: ['jira', 'config'],
    queryFn:  integrationApi.getJIRAConfig,
    staleTime: 5 * 60_000,
  });

  const saveConfig = useMutation({
    mutationFn: integrationApi.saveJIRAConfig,
    onSuccess: () => { qc.invalidateQueries({ queryKey: ['jira'] }); toast.success('JIRA config saved'); },
  });

  const testConfig = useMutation({
    mutationFn: integrationApi.testJIRAConfig,
    onSuccess: (data) => {
      data.success
        ? toast.success(`JIRA connected: v${data.jira_version}, project found: ${data.project_found}`)
        : toast.error('JIRA connection failed');
    },
  });

  return { configQuery, saveConfig, testConfig };
}
```

> [!NOTE]
> Tách useJIRAConfig thành file riêng với proper imports.

### File 7: `ui/src/features/profile/api/profileApi.ts`

```typescript
import { apiClient } from '@/shared/api/client';
import { ENDPOINTS } from '@/shared/api/endpoints';
import type { User } from '@/features/auth/types';

export const profileApi = {
  get: async (): Promise<User> => {
    const { data } = await apiClient.get<User>(ENDPOINTS.profile.get);
    return data;
  },

  patch: async (payload: { name?: string; avatar_url?: string }): Promise<User> => {
    const { data } = await apiClient.patch<User>(ENDPOINTS.profile.patch, payload);
    return data;
  },

  changePassword: async (payload: {
    current_password: string; new_password: string;
  }): Promise<{ success: boolean }> => {
    const { data } = await apiClient.post(ENDPOINTS.profile.changePassword, payload);
    return data as { success: boolean };
  },
};
```

### File 8: `ui/src/mocks/handlers/admin.handlers.ts`

```typescript
import { http, HttpResponse } from 'msw';
import { ENDPOINTS } from '@/shared/api/endpoints';
import { userFixtures } from '../fixtures/auth.fixture';

const users = Object.values(userFixtures);

export const adminHandlers = [
  http.get(ENDPOINTS.admin.users, () => {
    return HttpResponse.json({ users, total: users.length, page: 1, page_size: 20 });
  }),
  http.post(ENDPOINTS.admin.userInvite, async ({ request }) => {
    const body = await request.json() as any;
    return HttpResponse.json({ id: 'usr_new_' + Date.now(), email: body.email, name: body.name, role: body.role, status: 'invited', invite_sent_at: new Date().toISOString() }, { status: 201 });
  }),
  http.patch('/api/v1/admin/users/:id', async ({ params, request }) => {
    const body = await request.json() as any;
    const user = users.find(u => u.id === params.id) ?? users[0];
    return HttpResponse.json({ ...user, ...body });
  }),
  http.post('/api/v1/admin/users/:id/unlock', ({ params }) => {
    return HttpResponse.json({ success: true, user_id: params.id, is_locked: false });
  }),
  http.post('/api/v1/admin/users/:id/reset-password', () => {
    return HttpResponse.json({ success: true });
  }),
  http.get(ENDPOINTS.admin.roles, () => {
    return HttpResponse.json({
      roles: ['admin', 'user', 'readonly', 'agent'],
      permissions: [
        { permission: 'scan:create', description: 'Create and start scans', roles: { admin: true, user: true, readonly: false, agent: false } },
        { permission: 'finding:write', description: 'Update finding status', roles: { admin: true, user: true, readonly: false, agent: false } },
        { permission: 'user:manage', description: 'Manage users and roles', roles: { admin: true, user: false, readonly: false, agent: false } },
        { permission: 'system:configure', description: 'Configure system settings', roles: { admin: true, user: false, readonly: false, agent: false } },
        { permission: 'report:download', description: 'Download security reports', roles: { admin: true, user: true, readonly: true, agent: false } },
        { permission: 'agent:report', description: 'Submit scan reports via agent', roles: { admin: false, user: false, readonly: false, agent: true } },
      ],
    });
  }),
  http.get(ENDPOINTS.admin.health, () => {
    return HttpResponse.json({
      services: [
        { name: 'identity-service', status: 'healthy', response_time_ms: 12, last_checked_at: new Date().toISOString(), version: '2.2.0', details: null },
        { name: 'data-service', status: 'healthy', response_time_ms: 18, last_checked_at: new Date().toISOString(), version: '2.2.0', details: null },
        { name: 'search-service', status: 'degraded', response_time_ms: 850, last_checked_at: new Date().toISOString(), version: '2.2.0', details: 'OpenSearch high latency' },
        { name: 'finding-service', status: 'healthy', response_time_ms: 15, last_checked_at: new Date().toISOString(), version: '2.1.0', details: null },
      ],
      nats: { status: 'healthy', pending_messages: 12, consumer_lag: 0 },
      postgres: { status: 'healthy', active_connections: 45, max_connections: 200 },
      redis: { status: 'healthy', used_memory_mb: 128, max_memory_mb: 512 },
      opensearch: { status: 'degraded', indexed_docs: 312450 },
      overall_status: 'degraded',
      checked_at: new Date().toISOString(),
    });
  }),
  http.get(ENDPOINTS.admin.settings, () => {
    return HttpResponse.json({
      general: { platform_name: 'OSV Platform', platform_url: 'https://osv.company.com' },
      security: { session_timeout_minutes: 480, max_login_attempts: 5, lockout_duration_minutes: 15, mfa_required: false },
      ai: { ollama_url: 'http://ollama:11434', openai_api_key_preview: 'sk-...abc', default_provider: 'ollama', triage_enabled: true },
      notifications: { smtp_host: 'smtp.company.com', smtp_port: 587, smtp_from: 'security@company.com', slack_webhook_url: null, teams_webhook_url: null },
    });
  }),
  http.patch(ENDPOINTS.admin.settings, async ({ request }) => {
    const body = await request.json() as any;
    return HttpResponse.json(body);
  }),
  http.get(ENDPOINTS.audit.log, () => {
    return HttpResponse.json({
      events: [
        { id: 'aud_001', user_id: 'usr_bob123', user_name: 'Bob Smith', action: 'finding.status_changed', entity_type: 'finding', entity_id: 'F-2847', ip_address: '10.0.0.1', user_agent: 'Mozilla/5.0...', result: 'success', metadata: { from: 'active', to: 'mitigated' }, timestamp: '2026-06-16T11:00:00Z' },
      ],
      total: 1, page: 1, page_size: 50,
    });
  }),
  // Profile
  http.get(ENDPOINTS.profile.get, () => {
    return HttpResponse.json(userFixtures.bob);
  }),
  http.patch(ENDPOINTS.profile.patch, async ({ request }) => {
    const body = await request.json() as any;
    return HttpResponse.json({ ...userFixtures.bob, ...body });
  }),
  http.post(ENDPOINTS.profile.changePassword, () => {
    return HttpResponse.json({ success: true });
  }),
];
```

### File 9: `ui/src/mocks/handlers/integration.handlers.ts`

```typescript
import { http, HttpResponse } from 'msw';
import { ENDPOINTS } from '@/shared/api/endpoints';

let apiKeys = [
  { id: 'key_001', name: 'CI/CD Pipeline', prefix: 'ovs_live_a1b2c3', permissions: ['scan:read', 'finding:read'], created_at: '2026-03-01T00:00:00Z', last_used_at: '2026-06-16T08:00:00Z', expires_at: null, is_active: true },
];

export const integrationHandlers = [
  http.get(ENDPOINTS.apiKeys.list, () => {
    return HttpResponse.json({ api_keys: apiKeys, total: apiKeys.length });
  }),
  http.post(ENDPOINTS.apiKeys.create, async ({ request }) => {
    const body = await request.json() as any;
    const newKey = {
      id: 'key_' + Date.now(), name: body.name,
      prefix: 'ovs_live_' + Math.random().toString(36).slice(2, 8),
      plaintext_key: 'ovs_live_' + Math.random().toString(36).slice(2) + '_' + Math.random().toString(36).slice(2),
      permissions: body.permissions,
      created_at: new Date().toISOString(), expires_at: body.expires_at ?? null, is_active: true, last_used_at: null,
    };
    apiKeys = [newKey, ...apiKeys];
    return HttpResponse.json(newKey, { status: 201 });
  }),
  http.delete('/api/v1/api-keys/:id', ({ params }) => {
    apiKeys = apiKeys.filter(k => k.id !== params.id);
    return HttpResponse.json({ success: true, key_id: params.id, revoked_at: new Date().toISOString() });
  }),
  http.get(ENDPOINTS.jira.config, () => {
    return HttpResponse.json({ id: 'jira_001', jira_url: 'https://company.atlassian.net', project_key: 'SEC', username: 'security-bot@company.com', api_token_preview: 'ATATT...xyz', is_active: true, webhook_url: 'https://osv.company.com/api/v1/jira/webhook', created_at: '2026-03-01T00:00:00Z', last_sync_at: '2026-06-16T10:00:00Z' });
  }),
  http.post(ENDPOINTS.jira.config, async ({ request }) => {
    const body = await request.json() as any;
    return HttpResponse.json({ id: 'jira_new', jira_url: body.jira_url, project_key: body.project_key, username: body.username, api_token_preview: 'ATATT...new', is_active: true, webhook_url: 'https://osv.company.com/api/v1/jira/webhook', created_at: new Date().toISOString(), last_sync_at: null }, { status: 201 });
  }),
  http.post(ENDPOINTS.jira.test, () => {
    return HttpResponse.json({ success: true, jira_version: '9.4.0', project_found: true, response_time_ms: 456 });
  }),
];
```

---

## Verification

```bash
cd ui/
VITE_ENABLE_MSW=true pnpm dev

# Admin:
# 1. /admin/users (admin login) → 3 users
# 2. Invite user → success toast
# 3. /admin/roles → Permission matrix 6×4
# 4. /admin/health → service-search degraded, overall degraded (yellow banner)
# 5. /admin/settings → Edit AI settings → save

# Integrations:
# 6. /integrations/api-keys → 1 key, prefix visible, NO plaintext_key in list
# 7. Create API key → modal with plaintext_key shown ONCE
# 8. Close modal → key không thể xem lại
# 9. /integrations/jira → config loaded, test connection → success

# RBAC Guard:
# 10. Login as readonly user → /admin/users redirects to /dashboard

npx tsc --noEmit
# Expected: no errors
```

---

## Checklist

### Admin
- [ ] `features/admin/types.ts` — AdminUser, RBACResponse, SystemHealthResponse, SystemSettings, AuditEvent
- [ ] `features/admin/api/adminApi.ts` — 10 methods dùng `ENDPOINTS.admin.*`
- [ ] `useAdminUsers`, `useInviteUser`, `useUpdateUser`, `useUnlockUser`
- [ ] `useSystemHealth` — `refetchInterval: 60_000`
- [ ] `useSystemSettings` — query + update mutation
- [ ] `useAuditLog` — paginated

### Integrations
- [ ] `features/integrations/types.ts` — APIKey (không có `plaintext_key`), APIKeyCreatedResponse (có)
- [ ] `features/integrations/api/integrationApi.ts` — apiKeys + jira methods
- [ ] `useAPIKeys` — list (KHÔNG expose plaintext_key)
- [ ] `useCreateAPIKey` — local state `newKeySecret`, `clearSecret()`
- [ ] `useRevokeAPIKey`, `useJIRAConfig` (query + save + test)

### Profile
- [ ] `features/profile/api/profileApi.ts` — get, patch, changePassword

### MSW
- [ ] `admin.handlers.ts` — 12 handlers dùng `ENDPOINTS.admin.*` và `ENDPOINTS.profile.*`
- [ ] `integration.handlers.ts` — 6 handlers; createAPIKey trả `plaintext_key` (chỉ trong response)
- [ ] Handlers share `userFixtures` từ `auth.fixture.ts` (không duplicate user data)
- [ ] `npx tsc --noEmit` không lỗi
