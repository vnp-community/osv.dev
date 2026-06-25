# Solution 13 — LoginScreen.tsx + 14 — ProductSecurity.tsx + 15 — Design Tokens + 16 — MSW Setup

---

## Solution 13: LoginScreen.tsx

### Vấn đề
- Stats hardcode: `"240K+"`, `"1,847"`, `"99.99%"`
- Threat indicators hardcode: `{ count: 14 }`, `{ count: 7 }`, `{ count: 23 }`
- Version string hardcode: `"OSV Platform v3.2.1"`

### API Endpoint (Public — không cần auth)
```
GET /api/v2/public/stats    → Platform-wide stats (không cần JWT)
```

### Hook (public)

```typescript
// features/auth/hooks/usePublicStats.ts
import { useQuery } from '@tanstack/react-query';
import axios from 'axios';  // Plain axios — không dùng apiClient (không có auth header)

export interface PublicStats {
  totalCVEs: string;        // "240K+"
  scansToday: number;
  findingAccuracy: string;  // "98.4%"
  uptimeSLA: string;        // "99.99%"
  threatIndicators: {
    criticalThreats: number;
    kevActive: number;
    assetsAtRisk: number;
  };
}

export function usePublicStats() {
  return useQuery<PublicStats>({
    queryKey: ['public', 'stats'],
    queryFn: async () => {
      const { data } = await axios.get<PublicStats>('/api/v2/public/stats');
      return data;
    },
    staleTime: 5 * 60_000,
    retry: false,  // Login page — không retry nếu backend down
  });
}
```

### Version từ Vite env

```typescript
// vite.config.ts — thêm define
export default defineConfig({
  define: {
    __APP_VERSION__: JSON.stringify(process.env.npm_package_version ?? '0.0.0'),
  },
});

// vite-env.d.ts — thêm type
declare const __APP_VERSION__: string;
```

### Thay đổi trong LoginScreen.tsx

```typescript
// Xóa:
// const stats = [...]
// const threatIndicators = [...]
// "OSV Platform v3.2.1"

// Thêm:
import { usePublicStats } from '../hooks/usePublicStats';

// Trong component:
const { data: publicStats } = usePublicStats();

// Stats row — từ server với fallback
const stats = publicStats ? [
  { label: 'CVEs Tracked', value: publicStats.totalCVEs },
  { label: 'Scans Today', value: publicStats.scansToday.toLocaleString() },
  { label: 'Findings', value: publicStats.findingAccuracy },
  { label: 'Uptime SLA', value: publicStats.uptimeSLA },
] : [
  { label: 'CVEs Tracked', value: '—' },
  { label: 'Scans Today', value: '—' },
  { label: 'Findings', value: '—' },
  { label: 'Uptime SLA', value: '—' },
];

// Threat indicators — từ server
const threatIndicators = publicStats ? [
  { label: 'Critical Threats', count: publicStats.threatIndicators.criticalThreats, color: '#EF4444' },
  { label: 'KEV Active', count: publicStats.threatIndicators.kevActive, color: '#F59E0B' },
  { label: 'Assets At Risk', count: publicStats.threatIndicators.assetsAtRisk, color: '#3B82F6' },
] : [];

// Version từ build env — không hardcode
<p>OSV Platform v{__APP_VERSION__} · © {new Date().getFullYear()} OSV Security Inc.</p>
```

### MSW Handler

```typescript
http.get('/api/v2/public/stats', () => {
  return HttpResponse.json({
    totalCVEs: '240K+',
    scansToday: 1847,
    findingAccuracy: '98.4%',
    uptimeSLA: '99.99%',
    threatIndicators: {
      criticalThreats: 14,
      kevActive: 7,
      assetsAtRisk: 23,
    },
  });
}),
```

---

## Solution 14: ProductSecurity.tsx

### Vấn đề
```typescript
const [expandedTypes, setExpandedTypes] = useState<string[]>(["pt-1"]);  // hardcode "pt-1"
const [selectedProductId, setSelectedProductId] = useState<string>("p-1");  // hardcode "p-1"
```

### Fix — Dynamic default selection

```typescript
// Thay thế bằng logic chọn item đầu tiên từ server response:

export function ProductSecurity() {
  const productsQuery = useProducts();
  // Bắt đầu với state rỗng — sẽ được set sau khi data load
  const [expandedTypes, setExpandedTypes] = useState<string[]>([]);
  const [selectedProductId, setSelectedProductId] = useState<string | null>(null);

  // ...

  return (
    <QueryBoundary query={productsQuery} skeleton={<ProductSkeleton />}>
      {({ productTypes, riskTrend }) => {
        const allProducts = productTypes.flatMap(pt => pt.products);

        // ✅ Dynamic default — lấy item đầu tiên từ server
        const effectiveExpandedTypes = expandedTypes.length > 0
          ? expandedTypes
          : productTypes.length > 0 ? [productTypes[0].id] : [];

        const effectiveSelectedId = selectedProductId ?? allProducts[0]?.id ?? null;
        const selectedProduct = allProducts.find(p => p.id === effectiveSelectedId) ?? allProducts[0];

        // ...
      }}
    </QueryBoundary>
  );
}
```

---

## Solution 15: Design Tokens + Thresholds

### File 1: `shared/constants/thresholds.ts`

```typescript
// src/shared/constants/thresholds.ts
/**
 * Tập trung tất cả ngưỡng số nghiệp vụ.
 * Import từ đây thay vì hardcode trong component.
 */

// SLA thresholds
export const SLA_COMPLIANCE_GREEN = 95;   // >= 95% → green
export const SLA_COMPLIANCE_YELLOW = 90;  // >= 90% → yellow
export const SLA_DAYS_CRITICAL = 1;       // <= 1 day → critical
export const SLA_DAYS_WARNING = 7;        // <= 7 days → warning

// Finding count severity
export const FINDING_COUNT_HIGH = 20;     // > 20 → red
export const FINDING_COUNT_MEDIUM = 5;    // > 5 → yellow

// AI thresholds
export const AI_CONFIDENCE_HIGH = 0.90;   // > 90% → green
export const AI_CONFIDENCE_MEDIUM = 0.70; // > 70% → yellow

// Webhook performance
export const WEBHOOK_SLOW_MS = 1000;      // > 1000ms → slow (red)

// Risk acceptance
export const RISK_DAYS_EXPIRING = 30;     // <= 30 days → warning
```

### File 2: `shared/styles/tokens.css`

```css
/* src/shared/styles/tokens.css */
/* Design tokens — tập trung tất cả màu sắc lặp lại */

:root {
  /* Background */
  --color-bg-page: #0B1020;
  --color-bg-card: #151B2F;
  --color-bg-sidebar: #0F1629;
  --color-bg-hover: rgba(255, 255, 255, 0.02);
  --color-bg-input: rgba(255, 255, 255, 0.05);

  /* Borders */
  --color-border-subtle: rgba(255, 255, 255, 0.07);
  --color-border-input: rgba(255, 255, 255, 0.1);
  --color-border-section: rgba(255, 255, 255, 0.06);

  /* Text */
  --color-text-primary: #E5E7EB;
  --color-text-secondary: #9CA3AF;
  --color-text-muted: #6B7280;
  --color-text-faint: #4B5563;
  --color-text-disabled: #374151;

  /* Brand */
  --color-primary: #4F8CFF;
  --color-primary-dark: #3B6FCC;
  --color-primary-bg: rgba(79, 140, 255, 0.1);
  --color-primary-border: rgba(79, 140, 255, 0.3);

  /* Severity */
  --color-severity-critical: #EF4444;
  --color-severity-critical-bg: rgba(239, 68, 68, 0.1);
  --color-severity-high: #F97316;
  --color-severity-high-bg: rgba(249, 115, 22, 0.15);
  --color-severity-medium: #EAB308;
  --color-severity-medium-bg: rgba(234, 179, 8, 0.15);
  --color-severity-low: #3B82F6;
  --color-severity-low-bg: rgba(59, 130, 246, 0.15);

  /* Status */
  --color-status-success: #10B981;
  --color-status-success-bg: rgba(16, 185, 129, 0.1);
  --color-status-warning: #F59E0B;
  --color-status-warning-bg: rgba(245, 158, 11, 0.1);
  --color-status-error: #EF4444;
  --color-status-error-bg: rgba(239, 68, 68, 0.1);
  --color-status-info: #4F8CFF;
  --color-status-info-bg: rgba(79, 140, 255, 0.1);

  /* AI */
  --color-ai: #A78BFA;
  --color-ai-bg: rgba(167, 139, 250, 0.1);

  /* Charts */
  --color-chart-grid: rgba(255, 255, 255, 0.05);
  --color-chart-tooltip-bg: #1E2A45;
  --color-chart-tooltip-border: rgba(255, 255, 255, 0.1);
}
```

### Cách migrate từ inline hex sang tokens

```typescript
// ❌ Cũ — inline hex
style={{ background: '#0B1020', color: '#E5E7EB' }}

// ✅ Mới — dùng CSS variable
style={{ background: 'var(--color-bg-page)', color: 'var(--color-text-primary)' }}

// Hoặc dùng với Tailwind class (nếu extend config):
// className="bg-[var(--color-bg-page)] text-[var(--color-text-primary)]"
```

---

## Solution 16: MSW Setup tổng hợp

### Cấu trúc thư mục

```
src/mocks/
├── browser.ts                 # MSW worker setup
├── handlers/
│   ├── index.ts               # Export all handlers
│   ├── admin.handlers.ts      # Users, RBAC, Settings, Audit
│   ├── ai.handlers.ts         # AI Triage, Enrichment
│   ├── audit.handlers.ts      # Audit logs
│   ├── auth.handlers.ts       # Login, Refresh (đã có)
│   ├── cve.handlers.ts        # CVE, KEV, EPSS (đã có)
│   ├── findings.handlers.ts   # Findings, Risk Acceptances
│   ├── integrations.handlers.ts # API Keys, Webhooks
│   ├── notifications.handlers.ts # Notifications
│   ├── products.handlers.ts   # Products, Engagements
│   ├── reports.handlers.ts    # Reports
│   └── scan.handlers.ts       # Scans, Stats (đã có một phần)
└── fixtures/
    ├── users.fixture.ts
    ├── audit.fixture.ts
    ├── rbac.fixture.ts
    └── ...
```

### handlers/index.ts

```typescript
// src/mocks/handlers/index.ts
import { adminUserHandlers } from './admin.handlers';
import { auditHandlers } from './audit.handlers';
import { aiHandlers } from './ai.handlers';
import { notificationHandlers } from './notifications.handlers';
import { integrationHandlers } from './integrations.handlers';
import { riskAcceptanceHandlers } from './findings.handlers';
// ... import handlers đã có

export const handlers = [
  // Admin
  ...adminUserHandlers,
  ...auditHandlers,

  // AI
  ...aiHandlers,

  // Notifications
  ...notificationHandlers,

  // Integrations
  ...integrationHandlers,

  // Findings
  ...riskAcceptanceHandlers,

  // ... handlers đã có
];
```

### Bật MSW trong development

```typescript
// src/main.tsx — đã có pattern này theo architecture.md Section 5.6.1
async function enableMocking() {
  if (import.meta.env.VITE_ENABLE_MSW !== 'true') return;
  const { worker } = await import('./mocks/browser');
  return worker.start({ onUnhandledRequest: 'warn' });
}

enableMocking().then(() => {
  createRoot(document.getElementById('root')!).render(<App />);
});
```

```env
# .env.development
VITE_ENABLE_MSW=true

# .env.development.local (khi backend sẵn sàng)
VITE_ENABLE_MSW=false
VITE_API_BASE_URL=http://localhost:8080
```
