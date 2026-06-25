import { useState } from "react";
import { useNavigate } from "react-router";
import { Shield, Eye, EyeOff, Github, Lock, Mail, AlertTriangle, Activity, Wifi, Server } from "lucide-react";
import { useAuthStore } from "@/features/auth/store/authStore";
import { apiClient } from "@/shared/api/client";
import { ENDPOINTS } from "@/shared/api/endpoints";
import type { User } from "@/shared/types/auth";

export function LoginScreen() {
  const navigate = useNavigate();
  const setUser = useAuthStore((s) => s.setUser);
  const [showPassword, setShowPassword] = useState(false);
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [rememberMe, setRememberMe] = useState(false);
  const [isLoading, setIsLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setIsLoading(true);
    setError(null);
    try {
      const { data } = await apiClient.post<{ user: User }>(ENDPOINTS.auth.login, { email, password });
      setUser(data.user);
      navigate("/dashboard", { replace: true });
    } catch (err: any) {
      setError(err?.response?.data?.message ?? "Invalid credentials. Please try again.");
    } finally {
      setIsLoading(false);
    }
  };

  const stats = [
    { label: "CVEs Tracked", value: "240K+" },
    { label: "Scans Today", value: "1,847" },
    { label: "Findings", value: "98.4%" },
    { label: "Uptime SLA", value: "99.99%" },
  ];

  const threatIndicators = [
    { label: "Critical Threats", count: 14, color: "#EF4444" },
    { label: "KEV Active", count: 7, color: "#F59E0B" },
    { label: "Assets At Risk", count: 23, color: "#3B82F6" },
  ];

  return (
    <div
      className="min-h-screen w-full flex"
      style={{ background: "#0B1020", fontFamily: "'Inter', sans-serif" }}
    >
      {/* Left Panel */}
      <div
        className="hidden lg:flex flex-col justify-between w-1/2 p-12 relative overflow-hidden"
        style={{ background: "linear-gradient(135deg, #0D1628 0%, #0B1020 50%, #071020 100%)" }}
      >
        {/* Grid pattern */}
        <div
          className="absolute inset-0 opacity-5"
          style={{
            backgroundImage: `linear-gradient(rgba(79,140,255,0.3) 1px, transparent 1px), linear-gradient(90deg, rgba(79,140,255,0.3) 1px, transparent 1px)`,
            backgroundSize: "40px 40px",
          }}
        />
        {/* Glow effects */}
        <div
          className="absolute top-1/4 left-1/4 w-96 h-96 rounded-full opacity-10 blur-3xl pointer-events-none"
          style={{ background: "#4F8CFF" }}
        />
        <div
          className="absolute bottom-1/4 right-1/4 w-64 h-64 rounded-full opacity-10 blur-3xl pointer-events-none"
          style={{ background: "#10B981" }}
        />

        {/* Logo */}
        <div className="relative z-10">
          <div className="flex items-center gap-3 mb-2">
            <div
              className="w-10 h-10 rounded-xl flex items-center justify-center"
              style={{ background: "linear-gradient(135deg, #4F8CFF, #7C3AED)" }}
            >
              <Shield size={22} color="white" />
            </div>
            <span style={{ color: "#E5E7EB", fontSize: 22, fontWeight: 700, letterSpacing: -0.5 }}>
              OSV <span style={{ color: "#4F8CFF" }}>Platform</span>
            </span>
          </div>
          <p style={{ color: "#6B7280", fontSize: 13 }}>Enterprise Security Operations</p>
        </div>

        {/* Center content */}
        <div className="relative z-10 flex flex-col gap-10">
          <div>
            <h1
              style={{
                color: "#E5E7EB",
                fontSize: 38,
                fontWeight: 800,
                lineHeight: 1.2,
                letterSpacing: -1,
              }}
            >
              The Complete<br />
              <span style={{ color: "#4F8CFF" }}>Vulnerability Intelligence</span>
              <br />& Scanning Platform
            </h1>
            <p style={{ color: "#9CA3AF", marginTop: 16, fontSize: 15, lineHeight: 1.7, maxWidth: 420 }}>
              Unified vulnerability management, active scanning, asset intelligence, and AI-powered triage — all in one enterprise platform.
            </p>
          </div>

          {/* Threat indicators */}
          <div
            className="rounded-2xl p-5"
            style={{ background: "rgba(255,255,255,0.04)", border: "1px solid rgba(255,255,255,0.08)" }}
          >
            <div className="flex items-center gap-2 mb-4">
              <div className="w-2 h-2 rounded-full animate-pulse" style={{ background: "#10B981" }} />
              <span style={{ color: "#9CA3AF", fontSize: 12, letterSpacing: 1 }}>LIVE THREAT INTELLIGENCE</span>
            </div>
            <div className="grid grid-cols-3 gap-3">
              {threatIndicators.map((t) => (
                <div
                  key={t.label}
                  className="rounded-xl p-3"
                  style={{ background: "rgba(255,255,255,0.04)", borderLeft: `3px solid ${t.color}` }}
                >
                  <div style={{ color: t.color, fontSize: 22, fontWeight: 700 }}>{t.count}</div>
                  <div style={{ color: "#6B7280", fontSize: 11, marginTop: 2 }}>{t.label}</div>
                </div>
              ))}
            </div>
          </div>

          {/* Security illustration */}
          <div
            className="rounded-2xl p-5"
            style={{ background: "rgba(255,255,255,0.04)", border: "1px solid rgba(255,255,255,0.08)" }}
          >
            <div className="flex items-center justify-between mb-3">
              <span style={{ color: "#9CA3AF", fontSize: 12 }}>System Status</span>
              <span style={{ color: "#10B981", fontSize: 12 }}>● All Systems Operational</span>
            </div>
            <div className="flex gap-6">
              {[
                { icon: Activity, label: "Scan Engine", ok: true },
                { icon: Wifi, label: "AI Service", ok: true },
                { icon: Server, label: "Data Feed", ok: true },
              ].map(({ icon: Icon, label, ok }) => (
                <div key={label} className="flex items-center gap-2">
                  <Icon size={14} color={ok ? "#10B981" : "#EF4444"} />
                  <span style={{ color: "#6B7280", fontSize: 12 }}>{label}</span>
                </div>
              ))}
            </div>
          </div>
        </div>

        {/* Stats row */}
        <div className="relative z-10 grid grid-cols-4 gap-4">
          {stats.map((s) => (
            <div key={s.label} className="text-center">
              <div style={{ color: "#E5E7EB", fontSize: 18, fontWeight: 700 }}>{s.value}</div>
              <div style={{ color: "#6B7280", fontSize: 11, marginTop: 2 }}>{s.label}</div>
            </div>
          ))}
        </div>
      </div>

      {/* Right Panel - Login */}
      <div className="flex-1 flex items-center justify-center p-8">
        <div className="w-full max-w-md">
          {/* Mobile logo */}
          <div className="flex items-center gap-3 mb-8 lg:hidden">
            <div
              className="w-9 h-9 rounded-xl flex items-center justify-center"
              style={{ background: "linear-gradient(135deg, #4F8CFF, #7C3AED)" }}
            >
              <Shield size={18} color="white" />
            </div>
            <span style={{ color: "#E5E7EB", fontSize: 20, fontWeight: 700 }}>
              OSV <span style={{ color: "#4F8CFF" }}>Platform</span>
            </span>
          </div>

          <div
            className="rounded-2xl p-8"
            style={{
              background: "#151B2F",
              border: "1px solid rgba(255,255,255,0.08)",
              boxShadow: "0 25px 50px rgba(0,0,0,0.5)",
            }}
          >
            <div className="mb-8">
              <h2 style={{ color: "#E5E7EB", fontSize: 24, fontWeight: 700 }}>Welcome back</h2>
              <p style={{ color: "#6B7280", marginTop: 6, fontSize: 14 }}>
                Sign in to your OSV Platform account
              </p>
            </div>

            <form onSubmit={handleSubmit} className="flex flex-col gap-5">
              {/* Email */}
              <div>
                <label style={{ color: "#9CA3AF", fontSize: 13, fontWeight: 500, display: "block", marginBottom: 6 }}>
                  Email address
                </label>
                <div className="relative">
                  <Mail
                    size={16}
                    color="#4B5563"
                    style={{ position: "absolute", left: 14, top: "50%", transform: "translateY(-50%)" }}
                  />
                  <input
                    id="email"
                    type="email"
                    value={email}
                    onChange={(e) => setEmail(e.target.value)}
                    placeholder="analyst@company.com"
                    className="w-full rounded-xl pl-10 pr-4 py-3 outline-none transition-all"
                    style={{
                      background: "rgba(255,255,255,0.05)",
                      border: "1px solid rgba(255,255,255,0.1)",
                      color: "#E5E7EB",
                      fontSize: 14,
                    }}
                    onFocus={(e) => (e.target.style.borderColor = "#4F8CFF")}
                    onBlur={(e) => (e.target.style.borderColor = "rgba(255,255,255,0.1)")}
                  />
                </div>
              </div>

              {/* Password */}
              <div>
                <div className="flex items-center justify-between mb-2">
                  <label style={{ color: "#9CA3AF", fontSize: 13, fontWeight: 500 }}>Password</label>
                  <button
                    type="button"
                    style={{ color: "#4F8CFF", fontSize: 13, background: "none", border: "none", cursor: "pointer" }}
                  >
                    Forgot password?
                  </button>
                </div>
                <div className="relative">
                  <Lock
                    size={16}
                    color="#4B5563"
                    style={{ position: "absolute", left: 14, top: "50%", transform: "translateY(-50%)" }}
                  />
                  <input
                    id="password"
                    type={showPassword ? "text" : "password"}
                    value={password}
                    onChange={(e) => setPassword(e.target.value)}
                    placeholder="••••••••••"
                    className="w-full rounded-xl pl-10 pr-12 py-3 outline-none transition-all"
                    style={{
                      background: "rgba(255,255,255,0.05)",
                      border: "1px solid rgba(255,255,255,0.1)",
                      color: "#E5E7EB",
                      fontSize: 14,
                    }}
                    onFocus={(e) => (e.target.style.borderColor = "#4F8CFF")}
                    onBlur={(e) => (e.target.style.borderColor = "rgba(255,255,255,0.1)")}
                  />
                  <button
                    type="button"
                    onClick={() => setShowPassword(!showPassword)}
                    style={{
                      position: "absolute",
                      right: 14,
                      top: "50%",
                      transform: "translateY(-50%)",
                      background: "none",
                      border: "none",
                      cursor: "pointer",
                      color: "#6B7280",
                    }}
                  >
                    {showPassword ? <EyeOff size={16} /> : <Eye size={16} />}
                  </button>
                </div>
              </div>

              {/* Remember me */}
              <div className="flex items-center gap-3">
                <input
                  type="checkbox"
                  id="remember"
                  checked={rememberMe}
                  onChange={(e) => setRememberMe(e.target.checked)}
                  className="w-4 h-4 rounded"
                  style={{ accentColor: "#4F8CFF" }}
                />
                <label htmlFor="remember" style={{ color: "#9CA3AF", fontSize: 13, cursor: "pointer" }}>
                  Remember me for 30 days
                </label>
              </div>

              {/* Error message */}
              {error && (
                <div
                  role="alert"
                  data-testid="login-error"
                  className="flex items-center gap-2 px-3 py-2 rounded-xl"
                  style={{ background: "rgba(239,68,68,0.1)", border: "1px solid rgba(239,68,68,0.2)", color: "#EF4444", fontSize: 13 }}>
                  <AlertTriangle size={14} />
                  {error}
                </div>
              )}

              {/* Sign In button */}
              <button
                id="login-btn"
                type="submit"
                disabled={isLoading}
                className="w-full py-3 rounded-xl transition-all relative overflow-hidden"
                style={{
                  background: isLoading
                    ? "rgba(79,140,255,0.5)"
                    : "linear-gradient(135deg, #4F8CFF, #3B6FCC)",
                  color: "white",
                  border: "none",
                  cursor: isLoading ? "not-allowed" : "pointer",
                  fontSize: 15,
                  fontWeight: 600,
                  boxShadow: "0 4px 15px rgba(79,140,255,0.3)",
                }}
              >
                {isLoading ? (
                  <div className="flex items-center justify-center gap-2">
                    <div
                      className="w-4 h-4 rounded-full border-2 border-white/30 border-t-white animate-spin"
                    />
                    Authenticating...
                  </div>
                ) : (
                  "Sign In"
                )}
              </button>

              <div className="relative flex items-center gap-3 my-1">
                <div style={{ flex: 1, height: 1, background: "rgba(255,255,255,0.08)" }} />
                <span style={{ color: "#6B7280", fontSize: 12 }}>or continue with</span>
                <div style={{ flex: 1, height: 1, background: "rgba(255,255,255,0.08)" }} />
              </div>

              {/* OAuth buttons */}
              <div className="grid grid-cols-2 gap-3">
                <button
                  type="button"
                  className="flex items-center justify-center gap-2 py-2.5 rounded-xl transition-all"
                  style={{
                    background: "rgba(255,255,255,0.05)",
                    border: "1px solid rgba(255,255,255,0.1)",
                    color: "#E5E7EB",
                    fontSize: 14,
                    cursor: "pointer",
                  }}
                >
                  <svg width="16" height="16" viewBox="0 0 24 24" fill="currentColor">
                    <path d="M22.56 12.25c0-.78-.07-1.53-.2-2.25H12v4.26h5.92c-.26 1.37-1.04 2.53-2.21 3.31v2.77h3.57c2.08-1.92 3.28-4.74 3.28-8.09z" fill="#4285F4"/>
                    <path d="M12 23c2.97 0 5.46-.98 7.28-2.66l-3.57-2.77c-.98.66-2.23 1.06-3.71 1.06-2.86 0-5.29-1.93-6.16-4.53H2.18v2.84C3.99 20.53 7.7 23 12 23z" fill="#34A853"/>
                    <path d="M5.84 14.09c-.22-.66-.35-1.36-.35-2.09s.13-1.43.35-2.09V7.07H2.18C1.43 8.55 1 10.22 1 12s.43 3.45 1.18 4.93l2.85-2.22.81-.62z" fill="#FBBC05"/>
                    <path d="M12 5.38c1.62 0 3.06.56 4.21 1.64l3.15-3.15C17.45 2.09 14.97 1 12 1 7.7 1 3.99 3.47 2.18 7.07l3.66 2.84c.87-2.6 3.3-4.53 6.16-4.53z" fill="#EA4335"/>
                  </svg>
                  Google
                </button>
                <button
                  type="button"
                  className="flex items-center justify-center gap-2 py-2.5 rounded-xl transition-all"
                  style={{
                    background: "rgba(255,255,255,0.05)",
                    border: "1px solid rgba(255,255,255,0.1)",
                    color: "#E5E7EB",
                    fontSize: 14,
                    cursor: "pointer",
                  }}
                >
                  <Github size={16} />
                  GitHub
                </button>
              </div>
            </form>

            <p style={{ color: "#6B7280", fontSize: 13, textAlign: "center", marginTop: 24 }}>
              Don't have an account?{" "}
              <button style={{ color: "#4F8CFF", background: "none", border: "none", cursor: "pointer", fontSize: 13 }}>
                Request access
              </button>
            </p>
          </div>

          {/* Security badges */}
          <div className="flex items-center justify-center gap-4 mt-6">
            {[
              { icon: Lock, label: "SOC 2 Type II" },
              { icon: Shield, label: "ISO 27001" },
              { icon: AlertTriangle, label: "GDPR Compliant" },
            ].map(({ icon: Icon, label }) => (
              <div key={label} className="flex items-center gap-1.5">
                <Icon size={12} color="#6B7280" />
                <span style={{ color: "#6B7280", fontSize: 11 }}>{label}</span>
              </div>
            ))}
          </div>

          <p style={{ color: "#4B5563", fontSize: 11, textAlign: "center", marginTop: 12 }}>
            OSV Platform v3.2.1 · © 2026 OSV Security Inc.
          </p>
        </div>
      </div>
    </div>
  );
}
