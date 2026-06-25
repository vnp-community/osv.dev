# SOL-004 — Phase 3: Polish, Performance & Testing

**Version:** 1.1  
**Ngày tạo:** 2026-06-16  
**Cập nhật:** 2026-06-17  
**Trạng thái:** 🔄 In Progress — Testing infra + ESLint rule + CI hoàn tất; CVETable virtualized; component migrations + E2E runtime verification pending  
**Phase:** Phase 3 (Sprint 6-7) — Infra Done / Polish Pending  
**Liên quan:** [SOL-003](./SOL-003-phase2-api-migration.md)

---

## 1. Mục tiêu Phase 3

1. Virtualized lists cho datasets lớn (300K CVEs)
2. Code splitting + lazy loading đầy đủ
3. Skeleton UI hoàn chỉnh
4. Unit tests cho utilities và hooks
5. Component tests với React Testing Library + MSW
6. CI gate cho hardcoded data detection
7. ESLint custom rule: `no-hardcode-mock-data`
8. Error boundaries + graceful degradation
9. React Hook Form + Zod cho Scan Wizard

---

## 2. Virtualized Lists

### 2.1 CVETable with Virtualization

```typescript
// src/features/cve-intel/components/CVETable.tsx
import { useRef } from 'react';
import { useVirtualizer } from '@tanstack/react-virtual';
import type { CVE } from '@/shared/types/cve';
import { SeverityBadge } from '@/shared/components/data-display/SeverityBadge';
import { EPSSBar } from '@/shared/components/data-display/EPSSBar';
import { KEVIndicator } from '@/shared/components/data-display/KEVIndicator';

interface CVETableProps {
  cves: CVE[];
  total: number;
  page: number;
  pageSize: number;
  onPageChange: (page: number) => void;
  onRowClick: (cve: CVE) => void;
}

export function CVETable({
  cves,
  total,
  page,
  pageSize,
  onPageChange,
  onRowClick,
}: CVETableProps) {
  const parentRef = useRef<HTMLDivElement>(null);

  const rowVirtualizer = useVirtualizer({
    count: cves.length,
    getScrollElement: () => parentRef.current,
    estimateSize: () => 52,   // row height in px
    overscan: 10,             // render 10 extra rows above/below viewport
  });

  return (
    <div className="flex flex-col h-full">
      {/* Table Header */}
      <div
        className="grid px-4 py-2.5"
        style={{
          gridTemplateColumns: '160px 80px 70px 80px 60px 120px 120px 100px',
          borderBottom: '1px solid rgba(255,255,255,0.06)',
          background: '#0F1629',
        }}
      >
        {['CVE ID', 'SEVERITY', 'CVSS', 'EPSS', 'KEV', 'VENDOR', 'PRODUCT', 'UPDATED'].map(
          (col) => (
            <div
              key={col}
              style={{ color: '#6B7280', fontSize: 11, fontWeight: 600, letterSpacing: 0.5 }}
            >
              {col}
            </div>
          )
        )}
      </div>

      {/* Virtualized rows */}
      <div
        ref={parentRef}
        className="flex-1 overflow-auto"
      >
        <div
          style={{
            height: `${rowVirtualizer.getTotalSize()}px`,
            width: '100%',
            position: 'relative',
          }}
        >
          {rowVirtualizer.getVirtualItems().map((virtualItem) => {
            const cve = cves[virtualItem.index];
            return (
              <div
                key={cve.id}
                data-index={virtualItem.index}
                ref={rowVirtualizer.measureElement}
                onClick={() => onRowClick(cve)}
                className="grid absolute top-0 left-0 w-full px-4"
                style={{
                  gridTemplateColumns: '160px 80px 70px 80px 60px 120px 120px 100px',
                  transform: `translateY(${virtualItem.start}px)`,
                  height: 52,
                  alignItems: 'center',
                  cursor: 'pointer',
                  borderBottom: '1px solid rgba(255,255,255,0.04)',
                }}
                onMouseEnter={(e) => {
                  e.currentTarget.style.background = 'rgba(255,255,255,0.02)';
                }}
                onMouseLeave={(e) => {
                  e.currentTarget.style.background = 'transparent';
                }}
              >
                <span style={{ color: '#4F8CFF', fontSize: 12, fontWeight: 500 }}>
                  {cve.id}
                </span>
                <SeverityBadge severity={cve.severity} />
                <span style={{ color: getCVSSColor(cve.cvssV3), fontSize: 12, fontWeight: 600 }}>
                  {cve.cvssV3?.toFixed(1) ?? '—'}
                </span>
                <EPSSBar score={cve.epssScore} />
                <KEVIndicator isKEV={cve.isKEV} />
                <span style={{ color: '#9CA3AF', fontSize: 12 }} className="truncate">
                  {cve.vendor}
                </span>
                <span style={{ color: '#9CA3AF', fontSize: 12 }} className="truncate">
                  {cve.product}
                </span>
                <span style={{ color: '#6B7280', fontSize: 11 }}>
                  {formatDate(cve.updatedAt)}
                </span>
              </div>
            );
          })}
        </div>
      </div>

      {/* Pagination */}
      <div
        className="flex items-center justify-between px-4 py-3"
        style={{ borderTop: '1px solid rgba(255,255,255,0.06)' }}
      >
        <span style={{ color: '#6B7280', fontSize: 12 }}>
          {total.toLocaleString()} CVEs found
        </span>
        <div className="flex items-center gap-2">
          <button
            onClick={() => onPageChange(page - 1)}
            disabled={page <= 1}
            style={{
              padding: '4px 12px',
              borderRadius: 8,
              background: 'rgba(255,255,255,0.05)',
              border: '1px solid rgba(255,255,255,0.08)',
              color: page <= 1 ? '#4B5563' : '#9CA3AF',
              fontSize: 12,
              cursor: page <= 1 ? 'not-allowed' : 'pointer',
            }}
          >
            ← Prev
          </button>
          <span style={{ color: '#6B7280', fontSize: 12 }}>
            Page {page} / {Math.ceil(total / pageSize)}
          </span>
          <button
            onClick={() => onPageChange(page + 1)}
            disabled={page >= Math.ceil(total / pageSize)}
            style={{
              padding: '4px 12px',
              borderRadius: 8,
              background: 'rgba(255,255,255,0.05)',
              border: '1px solid rgba(255,255,255,0.08)',
              color: page >= Math.ceil(total / pageSize) ? '#4B5563' : '#9CA3AF',
              fontSize: 12,
              cursor: page >= Math.ceil(total / pageSize) ? 'not-allowed' : 'pointer',
            }}
          >
            Next →
          </button>
        </div>
      </div>
    </div>
  );
}

function getCVSSColor(cvss?: number): string {
  if (!cvss) return '#6B7280';
  if (cvss >= 9.0) return '#EF4444';
  if (cvss >= 7.0) return '#F97316';
  if (cvss >= 4.0) return '#EAB308';
  return '#3B82F6';
}

function formatDate(iso: string): string {
  return new Date(iso).toLocaleDateString('en-US', {
    month: 'short',
    day: 'numeric',
    year: '2-digit',
  });
}
```

---

## 3. Scan Wizard với React Hook Form + Zod

### 3.1 Scan Form Schema

```typescript
// src/features/scanning/schemas/scanWizardSchema.ts
import { z } from 'zod';

export const scanWizardSchema = z.object({
  // Step 1: Scan Type
  type: z.enum(['nmap_full', 'nmap_discovery', 'zap', 'import']),

  // Step 2: Target
  name: z.string().min(3, 'Scan name must be at least 3 characters'),
  targets: z
    .string()
    .min(1, 'At least one target required')
    .transform((val) => val.split('\n').map((t) => t.trim()).filter(Boolean)),

  // Nmap-specific
  scanProfile: z.enum(['discovery', 'full', 'custom']).optional(),
  portRange: z.string().optional(),

  // ZAP-specific
  maxDepth: z.number().min(1).max(10).optional(),
  timeout: z.number().min(30).max(3600).optional(),

  // Step 3: Schedule
  frequency: z.enum(['once', 'daily', 'weekly', 'custom']).default('once'),
  cronExpr: z.string().optional(),
  engagementId: z.string().optional(),
});

export type ScanWizardFormData = z.infer<typeof scanWizardSchema>;
```

### 3.2 ScanWizard Component

```typescript
// src/features/scanning/components/ScanWizard.tsx
import { useState } from 'react';
import { useNavigate } from 'react-router';
import { useForm } from 'react-hook-form';
import { zodResolver } from '@hookform/resolvers/zod';
import { useMutation } from '@tanstack/react-query';
import { toast } from 'sonner';
import { scanWizardSchema, type ScanWizardFormData } from '../schemas/scanWizardSchema';
import { scanApi } from '../api/scanApi';
import { scanKeys, queryClient } from '@/shared/api/queryClient';

export function ScanWizard() {
  const navigate = useNavigate();
  const [step, setStep] = useState(1);

  const form = useForm<ScanWizardFormData>({
    resolver: zodResolver(scanWizardSchema),
    defaultValues: {
      type: 'nmap_full',
      frequency: 'once',
    },
  });

  const createScan = useMutation({
    mutationFn: (data: ScanWizardFormData) =>
      scanApi.create({
        name: data.name,
        type: data.type,
        targets: data.targets as string[],
        options: {
          scanProfile: data.scanProfile,
          portRange: data.portRange,
          maxDepth: data.maxDepth,
          timeout: data.timeout,
        },
        engagementId: data.engagementId,
      }),
    onSuccess: (scan) => {
      queryClient.invalidateQueries({ queryKey: scanKeys.all });
      toast.success(`Scan "${scan.name}" started`);
      navigate(`/scans/${scan.id}`);
    },
  });

  const onSubmit = (data: ScanWizardFormData) => {
    createScan.mutate(data);
  };

  const nextStep = async () => {
    const stepFields: Record<number, (keyof ScanWizardFormData)[]> = {
      1: ['type'],
      2: ['name', 'targets'],
      3: ['frequency'],
    };
    const valid = await form.trigger(stepFields[step]);
    if (valid) setStep((s) => s + 1);
  };

  return (
    <form onSubmit={form.handleSubmit(onSubmit)}>
      {/* Step indicators */}
      <div className="flex items-center gap-3 mb-8">
        {['Scan Type', 'Target', 'Schedule', 'Review'].map((label, i) => (
          <div
            key={label}
            className="flex items-center gap-2"
          >
            <div
              className="w-7 h-7 rounded-full flex items-center justify-center text-xs font-bold"
              style={{
                background: step > i + 1
                  ? '#10B981'
                  : step === i + 1
                  ? 'linear-gradient(135deg, #4F8CFF, #7C3AED)'
                  : 'rgba(255,255,255,0.08)',
                color: step >= i + 1 ? 'white' : '#6B7280',
              }}
            >
              {step > i + 1 ? '✓' : i + 1}
            </div>
            <span
              style={{
                color: step >= i + 1 ? '#E5E7EB' : '#6B7280',
                fontSize: 13,
                fontWeight: step === i + 1 ? 600 : 400,
              }}
            >
              {label}
            </span>
            {i < 3 && (
              <div
                className="w-8 h-px"
                style={{ background: 'rgba(255,255,255,0.08)' }}
              />
            )}
          </div>
        ))}
      </div>

      {/* Step content */}
      {step === 1 && <ScanTypeStep form={form} />}
      {step === 2 && <TargetStep form={form} />}
      {step === 3 && <ScheduleStep form={form} />}
      {step === 4 && <ReviewStep form={form} />}

      {/* Navigation */}
      <div className="flex justify-between mt-8">
        <button
          type="button"
          onClick={() => step > 1 ? setStep((s) => s - 1) : navigate('/scans')}
          style={{
            padding: '10px 24px',
            borderRadius: 12,
            background: 'rgba(255,255,255,0.05)',
            border: '1px solid rgba(255,255,255,0.1)',
            color: '#9CA3AF',
            fontSize: 13,
            cursor: 'pointer',
          }}
        >
          ← {step > 1 ? 'Back' : 'Cancel'}
        </button>
        {step < 4 ? (
          <button
            type="button"
            onClick={nextStep}
            style={{
              padding: '10px 24px',
              borderRadius: 12,
              background: 'linear-gradient(135deg, #4F8CFF, #3B6FCC)',
              color: 'white',
              fontSize: 13,
              border: 'none',
              cursor: 'pointer',
            }}
          >
            Next →
          </button>
        ) : (
          <button
            type="submit"
            disabled={createScan.isPending}
            style={{
              padding: '10px 24px',
              borderRadius: 12,
              background: createScan.isPending
                ? 'rgba(16,185,129,0.5)'
                : '#10B981',
              color: 'white',
              fontSize: 13,
              border: 'none',
              cursor: createScan.isPending ? 'not-allowed' : 'pointer',
            }}
          >
            {createScan.isPending ? 'Launching...' : '🚀 Launch Scan'}
          </button>
        )}
      </div>
    </form>
  );
}
```

---

## 4. Unit Tests

### 4.1 Test setup

```typescript
// src/test/setup.ts
import '@testing-library/jest-dom';
import { server } from '../mocks/server';

// Start MSW server before all tests
beforeAll(() => server.listen({ onUnhandledRequest: 'error' }));
afterEach(() => server.resetHandlers());
afterAll(() => server.close());
```

```typescript
// src/mocks/server.ts (Node.js environment for tests)
import { setupServer } from 'msw/node';
import { handlers } from './handlers';

export const server = setupServer(...handlers);
```

### 4.2 Utility Tests

```typescript
// src/shared/utils/__tests__/severity.test.ts
import { describe, it, expect } from 'vitest';
import { SEVERITY_COLORS, getSeverityColor } from '../severity';

describe('severity utils', () => {
  it('returns correct color for Critical', () => {
    expect(getSeverityColor('Critical')).toBe('#EF4444');
  });

  it('returns correct color for High', () => {
    expect(getSeverityColor('High')).toBe('#F97316');
  });

  it('returns fallback for unknown severity', () => {
    expect(getSeverityColor('Unknown' as any)).toBe('#6B7280');
  });

  it('has all severity levels defined', () => {
    const severities = ['Critical', 'High', 'Medium', 'Low', 'Info'];
    severities.forEach((s) => {
      expect(SEVERITY_COLORS).toHaveProperty(s);
    });
  });
});
```

```typescript
// src/shared/utils/__tests__/sla.test.ts
import { describe, it, expect, vi } from 'vitest';
import { getSLAStatus, getSLADaysLeft } from '../sla';

describe('SLA utils', () => {
  it('returns breached when past expiration', () => {
    const pastDate = '2020-01-01T00:00:00Z';
    expect(getSLAStatus(pastDate)).toBe('breached');
  });

  it('returns at_risk when less than 3 days left', () => {
    const nearFuture = new Date(Date.now() + 2 * 24 * 60 * 60 * 1000).toISOString();
    expect(getSLAStatus(nearFuture)).toBe('at_risk');
  });

  it('returns ok when more than 3 days left', () => {
    const farFuture = new Date(Date.now() + 10 * 24 * 60 * 60 * 1000).toISOString();
    expect(getSLAStatus(farFuture)).toBe('ok');
  });
});
```

```typescript
// src/shared/utils/__tests__/findingStateMachine.test.ts
import { describe, it, expect } from 'vitest';
import { canTransition, VALID_TRANSITIONS } from '../findingStateMachine';

describe('finding state machine', () => {
  it('allows active → mitigated', () => {
    expect(canTransition('active', 'mitigated')).toBe(true);
  });

  it('allows mitigated → active (reopen)', () => {
    expect(canTransition('mitigated', 'active')).toBe(true);
  });

  it('disallows duplicate → any transition', () => {
    expect(VALID_TRANSITIONS.duplicate).toHaveLength(0);
  });

  it('disallows direct mitigated → false_positive', () => {
    expect(canTransition('mitigated', 'false_positive')).toBe(false);
  });
});
```

### 4.3 Hook Tests

```typescript
// src/features/dashboard/hooks/__tests__/useDashboardMetrics.test.tsx
import { renderHook, waitFor } from '@testing-library/react';
import { QueryClientProvider } from '@tanstack/react-query';
import { QueryClient } from '@tanstack/react-query';
import { useDashboardMetrics } from '../useDashboardMetrics';
import { dashboardFixture } from '@/mocks/fixtures/dashboard.fixture';

function createWrapper() {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  return ({ children }: { children: React.ReactNode }) => (
    <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
  );
}

describe('useDashboardMetrics', () => {
  it('fetches dashboard metrics successfully', async () => {
    const { result } = renderHook(
      () => useDashboardMetrics('30d'),
      { wrapper: createWrapper() }
    );

    await waitFor(() => expect(result.current.isSuccess).toBe(true));

    expect(result.current.data?.kpis.criticalFindings).toBe(
      dashboardFixture['30d'].kpis.criticalFindings
    );
  });

  it('starts in loading state', () => {
    const { result } = renderHook(
      () => useDashboardMetrics('30d'),
      { wrapper: createWrapper() }
    );
    expect(result.current.isLoading).toBe(true);
  });
});
```

### 4.4 Component Tests

```typescript
// src/features/dashboard/components/__tests__/Dashboard.test.tsx
import { render, screen, waitFor } from '@testing-library/react';
import { Dashboard } from '../Dashboard';
import { createTestProviders } from '@/test/utils';

describe('Dashboard', () => {
  it('shows skeleton while loading', () => {
    render(<Dashboard />, { wrapper: createTestProviders() });
    // Should show skeleton, not actual numbers
    expect(screen.queryByText('245')).not.toBeInTheDocument();
    expect(screen.getByTestId('dashboard-skeleton')).toBeInTheDocument();
  });

  it('renders KPI values after load', async () => {
    render(<Dashboard />, { wrapper: createTestProviders() });
    await waitFor(() => {
      expect(screen.getByText('245')).toBeInTheDocument(); // criticalFindings
    });
  });

  it('does NOT hardcode any business data', async () => {
    // This test ensures data comes from MSW, not component
    render(<Dashboard />, { wrapper: createTestProviders() });
    // If MSW returns 0 findings, component should show 0
    // not a hardcoded value
    await waitFor(() => {
      expect(screen.getByText('245')).toBeInTheDocument();
    });
  });
});
```

---

## 5. ESLint Custom Rule: no-hardcode-mock-data

### 5.1 Rule definition

```javascript
// eslint-rules/no-hardcode-mock-data.js
/**
 * ESLint rule to prevent hardcoded business data arrays in component files.
 * Allowlist:
 * - src/mocks/ directory (fixtures)
 * - SEVERITY_CONFIG, SEVERITY_COLORS, SLA_CONFIG (UI constants)
 * - Route definitions
 */
module.exports = {
  meta: {
    type: 'problem',
    docs: {
      description: 'Disallow hardcoded data arrays in component files',
    },
    schema: [],
  },
  create(context) {
    const filename = context.getFilename();

    // Allow in mocks directory
    if (filename.includes('/mocks/')) return {};
    // Allow in utils (SEVERITY_COLORS etc)
    if (filename.includes('/utils/')) return {};
    // Allow in test files
    if (filename.includes('.test.') || filename.includes('.spec.')) return {};

    return {
      VariableDeclaration(node) {
        node.declarations.forEach((decl) => {
          if (
            decl.init?.type === 'ArrayExpression' &&
            decl.init.elements.length > 2 &&
            decl.init.elements.some(
              (el) => el?.type === 'ObjectExpression'
            )
          ) {
            const varName = decl.id.name ?? '';
            // Skip allowed patterns (UI constants)
            const allowedPatterns = [
              'COLUMNS', 'OPTIONS', 'TABS', 'STEPS', 'ROUTES',
              'MENU', 'NAV', 'BREADCRUMB'
            ];
            if (allowedPatterns.some((p) => varName.toUpperCase().includes(p))) {
              return;
            }
            context.report({
              node,
              message:
                `Hardcoded data array "${varName}" detected in component. ` +
                `Use React Query hook instead. See architecture.md Section 5.5.`,
            });
          }
        });
      },
    };
  },
};
```

### 5.2 ESLint config update

```javascript
// eslint.config.js (or .eslintrc)
import noHardcodeMockData from './eslint-rules/no-hardcode-mock-data.js';

export default [
  {
    plugins: {
      local: { rules: { 'no-hardcode-mock-data': noHardcodeMockData } },
    },
    rules: {
      'local/no-hardcode-mock-data': 'error',
    },
    files: ['src/features/**/*.tsx', 'src/app/**/*.tsx'],
  },
];
```

---

## 6. CI Gate Configuration

```yaml
# .github/workflows/ci.yml
name: CI

on: [push, pull_request]

jobs:
  check:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: pnpm/action-setup@v3
      - uses: actions/setup-node@v4
        with: { node-version: '22', cache: 'pnpm' }
      - run: pnpm install

      # Hardcode data detection gate
      - name: Check for hardcoded data in components
        run: |
          if grep -rn 'const.*Data.*=.*\[' src/features/ --include='*.tsx' | \
             grep -v '//.*COLUMNS\|TABS\|OPTIONS\|STEPS'; then
            echo "❌ Hardcoded data arrays found in feature components!"
            echo "Remove them and use React Query hooks instead."
            exit 1
          fi
          echo "✅ No hardcoded data found"

      # MSW env check
      - name: Verify VITE_ENABLE_MSW is false in production env
        run: |
          if grep -q 'VITE_ENABLE_MSW=true' .env.production 2>/dev/null; then
            echo "❌ VITE_ENABLE_MSW=true in .env.production!"
            exit 1
          fi

      - name: Run ESLint
        run: pnpm lint

      - name: Run Tests
        run: pnpm test --coverage

      - name: Check Coverage Thresholds
        run: |
          # Vitest will fail if coverage is below threshold
          # Configured in vitest.config.ts
          echo "Coverage check passed"

      - name: Build
        run: pnpm build
```

---

## 7. Skeleton UI Components

### 7.1 Dashboard Skeleton

```typescript
// src/features/dashboard/components/DashboardSkeleton.tsx
function SkeletonBox({
  w,
  h,
  className = '',
}: {
  w?: string;
  h: string;
  className?: string;
}) {
  return (
    <div
      className={`rounded-xl animate-pulse ${className}`}
      style={{
        width: w,
        height: h,
        background: 'rgba(255,255,255,0.06)',
      }}
    />
  );
}

export function DashboardSkeleton() {
  return (
    <div
      className="flex-1 overflow-y-auto p-6"
      data-testid="dashboard-skeleton"
      style={{ background: '#0B1020' }}
    >
      {/* KPI Row skeleton */}
      <div className="grid grid-cols-6 gap-4 mb-6">
        {Array.from({ length: 6 }).map((_, i) => (
          <div
            key={i}
            className="rounded-2xl p-5"
            style={{ background: '#151B2F', border: '1px solid rgba(255,255,255,0.07)' }}
          >
            <SkeletonBox h="40px" w="40px" className="mb-4" />
            <SkeletonBox h="28px" w="80px" className="mb-2" />
            <SkeletonBox h="14px" w="60px" />
          </div>
        ))}
      </div>

      {/* Charts row skeleton */}
      <div className="grid grid-cols-3 gap-4 mb-6">
        <SkeletonBox h="280px" className="col-span-2 rounded-2xl" />
        <SkeletonBox h="280px" className="rounded-2xl" />
      </div>

      {/* Table skeleton */}
      <div
        className="rounded-2xl p-6"
        style={{ background: '#151B2F', border: '1px solid rgba(255,255,255,0.07)' }}
      >
        <SkeletonBox h="20px" w="200px" className="mb-6" />
        {Array.from({ length: 5 }).map((_, i) => (
          <div key={i} className="flex gap-4 mb-4">
            <SkeletonBox h="14px" w="80px" />
            <SkeletonBox h="14px" w="120px" />
            <SkeletonBox h="14px" w="200px" />
            <SkeletonBox h="14px" w="60px" />
            <SkeletonBox h="14px" w="80px" />
          </div>
        ))}
      </div>
    </div>
  );
}
```

---

## 8. Vitest Configuration

```typescript
// vitest.config.ts
import { defineConfig } from 'vitest/config';
import path from 'path';

export default defineConfig({
  test: {
    environment: 'jsdom',
    globals: true,
    setupFiles: './src/test/setup.ts',
    coverage: {
      provider: 'v8',
      reporter: ['text', 'json', 'html'],
      thresholds: {
        // Architecture requirements
        'src/shared/utils': {
          statements: 90,
          branches: 90,
          functions: 90,
          lines: 90,
        },
        'src/features/**/hooks': {
          statements: 80,
          branches: 80,
          functions: 80,
          lines: 80,
        },
        'src/features/**/components': {
          statements: 70,
          branches: 70,
          functions: 70,
          lines: 70,
        },
      },
    },
  },
  resolve: {
    alias: {
      '@': path.resolve(__dirname, './src'),
    },
  },
});
```

---

## 9. E2E Tests (Playwright)

### 9.1 Setup

```typescript
// playwright.config.ts
import { defineConfig, devices } from '@playwright/test';

export default defineConfig({
  testDir: './e2e',
  fullyParallel: true,
  forbidOnly: !!process.env.CI,
  retries: process.env.CI ? 2 : 0,
  workers: process.env.CI ? 1 : undefined,
  reporter: 'html',
  use: {
    baseURL: 'http://localhost:3000',
    trace: 'on-first-retry',
  },
  projects: [
    { name: 'chromium', use: { ...devices['Desktop Chrome'] } },
    { name: 'firefox', use: { ...devices['Desktop Firefox'] } },
  ],
  webServer: {
    command: 'VITE_ENABLE_MSW=true pnpm dev',
    port: 3000,
    reuseExistingServer: !process.env.CI,
  },
});
```

### 9.2 Critical Flow Tests

```typescript
// e2e/auth.spec.ts
import { test, expect } from '@playwright/test';

test.describe('Authentication Flow', () => {
  test('login → dashboard', async ({ page }) => {
    await page.goto('/login');
    await page.fill('[id="email"]', 'admin@osv.local');
    await page.fill('[id="password"]', 'password');
    await page.click('[id="login-btn"]');

    await expect(page).toHaveURL('/dashboard');
    await expect(page.locator('h1')).toContainText('Executive Security Dashboard');
  });

  test('protected route → redirect to login', async ({ page }) => {
    await page.goto('/findings');
    await expect(page).toHaveURL('/login');
  });
});

// e2e/cve.spec.ts
test.describe('CVE Intelligence', () => {
  test('CVE search → filter → detail', async ({ page }) => {
    await page.goto('/cve/search');

    // Search
    await page.fill('[id="cve-search-input"]', 'log4j');
    await page.waitForSelector('[data-testid="cve-table-row"]');

    // Filter critical only
    await page.click('[id="severity-critical"]');
    await expect(page.locator('[data-testid="cve-table-row"]')).toHaveCount(
      await page.locator('[data-severity="Critical"]').count()
    );

    // View detail
    await page.click('[data-testid="cve-table-row"]', { clickCount: 1 });
    await expect(page.locator('[data-testid="cve-detail-drawer"]')).toBeVisible();
  });
});
```

---

## 10. Checklist Phase 3

### Performance
- [ ] Install `@tanstack/react-virtual`
- [ ] Implement `CVETable` with virtualization
- [ ] Implement `FindingsList` with virtualization (>100 rows)
- [ ] Verify lazy loading cho tất cả routes

### Forms
- [ ] Install `@hookform/resolvers` + `zod`
- [ ] Implement `scanWizardSchema.ts`
- [ ] Migrate `ScanWizard.tsx` → React Hook Form + Zod
- [ ] Add form validation error display

### Testing
- [ ] Setup Vitest + `@testing-library/react`
- [ ] Setup MSW server cho test environment
- [ ] Write utility tests (severity, sla, findingStateMachine) — target: ≥90%
- [ ] Write hook tests (useDashboardMetrics, useCVESearch) — target: ≥80%
- [ ] Write component tests (Dashboard, CVESearch, FindingsList) — target: ≥70%
- [ ] Setup Playwright
- [ ] Write 6 critical E2E flows

### Code Quality
- [ ] Implement ESLint custom rule `no-hardcode-mock-data`
- [ ] Setup CI pipeline (GitHub Actions)
- [ ] Add CI gate: grep for hardcoded data
- [ ] Add CI gate: verify `VITE_ENABLE_MSW=false` in production
- [ ] Configure Vitest coverage thresholds

### Skeleton UI
- [ ] `DashboardSkeleton.tsx`
- [ ] `CVETableSkeleton.tsx`
- [ ] `FindingsListSkeleton.tsx`
- [ ] `ScanDashboardSkeleton.tsx`
- [ ] Generic `TableSkeleton.tsx`
