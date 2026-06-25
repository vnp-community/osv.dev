import { http, HttpResponse, delay } from 'msw';
import { ENDPOINTS } from '@/shared/api/endpoints';
import { adminUsersFixture } from '../fixtures/admin-users.fixture';
import { auditLogsFixture } from '../fixtures/audit-logs.fixture';

const BASE = '';

// ── Mutable clone so PATCH/unlock mutations take effect ──────────────────────
const users = adminUsersFixture.map((u) => ({ ...u }));

const healthFixture = {
  services: [
    { name: 'identity-service', status: 'healthy', response_time_ms: 12, last_checked_at: '2026-06-16T12:00:00Z', version: '2.2.0', details: null },
    { name: 'data-service', status: 'healthy', response_time_ms: 18, last_checked_at: '2026-06-16T12:00:00Z', version: '2.2.0', details: null },
    { name: 'search-service', status: 'degraded', response_time_ms: 850, last_checked_at: '2026-06-16T12:00:00Z', version: '2.2.0', details: 'OpenSearch high latency' },
    { name: 'finding-service', status: 'healthy', response_time_ms: 15, last_checked_at: '2026-06-16T12:00:00Z', version: '2.1.0', details: null },
  ],
  nats: { status: 'healthy', pending_messages: 12, consumer_lag: 0 },
  postgres: { status: 'healthy', active_connections: 45, max_connections: 200 },
};

// ── RBAC Matrix — uses PERMISSIONS/MATRIX from current RBACManagement ────────
const rolesFixture = {
  roles: ['admin', 'user', 'readonly', 'agent'],
  permissions: [
    { permission: 'dashboard.view', description: 'View dashboard', roles: { admin: true, user: true, readonly: true, agent: false } },
    { permission: 'dashboard.export', description: 'Export dashboard data', roles: { admin: true, user: true, readonly: false, agent: false } },
    { permission: 'cve.read', description: 'View CVE intelligence', roles: { admin: true, user: true, readonly: true, agent: false } },
    { permission: 'cve.search', description: 'Search CVEs', roles: { admin: true, user: true, readonly: true, agent: false } },
    { permission: 'cve.export', description: 'Export CVE data', roles: { admin: true, user: true, readonly: false, agent: false } },
    { permission: 'scan.create', description: 'Create and start scans', roles: { admin: true, user: true, readonly: false, agent: true } },
    { permission: 'scan.read', description: 'View scan results', roles: { admin: true, user: true, readonly: true, agent: true } },
    { permission: 'scan.cancel', description: 'Cancel running scans', roles: { admin: true, user: true, readonly: false, agent: false } },
    { permission: 'scan.delete', description: 'Delete scans', roles: { admin: true, user: false, readonly: false, agent: false } },
    { permission: 'finding.read', description: 'View findings', roles: { admin: true, user: true, readonly: true, agent: true } },
    { permission: 'finding.create', description: 'Create findings', roles: { admin: true, user: true, readonly: false, agent: true } },
    { permission: 'finding.update', description: 'Update finding status', roles: { admin: true, user: true, readonly: false, agent: false } },
    { permission: 'finding.close', description: 'Close findings', roles: { admin: true, user: true, readonly: false, agent: false } },
    { permission: 'finding.accept_risk', description: 'Accept risk on findings', roles: { admin: true, user: false, readonly: false, agent: false } },
    { permission: 'asset.read', description: 'View assets', roles: { admin: true, user: true, readonly: true, agent: true } },
    { permission: 'asset.create', description: 'Create assets', roles: { admin: true, user: true, readonly: false, agent: true } },
    { permission: 'asset.update', description: 'Update assets', roles: { admin: true, user: true, readonly: false, agent: false } },
    { permission: 'asset.delete', description: 'Delete assets', roles: { admin: true, user: false, readonly: false, agent: false } },
    { permission: 'product.read', description: 'View products', roles: { admin: true, user: true, readonly: true, agent: false } },
    { permission: 'product.create', description: 'Create products', roles: { admin: true, user: true, readonly: false, agent: false } },
    { permission: 'product.update', description: 'Update products', roles: { admin: true, user: true, readonly: false, agent: false } },
    { permission: 'report.read', description: 'View reports', roles: { admin: true, user: true, readonly: true, agent: false } },
    { permission: 'report.create', description: 'Create reports', roles: { admin: true, user: true, readonly: false, agent: false } },
    { permission: 'report.delete', description: 'Delete reports', roles: { admin: true, user: false, readonly: false, agent: false } },
    { permission: 'report.share', description: 'Share reports', roles: { admin: true, user: true, readonly: false, agent: false } },
    { permission: 'user.read', description: 'View user list', roles: { admin: true, user: false, readonly: false, agent: false } },
    { permission: 'user.create', description: 'Invite users', roles: { admin: true, user: false, readonly: false, agent: false } },
    { permission: 'user.disable', description: 'Disable users', roles: { admin: true, user: false, readonly: false, agent: false } },
    { permission: 'role.manage', description: 'Manage roles and permissions', roles: { admin: true, user: false, readonly: false, agent: false } },
    { permission: 'audit.read', description: 'View audit logs', roles: { admin: true, user: false, readonly: false, agent: false } },
    { permission: 'settings.manage', description: 'Manage system settings', roles: { admin: true, user: false, readonly: false, agent: false } },
    { permission: 'api_key.create', description: 'Create API keys', roles: { admin: true, user: true, readonly: false, agent: false } },
    { permission: 'webhook.manage', description: 'Manage webhooks', roles: { admin: true, user: false, readonly: false, agent: false } },
    { permission: 'integration.manage', description: 'Manage integrations', roles: { admin: true, user: false, readonly: false, agent: false } },
  ],
};

// ── Mutable settings store ────────────────────────────────────────────────────
let settingsStore = {
  general: {
    platform_name: 'OSV Platform',
    organization: 'Company Security',
    support_email: 'security@company.com',
    timezone: 'Asia/Ho_Chi_Minh',
    date_format: 'YYYY-MM-DD',
  },
  smtp: {
    host: 'smtp.company.com',
    port: 587,
    username: 'noreply@company.com',
    from_name: 'OSV Platform',
  },
  security: {
    password_min_length: 12,
    password_max_age_days: 90,
    session_timeout_minutes: 60,
    max_concurrent_sessions: 3,
    mfa_required: true,
    allow_sms_otp: false,
  },
  ai: {
    providers: [
      { id: 'openai', name: 'OpenAI', model: 'gpt-4o', status: 'active', latency: '203ms', usage: '4,821 req/day', cost: '$12.40/day' },
      { id: 'azure', name: 'Azure OpenAI', model: 'gpt-4-turbo', status: 'standby', latency: '—', usage: '0 req/day', cost: '$0.00/day' },
      { id: 'ollama', name: 'Ollama (Local)', model: 'llama3:8b', status: 'inactive', latency: '—', usage: '0 req/day', cost: '$0.00' },
    ],
    active_provider_id: 'openai',
  },
};

export const adminHandlers = [
  // ── Admin Users ──────────────────────────────────────────────────────────────
  http.get(`${BASE}${ENDPOINTS.admin.users}`, async ({ request }) => {
    await delay(300);
    const url = new URL(request.url);
    const search = url.searchParams.get('search')?.toLowerCase();
    const role = url.searchParams.get('role');
    const page = Number(url.searchParams.get('page') ?? '1');
    const pageSize = Number(url.searchParams.get('page_size') ?? '20');

    let filtered = [...users];
    if (search) {
      filtered = filtered.filter(
        (u) =>
          u.name.toLowerCase().includes(search) ||
          u.email.toLowerCase().includes(search)
      );
    }
    if (role) filtered = filtered.filter((u) => u.role === role);

    const start = (page - 1) * pageSize;
    return HttpResponse.json({
      users: filtered.slice(start, start + pageSize),
      total: filtered.length,
      page,
      page_size: pageSize,
    });
  }),

  http.post(`${BASE}${ENDPOINTS.admin.userInvite}`, async ({ request }) => {
    await delay(400);
    const body = (await request.json()) as any;
    const newUser = {
      id: `u-${Date.now()}`,
      email: body.email,
      name: body.name,
      role: body.role,
      is_active: true,
      mfa_enabled: false,
      last_login_at: undefined,
      created_at: new Date().toISOString(),
      login_attempts: 0,
      is_locked: false,
    };
    users.push(newUser);
    return HttpResponse.json(newUser, { status: 201 });
  }),

  http.patch(`${BASE}/api/v1/admin/users/:id`, async ({ request, params }) => {
    await delay(300);
    const body = (await request.json()) as any;
    const user = users.find((u) => u.id === params.id);
    if (!user) return new HttpResponse(null, { status: 404 });
    Object.assign(user, body);
    return HttpResponse.json(user);
  }),

  http.post(`${BASE}${ENDPOINTS.admin.userUnlock(':id')}`, async ({ params }) => {
    await delay(300);
    const user = users.find((u) => u.id === params.id);
    if (user) {
      user.is_locked = false;
      user.login_attempts = 0;
    }
    return HttpResponse.json({ success: true, user_id: params.id, is_locked: false });
  }),

  http.post(`${BASE}${ENDPOINTS.admin.userReset(':id')}`, async ({ params }) => {
    await delay(300);
    const user = users.find((u) => u.id === params.id);
    return HttpResponse.json({ success: true, email: user?.email, reset_sent_at: new Date().toISOString() });
  }),

  // ── System Health ────────────────────────────────────────────────────────────
  http.get(`${BASE}${ENDPOINTS.admin.health}`, async () => {
    await delay(300);
    return HttpResponse.json(healthFixture);
  }),

  // ── RBAC Roles ───────────────────────────────────────────────────────────────
  http.get(`${BASE}${ENDPOINTS.admin.roles}`, async () => {
    await delay(300);
    return HttpResponse.json(rolesFixture);
  }),

  // ── Audit Logs ───────────────────────────────────────────────────────────────
  http.get(`${BASE}${ENDPOINTS.audit.log}`, async ({ request }) => {
    await delay(300);
    const url = new URL(request.url);
    const search = url.searchParams.get('search')?.toLowerCase();
    const severity = url.searchParams.get('severity');
    const page = Number(url.searchParams.get('page') ?? '1');
    const pageSize = Number(url.searchParams.get('page_size') ?? '50');

    let filtered = [...auditLogsFixture];
    if (severity) filtered = filtered.filter((e) => e.severity === severity);
    if (search) {
      filtered = filtered.filter(
        (e) =>
          e.action.toLowerCase().includes(search) ||
          e.user_name.toLowerCase().includes(search) ||
          (e.resource ?? '').toLowerCase().includes(search)
      );
    }

    const start = (page - 1) * pageSize;
    return HttpResponse.json({
      entries: filtered.slice(start, start + pageSize),
      total: filtered.length,
      page,
      page_size: pageSize,
    });
  }),

  // ── System Settings ──────────────────────────────────────────────────────────
  http.get(`${BASE}${ENDPOINTS.admin.settings}`, async () => {
    await delay(300);
    return HttpResponse.json(settingsStore);
  }),

  http.put(`${BASE}${ENDPOINTS.admin.settings}`, async ({ request }) => {
    await delay(400);
    const body = (await request.json()) as any;
    settingsStore = { ...settingsStore, ...body };
    return HttpResponse.json(settingsStore);
  }),

  // ── JIRA Config ──────────────────────────────────────────────────────────────
  http.post(`${BASE}/api/v1/jira/config`, async () => {
    await delay(300);
    return HttpResponse.json({ success: true });
  }),
  http.post(`${BASE}/api/v1/jira/config/test`, async () => {
    await delay(300);
    return HttpResponse.json({ success: true, message: 'Connection successful' });
  }),
];
