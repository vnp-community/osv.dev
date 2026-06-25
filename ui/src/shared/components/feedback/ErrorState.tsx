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
