import { http, HttpResponse } from 'msw';
import type { Asset } from '@/shared/types/scan';

const BASE = '';

const assetsFixture: Asset[] = [
  {
    id: 'asset-001',
    ip: '10.0.1.45',
    hostname: 'prod-web-01',
    os: 'Ubuntu 22.04',
    services: [
      { port: 80, protocol: 'tcp', service: 'http', version: 'nginx/1.24', cveIds: [] },
      { port: 443, protocol: 'tcp', service: 'https', version: 'nginx/1.24', cveIds: ['CVE-2025-44228'] },
      { port: 22, protocol: 'tcp', service: 'ssh', version: 'OpenSSH_8.9', cveIds: [] },
    ],
    webTechnologies: ['nginx', 'React', 'Node.js'],
    tags: ['production', 'web'],
    riskScore: 9.8,
    activeFindingCount: 12,
    firstSeenAt: '2025-01-15T00:00:00Z',
    lastSeenAt: '2026-06-14T08:30:00Z',
    lastScanId: 'SC-0047',
  },
  {
    id: 'asset-002',
    ip: '10.0.1.60',
    hostname: 'api-gw-prod-01',
    os: 'Amazon Linux 2',
    services: [
      { port: 8080, protocol: 'tcp', service: 'http-alt', version: 'Go/1.21', cveIds: [] },
    ],
    webTechnologies: ['Go', 'gRPC'],
    tags: ['production', 'api'],
    riskScore: 7.5,
    activeFindingCount: 5,
    firstSeenAt: '2025-02-01T00:00:00Z',
    lastSeenAt: '2026-06-14T09:00:00Z',
  },
];

export const assetHandlers = [
  http.get(`${BASE}/api/v1/assets`, () => {
    return HttpResponse.json({ assets: assetsFixture, total: assetsFixture.length });
  }),

  http.get(`${BASE}/api/v1/assets/:id`, ({ params }) => {
    const asset = assetsFixture.find((a) => a.id === params.id);
    if (!asset) return new HttpResponse(null, { status: 404 });
    return HttpResponse.json(asset);
  }),
];

