# Solution 04 — SystemSettings.tsx

## Vấn đề
- AI providers hardcode: `const aiProviders = [...]`
- Form values hardcode: `Platform Name = "OSV Platform"`, `SMTP = "smtp.company.com"`, ...
- Security policy hardcode: `Min Length = "12"`, `Session Timeout = "60"`

## API Endpoints
```
GET  /api/v1/admin/settings       → Load toàn bộ settings
PUT  /api/v1/admin/settings       → Save settings
GET  /api/v1/admin/health         → AI provider status (dùng health endpoint)
```

## TypeScript Types

```typescript
// features/admin/types.ts — thêm vào
export interface SystemSettings {
  general: {
    platformName: string;
    organization: string;
    supportEmail: string;
    timezone: string;
    logoUrl?: string;
  };
  smtp: {
    host: string;
    port: number;
    username?: string;
    useTls: boolean;
    fromEmail: string;
  };
  security: {
    passwordMinLength: number;
    passwordMaxAgeDays: number;
    sessionTimeoutMinutes: number;
    maxConcurrentSessions: number;
    mfaRequired: boolean;
    allowOAuth: boolean;
  };
  ai: {
    providers: AIProviderConfig[];
    activeProviderId: string;
  };
}

export interface AIProviderConfig {
  id: string;
  name: string;
  model: string;
  status: 'active' | 'standby' | 'inactive';
  latencyMs?: number;
  requestsPerDay?: number;
  costPerDay?: number;
}
```

## Hook mới: `useSystemSettings`

```typescript
// features/admin/hooks/useSystemSettings.ts
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { apiClient } from '@/shared/api/client';
import { ENDPOINTS } from '@/shared/api/endpoints';
import type { SystemSettings } from '../types';

const settingsKeys = {
  all: ['admin', 'settings'] as const,
};

export function useSystemSettings() {
  return useQuery<SystemSettings>({
    queryKey: settingsKeys.all,
    queryFn: async () => {
      const { data } = await apiClient.get<SystemSettings>(ENDPOINTS.admin.settings);
      return data;
    },
    staleTime: 5 * 60_000,
  });
}

export function useUpdateSettings() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (settings: Partial<SystemSettings>) =>
      apiClient.put(ENDPOINTS.admin.settings, settings),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: settingsKeys.all });
    },
  });
}
```

## Component sau khi fix

```typescript
// features/admin/components/SystemSettings.tsx
import { useState, useEffect } from 'react';
import { Settings, Save, Loader } from 'lucide-react';
import { QueryBoundary } from '@/shared/components/feedback/QueryBoundary';
import { useSystemSettings, useUpdateSettings } from '../hooks/useSystemSettings';
import type { SystemSettings } from '../types';

function SystemSettingsSkeleton() {
  return (
    <div className="flex-1 overflow-y-auto px-6 py-5 animate-pulse" style={{ background: '#0B1020' }}>
      <div className="rounded-2xl h-96" style={{ background: '#151B2F' }} />
    </div>
  );
}

export function SystemSettings() {
  const settingsQuery = useSystemSettings();
  const updateSettings = useUpdateSettings();
  const [activeTab, setActiveTab] = useState('General');
  // Local form state — initialized từ server data
  const [form, setForm] = useState<Partial<SystemSettings> | null>(null);

  // Sync form khi data load
  useEffect(() => {
    if (settingsQuery.data && !form) {
      setForm(settingsQuery.data);
    }
  }, [settingsQuery.data]);

  const handleSave = async () => {
    if (form) await updateSettings.mutateAsync(form);
  };

  const tabs = ['General', 'Security', 'AI Providers', 'SMTP', 'Notifications'];

  return (
    <div className="flex-1 overflow-y-auto px-6 py-5" style={{ background: '#0B1020' }}>
      {/* Header */}
      <div className="flex items-center justify-between mb-5">
        <div className="flex items-center gap-3">
          <div className="w-9 h-9 rounded-xl flex items-center justify-center" style={{ background: 'rgba(79,140,255,0.1)' }}>
            <Settings size={18} color="#4F8CFF" />
          </div>
          <div>
            <h2 style={{ color: '#E5E7EB', fontSize: 18, fontWeight: 700 }}>System Settings</h2>
            <p style={{ color: '#6B7280', fontSize: 12 }}>Platform configuration</p>
          </div>
        </div>
        <button
          onClick={handleSave}
          disabled={updateSettings.isPending || !form}
          className="flex items-center gap-2 px-4 py-2 rounded-xl"
          style={{ background: 'linear-gradient(135deg, #10B981, #059669)', color: 'white', border: 'none', fontSize: 13, cursor: 'pointer' }}>
          {updateSettings.isPending ? <Loader size={14} className="animate-spin" /> : <Save size={14} />}
          {updateSettings.isPending ? 'Saving...' : 'Save Changes'}
        </button>
      </div>

      {/* Tabs */}
      <div className="flex gap-1 mb-5 p-1 rounded-xl" style={{ background: '#0F1629' }}>
        {tabs.map(tab => (
          <button key={tab} onClick={() => setActiveTab(tab)}
            className="px-4 py-2 rounded-lg flex-1"
            style={{
              background: activeTab === tab ? '#151B2F' : 'transparent',
              color: activeTab === tab ? '#E5E7EB' : '#6B7280',
              fontSize: 13, border: 'none', cursor: 'pointer',
            }}>
            {tab}
          </button>
        ))}
      </div>

      <QueryBoundary query={settingsQuery} skeleton={<SystemSettingsSkeleton />}>
        {(data) => {
          if (!form) return <SystemSettingsSkeleton />;

          return (
            <div className="rounded-2xl p-5" style={{ background: '#151B2F', border: '1px solid rgba(255,255,255,0.07)' }}>

              {/* General Tab — data từ server */}
              {activeTab === 'General' && (
                <div className="grid grid-cols-2 gap-4">
                  {[
                    { label: 'Platform Name', field: 'platformName' as const, section: 'general' as const },
                    { label: 'Organization', field: 'organization' as const, section: 'general' as const },
                    { label: 'Support Email', field: 'supportEmail' as const, section: 'general' as const },
                    { label: 'Timezone', field: 'timezone' as const, section: 'general' as const },
                  ].map(({ label, field, section }) => (
                    <div key={field}>
                      <label style={{ color: '#9CA3AF', fontSize: 13, display: 'block', marginBottom: 6 }}>{label}</label>
                      <input
                        value={(form[section] as any)?.[field] ?? ''}
                        onChange={e => setForm(prev => ({
                          ...prev,
                          [section]: { ...(prev?.[section] as any), [field]: e.target.value }
                        }))}
                        className="w-full rounded-xl px-3 py-2.5 outline-none"
                        style={{ background: '#0F1629', border: '1px solid rgba(255,255,255,0.08)', color: '#E5E7EB', fontSize: 13 }} />
                    </div>
                  ))}
                </div>
              )}

              {/* Security Tab — data từ server */}
              {activeTab === 'Security' && (
                <div className="grid grid-cols-2 gap-4">
                  {[
                    { label: 'Min Password Length', field: 'passwordMinLength', type: 'number' },
                    { label: 'Password Max Age (days)', field: 'passwordMaxAgeDays', type: 'number' },
                    { label: 'Session Timeout (min)', field: 'sessionTimeoutMinutes', type: 'number' },
                    { label: 'Max Concurrent Sessions', field: 'maxConcurrentSessions', type: 'number' },
                  ].map(({ label, field, type }) => (
                    <div key={field}>
                      <label style={{ color: '#9CA3AF', fontSize: 13, display: 'block', marginBottom: 6 }}>{label}</label>
                      <input
                        type={type}
                        value={(form.security as any)?.[field] ?? ''}
                        onChange={e => setForm(prev => ({
                          ...prev,
                          security: { ...(prev?.security as any), [field]: Number(e.target.value) }
                        }))}
                        className="w-full rounded-xl px-3 py-2.5 outline-none"
                        style={{ background: '#0F1629', border: '1px solid rgba(255,255,255,0.08)', color: '#E5E7EB', fontSize: 13 }} />
                    </div>
                  ))}
                  <div className="col-span-2 flex items-center gap-4">
                    <label className="flex items-center gap-2 cursor-pointer">
                      <input type="checkbox" checked={form.security?.mfaRequired ?? false}
                        onChange={e => setForm(prev => ({
                          ...prev, security: { ...(prev?.security as any), mfaRequired: e.target.checked }
                        }))}
                        style={{ accentColor: '#4F8CFF' }} />
                      <span style={{ color: '#E5E7EB', fontSize: 13 }}>Require MFA for all users</span>
                    </label>
                  </div>
                </div>
              )}

              {/* AI Providers Tab — data từ server */}
              {activeTab === 'AI Providers' && (
                <div className="flex flex-col gap-3">
                  {data.ai.providers.map(provider => (
                    <div key={provider.id} className="rounded-2xl p-4"
                      style={{ background: '#0F1629', border: `1px solid ${provider.status === 'active' ? 'rgba(16,185,129,0.3)' : 'rgba(255,255,255,0.06)'}` }}>
                      <div className="flex items-center justify-between">
                        <div className="flex items-center gap-3">
                          <div className="w-2 h-2 rounded-full" style={{ background: provider.status === 'active' ? '#10B981' : provider.status === 'standby' ? '#F59E0B' : '#374151' }} />
                          <div>
                            <div style={{ color: '#E5E7EB', fontSize: 13, fontWeight: 500 }}>{provider.name}</div>
                            <div style={{ color: '#6B7280', fontSize: 11, fontFamily: 'monospace' }}>{provider.model}</div>
                          </div>
                        </div>
                        <div className="flex items-center gap-4">
                          {provider.latencyMs && <span style={{ color: '#9CA3AF', fontSize: 11 }}>{provider.latencyMs}ms</span>}
                          {provider.requestsPerDay && <span style={{ color: '#9CA3AF', fontSize: 11 }}>{provider.requestsPerDay.toLocaleString()} req/day</span>}
                          {provider.costPerDay && <span style={{ color: '#10B981', fontSize: 11 }}>${provider.costPerDay.toFixed(2)}/day</span>}
                          <span className="px-2 py-0.5 rounded" style={{
                            background: provider.status === 'active' ? 'rgba(16,185,129,0.1)' : 'rgba(107,114,128,0.1)',
                            color: provider.status === 'active' ? '#10B981' : '#9CA3AF', fontSize: 11
                          }}>
                            {provider.status}
                          </span>
                        </div>
                      </div>
                    </div>
                  ))}
                </div>
              )}

            </div>
          );
        }}
      </QueryBoundary>
    </div>
  );
}
```

## MSW Handler

```typescript
// src/mocks/handlers/admin.handlers.ts — thêm vào
let settingsStore: SystemSettings = {
  general: {
    platformName: 'OSV Platform',
    organization: 'Company Security',
    supportEmail: 'security@company.com',
    timezone: 'Asia/Ho_Chi_Minh',
  },
  smtp: { host: 'smtp.company.com', port: 587, useTls: true, fromEmail: 'no-reply@company.com' },
  security: {
    passwordMinLength: 12,
    passwordMaxAgeDays: 90,
    sessionTimeoutMinutes: 60,
    maxConcurrentSessions: 3,
    mfaRequired: false,
    allowOAuth: true,
  },
  ai: {
    activeProviderId: 'openai',
    providers: [
      { id: 'openai', name: 'OpenAI', model: 'gpt-4o', status: 'active', latencyMs: 203, requestsPerDay: 4821, costPerDay: 12.40 },
      { id: 'azure', name: 'Azure OpenAI', model: 'gpt-4-turbo', status: 'standby', latencyMs: 245 },
      { id: 'ollama', name: 'Ollama (Local)', model: 'llama3:8b', status: 'inactive' },
    ],
  },
};

http.get('/api/v1/admin/settings', () => HttpResponse.json(settingsStore)),
http.put('/api/v1/admin/settings', async ({ request }) => {
  const body = await request.json() as Partial<SystemSettings>;
  settingsStore = { ...settingsStore, ...body };
  return HttpResponse.json(settingsStore);
}),
```
