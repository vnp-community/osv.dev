import { useState } from "react";
import { useSearchParams } from "react-router";
import { FileText, Download, Eye, Plus, CheckCircle, Shield, BarChart2, Loader2, AlertTriangle } from "lucide-react";
import { useReports, useReportTemplates, useCreateReport, type ReportRun, type ReportType } from "../hooks/useReports";

// ─── Icon mapping for template types ─────────────────────────────────────────

const TEMPLATE_ICONS: Record<string, React.ElementType> = {
  Executive:  Shield,
  Technical:  FileText,
  Compliance: BarChart2,
};

const TYPE_COLORS: Record<ReportType, string> = {
  Executive:  "var(--color-primary, #4F8CFF)",
  Technical:  "var(--color-status-success, #10B981)",
  Compliance: "var(--color-ai-accent, #A78BFA)",
};

const STATUS_CONFIG: Record<string, { color: string; icon: React.ElementType }> = {
  completed:  { color: "var(--color-status-success, #10B981)",  icon: CheckCircle },
  generating: { color: "var(--color-status-warning, #F59E0B)",  icon: Loader2 },
  pending:    { color: "var(--color-primary, #4F8CFF)",         icon: Loader2 },
  failed:     { color: "var(--color-status-error, #EF4444)",    icon: AlertTriangle },
};

function formatDate(iso?: string): string {
  if (!iso) return "—";
  const d = new Date(iso);
  return d.toLocaleDateString("en-GB", { day: "numeric", month: "short", year: "numeric" }) +
    ", " + d.toLocaleTimeString("en-GB", { hour: "2-digit", minute: "2-digit" });
}

function formatFileSize(bytes?: number): string {
  if (!bytes) return "—";
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(0)} KB`;
  return `${(bytes / (1024 * 1024)).toFixed(1)} MB`;
}

function timeAgo(iso?: string): string {
  if (!iso) return "—";
  const diff = Date.now() - new Date(iso).getTime();
  const hrs = Math.floor(diff / 3_600_000);
  if (hrs < 1) return `${Math.floor(diff / 60_000)}m ago`;
  if (hrs < 24) return `${hrs}h ago`;
  return `${Math.floor(hrs / 24)}d ago`;
}

// ─── Generate View ─────────────────────────────────────────────────────────────

interface GenerateViewProps {
  onBack: () => void;
  preSelectedTemplateId?: string;
}

function GenerateView({ onBack, preSelectedTemplateId }: GenerateViewProps) {
  const [selectedTemplateId, setSelectedTemplateId] = useState(preSelectedTemplateId ?? "");
  const [format, setFormat] = useState<"pdf" | "html" | "excel">("pdf");
  const [dateRange, setDateRange] = useState("Last 30 days");
  const [severity, setSeverity] = useState("All Severities");

  const { data: templatesData, isLoading: loadingTemplates } = useReportTemplates();
  const createReport = useCreateReport();

  const templates = templatesData?.templates ?? [];
  const selected = templates.find((t) => t.id === selectedTemplateId);

  const handleGenerate = async () => {
    if (!selected) return;
    await createReport.mutateAsync({
      template_id: selectedTemplateId,
      type: selected.type,
      format,
      date_range: dateRange,
      severity_filter: severity,
    });
    onBack();
  };

  return (
    <div className="flex-1 overflow-y-auto px-6 py-5" style={{ background: "var(--color-bg-page, #0B1020)" }}>
      <div className="max-w-2xl mx-auto">
        <div className="flex items-center gap-3 mb-6">
          <button
            onClick={onBack}
            className="px-4 py-2 rounded-xl"
            style={{ background: "var(--color-bg-input, rgba(255,255,255,0.05))", border: "1px solid var(--color-border-card, rgba(255,255,255,0.09))", color: "var(--color-text-secondary, #9CA3AF)", fontSize: 13, cursor: "pointer" }}
          >
            ← Back
          </button>
          <h2 style={{ color: "var(--color-text-primary, #E5E7EB)", fontSize: 18, fontWeight: 700 }}>Generate Report</h2>
        </div>

        {/* Template selection */}
        <div
          className="rounded-2xl p-5 mb-5"
          style={{ background: "var(--color-bg-card, #151B2F)", border: "1px solid var(--color-border-subtle, rgba(255,255,255,0.07))" }}
        >
          <div style={{ color: "var(--color-text-secondary, #9CA3AF)", fontSize: 12, fontWeight: 600, marginBottom: 12 }}>SELECT TEMPLATE</div>
          {loadingTemplates ? (
            <div className="flex items-center justify-center py-6">
              <Loader2 size={20} className="animate-spin" style={{ color: "var(--color-primary, #4F8CFF)" }} />
            </div>
          ) : (
            <div className="grid grid-cols-3 gap-3">
              {templates.map((t) => {
                const Icon = TEMPLATE_ICONS[t.type] ?? FileText;
                const color = TYPE_COLORS[t.type] ?? "var(--color-primary, #4F8CFF)";
                return (
                  <div
                    key={t.id}
                    onClick={() => setSelectedTemplateId(t.id)}
                    className="rounded-xl p-4 cursor-pointer transition-all"
                    style={{
                      background: selectedTemplateId === t.id ? `${color}12` : "var(--color-bg-input, rgba(255,255,255,0.04))",
                      border: selectedTemplateId === t.id ? `2px solid ${color}` : "2px solid var(--color-border-card, rgba(255,255,255,0.07))",
                    }}
                  >
                    <Icon size={20} color={color} style={{ marginBottom: 8 }} />
                    <div style={{ color: "var(--color-text-primary, #E5E7EB)", fontSize: 13, fontWeight: 500 }}>{t.name}</div>
                    <div style={{ color: "var(--color-text-muted, #6B7280)", fontSize: 11, marginTop: 4 }}>{t.description}</div>
                  </div>
                );
              })}
            </div>
          )}
        </div>

        {/* Filters */}
        <div
          className="rounded-2xl p-5 mb-5"
          style={{ background: "var(--color-bg-card, #151B2F)", border: "1px solid var(--color-border-subtle, rgba(255,255,255,0.07))" }}
        >
          <div style={{ color: "var(--color-text-secondary, #9CA3AF)", fontSize: 12, fontWeight: 600, marginBottom: 12 }}>REPORT FILTERS</div>
          <div className="grid grid-cols-2 gap-4">
            {[
              { label: "Date Range",  options: ["Last 30 days", "Last Quarter", "Last 6 months", "Custom"], value: dateRange,  onChange: setDateRange },
              { label: "Severity",    options: ["All Severities", "Critical Only", "Critical + High", "All except Low"], value: severity, onChange: setSeverity },
              { label: "Format",      options: ["pdf", "html", "excel"], value: format, onChange: (v: string) => setFormat(v as typeof format) },
            ].map(({ label, options, value, onChange }) => (
              <div key={label}>
                <label style={{ color: "var(--color-text-secondary, #9CA3AF)", fontSize: 12, display: "block", marginBottom: 6 }}>{label}</label>
                <select
                  value={value}
                  onChange={(e) => onChange(e.target.value)}
                  className="w-full rounded-xl px-3 py-2.5 outline-none"
                  style={{ background: "var(--color-bg-sidebar, #0F1629)", border: "1px solid var(--color-border-input, rgba(255,255,255,0.08))", color: "var(--color-text-primary, #E5E7EB)", fontSize: 13 }}
                >
                  {options.map((o) => <option key={o}>{o}</option>)}
                </select>
              </div>
            ))}
          </div>
        </div>

        {/* Generate button */}
        <button
          id="generate-report-btn"
          onClick={handleGenerate}
          disabled={!selectedTemplateId || createReport.isPending}
          className="w-full py-3 rounded-xl flex items-center justify-center gap-2"
          style={{
            background: !selectedTemplateId
              ? "var(--color-primary-bg, rgba(79,140,255,0.3))"
              : "var(--color-primary-grad, linear-gradient(135deg, #4F8CFF, #3B6FCC))",
            color: "white",
            border: "none",
            fontSize: 14,
            fontWeight: 600,
            cursor: !selectedTemplateId ? "not-allowed" : "pointer",
          }}
        >
          {createReport.isPending ? (
            <><Loader2 size={16} className="animate-spin" /> Generating...</>
          ) : (
            <><FileText size={16} /> Generate Report</>
          )}
        </button>
      </div>
    </div>
  );
}

// ─── Main Component ───────────────────────────────────────────────────────────

export function ReportCenter() {
  const [step, setStep] = useState<"list" | "generate">("list");
  const [preSelectedTemplate, setPreSelectedTemplate] = useState("");
  const [searchParams] = useSearchParams();

  const { data, isLoading, isError, refetch } = useReports();
  const { data: templatesData } = useReportTemplates();

  if (step === "generate") {
    return (
      <GenerateView
        onBack={() => { setStep("list"); setPreSelectedTemplate(""); }}
        preSelectedTemplateId={preSelectedTemplate}
      />
    );
  }

  const reports: ReportRun[] = data?.reports ?? [];
  const templates = templatesData?.templates ?? [];

  // TASK-06: filter templates list when navigated via ?type= sidebar link
  // Maps URL param values (executive/technical/compliance) to template type labels (Executive/Technical/Compliance)
  const urlType = searchParams.get("type"); // 'executive' | 'technical' | 'compliance' | null
  const typeFilter: string | null =
    urlType === "executive"  ? "Executive"  :
    urlType === "technical"  ? "Technical"  :
    urlType === "compliance" ? "Compliance" : null;

  // Subtitle from server data
  const subtitle = data
    ? `${data.total} reports · Last report ${timeAgo(data.last_generated_at)}`
    : "Loading...";

  return (
    <div className="flex-1 overflow-y-auto px-6 py-5" style={{ background: "var(--color-bg-page, #0B1020)" }}>
      {/* Header */}
      <div className="flex items-center justify-between mb-6">
        <div>
          <h2 style={{ color: "var(--color-text-primary, #E5E7EB)", fontSize: 18, fontWeight: 700 }}>Report Center</h2>
          <p style={{ color: "var(--color-text-muted, #6B7280)", fontSize: 12 }}>{subtitle}</p>
        </div>
        <button
          id="report-generate-btn"
          onClick={() => setStep("generate")}
          className="flex items-center gap-2 px-5 py-2.5 rounded-xl"
          style={{ background: "var(--color-primary-grad, linear-gradient(135deg, #4F8CFF, #3B6FCC))", color: "white", border: "none", fontSize: 13, cursor: "pointer" }}
        >
          <Plus size={15} /> Generate Report
        </button>
      </div>

      {/* Templates row — from API */}
      {isLoading ? (
        <div className="grid grid-cols-3 gap-4 mb-6 animate-pulse">
          {[0, 1, 2].map((i) => (
            <div key={i} className="rounded-2xl h-28" style={{ background: "var(--color-bg-card, #151B2F)" }} />
          ))}
        </div>
      ) : (
        <div className="grid grid-cols-3 gap-4 mb-6">
          {templates
            .filter((t) => typeFilter === null || t.type === typeFilter)
            .map((t) => {
            const Icon = TEMPLATE_ICONS[t.type] ?? FileText;
            const color = TYPE_COLORS[t.type] ?? "var(--color-primary, #4F8CFF)";
            return (
              <div
                key={t.id}
                onClick={() => { setPreSelectedTemplate(t.id); setStep("generate"); }}
                className="rounded-2xl p-5 cursor-pointer transition-all"
                style={{
                  background: "var(--color-bg-card, #151B2F)",
                  border: typeFilter && t.type === typeFilter
                    ? `1px solid ${color}80`
                    : "1px solid var(--color-border-subtle, rgba(255,255,255,0.07))",
                }}
                onMouseEnter={(e) => (e.currentTarget.style.borderColor = color + "60")}
                onMouseLeave={(e) => (e.currentTarget.style.borderColor = typeFilter && t.type === typeFilter ? color + "80" : "rgba(255,255,255,0.07)")}
              >
                <div className="w-10 h-10 rounded-xl flex items-center justify-center mb-3" style={{ background: `${color}20` }}>
                  <Icon size={20} color={color} />
                </div>
                <div style={{ color: "var(--color-text-primary, #E5E7EB)", fontSize: 14, fontWeight: 600 }}>{t.name}</div>
                <div style={{ color: "var(--color-text-muted, #6B7280)", fontSize: 12, marginTop: 4 }}>{t.description}</div>
              </div>
            );
          })}
          {/* Show all button when filtered */}
          {typeFilter && (
            <div
              className="rounded-2xl p-5 cursor-pointer flex items-center justify-center"
              style={{ background: "rgba(255,255,255,0.03)", border: "1px dashed rgba(255,255,255,0.1)" }}
              onClick={() => window.history.replaceState(null, "", "/reports")}
            >
              <span style={{ color: "var(--color-text-muted, #6B7280)", fontSize: 13 }}>View all types</span>
            </div>
          )}
        </div>

      )}

      {/* Error state */}
      {isError && (
        <div className="rounded-2xl p-6 mb-5 text-center" style={{ background: "var(--color-status-error-bg, rgba(239,68,68,0.08))", border: "1px solid var(--color-status-error-border, rgba(239,68,68,0.2))" }}>
          <AlertTriangle size={24} style={{ color: "var(--color-status-error, #EF4444)", margin: "0 auto 8px" }} />
          <p style={{ color: "var(--color-text-secondary, #9CA3AF)", fontSize: 13 }}>Failed to load reports</p>
          <button onClick={() => refetch()} className="mt-3 px-4 py-2 rounded-xl" style={{ background: "var(--color-primary-bg, rgba(79,140,255,0.1))", color: "var(--color-primary, #4F8CFF)", border: "none", cursor: "pointer", fontSize: 13 }}>
            Retry
          </button>
        </div>
      )}

      {/* Reports table — from API */}
      <div
        className="rounded-2xl"
        style={{ background: "var(--color-bg-card, #151B2F)", border: "1px solid var(--color-border-subtle, rgba(255,255,255,0.07))" }}
      >
        <div className="px-5 py-4" style={{ borderBottom: "1px solid var(--color-border-section, rgba(255,255,255,0.06))" }}>
          <h3 style={{ color: "var(--color-text-primary, #E5E7EB)", fontSize: 14, fontWeight: 600 }}>Generated Reports</h3>
        </div>
        <table className="w-full">
          <thead>
            <tr style={{ borderBottom: "1px solid rgba(255,255,255,0.05)" }}>
              {["ID", "Name", "Type", "Format", "Created", "Size", "Findings", "Status", ""].map((h) => (
                <th key={h || "_actions"} className="px-5 py-3 text-left" style={{ color: "var(--color-text-muted, #6B7280)", fontSize: 11, fontWeight: 600, letterSpacing: 0.5 }}>
                  {h}
                </th>
              ))}
            </tr>
          </thead>
          <tbody>
            {reports.map((r, i) => {
              const sc = STATUS_CONFIG[r.status] ?? STATUS_CONFIG.completed;
              const StatusIcon = sc.icon;
              const typeColor = TYPE_COLORS[r.type] ?? "var(--color-text-secondary, #9CA3AF)";
              return (
                <tr
                  key={r.id}
                  className="transition-all"
                  style={{ borderBottom: i < reports.length - 1 ? "1px solid rgba(255,255,255,0.04)" : "none" }}
                  onMouseEnter={(e) => (e.currentTarget.style.background = "var(--color-bg-hover, rgba(255,255,255,0.02))")}
                  onMouseLeave={(e) => (e.currentTarget.style.background = "transparent")}
                >
                  <td className="px-5 py-3"><span style={{ color: "var(--color-text-muted, #6B7280)", fontSize: 12 }}>{r.id}</span></td>
                  <td className="px-5 py-3"><span style={{ color: "var(--color-text-primary, #E5E7EB)", fontSize: 13 }}>{r.name ?? `${r.type} Report`}</span></td>
                  <td className="px-5 py-3">
                    <span className="px-2 py-0.5 rounded" style={{ background: typeColor + "20", color: typeColor, fontSize: 11 }}>{r.type}</span>
                  </td>
                  <td className="px-5 py-3"><span style={{ color: "var(--color-text-muted, #6B7280)", fontSize: 12, textTransform: "uppercase" }}>{r.format}</span></td>
                  <td className="px-5 py-3"><span style={{ color: "var(--color-text-muted, #6B7280)", fontSize: 12 }}>{formatDate(r.generated_at ?? r.created_at)}</span></td>
                  <td className="px-5 py-3"><span style={{ color: "var(--color-text-muted, #6B7280)", fontSize: 12 }}>{formatFileSize(r.file_size_bytes)}</span></td>
                  <td className="px-5 py-3"><span style={{ color: "var(--color-text-secondary, #9CA3AF)", fontSize: 12 }}>{r.finding_count ?? "—"}</span></td>
                  <td className="px-5 py-3">
                    <span className="flex items-center gap-1.5" style={{ color: sc.color, fontSize: 12 }}>
                      <StatusIcon size={11} className={r.status === "generating" || r.status === "pending" ? "animate-spin" : undefined} />
                      {r.status}
                    </span>
                  </td>
                  <td className="px-5 py-3">
                    <div className="flex items-center gap-2">
                      <button
                        className="w-7 h-7 rounded-lg flex items-center justify-center"
                        style={{ background: "var(--color-bg-input, rgba(255,255,255,0.05))", color: "var(--color-text-muted, #6B7280)", border: "none", cursor: "pointer" }}
                        title="Preview"
                      >
                        <Eye size={12} />
                      </button>
                      {r.status === "completed" && (
                        <button
                          className="w-7 h-7 rounded-lg flex items-center justify-center"
                          style={{ background: "var(--color-primary-bg, rgba(79,140,255,0.1))", color: "var(--color-primary, #4F8CFF)", border: "none", cursor: "pointer" }}
                          title="Download"
                        >
                          <Download size={12} />
                        </button>
                      )}
                    </div>
                  </td>
                </tr>
              );
            })}
          </tbody>
        </table>

        {reports.length === 0 && !isLoading && (
          <div className="text-center py-10">
            <p style={{ color: "var(--color-text-muted, #6B7280)", fontSize: 13 }}>No reports yet</p>
          </div>
        )}
      </div>
    </div>
  );
}
