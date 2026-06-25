import { useState } from "react";
import { useParams } from "react-router";
import { AlertTriangle, Globe, Loader2 } from "lucide-react";
import { useZAPResults } from "../hooks/useScanResults";
import type { ZapAlert } from "../hooks/useScanResults";

const RISK_COLORS: Record<string, string> = {
  Critical: "#EF4444",
  High:     "#F97316",
  Medium:   "#EAB308",
  Low:      "#3B82F6",
};
const TABS = ["Alerts", "Risk Breakdown"];

// ── Skeleton ───────────────────────────────────────────────────────────────────

function ZAPSkeleton() {
  return (
    <div className="flex flex-col flex-1 overflow-hidden" style={{ background: "var(--color-bg-page, #0B1020)" }}>
      <div className="px-5 py-4" style={{ borderBottom: "1px solid rgba(255,255,255,0.06)" }}>
        <div className="h-5 w-48 rounded animate-pulse mb-3" style={{ background: "rgba(255,255,255,0.08)" }} />
        <div className="grid grid-cols-4 gap-3">
          {[...Array(4)].map((_, i) => (
            <div key={i} className="h-14 rounded-xl animate-pulse" style={{ background: "rgba(255,255,255,0.05)" }} />
          ))}
        </div>
      </div>
      <div className="flex flex-1 overflow-hidden">
        <div className="w-80" style={{ borderRight: "1px solid rgba(255,255,255,0.06)" }}>
          {[...Array(6)].map((_, i) => (
            <div key={i} className="p-4" style={{ borderBottom: "1px solid rgba(255,255,255,0.04)" }}>
              <div className="h-3 w-16 rounded animate-pulse mb-2" style={{ background: "rgba(255,255,255,0.08)" }} />
              <div className="h-3 w-48 rounded animate-pulse" style={{ background: "rgba(255,255,255,0.05)" }} />
            </div>
          ))}
        </div>
        <div className="flex-1 p-5">
          <div className="h-6 w-64 rounded animate-pulse mb-4" style={{ background: "rgba(255,255,255,0.08)" }} />
          <div className="h-32 rounded-2xl animate-pulse" style={{ background: "rgba(255,255,255,0.05)" }} />
        </div>
      </div>
    </div>
  );
}

// ── Main Component ─────────────────────────────────────────────────────────────

export function ZAPResults({ scanId: propScanId }: { scanId?: string }) {
  const params = useParams<{ id: string }>();
  const scanId = propScanId ?? params.id;

  const { data, isLoading, isError } = useZAPResults(scanId);
  const alerts: ZapAlert[] = data?.alerts ?? [];

  const [selected, setSelected] = useState<ZapAlert | null>(null);
  const [activeTab, setActiveTab] = useState("Alerts");

  const activeAlert = selected ?? alerts[0] ?? null;

  if (isLoading) return <ZAPSkeleton />;

  if (isError) {
    return (
      <div className="flex flex-1 items-center justify-center" style={{ background: "var(--color-bg-page, #0B1020)" }}>
        <div className="flex flex-col items-center gap-3">
          <AlertTriangle size={28} color="#EF4444" />
          <p style={{ color: "#9CA3AF", fontSize: 14 }}>Failed to load ZAP results.</p>
        </div>
      </div>
    );
  }

  // Compute breakdown from live data
  const breakdown = Object.entries(
    alerts.reduce((acc, a) => {
      acc[a.risk] = (acc[a.risk] ?? 0) + a.count;
      return acc;
    }, {} as Record<string, number>)
  );

  const riskSummary = (["Critical", "High", "Medium", "Low"] as const).map(label => ({
    label,
    val: alerts.filter(a => a.risk === label).reduce((s, a) => s + a.count, 0),
    color: RISK_COLORS[label],
  }));

  return (
    <div className="flex flex-col flex-1 overflow-hidden" style={{ background: "var(--color-bg-page, #0B1020)" }}>
      {/* Header */}
      <div className="px-5 py-4" style={{ borderBottom: "1px solid rgba(255,255,255,0.06)" }}>
        <div className="flex items-center justify-between mb-3">
          <div className="flex items-center gap-3">
            <Globe size={20} color="#F97316" />
            <div>
              <h2 style={{ color: "var(--color-text-primary, #E5E7EB)", fontSize: 16, fontWeight: 700 }}>OWASP ZAP Results</h2>
              <p style={{ color: "var(--color-text-muted, #6B7280)", fontSize: 12 }}>Scan {scanId ?? "—"}</p>
            </div>
          </div>
          <div className="grid grid-cols-4 gap-3">
            {riskSummary.map(s => (
              <div key={s.label} className="rounded-xl px-3 py-2 text-center" style={{ background: s.color + "10" }}>
                <div style={{ color: s.color, fontSize: 18, fontWeight: 700 }}>{s.val}</div>
                <div style={{ color: "var(--color-text-muted, #6B7280)", fontSize: 10 }}>{s.label}</div>
              </div>
            ))}
          </div>
        </div>
        <div className="flex gap-1">
          {TABS.map(t => (
            <button key={t} onClick={() => setActiveTab(t)} className="px-4 py-1.5 rounded-lg"
              style={{ background: activeTab === t ? "rgba(249,115,22,0.12)" : "transparent", color: activeTab === t ? "#F97316" : "#6B7280", fontSize: 13, border: "none", cursor: "pointer" }}>
              {t}
            </button>
          ))}
        </div>
      </div>

      <div className="flex flex-1 overflow-hidden">
        {/* Alert list */}
        <div className="w-80 flex-shrink-0 overflow-y-auto" style={{ borderRight: "1px solid rgba(255,255,255,0.06)" }}>
          {alerts.length === 0 && (
            <div className="p-6 text-center" style={{ color: "#6B7280", fontSize: 13 }}>No alerts found.</div>
          )}
          {alerts.map(a => (
            <div key={a.id} onClick={() => setSelected(a)} className="p-4 cursor-pointer transition-all"
              style={{ borderBottom: "1px solid rgba(255,255,255,0.04)", background: activeAlert?.id === a.id ? "rgba(249,115,22,0.07)" : "transparent", borderLeft: activeAlert?.id === a.id ? "2px solid #F97316" : "2px solid transparent" }}
              onMouseEnter={e => { if (activeAlert?.id !== a.id) e.currentTarget.style.background = "rgba(255,255,255,0.02)"; }}
              onMouseLeave={e => { if (activeAlert?.id !== a.id) e.currentTarget.style.background = "transparent"; }}
            >
              <div className="flex items-center gap-2 mb-1.5">
                <span className="px-2 py-0.5 rounded" style={{ background: (RISK_COLORS[a.risk] ?? "#6B7280") + "20", color: RISK_COLORS[a.risk] ?? "#6B7280", fontSize: 11, fontWeight: 600 }}>{a.risk}</span>
                <span style={{ color: "var(--color-text-faint, #4B5563)", fontSize: 11 }}>{a.count}x</span>
              </div>
              <div style={{ color: "var(--color-text-primary, #E5E7EB)", fontSize: 13 }}>{a.name}</div>
              <div style={{ color: "var(--color-text-muted, #6B7280)", fontSize: 11, marginTop: 4, fontFamily: "monospace" }}>{a.url}</div>
            </div>
          ))}
        </div>

        {/* Detail */}
        {activeAlert && (
          <div className="flex-1 overflow-y-auto p-5">
            <div className="max-w-2xl">
              <div className="flex items-center gap-3 mb-5">
                <span className="px-3 py-1 rounded-lg" style={{ background: (RISK_COLORS[activeAlert.risk] ?? "#6B7280") + "20", color: RISK_COLORS[activeAlert.risk] ?? "#6B7280", fontSize: 13, fontWeight: 600 }}>{activeAlert.risk}</span>
                <h3 style={{ color: "var(--color-text-primary, #E5E7EB)", fontSize: 16, fontWeight: 600 }}>{activeAlert.name}</h3>
              </div>

              <div className="grid grid-cols-3 gap-3 mb-5">
                {[{ label: "Method", value: activeAlert.method }, { label: "Instances", value: activeAlert.count.toString() }, { label: "Confidence", value: activeAlert.confidence }].map(m => (
                  <div key={m.label} className="rounded-xl p-3" style={{ background: "var(--color-bg-card, #151B2F)", border: "1px solid rgba(255,255,255,0.07)" }}>
                    <div style={{ color: "var(--color-text-muted, #6B7280)", fontSize: 10 }}>{m.label}</div>
                    <div style={{ color: "var(--color-text-primary, #E5E7EB)", fontSize: 14, fontWeight: 600, marginTop: 2 }}>{m.value}</div>
                  </div>
                ))}
              </div>

              {activeAlert.url && (
                <div className="rounded-xl p-3 mb-4" style={{ background: "rgba(255,255,255,0.04)", border: "1px solid rgba(255,255,255,0.07)" }}>
                  <div style={{ color: "var(--color-text-muted, #6B7280)", fontSize: 10, marginBottom: 4 }}>URL</div>
                  <code style={{ color: "var(--color-primary, #4F8CFF)", fontSize: 12 }}>{activeAlert.url}</code>
                  {activeAlert.param && <span style={{ color: "var(--color-status-warning, #F59E0B)", fontSize: 12 }}> · param: <b>{activeAlert.param}</b></span>}
                </div>
              )}

              {activeAlert.evidence && (
                <div className="rounded-xl p-4 mb-4 font-mono" style={{ background: "#060D1A", border: "1px solid rgba(239,68,68,0.2)" }}>
                  <div style={{ color: "var(--color-status-error, #EF4444)", fontSize: 10, marginBottom: 6 }}>EVIDENCE</div>
                  <code style={{ color: "var(--color-severity-high, #F97316)", fontSize: 12 }}>{activeAlert.evidence}</code>
                </div>
              )}

              <div className="rounded-2xl p-5 mb-4" style={{ background: "var(--color-bg-card, #151B2F)", border: "1px solid rgba(255,255,255,0.07)" }}>
                <div style={{ color: "var(--color-text-secondary, #9CA3AF)", fontSize: 11, fontWeight: 600, marginBottom: 8 }}>DESCRIPTION</div>
                <p style={{ color: "var(--color-text-primary, #E5E7EB)", fontSize: 13, lineHeight: 1.7 }}>{activeAlert.desc}</p>
              </div>

              <div className="rounded-2xl p-5" style={{ background: "rgba(16,185,129,0.07)", border: "1px solid rgba(16,185,129,0.2)" }}>
                <div style={{ color: "var(--color-status-success, #10B981)", fontSize: 11, fontWeight: 600, marginBottom: 8 }}>SOLUTION</div>
                <p style={{ color: "#A7F3D0", fontSize: 13, lineHeight: 1.7 }}>{activeAlert.solution}</p>
              </div>
            </div>
          </div>
        )}

        {/* Risk Breakdown tab */}
        {activeTab === "Risk Breakdown" && (
          <div className="flex-1 overflow-y-auto p-5">
            <h3 style={{ color: "var(--color-text-primary, #E5E7EB)", fontSize: 14, fontWeight: 600, marginBottom: 16 }}>Risk Breakdown</h3>
            <div className="flex flex-col gap-3">
              {breakdown.map(([risk, count]) => (
                <div key={risk} className="flex items-center gap-3 rounded-xl px-4 py-3"
                  style={{ background: "var(--color-bg-card, #151B2F)", border: "1px solid rgba(255,255,255,0.07)" }}>
                  <span className="px-2 py-0.5 rounded" style={{ background: (RISK_COLORS[risk] ?? "#6B7280") + "20", color: RISK_COLORS[risk] ?? "#6B7280", fontSize: 12, fontWeight: 600, minWidth: 72 }}>{risk}</span>
                  <span style={{ color: "var(--color-text-primary, #E5E7EB)", fontSize: 20, fontWeight: 700 }}>{count}</span>
                  <span style={{ color: "var(--color-text-muted, #6B7280)", fontSize: 12 }}>instances</span>
                </div>
              ))}
            </div>
          </div>
        )}
      </div>
    </div>
  );
}
