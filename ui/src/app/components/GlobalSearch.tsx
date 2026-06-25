import { useState, useEffect, useRef } from "react";
import { Search, AlertTriangle, Server, Package, FileText, User, Shield, Clock, ArrowRight, Command, Hash, X } from "lucide-react";
import { useSearchStore } from "@/shared/stores/useSearchStore";
import { useNavigate } from "react-router";

const RECENT = [
  "CVE-2025-44228",
  "webserver01.prod findings",
  "Banking App security report",
  "log4j vulnerabilities",
];

const SUGGESTED = [
  "Critical findings due this week",
  "Assets with KEV vulnerabilities",
  "Products with grade below B",
  "Scans completed today",
];

const ALL_RESULTS = [
  { type: "cve", icon: Shield, color: "#EF4444", name: "CVE-2025-44228", desc: "Apache Log4j2 JNDI Remote Code Execution — CVSS 10.0", updated: "2h ago", badge: "CRITICAL", path: "/cve/search?q=CVE-2025-44228" },
  { type: "cve", icon: Shield, color: "#EF4444", name: "CVE-2025-22965", desc: "Spring Framework Path Traversal — CVSS 9.8", updated: "4h ago", badge: "CRITICAL", path: "/cve/search?q=CVE-2025-22965" },
  { type: "finding", icon: AlertTriangle, color: "#F97316", name: "F-2847 — Log4j RCE on webserver01", desc: "Status: Active · Product: Banking App · SLA: 2d left", updated: "10m ago", badge: "ACTIVE", path: "/findings/F-2847" },
  { type: "finding", icon: AlertTriangle, color: "#F97316", name: "F-2846 — Spring RCE on api-gw", desc: "Status: Active · Product: API Gateway · OVERDUE", updated: "25m ago", badge: "OVERDUE", path: "/findings/F-2846" },
  { type: "asset", icon: Server, color: "#4F8CFF", name: "webserver01.prod (10.0.1.45)", desc: "Ubuntu 22.04 · Risk Score: 9.8 · 8 active findings", updated: "2h ago", badge: "HIGH RISK", path: "/assets/webserver01" },
  { type: "asset", icon: Server, color: "#4F8CFF", name: "db01.prod (10.0.2.11)", desc: "CentOS 8 · Risk Score: 8.9 · 5 active findings", updated: "2h ago", badge: "HIGH RISK", path: "/assets/db01" },
  { type: "product", icon: Package, color: "#10B981", name: "Banking App", desc: "Grade B · 8 critical · 24 high findings", updated: "1d ago", badge: "GRADE B", path: "/products/banking-app" },
  { type: "product", icon: Package, color: "#10B981", name: "API Gateway", desc: "Grade C+ · 14 critical · 38 high findings", updated: "1d ago", badge: "GRADE C+", path: "/products/api-gateway" },
  { type: "report", icon: FileText, color: "#A78BFA", name: "Q2 2026 Executive Summary", desc: "Executive Report · 2.4 MB · Ready to download", updated: "6h ago", badge: "READY", path: "/reports/q2-2026" },
  { type: "user", icon: User, color: "#6B7280", name: "Bob Chen", desc: "Security Analyst · bob.chen@company.com · MFA enabled", updated: "1h ago", badge: "ACTIVE", path: "/users/bob.chen" },
];

const TYPE_LABELS: Record<string, string> = {
  cve: "CVEs", finding: "Findings", asset: "Assets", product: "Products", report: "Reports", user: "Users",
};

const SHORTCUTS = [
  { key: "⌘K", desc: "Open search" },
  { key: "↑↓", desc: "Navigate" },
  { key: "↵", desc: "Open" },
  { key: "Esc", desc: "Close" },
];

export function GlobalSearch() {
  const { isOpen, closeSearch } = useSearchStore();
  const navigate = useNavigate();
  const [query, setQuery] = useState("");
  const [focused, setFocused] = useState(-1);
  const [activeCategory, setActiveCategory] = useState("all");
  const inputRef = useRef<HTMLInputElement>(null);

  useEffect(() => {
    if (isOpen) {
      setQuery("");
      setActiveCategory("all");
      setTimeout(() => inputRef.current?.focus(), 50);
    }
  }, [isOpen]);

  useEffect(() => {
    const handleKeyDown = (e: KeyboardEvent) => {
      if (e.key === 'Escape') {
        closeSearch();
      }
      // Add global Cmd+K listener
      if ((e.metaKey || e.ctrlKey) && e.key === 'k') {
        e.preventDefault();
        useSearchStore.getState().toggleSearch();
      }
    };
    window.addEventListener('keydown', handleKeyDown);
    return () => window.removeEventListener('keydown', handleKeyDown);
  }, [closeSearch]);

  if (!isOpen) return null;

  const filtered = query.length > 1
    ? ALL_RESULTS.filter(r =>
        r.name.toLowerCase().includes(query.toLowerCase()) ||
        r.desc.toLowerCase().includes(query.toLowerCase())
      )
    : [];

  const categories = ["all", ...Object.keys(TYPE_LABELS)];

  const shown = activeCategory === "all" ? filtered : filtered.filter(r => r.type === activeCategory);

  const grouped: Record<string, typeof ALL_RESULTS> = {};
  for (const r of shown) {
    if (!grouped[r.type]) grouped[r.type] = [];
    grouped[r.type].push(r);
  }

  const handleResultClick = (path: string) => {
    closeSearch();
    navigate(path);
  };

  return (
    <div className="fixed inset-0 z-50 flex justify-center pt-[10vh] px-4" style={{ background: "rgba(0,0,0,0.6)", backdropFilter: "blur(4px)" }} onClick={closeSearch}>
      <div 
        className="w-full max-w-3xl max-h-[80vh] flex flex-col rounded-2xl overflow-hidden shadow-2xl relative" 
        style={{ background: "#0B1020", border: "1px solid rgba(255,255,255,0.1)" }}
        onClick={e => e.stopPropagation()}
      >
        {/* Header search */}
        <div className="flex items-center px-4 py-4" style={{ borderBottom: "1px solid rgba(255,255,255,0.06)", background: "#151B2F" }}>
          <Search size={20} color="#4B5563" className="mr-3" />
          <input
            ref={inputRef}
            value={query}
            onChange={e => setQuery(e.target.value)}
            placeholder="Search CVEs, findings, assets, products, reports..."
            className="flex-1 bg-transparent outline-none border-none"
            style={{ color: "#E5E7EB", fontSize: 16 }}
          />
          <button onClick={closeSearch} className="ml-3 p-1.5 rounded-lg hover:bg-white/5 transition-colors">
            <X size={18} color="#9CA3AF" />
          </button>
        </div>

        <div className="flex-1 overflow-y-auto px-6 py-5">
          {!query && (
            <div className="grid grid-cols-2 gap-6">
              {/* Recent */}
              <div>
                <div style={{ color: "#6B7280", fontSize: 11, fontWeight: 600, letterSpacing: 1, marginBottom: 12 }}>RECENT SEARCHES</div>
                {RECENT.map(r => (
                  <button key={r} onClick={() => setQuery(r)}
                    className="w-full flex items-center gap-3 px-3 py-2.5 rounded-xl mb-1 text-left transition-all"
                    style={{ background: "rgba(255,255,255,0.03)" }}
                    onMouseEnter={e => (e.currentTarget.style.background = "rgba(255,255,255,0.06)")}
                    onMouseLeave={e => (e.currentTarget.style.background = "rgba(255,255,255,0.03)")}
                  >
                    <Clock size={13} color="#4B5563" />
                    <span style={{ color: "#9CA3AF", fontSize: 13 }}>{r}</span>
                  </button>
                ))}
              </div>
              {/* Suggested */}
              <div>
                <div style={{ color: "#6B7280", fontSize: 11, fontWeight: 600, letterSpacing: 1, marginBottom: 12 }}>SUGGESTED QUERIES</div>
                {SUGGESTED.map(s => (
                  <button key={s} onClick={() => setQuery(s)}
                    className="w-full flex items-center gap-3 px-3 py-2.5 rounded-xl mb-1 text-left transition-all"
                    style={{ background: "rgba(255,255,255,0.03)" }}
                    onMouseEnter={e => (e.currentTarget.style.background = "rgba(255,255,255,0.06)")}
                    onMouseLeave={e => (e.currentTarget.style.background = "rgba(255,255,255,0.03)")}
                  >
                    <Hash size={13} color="#4F8CFF" />
                    <span style={{ color: "#9CA3AF", fontSize: 13 }}>{s}</span>
                    <ArrowRight size={12} color="#4B5563" style={{ marginLeft: "auto" }} />
                  </button>
                ))}
              </div>
            </div>
          )}

          {query.length > 0 && filtered.length === 0 && (
            <div className="text-center py-16">
              <Search size={40} color="#374151" style={{ margin: "0 auto 12px" }} />
              <p style={{ color: "#6B7280", fontSize: 14 }}>No results for "<span style={{ color: "#E5E7EB" }}>{query}</span>"</p>
              <p style={{ color: "#4B5563", fontSize: 12, marginTop: 6 }}>Try CVE IDs, asset IPs, product names, or finding IDs</p>
            </div>
          )}

          {query.length > 0 && filtered.length > 0 && (
            <div>
              {/* Category pills */}
              <div className="flex gap-2 mb-5 overflow-x-auto pb-2 scrollbar-hide">
                {categories.map(cat => (
                  <button key={cat} onClick={() => setActiveCategory(cat)}
                    className="px-3 py-1.5 rounded-lg capitalize whitespace-nowrap"
                    style={{ background: activeCategory === cat ? "rgba(79,140,255,0.15)" : "rgba(255,255,255,0.05)", color: activeCategory === cat ? "#4F8CFF" : "#6B7280", fontSize: 12, border: "none", cursor: "pointer" }}
                  >
                    {cat === "all" ? `All (${filtered.length})` : `${TYPE_LABELS[cat]} (${filtered.filter(r => r.type === cat).length})`}
                  </button>
                ))}
              </div>

              {Object.entries(grouped).map(([type, items]) => (
                <div key={type} className="mb-6">
                  <div style={{ color: "#6B7280", fontSize: 11, fontWeight: 600, letterSpacing: 1, marginBottom: 8 }}>{TYPE_LABELS[type]?.toUpperCase()}</div>
                  {items.map((item, i) => {
                    const Icon = item.icon;
                    return (
                      <div key={i}
                        onClick={() => handleResultClick(item.path)}
                        className="flex items-center gap-4 px-4 py-3.5 rounded-xl mb-1.5 cursor-pointer transition-all"
                        style={{ background: "rgba(255,255,255,0.03)", border: "1px solid rgba(255,255,255,0.05)" }}
                        onMouseEnter={e => (e.currentTarget.style.background = "rgba(255,255,255,0.06)")}
                        onMouseLeave={e => (e.currentTarget.style.background = "rgba(255,255,255,0.03)")}
                      >
                        <div className="w-9 h-9 rounded-xl flex items-center justify-center flex-shrink-0" style={{ background: item.color + "15" }}>
                          <Icon size={17} color={item.color} />
                        </div>
                        <div className="flex-1 min-w-0">
                          <div style={{ color: "#E5E7EB", fontSize: 13, fontWeight: 500 }}>{item.name}</div>
                          <div style={{ color: "#6B7280", fontSize: 12, marginTop: 2 }} className="truncate">{item.desc}</div>
                        </div>
                        <div className="flex items-center gap-3 flex-shrink-0">
                          <span className="px-2 py-0.5 rounded" style={{ background: item.color + "15", color: item.color, fontSize: 10, fontWeight: 600 }}>{item.badge}</span>
                          <span style={{ color: "#4B5563", fontSize: 11 }}>{item.updated}</span>
                          <ArrowRight size={13} color="#4B5563" />
                        </div>
                      </div>
                    );
                  })}
                </div>
              ))}
            </div>
          )}
        </div>
        
        {/* Footer */}
        <div className="flex items-center gap-4 px-6 py-3" style={{ borderTop: "1px solid rgba(255,255,255,0.06)", background: "#0B1020" }}>
          {SHORTCUTS.map(s => (
            <div key={s.key} className="flex items-center gap-1.5">
              <kbd style={{ background: "rgba(255,255,255,0.07)", border: "1px solid rgba(255,255,255,0.1)", borderRadius: 4, padding: "2px 6px", color: "#9CA3AF", fontSize: 11 }}>{s.key}</kbd>
              <span style={{ color: "#4B5563", fontSize: 11 }}>{s.desc}</span>
            </div>
          ))}
        </div>
      </div>
    </div>
  );
}
