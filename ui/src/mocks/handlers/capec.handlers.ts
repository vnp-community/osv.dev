/**
 * capec.handlers.ts — MSW handlers cho /api/v2/capec
 * Phục vụ CAPECLibrary.tsx
 */
import { http, HttpResponse } from 'msw';
import { capecPatternsFixture } from '../fixtures/capec.fixture';

const BASE = '';

export const capecHandlers = [
  // GET /api/v2/capec — list all CAPEC patterns (with optional search)
  http.get(`${BASE}/api/v2/capec`, ({ request }) => {
    const url = new URL(request.url);
    const search = url.searchParams.get('search')?.toLowerCase() ?? '';
    const page = Number(url.searchParams.get('page') ?? '1');
    const limit = Number(url.searchParams.get('limit') ?? '50');

    let items = capecPatternsFixture;
    if (search) {
      items = items.filter(p =>
        p.id.toLowerCase().includes(search) ||
        p.name.toLowerCase().includes(search) ||
        p.mechanism.toLowerCase().includes(search)
      );
    }

    const start = (page - 1) * limit;
    const paginated = items.slice(start, start + limit);

    return HttpResponse.json({
      items: paginated,
      total: items.length,
      page,
      limit,
    });
  }),

  // GET /api/v2/capec/:id — single CAPEC detail
  http.get(`${BASE}/api/v2/capec/:id`, ({ params }) => {
    const pattern = capecPatternsFixture.find(p => p.id === params.id);
    if (!pattern) return new HttpResponse(null, { status: 404 });
    return HttpResponse.json(pattern);
  }),
];
