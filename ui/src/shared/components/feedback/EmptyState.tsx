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
