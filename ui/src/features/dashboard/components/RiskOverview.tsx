import { useState } from "react";
import {
  RadarChart, Radar, PolarGrid, PolarAngleAxis,
  BarChart, Bar, XAxis, YAxis, Tooltip, ResponsiveContainer, Cell,
  LineChart, Line, CartesianGrid, Legend,
} from "recharts";
import {
  ShieldAlert, TrendingUp, TrendingDown, AlertTriangle, AlertCircle,
  CheckCircle, Clock, Target, Activity, ArrowRight, ChevronRight,
  Flame, Bug, Lock, Eye, ExternalLink,
} from "lucide-react";
import { useDashboardMetrics } from "@/features/dashboard/hooks/useDashboardMetrics";
import { DashboardSkeleton } from "@/features/dashboard/components/DashboardSkeleton";
import { QueryBoundary } from "@/shared/components/feedback/QueryBoundary";
import type { DashboardData, SLABreach } from "@/features/dashboard/types";

// ─── Constants ────────────────────────────────────────────────────────────────

const SEVERITY_COLORS: Record<string, string> = {
  Critical: "#EF4444",
  High:     "#F97316",
  Medium:   "#EAB308",
  Low:      "#3B82F6",
};

const PERIODS = ["30d", "90d", "1y"] as const;
type Period = typeof PERIODS[number];

// Risk score thresholds
const riskLabel = (score: number) =>
  score >= 80 ? { label: "Critical", color: "var(--color-status-error, #EF4444)" }
  : score >= 60 ? { label: "High", color: "var(--color-severity-high, #F97316)" }
  : score >= 40 ? { label: "Medium", color: "var(--color-severity-medium, #EAB308)" }
  : { label: "Low", color: "var(--color-status-success, #10B981)" };

// ─── Sub-components ──────────────────────────────────────────────────────────

function RiskScoreGauge({ score }: { score: number }) {
  const { label, color } = riskLabel(score);
  const circumference = 2 * Math.PI * 54;
  const strokeDasharray = `${(score / 100) * circumference} ${circumference}`;

  return (
    <div className="flex flex-col items-center">
      <div className="relative w-36 h-36">
        <svg viewBox="0 0 120 120" className="w-full h-full -rotate-90">
          {/* Background track */}
          <circle cx="60" cy="60" r="54" fill="none"
            stroke="rgba(255,255,255,0.06)" strokeWidth="10" />
          {/* Score arc */}
          <circle cx="60" cy="60" r="54" fill="none"
            stroke={color} strokeWidth="10"
            strokeDasharray={strokeDasharray}
            strokeLinecap="round"
            style={{ transition: "stroke-dasharray 1s ease" }} />
        </svg>
        <div className="absolute inset-0 flex flex-col items-center justify-center">
          <div style={{ color, fontSize: 30, fontWeight: 800, lineHeight: 1 }}>{score}</div>
          <div style={{ color: "var(--color-text-secondary, #9CA3AF)", fontSize: 11, marginTop: 2 }}>/ 100</div>
        </div>
      </div>
      <div className="mt-2 px-3 py-1 rounded-full text-xs font-semibold"
        style={{ background: `${color}20`, color }}>
        {label} Risk
      </div>
    </div>
  );
}

function RiskCard({
  label, value, sub, delta, deltaUp, color, icon: Icon,
}: {
  label: string; value: string | number; sub?: string;
  delta?: string; deltaUp?: boolean; color: string; icon: React.ElementType;
}) {
  return (
    <div className="rounded-2xl p-4 relative overflow-hidden"
      style={{ background: "var(--color-bg-card, #151B2F)", border: "1px solid rgba(255,255,255,0.07)" }}>
      <div className="absolute top-0 right-0 w-20 h-20 rounded-full -mr-6 -mt-6 opacity-10"
        style={{ background: color }} />
      <div className="flex items-start justify-between mb-3">
        <div className="w-9 h-9 rounded-xl flex items-center justify-center"
          style={{ background: `${color}20` }}>
          <Icon size={16} color={color} />
        </div>
        {delta && (
          <div className="flex items-center gap-1 px-2 py-0.5 rounded-lg text-xs"
            style={{
              background: deltaUp ? "rgba(239,68,68,0.1)" : "rgba(16,185,129,0.1)",
              color: deltaUp ? "#EF4444" : "#10B981",
            }}>
            {deltaUp ? <TrendingUp size={10} /> : <TrendingDown size={10} />}
            {delta}
          </div>
        )}
      </div>
      <div style={{ color: "var(--color-text-primary, #E5E7EB)", fontSize: 26, fontWeight: 700, lineHeight: 1 }}>{value}</div>
      <div style={{ color: "var(--color-text-muted, #6B7280)", fontSize: 12, marginTop: 5 }}>{label}</div>
      {sub && <div style={{ color, fontSize: 11, marginTop: 3 }}>{sub}</div>}
    </div>
  );
}

const CustomTooltip = ({ active, payload, label }: any) => {
  if (active && payload?.length) {
    return (
      <div className="rounded-xl p-3"
        style={{ background: "#1E2A45", border: "1px solid rgba(255,255,255,0.1)", fontSize: 12 }}>
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

// ─── Risk Overview Content ────────────────────────────────────────────────────

function RiskOverviewContent({
  data, period, onPeriodChange,
}: {
  data: DashboardData;
  period: Period;
  onPeriodChange: (p: Period) => void;
}) {
  const { kpis, risk_trend, severity_distribution, product_grades, kev_alerts, sla_breaches } = data;

  // Compute an overall risk score from KPIs
  const riskScore = Math.min(
    100,
    Math.round(
      (kpis.critical_findings * 4 + kpis.high_findings * 2) /
      Math.max(kpis.total_assets, 1) +
      (100 - kpis.sla_compliance) * 0.5
    )
  );

  // Radar data — risk posture across dimensions
  const radarData = [
    { subject: "Vuln Density",  score: Math.min(100, Math.round((kpis.critical_findings + kpis.high_findings) / Math.max(kpis.total_assets, 1) * 100)) },
    { subject: "SLA Risk",      score: Math.round(100 - kpis.sla_compliance) },
    { subject: "KEV Exposure",  score: Math.min(100, kev_alerts.length * 8) },
    { subject: "Asset Risk",    score: Math.min(100, Math.round(kpis.high_risk_assets / Math.max(kpis.total_assets, 1) * 100)) },
    { subject: "Scan Coverage", score: Math.max(0, 100 - kpis.active_scans * 10) },
    { subject: "Breach Rate",   score: Math.min(100, kpis.sla_breached * 15) },
  ];

  // Severity bar chart
  const severityBars = [
    { name: "Critical", value: severity_distribution.critical, color: "var(--color-status-error, #EF4444)" },
    { name: "High",     value: severity_distribution.high,     color: "var(--color-severity-high, #F97316)" },
    { name: "Medium",   value: severity_distribution.medium,   color: "var(--color-severity-medium, #EAB308)" },
    { name: "Low",      value: severity_distribution.low,      color: "var(--color-severity-low, #3B82F6)" },
  ];

  // Top risky products
  const riskyProducts = [...product_grades].sort((a, b) => a.score - b.score).slice(0, 5);

  // Critical SLA breaches
  const criticalBreaches: SLABreach[] = sla_breaches
    .filter((b) => b.days_overdue > 0)
    .sort((a, b) => b.days_overdue - a.days_overdue)
    .slice(0, 5);

  const handleExportPDF = () => {
    window.print();
  };

  return (
    <div className="flex-1 overflow-y-auto" style={{ background: "var(--color-bg-page, #0B1020)", padding: "24px" }}>

      {/* ── Header ── */}
      <div className="flex items-center justify-between mb-6">
        <div>
          <h1 style={{ color: "var(--color-text-primary, #E5E7EB)", fontWeight: 700, fontSize: 20 }}>Risk Overview</h1>
          <p style={{ color: "var(--color-text-muted, #6B7280)", fontSize: 13, marginTop: 2 }}>
            Consolidated risk posture · Real-time
          </p>
        </div>
        <div className="flex items-center gap-3">
          {/* Period picker */}
          <div className="flex gap-1 rounded-xl p-1"
            style={{ background: "rgba(255,255,255,0.04)", border: "1px solid rgba(255,255,255,0.08)" }}>
            {PERIODS.map((p) => (
              <button key={p} onClick={() => onPeriodChange(p)}
                className="px-3 py-1.5 rounded-lg text-xs font-medium transition-all"
                style={{
                  background: period === p ? "rgba(79,140,255,0.2)" : "transparent",
                  color: period === p ? "#4F8CFF" : "#6B7280",
                  border: period === p ? "1px solid rgba(79,140,255,0.3)" : "1px solid transparent",
                  cursor: "pointer",
                }}>
                {p}
              </button>
            ))}
          </div>
          <button
            onClick={handleExportPDF}
            className="flex items-center gap-2 px-4 py-2 rounded-xl"
            style={{ background: "linear-gradient(135deg,#EF4444,#C53030)", color: "white", fontSize: 13, cursor: "pointer", border: "none" }}>
            <ExternalLink size={14} />
            Export Risk Report
          </button>
        </div>
      </div>

      {/* ── KPI Row ── */}
      <div className="grid grid-cols-5 gap-4 mb-6">
        <RiskCard label="Critical Findings" value={kpis.critical_findings}
          sub={`${kpis.sla_breached} SLA breached`} delta="+5%" deltaUp
          color="#EF4444" icon={AlertTriangle} />
        <RiskCard label="High Risk Assets" value={kpis.high_risk_assets}
          sub={`of ${kpis.total_assets} total`} delta="+2%" deltaUp
          color="#F97316" icon={ShieldAlert} />
        <RiskCard label="KEV Exposures" value={kev_alerts.length}
          sub={`${kev_alerts.filter(k => k.is_ransomware).length} ransomware`}
          color="#A78BFA" icon={Flame} />
        <RiskCard label="SLA Breaches" value={kpis.sla_breached}
          sub={`${kpis.sla_at_risk} at risk`} delta="-1%" deltaUp={false}
          color="#EAB308" icon={Clock} />
        <RiskCard label="SLA Compliance" value={`${kpis.sla_compliance}%`}
          sub="Target: 95%" delta="-1.2%" deltaUp
          color="#10B981" icon={Target} />
      </div>

      {/* ── Main content grid ── */}
      <div className="grid grid-cols-3 gap-4 mb-6">

        {/* Risk Posture Radar */}
        <div className="rounded-2xl p-5 flex flex-col items-center"
          style={{ background: "var(--color-bg-card, #151B2F)", border: "1px solid rgba(255,255,255,0.07)" }}>
          <div className="w-full mb-4">
            <h3 style={{ color: "var(--color-text-primary, #E5E7EB)", fontSize: 14, fontWeight: 600 }}>Risk Posture Radar</h3>
            <p style={{ color: "var(--color-text-muted, #6B7280)", fontSize: 12 }}>Multi-dimensional risk assessment</p>
          </div>
          <RiskScoreGauge score={riskScore} />
          <div className="w-full mt-4">
            <ResponsiveContainer width="100%" height={200}>
              <RadarChart data={radarData} outerRadius={70}>
                <PolarGrid stroke="rgba(255,255,255,0.06)" />
                <PolarAngleAxis dataKey="subject"
                  tick={{ fill: "#6B7280", fontSize: 10 }} />
                <Radar name="Risk Score" dataKey="score"
                  stroke="#EF4444" fill="#EF4444" fillOpacity={0.15}
                  strokeWidth={2} />
                <Tooltip
                  contentStyle={{ background: "#1E2A45", border: "1px solid rgba(255,255,255,0.1)", borderRadius: 8, fontSize: 12 }}
                  formatter={(v: any) => [`${v}/100`, "Risk"]} />
              </RadarChart>
            </ResponsiveContainer>
          </div>
        </div>

        {/* Risk Trend Line Chart */}
        <div className="col-span-2 rounded-2xl p-5"
          style={{ background: "var(--color-bg-card, #151B2F)", border: "1px solid rgba(255,255,255,0.07)" }}>
          <div className="flex items-center justify-between mb-5">
            <div>
              <h3 style={{ color: "var(--color-text-primary, #E5E7EB)", fontSize: 14, fontWeight: 600 }}>Risk Trend Over Time</h3>
              <p style={{ color: "var(--color-text-muted, #6B7280)", fontSize: 12 }}>Finding volume by severity</p>
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
          <ResponsiveContainer width="100%" height={280}>
            <LineChart data={risk_trend}>
              <CartesianGrid stroke="rgba(255,255,255,0.04)" />
              <XAxis dataKey="month" tick={{ fill: "#6B7280", fontSize: 11 }} axisLine={false} tickLine={false} />
              <YAxis tick={{ fill: "#6B7280", fontSize: 11 }} axisLine={false} tickLine={false} />
              <Tooltip content={<CustomTooltip />} />
              <Line type="monotone" dataKey="critical" stroke="#EF4444" strokeWidth={2.5}
                dot={{ r: 3, fill: "#EF4444" }} name="Critical" />
              <Line type="monotone" dataKey="high" stroke="#F97316" strokeWidth={2}
                dot={{ r: 3, fill: "#F97316" }} name="High" />
              <Line type="monotone" dataKey="medium" stroke="#EAB308" strokeWidth={2}
                dot={{ r: 3, fill: "#EAB308" }} name="Medium" strokeDasharray="4 2" />
            </LineChart>
          </ResponsiveContainer>
        </div>
      </div>

      {/* ── Second row ── */}
      <div className="grid grid-cols-3 gap-4 mb-6">

        {/* Severity Distribution Bar */}
        <div className="rounded-2xl p-5"
          style={{ background: "var(--color-bg-card, #151B2F)", border: "1px solid rgba(255,255,255,0.07)" }}>
          <div className="mb-4">
            <h3 style={{ color: "var(--color-text-primary, #E5E7EB)", fontSize: 14, fontWeight: 600 }}>Findings by Severity</h3>
            <p style={{ color: "var(--color-text-muted, #6B7280)", fontSize: 12 }}>
              Total: {severity_distribution.total.toLocaleString()}
            </p>
          </div>
          <ResponsiveContainer width="100%" height={160}>
            <BarChart data={severityBars} layout="vertical" barCategoryGap={8}>
              <XAxis type="number" tick={{ fill: "#6B7280", fontSize: 10 }} axisLine={false} tickLine={false} />
              <YAxis type="category" dataKey="name" tick={{ fill: "#9CA3AF", fontSize: 12 }} axisLine={false} tickLine={false} width={60} />
              <Tooltip
                contentStyle={{ background: "#1E2A45", border: "1px solid rgba(255,255,255,0.1)", borderRadius: 8, fontSize: 12 }}
                cursor={{ fill: "rgba(255,255,255,0.03)" }} />
              <Bar dataKey="value" radius={[0, 6, 6, 0]} name="Findings">
                {severityBars.map((entry, i) => <Cell key={i} fill={entry.color} />)}
              </Bar>
            </BarChart>
          </ResponsiveContainer>
          {/* Quick stat pills */}
          <div className="grid grid-cols-2 gap-2 mt-4">
            {severityBars.map((d) => (
              <div key={d.name} className="flex items-center gap-2 px-3 py-2 rounded-xl"
                style={{ background: `${d.color}10`, border: `1px solid ${d.color}20` }}>
                <div className="w-2 h-2 rounded-full" style={{ background: d.color }} />
                <span style={{ color: d.color, fontSize: 13, fontWeight: 700 }}>{d.value}</span>
                <span style={{ color: "var(--color-text-muted, #6B7280)", fontSize: 11 }}>{d.name}</span>
              </div>
            ))}
          </div>
        </div>

        {/* Top Risky Products */}
        <div className="rounded-2xl p-5"
          style={{ background: "var(--color-bg-card, #151B2F)", border: "1px solid rgba(255,255,255,0.07)" }}>
          <div className="flex items-center justify-between mb-4">
            <div>
              <h3 style={{ color: "var(--color-text-primary, #E5E7EB)", fontSize: 14, fontWeight: 600 }}>Highest Risk Products</h3>
              <p style={{ color: "var(--color-text-muted, #6B7280)", fontSize: 12 }}>Lowest security score first</p>
            </div>
            <button style={{ color: "var(--color-primary, #4F8CFF)", fontSize: 12, background: "none", border: "none", cursor: "pointer" }}>
              View all
            </button>
          </div>
          <div className="flex flex-col gap-3">
            {riskyProducts.map((p, i) => {
              const { color } = riskLabel(100 - p.score);
              return (
                <div key={p.id} className="flex items-center gap-3">
                  <div className="w-6 h-6 rounded-lg flex items-center justify-center flex-shrink-0"
                    style={{ background: `${color}20`, color, fontSize: 11, fontWeight: 700 }}>
                    {i + 1}
                  </div>
                  <div className="flex-1 min-w-0">
                    <div style={{ color: "var(--color-text-primary, #E5E7EB)", fontSize: 13, fontWeight: 500 }}
                      className="truncate">{p.name}</div>
                    <div className="flex gap-2 mt-0.5">
                      <span style={{ color: "var(--color-status-error, #EF4444)", fontSize: 11 }}>{p.critical_count} crit</span>
                      <span style={{ color: "var(--color-severity-high, #F97316)", fontSize: 11 }}>{p.high_count} high</span>
                    </div>
                  </div>
                  {/* Score bar */}
                  <div className="flex flex-col items-end gap-1">
                    <div style={{ color, fontSize: 12, fontWeight: 700 }}>{p.score}</div>
                    <div className="w-16 h-1.5 rounded-full overflow-hidden"
                      style={{ background: "rgba(255,255,255,0.06)" }}>
                      <div className="h-full rounded-full"
                        style={{ width: `${p.score}%`, background: color }} />
                    </div>
                  </div>
                </div>
              );
            })}
          </div>
        </div>

        {/* Critical SLA Breaches */}
        <div className="rounded-2xl p-5"
          style={{ background: "var(--color-bg-card, #151B2F)", border: "1px solid rgba(255,255,255,0.07)" }}>
          <div className="flex items-center justify-between mb-4">
            <div className="flex items-center gap-2">
              <div className="w-2 h-2 rounded-full animate-pulse" style={{ background: "#EF4444" }} />
              <h3 style={{ color: "var(--color-text-primary, #E5E7EB)", fontSize: 14, fontWeight: 600 }}>Critical SLA Overdue</h3>
            </div>
            <span className="px-2 py-1 rounded-lg text-xs"
              style={{ background: "rgba(239,68,68,0.1)", color: "var(--color-status-error, #EF4444)" }}>
              {criticalBreaches.length} items
            </span>
          </div>
          {criticalBreaches.length === 0 ? (
            <div className="flex flex-col items-center justify-center h-32 gap-2">
              <CheckCircle size={28} color="#10B981" />
              <p style={{ color: "var(--color-text-muted, #6B7280)", fontSize: 13 }}>No overdue breaches</p>
            </div>
          ) : (
            <div className="flex flex-col gap-3">
              {criticalBreaches.map((b) => (
                <div key={b.finding_id}
                  className="flex items-start gap-3 p-3 rounded-xl"
                  style={{ background: "rgba(239,68,68,0.05)", border: "1px solid rgba(239,68,68,0.1)" }}>
                  <AlertCircle size={14} color="#EF4444" style={{ flexShrink: 0, marginTop: 2 }} />
                  <div className="flex-1 min-w-0">
                    <div style={{ color: "var(--color-text-primary, #E5E7EB)", fontSize: 12 }} className="truncate">{b.title}</div>
                    <div style={{ color: "var(--color-text-muted, #6B7280)", fontSize: 10 }}>{b.product_name}</div>
                  </div>
                  <div className="flex flex-col items-end gap-1 flex-shrink-0">
                    <span style={{ color: "var(--color-status-error, #EF4444)", fontSize: 11, fontWeight: 700 }}>
                      +{b.days_overdue}d
                    </span>
                    <span className="px-1.5 py-0.5 rounded text-xs"
                      style={{
                        background: (SEVERITY_COLORS[b.severity] ?? "#6B7280") + "20",
                        color: SEVERITY_COLORS[b.severity] ?? "#6B7280",
                      }}>
                      {b.severity}
                    </span>
                  </div>
                </div>
              ))}
            </div>
          )}
        </div>
      </div>

      {/* ── KEV Threat Intelligence ── */}
      <div className="rounded-2xl p-5"
        style={{ background: "var(--color-bg-card, #151B2F)", border: "1px solid rgba(255,255,255,0.07)" }}>
        <div className="flex items-center justify-between mb-4">
          <div className="flex items-center gap-2">
            <Flame size={16} color="#EF4444" />
            <h3 style={{ color: "var(--color-text-primary, #E5E7EB)", fontSize: 14, fontWeight: 600 }}>
              Known Exploited Vulnerabilities (KEV) Affecting Your Environment
            </h3>
          </div>
          <div className="flex items-center gap-3">
            <span className="px-2 py-1 rounded-lg text-xs"
              style={{ background: "rgba(239,68,68,0.1)", color: "var(--color-status-error, #EF4444)" }}>
              {kev_alerts.length} active
            </span>
            <span className="px-2 py-1 rounded-lg text-xs"
              style={{ background: "rgba(245,158,11,0.1)", color: "var(--color-status-warning, #F59E0B)" }}>
              {kev_alerts.filter(k => k.is_ransomware).length} ransomware
            </span>
          </div>
        </div>
        <div className="grid grid-cols-4 gap-3">
          {kev_alerts.map((k) => (
            <div key={k.cve_id}
              className="flex flex-col gap-2 p-3 rounded-xl"
              style={{
                background: k.is_ransomware ? "rgba(239,68,68,0.07)" : "rgba(255,255,255,0.03)",
                border: k.is_ransomware ? "1px solid rgba(239,68,68,0.15)" : "1px solid rgba(255,255,255,0.06)",
              }}>
              <div className="flex items-center justify-between">
                <span style={{ color: "var(--color-status-error, #EF4444)", fontSize: 12, fontWeight: 700 }}>{k.cve_id}</span>
                {k.is_ransomware && (
                  <span className="px-1.5 py-0.5 rounded text-xs"
                    style={{ background: "rgba(239,68,68,0.15)", color: "var(--color-status-error, #EF4444)" }}>
                    Ransomware
                  </span>
                )}
              </div>
              <div style={{ color: "var(--color-text-secondary, #9CA3AF)", fontSize: 11 }}>{k.vendor} · {k.product}</div>
              <div style={{ color: "var(--color-text-faint, #4B5563)", fontSize: 10 }}>
                Added: {new Date(k.date_added).toLocaleDateString()}
              </div>
            </div>
          ))}
        </div>
      </div>
    </div>
  );
}

// ─── Main Export ──────────────────────────────────────────────────────────────

export function RiskOverview() {
  const [period, setPeriod] = useState<Period>("30d");
  const metricsQuery = useDashboardMetrics(period);

  return (
    <QueryBoundary query={metricsQuery} skeleton={<DashboardSkeleton />}>
      {(data) => (
        <RiskOverviewContent data={data} period={period} onPeriodChange={setPeriod} />
      )}
    </QueryBoundary>
  );
}
