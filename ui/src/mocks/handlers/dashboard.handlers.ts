import { http, HttpResponse } from 'msw';
import { dashboardFixture } from '../fixtures/dashboard.fixture';

const BASE = 'http://localhost:8080';

// ─── SLA Dashboard fixture ─────────────────────────────────────────────────
const slaFixture = {
  summary: {
    total_active_findings: 269,
    compliance_percent: 82.5,
    breached: 3,
    at_risk: 8,
    ok: 258,
  },
  compliance_trend: [
    { month: 'Jan', compliance_percent: 91.0 },
    { month: 'Feb', compliance_percent: 88.5 },
    { month: 'Mar', compliance_percent: 85.2 },
    { month: 'Apr', compliance_percent: 87.1 },
    { month: 'May', compliance_percent: 83.8 },
    { month: 'Jun', compliance_percent: 82.5 },
  ],
  breached_findings: [
    { finding_id: 'F-2846', title: 'Spring Framework RCE', severity: 'Critical', product_name: 'API Gateway', sla_expiration_date: '2026-06-12', days_overdue: 5 },
    { finding_id: 'F-2841', title: 'Kubernetes API Exposure', severity: 'Critical', product_name: 'Banking App', sla_expiration_date: '2026-06-11', days_overdue: 6 },
    { finding_id: 'F-2835', title: 'Redis Unauthorized Access', severity: 'High', product_name: 'Data Pipeline', sla_expiration_date: '2026-06-15', days_overdue: 2 },
  ],
  at_risk_findings: [
    { finding_id: 'F-2847', title: 'Apache Log4j2 RCE', severity: 'Critical', product_name: 'Banking App', sla_expiration_date: '2026-06-18', hours_remaining: 18 },
    { finding_id: 'F-2844', title: 'nginx HTTP/2 Memory Corruption', severity: 'High', product_name: 'Admin Portal', sla_expiration_date: '2026-06-17', hours_remaining: 6 },
  ],
  by_product: [
    { product_id: 'p1', product_name: 'Banking App', compliance_percent: 71.0, breached: 2, at_risk: 3, ok: 12 },
    { product_id: 'p2', product_name: 'Mobile App', compliance_percent: 95.0, breached: 0, at_risk: 1, ok: 24 },
    { product_id: 'p3', product_name: 'API Gateway', compliance_percent: 62.5, breached: 1, at_risk: 4, ok: 18 },
    { product_id: 'p4', product_name: 'Admin Portal', compliance_percent: 88.0, breached: 0, at_risk: 2, ok: 22 },
    { product_id: 'p5', product_name: 'Data Pipeline', compliance_percent: 79.0, breached: 1, at_risk: 1, ok: 15 },
  ],
  total_breached: 3,
  total_at_risk: 8,
  page: 1,
  page_size: 20,
};

export const dashboardHandlers = [
  // GET /api/v1/dashboard — aggregate BFF endpoint
  http.get(`${BASE}/api/v1/dashboard`, ({ request }) => {
    const url = new URL(request.url);
    const period = url.searchParams.get('period') ?? '30d';
    const data = dashboardFixture[period as keyof typeof dashboardFixture]
      ?? dashboardFixture['30d'];
    return HttpResponse.json(data);
  }),

  // Also handle relative URL (for MSW with empty BASE)
  http.get('/api/v1/dashboard', ({ request }) => {
    const url = new URL(request.url);
    const period = url.searchParams.get('period') ?? '30d';
    const data = dashboardFixture[period as keyof typeof dashboardFixture]
      ?? dashboardFixture['30d'];
    return HttpResponse.json(data);
  }),

  // GET /api/v1/dashboard/sla — SLA detail dashboard
  http.get(`${BASE}/api/v1/dashboard/sla`, ({ request }) => {
    const url = new URL(request.url);
    const page = Number(url.searchParams.get('page') ?? '1');
    const pageSize = Number(url.searchParams.get('page_size') ?? '20');
    return HttpResponse.json({ ...slaFixture, page, page_size: pageSize });
  }),

  http.get('/api/v1/dashboard/sla', ({ request }) => {
    const url = new URL(request.url);
    const page = Number(url.searchParams.get('page') ?? '1');
    const pageSize = Number(url.searchParams.get('page_size') ?? '20');
    return HttpResponse.json({ ...slaFixture, page, page_size: pageSize });
  }),

  // GET /api/v1/notifications/stream — SSE mock
  http.get(`${BASE}/api/v1/notifications/stream`, () => {
    const encoder = new TextEncoder();
    let eventCount = 0;

    const notifications = [
      { type: 'finding.sla.breached', title: 'SLA Breached: CVE-2026-12345', severity: 'Critical', entity_id: 'F-2801', timestamp: new Date().toISOString() },
      { type: 'kev.new', title: 'New KEV: CVE-2026-11111 (Apache Struts)', entity_id: 'CVE-2026-11111', timestamp: new Date().toISOString() },
      { type: 'scan.completed', title: 'Scan completed: Weekly Network Scan', entity_id: 'SC-0047', timestamp: new Date().toISOString() },
    ];

    const stream = new ReadableStream({
      async start(controller) {
        // Send initial notification events
        for (const notif of notifications) {
          await new Promise((r) => setTimeout(r, 500));
          controller.enqueue(
            encoder.encode(`event: notification\ndata: ${JSON.stringify(notif)}\n\n`)
          );
          eventCount++;
        }

        // Send ping keep-alive every 30s (mocked as 5s intervals for demo)
        while (eventCount < 20) {
          await new Promise((r) => setTimeout(r, 5000));
          controller.enqueue(
            encoder.encode(`event: ping\ndata: ${JSON.stringify({ ts: new Date().toISOString() })}\n\n`)
          );
          eventCount++;
        }
        controller.close();
      },
    });

    return new HttpResponse(stream, {
      headers: {
        'Content-Type': 'text/event-stream',
        'Cache-Control': 'no-cache',
        'Connection': 'keep-alive',
        'X-Accel-Buffering': 'no',
      },
    });
  }),

  http.get('/api/v1/notifications/stream', () => {
    const encoder = new TextEncoder();
    const notifications = [
      { type: 'finding.sla.breached', title: 'SLA Breached: CVE-2025-44228', severity: 'Critical', entity_id: 'F-2847', timestamp: new Date().toISOString() },
      { type: 'kev.new', title: 'New KEV: CVE-2025-22965 (Spring)', entity_id: 'CVE-2025-22965', timestamp: new Date().toISOString() },
    ];

    let idx = 0;
    const stream = new ReadableStream({
      async start(controller) {
        while (idx < notifications.length) {
          await new Promise((r) => setTimeout(r, 1000));
          controller.enqueue(
            encoder.encode(`event: notification\ndata: ${JSON.stringify(notifications[idx])}\n\n`)
          );
          idx++;
        }
        // Keep alive ping
        await new Promise((r) => setTimeout(r, 30000));
        controller.enqueue(
          encoder.encode(`event: ping\ndata: ${JSON.stringify({ ts: new Date().toISOString() })}\n\n`)
        );
        controller.close();
      },
    });

    return new HttpResponse(stream, {
      headers: {
        'Content-Type': 'text/event-stream',
        'Cache-Control': 'no-cache',
        'Connection': 'keep-alive',
      },
    });
  }),
];
