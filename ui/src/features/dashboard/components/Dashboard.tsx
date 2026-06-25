import { useState } from "react";
import {
  AreaChart, Area, PieChart, Pie, Cell,
  XAxis, YAxis, Tooltip, ResponsiveContainer,
} from "recharts";
import {
  AlertTriangle, Activity, Server, Shield, TrendingUp,
  TrendingDown, ArrowRight, Clock, ExternalLink, ChevronRight,
  AlertCircle, CheckCircle, Target, Eye,
} from "lucide-react";
import { useDashboardMetrics } from "@/features/dashboard/hooks/useDashboardMetrics";
import { DashboardSkeleton } from "@/features/dashboard/components/DashboardSkeleton";
import { QueryBoundary } from "@/shared/components/feedback/QueryBoundary";
import type { DashboardData } from "@/features/dashboard/types";

// ─── UI Constants (allowed: tidak ada business data) ─────────────────────────

const SEVERITY_COLORS: Record<string, string> = {
  Critical: "#EF4444",
  High: "#F97316",
  Medium: "#EAB308",
  Low: "#3B82F6",
};

const STATUS_COLORS: Record<string, { bg: string; text: string }> = {
  active:           { bg: "rgba(239,68,68,0.15)",   text: "#EF4444" },
  mitigated:        { bg: "rgba(16,185,129,0.15)",  text: "#10B981" },
  false_positive:   { bg: "rgba(107,114,128,0.15)", text: "#6B7280" },
  risk_accepted:    { bg: "rgba(79,140,255,0.15)",  text: "#4F8CFF" },
  out_of_scope:     { bg: "rgba(107,114,128,0.15)", text: "#6B7280" },
};

const SLA_COLORS: Record<string, { bg: string; text: string }> = {
  ok:       { bg: "rgba(16,185,129,0.1)",  text: "#10B981" },
  at_risk:  { bg: "rgba(245,158,11,0.1)",  text: "#F59E0B" },
  breached: { bg: "rgba(239,68,68,0.1)",   text: "#EF4444" },
};

const PERIODS = ["30d", "90d", "1y"] as const;
type Period = typeof PERIODS[number];

// ─── Sub-components ───────────────────────────────────────────────────────────

function KPICard({
  label, value, sub, trend, trendUp, color, icon: Icon, bgColor,
}: {
  label: string; value: string; sub?: string; trend?: string; trendUp?: boolean;
  color: string; icon: React.ElementType; bgColor: string;
}) {
  return (
    <div
      className="rounded-2xl p-5 relative overflow-hidden"
      style={{ background: "var(--color-bg-card, #151B2F)", border: "1px solid rgba(255,255,255,0.07)" }}
    >
      <div className="absolute top-0 right-0 w-24 h-24 rounded-full opacity-10 -mr-8 -mt-8" style={{ background: bgColor }} />
      <div className="flex items-start justify-between mb-4">
        <div className="w-10 h-10 rounded-xl flex items-center justify-center" style={{ background: `${bgColor}20` }}>
          <Icon size={18} color={color} />
        </div>
        {trend && (
          <div
            className="flex items-center gap-1 px-2 py-1 rounded-lg"
            style={{ background: trendUp ? "rgba(239,68,68,0.1)" : "rgba(16,185,129,0.1)", color: trendUp ? "#EF4444" : "#10B981", fontSize: 11 }}
          >
            {trendUp ? <TrendingUp size={10} /> : <TrendingDown size={10} />}
            {trend}
          </div>
        )}
      </div>
      <div style={{ color: "var(--color-text-primary, #E5E7EB)", fontSize: 28, fontWeight: 700, lineHeight: 1 }}>{value}</div>
      <div style={{ color: "var(--color-text-muted, #6B7280)", fontSize: 12, marginTop: 6 }}>{label}</div>
      {sub && <div style={{ color, fontSize: 11, marginTop: 4 }}>{sub}</div>}
    </div>
  );
}

function GradeCircle({ grade, score }: { grade: string; score: number }) {
  const color = score >= 75 ? "#10B981" : score >= 55 ? "#F59E0B" : "#EF4444";
  return (
    <div className="relative w-12 h-12">
      <svg viewBox="0 0 40 40" className="w-full h-full -rotate-90">
        <circle cx="20" cy="20" r="16" fill="none" stroke="rgba(255,255,255,0.06)" strokeWidth="4" />
        <circle cx="20" cy="20" r="16" fill="none" stroke={color} strokeWidth="4"
          strokeDasharray={`${(score / 100) * 100.5} 100.5`} strokeLinecap="round" />
      </svg>
      <div className="absolute inset-0 flex items-center justify-center" style={{ color, fontSize: 11, fontWeight: 700 }}>
        {grade}
      </div>
    </div>
  );
}

const CustomTooltip = ({ active, payload, label }: any) => {
  if (active && payload && payload.length) {
    return (
      <div className="rounded-xl p-3" style={{ background: "#1E2A45", border: "1px solid rgba(255,255,255,0.1)", fontSize: 12 }}>
        <div style={{ color: "var(--color-text-secondary, #9CA3AF)", marginBottom: 4 }}>{label}</div>
        {payload.map((p: any) => (
          <div key={p.name} className="flex items-center gap-2">
            <div className="w-2 h-2 rounded-full" style={{ background: p.color }} />
            <span style={{ color: "var(--color-text-primary, #E5E7EB)" }}>{p.name}: <b>{p.value}</b></span>
          </div>
        ))}
      </div>
    );
  }
  return null;
};

// ─── Dashboard Content ────────────────────────────────────────────────────────

function DashboardContent({ data, period, onPeriodChange }: {
  data: DashboardData;
  period: Period;
  onPeriodChange: (p: Period) => void;
}) {
  const { kpis, risk_trend, severity_distribution, product_grades, kev_alerts, recent_scans, sla_breaches } = data;

  const severityChartItems = [
    { name: "Critical", value: severity_distribution.critical, color: "var(--color-status-error, #EF4444)" },
    { name: "High",     value: severity_distribution.high,     color: "var(--color-severity-high, #F97316)" },
    { name: "Medium",   value: severity_distribution.medium,   color: "var(--color-severity-medium, #EAB308)" },
    { name: "Low",      value: severity_distribution.low,      color: "var(--color-severity-low, #3B82F6)" },
  ];

  const handleExportPDF = () => {
    window.print();
  };

  return (
    <div className="flex-1 overflow-y-auto" style={{ background: "var(--color-bg-page, #0B1020)", padding: "24px" }}>
      {/* Page header */}
      <div className="flex items-center justify-between mb-6">
        <div>
          <h1 style={{ color: "var(--color-text-primary, #E5E7EB)", fontWeight: 700, fontSize: 20 }}>Executive Security Dashboard</h1>
          <p style={{ color: "var(--color-text-muted, #6B7280)", fontSize: 13, marginTop: 2 }}>
            Real-time data · Refreshes every 60s
          </p>
        </div>
        <div className="flex items-center gap-3">
          {/* Period picker */}
          <div className="flex gap-1 rounded-xl p-1" style={{ background: "rgba(255,255,255,0.04)", border: "1px solid rgba(255,255,255,0.08)" }}>
            {PERIODS.map((p) => (
              <button
                key={p}
                onClick={() => onPeriodChange(p)}
                className="px-3 py-1.5 rounded-lg text-xs font-medium transition-all"
                style={{
                  background: period === p ? "rgba(79,140,255,0.2)" : "transparent",
                  color: period === p ? "#4F8CFF" : "#6B7280",
                  border: period === p ? "1px solid rgba(79,140,255,0.3)" : "1px solid transparent",
                  cursor: "pointer",
                }}
              >
                {p}
              </button>
            ))}
          </div>
          <button
            onClick={handleExportPDF}
            className="flex items-center gap-2 px-4 py-2 rounded-xl"
            style={{ background: "linear-gradient(135deg, #4F8CFF, #3B6FCC)", color: "white", fontSize: 13, cursor: "pointer", border: "none" }}
          >
            <ExternalLink size={14} />
            Export PDF
          </button>
        </div>
      </div>

      {/* KPI Row */}
      <div className="grid grid-cols-6 gap-4 mb-6">
        <KPICard label="Critical Findings" value={kpis.critical_findings.toLocaleString()}
          sub={`SLA: ${kpis.sla_breached} breached`} trend="+5%" trendUp color="#EF4444" bgColor="#EF4444" icon={AlertTriangle} />
        <KPICard label="High Findings" value={kpis.high_findings.toLocaleString()}
          sub={`${kpis.sla_at_risk} at risk`} trend="-3%" trendUp={false} color="#F97316" bgColor="#F97316" icon={AlertCircle} />
        <KPICard label="Total Assets" value={kpis.total_assets.toLocaleString()}
          sub={`${kpis.high_risk_assets} high risk`} trend="+2%" trendUp={false} color="#4F8CFF" bgColor="#4F8CFF" icon={Server} />
        <KPICard label="Active Scans" value={String(kpis.active_scans)}
          sub={`${kpis.queued_scans} queued`} color="#10B981" bgColor="#10B981" icon={Activity} />
        <KPICard label="Security Grade" value={kpis.security_grade}
          sub={`Score: ${kpis.security_score}/100`} color="#EAB308" bgColor="#EAB308" icon={Shield} />
        <KPICard label="SLA Compliance" value={`${kpis.sla_compliance}%`}
          sub={`${kpis.sla_at_risk} at risk`} trend="-1.2%" trendUp color="#A78BFA" bgColor="#A78BFA" icon={Target} />
      </div>

      {/* Charts row */}
      <div className="grid grid-cols-3 gap-4 mb-6">
        {/* Risk Trend */}
        <div className="col-span-2 rounded-2xl p-5" style={{ background: "var(--color-bg-card, #151B2F)", border: "1px solid rgba(255,255,255,0.07)" }}>
          <div className="flex items-center justify-between mb-5">
            <div>
              <h3 style={{ color: "var(--color-text-primary, #E5E7EB)", fontSize: 14, fontWeight: 600 }}>Risk Trend</h3>
              <p style={{ color: "var(--color-text-muted, #6B7280)", fontSize: 12 }}>Findings over time by severity</p>
            </div>
            <div className="flex gap-3">
              {["Critical", "High", "Medium"].map((s) => (
                <div key={s} className="flex items-center gap-1.5">
                  <div className="w-2 h-2 rounded-full" style={{ background: SEVERITY_COLORS[s] }} />
                  <span style={{ color: "var(--color-text-muted, #6B7280)", fontSize: 11 }}>{s}</span>
                </div>
              ))}
            </div>
          </div>
          <ResponsiveContainer width="100%" height={200}>
            <AreaChart data={risk_trend}>
              <defs>
                <linearGradient id="critGrad" x1="0" y1="0" x2="0" y2="1">
                  <stop offset="0%" stopColor="#EF4444" stopOpacity={0.3} /><stop offset="100%" stopColor="#EF4444" stopOpacity={0} />
                </linearGradient>
                <linearGradient id="highGrad" x1="0" y1="0" x2="0" y2="1">
                  <stop offset="0%" stopColor="#F97316" stopOpacity={0.25} /><stop offset="100%" stopColor="#F97316" stopOpacity={0} />
                </linearGradient>
                <linearGradient id="medGrad" x1="0" y1="0" x2="0" y2="1">
                  <stop offset="0%" stopColor="#EAB308" stopOpacity={0.2} /><stop offset="100%" stopColor="#EAB308" stopOpacity={0} />
                </linearGradient>
              </defs>
              <XAxis dataKey="month" tick={{ fill: "#6B7280", fontSize: 11 }} axisLine={false} tickLine={false} />
              <YAxis tick={{ fill: "#6B7280", fontSize: 11 }} axisLine={false} tickLine={false} />
              <Tooltip content={<CustomTooltip />} />
              <Area type="monotone" dataKey="medium" stroke="#EAB308" strokeWidth={2} fill="url(#medGrad)" name="Medium" />
              <Area type="monotone" dataKey="high"   stroke="#F97316" strokeWidth={2} fill="url(#highGrad)" name="High" />
              <Area type="monotone" dataKey="critical" stroke="#EF4444" strokeWidth={2} fill="url(#critGrad)" name="Critical" />
            </AreaChart>
          </ResponsiveContainer>
        </div>

        {/* Severity Donut */}
        <div className="rounded-2xl p-5" style={{ background: "var(--color-bg-card, #151B2F)", border: "1px solid rgba(255,255,255,0.07)" }}>
          <div className="mb-4">
            <h3 style={{ color: "var(--color-text-primary, #E5E7EB)", fontSize: 14, fontWeight: 600 }}>Severity Distribution</h3>
            <p style={{ color: "var(--color-text-muted, #6B7280)", fontSize: 12 }}>Total: {(severity_distribution.total).toLocaleString()} findings</p>
          </div>
          <div className="flex items-center justify-center">
            <ResponsiveContainer width="100%" height={160}>
              <PieChart>
                <Pie data={severityChartItems} cx="50%" cy="50%" innerRadius={50} outerRadius={75} paddingAngle={3} dataKey="value">
                  {severityChartItems.map((entry, index) => (<Cell key={index} fill={entry.color} />))}
                </Pie>
                <Tooltip formatter={(v: any) => [v, ""]} contentStyle={{ background: "#1E2A45", border: "1px solid rgba(255,255,255,0.1)", borderRadius: 8, fontSize: 12 }} />
              </PieChart>
            </ResponsiveContainer>
          </div>
          <div className="grid grid-cols-2 gap-2 mt-2">
            {severityChartItems.map((d) => (
              <div key={d.name} className="flex items-center gap-2">
                <div className="w-2.5 h-2.5 rounded-sm" style={{ background: d.color }} />
                <div>
                  <div style={{ color: "var(--color-text-primary, #E5E7EB)", fontSize: 12, fontWeight: 600 }}>{d.value}</div>
                  <div style={{ color: "var(--color-text-muted, #6B7280)", fontSize: 10 }}>{d.name}</div>
                </div>
              </div>
            ))}
          </div>
        </div>
      </div>

      {/* Middle row */}
      <div className="grid grid-cols-3 gap-4 mb-6">
        {/* Product Security Grades */}
        <div className="col-span-1 rounded-2xl p-5" style={{ background: "var(--color-bg-card, #151B2F)", border: "1px solid rgba(255,255,255,0.07)" }}>
          <div className="flex items-center justify-between mb-4">
            <h3 style={{ color: "var(--color-text-primary, #E5E7EB)", fontSize: 14, fontWeight: 600 }}>Product Security</h3>
            <button style={{ color: "var(--color-primary, #4F8CFF)", fontSize: 12, background: "none", border: "none", cursor: "pointer" }}>View all</button>
          </div>
          <div className="flex flex-col gap-3">
            {product_grades.map((p) => (
              <div key={p.id} className="flex items-center gap-3">
                <GradeCircle grade={p.grade} score={p.score} />
                <div className="flex-1 min-w-0">
                  <div style={{ color: "var(--color-text-primary, #E5E7EB)", fontSize: 13, fontWeight: 500 }}>{p.name}</div>
                  <div className="flex gap-2 mt-0.5">
                    <span style={{ color: "var(--color-status-error, #EF4444)", fontSize: 11 }}>{p.critical_count} crit</span>
                    <span style={{ color: "var(--color-severity-high, #F97316)", fontSize: 11 }}>{p.high_count} high</span>
                  </div>
                </div>
                <div className="w-1.5 h-8 rounded-full" style={{ background: p.score >= 75 ? "#10B981" : p.score >= 55 ? "#F59E0B" : "#EF4444", opacity: 0.6 }} />
              </div>
            ))}
          </div>
        </div>

        {/* KEV Alerts */}
        <div className="rounded-2xl p-5" style={{ background: "var(--color-bg-card, #151B2F)", border: "1px solid rgba(255,255,255,0.07)" }}>
          <div className="flex items-center justify-between mb-4">
            <div className="flex items-center gap-2">
              <div className="w-2 h-2 rounded-full animate-pulse" style={{ background: "#EF4444" }} />
              <h3 style={{ color: "var(--color-text-primary, #E5E7EB)", fontSize: 14, fontWeight: 600 }}>KEV Alerts</h3>
            </div>
            <span className="px-2 py-1 rounded-lg" style={{ background: "rgba(239,68,68,0.1)", color: "var(--color-status-error, #EF4444)", fontSize: 11 }}>
              {kev_alerts.length} active
            </span>
          </div>
          <div className="flex flex-col gap-3">
            {kev_alerts.map((k) => (
              <div key={k.cve_id} className="flex items-center gap-3 p-3 rounded-xl" style={{ background: "rgba(239,68,68,0.05)", border: "1px solid rgba(239,68,68,0.1)" }}>
                <AlertCircle size={14} color="#EF4444" style={{ flexShrink: 0 }} />
                <div className="flex-1 min-w-0">
                  <div style={{ color: "var(--color-status-error, #EF4444)", fontSize: 12, fontWeight: 600 }}>{k.cve_id}</div>
                  <div style={{ color: "var(--color-text-muted, #6B7280)", fontSize: 11 }}>{k.vendor} · {k.product}</div>
                </div>
                <span style={{ color: "var(--color-text-faint, #4B5563)", fontSize: 10 }}>{new Date(k.date_added).toLocaleDateString()}</span>
              </div>
            ))}
          </div>
          <div className="mt-4 pt-3" style={{ borderTop: "1px solid rgba(255,255,255,0.06)" }}>
            <div className="grid grid-cols-3 gap-2 text-center">
              <div>
                <div style={{ color: "var(--color-status-error, #EF4444)", fontSize: 16, fontWeight: 700 }}>{kev_alerts.length}</div>
                <div style={{ color: "var(--color-text-muted, #6B7280)", fontSize: 10 }}>In Platform</div>
              </div>
              <div>
                <div style={{ color: "var(--color-status-warning, #F59E0B)", fontSize: 16, fontWeight: 700 }}>{kev_alerts.filter(k => k.is_ransomware).length}</div>
                <div style={{ color: "var(--color-text-muted, #6B7280)", fontSize: 10 }}>Ransomware</div>
              </div>
              <div>
                <div style={{ color: "var(--color-primary, #4F8CFF)", fontSize: 16, fontWeight: 700 }}>{kpis.critical_findings}</div>
                <div style={{ color: "var(--color-text-muted, #6B7280)", fontSize: 10 }}>Unmitigated</div>
              </div>
            </div>
          </div>
        </div>

        {/* Recent Scans + SLA Breaches */}
        <div className="flex flex-col gap-4">
          {/* Recent Scans */}
          <div className="rounded-2xl p-4" style={{ background: "var(--color-bg-card, #151B2F)", border: "1px solid rgba(255,255,255,0.07)" }}>
            <h3 style={{ color: "var(--color-text-primary, #E5E7EB)", fontSize: 13, fontWeight: 600, marginBottom: 12 }}>Recent Scans</h3>
            <div className="flex flex-col gap-2">
              {recent_scans.map((s) => (
                <div key={s.id} className="flex items-center gap-3">
                  <div className="w-7 h-7 rounded-lg flex items-center justify-center"
                    style={{ background: s.status === "running" ? "rgba(79,140,255,0.15)" : "rgba(16,185,129,0.15)" }}>
                    {s.status === "running" ? <Activity size={12} color="#4F8CFF" /> : <CheckCircle size={12} color="#10B981" />}
                  </div>
                  <div className="flex-1 min-w-0">
                    <div style={{ color: "var(--color-text-primary, #E5E7EB)", fontSize: 12 }}>{s.name}</div>
                    <div style={{ color: "var(--color-text-muted, #6B7280)", fontSize: 10 }}>{s.type.toUpperCase()} · {s.started_at ? new Date(s.started_at).toLocaleTimeString() : "—"}</div>
                  </div>
                  <span style={{ color: s.finding_count > 20 ? "#EF4444" : "#F59E0B", fontSize: 11, fontWeight: 600 }}>
                    {s.finding_count}
                  </span>
                </div>
              ))}
            </div>
          </div>

          {/* SLA Breaches */}
          <div className="rounded-2xl p-4" style={{ background: "var(--color-bg-card, #151B2F)", border: "1px solid rgba(255,255,255,0.07)" }}>
            <div className="flex items-center justify-between mb-3">
              <h3 style={{ color: "var(--color-text-primary, #E5E7EB)", fontSize: 13, fontWeight: 600 }}>SLA Breaches</h3>
              <span className="px-2 py-0.5 rounded-lg" style={{ background: "rgba(239,68,68,0.1)", color: "var(--color-status-error, #EF4444)", fontSize: 11 }}>
                {sla_breaches.filter(b => b.days_overdue > 0).length} overdue
              </span>
            </div>
            <div className="flex flex-col gap-2">
              {sla_breaches.map((b) => (
                <div key={b.finding_id} className="flex items-center gap-3">
                  <Clock size={12} color={b.days_overdue > 0 ? "#EF4444" : "#F59E0B"} />
                  <div className="flex-1 min-w-0">
                    <div style={{ color: "var(--color-text-primary, #E5E7EB)", fontSize: 11 }} className="truncate">{b.title}</div>
                    <div style={{ color: b.days_overdue > 0 ? "#EF4444" : "#F59E0B", fontSize: 10 }}>{b.days_overdue > 0 ? `Overdue ${b.days_overdue}d` : `${Math.abs(b.days_overdue)}d left`}</div>
                  </div>
                  <span className="px-1.5 py-0.5 rounded"
                    style={{ background: (SEVERITY_COLORS[b.severity] ?? "#6B7280") + "20", color: SEVERITY_COLORS[b.severity] ?? "#6B7280", fontSize: 10 }}>
                    {b.severity}
                  </span>
                </div>
              ))}
            </div>
          </div>
        </div>
      </div>

      {/* SLA Summary footer */}
      <div className="flex items-center justify-end">
        <button className="flex items-center gap-1 text-sm" style={{ color: "var(--color-primary, #4F8CFF)", background: "none", border: "none", cursor: "pointer", fontSize: 12 }}>
          View all findings <ChevronRight size={13} />
        </button>
      </div>
    </div>
  );
}

// ─── Main Dashboard component ─────────────────────────────────────────────────

export function Dashboard() {
  const [period, setPeriod] = useState<Period>("30d");
  const metricsQuery = useDashboardMetrics(period);

  return (
    <QueryBoundary query={metricsQuery} skeleton={<DashboardSkeleton />}>
      {(data) => (
        <DashboardContent data={data} period={period} onPeriodChange={setPeriod} />
      )}
    </QueryBoundary>
  );
}
