import { http, HttpResponse } from 'msw';
import { scansFixture } from '../fixtures/scans.fixture';
import type { ScanProgress } from '@/shared/types/scan';

const BASE = '';

// ─── Mock Nmap results fixture ─────────────────────────────────────────────
const nmapResultsFixture = {
  'SC-0047': {
    scan_id: 'SC-0047',
    hosts: [
      {
        ip: '10.0.0.45',
        hostname: 'prod-web-01.internal',
        os: 'Linux 5.4.0',
        state: 'up',
        ports: [
          { port: 443, protocol: 'tcp', state: 'open', service: 'https', version: 'nginx 1.24.0', cve_ids: ['CVE-2025-44228', 'CVE-2024-56789'] },
          { port: 22, protocol: 'tcp', state: 'open', service: 'ssh', version: 'OpenSSH 8.9', cve_ids: [] },
          { port: 80, protocol: 'tcp', state: 'open', service: 'http', version: 'nginx 1.24.0', cve_ids: [] },
        ],
        cve_ids: ['CVE-2025-44228', 'CVE-2024-56789'],
        risk_score: 10.0,
      },
      {
        ip: '10.0.0.60',
        hostname: 'prod-api-01.internal',
        os: 'Ubuntu 22.04',
        state: 'up',
        ports: [
          { port: 8080, protocol: 'tcp', state: 'open', service: 'http-alt', version: 'Spring Boot 2.7.0', cve_ids: ['CVE-2025-22965'] },
          { port: 22, protocol: 'tcp', state: 'open', service: 'ssh', version: 'OpenSSH 8.9', cve_ids: [] },
        ],
        cve_ids: ['CVE-2025-22965'],
        risk_score: 9.8,
      },
      {
        ip: '10.0.0.12',
        hostname: 'db-primary.internal',
        os: 'Debian 11',
        state: 'up',
        ports: [
          { port: 5432, protocol: 'tcp', state: 'open', service: 'postgresql', version: 'PostgreSQL 14.5', cve_ids: [] },
        ],
        cve_ids: [],
        risk_score: 2.5,
      },
    ],
    total_hosts: 254,
    hosts_up: 87,
    total_findings: 47,
  },
};

// ─── Mock ZAP results fixture ──────────────────────────────────────────────
const zapResultsFixture = {
  'SC-0048': {
    scan_id: 'SC-0048',
    target_url: 'https://api.internal',
    alerts: [
      {
        id: 'zap_001',
        name: 'SQL Injection',
        risk: 'High',
        confidence: 'High',
        url: 'https://api.internal/users?id=1',
        description: 'SQL injection may be possible. The page results changed when a known SQL injection payload was submitted.',
        solution: 'Do not trust client side input, even if there is client side validation in place. Use parameterized queries.',
        evidence: "select",
        cwe_id: 'CWE-89',
        references: ['https://owasp.org/www-community/attacks/SQL_Injection'],
      },
      {
        id: 'zap_002',
        name: 'Cross Site Scripting (Reflected)',
        risk: 'High',
        confidence: 'Medium',
        url: 'https://api.internal/search?q=test',
        description: 'Cross-site Scripting (XSS) - Reflected',
        solution: 'Phase: Architecture and Design. Use a vetted library or framework that does not allow this weakness to occur or provides constructs that make this weakness easier to avoid.',
        evidence: '<script>alert(1)</script>',
        cwe_id: 'CWE-79',
        references: ['https://owasp.org/www-community/attacks/xss/'],
      },
      {
        id: 'zap_003',
        name: 'Content Security Policy (CSP) Header Not Set',
        risk: 'Medium',
        confidence: 'High',
        url: 'https://api.internal/',
        description: 'Content Security Policy (CSP) is an added layer of security that helps to detect and mitigate certain types of attacks.',
        solution: "Ensure that your web server, application server, load balancer, etc. is configured to set the Content-Security-Policy header.",
        evidence: '',
        cwe_id: 'CWE-693',
        references: ['https://developer.mozilla.org/en-US/docs/Web/HTTP/CSP'],
      },
      {
        id: 'zap_004',
        name: 'Server Leaks Information via "X-Powered-By" HTTP Response Header',
        risk: 'Low',
        confidence: 'Medium',
        url: 'https://api.internal/api/users',
        description: 'The web/application server is leaking information via one or more "X-Powered-By" HTTP response headers.',
        solution: 'Ensure that your web server, application server, load balancer, etc. is configured to suppress "X-Powered-By" headers.',
        evidence: 'Express',
        cwe_id: 'CWE-200',
        references: ['http://blogs.msdn.com/b/david.wang/archive/2005/05/04/HOWTO-Detect-ASP-NET-and-ASP-Frames.aspx'],
      },
    ],
    total: 4,
    by_risk: { High: 2, Medium: 1, Low: 1, Informational: 0 },
  },
};

// ─── Mock scheduled scans fixture ─────────────────────────────────────────
const scheduledScansFixture = [
  {
    id: 'sched_001',
    name: 'Daily Production Scan',
    type: 'nmap_full',
    targets: ['10.0.0.0/24'],
    frequency: 'daily',
    cron_expr: '0 2 * * *',
    next_run_at: new Date(Date.now() + 86400000).toISOString(),
    last_run_at: new Date(Date.now() - 3600000).toISOString(),
    enabled: true,
  },
  {
    id: 'sched_002',
    name: 'Weekly API Security Scan',
    type: 'zap',
    targets: ['https://api.internal', 'https://admin.internal'],
    frequency: 'weekly',
    cron_expr: '0 3 * * 0',
    next_run_at: new Date(Date.now() + 7 * 86400000).toISOString(),
    last_run_at: new Date(Date.now() - 7 * 86400000).toISOString(),
    enabled: true,
  },
  {
    id: 'sched_003',
    name: 'Monthly Full Network Discovery',
    type: 'nmap_discovery',
    targets: ['10.0.0.0/8'],
    frequency: 'custom',
    cron_expr: '0 4 1 * *',
    next_run_at: new Date(Date.now() + 14 * 86400000).toISOString(),
    last_run_at: new Date(Date.now() - 17 * 86400000).toISOString(),
    enabled: false,
  },
];

export const scanHandlers = [
  // GET /api/v1/scans — list scans with stats
  http.get(`${BASE}/api/v1/scans`, ({ request }) => {
    const url = new URL(request.url);
    const status = url.searchParams.get('status');
    const type = url.searchParams.get('type');

    let scans = [...scansFixture];
    if (status) scans = scans.filter((s) => status.split(',').includes(s.status));
    if (type) scans = scans.filter((s) => type.split(',').includes(s.type));

    const page = Number(url.searchParams.get('page') ?? '1');
    const pageSize = Number(url.searchParams.get('page_size') ?? '20');
    const start = (page - 1) * pageSize;
    const paginated = scans.slice(start, start + pageSize);

    return HttpResponse.json({
      scans: paginated,
      total: scans.length,
      page,
      page_size: pageSize,
      stats: {
        active_scans: scans.filter((s) => s.status === 'running').length,
        completed_today: scans.filter((s) => s.status === 'completed').length,
        total_findings_today: scans.reduce((acc, s) => acc + s.findingCount, 0),
        scheduled_scans: scheduledScansFixture.filter((s) => s.enabled).length,
      },
    });
  }),

  // GET /api/v1/scans/history — scan history list
  http.get(`${BASE}/api/v1/scans/history`, ({ request }) => {
    const url = new URL(request.url);
    const page = Number(url.searchParams.get('page') ?? '1');
    const limit = Number(url.searchParams.get('limit') ?? '20');
    const start = (page - 1) * limit;
    const items = scansFixture
      .filter(s => s.status === 'completed' || s.status === 'failed')
      .slice(start, start + limit);
    return HttpResponse.json({
      items,
      total: scansFixture.length,
      page,
      limit,
    });
  }),

  // GET /api/v1/scans/:id
  http.get(`${BASE}/api/v1/scans/:id`, ({ params }) => {
    // Don't match sub-paths like /scans/scheduled or /scans/import
    if (['scheduled', 'import'].includes(params.id as string)) return;
    const scan = scansFixture.find((s) => s.id === params.id);
    if (!scan) return new HttpResponse(null, { status: 404 });
    return HttpResponse.json(scan);
  }),

  // POST /api/v1/scans — create and start new scan
  http.post(`${BASE}/api/v1/scans`, async ({ request }) => {
    const body = await request.json() as {
      name: string;
      type: string;
      targets: string[];
      options?: Record<string, unknown>;
      engagement_id?: string;
      schedule_frequency?: string;
    };

    // Validate targets format
    if (!body.targets || body.targets.length === 0) {
      return HttpResponse.json(
        { error: 'VALIDATION_ERROR', message: 'targets is required and must not be empty' },
        { status: 400 }
      );
    }

    const newScan = {
      id: `SC-${Date.now()}`,
      name: body.name,
      type: body.type,
      status: 'queued' as const,
      targets: body.targets,
      progress: 0,
      finding_count: 0,
      started_at: null,
      completed_at: null,
      created_by: 'admin@osv.local',
      engagement_id: body.engagement_id ?? null,
    };
    return HttpResponse.json(newScan, { status: 201 });
  }),

  // SSE mock — scan progress simulation
  http.get(`${BASE}/api/v1/scans/:id/stream`, ({ params }) => {
    const encoder = new TextEncoder();
    let progress = 0;

    const stream = new ReadableStream({
      async start(controller) {
        // Send initial status
        const initial: ScanProgress = {
          scanId: params.id as string,
          status: 'running',
          progress: 0,
          findingsFound: 0,
          message: 'Initializing scan...',
        };
        controller.enqueue(encoder.encode(`event: message\ndata: ${JSON.stringify(initial)}\n\n`));

        while (progress < 100) {
          await new Promise((r) => setTimeout(r, 600));
          progress = Math.min(progress + Math.floor(Math.random() * 15) + 5, 100);

          const data: ScanProgress = {
            scanId: params.id as string,
            status: progress < 100 ? 'running' : 'completed',
            progress,
            findingsFound: Math.floor(progress * 0.47),
            currentTarget: `10.0.0.${Math.floor(progress * 2.5)}`,
            message: `Scanning targets... ${progress}%`,
          };

          controller.enqueue(
            encoder.encode(`event: message\ndata: ${JSON.stringify(data)}\n\n`)
          );
        }

        // Send done event
        controller.enqueue(
          encoder.encode(`event: done\ndata: ${JSON.stringify({ scan_id: params.id, status: 'completed', progress: 100, findings_found: 47 })}\n\n`)
        );
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

  // POST /api/v1/scans/:id/cancel
  http.post(`${BASE}/api/v1/scans/:id/cancel`, ({ params }) => {
    const scan = scansFixture.find((s) => s.id === params.id);
    if (!scan) return new HttpResponse(null, { status: 404 });

    // Only running/queued scans can be cancelled
    if (scan.status === 'completed' || scan.status === 'failed') {
      return HttpResponse.json(
        { error: 'INVALID_STATE', message: `Scan is already ${scan.status}` },
        { status: 409 }
      );
    }
    return HttpResponse.json({ success: true, scan_id: params.id, status: 'cancelled' });
  }),

  // GET /api/v1/scans/:id/results/nmap
  http.get(`${BASE}/api/v1/scans/:id/results/nmap`, ({ params }) => {
    const results = nmapResultsFixture[params.id as keyof typeof nmapResultsFixture]
      ?? {
        scan_id: params.id,
        hosts: [],
        total_hosts: 0,
        hosts_up: 0,
        total_findings: 0,
      };
    return HttpResponse.json(results);
  }),

  // GET /api/v1/scans/:id/results/zap
  http.get(`${BASE}/api/v1/scans/:id/results/zap`, ({ params }) => {
    const results = zapResultsFixture[params.id as keyof typeof zapResultsFixture]
      ?? {
        scan_id: params.id,
        target_url: '',
        alerts: [],
        total: 0,
        by_risk: { High: 0, Medium: 0, Low: 0, Informational: 0 },
      };
    return HttpResponse.json(results);
  }),

  // GET /api/v1/scans/scheduled — list scheduled scans
  http.get(`${BASE}/api/v1/scans/scheduled`, () => {
    return HttpResponse.json({
      scheduled_scans: scheduledScansFixture,
      total: scheduledScansFixture.length,
    });
  }),

  // POST /api/v1/scans/import — import scan report (multipart)
  http.post(`${BASE}/api/v1/scans/import`, async () => {
    // Simulate async processing
    await new Promise((r) => setTimeout(r, 200));
    return HttpResponse.json(
      {
        import_id: `imp_${Date.now()}`,
        status: 'processing',
        findings_count: null,
      },
      { status: 201 }
    );
  }),

  // GET /api/v1/scans/stats — TASK-P4-02: stable KPI data (no Math.random())
  http.get(`${BASE}/api/v1/scans/stats`, () => {
    const running = scansFixture.filter((s) => s.status === 'running').length;
    const completed = scansFixture.filter((s) => s.status === 'completed').length;
    const totalFindings = scansFixture.reduce((sum, s) => sum + (s.findingCount ?? 0), 0);
    const scheduled = scansFixture.filter((s) => s.status === 'queued').length;
    return HttpResponse.json({
      active_scans: running,
      completed_today: completed,
      total_findings: totalFindings,
      scheduled_scans: scheduled,
      failed_today: scansFixture.filter((s) => s.status === 'failed').length,
    });
  }),

  // GET /api/v1/scans/stats/weekly — stable weekly chart data
  http.get(`${BASE}/api/v1/scans/stats/weekly`, () => {
    return HttpResponse.json([
      { day: 'Mon', scans: 3,  findings: 42 },
      { day: 'Tue', scans: 5,  findings: 78 },
      { day: 'Wed', scans: 4,  findings: 55 },
      { day: 'Thu', scans: 7,  findings: 93 },
      { day: 'Fri', scans: 6,  findings: 67 },
      { day: 'Sat', scans: 2,  findings: 28 },
      { day: 'Sun', scans: 1,  findings: 15 },
    ]);
  }),
];

