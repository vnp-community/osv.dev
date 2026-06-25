import { useState } from "react";
import { Sidebar } from "./components/Sidebar";
import { Topbar } from "./components/Topbar";
import { Dashboard } from "./components/Dashboard";
import { CVESearch } from "../features/cve-intel/components/CVESearch";
import { KEVCatalog } from "../features/cve-intel/components/KEVCatalog";
import { SemanticSearch } from "../features/cve-intel/components/SemanticSearch";
import { VendorCatalog } from "../features/cve-intel/components/VendorCatalog";
import { CWELibrary } from "../features/cve-intel/components/CWELibrary";
import { EPSSAnalytics } from "../features/cve-intel/components/EPSSAnalytics";
import { ScanDashboard } from "./components/ScanDashboard";
import { ScanWizard } from "./components/ScanWizard";
import { RunningScan } from "./components/RunningScan";
import { ScanHistory } from "./components/ScanHistory";
import { NmapResults } from "./components/NmapResults";
import { ZAPResults } from "./components/ZAPResults";
import { FindingsList } from "./components/FindingsList";
import { FindingDetail } from "./components/FindingDetail";
import { AssetInventory } from "./components/AssetInventory";
import { AssetDetail } from "./components/AssetDetail";
import { ProductSecurity } from "./components/ProductSecurity";
import { AITriage } from "./components/AITriage";
import { AIEnrichment } from "./components/AIEnrichment";
import { ReportCenter } from "./components/ReportCenter";
import { NotificationCenter } from "./components/NotificationCenter";
import { APIKeyManagement } from "./components/APIKeyManagement";
import { WebhookEvents } from "./components/WebhookEvents";
import { UserManagement } from "./components/UserManagement";
import { AuditLogs } from "./components/AuditLogs";
import { GlobalSearch } from "./components/GlobalSearch";
import { SLADashboard } from "./components/SLADashboard";
import { RiskAcceptanceCenter } from "./components/RiskAcceptanceCenter";
import { RBACManagement } from "./components/RBACManagement";
import { UserProfile } from "./components/UserProfile";
import { SystemHealth } from "./components/SystemHealth";
import { SystemSettings } from "./components/SystemSettings";
import { OnboardingExperience } from "./components/OnboardingExperience";
import { Plus } from "lucide-react";

type Screen = "login" | "app";
type AppView = string;

const BREADCRUMBS: Record<string, string[]> = {
  dashboard: ["Home", "Executive Dashboard"],
  "exec-overview": ["Home", "Dashboard", "Executive Overview"],
  "risk-overview": ["Home", "Dashboard", "Risk Overview"],
  "sla-dashboard": ["Home", "Dashboard", "SLA Dashboard"],
  "global-search": ["Home", "Global Search"],
  "cve-search": ["Home", "Vulnerability Intel", "CVE Search"],
  "semantic-search": ["Home", "Vulnerability Intel", "Semantic Search"],
  "kev-catalog": ["Home", "Vulnerability Intel", "KEV Catalog"],
  "epss-analytics": ["Home", "Vulnerability Intel", "EPSS Analytics"],
  "cwe-catalog": ["Home", "Vulnerability Intel", "CWE Library"],
  "capec-catalog": ["Home", "Vulnerability Intel", "CAPEC Library"],
  "vendor-catalog": ["Home", "Vulnerability Intel", "Vendor Catalog"],
  scanning: ["Home", "Active Scanning"],
  "scan-dashboard": ["Home", "Active Scanning", "Scan Dashboard"],
  "new-scan": ["Home", "Active Scanning", "New Scan"],
  "running-scans": ["Home", "Active Scanning", "Running Scans"],
  "running-scan-detail": ["Home", "Active Scanning", "SC-0047"],
  "scan-history": ["Home", "Active Scanning", "Scan History"],
  "nmap-results": ["Home", "Active Scanning", "Nmap Results"],
  "zap-results": ["Home", "Active Scanning", "ZAP Results"],
  findings: ["Home", "Findings"],
  "all-findings": ["Home", "Findings", "All Findings"],
  "active-findings": ["Home", "Findings", "Active"],
  "finding-detail": ["Home", "Findings", "F-2847"],
  "sla-breaches": ["Home", "Findings", "SLA Dashboard"],
  "risk-acceptance": ["Home", "Findings", "Risk Acceptance"],
  assets: ["Home", "Assets"],
  "asset-inventory": ["Home", "Assets", "Asset Inventory"],
  "asset-detail": ["Home", "Assets", "10.0.1.45"],
  "product-security": ["Home", "Product Security"],
  products: ["Home", "Product Security", "Products"],
  engagements: ["Home", "Product Security", "Engagements"],
  scorecards: ["Home", "Product Security", "Scorecards"],
  "ai-center": ["Home", "AI Center"],
  "ai-triage": ["Home", "AI Center", "AI Triage"],
  "ai-enrichment": ["Home", "AI Center", "AI Enrichment"],
  "ai-insights": ["Home", "AI Center", "AI Insights"],
  reports: ["Home", "Reports"],
  "exec-reports": ["Home", "Reports", "Executive"],
  "tech-reports": ["Home", "Reports", "Technical"],
  "compliance-reports": ["Home", "Reports", "Compliance"],
  notifications: ["Home", "Notifications"],
  integrations: ["Home", "Integrations"],
  "api-keys": ["Home", "Integrations", "API Keys"],
  webhooks: ["Home", "Integrations", "Webhooks"],
  jira: ["Home", "Integrations", "Jira"],
  admin: ["Home", "Administration"],
  users: ["Home", "Administration", "Users"],
  roles: ["Home", "Administration", "Roles & Permissions"],
  "audit-logs": ["Home", "Administration", "Audit Logs"],
  "system-settings": ["Home", "Administration", "System Settings"],
  "system-health": ["Home", "Administration", "System Health"],
  "user-profile": ["Home", "User Profile"],
  onboarding: ["Home", "Getting Started"],
};

export default function App() {
  const [view, setView] = useState<AppView>("dashboard");

  const navigate = (v: AppView) => setView(v);

  const breadcrumbs = BREADCRUMBS[view] || ["Home"];

  const renderContent = () => {
    // Dashboard
    if (["dashboard", "exec-overview", "risk-overview"].includes(view)) return <Dashboard />;
    if (view === "sla-dashboard" || view === "sla-breaches") return <SLADashboard />;
    if (view === "global-search") return <GlobalSearch onNavigate={navigate} />;

    // CVE Intelligence
    if (["cve-search", "epss-analytics"].includes(view)) return <CVESearch />;
    if (view === "kev-catalog") return <KEVCatalog />;
    if (view === "semantic-search") return <SemanticSearch />;
    if (view === "vendor-catalog") return <VendorCatalog onSelectVendor={() => navigate("cve-search")} />;
    if (view === "cwe-catalog" || view === "capec-catalog") return <CWELibrary />;
    if (view === "epss-analytics") return <EPSSAnalytics />;

    // Scanning
    if (["scanning", "scan-dashboard"].includes(view)) return <ScanDashboard onNewScan={() => navigate("new-scan")} onViewScan={() => navigate("running-scan-detail")} />;
    if (view === "new-scan") return <ScanWizard onClose={() => navigate("scan-dashboard")} onStart={() => navigate("running-scan-detail")} />;
    if (["running-scans", "running-scan-detail"].includes(view)) return <RunningScan onBack={() => navigate("scan-dashboard")} />;
    if (view === "scan-history") return <ScanHistory onViewScan={() => navigate("running-scan-detail")} />;
    if (view === "nmap-results") return <NmapResults onBack={() => navigate("scan-dashboard")} />;
    if (view === "zap-results") return <ZAPResults />;

    // Findings
    if (["findings", "all-findings", "active-findings", "mitigated", "false-positive"].includes(view)) return <FindingsList />;
    if (view === "finding-detail") return <FindingDetail onBack={() => navigate("all-findings")} />;
    if (view === "risk-acceptance") return <RiskAcceptanceCenter />;

    // Assets
    if (["assets", "asset-inventory"].includes(view)) return <AssetInventory onViewDetail={() => navigate("asset-detail")} />;
    if (view === "asset-detail") return <AssetDetail />;

    // Product Security
    if (["product-security", "products", "engagements", "scorecards"].includes(view)) return <ProductSecurity />;

    // AI Center
    if (["ai-center", "ai-triage", "ai-insights"].includes(view)) return <AITriage />;
    if (view === "ai-enrichment") return <AIEnrichment />;

    // Reports
    if (["reports", "exec-reports", "tech-reports", "compliance-reports"].includes(view)) return <ReportCenter />;

    // Notifications
    if (view === "notifications") return <NotificationCenter />;

    // Integrations
    if (["integrations", "api-keys"].includes(view)) return <APIKeyManagement />;
    if (view === "webhooks") return <WebhookEvents />;

    // Admin
    if (view === "users") return <UserManagement />;
    if (["audit-logs", "audit-timeline"].includes(view)) return <AuditLogs />;
    if (["system-settings", "security-settings", "ai-settings"].includes(view)) return <SystemSettings />;
    if (view === "system-health") return <SystemHealth />;
    if (view === "roles") return <RBACManagement />;

    // User
    if (view === "user-profile") return <UserProfile />;
    if (view === "onboarding") return <OnboardingExperience onNavigate={navigate} />;

    // Fallback
    return (
      <div className="flex-1 flex items-center justify-center" style={{ background: "#0B1020" }}>
        <div className="text-center">
          <div className="w-16 h-16 rounded-2xl flex items-center justify-center mx-auto mb-4" style={{ background: "rgba(79,140,255,0.1)", border: "1px solid rgba(79,140,255,0.2)" }}>
            <span style={{ color: "#4F8CFF", fontSize: 28 }}>🔐</span>
          </div>
          <h2 style={{ color: "#E5E7EB", fontSize: 18, fontWeight: 600 }}>{breadcrumbs[breadcrumbs.length - 1]}</h2>
          <p style={{ color: "#6B7280", fontSize: 13, marginTop: 8, maxWidth: 300 }}>Navigate using the sidebar to explore all 60 screens.</p>
          <button onClick={() => navigate("dashboard")} className="mt-6 px-5 py-2.5 rounded-xl" style={{ background: "linear-gradient(135deg,#4F8CFF,#3B6FCC)", color: "white", border: "none", cursor: "pointer", fontSize: 13 }}>
            ← Back to Dashboard
          </button>
        </div>
      </div>
    );
  };

  const showScanButton = ["dashboard", "exec-overview", "scanning", "scan-dashboard"].includes(view);

  return (
    <div className="w-full h-screen flex flex-col overflow-hidden" style={{ background: "#0B1020", fontFamily: "'Inter', sans-serif" }}>
      <div className="flex flex-1 overflow-hidden">
        <Sidebar />
        <div className="flex flex-col flex-1 overflow-hidden">
          <Topbar
            breadcrumbs={breadcrumbs}
            actions={
              showScanButton ? (
                <button onClick={() => navigate("new-scan")} className="flex items-center gap-2 px-3 py-1.5 rounded-xl"
                  style={{ background: "rgba(79,140,255,0.1)", border: "1px solid rgba(79,140,255,0.25)", color: "#4F8CFF", fontSize: 12, cursor: "pointer" }}>
                  <Plus size={13} />New Scan
                </button>
              ) : undefined
            }
          />
          {renderContent()}
        </div>
      </div>
    </div>
  );
}
