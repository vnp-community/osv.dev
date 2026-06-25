import { useState } from "react";
import { Key, Plus, Copy, Trash2, RotateCcw, Check, X, Shield, AlertTriangle, Loader2, RefreshCw } from "lucide-react";
import { useAPIKeys, useCreateAPIKey, useRevokeAPIKey, type APIKey } from "../hooks/useAPIKeys";

// ─── Scope colors ─────────────────────────────────────────────────────────────

const SCOPE_COLORS: Record<string, string> = {
  "scan:write":    "var(--color-primary, #4F8CFF)",
  "scan:read":     "#60A5FA",
  "finding:read":  "var(--color-status-warning, #F59E0B)",
  "finding:write": "#F97316",
  "report:read":   "var(--color-status-success, #10B981)",
  "dashboard:read": "var(--color-ai-accent, #A78BFA)",
  "agent:report":  "#34D399",
};

const ALL_SCOPES = ["scan:read", "scan:write", "finding:read", "finding:write", "report:read", "dashboard:read", "agent:report"];

function formatDate(iso?: string): string {
  if (!iso) return "Never";
  const d = new Date(iso);
  return d.toLocaleDateString("en-GB", { day: "numeric", month: "short", year: "numeric" });
}

function formatRelative(iso?: string): string {
  if (!iso) return "Never";
  const diff = Date.now() - new Date(iso).getTime();
  const mins = Math.floor(diff / 60_000);
  if (mins < 60) return `${mins}m ago`;
  const hrs = Math.floor(mins / 60);
  if (hrs < 24) return `${hrs}h ago`;
  return `${Math.floor(hrs / 24)}d ago`;
}

// ─── Create Key Modal ─────────────────────────────────────────────────────────

interface CreateKeyModalProps {
  onClose: () => void;
}

function CreateKeyModal({ onClose }: CreateKeyModalProps) {
  const [name, setName] = useState("");
  const [selectedScopes, setSelectedScopes] = useState<string[]>([]);
  const [plainKey, setPlainKey] = useState("");
  const [copied, setCopied] = useState(false);
  const createAPIKey = useCreateAPIKey();

  const handleGenerate = async () => {
    if (!name || selectedScopes.length === 0) return;
    const result = await createAPIKey.mutateAsync({ name, scopes: selectedScopes });
    setPlainKey(result.plain_key);
  };

  const handleCopy = () => {
    navigator.clipboard.writeText(plainKey);
    setCopied(true);
    setTimeout(() => setCopied(false), 1500);
  };

  return (
    <div
      className="fixed inset-0 z-50 flex items-center justify-center"
      style={{ background: "var(--color-bg-overlay, rgba(0,0,0,0.7))", backdropFilter: "blur(4px)" }}
    >
      <div
        className="w-full max-w-lg rounded-2xl p-6"
        style={{
          background: "var(--color-bg-card, #151B2F)",
          border: "1px solid var(--color-border-input, rgba(255,255,255,0.1))",
        }}
      >
        <div className="flex items-center justify-between mb-5">
          <h3 style={{ color: "var(--color-text-primary, #E5E7EB)", fontSize: 16, fontWeight: 600 }}>Create API Key</h3>
          <button onClick={onClose} style={{ color: "var(--color-text-muted, #6B7280)", background: "none", border: "none", cursor: "pointer" }}>
            <X size={18} />
          </button>
        </div>

        {!plainKey ? (
          <>
            <div className="mb-4">
              <label style={{ color: "var(--color-text-secondary, #9CA3AF)", fontSize: 13, display: "block", marginBottom: 6 }}>
                Key Name
              </label>
              <input
                id="api-key-name"
                value={name}
                onChange={(e) => setName(e.target.value)}
                placeholder="e.g. CI/CD Pipeline"
                className="w-full rounded-xl px-4 py-3 outline-none"
                style={{
                  background: "var(--color-bg-sidebar, #0F1629)",
                  border: "1px solid var(--color-border-input, rgba(255,255,255,0.09))",
                  color: "var(--color-text-primary, #E5E7EB)",
                  fontSize: 13,
                }}
              />
            </div>
            <div className="mb-5">
              <label style={{ color: "var(--color-text-secondary, #9CA3AF)", fontSize: 13, display: "block", marginBottom: 8 }}>
                Permissions
              </label>
              <div className="grid grid-cols-2 gap-2">
                {ALL_SCOPES.map((scope) => (
                  <div
                    key={scope}
                    onClick={() =>
                      setSelectedScopes((prev) =>
                        prev.includes(scope) ? prev.filter((s) => s !== scope) : [...prev, scope]
                      )
                    }
                    className="flex items-center gap-2 px-3 py-2 rounded-xl cursor-pointer"
                    style={{
                      background: selectedScopes.includes(scope)
                        ? SCOPE_COLORS[scope] + "15"
                        : "var(--color-bg-input, rgba(255,255,255,0.04))",
                      border: `1px solid ${selectedScopes.includes(scope) ? SCOPE_COLORS[scope] + "40" : "var(--color-border-card, rgba(255,255,255,0.07))"}`,
                    }}
                  >
                    <div
                      className="w-4 h-4 rounded flex items-center justify-center"
                      style={{ background: selectedScopes.includes(scope) ? SCOPE_COLORS[scope] : "var(--color-bg-input, rgba(255,255,255,0.1))" }}
                    >
                      {selectedScopes.includes(scope) && <Check size={10} color="white" />}
                    </div>
                    <span style={{ color: SCOPE_COLORS[scope] ?? "var(--color-text-secondary, #9CA3AF)", fontSize: 12 }}>
                      {scope}
                    </span>
                  </div>
                ))}
              </div>
            </div>
            <button
              onClick={handleGenerate}
              disabled={!name || selectedScopes.length === 0 || createAPIKey.isPending}
              className="w-full py-3 rounded-xl flex items-center justify-center gap-2"
              style={{
                background: !name || selectedScopes.length === 0
                  ? "var(--color-primary-bg, rgba(79,140,255,0.3))"
                  : "var(--color-primary-grad, linear-gradient(135deg, #4F8CFF, #3B6FCC))",
                color: "white",
                border: "none",
                fontSize: 14,
                cursor: !name || selectedScopes.length === 0 ? "not-allowed" : "pointer",
              }}
            >
              {createAPIKey.isPending && <Loader2 size={14} className="animate-spin" />}
              Generate API Key
            </button>
          </>
        ) : (
          <div>
            <div
              className="rounded-xl p-4 mb-4"
              style={{
                background: "var(--color-status-success-bg, rgba(16,185,129,0.08))",
                border: "1px solid var(--color-status-success-border, rgba(16,185,129,0.25))",
              }}
            >
              <div className="flex items-center gap-2 mb-2">
                <Shield size={14} color="var(--color-status-success, #10B981)" />
                <span style={{ color: "var(--color-status-success, #10B981)", fontSize: 13, fontWeight: 600 }}>
                  Key created — copy it now!
                </span>
              </div>
              <p style={{ color: "var(--color-text-secondary, #9CA3AF)", fontSize: 12 }}>
                This key will only be shown once. Store it securely.
              </p>
            </div>
            <div
              className="flex items-center gap-2 p-3 rounded-xl mb-4"
              style={{
                background: "var(--color-bg-sidebar, #0F1629)",
                border: "1px solid var(--color-border-input, rgba(255,255,255,0.09))",
              }}
            >
              <code style={{ color: "var(--color-status-success, #10B981)", fontSize: 12, flex: 1, fontFamily: "monospace", wordBreak: "break-all" }}>
                {plainKey}
              </code>
              <button
                onClick={handleCopy}
                style={{ color: copied ? "var(--color-status-success, #10B981)" : "var(--color-text-muted, #6B7280)", background: "none", border: "none", cursor: "pointer" }}
              >
                {copied ? <Check size={15} /> : <Copy size={15} />}
              </button>
            </div>
            <button
              onClick={onClose}
              className="w-full py-3 rounded-xl"
              style={{
                background: "var(--color-bg-input, rgba(255,255,255,0.07))",
                color: "var(--color-text-primary, #E5E7EB)",
                border: "none",
                fontSize: 14,
                cursor: "pointer",
              }}
            >
              Done
            </button>
          </div>
        )}
      </div>
    </div>
  );
}

// ─── Main Component ───────────────────────────────────────────────────────────

export function APIKeyManagement() {
  const [showModal, setShowModal] = useState(false);
  const { data, isLoading, isError, refetch } = useAPIKeys();
  const revokeAPIKey = useRevokeAPIKey();

  const keys: APIKey[] = data?.items ?? [];

  if (isLoading) {
    return (
      <div className="flex-1 flex items-center justify-center" style={{ background: "var(--color-bg-page, #0B1020)" }}>
        <div className="flex flex-col items-center gap-3">
          <Loader2 size={28} className="animate-spin" style={{ color: "var(--color-primary, #4F8CFF)" }} />
          <p style={{ color: "var(--color-text-muted, #6B7280)", fontSize: 13 }}>Loading API keys...</p>
        </div>
      </div>
    );
  }

  if (isError) {
    return (
      <div className="flex-1 flex items-center justify-center" style={{ background: "var(--color-bg-page, #0B1020)" }}>
        <div className="text-center">
          <AlertTriangle size={32} style={{ color: "var(--color-status-error, #EF4444)", margin: "0 auto 12px" }} />
          <p style={{ color: "var(--color-text-secondary, #9CA3AF)", fontSize: 13 }}>Failed to load API keys</p>
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

  return (
    <div className="flex-1 overflow-y-auto px-6 py-5" style={{ background: "var(--color-bg-page, #0B1020)" }}>
      {/* Header */}
      <div className="flex items-center justify-between mb-6">
        <div>
          <h2 style={{ color: "var(--color-text-primary, #E5E7EB)", fontSize: 18, fontWeight: 700 }}>
            API Key Management
          </h2>
          <p style={{ color: "var(--color-text-muted, #6B7280)", fontSize: 12 }}>
            Manage API keys for programmatic access · {data?.total ?? 0} total
          </p>
        </div>
        <button
          id="create-api-key-btn"
          onClick={() => setShowModal(true)}
          className="flex items-center gap-2 px-4 py-2 rounded-xl"
          style={{
            background: "var(--color-primary-grad, linear-gradient(135deg, #4F8CFF, #3B6FCC))",
            color: "white",
            border: "none",
            fontSize: 13,
            cursor: "pointer",
          }}
        >
          <Plus size={14} /> Create API Key
        </button>
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
              {["Name", "Key Prefix", "Scopes", "Created", "Last Used", "Expiration", "Status", ""].map((h) => (
                <th
                  key={h || "_actions"}
                  className="px-5 py-3 text-left"
                  style={{ color: "var(--color-text-muted, #6B7280)", fontSize: 11, fontWeight: 600, letterSpacing: 0.5 }}
                >
                  {h}
                </th>
              ))}
            </tr>
          </thead>
          <tbody>
            {keys.map((k, i) => (
              <tr
                key={k.id}
                className="transition-all"
                style={{ borderBottom: i < keys.length - 1 ? "1px solid var(--color-border-section, rgba(255,255,255,0.04))" : "none" }}
                onMouseEnter={(e) => (e.currentTarget.style.background = "var(--color-bg-hover, rgba(255,255,255,0.02))")}
                onMouseLeave={(e) => (e.currentTarget.style.background = "transparent")}
              >
                <td className="px-5 py-4">
                  <div className="flex items-center gap-2">
                    <Key size={13} color="var(--color-primary, #4F8CFF)" />
                    <span style={{ color: "var(--color-text-primary, #E5E7EB)", fontSize: 13, fontWeight: 500 }}>{k.name}</span>
                  </div>
                </td>
                <td className="px-5 py-4">
                  <span style={{ color: "var(--color-text-secondary, #9CA3AF)", fontSize: 12, fontFamily: "monospace" }}>
                    {k.prefix}•••••••
                  </span>
                </td>
                <td className="px-5 py-4">
                  <div className="flex gap-1 flex-wrap">
                    {k.scopes.map((s) => (
                      <span
                        key={s}
                        className="px-1.5 py-0.5 rounded"
                        style={{ background: (SCOPE_COLORS[s] ?? "#6B7280") + "20", color: SCOPE_COLORS[s] ?? "#9CA3AF", fontSize: 10 }}
                      >
                        {s}
                      </span>
                    ))}
                  </div>
                </td>
                <td className="px-5 py-4">
                  <span style={{ color: "var(--color-text-muted, #6B7280)", fontSize: 12 }}>
                    {formatDate(k.created_at)}
                  </span>
                </td>
                <td className="px-5 py-4">
                  <span style={{ color: "var(--color-text-muted, #6B7280)", fontSize: 12 }}>
                    {formatRelative(k.last_used_at)}
                  </span>
                </td>
                <td className="px-5 py-4">
                  <span
                    style={{
                      color: !k.expires_at
                        ? "var(--color-text-muted, #6B7280)"
                        : new Date(k.expires_at) < new Date()
                        ? "var(--color-status-error, #EF4444)"
                        : "var(--color-text-muted, #6B7280)",
                      fontSize: 12,
                    }}
                  >
                    {k.expires_at ? formatDate(k.expires_at) : "Never"}
                  </span>
                </td>
                <td className="px-5 py-4">
                  <span
                    className="px-2 py-0.5 rounded"
                    style={{
                      background: k.status === "active"
                        ? "var(--color-status-success-bg, rgba(16,185,129,0.1))"
                        : "var(--color-status-error-bg, rgba(239,68,68,0.1))",
                      color: k.status === "active"
                        ? "var(--color-status-success, #10B981)"
                        : "var(--color-status-error, #EF4444)",
                      fontSize: 11,
                    }}
                  >
                    {k.status}
                  </span>
                </td>
                <td className="px-5 py-4">
                  <div className="flex items-center gap-2">
                    <button
                      className="w-7 h-7 rounded-lg flex items-center justify-center"
                      style={{ background: "var(--color-primary-bg, rgba(79,140,255,0.1))", color: "var(--color-primary, #4F8CFF)", border: "none", cursor: "pointer" }}
                      title="Rotate"
                    >
                      <RotateCcw size={11} />
                    </button>
                    {k.status === "active" && (
                      <button
                        onClick={() => revokeAPIKey.mutate(k.id)}
                        className="w-7 h-7 rounded-lg flex items-center justify-center"
                        style={{ background: "var(--color-status-error-bg, rgba(239,68,68,0.1))", color: "var(--color-status-error, #EF4444)", border: "none", cursor: "pointer" }}
                        title="Revoke"
                      >
                        <Trash2 size={11} />
                      </button>
                    )}
                  </div>
                </td>
              </tr>
            ))}
          </tbody>
        </table>

        {keys.length === 0 && (
          <div className="text-center py-10">
            <p style={{ color: "var(--color-text-muted, #6B7280)", fontSize: 13 }}>No API keys found</p>
          </div>
        )}
      </div>

      {showModal && <CreateKeyModal onClose={() => setShowModal(false)} />}
    </div>
  );
}
