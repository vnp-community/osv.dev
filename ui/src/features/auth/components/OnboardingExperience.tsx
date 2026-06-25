import { useState } from "react";
import { useNavigate } from "react-router";
import { Package, Server, Scan, AlertTriangle, FileText, Check, ChevronRight, ArrowRight, Shield, Zap } from "lucide-react";


const STEPS = [
  { id: 1, icon: Package, color: "var(--color-primary, #4F8CFF)", title: "Create Your First Product", desc: "Define the application or service you want to secure", action: "Create Product" },
  { id: 2, icon: Server, color: "var(--color-status-success, #10B981)", title: "Add Assets", desc: "Register the servers, IPs, or URLs to protect", action: "Add Asset" },
  { id: 3, icon: Scan, color: "var(--color-ai, #A78BFA)", title: "Run Your First Scan", desc: "Launch a network or web application scan", action: "Start Scan" },
  { id: 4, icon: AlertTriangle, color: "var(--color-severity-high, #F97316)", title: "Review Findings", desc: "Triage the vulnerabilities discovered by the scan", action: "Review Findings" },
  { id: 5, icon: FileText, color: "var(--color-status-warning, #F59E0B)", title: "Generate a Report", desc: "Create an executive or technical security report", action: "Generate Report" },
];

const TIPS = [
  { title: "Start with your production assets", desc: "High-value targets should be scanned first." },
  { title: "Enable AI triage", desc: "AI will auto-classify findings to save analyst time." },
  { title: "Set up KEV alerts", desc: "Get notified when CISA adds new actively-exploited CVEs." },
  { title: "Configure SLA policies", desc: "Define deadlines for Critical and High findings." },
];

export function OnboardingExperience({ onNavigate }: { onNavigate?: (s: string) => void }) {
  const navigate = useNavigate();
  const [completed, setCompleted] = useState<number[]>([]);

  const handleNavigate = (path: string) => {
    if (onNavigate) {
      onNavigate(path);
    } else {
      navigate(`/${path}`);
    }
  };

  const markDone = (id: number) => setCompleted(prev => prev.includes(id) ? prev : [...prev, id]);
  const progress = Math.round((completed.length / STEPS.length) * 100);

  return (
    <div className="flex-1 overflow-y-auto px-6 py-8" style={{ background: "var(--color-bg-page, #0B1020)" }}>
      {/* Hero */}
      <div className="text-center mb-10">
        <div className="w-16 h-16 rounded-2xl flex items-center justify-center mx-auto mb-4" style={{ background: "linear-gradient(135deg,#4F8CFF,#7C3AED)" }}>
          <Shield size={32} color="white" />
        </div>
        <h1 style={{ color: "var(--color-text-primary, #E5E7EB)", fontSize: 28, fontWeight: 800, letterSpacing: -0.5 }}>Welcome to OSV Platform</h1>
        <p style={{ color: "var(--color-text-secondary, #9CA3AF)", fontSize: 15, marginTop: 8, maxWidth: 480, margin: "8px auto 0" }}>
          Let's get you set up. Follow these steps to start securing your infrastructure in minutes.
        </p>
        {/* Progress bar */}
        <div className="max-w-sm mx-auto mt-6">
          <div className="flex justify-between mb-2">
            <span style={{ color: "var(--color-text-muted, #6B7280)", fontSize: 12 }}>Setup Progress</span>
            <span style={{ color: "var(--color-primary, #4F8CFF)", fontSize: 12, fontWeight: 600 }}>{completed.length}/{STEPS.length} completed</span>
          </div>
          <div className="h-2 rounded-full overflow-hidden" style={{ background: "rgba(255,255,255,0.08)" }}>
            <div className="h-full rounded-full transition-all" style={{ width: `${progress}%`, background: "linear-gradient(90deg,#4F8CFF,#7C3AED)" }} />
          </div>
        </div>
      </div>

      {/* Steps */}
      <div className="max-w-2xl mx-auto mb-10">
        <div className="relative pl-8">
          {/* Vertical line */}
          <div className="absolute left-3.5 top-8 bottom-8 w-px" style={{ background: "rgba(255,255,255,0.08)" }} />

          {STEPS.map((step, i) => {
            const Icon = step.icon;
            const done = completed.includes(step.id);
            return (
              <div key={step.id} className="relative mb-5 last:mb-0">
                {/* Step dot */}
                <div className="absolute -left-8 w-7 h-7 rounded-full flex items-center justify-center" style={{ background: done ? step.color : "rgba(255,255,255,0.08)", border: done ? "none" : "1px solid rgba(255,255,255,0.12)" }}>
                  {done ? <Check size={13} color="white" /> : <span style={{ color: "var(--color-text-muted, #6B7280)", fontSize: 11, fontWeight: 600 }}>{step.id}</span>}
                </div>

                <div className="rounded-2xl p-5 transition-all" style={{ background: done ? "rgba(255,255,255,0.03)" : "#151B2F", border: `1px solid ${done ? "rgba(255,255,255,0.05)" : "rgba(255,255,255,0.09)"}`, opacity: done ? 0.7 : 1 }}>
                  <div className="flex items-center justify-between">
                    <div className="flex items-center gap-3">
                      <div className="w-10 h-10 rounded-xl flex items-center justify-center" style={{ background: step.color + "15" }}>
                        <Icon size={20} color={done ? "#6B7280" : step.color} />
                      </div>
                      <div>
                        <div style={{ color: done ? "#6B7280" : "#E5E7EB", fontSize: 14, fontWeight: 600 }}>{step.title}</div>
                        <div style={{ color: "var(--color-text-muted, #6B7280)", fontSize: 12, marginTop: 2 }}>{step.desc}</div>
                      </div>
                    </div>
                    {!done ? (
                      <button
                        onClick={() => { markDone(step.id); handleNavigate(["products", "asset-inventory", "scans/new", "findings", "reports"][i]); }}
                        className="flex items-center gap-2 px-4 py-2 rounded-xl flex-shrink-0"
                        style={{ background: `${step.color}15`, border: `1px solid ${step.color}30`, color: step.color, fontSize: 12, cursor: "pointer" }}
                      >
                        {step.action} <ArrowRight size={12} />
                      </button>
                    ) : (
                      <div className="flex items-center gap-1.5 px-3 py-1.5 rounded-xl" style={{ background: "rgba(16,185,129,0.1)" }}>
                        <Check size={12} color="#10B981" />
                        <span style={{ color: "var(--color-status-success, #10B981)", fontSize: 12 }}>Done</span>
                      </div>
                    )}
                  </div>
                </div>
              </div>
            );
          })}
        </div>
      </div>

      {/* Tips */}
      <div className="max-w-2xl mx-auto">
        <div style={{ color: "var(--color-text-muted, #6B7280)", fontSize: 11, fontWeight: 600, letterSpacing: 1, marginBottom: 12 }}>PRO TIPS</div>
        <div className="grid grid-cols-2 gap-3">
          {TIPS.map(tip => (
            <div key={tip.title} className="rounded-xl p-4" style={{ background: "rgba(79,140,255,0.05)", border: "1px solid rgba(79,140,255,0.12)" }}>
              <div className="flex items-start gap-2">
                <Zap size={13} color="#4F8CFF" style={{ flexShrink: 0, marginTop: 2 }} />
                <div>
                  <div style={{ color: "var(--color-text-primary, #E5E7EB)", fontSize: 12, fontWeight: 500 }}>{tip.title}</div>
                  <div style={{ color: "var(--color-text-muted, #6B7280)", fontSize: 11, marginTop: 3 }}>{tip.desc}</div>
                </div>
              </div>
            </div>
          ))}
        </div>

        {progress === 100 && (
          <div className="mt-6 rounded-2xl p-6 text-center" style={{ background: "rgba(16,185,129,0.08)", border: "1px solid rgba(16,185,129,0.25)" }}>
            <div className="w-14 h-14 rounded-full flex items-center justify-center mx-auto mb-3" style={{ background: "rgba(16,185,129,0.2)" }}>
              <Check size={28} color="#10B981" />
            </div>
            <h3 style={{ color: "var(--color-status-success, #10B981)", fontSize: 18, fontWeight: 700 }}>Setup Complete!</h3>
            <p style={{ color: "#A7F3D0", fontSize: 13, marginTop: 6 }}>Your OSV Platform is ready. Head to the dashboard to see your security posture.</p>
            <button onClick={() => handleNavigate("dashboard")} className="mt-4 px-6 py-3 rounded-xl" style={{ background: "linear-gradient(135deg,#10B981,#059669)", color: "white", border: "none", fontSize: 14, cursor: "pointer" }}>
              Go to Dashboard →
            </button>
          </div>
        )}
      </div>
    </div>
  );
}
