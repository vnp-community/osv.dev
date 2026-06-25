# TASK-API-008 — Findings MSW Handlers + Risk Acceptance Handlers

| Field | Value |
|-------|-------|
| **Task ID** | TASK-API-008 |
| **Module** | `ui/src/mocks/handlers/`, `ui/src/mocks/fixtures/` |
| **Solution Ref** | [SOL-UI-005 §5](../solutions/SOL-UI-005-finding-api.md) |
| **Priority** | 🔴 P0 |
| **Depends On** | TASK-API-007 |
| **Estimated** | 1h |

---

## Context

Cần mock finding endpoints để UI hoạt động đầy đủ: list với filter + pagination, update với state machine, bulk ops, notes, audit trail, risk acceptances.

---

## Goal

Tạo MSW fixtures cho 30 findings đa dạng và handlers cho tất cả finding endpoints.

---

## Target Files

| Action | File Path |
|--------|-----------|
| CREATE | `ui/src/mocks/fixtures/finding.fixture.ts` |
| CREATE | `ui/src/mocks/handlers/finding.handlers.ts` |

---

## Implementation

### File 1: `ui/src/mocks/fixtures/finding.fixture.ts`

```typescript
import type { Finding } from '@/features/findings/types';

// 30 findings fixtures — CHỈ dùng trong MSW handlers
export const findingsFixture: Finding[] = [
  {
    id: 'F-2847', title: 'Apache Log4j2 JNDI Remote Code Execution',
    description: 'log4j-core 2.14.1 allows attackers to execute arbitrary code via crafted log messages.',
    severity: 'Critical', cvss_v3: 10.0, cve_id: 'CVE-2021-44228', cwe_id: 'CWE-917',
    status: 'active',
    asset_id: 'ast_001', asset_name: 'app-server-01.internal',
    product_id: 'prod_1', product_name: 'Banking Portal',
    engagement_id: 'eng_001', scanner: 'grype',
    endpoint: 'https://banking.company.com/api/transfer',
    sla_expiration_date: '2026-06-09', sla_status: 'breached', days_overdue: 7,
    duplicate_of: null, assigned_to: 'bob@company.com', assigned_to_name: 'Bob Smith',
    risk_acceptance: null, is_kev: true, epss_score: 0.97567,
    created_at: '2026-06-01T08:00:00Z', updated_at: '2026-06-16T10:00:00Z',
  },
  {
    id: 'F-2848', title: 'OpenSSH regreSSHion Race Condition RCE',
    description: 'CVE-2024-6387 allows unauthenticated RCE via SSH signal handler race condition.',
    severity: 'Critical', cvss_v3: 8.1, cve_id: 'CVE-2024-6387', cwe_id: 'CWE-362',
    status: 'active',
    asset_id: 'ast_002', asset_name: 'bastion-host.internal',
    product_id: 'prod_1', product_name: 'Banking Portal',
    engagement_id: 'eng_001', scanner: 'nmap',
    endpoint: 'ssh://bastion-host.internal:22',
    sla_expiration_date: '2026-06-23', sla_status: 'at_risk', days_overdue: null,
    duplicate_of: null, assigned_to: null, assigned_to_name: null,
    risk_acceptance: null, is_kev: false, epss_score: 0.56789,
    created_at: '2026-06-09T09:00:00Z', updated_at: '2026-06-16T10:00:00Z',
  },
  {
    id: 'F-2849', title: 'Nginx Path Traversal Information Disclosure',
    description: 'nginx 1.18.0 allows reading arbitrary files via /../ sequence.',
    severity: 'Medium', cvss_v3: 5.3, cve_id: 'CVE-2024-56789', cwe_id: 'CWE-22',
    status: 'false_positive',
    asset_id: 'ast_003', asset_name: 'web-proxy-01.internal',
    product_id: 'prod_2', product_name: 'Mobile App',
    engagement_id: 'eng_002', scanner: 'zap',
    endpoint: 'https://mobile.company.com',
    sla_expiration_date: '2026-09-16', sla_status: 'ok', days_overdue: null,
    duplicate_of: null, assigned_to: null, assigned_to_name: null,
    risk_acceptance: null, is_kev: false, epss_score: 0.12300,
    created_at: '2026-06-08T07:00:00Z', updated_at: '2026-06-12T14:00:00Z',
  },
  {
    id: 'F-2850', title: 'Spring Framework RCE (Spring4Shell)',
    description: 'Allows RCE via DataBinder and ClassLoader manipulation.',
    severity: 'High', cvss_v3: 9.8, cve_id: 'CVE-2022-22965', cwe_id: 'CWE-94',
    status: 'mitigated',
    asset_id: 'ast_001', asset_name: 'app-server-01.internal',
    product_id: 'prod_3', product_name: 'Internal API',
    engagement_id: 'eng_003', scanner: 'grype',
    endpoint: null,
    sla_expiration_date: '2026-07-01', sla_status: 'ok', days_overdue: null,
    duplicate_of: null, assigned_to: 'carol@company.com', assigned_to_name: 'Carol Jones',
    risk_acceptance: null, is_kev: true, epss_score: 0.88900,
    created_at: '2026-05-20T10:00:00Z', updated_at: '2026-06-10T16:00:00Z',
  },
  {
    id: 'F-2851', title: 'Exposed .env File with Database Credentials',
    description: 'The .env file is publicly accessible, exposing DB_PASSWORD and API keys.',
    severity: 'High', cvss_v3: 7.5, cve_id: null, cwe_id: 'CWE-200',
    status: 'risk_accepted',
    asset_id: 'ast_004', asset_name: 'legacy-app.internal',
    product_id: 'prod_3', product_name: 'Internal API',
    engagement_id: 'eng_003', scanner: 'zap',
    endpoint: 'https://internal-api.company.com/.env',
    sla_expiration_date: '2026-07-01', sla_status: 'ok', days_overdue: null,
    duplicate_of: null, assigned_to: null, assigned_to_name: null,
    risk_acceptance: {
      id: 'ra_001', finding_id: 'F-2851',
      accepted_by: 'admin@company.com', accepted_by_name: 'Admin User',
      reason: 'Legacy app, plan to decommission in Q3.',
      expires_at: '2026-09-30', created_at: '2026-06-01T00:00:00Z',
    },
    is_kev: false, epss_score: null,
    created_at: '2026-05-15T08:00:00Z', updated_at: '2026-06-01T09:00:00Z',
  },
  // ... thêm 25 findings tương tự với các severity và status khác nhau
  // (truncated để giảm độ dài — AI tạo 25 entries tương tự với dữ liệu realistic)
];

// Helper functions
export function findFindingById(id: string): Finding | undefined {
  return findingsFixture.find(f => f.id === id);
}

export const findingStatsFixture = {
  by_severity: { critical: 8, high: 12, medium: 7, low: 3 },
  by_status:   { active: 20, mitigated: 5, false_positive: 2, risk_accepted: 3 },
  sla_summary: { ok: 18, at_risk: 5, breached: 7 },
  sla_compliance_pct: 82.5,
};
```

### File 2: `ui/src/mocks/handlers/finding.handlers.ts`

```typescript
import { http, HttpResponse } from 'msw';
import { ENDPOINTS } from '@/shared/api/endpoints';
import { findingsFixture, findingStatsFixture } from '../fixtures/finding.fixture';
import type { Finding, FindingStatus } from '@/features/findings/types';
import { canTransition } from '@/shared/utils/findingStateMachine';

// In-memory mutable state
let findings = [...findingsFixture];

export const findingHandlers = [
  // GET /api/v1/findings — với filter
  http.get(ENDPOINTS.findings.list, ({ request }) => {
    const url = new URL(request.url);
    const severity = url.searchParams.getAll('severity');
    const status   = url.searchParams.getAll('status');
    const productId = url.searchParams.get('product_id');
    const q        = url.searchParams.get('q');
    const slaStatus= url.searchParams.get('sla_status');
    const page     = Number(url.searchParams.get('page') || '1');
    const pageSize = Number(url.searchParams.get('page_size') || '20');

    let results = [...findings];
    if (severity.length)  results = results.filter(f => severity.includes(f.severity));
    if (status.length)    results = results.filter(f => status.includes(f.status));
    if (productId)        results = results.filter(f => f.product_id === productId);
    if (slaStatus)        results = results.filter(f => f.sla_status === slaStatus);
    if (q) {
      const qLower = q.toLowerCase();
      results = results.filter(f =>
        f.title.toLowerCase().includes(qLower) ||
        (f.cve_id?.toLowerCase().includes(qLower))
      );
    }

    const paginated = results.slice((page - 1) * pageSize, page * pageSize);
    return HttpResponse.json({ findings: paginated, total: results.length, page, page_size: pageSize });
  }),

  // GET /api/v1/findings/stats
  http.get(ENDPOINTS.findings.stats, () => {
    return HttpResponse.json(findingStatsFixture);
  }),

  // GET /api/v1/findings/:id
  http.get('/api/v1/findings/:id', ({ params }) => {
    const finding = findings.find(f => f.id === params.id);
    if (!finding) {
      return HttpResponse.json(
        { error: 'NOT_FOUND', message: `Finding ${params.id} not found`, details: {}, trace_id: 'mock-f-001' },
        { status: 404 }
      );
    }
    return HttpResponse.json(finding);
  }),

  // PATCH /api/v1/findings/:id
  http.patch('/api/v1/findings/:id', async ({ params, request }) => {
    const body = await request.json() as Partial<Finding & { status: FindingStatus }>;
    const idx = findings.findIndex(f => f.id === params.id);
    if (idx === -1) return HttpResponse.json({ error: 'NOT_FOUND' }, { status: 404 });

    const current = findings[idx];

    // Validate state transition
    if (body.status && !canTransition(current.status, body.status)) {
      return HttpResponse.json(
        {
          error: 'INVALID_STATE_TRANSITION',
          message: `Cannot transition from '${current.status}' to '${body.status}'`,
          details: {},
          trace_id: 'mock-f-002',
        },
        { status: 409 }
      );
    }

    findings[idx] = {
      ...current,
      ...body,
      updated_at: new Date().toISOString(),
    };
    return HttpResponse.json(findings[idx]);
  }),

  // GET /api/v1/findings/:id/notes
  http.get('/api/v1/findings/:id/notes', () => {
    return HttpResponse.json({
      notes: [
        { id: 'note_001', author_id: 'usr_bob123', author_name: 'Bob Smith',
          content: 'Triaged with security team. Patching scheduled for next sprint.',
          created_at: '2026-06-12T09:00:00Z', updated_at: '2026-06-12T09:00:00Z' },
      ],
      total: 1,
    });
  }),

  // POST /api/v1/findings/:id/notes
  http.post('/api/v1/findings/:id/notes', async ({ request }) => {
    const body = await request.json() as { content: string };
    return HttpResponse.json({
      id: 'note_' + Date.now(), author_id: 'usr_bob123', author_name: 'Bob Smith',
      content: body.content, created_at: new Date().toISOString(), updated_at: new Date().toISOString(),
    }, { status: 201 });
  }),

  // GET /api/v1/findings/:id/audit
  http.get('/api/v1/findings/:id/audit', () => {
    return HttpResponse.json({
      events: [
        { id: 'aud_001', user_id: 'usr_bob123', user_name: 'Bob Smith',
          action: 'finding.status_changed', from_value: null, to_value: 'active',
          note: null, timestamp: '2026-06-01T08:00:00Z' },
        { id: 'aud_002', user_id: 'usr_admin001', user_name: 'Admin User',
          action: 'finding.assigned', from_value: null, to_value: 'bob@company.com',
          note: null, timestamp: '2026-06-05T10:00:00Z' },
      ],
      total: 2,
    });
  }),

  // POST /api/v1/findings/bulk/close
  http.post(ENDPOINTS.findings.bulkClose, async ({ request }) => {
    const body = await request.json() as { finding_ids: string[]; status: string };
    // Cập nhật state
    body.finding_ids.forEach(id => {
      const idx = findings.findIndex(f => f.id === id);
      if (idx >= 0) {
        findings[idx] = { ...findings[idx], status: body.status as FindingStatus, updated_at: new Date().toISOString() };
      }
    });
    return HttpResponse.json({ processed: body.finding_ids.length, failed: 0, failed_ids: [] });
  }),

  // POST /api/v1/findings/bulk/reopen
  http.post(ENDPOINTS.findings.bulkReopen, async ({ request }) => {
    const body = await request.json() as { finding_ids: string[] };
    body.finding_ids.forEach(id => {
      const idx = findings.findIndex(f => f.id === id);
      if (idx >= 0) {
        findings[idx] = { ...findings[idx], status: 'active', updated_at: new Date().toISOString() };
      }
    });
    return HttpResponse.json({ processed: body.finding_ids.length, failed: 0 });
  }),

  // POST /api/v1/findings/bulk/assign
  http.post(ENDPOINTS.findings.bulkAssign, async ({ request }) => {
    const body = await request.json() as { finding_ids: string[]; assigned_to: string };
    body.finding_ids.forEach(id => {
      const idx = findings.findIndex(f => f.id === id);
      if (idx >= 0) {
        findings[idx] = { ...findings[idx], assigned_to: body.assigned_to, updated_at: new Date().toISOString() };
      }
    });
    return HttpResponse.json({ processed: body.finding_ids.length });
  }),

  // Risk Acceptances
  http.post(ENDPOINTS.riskAcceptances.create, async ({ request }) => {
    const body = await request.json() as any;
    const ra = {
      id: 'ra_' + Date.now(), finding_id: body.finding_id,
      accepted_by: 'bob@company.com', accepted_by_name: 'Bob Smith',
      reason: body.reason, expires_at: body.expires_at ?? null,
      created_at: new Date().toISOString(),
    };
    // Link to finding
    const idx = findings.findIndex(f => f.id === body.finding_id);
    if (idx >= 0) {
      findings[idx] = {
        ...findings[idx], status: 'risk_accepted', risk_acceptance: ra,
        updated_at: new Date().toISOString(),
      };
    }
    return HttpResponse.json(ra, { status: 201 });
  }),

  http.delete('/api/v1/risk-acceptances/:id', ({ params }) => {
    // Remove risk acceptance, reopen finding
    const idx = findings.findIndex(f => f.risk_acceptance?.id === params.id);
    if (idx >= 0) {
      findings[idx] = { ...findings[idx], status: 'active', risk_acceptance: null };
    }
    return HttpResponse.json({ success: true, reverted_to_status: 'active' });
  }),
];
```

---

## Verification

```bash
cd ui/
VITE_ENABLE_MSW=true pnpm dev

# 1. /findings → list 30 findings
# 2. Filter Critical severity → chỉ Critical
# 3. Filter SLA Breached → F-2847 hiện (days_overdue: 7)
# 4. PATCH F-2847: active → mitigated → OK
# 5. PATCH F-2847: mitigated → duplicate → 409 INVALID_STATE_TRANSITION
# 6. Bulk close 3 findings → success toast
# 7. Risk Acceptance: tạo cho F-2848 → status = risk_accepted

npx tsc --noEmit
# Expected: no errors
```

---

## Checklist

- [ ] `mocks/fixtures/finding.fixture.ts` — 5+ fixtures đủ diversity (critical/high/medium, active/mitigated/fp/ra)
- [ ] Fixture có `sla_status: 'breached'` (F-2847), `'at_risk'` (F-2848), `'ok'` (F-2849)
- [ ] `mocks/handlers/finding.handlers.ts` — 12 handlers
- [ ] List handler: filter severity, status, product_id, sla_status, q, pagination
- [ ] PATCH handler: validate `canTransition()` → 409 nếu invalid
- [ ] PATCH handler: cập nhật in-memory state (stateful mock)
- [ ] Bulk handlers: cập nhật in-memory state
- [ ] Notes handlers: GET + POST
- [ ] Risk acceptance handlers: POST create + DELETE revoke
- [ ] `npx tsc --noEmit` không lỗi
