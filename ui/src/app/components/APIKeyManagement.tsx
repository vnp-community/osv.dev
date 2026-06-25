import { useState } from "react";
import { Key, Plus, Copy, Eye, EyeOff, Trash2, RotateCcw, Check, X, Shield } from "lucide-react";

const apiKeys = [
  { id: "k-001", name: "CI/CD Pipeline", prefix: "osv_prod_xK7m", scope: ["scan:write", "finding:read"], created: "Jun 1, 2026", lastUsed: "2 min ago", expires: "Dec 31, 2026", status: "active" },
  { id: "k-002", name: "SIEM Integration", prefix: "osv_prod_aR9p", scope: ["finding:read", "report:read"], created: "May 15, 2026", lastUsed: "1h ago", expires: "Never", status: "active" },
  { id: "k-003", name: "Monitoring Dashboard", prefix: "osv_ro_bN4t", scope: ["dashboard:read"], created: "Apr 10, 2026", lastUsed: "30 min ago", expires: "Never", status: "active" },
  { id: "k-004", name: "Legacy Scanner", prefix: "osv_prod_cM2s", scope: ["scan:read", "scan:write"], created: "Jan 5, 2026", lastUsed: "30 days ago", expires: "Expired", status: "expired" },
];

const SCOPE_COLORS: Record<string, string> = {
  "scan:write": "#4F8CFF",
  "scan:read": "#60A5FA",
  "finding:read": "#F59E0B",
  "finding:write": "#F97316",
  "report:read": "#10B981",
  "dashboard:read": "#A78BFA",
};

export function APIKeyManagement() {
  const [showModal, setShowModal] = useState(false);
  const [newKeyName, setNewKeyName] = useState("");
  const [selectedScopes, setSelectedScopes] = useState<string[]>([]);
  const [generatedKey, setGeneratedKey] = useState("");
  const [copiedKey, setCopiedKey] = useState(false);

  const allScopes = ["scan:read", "scan:write", "finding:read", "finding:write", "report:read", "dashboard:read"];

  const handleGenerate = () => {
    const key = `osv_prod_${"abcdefghijklmnopqrstuvwxyz0123456789".split("").sort(() => Math.random() - 0.5).slice(0, 24).join("")}`;
    setGeneratedKey(key);
  };

  const handleCopy = () => {
    navigator.clipboard.writeText(generatedKey);
    setCopiedKey(true);
    setTimeout(() => setCopiedKey(false), 1500);
  };

  return (
    <div className="flex-1 overflow-y-auto px-6 py-5" style={{ background: "#0B1020" }}>
      <div className="flex items-center justify-between mb-6">
        <div>
          <h2 style={{ color: "#E5E7EB", fontSize: 18, fontWeight: 700 }}>API Key Management</h2>
          <p style={{ color: "#6B7280", fontSize: 12 }}>Manage API keys for programmatic access to OSV Platform</p>
        </div>
        <button
          onClick={() => { setShowModal(true); setGeneratedKey(""); setNewKeyName(""); setSelectedScopes([]); }}
          className="flex items-center gap-2 px-4 py-2 rounded-xl"
          style={{ background: "linear-gradient(135deg, #4F8CFF, #3B6FCC)", color: "white", border: "none", fontSize: 13, cursor: "pointer" }}
        >
          <Plus size={14} />Create API Key
        </button>
      </div>

      <div className="rounded-2xl" style={{ background: "#151B2F", border: "1px solid rgba(255,255,255,0.07)" }}>
        <table className="w-full">
          <thead>
            <tr style={{ borderBottom: "1px solid rgba(255,255,255,0.06)" }}>
              {["Name", "Key Prefix", "Scopes", "Created", "Last Used", "Expiration", "Status", ""].map((h) => (
                <th key={h} className="px-5 py-3 text-left" style={{ color: "#6B7280", fontSize: 11, fontWeight: 600, letterSpacing: 0.5 }}>{h}</th>
              ))}
            </tr>
          </thead>
          <tbody>
            {apiKeys.map((k, i) => (
              <tr key={k.id} className="transition-all" style={{ borderBottom: i < apiKeys.length - 1 ? "1px solid rgba(255,255,255,0.04)" : "none" }}
                onMouseEnter={(e) => (e.currentTarget.style.background = "rgba(255,255,255,0.02)")}
                onMouseLeave={(e) => (e.currentTarget.style.background = "transparent")}
              >
                <td className="px-5 py-4">
                  <div className="flex items-center gap-2">
                    <Key size={13} color="#4F8CFF" />
                    <span style={{ color: "#E5E7EB", fontSize: 13, fontWeight: 500 }}>{k.name}</span>
                  </div>
                </td>
                <td className="px-5 py-4"><span style={{ color: "#9CA3AF", fontSize: 12, fontFamily: "monospace" }}>{k.prefix}•••••••</span></td>
                <td className="px-5 py-4">
                  <div className="flex gap-1 flex-wrap">
                    {k.scope.map((s) => (
                      <span key={s} className="px-1.5 py-0.5 rounded" style={{ background: SCOPE_COLORS[s] + "20", color: SCOPE_COLORS[s], fontSize: 10 }}>{s}</span>
                    ))}
                  </div>
                </td>
                <td className="px-5 py-4"><span style={{ color: "#6B7280", fontSize: 12 }}>{k.created}</span></td>
                <td className="px-5 py-4"><span style={{ color: "#6B7280", fontSize: 12 }}>{k.lastUsed}</span></td>
                <td className="px-5 py-4"><span style={{ color: k.expires === "Expired" ? "#EF4444" : "#6B7280", fontSize: 12 }}>{k.expires}</span></td>
                <td className="px-5 py-4">
                  <span className="px-2 py-0.5 rounded" style={{ background: k.status === "active" ? "rgba(16,185,129,0.1)" : "rgba(239,68,68,0.1)", color: k.status === "active" ? "#10B981" : "#EF4444", fontSize: 11 }}>
                    {k.status}
                  </span>
                </td>
                <td className="px-5 py-4">
                  <div className="flex items-center gap-2">
                    <button className="w-7 h-7 rounded-lg flex items-center justify-center" style={{ background: "rgba(79,140,255,0.1)", color: "#4F8CFF", border: "none", cursor: "pointer" }}>
                      <RotateCcw size={11} />
                    </button>
                    <button className="w-7 h-7 rounded-lg flex items-center justify-center" style={{ background: "rgba(239,68,68,0.1)", color: "#EF4444", border: "none", cursor: "pointer" }}>
                      <Trash2 size={11} />
                    </button>
                  </div>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>

      {/* Create Key Modal */}
      {showModal && (
        <div className="fixed inset-0 z-50 flex items-center justify-center" style={{ background: "rgba(0,0,0,0.7)", backdropFilter: "blur(4px)" }}>
          <div className="w-full max-w-lg rounded-2xl p-6" style={{ background: "#151B2F", border: "1px solid rgba(255,255,255,0.1)" }}>
            <div className="flex items-center justify-between mb-5">
              <h3 style={{ color: "#E5E7EB", fontSize: 16, fontWeight: 600 }}>Create API Key</h3>
              <button onClick={() => setShowModal(false)} style={{ color: "#6B7280", background: "none", border: "none", cursor: "pointer" }}><X size={18} /></button>
            </div>

            {!generatedKey ? (
              <>
                <div className="mb-4">
                  <label style={{ color: "#9CA3AF", fontSize: 13, display: "block", marginBottom: 6 }}>Key Name</label>
                  <input
                    value={newKeyName}
                    onChange={(e) => setNewKeyName(e.target.value)}
                    placeholder="e.g. CI/CD Pipeline"
                    className="w-full rounded-xl px-4 py-3 outline-none"
                    style={{ background: "#0F1629", border: "1px solid rgba(255,255,255,0.09)", color: "#E5E7EB", fontSize: 13 }}
                  />
                </div>
                <div className="mb-5">
                  <label style={{ color: "#9CA3AF", fontSize: 13, display: "block", marginBottom: 8 }}>Permissions</label>
                  <div className="grid grid-cols-2 gap-2">
                    {allScopes.map((scope) => (
                      <div
                        key={scope}
                        onClick={() => setSelectedScopes((s) => s.includes(scope) ? s.filter((x) => x !== scope) : [...s, scope])}
                        className="flex items-center gap-2 px-3 py-2 rounded-xl cursor-pointer"
                        style={{ background: selectedScopes.includes(scope) ? SCOPE_COLORS[scope] + "15" : "rgba(255,255,255,0.04)", border: `1px solid ${selectedScopes.includes(scope) ? SCOPE_COLORS[scope] + "40" : "rgba(255,255,255,0.07)"}` }}
                      >
                        <div className="w-4 h-4 rounded flex items-center justify-center" style={{ background: selectedScopes.includes(scope) ? SCOPE_COLORS[scope] : "rgba(255,255,255,0.1)" }}>
                          {selectedScopes.includes(scope) && <Check size={10} color="white" />}
                        </div>
                        <span style={{ color: SCOPE_COLORS[scope] || "#9CA3AF", fontSize: 12 }}>{scope}</span>
                      </div>
                    ))}
                  </div>
                </div>
                <button
                  onClick={handleGenerate}
                  disabled={!newKeyName}
                  className="w-full py-3 rounded-xl"
                  style={{ background: !newKeyName ? "rgba(79,140,255,0.3)" : "linear-gradient(135deg, #4F8CFF, #3B6FCC)", color: "white", border: "none", fontSize: 14, cursor: !newKeyName ? "not-allowed" : "pointer" }}
                >
                  Generate API Key
                </button>
              </>
            ) : (
              <div>
                <div className="rounded-xl p-4 mb-4" style={{ background: "rgba(16,185,129,0.08)", border: "1px solid rgba(16,185,129,0.25)" }}>
                  <div className="flex items-center gap-2 mb-2">
                    <Shield size={14} color="#10B981" />
                    <span style={{ color: "#10B981", fontSize: 13, fontWeight: 600 }}>Key created — copy it now!</span>
                  </div>
                  <p style={{ color: "#9CA3AF", fontSize: 12 }}>This key will only be shown once. Store it securely.</p>
                </div>
                <div className="flex items-center gap-2 p-3 rounded-xl mb-4" style={{ background: "#0F1629", border: "1px solid rgba(255,255,255,0.09)" }}>
                  <code style={{ color: "#10B981", fontSize: 12, flex: 1, fontFamily: "monospace", wordBreak: "break-all" }}>{generatedKey}</code>
                  <button onClick={handleCopy} style={{ color: copiedKey ? "#10B981" : "#6B7280", background: "none", border: "none", cursor: "pointer" }}>
                    {copiedKey ? <Check size={15} /> : <Copy size={15} />}
                  </button>
                </div>
                <button onClick={() => setShowModal(false)} className="w-full py-3 rounded-xl" style={{ background: "rgba(255,255,255,0.07)", color: "#E5E7EB", border: "none", fontSize: 14, cursor: "pointer" }}>
                  Done
                </button>
              </div>
            )}
          </div>
        </div>
      )}
    </div>
  );
}
