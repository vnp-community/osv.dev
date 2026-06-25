function SkeletonBox({
  w, h, className = '',
}: {
  w?: string; h: string; className?: string;
}) {
  return (
    <div
      data-testid="skeleton-box"
      className={`rounded-xl animate-pulse ${className}`}
      style={{ width: w, height: h, background: 'rgba(255,255,255,0.06)' }}
    />
  );
}

export function DashboardSkeleton() {
  return (
    <div
      className="flex-1 overflow-y-auto p-6"
      data-testid="dashboard-skeleton"
      style={{ background: 'var(--color-bg-page, #0B1020)' }}
    >
      {/* Header */}
      <div className="flex items-center justify-between mb-6">
        <SkeletonBox h="28px" w="300px" />
        <SkeletonBox h="36px" w="200px" />
      </div>

      {/* KPI Row skeleton (6 cards) */}
      <div className="grid grid-cols-6 gap-4 mb-6">
        {Array.from({ length: 6 }).map((_, i) => (
          <div
            key={i}
            className="rounded-2xl p-5"
            style={{
              background: 'var(--color-bg-card, #151B2F)',
              border: '1px solid rgba(255,255,255,0.07)',
            }}
          >
            <SkeletonBox h="36px" w="36px" className="mb-4 rounded-xl" />
            <SkeletonBox h="24px" w="70px" className="mb-2" />
            <SkeletonBox h="12px" w="50px" />
          </div>
        ))}
      </div>

      {/* Charts row skeleton */}
      <div className="grid grid-cols-3 gap-4 mb-6">
        <SkeletonBox h="280px" className="col-span-2 rounded-2xl" />
        <SkeletonBox h="280px" className="rounded-2xl" />
      </div>

      {/* Bottom row skeleton */}
      <div className="grid grid-cols-2 gap-4">
        <SkeletonBox h="300px" className="rounded-2xl" />
        <SkeletonBox h="300px" className="rounded-2xl" />
      </div>
    </div>
  );
}
