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
