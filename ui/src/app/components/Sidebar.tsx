import { useState, useEffect } from "react";
import { useNavigate, useLocation } from "react-router";
import {
  LayoutDashboard, Shield, Scan, AlertTriangle, Server, Package,
  Brain, FileText, Bell, Plug, Settings, ChevronDown, ChevronRight,
  Activity, Search, Key, Users, Zap, Globe, BookOpen, Network,
  Clock, ShieldAlert, Cpu, GitBranch, Webhook, Database, HeartPulse,
  Rocket, UserCircle
} from "lucide-react";
import { useAuthStore } from "@/features/auth/store/authStore";

// Section-id → URL path mapping
const SECTION_TO_PATH: Record<string, string> = {
  "dashboard":      "/dashboard",
  "exec-overview":  "/dashboard/executive",
  "risk-overview":  "/dashboard/risk",
  "sla-dashboard":  "/dashboard/sla",
  "global-search": "/",
  "cve-search": "/cve/search",
  "semantic-search": "/cve/semantic",
  "vendor-catalog": "/cve/vendors",
  "kev-catalog": "/cve/kev",
  "epss-analytics": "/cve/epss",
  "cwe-catalog": "/cve/cwe",
  "capec-catalog": "/cve/capec",
  "scan-dashboard": "/scans",
  "new-scan": "/scans/new",
  "running-scans": "/scans/running",
  "scan-history": "/scans/history",
  "nmap-results": "/scans/latest/nmap",
  "zap-results": "/scans/latest/zap",
  "all-findings": "/findings",
  "active-findings": "/findings?status=active",
  "mitigated":       "/findings?status=mitigated",
  "sla-breaches": "/dashboard/sla",
  "risk-acceptance": "/findings/risk-acceptance",
  "asset-inventory": "/assets",
  products: "/products",
  engagements: "/products?tab=engagements",
  scorecards: "/products?tab=scorecards",
  "ai-triage": "/ai/triage",
  "ai-enrichment": "/ai/enrichment",
  "ai-insights": "/ai/insights",
  "exec-reports": "/reports?type=executive",
  "tech-reports": "/reports?type=technical",
  "compliance-reports": "/reports?type=compliance",
  notifications: "/notifications",
  "api-keys": "/integrations/api-keys",
  webhooks: "/integrations/webhooks",
  jira: "/integrations/jira",
  users: "/admin/users",
  roles: "/admin/roles",
  "audit-logs": "/admin/audit",
  "system-health": "/admin/health",
  "system-settings": "/admin/settings",
  onboarding: "/onboarding",
  "user-profile": "/profile",
  // ── Parent node navigation defaults (TASK-01) ──────────────────────────
  "vuln-intel":       "/cve/search",
  "scanning":         "/scans",
  "findings":         "/findings",
  "assets":           "/assets",
  "product-security": "/products",
  "ai-center":        "/ai/triage",
  "reports":          "/reports",
  "integrations":     "/integrations/api-keys",
  "admin":            "/admin/users",
};

// Derive active section from current pathname+search — longest match wins.
// Handles two kinds of entries in SECTION_TO_PATH:
//   • Pure paths  e.g. "/dashboard/sla"         → matched via pathname.startsWith
//   • Path+query  e.g. "/findings?status=active" → matched via exact pathname+search
function pathToSection(pathname: string, search: string): string {
  // Special handling for dynamic scan result routes that don't match the literal shortcuts in SECTION_TO_PATH
  if (pathname.match(/^\/scans\/[^/]+\/results\/nmap\/?$/)) return "nmap-results";
  if (pathname.match(/^\/scans\/[^/]+\/results\/zap\/?$/)) return "zap-results";

  let best = "dashboard";
  let bestLen = 0;
  for (const [section, entry] of Object.entries(SECTION_TO_PATH)) {
    if (entry === "/") continue;
    const qMark = entry.indexOf("?");
    if (qMark === -1) {
      // Pure-path entry: use startsWith on pathname
      if (pathname.startsWith(entry) && entry.length > bestLen) {
        best = section;
        bestLen = entry.length;
      }
    } else {
      // Path+query entry: require exact pathname match AND query string match
      const entryPath  = entry.slice(0, qMark);
      const entryQuery = "?" + entry.slice(qMark + 1);
      if (pathname === entryPath && search === entryQuery && entry.length > bestLen) {
        best = section;
        bestLen = entry.length;
      }
    }
  }
  return best;
}

const navItems = [
  {
    id: "dashboard", label: "Dashboard", icon: LayoutDashboard,
    children: [
      { id: "exec-overview", label: "Executive Overview" },
      { id: "risk-overview", label: "Risk Overview" },
      { id: "sla-dashboard", label: "SLA Dashboard" },
    ],
  },
  {
    id: "vuln-intel", label: "Vulnerability Intel", icon: Search,
    children: [
      { id: "cve-search", label: "CVE Search" },
      { id: "semantic-search", label: "Semantic Search" },
      { id: "vendor-catalog", label: "Vendor Catalog" },
      { id: "kev-catalog", label: "KEV Catalog" },
      { id: "epss-analytics", label: "EPSS Analytics" },
      { id: "cwe-catalog", label: "CWE Library" },
      { id: "capec-catalog", label: "CAPEC Library" },
    ],
  },
  {
    id: "scanning", label: "Active Scanning", icon: Scan, badge: "3", badgeColor: "#4F8CFF",
    children: [
      { id: "scan-dashboard", label: "Scan Dashboard" },
      { id: "new-scan", label: "New Scan" },
      { id: "running-scans", label: "Running Scans", badge: "3" },
      { id: "scan-history", label: "Scan History" },
      { id: "nmap-results", label: "Nmap Results" },
      { id: "zap-results", label: "ZAP Results" },
    ],
  },
  {
    id: "findings", label: "Findings", icon: AlertTriangle, badge: "245", badgeColor: "#EF4444",
    children: [
      { id: "all-findings", label: "All Findings", badge: "245" },
      { id: "active-findings", label: "Active" },
      { id: "mitigated", label: "Mitigated" },
      { id: "sla-breaches", label: "SLA Dashboard" },
      { id: "risk-acceptance", label: "Risk Acceptance" },
    ],
  },
  {
    id: "assets", label: "Assets", icon: Server,
    children: [
      { id: "asset-inventory", label: "Asset Inventory" },
      // Asset Detail is a context-specific page (/assets/:id), not a valid sidebar entry point
    ],
  },
  {
    id: "product-security", label: "Product Security", icon: Package,
    children: [
      { id: "products", label: "Products" },
      { id: "engagements", label: "Engagements" },
      { id: "scorecards", label: "Security Scorecards" },
    ],
  },
  {
    id: "ai-center", label: "AI Center", icon: Brain,
    children: [
      { id: "ai-triage", label: "AI Triage Queue" },
      { id: "ai-enrichment", label: "AI Enrichment" },
      { id: "ai-insights", label: "AI Insights" },
    ],
  },
  {
    id: "reports", label: "Reports", icon: FileText,
    children: [
      { id: "exec-reports", label: "Executive Reports" },
      { id: "tech-reports", label: "Technical Reports" },
      { id: "compliance-reports", label: "Compliance Reports" },
    ],
  },
  { id: "notifications", label: "Notifications", icon: Bell, badge: "12", badgeColor: "#F59E0B" },
  {
    id: "integrations", label: "Integrations", icon: Plug,
    children: [
      { id: "api-keys", label: "API Keys" },
      { id: "webhooks", label: "Webhooks" },
      { id: "jira", label: "Jira" },
    ],
  },
  {
    id: "admin", label: "Administration", icon: Settings,
    children: [
      { id: "users", label: "Users" },
      { id: "roles", label: "Roles & Permissions" },
      { id: "audit-logs", label: "Audit Logs" },
      { id: "system-health", label: "System Health" },
      { id: "system-settings", label: "System Settings" },
    ],
  },
];

const SECTION_CHILDREN: Record<string, string[]> = {
  // "finding-detail" kept so /findings/:id correctly highlights the parent
  findings: ["finding-detail", "all-findings", "active-findings", "mitigated", "sla-breaches", "risk-acceptance"],
  // "sla-dashboard" kept so /dashboard/sla highlights the Dashboard parent
  dashboard: ["exec-overview", "risk-overview", "sla-dashboard"],
  scanning: ["scan-dashboard", "new-scan", "running-scans", "scan-history", "nmap-results", "zap-results"],
  // "asset-detail" kept so /assets/:id correctly highlights the Assets parent
  assets: ["asset-inventory", "asset-detail"],
  "ai-center": ["ai-triage", "ai-enrichment", "ai-insights"],
  "vuln-intel": ["cve-search", "semantic-search", "vendor-catalog", "kev-catalog", "epss-analytics", "cwe-catalog", "capec-catalog"],
  "product-security": ["products", "engagements", "scorecards"],
  reports: ["exec-reports", "tech-reports", "compliance-reports"],
  integrations: ["api-keys", "webhooks", "jira"],
  admin: ["users", "roles", "audit-logs", "system-health", "system-settings"],
};

// ── Helper: find which top-level navItem is the parent of a given section id ──
// Returns the parent item id, or null if sectionId is itself a top-level item.
function findParentId(sectionId: string): string | null {
  for (const item of navItems) {
    if (!item.children) continue;
    if (item.children.some(c => c.id === sectionId)) return item.id;
    const extra = SECTION_CHILDREN[item.id] ?? [];
    if (extra.includes(sectionId)) return item.id;
  }
  return null;
}

export function Sidebar() {
  const navigate = useNavigate();
  const location = useLocation();
  const activeSection = pathToSection(location.pathname, location.search);
  const user = useAuthStore((state) => state.user);

  const getInitials = (name?: string, email?: string) => {
    if (name) return name.split(" ").map(n => n[0]).join("").toUpperCase().substring(0, 2);
    if (email) return email.substring(0, 2).toUpperCase();
    return "U";
  };
  
  const displayName = user?.name || user?.email?.split('@')[0] || "User";
  const initials = getInitials(user?.name, user?.email);
  const displayRole = user?.role ? user.role.charAt(0).toUpperCase() + user.role.slice(1) : "Guest";

  // Lazy initializer: open dashboard + the parent of whichever section is active on first render.
  const [expanded, setExpanded] = useState<string[]>(() => {
    const set = new Set(["dashboard"]);
    const p = findParentId(activeSection);
    if (p) set.add(p);
    // If activeSection itself is a top-level item with children, expand it too.
    const self = navItems.find(i => i.id === activeSection);
    if (self?.children) set.add(activeSection);
    return Array.from(set);
  });

  // Auto-expand the correct parent section whenever the route changes.
  // This ensures the highlighted child is always visible regardless of prior
  // collapse actions or cross-section navigation (e.g. sla-breaches → /dashboard/sla).
  useEffect(() => {
    const p = findParentId(activeSection);
    if (p) {
      setExpanded(prev => prev.includes(p) ? prev : [...prev, p]);
      return;
    }
    // activeSection may itself be a top-level item with children → expand it.
    const self = navItems.find(i => i.id === activeSection);
    if (self?.children) {
      setExpanded(prev => prev.includes(activeSection) ? prev : [...prev, activeSection]);
    }
  }, [activeSection]);

  const handleNavigate = (sectionId: string) => {
    const path = SECTION_TO_PATH[sectionId];
    if (path) navigate(path);
  };

  const toggleExpand = (id: string) => {
    setExpanded(prev => prev.includes(id) ? prev.filter(x => x !== id) : [...prev, id]);
  };

  const isParentActive = (item: typeof navItems[number]) => {
    if (activeSection === item.id) return true;
    if (item.children?.some(c => c.id === activeSection)) return true;
    const extra = SECTION_CHILDREN[item.id] ?? [];
    return extra.includes(activeSection);
  };

  // For child items: a child is active if its id matches activeSection,
  // OR if the child's path maps to the same URL as the current active section
  // (covers cross-section aliases like sla-breaches ↔ sla-dashboard).
  const isChildActive = (childId: string) => {
    if (activeSection === childId) return true;
    const childPath = SECTION_TO_PATH[childId];
    const activePath = SECTION_TO_PATH[activeSection];
    return !!childPath && !!activePath && childPath === activePath;
  };

  return (
    <div className="flex flex-col h-full w-56 flex-shrink-0" style={{ background: "#0F1629", borderRight: "1px solid rgba(255,255,255,0.06)" }}>
      {/* Logo */}
      <div className="flex items-center gap-3 px-5 py-4" style={{ borderBottom: "1px solid rgba(255,255,255,0.06)" }}>
        <div className="w-8 h-8 rounded-lg flex items-center justify-center flex-shrink-0" style={{ background: "linear-gradient(135deg,#4F8CFF,#7C3AED)" }}>
          <Shield size={16} color="white" />
        </div>
        <div>
          <div style={{ color: "#E5E7EB", fontSize: 14, fontWeight: 700, lineHeight: 1 }}>OSV <span style={{ color: "#4F8CFF" }}>Platform</span></div>
          <div style={{ color: "#4B5563", fontSize: 10, marginTop: 2 }}>Enterprise Security</div>
        </div>
      </div>

      {/* Quick stats */}
      <div className="px-4 py-3" style={{ borderBottom: "1px solid rgba(255,255,255,0.06)" }}>
        <div className="grid grid-cols-3 gap-2">
          <div className="rounded-lg p-2 text-center" style={{ background: "rgba(239,68,68,0.1)" }}>
            <div style={{ color: "#EF4444", fontSize: 14, fontWeight: 700 }}>245</div>
            <div style={{ color: "#6B7280", fontSize: 9 }}>Critical</div>
          </div>
          <div className="rounded-lg p-2 text-center" style={{ background: "rgba(79,140,255,0.1)" }}>
            <div style={{ color: "#4F8CFF", fontSize: 14, fontWeight: 700 }}>3</div>
            <div style={{ color: "#6B7280", fontSize: 9 }}>Running</div>
          </div>
          <div className="rounded-lg p-2 text-center" style={{ background: "rgba(16,185,129,0.1)" }}>
            <div style={{ color: "#10B981", fontSize: 14, fontWeight: 700 }}>98%</div>
            <div style={{ color: "#6B7280", fontSize: 9 }}>SLA</div>
          </div>
        </div>
      </div>



      {/* Nav */}
      <nav className="flex-1 overflow-y-auto py-3 px-2" style={{ scrollbarWidth: "none" }}>
        {navItems.map(item => {
          const Icon = item.icon;
          const isExpanded = expanded.includes(item.id);
          const isActive = isParentActive(item);

          return (
            <div key={item.id} className="mb-0.5">
              <button
                onClick={() => { if (item.children) toggleExpand(item.id); handleNavigate(item.id); }}
                className="w-full flex items-center gap-2.5 px-3 py-2 rounded-lg transition-all text-left"
                style={{ background: isActive ? "rgba(79,140,255,0.12)" : "transparent", color: isActive ? "#4F8CFF" : "#9CA3AF", fontSize: 13 }}
              >
                <Icon size={15} />
                <span style={{ flex: 1, fontWeight: isActive ? 600 : 400 }}>{item.label}</span>
                {(item as any).badge && (
                  <span className="rounded-full px-1.5 py-0.5 text-white" style={{ background: (item as any).badgeColor || "#4F8CFF", fontSize: 10, fontWeight: 600 }}>
                    {(item as any).badge}
                  </span>
                )}
                {item.children && (isExpanded ? <ChevronDown size={12} /> : <ChevronRight size={12} />)}
              </button>

              {item.children && isExpanded && (
                <div className="ml-5 mt-0.5 mb-1 border-l pl-3" style={{ borderColor: "rgba(255,255,255,0.06)" }}>
                  {item.children.map(child => (
                    <button key={child.id} onClick={() => handleNavigate(child.id)}
                      className="w-full flex items-center justify-between px-2 py-1.5 rounded-lg transition-all text-left"
                      style={{ background: isChildActive(child.id) ? "rgba(79,140,255,0.1)" : "transparent", color: isChildActive(child.id) ? "#4F8CFF" : "#6B7280", fontSize: 12 }}
                    >
                      <span>{child.label}</span>
                      {(child as any).badge && (
                        <span className="rounded-full px-1.5 py-0.5 text-white" style={{ background: "#EF4444", fontSize: 10 }}>{(child as any).badge}</span>
                      )}
                    </button>
                  ))}
                </div>
              )}
            </div>
          );
        })}

        {/* Extra quick links */}
        <div className="mt-3 pt-3" style={{ borderTop: "1px solid rgba(255,255,255,0.06)" }}>
          {[
            { id: "onboarding", icon: Rocket, label: "Getting Started" },
            { id: "user-profile", icon: UserCircle, label: "My Profile" },
          ].map(({ id, icon: Icon, label }) => (
            <button key={id} onClick={() => handleNavigate(id)}
              className="w-full flex items-center gap-2.5 px-3 py-2 rounded-lg mb-0.5"
              style={{ background: activeSection === id ? "rgba(79,140,255,0.12)" : "transparent", color: activeSection === id ? "#4F8CFF" : "#6B7280", fontSize: 12, border: "none", cursor: "pointer" }}
            >
              <Icon size={14} />{label}
            </button>
          ))}
        </div>
      </nav>

      {/* User footer */}
      <div className="px-4 py-3 flex items-center gap-3" style={{ borderTop: "1px solid rgba(255,255,255,0.06)" }}>
        <div className="w-8 h-8 rounded-full flex items-center justify-center text-white flex-shrink-0" style={{ background: "linear-gradient(135deg,#4F8CFF,#7C3AED)", fontSize: 12, fontWeight: 700 }}>
          {initials}
        </div>
        <div className="flex-1 min-w-0">
          <div className="truncate" style={{ color: "#E5E7EB", fontSize: 12, fontWeight: 600 }}>{displayName}</div>
          <div style={{ color: "#6B7280", fontSize: 10 }}>{displayRole}</div>
        </div>
        <button onClick={() => handleNavigate("system-settings")} style={{ color: "#4B5563", background: "none", border: "none", cursor: "pointer" }}>
          <Settings size={14} />
        </button>
      </div>
    </div>
  );
}
