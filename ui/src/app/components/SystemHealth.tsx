import { Activity, Database, Server, Wifi, CheckCircle, AlertTriangle } from "lucide-react";
import { LineChart, Line, XAxis, YAxis, Tooltip, ResponsiveContainer } from "recharts";
import { useQuery } from "@tanstack/react-query";
import { apiClient } from "@/shared/api/client";
import { ENDPOINTS } from "@/shared/api/endpoints";
import { QueryBoundary } from "@/shared/components/feedback/QueryBoundary";

// ── Types ────────────────────────────────────────────────────────────────────

interface HealthData {
  status: string;
  uptime: number;
  latencyMs: number;
  latencyTrend: Array<{ h: string; p50: number; p99: number }>;
  services: Array<{ name: string; status: string; latency: number; uptime: number }>;
}

function useSystemHealth() {
  return useQuery<HealthData>({
    queryKey: ['admin', 'health'],
    queryFn: async () => {
      const { data } = await apiClient.get<HealthData>(ENDPOINTS.admin.health);
      return data;
    },
    staleTime: 30_000,
    refetchInterval: 30_000,
  });
}

// ── UI helpers ────────────────────────────────────────────────────────────────

const STATUS_STYLES: Record<string, { bg: string; color: string }> = {
  healthy: { bg: "rgba(16,185,129,0.1)", color: "#10B981" },
  degraded: { bg: "rgba(245,158,11,0.1)", color: "#F59E0B" },
  down: { bg: "rgba(239,68,68,0.1)", color: "#EF4444" },
};

const SERVICE_ICONS: Record<string, React.ElementType> = {
  API: Server, Scan: Activity, Finding: CheckCircle, AI: Activity,
  Report: Server, Notification: Wifi, Auth: CheckCircle, Database, Redis: Database,
};

function getServiceIcon(name: string): React.ElementType {
  const key = Object.keys(SERVICE_ICONS).find(k => name.includes(k)) ?? "API";
  return SERVICE_ICONS[key] ?? Server;
}

function HealthSkeleton() {
  return (
    <div className="flex-1 overflow-y-auto px-6 py-5 animate-pulse" style={{ background: "#0B1020" }}>
      <div className="grid grid-cols-4 gap-4 mb-5">
        {Array.from({ length: 4 }).map((_, i) => <div key={i} className="rounded-2xl h-24" style={{ background: "#151B2F" }} />)}
      </div>
      <div className="grid grid-cols-3 gap-3">
        {Array.from({ length: 9 }).map((_, i) => <div key={i} className="rounded-2xl h-28" style={{ background: "#151B2F" }} />)}
      </div>
    </div>
  );
}

// ── Main component ────────────────────────────────────────────────────────────

export function SystemHealth() {
  const healthQuery = useSystemHealth();

  return (
    <QueryBoundary query={healthQuery} skeleton={<HealthSkeleton />}>
      {(health) => {
        const healthy = health.services.filter(s => s.status === "healthy").length;
        const degraded = health.services.filter(s => s.status === "degraded").length;

        return (
          <div className="flex-1 overflow-y-auto px-6 py-5" style={{ background: "#0B1020" }}>
            <div className="flex items-center justify-between mb-5">
              <div>
                <h2 style={{ color: "#E5E7EB", fontSize: 18, fontWeight: 700 }}>System Health</h2>
                <p style={{ color: "#6B7280", fontSize: 12 }}>Platform observability — {healthy} healthy · {degraded} degraded</p>
              </div>
              <div className="flex items-center gap-2 px-3 py-1.5 rounded-xl" style={{ background: degraded > 0 ? "rgba(245,158,11,0.1)" : "rgba(16,185,129,0.1)", border: `1px solid ${degraded > 0 ? "rgba(245,158,11,0.2)" : "rgba(16,185,129,0.2)"}` }}>
                <div className="w-2 h-2 rounded-full animate-pulse" style={{ background: degraded > 0 ? "#F59E0B" : "#10B981" }} />
                <span style={{ color: degraded > 0 ? "#F59E0B" : "#10B981", fontSize: 12 }}>
                  {degraded > 0 ? "Partial Degradation" : "All Systems Operational"}
                </span>
              </div>
            </div>

            {/* KPIs */}
            <div className="grid grid-cols-4 gap-4 mb-5">
              {[
                { label: "API Latency (P50)", value: `${health.latencyMs}ms`, color: "#10B981", sub: "Current" },
                { label: "Services Healthy", value: `${healthy}/${health.services.length}`, color: "#4F8CFF", sub: `${degraded} degraded` },
                { label: "Platform Uptime", value: `${health.uptime}%`, color: "#10B981", sub: "30-day SLA" },
                { label: "Status", value: health.status === "healthy" ? "Operational" : "Degraded", color: degraded > 0 ? "#F59E0B" : "#10B981", sub: "Overall" },
              ].map(s => (
                <div key={s.label} className="rounded-2xl p-5" style={{ background: "#151B2F", border: "1px solid rgba(255,255,255,0.07)" }}>
                  <div style={{ color: s.color, fontSize: 22, fontWeight: 700 }}>{s.value}</div>
                  <div style={{ color: "#E5E7EB", fontSize: 12, marginTop: 4 }}>{s.label}</div>
                  <div style={{ color: "#6B7280", fontSize: 11, marginTop: 2 }}>{s.sub}</div>
                </div>
              ))}
            </div>

            {/* Latency chart */}
            <div className="rounded-2xl p-5 mb-5" style={{ background: "#151B2F", border: "1px solid rgba(255,255,255,0.07)" }}>
              <h3 style={{ color: "#E5E7EB", fontSize: 13, fontWeight: 600, marginBottom: 12 }}>API Response Latency</h3>
              <ResponsiveContainer width="100%" height={160}>
                <LineChart data={health.latencyTrend}>
                  <XAxis dataKey="h" tick={{ fill: "#6B7280", fontSize: 10 }} axisLine={false} tickLine={false} />
                  <YAxis tick={{ fill: "#6B7280", fontSize: 10 }} axisLine={false} tickLine={false} unit="ms" />
                  <Tooltip contentStyle={{ background: "#1E2A45", border: "1px solid rgba(255,255,255,0.1)", borderRadius: 8, fontSize: 11 }} />
                  <Line type="monotone" dataKey="p50" stroke="#4F8CFF" strokeWidth={2} dot={false} name="P50" />
                  <Line type="monotone" dataKey="p99" stroke="#A78BFA" strokeWidth={2} dot={false} strokeDasharray="4 2" name="P99" />
                </LineChart>
              </ResponsiveContainer>
            </div>

            {/* Services grid */}
            <div className="grid grid-cols-3 gap-3">
              {health.services.map(svc => {
                const Icon = getServiceIcon(svc.name);
                const style = STATUS_STYLES[svc.status] ?? STATUS_STYLES.healthy;
                return (
                  <div key={svc.name} className="rounded-2xl p-4" style={{ background: "#151B2F", border: "1px solid rgba(255,255,255,0.07)" }}>
                    <div className="flex items-center justify-between mb-3">
                      <div className="flex items-center gap-2">
                        <Icon size={15} color={style.color} />
                        <span style={{ color: "#E5E7EB", fontSize: 13, fontWeight: 500 }}>{svc.name}</span>
                      </div>
                      <span className="px-2 py-0.5 rounded" style={{ ...style, fontSize: 10 }}>{svc.status}</span>
                    </div>
                    <div className="grid grid-cols-2 gap-2">
                      {[
                        { label: "Latency", value: `${svc.latency}ms` },
                        { label: "Uptime", value: `${svc.uptime}%` },
                      ].map(m => (
                        <div key={m.label}>
                          <div style={{ color: "#4B5563", fontSize: 9, letterSpacing: 0.5 }}>{m.label.toUpperCase()}</div>
                          <div style={{ color: "#9CA3AF", fontSize: 11, fontWeight: 600, marginTop: 1 }}>{m.value}</div>
                        </div>
                      ))}
                    </div>
                  </div>
                );
              })}
            </div>
          </div>
        );
      }}
    </QueryBoundary>
  );
}
