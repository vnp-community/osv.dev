import { Clock, AlertTriangle, CheckCircle, TrendingDown } from "lucide-react";
import { BarChart, Bar, LineChart, Line, XAxis, YAxis, Tooltip, ResponsiveContainer, Cell } from "recharts";
import { useQuery } from "@tanstack/react-query";
import { apiClient } from "@/shared/api/client";
import { ENDPOINTS } from "@/shared/api/endpoints";
import { QueryBoundary } from "@/shared/components/feedback/QueryBoundary";

// ── Types ─────────────────────────────────────────────────────────────────────
// The gateway falls back to { total, breached, at_risk, on_track, items }
// when sla-service is unavailable.  The real response from sla-service follows
// the SLADashboardData shape (snake_case, summary-keyed).  We accept both by
// making every field optional and normalising inside the hook.

interface SLAOverviewRaw {
  // Flat fallback shape (gateway stub)
  total?: number;
  breached?: number;
  at_risk?: number;
  on_track?: number;
  items?: unknown[];

  // Real sla-service shape
  summary?: {
    total_active_findings?: number;
    compliance_percent?: number;
    breached?: number;
    at_risk?: number;
    ok?: number;
  };
  compliance_trend?: Array<{ month: string; compliance_percent: number }>;
  by_product?: Array<{
    product_name: string;
    compliance_percent: number;
    breached?: number;
    at_risk?: number;
    ok?: number;
  }>;
  at_risk_findings?: Array<{
    finding_id: string;
    title: string;
    severity: string;
    sla_expiration_date: string;
    hours_remaining?: number;
  }>;
}

interface SLANormalised {
  compliant: number;
  atRisk: number;
  breached: number;
  avgDaysLeft: number;
  monthlyTrend: Array<{ month: string; compliance: number }>;
  productCompliance: Array<{ name: string; compliance: number }>;
  upcomingBreaches: Array<{ id: string; title: string; severity: string; dueDate: string; daysLeft: number }>;
}

function normalise(raw: SLAOverviewRaw): SLANormalised {
  const compliant = raw.summary?.ok ?? raw.on_track ?? 0;
  const atRisk    = raw.summary?.at_risk ?? raw.at_risk ?? 0;
  const breached  = raw.summary?.breached ?? raw.breached ?? 0;

  const monthlyTrend = (raw.compliance_trend ?? []).map((t) => ({
    month: t.month,
    compliance: t.compliance_percent ?? 0,
  }));

  const productCompliance = (raw.by_product ?? []).map((p) => ({
    name: p.product_name ?? "Unknown",
    compliance: p.compliance_percent ?? 0,
  }));

  const upcoming = (raw.at_risk_findings ?? [])
    .slice()
    .sort((a, b) => (a.hours_remaining ?? 0) - (b.hours_remaining ?? 0))
    .map((f) => ({
      id:       f.finding_id,
      title:    f.title,
      severity: f.severity ?? "Unknown",
      dueDate:  f.sla_expiration_date
        ? new Date(f.sla_expiration_date).toLocaleDateString()
        : "—",
      daysLeft: Math.max(0, Math.ceil((f.hours_remaining ?? 0) / 24)),
    }));

  const avgDaysLeft = upcoming.length > 0
    ? Math.round(upcoming.reduce((s, f) => s + f.daysLeft, 0) / upcoming.length)
    : 0;

  return { compliant, atRisk, breached, avgDaysLeft, monthlyTrend, productCompliance, upcomingBreaches: upcoming };
}

function useSLAOverview() {
  return useQuery<SLANormalised>({
    queryKey: ['sla', 'overview'],
    queryFn: async () => {
      const { data } = await apiClient.get<SLAOverviewRaw>(ENDPOINTS.sla.overview);
      return normalise(data ?? {});
    },
    staleTime: 5 * 60_000,
  });
}

// ── UI ────────────────────────────────────────────────────────────────────────

const CustomTooltip = ({ active, payload, label }: any) => {
  if (active && payload?.length) return (
    <div className="rounded-xl p-2" style={{ background: "#1E2A45", border: "1px solid rgba(255,255,255,0.1)", fontSize: 11 }}>
      <div style={{ color: "#9CA3AF", marginBottom: 3 }}>{label}</div>
      {payload.map((p: any) => <div key={p.name} style={{ color: p.color }}>{p.name}: {p.value}%</div>)}
    </div>
  );
  return null;
};

function SLASkeleton() {
  return (
    <div className="flex-1 overflow-y-auto px-6 py-5 animate-pulse" style={{ background: "#0B1020" }}>
      <div className="grid grid-cols-4 gap-4 mb-5">
        {Array.from({ length: 4 }).map((_, i) => <div key={i} className="rounded-2xl h-24" style={{ background: "#151B2F" }} />)}
      </div>
      <div className="grid grid-cols-2 gap-4 mb-5">
        <div className="rounded-2xl h-48" style={{ background: "#151B2F" }} />
        <div className="rounded-2xl h-48" style={{ background: "#151B2F" }} />
      </div>
    </div>
  );
}

export function SLADashboard() {
  const slaQuery = useSLAOverview();

  return (
    <QueryBoundary query={slaQuery} skeleton={<SLASkeleton />}>
      {(sla) => {
        const total = sla.compliant + sla.atRisk + sla.breached;
        const compliancePct = total > 0 ? ((sla.compliant / total) * 100).toFixed(1) : "0";

        return (
          <div className="flex-1 overflow-y-auto px-6 py-5" style={{ background: "#0B1020" }}>
            <div className="mb-5">
              <h2 style={{ color: "#E5E7EB", fontSize: 18, fontWeight: 700 }}>SLA Dashboard</h2>
              <p style={{ color: "#6B7280", fontSize: 12 }}>Service Level Agreement tracking across all products</p>
            </div>

            {/* KPIs */}
            <div className="grid grid-cols-4 gap-4 mb-5">
              {[
                { label: "Overall Compliance", value: `${compliancePct}%`, trend: null, color: "#10B981", icon: CheckCircle },
                { label: "Compliant Findings", value: sla.compliant.toLocaleString(), sub: `of ${total} total`, color: "#10B981", icon: CheckCircle },
                { label: "SLA Breached", value: sla.breached, sub: `${sla.atRisk} at risk`, color: "#EF4444", icon: AlertTriangle },
                { label: "Avg Days Left", value: sla.avgDaysLeft, sub: "To SLA deadline", color: "#F59E0B", icon: Clock },
              ].map(({ label, value, trend, sub, color, icon: Icon }) => (
                <div key={label} className="rounded-2xl p-5" style={{ background: "#151B2F", border: "1px solid rgba(255,255,255,0.07)" }}>
                  <div className="flex items-center justify-between mb-3">
                    <div className="w-9 h-9 rounded-xl flex items-center justify-center" style={{ background: color + "15" }}>
                      <Icon size={17} color={color} />
                    </div>
                    {trend && (
                      <span className="flex items-center gap-1 text-xs" style={{ color: "#10B981" }}>
                        <TrendingDown size={11} />{trend}
                      </span>
                    )}
                  </div>
                  <div style={{ color, fontSize: 26, fontWeight: 700 }}>{value}</div>
                  <div style={{ color: "#E5E7EB", fontSize: 12, marginTop: 4 }}>{label}</div>
                  {sub && <div style={{ color: "#6B7280", fontSize: 11, marginTop: 2 }}>{sub}</div>}
                </div>
              ))}
            </div>

            <div className="grid grid-cols-2 gap-4 mb-5">
              {/* SLA Trend */}
              <div className="rounded-2xl p-5" style={{ background: "#151B2F", border: "1px solid rgba(255,255,255,0.07)" }}>
                <h3 style={{ color: "#E5E7EB", fontSize: 14, fontWeight: 600, marginBottom: 4 }}>Compliance Trend (6m)</h3>
                <p style={{ color: "#6B7280", fontSize: 12, marginBottom: 14 }}>Platform-wide SLA compliance %</p>
                {sla.monthlyTrend.length > 0 ? (
                  <ResponsiveContainer width="100%" height={160}>
                    <LineChart data={sla.monthlyTrend}>
                      <XAxis dataKey="month" tick={{ fill: "#6B7280", fontSize: 10 }} axisLine={false} tickLine={false} />
                      <YAxis domain={[80, 100]} tick={{ fill: "#6B7280", fontSize: 10 }} axisLine={false} tickLine={false} unit="%" />
                      <Tooltip content={<CustomTooltip />} />
                      <Line type="monotone" dataKey="compliance" stroke="#10B981" strokeWidth={2.5} dot={false} name="Compliant" />
                    </LineChart>
                  </ResponsiveContainer>
                ) : (
                  <div className="flex items-center justify-center h-40" style={{ color: "#6B7280", fontSize: 12 }}>
                    No trend data available
                  </div>
                )}
              </div>

              {/* Product compliance */}
              <div className="rounded-2xl p-5" style={{ background: "#151B2F", border: "1px solid rgba(255,255,255,0.07)" }}>
                <h3 style={{ color: "#E5E7EB", fontSize: 14, fontWeight: 600, marginBottom: 4 }}>By Product</h3>
                <p style={{ color: "#6B7280", fontSize: 12, marginBottom: 14 }}>SLA compliance per product</p>
                {sla.productCompliance.length > 0 ? (
                  <ResponsiveContainer width="100%" height={160}>
                    <BarChart data={sla.productCompliance} layout="vertical" barSize={14}>
                      <XAxis type="number" domain={[0, 100]} tick={{ fill: "#6B7280", fontSize: 9 }} axisLine={false} tickLine={false} unit="%" />
                      <YAxis type="category" dataKey="name" tick={{ fill: "#9CA3AF", fontSize: 10 }} axisLine={false} tickLine={false} width={80} />
                      <Tooltip content={<CustomTooltip />} />
                      <Bar dataKey="compliance" name="Compliance" radius={[0, 3, 3, 0]}>
                        {sla.productCompliance.map((p, i) => <Cell key={i} fill={p.compliance >= 95 ? "#10B981" : p.compliance >= 90 ? "#F59E0B" : "#EF4444"} />)}
                      </Bar>
                    </BarChart>
                  </ResponsiveContainer>
                ) : (
                  <div className="flex items-center justify-center h-40" style={{ color: "#6B7280", fontSize: 12 }}>
                    No product data available
                  </div>
                )}
              </div>
            </div>

            {/* Upcoming Breaches */}
            <div className="rounded-2xl" style={{ background: "#151B2F", border: "1px solid rgba(245,158,11,0.2)" }}>
              <div className="px-5 py-3 flex items-center gap-2" style={{ borderBottom: "1px solid rgba(255,255,255,0.06)" }}>
                <Clock size={14} color="#F59E0B" />
                <h3 style={{ color: "#F59E0B", fontSize: 13, fontWeight: 600 }}>Upcoming SLA Breaches</h3>
                <span className="ml-auto px-2 py-0.5 rounded-lg" style={{ background: "rgba(245,158,11,0.1)", color: "#F59E0B", fontSize: 11 }}>Next 7 days</span>
              </div>
              <div className="p-4 flex flex-col gap-3">
                {sla.upcomingBreaches.length === 0 ? (
                  <div className="text-center py-6" style={{ color: "#6B7280", fontSize: 12 }}>
                    No upcoming SLA breaches 🎉
                  </div>
                ) : sla.upcomingBreaches.map(f => (
                  <div key={f.id} className="flex items-center gap-3 p-3 rounded-xl" style={{ background: "rgba(245,158,11,0.05)", border: "1px solid rgba(245,158,11,0.1)" }}>
                    <div className="w-8 h-8 rounded-xl flex items-center justify-center" style={{ background: f.daysLeft <= 1 ? "rgba(239,68,68,0.15)" : "rgba(245,158,11,0.15)" }}>
                      <span style={{ color: f.daysLeft <= 1 ? "#EF4444" : "#F59E0B", fontSize: 11, fontWeight: 700 }}>{f.daysLeft}d</span>
                    </div>
                    <div className="flex-1 min-w-0">
                      <div style={{ color: "#E5E7EB", fontSize: 12 }} className="truncate">{f.title}</div>
                      <div style={{ color: "#6B7280", fontSize: 11 }}>Due {f.dueDate}</div>
                    </div>
                    <span className="px-2 py-0.5 rounded" style={{ background: f.severity === "Critical" ? "rgba(239,68,68,0.15)" : "rgba(249,115,22,0.15)", color: f.severity === "Critical" ? "#EF4444" : "#F97316", fontSize: 10 }}>{f.severity}</span>
                  </div>
                ))}
              </div>
            </div>
          </div>
        );
      }}
    </QueryBoundary>
  );
}
