import { http, HttpResponse, delay } from 'msw';
import { ENDPOINTS } from '@/shared/api/endpoints';

const BASE = '';

const reportsFixture = [
  {
    id: "rpt_001",
    product_id: "prod_1",
    product_name: "Banking Portal",
    engagement_id: null,
    format: "pdf",
    status: "completed",
    exit_code: 1,
    min_severity: "High",
    min_score: 7.0,
    finding_count: 10,
    generated_at: "2026-06-16T11:00:00Z",
    artifact_url: "https://storage.company.com/reports/rpt_001.pdf",
    expires_at: "2026-07-16T11:00:00Z",
    created_at: "2026-06-16T10:58:00Z",
    created_by: "carol@company.com"
  }
] as any[];

export const reportHandlers = [
  http.get(`${BASE}${ENDPOINTS.reports.list}`, async () => {
    await delay(300);
    return HttpResponse.json({
      reports: reportsFixture,
      total: reportsFixture.length,
      last_generated_at: reportsFixture[0]?.generated_at ?? reportsFixture[0]?.created_at,
      page: 1,
      page_size: 20
    });
  }),

  // GET /api/v1/reports/templates — TASK-P4-04: report templates from server
  http.get(`${BASE}${ENDPOINTS.reports.list}/templates`, async () => {
    await delay(200);
    return HttpResponse.json({
      templates: [
        {
          id: 'tmpl_exec',
          name: 'Executive Summary',
          description: 'High-level overview for C-level and board presentations',
          type: 'Executive',
        },
        {
          id: 'tmpl_tech',
          name: 'Technical Report',
          description: 'Detailed findings with CVE details, evidence, and remediation steps',
          type: 'Technical',
        },
        {
          id: 'tmpl_comp',
          name: 'Compliance Report',
          description: 'Mapped to PCI DSS, ISO 27001, SOC2, NIST frameworks',
          type: 'Compliance',
        },
      ],
    });
  }),

  http.post(`${BASE}${ENDPOINTS.reports.create}`, async ({ request }) => {
    await delay(400);
    const body = await request.json() as any;
    const newReport = {
      id: `rpt_${Date.now()}`,
      product_id: body.product_id,
      product_name: "Mock Product",
      engagement_id: body.engagement_id || null,
      format: body.format || "pdf",
      status: "pending",
      exit_code: 0,
      min_severity: body.min_severity || "High",
      min_score: body.min_score || 7.0,
      finding_count: 0,
      generated_at: null,
      artifact_url: null,
      expires_at: null,
      created_at: new Date().toISOString(),
      created_by: "user@example.com"
    };
    reportsFixture.unshift(newReport);
    return HttpResponse.json(newReport, { status: 202 });
  }),

  http.get(`${BASE}${ENDPOINTS.reports.detail(':id')}`, async ({ params }) => {
    await delay(300);
    const rpt = reportsFixture.find(r => r.id === params.id);
    if (!rpt) return new HttpResponse(null, { status: 404 });
    return HttpResponse.json(rpt);
  }),

  http.delete(`${BASE}${ENDPOINTS.reports.delete(':id')}`, async ({ params }) => {
    await delay(300);
    const idx = reportsFixture.findIndex(r => r.id === params.id);
    if (idx > -1) reportsFixture.splice(idx, 1);
    return HttpResponse.json({ success: true });
  }),

  http.get(`${BASE}${ENDPOINTS.reports.download(':id')}`, async ({ params }) => {
    await delay(300);
    // Return a dummy blob for download
    const blob = new Blob(["Mock PDF Content"], { type: "application/pdf" });
    return new HttpResponse(blob, {
      headers: {
        'Content-Type': 'application/pdf',
        'Content-Disposition': `attachment; filename="report-${params.id}.pdf"`
      }
    });
  })
];
