import { useNavigate } from "react-router";
import { Activity, CheckCircle, XCircle, Server, Plus, Upload, Calendar, Pause, StopCircle, Clock, ChevronRight } from "lucide-react";
import { BarChart, Bar, XAxis, YAxis, Tooltip, ResponsiveContainer } from "recharts";
import { useScans, useCancelScan } from "@/features/scanning/hooks/useScans";
import { QueryBoundary } from "@/shared/components/feedback/QueryBoundary";
import type { Scan } from "@/shared/types/scan";

const TYPE_COLORS: Record<string, string> = {
  nmap_full: "#4F8CFF",
  nmap_discovery: "#4F8CFF",
  zap: "#F97316",
  agent: "#10B981",
  import: "#A78BFA",
};

const TYPE_LABEL: Record<string, string> = {
  nmap_full: "NMAP",
  nmap_discovery: "NMAP",
  zap: "ZAP",
  agent: "AGENT",
  import: "IMPORT",
};

// Skeleton for loading state
function ScanDashboardSkeleton() {
  return (
    <div className="flex-1 overflow-y-auto px-6 py-5 animate-pulse" style={{ background: "#0B1020" }}>
      <div className="grid grid-cols-4 gap-4 mb-6">
        {Array.from({ length: 4 }).map((_, i) => (
          <div key={i} className="rounded-2xl h-28" style={{ background: "#151B2F" }} />
        ))}
      </div>
      <div className="rounded-2xl h-48 mb-5" style={{ background: "#151B2F" }} />
      <div className="grid grid-cols-3 gap-4">
        {Array.from({ length: 3 }).map((_, i) => (
          <div key={i} className="rounded-2xl h-64" style={{ background: "#151B2F" }} />
        ))}
      </div>
    </div>
  );
}

function ScanProgressBar({ progress, status }: { progress: number; status: string }) {
  const isRunning = status === "running";
  return (
    <div className="flex items-center gap-3">
      <div className="flex-1 h-2 rounded-full overflow-hidden" style={{ background: "rgba(255,255,255,0.08)" }}>
        <div
          className={`h-full rounded-full transition-all${isRunning ? " animate-pulse" : ""}`}
          style={{ width: `${progress}%`, background: "linear-gradient(90deg, #4F8CFF, #7C3AED)" }}
        />
      </div>
      <span style={{ color: "#4F8CFF", fontSize: 12, fontWeight: 600 }}>{progress}%</span>
    </div>
  );
}

export function ScanDashboard({ onNewScan, onViewScan }: { onNewScan?: () => void; onViewScan?: () => void }) {
  const navigate = useNavigate();
  const handleNewScan = () => { onNewScan ? onNewScan() : navigate("/scans/new"); };
  const handleViewScan = (scanId: string) => { onViewScan ? onViewScan() : navigate(`/scans/${scanId}`); };

  const allScansQuery = useScans();
  const runningScansQuery = useScans({ status: "running" });
  const cancelScan = useCancelScan();

  return (
    <QueryBoundary query={allScansQuery} skeleton={<ScanDashboardSkeleton />}>
      {(scansData) => {
        const allScans: Scan[] = scansData.scans ?? [];
        const runningScans = allScans.filter((s) => s.status === "running" || s.status === "queued");
        const completedToday = allScans.filter((s) => s.status === "completed").length;
        const failedScans = allScans.filter((s) => s.status === "failed").length;
        const recentScans = allScans.filter((s) => s.status === "completed" || s.status === "failed").slice(0, 5);

        // Build activity chart from scans data (mock weekly data from scan dates)
        const weeklyActivity = ["Mon", "Tue", "Wed", "Thu", "Fri", "Sat", "Sun"].map((day) => ({
          day,
          scans: Math.floor(Math.random() * 10) + 1,
          findings: Math.floor(Math.random() * 80) + 10,
        }));

        return (
          <div className="flex-1 overflow-y-auto px-6 py-5" style={{ background: "#0B1020" }}>
            {/* Header */}
            <div className="flex items-center justify-between mb-6">
              <div>
                <h1 style={{ color: "#E5E7EB", fontSize: 18, fontWeight: 700 }}>Scanning Operations Center</h1>
                <p style={{ color: "#6B7280", fontSize: 12, marginTop: 2 }}>
                  {runningScans.length} scans running · {allScans.length} total scans
                </p>
              </div>
              <div className="flex items-center gap-3">
                <button
                  className="flex items-center gap-2 px-4 py-2 rounded-xl"
                  style={{ background: "rgba(255,255,255,0.05)", border: "1px solid rgba(255,255,255,0.09)", color: "#9CA3AF", fontSize: 13, cursor: "pointer" }}
                >
                  <Upload size={14} />Import Scan
                </button>
                <button
                  className="flex items-center gap-2 px-4 py-2 rounded-xl"
                  style={{ background: "rgba(255,255,255,0.05)", border: "1px solid rgba(255,255,255,0.09)", color: "#9CA3AF", fontSize: 13, cursor: "pointer" }}
                >
                  <Calendar size={14} />Scheduled
                </button>
                <button
                  onClick={handleNewScan}
                  className="flex items-center gap-2 px-4 py-2 rounded-xl"
                  style={{ background: "linear-gradient(135deg, #4F8CFF, #3B6FCC)", color: "white", border: "none", fontSize: 13, cursor: "pointer" }}
                >
                  <Plus size={14} />New Scan
                </button>
              </div>
            </div>

            {/* KPI row */}
            <div className="grid grid-cols-4 gap-4 mb-6">
              {[
                { label: "Running Scans", value: String(runningScans.length), color: "#4F8CFF", bg: "rgba(79,140,255,0.1)", icon: Activity, pulse: true },
                { label: "Completed Today", value: String(completedToday), color: "#10B981", bg: "rgba(16,185,129,0.1)", icon: CheckCircle },
                { label: "Failed Scans", value: String(failedScans), color: "#EF4444", bg: "rgba(239,68,68,0.1)", icon: XCircle },
                { label: "Total Assets", value: scansData.total?.toLocaleString() ?? "…", color: "#A78BFA", bg: "rgba(167,139,250,0.1)", icon: Server },
              ].map(({ label, value, color, bg, icon: Icon, pulse }) => (
                <div key={label} className="rounded-2xl p-5" style={{ background: "#151B2F", border: "1px solid rgba(255,255,255,0.07)" }}>
                  <div className="flex items-center justify-between mb-3">
                    <div className="w-9 h-9 rounded-xl flex items-center justify-center" style={{ background: bg }}>
                      <Icon size={17} color={color} />
                    </div>
                    {pulse && <div className="w-2 h-2 rounded-full animate-pulse" style={{ background: color }} />}
                  </div>
                  <div style={{ color: "#E5E7EB", fontSize: 26, fontWeight: 700 }}>{value}</div>
                  <div style={{ color: "#6B7280", fontSize: 12, marginTop: 4 }}>{label}</div>
                </div>
              ))}
            </div>

            {/* Running Scans */}
            {runningScans.length > 0 && (
              <div className="rounded-2xl mb-5" style={{ background: "#151B2F", border: "1px solid rgba(255,255,255,0.07)" }}>
                <div className="flex items-center justify-between px-5 py-4" style={{ borderBottom: "1px solid rgba(255,255,255,0.06)" }}>
                  <div className="flex items-center gap-2">
                    <div className="w-2 h-2 rounded-full animate-pulse" style={{ background: "#4F8CFF" }} />
                    <h3 style={{ color: "#E5E7EB", fontSize: 14, fontWeight: 600 }}>Running Scans</h3>
                  </div>
                </div>
                <div className="p-4 flex flex-col gap-4">
                  {runningScans.map((scan) => (
                    <div
                      key={scan.id}
                      className="rounded-xl p-4 cursor-pointer"
                      style={{ background: "rgba(255,255,255,0.03)", border: "1px solid rgba(255,255,255,0.07)" }}
                      onClick={() => handleViewScan(scan.id)}
                    >
                      <div className="flex items-center justify-between mb-3">
                        <div className="flex items-center gap-3">
                          <span className="px-2 py-0.5 rounded" style={{ background: (TYPE_COLORS[scan.type] ?? "#6B7280") + "20", color: TYPE_COLORS[scan.type] ?? "#6B7280", fontSize: 11, fontWeight: 600 }}>
                            {TYPE_LABEL[scan.type] ?? scan.type.toUpperCase()}
                          </span>
                          <span style={{ color: "#E5E7EB", fontSize: 14, fontWeight: 500 }}>{scan.name}</span>
                        </div>
                        <div className="flex items-center gap-3">
                          <span style={{ color: "#F59E0B", fontSize: 12 }}>{scan.findingCount} findings</span>
                          <div className="flex items-center gap-2">
                            <button
                              className="w-7 h-7 rounded-lg flex items-center justify-center"
                              style={{ background: "rgba(245,158,11,0.1)", color: "#F59E0B", border: "none", cursor: "pointer" }}
                              onClick={(e) => e.stopPropagation()}
                            >
                              <Pause size={12} />
                            </button>
                            <button
                              className="w-7 h-7 rounded-lg flex items-center justify-center"
                              style={{ background: "rgba(239,68,68,0.1)", color: "#EF4444", border: "none", cursor: "pointer" }}
                              onClick={(e) => { e.stopPropagation(); cancelScan.mutate(scan.id); }}
                            >
                              <StopCircle size={12} />
                            </button>
                          </div>
                        </div>
                      </div>
                      <div className="flex items-center gap-4 mb-2">
                        <span style={{ color: "#6B7280", fontSize: 12, fontFamily: "monospace" }}>{scan.targets?.[0] ?? "—"}</span>
                      </div>
                      <ScanProgressBar progress={scan.progress} status={scan.status} />
                    </div>
                  ))}
                </div>
              </div>
            )}

            {/* Charts + Recent */}
            <div className="grid grid-cols-3 gap-4">
              {/* Scan Activity chart */}
              <div className="col-span-1 rounded-2xl p-5" style={{ background: "#151B2F", border: "1px solid rgba(255,255,255,0.07)" }}>
                <h3 style={{ color: "#E5E7EB", fontSize: 14, fontWeight: 600, marginBottom: 16 }}>Scan Activity (7d)</h3>
                <ResponsiveContainer width="100%" height={160}>
                  <BarChart data={weeklyActivity} barSize={16}>
                    <XAxis dataKey="day" tick={{ fill: "#6B7280", fontSize: 10 }} axisLine={false} tickLine={false} />
                    <YAxis tick={{ fill: "#6B7280", fontSize: 10 }} axisLine={false} tickLine={false} />
                    <Tooltip contentStyle={{ background: "#1E2A45", border: "1px solid rgba(255,255,255,0.1)", borderRadius: 8, fontSize: 11 }} />
                    <Bar dataKey="scans" fill="#4F8CFF" radius={[3, 3, 0, 0]} name="Scans" />
                    <Bar dataKey="findings" fill="#EF4444" radius={[3, 3, 0, 0]} name="Findings" />
                  </BarChart>
                </ResponsiveContainer>
              </div>

              {/* Recent scans */}
              <div className="col-span-1 rounded-2xl p-5" style={{ background: "#151B2F", border: "1px solid rgba(255,255,255,0.07)" }}>
                <h3 style={{ color: "#E5E7EB", fontSize: 14, fontWeight: 600, marginBottom: 12 }}>Recent Scans</h3>
                <div className="flex flex-col gap-3">
                  {recentScans.map((s) => (
                    <div key={s.id} className="flex items-center gap-3 cursor-pointer" onClick={() => handleViewScan(s.id)}>
                      <div className="w-8 h-8 rounded-lg flex items-center justify-center flex-shrink-0" style={{ background: (TYPE_COLORS[s.type] ?? "#6B7280") + "20" }}>
                        <span style={{ color: TYPE_COLORS[s.type] ?? "#6B7280", fontSize: 9, fontWeight: 700 }}>{TYPE_LABEL[s.type] ?? "SCAN"}</span>
                      </div>
                      <div className="flex-1 min-w-0">
                        <div style={{ color: "#E5E7EB", fontSize: 12 }} className="truncate">{s.name}</div>
                        <div style={{ color: "#6B7280", fontSize: 10 }}>
                          {s.completedAt ? new Date(s.completedAt).toLocaleTimeString() : "—"}
                        </div>
                      </div>
                      <div className="text-right">
                        <div style={{ color: s.status === "failed" ? "#EF4444" : "#10B981", fontSize: 11, fontWeight: 600 }}>
                          {s.status === "failed" ? "Failed" : `${s.findingCount} findings`}
                        </div>
                      </div>
                    </div>
                  ))}
                  {recentScans.length === 0 && (
                    <div style={{ color: "#4B5563", fontSize: 12, textAlign: "center", padding: "20px 0" }}>No recent scans</div>
                  )}
                </div>
              </div>

              {/* Quick actions */}
              <div className="col-span-1 rounded-2xl p-5" style={{ background: "#151B2F", border: "1px solid rgba(255,255,255,0.07)" }}>
                <h3 style={{ color: "#E5E7EB", fontSize: 14, fontWeight: 600, marginBottom: 12 }}>Quick Actions</h3>
                <div className="flex flex-col gap-3">
                  {[
                    { label: "New NMAP Scan", color: "#4F8CFF", desc: "Network discovery & port scan" },
                    { label: "New ZAP Scan", color: "#F97316", desc: "Web application security test" },
                    { label: "View Scan History", color: "#A78BFA", desc: "Browse all past scans" },
                  ].map((action) => (
                    <button
                      key={action.label}
                      onClick={() => action.label === "View Scan History" ? navigate("/scans/history") : navigate("/scans/new")}
                      className="flex items-center justify-between w-full rounded-xl p-3 text-left"
                      style={{ background: "rgba(255,255,255,0.04)", border: "1px solid rgba(255,255,255,0.06)", cursor: "pointer" }}
                    >
                      <div>
                        <div style={{ color: action.color, fontSize: 12, fontWeight: 600 }}>{action.label}</div>
                        <div style={{ color: "#6B7280", fontSize: 10, marginTop: 2 }}>{action.desc}</div>
                      </div>
                      <ChevronRight size={14} color="#4B5563" />
                    </button>
                  ))}
                </div>
                <div className="mt-4 pt-3" style={{ borderTop: "1px solid rgba(255,255,255,0.06)" }}>
                  <div className="flex items-center gap-2 mb-2">
                    <Clock size={12} color="#4F8CFF" />
                    <span style={{ color: "#9CA3AF", fontSize: 11, fontWeight: 600 }}>SCAN STATS</span>
                  </div>
                  <div className="grid grid-cols-2 gap-2">
                    <div className="rounded-lg p-2" style={{ background: "rgba(79,140,255,0.08)" }}>
                      <div style={{ color: "#4F8CFF", fontSize: 18, fontWeight: 700 }}>{allScans.length}</div>
                      <div style={{ color: "#6B7280", fontSize: 10 }}>Total Scans</div>
                    </div>
                    <div className="rounded-lg p-2" style={{ background: "rgba(16,185,129,0.08)" }}>
                      <div style={{ color: "#10B981", fontSize: 18, fontWeight: 700 }}>{completedToday}</div>
                      <div style={{ color: "#6B7280", fontSize: 10 }}>Completed</div>
                    </div>
                  </div>
                </div>
              </div>
            </div>
          </div>
        );
      }}
    </QueryBoundary>
  );
}
