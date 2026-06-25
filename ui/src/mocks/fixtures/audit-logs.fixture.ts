/**
 * Fixture dữ liệu cho Audit Logs.
 * Dùng trong admin.handlers.ts và các unit tests.
 *
 * ⚠️ Chỉ dùng trong src/mocks/
 */

export interface AuditEventFixture {
  id: string;
  timestamp: string;
  user_id: string;
  user_name: string;
  action: string;
  entity_type: string;
  entity_id: string;
  resource?: string;
  severity: 'Info' | 'Warning' | 'Critical';
  ip_address: string;
  result: 'success' | 'failure';
  metadata?: Record<string, unknown>;
  before?: string;
  after?: string;
}

export const auditLogsFixture: AuditEventFixture[] = [
  {
    id: 'AL-1001',
    timestamp: new Date(Date.now() - 30 * 60_000).toISOString(),
    user_id: 'u-1',
    user_name: 'carol@company.com',
    action: 'CREATE_SCAN',
    entity_type: 'Scan',
    entity_id: 'SC-0047',
    resource: 'Scan / SC-0047',
    severity: 'Info',
    ip_address: '10.0.0.1',
    result: 'success',
    after: JSON.stringify({ type: 'NMAP', target: '10.0.0.0/16' }),
  },
  {
    id: 'AL-1002',
    timestamp: new Date(Date.now() - 90 * 60_000).toISOString(),
    user_id: 'u-2',
    user_name: 'bob.chen@company.com',
    action: 'UPDATE_FINDING',
    entity_type: 'Finding',
    entity_id: 'F-2847',
    resource: 'Finding / F-2847',
    severity: 'Warning',
    ip_address: '10.0.0.2',
    result: 'success',
    before: JSON.stringify({ status: 'New' }),
    after: JSON.stringify({ status: 'Active' }),
  },
  {
    id: 'AL-1003',
    timestamp: new Date(Date.now() - 3 * 3_600_000).toISOString(),
    user_id: 'u-1',
    user_name: 'carol@company.com',
    action: 'CREATE_API_KEY',
    entity_type: 'APIKey',
    entity_id: 'k-003',
    resource: 'APIKey / k-003',
    severity: 'Warning',
    ip_address: '10.0.0.1',
    result: 'success',
    after: JSON.stringify({ name: 'CI/CD Pipeline', scopes: ['scan:write', 'finding:read'] }),
  },
  {
    id: 'AL-1004',
    timestamp: new Date(Date.now() - 6 * 3_600_000).toISOString(),
    user_id: 'system',
    user_name: 'system',
    action: 'SCAN_COMPLETED',
    entity_type: 'Scan',
    entity_id: 'SC-0046',
    resource: 'Scan / SC-0046',
    severity: 'Info',
    ip_address: '127.0.0.1',
    result: 'success',
    metadata: { findings: 23, duration_ms: 9_000_000 },
  },
  {
    id: 'AL-1005',
    timestamp: new Date(Date.now() - 12 * 3_600_000).toISOString(),
    user_id: 'u-4',
    user_name: 'dave.kim@company.com',
    action: 'LOGIN_FAILED',
    entity_type: 'User',
    entity_id: 'u-4',
    resource: 'User / dave.kim@company.com',
    severity: 'Critical',
    ip_address: '203.0.113.42',
    result: 'failure',
    metadata: { reason: 'wrong_password', attempt: 3 },
  },
  {
    id: 'AL-1006',
    timestamp: new Date(Date.now() - 24 * 3_600_000).toISOString(),
    user_id: 'u-1',
    user_name: 'carol@company.com',
    action: 'ACCEPT_RISK',
    entity_type: 'Finding',
    entity_id: 'F-2841',
    resource: 'Finding / F-2841',
    severity: 'Warning',
    ip_address: '10.0.0.1',
    result: 'success',
    after: JSON.stringify({ expiration_date: '2026-09-19', reason: 'Network controls mitigate' }),
  },
];
