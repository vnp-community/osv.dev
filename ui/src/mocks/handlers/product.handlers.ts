import { http, HttpResponse, delay } from 'msw';
import { ENDPOINTS } from '@/shared/api/endpoints';

const BASE = '';

const productsFixture = [
  {
    id: "prod_1",
    name: "Banking Portal",
    description: "Customer-facing online banking application",
    type: "web_app",
    criticality: "critical",
    lifecycle: "production",
    grade: "D",
    score: 42,
    finding_summary: {
      critical: 2,
      high: 8,
      medium: 15,
      low: 20,
      total_active: 45
    },
    sla_config: {
      product_id: "prod_1",
      critical_days: 3,
      high_days: 14,
      medium_days: 60,
      low_days: 120
    },
    tags: ["banking", "pci-dss", "production"],
    created_at: "2026-01-15T08:00:00Z"
  },
  {
    id: "prod_2",
    name: "Mobile App",
    description: "iOS and Android mobile banking app",
    type: "mobile",
    criticality: "high",
    lifecycle: "production",
    grade: "B",
    score: 76,
    finding_summary: {
      critical: 0,
      high: 4,
      medium: 12,
      low: 18,
      total_active: 34
    },
    sla_config: null,
    tags: ["mobile", "ios", "android"],
    created_at: "2026-02-10T08:00:00Z"
  }
];

const engagementsFixture = [
  {
    id: "eng_001",
    product_id: "prod_1",
    name: "Q2 2026 Security Assessment",
    type: "interactive",
    start_date: "2026-04-01",
    end_date: "2026-04-30",
    status: "completed",
    lead_id: "usr_bob123",
    cicd_url: null,
    test_count: 3,
    finding_count: 15
  },
  {
    id: "eng_002",
    product_id: "prod_1",
    name: "CI/CD Pipeline Integration",
    type: "cicd",
    start_date: "2026-01-15",
    end_date: null,
    status: "in_progress",
    lead_id: null,
    cicd_url: "https://github.com/company/banking-portal/actions",
    test_count: 150,
    finding_count: 30
  }
];

const testsFixture = [
  {
    id: "test_001",
    engagement_id: "eng_001",
    title: "Nmap Network Scan - Q2",
    scan_type: "nmap_full",
    test_date: "2026-04-15",
    finding_count: 8
  }
];

export const productHandlers = [
  http.get(`${BASE}${ENDPOINTS.products.list}`, async ({ request }) => {
    await delay(300);
    return HttpResponse.json({
      products: productsFixture,
      total: productsFixture.length,
      page: 1,
      page_size: 20,
    });
  }),

  http.post(`${BASE}${ENDPOINTS.products.create}`, async ({ request }) => {
    await delay(400);
    const body = await request.json() as any;
    const newProduct = {
      id: `prod_${Date.now()}`,
      ...body,
      grade: 'A',
      score: 100,
      finding_summary: { critical: 0, high: 0, medium: 0, low: 0, total_active: 0 },
      created_at: new Date().toISOString(),
    };
    productsFixture.push(newProduct);
    return HttpResponse.json(newProduct, { status: 201 });
  }),

  http.get(`${BASE}/api/v1/products/grades`, async () => {
    await delay(300);
    return HttpResponse.json({
      products: productsFixture.map(p => ({
        id: p.id,
        name: p.name,
        grade: p.grade,
        score: p.score,
        critical_count: p.finding_summary.critical,
        high_count: p.finding_summary.high,
        trend: p.grade === 'A' || p.grade === 'B' ? 'improving' : 'worsening'
      })).sort((a, b) => a.score - b.score), // worst first
      overall_grade: "C",
      overall_score: 58
    });
  }),

  http.get(`${BASE}/api/v1/products/types`, async () => {
    return HttpResponse.json({
      types: [
        { value: "web_app", label: "Web Application" },
        { value: "api", label: "API" },
        { value: "infrastructure", "label": "Infrastructure" },
        { value: "mobile", "label": "Mobile" }
      ]
    });
  }),

  http.get(`${BASE}${ENDPOINTS.products.detail(':id')}`, async ({ params }) => {
    await delay(300);
    const p = productsFixture.find(p => p.id === params.id);
    if (!p) return new HttpResponse(null, { status: 404 });
    const engs = engagementsFixture.filter(e => e.product_id === params.id);
    return HttpResponse.json({
      ...p,
      engagements: engs
    });
  }),

  http.patch(`${BASE}${ENDPOINTS.products.patch(':id')}`, async ({ params, request }) => {
    await delay(400);
    const body = await request.json() as any;
    const idx = productsFixture.findIndex(p => p.id === params.id);
    if (idx === -1) return new HttpResponse(null, { status: 404 });
    productsFixture[idx] = { ...productsFixture[idx], ...body };
    return HttpResponse.json(productsFixture[idx]);
  }),

  http.get(`${BASE}${ENDPOINTS.products.engagements(':id')}`, async ({ params }) => {
    await delay(300);
    const engs = engagementsFixture.filter(e => e.product_id === params.id);
    return HttpResponse.json({
      engagements: engs,
      total: engs.length
    });
  }),

  http.post(`${BASE}${ENDPOINTS.products.engagements(':id')}`, async ({ params, request }) => {
    await delay(400);
    const body = await request.json() as any;
    const newEng = {
      id: `eng_${Date.now()}`,
      product_id: params.id,
      ...body,
      status: "not_started",
      test_count: 0,
      finding_count: 0
    };
    engagementsFixture.push(newEng);
    return HttpResponse.json(newEng, { status: 201 });
  }),

  http.get(`${BASE}${ENDPOINTS.engagements.tests(':id')}`, async ({ params }) => {
    await delay(300);
    const tests = testsFixture.filter(t => t.engagement_id === params.id);
    return HttpResponse.json({
      tests,
      total: tests.length
    });
  })
];
