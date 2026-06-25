import { Brain, Zap, CheckCircle, AlertTriangle, RefreshCw } from "lucide-react";
import { LineChart, Line, XAxis, YAxis, Tooltip, ResponsiveContainer, BarChart, Bar, Cell } from "recharts";
import { useQuery } from "@tanstack/react-query";
import { apiClient } from "@/shared/api/client";
import { ENDPOINTS } from "@/shared/api/endpoints";
import { QueryBoundary } from "@/shared/components/feedback/QueryBoundary";

// ── Types ────────────────────────────────────────────────────────────────────

interface AIEnrichmentData {
  queue: { total: number; processing: number; pending: number; completedToday: number };
  processingTrend: Array<{ h: string; enr: number }>;
  mitreDistribution: Array<{ tactic: string; count: number }>;
  recentJobs: Array<{ id: string; findingId: string; status: string; confidence: number | null; duration: number | null }>;
}

// ── Real API response shape from the server ───────────────────────────────────
// GET /api/v1/ai/enrichment → { last_run_at: string|null, status: string, total_enriched: number }
// We normalise this flat payload into the richer AIEnrichmentData structure the UI expects.

interface RawAIEnrichmentResponse {
  last_run_at: string | null;
  status: string;
  total_enriched: number;
  // optional extra fields the server might add in future
  pending?: number;
  processing?: number;
  completed_today?: number;
  recent_jobs?: Array<{ id: string; finding_id: string; status: string; confidence: number | null; duration: number | null }>;
}

function normalizeEnrichment(raw: RawAIEnrichmentResponse): AIEnrichmentData {
  const totalEnriched = raw.total_enriched ?? 0;
  const isProcessing = raw.status === 'processing' || raw.status === 'running';

  return {
    queue: {
      total:          raw.pending ?? totalEnriched,
      pending:        raw.pending ?? 0,
      processing:     raw.processing ?? (isProcessing ? 1 : 0),
      completedToday: raw.completed_today ?? totalEnriched,
    },
    processingTrend: [],
    mitreDistribution: [],
    recentJobs: (raw.recent_jobs ?? []).map(j => ({
      id:         j.id,
      findingId:  j.finding_id,
      status:     j.status,
      confidence: j.confidence,
      duration:   j.duration,
    })),
  };
}

function useAIEnrichment() {
  return useQuery<AIEnrichmentData>({
    queryKey: ['ai', 'enrichment'],
    queryFn: async () => {
      const { data } = await apiClient.get<RawAIEnrichmentResponse>(ENDPOINTS.ai.enrichment);
      return normalizeEnrichment(data);
    },
    staleTime: 30_000,
    refetchInterval: 30_000,
  });
}


// ── UI helpers ───────────────────────────────────────────────────────────────

const STATUS_STYLES: Record<string, { bg: string; color: string; icon: React.ElementType }> = {
  completed: { bg: "rgba(16,185,129,0.1)", color: "var(--color-status-success, #10B981)", icon: CheckCircle },
  processing: { bg: "rgba(79,140,255,0.1)", color: "var(--color-primary, #4F8CFF)", icon: RefreshCw },
  queued: { bg: "rgba(245,158,11,0.1)", color: "var(--color-status-warning, #F59E0B)", icon: RefreshCw },
  failed: { bg: "rgba(239,68,68,0.1)", color: "var(--color-status-error, #EF4444)", icon: AlertTriangle },
};

function AISkeleton() {
  return (
    <div className="flex-1 overflow-y-auto px-6 py-5 animate-pulse" style={{ background: "var(--color-bg-page, #0B1020)" }}>
      <div className="grid grid-cols-4 gap-4 mb-5">
        {Array.from({ length: 4 }).map((_, i) => <div key={i} className="rounded-2xl h-24" style={{ background: "var(--color-bg-card, #151B2F)" }} />)}
      </div>
      <div className="rounded-2xl h-64" style={{ background: "var(--color-bg-card, #151B2F)" }} />
    </div>
  );
}

// ── Main component ───────────────────────────────────────────────────────────

export function AIEnrichment() {
  const enrichmentQuery = useAIEnrichment();

  return (
    <QueryBoundary query={enrichmentQuery} skeleton={<AISkeleton />}>
      {(enrichment) => (
        <div className="flex-1 overflow-y-auto px-6 py-5" style={{ background: "var(--color-bg-page, #0B1020)" }}>
          <div className="flex items-center gap-3 mb-5">
            <div className="w-10 h-10 rounded-xl flex items-center justify-center" style={{ background: "rgba(124,58,237,0.2)" }}>
              <Brain size={20} color="#A78BFA" />
            </div>
            <div>
              <h2 style={{ color: "var(--color-text-primary, #E5E7EB)", fontSize: 18, fontWeight: 700 }}>AI Enrichment Center</h2>
              <p style={{ color: "var(--color-text-muted, #6B7280)", fontSize: 12 }}>Automated vulnerability intelligence enrichment pipeline</p>
            </div>
            <button className="ml-auto flex items-center gap-2 px-4 py-2 rounded-xl" style={{ background: "linear-gradient(135deg,#7C3AED,#4F8CFF)", color: "white", border: "none", fontSize: 13, cursor: "pointer" }}>
              <Zap size={14} />Run Enrichment
            </button>
          </div>

          {/* KPIs */}
          <div className="grid grid-cols-4 gap-4 mb-5">
            {[
              { label: "Total Queue", value: enrichment.queue.total.toLocaleString(), color: "var(--color-status-success, #10B981)" },
              { label: "Pending Analysis", value: enrichment.queue.pending.toLocaleString(), color: "var(--color-status-warning, #F59E0B)" },
              { label: "Processing Now", value: enrichment.queue.processing, color: "var(--color-primary, #4F8CFF)" },
              { label: "Completed Today", value: enrichment.queue.completedToday.toLocaleString(), color: "var(--color-ai, #A78BFA)" },
            ].map(s => (
              <div key={s.label} className="rounded-2xl p-5" style={{ background: "var(--color-bg-card, #151B2F)", border: "1px solid rgba(255,255,255,0.07)" }}>
                <div style={{ color: s.color, fontSize: 26, fontWeight: 700 }}>{s.value}</div>
                <div style={{ color: "var(--color-text-secondary, #9CA3AF)", fontSize: 12, marginTop: 4 }}>{s.label}</div>
              </div>
            ))}
          </div>

          <div className="grid grid-cols-2 gap-4 mb-5">
            {/* Processing Trend */}
            <div className="rounded-2xl p-5" style={{ background: "var(--color-bg-card, #151B2F)", border: "1px solid rgba(255,255,255,0.07)" }}>
              <h3 style={{ color: "var(--color-text-primary, #E5E7EB)", fontSize: 14, fontWeight: 600, marginBottom: 12 }}>Processing Volume (24h)</h3>
              {enrichment.processingTrend.length > 0 ? (
                <ResponsiveContainer width="100%" height={160}>
                  <LineChart data={enrichment.processingTrend}>
                    <XAxis dataKey="h" tick={{ fill: "#6B7280", fontSize: 10 }} axisLine={false} tickLine={false} />
                    <YAxis tick={{ fill: "#6B7280", fontSize: 10 }} axisLine={false} tickLine={false} />
                    <Tooltip contentStyle={{ background: "#1E2A45", border: "1px solid rgba(255,255,255,0.1)", borderRadius: 8, fontSize: 11 }} />
                    <Line type="monotone" dataKey="enr" stroke="#A78BFA" strokeWidth={2} dot={false} name="Enriched" />
                  </LineChart>
                </ResponsiveContainer>
              ) : (
                <div className="flex flex-col items-center justify-center" style={{ height: 160, color: "#4B5563" }}>
                  <Brain size={28} style={{ marginBottom: 8, opacity: 0.4 }} />
                  <span style={{ fontSize: 12 }}>No enrichment data yet</span>
                </div>
              )}
            </div>

            {/* MITRE Distribution */}
            <div className="rounded-2xl p-5" style={{ background: "var(--color-bg-card, #151B2F)", border: "1px solid rgba(255,255,255,0.07)" }}>
              <h3 style={{ color: "var(--color-text-primary, #E5E7EB)", fontSize: 14, fontWeight: 600, marginBottom: 12 }}>MITRE ATT&CK Distribution</h3>
              {enrichment.mitreDistribution.length > 0 ? (
                <ResponsiveContainer width="100%" height={160}>
                  <BarChart data={enrichment.mitreDistribution} barSize={20} layout="vertical">
                    <XAxis type="number" tick={{ fill: "#6B7280", fontSize: 10 }} axisLine={false} tickLine={false} />
                    <YAxis type="category" dataKey="tactic" tick={{ fill: "#9CA3AF", fontSize: 10 }} axisLine={false} tickLine={false} width={80} />
                    <Tooltip contentStyle={{ background: "#1E2A45", border: "1px solid rgba(255,255,255,0.1)", borderRadius: 8, fontSize: 11 }} />
                    <Bar dataKey="count" name="CVEs" radius={[0, 3, 3, 0]}>
                      {enrichment.mitreDistribution.map((_, i) => <Cell key={i} fill={["#4F8CFF", "#A78BFA", "#7C3AED", "#EC4899", "#EF4444"][i % 5]} />)}
                    </Bar>
                  </BarChart>
                </ResponsiveContainer>
              ) : (
                <div className="flex flex-col items-center justify-center" style={{ height: 160, color: "#4B5563" }}>
                  <Zap size={28} style={{ marginBottom: 8, opacity: 0.4 }} />
                  <span style={{ fontSize: 12 }}>No MITRE data available</span>
                </div>
              )}
            </div>
          </div>

          {/* Recent Jobs */}
          <div className="rounded-2xl" style={{ background: "var(--color-bg-card, #151B2F)", border: "1px solid rgba(255,255,255,0.07)" }}>
            <div className="px-5 py-4" style={{ borderBottom: "1px solid rgba(255,255,255,0.06)" }}>
              <h3 style={{ color: "var(--color-text-primary, #E5E7EB)", fontSize: 14, fontWeight: 600 }}>Recent Enrichment Jobs</h3>
            </div>
            {enrichment.recentJobs.length === 0 ? (
              <div className="flex flex-col items-center justify-center py-12" style={{ color: "#4B5563" }}>
                <Brain size={36} style={{ marginBottom: 10, opacity: 0.3 }} />
                <p style={{ fontSize: 13 }}>No enrichment jobs have run yet.</p>
                <p style={{ fontSize: 12, marginTop: 4, color: "#374151" }}>Click "Run Enrichment" to start processing findings.</p>
              </div>
            ) : (
              <table className="w-full">
                <thead>
                  <tr style={{ borderBottom: "1px solid rgba(255,255,255,0.05)" }}>
                    {["Job ID", "Finding", "Status", "Confidence", "Duration"].map(h => (
                      <th key={h} className="px-5 py-3 text-left" style={{ color: "var(--color-text-muted, #6B7280)", fontSize: 11, fontWeight: 600 }}>{h}</th>
                    ))}
                  </tr>
                </thead>
                <tbody>
                  {enrichment.recentJobs.map((job) => {
                    const style = STATUS_STYLES[job.status] ?? STATUS_STYLES.queued;
                    const Icon = style.icon;
                    return (
                      <tr key={job.id} style={{ borderBottom: "1px solid rgba(255,255,255,0.04)" }}>
                        <td className="px-5 py-3"><span style={{ color: "var(--color-primary, #4F8CFF)", fontSize: 12, fontFamily: "monospace" }}>{job.id}</span></td>
                        <td className="px-5 py-3"><span style={{ color: "var(--color-text-secondary, #9CA3AF)", fontSize: 12 }}>{job.findingId}</span></td>
                        <td className="px-5 py-3">
                          <span className="flex items-center gap-1 w-fit px-2 py-0.5 rounded-lg" style={{ background: style.bg, color: style.color, fontSize: 11 }}>
                            <Icon size={10} />{job.status}
                          </span>
                        </td>
                        <td className="px-5 py-3">
                          {job.confidence != null
                            ? <span style={{ color: job.confidence > 90 ? "#10B981" : "#F59E0B", fontSize: 12, fontWeight: 600 }}>{job.confidence}%</span>
                            : <span style={{ color: "var(--color-text-faint, #4B5563)", fontSize: 12 }}>—</span>
                          }
                        </td>
                        <td className="px-5 py-3">
                          {job.duration != null
                            ? <span style={{ color: "var(--color-text-muted, #6B7280)", fontSize: 12 }}>{job.duration}s</span>
                            : <span style={{ color: "var(--color-text-faint, #4B5563)", fontSize: 12 }}>—</span>
                          }
                        </td>
                      </tr>
                    );
                  })}
                </tbody>
              </table>
            )}
          </div>
        </div>
      )}
    </QueryBoundary>
  );
}
