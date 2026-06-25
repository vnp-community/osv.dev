import { useNavigate } from "react-router";
import { Activity, Clock, Target, Cpu, ArrowRight, Zap, AlertCircle, RefreshCw } from "lucide-react";
import { useScans } from "@/features/scanning/hooks/useScans";
import { QueryBoundary } from "@/shared/components/feedback/QueryBoundary";
import type { Scan } from "@/features/scanning/types";

// ─── Skeleton ────────────────────────────────────────────────────────────────

function Skeleton() {
  return (
    <div className="flex-1 overflow-y-auto animate-pulse" style={{ background: "var(--color-bg-page, #0B1020)", padding: "24px" }}>
      {[...Array(4)].map((_, i) => (
        <div key={i} className="h-28 rounded-2xl mb-4" style={{ background: "var(--color-bg-card, #151B2F)" }} />
      ))}
    </div>
  );
}

// ─── Scan Card ───────────────────────────────────────────────────────────────

function ScanCard({ scan, onClick }: { scan: Scan; onClick: () => void }) {
  const elapsedSec = scan.started_at
    ? Math.round((Date.now() - new Date(scan.started_at).getTime()) / 1000)
    : null;

  const formatElapsed = (sec: number) => {
    if (sec < 60) return `${sec}s`;
    if (sec < 3600) return `${Math.floor(sec / 60)}m ${sec % 60}s`;
    return `${Math.floor(sec / 3600)}h ${Math.floor((sec % 3600) / 60)}m`;
  };

  const typeColor: Record<string, string> = {
    nmap: "#4F8CFF",
    zap:  "#A78BFA",
    full: "#F97316",
  };
  const color = typeColor[scan.type.toLowerCase()] ?? "#4F8CFF";

  return (
    <div
      onClick={onClick}
      className="rounded-2xl p-5 cursor-pointer transition-all"
      style={{
        background: "var(--color-bg-card, #151B2F)",
        border: "1px solid rgba(79,140,255,0.2)",
        boxShadow: "0 0 0 1px rgba(79,140,255,0.05)",
      }}
      onMouseEnter={(e) => { e.currentTarget.style.borderColor = "rgba(79,140,255,0.5)"; }}
      onMouseLeave={(e) => { e.currentTarget.style.borderColor = "rgba(79,140,255,0.2)"; }}
    >
      <div className="flex items-start justify-between mb-4">
        <div className="flex items-center gap-3">
          {/* Pulsing indicator */}
          <div className="relative w-10 h-10 rounded-xl flex items-center justify-center flex-shrink-0"
            style={{ background: `${color}20` }}>
            <Activity size={18} color={color} />
            <span className="absolute top-0 right-0 w-2.5 h-2.5 rounded-full animate-pulse"
              style={{ background: "#10B981", border: "2px solid #0B1020" }} />
          </div>
          <div>
            <div style={{ color: "var(--color-text-primary, #E5E7EB)", fontSize: 14, fontWeight: 600 }}>{scan.name}</div>
            <div className="flex items-center gap-2 mt-0.5">
              <span className="px-1.5 py-0.5 rounded text-xs font-mono font-bold uppercase"
                style={{ background: `${color}20`, color }}>{scan.type}</span>
              <span style={{ color: "var(--color-text-muted, #6B7280)", fontSize: 11 }}>
                {scan.targets.length} target{scan.targets.length !== 1 ? "s" : ""}
              </span>
            </div>
          </div>
        </div>
        <div className="flex items-center gap-2">
          <span className="px-2 py-1 rounded-lg text-xs font-medium"
            style={{ background: "rgba(16,185,129,0.1)", color: "var(--color-status-success, #10B981)" }}>
            Running
          </span>
          <ArrowRight size={14} color="#4B5563" />
        </div>
      </div>

      {/* Targets */}
      {scan.targets.length > 0 && (
        <div className="flex flex-wrap gap-1.5 mb-4">
          {scan.targets.slice(0, 4).map((t, i) => (
            <span key={i} className="px-2 py-0.5 rounded-lg font-mono"
              style={{ background: "rgba(255,255,255,0.04)", color: "var(--color-text-secondary, #9CA3AF)", fontSize: 11 }}>
              {t}
            </span>
          ))}
          {scan.targets.length > 4 && (
            <span style={{ color: "var(--color-text-faint, #4B5563)", fontSize: 11 }}>+{scan.targets.length - 4} more</span>
          )}
        </div>
      )}

      {/* Stats row */}
      <div className="grid grid-cols-3 gap-3">
        <div className="rounded-xl p-3" style={{ background: "rgba(255,255,255,0.03)" }}>
          <div className="flex items-center gap-1.5 mb-1">
            <Clock size={11} color="#6B7280" />
            <span style={{ color: "var(--color-text-muted, #6B7280)", fontSize: 10 }}>Elapsed</span>
          </div>
          <div style={{ color: "var(--color-text-primary, #E5E7EB)", fontSize: 14, fontWeight: 700, fontFamily: "monospace" }}>
            {elapsedSec != null ? formatElapsed(elapsedSec) : "—"}
          </div>
        </div>
        <div className="rounded-xl p-3" style={{ background: "rgba(255,255,255,0.03)" }}>
          <div className="flex items-center gap-1.5 mb-1">
            <AlertCircle size={11} color="#6B7280" />
            <span style={{ color: "var(--color-text-muted, #6B7280)", fontSize: 10 }}>Findings</span>
          </div>
          <div style={{ color: scan.finding_count > 20 ? "#EF4444" : "#E5E7EB", fontSize: 14, fontWeight: 700 }}>
            {scan.finding_count}
          </div>
        </div>
        <div className="rounded-xl p-3" style={{ background: "rgba(255,255,255,0.03)" }}>
          <div className="flex items-center gap-1.5 mb-1">
            <Target size={11} color="#6B7280" />
            <span style={{ color: "var(--color-text-muted, #6B7280)", fontSize: 10 }}>Started</span>
          </div>
          <div style={{ color: "var(--color-text-secondary, #9CA3AF)", fontSize: 11 }}>
            {scan.started_at ? new Date(scan.started_at).toLocaleTimeString() : "—"}
          </div>
        </div>
      </div>
    </div>
  );
}

// ─── Empty state ─────────────────────────────────────────────────────────────

function EmptyState({ onNewScan }: { onNewScan: () => void }) {
  return (
    <div className="flex flex-col items-center justify-center h-full gap-4">
      <div className="w-16 h-16 rounded-2xl flex items-center justify-center"
        style={{ background: "rgba(79,140,255,0.1)", border: "1px solid rgba(79,140,255,0.2)" }}>
        <Cpu size={28} color="#4F8CFF" />
      </div>
      <div className="text-center">
        <div style={{ color: "var(--color-text-primary, #E5E7EB)", fontSize: 16, fontWeight: 600 }}>No Active Scans</div>
        <div style={{ color: "var(--color-text-muted, #6B7280)", fontSize: 13, marginTop: 4 }}>
          All scans have completed. Start a new scan to monitor progress here.
        </div>
      </div>
      <button onClick={onNewScan}
        className="flex items-center gap-2 px-5 py-2.5 rounded-xl"
        style={{ background: "linear-gradient(135deg,#4F8CFF,#3B6FCC)", color: "white", fontSize: 13, border: "none", cursor: "pointer" }}>
        <Zap size={14} />
        Start New Scan
      </button>
    </div>
  );
}

// ─── Main page ───────────────────────────────────────────────────────────────

function RunningScansContent({ scans }: { scans: Scan[] }) {
  const navigate = useNavigate();
  const running = scans.filter((s) => s.status === "running");

  return (
    <div className="flex-1 overflow-y-auto" style={{ background: "var(--color-bg-page, #0B1020)", padding: "24px" }}>
      {/* Header */}
      <div className="flex items-center justify-between mb-6">
        <div>
          <h1 style={{ color: "var(--color-text-primary, #E5E7EB)", fontWeight: 700, fontSize: 20 }}>
            Running Scans
            {running.length > 0 && (
              <span className="ml-3 px-2.5 py-1 rounded-full text-sm font-semibold align-middle"
                style={{ background: "rgba(79,140,255,0.15)", color: "var(--color-primary, #4F8CFF)" }}>
                {running.length} active
              </span>
            )}
          </h1>
          <p style={{ color: "var(--color-text-muted, #6B7280)", fontSize: 13, marginTop: 2 }}>
            Live scan activity · Auto-refreshes every 15s
          </p>
        </div>
        <div className="flex items-center gap-3">
          <div className="flex items-center gap-2 px-3 py-1.5 rounded-xl"
            style={{ background: "rgba(16,185,129,0.1)", border: "1px solid rgba(16,185,129,0.2)" }}>
            <RefreshCw size={12} color="#10B981" className="animate-spin" style={{ animationDuration: "3s" }} />
            <span style={{ color: "var(--color-status-success, #10B981)", fontSize: 12 }}>Live</span>
          </div>
          <button onClick={() => navigate("/scans/new")}
            className="flex items-center gap-2 px-4 py-2 rounded-xl"
            style={{ background: "linear-gradient(135deg,#4F8CFF,#3B6FCC)", color: "white", fontSize: 13, border: "none", cursor: "pointer" }}>
            <Zap size={14} />
            New Scan
          </button>
        </div>
      </div>

      {/* Summary bar */}
      {running.length > 0 && (
        <div className="grid grid-cols-3 gap-4 mb-6">
          {[
            { label: "Active Scans",    value: running.length,                                    color: "var(--color-primary, #4F8CFF)",  bg: "rgba(79,140,255,0.1)"  },
            { label: "Total Findings",  value: running.reduce((s, sc) => s + sc.finding_count, 0), color: "var(--color-severity-high, #F97316)", bg: "rgba(249,115,22,0.1)"  },
            { label: "Targets Scanned", value: running.reduce((s, sc) => s + sc.targets.length, 0), color: "var(--color-status-success, #10B981)", bg: "rgba(16,185,129,0.1)" },
          ].map((stat) => (
            <div key={stat.label} className="rounded-2xl p-4"
              style={{ background: "var(--color-bg-card, #151B2F)", border: "1px solid rgba(255,255,255,0.07)" }}>
              <div style={{ color: stat.color, fontSize: 28, fontWeight: 700 }}>{stat.value}</div>
              <div style={{ color: "var(--color-text-muted, #6B7280)", fontSize: 12, marginTop: 4 }}>{stat.label}</div>
            </div>
          ))}
        </div>
      )}

      {/* Cards */}
      {running.length === 0 ? (
        <div style={{ height: "calc(100vh - 260px)" }}>
          <EmptyState onNewScan={() => navigate("/scans/new")} />
        </div>
      ) : (
        <div className="flex flex-col gap-4">
          {running.map((scan) => (
            <ScanCard key={scan.id} scan={scan} onClick={() => navigate(`/scans/${scan.id}`)} />
          ))}
        </div>
      )}
    </div>
  );
}

export function RunningScans() {
  const query = useScans({ status: "running" });
  return (
    <QueryBoundary query={query} skeleton={<Skeleton />}>
      {(data) => <RunningScansContent scans={data.scans ?? []} />}
    </QueryBoundary>
  );
}
