import { http, HttpResponse } from 'msw';
import { cvesFixture } from '../fixtures/cves.fixture';
import type { Severity } from '@/shared/types/cve';

// ─── EPSS fixture data ─────────────────────────────────────────────────────
const epssOverview = {
  trendData: [
    { date: 'Jan', avg: 38.2, p90: 78.4 }, { date: 'Feb', avg: 39.1, p90: 79.8 },
    { date: 'Mar', avg: 41.3, p90: 82.1 }, { date: 'Apr', avg: 43.7, p90: 85.3 },
    { date: 'May', avg: 45.2, p90: 88.6 }, { date: 'Jun', avg: 47.8, p90: 91.2 },
  ],
  distribution: [
    { range: '0-10%', count: 18240 }, { range: '10-20%', count: 12480 },
    { range: '20-40%', count: 8320 }, { range: '40-60%', count: 4210 },
    { range: '60-80%', count: 2140 }, { range: '80-90%', count: 980 },
    { range: '90-100%', count: 1240 },
  ],
  kpis: { avgEpss: 47.8, highRisk: 2220, criticalEpss: 1240, tracked: 47610 },
  topCVEs: cvesFixture
    .sort((a, b) => (b.epssScore ?? 0) - (a.epssScore ?? 0))
    .slice(0, 10)
    .map(c => ({
      cve: c.id, epss: Math.round((c.epssScore ?? 0) * 1000) / 10,
      cvss: c.cvssV3 ?? 0, severity: c.severity,
    })),
};

// ─── CWE fixture data ──────────────────────────────────────────────────────
const cweFixture = [
  { id: 'CWE-79', name: 'Improper Neutralization of Input During Web Page Generation (XSS)', category: 'Input Validation', impact: 'High', linkedCVEs: 8421, capecCount: 12, description: 'Software does not neutralize user-controllable input before it is placed in output used as a web page.', mitigation: 'Use output encoding, implement CSP headers, validate and sanitize all input data.' },
  { id: 'CWE-89', name: 'Improper Neutralization of Special Elements in SQL Command (SQLi)', category: 'Injection', impact: 'Critical', linkedCVEs: 4231, capecCount: 8, description: 'The software constructs SQL commands using externally-influenced input without neutralizing special elements.', mitigation: 'Use parameterized queries, prepared statements. Apply least privilege.' },
  { id: 'CWE-78', name: 'Improper Neutralization of Special Elements in OS Command (CmdI)', category: 'Injection', impact: 'Critical', linkedCVEs: 2847, capecCount: 6, description: 'The software constructs OS commands using externally-influenced input without properly neutralizing special elements.', mitigation: 'Avoid shell commands when possible. Use parameterized APIs. Apply allowlisting.' },
  { id: 'CWE-22', name: 'Improper Limitation of a Pathname (Path Traversal)', category: 'File Handling', impact: 'High', linkedCVEs: 3102, capecCount: 9, description: 'The software uses external input to construct a pathname without neutralizing ".." sequences.', mitigation: 'Canonicalize paths before validation. Use allowlisting for permitted directories.' },
  { id: 'CWE-120', name: 'Buffer Copy without Checking Size of Input (Classic Buffer Overflow)', category: 'Memory Safety', impact: 'Critical', linkedCVEs: 5678, capecCount: 14, description: 'The program copies an input buffer to an output buffer without verifying input size.', mitigation: 'Use memory-safe languages. Enable ASLR, DEP, stack canaries.' },
  { id: 'CWE-287', name: 'Improper Authentication', category: 'Authentication', impact: 'Critical', linkedCVEs: 2341, capecCount: 11, description: 'The software does not correctly implement authentication, allowing attackers to assume another identity.', mitigation: 'Implement MFA, strong password policies, secure session management.' },
  { id: 'CWE-269', name: 'Improper Privilege Management', category: 'Privilege', impact: 'High', linkedCVEs: 1876, capecCount: 7, description: 'The software does not properly assign, modify, track, or check privileges for an actor.', mitigation: 'Apply principle of least privilege. Regularly audit permissions. Use RBAC.' },
  { id: 'CWE-400', name: 'Uncontrolled Resource Consumption (DoS)', category: 'Resource Management', impact: 'Medium', linkedCVEs: 1432, capecCount: 5, description: 'The software does not properly control the allocation or consumption of resources.', mitigation: 'Implement rate limiting, timeouts, resource quotas, and input size limits.' },
  { id: 'CWE-611', name: 'Improper Restriction of XML External Entity Reference (XXE)', category: 'Input Validation', impact: 'High', linkedCVEs: 987, capecCount: 4, description: 'The software processes XML documents that can contain external entity references.', mitigation: 'Disable external entity processing in XML parsers. Use JSON instead.' },
  { id: 'CWE-502', name: 'Deserialization of Untrusted Data', category: 'Injection', impact: 'Critical', linkedCVEs: 1654, capecCount: 8, description: 'The application deserializes untrusted data without sufficiently verifying the resulting data.', mitigation: 'Avoid deserializing from untrusted sources. Implement integrity checks.' },
];

// ─── Vendor fixture ────────────────────────────────────────────────────────
const vendorFixture = [
  { name: 'Microsoft', logo: 'MS', color: '#00A4EF', totalCVEs: 4821, criticalCVEs: 312, kevCount: 89, avgEPSS: 42.3, trend: 'down', products: 48 },
  { name: 'Apache', logo: 'AP', color: '#D22128', totalCVEs: 2341, criticalCVEs: 187, kevCount: 34, avgEPSS: 61.8, trend: 'up', products: 32 },
  { name: 'Cisco', logo: 'CI', color: '#1BA0D7', totalCVEs: 3102, criticalCVEs: 241, kevCount: 67, avgEPSS: 38.4, trend: 'up', products: 27 },
  { name: 'VMware', logo: 'VM', color: '#607078', totalCVEs: 1847, criticalCVEs: 143, kevCount: 52, avgEPSS: 57.2, trend: 'down', products: 18 },
  { name: 'Oracle', logo: 'OR', color: '#F80000', totalCVEs: 5234, criticalCVEs: 298, kevCount: 41, avgEPSS: 29.7, trend: 'up', products: 62 },
  { name: 'Fortinet', logo: 'FO', color: '#EE3124', totalCVEs: 892, criticalCVEs: 112, kevCount: 28, avgEPSS: 71.4, trend: 'up', products: 12 },
  { name: 'Atlassian', logo: 'AT', color: '#0052CC', totalCVEs: 643, criticalCVEs: 54, kevCount: 18, avgEPSS: 45.9, trend: 'down', products: 8 },
  { name: 'Linux', logo: 'LX', color: '#FCC624', totalCVEs: 8721, criticalCVEs: 521, kevCount: 92, avgEPSS: 33.1, trend: 'up', products: 4 },
  { name: 'OpenSSL', logo: 'OS', color: '#721412', totalCVEs: 412, criticalCVEs: 67, kevCount: 14, avgEPSS: 58.6, trend: 'down', products: 3 },
  { name: 'Ivanti', logo: 'IV', color: '#FF6700', totalCVEs: 318, criticalCVEs: 98, kevCount: 31, avgEPSS: 78.3, trend: 'up', products: 6 },
  { name: 'SolarWinds', logo: 'SW', color: '#00AEF0', totalCVEs: 267, criticalCVEs: 41, kevCount: 22, avgEPSS: 52.1, trend: 'down', products: 7 },
  { name: 'Palo Alto', logo: 'PA', color: '#FA582D', totalCVEs: 534, criticalCVEs: 78, kevCount: 19, avgEPSS: 44.7, trend: 'down', products: 9 },
];

const BASE = '';

export const cveHandlers = [
  // POST /api/v2/cves/search
  http.post(`${BASE}/api/v2/cves/search`, async ({ request }) => {
    const body = await request.json() as {
      query?: string;
      severity?: Severity[];
      kevOnly?: boolean;
      minCvss?: number;
      maxCvss?: number;
      page?: number;
      pageSize?: number;
    };

    let results = [...cvesFixture];

    if (body.severity?.length) {
      results = results.filter((c) => body.severity!.includes(c.severity));
    }
    if (body.query) {
      const q = body.query.toLowerCase();
      results = results.filter(
        (c) =>
          c.id.toLowerCase().includes(q) ||
          c.description.toLowerCase().includes(q) ||
          c.vendor.toLowerCase().includes(q)
      );
    }
    if (body.kevOnly) {
      results = results.filter((c) => c.isKEV);
    }
    if (body.minCvss !== undefined) {
      results = results.filter((c) => (c.cvssV3 ?? 0) >= body.minCvss!);
    }
    if (body.maxCvss !== undefined) {
      results = results.filter((c) => (c.cvssV3 ?? 10) <= body.maxCvss!);
    }

    const page = body.page ?? 1;
    const pageSize = body.pageSize ?? 50;
    const start = (page - 1) * pageSize;
    const paginated = results.slice(start, start + pageSize);

    const bySeverity = results.reduce((acc, c) => {
      acc[c.severity] = (acc[c.severity] ?? 0) + 1;
      return acc;
    }, {} as Record<string, number>);

    const topVendors = Object.entries(
      results.reduce((acc, c) => { acc[c.vendor] = (acc[c.vendor] ?? 0) + 1; return acc; }, {} as Record<string, number>)
    ).map(([vendor, count]) => ({ vendor, count })).sort((a, b) => b.count - a.count).slice(0, 5);

    const byYear = Object.entries(
      results.reduce((acc, c) => {
        const y = new Date(c.publishedAt ?? '2025-01-01').getFullYear();
        acc[y] = (acc[y] ?? 0) + 1; return acc;
      }, {} as Record<number, number>)
    ).map(([year, count]) => ({ year: Number(year), count }));

    return HttpResponse.json({
      data: paginated,
      total: results.length,
      page,
      page_size: pageSize,
      aggregations: { by_severity: bySeverity, top_vendors: topVendors, by_year: byYear },
    });
  }),

  // GET /api/v2/cves/:id
  http.get(`${BASE}/api/v2/cves/:id`, ({ params }) => {
    const cve = cvesFixture.find((c) => c.id === params.id);
    if (!cve) return new HttpResponse(null, { status: 404 });
    return HttpResponse.json(cve);
  }),

  // GET /api/v2/kev
  http.get(`${BASE}/api/v2/kev`, () => {
    const kevEntries = cvesFixture
      .filter((c) => c.isKEV)
      .map((c) => ({
        cveId: c.id,
        vendor: c.vendor,
        product: c.product,
        vulnerabilityName: c.description.substring(0, 60),
        dateAdded: c.publishedAt,
        shortDescription: c.description.substring(0, 120),
        requiredAction: 'Apply vendor patch immediately',
        knownRansomwareCampaignUse: c.epssScore > 0.9,
      }));
    return HttpResponse.json({
      entries: kevEntries,
      total: kevEntries.length,
      stats: {
        total: kevEntries.length,
        ransomwareLinked: kevEntries.filter((k) => k.knownRansomwareCampaignUse).length,
        addedThisWeek: 3,
        unmitigatedInPlatform: 7,
      },
    });
  }),

  // GET /api/v2/kev/stats
  http.get(`${BASE}/api/v2/kev/stats`, () => {
    const kevEntries = cvesFixture.filter((c) => c.isKEV);
    return HttpResponse.json({
      top_vendors: [
        { vendor: 'Microsoft', count: 280, pct: 25.7 },
        { vendor: 'Apache', count: 142, pct: 13.1 },
        { vendor: 'Cisco', count: 98, pct: 9.0 },
      ],
      by_month: [
        { month: '2026-01', count: 28 }, { month: '2026-02', count: 31 },
        { month: '2026-03', count: 22 }, { month: '2026-04', count: 35 },
        { month: '2026-05', count: 29 }, { month: '2026-06', count: 18 },
      ],
      avg_days_to_patch_requirement: 14.2,
      ransomware_pct: 27.8,
    });
  }),

  // POST /api/v2/cves/search/semantic
  http.post(`${BASE}/api/v2/cves/search/semantic`, async ({ request }) => {
    const body = await request.json() as { query: string; limit?: number };
    const limit = body.limit ?? 20;
    const results = cvesFixture.slice(0, limit).map((c) => ({
      ...c,
      similarityScore: Math.random() * 0.3 + 0.7,  // 0.7–1.0
    }));
    return HttpResponse.json({ results, queryEmbeddingMs: 42 });
  }),

  // GET /api/v2/browse (vendors list)
  http.get(`${BASE}/api/v2/browse`, () => {
    return HttpResponse.json({ vendors: vendorFixture, total: vendorFixture.length });
  }),

  // GET /api/v2/epss/overview — EPSS analytics overview
  http.get(`${BASE}/api/v2/epss/overview`, () => {
    return HttpResponse.json(epssOverview);
  }),

  // GET /api/v2/epss/:cveId — single CVE EPSS
  http.get(`${BASE}/api/v2/epss/:cveId`, ({ params }) => {
    const cve = cvesFixture.find((c) => c.id === params.cveId);
    return HttpResponse.json({
      cveId: params.cveId,
      epssScore: cve?.epssScore ?? 0.5,
      epssPercentile: 85,
      trend: epssOverview.trendData.map((d) => ({ date: d.date, score: d.avg / 100 })),
    });
  }),

  // GET /api/v2/cwe — CWE list
  http.get(`${BASE}/api/v2/cwe`, ({ request }) => {
    const url = new URL(request.url);
    const query = url.searchParams.get('q')?.toLowerCase();
    const filtered = query
      ? cweFixture.filter(c => c.id.toLowerCase().includes(query) || c.name.toLowerCase().includes(query))
      : cweFixture;
    return HttpResponse.json({ cweList: filtered, total: filtered.length });
  }),

  // GET /api/v2/cwe/:id
  http.get(`${BASE}/api/v2/cwe/:id`, ({ params }) => {
    const cwe = cweFixture.find((c) => c.id === params.id);
    if (!cwe) return new HttpResponse(null, { status: 404 });
    return HttpResponse.json({ ...cwe, capec_patterns: [{ id: 'CAPEC-86', name: 'XSS', likelihood: 'High', description: 'Cross-site scripting attack.' }], related_cve_count: cwe.linkedCVEs });
  }),

  // GET /api/v2/capec/:id
  http.get(`${BASE}/api/v2/capec/:id`, ({ params }) => {
    return HttpResponse.json({
      id: params.id,
      name: 'Attack Pattern',
      description: 'An attack pattern representing a common vulnerability exploitation approach.',
      likelihood: 'Medium',
      severity: 'High',
      mitigations: ['Use parameterized queries.', 'Apply input validation.'],
      related_cwe_ids: ['CWE-89', 'CWE-79'],
      related_cve_count: 1250,
    });
  }),

  // GET /api/v2/dbinfo — database statistics
  http.get(`${BASE}/api/v2/dbinfo`, () => {
    return HttpResponse.json({
      total_cves: 312450,
      sources: [
        { name: 'NVD', cve_count: 285000, last_sync_at: new Date(Date.now() - 45 * 60000).toISOString(), lag_minutes: 45, status: 'ok' },
        { name: 'JVN', cve_count: 18200, last_sync_at: new Date(Date.now() - 75 * 60000).toISOString(), lag_minutes: 75, status: 'ok' },
        { name: 'CIRCL', cve_count: 9250, last_sync_at: new Date(Date.now() - 420 * 60000).toISOString(), lag_minutes: 420, status: 'ok' },
      ],
    });
  }),

  // GET /api/v2/epss/top — top N CVEs by EPSS score
  http.get(`${BASE}/api/v2/epss/top`, ({ request }) => {
    const url = new URL(request.url);
    const limit = Number(url.searchParams.get('limit') ?? '10');
    const minEpss = Number(url.searchParams.get('min_epss') ?? '0');
    const topCVEs = cvesFixture
      .filter((c) => (c.epssScore ?? 0) >= minEpss)
      .sort((a, b) => (b.epssScore ?? 0) - (a.epssScore ?? 0))
      .slice(0, Math.min(limit, 50))
      .map((c) => ({
        cve_id: c.id,
        epss_score: c.epssScore,
        epss_percentile: Math.round((c.epssScore ?? 0) * 1000) / 10 / 100,
        severity: c.severity,
        vendor: c.vendor,
        product: c.product,
        is_kev: c.isKEV,
      }));
    return HttpResponse.json({ cves: topCVEs, total: topCVEs.length });
  }),

  // GET /api/v2/epss/distribution — EPSS score distribution histogram
  http.get(`${BASE}/api/v2/epss/distribution`, () => {
    return HttpResponse.json({
      buckets: [
        { range: '0-0.1', count: 95000 },
        { range: '0.1-0.2', count: 12000 },
        { range: '0.2-0.5', count: 8500 },
        { range: '0.5-0.9', count: 3200 },
        { range: '0.9-1.0', count: 1800 },
      ],
      total_cves: 120500,
      mean_epss: 0.042,
      median_epss: 0.004,
    });
  }),

  // GET /api/v2/cves/aggregations — aggregation data for charts
  http.get(`${BASE}/api/v2/cves/aggregations`, () => {
    return HttpResponse.json({
      severity_distribution: { Critical: 15234, High: 89432, Medium: 145678, Low: 52890 },
      top_vendors: [
        { vendor: 'Microsoft', count: 28450 },
        { vendor: 'Apache', count: 15280 },
        { vendor: 'Google', count: 12100 },
        { vendor: 'Oracle', count: 9870 },
        { vendor: 'Cisco', count: 8640 },
      ],
      by_year: [
        { year: 2023, count: 28902 },
        { year: 2024, count: 31456 },
        { year: 2025, count: 18234 },
      ],
      epss_distribution: { '0-10': 95000, '10-20': 12000, '20-50': 8000, '50-100': 3000 },
    });
  }),

  // GET /api/v2/vendors — autocomplete vendors
  http.get(`${BASE}/api/v2/vendors`, ({ request }) => {
    const url = new URL(request.url);
    const q = (url.searchParams.get('q') ?? '').toLowerCase();
    const limit = Number(url.searchParams.get('limit') ?? '20');
    const allVendors = ['Microsoft', 'Apache', 'Cisco', 'VMware', 'Oracle', 'Fortinet', 'Atlassian', 'Linux', 'OpenSSL', 'Ivanti', 'SolarWinds', 'Palo Alto', 'Google', 'Apple', 'Adobe'];
    const filtered = q ? allVendors.filter((v) => v.toLowerCase().startsWith(q)) : allVendors;
    return HttpResponse.json({ vendors: filtered.slice(0, limit) });
  }),

  // GET /api/v2/browse/:vendor — products for a vendor
  http.get(`${BASE}/api/v2/browse/:vendor`, ({ params }) => {
    const vendor = params.vendor as string;
    return HttpResponse.json({
      vendor,
      products: [
        { name: 'Log4j2', cve_count: 24, latest_cve_date: '2026-06-10' },
        { name: 'Struts', cve_count: 18, latest_cve_date: '2026-06-08' },
        { name: 'Tomcat', cve_count: 35, latest_cve_date: '2026-06-05' },
      ],
      total: 3,
    });
  }),

  // GET /api/v2/browse/:vendor/:product — CVEs for a product
  http.get(`${BASE}/api/v2/browse/:vendor/:product`, ({ params }) => {
    const relevant = cvesFixture.filter((c) => c.vendor.toLowerCase() === (params.vendor as string).toLowerCase());
    return HttpResponse.json({
      vendor: params.vendor,
      product: params.product,
      cves: relevant.map((c) => ({ id: c.id, severity: c.severity, cvss_v3: c.cvssV3, published_at: c.publishedAt })),
      total: relevant.length,
    });
  }),

  // GET /api/v2/cves/export — bulk export (mock: returns a simple CSV blob)
  http.get(`${BASE}/api/v2/cves/export`, ({ request }) => {
    const url = new URL(request.url);
    const format = url.searchParams.get('format') ?? 'csv';
    if (format === 'csv') {
      const csvData = 'id,severity,cvss_v3,epss_score,vendor,product,published_at\n' +
        cvesFixture.map((c) => `${c.id},${c.severity},${c.cvssV3},${c.epssScore},${c.vendor},${c.product},${c.publishedAt}`).join('\n');
      return new HttpResponse(csvData, {
        headers: {
          'Content-Type': 'text/csv',
          'Content-Disposition': `attachment; filename="cves-export-${new Date().toISOString().split('T')[0]}.csv"`,
        },
      });
    }
    return HttpResponse.json(cvesFixture);
  }),
];

