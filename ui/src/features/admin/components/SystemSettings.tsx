import { useState } from "react";
import { Settings, Mail, Database, Cpu, Bell, Shield, Loader2, AlertTriangle, RefreshCw, Save } from "lucide-react";
import { useSystemSettings, useUpdateSettings } from "../hooks/useSystemSettings";
import type { AIProviderConfig } from "../types";

const NAV_SECTIONS = [
  { id: "general",       label: "General",        icon: Settings },
  { id: "email",         label: "Email / SMTP",   icon: Mail },
  { id: "storage",       label: "Storage",        icon: Database },
  { id: "ai",            label: "AI Providers",   icon: Cpu },
  { id: "security",      label: "Security Policy", icon: Shield },
  { id: "notifications", label: "Notifications",  icon: Bell },
];

const AI_STATUS_STYLES: Record<string, { bg: string; color: string }> = {
  active:   { bg: "var(--color-status-success-bg, rgba(16,185,129,0.1))",  color: "var(--color-status-success, #10B981)" },
  standby:  { bg: "var(--color-status-warning-bg, rgba(245,158,11,0.1))",  color: "var(--color-status-warning, #F59E0B)" },
  inactive: { bg: "var(--color-status-neutral-bg, rgba(107,114,128,0.1))", color: "var(--color-status-neutral, #6B7280)" },
};

// ─── Sub-sections ─────────────────────────────────────────────────────────────

function GeneralSection({ settings }: { settings: NonNullable<ReturnType<typeof useSystemSettings>["data"]> }) {
  const updateSettings = useUpdateSettings();
  const [form, setForm] = useState(settings.general || {});
  const [saved, setSaved] = useState(false);

  const handleSave = async () => {
    await updateSettings.mutateAsync({ general: form });
    setSaved(true);
    setTimeout(() => setSaved(false), 2000);
  };

  return (
    <div className="max-w-xl">
      <h2 style={{ color: "var(--color-text-primary, #E5E7EB)", fontSize: 18, fontWeight: 700, marginBottom: 20 }}>
        General Settings
      </h2>
      <div
        className="rounded-2xl p-5"
        style={{
          background: "var(--color-bg-card, #151B2F)",
          border: "1px solid var(--color-border-subtle, rgba(255,255,255,0.07))",
        }}
      >
        {([
          { label: "Platform Name",  key: "platform_name" },
          { label: "Organization",   key: "organization" },
          { label: "Support Email",  key: "support_email" },
          { label: "Timezone",       key: "timezone" },
          { label: "Date Format",    key: "date_format" },
        ] as { label: string; key: keyof typeof form }[]).map(({ label, key }) => (
          <div key={key} className="mb-4">
            <label style={{ color: "var(--color-text-secondary, #9CA3AF)", fontSize: 12, display: "block", marginBottom: 5 }}>
              {label}
            </label>
            <input
              value={form[key] || ""}
              onChange={(e) => setForm((p) => ({ ...p, [key]: e.target.value }))}
              className="w-full rounded-xl px-4 py-2.5 outline-none"
              style={{
                background: "var(--color-bg-sidebar, #0F1629)",
                border: "1px solid var(--color-border-card, rgba(255,255,255,0.08))",
                color: "var(--color-text-primary, #E5E7EB)",
                fontSize: 13,
              }}
            />
          </div>
        ))}
        <button
          onClick={handleSave}
          disabled={updateSettings.isPending}
          className="px-5 py-2.5 rounded-xl flex items-center gap-2"
          style={{
            background: saved
              ? "var(--color-status-success-bg, rgba(16,185,129,0.15))"
              : "var(--color-primary-grad, linear-gradient(135deg,#4F8CFF,#3B6FCC))",
            color: saved ? "var(--color-status-success, #10B981)" : "white",
            border: "none",
            fontSize: 13,
            cursor: updateSettings.isPending ? "not-allowed" : "pointer",
            opacity: updateSettings.isPending ? 0.7 : 1,
          }}
        >
          {updateSettings.isPending && <Loader2 size={13} className="animate-spin" />}
          <Save size={13} />
          {saved ? "Saved!" : "Save Changes"}
        </button>
      </div>
    </div>
  );
}

function AISection({ providers }: { providers?: AIProviderConfig[] }) {
  const safeProviders = providers || [];
  return (
    <div className="max-w-2xl">
      <h2 style={{ color: "var(--color-text-primary, #E5E7EB)", fontSize: 18, fontWeight: 700, marginBottom: 20 }}>
        AI Provider Configuration
      </h2>
      <div className="flex flex-col gap-4">
        {safeProviders.length === 0 ? (
          <div className="text-center py-8" style={{ color: "var(--color-text-muted, #6B7280)", fontSize: 13 }}>
            No AI providers configured.
          </div>
        ) : safeProviders.map((p) => {
          const s = AI_STATUS_STYLES[p.status] ?? AI_STATUS_STYLES.inactive;
          return (
            <div
              key={p.id}
              className="rounded-2xl p-5"
              style={{
                background: "var(--color-bg-card, #151B2F)",
                border: `1px solid ${p.status === "active" ? "var(--color-status-success-border, rgba(16,185,129,0.25))" : "var(--color-border-subtle, rgba(255,255,255,0.07))"}`,
              }}
            >
              <div className="flex items-center justify-between mb-4">
                <div>
                  <div style={{ color: "var(--color-text-primary, #E5E7EB)", fontSize: 15, fontWeight: 600 }}>
                    {p.name}
                  </div>
                  <div style={{ color: "var(--color-text-muted, #6B7280)", fontSize: 12, marginTop: 2 }}>
                    Model: {p.model}
                  </div>
                </div>
                <span className="px-2.5 py-1 rounded-lg" style={{ ...s, fontSize: 12 }}>
                  {p.status}
                </span>
              </div>
              <div className="grid grid-cols-3 gap-3 mb-4">
                {[
                  { label: "Latency", value: p.latency ?? "—" },
                  { label: "Usage",   value: p.usage ?? "—" },
                  { label: "Cost",    value: p.cost ?? "—" },
                ].map(({ label, value }) => (
                  <div key={label} className="rounded-xl p-3" style={{ background: "var(--color-bg-input, rgba(255,255,255,0.04))" }}>
                    <div style={{ color: "var(--color-text-muted, #6B7280)", fontSize: 10 }}>{label}</div>
                    <div style={{ color: "var(--color-text-primary, #E5E7EB)", fontSize: 13, fontWeight: 600, marginTop: 2 }}>
                      {value}
                    </div>
                  </div>
                ))}
              </div>
              <div className="flex gap-2">
                <button
                  className="px-3 py-1.5 rounded-lg"
                  style={{ background: "var(--color-primary-bg, rgba(79,140,255,0.1))", color: "var(--color-primary, #4F8CFF)", border: "none", fontSize: 12, cursor: "pointer" }}
                >
                  Configure
                </button>
                {p.status !== "active" && (
                  <button
                    className="px-3 py-1.5 rounded-lg"
                    style={{ background: "var(--color-status-success-bg, rgba(16,185,129,0.1))", color: "var(--color-status-success, #10B981)", border: "none", fontSize: 12, cursor: "pointer" }}
                  >
                    Set Active
                  </button>
                )}
                <button
                  className="px-3 py-1.5 rounded-lg"
                  style={{ background: "var(--color-bg-input, rgba(255,255,255,0.05))", color: "var(--color-text-secondary, #9CA3AF)", border: "none", fontSize: 12, cursor: "pointer" }}
                >
                  Test
                </button>
              </div>
            </div>
          );
        })}
      </div>
    </div>
  );
}

function PlaceholderSection({ label }: { label: string }) {
  return (
    <div className="flex-1 flex items-center justify-center pt-20">
      <div className="text-center">
        <Settings size={40} color="var(--color-text-disabled, #374151)" style={{ margin: "0 auto 12px" }} />
        <p style={{ color: "var(--color-text-muted, #6B7280)", fontSize: 13 }}>{label} settings</p>
        <p style={{ color: "var(--color-text-faint, #4B5563)", fontSize: 12, marginTop: 4 }}>
          Configuration options available in production deployment
        </p>
      </div>
    </div>
  );
}

// ─── Main Component ───────────────────────────────────────────────────────────

export function SystemSettings() {
  const [section, setSection] = useState("general");
  const { data, isLoading, isError, refetch } = useSystemSettings();

  return (
    <div className="flex flex-1 overflow-hidden" style={{ background: "var(--color-bg-page, #0B1020)" }}>
      {/* Left nav */}
      <div
        className="w-52 flex-shrink-0 py-5 px-3 overflow-y-auto"
        style={{
          background: "var(--color-bg-sidebar, #0F1629)",
          borderRight: "1px solid var(--color-border-section, rgba(255,255,255,0.06))",
        }}
      >
        <div
          style={{
            color: "var(--color-text-muted, #6B7280)",
            fontSize: 10,
            fontWeight: 600,
            letterSpacing: 1,
            marginBottom: 10,
            paddingLeft: 8,
          }}
        >
          SETTINGS
        </div>
        {NAV_SECTIONS.map((s) => {
          const Icon = s.icon;
          return (
            <button
              key={s.id}
              id={`settings-nav-${s.id}`}
              onClick={() => setSection(s.id)}
              className="w-full flex items-center gap-2.5 px-3 py-2 rounded-lg mb-0.5 text-left transition-all"
              style={{
                background: section === s.id ? "var(--color-primary-bg, rgba(79,140,255,0.12))" : "transparent",
                color: section === s.id ? "var(--color-primary, #4F8CFF)" : "var(--color-text-secondary, #9CA3AF)",
                fontSize: 13,
                border: "none",
                cursor: "pointer",
              }}
            >
              <Icon size={14} />
              {s.label}
            </button>
          );
        })}
      </div>

      {/* Content */}
      <div className="flex-1 overflow-y-auto p-6">
        {isLoading && (
          <div className="flex items-center justify-center pt-20">
            <div className="flex flex-col items-center gap-3">
              <Loader2 size={28} className="animate-spin" style={{ color: "var(--color-primary, #4F8CFF)" }} />
              <p style={{ color: "var(--color-text-muted, #6B7280)", fontSize: 13 }}>Loading settings...</p>
            </div>
          </div>
        )}

        {isError && (
          <div className="flex items-center justify-center pt-20">
            <div className="text-center">
              <AlertTriangle size={32} style={{ color: "var(--color-status-error, #EF4444)", margin: "0 auto 12px" }} />
              <p style={{ color: "var(--color-text-secondary, #9CA3AF)", fontSize: 13 }}>Failed to load settings</p>
              <button
                onClick={() => refetch()}
                className="mt-3 px-4 py-2 rounded-xl flex items-center gap-2 mx-auto"
                style={{ background: "var(--color-primary-bg, rgba(79,140,255,0.1))", color: "var(--color-primary, #4F8CFF)", border: "none", cursor: "pointer", fontSize: 13 }}
              >
                <RefreshCw size={13} /> Retry
              </button>
            </div>
          </div>
        )}

        {data && (
          <>
            {section === "general" && <GeneralSection settings={data} />}
            {section === "ai" && <AISection providers={data.ai?.providers} />}
            {(section === "email" || section === "storage" || section === "security" || section === "notifications") && (
              <PlaceholderSection label={NAV_SECTIONS.find((s) => s.id === section)?.label ?? section} />
            )}
          </>
        )}
      </div>
    </div>
  );
}
