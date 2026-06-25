import { useState } from "react";
import { useNavigate } from "react-router";
import { Search, Eye } from "lucide-react";
import { useAssets } from "@/features/assets/hooks/useAssets";
import { QueryBoundary } from "@/shared/components/feedback/QueryBoundary";
import type { AssetsListParams } from "@/features/assets/api/assetApi";

const RISK_COLOR = (score: number) =>
  score >= 9 ? "#EF4444" : score >= 7 ? "#F97316" : score >= 4 ? "#EAB308" : "#10B981";

const ENV_STYLES: Record<string, { bg: string; color: string }> = {
  Production:    { bg: "rgba(239,68,68,0.1)",    color: "var(--color-status-error, #EF4444)" },
  Development:   { bg: "rgba(79,140,255,0.1)",   color: "var(--color-primary, #4F8CFF)" },
  Infrastructure:{ bg: "rgba(167,139,250,0.1)",  color: "var(--color-ai, #A78BFA)" },
  Network:       { bg: "rgba(245,158,11,0.1)",   color: "var(--color-status-warning, #F59E0B)" },
};

function AssetsSkeleton() {
  return (
    <div className="flex-1 overflow-hidden animate-pulse">
      {Array.from({ length: 8 }).map((_, i) => (
        <div key={i} className="h-12 mx-4 my-1.5 rounded-xl" style={{ background: "rgba(255,255,255,0.06)" }} />
      ))}
    </div>
  );
}

export function AssetInventory({ onViewDetail }: { onViewDetail?: () => void }) {
  const navigate = useNavigate();
  const handleViewDetail = (assetId: string) => {
    onViewDetail ? onViewDetail() : navigate(`/assets/${assetId}`);
  };

  const [search, setSearch] = useState("");
  const [filterEnv, setFilterEnv] = useState("All");
  const [filterRisk, setFilterRisk] = useState<string>("All");

  const queryParams: AssetsListParams = {
    query: search || undefined,
    riskLevel: filterRisk !== "All" ? (filterRisk.toLowerCase() as AssetsListParams["riskLevel"]) : undefined,
  };

  const assetsQuery = useAssets(queryParams);

  return (
    <div className="flex flex-col flex-1 overflow-hidden" style={{ background: "var(--color-bg-page, #0B1020)" }}>
      {/* Header + KPIs */}
      <div className="px-6 py-4" style={{ borderBottom: "1px solid rgba(255,255,255,0.06)" }}>
        <div className="flex items-center justify-between mb-4">
          <div>
            <h1 style={{ color: "var(--color-text-primary, #E5E7EB)", fontSize: 18, fontWeight: 700 }}>Asset Inventory</h1>
            <p style={{ color: "var(--color-text-muted, #6B7280)", fontSize: 12 }}>
              {assetsQuery.data?.total ?? "…"} total assets · Last sync 2h ago
            </p>
          </div>
        </div>
        {/* Filters */}
        <div className="flex items-center gap-3">
          <div className="relative">
            <Search size={13} color="#4B5563" style={{ position: "absolute", left: 10, top: "50%", transform: "translateY(-50%)" }} />
            <input
              value={search}
              onChange={(e) => setSearch(e.target.value)}
              placeholder="Search IP, hostname..."
              className="rounded-lg pl-8 pr-3 py-2 outline-none"
              style={{ background: "var(--color-bg-card, #151B2F)", border: "1px solid rgba(255,255,255,0.08)", color: "var(--color-text-primary, #E5E7EB)", fontSize: 12, width: 220 }}
            />
          </div>
          <select
            value={filterEnv}
            onChange={(e) => setFilterEnv(e.target.value)}
            className="rounded-lg px-3 py-2 outline-none"
            style={{ background: "var(--color-bg-card, #151B2F)", border: "1px solid rgba(255,255,255,0.08)", color: "var(--color-text-secondary, #9CA3AF)", fontSize: 12 }}
          >
            <option value="All">All Environments</option>
            <option>Production</option>
            <option>Development</option>
            <option>Infrastructure</option>
            <option>Network</option>
          </select>
          <select
            value={filterRisk}
            onChange={(e) => setFilterRisk(e.target.value)}
            className="rounded-lg px-3 py-2 outline-none"
            style={{ background: "var(--color-bg-card, #151B2F)", border: "1px solid rgba(255,255,255,0.08)", color: "var(--color-text-secondary, #9CA3AF)", fontSize: 12 }}
          >
            <option value="All">All Risk Levels</option>
            <option value="critical">Critical</option>
            <option value="high">High</option>
            <option value="medium">Medium</option>
            <option value="low">Low</option>
          </select>
        </div>
      </div>

      <QueryBoundary query={assetsQuery} skeleton={<AssetsSkeleton />}>
        {(data) => {
          const filtered = data.assets.filter((a) => {
            if (filterEnv !== "All" && (a as any).environment !== filterEnv) return false;
            return true;
          });

          return (
            <div className="flex-1 overflow-y-auto">
              <table className="w-full">
                <thead style={{ position: "sticky", top: 0, background: "#0D1525", zIndex: 5 }}>
                  <tr style={{ borderBottom: "1px solid rgba(255,255,255,0.06)" }}>
                    {["IP Address", "Hostname", "OS", "Environment", "Risk Score", "Open Ports", "Last Scan", "Findings", "Tags", ""].map((h) => (
                      <th key={h} className="px-4 py-3 text-left" style={{ color: "var(--color-text-muted, #6B7280)", fontSize: 11, fontWeight: 600, letterSpacing: 0.5 }}>{h}</th>
                    ))}
                  </tr>
                </thead>
                <tbody>
                  {filtered.map((a) => {
                    const env = (a as any).environment ?? "Production";
                    const envStyle = ENV_STYLES[env] ?? ENV_STYLES.Production;
                    const riskScore = a.riskScore ?? 0;
                    const ports: number[] = a.services?.map((s) => s.port) ?? [];
                    const tags: string[] = a.tags ?? [];
                    const lastScan = a.lastSeenAt ? new Date(a.lastSeenAt).toLocaleDateString() : "—";

                    return (
                      <tr
                        key={a.id}
                        className="cursor-pointer transition-all"
                        style={{ borderBottom: "1px solid rgba(255,255,255,0.04)" }}
                        onMouseEnter={(e) => (e.currentTarget.style.background = "rgba(255,255,255,0.02)")}
                        onMouseLeave={(e) => (e.currentTarget.style.background = "transparent")}
                      >
                        <td className="px-4 py-3"><span style={{ color: "var(--color-primary, #4F8CFF)", fontSize: 12, fontFamily: "monospace" }}>{a.ip}</span></td>
                        <td className="px-4 py-3"><span style={{ color: "var(--color-text-primary, #E5E7EB)", fontSize: 12 }}>{a.hostname}</span></td>
                        <td className="px-4 py-3"><span style={{ color: "var(--color-text-secondary, #9CA3AF)", fontSize: 12 }}>{a.os}</span></td>
                        <td className="px-4 py-3">
                          <span className="px-2 py-0.5 rounded" style={{ ...envStyle, fontSize: 11 }}>{env}</span>
                        </td>
                        <td className="px-4 py-3">
                          <div className="flex items-center gap-2">
                            <div className="w-16 h-1.5 rounded-full overflow-hidden" style={{ background: "rgba(255,255,255,0.1)" }}>
                              <div className="h-full rounded-full" style={{ width: `${(riskScore / 10) * 100}%`, background: RISK_COLOR(riskScore) }} />
                            </div>
                            <span style={{ color: RISK_COLOR(riskScore), fontSize: 13, fontWeight: 700 }}>{riskScore.toFixed(1)}</span>
                          </div>
                        </td>
                        <td className="px-4 py-3">
                          <div className="flex gap-1 flex-wrap">
                            {ports.slice(0, 4).map((p) => (
                              <span key={p} className="px-1.5 py-0.5 rounded" style={{ background: "rgba(79,140,255,0.1)", color: "var(--color-primary, #4F8CFF)", fontSize: 10 }}>{p}</span>
                            ))}
                            {ports.length > 4 && <span style={{ color: "var(--color-text-muted, #6B7280)", fontSize: 10 }}>+{ports.length - 4}</span>}
                          </div>
                        </td>
                        <td className="px-4 py-3"><span style={{ color: "var(--color-text-muted, #6B7280)", fontSize: 12 }}>{lastScan}</span></td>
                        <td className="px-4 py-3">
                          <div className="flex gap-2">
                            {a.activeFindingCount > 0 ? (
                              <span style={{ color: "var(--color-severity-high, #F97316)", fontSize: 11 }}>{a.activeFindingCount} open</span>
                            ) : (
                              <span style={{ color: "var(--color-status-success, #10B981)", fontSize: 11 }}>Clean</span>
                            )}
                          </div>
                        </td>
                        <td className="px-4 py-3">
                          <div className="flex gap-1 flex-wrap">
                            {tags.slice(0, 2).map((t) => (
                              <span key={t} className="px-1.5 py-0.5 rounded" style={{ background: "rgba(255,255,255,0.06)", color: "var(--color-text-secondary, #9CA3AF)", fontSize: 10 }}>{t}</span>
                            ))}
                          </div>
                        </td>
                        <td className="px-4 py-3">
                          <button
                            onClick={() => handleViewDetail(a.id)}
                            className="w-7 h-7 rounded-lg flex items-center justify-center"
                            style={{ background: "rgba(255,255,255,0.05)", color: "var(--color-text-muted, #6B7280)", border: "none", cursor: "pointer" }}
                          >
                            <Eye size={12} />
                          </button>
                        </td>
                      </tr>
                    );
                  })}
                </tbody>
              </table>
            </div>
          );
        }}
      </QueryBoundary>
    </div>
  );
}
