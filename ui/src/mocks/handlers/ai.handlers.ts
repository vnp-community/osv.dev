/**
 * AI Handlers — MSW mock for AI triage & enrichment endpoints.
 * Updated: TASK-P3-02 — triage queue returns AITriageQueueResponse format
 */
import { http, HttpResponse, delay } from 'msw';
import { ENDPOINTS } from '@/shared/api/endpoints';

const BASE = '';

// ─── Mutable triage queue store (so accept/reject persists) ───────────────────
let triageStore = [
  {
    id: 'AT-001',
    finding_id: 'F-2847',
    title: 'Apache Log4j2 JNDI RCE',
    verdict: 'Patch Immediately' as const,
    confidence: 98,
    severity: 'Critical' as const,
    created_at: new Date(Date.now() - 10 * 60_000).toISOString(),
    status: 'pending' as const,
    reasoning: 'CVSS 10.0, EPSS 98.2%, active CISA KEV. High confidence real vulnerability requiring immediate patching.',
    suggested_fixes: [
      'Upgrade Log4j2 to 2.17.1+',
      'Set log4j2.formatMsgNoLookups=true',
      'Implement WAF rules blocking JNDI lookup patterns',
    ],
  },
  {
    id: 'AT-002',
    finding_id: 'F-2846',
    title: 'Spring Framework Path Traversal',
    verdict: 'Patch Immediately' as const,
    confidence: 95,
    severity: 'Critical' as const,
    created_at: new Date(Date.now() - 15 * 60_000).toISOString(),
    status: 'pending' as const,
    reasoning: 'CVSS 9.8, confirmed exploitation in the wild via Spring4Shell attack patterns detected in logs.',
    suggested_fixes: [
      'Upgrade Spring Framework to 5.3.18+',
      'Restrict file access patterns via SecurityConfig',
      'Update Tomcat to 10.0.20+',
    ],
  },
  {
    id: 'AT-003',
    finding_id: 'F-2840',
    title: 'Redis No Authentication',
    verdict: 'Configure Auth' as const,
    confidence: 87,
    severity: 'Medium' as const,
    created_at: new Date(Date.now() - 60 * 60_000).toISOString(),
    status: 'accepted' as const,
    reasoning: 'Redis instance accessible without authentication on internal network. Low exploitation probability but significant risk if network is compromised.',
    suggested_fixes: [
      'Enable Redis AUTH with strong password',
      'Bind Redis to specific internal IP only',
      'Enable TLS for Redis connections',
    ],
  },
  {
    id: 'AT-004',
    finding_id: 'F-2839',
    title: 'WordPress XSS Comment Handler',
    verdict: 'False Positive' as const,
    confidence: 92,
    severity: 'Low' as const,
    created_at: new Date(Date.now() - 2 * 60 * 60_000).toISOString(),
    status: 'rejected' as const,
    reasoning: 'WordPress instance uses custom sanitization that prevents XSS. Scanner triggered on benign HTML encoding. False positive confirmed.',
    suggested_fixes: [],
  },
  {
    id: 'AT-005',
    finding_id: 'F-2844',
    title: 'nginx HTTP/2 DoS',
    verdict: 'Schedule Patch' as const,
    confidence: 78,
    severity: 'High' as const,
    created_at: new Date(Date.now() - 3 * 60 * 60_000).toISOString(),
    status: 'pending' as const,
    reasoning: 'CVSS 7.5. Requires specific HTTP/2 configuration to be exploitable. Recommend patching in next maintenance window.',
    suggested_fixes: [
      'Update nginx to 1.25.3+',
      'Configure HTTP/2 rate limiting rules',
    ],
  },
  {
    id: 'AT-006',
    finding_id: 'F-2841',
    title: 'K8s API Server Exposure',
    verdict: 'Accept Risk' as const,
    confidence: 71,
    severity: 'High' as const,
    created_at: new Date(Date.now() - 4 * 60 * 60_000).toISOString(),
    status: 'pending' as const,
    reasoning: 'K8s API accessible only from internal VPN. Risk is mitigated by network controls. Consider accepting risk with quarterly review.',
    suggested_fixes: [
      'Apply NetworkPolicy to restrict API server access',
      'Enable RBAC audit logging',
    ],
  },
];

export const aiHandlers = [
  // POST /api/v1/ai/triage/:id — trigger AI triage for a finding
  http.post(`${BASE}${ENDPOINTS.ai.triage(':id')}`, async () => {
    await delay(2000); // simulate AI processing
    return HttpResponse.json({
      finding_id: 'F-2847',
      verdict: 'Patch Immediately',
      confidence: 94,
      reasoning: 'Simulated AI triage response.',
      suggested_fixes: ['Apply vendor patch', 'Enable WAF rule'],
      generated_at: new Date().toISOString(),
      ai_provider: 'ollama',
    });
  }),

  // GET /api/v1/ai/triage/queue — AITriageQueueResponse
  http.get(`${BASE}${ENDPOINTS.ai.triageQueue}`, async ({ request }) => {
    await delay(350);
    const url = new URL(request.url);
    const statusFilter = url.searchParams.get('status');

    const items = statusFilter
      ? triageStore.filter((i) => i.status === statusFilter)
      : triageStore;

    const pendingCount = triageStore.filter((i) => i.status === 'pending').length;
    const avgConf = Math.round(
      triageStore.reduce((sum, i) => sum + i.confidence, 0) / triageStore.length
    );

    return HttpResponse.json({
      items,
      total: items.length,
      pending_count: pendingCount,
      accepted_today: 8,
      avg_confidence: avgConf,
    });
  }),

  // POST /api/v1/ai/triage/:findingId/review — accept or reject recommendation
  http.post(`${BASE}${ENDPOINTS.ai.triageReview(':id')}`, async ({ request, params }) => {
    await delay(300);
    const body = await request.json() as { decision: 'accepted' | 'rejected'; note?: string };
    const item = triageStore.find((i) => i.finding_id === params.id);
    if (item) {
      (item as { status: string }).status = body.decision;
    }
    return HttpResponse.json({
      finding_id: params.id,
      decision: body.decision,
      note: body.note ?? null,
      reviewed_by: 'admin@osv.local',
      reviewed_at: new Date().toISOString(),
    });
  }),

  // GET /api/v1/ai/enrichment
  http.get(`${BASE}${ENDPOINTS.ai.enrichment}`, async () => {
    await delay(300);
    return HttpResponse.json({
      stats: {
        total_cves: 312450,
        with_embedding: 298000,
        embedding_coverage_pct: 95.4,
        last_enrichment_run: '2026-06-16T06:00:00Z',
        semantic_search_accuracy: 0.82,
      },
      recent_enrichments: [
        {
          cve_id: 'CVE-2026-12345',
          has_embedding: true,
          embedding_dims: 1536,
          is_cached: true,
          ai_severity: 'Critical',
          ai_provider: 'ollama',
          enriched_at: '2026-06-16T06:00:00Z',
        },
      ],
      total: 298000,
    });
  }),

  // POST /api/v1/ai/enrichment/trigger
  http.post(`${BASE}${ENDPOINTS.ai.enrichTrigger}`, async () => {
    await delay(300);
    return HttpResponse.json(
      { job_id: `enrich_job_${Date.now()}`, status: 'queued', cve_count: 2 },
      { status: 202 }
    );
  }),

  // GET /api/v1/ai/enrichment/:id
  http.get(`${BASE}${ENDPOINTS.ai.enrichByCve(':id')}`, async ({ params }) => {
    await delay(300);
    return HttpResponse.json({
      cve_id: params.id,
      has_embedding: true,
      embedding_dims: 1536,
      is_cached: true,
      cache_ttl_seconds: 432000,
      ai_severity: {
        severity: 'Critical',
        confidence: 0.98,
        reasoning: 'CVSS 10.0, actively exploited',
        source: 'cvss_v3',
      },
      ai_provider: 'ollama',
      enriched_at: '2026-06-16T06:00:00Z',
    });
  }),
];
