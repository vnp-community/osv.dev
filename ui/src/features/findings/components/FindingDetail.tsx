import { useState } from "react";
import { useParams, useNavigate } from "react-router";
import {
  ArrowLeft, Brain, Clock, CheckCircle, AlertCircle,
  ExternalLink, MessageSquare, ChevronRight, AlertTriangle,
} from "lucide-react";
import { useFindingDetail } from "@/features/findings/hooks/useFindings";
import { useUpdateFinding } from "@/features/findings/hooks/useUpdateFinding";
import { QueryBoundary } from "@/shared/components/feedback/QueryBoundary";
import type { FindingStatus } from "@/shared/types/finding";
import type { Finding } from "@/shared/types/finding";

const SEVERITY_COLORS: Record<string, string> = {
  Critical: "#EF4444", High: "#F97316", Medium: "#EAB308", Low: "#3B82F6",
};

const TABS = ["Overview", "Evidence", "Timeline", "Audit Trail", "Comments"] as const;
type Tab = typeof TABS[number];

// ── Props version (backward compat when called without routing) ──────────────
interface FindingDetailProps {
  findingId?: string;
  onBack?: () => void;
}

function FindingDetailContent({
  finding,
  onBack,
}: {
  finding: Finding;
  onBack: () => void;
}) {
  const [activeTab, setActiveTab] = useState<Tab>("Overview");
  const [comment, setComment] = useState("");
  const updateMutation = useUpdateFinding();

  const handleStatusChange = (status: FindingStatus) => {
    updateMutation.mutate({ id: finding.id, status });
  };

  const severityColor = SEVERITY_COLORS[finding.severity] ?? "#6B7280";

  return (
    <div className="flex flex-col flex-1 overflow-hidden" style={{ background: "var(--color-bg-page, #0B1020)" }}>
      {/* Header */}
      <div className="px-6 py-4" style={{ background: "var(--color-bg-sidebar, #0F1629)", borderBottom: "1px solid rgba(255,255,255,0.06)" }}>
        <div className="flex items-center gap-3 mb-4">
          <button
            id="finding-detail-back"
            onClick={onBack}
            className="w-8 h-8 rounded-lg flex items-center justify-center"
            style={{ background: "rgba(255,255,255,0.05)", color: "var(--color-text-secondary, #9CA3AF)", border: "none", cursor: "pointer" }}
          >
            <ArrowLeft size={15} />
          </button>
          <div className="flex items-center gap-3 flex-1">
            <span style={{ color: "var(--color-text-muted, #6B7280)", fontSize: 13 }}>{finding.id}</span>
            <ChevronRight size={13} color="#4B5563" />
            <h2 style={{ color: "var(--color-text-primary, #E5E7EB)", fontSize: 16, fontWeight: 700 }}>{finding.title}</h2>
            <span className="px-2.5 py-1 rounded-lg" style={{ background: severityColor + "25", color: severityColor, fontSize: 13, fontWeight: 700 }}>
              {finding.severity}
            </span>
            {finding.isKEV && (
              <span className="px-2 py-0.5 rounded" style={{ background: "rgba(239,68,68,0.1)", color: "var(--color-status-error, #EF4444)", fontSize: 12 }}>CISA KEV</span>
            )}
          </div>
          {/* Actions */}
          <div className="flex items-center gap-2">
            {([
              { label: "Mitigate", status: "mitigated" as FindingStatus, color: "var(--color-status-success, #10B981)", bg: "rgba(16,185,129,0.1)", border: "rgba(16,185,129,0.25)" },
              { label: "Accept Risk", status: "risk_accepted" as FindingStatus, color: "var(--color-ai, #A78BFA)", bg: "rgba(167,139,250,0.1)", border: "rgba(167,139,250,0.25)" },
              { label: "False Positive", status: "false_positive" as FindingStatus, color: "var(--color-text-muted, #6B7280)", bg: "rgba(107,114,128,0.1)", border: "rgba(107,114,128,0.2)" },
            ]).map(({ label, status, color, bg, border }) => (
              <button
                key={label}
                id={`finding-action-${status}`}
                disabled={updateMutation.isPending}
                onClick={() => handleStatusChange(status)}
                className="px-4 py-2 rounded-xl"
                style={{ background: bg, border: `1px solid ${border}`, color, fontSize: 13, cursor: "pointer", opacity: updateMutation.isPending ? 0.6 : 1 }}
              >
                {label}
              </button>
            ))}
          </div>
        </div>
        {/* Tabs */}
        <div className="flex gap-1">
          {TABS.map((tab) => (
            <button
              key={tab}
              id={`finding-tab-${tab.toLowerCase().replace(/\s/g, '-')}`}
              onClick={() => setActiveTab(tab)}
              className="px-4 py-2 rounded-lg"
              style={{
                background: activeTab === tab ? "rgba(79,140,255,0.12)" : "transparent",
                color: activeTab === tab ? "#4F8CFF" : "#6B7280",
                fontSize: 13, border: "none", cursor: "pointer",
                borderBottom: activeTab === tab ? "2px solid #4F8CFF" : "2px solid transparent",
              }}
            >
              {tab}
            </button>
          ))}
        </div>
      </div>

      <div className="flex-1 overflow-y-auto p-6">
        {activeTab === "Overview" && (
          <div className="grid grid-cols-3 gap-5">
            {/* Left: main info */}
            <div className="col-span-2 flex flex-col gap-5">
              {/* Metrics */}
              <div className="grid grid-cols-4 gap-4">
                {[
                  { label: "CVSS Score", value: finding.cvssV3?.toFixed(1) ?? "N/A", color: severityColor, sub: finding.severity },
                  { label: "EPSS Score", value: finding.epssScore ? `${(finding.epssScore * 100).toFixed(1)}%` : "N/A", color: "var(--color-status-error, #EF4444)", sub: "Exploit prediction" },
                  { label: "SLA Status", value: finding.slaStatus ?? "—", color: finding.slaStatus === "breached" ? "#EF4444" : "#F59E0B", sub: finding.slaDaysLeft != null ? `${finding.slaDaysLeft}d remaining` : "SLA tracking" },
                  { label: "Status", value: finding.status, color: "var(--color-primary, #4F8CFF)", sub: "Current status" },
                ].map((m) => (
                  <div key={m.label} className="rounded-2xl p-4" style={{ background: "var(--color-bg-card, #151B2F)", border: "1px solid rgba(255,255,255,0.07)" }}>
                    <div style={{ color: "var(--color-text-muted, #6B7280)", fontSize: 11, marginBottom: 6 }}>{m.label}</div>
                    <div style={{ color: m.color, fontSize: 22, fontWeight: 700 }}>{m.value}</div>
                    <div style={{ color: "var(--color-text-muted, #6B7280)", fontSize: 11, marginTop: 3 }}>{m.sub}</div>
                  </div>
                ))}
              </div>

              {/* Description */}
              {finding.description && (
                <div className="rounded-2xl p-5" style={{ background: "var(--color-bg-card, #151B2F)", border: "1px solid rgba(255,255,255,0.07)" }}>
                  <div style={{ color: "var(--color-text-secondary, #9CA3AF)", fontSize: 12, fontWeight: 600, marginBottom: 12 }}>DESCRIPTION</div>
                  <p style={{ color: "var(--color-text-primary, #E5E7EB)", fontSize: 13, lineHeight: 1.7 }}>{finding.description}</p>
                </div>
              )}

              {/* AI Section */}
              <div className="rounded-2xl p-5" style={{ background: "var(--color-bg-card, #151B2F)", border: "1px solid rgba(167,139,250,0.2)" }}>
                <div className="flex items-center gap-2 mb-4">
                  <Brain size={16} color="#A78BFA" />
                  <span style={{ color: "var(--color-ai, #A78BFA)", fontSize: 14, fontWeight: 600 }}>AI Triage Analysis</span>
                  <span className="ml-auto px-2 py-0.5 rounded-lg" style={{ background: "rgba(16,185,129,0.1)", color: "var(--color-status-success, #10B981)", fontSize: 12 }}>
                    {finding.aiTriageResult?.confidence ? `Confidence: ${Math.round(finding.aiTriageResult.confidence * 100)}%` : "Pending"}
                  </span>
                </div>
                {finding.aiTriageResult?.justification && (
                  <div className="rounded-xl p-4 mb-4" style={{ background: "rgba(124,58,237,0.08)", border: "1px solid rgba(124,58,237,0.15)" }}>
                    <p style={{ color: "#C4B5FD", fontSize: 13, lineHeight: 1.7 }}>{finding.aiTriageResult.justification}</p>
                  </div>
                )}
                {finding.aiTriageResult?.actions && finding.aiTriageResult.actions.length > 0 && (
                  <>
                    <div style={{ color: "var(--color-text-secondary, #9CA3AF)", fontSize: 12, fontWeight: 600, marginBottom: 8 }}>SUGGESTED ACTIONS</div>
                    {finding.aiTriageResult.actions.map((action, i) => (
                      <div key={i} className="flex items-start gap-2 mb-2">
                        <CheckCircle size={13} color="#10B981" style={{ marginTop: 2, flexShrink: 0 }} />
                        <span style={{ color: "var(--color-text-primary, #E5E7EB)", fontSize: 12 }}>{action}</span>
                      </div>
                    ))}
                  </>
                )}
              </div>
            </div>

            {/* Right: sidebar info */}
            <div className="flex flex-col gap-4">
              <div className="rounded-2xl p-4" style={{ background: "var(--color-bg-card, #151B2F)", border: "1px solid rgba(255,255,255,0.07)" }}>
                <div style={{ color: "var(--color-text-secondary, #9CA3AF)", fontSize: 11, fontWeight: 600, marginBottom: 10 }}>DETAILS</div>
                {[
                  { label: "CVE", value: finding.cveId ?? "N/A", highlight: !!finding.cveId },
                  { label: "Severity", value: finding.severity },
                  { label: "Product", value: finding.productName ?? "—" },
                  { label: "Assignee", value: finding.assignedTo ?? "Unassigned" },
                  { label: "Found", value: finding.createdAt ? new Date(finding.createdAt).toLocaleDateString() : "—" },
                  { label: "SLA Due", value: finding.slaExpirationDate ? new Date(finding.slaExpirationDate).toLocaleDateString() : "—" },
                  { label: "JIRA", value: finding.jiraIssueKey ?? "—" },
                ].map(({ label, value, highlight }) => (
                  <div key={label} className="flex justify-between py-2" style={{ borderBottom: "1px solid rgba(255,255,255,0.04)" }}>
                    <span style={{ color: "var(--color-text-muted, #6B7280)", fontSize: 12 }}>{label}</span>
                    <span style={{ color: highlight ? "#4F8CFF" : "#E5E7EB", fontSize: 12, fontFamily: highlight ? "monospace" : undefined }}>{value}</span>
                  </div>
                ))}
              </div>

              {/* No references field in Finding type - show JIRA link if available */}
              {finding.jiraUrl && (
                <div className="rounded-2xl p-4" style={{ background: "var(--color-bg-card, #151B2F)", border: "1px solid rgba(255,255,255,0.07)" }}>
                  <div style={{ color: "var(--color-text-secondary, #9CA3AF)", fontSize: 11, fontWeight: 600, marginBottom: 10 }}>REFERENCES</div>
                  <div className="flex items-center gap-2 py-2">
                    <ExternalLink size={11} color="#4F8CFF" />
                    <a href={finding.jiraUrl} target="_blank" rel="noopener noreferrer" style={{ color: "var(--color-primary, #4F8CFF)", fontSize: 12 }}>
                      {finding.jiraIssueKey ?? "JIRA Issue"}
                    </a>
                  </div>
                </div>
              )}
            </div>
          </div>
        )}

        {activeTab === "Evidence" && (
          <div className="flex flex-col gap-5">
              {/* Evidence: stored in description if no dedicated field */}
              <div className="rounded-2xl p-5" style={{ background: "var(--color-bg-card, #151B2F)", border: "1px solid rgba(255,255,255,0.07)" }}>
                <div style={{ color: "var(--color-text-secondary, #9CA3AF)", fontSize: 12, fontWeight: 600, marginBottom: 12 }}>SCAN EVIDENCE</div>
                <div style={{ color: "var(--color-text-muted, #6B7280)", fontSize: 13 }}>
                  Detailed evidence is available in the backend scan report for this finding.
                </div>
              </div>
          </div>
        )}

        {activeTab === "Audit Trail" && (
          <div className="max-w-2xl">
            <div className="flex flex-col gap-4">
              {(finding.vexJustification ? [
                { user: 'System', action: `VEX Justification: ${finding.vexJustification}`, time: finding.createdAt, type: 'system' },
              ] : []).map((entry, i) => (
                <div key={i} className="flex items-start gap-4">
                  <div
                    className="w-8 h-8 rounded-full flex items-center justify-center flex-shrink-0 text-white"
                    style={{ background: entry.type === "ai" ? "#7C3AED" : entry.type === "system" ? "#374151" : "#4F8CFF", fontSize: 10, fontWeight: 700 }}
                  >
                    {entry.type === "ai" ? "AI" : entry.type === "system" ? "SYS" : (entry.user ?? "?").split(" ").map((n: string) => n[0]).join("")}
                  </div>
                  <div className="flex-1 rounded-xl p-3" style={{ background: "var(--color-bg-card, #151B2F)", border: "1px solid rgba(255,255,255,0.07)" }}>
                    <div className="flex items-center justify-between mb-1">
                      <span style={{ color: "var(--color-text-primary, #E5E7EB)", fontSize: 13, fontWeight: 500 }}>{entry.user}</span>
                      <span style={{ color: "var(--color-text-muted, #6B7280)", fontSize: 11 }}>{entry.time ? new Date(entry.time).toLocaleString() : "—"}</span>
                    </div>
                    <p style={{ color: "var(--color-text-secondary, #9CA3AF)", fontSize: 12 }}>{entry.action}</p>
                  </div>
                </div>
              ))}
              {(!finding.vexJustification) && (
                <div style={{ color: "var(--color-text-muted, #6B7280)", fontSize: 13 }}>No audit trail available.</div>
              )}
            </div>
          </div>
        )}

        {activeTab === "Comments" && (
          <div className="max-w-2xl flex flex-col gap-4">
            <div className="flex gap-3">
              <div className="w-8 h-8 rounded-full flex items-center justify-center flex-shrink-0 text-white" style={{ background: "linear-gradient(135deg, #4F8CFF, #7C3AED)", fontSize: 10, fontWeight: 700 }}>ME</div>
              <div className="flex-1">
                <textarea
                  id="finding-comment-input"
                  value={comment}
                  onChange={(e) => setComment(e.target.value)}
                  placeholder="Add a comment..."
                  rows={3}
                  className="w-full rounded-xl p-3 outline-none resize-none"
                  style={{ background: "var(--color-bg-card, #151B2F)", border: "1px solid rgba(255,255,255,0.09)", color: "var(--color-text-primary, #E5E7EB)", fontSize: 13 }}
                />
                <button
                  id="finding-comment-submit"
                  className="mt-2 px-4 py-2 rounded-lg"
                  style={{ background: "linear-gradient(135deg, #4F8CFF, #3B6FCC)", color: "white", border: "none", fontSize: 13, cursor: "pointer" }}
                >
                  <MessageSquare size={13} style={{ display: "inline", marginRight: 6 }} />Post Comment
                </button>
              </div>
            </div>
          </div>
        )}

        {activeTab === "Timeline" && (
          <div className="max-w-2xl">
            <div className="relative pl-6">
              <div className="absolute left-2 top-0 bottom-0 w-px" style={{ background: "rgba(255,255,255,0.08)" }} />
              {(finding.vexJustification ? [
                { user: 'System', action: `VEX: ${finding.vexJustification}`, time: finding.createdAt, type: 'system' },
                { user: finding.createdBy, action: 'Finding created', time: finding.createdAt, type: 'system' },
                { user: finding.assignedTo ?? 'System', action: 'Status: ' + finding.status, time: finding.updatedAt, type: 'update' },
              ] : [
                { user: finding.createdBy, action: 'Finding created', time: finding.createdAt, type: 'system' },
                { user: finding.assignedTo ?? 'System', action: 'Status: ' + finding.status, time: finding.updatedAt, type: 'update' },
              ] as Array<{ user: string; action: string; time: string; type: string }>).map((entry, i) => (
                <div key={i} className="relative mb-5">
                  <div className="absolute -left-4 w-3 h-3 rounded-full" style={{ background: entry.type === "ai" ? "#7C3AED" : entry.type === "system" ? "#374151" : "#4F8CFF", top: 12 }} />
                  <div className="rounded-xl p-3" style={{ background: "var(--color-bg-card, #151B2F)", border: "1px solid rgba(255,255,255,0.07)" }}>
                    <div style={{ color: "var(--color-text-muted, #6B7280)", fontSize: 11, marginBottom: 4 }}>{entry.time ? new Date(entry.time).toLocaleString() : "—"}</div>
                    <div style={{ color: "var(--color-text-secondary, #9CA3AF)", fontSize: 12 }}><span style={{ color: "var(--color-text-primary, #E5E7EB)" }}>{entry.user}</span> — {entry.action}</div>
                  </div>
                </div>
              ))}
            </div>
          </div>
        )}
      </div>
    </div>
  );
}

function FindingDetailSkeleton() {
  return (
    <div className="flex flex-col flex-1 overflow-hidden animate-pulse" style={{ background: "var(--color-bg-page, #0B1020)" }}>
      <div className="h-24 mb-4" style={{ background: "var(--color-bg-sidebar, #0F1629)" }} />
      <div className="p-6 grid grid-cols-3 gap-5">
        <div className="col-span-2">
          <div className="grid grid-cols-4 gap-4 mb-5">
            {Array.from({ length: 4 }).map((_, i) => <div key={i} className="rounded-2xl h-20" style={{ background: "var(--color-bg-card, #151B2F)" }} />)}
          </div>
          <div className="rounded-2xl h-40" style={{ background: "var(--color-bg-card, #151B2F)" }} />
        </div>
        <div className="rounded-2xl h-64" style={{ background: "var(--color-bg-card, #151B2F)" }} />
      </div>
    </div>
  );
}

export function FindingDetail({ findingId: propFindingId, onBack }: FindingDetailProps) {
  const params = useParams<{ id: string }>();
  const navigate = useNavigate();
  const id = propFindingId ?? params.id ?? null;

  const findingQuery = useFindingDetail(id);

  const handleBack = onBack ?? (() => navigate(-1));

  return (
    <QueryBoundary query={findingQuery} skeleton={<FindingDetailSkeleton />}>
      {(finding) => (
        <FindingDetailContent finding={finding} onBack={handleBack} />
      )}
    </QueryBoundary>
  );
}
