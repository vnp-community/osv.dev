import { useState } from "react";
import { Brain, CheckCircle, XCircle, Eye, AlertTriangle, Loader2, RefreshCw } from "lucide-react";
import { useAITriageQueue, useReviewTriage, type AITriageItem, type TriageStatus } from "../hooks/useAITriage";
import type { AxiosError } from 'axios';

// ─── Styling maps ─────────────────────────────────────────────────────────────

const VERDICT_COLORS: Record<string, string> = {
  "Patch Immediately": "var(--color-status-error, #EF4444)",
  "Schedule Patch":    "var(--color-status-warning, #F59E0B)",
  "Configure Auth":    "var(--color-primary, #4F8CFF)",
  "False Positive":    "var(--color-text-muted, #6B7280)",
  "Accept Risk":       "var(--color-ai-accent, #A78BFA)",
};

const STATUS_STYLES: Record<TriageStatus, { bg: string; color: string }> = {
  pending:  { bg: "var(--color-status-warning-bg, rgba(245,158,11,0.1))",  color: "var(--color-status-warning, #F59E0B)" },
  accepted: { bg: "var(--color-status-success-bg, rgba(16,185,129,0.1))",  color: "var(--color-status-success, #10B981)" },
  rejected: { bg: "var(--color-status-neutral-bg, rgba(107,114,128,0.1))", color: "var(--color-text-muted, #6B7280)" },
};

const SEVERITY_COLORS: Record<string, string> = {
  Critical: "var(--color-severity-critical, #EF4444)",
  High:     "var(--color-severity-high, #F97316)",
  Medium:   "var(--color-severity-medium, #EAB308)",
  Low:      "var(--color-severity-low, #3B82F6)",
};

function formatTimeAgo(iso: string): string {
  const diff = Date.now() - new Date(iso).getTime();
  const mins = Math.floor(diff / 60_000);
  if (mins < 60) return `${mins}m ago`;
  const hrs = Math.floor(mins / 60);
  if (hrs < 24) return `${hrs}h ago`;
  return `${Math.floor(hrs / 24)}d ago`;
}

// ─── Main Component ───────────────────────────────────────────────────────────

export function AITriage() {
  const [filter, setFilter] = useState<TriageStatus | "All">("All");
  const [selectedId, setSelectedId] = useState<string | null>(null);

  const { data, isLoading, isError, refetch } = useAITriageQueue();
  const reviewTriage = useReviewTriage();

  const allItems = data?.items ?? [];
  const filtered = filter === "All"
    ? allItems
    : allItems.filter((q) => q.status === filter.toLowerCase());

  const selected = filtered.find((q) => q.id === selectedId) ?? filtered[0] ?? null;

  if (isLoading) {
    return (
      <div className="flex-1 flex items-center justify-center" style={{ background: "var(--color-bg-page, #0B1020)" }}>
        <div className="flex flex-col items-center gap-3">
          <Loader2 size={28} className="animate-spin" style={{ color: "var(--color-ai-accent, #A78BFA)" }} />
          <p style={{ color: "var(--color-text-muted, #6B7280)", fontSize: 13 }}>Loading AI triage queue...</p>
        </div>
      </div>
    );
  }

  if (isError) {
    const is503 = (error as AxiosError)?.response?.status === 503;
    return (
      <div className="flex-1 flex items-center justify-center" style={{ background: "var(--color-bg-page, #0B1020)" }}>
        <div className="text-center">
          <AlertTriangle size={32} style={{ color: "var(--color-status-error, #EF4444)", margin: "0 auto 12px" }} />
          <p style={{ color: "var(--color-text-secondary, #9CA3AF)", fontSize: 13 }}>
            {is503 ? "AI Service Unavailable" : "Failed to load AI triage queue"}
          </p>
          <p style={{ color: "var(--color-text-muted, #6B7280)", fontSize: 12, marginTop: 4 }}>
            {is503
              ? "AI processing service is starting up. No action needed."
              : "Please try again."}
          </p>
          {!is503 && (
            <button
              onClick={() => refetch()}
              className="mt-3 px-4 py-2 rounded-xl flex items-center gap-2 mx-auto"
              style={{ background: "var(--color-primary-bg, rgba(79,140,255,0.1))", color: "var(--color-primary, #4F8CFF)", border: "none", cursor: "pointer", fontSize: 13 }}
            >
              <RefreshCw size={13} /> Retry
            </button>
          )}
        </div>
      </div>
    );
  }

  const pendingCount = data?.pending_count ?? allItems.filter((q) => q.status === "pending").length;
  const acceptedToday = data?.accepted_today ?? 0;
  const avgConfidence = data?.avg_confidence ?? 0;

  return (
    <div className="flex flex-col flex-1 overflow-hidden" style={{ background: "var(--color-bg-page, #0B1020)" }}>
      {/* Header */}
      <div className="px-6 py-4" style={{ borderBottom: "1px solid var(--color-border-section, rgba(255,255,255,0.06))" }}>
        <div className="flex items-center justify-between mb-4">
          <div className="flex items-center gap-3">
            <div
              className="w-10 h-10 rounded-xl flex items-center justify-center"
              style={{ background: "var(--color-ai-bg, rgba(124,58,237,0.2))" }}
            >
              <Brain size={20} color="var(--color-ai-accent, #A78BFA)" />
            </div>
            <div>
              <h2 style={{ color: "var(--color-text-primary, #E5E7EB)", fontSize: 18, fontWeight: 700 }}>
                AI Triage Center
              </h2>
              <p style={{ color: "var(--color-text-muted, #6B7280)", fontSize: 12 }}>
                {pendingCount} findings awaiting review
              </p>
            </div>
          </div>
          <div className="grid grid-cols-3 gap-3">
            {[
              { label: "Pending Review", value: pendingCount, color: "var(--color-status-warning, #F59E0B)" },
              { label: "Accepted Today", value: acceptedToday, color: "var(--color-status-success, #10B981)" },
              { label: "Avg Confidence", value: `${Math.round(avgConfidence)}%`, color: "var(--color-ai-accent, #A78BFA)" },
            ].map((s) => (
              <div
                key={s.label}
                className="rounded-xl px-4 py-2 text-center"
                style={{ background: "var(--color-bg-input, rgba(255,255,255,0.04))", border: "1px solid var(--color-border-card, rgba(255,255,255,0.07))" }}
              >
                <div style={{ color: s.color, fontSize: 18, fontWeight: 700 }}>{s.value}</div>
                <div style={{ color: "var(--color-text-muted, #6B7280)", fontSize: 11 }}>{s.label}</div>
              </div>
            ))}
          </div>
        </div>
        <div className="flex gap-2">
          {(["All", "Pending", "Accepted", "Rejected"] as const).map((f) => (
            <button
              key={f}
              onClick={() => setFilter(f === "All" ? "All" : (f.toLowerCase() as TriageStatus))}
              className="px-3 py-1.5 rounded-lg"
              style={{
                background: filter === (f === "All" ? "All" : f.toLowerCase())
                  ? "var(--color-ai-bg, rgba(167,139,250,0.12))"
                  : "var(--color-bg-input, rgba(255,255,255,0.05))",
                color: filter === (f === "All" ? "All" : f.toLowerCase())
                  ? "var(--color-ai-accent, #A78BFA)"
                  : "var(--color-text-muted, #6B7280)",
                fontSize: 12,
                border: "none",
                cursor: "pointer",
              }}
            >
              {f}
            </button>
          ))}
        </div>
      </div>

      <div className="flex flex-1 overflow-hidden">
        {/* Queue list */}
        <div
          className="w-96 flex-shrink-0 overflow-y-auto"
          style={{ borderRight: "1px solid var(--color-border-section, rgba(255,255,255,0.06))" }}
        >
          {filtered.map((item) => (
            <div
              key={item.id}
              onClick={() => setSelectedId(item.id)}
              className="p-4 cursor-pointer transition-all"
              style={{
                borderBottom: "1px solid var(--color-border-section, rgba(255,255,255,0.05))",
                background: selected?.id === item.id ? "var(--color-ai-bg, rgba(167,139,250,0.07))" : "transparent",
                borderLeft: selected?.id === item.id ? "2px solid var(--color-ai-accent, #A78BFA)" : "2px solid transparent",
              }}
              onMouseEnter={(e) => { if (selected?.id !== item.id) e.currentTarget.style.background = "var(--color-bg-hover, rgba(255,255,255,0.02))"; }}
              onMouseLeave={(e) => { if (selected?.id !== item.id) e.currentTarget.style.background = "transparent"; }}
            >
              <div className="flex items-start justify-between mb-2">
                <div className="flex items-center gap-2">
                  <span
                    className="px-2 py-0.5 rounded"
                    style={{ background: SEVERITY_COLORS[item.severity] + "20", color: SEVERITY_COLORS[item.severity], fontSize: 11 }}
                  >
                    {item.severity}
                  </span>
                  <span style={{ color: "var(--color-text-muted, #6B7280)", fontSize: 11 }}>{item.finding_id}</span>
                </div>
                <span className="px-2 py-0.5 rounded" style={{ ...STATUS_STYLES[item.status], fontSize: 11 }}>
                  {item.status}
                </span>
              </div>
              <div style={{ color: "var(--color-text-primary, #E5E7EB)", fontSize: 13 }} className="mb-2">
                {item.title}
              </div>
              <div className="flex items-center justify-between">
                <div className="flex items-center gap-2">
                  <Brain size={11} color={VERDICT_COLORS[item.verdict] ?? "var(--color-text-muted, #6B7280)"} />
                  <span style={{ color: VERDICT_COLORS[item.verdict] ?? "var(--color-text-muted, #6B7280)", fontSize: 12 }}>
                    {item.verdict}
                  </span>
                </div>
                <div className="flex items-center gap-1">
                  <div
                    className="w-12 h-1.5 rounded-full overflow-hidden"
                    style={{ background: "var(--color-bg-input, rgba(255,255,255,0.1))" }}
                  >
                    <div
                      className="h-full rounded-full"
                      style={{
                        width: `${item.confidence}%`,
                        background: item.confidence > 90
                          ? "var(--color-status-success, #10B981)"
                          : "var(--color-status-warning, #F59E0B)",
                      }}
                    />
                  </div>
                  <span style={{ color: "var(--color-text-muted, #6B7280)", fontSize: 11 }}>{item.confidence}%</span>
                </div>
              </div>
              <div style={{ color: "var(--color-text-faint, #4B5563)", fontSize: 10, marginTop: 6 }}>
                {formatTimeAgo(item.created_at)}
              </div>
            </div>
          ))}

          {filtered.length === 0 && (
            <div className="text-center py-10">
              <p style={{ color: "var(--color-text-muted, #6B7280)", fontSize: 13 }}>No items in queue</p>
            </div>
          )}
        </div>

        {/* Detail panel */}
        {selected && (
          <div className="flex-1 overflow-y-auto p-6">
            <div className="max-w-2xl">
              {/* Title */}
              <div className="flex items-center gap-3 mb-5">
                <span
                  className="px-2.5 py-1 rounded-lg"
                  style={{ background: SEVERITY_COLORS[selected.severity] + "20", color: SEVERITY_COLORS[selected.severity], fontSize: 13, fontWeight: 600 }}
                >
                  {selected.severity}
                </span>
                <h3 style={{ color: "var(--color-text-primary, #E5E7EB)", fontSize: 16, fontWeight: 600 }}>
                  {selected.title}
                </h3>
              </div>

              {/* AI Verdict */}
              <div
                className="rounded-2xl p-5 mb-5"
                style={{
                  background: "var(--color-ai-bg, rgba(124,58,237,0.08))",
                  border: "1px solid var(--color-ai-border, rgba(124,58,237,0.2))",
                }}
              >
                <div className="flex items-center gap-2 mb-3">
                  <Brain size={16} color="var(--color-ai-accent, #A78BFA)" />
                  <span style={{ color: "var(--color-ai-accent, #A78BFA)", fontSize: 14, fontWeight: 600 }}>AI Verdict</span>
                  <div className="ml-auto flex items-center gap-2">
                    <div
                      className="w-20 h-2 rounded-full overflow-hidden"
                      style={{ background: "var(--color-bg-input, rgba(255,255,255,0.1))" }}
                    >
                      <div
                        className="h-full rounded-full"
                        style={{
                          width: `${selected.confidence}%`,
                          background: selected.confidence > 90
                            ? "var(--color-status-success, #10B981)"
                            : "var(--color-status-warning, #F59E0B)",
                        }}
                      />
                    </div>
                    <span style={{ color: "var(--color-ai-accent, #A78BFA)", fontSize: 13, fontWeight: 700 }}>
                      {selected.confidence}% confidence
                    </span>
                  </div>
                </div>
                <div className="text-xl font-bold mb-3" style={{ color: VERDICT_COLORS[selected.verdict] ?? "var(--color-text-primary, #E5E7EB)" }}>
                  {selected.verdict}
                </div>
                <p style={{ color: "#C4B5FD", fontSize: 13, lineHeight: 1.7 }}>{selected.reasoning}</p>
              </div>

              {/* Suggested fixes */}
              {selected.suggested_fixes.length > 0 && (
                <div
                  className="rounded-2xl p-5 mb-5"
                  style={{
                    background: "var(--color-bg-card, #151B2F)",
                    border: "1px solid var(--color-border-subtle, rgba(255,255,255,0.07))",
                  }}
                >
                  <div style={{ color: "var(--color-text-secondary, #9CA3AF)", fontSize: 12, fontWeight: 600, marginBottom: 12 }}>
                    SUGGESTED REMEDIATION STEPS
                  </div>
                  {selected.suggested_fixes.map((fix, i) => (
                    <div key={i} className="flex items-start gap-3 mb-3">
                      <div
                        className="w-5 h-5 rounded-full flex items-center justify-center flex-shrink-0 mt-0.5"
                        style={{ background: "var(--color-status-success-bg, rgba(16,185,129,0.2))", color: "var(--color-status-success, #10B981)", fontSize: 10, fontWeight: 700 }}
                      >
                        {i + 1}
                      </div>
                      <span style={{ color: "var(--color-text-primary, #E5E7EB)", fontSize: 13 }}>{fix}</span>
                    </div>
                  ))}
                </div>
              )}

              {/* Action buttons */}
              {selected.status === "pending" && (
                <div className="flex items-center gap-3">
                  <button
                    onClick={() => reviewTriage.mutate({ findingId: selected.finding_id, decision: "accepted" })}
                    disabled={reviewTriage.isPending}
                    className="flex items-center gap-2 px-5 py-2.5 rounded-xl flex-1 justify-center"
                    style={{
                      background: "var(--color-status-success-bg, rgba(16,185,129,0.1))",
                      border: "1px solid var(--color-status-success-border, rgba(16,185,129,0.25))",
                      color: "var(--color-status-success, #10B981)",
                      fontSize: 14,
                      fontWeight: 500,
                      cursor: reviewTriage.isPending ? "not-allowed" : "pointer",
                    }}
                  >
                    <CheckCircle size={15} /> Accept Recommendation
                  </button>
                  <button
                    onClick={() => reviewTriage.mutate({ findingId: selected.finding_id, decision: "rejected" })}
                    disabled={reviewTriage.isPending}
                    className="flex items-center gap-2 px-5 py-2.5 rounded-xl flex-1 justify-center"
                    style={{
                      background: "var(--color-status-error-bg, rgba(239,68,68,0.1))",
                      border: "1px solid var(--color-status-error-border, rgba(239,68,68,0.25))",
                      color: "var(--color-status-error, #EF4444)",
                      fontSize: 14,
                      fontWeight: 500,
                      cursor: reviewTriage.isPending ? "not-allowed" : "pointer",
                    }}
                  >
                    <XCircle size={15} /> Reject
                  </button>
                  <button
                    className="flex items-center gap-2 px-5 py-2.5 rounded-xl"
                    style={{
                      background: "var(--color-bg-input, rgba(255,255,255,0.05))",
                      border: "1px solid var(--color-border-card, rgba(255,255,255,0.09))",
                      color: "var(--color-text-secondary, #9CA3AF)",
                      fontSize: 14,
                      cursor: "pointer",
                    }}
                  >
                    <Eye size={15} /> Manual Review
                  </button>
                </div>
              )}
              {selected.status !== "pending" && (
                <div
                  className="rounded-xl p-4"
                  style={{ background: STATUS_STYLES[selected.status].bg, border: `1px solid ${STATUS_STYLES[selected.status].color}30` }}
                >
                  <span style={{ color: STATUS_STYLES[selected.status].color, fontSize: 13 }}>
                    This recommendation has been {selected.status}.
                  </span>
                </div>
              )}
            </div>
          </div>
        )}
      </div>
    </div>
  );
}
