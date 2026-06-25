import { useState, useRef } from "react";
import { useNavigate, useSearchParams } from "react-router";
import { Search, Brain, AlertTriangle, CheckCircle, RotateCcw, Tag, Eye } from "lucide-react";
import { useFindings } from "@/features/findings/hooks/useFindings";
import { useUpdateFinding } from "@/features/findings/hooks/useUpdateFinding";
import { QueryBoundary } from "@/shared/components/feedback/QueryBoundary";
import { SeverityBadge } from "@/shared/components/data-display/SeverityBadge";
import { StatusBadge } from "@/shared/components/data-display/StatusBadge";
import { SLABadge } from "@/shared/components/data-display/SLABadge";
import type { FindingStatus } from "@/shared/types/finding";
import type { Finding } from "@/shared/types/finding";
import { useVirtualizer } from "@tanstack/react-virtual";

// ─── Virtualized table body (used when findings > 100) ───────────────────────
function VirtualizedFindingsBody({
  findings,
  selected,
  toggleSelect,
  navigate,
}: {
  findings: Finding[];
  selected: string[];
  toggleSelect: (id: string) => void;
  navigate: (path: string) => void;
}) {
  const parentRef = useRef<HTMLDivElement>(null);
  const rowVirtualizer = useVirtualizer({
    count: findings.length,
    getScrollElement: () => parentRef.current,
    estimateSize: () => 48,
    overscan: 5,
  });

  return (
    <div ref={parentRef} style={{ height: "calc(100vh - 260px)", overflowY: "auto" }}>
      <div style={{ height: rowVirtualizer.getTotalSize(), position: "relative", width: "100%" }}>
        {rowVirtualizer.getVirtualItems().map((virtualItem) => {
          const f = findings[virtualItem.index];
          return (
            <div
              key={f.id}
              data-index={virtualItem.index}
              ref={rowVirtualizer.measureElement}
              className="flex items-center gap-2 px-4 cursor-pointer"
              style={{
                position: "absolute",
                top: 0,
                left: 0,
                width: "100%",
                height: 48,
                transform: `translateY(${virtualItem.start}px)`,
                borderBottom: "1px solid rgba(255,255,255,0.04)",
                background: selected.includes(f.id) ? "rgba(79,140,255,0.05)" : "transparent",
              }}
              onMouseEnter={(e) => {
                if (!selected.includes(f.id)) e.currentTarget.style.background = "rgba(255,255,255,0.02)";
              }}
              onMouseLeave={(e) => {
                if (!selected.includes(f.id)) e.currentTarget.style.background = "transparent";
              }}
            >
              <input type="checkbox" checked={selected.includes(f.id)} onChange={() => toggleSelect(f.id)} style={{ accentColor: "#4F8CFF", flexShrink: 0 }} />
              <span style={{ color: "var(--color-text-muted, #6B7280)", fontSize: 11, width: 90, flexShrink: 0 }}>{f.id}</span>
              <span style={{ color: "var(--color-text-primary, #E5E7EB)", fontSize: 12, flex: 1, overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" }} onClick={() => navigate(`/findings/${f.id}`)}>{f.title}</span>
              <span style={{ color: "var(--color-primary, #4F8CFF)", fontSize: 12, fontFamily: "monospace", width: 120, flexShrink: 0 }}>{f.cveId ?? "—"}</span>
              <div style={{ flexShrink: 0 }}><SeverityBadge severity={f.severity} /></div>
              <div style={{ flexShrink: 0 }}><StatusBadge status={f.status} /></div>
              <div style={{ flexShrink: 0 }}><SLABadge status={f.slaStatus} daysLeft={f.slaDaysLeft} /></div>
            </div>
          );
        })}
      </div>
    </div>
  );
}

const AI_COLORS: Record<string, string> = {
  Confirmed: "#EF4444",
  FalsePositive: "#6B7280",
  NotAffected: "#10B981",
  Unexplored: "#F59E0B",
};

export function FindingsList() {
  const navigate = useNavigate();
  const [searchParams, setSearchParams] = useSearchParams();
  const q = searchParams.get("q") ?? "";
  const filterSeverity = searchParams.get("severity") ?? "All";
  const filterStatus = searchParams.get("status") ?? "All";
  const [selected, setSelected] = useState<string[]>([]);

  const findingsQuery = useFindings({
    severity: filterSeverity !== "All" ? [filterSeverity] : undefined,
    status: filterStatus !== "All" ? [filterStatus as FindingStatus] : undefined,
    page: 1,
    pageSize: 50,
  });

  const { mutate: updateFinding } = useUpdateFinding();

  const setParam = (key: string, value: string) => {
    const next = new URLSearchParams(searchParams);
    if (!value || value === "All") next.delete(key); else next.set(key, value);
    setSearchParams(next);
  };

  const toggleSelect = (id: string) =>
    setSelected((prev) => prev.includes(id) ? prev.filter((x) => x !== id) : [...prev, id]);

  return (
    <div className="flex flex-col flex-1 overflow-hidden" style={{ background: "var(--color-bg-page, #0B1020)" }}>
      {/* Header */}
      <div className="px-6 py-4" style={{ borderBottom: "1px solid rgba(255,255,255,0.06)", background: "var(--color-bg-sidebar, #0F1629)" }}>
        <div className="flex items-center justify-between mb-4">
          <div>
            <h2 style={{ color: "var(--color-text-primary, #E5E7EB)", fontSize: 18, fontWeight: 700 }}>Findings</h2>
            <p style={{ color: "var(--color-text-muted, #6B7280)", fontSize: 12 }}>
              {findingsQuery.data?.total ?? "…"} total findings
            </p>
          </div>
          {selected.length > 0 && (
            <div className="flex items-center gap-2 px-4 py-2 rounded-xl" style={{ background: "rgba(79,140,255,0.08)", border: "1px solid rgba(79,140,255,0.2)" }}>
              <span style={{ color: "var(--color-primary, #4F8CFF)", fontSize: 13 }}>{selected.length} selected</span>
              {[
                { label: "Close", icon: CheckCircle, color: "var(--color-status-success, #10B981)", action: () => selected.forEach(id => updateFinding({ id, status: "mitigated" as FindingStatus })) },
                { label: "Reopen", icon: RotateCcw, color: "var(--color-status-warning, #F59E0B)", action: () => selected.forEach(id => updateFinding({ id, status: "active" as FindingStatus })) },
                { label: "Accept Risk", icon: AlertTriangle, color: "var(--color-ai, #A78BFA)", action: () => selected.forEach(id => updateFinding({ id, status: "risk_accepted" as FindingStatus })) },
                { label: "Tag", icon: Tag, color: "var(--color-text-muted, #6B7280)", action: () => {} },
              ].map(({ label, icon: Icon, color, action }) => (
                <button
                  key={label}
                  onClick={action}
                  className="flex items-center gap-1.5 px-3 py-1.5 rounded-lg"
                  style={{ background: `${color}15`, color, fontSize: 12, border: "none", cursor: "pointer" }}
                >
                  <Icon size={12} />{label}
                </button>
              ))}
            </div>
          )}
        </div>
        {/* Filters */}
        <div className="flex items-center gap-3">
          <div className="relative">
            <Search size={13} color="#4B5563" style={{ position: "absolute", left: 10, top: "50%", transform: "translateY(-50%)" }} />
            <input
              value={q}
              onChange={(e) => setParam("q", e.target.value)}
              placeholder="Search findings..."
              className="rounded-lg pl-8 pr-3 py-1.5 outline-none"
              style={{ background: "#1A2236", border: "1px solid rgba(255,255,255,0.08)", color: "var(--color-text-primary, #E5E7EB)", fontSize: 12, width: 220 }}
            />
          </div>
          {["All", "Critical", "High", "Medium", "Low"].map((s) => (
            <button
              key={s}
              onClick={() => setParam("severity", s)}
              className="px-3 py-1.5 rounded-lg"
              style={{
                background: filterSeverity === s ? "rgba(79,140,255,0.12)" : "rgba(255,255,255,0.05)",
                color: filterSeverity === s ? "#4F8CFF" : "#6B7280",
                fontSize: 12, border: "none", cursor: "pointer",
              }}
            >
              {s}
            </button>
          ))}
          <div style={{ width: 1, height: 18, background: "rgba(255,255,255,0.08)" }} />
          <select
            value={filterStatus}
            onChange={(e) => setParam("status", e.target.value)}
            className="rounded-lg px-3 py-1.5 outline-none"
            style={{ background: "#1A2236", border: "1px solid rgba(255,255,255,0.08)", color: "var(--color-text-secondary, #9CA3AF)", fontSize: 12 }}
          >
            {["All", "active", "mitigated", "risk_accepted", "false_positive"].map((s) => (
              <option key={s} value={s}>{s === "All" ? "All Status" : s.replace("_", " ")}</option>
            ))}
          </select>
        </div>
      </div>

      {/* Table — virtualized when findings > 100 */}
      <QueryBoundary query={findingsQuery}>
        {(result) => {
          const findings = result.findings;
          const allSelected = findings.length > 0 && findings.every((f) => selected.includes(f.id));

          // Use virtualization for large datasets (> 100 rows)
          if (findings.length > 100) {
            return (
              <div className="flex-1 overflow-hidden">
                {/* Sticky header */}
                <div className="flex items-center gap-2 px-4 py-2.5" style={{ background: "#0D1525", borderBottom: "1px solid rgba(255,255,255,0.06)", position: "sticky", top: 0, zIndex: 5 }}>
                  <input
                    type="checkbox"
                    checked={allSelected}
                    onChange={() => setSelected(allSelected ? [] : findings.map((f) => f.id))}
                    style={{ accentColor: "#4F8CFF", flexShrink: 0 }}
                  />
                  {["ID", "Title", "CVE ID", "Severity", "Status", "SLA"].map((h) => (
                    <span key={h} style={{ color: "var(--color-text-muted, #6B7280)", fontSize: 11, fontWeight: 600, letterSpacing: 0.5, flex: h === "Title" ? 1 : undefined, width: h === "ID" ? 90 : h === "CVE ID" ? 120 : undefined, flexShrink: 0 }}>{h}</span>
                  ))}
                </div>
                <VirtualizedFindingsBody
                  findings={findings}
                  selected={selected}
                  toggleSelect={toggleSelect}
                  navigate={navigate}
                />
              </div>
            );
          }

          return (
            <div className="flex-1 overflow-y-auto">
              <table className="w-full">
                <thead style={{ position: "sticky", top: 0, background: "#0D1525", zIndex: 5 }}>
                  <tr style={{ borderBottom: "1px solid rgba(255,255,255,0.06)" }}>
                    <th className="px-4 py-3 text-left w-10">
                      <input
                        type="checkbox"
                        checked={allSelected}
                        onChange={() => setSelected(allSelected ? [] : findings.map((f) => f.id))}
                        style={{ accentColor: "#4F8CFF" }}
                      />
                    </th>
                    {["ID", "Title", "CVE", "Severity", "Product", "Asset", "EPSS", "Status", "SLA", "AI Triage", ""].map((h) => (
                      <th key={h} className="px-3 py-3 text-left" style={{ color: "var(--color-text-muted, #6B7280)", fontSize: 11, fontWeight: 600, letterSpacing: 0.5 }}>{h}</th>
                    ))}
                  </tr>
                </thead>
                <tbody>
                  {findings.map((f) => (
                    <tr
                      key={f.id}
                      className="transition-all cursor-pointer"
                      style={{ borderBottom: "1px solid rgba(255,255,255,0.04)", background: selected.includes(f.id) ? "rgba(79,140,255,0.05)" : "transparent" }}
                      onMouseEnter={(e) => { if (!selected.includes(f.id)) e.currentTarget.style.background = "rgba(255,255,255,0.02)"; }}
                      onMouseLeave={(e) => { if (!selected.includes(f.id)) e.currentTarget.style.background = "transparent"; }}
                    >
                      <td className="px-4 py-2.5">
                        <input type="checkbox" checked={selected.includes(f.id)} onChange={() => toggleSelect(f.id)} style={{ accentColor: "#4F8CFF" }} />
                      </td>
                      <td className="px-3 py-2.5"><span style={{ color: "var(--color-text-muted, #6B7280)", fontSize: 11 }}>{f.id}</span></td>
                      <td className="px-3 py-2.5" style={{ maxWidth: 220 }}>
                        <span
                          style={{ color: "var(--color-text-primary, #E5E7EB)", fontSize: 12 }}
                          className="line-clamp-1"
                          onClick={() => navigate(`/findings/${f.id}`)}
                        >{f.title}</span>
                      </td>
                      <td className="px-3 py-2.5"><span style={{ color: "var(--color-primary, #4F8CFF)", fontSize: 12, fontFamily: "monospace" }}>{f.cveId ?? "—"}</span></td>
                      <td className="px-3 py-2.5"><SeverityBadge severity={f.severity} /></td>
                      <td className="px-3 py-2.5"><span style={{ color: "var(--color-text-secondary, #9CA3AF)", fontSize: 12 }}>{f.productName ?? "—"}</span></td>
                      <td className="px-3 py-2.5"><span style={{ color: "var(--color-text-muted, #6B7280)", fontSize: 12, fontFamily: "monospace" }}>{f.assetIp ?? "—"}</span></td>
                      <td className="px-3 py-2.5">
                        <span style={{ color: (f.epssScore ?? 0) > 0.8 ? "#EF4444" : "#F59E0B", fontSize: 12, fontWeight: 600 }}>
                          {((f.epssScore ?? 0) * 100).toFixed(1)}%
                        </span>
                      </td>
                      <td className="px-3 py-2.5"><StatusBadge status={f.status} /></td>
                      <td className="px-3 py-2.5"><SLABadge status={f.slaStatus} daysLeft={f.slaDaysLeft} /></td>
                      <td className="px-3 py-2.5">
                        {f.aiTriageResult && (
                          <div className="flex items-center gap-1">
                            <Brain size={10} color={AI_COLORS[f.aiTriageResult.remarks] || "#6B7280"} />
                            <span style={{ color: AI_COLORS[f.aiTriageResult.remarks] || "#6B7280", fontSize: 11 }}>
                              {f.aiTriageResult.remarks}
                            </span>
                          </div>
                        )}
                      </td>
                      <td className="px-3 py-2.5">
                        <button
                          onClick={() => navigate(`/findings/${f.id}`)}
                          className="w-7 h-7 rounded-lg flex items-center justify-center"
                          style={{ background: "rgba(255,255,255,0.05)", color: "var(--color-text-muted, #6B7280)", border: "none", cursor: "pointer" }}
                        >
                          <Eye size={12} />
                        </button>
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          );
        }}
      </QueryBoundary>

      {/* Pagination */}
      <div className="flex items-center justify-between px-6 py-3" style={{ borderTop: "1px solid rgba(255,255,255,0.06)", background: "var(--color-bg-sidebar, #0F1629)" }}>
        <span style={{ color: "var(--color-text-muted, #6B7280)", fontSize: 12 }}>
          Showing {findingsQuery.data?.findings.length ?? 0} of {findingsQuery.data?.total ?? 0} findings
        </span>
        <div className="flex items-center gap-2">
          {[1, 2, 3, "...", 10].map((p, i) => (
            <button
              key={i}
              className="w-8 h-8 rounded-lg flex items-center justify-center"
              style={{ background: p === 1 ? "#4F8CFF" : "rgba(255,255,255,0.05)", color: p === 1 ? "white" : "#6B7280", border: "none", cursor: "pointer", fontSize: 12 }}
            >
              {p}
            </button>
          ))}
        </div>
      </div>
    </div>
  );
}
