// Re-export shared types + thêm auth-specific types
export type { UserRole, Permission, User, AuthState } from '@/shared/types/auth';

export interface LoginRequest {
  email: string;
  password: string;
  mfa_code?: string;
}

export interface LoginResponse {
  access_token: string | null;
  expires_in: number;
  user: import('@/shared/types/auth').User | null;
  mfa_required?: boolean;
}

export interface RefreshResponse {
  access_token: string;
  expires_in: number;
}

export interface MFASetupResponse {
  secret: string;
  qr_url: string;
  backup_codes: string[];
}
