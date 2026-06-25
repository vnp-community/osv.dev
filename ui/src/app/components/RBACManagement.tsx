import { Check, X } from "lucide-react";

const ROLES = ["Admin", "User", "Readonly", "Agent"];
const PERMISSIONS = [
  { category: "Dashboard", items: ["dashboard.view", "dashboard.export"] },
  { category: "CVE Intelligence", items: ["cve.read", "cve.search", "cve.export"] },
  { category: "Scanning", items: ["scan.create", "scan.read", "scan.cancel", "scan.delete"] },
  { category: "Findings", items: ["finding.read", "finding.create", "finding.update", "finding.close", "finding.accept_risk"] },
  { category: "Assets", items: ["asset.read", "asset.create", "asset.update", "asset.delete"] },
  { category: "Products", items: ["product.read", "product.create", "product.update"] },
  { category: "Reports", items: ["report.read", "report.create", "report.delete", "report.share"] },
  { category: "Administration", items: ["user.read", "user.create", "user.disable", "role.manage", "audit.read", "settings.manage"] },
  { category: "API & Integrations", items: ["api_key.create", "webhook.manage", "integration.manage"] },
];

const MATRIX: Record<string, Record<string, boolean>> = {
  "dashboard.view": { Admin: true, User: true, Readonly: true, Agent: false },
  "dashboard.export": { Admin: true, User: true, Readonly: false, Agent: false },
  "cve.read": { Admin: true, User: true, Readonly: true, Agent: false },
  "cve.search": { Admin: true, User: true, Readonly: true, Agent: false },
  "cve.export": { Admin: true, User: true, Readonly: false, Agent: false },
  "scan.create": { Admin: true, User: true, Readonly: false, Agent: true },
  "scan.read": { Admin: true, User: true, Readonly: true, Agent: true },
  "scan.cancel": { Admin: true, User: true, Readonly: false, Agent: false },
  "scan.delete": { Admin: true, User: false, Readonly: false, Agent: false },
  "finding.read": { Admin: true, User: true, Readonly: true, Agent: true },
  "finding.create": { Admin: true, User: true, Readonly: false, Agent: true },
  "finding.update": { Admin: true, User: true, Readonly: false, Agent: false },
  "finding.close": { Admin: true, User: true, Readonly: false, Agent: false },
  "finding.accept_risk": { Admin: true, User: false, Readonly: false, Agent: false },
  "asset.read": { Admin: true, User: true, Readonly: true, Agent: true },
  "asset.create": { Admin: true, User: true, Readonly: false, Agent: true },
  "asset.update": { Admin: true, User: true, Readonly: false, Agent: false },
  "asset.delete": { Admin: true, User: false, Readonly: false, Agent: false },
  "product.read": { Admin: true, User: true, Readonly: true, Agent: false },
  "product.create": { Admin: true, User: true, Readonly: false, Agent: false },
  "product.update": { Admin: true, User: true, Readonly: false, Agent: false },
  "report.read": { Admin: true, User: true, Readonly: true, Agent: false },
  "report.create": { Admin: true, User: true, Readonly: false, Agent: false },
  "report.delete": { Admin: true, User: false, Readonly: false, Agent: false },
  "report.share": { Admin: true, User: true, Readonly: false, Agent: false },
  "user.read": { Admin: true, User: false, Readonly: false, Agent: false },
  "user.create": { Admin: true, User: false, Readonly: false, Agent: false },
  "user.disable": { Admin: true, User: false, Readonly: false, Agent: false },
  "role.manage": { Admin: true, User: false, Readonly: false, Agent: false },
  "audit.read": { Admin: true, User: false, Readonly: false, Agent: false },
  "settings.manage": { Admin: true, User: false, Readonly: false, Agent: false },
  "api_key.create": { Admin: true, User: true, Readonly: false, Agent: false },
  "webhook.manage": { Admin: true, User: false, Readonly: false, Agent: false },
  "integration.manage": { Admin: true, User: false, Readonly: false, Agent: false },
};

const ROLE_COLORS: Record<string, string> = { Admin: "#EF4444", User: "#4F8CFF", Readonly: "#9CA3AF", Agent: "#10B981" };

export function RBACManagement() {
  return (
    <div className="flex-1 overflow-y-auto px-6 py-5" style={{ background: "#0B1020" }}>
      <div className="mb-5">
        <h2 style={{ color: "#E5E7EB", fontSize: 18, fontWeight: 700 }}>Roles & Permissions</h2>
        <p style={{ color: "#6B7280", fontSize: 12 }}>RBAC permission matrix — 4 roles · {PERMISSIONS.flatMap(p => p.items).length} permissions</p>
      </div>

      {/* Role cards */}
      <div className="grid grid-cols-4 gap-4 mb-5">
        {[
          { role: "Admin", desc: "Full system access", users: 2, color: "#EF4444" },
          { role: "User", desc: "Standard analyst access", users: 4, color: "#4F8CFF" },
          { role: "Readonly", desc: "View-only access", users: 3, color: "#9CA3AF" },
          { role: "Agent", desc: "Automated scanning", users: 1, color: "#10B981" },
        ].map(r => (
          <div key={r.role} className="rounded-2xl p-4" style={{ background: "#151B2F", border: `1px solid ${r.color}30` }}>
            <div className="flex items-center justify-between mb-2">
              <span className="px-2.5 py-1 rounded-lg" style={{ background: r.color + "15", color: r.color, fontSize: 12, fontWeight: 600 }}>{r.role}</span>
              <span style={{ color: "#6B7280", fontSize: 12 }}>{r.users} users</span>
            </div>
            <div style={{ color: "#9CA3AF", fontSize: 12 }}>{r.desc}</div>
          </div>
        ))}
      </div>

      {/* Permission matrix */}
      <div className="rounded-2xl overflow-hidden" style={{ background: "#151B2F", border: "1px solid rgba(255,255,255,0.07)" }}>
        <table className="w-full">
          <thead>
            <tr style={{ borderBottom: "1px solid rgba(255,255,255,0.06)" }}>
              <th className="px-5 py-3 text-left" style={{ color: "#6B7280", fontSize: 11, fontWeight: 600, minWidth: 240 }}>PERMISSION</th>
              {ROLES.map(r => (
                <th key={r} className="px-4 py-3 text-center" style={{ color: ROLE_COLORS[r], fontSize: 12, fontWeight: 600, minWidth: 80 }}>{r}</th>
              ))}
            </tr>
          </thead>
          <tbody>
            {PERMISSIONS.map(group => (
              <>
                <tr key={group.category} style={{ background: "rgba(255,255,255,0.03)", borderBottom: "1px solid rgba(255,255,255,0.06)" }}>
                  <td className="px-5 py-2.5" colSpan={5}>
                    <span style={{ color: "#9CA3AF", fontSize: 11, fontWeight: 600, letterSpacing: 0.5 }}>{group.category.toUpperCase()}</span>
                  </td>
                </tr>
                {group.items.map((perm, i) => (
                  <tr key={perm} style={{ borderBottom: "1px solid rgba(255,255,255,0.04)" }}
                    onMouseEnter={e => (e.currentTarget.style.background = "rgba(255,255,255,0.02)")}
                    onMouseLeave={e => (e.currentTarget.style.background = "transparent")}
                  >
                    <td className="px-5 py-2.5"><span style={{ color: "#9CA3AF", fontSize: 12, fontFamily: "monospace" }}>{perm}</span></td>
                    {ROLES.map(role => (
                      <td key={role} className="px-4 py-2.5 text-center">
                        {MATRIX[perm]?.[role]
                          ? <Check size={14} color={ROLE_COLORS[role]} style={{ margin: "0 auto" }} />
                          : <X size={14} color="#374151" style={{ margin: "0 auto" }} />
                        }
                      </td>
                    ))}
                  </tr>
                ))}
              </>
            ))}
          </tbody>
        </table>
      </div>
    </div>
  );
}
