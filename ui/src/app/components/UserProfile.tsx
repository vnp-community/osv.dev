import { useState } from "react";
import { Shield, Key, Bell, Monitor, Lock, Camera, Check, AlertTriangle } from "lucide-react";

const TABS = ["Profile", "Security", "Notifications", "API Keys", "Sessions"];

const sessions = [
  { device: "Chrome on macOS", ip: "192.168.1.100", location: "Ho Chi Minh City, VN", lastActive: "Active now", current: true },
  { device: "Firefox on Windows", ip: "10.10.5.22", location: "Hanoi, VN", lastActive: "2h ago", current: false },
  { device: "Mobile — Safari iOS", ip: "203.113.131.45", location: "Singapore, SG", lastActive: "1d ago", current: false },
];

const notifSettings = [
  { label: "Critical Finding Alerts", desc: "Notify when Critical severity findings are created", value: true },
  { label: "SLA Breach Warnings", desc: "Alert 48h before SLA expiration", value: true },
  { label: "KEV Updates", desc: "New CISA KEV additions affecting your assets", value: true },
  { label: "Scan Completion", desc: "Notify when scans complete or fail", value: false },
  { label: "Weekly Digest", desc: "Weekly summary of platform activity", value: true },
];

export function UserProfile() {
  const [activeTab, setActiveTab] = useState("Profile");
  const [notifs, setNotifs] = useState(notifSettings);

  return (
    <div className="flex-1 overflow-y-auto px-6 py-5" style={{ background: "#0B1020" }}>
      {/* Header */}
      <div className="flex items-center gap-5 mb-6 pb-6" style={{ borderBottom: "1px solid rgba(255,255,255,0.06)" }}>
        <div className="relative">
          <div className="w-20 h-20 rounded-2xl flex items-center justify-center text-white" style={{ background: "linear-gradient(135deg,#4F8CFF,#7C3AED)", fontSize: 28, fontWeight: 700 }}>CA</div>
          <button className="absolute -bottom-1 -right-1 w-7 h-7 rounded-lg flex items-center justify-center" style={{ background: "#151B2F", border: "1px solid rgba(255,255,255,0.12)", color: "#9CA3AF", cursor: "pointer" }}>
            <Camera size={12} />
          </button>
        </div>
        <div>
          <h2 style={{ color: "#E5E7EB", fontSize: 20, fontWeight: 700 }}>Carol Anderson</h2>
          <p style={{ color: "#6B7280", fontSize: 13 }}>CISO · carol@company.com</p>
          <div className="flex items-center gap-3 mt-2">
            <span className="px-2.5 py-1 rounded-lg" style={{ background: "rgba(239,68,68,0.1)", color: "#EF4444", fontSize: 12 }}>Admin</span>
            <span className="flex items-center gap-1.5" style={{ color: "#10B981", fontSize: 12 }}>
              <Check size={12} />MFA Enabled
            </span>
          </div>
        </div>
      </div>

      {/* Tabs */}
      <div className="flex gap-1 mb-6">
        {TABS.map(tab => (
          <button key={tab} onClick={() => setActiveTab(tab)} className="px-4 py-2 rounded-xl"
            style={{ background: activeTab === tab ? "rgba(79,140,255,0.12)" : "transparent", color: activeTab === tab ? "#4F8CFF" : "#6B7280", fontSize: 13, border: "none", cursor: "pointer", borderBottom: activeTab === tab ? "2px solid #4F8CFF" : "2px solid transparent" }}
          >{tab}</button>
        ))}
      </div>

      {activeTab === "Profile" && (
        <div className="max-w-xl">
          <div className="rounded-2xl p-5" style={{ background: "#151B2F", border: "1px solid rgba(255,255,255,0.07)" }}>
            {[{ label: "Full Name", value: "Carol Anderson" }, { label: "Email", value: "carol@company.com" }, { label: "Department", value: "Security Operations" }, { label: "Job Title", value: "Chief Information Security Officer" }, { label: "Phone", value: "+84 901 234 567" }, { label: "Timezone", value: "Asia/Ho_Chi_Minh (UTC+7)" }].map(({ label, value }) => (
              <div key={label} className="mb-4">
                <label style={{ color: "#9CA3AF", fontSize: 12, display: "block", marginBottom: 6 }}>{label}</label>
                <input defaultValue={value} className="w-full rounded-xl px-4 py-2.5 outline-none" style={{ background: "#0F1629", border: "1px solid rgba(255,255,255,0.08)", color: "#E5E7EB", fontSize: 13 }} />
              </div>
            ))}
            <button className="px-5 py-2.5 rounded-xl" style={{ background: "linear-gradient(135deg,#4F8CFF,#3B6FCC)", color: "white", border: "none", fontSize: 13, cursor: "pointer" }}>Save Changes</button>
          </div>
        </div>
      )}

      {activeTab === "Security" && (
        <div className="max-w-xl flex flex-col gap-4">
          <div className="rounded-2xl p-5" style={{ background: "#151B2F", border: "1px solid rgba(255,255,255,0.07)" }}>
            <h3 style={{ color: "#E5E7EB", fontSize: 14, fontWeight: 600, marginBottom: 12 }}>Change Password</h3>
            {["Current Password", "New Password", "Confirm New Password"].map(l => (
              <div key={l} className="mb-3">
                <label style={{ color: "#9CA3AF", fontSize: 12, display: "block", marginBottom: 5 }}>{l}</label>
                <input type="password" className="w-full rounded-xl px-4 py-2.5 outline-none" style={{ background: "#0F1629", border: "1px solid rgba(255,255,255,0.08)", color: "#E5E7EB", fontSize: 13 }} />
              </div>
            ))}
            <button className="px-5 py-2.5 rounded-xl mt-2" style={{ background: "linear-gradient(135deg,#4F8CFF,#3B6FCC)", color: "white", border: "none", fontSize: 13, cursor: "pointer" }}>Update Password</button>
          </div>
          <div className="rounded-2xl p-5" style={{ background: "#151B2F", border: "1px solid rgba(16,185,129,0.2)" }}>
            <div className="flex items-center justify-between">
              <div>
                <h3 style={{ color: "#E5E7EB", fontSize: 14, fontWeight: 600 }}>Two-Factor Authentication</h3>
                <p style={{ color: "#6B7280", fontSize: 12, marginTop: 4 }}>TOTP via Authenticator App · Enabled Jun 1, 2026</p>
              </div>
              <div className="flex items-center gap-2">
                <Check size={14} color="#10B981" />
                <span style={{ color: "#10B981", fontSize: 12 }}>Active</span>
              </div>
            </div>
          </div>
        </div>
      )}

      {activeTab === "Notifications" && (
        <div className="max-w-xl">
          <div className="rounded-2xl p-5" style={{ background: "#151B2F", border: "1px solid rgba(255,255,255,0.07)" }}>
            {notifs.map((n, i) => (
              <div key={n.label} className="flex items-center justify-between py-4" style={{ borderBottom: i < notifs.length - 1 ? "1px solid rgba(255,255,255,0.05)" : "none" }}>
                <div>
                  <div style={{ color: "#E5E7EB", fontSize: 13 }}>{n.label}</div>
                  <div style={{ color: "#6B7280", fontSize: 11, marginTop: 2 }}>{n.desc}</div>
                </div>
                <div className="relative w-10 h-5 rounded-full cursor-pointer" style={{ background: n.value ? "#4F8CFF" : "rgba(255,255,255,0.1)" }}
                  onClick={() => setNotifs(prev => prev.map((x, j) => j === i ? { ...x, value: !x.value } : x))}>
                  <div className="absolute top-0.5 w-4 h-4 rounded-full bg-white transition-all" style={{ left: n.value ? "22px" : "2px" }} />
                </div>
              </div>
            ))}
          </div>
        </div>
      )}

      {activeTab === "Sessions" && (
        <div className="max-w-xl flex flex-col gap-3">
          {sessions.map((s, i) => (
            <div key={i} className="rounded-2xl p-4 flex items-center gap-4" style={{ background: "#151B2F", border: s.current ? "1px solid rgba(79,140,255,0.3)" : "1px solid rgba(255,255,255,0.07)" }}>
              <Monitor size={20} color={s.current ? "#4F8CFF" : "#6B7280"} />
              <div className="flex-1">
                <div style={{ color: "#E5E7EB", fontSize: 13 }}>{s.device}{s.current && <span style={{ color: "#4F8CFF", fontSize: 11, marginLeft: 8 }}>Current</span>}</div>
                <div style={{ color: "#6B7280", fontSize: 11 }}>{s.ip} · {s.location} · {s.lastActive}</div>
              </div>
              {!s.current && <button className="px-3 py-1.5 rounded-lg" style={{ background: "rgba(239,68,68,0.1)", color: "#EF4444", border: "none", fontSize: 12, cursor: "pointer" }}>Revoke</button>}
            </div>
          ))}
        </div>
      )}

      {activeTab === "API Keys" && (
        <div className="max-w-xl">
          <div className="rounded-2xl p-8 text-center" style={{ background: "#151B2F", border: "1px solid rgba(255,255,255,0.07)" }}>
            <Key size={32} color="#6B7280" style={{ margin: "0 auto 12px" }} />
            <p style={{ color: "#9CA3AF", fontSize: 13 }}>Manage your personal API keys from the Integrations → API Keys section.</p>
          </div>
        </div>
      )}
    </div>
  );
}
