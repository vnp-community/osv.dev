import { Brain, TrendingUp, Target, Zap, AlertTriangle } from "lucide-react";
import { useQuery } from "@tanstack/react-query";
import { apiClient } from "@/shared/api/client";
import { ENDPOINTS } from "@/shared/api/endpoints";
import { QueryBoundary } from "@/shared/components/feedback/QueryBoundary";
import {
  AreaChart, Area, XAxis, YAxis, Tooltip, ResponsiveContainer,
  BarChart, Bar,
} from "recharts";

// ── Types ────────────────────────────────────────────────────────────────────

interface AIInsightsData {
  triageAccuracy: number;
  falsePositiveRate: number;
  autoEnrichedCount: number;
  timeSavedHours: number;
  weeklyTrend: Array<{ day: string; triaged: number; confirmed: number; falsePositive: number }>;
  topCweFindings: Array<{ cwe: string; count: number; label: string }>;
  recommendations: Array<{ id: string; title: string; severity: "critical" | "high" | "medium"; description: string }>;
}

// ── Hook ─────────────────────────────────────────────────────────────────────

function useAIInsights() {
  return useQuery<AIInsightsData>({
    queryKey: ["ai", "insights"],
    queryFn: async () => {
      const { data } = await apiClient.get<AIInsightsData>(ENDPOINTS.ai.insights);
      return data;
    },
    staleTime: 5 * 60_000,
  });
}

// ── Sub-components ────────────────────────────────────────────────────────────

const SEVERITY_COLORS = {
  critical: "#EF4444",
  high: "#F97316",
  medium: "#EAB308",
} as const;

function KPICard({
  icon: Icon,
  label,
  value,
  color,
  suffix = "",
}: {
  icon: React.ElementType;
  label: string;
  value: number | string;
  color: string;
  suffix?: string;
}) {
  return (
    <div
      className="rounded-2xl p-5"
      style={{ background: "var(--color-bg-card, #151B2F)", border: "1px solid rgba(255,255,255,0.07)" }}
    >
      <div className="flex items-center gap-2 mb-3">
        <div className="w-8 h-8 rounded-lg flex items-center justify-center" style={{ background: `${color}18` }}>
          <Icon size={16} color={color} />
        </div>
        <span style={{ color: "var(--color-text-muted, #6B7280)", fontSize: 12 }}>{label}</span>
      </div>
      <div style={{ color, fontSize: 28, fontWeight: 700 }}>
        {value}
        {suffix && <span style={{ fontSize: 16, fontWeight: 400, color: "var(--color-text-muted, #6B7280)" }}>{suffix}</span>}
      </div>
    </div>
  );
}

// ── Main Component ────────────────────────────────────────────────────────────

export function AIInsights() {
  const query = useAIInsights();

  return (
    <div className="flex-1 overflow-y-auto px-6 py-5" style={{ background: "var(--color-bg-page, #0B1020)" }}>
      {/* Header */}
      <div className="flex items-center gap-3 mb-6">
        <div
          className="w-10 h-10 rounded-xl flex items-center justify-center"
          style={{ background: "rgba(167,139,250,0.15)" }}
        >
          <Brain size={20} color="#A78BFA" />
        </div>
        <div>
          <h2 style={{ color: "var(--color-text-primary, #E5E7EB)", fontSize: 18, fontWeight: 700 }}>AI Insights</h2>
          <p style={{ color: "var(--color-text-muted, #6B7280)", fontSize: 12, marginTop: 2 }}>
            AI-driven analytics and actionable recommendations
          </p>
        </div>
      </div>

      <QueryBoundary query={query}>
        {(data) => (
          <div className="flex flex-col gap-6">
            {/* KPI Cards */}
            <div className="grid grid-cols-4 gap-4">
              <KPICard icon={Target}     label="Triage Accuracy"      value={data.triageAccuracy}     color="#A78BFA" suffix="%" />
              <KPICard icon={AlertTriangle} label="False Positive Rate" value={data.falsePositiveRate} color="#F59E0B" suffix="%" />
              <KPICard icon={Zap}        label="Auto-Enriched"         value={data.autoEnrichedCount}  color="#4F8CFF" />
              <KPICard icon={TrendingUp} label="Time Saved"            value={data.timeSavedHours}     color="#10B981" suffix="h" />
            </div>

            {/* Charts */}
            <div className="grid grid-cols-2 gap-4">
              {/* Weekly triage trend */}
              <div
                className="rounded-2xl p-5"
                style={{ background: "var(--color-bg-card, #151B2F)", border: "1px solid rgba(255,255,255,0.07)" }}
              >
                <div style={{ color: "var(--color-text-primary, #E5E7EB)", fontSize: 13, fontWeight: 600, marginBottom: 16 }}>
                  Weekly AI Triage Activity
                </div>
                <ResponsiveContainer width="100%" height={180}>
                  <AreaChart data={data.weeklyTrend}>
                    <defs>
                      <linearGradient id="triageGrad" x1="0" y1="0" x2="0" y2="1">
                        <stop offset="0%" stopColor="#A78BFA" stopOpacity={0.3} />
                        <stop offset="100%" stopColor="#A78BFA" stopOpacity={0} />
                      </linearGradient>
                    </defs>
                    <XAxis dataKey="day" tick={{ fill: "#6B7280", fontSize: 10 }} axisLine={false} tickLine={false} />
                    <YAxis tick={{ fill: "#6B7280", fontSize: 10 }} axisLine={false} tickLine={false} />
                    <Tooltip
                      contentStyle={{ background: "#1E2A45", border: "1px solid rgba(255,255,255,0.1)", borderRadius: 8, fontSize: 11 }}
                    />
                    <Area type="monotone" dataKey="triaged"      stroke="#A78BFA" strokeWidth={2} fill="url(#triageGrad)"   name="Triaged" />
                    <Area type="monotone" dataKey="confirmed"    stroke="#EF4444" strokeWidth={2} fill="none"                name="Confirmed" />
                    <Area type="monotone" dataKey="falsePositive" stroke="#6B7280" strokeWidth={2} fill="none"               name="False Positive" />
                  </AreaChart>
                </ResponsiveContainer>
              </div>

              {/* Top CWEs */}
              <div
                className="rounded-2xl p-5"
                style={{ background: "var(--color-bg-card, #151B2F)", border: "1px solid rgba(255,255,255,0.07)" }}
              >
                <div style={{ color: "var(--color-text-primary, #E5E7EB)", fontSize: 13, fontWeight: 600, marginBottom: 16 }}>
                  Top CWE Findings (AI Detected)
                </div>
                <ResponsiveContainer width="100%" height={180}>
                  <BarChart data={data.topCweFindings} layout="vertical">
                    <XAxis type="number" tick={{ fill: "#6B7280", fontSize: 10 }} axisLine={false} tickLine={false} />
                    <YAxis type="category" dataKey="cwe" tick={{ fill: "#9CA3AF", fontSize: 10 }} axisLine={false} tickLine={false} width={60} />
                    <Tooltip
                      contentStyle={{ background: "#1E2A45", border: "1px solid rgba(255,255,255,0.1)", borderRadius: 8, fontSize: 11 }}
                      formatter={(value, _name, props) => [value, props.payload.label]}
                    />
                    <Bar dataKey="count" fill="#4F8CFF" radius={[0, 4, 4, 0]} name="Findings" />
                  </BarChart>
                </ResponsiveContainer>
              </div>
            </div>

            {/* AI Recommendations */}
            <div
              className="rounded-2xl overflow-hidden"
              style={{ background: "var(--color-bg-card, #151B2F)", border: "1px solid rgba(255,255,255,0.07)" }}
            >
              <div className="px-5 py-4" style={{ borderBottom: "1px solid rgba(255,255,255,0.06)" }}>
                <h3 style={{ color: "var(--color-text-primary, #E5E7EB)", fontSize: 14, fontWeight: 600 }}>
                  AI Recommendations
                </h3>
              </div>
              <div className="flex flex-col">
                {data.recommendations.map((rec, i) => {
                  const color = SEVERITY_COLORS[rec.severity] ?? "#6B7280";
                  return (
                    <div
                      key={rec.id}
                      className="flex items-start gap-4 px-5 py-4"
                      style={{ borderBottom: i < data.recommendations.length - 1 ? "1px solid rgba(255,255,255,0.04)" : "none" }}
                    >
                      <div
                        className="w-2 h-2 rounded-full mt-1.5 flex-shrink-0"
                        style={{ background: color }}
                      />
                      <div className="flex-1">
                        <div className="flex items-center gap-2 mb-1">
                          <span style={{ color: "var(--color-text-primary, #E5E7EB)", fontSize: 13, fontWeight: 500 }}>
                            {rec.title}
                          </span>
                          <span
                            className="px-2 py-0.5 rounded"
                            style={{ background: `${color}20`, color, fontSize: 10, fontWeight: 600 }}
                          >
                            {rec.severity.toUpperCase()}
                          </span>
                        </div>
                        <p style={{ color: "var(--color-text-muted, #6B7280)", fontSize: 12, lineHeight: 1.5 }}>
                          {rec.description}
                        </p>
                      </div>
                    </div>
                  );
                })}
              </div>
            </div>
          </div>
        )}
      </QueryBoundary>
    </div>
  );
}
