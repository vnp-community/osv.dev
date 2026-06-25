/**
 * Integration Handlers — MSW mock for API keys & webhooks endpoints.
 * Updated: TASK-P3-03 — API keys handler returns APIKeysResponse format for useAPIKeys()
 */
import { http, HttpResponse, delay } from 'msw';
import { ENDPOINTS } from '@/shared/api/endpoints';
import { apiKeysFixture } from '../fixtures/api-keys.fixture';

const BASE = '';

// ─── Mutable stores ───────────────────────────────────────────────────────────
let apiKeysStore = apiKeysFixture.map((k) => ({ ...k }));

const webhooksStore = [
  {
    id: 'wh_001',
    url: 'https://hooks.slack.com/services/T000/B000/xxxx',
    events: ['kev.new', 'finding.sla.breached', 'scan.completed'],
    is_active: true,
    secret_preview: 'sha256:a1b2c3...',
    created_at: '2026-06-01T00:00:00Z',
    last_delivery_at: new Date(Date.now() - 3_600_000).toISOString(),
    last_delivery_status: 'success' as const,
  },
  {
    id: 'wh_002',
    url: 'https://hooks.pagerduty.com/integration/xxx/enqueue',
    events: ['finding.sla.breached'],
    is_active: true,
    secret_preview: 'sha256:d4e5f6...',
    created_at: '2026-05-15T00:00:00Z',
    last_delivery_at: new Date(Date.now() - 86_400_000).toISOString(),
    last_delivery_status: 'success' as const,
  },
];

export const integrationHandlers = [
  // ─── API Keys ───────────────────────────────────────────────────────────────

  // GET /api/v1/api-keys → APIKeysResponse
  http.get(`${BASE}${ENDPOINTS.apiKeys.list}`, async () => {
    await delay(350);
    return HttpResponse.json({
      items: apiKeysStore,
      total: apiKeysStore.length,
    });
  }),

  // POST /api/v1/api-keys → CreateAPIKeyResponse
  http.post(`${BASE}${ENDPOINTS.apiKeys.create}`, async ({ request }) => {
    await delay(500);
    const body = await request.json() as { name: string; scopes: string[]; expires_at?: string };

    // Generate a fake but realistic-looking prefix
    const chars = 'abcdefghijklmnopqrstuvwxyz0123456789';
    const randomSuffix = Array.from({ length: 8 }, () => chars[Math.floor(Math.random() * chars.length)]).join('');
    const prefix = `osv_prod_${randomSuffix.slice(0, 4)}`;

    // NEVER expose real random key gen via Math.random in prod — this is MSW mock only
    const fakeKey = `${prefix}_MOCK_${randomSuffix.toUpperCase()}`;

    const newKey = {
      id: `k-${Date.now()}`,
      name: body.name,
      prefix,
      scopes: body.scopes,
      created_at: new Date().toISOString(),
      last_used_at: undefined,
      expires_at: body.expires_at,
      status: 'active' as const,
      created_by: 'carol@company.com',
    };

    apiKeysStore.push(newKey);

    return HttpResponse.json(
      {
        api_key: newKey,
        plain_key: fakeKey,
      },
      { status: 201 }
    );
  }),

  // DELETE /api/v1/api-keys/:id — revoke key
  http.delete(`${BASE}${ENDPOINTS.apiKeys.revoke(':id')}`, async ({ params }) => {
    await delay(300);
    const idx = apiKeysStore.findIndex((k) => k.id === params.id);
    if (idx > -1) {
      apiKeysStore[idx] = { ...apiKeysStore[idx], status: 'revoked' };
    }
    return new HttpResponse(null, { status: 204 });
  }),

  // ─── Webhooks ───────────────────────────────────────────────────────────────

  // GET /api/v1/webhooks
  http.get(`${BASE}${ENDPOINTS.webhooks.list}`, async () => {
    await delay(300);
    return HttpResponse.json({
      webhooks: webhooksStore,
      total: webhooksStore.length,
    });
  }),

  // POST /api/v1/webhooks
  http.post(`${BASE}${ENDPOINTS.webhooks.create}`, async ({ request }) => {
    await delay(400);
    const body = await request.json() as { url: string; events: string[]; secret?: string };
    const newHook = {
      id: `wh_${Date.now()}`,
      url: body.url,
      events: body.events,
      is_active: true,
      secret_preview: 'sha256:new...',
      created_at: new Date().toISOString(),
      last_delivery_at: null,
      last_delivery_status: null,
    };
    webhooksStore.push(newHook as unknown as typeof webhooksStore[0]);
    return HttpResponse.json(newHook, { status: 201 });
  }),

  // DELETE /api/v1/webhooks/:id
  http.delete(`${BASE}${ENDPOINTS.webhooks.delete(':id')}`, async ({ params }) => {
    await delay(300);
    const idx = webhooksStore.findIndex((w) => w.id === params.id);
    if (idx > -1) webhooksStore.splice(idx, 1);
    return HttpResponse.json({ success: true });
  }),

  // POST /api/v1/webhooks/:id/test
  http.post(`${BASE}${ENDPOINTS.webhooks.test(':id')}`, async ({ params }) => {
    await delay(500);
    return HttpResponse.json({
      delivery_id: `dlv_test_${Date.now()}`,
      webhook_id: params.id,
      status: 'success',
      response_code: 200,
      response_time_ms: 245,
    });
  }),

  // GET /api/v1/webhooks/deliveries — TASK-P4-03: delivery log
  http.get(`${BASE}/api/v1/webhooks/deliveries`, ({ request }) => {
    const url = new URL(request.url);
    const webhookId = url.searchParams.get('webhook_id');

    const deliveries = [
      {
        id: 'DEL-0441', webhook_id: 'wh-1', event: 'finding.created',
        endpoint: 'siem.company.com', status: 'success', response_time: 124,
        status_code: 200, time: new Date(Date.now() - 120_000).toISOString(),
      },
      {
        id: 'DEL-0440', webhook_id: 'wh-1', event: 'scan.completed',
        endpoint: 'siem.company.com', status: 'success', response_time: 89,
        status_code: 200, time: new Date(Date.now() - 7_200_000).toISOString(),
      },
      {
        id: 'DEL-0439', webhook_id: 'wh-2', event: 'sla.breached',
        endpoint: 'jira.company.com', status: 'failed', response_time: 5001,
        status_code: 503, time: new Date(Date.now() - 10_800_000).toISOString(),
      },
      {
        id: 'DEL-0438', webhook_id: 'wh-1', event: 'kev.alert',
        endpoint: 'slack.company.com', status: 'success', response_time: 201,
        status_code: 200, time: new Date(Date.now() - 14_400_000).toISOString(),
      },
    ];

    const filtered = webhookId
      ? deliveries.filter((d) => d.webhook_id === webhookId)
      : deliveries;

    return HttpResponse.json({ deliveries: filtered, total: filtered.length });
  }),

  // GET /api/v1/webhooks/stats/hourly — stable chart data
  http.get(`${BASE}/api/v1/webhooks/stats/hourly`, () => {
    return HttpResponse.json([
      { h: '06:00', success: 42, failed: 1 },
      { h: '09:00', success: 87, failed: 2 },
      { h: '12:00', success: 65, failed: 0 },
      { h: '15:00', success: 93, failed: 3 },
      { h: '18:00', success: 78, failed: 1 },
      { h: '21:00', success: 45, failed: 0 },
    ]);
  }),

  // POST /api/v1/webhooks/deliveries/:id/retry
  http.post(`${BASE}/api/v1/webhooks/deliveries/:id/retry`, ({ params }) => {
    return HttpResponse.json({
      delivery_id: params.id,
      status: 'retried',
      queued_at: new Date().toISOString(),
    });
  }),
];

