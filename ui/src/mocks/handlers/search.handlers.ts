/**
 * search.handlers.ts — MSW handlers cho /api/v1/search/* và semantic suggestions
 * Phục vụ CommandPalette.tsx, SemanticSearch.tsx
 */
import { http, HttpResponse } from 'msw';

const BASE = '';

const recentSearches = [
  'CVE-2025-44228',
  'webserver01.prod findings',
  'Banking App security report',
  'log4j vulnerabilities',
];

const suggestedSearches = [
  'Critical findings due this week',
  'Assets with KEV vulnerabilities',
  'Products with grade below B',
  'Scans completed today',
];

const ALL_RESULTS = [
  { type: 'cve',     id: 'CVE-2025-44228',     name: 'CVE-2025-44228',     desc: 'Apache Log4j2 JNDI Remote Code Execution — CVSS 10.0',    updated: '2h ago',  badge: 'CRITICAL' },
  { type: 'cve',     id: 'CVE-2025-22965',     name: 'CVE-2025-22965',     desc: 'Spring Framework Path Traversal — CVSS 9.8',               updated: '4h ago',  badge: 'CRITICAL' },
  { type: 'finding', id: 'F-2847',             name: 'F-2847',             desc: 'Log4Shell detected on webserver01.prod',                   updated: '1d ago',  badge: 'OPEN' },
  { type: 'asset',   id: 'webserver01.prod',   name: 'webserver01.prod',   desc: 'Ubuntu 22.04 — Risk Score 9.8',                           updated: '30m ago', badge: null },
  { type: 'scan',    id: 'SC-0044',            name: 'SC-0044',            desc: 'Nmap full scan — 47 findings',                             updated: '1d ago',  badge: 'COMPLETED' },
];

const semanticSuggestions = [
  'critical remote code execution vulnerabilities',
  'authentication bypass in web frameworks',
  'memory corruption vulnerabilities in C/C++',
  'supply chain attack patterns',
  'privilege escalation on Linux kernel',
];

export const searchHandlers = [
  // GET /api/v1/search/recent
  http.get(`${BASE}/api/v1/search/recent`, () => {
    return HttpResponse.json({
      items: recentSearches,
    });
  }),

  // GET /api/v1/search/suggested
  http.get(`${BASE}/api/v1/search/suggested`, () => {
    return HttpResponse.json({
      items: suggestedSearches,
    });
  }),

  // GET /api/v2/cves/search/semantic/suggestions
  http.get(`${BASE}/api/v2/cves/search/semantic/suggestions`, ({ request }) => {
    const url = new URL(request.url);
    const q = url.searchParams.get('q')?.toLowerCase() ?? '';

    const filtered = q
      ? semanticSuggestions.filter(s => s.toLowerCase().includes(q))
      : semanticSuggestions;

    return HttpResponse.json({
      suggestions: filtered,
    });
  }),

  // GET /api/v1/search?q=... — universal search
  http.get(`${BASE}/api/v1/search`, ({ request }) => {
    const url = new URL(request.url);
    const q = url.searchParams.get('q')?.toLowerCase() ?? '';

    const results = q
      ? ALL_RESULTS.filter(r =>
          r.name.toLowerCase().includes(q) ||
          r.desc.toLowerCase().includes(q)
        )
      : ALL_RESULTS;

    return HttpResponse.json({
      items: results,
      total: results.length,
    });
  }),
];
