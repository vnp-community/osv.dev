import { createBrowserRouter, Navigate, Outlet, useRouteError } from 'react-router';
import { lazy, Suspense, useEffect } from 'react';
import { AuthGuard, FullPageSpinner } from './components/AuthGuard';
import { AppLayout } from './components/AppLayout';
import { useAuth } from '@/features/auth/hooks/useAuth';

// ─── Auth feature ──────────────────────────────────────────────────────────
const LoginScreen = lazy(() =>
  import('@/features/auth/components/LoginScreen').then((m) => ({ default: m.LoginScreen }))
);
const UserProfile = lazy(() =>
  import('@/features/auth/components/UserProfile').then((m) => ({ default: m.UserProfile }))
);
const OnboardingExperience = lazy(() =>
  import('@/features/auth/components/OnboardingExperience').then((m) => ({ default: m.OnboardingExperience }))
);

// ─── Dashboard feature ─────────────────────────────────────────────────────
const Dashboard = lazy(() =>
  import('@/features/dashboard/components/Dashboard').then((m) => ({ default: m.Dashboard }))
);
const RiskOverview = lazy(() =>
  import('@/features/dashboard/components/RiskOverview').then((m) => ({ default: m.RiskOverview }))
);

// ─── CVE Intelligence feature ──────────────────────────────────────────────
const CVESearch = lazy(() =>
  import('@/features/cve-intel/components/CVESearch').then((m) => ({ default: m.CVESearch }))
);
const KEVCatalog = lazy(() =>
  import('@/features/cve-intel/components/KEVCatalog').then((m) => ({ default: m.KEVCatalog }))
);
const SemanticSearch = lazy(() =>
  import('@/features/cve-intel/components/SemanticSearch').then((m) => ({ default: m.SemanticSearch }))
);
const EPSSAnalytics = lazy(() =>
  import('@/features/cve-intel/components/EPSSAnalytics').then((m) => ({ default: m.EPSSAnalytics }))
);
const VendorCatalog = lazy(() =>
  import('@/features/cve-intel/components/VendorCatalog').then((m) => ({ default: m.VendorCatalog }))
);
const CWELibrary = lazy(() =>
  import('@/features/cve-intel/components/CWELibrary').then((m) => ({ default: m.CWELibrary }))
);
const CAPECLibrary = lazy(() =>
  import('@/features/cve-intel/components/CAPECLibrary').then((m) => ({ default: m.CAPECLibrary }))
);

// ─── Scanning feature ──────────────────────────────────────────────────────
const ScanDashboard = lazy(() =>
  import('@/features/scanning/components/ScanDashboard').then((m) => ({ default: m.ScanDashboard }))
);
const ScanWizard = lazy(() =>
  import('@/features/scanning/components/ScanWizard').then((m) => ({ default: m.ScanWizard }))
);
const RunningScan = lazy(() =>
  import('@/features/scanning/components/RunningScan').then((m) => ({ default: m.RunningScan }))
);
const RunningScans = lazy(() =>
  import('@/features/scanning/components/RunningScans').then((m) => ({ default: m.RunningScans }))
);
const ScanHistory = lazy(() =>
  import('@/features/scanning/components/ScanHistory').then((m) => ({ default: m.ScanHistory }))
);
const NmapResults = lazy(() =>
  import('@/features/scanning/components/NmapResults').then((m) => ({ default: m.NmapResults }))
);
const ZAPResults = lazy(() =>
  import('@/features/scanning/components/ZAPResults').then((m) => ({ default: m.ZAPResults }))
);
const LatestNmapRedirect = lazy(() =>
  import('@/features/scanning/components/LatestNmapRedirect').then((m) => ({ default: m.LatestNmapRedirect }))
);
const LatestZAPRedirect = lazy(() =>
  import('@/features/scanning/components/LatestZAPRedirect').then((m) => ({ default: m.LatestZAPRedirect }))
);

// ─── Findings feature ─────────────────────────────────────────────────────
const FindingsList = lazy(() =>
  import('@/features/findings/components/FindingsList').then((m) => ({ default: m.FindingsList }))
);
const FindingDetail = lazy(() =>
  import('@/features/findings/components/FindingDetail').then((m) => ({ default: m.FindingDetail }))
);
const SLADashboard = lazy(() =>
  import('@/features/findings/components/SLADashboard').then((m) => ({ default: m.SLADashboard }))
);
const RiskAcceptanceCenter = lazy(() =>
  import('@/features/findings/components/RiskAcceptanceCenter').then((m) => ({ default: m.RiskAcceptanceCenter }))
);

// ─── Assets feature ───────────────────────────────────────────────────────
const AssetInventory = lazy(() =>
  import('@/features/assets/components/AssetInventory').then((m) => ({ default: m.AssetInventory }))
);
const AssetDetail = lazy(() =>
  import('@/features/assets/components/AssetDetail').then((m) => ({ default: m.AssetDetail }))
);

// ─── Product Security feature ─────────────────────────────────────────────
const ProductSecurity = lazy(() =>
  import('@/features/product-security/components/ProductSecurity').then((m) => ({ default: m.ProductSecurity }))
);

// ─── AI Center feature ────────────────────────────────────────────────────
const AITriage = lazy(() =>
  import('@/features/ai-center/components/AITriage').then((m) => ({ default: m.AITriage }))
);
const AIEnrichment = lazy(() =>
  import('@/features/ai-center/components/AIEnrichment').then((m) => ({ default: m.AIEnrichment }))
);
const AIInsights = lazy(() =>
  import('@/features/ai-center/components/AIInsights').then((m) => ({ default: m.AIInsights }))
);

// ─── Reports feature ──────────────────────────────────────────────────────
const ReportCenter = lazy(() =>
  import('@/features/reports/components/ReportCenter').then((m) => ({ default: m.ReportCenter }))
);

// ─── Notifications feature ────────────────────────────────────────────────
const NotificationCenter = lazy(() =>
  import('@/features/notifications/components/NotificationCenter').then((m) => ({ default: m.NotificationCenter }))
);

// ─── Integrations feature ─────────────────────────────────────────────────
const APIKeyManagement = lazy(() =>
  import('@/features/integrations/components/APIKeyManagement').then((m) => ({ default: m.APIKeyManagement }))
);
const WebhookEvents = lazy(() =>
  import('@/features/integrations/components/WebhookEvents').then((m) => ({ default: m.WebhookEvents }))
);
const JiraConfig = lazy(() =>
  import('@/features/integrations/components/JiraConfig').then((m) => ({ default: m.JiraConfig }))
);

// ─── Admin feature ────────────────────────────────────────────────────────
const UserManagement = lazy(() =>
  import('@/features/admin/components/UserManagement').then((m) => ({ default: m.UserManagement }))
);
const RBACManagement = lazy(() =>
  import('@/features/admin/components/RBACManagement').then((m) => ({ default: m.RBACManagement }))
);
const AuditLogs = lazy(() =>
  import('@/features/admin/components/AuditLogs').then((m) => ({ default: m.AuditLogs }))
);
const SystemHealth = lazy(() =>
  import('@/features/admin/components/SystemHealth').then((m) => ({ default: m.SystemHealth }))
);
const SystemSettings = lazy(() =>
  import('@/features/admin/components/SystemSettings').then((m) => ({ default: m.SystemSettings }))
);

// ─── OAuth Callback ────────────────────────────────────────────────────────
const OAuthCallback = lazy(() =>
  import('@/features/auth/components/OAuthCallback').then((m) => ({ default: m.OAuthCallback }))
);

// ─── Session Restorer ─────────────────────────────────────────────────────
// Runs on every route to restore auth session from httpOnly refresh cookie.
function SessionRestorer() {
  const { restoreSession } = useAuth();
  useEffect(() => {
    restoreSession();
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);
  return <Outlet />;
}

// ─── Suspense page wrapper ─────────────────────────────────────────────────
const P = ({ children }: { children: React.ReactNode }) => (
  <Suspense fallback={<FullPageSpinner />}>{children}</Suspense>
);

// ─── Global Error Boundary ───────────────────────────────────────────────────
function GlobalErrorBoundary() {
  const error = useRouteError() as Error;
  
  useEffect(() => {
    // If it's a dynamic import error (chunk missing after deployment), auto reload once.
    if (error?.message?.includes("Failed to fetch dynamically imported module") || error?.message?.includes("Importing a module script failed")) {
      const reloaded = sessionStorage.getItem("chunk_failed_reload");
      if (!reloaded) {
        sessionStorage.setItem("chunk_failed_reload", "true");
        window.location.reload();
      }
    }
  }, [error]);

  return (
    <div className="flex items-center justify-center min-h-screen text-center p-6" style={{ background: "var(--color-bg-page, #0B1020)", color: "var(--color-text-primary, #E5E7EB)" }}>
      <div>
        <h1 className="text-2xl font-bold mb-4">Oops, something went wrong</h1>
        <p className="text-gray-400 mb-6">{error?.message || "An unexpected error occurred."}</p>
        <button onClick={() => { sessionStorage.removeItem("chunk_failed_reload"); window.location.reload(); }} className="px-5 py-2.5 rounded-xl text-white font-medium cursor-pointer" style={{ background: "linear-gradient(135deg,#4F8CFF,#3B6FCC)", border: "none" }}>
          Refresh Page
        </button>
      </div>
    </div>
  );
}

export const router = createBrowserRouter([
  {
    // Root: runs SessionRestorer for every route
    element: <SessionRestorer />,
    errorElement: <GlobalErrorBoundary />,
    children: [
      // ── Public routes (no auth required) ─────────────────────────────────
      { path: '/login',         element: <P><LoginScreen /></P> },
      { path: '/auth/callback', element: <P><OAuthCallback /></P> },

      // ── Protected shell — AuthGuard checks JWT, AppLayout provides Sidebar+Topbar
      // All child routes automatically inherit the sidebar navigation shell.
      {
        element: <AuthGuard><AppLayout /></AuthGuard>,
        children: [
          // Root → dashboard
          { index: true,                       element: <Navigate to="/dashboard" replace /> },

          // ── Dashboard ─────────────────────────────────────────────────
          { path: '/dashboard',                element: <P><Dashboard /></P> },
          { path: '/dashboard/executive',      element: <P><Dashboard /></P> },
          { path: '/dashboard/risk',           element: <P><RiskOverview /></P> },
          { path: '/dashboard/sla',            element: <P><SLADashboard /></P> },

          // ── CVE Intelligence ──────────────────────────────────────────
          { path: '/cve/search',               element: <P><CVESearch /></P> },
          { path: '/cve/kev',                  element: <P><KEVCatalog /></P> },
          { path: '/cve/semantic',             element: <P><SemanticSearch /></P> },
          { path: '/cve/epss',                 element: <P><EPSSAnalytics /></P> },
          { path: '/cve/vendors',              element: <P><VendorCatalog /></P> },
          { path: '/cve/cwe',                  element: <P><CWELibrary /></P> },
          { path: '/cve/capec',                element: <P><CAPECLibrary /></P> },

          // ── Scanning ──────────────────────────────────────────────────
          { path: '/scans',                    element: <P><ScanDashboard /></P> },
          { path: '/scans/new',                element: <P><ScanWizard /></P> },
          { path: '/scans/running',            element: <P><RunningScans /></P> },
          { path: '/scans/history',            element: <P><ScanHistory /></P> },
          { path: '/scans/:id',                element: <P><RunningScan /></P> },
          { path: '/scans/:id/results/nmap',   element: <P><NmapResults /></P> },
          { path: '/scans/:id/results/zap',    element: <P><ZAPResults /></P> },
          // Sidebar shortcut: resolves latest scan then redirects (TASK-03)
          { path: '/scans/latest/nmap',        element: <P><LatestNmapRedirect /></P> },
          { path: '/scans/latest/zap',         element: <P><LatestZAPRedirect /></P> },

          // ── Findings ──────────────────────────────────────────────────
          { path: '/findings',                 element: <P><FindingsList /></P> },
          { path: '/findings/risk-acceptance', element: <P><RiskAcceptanceCenter /></P> },
          { path: '/findings/:id',             element: <P><FindingDetail /></P> },

          // ── Assets ────────────────────────────────────────────────────
          { path: '/assets',                   element: <P><AssetInventory /></P> },
          { path: '/assets/:id',               element: <P><AssetDetail /></P> },

          // ── Product Security ──────────────────────────────────────────
          { path: '/products',                 element: <P><ProductSecurity /></P> },

          // ── AI Center ─────────────────────────────────────────────────
          { path: '/ai/triage',                element: <P><AITriage /></P> },
          { path: '/ai/enrichment',            element: <P><AIEnrichment /></P> },
          { path: '/ai/insights',              element: <P><AIInsights /></P> },

          // ── Reports ───────────────────────────────────────────────────
          { path: '/reports',                  element: <P><ReportCenter /></P> },

          // ── Notifications ─────────────────────────────────────────────
          { path: '/notifications',            element: <P><NotificationCenter /></P> },

          // ── Integrations ──────────────────────────────────────────────
          { path: '/integrations/api-keys',    element: <P><APIKeyManagement /></P> },
          { path: '/integrations/webhooks',    element: <P><WebhookEvents /></P> },
          { path: '/integrations/jira',        element: <P><JiraConfig /></P> },

          // ── Administration ────────────────────────────────────────────
          { path: '/admin/users',              element: <P><UserManagement /></P> },
          { path: '/admin/roles',              element: <P><RBACManagement /></P> },
          { path: '/admin/audit',              element: <P><AuditLogs /></P> },
          { path: '/admin/health',             element: <P><SystemHealth /></P> },
          { path: '/admin/settings',           element: <P><SystemSettings /></P> },

          // ── User ──────────────────────────────────────────────────────
          { path: '/profile',                  element: <P><UserProfile /></P> },
          { path: '/onboarding',               element: <P><OnboardingExperience /></P> },
        ],
      },

      // Catch-all → dashboard (AuthGuard redirects to /login if unauthenticated)
      { path: '*', element: <Navigate to="/dashboard" replace /> },
    ],
  },
]);
