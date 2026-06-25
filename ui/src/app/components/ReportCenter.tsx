import { useState } from "react";
import { FileText, Download, Eye, Plus, Clock, CheckCircle, Loader, AlertTriangle, BarChart2, Shield } from "lucide-react";

const templates = [
  { id: "exec", name: "Executive Summary", desc: "High-level overview for C-level and board presentations", icon: Shield, color: "#4F8CFF" },
  { id: "tech", name: "Technical Report", desc: "Detailed findings with CVE details, evidence, and remediation steps", icon: FileText, color: "#10B981" },
  { id: "comp", name: "Compliance Report", desc: "Mapped to PCI DSS, ISO 27001, SOC2, NIST frameworks", icon: BarChart2, color: "#A78BFA" },
];

const reports = [
  { id: "R-047", name: "Q2 2026 Executive Summary", type: "Executive", created: "Jun 14, 09:00 AM", status: "ready", size: "2.4 MB" },
  { id: "R-046", name: "Banking App Technical Report", type: "Technical", created: "Jun 13, 04:30 PM", status: "ready", size: "8.7 MB" },
  { id: "R-045", name: "PCI DSS Compliance Q2", type: "Compliance", created: "Jun 12, 11:00 AM", status: "ready", size: "4.1 MB" },
  { id: "R-044", name: "API Gateway Security Report", type: "Technical", created: "Jun 10, 02:00 PM", status: "ready", size: "5.8 MB" },
  { id: "R-043", name: "May 2026 Executive Summary", type: "Executive", created: "Jun 1, 09:00 AM", status: "ready", size: "2.1 MB" },
];

const TYPE_COLORS: Record<string, string> = {
  Executive: "#4F8CFF",
  Technical: "#10B981",
  Compliance: "#A78BFA",
};

export function ReportCenter() {
  const [step, setStep] = useState<"list" | "generate">("list");
  const [selectedTemplate, setSelectedTemplate] = useState("");
  const [generating, setGenerating] = useState(false);
  const [generated, setGenerated] = useState(false);

  const handleGenerate = () => {
    setGenerating(true);
    setTimeout(() => {
      setGenerating(false);
      setGenerated(true);
    }, 2000);
  };

  if (step === "generate") {
    return (
      <div className="flex-1 overflow-y-auto px-6 py-5" style={{ background: "#0B1020" }}>
        <div className="max-w-2xl mx-auto">
          <div className="flex items-center gap-3 mb-6">
            <button onClick={() => { setStep("list"); setGenerated(false); }} className="px-4 py-2 rounded-xl" style={{ background: "rgba(255,255,255,0.05)", border: "1px solid rgba(255,255,255,0.09)", color: "#9CA3AF", fontSize: 13, cursor: "pointer" }}>← Back</button>
            <h2 style={{ color: "#E5E7EB", fontSize: 18, fontWeight: 700 }}>Generate Report</h2>
          </div>

          {/* Template selection */}
          <div className="rounded-2xl p-5 mb-5" style={{ background: "#151B2F", border: "1px solid rgba(255,255,255,0.07)" }}>
            <div style={{ color: "#9CA3AF", fontSize: 12, fontWeight: 600, marginBottom: 12 }}>SELECT TEMPLATE</div>
            <div className="grid grid-cols-3 gap-3">
              {templates.map((t) => {
                const Icon = t.icon;
                return (
                  <div
                    key={t.id}
                    onClick={() => setSelectedTemplate(t.id)}
                    className="rounded-xl p-4 cursor-pointer transition-all"
                    style={{
                      background: selectedTemplate === t.id ? `${t.color}12` : "rgba(255,255,255,0.04)",
                      border: selectedTemplate === t.id ? `2px solid ${t.color}` : "2px solid rgba(255,255,255,0.07)",
                    }}
                  >
                    <Icon size={20} color={t.color} style={{ marginBottom: 8 }} />
                    <div style={{ color: "#E5E7EB", fontSize: 13, fontWeight: 500 }}>{t.name}</div>
                    <div style={{ color: "#6B7280", fontSize: 11, marginTop: 4 }}>{t.desc}</div>
                  </div>
                );
              })}
            </div>
          </div>

          {/* Filters */}
          <div className="rounded-2xl p-5 mb-5" style={{ background: "#151B2F", border: "1px solid rgba(255,255,255,0.07)" }}>
            <div style={{ color: "#9CA3AF", fontSize: 12, fontWeight: 600, marginBottom: 12 }}>REPORT FILTERS</div>
            <div className="grid grid-cols-2 gap-4">
              {[
                { label: "Date Range", options: ["Last 30 days", "Last Quarter", "Last 6 months", "Custom"] },
                { label: "Product", options: ["All Products", "Banking App", "Mobile App", "API Gateway"] },
                { label: "Severity", options: ["All Severities", "Critical Only", "Critical + High", "All except Low"] },
                { label: "Status", options: ["All Statuses", "Active Only", "Include Mitigated", "All"] },
              ].map(({ label, options }) => (
                <div key={label}>
                  <label style={{ color: "#9CA3AF", fontSize: 12, display: "block", marginBottom: 6 }}>{label}</label>
                  <select className="w-full rounded-xl px-3 py-2.5 outline-none" style={{ background: "#0F1629", border: "1px solid rgba(255,255,255,0.08)", color: "#E5E7EB", fontSize: 13 }}>
                    {options.map((o) => <option key={o}>{o}</option>)}
                  </select>
                </div>
              ))}
            </div>
          </div>

          {/* Generate button */}
          {!generated && (
            <button
              onClick={handleGenerate}
              disabled={!selectedTemplate || generating}
              className="w-full py-3 rounded-xl flex items-center justify-center gap-2"
              style={{
                background: !selectedTemplate ? "rgba(79,140,255,0.3)" : "linear-gradient(135deg, #4F8CFF, #3B6FCC)",
                color: "white", border: "none", fontSize: 14, fontWeight: 600, cursor: !selectedTemplate ? "not-allowed" : "pointer",
              }}
            >
              {generating ? (
                <>
                  <div className="w-4 h-4 rounded-full border-2 border-white/30 border-t-white animate-spin" />
                  Generating Report...
                </>
              ) : (
                <><FileText size={16} />Generate Report</>
              )}
            </button>
          )}

          {generated && (
            <div className="rounded-2xl p-5" style={{ background: "rgba(16,185,129,0.08)", border: "1px solid rgba(16,185,129,0.25)" }}>
              <div className="flex items-center gap-3 mb-4">
                <CheckCircle size={20} color="#10B981" />
                <span style={{ color: "#10B981", fontSize: 15, fontWeight: 600 }}>Report generated successfully!</span>
              </div>
              <div className="flex gap-3">
                {[
                  { label: "Download PDF", color: "#EF4444", bg: "rgba(239,68,68,0.1)" },
                  { label: "Download HTML", color: "#4F8CFF", bg: "rgba(79,140,255,0.1)" },
                  { label: "Download Excel", color: "#10B981", bg: "rgba(16,185,129,0.1)" },
                ].map(({ label, color, bg }) => (
                  <button
                    key={label}
                    className="flex items-center gap-2 px-4 py-2 rounded-xl"
                    style={{ background: bg, color, border: "none", fontSize: 13, cursor: "pointer" }}
                  >
                    <Download size={13} />{label}
                  </button>
                ))}
              </div>
            </div>
          )}
        </div>
      </div>
    );
  }

  return (
    <div className="flex-1 overflow-y-auto px-6 py-5" style={{ background: "#0B1020" }}>
      {/* Header */}
      <div className="flex items-center justify-between mb-6">
        <div>
          <h2 style={{ color: "#E5E7EB", fontSize: 18, fontWeight: 700 }}>Report Center</h2>
          <p style={{ color: "#6B7280", fontSize: 12 }}>{reports.length} reports generated · Last report 6h ago</p>
        </div>
        <button
          onClick={() => setStep("generate")}
          className="flex items-center gap-2 px-5 py-2.5 rounded-xl"
          style={{ background: "linear-gradient(135deg, #4F8CFF, #3B6FCC)", color: "white", border: "none", fontSize: 13, cursor: "pointer" }}
        >
          <Plus size={15} />Generate Report
        </button>
      </div>

      {/* Templates row */}
      <div className="grid grid-cols-3 gap-4 mb-6">
        {templates.map((t) => {
          const Icon = t.icon;
          return (
            <div
              key={t.id}
              onClick={() => { setSelectedTemplate(t.id); setStep("generate"); }}
              className="rounded-2xl p-5 cursor-pointer transition-all"
              style={{ background: "#151B2F", border: "1px solid rgba(255,255,255,0.07)" }}
              onMouseEnter={(e) => (e.currentTarget.style.borderColor = t.color + "60")}
              onMouseLeave={(e) => (e.currentTarget.style.borderColor = "rgba(255,255,255,0.07)")}
            >
              <div className="w-10 h-10 rounded-xl flex items-center justify-center mb-3" style={{ background: `${t.color}20` }}>
                <Icon size={20} color={t.color} />
              </div>
              <div style={{ color: "#E5E7EB", fontSize: 14, fontWeight: 600 }}>{t.name}</div>
              <div style={{ color: "#6B7280", fontSize: 12, marginTop: 4 }}>{t.desc}</div>
            </div>
          );
        })}
      </div>

      {/* Reports table */}
      <div className="rounded-2xl" style={{ background: "#151B2F", border: "1px solid rgba(255,255,255,0.07)" }}>
        <div className="px-5 py-4" style={{ borderBottom: "1px solid rgba(255,255,255,0.06)" }}>
          <h3 style={{ color: "#E5E7EB", fontSize: 14, fontWeight: 600 }}>Generated Reports</h3>
        </div>
        <table className="w-full">
          <thead>
            <tr style={{ borderBottom: "1px solid rgba(255,255,255,0.05)" }}>
              {["ID", "Name", "Type", "Created", "Size", "Status", ""].map((h) => (
                <th key={h} className="px-5 py-3 text-left" style={{ color: "#6B7280", fontSize: 11, fontWeight: 600, letterSpacing: 0.5 }}>{h}</th>
              ))}
            </tr>
          </thead>
          <tbody>
            {reports.map((r, i) => (
              <tr key={r.id} className="transition-all" style={{ borderBottom: i < reports.length - 1 ? "1px solid rgba(255,255,255,0.04)" : "none" }}
                onMouseEnter={(e) => (e.currentTarget.style.background = "rgba(255,255,255,0.02)")}
                onMouseLeave={(e) => (e.currentTarget.style.background = "transparent")}
              >
                <td className="px-5 py-3"><span style={{ color: "#6B7280", fontSize: 12 }}>{r.id}</span></td>
                <td className="px-5 py-3"><span style={{ color: "#E5E7EB", fontSize: 13 }}>{r.name}</span></td>
                <td className="px-5 py-3">
                  <span className="px-2 py-0.5 rounded" style={{ background: TYPE_COLORS[r.type] + "20", color: TYPE_COLORS[r.type], fontSize: 11 }}>{r.type}</span>
                </td>
                <td className="px-5 py-3"><span style={{ color: "#6B7280", fontSize: 12 }}>{r.created}</span></td>
                <td className="px-5 py-3"><span style={{ color: "#6B7280", fontSize: 12 }}>{r.size}</span></td>
                <td className="px-5 py-3">
                  <span className="flex items-center gap-1.5" style={{ color: "#10B981", fontSize: 12 }}>
                    <CheckCircle size={11} />Ready
                  </span>
                </td>
                <td className="px-5 py-3">
                  <div className="flex items-center gap-2">
                    <button className="w-7 h-7 rounded-lg flex items-center justify-center" style={{ background: "rgba(255,255,255,0.05)", color: "#6B7280", border: "none", cursor: "pointer" }}>
                      <Eye size={12} />
                    </button>
                    <button className="w-7 h-7 rounded-lg flex items-center justify-center" style={{ background: "rgba(79,140,255,0.1)", color: "#4F8CFF", border: "none", cursor: "pointer" }}>
                      <Download size={12} />
                    </button>
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
