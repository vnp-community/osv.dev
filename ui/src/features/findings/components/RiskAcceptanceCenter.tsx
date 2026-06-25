import { useState } from "react";
import { CheckCircle, XCircle, Clock, Plus, AlertTriangle, Loader2, RefreshCw } from "lucide-react";
import { useRiskAcceptances, useUpdateRiskAcceptance, type RiskAcceptance, type RAStatus } from "../hooks/useRiskAcceptances";

// ─── Styling maps ─────────────────────────────────────────────────────────────

const STATUS_STYLES: Record<RAStatus, { bg: string; color: string }> = {
  approved: { bg: "var(--color-status-success-bg, rgba(16,185,129,0.1))", color: "var(--color-status-success, #10B981)" },
  pending:  { bg: "var(--color-status-warning-bg, rgba(245,158,11,0.1))", color: "var(--color-status-warning, #F59E0B)" },
  rejected: { bg: "var(--color-status-error-bg, rgba(239,68,68,0.1))",    color: "var(--color-status-error, #EF4444)" },
  expired:  { bg: "var(--color-status-neutral-bg, rgba(107,114,128,0.1))", color: "var(--color-text-muted, #6B7280)" },
};

const SEVERITY_COLORS: Record<string, string> = {
  Critical: "var(--color-severity-critical, #EF4444)",
  High:     "var(--color-severity-high, #F97316)",
  Medium:   "var(--color-severity-medium, #EAB308)",
  Low:      "var(--color-severity-low, #3B82F6)",
};

function formatExpiration(dateStr: string): string {
  const d = new Date(dateStr);
  return d.toLocaleDateString("en-GB", { day: "numeric", month: "short", year: "numeric" });
}

function getDaysLeft(dateStr: string): number {
  return Math.round((new Date(dateStr).getTime() - Date.now()) / 86400000);
}

// ─── Main Component ───────────────────────────────────────────────────────────

export function RiskAcceptanceCenter() {
  const [filter, setFilter] = useState("All");
  const [selectedId, setSelectedId] = useState<string | null>(null);

  const statusParam = filter !== "All" ? filter.toLowerCase() : undefined;
  const { data, isLoading, isError, refetch } = useRiskAcceptances(
    statusParam ? { status: statusParam } : undefined
  );
  const updateRA = useUpdateRiskAcceptance();

  const items = data?.items ?? [];
  // Client-side filter fallback (server might not filter)
  const filtered = filter === "All"
    ? items
    : items.filter((a) => a.status === filter.toLowerCase());
  const selected = filtered.find((a) => a.id === selectedId) ?? filtered[0] ?? null;

  if (isLoading) {
    return (
      <div className="flex-1 flex items-center justify-center" style={{ background: "var(--color-bg-page, #0B1020)" }}>
        <div className="flex flex-col items-center gap-3">
          <Loader2 size={28} className="animate-spin" style={{ color: "var(--color-primary, #4F8CFF)" }} />
          <p style={{ color: "var(--color-text-muted, #6B7280)", fontSize: 13 }}>Loading risk acceptances...</p>
        </div>
      </div>
    );
  }

  if (isError) {
    return (
      <div className="flex-1 flex items-center justify-center" style={{ background: "var(--color-bg-page, #0B1020)" }}>
        <div className="text-center">
          <AlertTriangle size={32} style={{ color: "var(--color-status-error, #EF4444)", margin: "0 auto 12px" }} />
          <p style={{ color: "var(--color-text-secondary, #9CA3AF)", fontSize: 13 }}>Failed to load risk acceptances</p>
          <button
            onClick={() => refetch()}
            className="mt-3 px-4 py-2 rounded-xl flex items-center gap-2 mx-auto"
            style={{ background: "var(--color-primary-bg, rgba(79,140,255,0.1))", color: "var(--color-primary, #4F8CFF)", border: "none", cursor: "pointer", fontSize: 13 }}
          >
            <RefreshCw size={13} /> Retry
          </button>
        </div>
      </div>
    );
  }

  return (
    <div className="flex flex-col flex-1 overflow-hidden" style={{ background: "var(--color-bg-page, #0B1020)" }}>
      {/* Header */}
      <div className="px-6 py-4" style={{ borderBottom: "1px solid var(--color-border-section, rgba(255,255,255,0.06))" }}>
        <div className="flex items-center justify-between mb-3">
          <div>
            <h2 style={{ color: "var(--color-text-primary, #E5E7EB)", fontSize: 18, fontWeight: 700 }}>
              Risk Acceptance Center
            </h2>
            <p style={{ color: "var(--color-text-muted, #6B7280)", fontSize: 12 }}>
              Manage accepted risks and formal exceptions · {data?.total ?? 0} total
            </p>
          </div>
          <button
            className="flex items-center gap-2 px-4 py-2 rounded-xl"
            style={{
              background: "var(--color-primary-grad, linear-gradient(135deg,#4F8CFF,#3B6FCC))",
              color: "white",
              border: "none",
              fontSize: 13,
              cursor: "pointer",
            }}
          >
            <Plus size={14} /> New Acceptance
          </button>
        </div>
        <div className="flex gap-2">
          {["All", "Pending", "Approved", "Expired"].map((f) => (
            <button
              key={f}
              onClick={() => setFilter(f)}
              className="px-3 py-1.5 rounded-lg"
              style={{
                background: filter === f ? "var(--color-primary-bg, rgba(79,140,255,0.12))" : "var(--color-bg-input, rgba(255,255,255,0.05))",
                color: filter === f ? "var(--color-primary, #4F8CFF)" : "var(--color-text-muted, #6B7280)",
                fontSize: 12,
                border: "none",
                cursor: "pointer",
              }}
            >
              {f}
            </button>
          ))}
        </div>
      </div>

      <div className="flex flex-1 overflow-hidden">
        {/* Table */}
        <div className="flex-1 overflow-y-auto">
          <table className="w-full">
            <thead style={{ position: "sticky", top: 0, background: "var(--color-bg-sidebar, #0D1525)", zIndex: 5 }}>
              <tr style={{ borderBottom: "1px solid var(--color-border-section, rgba(255,255,255,0.06))" }}>
                {["ID", "Finding", "Product", "Severity", "Expiration", "Owner", "Status", ""].map((h) => (
                  <th
                    key={h || "_actions"}
                    className="px-4 py-3 text-left"
                    style={{ color: "var(--color-text-muted, #6B7280)", fontSize: 11, fontWeight: 600, letterSpacing: 0.5 }}
                  >
                    {h}
                  </th>
                ))}
              </tr>
            </thead>
            <tbody>
              {filtered.map((a) => {
                const daysLeft = getDaysLeft(a.expiration_date);
                const expirationColor =
                  daysLeft < 0
                    ? "var(--color-status-error, #EF4444)"
                    : daysLeft < 30
                    ? "var(--color-status-warning, #F59E0B)"
                    : "var(--color-text-muted, #6B7280)";

                return (
                  <tr
                    key={a.id}
                    onClick={() => setSelectedId(a.id)}
                    className="cursor-pointer transition-all"
                    style={{
                      borderBottom: "1px solid var(--color-border-section, rgba(255,255,255,0.04))",
                      background: selected?.id === a.id ? "var(--color-primary-bg, rgba(79,140,255,0.07))" : "transparent",
                      borderLeft: selected?.id === a.id ? "2px solid var(--color-primary, #4F8CFF)" : "2px solid transparent",
                    }}
                    onMouseEnter={(e) => { if (selected?.id !== a.id) e.currentTarget.style.background = "var(--color-bg-hover, rgba(255,255,255,0.02))"; }}
                    onMouseLeave={(e) => { if (selected?.id !== a.id) e.currentTarget.style.background = "transparent"; }}
                  >
                    <td className="px-4 py-3">
                      <span style={{ color: "var(--color-text-muted, #6B7280)", fontSize: 12 }}>{a.id}</span>
                    </td>
                    <td className="px-4 py-3">
                      <div>
                        <span style={{ color: "var(--color-primary, #4F8CFF)", fontSize: 11 }}>{a.finding_id}</span>
                      </div>
                      <div style={{ color: "var(--color-text-primary, #E5E7EB)", fontSize: 12 }} className="truncate max-w-xs">
                        {a.finding_title}
                      </div>
                    </td>
                    <td className="px-4 py-3">
                      <span style={{ color: "var(--color-text-secondary, #9CA3AF)", fontSize: 12 }}>{a.product_name}</span>
                    </td>
                    <td className="px-4 py-3">
                      <span
                        className="px-2 py-0.5 rounded"
                        style={{ background: SEVERITY_COLORS[a.severity] + "20", color: SEVERITY_COLORS[a.severity], fontSize: 11 }}
                      >
                        {a.severity}
                      </span>
                    </td>
                    <td className="px-4 py-3">
                      <span style={{ color: expirationColor, fontSize: 12 }}>
                        {formatExpiration(a.expiration_date)}
                      </span>
                    </td>
                    <td className="px-4 py-3">
                      <span style={{ color: "var(--color-text-secondary, #9CA3AF)", fontSize: 12 }}>{a.owner}</span>
                    </td>
                    <td className="px-4 py-3">
                      <span className="px-2 py-0.5 rounded" style={{ ...STATUS_STYLES[a.status], fontSize: 11 }}>
                        {a.status}
                      </span>
                    </td>
                    <td className="px-4 py-3">
                      {a.status === "pending" && (
                        <div className="flex gap-1.5">
                          <button
                            onClick={(e) => { e.stopPropagation(); updateRA.mutate({ id: a.id, status: "approved" }); }}
                            className="w-7 h-7 rounded-lg flex items-center justify-center"
                            style={{ background: "var(--color-status-success-bg, rgba(16,185,129,0.1))", color: "var(--color-status-success, #10B981)", border: "none", cursor: "pointer" }}
                            title="Approve"
                          >
                            <CheckCircle size={12} />
                          </button>
                          <button
                            onClick={(e) => { e.stopPropagation(); updateRA.mutate({ id: a.id, status: "rejected" }); }}
                            className="w-7 h-7 rounded-lg flex items-center justify-center"
                            style={{ background: "var(--color-status-error-bg, rgba(239,68,68,0.1))", color: "var(--color-status-error, #EF4444)", border: "none", cursor: "pointer" }}
                            title="Reject"
                          >
                            <XCircle size={12} />
                          </button>
                          <button
                            className="w-7 h-7 rounded-lg flex items-center justify-center"
                            style={{ background: "var(--color-primary-bg, rgba(79,140,255,0.1))", color: "var(--color-primary, #4F8CFF)", border: "none", cursor: "pointer" }}
                            title="Schedule review"
                          >
                            <Clock size={12} />
                          </button>
                        </div>
                      )}
                    </td>
                  </tr>
                );
              })}
            </tbody>
          </table>

          {filtered.length === 0 && (
            <div className="text-center py-12">
              <p style={{ color: "var(--color-text-muted, #6B7280)", fontSize: 13 }}>No risk acceptances found</p>
            </div>
          )}
        </div>

        {/* Detail panel */}
        {selected && (
          <div
            className="w-80 flex-shrink-0 overflow-y-auto"
            style={{
              background: "var(--color-bg-sidebar, #0F1629)",
              borderLeft: "1px solid var(--color-border-section, rgba(255,255,255,0.06))",
            }}
          >
            <div className="p-5" style={{ borderBottom: "1px solid var(--color-border-section, rgba(255,255,255,0.06))" }}>
              <div style={{ color: "var(--color-text-muted, #6B7280)", fontSize: 11, marginBottom: 4 }}>{selected.id}</div>
              <div style={{ color: "var(--color-text-primary, #E5E7EB)", fontSize: 14, fontWeight: 600, marginBottom: 6 }}>
                {selected.finding_title}
              </div>
              <div className="flex gap-2">
                <span
                  className="px-2 py-0.5 rounded"
                  style={{ background: SEVERITY_COLORS[selected.severity] + "20", color: SEVERITY_COLORS[selected.severity], fontSize: 11 }}
                >
                  {selected.severity}
                </span>
                <span className="px-2 py-0.5 rounded" style={{ ...STATUS_STYLES[selected.status], fontSize: 11 }}>
                  {selected.status}
                </span>
              </div>
            </div>

            <div className="p-4" style={{ borderBottom: "1px solid var(--color-border-section, rgba(255,255,255,0.06))" }}>
              <div style={{ color: "var(--color-text-secondary, #9CA3AF)", fontSize: 11, fontWeight: 600, marginBottom: 8 }}>
                BUSINESS JUSTIFICATION
              </div>
              <div className="rounded-xl p-3" style={{ background: "var(--color-bg-input, rgba(255,255,255,0.04))" }}>
                <p style={{ color: "var(--color-text-primary, #E5E7EB)", fontSize: 12, lineHeight: 1.6 }}>
                  {selected.reason}
                </p>
              </div>
            </div>

            <div className="p-4">
              {[
                { label: "Finding",   value: selected.finding_id },
                { label: "Product",   value: selected.product_name },
                { label: "Owner",     value: selected.owner },
                { label: "Expires",   value: formatExpiration(selected.expiration_date) },
                {
                  label: "Days Left",
                  value: (() => {
                    const dl = getDaysLeft(selected.expiration_date);
                    return dl < 0 ? `${Math.abs(dl)} days overdue` : `${dl} days`;
                  })(),
                },
              ].map(({ label, value }) => {
                const dl = getDaysLeft(selected.expiration_date);
                return (
                  <div
                    key={label}
                    className="flex justify-between py-2"
                    style={{ borderBottom: "1px solid var(--color-border-section, rgba(255,255,255,0.04))" }}
                  >
                    <span style={{ color: "var(--color-text-muted, #6B7280)", fontSize: 12 }}>{label}</span>
                    <span
                      style={{
                        color: label === "Days Left" && dl < 0
                          ? "var(--color-status-error, #EF4444)"
                          : "var(--color-text-primary, #E5E7EB)",
                        fontSize: 12,
                      }}
                    >
                      {value}
                    </span>
                  </div>
                );
              })}
            </div>
          </div>
        )}
      </div>
    </div>
  );
}
