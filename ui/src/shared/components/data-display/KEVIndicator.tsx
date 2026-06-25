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
