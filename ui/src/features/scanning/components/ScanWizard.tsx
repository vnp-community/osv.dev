/**
 * ScanWizard — Multi-step form với React Hook Form + Zod validation
 * Theo SOL-004: Phase 3 Polish — ScanWizard với RHF + zodResolver
 */
import { useState } from "react";
import { useNavigate } from "react-router";
import { useForm, Controller } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { Network, Globe, Cpu, ChevronRight, Check, Play, ArrowLeft, X, Shield, AlertCircle } from "lucide-react";
import { useCreateScan } from "@/features/scanning/hooks/useScans";
import { scanWizardSchema, type ScanWizardFormData, parseTargets } from "@/features/scanning/schemas/scanWizardSchema";

// ── UI constants (not business data) ──────────────────────────────────────────

const SCAN_TYPES = [
  {
    id: "nmap_full" as const,
    icon: Network,
    name: "Network Scan",
    subtitle: "Nmap",
    description: "Discover hosts, open ports, services, and OS detection across network ranges.",
    tags: ["Host Discovery", "Port Scan", "Service Detection", "OS Detection"],
    color: "var(--color-primary, #4F8CFF)",
  },
  {
    id: "zap" as const,
    icon: Globe,
    name: "Web Application Scan",
    subtitle: "OWASP ZAP",
    description: "Dynamic analysis of web applications for OWASP Top 10 and common vulnerabilities.",
    tags: ["XSS", "SQL Injection", "CSRF", "Auth Issues"],
    color: "var(--color-severity-high, #F97316)",
  },
  {
    id: "nmap_discovery" as const,
    icon: Cpu,
    name: "Agent Scan",
    subtitle: "OSV Agent",
    description: "Deep inspection using installed agents on target hosts for local vulnerability assessment.",
    tags: ["Package Audit", "CVE Matching", "Config Check", "Compliance"],
    color: "var(--color-status-success, #10B981)",
  },
];

const STEP_LABELS = ["Scan Type", "Targets", "Configuration", "Review"];

// ── Step indicator ────────────────────────────────────────────────────────────

function StepIndicator({ step }: { step: number }) {
  return (
    <div className="flex items-center px-8 py-5" style={{ borderBottom: "1px solid rgba(255,255,255,0.06)" }}>
      {STEP_LABELS.map((label, i) => (
        <div key={label} className="flex items-center">
          <div className="flex items-center gap-2.5">
            <div
              className="w-7 h-7 rounded-full flex items-center justify-center flex-shrink-0"
              style={{
                background: step > i + 1 ? "#10B981" : step === i + 1 ? "#4F8CFF" : "rgba(255,255,255,0.06)",
                border: step === i + 1 ? "2px solid #4F8CFF" : "none",
              }}
            >
              {step > i + 1 ? (
                <Check size={13} color="white" />
              ) : (
                <span style={{ color: step === i + 1 ? "white" : "#6B7280", fontSize: 12, fontWeight: 600 }}>{i + 1}</span>
              )}
            </div>
            <span style={{ color: step === i + 1 ? "#E5E7EB" : step > i + 1 ? "#10B981" : "#6B7280", fontSize: 13 }}>{label}</span>
          </div>
          {i < STEP_LABELS.length - 1 && (
            <div className="w-16 h-px mx-4" style={{ background: step > i + 1 ? "#10B981" : "rgba(255,255,255,0.08)" }} />
          )}
        </div>
      ))}
    </div>
  );
}

// ── Field error ───────────────────────────────────────────────────────────────

function FieldError({ message }: { message?: string }) {
  if (!message) return null;
  return (
    <div className="flex items-center gap-1.5 mt-1.5">
      <AlertCircle size={12} color="#EF4444" />
      <span style={{ color: "var(--color-status-error, #EF4444)", fontSize: 11 }}>{message}</span>
    </div>
  );
}

// ── Main component ────────────────────────────────────────────────────────────

export function ScanWizard({ onClose }: { onClose?: () => void; onStart?: () => void }) {
  const navigate = useNavigate();
  const createScan = useCreateScan();
  const [step, setStep] = useState(1);

  const form = useForm<ScanWizardFormData>({
    resolver: zodResolver(scanWizardSchema),
    defaultValues: {
      type: "nmap_full",
      name: "",
      targetsRaw: "",
      frequency: "once",
      scanProfile: undefined,
      portRange: "1-65535",
      timeout: 30,
    },
    mode: "onChange",
  });

  const { register, control, handleSubmit, watch, trigger, formState: { errors } } = form;
  const scanType = watch("type");
  const targetsRaw = watch("targetsRaw");
  const selectedType = SCAN_TYPES.find((t) => t.id === scanType);

  // ── Step validation (per-step field trigger) ──────────────────────────────
  const nextStep = async () => {
    const stepFields: Record<number, (keyof ScanWizardFormData)[]> = {
      1: ["type"],
      2: ["name", "targetsRaw"],
      3: ["portRange", "timeout"],
    };
    const valid = await trigger(stepFields[step]);
    if (valid) setStep((s) => s + 1);
  };

  const onSubmit = (data: ScanWizardFormData) => {
    createScan.mutate({
      name: data.name,
      type: data.type,
      targets: parseTargets(data.targetsRaw),
      options: {
        port_range: data.portRange,
        timeout: data.timeout,
        scan_profile: data.scanProfile,
      },
      engagement_id: data.engagementId,
    });
  };

  const handleBack = () => step > 1 ? setStep((s) => s - 1) : (onClose ? onClose() : navigate(-1));

  return (
    <form onSubmit={handleSubmit(onSubmit)} className="flex flex-col flex-1 overflow-hidden" style={{ background: "var(--color-bg-page, #0B1020)" }}>
      {/* Header */}
      <div className="flex items-center justify-between px-6 py-4" style={{ borderBottom: "1px solid rgba(255,255,255,0.06)", background: "var(--color-bg-sidebar, #0F1629)" }}>
        <div className="flex items-center gap-3">
          <button type="button" onClick={handleBack} className="w-8 h-8 rounded-lg flex items-center justify-center" style={{ background: "rgba(255,255,255,0.05)", color: "var(--color-text-secondary, #9CA3AF)", border: "none", cursor: "pointer" }}>
            <ArrowLeft size={15} />
          </button>
          <div>
            <h2 style={{ color: "var(--color-text-primary, #E5E7EB)", fontSize: 16, fontWeight: 700 }}>Create New Scan</h2>
            <p style={{ color: "var(--color-text-muted, #6B7280)", fontSize: 12 }}>Step {step} of 4 — {STEP_LABELS[step - 1]}</p>
          </div>
        </div>
        <button type="button" onClick={handleBack} style={{ color: "var(--color-text-muted, #6B7280)", background: "none", border: "none", cursor: "pointer" }}>
          <X size={18} />
        </button>
      </div>

      {/* Step indicator */}
      <StepIndicator step={step} />

      {/* Step content */}
      <div className="flex-1 overflow-y-auto px-8 py-6">

        {/* Step 1: Scan Type */}
        {step === 1 && (
          <div>
            <h3 style={{ color: "var(--color-text-primary, #E5E7EB)", fontSize: 16, fontWeight: 600, marginBottom: 6 }}>Choose Scan Type</h3>
            <p style={{ color: "var(--color-text-muted, #6B7280)", fontSize: 13, marginBottom: 24 }}>Select the scanning engine that matches your target.</p>
            <Controller
              name="type"
              control={control}
              render={({ field }) => (
                <div className="grid grid-cols-3 gap-4">
                  {SCAN_TYPES.map((type) => {
                    const Icon = type.icon;
                    const isSelected = field.value === type.id;
                    return (
                      <div
                        key={type.id}
                        onClick={() => field.onChange(type.id)}
                        className="rounded-2xl p-5 cursor-pointer transition-all"
                        style={{
                          background: isSelected ? `${type.color}12` : "#151B2F",
                          border: isSelected ? `2px solid ${type.color}` : "2px solid rgba(255,255,255,0.07)",
                        }}
                      >
                        <div className="flex items-center justify-between mb-4">
                          <div className="w-10 h-10 rounded-xl flex items-center justify-center" style={{ background: `${type.color}20` }}>
                            <Icon size={20} color={type.color} />
                          </div>
                          {isSelected && (
                            <div className="w-5 h-5 rounded-full flex items-center justify-center" style={{ background: type.color }}>
                              <Check size={11} color="white" />
                            </div>
                          )}
                        </div>
                        <div style={{ color: "var(--color-text-primary, #E5E7EB)", fontSize: 15, fontWeight: 600 }}>{type.name}</div>
                        <div style={{ color: type.color, fontSize: 12, marginBottom: 8 }}>{type.subtitle}</div>
                        <p style={{ color: "var(--color-text-secondary, #9CA3AF)", fontSize: 12, lineHeight: 1.6, marginBottom: 12 }}>{type.description}</p>
                        <div className="flex flex-wrap gap-1.5">
                          {type.tags.map((tag) => (
                            <span key={tag} className="px-2 py-0.5 rounded" style={{ background: `${type.color}15`, color: type.color, fontSize: 10 }}>{tag}</span>
                          ))}
                        </div>
                      </div>
                    );
                  })}
                </div>
              )}
            />
            <FieldError message={errors.type?.message} />
          </div>
        )}

        {/* Step 2: Targets + Name */}
        {step === 2 && (
          <div className="max-w-xl">
            <h3 style={{ color: "var(--color-text-primary, #E5E7EB)", fontSize: 16, fontWeight: 600, marginBottom: 6 }}>Scan Name & Targets</h3>
            <p style={{ color: "var(--color-text-muted, #6B7280)", fontSize: 13, marginBottom: 24 }}>Give your scan a name and define the targets.</p>

            {/* Scan name */}
            <div className="mb-5">
              <label style={{ color: "var(--color-text-secondary, #9CA3AF)", fontSize: 13, display: "block", marginBottom: 8 }}>
                Scan Name <span style={{ color: "var(--color-status-error, #EF4444)" }}>*</span>
              </label>
              <input
                {...register("name")}
                placeholder="e.g. Production Network Q3 2026"
                className="w-full rounded-xl px-4 py-2.5 outline-none"
                style={{
                  background: "var(--color-bg-card, #151B2F)",
                  border: `1px solid ${errors.name ? "#EF4444" : "rgba(255,255,255,0.09)"}`,
                  color: "var(--color-text-primary, #E5E7EB)", fontSize: 13,
                }}
              />
              <FieldError message={errors.name?.message} />
            </div>

            {/* Targets */}
            <div className="mb-5">
              <label style={{ color: "var(--color-text-secondary, #9CA3AF)", fontSize: 13, display: "block", marginBottom: 8 }}>
                {selectedType?.id === "zap" ? "Target URLs" : "IP Ranges / Hostnames"} <span style={{ color: "var(--color-status-error, #EF4444)" }}>*</span>
              </label>
              <textarea
                {...register("targetsRaw")}
                placeholder={selectedType?.id === "zap" ? "https://app.company.com\nhttps://api.company.com" : "10.0.0.0/24\n192.168.1.0/24\nhostname.internal"}
                rows={6}
                className="w-full rounded-xl p-4 outline-none resize-none"
                style={{
                  background: "var(--color-bg-card, #151B2F)",
                  border: `1px solid ${errors.targetsRaw ? "#EF4444" : "rgba(255,255,255,0.09)"}`,
                  color: "var(--color-text-primary, #E5E7EB)", fontSize: 13, fontFamily: "monospace",
                }}
              />
              <FieldError message={errors.targetsRaw?.message} />
              {targetsRaw && (
                <div style={{ color: "var(--color-text-muted, #6B7280)", fontSize: 11, marginTop: 4 }}>
                  {parseTargets(targetsRaw).length} target(s) detected
                </div>
              )}
            </div>

            <div className="rounded-xl p-4" style={{ background: "rgba(79,140,255,0.06)", border: "1px solid rgba(79,140,255,0.15)" }}>
              <div style={{ color: "var(--color-primary, #4F8CFF)", fontSize: 12, fontWeight: 600, marginBottom: 4 }}>Format Examples</div>
              <div style={{ color: "var(--color-text-muted, #6B7280)", fontSize: 11, fontFamily: "monospace", lineHeight: 1.8 }}>
                {selectedType?.id === "zap" ? "https://target.com · https://api.target.com/v1" : "10.0.0.0/24 · 192.168.1.1-254 · hostname.local"}
              </div>
            </div>
          </div>
        )}

        {/* Step 3: Configuration */}
        {step === 3 && (
          <div className="max-w-2xl">
            <h3 style={{ color: "var(--color-text-primary, #E5E7EB)", fontSize: 16, fontWeight: 600, marginBottom: 6 }}>Scan Configuration</h3>
            <p style={{ color: "var(--color-text-muted, #6B7280)", fontSize: 13, marginBottom: 24 }}>Fine-tune scan options for better results.</p>
            <div className="grid grid-cols-2 gap-6">
              <div>
                <div style={{ color: "var(--color-text-secondary, #9CA3AF)", fontSize: 12, fontWeight: 600, marginBottom: 12 }}>SCAN PROFILE</div>
                <Controller
                  name="scanProfile"
                  control={control}
                  render={({ field }) => (
                    <div className="flex flex-col gap-2">
                      {[{ v: "discovery", label: "Discovery", desc: "Fast host/port discovery" }, { v: "full", label: "Full", desc: "Comprehensive with scripts" }, { v: "custom", label: "Custom", desc: "Manual configuration" }].map(({ v, label, desc }) => (
                        <div
                          key={v}
                          onClick={() => field.onChange(v)}
                          className="flex items-center gap-3 p-3 rounded-xl cursor-pointer"
                          style={{ background: field.value === v ? "rgba(79,140,255,0.1)" : "rgba(255,255,255,0.03)", border: `1px solid ${field.value === v ? "rgba(79,140,255,0.3)" : "rgba(255,255,255,0.06)"}` }}
                        >
                          <div className="w-4 h-4 rounded-full border-2 flex items-center justify-center" style={{ borderColor: field.value === v ? "#4F8CFF" : "#4B5563" }}>
                            {field.value === v && <div className="w-2 h-2 rounded-full" style={{ background: "#4F8CFF" }} />}
                          </div>
                          <div>
                            <div style={{ color: "var(--color-text-primary, #E5E7EB)", fontSize: 13 }}>{label}</div>
                            <div style={{ color: "var(--color-text-muted, #6B7280)", fontSize: 11 }}>{desc}</div>
                          </div>
                        </div>
                      ))}
                    </div>
                  )}
                />
              </div>

              <div>
                <div style={{ color: "var(--color-text-secondary, #9CA3AF)", fontSize: 12, fontWeight: 600, marginBottom: 12 }}>PERFORMANCE</div>
                <div className="mb-4">
                  <label style={{ color: "var(--color-text-primary, #E5E7EB)", fontSize: 13, display: "block", marginBottom: 8 }}>Port Range</label>
                  <input
                    {...register("portRange")}
                    className="w-full rounded-xl px-4 py-2.5 outline-none"
                    style={{ background: "var(--color-bg-card, #151B2F)", border: "1px solid rgba(255,255,255,0.09)", color: "var(--color-text-primary, #E5E7EB)", fontSize: 13, fontFamily: "monospace" }}
                  />
                  <FieldError message={errors.portRange?.message} />
                </div>
                <div>
                  <label style={{ color: "var(--color-text-primary, #E5E7EB)", fontSize: 13, display: "block", marginBottom: 8 }}>Scan Frequency</label>
                  <Controller
                    name="frequency"
                    control={control}
                    render={({ field }) => (
                      <select
                        {...field}
                        className="w-full rounded-xl px-4 py-2.5 outline-none"
                        style={{ background: "var(--color-bg-card, #151B2F)", border: "1px solid rgba(255,255,255,0.09)", color: "var(--color-text-primary, #E5E7EB)", fontSize: 13 }}
                      >
                        {[["once", "One-time"], ["daily", "Daily"], ["weekly", "Weekly"], ["custom", "Custom cron"]].map(([v, l]) => (
                          <option key={v} value={v}>{l}</option>
                        ))}
                      </select>
                    )}
                  />
                </div>
              </div>
            </div>
          </div>
        )}

        {/* Step 4: Review */}
        {step === 4 && selectedType && (
          <div className="max-w-xl">
            <h3 style={{ color: "var(--color-text-primary, #E5E7EB)", fontSize: 16, fontWeight: 600, marginBottom: 6 }}>Review & Launch</h3>
            <p style={{ color: "var(--color-text-muted, #6B7280)", fontSize: 13, marginBottom: 24 }}>Confirm your scan configuration before starting.</p>

            <div className="rounded-2xl overflow-hidden" style={{ border: "1px solid rgba(255,255,255,0.09)" }}>
              <div className="p-5" style={{ background: `${selectedType.color}10`, borderBottom: "1px solid rgba(255,255,255,0.07)" }}>
                <div className="flex items-center gap-3">
                  <div className="w-10 h-10 rounded-xl flex items-center justify-center" style={{ background: `${selectedType.color}25` }}>
                    <selectedType.icon size={20} color={selectedType.color} />
                  </div>
                  <div>
                    <div style={{ color: "var(--color-text-primary, #E5E7EB)", fontSize: 15, fontWeight: 600 }}>{selectedType.name}</div>
                    <div style={{ color: selectedType.color, fontSize: 12 }}>{selectedType.subtitle}</div>
                  </div>
                </div>
              </div>
              {[
                { label: "Scan Name", value: watch("name") },
                { label: "Targets", value: `${parseTargets(watch("targetsRaw")).length} target(s)`, mono: false },
                { label: "Profile", value: watch("scanProfile") ?? "default" },
                { label: "Port Range", value: watch("portRange") ?? "1-65535", mono: true },
                { label: "Frequency", value: watch("frequency") },
              ].map(({ label, value, mono }) => (
                <div key={label} className="flex px-5 py-3" style={{ borderBottom: "1px solid rgba(255,255,255,0.05)" }}>
                  <span style={{ color: "var(--color-text-muted, #6B7280)", fontSize: 13, width: 160 }}>{label}</span>
                  <span style={{ color: "var(--color-text-primary, #E5E7EB)", fontSize: 13, fontFamily: mono ? "monospace" : undefined }}>{value}</span>
                </div>
              ))}
            </div>

            <div className="mt-5 rounded-xl p-4" style={{ background: "rgba(79,140,255,0.08)", border: "1px solid rgba(79,140,255,0.2)" }}>
              <div style={{ color: "var(--color-primary, #4F8CFF)", fontSize: 12 }}>
                <Shield size={13} style={{ display: "inline", marginRight: 6 }} />
                Estimated duration: <strong>15–45 minutes</strong> depending on network size and response times.
              </div>
            </div>
          </div>
        )}
      </div>

      {/* Footer */}
      <div className="flex items-center justify-between px-8 py-4" style={{ borderTop: "1px solid rgba(255,255,255,0.06)", background: "var(--color-bg-sidebar, #0F1629)" }}>
        <button
          type="button"
          onClick={handleBack}
          className="flex items-center gap-2 px-5 py-2.5 rounded-xl"
          style={{ background: "rgba(255,255,255,0.05)", border: "1px solid rgba(255,255,255,0.09)", color: "var(--color-text-secondary, #9CA3AF)", fontSize: 13, cursor: "pointer" }}
        >
          <ArrowLeft size={14} />{step > 1 ? "Back" : "Cancel"}
        </button>

        {step < 4 ? (
          <button
            type="button"
            onClick={nextStep}
            className="flex items-center gap-2 px-6 py-2.5 rounded-xl"
            style={{ background: "linear-gradient(135deg, #4F8CFF, #3B6FCC)", color: "white", border: "none", fontSize: 13, cursor: "pointer" }}
          >
            Continue <ChevronRight size={14} />
          </button>
        ) : (
          <button
            type="submit"
            disabled={createScan.isPending}
            className="flex items-center gap-2 px-6 py-2.5 rounded-xl"
            style={{
              background: createScan.isPending ? "rgba(16,185,129,0.5)" : "linear-gradient(135deg, #10B981, #059669)",
              color: "white", border: "none", fontSize: 13,
              cursor: createScan.isPending ? "not-allowed" : "pointer",
              boxShadow: "0 4px 15px rgba(16,185,129,0.3)",
            }}
          >
            <Play size={14} />{createScan.isPending ? "Launching..." : "Start Scan"}
          </button>
        )}
      </div>
    </form>
  );
}
