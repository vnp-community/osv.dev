import { useState } from "react";
import { RotateCcw, Plus } from "lucide-react";
import { LineChart, Line, XAxis, YAxis, Tooltip, ResponsiveContainer } from "recharts";
import { useQuery } from "@tanstack/react-query";
import { apiClient } from "@/shared/api/client";
import { ENDPOINTS } from "@/shared/api/endpoints";
import { QueryBoundary } from "@/shared/components/feedback/QueryBoundary";

// ── Types ────────────────────────────────────────────────────────────────────

interface WebhookItem {
  id: string; name: string; url: string;
  events: string[]; status: string; successRate: number;
}
interface WebhooksResponse {
  webhooks: WebhookItem[];
  total: number;
}

function useWebhooks() {
  return useQuery<WebhooksResponse>({
    queryKey: ['webhooks'],
    queryFn: async () => {
      const { data } = await apiClient.get<WebhooksResponse>(ENDPOINTS.webhooks.list);
      return data;
    },
    staleTime: 5 * 60_000,
  });
}

// ── UI helpers ────────────────────────────────────────────────────────────────

const STATUS_S: Record<string, { bg: string; color: string }> = {
  success: { bg: "rgba(16,185,129,0.1)", color: "#10B981" },
  failed: { bg: "rgba(239,68,68,0.1)", color: "#EF4444" },
  retried: { bg: "rgba(245,158,11,0.1)", color: "#F59E0B" },
};

// Simulated delivery history (read from fixtures via API in production)
const DELIVERY_HISTORY = [
  { id: "DEL-0441", event: "finding.created", endpoint: "siem.company.com", status: "success", responseTime: 124, time: "2 min ago", statusCode: 200 },
  { id: "DEL-0440", event: "scan.completed", endpoint: "siem.company.com", status: "success", responseTime: 89, time: "2h ago", statusCode: 200 },
  { id: "DEL-0439", event: "sla.breached", endpoint: "jira.company.com", status: "failed", responseTime: 5001, time: "3h ago", statusCode: 503 },
  { id: "DEL-0438", event: "kev.alert", endpoint: "slack.company.com", status: "success", responseTime: 201, time: "4h ago", statusCode: 200 },
];

const ACTIVITY_CHART = [
  { h: "06:00", success: 42, failed: 1 }, { h: "09:00", success: 87, failed: 2 },
  { h: "12:00", success: 65, failed: 0 }, { h: "15:00", success: 93, failed: 3 }, { h: "18:00", success: 78, failed: 1 },
];

function WebhookSkeleton() {
  return (
    <div className="flex-1 overflow-y-auto px-6 py-5 animate-pulse" style={{ background: "#0B1020" }}>
      <div className="grid grid-cols-3 gap-4 mb-5">
        {Array.from({ length: 3 }).map((_, i) => <div key={i} className="rounded-xl h-16" style={{ background: "#151B2F" }} />)}
      </div>
      <div className="grid grid-cols-3 gap-4 mb-5">
        {Array.from({ length: 3 }).map((_, i) => <div key={i} className="rounded-2xl h-28" style={{ background: "#151B2F" }} />)}
      </div>
    </div>
  );
}

// ── Main component ────────────────────────────────────────────────────────────

export function WebhookEvents() {
  const webhooksQuery = useWebhooks();
  const [selectedId, setSelectedId] = useState<string | null>(null);

  return (
    <QueryBoundary query={webhooksQuery} skeleton={<WebhookSkeleton />}>
      {({ webhooks }) => {
        const selected = webhooks.find(w => w.id === selectedId) ?? webhooks[0];
        const avgSuccessRate = webhooks.length > 0
          ? (webhooks.reduce((s, w) => s + w.successRate, 0) / webhooks.length).toFixed(1)
          : "0";

        return (
          <div className="flex-1 overflow-y-auto px-6 py-5" style={{ background: "#0B1020" }}>
            <div className="flex items-center justify-between mb-5">
              <div>
                <h2 style={{ color: "#E5E7EB", fontSize: 18, fontWeight: 700 }}>Webhook Management</h2>
                <p style={{ color: "#6B7280", fontSize: 12 }}>{webhooks.filter(w => w.status === "active").length} active webhooks · {avgSuccessRate}% success rate</p>
              </div>
              <button className="flex items-center gap-2 px-4 py-2 rounded-xl" style={{ background: "linear-gradient(135deg,#4F8CFF,#3B6FCC)", color: "white", border: "none", fontSize: 13, cursor: "pointer" }}>
                <Plus size={14} />Add Webhook
              </button>
            </div>

            {/* Stats */}
            <div className="grid grid-cols-3 gap-4 mb-5">
              {[
                { label: "Success Rate", value: `${avgSuccessRate}%`, color: "#10B981", sub: "All webhooks" },
                { label: "Active Webhooks", value: webhooks.filter(w => w.status === "active").length, color: "#4F8CFF", sub: "Listening" },
                { label: "Total Webhooks", value: webhooks.length, color: "#9CA3AF", sub: "Configured" },
              ].map(s => (
                <div key={s.label} className="rounded-xl px-4 py-3 flex items-center gap-3" style={{ background: "#151B2F", border: "1px solid rgba(255,255,255,0.07)" }}>
                  <div style={{ color: s.color, fontSize: 22, fontWeight: 700 }}>{s.value}</div>
                  <div><div style={{ color: "#E5E7EB", fontSize: 12 }}>{s.label}</div><div style={{ color: "#6B7280", fontSize: 11 }}>{s.sub}</div></div>
                </div>
              ))}
            </div>

            {/* Chart */}
            <div className="rounded-2xl p-5 mb-5" style={{ background: "#151B2F", border: "1px solid rgba(255,255,255,0.07)" }}>
              <h3 style={{ color: "#E5E7EB", fontSize: 13, fontWeight: 600, marginBottom: 12 }}>Delivery Activity (Today)</h3>
              <ResponsiveContainer width="100%" height={120}>
                <LineChart data={ACTIVITY_CHART}>
                  <XAxis dataKey="h" tick={{ fill: "#6B7280", fontSize: 10 }} axisLine={false} tickLine={false} />
                  <YAxis tick={{ fill: "#6B7280", fontSize: 10 }} axisLine={false} tickLine={false} />
                  <Tooltip contentStyle={{ background: "#1E2A45", border: "1px solid rgba(255,255,255,0.1)", borderRadius: 8, fontSize: 11 }} />
                  <Line type="monotone" dataKey="success" stroke="#10B981" strokeWidth={2} dot={false} name="Success" />
                  <Line type="monotone" dataKey="failed" stroke="#EF4444" strokeWidth={2} dot={false} strokeDasharray="4 2" name="Failed" />
                </LineChart>
              </ResponsiveContainer>
            </div>

            {/* Webhooks list */}
            <div className="grid grid-cols-3 gap-4 mb-5">
              {webhooks.map(w => (
                <div key={w.id} onClick={() => setSelectedId(w.id)}
                  className="rounded-2xl p-4 cursor-pointer"
                  style={{ background: selected?.id === w.id ? "rgba(79,140,255,0.08)" : "#151B2F", border: selected?.id === w.id ? "1px solid rgba(79,140,255,0.3)" : "1px solid rgba(255,255,255,0.07)" }}
                >
                  <div className="flex items-center gap-2 mb-2">
                    <div className="w-2 h-2 rounded-full" style={{ background: w.status === "active" ? "#10B981" : "#6B7280" }} />
                    <span style={{ color: "#E5E7EB", fontSize: 12, fontWeight: 500 }}>{w.name}</span>
                    <span className="ml-auto" style={{ color: "#10B981", fontSize: 11 }}>{w.successRate}%</span>
                  </div>
                  <div style={{ color: "#4F8CFF", fontSize: 11, fontFamily: "monospace" }} className="truncate">{w.url}</div>
                  <div style={{ color: "#6B7280", fontSize: 11, marginTop: 4 }}>{w.events.length} events</div>
                </div>
              ))}
            </div>

            {/* Delivery history */}
            <div className="rounded-2xl" style={{ background: "#151B2F", border: "1px solid rgba(255,255,255,0.07)" }}>
              <div className="px-5 py-4" style={{ borderBottom: "1px solid rgba(255,255,255,0.06)" }}>
                <h3 style={{ color: "#E5E7EB", fontSize: 14, fontWeight: 600 }}>Recent Deliveries</h3>
              </div>
              <table className="w-full">
                <thead><tr style={{ borderBottom: "1px solid rgba(255,255,255,0.05)" }}>
                  {["Delivery ID", "Event", "Endpoint", "Status Code", "Response", "Status", "Time", ""].map(h => (
                    <th key={h} className="px-4 py-3 text-left" style={{ color: "#6B7280", fontSize: 11, fontWeight: 600 }}>{h}</th>
                  ))}
                </tr></thead>
                <tbody>
                  {DELIVERY_HISTORY.map((d, i) => (
                    <tr key={d.id} style={{ borderBottom: i < DELIVERY_HISTORY.length - 1 ? "1px solid rgba(255,255,255,0.04)" : "none" }}
                      onMouseEnter={e => (e.currentTarget.style.background = "rgba(255,255,255,0.02)")}
                      onMouseLeave={e => (e.currentTarget.style.background = "transparent")}
                    >
                      <td className="px-4 py-3"><span style={{ color: "#6B7280", fontSize: 12 }}>{d.id}</span></td>
                      <td className="px-4 py-3"><span style={{ color: "#E5E7EB", fontSize: 12 }}>{d.event}</span></td>
                      <td className="px-4 py-3"><span style={{ color: "#4F8CFF", fontSize: 11 }}>{d.endpoint}</span></td>
                      <td className="px-4 py-3"><span style={{ color: d.statusCode === 200 ? "#10B981" : "#EF4444", fontSize: 12, fontWeight: 600 }}>{d.statusCode}</span></td>
                      <td className="px-4 py-3"><span style={{ color: d.responseTime > 1000 ? "#EF4444" : "#9CA3AF", fontSize: 12 }}>{d.responseTime}ms</span></td>
                      <td className="px-4 py-3"><span className="px-2 py-0.5 rounded" style={{ ...STATUS_S[d.status], fontSize: 11 }}>{d.status}</span></td>
                      <td className="px-4 py-3"><span style={{ color: "#4B5563", fontSize: 11 }}>{d.time}</span></td>
                      <td className="px-4 py-3">{d.status === "failed" && <button className="w-7 h-7 rounded-lg flex items-center justify-center" style={{ background: "rgba(245,158,11,0.1)", color: "#F59E0B", border: "none", cursor: "pointer" }}><RotateCcw size={11} /></button>}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          </div>
        );
      }}
    </QueryBoundary>
  );
}
