import { Check, X, Loader2, AlertTriangle, RefreshCw } from "lucide-react";
import { useRBACMatrix } from "../hooks/useRBACMatrix";

// ─── Role color map — dùng CSS variables ─────────────────────────────────────
const ROLE_COLORS: Record<string, string> = {
  admin:    "var(--color-role-admin, #EF4444)",
  user:     "var(--color-role-user, #4F8CFF)",
  readonly: "var(--color-role-readonly, #9CA3AF)",
  agent:    "var(--color-role-agent, #10B981)",
};

const ROLE_DISPLAY: Record<string, string> = {
  admin: "Admin", user: "User", readonly: "Readonly", agent: "Agent",
};

// ─── Role summary cards — static descriptions ─────────────────────────────────
const ROLE_DESCS: Record<string, string> = {
  admin:    "Full system access",
  user:     "Standard analyst access",
  readonly: "View-only access",
  agent:    "Automated scanning",
};

// Group permissions by category prefix
function groupPermissions(permissions: Array<{ permission: string; description: string; roles: Record<string, boolean> }>) {
  const categories: Record<string, typeof permissions> = {};
  for (const perm of permissions) {
    const [prefix] = perm.permission.split(".");
    const category = prefix.charAt(0).toUpperCase() + prefix.slice(1);
    if (!categories[category]) categories[category] = [];
    categories[category].push(perm);
  }
  return Object.entries(categories).map(([category, items]) => ({ category, items }));
}

export function RBACManagement() {
  const { data, isLoading, isError, refetch } = useRBACMatrix();

  if (isLoading) {
    return (
      <div className="flex-1 flex items-center justify-center" style={{ background: "var(--color-bg-page, #0B1020)" }}>
        <div className="flex flex-col items-center gap-3">
          <Loader2 size={28} className="animate-spin" style={{ color: "var(--color-primary, #4F8CFF)" }} />
          <p style={{ color: "var(--color-text-muted, #6B7280)", fontSize: 13 }}>Loading permissions...</p>
        </div>
      </div>
    );
  }

  if (isError) {
    return (
      <div className="flex-1 flex items-center justify-center" style={{ background: "var(--color-bg-page, #0B1020)" }}>
        <div className="text-center">
          <AlertTriangle size={32} style={{ color: "var(--color-status-error, #EF4444)", margin: "0 auto 12px" }} />
          <p style={{ color: "var(--color-text-secondary, #9CA3AF)", fontSize: 13 }}>Failed to load RBAC matrix</p>
          <button
            onClick={() => refetch()}
            className="mt-3 px-4 py-2 rounded-xl flex items-center gap-2 mx-auto"
            style={{ background: "var(--color-primary-bg, rgba(79,140,255,0.1))", color: "var(--color-primary, #4F8CFF)", border: "none", cursor: "pointer", fontSize: 13 }}
          >
            <RefreshCw size={13} /> Retry
          </button>
        </div>
      </div>
    );
  }

  const roles = data?.roles ?? [];
  const permissions = data?.permissions ?? [];
  const groups = groupPermissions(permissions);

  return (
    <div className="flex-1 overflow-y-auto px-6 py-5" style={{ background: "var(--color-bg-page, #0B1020)" }}>
      {/* Header */}
      <div className="mb-5">
        <h2 style={{ color: "var(--color-text-primary, #E5E7EB)", fontSize: 18, fontWeight: 700 }}>
          Roles &amp; Permissions
        </h2>
        <p style={{ color: "var(--color-text-muted, #6B7280)", fontSize: 12 }}>
          RBAC permission matrix — {roles.length} roles · {permissions.length} permissions
        </p>
      </div>

      {/* Role cards */}
      <div className="grid grid-cols-4 gap-4 mb-5">
        {roles.map((role) => {
          const color = ROLE_COLORS[role] ?? "#6B7280";
          return (
            <div
              key={role}
              className="rounded-2xl p-4"
              style={{
                background: "var(--color-bg-card, #151B2F)",
                border: `1px solid ${color}30`,
              }}
            >
              <div className="flex items-center justify-between mb-2">
                <span
                  className="px-2.5 py-1 rounded-lg"
                  style={{ background: `${color}15`, color, fontSize: 12, fontWeight: 600 }}
                >
                  {ROLE_DISPLAY[role] ?? role}
                </span>
                <span style={{ color: "var(--color-text-muted, #6B7280)", fontSize: 12 }}>
                  {permissions.filter((p) => p.roles[role]).length} perms
                </span>
              </div>
              <div style={{ color: "var(--color-text-secondary, #9CA3AF)", fontSize: 12 }}>
                {ROLE_DESCS[role] ?? role}
              </div>
            </div>
          );
        })}
      </div>

      {/* Permission matrix */}
      <div
        className="rounded-2xl overflow-hidden"
        style={{
          background: "var(--color-bg-card, #151B2F)",
          border: "1px solid var(--color-border-subtle, rgba(255,255,255,0.07))",
        }}
      >
        <table className="w-full">
          <thead>
            <tr style={{ borderBottom: "1px solid var(--color-border-section, rgba(255,255,255,0.06))" }}>
              <th
                className="px-5 py-3 text-left"
                style={{ color: "var(--color-text-muted, #6B7280)", fontSize: 11, fontWeight: 600, minWidth: 240 }}
              >
                PERMISSION
              </th>
              {roles.map((r) => (
                <th
                  key={r}
                  className="px-4 py-3 text-center"
                  style={{ color: ROLE_COLORS[r] ?? "#6B7280", fontSize: 12, fontWeight: 600, minWidth: 80 }}
                >
                  {ROLE_DISPLAY[r] ?? r}
                </th>
              ))}
            </tr>
          </thead>
          <tbody>
            {groups.map((group) => (
              <>
                <tr
                  key={group.category}
                  style={{
                    background: "var(--color-bg-hover, rgba(255,255,255,0.03))",
                    borderBottom: "1px solid var(--color-border-section, rgba(255,255,255,0.06))",
                  }}
                >
                  <td className="px-5 py-2.5" colSpan={roles.length + 1}>
                    <span style={{ color: "var(--color-text-secondary, #9CA3AF)", fontSize: 11, fontWeight: 600, letterSpacing: 0.5 }}>
                      {group.category.toUpperCase()}
                    </span>
                  </td>
                </tr>
                {group.items.map((perm) => (
                  <tr
                    key={perm.permission}
                    style={{ borderBottom: "1px solid var(--color-border-section, rgba(255,255,255,0.04))" }}
                    onMouseEnter={(e) => (e.currentTarget.style.background = "var(--color-bg-hover, rgba(255,255,255,0.02))")}
                    onMouseLeave={(e) => (e.currentTarget.style.background = "transparent")}
                  >
                    <td className="px-5 py-2.5">
                      <span style={{ color: "var(--color-text-secondary, #9CA3AF)", fontSize: 12, fontFamily: "monospace" }}>
                        {perm.permission}
                      </span>
                    </td>
                    {roles.map((role) => (
                      <td key={role} className="px-4 py-2.5 text-center">
                        {perm.roles[role]
                          ? <Check size={14} color={ROLE_COLORS[role] ?? "#10B981"} style={{ margin: "0 auto" }} />
                          : <X size={14} color="var(--color-text-disabled, #374151)" style={{ margin: "0 auto" }} />}
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
