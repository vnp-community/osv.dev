import { useState } from "react";
import { Search, AlertTriangle, Server, Package, FileText, User, Shield, Clock, ArrowRight, Hash } from "lucide-react";
import { useQuery } from "@tanstack/react-query";
import { apiClient } from "@/shared/api/client";
import { ENDPOINTS } from "@/shared/api/endpoints";

// ── Icon lookup (server returns type string, not icon component) ──────────────

const TYPE_ICON: Record<string, React.ElementType> = {
  cve:     Shield,
  finding: AlertTriangle,
  asset:   Server,
  product: Package,
  report:  FileText,
  user:    User,
};
const TYPE_COLOR: Record<string, string> = {
  cve:     "#EF4444",
  finding: "#F97316",
  asset:   "#4F8CFF",
  product: "#10B981",
  report:  "#A78BFA",
  user:    "#6B7280",
};
const TYPE_LABELS: Record<string, string> = {
  cve: "CVEs", finding: "Findings", asset: "Assets", product: "Products", report: "Reports", user: "Users",
};

const SHORTCUTS = [
  { key: "⌘K", desc: "Open search" },
  { key: "↑↓", desc: "Navigate" },
  { key: "↵", desc: "Open" },
  { key: "Esc", desc: "Close" },
];

// ── Types ─────────────────────────────────────────────────────────────────────

interface SearchResult {
  type: string;
  id:   string;
  name: string;
  desc: string;
  updated: string;
  badge: string | null;
}

// ── Hooks ─────────────────────────────────────────────────────────────────────

function useRecentSearches() {
  return useQuery<{ items: string[] }>({
    queryKey: ["search", "recent"],
    queryFn: async () => {
      const { data } = await apiClient.get<{ items: string[] }>(ENDPOINTS.search.recent);
      return { items: Array.isArray(data?.items) ? data.items : [] };
    },
    staleTime: 5 * 60_000,
  });
}

function useSuggestedSearches() {
  return useQuery<{ items: string[] }>({
    queryKey: ["search", "suggested"],
    queryFn: async () => {
      const { data } = await apiClient.get<{ items: string[] }>(ENDPOINTS.search.suggested);
      return { items: Array.isArray(data?.items) ? data.items : [] };
    },
    staleTime: 10 * 60_000,
  });
}

function useSearch(q: string) {
  return useQuery<{ items: SearchResult[]; total: number }>({
    queryKey: ["search", "results", q],
    queryFn: async () => {
      const { data } = await apiClient.get<{ items: SearchResult[]; total: number }>(
        "/api/v1/search",
        { params: { q } }
      );
      return {
        items: Array.isArray(data?.items) ? data.items : [],
        total: typeof data?.total === "number" ? data.total : 0,
      };
    },
    enabled: q.length > 1,
    staleTime: 30_000,
    placeholderData: (prev) => prev,
  });
}

// ── Component ─────────────────────────────────────────────────────────────────

export function GlobalSearch({ onNavigate }: { onNavigate: (s: string) => void }) {
  const [query, setQuery] = useState("");
  const [activeCategory, setActiveCategory] = useState("all");

  const { data: recentData } = useRecentSearches();
  const { data: suggestedData } = useSuggestedSearches();
  const { data: searchData } = useSearch(query);

  const recent    = recentData?.items    ?? [];
  const suggested = suggestedData?.items ?? [];
  const results   = searchData?.items    ?? [];

  const categories = ["all", ...Object.keys(TYPE_LABELS)];
  const filtered = activeCategory === "all" ? results : results.filter(r => r.type === activeCategory);

  const grouped: Record<string, SearchResult[]> = {};
  for (const r of filtered) {
    if (!grouped[r.type]) grouped[r.type] = [];
    grouped[r.type].push(r);
  }

  return (
    <div className="flex flex-col flex-1 overflow-hidden" style={{ background: "#0B1020" }}>
      {/* Hero search */}
      <div className="flex flex-col items-center px-6 pt-14 pb-6" style={{ borderBottom: "1px solid rgba(255,255,255,0.06)" }}>
        <div className="flex items-center gap-2 mb-6">
          <div className="w-8 h-8 rounded-xl flex items-center justify-center" style={{ background: "linear-gradient(135deg,#4F8CFF,#7C3AED)" }}>
            <Search size={16} color="white" />
          </div>
          <h1 style={{ color: "#E5E7EB", fontSize: 22, fontWeight: 700 }}>Universal Search</h1>
        </div>
        <div className="relative w-full max-w-2xl">
          <Search size={18} color="#4B5563" style={{ position: "absolute", left: 18, top: "50%", transform: "translateY(-50%)" }} />
          <input
            autoFocus
            value={query}
            onChange={e => setQuery(e.target.value)}
            placeholder="Search CVEs, findings, assets, products, reports..."
            className="w-full rounded-2xl pl-14 pr-20 py-4 outline-none"
            style={{ background: "#151B2F", border: "1px solid rgba(79,140,255,0.3)", color: "#E5E7EB", fontSize: 16, boxShadow: "0 0 30px rgba(79,140,255,0.1)" }}
          />
          <kbd style={{ position: "absolute", right: 16, top: "50%", transform: "translateY(-50%)", background: "rgba(255,255,255,0.07)", border: "1px solid rgba(255,255,255,0.1)", borderRadius: 6, padding: "4px 8px", color: "#6B7280", fontSize: 12 }}>⌘K</kbd>
        </div>
        {/* Shortcuts */}
        <div className="flex items-center gap-4 mt-4">
          {SHORTCUTS.map(s => (
            <div key={s.key} className="flex items-center gap-1.5">
              <kbd style={{ background: "rgba(255,255,255,0.07)", border: "1px solid rgba(255,255,255,0.1)", borderRadius: 4, padding: "2px 6px", color: "#9CA3AF", fontSize: 11 }}>{s.key}</kbd>
              <span style={{ color: "#4B5563", fontSize: 11 }}>{s.desc}</span>
            </div>
          ))}
        </div>
      </div>

      <div className="flex-1 overflow-y-auto px-6 py-5">
        {/* Empty state — show recent + suggested */}
        {!query && (
          <div className="grid grid-cols-2 gap-6 max-w-3xl mx-auto">
            <div>
              <div style={{ color: "#6B7280", fontSize: 11, fontWeight: 600, letterSpacing: 1, marginBottom: 12 }}>RECENT SEARCHES</div>
              {recent.map(r => (
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
            <div>
              <div style={{ color: "#6B7280", fontSize: 11, fontWeight: 600, letterSpacing: 1, marginBottom: 12 }}>SUGGESTED QUERIES</div>
              {suggested.map(s => (
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

        {/* No results */}
        {query.length > 1 && results.length === 0 && (
          <div className="text-center py-16">
            <Search size={40} color="#374151" style={{ margin: "0 auto 12px" }} />
            <p style={{ color: "#6B7280", fontSize: 14 }}>No results for "<span style={{ color: "#E5E7EB" }}>{query}</span>"</p>
            <p style={{ color: "#4B5563", fontSize: 12, marginTop: 6 }}>Try CVE IDs, asset IPs, product names, or finding IDs</p>
          </div>
        )}

        {/* Search results */}
        {query.length > 1 && results.length > 0 && (
          <div className="max-w-3xl mx-auto">
            {/* Category pills */}
            <div className="flex gap-2 mb-5">
              {categories.map(cat => (
                <button key={cat} onClick={() => setActiveCategory(cat)}
                  className="px-3 py-1.5 rounded-lg capitalize"
                  style={{ background: activeCategory === cat ? "rgba(79,140,255,0.15)" : "rgba(255,255,255,0.05)", color: activeCategory === cat ? "#4F8CFF" : "#6B7280", fontSize: 12, border: "none", cursor: "pointer" }}
                >
                  {cat === "all" ? `All (${results.length})` : `${TYPE_LABELS[cat]} (${results.filter(r => r.type === cat).length})`}
                </button>
              ))}
            </div>

            {Object.entries(grouped).map(([type, items]) => {
              const Icon = TYPE_ICON[type] ?? Shield;
              const color = TYPE_COLOR[type] ?? "#6B7280";
              return (
                <div key={type} className="mb-6">
                  <div style={{ color: "#6B7280", fontSize: 11, fontWeight: 600, letterSpacing: 1, marginBottom: 8 }}>{TYPE_LABELS[type]?.toUpperCase()}</div>
                  {items.map(item => (
                    <div key={item.id}
                      className="flex items-center gap-4 px-4 py-3.5 rounded-xl mb-1.5 cursor-pointer transition-all"
                      style={{ background: "rgba(255,255,255,0.03)", border: "1px solid rgba(255,255,255,0.05)" }}
                      onMouseEnter={e => (e.currentTarget.style.background = "rgba(255,255,255,0.06)")}
                      onMouseLeave={e => (e.currentTarget.style.background = "rgba(255,255,255,0.03)")}
                    >
                      <div className="w-9 h-9 rounded-xl flex items-center justify-center flex-shrink-0" style={{ background: color + "15" }}>
                        <Icon size={17} color={color} />
                      </div>
                      <div className="flex-1 min-w-0">
                        <div style={{ color: "#E5E7EB", fontSize: 13, fontWeight: 500 }}>{item.name}</div>
                        <div style={{ color: "#6B7280", fontSize: 12, marginTop: 2 }} className="truncate">{item.desc}</div>
                      </div>
                      <div className="flex items-center gap-3 flex-shrink-0">
                        {item.badge && <span className="px-2 py-0.5 rounded" style={{ background: color + "15", color, fontSize: 10, fontWeight: 600 }}>{item.badge}</span>}
                        <span style={{ color: "#4B5563", fontSize: 11 }}>{item.updated}</span>
                        <ArrowRight size={13} color="#4B5563" />
                      </div>
                    </div>
                  ))}
                </div>
              );
            })}
          </div>
        )}
      </div>
    </div>
  );
}
