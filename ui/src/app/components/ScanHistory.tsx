import { useState } from "react";
import { Search, Eye, Download, Copy, Calendar, Filter } from "lucide-react";

const history = [
  { id: "SC-0047", name: "Production Network Sweep", target: "10.0.0.0/16", type: "NMAP", duration: "00:24:15", findings: 47, status: "completed", user: "carol@", date: "Jun 14, 09:18 AM" },
  { id: "SC-0046", name: "Dev Environment Scan", target: "192.168.1.0/24", type: "NMAP", duration: "00:18:32", findings: 12, status: "completed", user: "bob.chen@", date: "Jun 14, 08:55 AM" },
  { id: "SC-0045", name: "Staging Network", target: "172.16.0.0/24", type: "NMAP", duration: "00:18:32", findings: 8, status: "completed", user: "bob.chen@", date: "Jun 13, 08:00 PM" },
  { id: "SC-0044", name: "Web App Pentest", target: "https://app.company.com", type: "ZAP", duration: "01:12:18", findings: 34, status: "completed", user: "alice.wu@", date: "Jun 13, 02:00 PM" },
  { id: "SC-0043", name: "Agent Scan - DC01", target: "dc01.internal", type: "AGENT", duration: "00:08:05", findings: 5, status: "completed", user: "system", date: "Jun 13, 10:00 AM" },
  { id: "SC-0042", name: "Database Servers", target: "10.0.5.0/24", type: "NMAP", duration: "—", findings: 0, status: "failed", user: "carol@", date: "Jun 12, 11:00 PM" },
  { id: "SC-0041", name: "Production Network Sweep", target: "10.0.0.0/16", type: "NMAP", duration: "00:31:42", findings: 52, status: "completed", user: "system", date: "Jun 7, 02:00 AM" },
  { id: "SC-0040", name: "API Security Scan", target: "https://api.company.com", type: "ZAP", duration: "00:55:21", findings: 28, status: "completed", user: "alice.wu@", date: "Jun 5, 03:00 PM" },
];

const TYPE_COLORS: Record<string, string> = { NMAP: "#4F8CFF", ZAP: "#F97316", AGENT: "#10B981" };

export function ScanHistory({ onViewScan }: { onViewScan?: () => void }) {
  const [search, setSearch] = useState("");
  const [filterType, setFilterType] = useState("All");
  const [filterStatus, setFilterStatus] = useState("All");

  const filtered = history.filter(s =>
    (filterType === "All" || s.type === filterType) &&
    (filterStatus === "All" || s.status === filterStatus) &&
    (!search || s.name.toLowerCase().includes(search.toLowerCase()) || s.target.toLowerCase().includes(search.toLowerCase()))
  );

  return (
    <div className="flex-1 overflow-y-auto px-6 py-5" style={{ background: "#0B1020" }}>
      <div className="flex items-center justify-between mb-5">
        <div>
          <h2 style={{ color: "#E5E7EB", fontSize: 18, fontWeight: 700 }}>Scan History</h2>
          <p style={{ color: "#6B7280", fontSize: 12 }}>{history.length} scans · All time</p>
        </div>
        <button className="flex items-center gap-2 px-4 py-2 rounded-xl" style={{ background: "rgba(255,255,255,0.05)", border: "1px solid rgba(255,255,255,0.09)", color: "#9CA3AF", fontSize: 13, cursor: "pointer" }}>
          <Download size={14} />Export CSV
        </button>
      </div>

      {/* Filters */}
      <div className="flex items-center gap-3 mb-4">
        <div className="relative">
          <Search size={13} color="#4B5563" style={{ position: "absolute", left: 10, top: "50%", transform: "translateY(-50%)" }} />
          <input value={search} onChange={e => setSearch(e.target.value)} placeholder="Search scans..." className="rounded-xl pl-8 pr-4 py-2 outline-none" style={{ background: "#151B2F", border: "1px solid rgba(255,255,255,0.08)", color: "#E5E7EB", fontSize: 12, width: 200 }} />
        </div>
        {["All", "NMAP", "ZAP", "AGENT"].map(t => (
          <button key={t} onClick={() => setFilterType(t)} className="px-3 py-1.5 rounded-lg" style={{ background: filterType === t ? TYPE_COLORS[t] ? TYPE_COLORS[t] + "20" : "rgba(79,140,255,0.12)" : "rgba(255,255,255,0.05)", color: filterType === t ? TYPE_COLORS[t] || "#4F8CFF" : "#6B7280", fontSize: 12, border: "none", cursor: "pointer" }}>{t}</button>
        ))}
        <select value={filterStatus} onChange={e => setFilterStatus(e.target.value)} className="rounded-xl px-3 py-2 outline-none" style={{ background: "#151B2F", border: "1px solid rgba(255,255,255,0.08)", color: "#9CA3AF", fontSize: 12 }}>
          <option>All</option><option value="completed">Completed</option><option value="failed">Failed</option>
        </select>
      </div>

      <div className="rounded-2xl" style={{ background: "#151B2F", border: "1px solid rgba(255,255,255,0.07)" }}>
        <table className="w-full">
          <thead>
            <tr style={{ borderBottom: "1px solid rgba(255,255,255,0.06)" }}>
              {["Scan ID", "Name", "Target", "Type", "Date", "Duration", "Findings", "Status", ""].map(h => (
                <th key={h} className="px-4 py-3 text-left" style={{ color: "#6B7280", fontSize: 11, fontWeight: 600, letterSpacing: 0.5 }}>{h}</th>
              ))}
            </tr>
          </thead>
          <tbody>
            {filtered.map((s, i) => (
              <tr key={s.id} className="transition-all" style={{ borderBottom: i < filtered.length - 1 ? "1px solid rgba(255,255,255,0.04)" : "none" }}
                onMouseEnter={e => (e.currentTarget.style.background = "rgba(255,255,255,0.02)")}
                onMouseLeave={e => (e.currentTarget.style.background = "transparent")}
              >
                <td className="px-4 py-3"><span style={{ color: "#4F8CFF", fontSize: 12, fontFamily: "monospace" }}>{s.id}</span></td>
                <td className="px-4 py-3"><span style={{ color: "#E5E7EB", fontSize: 12 }}>{s.name}</span></td>
                <td className="px-4 py-3"><span style={{ color: "#6B7280", fontSize: 11, fontFamily: "monospace" }}>{s.target}</span></td>
                <td className="px-4 py-3"><span className="px-2 py-0.5 rounded" style={{ background: TYPE_COLORS[s.type] + "20", color: TYPE_COLORS[s.type], fontSize: 11 }}>{s.type}</span></td>
                <td className="px-4 py-3"><span style={{ color: "#6B7280", fontSize: 12 }}>{s.date}</span></td>
                <td className="px-4 py-3"><span style={{ color: "#9CA3AF", fontSize: 12, fontFamily: "monospace" }}>{s.duration}</span></td>
                <td className="px-4 py-3"><span style={{ color: s.findings > 20 ? "#EF4444" : s.findings > 0 ? "#F59E0B" : "#10B981", fontSize: 12, fontWeight: 600 }}>{s.findings}</span></td>
                <td className="px-4 py-3">
                  <span className="px-2 py-0.5 rounded" style={{ background: s.status === "completed" ? "rgba(16,185,129,0.1)" : "rgba(239,68,68,0.1)", color: s.status === "completed" ? "#10B981" : "#EF4444", fontSize: 11 }}>{s.status}</span>
                </td>
                <td className="px-4 py-3">
                  <div className="flex gap-1.5">
                    <button onClick={onViewScan} className="w-7 h-7 rounded-lg flex items-center justify-center" style={{ background: "rgba(79,140,255,0.1)", color: "#4F8CFF", border: "none", cursor: "pointer" }}><Eye size={12} /></button>
                    <button className="w-7 h-7 rounded-lg flex items-center justify-center" style={{ background: "rgba(255,255,255,0.05)", color: "#6B7280", border: "none", cursor: "pointer" }}><Copy size={12} /></button>
                    <button className="w-7 h-7 rounded-lg flex items-center justify-center" style={{ background: "rgba(255,255,255,0.05)", color: "#6B7280", border: "none", cursor: "pointer" }}><Download size={12} /></button>
                  </div>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </div>
  );
}
