import { useState } from "react";
import { Search, Eye, Download, Copy } from "lucide-react";
import { useScans } from "../hooks/useScans";
import type { ScansListResponse } from "../api/scanApi";
import type { Scan } from "@/shared/types/scan";

// ─── Styling maps ─────────────────────────────────────────────────────────────

const TYPE_COLORS: Record<string, string> = {
  nmap_full:      "var(--color-primary, #4F8CFF)",
  nmap_discovery: "var(--color-primary, #4F8CFF)",
  zap:            "var(--color-severity-high, #F97316)",
  agent:          "var(--color-status-success, #10B981)",
  import:         "var(--color-ai-accent, #A78BFA)",
  NMAP:           "var(--color-primary, #4F8CFF)",
  ZAP:            "var(--color-severity-high, #F97316)",
  AGENT:          "var(--color-status-success, #10B981)",
};

const TYPE_LABEL: Record<string, string> = {
  nmap_full:      "NMAP",
  nmap_discovery: "NMAP",
  zap:            "ZAP",
  agent:          "AGENT",
  import:         "IMPORT",
};

function formatDate(iso?: string): string {
  if (!iso) return "—";
  const d = new Date(iso);
  return d.toLocaleDateString("en-GB", { day: "numeric", month: "short" }) +
    ", " + d.toLocaleTimeString("en-GB", { hour: "2-digit", minute: "2-digit" });
}

function formatDuration(scan: Scan): string {
  if (!scan.startedAt || !scan.completedAt) return "—";
  const ms = new Date(scan.completedAt).getTime() - new Date(scan.startedAt).getTime();
  const s = Math.floor(ms / 1000);
  const m = Math.floor(s / 60);
  const h = Math.floor(m / 60);
  return `${String(h).padStart(2, "0")}:${String(m % 60).padStart(2, "0")}:${String(s % 60).padStart(2, "0")}`;
}

// ─── Skeleton ─────────────────────────────────────────────────────────────────

function ScanHistorySkeleton() {
  return (
    <div className="flex-1 overflow-y-auto px-6 py-5 animate-pulse" style={{ background: "var(--color-bg-page, #0B1020)" }}>
      <div className="rounded-2xl" style={{ background: "var(--color-bg-card, #151B2F)", height: 400 }} />
    </div>
  );
}

// ─── Main Component ───────────────────────────────────────────────────────────

export function ScanHistory({ onViewScan }: { onViewScan?: (id: string) => void }) {
  const [search, setSearch] = useState("");
  const [filterType, setFilterType] = useState("All");
  const [filterStatus, setFilterStatus] = useState("All");
  const [page, setPage] = useState(1);

  const statusParam =
    filterStatus === "All" ? "completed,failed,cancelled" : filterStatus;

  const scansQuery = useScans({
    status: statusParam,
  });

  if (scansQuery.isLoading) return <ScanHistorySkeleton />;

  const allScans: Scan[] = (scansQuery.data as unknown as ScansListResponse)?.scans ?? [];

  // Client-side filter for type and search
  const filtered = allScans.filter((s) => {
    const label = TYPE_LABEL[s.type] ?? s.type.toUpperCase();
    const matchType = filterType === "All" || label === filterType;
    const matchSearch = !search ||
      s.name.toLowerCase().includes(search.toLowerCase()) ||
      (s.targets?.[0] ?? "").toLowerCase().includes(search.toLowerCase());
    return matchType && matchSearch;
  });

  const PAGE_SIZE = 20;
  const totalPages = Math.ceil(filtered.length / PAGE_SIZE);
  const paginated = filtered.slice((page - 1) * PAGE_SIZE, page * PAGE_SIZE);

  return (
    <div className="flex-1 overflow-y-auto px-6 py-5" style={{ background: "var(--color-bg-page, #0B1020)" }}>
      {/* Header */}
      <div className="flex items-center justify-between mb-5">
        <div>
          <h2 style={{ color: "var(--color-text-primary, #E5E7EB)", fontSize: 18, fontWeight: 700 }}>Scan History</h2>
          <p style={{ color: "var(--color-text-muted, #6B7280)", fontSize: 12 }}>
            {filtered.length} scans · All time
          </p>
        </div>
        <button
          className="flex items-center gap-2 px-4 py-2 rounded-xl"
          style={{ background: "var(--color-bg-input, rgba(255,255,255,0.05))", border: "1px solid var(--color-border-card, rgba(255,255,255,0.09))", color: "var(--color-text-secondary, #9CA3AF)", fontSize: 13, cursor: "pointer" }}
        >
          <Download size={14} /> Export CSV
        </button>
      </div>

      {/* Filters */}
      <div className="flex items-center gap-3 mb-4">
        <div className="relative">
          <Search size={13} color="var(--color-text-faint, #4B5563)" style={{ position: "absolute", left: 10, top: "50%", transform: "translateY(-50%)" }} />
          <input
            id="scan-history-search"
            value={search}
            onChange={(e) => { setSearch(e.target.value); setPage(1); }}
            placeholder="Search scans..."
            className="rounded-xl pl-8 pr-4 py-2 outline-none"
            style={{ background: "var(--color-bg-card, #151B2F)", border: "1px solid var(--color-border-card, rgba(255,255,255,0.08))", color: "var(--color-text-primary, #E5E7EB)", fontSize: 12, width: 200 }}
          />
        </div>
        {["All", "NMAP", "ZAP", "AGENT"].map((t) => (
          <button
            key={t}
            onClick={() => { setFilterType(t); setPage(1); }}
            className="px-3 py-1.5 rounded-lg"
            style={{
              background: filterType === t
                ? ((TYPE_COLORS[t] ?? "rgba(79,140,255,0.12)") + "20")
                : "var(--color-bg-input, rgba(255,255,255,0.05))",
              color: filterType === t ? (TYPE_COLORS[t] ?? "var(--color-primary, #4F8CFF)") : "var(--color-text-muted, #6B7280)",
              fontSize: 12,
              border: "none",
              cursor: "pointer",
            }}
          >
            {t}
          </button>
        ))}
        <select
          value={filterStatus}
          onChange={(e) => { setFilterStatus(e.target.value); setPage(1); }}
          className="rounded-xl px-3 py-2 outline-none"
          style={{ background: "var(--color-bg-card, #151B2F)", border: "1px solid var(--color-border-card, rgba(255,255,255,0.08))", color: "var(--color-text-secondary, #9CA3AF)", fontSize: 12 }}
        >
          <option value="All">All</option>
          <option value="completed">Completed</option>
          <option value="failed">Failed</option>
          <option value="cancelled">Cancelled</option>
        </select>
      </div>

      <div className="rounded-2xl" style={{ background: "var(--color-bg-card, #151B2F)", border: "1px solid var(--color-border-subtle, rgba(255,255,255,0.07))" }}>
        <table className="w-full">
          <thead>
            <tr style={{ borderBottom: "1px solid var(--color-border-section, rgba(255,255,255,0.06))" }}>
              {["Scan ID", "Name", "Target", "Type", "Date", "Duration", "Findings", "Status", ""].map((h) => (
                <th key={h || "_actions"} className="px-4 py-3 text-left" style={{ color: "var(--color-text-muted, #6B7280)", fontSize: 11, fontWeight: 600, letterSpacing: 0.5 }}>
                  {h}
                </th>
              ))}
            </tr>
          </thead>
          <tbody>
            {paginated.map((s, i) => {
              const label = TYPE_LABEL[s.type] ?? s.type.toUpperCase();
              const typeColor = TYPE_COLORS[s.type] ?? "var(--color-text-muted, #6B7280)";
              const findingsColor = (s.findingCount ?? 0) > 20
                ? "var(--color-status-error, #EF4444)"
                : (s.findingCount ?? 0) > 0
                ? "var(--color-status-warning, #F59E0B)"
                : "var(--color-status-success, #10B981)";
              return (
                <tr
                  key={s.id}
                  className="transition-all"
                  style={{ borderBottom: i < paginated.length - 1 ? "1px solid var(--color-border-section, rgba(255,255,255,0.04))" : "none" }}
                  onMouseEnter={(e) => (e.currentTarget.style.background = "var(--color-bg-hover, rgba(255,255,255,0.02))")}
                  onMouseLeave={(e) => (e.currentTarget.style.background = "transparent")}
                >
                  <td className="px-4 py-3">
                    <span style={{ color: "var(--color-primary, #4F8CFF)", fontSize: 12, fontFamily: "monospace" }}>{s.id}</span>
                  </td>
                  <td className="px-4 py-3">
                    <span style={{ color: "var(--color-text-primary, #E5E7EB)", fontSize: 12 }}>{s.name}</span>
                  </td>
                  <td className="px-4 py-3">
                    <span style={{ color: "var(--color-text-muted, #6B7280)", fontSize: 11, fontFamily: "monospace" }}>{s.targets?.[0] ?? "—"}</span>
                  </td>
                  <td className="px-4 py-3">
                    <span className="px-2 py-0.5 rounded" style={{ background: typeColor + "20", color: typeColor, fontSize: 11 }}>{label}</span>
                  </td>
                  <td className="px-4 py-3">
                    <span style={{ color: "var(--color-text-muted, #6B7280)", fontSize: 12 }}>{formatDate(s.startedAt ?? s.completedAt)}</span>
                  </td>
                  <td className="px-4 py-3">
                    <span style={{ color: "var(--color-text-secondary, #9CA3AF)", fontSize: 12, fontFamily: "monospace" }}>{formatDuration(s)}</span>
                  </td>
                  <td className="px-4 py-3">
                    <span style={{ color: findingsColor, fontSize: 12, fontWeight: 600 }}>{s.findingCount ?? 0}</span>
                  </td>
                  <td className="px-4 py-3">
                    <span
                      className="px-2 py-0.5 rounded"
                      style={{
                        background: s.status === "completed"
                          ? "var(--color-status-success-bg, rgba(16,185,129,0.1))"
                          : "var(--color-status-error-bg, rgba(239,68,68,0.1))",
                        color: s.status === "completed"
                          ? "var(--color-status-success, #10B981)"
                          : "var(--color-status-error, #EF4444)",
                        fontSize: 11,
                      }}
                    >
                      {s.status}
                    </span>
                  </td>
                  <td className="px-4 py-3">
                    <div className="flex gap-1.5">
                      <button
                        onClick={() => onViewScan?.(s.id)}
                        className="w-7 h-7 rounded-lg flex items-center justify-center"
                        style={{ background: "var(--color-primary-bg, rgba(79,140,255,0.1))", color: "var(--color-primary, #4F8CFF)", border: "none", cursor: "pointer" }}
                        title="View"
                      >
                        <Eye size={12} />
                      </button>
                      <button
                        className="w-7 h-7 rounded-lg flex items-center justify-center"
                        style={{ background: "var(--color-bg-input, rgba(255,255,255,0.05))", color: "var(--color-text-muted, #6B7280)", border: "none", cursor: "pointer" }}
                        title="Copy ID"
                        onClick={() => navigator.clipboard.writeText(s.id)}
                      >
                        <Copy size={12} />
                      </button>
                      <button
                        className="w-7 h-7 rounded-lg flex items-center justify-center"
                        style={{ background: "var(--color-bg-input, rgba(255,255,255,0.05))", color: "var(--color-text-muted, #6B7280)", border: "none", cursor: "pointer" }}
                        title="Download report"
                      >
                        <Download size={12} />
                      </button>
                    </div>
                  </td>
                </tr>
              );
            })}
          </tbody>
        </table>

        {paginated.length === 0 && (
          <div className="text-center py-10">
            <p style={{ color: "var(--color-text-muted, #6B7280)", fontSize: 13 }}>No scans found</p>
          </div>
        )}

        {/* Pagination */}
        {totalPages > 1 && (
          <div
            className="flex items-center justify-between px-5 py-3"
            style={{ borderTop: "1px solid var(--color-border-section, rgba(255,255,255,0.06))" }}
          >
            <span style={{ color: "var(--color-text-muted, #6B7280)", fontSize: 12 }}>
              Page {page} of {totalPages} · {filtered.length} total
            </span>
            <div className="flex gap-2">
              <button
                onClick={() => setPage((p) => Math.max(1, p - 1))}
                disabled={page === 1}
                className="px-3 py-1.5 rounded-lg"
                style={{
                  background: "var(--color-bg-input, rgba(255,255,255,0.05))",
                  color: page === 1 ? "var(--color-text-faint, #4B5563)" : "var(--color-text-secondary, #9CA3AF)",
                  fontSize: 12,
                  border: "none",
                  cursor: page === 1 ? "not-allowed" : "pointer",
                }}
              >
                ← Prev
              </button>
              <button
                onClick={() => setPage((p) => Math.min(totalPages, p + 1))}
                disabled={page === totalPages}
                className="px-3 py-1.5 rounded-lg"
                style={{
                  background: "var(--color-bg-input, rgba(255,255,255,0.05))",
                  color: page === totalPages ? "var(--color-text-faint, #4B5563)" : "var(--color-text-secondary, #9CA3AF)",
                  fontSize: 12,
                  border: "none",
                  cursor: page === totalPages ? "not-allowed" : "pointer",
                }}
              >
                Next →
              </button>
            </div>
          </div>
        )}
      </div>
    </div>
  );
}
