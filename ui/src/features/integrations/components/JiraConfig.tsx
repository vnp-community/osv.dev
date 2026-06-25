import { useState } from "react";
import { Link2, CheckCircle, AlertTriangle, Loader2, Key } from "lucide-react";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { apiClient } from "@/shared/api/client";
import { ENDPOINTS } from "@/shared/api/endpoints";

// ── Types ────────────────────────────────────────────────────────────────────

interface JiraConfigData {
  jira_url: string;
  project_key: string;
  is_connected: boolean;
  last_synced_at?: string;
}

interface JiraConfigInput {
  jira_url: string;
  project_key: string;
  api_token: string;
  user_email: string;
}

// ── Hooks ────────────────────────────────────────────────────────────────────

const jiraKeys = {
  all: ["integrations", "jira"] as const,
};

function useJiraConfig() {
  return useQuery<JiraConfigData>({
    queryKey: jiraKeys.all,
    queryFn: async () => {
      const { data } = await apiClient.get<JiraConfigData>(ENDPOINTS.integrations.jira);
      return data;
    },
    staleTime: 5 * 60_000,
  });
}

function useUpdateJiraConfig() {
  const queryClient = useQueryClient();
  return useMutation<JiraConfigData, Error, JiraConfigInput>({
    mutationFn: async (payload) => {
      const { data } = await apiClient.put<JiraConfigData>(ENDPOINTS.integrations.jira, payload);
      return data;
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: jiraKeys.all });
    },
  });
}

// ── Component ─────────────────────────────────────────────────────────────────

export function JiraConfig() {
  const { data: config, isLoading, isError } = useJiraConfig();
  const { mutate: updateConfig, isPending, isSuccess, error } = useUpdateJiraConfig();

  const [jiraUrl, setJiraUrl]       = useState("");
  const [projectKey, setProjectKey] = useState("");
  const [apiToken, setApiToken]     = useState("");
  const [userEmail, setUserEmail]   = useState("");

  // Pre-fill form from server once loaded
  const initialUrl       = config?.jira_url      ?? "";
  const initialProject   = config?.project_key   ?? "";
  const effectiveUrl     = jiraUrl     || initialUrl;
  const effectiveProject = projectKey  || initialProject;

  const handleSave = () => {
    if (!effectiveUrl || !effectiveProject || !apiToken || !userEmail) return;
    updateConfig({
      jira_url:    effectiveUrl,
      project_key: effectiveProject,
      api_token:   apiToken,
      user_email:  userEmail,
    });
  };

  return (
    <div className="flex-1 overflow-y-auto px-6 py-5" style={{ background: "var(--color-bg-page, #0B1020)" }}>
      {/* Header */}
      <div className="flex items-center gap-3 mb-6">
        <div
          className="w-10 h-10 rounded-xl flex items-center justify-center"
          style={{ background: "rgba(79,140,255,0.12)" }}
        >
          <Link2 size={20} color="#4F8CFF" />
        </div>
        <div>
          <h2 style={{ color: "var(--color-text-primary, #E5E7EB)", fontSize: 18, fontWeight: 700 }}>
            Jira Integration
          </h2>
          <p style={{ color: "var(--color-text-muted, #6B7280)", fontSize: 12, marginTop: 2 }}>
            Sync findings with your Jira project as issues
          </p>
        </div>
        {config?.is_connected && (
          <span
            className="ml-auto flex items-center gap-1.5 px-3 py-1.5 rounded-full"
            style={{ background: "rgba(16,185,129,0.1)", color: "#10B981", fontSize: 12 }}
          >
            <CheckCircle size={12} /> Connected
          </span>
        )}
      </div>

      {/* Loading / Error states */}
      {isLoading && (
        <div className="flex items-center justify-center py-20">
          <Loader2 size={24} className="animate-spin" style={{ color: "#4F8CFF" }} />
        </div>
      )}
      {isError && (
        <div
          className="rounded-2xl p-5 mb-5 flex items-center gap-3"
          style={{ background: "rgba(239,68,68,0.08)", border: "1px solid rgba(239,68,68,0.2)" }}
        >
          <AlertTriangle size={16} color="#EF4444" />
          <span style={{ color: "#9CA3AF", fontSize: 13 }}>
            Failed to load Jira configuration. Please try again.
          </span>
        </div>
      )}

      {/* Config form */}
      {!isLoading && (
        <div
          className="rounded-2xl p-6"
          style={{ background: "var(--color-bg-card, #151B2F)", border: "1px solid rgba(255,255,255,0.07)", maxWidth: 600 }}
        >
          <div style={{ color: "var(--color-text-secondary, #9CA3AF)", fontSize: 12, fontWeight: 600, marginBottom: 16, letterSpacing: 0.5 }}>
            CONNECTION SETTINGS
          </div>

          <div className="flex flex-col gap-4">
            {[
              { id: "jira-url",      label: "Jira URL",     placeholder: "https://yourcompany.atlassian.net", value: effectiveUrl,     onChange: setJiraUrl,     type: "url" },
              { id: "project-key",   label: "Project Key",  placeholder: "SEC",                               value: effectiveProject, onChange: setProjectKey,  type: "text" },
              { id: "user-email",    label: "User Email",   placeholder: "you@company.com",                  value: userEmail,        onChange: setUserEmail,   type: "email" },
            ].map(({ id, label, placeholder, value, onChange, type }) => (
              <div key={id}>
                <label
                  htmlFor={id}
                  style={{ color: "var(--color-text-secondary, #9CA3AF)", fontSize: 12, display: "block", marginBottom: 6 }}
                >
                  {label}
                </label>
                <input
                  id={id}
                  type={type}
                  value={value}
                  onChange={(e) => onChange(e.target.value)}
                  placeholder={placeholder}
                  className="w-full rounded-xl px-4 py-2.5 outline-none"
                  style={{
                    background: "var(--color-bg-sidebar, #0F1629)",
                    border: "1px solid rgba(255,255,255,0.08)",
                    color: "var(--color-text-primary, #E5E7EB)",
                    fontSize: 13,
                  }}
                />
              </div>
            ))}

            {/* API Token — secret field */}
            <div>
              <label
                htmlFor="jira-api-token"
                style={{ color: "var(--color-text-secondary, #9CA3AF)", fontSize: 12, display: "block", marginBottom: 6 }}
              >
                <span className="flex items-center gap-1.5">
                  <Key size={11} /> API Token
                  <span style={{ color: "#6B7280", fontWeight: 400 }}>(stored encrypted, not shown after save)</span>
                </span>
              </label>
              <input
                id="jira-api-token"
                type="password"
                value={apiToken}
                onChange={(e) => setApiToken(e.target.value)}
                placeholder="••••••••••••••••••••"
                className="w-full rounded-xl px-4 py-2.5 outline-none"
                style={{
                  background: "var(--color-bg-sidebar, #0F1629)",
                  border: "1px solid rgba(255,255,255,0.08)",
                  color: "var(--color-text-primary, #E5E7EB)",
                  fontSize: 13,
                }}
              />
            </div>
          </div>

          {/* Success / Error feedback */}
          {isSuccess && (
            <div
              className="flex items-center gap-2 mt-4 px-4 py-3 rounded-xl"
              style={{ background: "rgba(16,185,129,0.08)", border: "1px solid rgba(16,185,129,0.2)" }}
            >
              <CheckCircle size={14} color="#10B981" />
              <span style={{ color: "#10B981", fontSize: 13 }}>Configuration saved and connection verified.</span>
            </div>
          )}
          {error && (
            <div
              className="flex items-center gap-2 mt-4 px-4 py-3 rounded-xl"
              style={{ background: "rgba(239,68,68,0.08)", border: "1px solid rgba(239,68,68,0.2)" }}
            >
              <AlertTriangle size={14} color="#EF4444" />
              <span style={{ color: "#EF4444", fontSize: 13 }}>{error.message}</span>
            </div>
          )}

          {/* Save button */}
          <button
            id="jira-save-btn"
            onClick={handleSave}
            disabled={isPending || !effectiveUrl || !effectiveProject || !apiToken || !userEmail}
            className="mt-5 w-full py-3 rounded-xl flex items-center justify-center gap-2"
            style={{
              background: "linear-gradient(135deg, #4F8CFF, #3B6FCC)",
              color: "white",
              border: "none",
              fontSize: 14,
              fontWeight: 600,
              cursor: isPending ? "not-allowed" : "pointer",
              opacity: (!effectiveUrl || !effectiveProject || !apiToken || !userEmail) ? 0.5 : 1,
            }}
          >
            {isPending ? <><Loader2 size={16} className="animate-spin" /> Saving…</> : "Save Configuration"}
          </button>
        </div>
      )}

      {/* Last sync info */}
      {config?.last_synced_at && (
        <p style={{ color: "var(--color-text-muted, #6B7280)", fontSize: 12, marginTop: 12 }}>
          Last synced: {new Date(config.last_synced_at).toLocaleString()}
        </p>
      )}
    </div>
  );
}
