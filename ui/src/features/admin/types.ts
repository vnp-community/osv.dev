/**
 * Admin module types — User Management, Audit Logs, RBAC, System Settings
 *
 * @see specs/bugs/hardcode/tasks/TASK-P2-01 to TASK-P2-04
 */

// ─── User Management ──────────────────────────────────────────────────────────

export type UserRole = 'admin' | 'user' | 'readonly' | 'agent';

export interface AdminUser {
  id: string;
  name: string;
  email: string;
  role: UserRole;
  is_active: boolean;
  mfa_enabled: boolean;
  last_login_at?: string;
  created_at: string;
  login_attempts: number;
  is_locked: boolean;
}

export interface AdminUsersResponse {
  users: AdminUser[];
  total: number;
  page: number;
  page_size: number;
}

export interface InviteUserRequest {
  email: string;
  name: string;
  role: UserRole;
}

// ─── Audit Logs ───────────────────────────────────────────────────────────────

export type AuditSeverity = 'Info' | 'Warning' | 'Critical';
export type AuditResult = 'success' | 'failure';

export interface AuditEvent {
  id: string;
  user_id: string;
  user_name: string;
  action: string;
  entity_type: string;
  entity_id: string;
  ip_address: string;
  result: AuditResult;
  metadata?: Record<string, unknown>;
  timestamp: string;
  // Derived / display fields
  resource?: string;
  severity?: AuditSeverity;
  before?: string;
  after?: string;
}

export interface AuditLogsResponse {
  entries: AuditEvent[];
  total: number;
  page: number;
  page_size: number;
}

export interface AuditLogsParams {
  search?: string;
  severity?: AuditSeverity;
  page?: number;
  page_size?: number;
}

// ─── RBAC ─────────────────────────────────────────────────────────────────────

export interface RBACPermission {
  permission: string;
  description: string;
  roles: Record<string, boolean>;
}

export interface RBACMatrixResponse {
  roles: string[];
  permissions: RBACPermission[];
}

// ─── System Settings ──────────────────────────────────────────────────────────

export interface AIProviderConfig {
  id: string;
  name: string;
  model: string;
  status: 'active' | 'standby' | 'inactive';
  latency?: string;
  usage?: string;
  cost?: string;
}

export interface SystemSettings {
  general: {
    platform_name: string;
    organization: string;
    support_email: string;
    timezone: string;
    date_format: string;
  };
  smtp: {
    host: string;
    port: number;
    username: string;
    from_name: string;
  };
  security: {
    password_min_length: number;
    password_max_age_days: number;
    session_timeout_minutes: number;
    max_concurrent_sessions: number;
    mfa_required: boolean;
    allow_sms_otp: boolean;
  };
  ai: {
    providers: AIProviderConfig[];
    active_provider_id: string;
  };
}
