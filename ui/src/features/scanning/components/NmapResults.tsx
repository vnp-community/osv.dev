import { useState } from "react";
import { useParams } from "react-router";
import { Server, Shield, ChevronRight, Wifi, Loader2, AlertTriangle } from "lucide-react";
import { useNmapResults } from "../hooks/useScanResults";
import type { NmapHost } from "../hooks/useScanResults";

const RISK_COLOR = (s: number) =>
  s >= 9 ? "#EF4444" : s >= 7 ? "#F97316" : s >= 4 ? "#EAB308" : "#10B981";

// ── Skeleton ───────────────────────────────────────────────────────────────────

function NmapSkeleton() {
  return (
    <div className="flex flex-1 overflow-hidden" style={{ background: "var(--color-bg-page, #0B1020)" }}>
      <div className="w-64 flex-shrink-0" style={{ background: "var(--color-bg-sidebar, #0F1629)", borderRight: "1px solid rgba(255,255,255,0.06)" }}>
        {[...Array(5)].map((_, i) => (
          <div key={i} className="px-4 py-3" style={{ borderBottom: "1px solid rgba(255,255,255,0.04)" }}>
            <div className="h-3 rounded animate-pulse mb-2" style={{ background: "rgba(255,255,255,0.08)", width: "60%" }} />
            <div className="h-2.5 rounded animate-pulse" style={{ background: "rgba(255,255,255,0.05)", width: "80%" }} />
          </div>
        ))}
      </div>
      <div className="flex-1 p-5">
        <div className="h-6 w-48 rounded animate-pulse mb-4" style={{ background: "rgba(255,255,255,0.08)" }} />
        <div className="h-40 rounded-2xl animate-pulse" style={{ background: "rgba(255,255,255,0.05)" }} />
      </div>
    </div>
  );
}

// ── Main Component ─────────────────────────────────────────────────────────────

export function NmapResults({ onBack, scanId: propScanId }: { onBack?: () => void; scanId?: string }) {
  // Support both prop-passed scanId and URL param
  const params = useParams<{ id: string }>();
  const scanId = propScanId ?? params.id;

  const { data, isLoading, isError } = useNmapResults(scanId);
  const hosts: NmapHost[] = data?.hosts ?? [];

  const [selected, setSelected] = useState<NmapHost | null>(null);
  const activeHost = selected ?? hosts[0] ?? null;

  if (isLoading) return <NmapSkeleton />;

  if (isError) {
    return (
      <div className="flex flex-1 items-center justify-center" style={{ background: "var(--color-bg-page, #0B1020)" }}>
        <div className="flex flex-col items-center gap-3">
          <AlertTriangle size={28} color="#EF4444" />
          <p style={{ color: "#9CA3AF", fontSize: 14 }}>Failed to load Nmap results.</p>
        </div>
      </div>
    );
  }

  if (!hosts.length) {
    return (
      <div className="flex flex-1 items-center justify-center" style={{ background: "var(--color-bg-page, #0B1020)" }}>
        <p style={{ color: "#6B7280", fontSize: 14 }}>No hosts found for this scan.</p>
      </div>
    );
  }

  return (
    <div className="flex flex-1 overflow-hidden" style={{ background: "var(--color-bg-page, #0B1020)" }}>
      {/* Host list */}
      <div className="w-64 flex-shrink-0 overflow-y-auto" style={{ background: "var(--color-bg-sidebar, #0F1629)", borderRight: "1px solid rgba(255,255,255,0.06)" }}>
        <div className="px-4 py-3" style={{ borderBottom: "1px solid rgba(255,255,255,0.06)" }}>
          <div style={{ color: "var(--color-text-muted, #6B7280)", fontSize: 11, fontWeight: 600, letterSpacing: 1 }}>HOSTS ({hosts.length})</div>
        </div>
        {hosts.map(h => (
          <button key={h.ip} onClick={() => setSelected(h)}
            className="w-full flex items-start gap-3 px-4 py-3 text-left transition-all"
            style={{ background: activeHost?.ip === h.ip ? "rgba(79,140,255,0.1)" : "transparent", borderBottom: "1px solid rgba(255,255,255,0.04)", borderLeft: activeHost?.ip === h.ip ? "2px solid #4F8CFF" : "2px solid transparent" }}
          >
            <div className="w-2 h-2 rounded-full mt-1.5 flex-shrink-0" style={{ background: RISK_COLOR(h.riskScore) }} />
            <div className="min-w-0">
              <div style={{ color: "var(--color-text-primary, #E5E7EB)", fontSize: 12, fontFamily: "monospace" }}>{h.ip}</div>
              <div style={{ color: "var(--color-text-muted, #6B7280)", fontSize: 11 }} className="truncate">{h.hostname}</div>
              <div className="flex items-center gap-2 mt-1">
                <span style={{ color: "var(--color-text-muted, #6B7280)", fontSize: 10 }}>{h.ports.length} ports</span>
                {h.cves.length > 0 && <span style={{ color: "var(--color-status-error, #EF4444)", fontSize: 10 }}>{h.cves.length} CVEs</span>}
              </div>
            </div>
          </button>
        ))}
      </div>

      {/* Host detail */}
      {activeHost && (
        <div className="flex-1 overflow-y-auto p-5">
          <div className="flex items-center gap-4 mb-5">
            <div className="w-12 h-12 rounded-xl flex items-center justify-center" style={{ background: RISK_COLOR(activeHost.riskScore) + "20" }}>
              <Server size={22} color={RISK_COLOR(activeHost.riskScore)} />
            </div>
            <div>
              <div style={{ color: "var(--color-text-primary, #E5E7EB)", fontSize: 18, fontFamily: "monospace", fontWeight: 700 }}>{activeHost.ip}</div>
              <div style={{ color: "var(--color-text-muted, #6B7280)", fontSize: 13 }}>{activeHost.hostname} · {activeHost.os}</div>
            </div>
            <div className="ml-auto flex items-center gap-3">
              <div className="text-center">
                <div style={{ color: RISK_COLOR(activeHost.riskScore), fontSize: 24, fontWeight: 800 }}>{activeHost.riskScore}</div>
                <div style={{ color: "var(--color-text-muted, #6B7280)", fontSize: 10 }}>Risk Score</div>
              </div>
            </div>
          </div>

          {/* Open ports */}
          <div className="rounded-2xl mb-4" style={{ background: "var(--color-bg-card, #151B2F)", border: "1px solid rgba(255,255,255,0.07)" }}>
            <div className="px-5 py-3" style={{ borderBottom: "1px solid rgba(255,255,255,0.06)" }}>
              <div style={{ color: "var(--color-text-primary, #E5E7EB)", fontSize: 13, fontWeight: 600 }}>Open Ports ({activeHost.ports.length})</div>
            </div>
            <table className="w-full">
              <thead>
                <tr style={{ borderBottom: "1px solid rgba(255,255,255,0.05)" }}>
                  {["Port", "Protocol", "Service", "Version", "State"].map(h => (
                    <th key={h} className="px-4 py-2 text-left" style={{ color: "var(--color-text-muted, #6B7280)", fontSize: 11, fontWeight: 600 }}>{h}</th>
                  ))}
                </tr>
              </thead>
              <tbody>
                {activeHost.ports.map(p => (
                  <tr key={p.port} style={{ borderBottom: "1px solid rgba(255,255,255,0.04)" }}>
                    <td className="px-4 py-2.5"><span style={{ color: "var(--color-primary, #4F8CFF)", fontSize: 13, fontFamily: "monospace", fontWeight: 600 }}>{p.port}</span></td>
                    <td className="px-4 py-2.5"><span style={{ color: "var(--color-text-muted, #6B7280)", fontSize: 12 }}>{p.proto}</span></td>
                    <td className="px-4 py-2.5"><span style={{ color: "var(--color-text-primary, #E5E7EB)", fontSize: 12 }}>{p.service}</span></td>
                    <td className="px-4 py-2.5"><span style={{ color: "var(--color-text-secondary, #9CA3AF)", fontSize: 12 }}>{p.version}</span></td>
                    <td className="px-4 py-2.5"><span style={{ color: "var(--color-status-success, #10B981)", fontSize: 11 }}>{p.state}</span></td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>

          {/* Detected CVEs */}
          {activeHost.cves.length > 0 && (
            <div className="rounded-2xl" style={{ background: "var(--color-bg-card, #151B2F)", border: "1px solid rgba(239,68,68,0.2)" }}>
              <div className="px-5 py-3" style={{ borderBottom: "1px solid rgba(255,255,255,0.06)" }}>
                <div style={{ color: "var(--color-status-error, #EF4444)", fontSize: 13, fontWeight: 600 }}>Detected CVEs ({activeHost.cves.length})</div>
              </div>
              <div className="p-4 flex flex-col gap-2">
                {activeHost.cves.map(cve => (
                  <div key={cve} className="flex items-center gap-3 px-3 py-2.5 rounded-xl" style={{ background: "rgba(239,68,68,0.07)", border: "1px solid rgba(239,68,68,0.12)" }}>
                    <Shield size={13} color="#EF4444" />
                    <span style={{ color: "var(--color-status-error, #EF4444)", fontSize: 12, fontFamily: "monospace", fontWeight: 600 }}>{cve}</span>
                    <span className="ml-auto px-2 py-0.5 rounded" style={{ background: "rgba(239,68,68,0.2)", color: "var(--color-status-error, #EF4444)", fontSize: 10 }}>CRITICAL</span>
                  </div>
                ))}
              </div>
            </div>
          )}
        </div>
      )}
    </div>
  );
}
