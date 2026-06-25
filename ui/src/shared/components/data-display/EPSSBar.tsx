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
