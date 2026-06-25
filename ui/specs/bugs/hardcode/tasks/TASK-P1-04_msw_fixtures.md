# TASK-P1-04 — Tạo MSW Fixtures cho tất cả endpoints mới

**Phase:** 1 — Foundation  
**Nguồn giải pháp:** Tổng hợp từ section MSW Handler trong tất cả solution files  
**Ưu tiên:** 🔴 PHẢI hoàn thành trước khi test bất kỳ component nào  
**Phụ thuộc:** TASK-P1-03 (thư mục fixtures/ phải tồn tại)

---

## Mục tiêu

Tạo tất cả MSW fixture data files và handler files tập trung, để khi các component Phase 2–5 được fix thì có ngay data để test trong development.

---

## Danh sách files cần tạo

### 1. `src/mocks/fixtures/users.fixture.ts`
> Dữ liệu từ: `solutions/01_admin_user_management.md` — MSW Handler

```typescript
export const usersFixture = [
  { id: 'u-1', name: 'Carol Anderson', email: 'carol@company.com', role: 'admin',
    isActive: true, mfaEnabled: true, lastLoginAt: new Date(Date.now() - 5 * 60000).toISOString(),
    createdAt: '2026-01-01T00:00:00Z', loginAttempts: 0, isLocked: false },
  { id: 'u-2', name: 'Bob Chen', email: 'bob.chen@company.com', role: 'user',
    isActive: true, mfaEnabled: true, lastLoginAt: new Date(Date.now() - 3600000).toISOString(),
    createdAt: '2026-01-05T00:00:00Z', loginAttempts: 0, isLocked: false },
  { id: 'u-3', name: 'Alice Wu', email: 'alice.wu@company.com', role: 'user',
    isActive: true, mfaEnabled: true, lastLoginAt: new Date(Date.now() - 7200000).toISOString(),
    createdAt: '2026-01-10T00:00:00Z', loginAttempts: 0, isLocked: false },
  { id: 'u-4', name: 'Dave Kim', email: 'dave.kim@company.com', role: 'user',
    isActive: true, mfaEnabled: false, lastLoginAt: new Date(Date.now() - 86400000).toISOString(),
    createdAt: '2026-02-01T00:00:00Z', loginAttempts: 0, isLocked: false },
  { id: 'u-5', name: 'Eve Martinez', email: 'eve.m@company.com', role: 'readonly',
    isActive: true, mfaEnabled: true, lastLoginAt: new Date(Date.now() - 3 * 86400000).toISOString(),
    createdAt: '2026-02-15T00:00:00Z', loginAttempts: 0, isLocked: false },
  { id: 'u-6', name: 'Frank Liu', email: 'frank.l@company.com', role: 'agent',
    isActive: false, mfaEnabled: false, lastLoginAt: undefined,
    createdAt: '2026-03-01T00:00:00Z', loginAttempts: 0, isLocked: false },
];
```

### 2. `src/mocks/fixtures/audit.fixture.ts`
> Dữ liệu từ: `solutions/02_admin_audit_logs.md` — MSW Handler

```typescript
export const auditFixture = [
  { id: 'AL-1001', timestamp: '2026-06-14T09:42:15Z', userId: 'u-1',
    userName: 'carol@company.com', action: 'CREATE_SCAN',
    entityType: 'Scan', entityId: 'SC-0047', resource: 'Scan / SC-0047',
    severity: 'Info', ipAddress: '10.0.0.1', result: 'success',
    after: '{ "type": "NMAP", "target": "10.0.0.0/16" }' },
  { id: 'AL-1002', timestamp: '2026-06-14T09:35:01Z', userId: 'u-2',
    userName: 'bob.chen@company.com', action: 'UPDATE_FINDING',
    entityType: 'Finding', entityId: 'F-2847', resource: 'Finding / F-2847',
    severity: 'Warning', ipAddress: '10.0.0.2', result: 'success',
    before: '{ "status": "New" }', after: '{ "status": "Active" }' },
  { id: 'AL-1003', timestamp: '2026-06-14T08:15:00Z', userId: 'u-1',
    userName: 'carol@company.com', action: 'CREATE_API_KEY',
    entityType: 'APIKey', entityId: 'k-003', resource: 'APIKey / k-003',
    severity: 'Warning', ipAddress: '10.0.0.1', result: 'success',
    after: '{ "name": "CI/CD Pipeline" }' },
  { id: 'AL-1004', timestamp: '2026-06-14T07:00:00Z', userId: 'system',
    userName: 'system', action: 'SCAN_COMPLETED',
    entityType: 'Scan', entityId: 'SC-0046', resource: 'Scan / SC-0046',
    severity: 'Info', ipAddress: '127.0.0.1', result: 'success', metadata: { findings: 23 } },
  { id: 'AL-1005', timestamp: '2026-06-13T23:59:00Z', userId: 'u-4',
    userName: 'dave.kim@company.com', action: 'LOGIN_FAILED',
    entityType: 'User', entityId: 'u-4', resource: 'User / dave.kim@company.com',
    severity: 'Critical', ipAddress: '203.0.113.42', result: 'failure', metadata: { reason: 'wrong_password', attempt: 3 } },
];
```

### 3. `src/mocks/fixtures/notifications.fixture.ts`
> Dữ liệu từ: `solutions/10_11_12_webhook_report_notification.md` — Solution 12

```typescript
export const notificationsFixture = [
  { id: 'n-1', type: 'critical', title: 'Critical Finding Detected',
    description: 'CVE-2025-44228 found on webserver01.prod — CVSS 10.0, KEV active',
    product: 'Banking App', read: false,
    createdAt: new Date(Date.now() - 10 * 60000).toISOString(), timeAgo: '10 min ago' },
  { id: 'n-2', type: 'sla', title: 'SLA Breach Imminent',
    description: 'F-2842 (Cisco IOS XE) SLA expires in 24h — escalation required',
    product: 'Network Infra', read: false,
    createdAt: new Date(Date.now() - 25 * 60000).toISOString(), timeAgo: '25 min ago' },
  { id: 'n-3', type: 'kev', title: 'New KEV Added',
    description: 'CISA added CVE-2025-77001 (Microsoft Exchange) to KEV catalog',
    product: 'Global', read: false,
    createdAt: new Date(Date.now() - 3600000).toISOString(), timeAgo: '1h ago' },
  { id: 'n-4', type: 'scan', title: 'Scan Completed',
    description: 'Production Network Sweep (SC-0047) completed — 47 findings discovered',
    product: 'Production', read: true,
    createdAt: new Date(Date.now() - 7200000).toISOString(), timeAgo: '2h ago' },
  { id: 'n-5', type: 'critical', title: 'SLA Overdue',
    description: 'F-2846 (Spring Framework RCE) is 2 days overdue — immediate action required',
    product: 'API Gateway', read: true,
    createdAt: new Date(Date.now() - 10800000).toISOString(), timeAgo: '3h ago' },
];
```

### 4. `src/mocks/fixtures/api-keys.fixture.ts`
> Dữ liệu từ: `solutions/09_api_key_management.md` — MSW Handler

```typescript
export const apiKeysFixture = [
  { id: 'k-001', name: 'CI/CD Pipeline', prefix: 'osv_prod_xK7m',
    scopes: ['scan:write', 'finding:read'], createdAt: '2026-06-01T00:00:00Z',
    lastUsedAt: new Date(Date.now() - 120000).toISOString(), expiresAt: '2026-12-31T00:00:00Z',
    status: 'active', createdBy: 'carol@company.com' },
  { id: 'k-002', name: 'SIEM Integration', prefix: 'osv_prod_mN2k',
    scopes: ['finding:read', 'asset:read'], createdAt: '2026-05-15T00:00:00Z',
    lastUsedAt: new Date(Date.now() - 1800000).toISOString(), expiresAt: undefined,
    status: 'active', createdBy: 'carol@company.com' },
  { id: 'k-003', name: 'Monitoring Agent', prefix: 'osv_agent_Rp9s',
    scopes: ['agent:report'], createdAt: '2026-04-01T00:00:00Z',
    lastUsedAt: new Date(Date.now() - 600000).toISOString(), expiresAt: undefined,
    status: 'active', createdBy: 'carol@company.com' },
  { id: 'k-004', name: 'Old Dev Key', prefix: 'osv_dev_j3Lm',
    scopes: ['scan:read', 'finding:read'], createdAt: '2026-01-10T00:00:00Z',
    lastUsedAt: '2026-02-20T00:00:00Z', expiresAt: undefined,
    status: 'revoked', createdBy: 'bob.chen@company.com' },
];
```

### 5. `src/mocks/fixtures/reports.fixture.ts`
> Dữ liệu từ: `solutions/10_11_12_webhook_report_notification.md` — Solution 11

```typescript
export const reportsFixture = [
  { id: 'R-047', name: 'Q2 2026 Executive Summary', type: 'Executive', format: 'pdf',
    status: 'completed', fileSizeBytes: 2516582, generatedAt: '2026-06-14T09:00:00Z',
    createdAt: '2026-06-14T09:00:00Z', createdBy: 'carol@company.com' },
  { id: 'R-046', name: 'Banking App Technical Report', type: 'Technical', format: 'pdf',
    status: 'completed', fileSizeBytes: 9123456, generatedAt: '2026-06-13T16:30:00Z',
    createdAt: '2026-06-13T16:30:00Z', createdBy: 'bob.chen@company.com' },
  { id: 'R-045', name: 'PCI DSS Compliance Q2', type: 'Compliance', format: 'pdf',
    status: 'completed', fileSizeBytes: 4300000, generatedAt: '2026-06-12T11:00:00Z',
    createdAt: '2026-06-12T11:00:00Z', createdBy: 'carol@company.com' },
];

export const reportTemplatesFixture = [
  { id: 'exec', name: 'Executive Summary', description: 'High-level overview for C-level presentations', type: 'Executive' },
  { id: 'tech', name: 'Technical Report', description: 'Detailed findings with CVE details and remediation', type: 'Technical' },
  { id: 'comp', name: 'Compliance Report', description: 'Mapped to PCI DSS, ISO 27001, SOC2, NIST', type: 'Compliance' },
];
```

---

## Tiêu chí hoàn thành

- [x] `src/mocks/fixtures/admin-users.fixture.ts` — tạo xong (6 users, typed interface)
- [x] `src/mocks/fixtures/audit-logs.fixture.ts` — tạo xong (6 events, mixed severity)
- [x] `src/mocks/fixtures/notifications.fixture.ts` — tạo xong (5 notifications, 3 unread)
- [x] `src/mocks/fixtures/api-keys.fixture.ts` — tạo xong (4 keys, 1 revoked)
- [x] `src/mocks/fixtures/reports.fixture.ts` — tạo xong (3 reports + 3 templates)
- [x] `src/mocks/fixtures/risk-acceptances.fixture.ts` — tạo xong (3 acceptances: active, expiring, expired)
- [x] `src/mocks/fixtures/ai-triage.fixture.ts` — tạo xong (3 items: 2 pending, 1 reviewed)
- [x] Tất cả file TypeScript không có lỗi compile mới

---

## ✅ Đã hoàn thành — 2026-06-19

**Files đã tạo (7 fixture files mới):**
- [`admin-users.fixture.ts`](../../../../ui/src/mocks/fixtures/admin-users.fixture.ts) — 6 users, 4 roles
- [`audit-logs.fixture.ts`](../../../../ui/src/mocks/fixtures/audit-logs.fixture.ts) — 6 events (Info/Warning/Critical)
- [`notifications.fixture.ts`](../../../../ui/src/mocks/fixtures/notifications.fixture.ts) — 5 notifications, 3 unread
- [`api-keys.fixture.ts`](../../../../ui/src/mocks/fixtures/api-keys.fixture.ts) — 4 API keys (3 active, 1 revoked)
- [`reports.fixture.ts`](../../../../ui/src/mocks/fixtures/reports.fixture.ts) — 3 reports + 3 templates
- [`risk-acceptances.fixture.ts`](../../../../ui/src/mocks/fixtures/risk-acceptances.fixture.ts) — 3 acceptances (1 expired, 1 expiring soon)
- [`ai-triage.fixture.ts`](../../../../ui/src/mocks/fixtures/ai-triage.fixture.ts) — 3 items (2 pending, 1 accepted)

> Các handler files Phase 2–5 sẽ **import từ fixtures** này thay vì định nghĩa data inline. Điều này giúp fixtures có thể tái sử dụng cho cả unit tests.

