import { useState } from "react";
import { Search, Download, Settings, User, AlertTriangle, Loader2, RefreshCw } from "lucide-react";
import { useAuditLogs } from "../hooks/useAuditLogs";
import type { AuditSeverity } from "../types";

// ─── Styling maps — dùng CSS variables từ tokens.css ────────────────────────
const SEVERITY_STYLES: Record<AuditSeverity, { bg: string; color: string }> = {
  Info:     { bg: "var(--color-status-info-bg, rgba(79,140,255,0.1))",     color: "var(--color-status-info, #4F8CFF)" },
  Warning:  { bg: "var(--color-status-warning-bg, rgba(245,158,11,0.1))",  color: "var(--color-status-warning, #F59E0B)" },
  Critical: { bg: "var(--color-status-error-bg, rgba(239,68,68,0.1))",     color: "var(--color-status-error, #EF4444)" },
};

const ACTION_ICON_MAP: Record<string, React.ElementType> = {
  CREATE_SCAN: Settings, SCAN_COMPLETED: Settings, AUTO_SCAN_START: Settings, UPDATE_SYSTEM_SETTINGS: Settings,
  UPDATE_FINDING: AlertTriangle, ASSIGN_FINDING: User, MARK_FALSE_POSITIVE: AlertTriangle,
  CREATE_API_KEY: Settings, ACCEPT_RISK: AlertTriangle,
  USER_DISABLED: User, LOGIN_FAILED: User,
};

function getActionIcon(action: string): React.ElementType {
  return ACTION_ICON_MAP[action] ?? Settings;
}

function getUserInitials(userName: string): string {
  if (userName === "system") return "SYS";
  return userName.split("@")[0].slice(0, 2).toUpperCase();
}

function formatTimestamp(iso: string): string {
  return new Date(iso).toLocaleString("en-GB", {
    year: "numeric", month: "2-digit", day: "2-digit",
    hour: "2-digit", minute: "2-digit", second: "2-digit",
    hour12: false,
  }).replace(",", "");
}

function tryParseJson(raw?: string): Record<string, unknown> | string | null {
  if (!raw) return null;
  try { return JSON.parse(raw) as Record<string, unknown>; } catch { return raw; }
}

// ─── Main Component ───────────────────────────────────────────────────────────

export function AuditLogs() {
  const [search, setSearch] = useState("");
  const [filterSeverity, setFilterSeverity] = useState<AuditSeverity | "All">("All");
  const [expanded, setExpanded] = useState<string | null>(null);

  const { data, isLoading, isError, refetch } = useAuditLogs({
    search: search || undefined,
    severity: filterSeverity !== "All" ? filterSeverity : undefined,
  });

  const entries = data?.entries ?? [];

  if (isLoading) {
    return (
      <div className="flex-1 flex items-center justify-center" style={{ background: "var(--color-bg-page, #0B1020)" }}>
        <div className="flex flex-col items-center gap-3">
          <Loader2 size={28} className="animate-spin" style={{ color: "var(--color-primary, #4F8CFF)" }} />
          <p style={{ color: "var(--color-text-muted, #6B7280)", fontSize: 13 }}>Loading audit logs...</p>
        </div>
      </div>
    );
  }

  if (isError) {
    return (
      <div className="flex-1 flex items-center justify-center" style={{ background: "var(--color-bg-page, #0B1020)" }}>
        <div className="text-center">
          <AlertTriangle size={32} style={{ color: "var(--color-status-error, #EF4444)", margin: "0 auto 12px" }} />
          <p style={{ color: "var(--color-text-secondary, #9CA3AF)", fontSize: 13 }}>Failed to load audit logs</p>
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
    <div className="flex-1 overflow-y-auto px-6 py-5" style={{ background: "var(--color-bg-page, #0B1020)" }}>
      {/* Header */}
      <div className="flex items-center justify-between mb-5">
        <div>
          <h2 style={{ color: "var(--color-text-primary, #E5E7EB)", fontSize: 18, fontWeight: 700 }}>Audit Logs</h2>
          <p style={{ color: "var(--color-text-muted, #6B7280)", fontSize: 12 }}>
            Immutable audit trail · {data?.total ?? 0} total events
          </p>
        </div>
        <button
          className="flex items-center gap-2 px-4 py-2 rounded-xl"
          style={{
            background: "var(--color-bg-input, rgba(255,255,255,0.05))",
            border: "1px solid var(--color-border-input, rgba(255,255,255,0.09))",
            color: "var(--color-text-secondary, #9CA3AF)",
            fontSize: 13,
            cursor: "pointer",
          }}
        >
          <Download size={14} /> Export CSV
        </button>
      </div>

      {/* Filters */}
      <div className="flex items-center gap-3 mb-5">
        <div className="relative">
          <Search
            size={13}
            color="var(--color-text-faint, #4B5563)"
            style={{ position: "absolute", left: 10, top: "50%", transform: "translateY(-50%)" }}
          />
          <input
            id="audit-search"
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            placeholder="Search actions, users..."
            className="rounded-xl pl-8 pr-4 py-2 outline-none"
            style={{
              background: "var(--color-bg-card, #151B2F)",
              border: "1px solid var(--color-border-card, rgba(255,255,255,0.08))",
              color: "var(--color-text-primary, #E5E7EB)",
              fontSize: 12,
              width: 240,
            }}
          />
        </div>
        {(["All", "Info", "Warning", "Critical"] as const).map((s) => {
          const style = s !== "All" ? SEVERITY_STYLES[s] : null;
          return (
            <button
              key={s}
              onClick={() => setFilterSeverity(s)}
              className="px-3 py-2 rounded-lg"
              style={{
                background: filterSeverity === s
                  ? (style?.bg ?? "var(--color-primary-bg, rgba(79,140,255,0.12))")
                  : "var(--color-bg-input, rgba(255,255,255,0.05))",
                color: filterSeverity === s
                  ? (style?.color ?? "var(--color-primary, #4F8CFF)")
                  : "var(--color-text-muted, #6B7280)",
                fontSize: 12,
                border: "none",
                cursor: "pointer",
              }}
            >
              {s}
            </button>
          );
        })}
        <span style={{ color: "var(--color-text-muted, #6B7280)", fontSize: 12, marginLeft: "auto" }}>
          {entries.length} events
        </span>
      </div>

      {/* Table */}
      <div
        className="rounded-2xl overflow-hidden"
        style={{
          background: "var(--color-bg-card, #151B2F)",
          border: "1px solid var(--color-border-subtle, rgba(255,255,255,0.07))",
        }}
      >
        <table className="w-full">
          <thead>
            <tr style={{ borderBottom: "1px solid var(--color-border-section, rgba(255,255,255,0.06))" }}>
              {["Timestamp", "User", "Action", "Resource", "Severity", ""].map((h) => (
                <th
                  key={h || "_expand"}
                  className="px-4 py-3 text-left"
                  style={{ color: "var(--color-text-muted, #6B7280)", fontSize: 11, fontWeight: 600, letterSpacing: 0.5 }}
                >
                  {h}
                </th>
              ))}
            </tr>
          </thead>
          <tbody>
            {entries.map((log) => {
              const Icon = getActionIcon(log.action);
              const isExpanded = expanded === log.id;
              const severityStyle = log.severity ? SEVERITY_STYLES[log.severity] : SEVERITY_STYLES.Info;
              const before = tryParseJson(log.before);
              const after = tryParseJson(log.after);

              return (
                <>
                  <tr
                    key={log.id}
                    className="cursor-pointer transition-all"
                    style={{
                      borderBottom: "1px solid var(--color-border-section, rgba(255,255,255,0.04))",
                      background: isExpanded ? "var(--color-primary-bg, rgba(79,140,255,0.04))" : "transparent",
                    }}
                    onClick={() => setExpanded(isExpanded ? null : log.id)}
                    onMouseEnter={(e) => { if (!isExpanded) e.currentTarget.style.background = "var(--color-bg-hover, rgba(255,255,255,0.02))"; }}
                    onMouseLeave={(e) => { if (!isExpanded) e.currentTarget.style.background = "transparent"; }}
                  >
                    <td className="px-4 py-3">
                      <span style={{ color: "var(--color-text-muted, #6B7280)", fontSize: 11, fontFamily: "monospace" }}>
                        {formatTimestamp(log.timestamp)}
                      </span>
                    </td>
                    <td className="px-4 py-3">
                      <div className="flex items-center gap-2">
                        <div
                          className="w-5 h-5 rounded-full flex items-center justify-center"
                          style={{
                            background: log.user_name === "system"
                              ? "var(--color-text-disabled, #374151)"
                              : "var(--color-primary-border, rgba(79,140,255,0.3))",
                            fontSize: 9,
                            color: "white",
                            fontWeight: 700,
                          }}
                        >
                          {getUserInitials(log.user_name)}
                        </div>
                        <span style={{ color: "var(--color-text-secondary, #9CA3AF)", fontSize: 12 }}>
                          {log.user_name}
                        </span>
                      </div>
                    </td>
                    <td className="px-4 py-3">
                      <div className="flex items-center gap-2">
                        <Icon size={12} color="var(--color-text-muted, #6B7280)" />
                        <span style={{ color: "var(--color-text-primary, #E5E7EB)", fontSize: 12, fontFamily: "monospace" }}>
                          {log.action}
                        </span>
                      </div>
                    </td>
                    <td className="px-4 py-3">
                      <span style={{ color: "var(--color-text-muted, #6B7280)", fontSize: 12 }}>
                        {log.resource ?? `${log.entity_type} / ${log.entity_id}`}
                      </span>
                    </td>
                    <td className="px-4 py-3">
                      {log.severity && (
                        <span className="px-2 py-0.5 rounded" style={{ ...severityStyle, fontSize: 11 }}>
                          {log.severity}
                        </span>
                      )}
                    </td>
                    <td className="px-4 py-3">
                      <span style={{ color: "var(--color-text-faint, #4B5563)", fontSize: 11 }}>
                        {isExpanded ? "▲" : "▼"}
                      </span>
                    </td>
                  </tr>

                  {isExpanded && (
                    <tr key={`${log.id}-detail`} style={{ borderBottom: "1px solid var(--color-border-section, rgba(255,255,255,0.04))" }}>
                      <td colSpan={6} className="px-4 py-3">
                        <div className="grid grid-cols-2 gap-4">
                          {before && (
                            <div
                              className="rounded-xl p-3"
                              style={{
                                background: "var(--color-status-error-bg, rgba(239,68,68,0.06))",
                                border: "1px solid var(--color-status-error-border, rgba(239,68,68,0.15))",
                              }}
                            >
                              <div style={{ color: "var(--color-status-error, #EF4444)", fontSize: 10, fontWeight: 600, marginBottom: 6 }}>
                                BEFORE
                              </div>
                              <pre style={{ color: "#FCA5A5", fontSize: 11, fontFamily: "monospace" }}>
                                {JSON.stringify(before, null, 2)}
                              </pre>
                            </div>
                          )}
                          {after && (
                            <div
                              className="rounded-xl p-3"
                              style={{
                                background: "var(--color-status-success-bg, rgba(16,185,129,0.06))",
                                border: "1px solid var(--color-status-success-border, rgba(16,185,129,0.15))",
                              }}
                            >
                              <div style={{ color: "var(--color-status-success, #10B981)", fontSize: 10, fontWeight: 600, marginBottom: 6 }}>
                                AFTER
                              </div>
                              <pre style={{ color: "#A7F3D0", fontSize: 11, fontFamily: "monospace" }}>
                                {JSON.stringify(after, null, 2)}
                              </pre>
                            </div>
                          )}
                        </div>
                      </td>
                    </tr>
                  )}
                </>
              );
            })}
          </tbody>
        </table>

        {entries.length === 0 && (
          <div className="text-center py-10">
            <p style={{ color: "var(--color-text-muted, #6B7280)", fontSize: 13 }}>No audit events found</p>
          </div>
        )}
      </div>
    </div>
  );
}
