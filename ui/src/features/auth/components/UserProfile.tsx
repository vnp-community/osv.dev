import { useState } from "react";
import { Shield, Key, Bell, Monitor, Lock, Camera, Check, AlertTriangle, Loader2 } from "lucide-react";
import { useAuthStore } from "@/features/auth/store/authStore";
import { useSessions, useNotificationSettings, useUpdateNotificationSettings } from "../hooks/useProfile";
import type { NotifSetting } from "../hooks/useProfile";

const TABS = ["Profile", "Security", "Notifications", "API Keys", "Sessions"];

// ── Helper ────────────────────────────────────────────────────────────────────

function formatLastActive(iso: string): string {
  const diff = Date.now() - new Date(iso).getTime();
  const mins = Math.floor(diff / 60_000);
  if (mins < 2) return "Active now";
  if (mins < 60) return `${mins}m ago`;
  const hrs = Math.floor(mins / 60);
  if (hrs < 24) return `${hrs}h ago`;
  return `${Math.floor(hrs / 24)}d ago`;
}

// ── Component ─────────────────────────────────────────────────────────────────

export function UserProfile() {
  const [activeTab, setActiveTab] = useState("Profile");
  const user = useAuthStore(s => s.user);

  // Sessions tab
  const { data: sessionsData, isLoading: sessionsLoading } = useSessions();
  const sessions = sessionsData?.items ?? [];

  // Notifications tab
  const { data: notifData, isLoading: notifLoading } = useNotificationSettings();
  const updateNotifs = useUpdateNotificationSettings();
  const notifs: NotifSetting[] = notifData?.items ?? [];

  const displayName = user?.name ?? user?.email?.split("@")[0] ?? "Unknown";
  const displayEmail = user?.email ?? "—";
  const initials = displayName.split(" ").map(w => w[0]).join("").slice(0, 2).toUpperCase();

  const toggleNotif = (id: string) => {
    const updated = notifs.map(n => n.id === id ? { ...n, enabled: !n.enabled } : n);
    updateNotifs.mutate(updated);
  };

  return (
    <div className="flex-1 overflow-y-auto px-6 py-5" style={{ background: "var(--color-bg-page, #0B1020)" }}>
      {/* Header */}
      <div className="flex items-center gap-5 mb-6 pb-6" style={{ borderBottom: "1px solid rgba(255,255,255,0.06)" }}>
        <div className="relative">
          <div className="w-20 h-20 rounded-2xl flex items-center justify-center text-white"
            style={{ background: "linear-gradient(135deg,#4F8CFF,#7C3AED)", fontSize: 28, fontWeight: 700 }}>
            {initials}
          </div>
          <button className="absolute -bottom-1 -right-1 w-7 h-7 rounded-lg flex items-center justify-center"
            style={{ background: "var(--color-bg-card, #151B2F)", border: "1px solid rgba(255,255,255,0.12)", color: "var(--color-text-secondary, #9CA3AF)", cursor: "pointer" }}>
            <Camera size={12} />
          </button>
        </div>
        <div>
          <h2 style={{ color: "var(--color-text-primary, #E5E7EB)", fontSize: 20, fontWeight: 700 }}>{displayName}</h2>
          <p style={{ color: "var(--color-text-muted, #6B7280)", fontSize: 13 }}>{displayEmail}</p>
          <div className="flex items-center gap-3 mt-2">
            <span className="px-2.5 py-1 rounded-lg"
              style={{ background: "rgba(239,68,68,0.1)", color: "var(--color-status-error, #EF4444)", fontSize: 12 }}>
              {user?.role ?? "User"}
            </span>
            <span className="flex items-center gap-1.5" style={{ color: "var(--color-status-success, #10B981)", fontSize: 12 }}>
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

      {/* Profile tab */}
      {activeTab === "Profile" && (
        <div className="max-w-xl">
          <div className="rounded-2xl p-5" style={{ background: "var(--color-bg-card, #151B2F)", border: "1px solid rgba(255,255,255,0.07)" }}>
            {[{ label: "Full Name", value: displayName }, { label: "Email", value: displayEmail }, { label: "Department", value: "Security Operations" }, { label: "Job Title", value: "Chief Information Security Officer" }].map(({ label, value }) => (
              <div key={label} className="mb-4">
                <label style={{ color: "var(--color-text-secondary, #9CA3AF)", fontSize: 12, display: "block", marginBottom: 6 }}>{label}</label>
                <input defaultValue={value} className="w-full rounded-xl px-4 py-2.5 outline-none"
                  style={{ background: "var(--color-bg-sidebar, #0F1629)", border: "1px solid rgba(255,255,255,0.08)", color: "var(--color-text-primary, #E5E7EB)", fontSize: 13 }} />
              </div>
            ))}
            <button className="px-5 py-2.5 rounded-xl"
              style={{ background: "linear-gradient(135deg,#4F8CFF,#3B6FCC)", color: "white", border: "none", fontSize: 13, cursor: "pointer" }}>
              Save Changes
            </button>
          </div>
        </div>
      )}

      {/* Security tab */}
      {activeTab === "Security" && (
        <div className="max-w-xl flex flex-col gap-4">
          <div className="rounded-2xl p-5" style={{ background: "var(--color-bg-card, #151B2F)", border: "1px solid rgba(255,255,255,0.07)" }}>
            <h3 style={{ color: "var(--color-text-primary, #E5E7EB)", fontSize: 14, fontWeight: 600, marginBottom: 12 }}>Change Password</h3>
            {["Current Password", "New Password", "Confirm New Password"].map(l => (
              <div key={l} className="mb-3">
                <label style={{ color: "var(--color-text-secondary, #9CA3AF)", fontSize: 12, display: "block", marginBottom: 5 }}>{l}</label>
                <input type="password" className="w-full rounded-xl px-4 py-2.5 outline-none"
                  style={{ background: "var(--color-bg-sidebar, #0F1629)", border: "1px solid rgba(255,255,255,0.08)", color: "var(--color-text-primary, #E5E7EB)", fontSize: 13 }} />
              </div>
            ))}
            <button className="px-5 py-2.5 rounded-xl mt-2"
              style={{ background: "linear-gradient(135deg,#4F8CFF,#3B6FCC)", color: "white", border: "none", fontSize: 13, cursor: "pointer" }}>
              Update Password
            </button>
          </div>
          <div className="rounded-2xl p-5" style={{ background: "var(--color-bg-card, #151B2F)", border: "1px solid rgba(16,185,129,0.2)" }}>
            <div className="flex items-center justify-between">
              <div>
                <h3 style={{ color: "var(--color-text-primary, #E5E7EB)", fontSize: 14, fontWeight: 600 }}>Two-Factor Authentication</h3>
                <p style={{ color: "var(--color-text-muted, #6B7280)", fontSize: 12, marginTop: 4 }}>TOTP via Authenticator App · Enabled</p>
              </div>
              <div className="flex items-center gap-2">
                <Check size={14} color="#10B981" />
                <span style={{ color: "var(--color-status-success, #10B981)", fontSize: 12 }}>Active</span>
              </div>
            </div>
          </div>
        </div>
      )}

      {/* Notifications tab */}
      {activeTab === "Notifications" && (
        <div className="max-w-xl">
          {notifLoading ? (
            <div className="flex items-center justify-center py-12">
              <Loader2 size={24} className="animate-spin" style={{ color: "#4F8CFF" }} />
            </div>
          ) : (
            <div className="rounded-2xl p-5" style={{ background: "var(--color-bg-card, #151B2F)", border: "1px solid rgba(255,255,255,0.07)" }}>
              {notifs.map((n, i) => (
                <div key={n.id} className="flex items-center justify-between py-4"
                  style={{ borderBottom: i < notifs.length - 1 ? "1px solid rgba(255,255,255,0.05)" : "none" }}>
                  <div>
                    <div style={{ color: "var(--color-text-primary, #E5E7EB)", fontSize: 13 }}>{n.label}</div>
                    <div style={{ color: "var(--color-text-muted, #6B7280)", fontSize: 11, marginTop: 2 }}>{n.desc}</div>
                  </div>
                  <div className="relative w-10 h-5 rounded-full cursor-pointer"
                    style={{ background: n.enabled ? "#4F8CFF" : "rgba(255,255,255,0.1)" }}
                    onClick={() => toggleNotif(n.id)}>
                    <div className="absolute top-0.5 w-4 h-4 rounded-full bg-white transition-all"
                      style={{ left: n.enabled ? "22px" : "2px" }} />
                  </div>
                </div>
              ))}
            </div>
          )}
        </div>
      )}

      {/* Sessions tab */}
      {activeTab === "Sessions" && (
        <div className="max-w-xl flex flex-col gap-3">
          {sessionsLoading ? (
            <div className="flex items-center justify-center py-12">
              <Loader2 size={24} className="animate-spin" style={{ color: "#4F8CFF" }} />
            </div>
          ) : sessions.length === 0 ? (
            <div className="text-center py-12" style={{ color: "#6B7280", fontSize: 13 }}>No active sessions found.</div>
          ) : sessions.map(s => (
            <div key={s.id} className="rounded-2xl p-4 flex items-center gap-4"
              style={{ background: "var(--color-bg-card, #151B2F)", border: s.current ? "1px solid rgba(79,140,255,0.3)" : "1px solid rgba(255,255,255,0.07)" }}>
              <Monitor size={20} color={s.current ? "#4F8CFF" : "#6B7280"} />
              <div className="flex-1">
                <div style={{ color: "var(--color-text-primary, #E5E7EB)", fontSize: 13 }}>
                  {s.device}
                  {s.current && <span style={{ color: "var(--color-primary, #4F8CFF)", fontSize: 11, marginLeft: 8 }}>Current</span>}
                </div>
                <div style={{ color: "var(--color-text-muted, #6B7280)", fontSize: 11 }}>
                  {s.ip} · {s.location} · {formatLastActive(s.lastActive)}
                </div>
              </div>
              {!s.current && (
                <button className="px-3 py-1.5 rounded-lg"
                  style={{ background: "rgba(239,68,68,0.1)", color: "var(--color-status-error, #EF4444)", border: "none", fontSize: 12, cursor: "pointer" }}>
                  Revoke
                </button>
              )}
            </div>
          ))}
        </div>
      )}

      {/* API Keys tab */}
      {activeTab === "API Keys" && (
        <div className="max-w-xl">
          <div className="rounded-2xl p-8 text-center"
            style={{ background: "var(--color-bg-card, #151B2F)", border: "1px solid rgba(255,255,255,0.07)" }}>
            <Key size={32} color="#6B7280" style={{ margin: "0 auto 12px" }} />
            <p style={{ color: "var(--color-text-secondary, #9CA3AF)", fontSize: 13 }}>
              Manage your personal API keys from the Integrations → API Keys section.
            </p>
          </div>
        </div>
      )}
    </div>
  );
}
