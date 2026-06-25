# TASK-P4-04 — Fix `ReportCenter.tsx` → `useReports()` + Dynamic Products

**Phase:** 4 — Scan & Report  
**Nguồn giải pháp:** [`solutions/10_11_12_webhook_report_notification.md` — Solution 11](../solutions/10_11_12_webhook_report_notification.md)  
**Ưu tiên:** 🔵 Scan & Report  
**Phụ thuộc:** TASK-P1-03, TASK-P1-04

---

## Vấn đề hiện tại

```typescript
// ❌ HIỆN TẠI — features/reports/components/ReportCenter.tsx
const reports = [
  { id: 'R-047', name: 'Q2 2026 Executive Summary', ... },
  // 5 báo cáo hardcode
];
const templates = [
  { id: 'exec', name: 'Executive Summary', ... },
  // templates không load từ server
];
const productOptions = ['All Products', 'Banking App', 'Mobile App', 'DevOps Platform'];
// ❌ Hardcode — không đồng bộ với products thực tế
// Subtitle "Last report 6h ago" hardcode
```

---

## API Endpoints

```
GET  /api/v1/reports            → Danh sách báo cáo (ENDPOINTS.reports.list)
GET  /api/v1/reports/templates  → Templates
POST /api/v1/reports            → Tạo báo cáo mới (ENDPOINTS.reports.create)
```

---

## Danh sách files cần tạo/sửa

### [NEW] `src/features/reports/hooks/useReports.ts`

```typescript
export interface ReportRun {
  id: string;
  name?: string;
  type: 'Executive' | 'Technical' | 'Compliance';
  format: 'pdf' | 'html' | 'csv' | 'excel' | 'json';
  status: 'pending' | 'generating' | 'completed' | 'failed';
  findingCount?: number;
  fileSizeBytes?: number;
  generatedAt?: string;
  artifactUrl?: string;
  createdAt: string;
  createdBy: string;
}

export interface ReportsResponse {
  reports: ReportRun[];
  total: number;
  lastGeneratedAt?: string;
}

export interface ReportTemplatesResponse {
  templates: Array<{
    id: string;
    name: string;
    description: string;
    type: 'Executive' | 'Technical' | 'Compliance';
  }>;
}

export function useReports() { ... }
export function useReportTemplates() { ... }
export function useCreateReport() { ... }
```

Xem code đầy đủ tại: [`solutions/10_11_12_webhook_report_notification.md`](../solutions/10_11_12_webhook_report_notification.md) — Solution 11

### [MODIFY] `src/features/reports/components/ReportCenter.tsx`

**Xóa:**
```typescript
const templates = [...]
const reports = [...]
const productOptions = ['All Products', 'Banking App', ...]
```

**Thêm:**
```typescript
import { useReports, useReportTemplates, useCreateReport } from '../hooks/useReports';
import { useProducts } from '@/features/product-security/hooks/useProducts';

const reportsQuery = useReports();
const templatesQuery = useReportTemplates();
const productsQuery = useProducts();  // Dùng lại hook sẵn có
const createReport = useCreateReport();

// Product filter options — dynamic từ server
const productOptions = ['All Products', ...(productsQuery.data?.products.map(p => p.name) ?? [])];

// Subtitle động
const subtitle = reportsQuery.data
  ? `${reportsQuery.data.total} reports · Last report ${timeAgo(reportsQuery.data.lastGeneratedAt)}`
  : 'Loading...';
```

### [MODIFY] `src/shared/api/endpoints.ts`

```typescript
reports: {
  list:      '/api/v1/reports',
  create:    '/api/v1/reports',
  templates: '/api/v1/reports/templates',
  download:  (id: string) => `/api/v1/reports/${id}/download`,
},
```

### [NEW] `src/mocks/handlers/reports.handlers.ts`

```typescript
import { reportsFixture, reportTemplatesFixture } from '../fixtures/reports.fixture';

export const reportHandlers = [
  http.get('/api/v1/reports', () => HttpResponse.json({
    reports: reportsFixture,
    total: reportsFixture.length,
    lastGeneratedAt: '2026-06-14T09:00:00Z',
  })),
  http.get('/api/v1/reports/templates', () => HttpResponse.json({
    templates: reportTemplatesFixture,
  })),
  http.post('/api/v1/reports', async ({ request }) => {
    const body = await request.json();
    const newReport = {
      id: `R-${Date.now()}`, ...body,
      status: 'generating', createdAt: new Date().toISOString(),
      createdBy: 'current-user@company.com',
    };
    return HttpResponse.json(newReport, { status: 201 });
  }),
];
```

---

## Tiêu chí hoàn thành

- [x] `features/reports/hooks/useReports.ts` tạo xong (useReports + useReportTemplates + useCreateReport)
- [x] `ReportCenter.tsx` không còn `const reports = [...]`, `const templates = [...]`
- [x] Templates load từ `useReportTemplates()` (không hardcode)
- [x] Subtitle hiển thị "Σ reports · Last report X ago" từ `data.total` + `data.last_generated_at`
- [x] Generate view dùng `createReport.mutateAsync()` (POST thực sự)
- [x] Status icon: CheckCircle (completed), Loader2 spin (generating/pending), AlertTriangle (failed)
- [x] formatDate/formatFileSize từ ISO string (không hardcode "Jun 14, 09:00 AM")
- [x] MSW handler: `last_generated_at` từ fixture, GET templates handler
- [x] TypeScript 0 lỗi mới

---

## ✅ Đã hoàn thành — 2026-06-19

**Files đã tạo/sửa:**
- [`features/reports/hooks/useReports.ts`](../../../../ui/src/features/reports/hooks/useReports.ts) — [NEW] 3 hooks
- [`features/reports/components/ReportCenter.tsx`](../../../../ui/src/features/reports/components/ReportCenter.tsx) — [MODIFY] Refactored
- [`mocks/handlers/report.handlers.ts`](../../../../ui/src/mocks/handlers/report.handlers.ts) — [MODIFY] +templates handler, +last_generated_at

---

## Kiểm tra

```bash
# Mở http://localhost:3000 → Reports
# 1. 3 reports từ MSW (R-047, R-046, R-045)
# 2. Subtitle: "3 reports · Last report ..." (tính từ lastGeneratedAt)
# 3. Product filter dropdown: products từ useProducts() hook
# 4. "Generate Report" → chọn template → tạo → hiện trong list (status: generating)
```
