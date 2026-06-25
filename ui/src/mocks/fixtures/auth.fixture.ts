import type { User } from '@/shared/types/auth';

// Fixtures — dùng trong MSW handlers. KHÔNG import vào components.
export const userFixtures: Record<string, User> = {
  admin: {
    id: 'usr_admin001',
    email: 'admin@company.com',
    name: 'Admin User',
    role: 'admin',
    permissions: [
      'scan:create', 'scan:read',
      'asset:write', 'asset:read',
      'finding:write', 'finding:read',
      'report:download',
      'user:manage',
      'system:configure',
    ],
    mfaEnabled: true,
    avatarUrl: undefined,
    createdAt: '2026-01-01T00:00:00Z',
  },
  bob: {
    id: 'usr_bob123',
    email: 'bob@company.com',
    name: 'Bob Smith',
    role: 'user',
    permissions: [
      'scan:create', 'scan:read',
      'asset:write', 'asset:read',
      'finding:write', 'finding:read',
      'report:download',
    ],
    mfaEnabled: false,
    avatarUrl: undefined,
    createdAt: '2026-01-15T08:00:00Z',
  },
  readonly: {
    id: 'usr_carol456',
    email: 'carol@company.com',
    name: 'Carol Jones',
    role: 'readonly',
    permissions: ['scan:read', 'asset:read', 'finding:read', 'report:download'],
    mfaEnabled: false,
    avatarUrl: undefined,
    createdAt: '2026-02-01T00:00:00Z',
  },
};

// Map email → fixture
export function getUserByEmail(email: string): User | null {
  const entry = Object.values(userFixtures).find(u => u.email === email);
  return entry ?? null;
}
