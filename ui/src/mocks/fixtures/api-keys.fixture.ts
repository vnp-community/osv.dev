/**
 * Fixture dữ liệu cho API Keys.
 * Dùng trong integration.handlers.ts và các unit tests.
 *
 * ⚠️ Chỉ dùng trong src/mocks/
 */

export interface APIKeyFixture {
  id: string;
  name: string;
  prefix: string;
  scopes: string[];
  created_at: string;
  last_used_at?: string;
  expires_at?: string;
  status: 'active' | 'revoked';
  created_by: string;
}

export const apiKeysFixture: APIKeyFixture[] = [
  {
    id: 'k-001',
    name: 'CI/CD Pipeline',
    prefix: 'osv_prod_xK7m',
    scopes: ['scan:write', 'finding:read'],
    created_at: '2026-06-01T00:00:00Z',
    last_used_at: new Date(Date.now() - 120_000).toISOString(),
    expires_at: '2026-12-31T00:00:00Z',
    status: 'active',
    created_by: 'carol@company.com',
  },
  {
    id: 'k-002',
    name: 'SIEM Integration',
    prefix: 'osv_prod_mN2k',
    scopes: ['finding:read', 'asset:read'],
    created_at: '2026-05-15T00:00:00Z',
    last_used_at: new Date(Date.now() - 1_800_000).toISOString(),
    expires_at: undefined,
    status: 'active',
    created_by: 'carol@company.com',
  },
  {
    id: 'k-003',
    name: 'Monitoring Agent',
    prefix: 'osv_agent_Rp9s',
    scopes: ['agent:report'],
    created_at: '2026-04-01T00:00:00Z',
    last_used_at: new Date(Date.now() - 600_000).toISOString(),
    expires_at: undefined,
    status: 'active',
    created_by: 'carol@company.com',
  },
  {
    id: 'k-004',
    name: 'Old Dev Key',
    prefix: 'osv_dev_j3Lm',
    scopes: ['scan:read', 'finding:read'],
    created_at: '2026-01-10T00:00:00Z',
    last_used_at: '2026-02-20T00:00:00Z',
    expires_at: undefined,
    status: 'revoked',
    created_by: 'bob.chen@company.com',
  },
];
