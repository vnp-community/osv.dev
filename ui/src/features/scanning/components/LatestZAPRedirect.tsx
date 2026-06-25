import { Navigate, useNavigate } from 'react-router';
import { useRecentScans } from '../hooks/useLatestScan';
import { ChevronRight, Calendar, Target, Activity } from 'lucide-react';

/**
 * Component for the sidebar "ZAP Results" menu item.
 *
 * Resolves recent ZAP scans via API:
 *   - If only 1 exists, redirects directly to its results.
 *   - If multiple exist, displays a list for the user to choose.
 *   - Falls back to a "No Scans Found" message when no scans exist.
 */
export function LatestZAPRedirect() {
  const navigate = useNavigate();
  const { data: scans, isLoading, isError } = useRecentScans('zap');

  if (isLoading) {
    return (
      <div className="flex flex-1 items-center justify-center" style={{ color: '#6B7280', fontSize: 14 }}>
        Loading ZAP results…
      </div>
    );
  }

  if (isError || !scans || scans.length === 0) {
    return (
      <div className="flex flex-col items-center justify-center flex-1 space-y-4 p-8">
        <div className="w-16 h-16 rounded-full bg-slate-800/50 flex items-center justify-center">
          <span className="text-slate-400 text-2xl">Z</span>
        </div>
        <h2 className="text-lg font-medium text-white">No ZAP Scans Found</h2>
        <p className="text-sm text-slate-400 text-center max-w-sm">
          There are no completed scans in the system yet. Please create and run a new scan to view ZAP results.
        </p>
      </div>
    );
  }

  // If there's exactly 1 scan, we can still automatically redirect
  if (scans.length === 1) {
    const singleScan = scans[0];
    const scanId = singleScan.id || (singleScan as any).scan_id || (singleScan as any).scanId || (singleScan as any).ID || (singleScan as any)._id;
    if (scanId) {
      return <Navigate to={`/scans/${scanId}/results/zap`} replace />;
    }
  }

  // If there are multiple scans, render a list for the user to select
  return (
    <div className="flex-1 overflow-y-auto px-6 py-5" style={{ background: "var(--color-bg-page, #0B1020)" }}>
      <div className="mb-6">
        <h2 style={{ color: "var(--color-text-primary, #E5E7EB)", fontSize: 20, fontWeight: 700 }}>Select ZAP Scan Results</h2>
        <p style={{ color: "var(--color-text-muted, #6B7280)", fontSize: 13, marginTop: 4 }}>
          Multiple ZAP scans were found. Please select one to view its detailed results.
        </p>
      </div>

      <div className="grid grid-cols-1 gap-3 max-w-4xl">
        {scans.map((scan) => {
          const scanId = scan.id || (scan as any).scan_id || (scan as any).scanId || (scan as any).ID || (scan as any)._id;
          if (!scanId) return null;

          const dateStr = scan.startedAt || scan.completedAt;
          const formattedDate = dateStr ? new Date(dateStr).toLocaleString('en-GB', { dateStyle: 'medium', timeStyle: 'short' }) : 'Unknown Date';

          return (
            <button
              key={scanId}
              onClick={() => navigate(`/scans/${scanId}/results/zap`)}
              className="w-full flex items-center justify-between p-4 rounded-xl text-left transition-all border"
              style={{
                background: "var(--color-bg-card, #151B2F)",
                borderColor: "var(--color-border-subtle, rgba(255,255,255,0.07))",
                cursor: "pointer",
              }}
              onMouseEnter={(e) => (e.currentTarget.style.background = "var(--color-bg-hover, rgba(255,255,255,0.02))")}
              onMouseLeave={(e) => (e.currentTarget.style.background = "var(--color-bg-card, #151B2F)")}
            >
              <div className="flex flex-col gap-2">
                <div className="flex items-center gap-3">
                  <span style={{ color: "#E5E7EB", fontSize: 15, fontWeight: 600 }}>{scan.name}</span>
                  <span
                    className="px-2 py-0.5 rounded"
                    style={{
                      background: scan.status === "completed" ? "rgba(16,185,129,0.1)" : "rgba(239,68,68,0.1)",
                      color: scan.status === "completed" ? "#10B981" : "#EF4444",
                      fontSize: 11,
                    }}
                  >
                    {scan.status}
                  </span>
                </div>
                
                <div className="flex items-center gap-4 mt-1">
                  <div className="flex items-center gap-1.5" style={{ color: "#9CA3AF", fontSize: 12 }}>
                    <Calendar size={13} />
                    {formattedDate}
                  </div>
                  <div className="flex items-center gap-1.5" style={{ color: "#9CA3AF", fontSize: 12 }}>
                    <Target size={13} />
                    {scan.targets?.[0] || 'Unknown Target'}
                    {scan.targets?.length > 1 && ` (+${scan.targets.length - 1})`}
                  </div>
                  <div className="flex items-center gap-1.5" style={{ color: scan.findingCount > 0 ? "#F59E0B" : "#10B981", fontSize: 12, fontWeight: 500 }}>
                    <Activity size={13} />
                    {scan.findingCount || 0} Findings
                  </div>
                </div>
              </div>

              <div className="w-8 h-8 rounded-full flex items-center justify-center" style={{ background: "rgba(255,255,255,0.05)", color: "#9CA3AF" }}>
                <ChevronRight size={16} />
              </div>
            </button>
          );
        })}
      </div>
    </div>
  );
}
