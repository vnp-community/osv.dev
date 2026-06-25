# TASK-UI-006 — Shared Utils + QueryBoundary + Shared Components

| Field | Value |
|-------|-------|
| **Task ID** | TASK-UI-006 |
| **Module** | `ui/src/shared/utils/`, `ui/src/shared/components/` |
| **Solution Ref** | [SOL-003 §7,8](../solutions/SOL-003-phase2-api-migration.md), [SOL-002 §8](../solutions/SOL-002-phase1-foundation.md) |
| **Priority** | 🔴 P0 |
| **Depends On** | TASK-UI-002 |
| **Estimated** | 2h |
| **Status** | ✅ Completed |
| **Completed** | 2026-06-17 |

---

## Context

Trước khi migrate từng feature module, cần tạo shared utilities và shared components để tránh duplicate code. `QueryBoundary` là component cốt lõi — mọi feature component đều dùng để wrap loading/error/empty states.

---

## Goal

Tạo shared utilities (severity, sla, grade, stateMachine), `QueryBoundary`, feedback components, và data-display components.

---

## Target Files

| Action | File Path |
|--------|-----------|
| CREATE | `ui/src/shared/utils/severity.ts` |
| CREATE | `ui/src/shared/utils/sla.ts` |
| CREATE | `ui/src/shared/utils/productGrade.ts` |
| CREATE | `ui/src/shared/utils/findingStateMachine.ts` |
| CREATE | `ui/src/shared/utils/date.ts` |
| CREATE | `ui/src/shared/utils/cn.ts` |
| CREATE | `ui/src/shared/components/feedback/QueryBoundary.tsx` |
| CREATE | `ui/src/shared/components/feedback/LoadingSpinner.tsx` |
| CREATE | `ui/src/shared/components/feedback/ErrorState.tsx` |
| CREATE | `ui/src/shared/components/feedback/EmptyState.tsx` |
| CREATE | `ui/src/shared/components/data-display/SeverityBadge.tsx` |
| CREATE | `ui/src/shared/components/data-display/KEVIndicator.tsx` |
| CREATE | `ui/src/shared/components/data-display/EPSSBar.tsx` |
| CREATE | `ui/src/shared/components/data-display/SLABadge.tsx` |
| CREATE | `ui/src/shared/components/data-display/StatusBadge.tsx` |
| CREATE | `ui/src/shared/components/data-display/GradeCircle.tsx` |

---

## Implementation

### File 1: `ui/src/shared/utils/severity.ts`

```typescript
import type { Severity } from '@/shared/types/cve';

export const SEVERITY_COLORS: Record<Severity, string> = {
  Critical: '#EF4444',
  High: '#F97316',
  Medium: '#EAB308',
  Low: '#3B82F6',
  Info: '#6B7280',
};

export const SEVERITY_BG_COLORS: Record<Severity, string> = {
  Critical: 'rgba(239,68,68,0.15)',
  High: 'rgba(249,115,22,0.15)',
  Medium: 'rgba(234,179,8,0.15)',
  Low: 'rgba(59,130,246,0.15)',
  Info: 'rgba(107,114,128,0.15)',
};

export const SEVERITY_BORDER_COLORS: Record<Severity, string> = {
  Critical: 'rgba(239,68,68,0.4)',
  High: 'rgba(249,115,22,0.4)',
  Medium: 'rgba(234,179,8,0.4)',
  Low: 'rgba(59,130,246,0.4)',
  Info: 'rgba(107,114,128,0.4)',
};

export const SEVERITY_ORDER: Record<Severity, number> = {
  Critical: 0, High: 1, Medium: 2, Low: 3, Info: 4,
};

export function getSeverityColor(severity: Severity): string {
  return SEVERITY_COLORS[severity] ?? '#6B7280';
}

export function getSeverityBgColor(severity: Severity): string {
  return SEVERITY_BG_COLORS[severity] ?? 'rgba(107,114,128,0.15)';
}

export function getCVSSColor(cvss?: number): string {
  if (!cvss) return '#6B7280';
  if (cvss >= 9.0) return '#EF4444';
  if (cvss >= 7.0) return '#F97316';
  if (cvss >= 4.0) return '#EAB308';
  return '#3B82F6';
}

export function sortBySeverity<T extends { severity: Severity }>(items: T[]): T[] {
  return [...items].sort(
    (a, b) => SEVERITY_ORDER[a.severity] - SEVERITY_ORDER[b.severity]
  );
}
```

### File 2: `ui/src/shared/utils/sla.ts`

```typescript
import type { SLAStatus } from '@/shared/types/finding';
import type { Severity } from '@/shared/types/cve';

export const SLA_DAYS_BY_SEVERITY: Record<Exclude<Severity, 'Info'>, number> = {
  Critical: 7,
  High: 30,
  Medium: 90,
  Low: 180,
};

export const SLA_STATUS_COLORS: Record<SLAStatus, string> = {
  ok: '#10B981',
  at_risk: '#F97316',
  breached: '#EF4444',
};

export function getSLAStatus(expirationDate: string): SLAStatus {
  const daysLeft = getSLADaysLeft(expirationDate);
  if (daysLeft < 0) return 'breached';
  if (daysLeft <= 3) return 'at_risk';
  return 'ok';
}

export function getSLADaysLeft(expirationDate: string): number {
  const now = new Date();
  const exp = new Date(expirationDate);
  return Math.floor((exp.getTime() - now.getTime()) / (1000 * 60 * 60 * 24));
}

export function formatSLALabel(daysLeft: number): string {
  if (daysLeft < 0) return `Overdue ${Math.abs(daysLeft)}d`;
  if (daysLeft === 0) return 'Due today';
  if (daysLeft === 1) return '1 day left';
  return `${daysLeft} days left`;
}

export function computeSLAExpiration(
  severity: Exclude<Severity, 'Info'>,
  createdAt: string,
  customDays?: number
): string {
  const days = customDays ?? SLA_DAYS_BY_SEVERITY[severity];
  const exp = new Date(createdAt);
  exp.setDate(exp.getDate() + days);
  return exp.toISOString();
}
```

### File 3: `ui/src/shared/utils/productGrade.ts`

```typescript
export type ProductGrade = 'A' | 'A-' | 'B+' | 'B' | 'B-' | 'C+' | 'C' | 'D' | 'F';

export const GRADE_COLORS: Record<ProductGrade, string> = {
  'A': '#10B981',
  'A-': '#34D399',
  'B+': '#60A5FA',
  'B': '#3B82F6',
  'B-': '#818CF8',
  'C+': '#EAB308',
  'C': '#F97316',
  'D': '#EF4444',
  'F': '#7F1D1D',
};

export function calculateProductGrade(findings: {
  critical: number;
  high: number;
  medium: number;
  total: number;
}): { grade: ProductGrade; score: number } {
  const { critical, high, medium } = findings;

  // Score = 100 - penalties
  const score = Math.max(
    0,
    100 - (critical * 15) - (high * 5) - (medium * 1)
  );

  let grade: ProductGrade;
  if (score >= 95) grade = 'A';
  else if (score >= 90) grade = 'A-';
  else if (score >= 85) grade = 'B+';
  else if (score >= 80) grade = 'B';
  else if (score >= 75) grade = 'B-';
  else if (score >= 65) grade = 'C+';
  else if (score >= 55) grade = 'C';
  else if (score >= 40) grade = 'D';
  else grade = 'F';

  return { grade, score };
}

export function getGradeColor(grade: ProductGrade): string {
  return GRADE_COLORS[grade] ?? '#6B7280';
}
```

### File 4: `ui/src/shared/utils/findingStateMachine.ts`

```typescript
import type { FindingStatus } from '@/shared/types/finding';

export const VALID_TRANSITIONS: Record<FindingStatus, FindingStatus[]> = {
  active: ['mitigated', 'false_positive', 'risk_accepted', 'out_of_scope'],
  mitigated: ['active'],
  false_positive: ['active'],
  risk_accepted: ['active'],
  out_of_scope: ['active'],
  duplicate: [],
};

export const STATUS_LABELS: Record<FindingStatus, string> = {
  active: 'Active',
  mitigated: 'Mitigated',
  false_positive: 'False Positive',
  risk_accepted: 'Risk Accepted',
  out_of_scope: 'Out of Scope',
  duplicate: 'Duplicate',
};

export const STATUS_COLORS: Record<FindingStatus, string> = {
  active: '#EF4444',
  mitigated: '#10B981',
  false_positive: '#6B7280',
  risk_accepted: '#8B5CF6',
  out_of_scope: '#6B7280',
  duplicate: '#374151',
};

export function canTransition(from: FindingStatus, to: FindingStatus): boolean {
  return VALID_TRANSITIONS[from].includes(to);
}

export function getAvailableTransitions(current: FindingStatus): FindingStatus[] {
  return VALID_TRANSITIONS[current];
}
```

### File 5: `ui/src/shared/utils/date.ts`

```typescript
export function formatDate(iso: string, options?: Intl.DateTimeFormatOptions): string {
  return new Date(iso).toLocaleDateString('en-US', options ?? {
    month: 'short',
    day: 'numeric',
    year: 'numeric',
  });
}

export function formatRelativeTime(iso: string): string {
  const now = Date.now();
  const diff = now - new Date(iso).getTime();
  const minutes = Math.floor(diff / 60_000);
  const hours = Math.floor(diff / 3_600_000);
  const days = Math.floor(diff / 86_400_000);

  if (minutes < 1) return 'just now';
  if (minutes < 60) return `${minutes}m ago`;
  if (hours < 24) return `${hours}h ago`;
  if (days < 7) return `${days}d ago`;
  return formatDate(iso);
}

export function formatDuration(ms: number): string {
  const seconds = Math.floor(ms / 1_000);
  const minutes = Math.floor(seconds / 60);
  const hours = Math.floor(minutes / 60);

  if (hours > 0) return `${hours}h ${minutes % 60}m`;
  if (minutes > 0) return `${minutes}m ${seconds % 60}s`;
  return `${seconds}s`;
}
```

### File 6: `ui/src/shared/utils/cn.ts`

```typescript
// Class name utility — merge Tailwind classes safely
// Simple implementation without clsx/twMerge dep (can upgrade if needed)
export function cn(...classes: (string | undefined | null | false)[]): string {
  return classes.filter(Boolean).join(' ');
}
```

### File 7: `ui/src/shared/components/feedback/LoadingSpinner.tsx`

```typescript
interface LoadingSpinnerProps {
  size?: 'sm' | 'md' | 'lg';
  label?: string;
}

const SIZES = { sm: 'w-5 h-5', md: 'w-8 h-8', lg: 'w-12 h-12' };

export function LoadingSpinner({ size = 'md', label }: LoadingSpinnerProps) {
  return (
    <div className="flex flex-col items-center justify-center gap-3">
      <div
        className={`${SIZES[size]} rounded-full border-2 animate-spin`}
        style={{ borderColor: '#4F8CFF', borderTopColor: 'transparent' }}
      />
      {label && (
        <span style={{ color: '#6B7280', fontSize: 13 }}>{label}</span>
      )}
    </div>
  );
}

export function FullPageSpinner({ label }: { label?: string }) {
  return (
    <div
      className="w-full h-screen flex items-center justify-center"
      style={{ background: '#0B1020' }}
    >
      <LoadingSpinner size="lg" label={label ?? 'Loading...'} />
    </div>
  );
}
```

### File 8: `ui/src/shared/components/feedback/ErrorState.tsx`

```typescript
interface ErrorStateProps {
  message?: string;
  onRetry?: () => void;
  fullPage?: boolean;
}

export function ErrorState({
  message = 'An unexpected error occurred',
  onRetry,
  fullPage = false,
}: ErrorStateProps) {
  const content = (
    <div className="flex flex-col items-center justify-center gap-4 p-8">
      <div
        className="w-12 h-12 rounded-full flex items-center justify-center text-xl"
        style={{ background: 'rgba(239,68,68,0.15)' }}
      >
        ⚠️
      </div>
      <div className="text-center">
        <p style={{ color: '#E5E7EB', fontSize: 14, fontWeight: 500 }}>Something went wrong</p>
        <p style={{ color: '#6B7280', fontSize: 12, marginTop: 4 }}>{message}</p>
      </div>
      {onRetry && (
        <button
          onClick={onRetry}
          style={{
            padding: '8px 20px',
            borderRadius: 10,
            background: 'rgba(79,140,255,0.1)',
            border: '1px solid rgba(79,140,255,0.3)',
            color: '#4F8CFF',
            fontSize: 13,
            cursor: 'pointer',
          }}
        >
          Try again
        </button>
      )}
    </div>
  );

  if (fullPage) {
    return (
      <div
        className="w-full h-screen flex items-center justify-center"
        style={{ background: '#0B1020' }}
      >
        {content}
      </div>
    );
  }

  return content;
}
```

### File 9: `ui/src/shared/components/feedback/EmptyState.tsx`

```typescript
interface EmptyStateProps {
  icon?: string;
  title?: string;
  description?: string;
  action?: { label: string; onClick: () => void };
}

export function EmptyState({
  icon = '📭',
  title = 'No data found',
  description = 'Try adjusting your filters or search terms.',
  action,
}: EmptyStateProps) {
  return (
    <div className="flex flex-col items-center justify-center gap-4 p-12">
      <div style={{ fontSize: 32 }}>{icon}</div>
      <div className="text-center">
        <p style={{ color: '#E5E7EB', fontSize: 14, fontWeight: 500 }}>{title}</p>
        <p style={{ color: '#6B7280', fontSize: 12, marginTop: 4 }}>{description}</p>
      </div>
      {action && (
        <button
          onClick={action.onClick}
          style={{
            padding: '8px 20px',
            borderRadius: 10,
            background: 'rgba(79,140,255,0.1)',
            border: '1px solid rgba(79,140,255,0.3)',
            color: '#4F8CFF',
            fontSize: 13,
            cursor: 'pointer',
          }}
        >
          {action.label}
        </button>
      )}
    </div>
  );
}
```

### File 10: `ui/src/shared/components/feedback/QueryBoundary.tsx`

```typescript
import type { UseQueryResult } from '@tanstack/react-query';
import { LoadingSpinner } from './LoadingSpinner';
import { ErrorState } from './ErrorState';
import { EmptyState } from './EmptyState';

interface QueryBoundaryProps<T> {
  query: UseQueryResult<T>;
  skeleton?: React.ReactNode;
  children: (data: NonNullable<T>) => React.ReactNode;
  emptyCheck?: (data: T) => boolean;
  emptyState?: React.ReactNode;
}

/**
 * Wraps a React Query result and handles loading, error, and empty states.
 * Usage:
 *   <QueryBoundary query={myQuery} skeleton={<Skeleton />}>
 *     {(data) => <MyComponent data={data} />}
 *   </QueryBoundary>
 */
export function QueryBoundary<T>({
  query,
  skeleton,
  children,
  emptyCheck,
  emptyState,
}: QueryBoundaryProps<T>) {
  if (query.isLoading) {
    return skeleton ? (
      <>{skeleton}</>
    ) : (
      <div className="flex-1 flex items-center justify-center p-12">
        <LoadingSpinner size="lg" />
      </div>
    );
  }

  if (query.isError) {
    return (
      <ErrorState
        message={
          (query.error as { message?: string })?.message ??
          'Failed to load data'
        }
        onRetry={() => query.refetch()}
      />
    );
  }

  if (!query.data) {
    return emptyState ? <>{emptyState}</> : <EmptyState />;
  }

  if (emptyCheck && emptyCheck(query.data)) {
    return emptyState ? <>{emptyState}</> : <EmptyState />;
  }

  return <>{children(query.data as NonNullable<T>)}</>;
}
```

### File 11: `ui/src/shared/components/data-display/SeverityBadge.tsx`

```typescript
import type { Severity } from '@/shared/types/cve';
import { SEVERITY_COLORS, SEVERITY_BG_COLORS } from '@/shared/utils/severity';

interface SeverityBadgeProps {
  severity: Severity;
  showDot?: boolean;
}

export function SeverityBadge({ severity, showDot = false }: SeverityBadgeProps) {
  return (
    <span
      style={{
        display: 'inline-flex',
        alignItems: 'center',
        gap: 4,
        background: SEVERITY_BG_COLORS[severity],
        color: SEVERITY_COLORS[severity],
        padding: '2px 8px',
        borderRadius: 6,
        fontSize: 11,
        fontWeight: 600,
        letterSpacing: 0.3,
      }}
    >
      {showDot && (
        <span
          style={{
            width: 5,
            height: 5,
            borderRadius: '50%',
            background: SEVERITY_COLORS[severity],
          }}
        />
      )}
      {severity}
    </span>
  );
}
```

### File 12: `ui/src/shared/components/data-display/KEVIndicator.tsx`

```typescript
interface KEVIndicatorProps {
  isKEV: boolean;
  compact?: boolean;
}

export function KEVIndicator({ isKEV, compact = false }: KEVIndicatorProps) {
  if (!isKEV) {
    return compact ? null : (
      <span style={{ color: '#4B5563', fontSize: 11 }}>—</span>
    );
  }

  return (
    <span
      style={{
        display: 'inline-flex',
        alignItems: 'center',
        gap: compact ? 0 : 4,
        background: 'rgba(239,68,68,0.15)',
        color: '#EF4444',
        padding: compact ? '2px 5px' : '2px 7px',
        borderRadius: 5,
        fontSize: 10,
        fontWeight: 700,
        letterSpacing: 0.5,
        border: '1px solid rgba(239,68,68,0.3)',
      }}
      title="CISA Known Exploited Vulnerability"
    >
      🔴{!compact && ' KEV'}
    </span>
  );
}
```

### File 13: `ui/src/shared/components/data-display/EPSSBar.tsx`

```typescript
interface EPSSBarProps {
  score: number;              // 0.0 - 1.0
  showLabel?: boolean;
  width?: number;
}

export function EPSSBar({ score, showLabel = true, width = 60 }: EPSSBarProps) {
  const pct = Math.round(score * 100);
  const color = pct >= 50 ? '#EF4444' : pct >= 20 ? '#F97316' : '#6B7280';

  return (
    <div style={{ display: 'inline-flex', alignItems: 'center', gap: 6 }}>
      <div
        style={{
          width,
          height: 4,
          borderRadius: 2,
          background: 'rgba(255,255,255,0.08)',
          overflow: 'hidden',
        }}
      >
        <div
          style={{
            width: `${pct}%`,
            height: '100%',
            background: color,
            borderRadius: 2,
            transition: 'width 0.3s ease',
          }}
        />
      </div>
      {showLabel && (
        <span style={{ color, fontSize: 11, fontWeight: 600, minWidth: 32 }}>
          {pct}%
        </span>
      )}
    </div>
  );
}
```

### File 14: `ui/src/shared/components/data-display/SLABadge.tsx`

```typescript
import type { SLAStatus } from '@/shared/types/finding';
import { SLA_STATUS_COLORS, formatSLALabel } from '@/shared/utils/sla';

interface SLABadgeProps {
  status: SLAStatus;
  daysLeft?: number;
  showIcon?: boolean;
}

const ICONS: Record<SLAStatus, string> = {
  ok: '✓',
  at_risk: '⚠',
  breached: '✕',
};

export function SLABadge({ status, daysLeft, showIcon = true }: SLABadgeProps) {
  const color = SLA_STATUS_COLORS[status];

  return (
    <span
      style={{
        display: 'inline-flex',
        alignItems: 'center',
        gap: 4,
        color,
        fontSize: 11,
        fontWeight: 600,
      }}
    >
      {showIcon && <span>{ICONS[status]}</span>}
      {daysLeft !== undefined ? formatSLALabel(daysLeft) : status}
    </span>
  );
}
```

### File 15: `ui/src/shared/components/data-display/StatusBadge.tsx`

```typescript
import type { FindingStatus } from '@/shared/types/finding';
import { STATUS_LABELS, STATUS_COLORS } from '@/shared/utils/findingStateMachine';

export function StatusBadge({ status }: { status: FindingStatus }) {
  const color = STATUS_COLORS[status];

  return (
    <span
      style={{
        display: 'inline-flex',
        alignItems: 'center',
        gap: 4,
        background: `${color}20`,
        color,
        padding: '2px 8px',
        borderRadius: 6,
        fontSize: 11,
        fontWeight: 600,
      }}
    >
      {STATUS_LABELS[status]}
    </span>
  );
}
```

### File 16: `ui/src/shared/components/data-display/GradeCircle.tsx`

```typescript
import type { ProductGrade } from '@/shared/utils/productGrade';
import { GRADE_COLORS } from '@/shared/utils/productGrade';

interface GradeCircleProps {
  grade: ProductGrade;
  size?: number;
}

export function GradeCircle({ grade, size = 48 }: GradeCircleProps) {
  const color = GRADE_COLORS[grade];

  return (
    <div
      style={{
        width: size,
        height: size,
        borderRadius: '50%',
        border: `2px solid ${color}`,
        background: `${color}15`,
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'center',
        color,
        fontSize: size * 0.38,
        fontWeight: 700,
        fontFamily: "'Inter', sans-serif",
      }}
    >
      {grade}
    </div>
  );
}
```

---

## Verification

```bash
cd ui/
npx tsc --noEmit
# Expected: No errors

# Kiểm tra files đã tạo
ls src/shared/utils/
# Expected: cn.ts  date.ts  findingStateMachine.ts  productGrade.ts  severity.ts  sla.ts

ls src/shared/components/feedback/
# Expected: EmptyState.tsx  ErrorState.tsx  LoadingSpinner.tsx  QueryBoundary.tsx

ls src/shared/components/data-display/
# Expected: EPSSBar.tsx  GradeCircle.tsx  KEVIndicator.tsx  SLABadge.tsx  SeverityBadge.tsx  StatusBadge.tsx
```

---

- [x] `shared/utils/severity.ts` — SEVERITY_COLORS, SEVERITY_BG_COLORS, getCVSSColor, sortBySeverity
- [x] `shared/utils/sla.ts` — getSLAStatus, getSLADaysLeft, formatSLALabel, computeSLAExpiration
- [x] `shared/utils/productGrade.ts` — calculateProductGrade, GRADE_COLORS
- [x] `shared/utils/findingStateMachine.ts` — canTransition, VALID_TRANSITIONS, STATUS_LABELS, STATUS_COLORS
- [x] `shared/utils/date.ts` — formatDate, formatRelativeTime, formatDuration
- [x] `shared/utils/cn.ts` — class name merger
- [x] `shared/components/feedback/QueryBoundary.tsx` — handles loading/error/empty
- [x] `shared/components/feedback/LoadingSpinner.tsx` — spinner + FullPageSpinner
- [x] `shared/components/feedback/ErrorState.tsx` — error + retry button
- [x] `shared/components/feedback/EmptyState.tsx` — empty state with action
- [x] `shared/components/data-display/SeverityBadge.tsx`
- [x] `shared/components/data-display/KEVIndicator.tsx`
- [x] `shared/components/data-display/EPSSBar.tsx`
- [x] `shared/components/data-display/SLABadge.tsx`
- [x] `shared/components/data-display/StatusBadge.tsx`
- [x] `shared/components/data-display/GradeCircle.tsx`
- [x] `npx tsc --noEmit` không có lỗi (chỉ 1 pre-existing trong AIEnrichment.tsx)
