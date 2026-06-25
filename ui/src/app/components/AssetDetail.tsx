import { useParams, useNavigate } from "react-router";
import { ArrowLeft } from "lucide-react";
import { LineChart, Line, XAxis, YAxis, Tooltip, ResponsiveContainer } from "recharts";
import { useAssetDetail } from "@/features/assets/hooks/useAssets";
import { QueryBoundary } from "@/shared/components/feedback/QueryBoundary";

// ─── Skeleton ─────────────────────────────────────────────────────────────────
function AssetDetailSkeleton() {
  return (
    <div className="flex flex-col flex-1 overflow-hidden" style={{ background: "#0B1020" }}>
      <div className="px-6 py-4 animate-pulse" style={{ background: "#0F1629", borderBottom: "1px solid rgba(255,255,255,0.06)" }}>
        <div className="h-6 w-64 rounded-lg" style={{ background: "rgba(255,255,255,0.06)" }} />
      </div>
      <div className="flex-1 p-6 grid grid-cols-3 gap-5 animate-pulse">
        <div className="col-span-2 flex flex-col gap-5">
          <div className="grid grid-cols-4 gap-4">
            {Array.from({ length: 4 }).map((_, i) => (
              <div key={i} className="rounded-2xl p-4 h-24" style={{ background: "#151B2F" }} />
            ))}
          </div>
          <div className="rounded-2xl h-48" style={{ background: "#151B2F" }} />
          <div className="rounded-2xl h-40" style={{ background: "#151B2F" }} />
        </div>
        <div className="flex flex-col gap-4">
          <div className="rounded-2xl h-64" style={{ background: "#151B2F" }} />
          <div className="rounded-2xl h-40" style={{ background: "#151B2F" }} />
        </div>
      </div>
    </div>
  );
}

// ─── Content ──────────────────────────────────────────────────────────────────
function AssetDetailContent({ asset, onBack }: { asset: any; onBack: () => void }) {
  const riskHistory: Array<{ date: string; score: number }> = asset.riskHistory ?? [
    { date: "Jan", score: 4.2 }, { date: "Feb", score: 5.8 }, { date: "Mar", score: 7.1 },
    { date: "Apr", score: 8.4 }, { date: "May", score: 9.2 }, { date: "Jun", score: 9.8 },
  ];

  const services: Array<{ name: string; version: string; port: number | null; status: string; cves: number }> =
    asset.services ?? [];

  const scanHistory: Array<{ scan: string; date: string; type: string; findings: number }> =
    asset.scanHistory ?? [];

  const kpis = [
    { label: "Risk Score", value: String(asset.riskScore ?? "—"), color: "#EF4444" },
    { label: "Active Findings", value: String(asset.findingCount ?? 0), color: "#F97316" },
    { label: "Open Ports", value: String(asset.openPorts ?? 0), color: "#4F8CFF" },
    { label: "Last Scan", value: asset.lastScanAt ?? "—", color: "#10B981" },
  ];

  const systemInfo = [
    { label: "OS",           value: asset.os ?? "—" },
    { label: "Kernel",       value: asset.kernel ?? "—" },
    { label: "Architecture", value: asset.arch ?? "—" },
    { label: "CPU",          value: asset.cpu ?? "—" },
    { label: "RAM",          value: asset.ram ?? "—" },
    { label: "Environment",  value: asset.environment ?? "—" },
    { label: "Tags",         value: (asset.tags ?? []).join(", ") || "—" },
  ];

  return (
    <div className="flex flex-col flex-1 overflow-hidden" style={{ background: "#0B1020" }}>
      {/* Header */}
      <div className="px-6 py-4" style={{ background: "#0F1629", borderBottom: "1px solid rgba(255,255,255,0.06)" }}>
        <div className="flex items-center gap-3 mb-2">
          <button
            onClick={onBack}
            className="w-8 h-8 rounded-lg flex items-center justify-center"
            style={{ background: "rgba(255,255,255,0.05)", color: "#9CA3AF", border: "none", cursor: "pointer" }}
          >
            <ArrowLeft size={15} />
          </button>
          <div>
            <div className="flex items-center gap-3">
              <h2 style={{ color: "#E5E7EB", fontSize: 16, fontWeight: 700, fontFamily: "monospace" }}>{asset.ip ?? asset.hostname}</h2>
              <span style={{ color: "#6B7280", fontSize: 14 }}>{asset.hostname}</span>
              <span className="px-2 py-0.5 rounded" style={{ background: "rgba(239,68,68,0.1)", color: "#EF4444", fontSize: 12 }}>{asset.environment ?? "Production"}</span>
            </div>
            <p style={{ color: "#6B7280", fontSize: 12, marginTop: 2 }}>{asset.os ?? "—"} · Last scanned {asset.lastScanAt ?? "—"}</p>
          </div>
        </div>
      </div>

      <div className="flex-1 overflow-y-auto p-6">
        <div className="grid grid-cols-3 gap-5">
          {/* Left col */}
          <div className="col-span-2 flex flex-col gap-5">
            {/* KPIs */}
            <div className="grid grid-cols-4 gap-4">
              {kpis.map((m) => (
                <div key={m.label} className="rounded-2xl p-4" style={{ background: "#151B2F", border: "1px solid rgba(255,255,255,0.07)" }}>
                  <div style={{ color: "#6B7280", fontSize: 11, marginBottom: 6 }}>{m.label}</div>
                  <div style={{ color: m.color, fontSize: 22, fontWeight: 700 }}>{m.value}</div>
                </div>
              ))}
            </div>

            {/* Services */}
            {services.length > 0 && (
              <div className="rounded-2xl p-5" style={{ background: "#151B2F", border: "1px solid rgba(255,255,255,0.07)" }}>
                <div style={{ color: "#9CA3AF", fontSize: 12, fontWeight: 600, marginBottom: 12 }}>SERVICES & TECHNOLOGIES</div>
                <div className="flex flex-col gap-2">
                  {services.map((s) => (
                    <div key={s.name} className="flex items-center gap-4 rounded-xl p-3" style={{ background: "rgba(255,255,255,0.03)" }}>
                      <div className="w-2 h-2 rounded-full flex-shrink-0" style={{ background: s.status === "running" ? "#10B981" : "#6B7280" }} />
                      <div className="flex-1">
                        <div style={{ color: "#E5E7EB", fontSize: 13, fontWeight: 500 }}>{s.name}</div>
                        <div style={{ color: "#6B7280", fontSize: 11 }}>{s.version}{s.port ? ` · Port ${s.port}` : ""}</div>
                      </div>
                      {s.cves > 0 ? (
                        <span className="px-2 py-0.5 rounded" style={{ background: "rgba(239,68,68,0.1)", color: "#EF4444", fontSize: 11 }}>{s.cves} CVEs</span>
                      ) : (
                        <span style={{ color: "#10B981", fontSize: 11 }}>Clean</span>
                      )}
                    </div>
                  ))}
                </div>
              </div>
            )}

            {/* Risk trend */}
            <div className="rounded-2xl p-5" style={{ background: "#151B2F", border: "1px solid rgba(255,255,255,0.07)" }}>
              <div style={{ color: "#9CA3AF", fontSize: 12, fontWeight: 600, marginBottom: 12 }}>RISK SCORE TREND</div>
              <ResponsiveContainer width="100%" height={140}>
                <LineChart data={riskHistory}>
                  <XAxis dataKey="date" tick={{ fill: "#6B7280", fontSize: 10 }} axisLine={false} tickLine={false} />
                  <YAxis domain={[0, 10]} tick={{ fill: "#6B7280", fontSize: 10 }} axisLine={false} tickLine={false} />
                  <Tooltip contentStyle={{ background: "#1E2A45", border: "1px solid rgba(255,255,255,0.1)", borderRadius: 8, fontSize: 11 }} />
                  <Line type="monotone" dataKey="score" stroke="#EF4444" strokeWidth={2} dot={{ fill: "#EF4444", r: 3 }} />
                </LineChart>
              </ResponsiveContainer>
            </div>
          </div>

          {/* Right col */}
          <div className="flex flex-col gap-4">
            {/* System info */}
            <div className="rounded-2xl p-4" style={{ background: "#151B2F", border: "1px solid rgba(255,255,255,0.07)" }}>
              <div style={{ color: "#9CA3AF", fontSize: 11, fontWeight: 600, marginBottom: 10 }}>SYSTEM INFORMATION</div>
              {systemInfo.map(({ label, value }) => (
                <div key={label} className="flex justify-between py-2" style={{ borderBottom: "1px solid rgba(255,255,255,0.04)" }}>
                  <span style={{ color: "#6B7280", fontSize: 12 }}>{label}</span>
                  <span style={{ color: "#E5E7EB", fontSize: 12 }}>{value}</span>
                </div>
              ))}
            </div>

            {/* Scan history */}
            {scanHistory.length > 0 && (
              <div className="rounded-2xl p-4" style={{ background: "#151B2F", border: "1px solid rgba(255,255,255,0.07)" }}>
                <div style={{ color: "#9CA3AF", fontSize: 11, fontWeight: 600, marginBottom: 10 }}>SCAN HISTORY</div>
                {scanHistory.map((s) => (
                  <div key={s.scan} className="py-2" style={{ borderBottom: "1px solid rgba(255,255,255,0.04)" }}>
                    <div className="flex items-center justify-between">
                      <span style={{ color: "#4F8CFF", fontSize: 12 }}>{s.scan}</span>
                      <span style={{ color: s.findings > 0 ? "#F59E0B" : "#10B981", fontSize: 11 }}>{s.findings} findings</span>
                    </div>
                    <div style={{ color: "#6B7280", fontSize: 11 }}>{s.date} · {s.type}</div>
                  </div>
                ))}
              </div>
            )}
          </div>
        </div>
      </div>
    </div>
  );
}

// ─── Main Export ──────────────────────────────────────────────────────────────
export function AssetDetail() {
  const { id } = useParams<{ id: string }>();
  const navigate = useNavigate();
  const assetQuery = useAssetDetail(id ?? null);

  return (
    <QueryBoundary query={assetQuery} skeleton={<AssetDetailSkeleton />}>
      {(asset) => (
        <AssetDetailContent asset={asset} onBack={() => navigate("/assets")} />
      )}
    </QueryBoundary>
  );
}
