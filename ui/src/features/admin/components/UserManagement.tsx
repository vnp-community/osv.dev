import { useState } from "react";
import {
  Users, UserPlus, AlertTriangle, Search,
  CheckCircle, Lock, RefreshCw, Loader2,
} from "lucide-react";
import { useAdminUsers, useInviteUser, useUpdateUser, useUnlockUser } from "../hooks/useAdminUsers";
import type { UserRole } from "../types";

// ─── Styling maps — dùng CSS variables từ tokens.css ────────────────────────
const ROLE_STYLES: Record<string, { bg: string; color: string }> = {
  admin:    { bg: "var(--color-role-admin, rgba(239,68,68,0.1))",    color: "var(--color-role-admin, #EF4444)" },
  user:     { bg: "var(--color-role-user, rgba(79,140,255,0.1))",    color: "var(--color-role-user, #4F8CFF)" },
  readonly: { bg: "var(--color-role-readonly, rgba(107,114,128,0.1))", color: "var(--color-role-readonly, #9CA3AF)" },
  agent:    { bg: "var(--color-role-agent, rgba(16,185,129,0.1))",   color: "var(--color-role-agent, #10B981)" },
};

const ROLE_DISPLAY: Record<string, string> = {
  admin: "Admin", user: "User", readonly: "Readonly", agent: "Agent",
};

function getInitials(name: string) {
  return name.split(" ").map((w) => w[0]).join("").slice(0, 2).toUpperCase();
}

function formatLastLogin(iso?: string): string {
  if (!iso) return "Never";
  const diff = Date.now() - new Date(iso).getTime();
  const mins = Math.floor(diff / 60_000);
  if (mins < 60) return `${mins}m ago`;
  const hrs = Math.floor(mins / 60);
  if (hrs < 24) return `${hrs}h ago`;
  return `${Math.floor(hrs / 24)}d ago`;
}

// ─── Invite Dialog ────────────────────────────────────────────────────────────

interface InviteDialogProps {
  onClose: () => void;
}

function InviteDialog({ onClose }: InviteDialogProps) {
  const [form, setForm] = useState({ email: "", name: "", role: "user" as UserRole });
  const inviteUser = useInviteUser();

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    await inviteUser.mutateAsync(form);
    onClose();
  };

  return (
    <div
      className="fixed inset-0 z-50 flex items-center justify-center"
      style={{ background: "var(--color-bg-overlay, rgba(0,0,0,0.7))", backdropFilter: "blur(4px)" }}
    >
      <div
        className="w-full max-w-md rounded-2xl p-6"
        style={{ background: "var(--color-bg-card, #151B2F)", border: "1px solid var(--color-border-input, rgba(255,255,255,0.1))" }}
      >
        <div className="flex items-center justify-between mb-5">
          <h3 style={{ color: "var(--color-text-primary, #E5E7EB)", fontSize: 16, fontWeight: 600 }}>
            Invite User
          </h3>
          <button onClick={onClose} style={{ color: "var(--color-text-muted, #6B7280)", background: "none", border: "none", cursor: "pointer" }}>
            ✕
          </button>
        </div>
        <form onSubmit={handleSubmit}>
          {[
            { label: "Email Address", key: "email", type: "email", placeholder: "user@company.com" },
            { label: "Full Name", key: "name", type: "text", placeholder: "Jane Smith" },
          ].map(({ label, key, type, placeholder }) => (
            <div key={key} className="mb-4">
              <label style={{ color: "var(--color-text-secondary, #9CA3AF)", fontSize: 13, display: "block", marginBottom: 6 }}>
                {label}
              </label>
              <input
                type={type}
                placeholder={placeholder}
                value={form[key as keyof typeof form] as string}
                onChange={(e) => setForm((p) => ({ ...p, [key]: e.target.value }))}
                required
                className="w-full rounded-xl px-4 py-3 outline-none"
                style={{
                  background: "var(--color-bg-sidebar, #0F1629)",
                  border: "1px solid var(--color-border-input, rgba(255,255,255,0.09))",
                  color: "var(--color-text-primary, #E5E7EB)",
                  fontSize: 13,
                }}
              />
            </div>
          ))}
          <div className="mb-5">
            <label style={{ color: "var(--color-text-secondary, #9CA3AF)", fontSize: 13, display: "block", marginBottom: 6 }}>
              Role
            </label>
            <select
              value={form.role}
              onChange={(e) => setForm((p) => ({ ...p, role: e.target.value as UserRole }))}
              className="w-full rounded-xl px-4 py-3 outline-none"
              style={{
                background: "var(--color-bg-sidebar, #0F1629)",
                border: "1px solid var(--color-border-input, rgba(255,255,255,0.09))",
                color: "var(--color-text-primary, #E5E7EB)",
                fontSize: 13,
              }}
            >
              {(["user", "admin", "readonly", "agent"] as UserRole[]).map((r) => (
                <option key={r} value={r}>{ROLE_DISPLAY[r]}</option>
              ))}
            </select>
          </div>
          <div className="flex gap-3">
            <button
              type="button"
              onClick={onClose}
              className="flex-1 py-2.5 rounded-xl"
              style={{
                background: "var(--color-bg-input, rgba(255,255,255,0.07))",
                color: "var(--color-text-secondary, #9CA3AF)",
                border: "none",
                cursor: "pointer",
              }}
            >
              Cancel
            </button>
            <button
              type="submit"
              disabled={inviteUser.isPending}
              className="flex-1 py-2.5 rounded-xl flex items-center justify-center gap-2"
              style={{
                background: "var(--color-primary-grad, linear-gradient(135deg, #4F8CFF, #3B6FCC))",
                color: "white",
                border: "none",
                cursor: inviteUser.isPending ? "not-allowed" : "pointer",
                opacity: inviteUser.isPending ? 0.7 : 1,
              }}
            >
              {inviteUser.isPending && <Loader2 size={14} className="animate-spin" />}
              Send Invite
            </button>
          </div>
        </form>
      </div>
    </div>
  );
}

// ─── Main Component ───────────────────────────────────────────────────────────

export function UserManagement() {
  const [search, setSearch] = useState("");
  const [showInvite, setShowInvite] = useState(false);

  const { data, isLoading, isError, refetch } = useAdminUsers();
  const updateUser = useUpdateUser();
  const unlockUser = useUnlockUser();

  if (isLoading) {
    return (
      <div className="flex-1 flex items-center justify-center" style={{ background: "var(--color-bg-page, #0B1020)" }}>
        <div className="flex flex-col items-center gap-3">
          <Loader2 size={28} className="animate-spin" style={{ color: "var(--color-primary, #4F8CFF)" }} />
          <p style={{ color: "var(--color-text-muted, #6B7280)", fontSize: 13 }}>Loading users...</p>
        </div>
      </div>
    );
  }

  if (isError) {
    return (
      <div className="flex-1 flex items-center justify-center" style={{ background: "var(--color-bg-page, #0B1020)" }}>
        <div className="text-center">
          <AlertTriangle size={32} style={{ color: "var(--color-status-error, #EF4444)", margin: "0 auto 12px" }} />
          <p style={{ color: "var(--color-text-secondary, #9CA3AF)", fontSize: 13 }}>Failed to load users</p>
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

  const users = data?.users ?? [];
  const filtered = search
    ? users.filter(
        (u) =>
          u.name.toLowerCase().includes(search.toLowerCase()) ||
          u.email.toLowerCase().includes(search.toLowerCase())
      )
    : users;

  return (
    <div className="flex-1 overflow-y-auto px-6 py-5" style={{ background: "var(--color-bg-page, #0B1020)" }}>
      {/* Header */}
      <div className="flex items-center justify-between mb-6">
        <div>
          <h2 style={{ color: "var(--color-text-primary, #E5E7EB)", fontSize: 18, fontWeight: 700 }}>
            User Management
          </h2>
          <p style={{ color: "var(--color-text-muted, #6B7280)", fontSize: 12 }}>
            {users.length} users · {users.filter((u) => !u.mfa_enabled).length} without MFA
          </p>
        </div>
        <button
          onClick={() => setShowInvite(true)}
          className="flex items-center gap-2 px-4 py-2 rounded-xl"
          style={{
            background: "var(--color-primary-grad, linear-gradient(135deg, #4F8CFF, #3B6FCC))",
            color: "white",
            border: "none",
            fontSize: 13,
            cursor: "pointer",
          }}
        >
          <UserPlus size={14} /> Invite User
        </button>
      </div>

      {/* Role stats */}
      <div className="grid grid-cols-4 gap-4 mb-5">
        {[
          { label: "Admin",   count: users.filter((u) => u.role === "admin").length,   ...ROLE_STYLES.admin },
          { label: "User",    count: users.filter((u) => u.role === "user").length,    ...ROLE_STYLES.user },
          { label: "Readonly", count: users.filter((u) => u.role === "readonly").length, ...ROLE_STYLES.readonly },
          {
            label: "No MFA",
            count: users.filter((u) => !u.mfa_enabled).length,
            bg: "var(--color-status-warning-bg, rgba(245,158,11,0.1))",
            color: "var(--color-status-warning, #F59E0B)",
          },
        ].map(({ label, count, bg, color }) => (
          <div
            key={label}
            className="rounded-xl px-4 py-3 flex items-center gap-3"
            style={{ background: bg, border: `1px solid ${color}30` }}
          >
            <div style={{ color, fontSize: 22, fontWeight: 700 }}>{count}</div>
            <div style={{ color: "var(--color-text-secondary, #9CA3AF)", fontSize: 12 }}>{label}</div>
          </div>
        ))}
      </div>

      {/* Search */}
      <div className="relative mb-4 max-w-sm">
        <Search
          size={13}
          color="var(--color-text-faint, #4B5563)"
          style={{ position: "absolute", left: 10, top: "50%", transform: "translateY(-50%)" }}
        />
        <input
          id="user-search"
          value={search}
          onChange={(e) => setSearch(e.target.value)}
          placeholder="Search users..."
          className="w-full rounded-xl pl-8 pr-4 py-2.5 outline-none"
          style={{
            background: "var(--color-bg-card, #151B2F)",
            border: "1px solid var(--color-border-card, rgba(255,255,255,0.08))",
            color: "var(--color-text-primary, #E5E7EB)",
            fontSize: 13,
          }}
        />
      </div>

      {/* Table */}
      <div
        className="rounded-2xl"
        style={{
          background: "var(--color-bg-card, #151B2F)",
          border: "1px solid var(--color-border-subtle, rgba(255,255,255,0.07))",
        }}
      >
        <table className="w-full">
          <thead>
            <tr style={{ borderBottom: "1px solid var(--color-border-section, rgba(255,255,255,0.06))" }}>
              {["User", "Email", "Role", "MFA", "Last Login", "Status", "Actions"].map((h) => (
                <th
                  key={h}
                  className="px-5 py-3 text-left"
                  style={{ color: "var(--color-text-muted, #6B7280)", fontSize: 11, fontWeight: 600, letterSpacing: 0.5 }}
                >
                  {h}
                </th>
              ))}
            </tr>
          </thead>
          <tbody>
            {filtered.map((u, i) => (
              <tr
                key={u.id}
                className="transition-all"
                style={{
                  borderBottom: i < filtered.length - 1 ? "1px solid var(--color-border-section, rgba(255,255,255,0.04))" : "none",
                }}
                onMouseEnter={(e) => (e.currentTarget.style.background = "var(--color-bg-hover, rgba(255,255,255,0.02))")}
                onMouseLeave={(e) => (e.currentTarget.style.background = "transparent")}
              >
                {/* User */}
                <td className="px-5 py-3">
                  <div className="flex items-center gap-3">
                    <div
                      className="w-8 h-8 rounded-full flex items-center justify-center text-white"
                      style={{
                        background: u.is_active
                          ? "linear-gradient(135deg, #4F8CFF, #7C3AED)"
                          : "var(--color-text-disabled, #374151)",
                        fontSize: 11,
                        fontWeight: 700,
                      }}
                    >
                      {getInitials(u.name)}
                    </div>
                    <div>
                      <div style={{ color: u.is_active ? "var(--color-text-primary, #E5E7EB)" : "var(--color-text-muted, #6B7280)", fontSize: 13 }}>
                        {u.name}
                        {u.is_locked && (
                          <Lock size={11} style={{ display: "inline", marginLeft: 5, color: "var(--color-status-error, #EF4444)" }} />
                        )}
                      </div>
                    </div>
                  </div>
                </td>

                {/* Email */}
                <td className="px-5 py-3">
                  <span style={{ color: "var(--color-text-muted, #6B7280)", fontSize: 12 }}>{u.email}</span>
                </td>

                {/* Role */}
                <td className="px-5 py-3">
                  <span
                    className="px-2 py-0.5 rounded"
                    style={{ ...ROLE_STYLES[u.role], fontSize: 11 }}
                  >
                    {ROLE_DISPLAY[u.role]}
                  </span>
                </td>

                {/* MFA */}
                <td className="px-5 py-3">
                  {u.mfa_enabled ? (
                    <div className="flex items-center gap-1.5">
                      <CheckCircle size={13} color="var(--color-status-success, #10B981)" />
                      <span style={{ color: "var(--color-status-success, #10B981)", fontSize: 12 }}>Enabled</span>
                    </div>
                  ) : (
                    <div className="flex items-center gap-1.5">
                      <AlertTriangle size={13} color="var(--color-status-warning, #F59E0B)" />
                      <span style={{ color: "var(--color-status-warning, #F59E0B)", fontSize: 12 }}>Disabled</span>
                    </div>
                  )}
                </td>

                {/* Last Login */}
                <td className="px-5 py-3">
                  <span style={{ color: "var(--color-text-muted, #6B7280)", fontSize: 12 }}>
                    {formatLastLogin(u.last_login_at)}
                  </span>
                </td>

                {/* Status */}
                <td className="px-5 py-3">
                  <span
                    className="px-2 py-0.5 rounded"
                    style={{
                      background: u.is_active
                        ? "var(--color-status-success-bg, rgba(16,185,129,0.1))"
                        : "var(--color-status-neutral-bg, rgba(107,114,128,0.1))",
                      color: u.is_active
                        ? "var(--color-status-success, #10B981)"
                        : "var(--color-text-muted, #6B7280)",
                      fontSize: 11,
                    }}
                  >
                    {u.is_active ? "active" : "disabled"}
                  </span>
                </td>

                {/* Actions */}
                <td className="px-5 py-3">
                  <div className="flex items-center gap-2">
                    {u.is_locked && (
                      <button
                        onClick={() => unlockUser.mutate(u.id)}
                        className="px-2.5 py-1 rounded-lg"
                        style={{
                          background: "var(--color-status-warning-bg, rgba(245,158,11,0.1))",
                          color: "var(--color-status-warning, #F59E0B)",
                          border: "none",
                          cursor: "pointer",
                          fontSize: 11,
                        }}
                        title="Unlock account"
                      >
                        Unlock
                      </button>
                    )}
                    <button
                      onClick={() => updateUser.mutate({ id: u.id, is_active: !u.is_active })}
                      className="px-2.5 py-1 rounded-lg"
                      style={{
                        background: u.is_active
                          ? "var(--color-status-error-bg, rgba(239,68,68,0.08))"
                          : "var(--color-status-success-bg, rgba(16,185,129,0.1))",
                        color: u.is_active
                          ? "var(--color-status-error, #EF4444)"
                          : "var(--color-status-success, #10B981)",
                        border: "none",
                        cursor: "pointer",
                        fontSize: 11,
                      }}
                    >
                      {u.is_active ? "Disable" : "Enable"}
                    </button>
                  </div>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>

      {showInvite && <InviteDialog onClose={() => setShowInvite(false)} />}
    </div>
  );
}
