import { useState } from "react";
import { Settings, Mail, Database, Cpu, Bell, Key, Lock, Shield, CheckCircle, AlertTriangle } from "lucide-react";

const SECTIONS = [
  { id: "general", label: "General", icon: Settings },
  { id: "email", label: "Email / SMTP", icon: Mail },
  { id: "storage", label: "Storage", icon: Database },
  { id: "ai", label: "AI Providers", icon: Cpu },
  { id: "security", label: "Security Policy", icon: Shield },
  { id: "notifications", label: "Notifications", icon: Bell },
];

const aiProviders = [
  { id: "openai", name: "OpenAI", model: "gpt-4o", status: "active", latency: "203ms", usage: "4,821 req/day", cost: "$12.40/day" },
  { id: "azure", name: "Azure OpenAI", model: "gpt-4-turbo", status: "standby", latency: "—", usage: "0 req/day", cost: "$0.00/day" },
  { id: "ollama", name: "Ollama (Local)", model: "llama3:8b", status: "inactive", latency: "—", usage: "0 req/day", cost: "$0.00" },
];

const STATUS_S: Record<string, { bg: string; color: string }> = {
  active: { bg: "rgba(16,185,129,0.1)", color: "#10B981" },
  standby: { bg: "rgba(245,158,11,0.1)", color: "#F59E0B" },
  inactive: { bg: "rgba(107,114,128,0.1)", color: "#6B7280" },
};

export function SystemSettings() {
  const [section, setSection] = useState("general");

  return (
    <div className="flex flex-1 overflow-hidden" style={{ background: "#0B1020" }}>
      {/* Left nav */}
      <div className="w-52 flex-shrink-0 py-5 px-3 overflow-y-auto" style={{ background: "#0F1629", borderRight: "1px solid rgba(255,255,255,0.06)" }}>
        <div style={{ color: "#6B7280", fontSize: 10, fontWeight: 600, letterSpacing: 1, marginBottom: 10, paddingLeft: 8 }}>SETTINGS</div>
        {SECTIONS.map(s => {
          const Icon = s.icon;
          return (
            <button key={s.id} onClick={() => setSection(s.id)}
              className="w-full flex items-center gap-2.5 px-3 py-2 rounded-lg mb-0.5 text-left transition-all"
              style={{ background: section === s.id ? "rgba(79,140,255,0.12)" : "transparent", color: section === s.id ? "#4F8CFF" : "#9CA3AF", fontSize: 13 }}
            >
              <Icon size={14} />{s.label}
            </button>
          );
        })}
      </div>

      {/* Content */}
      <div className="flex-1 overflow-y-auto p-6">
        {section === "general" && (
          <div className="max-w-xl">
            <h2 style={{ color: "#E5E7EB", fontSize: 18, fontWeight: 700, marginBottom: 20 }}>General Settings</h2>
            <div className="rounded-2xl p-5" style={{ background: "#151B2F", border: "1px solid rgba(255,255,255,0.07)" }}>
              {[{ label: "Platform Name", value: "OSV Platform" }, { label: "Organization", value: "Company Security" }, { label: "Support Email", value: "security@company.com" }, { label: "Timezone", value: "Asia/Ho_Chi_Minh" }, { label: "Date Format", value: "YYYY-MM-DD" }].map(f => (
                <div key={f.label} className="mb-4">
                  <label style={{ color: "#9CA3AF", fontSize: 12, display: "block", marginBottom: 5 }}>{f.label}</label>
                  <input defaultValue={f.value} className="w-full rounded-xl px-4 py-2.5 outline-none" style={{ background: "#0F1629", border: "1px solid rgba(255,255,255,0.08)", color: "#E5E7EB", fontSize: 13 }} />
                </div>
              ))}
              <button className="px-5 py-2.5 rounded-xl" style={{ background: "linear-gradient(135deg,#4F8CFF,#3B6FCC)", color: "white", border: "none", fontSize: 13, cursor: "pointer" }}>Save Changes</button>
            </div>
          </div>
        )}

        {section === "email" && (
          <div className="max-w-xl">
            <h2 style={{ color: "#E5E7EB", fontSize: 18, fontWeight: 700, marginBottom: 20 }}>Email / SMTP Settings</h2>
            <div className="rounded-2xl p-5" style={{ background: "#151B2F", border: "1px solid rgba(255,255,255,0.07)" }}>
              {[{ label: "SMTP Host", value: "smtp.company.com" }, { label: "SMTP Port", value: "587" }, { label: "Username", value: "noreply@company.com" }, { label: "From Name", value: "OSV Platform" }].map(f => (
                <div key={f.label} className="mb-4">
                  <label style={{ color: "#9CA3AF", fontSize: 12, display: "block", marginBottom: 5 }}>{f.label}</label>
                  <input defaultValue={f.value} className="w-full rounded-xl px-4 py-2.5 outline-none" style={{ background: "#0F1629", border: "1px solid rgba(255,255,255,0.08)", color: "#E5E7EB", fontSize: 13 }} />
                </div>
              ))}
              <div className="mb-4">
                <label style={{ color: "#9CA3AF", fontSize: 12, display: "block", marginBottom: 5 }}>SMTP Password</label>
                <input type="password" defaultValue="••••••••" className="w-full rounded-xl px-4 py-2.5 outline-none" style={{ background: "#0F1629", border: "1px solid rgba(255,255,255,0.08)", color: "#E5E7EB", fontSize: 13 }} />
              </div>
              <div className="flex gap-3">
                <button className="px-5 py-2.5 rounded-xl" style={{ background: "linear-gradient(135deg,#4F8CFF,#3B6FCC)", color: "white", border: "none", fontSize: 13, cursor: "pointer" }}>Save</button>
                <button className="px-5 py-2.5 rounded-xl" style={{ background: "rgba(255,255,255,0.07)", color: "#9CA3AF", border: "none", fontSize: 13, cursor: "pointer" }}>Test Connection</button>
              </div>
            </div>
          </div>
        )}

        {section === "ai" && (
          <div className="max-w-2xl">
            <h2 style={{ color: "#E5E7EB", fontSize: 18, fontWeight: 700, marginBottom: 20 }}>AI Provider Configuration</h2>
            <div className="flex flex-col gap-4">
              {aiProviders.map(p => (
                <div key={p.id} className="rounded-2xl p-5" style={{ background: "#151B2F", border: `1px solid ${p.status === "active" ? "rgba(16,185,129,0.25)" : "rgba(255,255,255,0.07)"}` }}>
                  <div className="flex items-center justify-between mb-4">
                    <div>
                      <div style={{ color: "#E5E7EB", fontSize: 15, fontWeight: 600 }}>{p.name}</div>
                      <div style={{ color: "#6B7280", fontSize: 12, marginTop: 2 }}>Model: {p.model}</div>
                    </div>
                    <span className="px-2.5 py-1 rounded-lg" style={{ ...STATUS_S[p.status], fontSize: 12 }}>{p.status}</span>
                  </div>
                  <div className="grid grid-cols-3 gap-3 mb-4">
                    {[{ label: "Latency", value: p.latency }, { label: "Usage", value: p.usage }, { label: "Cost", value: p.cost }].map(m => (
                      <div key={m.label} className="rounded-xl p-3" style={{ background: "rgba(255,255,255,0.04)" }}>
                        <div style={{ color: "#6B7280", fontSize: 10 }}>{m.label}</div>
                        <div style={{ color: "#E5E7EB", fontSize: 13, fontWeight: 600, marginTop: 2 }}>{m.value}</div>
                      </div>
                    ))}
                  </div>
                  <div className="flex gap-2">
                    <button className="px-3 py-1.5 rounded-lg" style={{ background: "rgba(79,140,255,0.1)", color: "#4F8CFF", border: "none", fontSize: 12, cursor: "pointer" }}>Configure</button>
                    {p.status !== "active" && <button className="px-3 py-1.5 rounded-lg" style={{ background: "rgba(16,185,129,0.1)", color: "#10B981", border: "none", fontSize: 12, cursor: "pointer" }}>Set Active</button>}
                    <button className="px-3 py-1.5 rounded-lg" style={{ background: "rgba(255,255,255,0.05)", color: "#9CA3AF", border: "none", fontSize: 12, cursor: "pointer" }}>Test</button>
                  </div>
                </div>
              ))}
            </div>
          </div>
        )}

        {section === "security" && (
          <div className="max-w-xl">
            <h2 style={{ color: "#E5E7EB", fontSize: 18, fontWeight: 700, marginBottom: 20 }}>Security Policy</h2>
            <div className="flex flex-col gap-4">
              {[
                { title: "Password Policy", items: [{ label: "Minimum Length", type: "number", value: "12" }, { label: "Require Uppercase", type: "toggle", value: true }, { label: "Require Special Char", type: "toggle", value: true }, { label: "Max Age (days)", type: "number", value: "90" }] },
                { title: "Session Policy", items: [{ label: "Session Timeout (min)", type: "number", value: "60" }, { label: "Max Concurrent Sessions", type: "number", value: "3" }] },
                { title: "MFA Policy", items: [{ label: "Require MFA for All Users", type: "toggle", value: true }, { label: "Allow SMS OTP", type: "toggle", value: false }] },
              ].map(section => (
                <div key={section.title} className="rounded-2xl p-5" style={{ background: "#151B2F", border: "1px solid rgba(255,255,255,0.07)" }}>
                  <div style={{ color: "#E5E7EB", fontSize: 14, fontWeight: 600, marginBottom: 14 }}>{section.title}</div>
                  {section.items.map(item => (
                    <div key={item.label} className="flex items-center justify-between py-2.5" style={{ borderBottom: "1px solid rgba(255,255,255,0.04)" }}>
                      <span style={{ color: "#9CA3AF", fontSize: 13 }}>{item.label}</span>
                      {item.type === "toggle" ? (
                        <div className="relative w-10 h-5 rounded-full cursor-pointer" style={{ background: item.value ? "#4F8CFF" : "rgba(255,255,255,0.1)" }}>
                          <div className="absolute top-0.5 w-4 h-4 rounded-full bg-white transition-all" style={{ left: item.value ? "22px" : "2px" }} />
                        </div>
                      ) : (
                        <input defaultValue={item.value as string} className="rounded-lg px-3 py-1.5 outline-none w-20 text-right" style={{ background: "#0F1629", border: "1px solid rgba(255,255,255,0.08)", color: "#E5E7EB", fontSize: 13 }} />
                      )}
                    </div>
                  ))}
                </div>
              ))}
              <button className="px-5 py-2.5 rounded-xl self-start" style={{ background: "linear-gradient(135deg,#4F8CFF,#3B6FCC)", color: "white", border: "none", fontSize: 13, cursor: "pointer" }}>Save Security Policy</button>
            </div>
          </div>
        )}

        {(section === "storage" || section === "notifications") && (
          <div className="flex-1 flex items-center justify-center pt-20">
            <div className="text-center">
              <Settings size={40} color="#374151" style={{ margin: "0 auto 12px" }} />
              <p style={{ color: "#6B7280", fontSize: 13 }}>{SECTIONS.find(s => s.id === section)?.label} settings</p>
              <p style={{ color: "#4B5563", fontSize: 12, marginTop: 4 }}>Configuration options available in production deployment</p>
            </div>
          </div>
        )}
      </div>
    </div>
  );
}
