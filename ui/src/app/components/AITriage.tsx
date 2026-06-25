import { useState } from "react";
import { Brain, CheckCircle, XCircle, Eye, Zap, AlertTriangle, ChevronRight, Clock } from "lucide-react";

const queue = [
  { id: "AT-001", finding: "F-2847", title: "Apache Log4j2 JNDI RCE", verdict: "Patch Immediately", confidence: 98, severity: "Critical", created: "10m ago", status: "pending", reasoning: "CVSS 10.0, EPSS 98.2%, active CISA KEV. High confidence real vulnerability requiring immediate patching.", fixes: ["Upgrade Log4j2 to 2.17.1+", "Set log4j2.formatMsgNoLookups=true", "Implement WAF rules"] },
  { id: "AT-002", finding: "F-2846", title: "Spring Framework Path Traversal", verdict: "Patch Immediately", confidence: 95, severity: "Critical", created: "15m ago", status: "pending", reasoning: "CVSS 9.8, confirmed exploitation in the wild via Spring4Shell attack patterns detected in logs.", fixes: ["Upgrade Spring Framework to 5.3.18+", "Restrict file access patterns", "Update Tomcat to 10.0.20+"] },
  { id: "AT-003", finding: "F-2840", title: "Redis No Authentication", verdict: "Configure Auth", confidence: 87, severity: "Medium", created: "1h ago", status: "accepted", reasoning: "Redis instance accessible without authentication on internal network. Low exploitation probability but significant risk if network is compromised.", fixes: ["Enable Redis AUTH", "Bind to specific IP", "Use TLS for connections"] },
  { id: "AT-004", finding: "F-2839", title: "WordPress XSS Comment Handler", verdict: "False Positive", confidence: 92, severity: "Low", created: "2h ago", status: "rejected", reasoning: "WordPress instance uses custom sanitization that prevents XSS. Scanner triggered on benign HTML encoding. False positive confirmed.", fixes: [] },
  { id: "AT-005", finding: "F-2844", title: "nginx HTTP/2 DoS", verdict: "Schedule Patch", confidence: 78, severity: "High", created: "3h ago", status: "pending", reasoning: "CVSS 7.5. Requires specific HTTP/2 configuration to be exploitable. Recommend patching in next maintenance window.", fixes: ["Update nginx to 1.25.3+", "Configure HTTP/2 rate limiting"] },
  { id: "AT-006", finding: "F-2841", title: "K8s API Server Exposure", verdict: "Accept Risk", confidence: 71, severity: "High", created: "4h ago", status: "pending", reasoning: "K8s API accessible only from internal VPN. Risk is mitigated by network controls. Consider accepting risk with quarterly review.", fixes: ["Network policy to restrict access", "Enable RBAC audit logging"] },
];

const VERDICT_COLORS: Record<string, string> = {
  "Patch Immediately": "#EF4444",
  "Schedule Patch": "#F59E0B",
  "Configure Auth": "#4F8CFF",
  "False Positive": "#6B7280",
  "Accept Risk": "#A78BFA",
};

const STATUS_STYLES: Record<string, { bg: string; color: string }> = {
  pending: { bg: "rgba(245,158,11,0.1)", color: "#F59E0B" },
  accepted: { bg: "rgba(16,185,129,0.1)", color: "#10B981" },
  rejected: { bg: "rgba(107,114,128,0.1)", color: "#6B7280" },
};

const SEVERITY_COLORS: Record<string, string> = { Critical: "#EF4444", High: "#F97316", Medium: "#EAB308", Low: "#3B82F6" };

export function AITriage() {
  const [selected, setSelected] = useState(queue[0]);
  const [filter, setFilter] = useState("All");

  const filtered = queue.filter((q) => {
    if (filter === "Pending") return q.status === "pending";
    if (filter === "Accepted") return q.status === "accepted";
    if (filter === "Rejected") return q.status === "rejected";
    return true;
  });

  const pendingCount = queue.filter((q) => q.status === "pending").length;

  return (
    <div className="flex flex-col flex-1 overflow-hidden" style={{ background: "#0B1020" }}>
      {/* Header */}
      <div className="px-6 py-4" style={{ borderBottom: "1px solid rgba(255,255,255,0.06)" }}>
        <div className="flex items-center justify-between mb-4">
          <div className="flex items-center gap-3">
            <div className="w-10 h-10 rounded-xl flex items-center justify-center" style={{ background: "rgba(124,58,237,0.2)" }}>
              <Brain size={20} color="#A78BFA" />
            </div>
            <div>
              <h2 style={{ color: "#E5E7EB", fontSize: 18, fontWeight: 700 }}>AI Triage Center</h2>
              <p style={{ color: "#6B7280", fontSize: 12 }}>{pendingCount} findings awaiting review</p>
            </div>
          </div>
          <div className="grid grid-cols-3 gap-3">
            {[
              { label: "Pending Review", value: pendingCount, color: "#F59E0B" },
              { label: "Accepted Today", value: 8, color: "#10B981" },
              { label: "Avg Confidence", value: "87%", color: "#A78BFA" },
            ].map((s) => (
              <div key={s.label} className="rounded-xl px-4 py-2 text-center" style={{ background: "rgba(255,255,255,0.04)", border: "1px solid rgba(255,255,255,0.07)" }}>
                <div style={{ color: s.color, fontSize: 18, fontWeight: 700 }}>{s.value}</div>
                <div style={{ color: "#6B7280", fontSize: 11 }}>{s.label}</div>
              </div>
            ))}
          </div>
        </div>
        <div className="flex gap-2">
          {["All", "Pending", "Accepted", "Rejected"].map((f) => (
            <button
              key={f}
              onClick={() => setFilter(f)}
              className="px-3 py-1.5 rounded-lg"
              style={{ background: filter === f ? "rgba(167,139,250,0.12)" : "rgba(255,255,255,0.05)", color: filter === f ? "#A78BFA" : "#6B7280", fontSize: 12, border: "none", cursor: "pointer" }}
            >
              {f}
            </button>
          ))}
        </div>
      </div>

      <div className="flex flex-1 overflow-hidden">
        {/* Queue */}
        <div className="w-96 flex-shrink-0 overflow-y-auto" style={{ borderRight: "1px solid rgba(255,255,255,0.06)" }}>
          {filtered.map((item) => (
            <div
              key={item.id}
              onClick={() => setSelected(item)}
              className="p-4 cursor-pointer transition-all"
              style={{
                borderBottom: "1px solid rgba(255,255,255,0.05)",
                background: selected?.id === item.id ? "rgba(167,139,250,0.07)" : "transparent",
                borderLeft: selected?.id === item.id ? "2px solid #A78BFA" : "2px solid transparent",
              }}
              onMouseEnter={(e) => { if (selected?.id !== item.id) e.currentTarget.style.background = "rgba(255,255,255,0.02)"; }}
              onMouseLeave={(e) => { if (selected?.id !== item.id) e.currentTarget.style.background = "transparent"; }}
            >
              <div className="flex items-start justify-between mb-2">
                <div className="flex items-center gap-2">
                  <span className="px-2 py-0.5 rounded" style={{ background: SEVERITY_COLORS[item.severity] + "20", color: SEVERITY_COLORS[item.severity], fontSize: 11 }}>{item.severity}</span>
                  <span style={{ color: "#6B7280", fontSize: 11 }}>{item.finding}</span>
                </div>
                <span className="px-2 py-0.5 rounded" style={{ ...STATUS_STYLES[item.status], fontSize: 11 }}>{item.status}</span>
              </div>
              <div style={{ color: "#E5E7EB", fontSize: 13 }} className="mb-2">{item.title}</div>
              <div className="flex items-center justify-between">
                <div className="flex items-center gap-2">
                  <Brain size={11} color={VERDICT_COLORS[item.verdict] || "#6B7280"} />
                  <span style={{ color: VERDICT_COLORS[item.verdict] || "#6B7280", fontSize: 12 }}>{item.verdict}</span>
                </div>
                <div className="flex items-center gap-1">
                  <div className="w-12 h-1.5 rounded-full overflow-hidden" style={{ background: "rgba(255,255,255,0.1)" }}>
                    <div className="h-full rounded-full" style={{ width: `${item.confidence}%`, background: item.confidence > 90 ? "#10B981" : "#F59E0B" }} />
                  </div>
                  <span style={{ color: "#6B7280", fontSize: 11 }}>{item.confidence}%</span>
                </div>
              </div>
              <div style={{ color: "#4B5563", fontSize: 10, marginTop: 6 }}>{item.created}</div>
            </div>
          ))}
        </div>

        {/* Detail panel */}
        {selected && (
          <div className="flex-1 overflow-y-auto p-6">
            <div className="max-w-2xl">
              {/* Header */}
              <div className="flex items-center gap-3 mb-5">
                <span className="px-2.5 py-1 rounded-lg" style={{ background: SEVERITY_COLORS[selected.severity] + "20", color: SEVERITY_COLORS[selected.severity], fontSize: 13, fontWeight: 600 }}>{selected.severity}</span>
                <h3 style={{ color: "#E5E7EB", fontSize: 16, fontWeight: 600 }}>{selected.title}</h3>
              </div>

              {/* AI Verdict */}
              <div className="rounded-2xl p-5 mb-5" style={{ background: "rgba(124,58,237,0.08)", border: "1px solid rgba(124,58,237,0.2)" }}>
                <div className="flex items-center gap-2 mb-3">
                  <Brain size={16} color="#A78BFA" />
                  <span style={{ color: "#A78BFA", fontSize: 14, fontWeight: 600 }}>AI Verdict</span>
                  <div className="ml-auto flex items-center gap-2">
                    <div className="w-20 h-2 rounded-full overflow-hidden" style={{ background: "rgba(255,255,255,0.1)" }}>
                      <div className="h-full rounded-full" style={{ width: `${selected.confidence}%`, background: selected.confidence > 90 ? "#10B981" : "#F59E0B" }} />
                    </div>
                    <span style={{ color: "#A78BFA", fontSize: 13, fontWeight: 700 }}>{selected.confidence}% confidence</span>
                  </div>
                </div>
                <div className="text-xl font-bold mb-3" style={{ color: VERDICT_COLORS[selected.verdict] || "#E5E7EB" }}>
                  {selected.verdict}
                </div>
                <p style={{ color: "#C4B5FD", fontSize: 13, lineHeight: 1.7 }}>{selected.reasoning}</p>
              </div>

              {/* Suggested fixes */}
              {selected.fixes.length > 0 && (
                <div className="rounded-2xl p-5 mb-5" style={{ background: "#151B2F", border: "1px solid rgba(255,255,255,0.07)" }}>
                  <div style={{ color: "#9CA3AF", fontSize: 12, fontWeight: 600, marginBottom: 12 }}>SUGGESTED REMEDIATION STEPS</div>
                  {selected.fixes.map((fix, i) => (
                    <div key={i} className="flex items-start gap-3 mb-3">
                      <div className="w-5 h-5 rounded-full flex items-center justify-center flex-shrink-0 mt-0.5" style={{ background: "rgba(16,185,129,0.2)", color: "#10B981", fontSize: 10, fontWeight: 700 }}>{i + 1}</div>
                      <span style={{ color: "#E5E7EB", fontSize: 13 }}>{fix}</span>
                    </div>
                  ))}
                </div>
              )}

              {/* Actions */}
              {selected.status === "pending" && (
                <div className="flex items-center gap-3">
                  <button
                    className="flex items-center gap-2 px-5 py-2.5 rounded-xl flex-1 justify-center"
                    style={{ background: "rgba(16,185,129,0.1)", border: "1px solid rgba(16,185,129,0.25)", color: "#10B981", fontSize: 14, fontWeight: 500, cursor: "pointer" }}
                  >
                    <CheckCircle size={15} />Accept Recommendation
                  </button>
                  <button
                    className="flex items-center gap-2 px-5 py-2.5 rounded-xl flex-1 justify-center"
                    style={{ background: "rgba(239,68,68,0.1)", border: "1px solid rgba(239,68,68,0.25)", color: "#EF4444", fontSize: 14, fontWeight: 500, cursor: "pointer" }}
                  >
                    <XCircle size={15} />Reject
                  </button>
                  <button
                    className="flex items-center gap-2 px-5 py-2.5 rounded-xl"
                    style={{ background: "rgba(255,255,255,0.05)", border: "1px solid rgba(255,255,255,0.09)", color: "#9CA3AF", fontSize: 14, cursor: "pointer" }}
                  >
                    <Eye size={15} />Manual Review
                  </button>
                </div>
              )}
              {selected.status !== "pending" && (
                <div className="rounded-xl p-4" style={{ background: STATUS_STYLES[selected.status].bg, border: `1px solid ${STATUS_STYLES[selected.status].color}30` }}>
                  <span style={{ color: STATUS_STYLES[selected.status].color, fontSize: 13 }}>
                    This recommendation has been {selected.status}.
                  </span>
                </div>
              )}
            </div>
          </div>
        )}
      </div>
    </div>
  );
}
