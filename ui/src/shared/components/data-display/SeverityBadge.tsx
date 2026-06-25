import type { Severity } from '@/shared/types/cve';
import { SEVERITY_COLORS, SEVERITY_BG_COLORS } from '@/shared/utils/severity';

interface SeverityBadgeProps {
  severity: Severity;
  showDot?: boolean;
}

export function SeverityBadge({ severity, showDot = false }: SeverityBadgeProps) {
  return (
    <span
      data-testid="severity-badge"
      className="severity-badge"
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
