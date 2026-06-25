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
