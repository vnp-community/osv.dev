/**
 * Fixture dữ liệu cho Admin Users.
 * Dùng trong admin.handlers.ts và các unit tests.
 *
 * ⚠️ Chỉ dùng trong src/mocks/
 */

export interface AdminUserFixture {
  id: string;
  name: string;
  email: string;
  role: 'admin' | 'user' | 'readonly' | 'agent';
  is_active: boolean;
  mfa_enabled: boolean;
  last_login_at?: string;
  created_at: string;
  login_attempts: number;
  is_locked: boolean;
}

export const adminUsersFixture: AdminUserFixture[] = [
  {
    id: 'u-1',
    name: 'Carol Anderson',
    email: 'carol@company.com',
    role: 'admin',
    is_active: true,
    mfa_enabled: true,
    last_login_at: new Date(Date.now() - 5 * 60_000).toISOString(),
    created_at: '2026-01-01T00:00:00Z',
    login_attempts: 0,
    is_locked: false,
  },
  {
    id: 'u-2',
    name: 'Bob Chen',
    email: 'bob.chen@company.com',
    role: 'user',
    is_active: true,
    mfa_enabled: true,
    last_login_at: new Date(Date.now() - 3_600_000).toISOString(),
    created_at: '2026-01-05T00:00:00Z',
    login_attempts: 0,
    is_locked: false,
  },
  {
    id: 'u-3',
    name: 'Alice Wu',
    email: 'alice.wu@company.com',
    role: 'user',
    is_active: true,
    mfa_enabled: true,
    last_login_at: new Date(Date.now() - 7_200_000).toISOString(),
    created_at: '2026-01-10T00:00:00Z',
    login_attempts: 0,
    is_locked: false,
  },
  {
    id: 'u-4',
    name: 'Dave Kim',
    email: 'dave.kim@company.com',
    role: 'user',
    is_active: true,
    mfa_enabled: false,
    last_login_at: new Date(Date.now() - 86_400_000).toISOString(),
    created_at: '2026-02-01T00:00:00Z',
    login_attempts: 0,
    is_locked: false,
  },
  {
    id: 'u-5',
    name: 'Eve Martinez',
    email: 'eve.m@company.com',
    role: 'readonly',
    is_active: true,
    mfa_enabled: true,
    last_login_at: new Date(Date.now() - 3 * 86_400_000).toISOString(),
    created_at: '2026-02-15T00:00:00Z',
    login_attempts: 0,
    is_locked: false,
  },
  {
    id: 'u-6',
    name: 'Frank Liu',
    email: 'frank.l@company.com',
    role: 'agent',
    is_active: false,
    mfa_enabled: false,
    last_login_at: undefined,
    created_at: '2026-03-01T00:00:00Z',
    login_attempts: 0,
    is_locked: false,
  },
];
