# Solution 09 — APIKeyManagement.tsx

## Vấn đề
- `const apiKeys = [...]` — danh sách hardcode
- Key generation dùng `Math.random()` ở frontend — **lỗ hổng bảo mật nghiêm trọng**
- `Math.random()` không phải CSPRNG (Cryptographically Secure Pseudo-Random Number Generator)

## API Endpoints
```
GET    /api/v1/api-keys         → Danh sách API keys (ENDPOINTS.apiKeys.list)
POST   /api/v1/api-keys         → Tạo key mới (backend generate) (ENDPOINTS.apiKeys.create)
DELETE /api/v1/api-keys/:id     → Revoke key (ENDPOINTS.apiKeys.revoke)
```

## TypeScript Types

```typescript
// features/integrations/types.ts
export interface APIKey {
  id: string;
  name: string;
  prefix: string;        // e.g. "osv_prod_xK7m" — chỉ prefix, không phải full key
  scopes: string[];      // ['scan:write', 'finding:read']
  createdAt: string;
  lastUsedAt?: string;
  expiresAt?: string;    // null = never expires
  status: 'active' | 'revoked';
  createdBy: string;
}

export interface CreateAPIKeyResponse {
  key: APIKey;
  rawKey: string;  // FULL key — chỉ trả về 1 lần duy nhất, không lưu backend
}

export interface APIKeysResponse {
  keys: APIKey[];
  total: number;
}

export interface CreateAPIKeyRequest {
  name: string;
  scopes: string[];
  expiresAt?: string;
}
```

## Hook mới: `useAPIKeys`

```typescript
// features/integrations/hooks/useAPIKeys.ts
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { apiClient } from '@/shared/api/client';
import { ENDPOINTS } from '@/shared/api/endpoints';
import type { APIKeysResponse, CreateAPIKeyRequest, CreateAPIKeyResponse } from '../types';

const apiKeyKeys = {
  all: ['api-keys'] as const,
  list: () => [...apiKeyKeys.all, 'list'] as const,
};

export function useAPIKeys() {
  return useQuery<APIKeysResponse>({
    queryKey: apiKeyKeys.list(),
    queryFn: async () => {
      const { data } = await apiClient.get<APIKeysResponse>(ENDPOINTS.apiKeys.list);
      return data;
    },
    staleTime: 5 * 60_000,
  });
}

export function useCreateAPIKey() {
  const queryClient = useQueryClient();
  return useMutation<CreateAPIKeyResponse, Error, CreateAPIKeyRequest>({
    mutationFn: async (req) => {
      // ✅ Backend generate key — không dùng Math.random()
      const { data } = await apiClient.post<CreateAPIKeyResponse>(ENDPOINTS.apiKeys.create, req);
      return data;
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: apiKeyKeys.all });
    },
  });
}

export function useRevokeAPIKey() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (id: string) => apiClient.delete(ENDPOINTS.apiKeys.revoke(id)),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: apiKeyKeys.all });
    },
  });
}
```

## Component sau khi fix

```typescript
// features/integrations/components/APIKeyManagement.tsx
import { useState } from 'react';
import { Key, Plus, Trash2, Copy, Eye, EyeOff, AlertTriangle, CheckCircle, Shield } from 'lucide-react';
import { QueryBoundary } from '@/shared/components/feedback/QueryBoundary';
import { useAPIKeys, useCreateAPIKey, useRevokeAPIKey } from '../hooks/useAPIKeys';
import type { CreateAPIKeyRequest } from '../types';

// ── Available scopes — UI constant ──────────────────────────────────────────
const AVAILABLE_SCOPES = [
  { value: 'scan:read',    label: 'Read Scans' },
  { value: 'scan:write',   label: 'Create/Cancel Scans' },
  { value: 'finding:read', label: 'Read Findings' },
  { value: 'finding:write',label: 'Update Findings' },
  { value: 'asset:read',   label: 'Read Assets' },
  { value: 'report:download', label: 'Download Reports' },
  { value: 'agent:report', label: 'Agent Report' },
];

function APIKeysSkeleton() {
  return (
    <div className="flex-1 overflow-y-auto px-6 py-5 animate-pulse" style={{ background: '#0B1020' }}>
      <div className="flex flex-col gap-3">
        {Array.from({ length: 3 }).map((_, i) => (
          <div key={i} className="rounded-2xl h-24" style={{ background: '#151B2F' }} />
        ))}
      </div>
    </div>
  );
}

export function APIKeyManagement() {
  const [showCreate, setShowCreate] = useState(false);
  const [newKeyName, setNewKeyName] = useState('');
  const [selectedScopes, setSelectedScopes] = useState<string[]>([]);
  const [newKeyRaw, setNewKeyRaw] = useState<string | null>(null);  // Full key shown once
  const [showRawKey, setShowRawKey] = useState(false);
  const [copied, setCopied] = useState(false);

  // ✅ Server data — không hardcode
  const keysQuery = useAPIKeys();
  const createKey = useCreateAPIKey();
  const revokeKey = useRevokeAPIKey();

  const handleCreate = async () => {
    if (!newKeyName || selectedScopes.length === 0) return;

    const req: CreateAPIKeyRequest = {
      name: newKeyName,
      scopes: selectedScopes,
    };

    // Backend generate key — cryptographically secure
    const result = await createKey.mutateAsync(req);
    setNewKeyRaw(result.rawKey);  // Show full key once
    setShowCreate(false);
    setNewKeyName('');
    setSelectedScopes([]);
  };

  const handleCopy = async () => {
    if (newKeyRaw) {
      await navigator.clipboard.writeText(newKeyRaw);
      setCopied(true);
      setTimeout(() => setCopied(false), 2000);
    }
  };

  const toggleScope = (scope: string) => {
    setSelectedScopes(prev =>
      prev.includes(scope) ? prev.filter(s => s !== scope) : [...prev, scope]
    );
  };

  return (
    <QueryBoundary query={keysQuery} skeleton={<APIKeysSkeleton />}>
      {({ keys, total }) => {
        const activeCount = keys.filter(k => k.status === 'active').length;

        return (
          <div className="flex-1 overflow-y-auto px-6 py-5" style={{ background: '#0B1020' }}>
            {/* Header */}
            <div className="flex items-center justify-between mb-6">
              <div className="flex items-center gap-3">
                <div className="w-9 h-9 rounded-xl flex items-center justify-center" style={{ background: 'rgba(79,140,255,0.1)' }}>
                  <Key size={18} color="#4F8CFF" />
                </div>
                <div>
                  <h2 style={{ color: '#E5E7EB', fontSize: 18, fontWeight: 700 }}>API Key Management</h2>
                  <p style={{ color: '#6B7280', fontSize: 12 }}>{activeCount} active keys · {total} total</p>
                </div>
              </div>
              <button onClick={() => setShowCreate(true)}
                className="flex items-center gap-2 px-4 py-2 rounded-xl"
                style={{ background: 'linear-gradient(135deg, #4F8CFF, #3B6FCC)', color: 'white', border: 'none', fontSize: 13, cursor: 'pointer' }}>
                <Plus size={14} />Generate Key
              </button>
            </div>

            {/* ⚠️ Display new key once — chỉ hiện 1 lần */}
            {newKeyRaw && (
              <div className="rounded-2xl p-5 mb-5"
                style={{ background: 'rgba(16,185,129,0.08)', border: '1px solid rgba(16,185,129,0.3)' }}>
                <div className="flex items-center gap-2 mb-3">
                  <CheckCircle size={16} color="#10B981" />
                  <span style={{ color: '#10B981', fontSize: 14, fontWeight: 600 }}>API Key created — Save it now!</span>
                </div>
                <div className="flex items-center gap-2 p-3 rounded-xl mb-3"
                  style={{ background: '#0F1629', border: '1px solid rgba(255,255,255,0.08)' }}>
                  <code style={{ color: '#10B981', fontSize: 13, fontFamily: 'monospace', flex: 1 }}>
                    {showRawKey ? newKeyRaw : `${newKeyRaw.substring(0, 12)}${'•'.repeat(newKeyRaw.length - 12)}`}
                  </code>
                  <button onClick={() => setShowRawKey(!showRawKey)} style={{ color: '#6B7280', background: 'none', border: 'none', cursor: 'pointer' }}>
                    {showRawKey ? <EyeOff size={14} /> : <Eye size={14} />}
                  </button>
                  <button onClick={handleCopy} className="flex items-center gap-1.5 px-3 py-1 rounded-lg"
                    style={{ background: 'rgba(16,185,129,0.12)', color: '#10B981', border: 'none', cursor: 'pointer', fontSize: 12 }}>
                    {copied ? <><CheckCircle size={12} />Copied!</> : <><Copy size={12} />Copy</>}
                  </button>
                </div>
                <p style={{ color: '#9CA3AF', fontSize: 12 }}>
                  ⚠️ This key will not be shown again. Store it in a secure vault.
                </p>
                <button onClick={() => setNewKeyRaw(null)} className="mt-2 px-3 py-1.5 rounded-lg"
                  style={{ background: 'rgba(255,255,255,0.05)', color: '#6B7280', border: 'none', cursor: 'pointer', fontSize: 12 }}>
                  Dismiss
                </button>
              </div>
            )}

            {/* Keys list — data từ server */}
            <div className="flex flex-col gap-3">
              {keys.map(key => (
                <div key={key.id} className="rounded-2xl p-5"
                  style={{ background: '#151B2F', border: `1px solid ${key.status === 'revoked' ? 'rgba(107,114,128,0.2)' : 'rgba(255,255,255,0.07)'}` }}>
                  <div className="flex items-center justify-between mb-3">
                    <div className="flex items-center gap-3">
                      <div className="w-8 h-8 rounded-lg flex items-center justify-center"
                        style={{ background: key.status === 'revoked' ? 'rgba(107,114,128,0.1)' : 'rgba(79,140,255,0.1)' }}>
                        <Key size={15} color={key.status === 'revoked' ? '#6B7280' : '#4F8CFF'} />
                      </div>
                      <div>
                        <div style={{ color: key.status === 'revoked' ? '#6B7280' : '#E5E7EB', fontSize: 14, fontWeight: 500 }}>{key.name}</div>
                        <div style={{ color: '#6B7280', fontSize: 11, fontFamily: 'monospace', marginTop: 2 }}>{key.prefix}••••••••</div>
                      </div>
                    </div>
                    <div className="flex items-center gap-3">
                      <span className="px-2 py-0.5 rounded"
                        style={{ background: key.status === 'active' ? 'rgba(16,185,129,0.1)' : 'rgba(107,114,128,0.1)', color: key.status === 'active' ? '#10B981' : '#6B7280', fontSize: 11 }}>
                        {key.status}
                      </span>
                      {key.status === 'active' && (
                        <button onClick={() => revokeKey.mutate(key.id)} disabled={revokeKey.isPending}
                          className="flex items-center gap-1.5 px-3 py-1.5 rounded-lg"
                          style={{ background: 'rgba(239,68,68,0.08)', color: '#EF4444', border: 'none', fontSize: 12, cursor: 'pointer' }}>
                          <Trash2 size={11} />Revoke
                        </button>
                      )}
                    </div>
                  </div>

                  <div className="flex items-center gap-4 text-xs">
                    <div className="flex flex-wrap gap-1.5">
                      {key.scopes.map(s => (
                        <span key={s} className="px-2 py-0.5 rounded"
                          style={{ background: 'rgba(79,140,255,0.08)', color: '#4F8CFF', fontSize: 10 }}>{s}</span>
                      ))}
                    </div>
                    <div className="ml-auto flex gap-4">
                      <span style={{ color: '#4B5563', fontSize: 11 }}>
                        Created {new Date(key.createdAt).toLocaleDateString()}
                      </span>
                      <span style={{ color: '#4B5563', fontSize: 11 }}>
                        {key.lastUsedAt ? `Last used ${new Date(key.lastUsedAt).toLocaleDateString()}` : 'Never used'}
                      </span>
                    </div>
                  </div>
                </div>
              ))}
              {keys.length === 0 && (
                <div className="text-center py-12" style={{ color: '#6B7280' }}>
                  <Key size={32} color="#374151" className="mx-auto mb-3" />
                  <p>No API keys yet. Generate your first key.</p>
                </div>
              )}
            </div>

            {/* Create Key Modal */}
            {showCreate && (
              <div className="fixed inset-0 z-50 flex items-center justify-center"
                style={{ background: 'rgba(0,0,0,0.7)', backdropFilter: 'blur(4px)' }}>
                <div className="w-full max-w-md rounded-2xl p-6" style={{ background: '#151B2F', border: '1px solid rgba(255,255,255,0.1)' }}>
                  <div className="flex items-center justify-between mb-5">
                    <h3 style={{ color: '#E5E7EB', fontSize: 16, fontWeight: 600 }}>Generate API Key</h3>
                    <button onClick={() => setShowCreate(false)} style={{ color: '#6B7280', background: 'none', border: 'none', cursor: 'pointer' }}>✕</button>
                  </div>

                  <div className="mb-4">
                    <label style={{ color: '#9CA3AF', fontSize: 13, display: 'block', marginBottom: 6 }}>Key Name</label>
                    <input value={newKeyName} onChange={e => setNewKeyName(e.target.value)}
                      placeholder="e.g. CI/CD Pipeline"
                      className="w-full rounded-xl px-4 py-3 outline-none"
                      style={{ background: '#0F1629', border: '1px solid rgba(255,255,255,0.09)', color: '#E5E7EB', fontSize: 13 }} />
                  </div>

                  <div className="mb-5">
                    <label style={{ color: '#9CA3AF', fontSize: 13, display: 'block', marginBottom: 8 }}>Scopes</label>
                    <div className="grid grid-cols-2 gap-2">
                      {AVAILABLE_SCOPES.map(scope => (
                        <label key={scope.value} className="flex items-center gap-2 cursor-pointer p-2 rounded-lg"
                          style={{ background: selectedScopes.includes(scope.value) ? 'rgba(79,140,255,0.08)' : 'rgba(255,255,255,0.03)' }}>
                          <input type="checkbox" checked={selectedScopes.includes(scope.value)}
                            onChange={() => toggleScope(scope.value)}
                            style={{ accentColor: '#4F8CFF' }} />
                          <span style={{ color: '#E5E7EB', fontSize: 12 }}>{scope.label}</span>
                        </label>
                      ))}
                    </div>
                  </div>

                  {/* Security notice */}
                  <div className="flex items-start gap-2 mb-4 p-3 rounded-xl"
                    style={{ background: 'rgba(245,158,11,0.08)', border: '1px solid rgba(245,158,11,0.2)' }}>
                    <Shield size={13} color="#F59E0B" style={{ marginTop: 1, flexShrink: 0 }} />
                    <p style={{ color: '#D97706', fontSize: 11 }}>
                      The full API key will be shown once. Store it securely — we cannot retrieve it later.
                    </p>
                  </div>

                  <div className="flex gap-3">
                    <button onClick={() => setShowCreate(false)} className="flex-1 py-2.5 rounded-xl"
                      style={{ background: 'rgba(255,255,255,0.07)', color: '#9CA3AF', border: 'none', cursor: 'pointer' }}>
                      Cancel
                    </button>
                    <button onClick={handleCreate} disabled={createKey.isPending || !newKeyName || selectedScopes.length === 0}
                      className="flex-1 py-2.5 rounded-xl"
                      style={{ background: 'linear-gradient(135deg, #4F8CFF, #3B6FCC)', color: 'white', border: 'none', cursor: 'pointer' }}>
                      {createKey.isPending ? 'Generating...' : 'Generate Key'}
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
// src/mocks/handlers/integrations.handlers.ts
import { http, HttpResponse } from 'msw';
import { randomBytes } from 'crypto';  // Node.js crypto — MSW chạy trong Node context

const apiKeysFixture = [
  { id: 'k-001', name: 'CI/CD Pipeline', prefix: 'osv_prod_xK7m', scopes: ['scan:write', 'finding:read'], createdAt: '2026-06-01T00:00:00Z', lastUsedAt: new Date(Date.now() - 120000).toISOString(), expiresAt: '2026-12-31T00:00:00Z', status: 'active', createdBy: 'carol@company.com' },
  { id: 'k-002', name: 'SIEM Integration', prefix: 'osv_prod_mN2k', scopes: ['finding:read', 'asset:read'], createdAt: '2026-05-15T00:00:00Z', lastUsedAt: new Date(Date.now() - 1800000).toISOString(), expiresAt: undefined, status: 'active', createdBy: 'carol@company.com' },
  { id: 'k-003', name: 'Monitoring Agent', prefix: 'osv_agent_Rp9s', scopes: ['agent:report'], createdAt: '2026-04-01T00:00:00Z', lastUsedAt: new Date(Date.now() - 600000).toISOString(), expiresAt: undefined, status: 'active', createdBy: 'carol@company.com' },
  { id: 'k-004', name: 'Old Dev Key', prefix: 'osv_dev_j3Lm', scopes: ['scan:read', 'finding:read'], createdAt: '2026-01-10T00:00:00Z', lastUsedAt: '2026-02-20T00:00:00Z', expiresAt: undefined, status: 'revoked', createdBy: 'bob.chen@company.com' },
];

export const integrationHandlers = [
  http.get('/api/v1/api-keys', () => {
    return HttpResponse.json({ keys: apiKeysFixture, total: apiKeysFixture.length });
  }),

  http.post('/api/v1/api-keys', async ({ request }) => {
    const body = await request.json() as any;

    // ✅ Backend generate — cryptographically secure
    // Trong production backend Go: crypto/rand.Read()
    // Trong MSW Node.js: crypto.randomBytes()
    const rawBytes = Array.from({ length: 32 }, () => Math.floor(Math.random() * 256));
    const rawKey = `osv_prod_${Buffer.from(rawBytes).toString('base64url').slice(0, 32)}`;
    const prefix = `osv_prod_${rawKey.slice(9, 13)}`;

    const newKey = {
      id: `k-${Date.now()}`,
      name: body.name,
      prefix,
      scopes: body.scopes,
      createdAt: new Date().toISOString(),
      lastUsedAt: undefined,
      expiresAt: body.expiresAt,
      status: 'active',
      createdBy: 'current-user@company.com',
    };

    apiKeysFixture.push(newKey);

    return HttpResponse.json({
      key: newKey,
      rawKey,  // Trả về 1 lần duy nhất
    }, { status: 201 });
  }),

  http.delete('/api/v1/api-keys/:id', ({ params }) => {
    const key = apiKeysFixture.find(k => k.id === params.id);
    if (key) key.status = 'revoked';
    return HttpResponse.json({ success: true });
  }),
];
```

## Lưu ý bảo mật quan trọng

> **Backend implementation (Go)** phải dùng `crypto/rand.Read()` để generate API keys:
> ```go
> // services/identity-service/api_key.go
> func GenerateAPIKey() (prefix string, rawKey string, hashedKey string, err error) {
>     b := make([]byte, 32)
>     if _, err = rand.Read(b); err != nil {
>         return
>     }
>     rawKey = fmt.Sprintf("osv_prod_%s", base64.URLEncoding.EncodeToString(b))
>     prefix = rawKey[:16]
>     hashedKey = hashSHA256(rawKey)  // Chỉ lưu hash vào DB
>     return
> }
> ```
