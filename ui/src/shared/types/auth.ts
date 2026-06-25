export type UserRole = 'admin' | 'user' | 'readonly' | 'agent';

export type Permission =
  | 'scan:create' | 'scan:read'
  | 'asset:write' | 'asset:read'
  | 'user:manage'
  | 'report:download'
  | 'system:configure'
  | 'finding:write' | 'finding:read'
  | 'agent:report';

export interface User {
  id: string;
  email: string;
  name: string;
  role: UserRole;
  permissions: Permission[];
  /** snake_case to match gateway-service response */
  mfa_enabled: boolean;
  avatar_url?: string | null;
  created_at: string;
  /** @deprecated aliases kept for backward compat */
  mfaEnabled?: boolean;
  avatarUrl?: string | null;
  createdAt?: string;
}

export interface AuthTokens {
  accessToken: string;    // JWT RS256, 15min TTL
  expiresIn: number;      // seconds
  // refresh_token via httpOnly cookie (server-managed)
}

export interface AuthState {
  user: User | null;
  accessToken: string | null;
  isAuthenticated: boolean;
  isLoading: boolean;
  setUser: (user: User) => void;
  setAccessToken: (token: string) => void;
  logout: () => void;
}
