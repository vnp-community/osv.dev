import { useState } from "react";
import { Users, UserPlus, Shield, Lock, MoreVertical, Search, CheckCircle, XCircle, AlertTriangle } from "lucide-react";

const users = [
  { id: "u-1", name: "Carol Anderson", email: "carol@company.com", role: "Admin", mfa: true, lastLogin: "5 min ago", status: "active", avatar: "CA" },
  { id: "u-2", name: "Bob Chen", email: "bob.chen@company.com", role: "User", mfa: true, lastLogin: "1h ago", status: "active", avatar: "BC" },
  { id: "u-3", name: "Alice Wu", email: "alice.wu@company.com", role: "User", mfa: true, lastLogin: "2h ago", status: "active", avatar: "AW" },
  { id: "u-4", name: "Dave Kim", email: "dave.kim@company.com", role: "User", mfa: false, lastLogin: "1d ago", status: "active", avatar: "DK" },
  { id: "u-5", name: "Eve Martinez", email: "eve.m@company.com", role: "Readonly", mfa: true, lastLogin: "3d ago", status: "active", avatar: "EM" },
  { id: "u-6", name: "Frank Liu", email: "frank.l@company.com", role: "Agent", mfa: false, lastLogin: "Never", status: "disabled", avatar: "FL" },
  { id: "u-7", name: "Grace Park", email: "grace.p@company.com", role: "Readonly", mfa: true, lastLogin: "1w ago", status: "active", avatar: "GP" },
];

const ROLE_STYLES: Record<string, { bg: string; color: string }> = {
  Admin: { bg: "rgba(239,68,68,0.1)", color: "#EF4444" },
  User: { bg: "rgba(79,140,255,0.1)", color: "#4F8CFF" },
  Readonly: { bg: "rgba(107,114,128,0.1)", color: "#9CA3AF" },
  Agent: { bg: "rgba(16,185,129,0.1)", color: "#10B981" },
};

export function UserManagement() {
  const [search, setSearch] = useState("");
  const [showInvite, setShowInvite] = useState(false);

  const filtered = users.filter((u) =>
    !search || u.name.toLowerCase().includes(search.toLowerCase()) || u.email.toLowerCase().includes(search.toLowerCase())
  );

  return (
    <div className="flex-1 overflow-y-auto px-6 py-5" style={{ background: "#0B1020" }}>
      <div className="flex items-center justify-between mb-6">
        <div>
          <h2 style={{ color: "#E5E7EB", fontSize: 18, fontWeight: 700 }}>User Management</h2>
          <p style={{ color: "#6B7280", fontSize: 12 }}>{users.length} users · {users.filter(u => !u.mfa).length} without MFA</p>
        </div>
        <button
          onClick={() => setShowInvite(true)}
          className="flex items-center gap-2 px-4 py-2 rounded-xl"
          style={{ background: "linear-gradient(135deg, #4F8CFF, #3B6FCC)", color: "white", border: "none", fontSize: 13, cursor: "pointer" }}
        >
          <UserPlus size={14} />Invite User
        </button>
      </div>

      {/* Role stats */}
      <div className="grid grid-cols-4 gap-4 mb-5">
        {[
          { label: "Admin", count: users.filter(u => u.role === "Admin").length, ...ROLE_STYLES.Admin },
          { label: "User", count: users.filter(u => u.role === "User").length, ...ROLE_STYLES.User },
          { label: "Readonly", count: users.filter(u => u.role === "Readonly").length, ...ROLE_STYLES.Readonly },
          { label: "No MFA", count: users.filter(u => !u.mfa).length, bg: "rgba(245,158,11,0.1)", color: "#F59E0B" },
        ].map(({ label, count, bg, color }) => (
          <div key={label} className="rounded-xl px-4 py-3 flex items-center gap-3" style={{ background: bg, border: `1px solid ${color}30` }}>
            <div style={{ color, fontSize: 22, fontWeight: 700 }}>{count}</div>
            <div style={{ color: "#9CA3AF", fontSize: 12 }}>{label}</div>
          </div>
        ))}
      </div>

      {/* Search */}
      <div className="relative mb-4 max-w-sm">
        <Search size={13} color="#4B5563" style={{ position: "absolute", left: 10, top: "50%", transform: "translateY(-50%)" }} />
        <input value={search} onChange={(e) => setSearch(e.target.value)} placeholder="Search users..." className="w-full rounded-xl pl-8 pr-4 py-2.5 outline-none" style={{ background: "#151B2F", border: "1px solid rgba(255,255,255,0.08)", color: "#E5E7EB", fontSize: 13 }} />
      </div>

      <div className="rounded-2xl" style={{ background: "#151B2F", border: "1px solid rgba(255,255,255,0.07)" }}>
        <table className="w-full">
          <thead>
            <tr style={{ borderBottom: "1px solid rgba(255,255,255,0.06)" }}>
              {["User", "Email", "Role", "MFA", "Last Login", "Status", "Actions"].map((h) => (
                <th key={h} className="px-5 py-3 text-left" style={{ color: "#6B7280", fontSize: 11, fontWeight: 600, letterSpacing: 0.5 }}>{h}</th>
              ))}
            </tr>
          </thead>
          <tbody>
            {filtered.map((u, i) => (
              <tr key={u.id} className="transition-all" style={{ borderBottom: i < filtered.length - 1 ? "1px solid rgba(255,255,255,0.04)" : "none" }}
                onMouseEnter={(e) => (e.currentTarget.style.background = "rgba(255,255,255,0.02)")}
                onMouseLeave={(e) => (e.currentTarget.style.background = "transparent")}
              >
                <td className="px-5 py-3">
                  <div className="flex items-center gap-3">
                    <div className="w-8 h-8 rounded-full flex items-center justify-center text-white" style={{ background: u.status === "disabled" ? "#374151" : "linear-gradient(135deg, #4F8CFF, #7C3AED)", fontSize: 11, fontWeight: 700 }}>{u.avatar}</div>
                    <span style={{ color: u.status === "disabled" ? "#6B7280" : "#E5E7EB", fontSize: 13 }}>{u.name}</span>
                  </div>
                </td>
                <td className="px-5 py-3"><span style={{ color: "#6B7280", fontSize: 12 }}>{u.email}</span></td>
                <td className="px-5 py-3">
                  <span className="px-2 py-0.5 rounded" style={{ ...ROLE_STYLES[u.role], fontSize: 11 }}>{u.role}</span>
                </td>
                <td className="px-5 py-3">
                  {u.mfa ? (
                    <div className="flex items-center gap-1.5">
                      <CheckCircle size={13} color="#10B981" />
                      <span style={{ color: "#10B981", fontSize: 12 }}>Enabled</span>
                    </div>
                  ) : (
                    <div className="flex items-center gap-1.5">
                      <AlertTriangle size={13} color="#F59E0B" />
                      <span style={{ color: "#F59E0B", fontSize: 12 }}>Disabled</span>
                    </div>
                  )}
                </td>
                <td className="px-5 py-3"><span style={{ color: "#6B7280", fontSize: 12 }}>{u.lastLogin}</span></td>
                <td className="px-5 py-3">
                  <span className="px-2 py-0.5 rounded" style={{ background: u.status === "active" ? "rgba(16,185,129,0.1)" : "rgba(107,114,128,0.1)", color: u.status === "active" ? "#10B981" : "#6B7280", fontSize: 11 }}>
                    {u.status}
                  </span>
                </td>
                <td className="px-5 py-3">
                  <div className="flex items-center gap-2">
                    <button className="px-2.5 py-1 rounded-lg" style={{ background: "rgba(255,255,255,0.05)", color: "#9CA3AF", border: "none", cursor: "pointer", fontSize: 11 }}>Edit</button>
                    <button className="px-2.5 py-1 rounded-lg" style={{ background: "rgba(239,68,68,0.08)", color: "#EF4444", border: "none", cursor: "pointer", fontSize: 11 }}>
                      {u.status === "active" ? "Disable" : "Enable"}
                    </button>
                  </div>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>

      {showInvite && (
        <div className="fixed inset-0 z-50 flex items-center justify-center" style={{ background: "rgba(0,0,0,0.7)", backdropFilter: "blur(4px)" }}>
          <div className="w-full max-w-md rounded-2xl p-6" style={{ background: "#151B2F", border: "1px solid rgba(255,255,255,0.1)" }}>
            <div className="flex items-center justify-between mb-5">
              <h3 style={{ color: "#E5E7EB", fontSize: 16, fontWeight: 600 }}>Invite User</h3>
              <button onClick={() => setShowInvite(false)} style={{ color: "#6B7280", background: "none", border: "none", cursor: "pointer" }}>✕</button>
            </div>
            {["Email Address", "Full Name"].map((label) => (
              <div key={label} className="mb-4">
                <label style={{ color: "#9CA3AF", fontSize: 13, display: "block", marginBottom: 6 }}>{label}</label>
                <input type={label === "Email Address" ? "email" : "text"} placeholder={label === "Email Address" ? "user@company.com" : "Jane Smith"} className="w-full rounded-xl px-4 py-3 outline-none" style={{ background: "#0F1629", border: "1px solid rgba(255,255,255,0.09)", color: "#E5E7EB", fontSize: 13 }} />
              </div>
            ))}
            <div className="mb-5">
              <label style={{ color: "#9CA3AF", fontSize: 13, display: "block", marginBottom: 6 }}>Role</label>
              <select className="w-full rounded-xl px-4 py-3 outline-none" style={{ background: "#0F1629", border: "1px solid rgba(255,255,255,0.09)", color: "#E5E7EB", fontSize: 13 }}>
                <option>User</option>
                <option>Admin</option>
                <option>Readonly</option>
                <option>Agent</option>
              </select>
            </div>
            <div className="flex gap-3">
              <button onClick={() => setShowInvite(false)} className="flex-1 py-2.5 rounded-xl" style={{ background: "rgba(255,255,255,0.07)", color: "#9CA3AF", border: "none", cursor: "pointer" }}>Cancel</button>
              <button className="flex-1 py-2.5 rounded-xl" style={{ background: "linear-gradient(135deg, #4F8CFF, #3B6FCC)", color: "white", border: "none", cursor: "pointer" }}>Send Invite</button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}
