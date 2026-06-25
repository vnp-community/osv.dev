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
