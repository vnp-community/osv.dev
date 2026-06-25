import { useState } from "react";
import { AlertTriangle, ChevronRight, Globe, Shield, Eye } from "lucide-react";

const alerts = [
  { id: "A-01", name: "Cross-Site Scripting (XSS)", risk: "High", confidence: "Medium", count: 14, url: "/api/v1/search", method: "GET", param: "q", evidence: '<script>alert(1)</script>', desc: "Cross-site Scripting (XSS) - Reflected. Attacker-supplied code is executed in the victim browser.", solution: "Phase: Architecture and Design. Use a vetted library or framework that does not allow this weakness to occur." },
  { id: "A-02", name: "SQL Injection", risk: "Critical", confidence: "High", count: 3, url: "/api/v1/users", method: "POST", param: "email", evidence: "' OR 1=1--", desc: "SQL injection may be possible. The page results were manipulated using a boolean-based injection.", solution: "Use parameterized queries, prepared statements. Apply input validation and least-privilege database accounts." },
  { id: "A-03", name: "Broken Authentication — Missing CSRF Token", risk: "Medium", confidence: "Low", count: 8, url: "/api/v1/account/update", method: "POST", param: "csrf_token", evidence: "No CSRF token found in form", desc: "No Anti-CSRF tokens were found in a HTML submission form. A CSRF attack forces a logged-on victim's browser to send a forged HTTP request.", solution: "Phase: Architecture and Design. Use a vetted library or framework that does not allow this weakness. Use anti-CSRF tokens." },
  { id: "A-04", name: "Sensitive Data Exposure — API Keys in Response", risk: "High", confidence: "High", count: 2, url: "/api/v1/config", method: "GET", param: null, evidence: '"api_key": "sk-prod-xxxx..."', desc: "The response contains a potentially sensitive API key or credential that should not be returned to the client.", solution: "Remove sensitive data from API responses. Use environment variables and secrets management." },
  { id: "A-05", name: "Missing Security Headers", risk: "Low", confidence: "High", count: 1, url: "/", method: "GET", param: null, evidence: "X-Frame-Options header not set", desc: "The response does not include a X-Frame-Options header, meaning it can be embedded in frames.", solution: "Ensure that your web server, application server, load balancer, etc. is configured to set these headers." },
  { id: "A-06", name: "Directory Traversal", risk: "High", confidence: "Medium", count: 5, url: "/api/v1/files", method: "GET", param: "path", evidence: "../../../etc/passwd", desc: "Path traversal is possible via the 'path' parameter.", solution: "Assume all input is malicious. Validate and canonicalize paths. Use a chroot jail or equivalent." },
];

const RISK_COLORS: Record<string, string> = { Critical: "#EF4444", High: "#F97316", Medium: "#EAB308", Low: "#3B82F6" };
const TABS = ["Alerts", "Risk Breakdown"];

export function ZAPResults() {
  const [selected, setSelected] = useState(alerts[0]);
  const [activeTab, setActiveTab] = useState("Alerts");

  const breakdown = Object.entries(
    alerts.reduce((acc, a) => { acc[a.risk] = (acc[a.risk] || 0) + a.count; return acc; }, {} as Record<string, number>)
  );

  return (
    <div className="flex flex-col flex-1 overflow-hidden" style={{ background: "#0B1020" }}>
      {/* Header */}
      <div className="px-5 py-4" style={{ borderBottom: "1px solid rgba(255,255,255,0.06)" }}>
        <div className="flex items-center justify-between mb-3">
          <div className="flex items-center gap-3">
            <Globe size={20} color="#F97316" />
            <div>
              <h2 style={{ color: "#E5E7EB", fontSize: 16, fontWeight: 700 }}>OWASP ZAP Results</h2>
              <p style={{ color: "#6B7280", fontSize: 12 }}>https://api.company.com · Scan SC-0044 · Jun 13</p>
            </div>
          </div>
          <div className="grid grid-cols-4 gap-3">
            {[{ label: "Critical", val: 3, color: "#EF4444" }, { label: "High", val: 21, color: "#F97316" }, { label: "Medium", val: 8, color: "#EAB308" }, { label: "Low", val: 1, color: "#3B82F6" }].map(s => (
              <div key={s.label} className="rounded-xl px-3 py-2 text-center" style={{ background: RISK_COLORS[s.label] + "10" }}>
                <div style={{ color: RISK_COLORS[s.label], fontSize: 18, fontWeight: 700 }}>{s.val}</div>
                <div style={{ color: "#6B7280", fontSize: 10 }}>{s.label}</div>
              </div>
            ))}
          </div>
        </div>
        <div className="flex gap-1">
          {TABS.map(t => (
            <button key={t} onClick={() => setActiveTab(t)} className="px-4 py-1.5 rounded-lg" style={{ background: activeTab === t ? "rgba(249,115,22,0.12)" : "transparent", color: activeTab === t ? "#F97316" : "#6B7280", fontSize: 13, border: "none", cursor: "pointer" }}>{t}</button>
          ))}
        </div>
      </div>

      <div className="flex flex-1 overflow-hidden">
        {/* Alert list */}
        <div className="w-80 flex-shrink-0 overflow-y-auto" style={{ borderRight: "1px solid rgba(255,255,255,0.06)" }}>
          {alerts.map(a => (
            <div key={a.id} onClick={() => setSelected(a)} className="p-4 cursor-pointer transition-all"
              style={{ borderBottom: "1px solid rgba(255,255,255,0.04)", background: selected.id === a.id ? "rgba(249,115,22,0.07)" : "transparent", borderLeft: selected.id === a.id ? "2px solid #F97316" : "2px solid transparent" }}
              onMouseEnter={e => { if (selected.id !== a.id) e.currentTarget.style.background = "rgba(255,255,255,0.02)"; }}
              onMouseLeave={e => { if (selected.id !== a.id) e.currentTarget.style.background = "transparent"; }}
            >
              <div className="flex items-center gap-2 mb-1.5">
                <span className="px-2 py-0.5 rounded" style={{ background: RISK_COLORS[a.risk] + "20", color: RISK_COLORS[a.risk], fontSize: 11, fontWeight: 600 }}>{a.risk}</span>
                <span style={{ color: "#4B5563", fontSize: 11 }}>{a.count}x</span>
              </div>
              <div style={{ color: "#E5E7EB", fontSize: 13 }}>{a.name}</div>
              <div style={{ color: "#6B7280", fontSize: 11, marginTop: 4, fontFamily: "monospace" }}>{a.url}</div>
            </div>
          ))}
        </div>

        {/* Detail */}
        {selected && (
          <div className="flex-1 overflow-y-auto p-5">
            <div className="max-w-2xl">
              <div className="flex items-center gap-3 mb-5">
                <span className="px-3 py-1 rounded-lg" style={{ background: RISK_COLORS[selected.risk] + "20", color: RISK_COLORS[selected.risk], fontSize: 13, fontWeight: 600 }}>{selected.risk}</span>
                <h3 style={{ color: "#E5E7EB", fontSize: 16, fontWeight: 600 }}>{selected.name}</h3>
              </div>

              <div className="grid grid-cols-3 gap-3 mb-5">
                {[{ label: "Method", value: selected.method }, { label: "Instances", value: selected.count.toString() }, { label: "Confidence", value: selected.confidence }].map(m => (
                  <div key={m.label} className="rounded-xl p-3" style={{ background: "#151B2F", border: "1px solid rgba(255,255,255,0.07)" }}>
                    <div style={{ color: "#6B7280", fontSize: 10 }}>{m.label}</div>
                    <div style={{ color: "#E5E7EB", fontSize: 14, fontWeight: 600, marginTop: 2 }}>{m.value}</div>
                  </div>
                ))}
              </div>

              {selected.url && (
                <div className="rounded-xl p-3 mb-4" style={{ background: "rgba(255,255,255,0.04)", border: "1px solid rgba(255,255,255,0.07)" }}>
                  <div style={{ color: "#6B7280", fontSize: 10, marginBottom: 4 }}>URL</div>
                  <code style={{ color: "#4F8CFF", fontSize: 12 }}>{selected.url}</code>
                  {selected.param && <span style={{ color: "#F59E0B", fontSize: 12 }}> · param: <b>{selected.param}</b></span>}
                </div>
              )}

              {selected.evidence && (
                <div className="rounded-xl p-4 mb-4 font-mono" style={{ background: "#060D1A", border: "1px solid rgba(239,68,68,0.2)" }}>
                  <div style={{ color: "#EF4444", fontSize: 10, marginBottom: 6 }}>EVIDENCE</div>
                  <code style={{ color: "#F97316", fontSize: 12 }}>{selected.evidence}</code>
                </div>
              )}

              <div className="rounded-2xl p-5 mb-4" style={{ background: "#151B2F", border: "1px solid rgba(255,255,255,0.07)" }}>
                <div style={{ color: "#9CA3AF", fontSize: 11, fontWeight: 600, marginBottom: 8 }}>DESCRIPTION</div>
                <p style={{ color: "#E5E7EB", fontSize: 13, lineHeight: 1.7 }}>{selected.desc}</p>
              </div>

              <div className="rounded-2xl p-5" style={{ background: "rgba(16,185,129,0.07)", border: "1px solid rgba(16,185,129,0.2)" }}>
                <div style={{ color: "#10B981", fontSize: 11, fontWeight: 600, marginBottom: 8 }}>SOLUTION</div>
                <p style={{ color: "#A7F3D0", fontSize: 13, lineHeight: 1.7 }}>{selected.solution}</p>
              </div>
            </div>
          </div>
        )}
      </div>
    </div>
  );
}
