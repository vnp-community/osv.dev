import { http, HttpResponse } from 'msw';
import { findingsFixture } from '../fixtures/findings.fixture';
import { riskAcceptancesFixture } from '../fixtures/risk-acceptances.fixture';
import type { FindingStatus } from '@/shared/types/finding';

const BASE = '';

// ─── Audit entries fixture ─────────────────────────────────────────────────
const auditEntriesFixture = [
  {
    id: 'aud_002',
    finding_id: 'F-2847',
    action: 'status_changed',
    before_state: { status: 'active' },
    after_state: { status: 'mitigated' },
    user_id: 'usr_bob123',
    user_name: 'Bob Smith',
    comment: 'Confirmed fixed in patch 2.15.0',
    timestamp: new Date(Date.now() - 3600000).toISOString(),
  },
  {
    id: 'aud_001',
    finding_id: 'F-2847',
    action: 'assigned',
    before_state: { assigned_to: null },
    after_state: { assigned_to: 'bob@company.com' },
    user_id: 'usr_alice456',
    user_name: 'Alice Wu',
    comment: 'Assigning to Bob for remediation',
    timestamp: new Date(Date.now() - 7200000).toISOString(),
  },
  {
    id: 'aud_000',
    finding_id: 'F-2847',
    action: 'created',
    before_state: null,
    after_state: { status: 'active', severity: 'Critical' },
    user_id: 'usr_system',
    user_name: 'System',
    comment: 'Imported from nmap_xml scan SC-0047',
    timestamp: new Date(Date.now() - 18000000).toISOString(),
  },
];

// ─── Risk Acceptances mutable store (import from fixture) ────────────────────
let raStore = riskAcceptancesFixture.map((a) => ({
  ...a,
  // Normalize to RiskAcceptance API shape expected by useRiskAcceptances
  finding_id: a.finding_ids[0] ?? '',
  owner: a.approved_by,
  status: (a.is_expired ? 'expired' : 'approved') as 'approved' | 'pending' | 'rejected' | 'expired',
}));

// ─── SLA config fixture ────────────────────────────────────────────────────
const slaConfigFixture = {
  global: {
    critical_days: 7,
    high_days: 30,
    medium_days: 90,
    low_days: 180,
  },
  product_overrides: [
    {
      product_id: 'p1',
      product_name: 'Banking App',
      critical_days: 3,
      high_days: 14,
      medium_days: 60,
      low_days: 120,
    },
    {
      product_id: 'p3',
      product_name: 'API Gateway',
      critical_days: 5,
      high_days: 21,
      medium_days: 75,
      low_days: 150,
    },
  ],
};

// ─── System audit log fixture ──────────────────────────────────────────────
const systemAuditFixture = [
  {
    id: 'aud_system_003',
    user_id: 'usr_bob123',
    user_name: 'Bob Smith',
    action: 'finding.status_changed',
    entity_type: 'finding',
    entity_id: 'F-2847',
    ip_address: '10.0.0.1',
    user_agent: 'Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7)',
    result: 'success',
    metadata: { from: 'active', to: 'mitigated' },
    timestamp: new Date(Date.now() - 3600000).toISOString(),
  },
  {
    id: 'aud_system_002',
    user_id: 'usr_alice456',
    user_name: 'Alice Wu',
    action: 'scan.created',
    entity_type: 'scan',
    entity_id: 'SC-0048',
    ip_address: '10.0.0.2',
    user_agent: 'Mozilla/5.0 (Windows NT 10.0; Win64; x64)',
    result: 'success',
    metadata: { type: 'zap', targets: ['https://api.internal'] },
    timestamp: new Date(Date.now() - 7200000).toISOString(),
  },
  {
    id: 'aud_system_001',
    user_id: 'usr_carol789',
    user_name: 'Carol Admin',
    action: 'risk_acceptance.created',
    entity_type: 'risk_acceptance',
    entity_id: 'ra_001',
    ip_address: '10.0.0.3',
    user_agent: 'Mozilla/5.0',
    result: 'success',
    metadata: { finding_ids: ['F-2800', 'F-2801'] },
    timestamp: new Date(Date.now() - 86400000).toISOString(),
  },
  {
    id: 'aud_system_000',
    user_id: 'usr_system',
    user_name: 'System',
    action: 'kev.new_entries',
    entity_type: 'system',
    entity_id: 'CVE-2025-44228',
    ip_address: '127.0.0.1',
    user_agent: 'osv-data-service/1.0',
    result: 'success',
    metadata: { count: 3 },
    timestamp: new Date(Date.now() - 172800000).toISOString(),
  },
];

// ─── Notes fixture ─────────────────────────────────────────────────────────
const notesFixture: Record<string, Array<{ id: string; finding_id: string; content: string; created_by: string; created_at: string }>> = {
  'F-2847': [
    {
      id: 'note_001',
      finding_id: 'F-2847',
      content: 'Confirmed via manual testing — JNDI lookup succeeds against attacker-controlled LDAP server.',
      created_by: 'bob@company.com',
      created_at: new Date(Date.now() - 3600000).toISOString(),
    },
  ],
};

export const findingHandlers = [
  // GET /api/v1/findings — list findings with advanced filtering
  http.get(`${BASE}/api/v1/findings`, ({ request }) => {
    const url = new URL(request.url);
    const statusParam = url.searchParams.getAll('status');
    const severityParam = url.searchParams.getAll('severity');
    const slaStatus = url.searchParams.get('sla_status');
    const isKev = url.searchParams.get('is_kev');
    const q = url.searchParams.get('q')?.toLowerCase();
    const productId = url.searchParams.get('product_id');
    const sortBy = url.searchParams.get('sort_by') ?? 'severity_desc';

    let findings = [...findingsFixture];

    if (statusParam.length) findings = findings.filter((f) => statusParam.includes(f.status));
    if (severityParam.length) findings = findings.filter((f) => severityParam.includes(f.severity));
    if (slaStatus) findings = findings.filter((f) => f.slaStatus === slaStatus);
    if (isKev === 'true') findings = findings.filter((f) => f.isKEV);
    if (isKev === 'false') findings = findings.filter((f) => !f.isKEV);
    if (productId) findings = findings.filter((f) => f.productId === productId);
    if (q) findings = findings.filter((f) =>
      f.title.toLowerCase().includes(q) ||
      (f.cveId?.toLowerCase().includes(q) ?? false)
    );

    // Sort
    if (sortBy === 'severity_desc') {
      const order = ['Critical', 'High', 'Medium', 'Low', 'Info'];
      findings.sort((a, b) => order.indexOf(a.severity) - order.indexOf(b.severity));
    } else if (sortBy === 'sla_asc') {
      findings.sort((a, b) => (a.slaDaysLeft ?? 999) - (b.slaDaysLeft ?? 999));
    } else if (sortBy === 'epss_desc') {
      findings.sort((a, b) => (b.epssScore ?? 0) - (a.epssScore ?? 0));
    }

    const page = Number(url.searchParams.get('page') ?? '1');
    const pageSize = Number(url.searchParams.get('page_size') ?? '50');
    const start = (page - 1) * pageSize;
    const paginated = findings.slice(start, start + pageSize);

    const bySeverity = findings.reduce((acc, f) => {
      acc[f.severity] = (acc[f.severity] ?? 0) + 1;
      return acc;
    }, {} as Record<string, number>);

    const byStatus = findings.reduce((acc, f) => {
      acc[f.status] = (acc[f.status] ?? 0) + 1;
      return acc;
    }, {} as Record<string, number>);

    return HttpResponse.json({
      findings: paginated,
      total: findings.length,
      page,
      page_size: pageSize,
      by_severity: bySeverity,
      by_status: byStatus,
      sla_stats: {
        breached: findings.filter((f) => f.slaStatus === 'breached').length,
        at_risk: findings.filter((f) => f.slaStatus === 'at_risk').length,
        ok: findings.filter((f) => f.slaStatus === 'ok').length,
      },
    });
  }),

  // GET /api/v1/findings/stats
  http.get(`${BASE}/api/v1/findings/stats`, ({ request }) => {
    const url = new URL(request.url);
    const productId = url.searchParams.get('product_id');
    let findings = [...findingsFixture];
    if (productId) findings = findings.filter((f) => f.productId === productId);

    return HttpResponse.json({
      total_active: findings.filter((f) => f.status === 'active').length,
      by_severity: {
        Critical: findings.filter((f) => f.severity === 'Critical').length,
        High: findings.filter((f) => f.severity === 'High').length,
        Medium: findings.filter((f) => f.severity === 'Medium').length,
        Low: findings.filter((f) => f.severity === 'Low').length,
      },
      by_status: {
        active: findings.filter((f) => f.status === 'active').length,
        mitigated: findings.filter((f) => f.status === 'mitigated').length,
        false_positive: findings.filter((f) => f.status === 'false_positive').length,
        risk_accepted: findings.filter((f) => f.status === 'risk_accepted').length,
        out_of_scope: findings.filter((f) => f.status === 'out_of_scope').length,
        duplicate: findings.filter((f) => f.status === 'duplicate').length,
      },
      sla_stats: {
        breached: findings.filter((f) => f.slaStatus === 'breached').length,
        at_risk: findings.filter((f) => f.slaStatus === 'at_risk').length,
        ok: findings.filter((f) => f.slaStatus === 'ok').length,
      },
      new_today: 5,
    });
  }),

  // GET /api/v1/findings/:id — finding detail
  http.get(`${BASE}/api/v1/findings/:id`, ({ params }) => {
    // Skip sub-paths like stats, bulk/close, bulk/reopen
    const reserved = ['stats', 'bulk'];
    if (reserved.some((r) => (params.id as string).startsWith(r))) return;

    const finding = findingsFixture.find((f) => f.id === params.id);
    if (!finding) return new HttpResponse(null, { status: 404 });
    return HttpResponse.json({
      ...finding,
      remediation_guidance: 'Update the affected component to the latest patched version per vendor advisory.',
      proof_of_concept: null,
      notes: notesFixture[finding.id] ?? [],
      tags: ['imported', 'auto-triaged'],
      hash_code: 'sha256:abc123def456',
      import_source: 'nmap_xml',
    });
  }),

  // PATCH /api/v1/findings/:id — update finding status
  http.patch(`${BASE}/api/v1/findings/:id`, async ({ params, request }) => {
    const body = await request.json() as { status?: FindingStatus; comment?: string; assigned_to?: string; vex_justification?: string };
    const finding = findingsFixture.find((f) => f.id === params.id);
    if (!finding) return new HttpResponse(null, { status: 404 });

    // Simple state machine validation
    const invalidTransitions: Partial<Record<FindingStatus, FindingStatus[]>> = {
      duplicate: ['mitigated', 'false_positive'],
      out_of_scope: ['duplicate'],
    };
    if (body.status && invalidTransitions[finding.status]?.includes(body.status)) {
      return HttpResponse.json(
        {
          error: 'INVALID_TRANSITION',
          message: `Cannot transition from '${finding.status}' to '${body.status}'`,
          valid_transitions: [],
        },
        { status: 409 }
      );
    }

    const updated = { ...finding, ...body, updated_at: new Date().toISOString() };
    if (body.status === 'mitigated') {
      (updated as Record<string, unknown>).mitigated_at = new Date().toISOString();
    }
    return HttpResponse.json(updated);
  }),

  // POST /api/v1/findings/bulk/close
  http.post(`${BASE}/api/v1/findings/bulk/close`, async ({ request }) => {
    const body = await request.json() as { finding_ids: string[]; comment?: string };
    return HttpResponse.json({
      success_count: body.finding_ids.length,
      failed_ids: [],
      errors: [],
    });
  }),

  // POST /api/v1/findings/bulk/reopen
  http.post(`${BASE}/api/v1/findings/bulk/reopen`, async ({ request }) => {
    const body = await request.json() as { finding_ids: string[]; comment?: string };
    return HttpResponse.json({
      success_count: body.finding_ids.length,
      failed_ids: [],
      errors: [],
    });
  }),

  // POST /api/v1/findings/bulk/assign
  http.post(`${BASE}/api/v1/findings/bulk/assign`, async ({ request }) => {
    const body = await request.json() as { finding_ids: string[]; assigned_to: string };
    return HttpResponse.json({
      success_count: body.finding_ids.length,
      failed_ids: [],
      errors: [],
    });
  }),

  // GET /api/v1/findings/:id/audit — audit trail
  http.get(`${BASE}/api/v1/findings/:id/audit`, ({ params }) => {
    const entries = auditEntriesFixture.map((e) => ({ ...e, finding_id: params.id }));
    return HttpResponse.json({ audits: entries });
  }),

  // GET /api/v1/findings/:id/notes — get notes
  http.get(`${BASE}/api/v1/findings/:id/notes`, ({ params }) => {
    return HttpResponse.json({
      notes: notesFixture[params.id as string] ?? [],
    });
  }),

  // POST /api/v1/findings/:id/notes — add note/comment
  http.post(`${BASE}/api/v1/findings/:id/notes`, async ({ params, request }) => {
    const body = await request.json() as { content: string };
    const newNote = {
      id: `note_${Date.now()}`,
      finding_id: params.id as string,
      content: body.content,
      created_by: 'admin@osv.local',
      created_at: new Date().toISOString(),
    };
    // Add to in-memory store
    if (!notesFixture[params.id as string]) {
      notesFixture[params.id as string] = [];
    }
    notesFixture[params.id as string].push(newNote);
    return HttpResponse.json(newNote, { status: 201 });
  }),

  // GET /api/v1/risk-acceptances — returns {items, total} for useRiskAcceptances
  http.get(`${BASE}/api/v1/risk-acceptances`, ({ request }) => {
    const url = new URL(request.url);
    const statusFilter = url.searchParams.get('status');
    const productId = url.searchParams.get('product_id');
    const page = Number(url.searchParams.get('page') ?? '1');
    const pageSize = Number(url.searchParams.get('page_size') ?? '20');

    let items = [...raStore];
    if (statusFilter) items = items.filter((a) => a.status === statusFilter);
    if (productId) items = items.filter((a) => a.product_id === productId);

    const start = (page - 1) * pageSize;
    const paginated = items.slice(start, start + pageSize);

    return HttpResponse.json({ items: paginated, total: items.length });
  }),

  // PATCH /api/v1/risk-acceptances/:id — approve/reject
  http.patch(`${BASE}/api/v1/risk-acceptances/:id`, async ({ params, request }) => {
    const body = await request.json() as { status: string };
    const item = raStore.find((a) => a.id === params.id);
    if (!item) return new HttpResponse(null, { status: 404 });
    (item as { status: string }).status = body.status;
    return HttpResponse.json(item);
  }),

  // POST /api/v1/risk-acceptances — create
  http.post(`${BASE}/api/v1/risk-acceptances`, async ({ request }) => {
    const body = await request.json() as {
      finding_id: string;
      reason: string;
      expiration_date: string;
      retest_date?: string;
    };
    const newRA = {
      id: `RA-${Date.now()}`,
      finding_id: body.finding_id,
      finding_title: 'New Risk Acceptance',
      product_id: 'p-1',
      product_name: 'Unknown Product',
      reason: body.reason,
      expiration_date: body.expiration_date,
      retest_date: body.retest_date,
      approved_by: 'carol@company.com',
      owner: 'carol@company.com',
      status: 'pending' as const,
      severity: 'Medium' as const,
      days_left: 30,
      finding_ids: [body.finding_id],
      is_expired: false,
      created_at: new Date().toISOString(),
    };
    raStore.push(newRA as typeof raStore[0]);
    return HttpResponse.json(newRA, { status: 201 });
  }),

  // DELETE /api/v1/risk-acceptances/:id — revoke
  http.delete(`${BASE}/api/v1/risk-acceptances/:id`, ({ params }) => {
    const idx = raStore.findIndex((a) => a.id === params.id);
    if (idx === -1) return new HttpResponse(null, { status: 404 });
    const ra = raStore[idx];
    raStore.splice(idx, 1);
    return HttpResponse.json({ success: true, finding_id: ra.finding_id });
  }),

  // GET /api/v1/sla/config — SLA configuration
  http.get(`${BASE}/api/v1/sla/config`, () => {
    return HttpResponse.json(slaConfigFixture);
  }),

  // PUT /api/v1/sla/config — update SLA configuration (admin only)
  http.put(`${BASE}/api/v1/sla/config`, async ({ request }) => {
    const body = await request.json() as typeof slaConfigFixture;
    return HttpResponse.json({ ...slaConfigFixture, ...body });
  }),

  // GET /api/v1/audit-log — system-wide audit log (admin scope)
  http.get(`${BASE}/api/v1/audit-log`, ({ request }) => {
    const url = new URL(request.url);
    const userId = url.searchParams.get('user_id');
    const action = url.searchParams.get('action');
    const entityType = url.searchParams.get('entity_type');
    const page = Number(url.searchParams.get('page') ?? '1');
    const pageSize = Number(url.searchParams.get('page_size') ?? '50');

    let events = [...systemAuditFixture];
    if (userId) events = events.filter((e) => e.user_id === userId);
    if (action) events = events.filter((e) => e.action.includes(action));
    if (entityType) events = events.filter((e) => e.entity_type === entityType);

    const start = (page - 1) * pageSize;
    const paginated = events.slice(start, start + pageSize);

    return HttpResponse.json({
      events: paginated,
      total: events.length,
      page,
      page_size: pageSize,
    });
  }),
];
