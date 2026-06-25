import { useState } from "react";
import { RotateCcw, Plus, AlertTriangle, Loader2 } from "lucide-react";
import { LineChart, Line, XAxis, YAxis, Tooltip, ResponsiveContainer } from "recharts";
import { useQuery } from "@tanstack/react-query";
import { apiClient } from "@/shared/api/client";
import { ENDPOINTS } from "@/shared/api/endpoints";
import { QueryBoundary } from "@/shared/components/feedback/QueryBoundary";
import { useWebhookDeliveries, useWebhookHourlyStats, useRetryDelivery } from "../hooks/useWebhookDeliveries";

// ── Types ────────────────────────────────────────────────────────────────────

interface WebhookItem {
  id: string;
  name?: string;
  url: string;
  events: string[];
  status: string;
  successRate?: number;
  is_active?: boolean;
  last_delivery_at?: string;
  last_delivery_status?: string;
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
      // Normalize — đảm bảo webhooks luôn là array dù backend trả shape khác
      return {
        webhooks: Array.isArray(data?.webhooks) ? data.webhooks : [],
        total:    typeof data?.total === 'number' ? data.total : 0,
      };
    },
    staleTime: 5 * 60_000,
  });
}

// ── UI helpers ────────────────────────────────────────────────────────────────

const STATUS_S: Record<string, { bg: string; color: string }> = {
  success: { bg: "var(--color-status-success-bg, rgba(16,185,129,0.1))", color: "var(--color-status-success, #10B981)" },
  failed:  { bg: "var(--color-status-error-bg, rgba(239,68,68,0.1))",    color: "var(--color-status-error, #EF4444)" },
  retried: { bg: "var(--color-status-warning-bg, rgba(245,158,11,0.1))", color: "var(--color-status-warning, #F59E0B)" },
};

function formatTimeAgo(iso?: string): string {
  if (!iso) return "—";
  const diff = Date.now() - new Date(iso).getTime();
  const mins = Math.floor(diff / 60_000);
  if (mins < 60) return `${mins}m ago`;
  const hrs = Math.floor(mins / 60);
  if (hrs < 24) return `${hrs}h ago`;
  return `${Math.floor(hrs / 24)}d ago`;
}

function WebhookSkeleton() {
  return (
    <div className="flex-1 overflow-y-auto px-6 py-5 animate-pulse" style={{ background: "var(--color-bg-page, #0B1020)" }}>
      <div className="grid grid-cols-3 gap-4 mb-5">
        {Array.from({ length: 3 }).map((_, i) => (
          <div key={i} className="rounded-xl h-16" style={{ background: "var(--color-bg-card, #151B2F)" }} />
        ))}
      </div>
      <div className="grid grid-cols-3 gap-4 mb-5">
        {Array.from({ length: 3 }).map((_, i) => (
          <div key={i} className="rounded-2xl h-28" style={{ background: "var(--color-bg-card, #151B2F)" }} />
        ))}
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
        const selected = webhooks.find((w) => w.id === selectedId) ?? webhooks[0];
        const activeCount = webhooks.filter((w) => w.is_active || w.status === "active").length;
        const avgSuccessRate = webhooks.length > 0
          ? (webhooks.reduce((s, w) => s + (w.successRate ?? 100), 0) / webhooks.length).toFixed(1)
          : "0";

        return (
          <WebhookEventsInner
            webhooks={webhooks}
            selected={selected}
            selectedId={selectedId}
            setSelectedId={setSelectedId}
            activeCount={activeCount}
            avgSuccessRate={avgSuccessRate}
          />
        );
      }}
    </QueryBoundary>
  );
}

// ── Inner component (uses delivery hooks) ─────────────────────────────────────

interface WebhookEventsInnerProps {
  webhooks: WebhookItem[];
  selected: WebhookItem | undefined;
  selectedId: string | null;
  setSelectedId: (id: string) => void;
  activeCount: number;
  avgSuccessRate: string;
}

function WebhookEventsInner({
  webhooks, selected, selectedId, setSelectedId, activeCount, avgSuccessRate,
}: WebhookEventsInnerProps) {
  const deliveriesQuery = useWebhookDeliveries(selected?.id);
  const hourlyStatsQuery = useWebhookHourlyStats();
  const retryDelivery = useRetryDelivery();

  const deliveries = deliveriesQuery.data?.deliveries ?? [];
  const chartData = hourlyStatsQuery.data ?? [];

  return (
    <div className="flex-1 overflow-y-auto px-6 py-5" style={{ background: "var(--color-bg-page, #0B1020)" }}>
      <div className="flex items-center justify-between mb-5">
        <div>
          <h2 style={{ color: "var(--color-text-primary, #E5E7EB)", fontSize: 18, fontWeight: 700 }}>Webhook Management</h2>
          <p style={{ color: "var(--color-text-muted, #6B7280)", fontSize: 12 }}>
            {activeCount} active webhooks · {avgSuccessRate}% success rate
          </p>
        </div>
        <button
          className="flex items-center gap-2 px-4 py-2 rounded-xl"
          style={{ background: "var(--color-primary-grad, linear-gradient(135deg,#4F8CFF,#3B6FCC))", color: "white", border: "none", fontSize: 13, cursor: "pointer" }}
        >
          <Plus size={14} /> Add Webhook
        </button>
      </div>

      {/* Stats row */}
      <div className="grid grid-cols-3 gap-4 mb-5">
        {[
          { label: "Success Rate",     value: `${avgSuccessRate}%`, color: "var(--color-status-success, #10B981)", sub: "All webhooks" },
          { label: "Active Webhooks",  value: activeCount,           color: "var(--color-primary, #4F8CFF)",       sub: "Listening" },
          { label: "Total Webhooks",   value: webhooks.length,       color: "var(--color-text-secondary, #9CA3AF)", sub: "Configured" },
        ].map((s) => (
          <div
            key={s.label}
            className="rounded-xl px-4 py-3 flex items-center gap-3"
            style={{ background: "var(--color-bg-card, #151B2F)", border: "1px solid var(--color-border-subtle, rgba(255,255,255,0.07))" }}
          >
            <div style={{ color: s.color, fontSize: 22, fontWeight: 700 }}>{s.value}</div>
            <div>
              <div style={{ color: "var(--color-text-primary, #E5E7EB)", fontSize: 12 }}>{s.label}</div>
              <div style={{ color: "var(--color-text-muted, #6B7280)", fontSize: 11 }}>{s.sub}</div>
            </div>
          </div>
        ))}
      </div>

      {/* Hourly chart — from API (stable) */}
      <div
        className="rounded-2xl p-5 mb-5"
        style={{ background: "var(--color-bg-card, #151B2F)", border: "1px solid var(--color-border-subtle, rgba(255,255,255,0.07))" }}
      >
        <div className="flex items-center justify-between mb-3">
          <h3 style={{ color: "var(--color-text-primary, #E5E7EB)", fontSize: 13, fontWeight: 600 }}>Delivery Activity (Today)</h3>
          {hourlyStatsQuery.isLoading && (
            <Loader2 size={13} className="animate-spin" style={{ color: "var(--color-text-muted, #6B7280)" }} />
          )}
        </div>
        <ResponsiveContainer width="100%" height={120}>
          <LineChart data={chartData}>
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
        {webhooks.map((w) => (
          <div
            key={w.id}
            onClick={() => setSelectedId(w.id)}
            className="rounded-2xl p-4 cursor-pointer"
            style={{
              background: selected?.id === w.id ? "var(--color-primary-bg, rgba(79,140,255,0.08))" : "var(--color-bg-card, #151B2F)",
              border: selected?.id === w.id ? "1px solid var(--color-primary-border, rgba(79,140,255,0.3))" : "1px solid var(--color-border-subtle, rgba(255,255,255,0.07))",
            }}
          >
            <div className="flex items-center gap-2 mb-2">
              <div
                className="w-2 h-2 rounded-full"
                style={{ background: (w.is_active || w.status === "active") ? "var(--color-status-success, #10B981)" : "var(--color-text-muted, #6B7280)" }}
              />
              <span style={{ color: "var(--color-text-primary, #E5E7EB)", fontSize: 12, fontWeight: 500 }}>
                {w.name ?? w.url.replace(/^https?:\/\//, "").split("/")[0]}
              </span>
              {w.successRate !== undefined && (
                <span className="ml-auto" style={{ color: "var(--color-status-success, #10B981)", fontSize: 11 }}>
                  {w.successRate}%
                </span>
              )}
            </div>
            <div style={{ color: "var(--color-primary, #4F8CFF)", fontSize: 11, fontFamily: "monospace" }} className="truncate">{w.url}</div>
            <div style={{ color: "var(--color-text-muted, #6B7280)", fontSize: 11, marginTop: 4 }}>{w.events.length} events</div>
          </div>
        ))}
      </div>

      {/* Delivery log — from API */}
      <div
        className="rounded-2xl"
        style={{ background: "var(--color-bg-card, #151B2F)", border: "1px solid var(--color-border-subtle, rgba(255,255,255,0.07))" }}
      >
        <div
          className="px-5 py-4 flex items-center gap-3"
          style={{ borderBottom: "1px solid var(--color-border-section, rgba(255,255,255,0.06))" }}
        >
          <h3 style={{ color: "var(--color-text-primary, #E5E7EB)", fontSize: 14, fontWeight: 600 }}>Recent Deliveries</h3>
          {deliveriesQuery.isLoading && (
            <Loader2 size={13} className="animate-spin" style={{ color: "var(--color-text-muted, #6B7280)" }} />
          )}
          {deliveriesQuery.isError && (
            <span style={{ color: "var(--color-status-error, #EF4444)", fontSize: 12 }}>
              <AlertTriangle size={12} style={{ display: "inline", marginRight: 4 }} />Failed to load
            </span>
          )}
        </div>
        <table className="w-full">
          <thead>
            <tr style={{ borderBottom: "1px solid rgba(255,255,255,0.05)" }}>
              {["Delivery ID", "Event", "Endpoint", "Status Code", "Response", "Status", "Time", ""].map((h) => (
                <th key={h || "_actions"} className="px-4 py-3 text-left" style={{ color: "var(--color-text-muted, #6B7280)", fontSize: 11, fontWeight: 600 }}>
                  {h}
                </th>
              ))}
            </tr>
          </thead>
          <tbody>
            {deliveries.map((d, i) => (
              <tr
                key={d.id}
                style={{ borderBottom: i < deliveries.length - 1 ? "1px solid rgba(255,255,255,0.04)" : "none" }}
                onMouseEnter={(e) => (e.currentTarget.style.background = "rgba(255,255,255,0.02)")}
                onMouseLeave={(e) => (e.currentTarget.style.background = "transparent")}
              >
                <td className="px-4 py-3"><span style={{ color: "var(--color-text-muted, #6B7280)", fontSize: 12 }}>{d.id}</span></td>
                <td className="px-4 py-3"><span style={{ color: "var(--color-text-primary, #E5E7EB)", fontSize: 12 }}>{d.event}</span></td>
                <td className="px-4 py-3"><span style={{ color: "var(--color-primary, #4F8CFF)", fontSize: 11 }}>{d.endpoint}</span></td>
                <td className="px-4 py-3">
                  <span style={{ color: d.status_code === 200 ? "var(--color-status-success, #10B981)" : "var(--color-status-error, #EF4444)", fontSize: 12, fontWeight: 600 }}>
                    {d.status_code}
                  </span>
                </td>
                <td className="px-4 py-3">
                  <span style={{ color: d.response_time > 1000 ? "var(--color-status-error, #EF4444)" : "var(--color-text-secondary, #9CA3AF)", fontSize: 12 }}>
                    {d.response_time}ms
                  </span>
                </td>
                <td className="px-4 py-3">
                  <span className="px-2 py-0.5 rounded" style={{ ...(STATUS_S[d.status] ?? STATUS_S.success), fontSize: 11 }}>{d.status}</span>
                </td>
                <td className="px-4 py-3">
                  <span style={{ color: "var(--color-text-faint, #4B5563)", fontSize: 11 }}>{formatTimeAgo(d.time)}</span>
                </td>
                <td className="px-4 py-3">
                  {d.status === "failed" && (
                    <button
                      onClick={() => retryDelivery.mutate(d.id)}
                      disabled={retryDelivery.isPending}
                      className="w-7 h-7 rounded-lg flex items-center justify-center"
                      style={{ background: "var(--color-status-warning-bg, rgba(245,158,11,0.1))", color: "var(--color-status-warning, #F59E0B)", border: "none", cursor: "pointer" }}
                      title="Retry delivery"
                    >
                      <RotateCcw size={11} />
                    </button>
                  )}
                </td>
              </tr>
            ))}
          </tbody>
        </table>

        {deliveries.length === 0 && !deliveriesQuery.isLoading && (
          <div className="text-center py-8">
            <p style={{ color: "var(--color-text-muted, #6B7280)", fontSize: 13 }}>No deliveries found</p>
          </div>
        )}
      </div>
    </div>
  );
}
