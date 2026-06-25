import { useState } from "react";
import { Search, Filter, Download, Shield, User, Settings, AlertTriangle } from "lucide-react";

const auditLogs = [
  { id: "AL-1001", timestamp: "2026-06-14 09:42:15", user: "carol@company.com", action: "CREATE_SCAN", resource: "Scan / SC-0047", severity: "Info", before: null, after: '{ "type": "NMAP", "target": "10.0.0.0/16" }' },
  { id: "AL-1002", timestamp: "2026-06-14 09:35:01", user: "bob.chen@company.com", action: "UPDATE_FINDING", resource: "Finding / F-2847", severity: "Warning", before: '{ "status": "New" }', after: '{ "status": "Active" }' },
  { id: "AL-1003", timestamp: "2026-06-14 09:20:44", user: "alice.wu@company.com", action: "ASSIGN_FINDING", resource: "Finding / F-2847", severity: "Info", before: '{ "assignee": null }', after: '{ "assignee": "bob.chen" }' },
  { id: "AL-1004", timestamp: "2026-06-14 08:55:12", user: "carol@company.com", action: "CREATE_API_KEY", resource: "API Key / k-005", severity: "Warning", before: null, after: '{ "name": "New Integration", "scope": ["scan:read"] }' },
  { id: "AL-1005", timestamp: "2026-06-14 08:30:00", user: "system", action: "AUTO_SCAN_START", resource: "Scan / SC-0046", severity: "Info", before: null, after: '{ "trigger": "schedule", "cron": "0 2 * * *" }' },
  { id: "AL-1006", timestamp: "2026-06-14 07:15:33", user: "dave.kim@company.com", action: "ACCEPT_RISK", resource: "Finding / F-2841", severity: "Critical", before: '{ "status": "Active" }', after: '{ "status": "Risk Accepted", "reason": "Network controls mitigate" }' },
  { id: "AL-1007", timestamp: "2026-06-14 06:02:18", user: "carol@company.com", action: "USER_DISABLED", resource: "User / frank.l", severity: "Warning", before: '{ "status": "active" }', after: '{ "status": "disabled" }' },
  { id: "AL-1008", timestamp: "2026-06-13 22:00:05", user: "system", action: "SCAN_COMPLETED", resource: "Scan / SC-0045", severity: "Info", before: null, after: '{ "findings": 8, "duration": "18m 32s" }' },
  { id: "AL-1009", timestamp: "2026-06-13 18:45:00", user: "bob.chen@company.com", action: "MARK_FALSE_POSITIVE", resource: "Finding / F-2839", severity: "Warning", before: '{ "status": "Active" }', after: '{ "status": "False Positive" }' },
  { id: "AL-1010", timestamp: "2026-06-13 14:30:22", user: "carol@company.com", action: "UPDATE_SYSTEM_SETTINGS", resource: "Settings / email", severity: "Critical", before: '{ "smtp": "old-smtp.company.com" }', after: '{ "smtp": "smtp.company.com" }' },
];

const SEVERITY_STYLES: Record<string, { bg: string; color: string }> = {
  Info: { bg: "rgba(79,140,255,0.1)", color: "#4F8CFF" },
  Warning: { bg: "rgba(245,158,11,0.1)", color: "#F59E0B" },
  Critical: { bg: "rgba(239,68,68,0.1)", color: "#EF4444" },
};

const ACTION_ICONS: Record<string, React.ElementType> = {
  CREATE_SCAN: Settings, UPDATE_FINDING: AlertTriangle, ASSIGN_FINDING: User,
  CREATE_API_KEY: Shield, AUTO_SCAN_START: Settings, ACCEPT_RISK: Shield,
  USER_DISABLED: User, SCAN_COMPLETED: Settings, MARK_FALSE_POSITIVE: AlertTriangle,
  UPDATE_SYSTEM_SETTINGS: Settings,
};

export function AuditLogs() {
  const [search, setSearch] = useState("");
  const [filterSeverity, setFilterSeverity] = useState("All");
  const [expanded, setExpanded] = useState<string | null>(null);

  const filtered = auditLogs.filter((l) => {
    if (filterSeverity !== "All" && l.severity !== filterSeverity) return false;
    if (search && !l.action.toLowerCase().includes(search.toLowerCase()) && !l.user.toLowerCase().includes(search.toLowerCase()) && !l.resource.toLowerCase().includes(search.toLowerCase())) return false;
    return true;
  });

  return (
    <div className="flex-1 overflow-y-auto px-6 py-5" style={{ background: "#0B1020" }}>
      <div className="flex items-center justify-between mb-5">
        <div>
          <h2 style={{ color: "#E5E7EB", fontSize: 18, fontWeight: 700 }}>Audit Logs</h2>
          <p style={{ color: "#6B7280", fontSize: 12 }}>Immutable audit trail · All user and system actions</p>
        </div>
        <button className="flex items-center gap-2 px-4 py-2 rounded-xl" style={{ background: "rgba(255,255,255,0.05)", border: "1px solid rgba(255,255,255,0.09)", color: "#9CA3AF", fontSize: 13, cursor: "pointer" }}>
          <Download size={14} />Export CSV
        </button>
      </div>

      <div className="flex items-center gap-3 mb-5">
        <div className="relative">
          <Search size={13} color="#4B5563" style={{ position: "absolute", left: 10, top: "50%", transform: "translateY(-50%)" }} />
          <input value={search} onChange={(e) => setSearch(e.target.value)} placeholder="Search actions, users..." className="rounded-xl pl-8 pr-4 py-2 outline-none" style={{ background: "#151B2F", border: "1px solid rgba(255,255,255,0.08)", color: "#E5E7EB", fontSize: 12, width: 240 }} />
        </div>
        {["All", "Info", "Warning", "Critical"].map((s) => (
          <button
            key={s}
            onClick={() => setFilterSeverity(s)}
            className="px-3 py-2 rounded-lg"
            style={{ background: filterSeverity === s ? (s === "All" ? "rgba(79,140,255,0.12)" : SEVERITY_STYLES[s]?.bg || "rgba(79,140,255,0.12)") : "rgba(255,255,255,0.05)", color: filterSeverity === s ? (s === "All" ? "#4F8CFF" : SEVERITY_STYLES[s]?.color || "#4F8CFF") : "#6B7280", fontSize: 12, border: "none", cursor: "pointer" }}
          >
            {s}
          </button>
        ))}
        <span style={{ color: "#6B7280", fontSize: 12, marginLeft: "auto" }}>{filtered.length} events</span>
      </div>

      <div className="rounded-2xl overflow-hidden" style={{ background: "#151B2F", border: "1px solid rgba(255,255,255,0.07)" }}>
        <table className="w-full">
          <thead>
            <tr style={{ borderBottom: "1px solid rgba(255,255,255,0.06)" }}>
              {["Timestamp", "User", "Action", "Resource", "Severity", ""].map((h) => (
                <th key={h} className="px-4 py-3 text-left" style={{ color: "#6B7280", fontSize: 11, fontWeight: 600, letterSpacing: 0.5 }}>{h}</th>
              ))}
            </tr>
          </thead>
          <tbody>
            {filtered.map((log, i) => {
              const Icon = ACTION_ICONS[log.action] || Settings;
              const isExpanded = expanded === log.id;
              return (
                <>
                  <tr
                    key={log.id}
                    className="cursor-pointer transition-all"
                    style={{ borderBottom: "1px solid rgba(255,255,255,0.04)", background: isExpanded ? "rgba(79,140,255,0.04)" : "transparent" }}
                    onClick={() => setExpanded(isExpanded ? null : log.id)}
                    onMouseEnter={(e) => { if (!isExpanded) e.currentTarget.style.background = "rgba(255,255,255,0.02)"; }}
                    onMouseLeave={(e) => { if (!isExpanded) e.currentTarget.style.background = "transparent"; }}
                  >
                    <td className="px-4 py-3"><span style={{ color: "#6B7280", fontSize: 11, fontFamily: "monospace" }}>{log.timestamp}</span></td>
                    <td className="px-4 py-3">
                      <div className="flex items-center gap-2">
                        <div className="w-5 h-5 rounded-full flex items-center justify-center" style={{ background: log.user === "system" ? "#374151" : "rgba(79,140,255,0.3)", fontSize: 9, color: "white", fontWeight: 700 }}>
                          {log.user === "system" ? "SYS" : log.user.split("@")[0].slice(0, 2).toUpperCase()}
                        </div>
                        <span style={{ color: "#9CA3AF", fontSize: 12 }}>{log.user}</span>
                      </div>
                    </td>
                    <td className="px-4 py-3">
                      <div className="flex items-center gap-2">
                        <Icon size={12} color="#6B7280" />
                        <span style={{ color: "#E5E7EB", fontSize: 12, fontFamily: "monospace" }}>{log.action}</span>
                      </div>
                    </td>
                    <td className="px-4 py-3"><span style={{ color: "#6B7280", fontSize: 12 }}>{log.resource}</span></td>
                    <td className="px-4 py-3">
                      <span className="px-2 py-0.5 rounded" style={{ ...SEVERITY_STYLES[log.severity], fontSize: 11 }}>{log.severity}</span>
                    </td>
                    <td className="px-4 py-3"><span style={{ color: "#4B5563", fontSize: 11 }}>{isExpanded ? "▲" : "▼"}</span></td>
                  </tr>
                  {isExpanded && (
                    <tr key={`${log.id}-detail`} style={{ borderBottom: "1px solid rgba(255,255,255,0.04)" }}>
                      <td colSpan={6} className="px-4 py-3">
                        <div className="grid grid-cols-2 gap-4">
                          {log.before && (
                            <div className="rounded-xl p-3" style={{ background: "rgba(239,68,68,0.06)", border: "1px solid rgba(239,68,68,0.15)" }}>
                              <div style={{ color: "#EF4444", fontSize: 10, fontWeight: 600, marginBottom: 6 }}>BEFORE</div>
                              <pre style={{ color: "#FCA5A5", fontSize: 11, fontFamily: "monospace" }}>{JSON.stringify(JSON.parse(log.before), null, 2)}</pre>
                            </div>
                          )}
                          {log.after && (
                            <div className="rounded-xl p-3" style={{ background: "rgba(16,185,129,0.06)", border: "1px solid rgba(16,185,129,0.15)" }}>
                              <div style={{ color: "#10B981", fontSize: 10, fontWeight: 600, marginBottom: 6 }}>AFTER</div>
                              <pre style={{ color: "#A7F3D0", fontSize: 11, fontFamily: "monospace" }}>{JSON.stringify(JSON.parse(log.after), null, 2)}</pre>
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
      </div>
    </div>
  );
}
