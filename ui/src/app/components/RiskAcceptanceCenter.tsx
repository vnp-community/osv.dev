import { useState } from "react";
import { CheckCircle, XCircle, Clock, Plus, ChevronRight, AlertTriangle } from "lucide-react";

const acceptances = [
  { id: "RA-012", finding: "F-2841", title: "Kubernetes API Server Exposure", product: "DevOps Platform", reason: "Network controls (VPN-only access) mitigate the risk. No public exposure.", expiration: "Sep 14, 2026", owner: "Carol Anderson", status: "approved", severity: "High", daysLeft: 92 },
  { id: "RA-011", finding: "F-2839", title: "WordPress XSS in Comment Handler", product: "Marketing Site", reason: "Marketing site has no sensitive user data. Risk impact classified as Low.", expiration: "Aug 14, 2026", owner: "Alice Wu", status: "approved", severity: "Low", daysLeft: 61 },
  { id: "RA-010", finding: "F-2835", title: "OpenSSL CVE in Legacy System", product: "Data Pipeline", reason: "Legacy system scheduled for decommission Q3 2026. Patching would cause downtime.", expiration: "Aug 1, 2026", owner: "Bob Chen", status: "pending", severity: "High", daysLeft: 48 },
  { id: "RA-009", finding: "F-2830", title: "Redis Weak Authentication", product: "API Gateway", reason: "Redis is internal-only, bound to localhost. No external access possible.", expiration: "Jul 14, 2026", owner: "Dave Kim", status: "pending", severity: "Medium", daysLeft: 30 },
  { id: "RA-008", finding: "F-2825", title: "FTP Service Exposed", product: "Dev Environment", reason: "Dev environment only. No production data. Will be removed next sprint.", expiration: "Jun 28, 2026", owner: "Bob Chen", status: "expired", severity: "Medium", daysLeft: -16 },
];

const STATUS_STYLES: Record<string, { bg: string; color: string }> = {
  approved: { bg: "rgba(16,185,129,0.1)", color: "#10B981" },
  pending: { bg: "rgba(245,158,11,0.1)", color: "#F59E0B" },
  rejected: { bg: "rgba(239,68,68,0.1)", color: "#EF4444" },
  expired: { bg: "rgba(107,114,128,0.1)", color: "#6B7280" },
};
const SEVERITY_COLORS: Record<string, string> = { Critical: "#EF4444", High: "#F97316", Medium: "#EAB308", Low: "#3B82F6" };

export function RiskAcceptanceCenter() {
  const [selected, setSelected] = useState(acceptances[0]);
  const [filter, setFilter] = useState("All");

  const filtered = acceptances.filter(a => filter === "All" || a.status === filter.toLowerCase());

  return (
    <div className="flex flex-col flex-1 overflow-hidden" style={{ background: "#0B1020" }}>
      {/* Header */}
      <div className="px-6 py-4" style={{ borderBottom: "1px solid rgba(255,255,255,0.06)" }}>
        <div className="flex items-center justify-between mb-3">
          <div>
            <h2 style={{ color: "#E5E7EB", fontSize: 18, fontWeight: 700 }}>Risk Acceptance Center</h2>
            <p style={{ color: "#6B7280", fontSize: 12 }}>Manage accepted risks and formal exceptions</p>
          </div>
          <button className="flex items-center gap-2 px-4 py-2 rounded-xl" style={{ background: "linear-gradient(135deg,#4F8CFF,#3B6FCC)", color: "white", border: "none", fontSize: 13, cursor: "pointer" }}>
            <Plus size={14} />New Acceptance
          </button>
        </div>
        <div className="flex gap-2">
          {["All", "Pending", "Approved", "Expired"].map(f => (
            <button key={f} onClick={() => setFilter(f)} className="px-3 py-1.5 rounded-lg" style={{ background: filter === f ? "rgba(79,140,255,0.12)" : "rgba(255,255,255,0.05)", color: filter === f ? "#4F8CFF" : "#6B7280", fontSize: 12, border: "none", cursor: "pointer" }}>{f}</button>
          ))}
        </div>
      </div>

      <div className="flex flex-1 overflow-hidden">
        {/* Table */}
        <div className="flex-1 overflow-y-auto">
          <table className="w-full">
            <thead style={{ position: "sticky", top: 0, background: "#0D1525", zIndex: 5 }}>
              <tr style={{ borderBottom: "1px solid rgba(255,255,255,0.06)" }}>
                {["ID", "Finding", "Product", "Severity", "Expiration", "Owner", "Status", ""].map(h => (
                  <th key={h} className="px-4 py-3 text-left" style={{ color: "#6B7280", fontSize: 11, fontWeight: 600, letterSpacing: 0.5 }}>{h}</th>
                ))}
              </tr>
            </thead>
            <tbody>
              {filtered.map(a => (
                <tr key={a.id} onClick={() => setSelected(a)} className="cursor-pointer transition-all"
                  style={{ borderBottom: "1px solid rgba(255,255,255,0.04)", background: selected.id === a.id ? "rgba(79,140,255,0.07)" : "transparent", borderLeft: selected.id === a.id ? "2px solid #4F8CFF" : "2px solid transparent" }}
                  onMouseEnter={e => { if (selected.id !== a.id) e.currentTarget.style.background = "rgba(255,255,255,0.02)"; }}
                  onMouseLeave={e => { if (selected.id !== a.id) e.currentTarget.style.background = "transparent"; }}
                >
                  <td className="px-4 py-3"><span style={{ color: "#6B7280", fontSize: 12 }}>{a.id}</span></td>
                  <td className="px-4 py-3">
                    <div><span style={{ color: "#4F8CFF", fontSize: 11 }}>{a.finding}</span></div>
                    <div style={{ color: "#E5E7EB", fontSize: 12 }} className="truncate max-w-xs">{a.title}</div>
                  </td>
                  <td className="px-4 py-3"><span style={{ color: "#9CA3AF", fontSize: 12 }}>{a.product}</span></td>
                  <td className="px-4 py-3"><span className="px-2 py-0.5 rounded" style={{ background: SEVERITY_COLORS[a.severity] + "20", color: SEVERITY_COLORS[a.severity], fontSize: 11 }}>{a.severity}</span></td>
                  <td className="px-4 py-3"><span style={{ color: a.daysLeft < 0 ? "#EF4444" : a.daysLeft < 30 ? "#F59E0B" : "#6B7280", fontSize: 12 }}>{a.expiration}</span></td>
                  <td className="px-4 py-3"><span style={{ color: "#9CA3AF", fontSize: 12 }}>{a.owner}</span></td>
                  <td className="px-4 py-3"><span className="px-2 py-0.5 rounded" style={{ ...STATUS_STYLES[a.status], fontSize: 11 }}>{a.status}</span></td>
                  <td className="px-4 py-3">
                    {a.status === "pending" && (
                      <div className="flex gap-1.5">
                        <button className="w-7 h-7 rounded-lg flex items-center justify-center" style={{ background: "rgba(16,185,129,0.1)", color: "#10B981", border: "none", cursor: "pointer" }}><CheckCircle size={12} /></button>
                        <button className="w-7 h-7 rounded-lg flex items-center justify-center" style={{ background: "rgba(239,68,68,0.1)", color: "#EF4444", border: "none", cursor: "pointer" }}><XCircle size={12} /></button>
                        <button className="w-7 h-7 rounded-lg flex items-center justify-center" style={{ background: "rgba(79,140,255,0.1)", color: "#4F8CFF", border: "none", cursor: "pointer" }}><Clock size={12} /></button>
                      </div>
                    )}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>

        {/* Detail panel */}
        {selected && (
          <div className="w-80 flex-shrink-0 overflow-y-auto" style={{ background: "#0F1629", borderLeft: "1px solid rgba(255,255,255,0.06)" }}>
            <div className="p-5" style={{ borderBottom: "1px solid rgba(255,255,255,0.06)" }}>
              <div style={{ color: "#6B7280", fontSize: 11, marginBottom: 4 }}>{selected.id}</div>
              <div style={{ color: "#E5E7EB", fontSize: 14, fontWeight: 600, marginBottom: 6 }}>{selected.title}</div>
              <div className="flex gap-2">
                <span className="px-2 py-0.5 rounded" style={{ background: SEVERITY_COLORS[selected.severity] + "20", color: SEVERITY_COLORS[selected.severity], fontSize: 11 }}>{selected.severity}</span>
                <span className="px-2 py-0.5 rounded" style={{ ...STATUS_STYLES[selected.status], fontSize: 11 }}>{selected.status}</span>
              </div>
            </div>
            <div className="p-4" style={{ borderBottom: "1px solid rgba(255,255,255,0.06)" }}>
              <div style={{ color: "#9CA3AF", fontSize: 11, fontWeight: 600, marginBottom: 8 }}>BUSINESS JUSTIFICATION</div>
              <div className="rounded-xl p-3" style={{ background: "rgba(255,255,255,0.04)" }}>
                <p style={{ color: "#E5E7EB", fontSize: 12, lineHeight: 1.6 }}>{selected.reason}</p>
              </div>
            </div>
            <div className="p-4">
              {[{ label: "Finding", value: selected.finding }, { label: "Product", value: selected.product }, { label: "Owner", value: selected.owner }, { label: "Expires", value: selected.expiration }, { label: "Days Left", value: selected.daysLeft < 0 ? `${Math.abs(selected.daysLeft)} days overdue` : `${selected.daysLeft} days` }].map(({ label, value }) => (
                <div key={label} className="flex justify-between py-2" style={{ borderBottom: "1px solid rgba(255,255,255,0.04)" }}>
                  <span style={{ color: "#6B7280", fontSize: 12 }}>{label}</span>
                  <span style={{ color: label === "Days Left" && selected.daysLeft < 0 ? "#EF4444" : "#E5E7EB", fontSize: 12 }}>{value}</span>
                </div>
              ))}
            </div>
          </div>
        )}
      </div>
    </div>
  );
}
