import { useState, useEffect } from "react";
import { useParams, useNavigate } from "react-router";
import { Pause, StopCircle, Download, ArrowLeft, Activity, Server, Shield, Clock, Terminal } from "lucide-react";
import { useScanDetail, useCancelScan } from "@/features/scanning/hooks/useScans";
import { useScanSSE } from "@/features/scanning/hooks/useScanSSE";

const LOG_COLORS: Record<string, string> = {
  info: "#6B7280", success: "#10B981", warn: "#F59E0B", error: "#EF4444",
};

const SEVERITY_COLORS: Record<string, string> = {
  Critical: "#EF4444", High: "#F97316", Medium: "#EAB308", Low: "#3B82F6",
};

interface LogEntry {
  time: string;
  level: "info" | "success" | "warn" | "error";
  msg: string;
}

export function RunningScan({ onBack }: { onBack?: () => void }) {
  const { id } = useParams<{ id: string }>();
  const navigate = useNavigate();
  const handleBack = () => (onBack ? onBack() : navigate("/scans"));

  const scanQuery = useScanDetail(id ?? null);
  const cancelScan = useCancelScan();

  const isActive =
    scanQuery.data?.status === "running" || scanQuery.data?.status === "queued";

  const { progress, sseStatus } = useScanSSE(id ?? "", isActive);

  const [elapsed, setElapsed] = useState(0);
  const [isPaused, setIsPaused] = useState(false);
  const [logs, setLogs] = useState<LogEntry[]>([
    { time: new Date().toLocaleTimeString(), level: "info", msg: `Scan ${id ?? "—"} initializing...` },
  ]);

  // Append SSE progress messages to logs
  useEffect(() => {
    if (progress?.message) {
      setLogs((prev) => [
        ...prev,
        {
          time: new Date().toLocaleTimeString(),
          level: "info",
          msg: progress.message!,
        },
      ]);
    }
  }, [progress]);

  // Elapsed timer
  useEffect(() => {
    if (isPaused || !isActive) return;
    const interval = setInterval(() => setElapsed((e) => e + 1), 1000);
    return () => clearInterval(interval);
  }, [isPaused, isActive]);

  const formatTime = (s: number) => {
    const h = Math.floor(s / 3600);
    const m = Math.floor((s % 3600) / 60);
    const sec = s % 60;
    return [h, m, sec].map((v) => String(v).padStart(2, "0")).join(":");
  };

  const scan = scanQuery.data;
  const overallProgress = progress?.progress ?? scan?.progress ?? 0;
  const findingsFound = progress?.findingsFound ?? scan?.findingCount ?? 0;

  if (scanQuery.isLoading) {
    return (
      <div className="flex-1 flex items-center justify-center" style={{ background: "var(--color-bg-page, #0B1020)" }}>
        <div className="w-10 h-10 rounded-full border-2 border-t-transparent animate-spin" style={{ borderColor: "#4F8CFF", borderTopColor: "transparent" }} />
      </div>
    );
  }

  if (!scan) {
    return (
      <div className="flex-1 flex items-center justify-center" style={{ background: "var(--color-bg-page, #0B1020)" }}>
        <div className="text-center">
          <p style={{ color: "var(--color-status-error, #EF4444)", fontSize: 14 }}>Scan not found</p>
          <button onClick={handleBack} style={{ color: "var(--color-primary, #4F8CFF)", background: "none", border: "none", cursor: "pointer", marginTop: 8, fontSize: 13 }}>← Back to Scans</button>
        </div>
      </div>
    );
  }

  return (
    <div className="flex flex-col flex-1 overflow-hidden" style={{ background: "var(--color-bg-page, #0B1020)" }}>
      {/* Header */}
      <div
        className="flex items-center justify-between px-6 py-4"
        style={{ background: "var(--color-bg-sidebar, #0F1629)", borderBottom: "1px solid rgba(255,255,255,0.06)" }}
      >
        <div className="flex items-center gap-4">
          <button onClick={handleBack} className="w-8 h-8 rounded-lg flex items-center justify-center" style={{ background: "rgba(255,255,255,0.05)", color: "var(--color-text-secondary, #9CA3AF)", border: "none", cursor: "pointer" }}>
            <ArrowLeft size={15} />
          </button>
          <div>
            <div className="flex items-center gap-3">
              <h2 style={{ color: "var(--color-text-primary, #E5E7EB)", fontSize: 16, fontWeight: 700 }}>{scan.name}</h2>
              <span
                className="flex items-center gap-1.5 px-2.5 py-1 rounded-lg"
                style={{
                  background: isActive ? "rgba(79,140,255,0.12)" : "rgba(16,185,129,0.12)",
                  color: isActive ? "#4F8CFF" : "#10B981",
                  fontSize: 12,
                }}
              >
                {isActive && <div className="w-1.5 h-1.5 rounded-full animate-pulse" style={{ background: "#4F8CFF" }} />}
                {scan.status.charAt(0).toUpperCase() + scan.status.slice(1)}
              </span>
            </div>
            <div className="flex items-center gap-4 mt-1">
              <span style={{ color: "var(--color-text-muted, #6B7280)", fontSize: 12 }}>{scan.type.toUpperCase()} · {scan.targets?.[0] ?? "—"}</span>
              <span style={{ color: "var(--color-text-muted, #6B7280)", fontSize: 12 }}>{scan.id}</span>
              <span style={{ color: "var(--color-primary, #4F8CFF)", fontSize: 12, fontFamily: "monospace" }}>
                <Clock size={11} style={{ display: "inline", marginRight: 4 }} />
                {formatTime(elapsed)}
              </span>
            </div>
          </div>
        </div>
        <div className="flex items-center gap-3">
          {isActive && (
            <>
              <button
                onClick={() => setIsPaused(!isPaused)}
                className="flex items-center gap-2 px-4 py-2 rounded-xl"
                style={{ background: "rgba(245,158,11,0.1)", border: "1px solid rgba(245,158,11,0.2)", color: "var(--color-status-warning, #F59E0B)", fontSize: 13, cursor: "pointer" }}
              >
                <Pause size={13} />{isPaused ? "Resume" : "Pause"}
              </button>
              <button
                onClick={() => id && cancelScan.mutate(id)}
                disabled={cancelScan.isPending}
                className="flex items-center gap-2 px-4 py-2 rounded-xl"
                style={{ background: "rgba(239,68,68,0.1)", border: "1px solid rgba(239,68,68,0.2)", color: "var(--color-status-error, #EF4444)", fontSize: 13, cursor: "pointer", opacity: cancelScan.isPending ? 0.6 : 1 }}
              >
                <StopCircle size={13} />Cancel
              </button>
            </>
          )}
          <button
            className="flex items-center gap-2 px-4 py-2 rounded-xl"
            style={{ background: "rgba(255,255,255,0.05)", border: "1px solid rgba(255,255,255,0.09)", color: "var(--color-text-secondary, #9CA3AF)", fontSize: 13, cursor: "pointer" }}
          >
            <Download size={13} />Export
          </button>
        </div>
      </div>

      <div className="flex flex-1 overflow-hidden">
        {/* Main content */}
        <div className="flex flex-col flex-1 overflow-hidden p-5 gap-4">
          {/* Overall progress */}
          <div className="rounded-2xl p-5" style={{ background: "var(--color-bg-card, #151B2F)", border: "1px solid rgba(255,255,255,0.07)" }}>
            <div className="flex items-center justify-between mb-3">
              <span style={{ color: "var(--color-text-primary, #E5E7EB)", fontSize: 14, fontWeight: 600 }}>Overall Progress</span>
              <div className="flex items-center gap-4">
                <span style={{ color: "var(--color-status-warning, #F59E0B)", fontSize: 13 }}>{findingsFound} findings</span>
                <span style={{ color: "var(--color-primary, #4F8CFF)", fontSize: 18, fontWeight: 700 }}>{overallProgress}%</span>
              </div>
            </div>
            <div className="h-3 rounded-full overflow-hidden mb-3" style={{ background: "rgba(255,255,255,0.07)" }}>
              <div
                className="h-full rounded-full transition-all"
                style={{
                  width: `${overallProgress}%`,
                  background: "linear-gradient(90deg, #4F8CFF, #7C3AED)",
                  boxShadow: "0 0 12px rgba(79,140,255,0.4)",
                }}
              />
            </div>
            {progress?.currentTarget && (
              <div style={{ color: "var(--color-text-muted, #6B7280)", fontSize: 12 }}>
                Current target: <span style={{ color: "var(--color-primary, #4F8CFF)", fontFamily: "monospace" }}>{progress.currentTarget}</span>
              </div>
            )}
            {sseStatus === "connecting" && (
              <div style={{ color: "var(--color-status-warning, #F59E0B)", fontSize: 11, marginTop: 4 }}>Connecting to live stream...</div>
            )}
          </div>

          {/* Stats */}
          <div className="grid grid-cols-3 gap-4 flex-1 overflow-hidden">
            {/* Scan metadata */}
            <div className="rounded-2xl flex flex-col overflow-hidden" style={{ background: "var(--color-bg-card, #151B2F)", border: "1px solid rgba(255,255,255,0.07)" }}>
              <div className="flex items-center gap-2 px-4 py-3" style={{ borderBottom: "1px solid rgba(255,255,255,0.06)" }}>
                <Activity size={14} color="#4F8CFF" />
                <span style={{ color: "var(--color-text-primary, #E5E7EB)", fontSize: 13, fontWeight: 600 }}>Scan Info</span>
              </div>
              <div className="flex-1 overflow-y-auto p-4">
                {[
                  { label: "Scan ID", value: scan.id },
                  { label: "Type", value: scan.type.toUpperCase() },
                  { label: "Status", value: scan.status },
                  { label: "Targets", value: scan.targets?.join(", ") ?? "—" },
                  { label: "Started", value: scan.startedAt ? new Date(scan.startedAt).toLocaleString() : "—" },
                  { label: "Findings", value: String(findingsFound) },
                ].map(({ label, value }) => (
                  <div key={label} className="flex justify-between py-2" style={{ borderBottom: "1px solid rgba(255,255,255,0.04)" }}>
                    <span style={{ color: "var(--color-text-muted, #6B7280)", fontSize: 12 }}>{label}</span>
                    <span style={{ color: "var(--color-text-primary, #E5E7EB)", fontSize: 12, fontFamily: label === "Scan ID" || label === "Targets" ? "monospace" : undefined }}>
                      {value}
                    </span>
                  </div>
                ))}
              </div>
            </div>

            {/* SSE Status / Live Findings */}
            <div className="rounded-2xl flex flex-col overflow-hidden" style={{ background: "var(--color-bg-card, #151B2F)", border: "1px solid rgba(255,255,255,0.07)" }}>
              <div className="flex items-center gap-2 px-4 py-3" style={{ borderBottom: "1px solid rgba(255,255,255,0.06)" }}>
                <Shield size={14} color="#EF4444" />
                <span style={{ color: "var(--color-text-primary, #E5E7EB)", fontSize: 13, fontWeight: 600 }}>Live Stats</span>
                {isActive && <div className="w-1.5 h-1.5 rounded-full animate-pulse ml-auto" style={{ background: "#10B981" }} />}
              </div>
              <div className="flex-1 overflow-y-auto p-4 flex flex-col gap-3">
                {[
                  { label: "Progress", value: `${overallProgress}%`, color: "var(--color-primary, #4F8CFF)" },
                  { label: "Findings Found", value: String(findingsFound), color: "var(--color-severity-high, #F97316)" },
                  { label: "Stream Status", value: sseStatus, color: sseStatus === "streaming" ? "#10B981" : "#F59E0B" },
                ].map(({ label, value, color }) => (
                  <div key={label} className="rounded-xl p-3" style={{ background: "rgba(255,255,255,0.04)" }}>
                    <div style={{ color: "var(--color-text-muted, #6B7280)", fontSize: 10, marginBottom: 2 }}>{label}</div>
                    <div style={{ color, fontSize: 18, fontWeight: 700 }}>{value}</div>
                  </div>
                ))}
              </div>
            </div>

            {/* Targets */}
            <div className="rounded-2xl flex flex-col overflow-hidden" style={{ background: "var(--color-bg-card, #151B2F)", border: "1px solid rgba(255,255,255,0.07)" }}>
              <div className="flex items-center gap-2 px-4 py-3" style={{ borderBottom: "1px solid rgba(255,255,255,0.06)" }}>
                <Server size={14} color="#4F8CFF" />
                <span style={{ color: "var(--color-text-primary, #E5E7EB)", fontSize: 13, fontWeight: 600 }}>Targets</span>
                <span className="ml-auto px-2 py-0.5 rounded-full" style={{ background: "rgba(79,140,255,0.1)", color: "var(--color-primary, #4F8CFF)", fontSize: 11 }}>
                  {scan.targets?.length ?? 0}
                </span>
              </div>
              <div className="flex-1 overflow-y-auto p-3 flex flex-col gap-2">
                {(scan.targets ?? []).map((t) => (
                  <div key={t} className="flex items-center gap-3 rounded-xl p-3" style={{ background: "rgba(255,255,255,0.03)" }}>
                    <div className="w-2 h-2 rounded-full flex-shrink-0" style={{ background: "#4F8CFF" }} />
                    <span style={{ color: "var(--color-text-primary, #E5E7EB)", fontSize: 12, fontFamily: "monospace" }}>{t}</span>
                  </div>
                ))}
                {(!scan.targets || scan.targets.length === 0) && (
                  <div style={{ color: "var(--color-text-faint, #4B5563)", fontSize: 12, textAlign: "center", padding: "20px 0" }}>No targets defined</div>
                )}
              </div>
            </div>
          </div>
        </div>

        {/* Live logs panel */}
        <div
          className="w-80 flex-shrink-0 flex flex-col"
          style={{ background: "#060D1A", borderLeft: "1px solid rgba(255,255,255,0.06)" }}
        >
          <div className="flex items-center gap-2 px-4 py-3" style={{ borderBottom: "1px solid rgba(255,255,255,0.06)" }}>
            <Terminal size={13} color="#4F8CFF" />
            <span style={{ color: "var(--color-text-primary, #E5E7EB)", fontSize: 13, fontWeight: 600 }}>Live Logs</span>
            {isActive && <div className="w-1.5 h-1.5 rounded-full animate-pulse ml-auto" style={{ background: "#10B981" }} />}
          </div>
          <div className="flex-1 overflow-y-auto p-3 font-mono" style={{ fontSize: 11 }}>
            {logs.map((log, i) => (
              <div key={i} className="mb-1.5">
                <span style={{ color: "var(--color-text-faint, #4B5563)" }}>[{log.time}]</span>{" "}
                <span style={{ color: LOG_COLORS[log.level] }}>{log.msg}</span>
              </div>
            ))}
            {isActive && (
              <div className="flex items-center gap-1 mt-2">
                <div className="w-1.5 h-3 animate-pulse" style={{ background: "#4F8CFF" }} />
              </div>
            )}
          </div>
        </div>
      </div>
    </div>
  );
}
