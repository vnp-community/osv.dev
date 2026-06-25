import { useState } from "react";
import { Search, BookOpen } from "lucide-react";
import { useCWELibrary, type CWEEntry } from "@/features/cve-intel/hooks/useTaxonomy";
import { QueryBoundary } from "@/shared/components/feedback/QueryBoundary";

const IMPACT_COLORS: Record<string, string> = { Critical: "#EF4444", High: "#F97316", Medium: "#EAB308", Low: "#3B82F6" };

function CWESkeleton() {
  return (
    <div className="flex flex-1 overflow-hidden animate-pulse" style={{ background: "#0B1020" }}>
      <div className="flex-1 p-4">
        <div className="h-8 rounded mb-4" style={{ background: "#151B2F" }} />
        {Array.from({ length: 8 }).map((_, i) => (
          <div key={i} className="h-10 rounded mb-2" style={{ background: "#151B2F" }} />
        ))}
      </div>
    </div>
  );
}

export function CWELibrary() {
  const [search, setSearch] = useState("");
  const [filterCategory, setFilterCategory] = useState("All");
  const [selected, setSelected] = useState<CWEEntry | null>(null);

  const cweQuery = useCWELibrary();

  return (
    <QueryBoundary query={cweQuery} skeleton={<CWESkeleton />}>
      {({ cweList }) => {
        const categories = ["All", ...Array.from(new Set(cweList.map(c => c.category)))];

        const filtered = cweList.filter(c =>
          (filterCategory === "All" || c.category === filterCategory) &&
          (!search || c.id.toLowerCase().includes(search.toLowerCase()) || c.name.toLowerCase().includes(search.toLowerCase()))
        );

        const selectedItem = selected ?? (filtered[0] ?? null);

        return (
          <div className="flex flex-1 overflow-hidden" style={{ background: "#0B1020" }}>
            {/* Table */}
            <div className="flex flex-col flex-1 overflow-hidden">
              <div className="px-5 py-4" style={{ borderBottom: "1px solid rgba(255,255,255,0.06)" }}>
                <div className="flex items-center justify-between mb-3">
                  <h2 style={{ color: "#E5E7EB", fontSize: 18, fontWeight: 700 }}>CWE Library</h2>
                  <span style={{ color: "#6B7280", fontSize: 12 }}>{filtered.length} weakness types</span>
                </div>
                <div className="flex gap-3">
                  <div className="relative">
                    <Search size={13} color="#4B5563" style={{ position: "absolute", left: 10, top: "50%", transform: "translateY(-50%)" }} />
                    <input value={search} onChange={e => setSearch(e.target.value)} placeholder="Search CWE..." className="rounded-xl pl-8 pr-4 py-2 outline-none" style={{ background: "#151B2F", border: "1px solid rgba(255,255,255,0.08)", color: "#E5E7EB", fontSize: 12, width: 200 }} />
                  </div>
                  <select value={filterCategory} onChange={e => setFilterCategory(e.target.value)} className="rounded-xl px-3 py-2 outline-none" style={{ background: "#151B2F", border: "1px solid rgba(255,255,255,0.08)", color: "#9CA3AF", fontSize: 12 }}>
                    {categories.map(c => <option key={c}>{c}</option>)}
                  </select>
                </div>
              </div>
              <div className="flex-1 overflow-y-auto">
                <table className="w-full">
                  <thead style={{ position: "sticky", top: 0, background: "#0D1525", zIndex: 5 }}>
                    <tr style={{ borderBottom: "1px solid rgba(255,255,255,0.06)" }}>
                      {["CWE ID", "Name", "Category", "Impact", "Linked CVEs", "CAPEC"].map(h => (
                        <th key={h} className="px-4 py-3 text-left" style={{ color: "#6B7280", fontSize: 11, fontWeight: 600, letterSpacing: 0.5 }}>{h}</th>
                      ))}
                    </tr>
                  </thead>
                  <tbody>
                    {filtered.map(c => (
                      <tr key={c.id} onClick={() => setSelected(c)} className="cursor-pointer transition-all"
                        style={{ borderBottom: "1px solid rgba(255,255,255,0.04)", background: selectedItem?.id === c.id ? "rgba(79,140,255,0.07)" : "transparent", borderLeft: selectedItem?.id === c.id ? "2px solid #4F8CFF" : "2px solid transparent" }}
                        onMouseEnter={e => { if (selectedItem?.id !== c.id) e.currentTarget.style.background = "rgba(255,255,255,0.02)"; }}
                        onMouseLeave={e => { if (selectedItem?.id !== c.id) e.currentTarget.style.background = "transparent"; }}
                      >
                        <td className="px-4 py-3"><span style={{ color: "#4F8CFF", fontSize: 12, fontFamily: "monospace", fontWeight: 600 }}>{c.id}</span></td>
                        <td className="px-4 py-3"><span style={{ color: "#E5E7EB", fontSize: 12 }} className="line-clamp-1">{c.name}</span></td>
                        <td className="px-4 py-3"><span style={{ color: "#9CA3AF", fontSize: 11 }}>{c.category}</span></td>
                        <td className="px-4 py-3"><span className="px-2 py-0.5 rounded" style={{ background: IMPACT_COLORS[c.impact] + "20", color: IMPACT_COLORS[c.impact], fontSize: 11 }}>{c.impact}</span></td>
                        <td className="px-4 py-3"><span style={{ color: "#E5E7EB", fontSize: 12, fontWeight: 600 }}>{c.linkedCVEs.toLocaleString()}</span></td>
                        <td className="px-4 py-3"><span style={{ color: "#A78BFA", fontSize: 12 }}>{c.capecCount}</span></td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            </div>

            {/* Detail drawer */}
            {selectedItem && (
              <div className="w-80 flex-shrink-0 overflow-y-auto" style={{ background: "#0F1629", borderLeft: "1px solid rgba(255,255,255,0.06)" }}>
                <div className="p-5" style={{ borderBottom: "1px solid rgba(255,255,255,0.06)" }}>
                  <div style={{ color: "#4F8CFF", fontSize: 14, fontWeight: 700, fontFamily: "monospace", marginBottom: 4 }}>{selectedItem.id}</div>
                  <div style={{ color: "#E5E7EB", fontSize: 13, lineHeight: 1.5 }}>{selectedItem.name}</div>
                  <div className="flex gap-2 mt-3">
                    <span className="px-2 py-0.5 rounded" style={{ background: IMPACT_COLORS[selectedItem.impact] + "20", color: IMPACT_COLORS[selectedItem.impact], fontSize: 11 }}>{selectedItem.impact}</span>
                    <span style={{ color: "#9CA3AF", fontSize: 11 }}>{selectedItem.category}</span>
                  </div>
                </div>
                <div className="p-4" style={{ borderBottom: "1px solid rgba(255,255,255,0.06)" }}>
                  <div style={{ color: "#9CA3AF", fontSize: 11, fontWeight: 600, marginBottom: 8 }}>DESCRIPTION</div>
                  <p style={{ color: "#E5E7EB", fontSize: 12, lineHeight: 1.7 }}>{selectedItem.description}</p>
                </div>
                <div className="p-4" style={{ borderBottom: "1px solid rgba(255,255,255,0.06)" }}>
                  <div style={{ color: "#9CA3AF", fontSize: 11, fontWeight: 600, marginBottom: 8 }}>MITIGATION</div>
                  <div className="rounded-xl p-3" style={{ background: "rgba(16,185,129,0.07)", border: "1px solid rgba(16,185,129,0.2)" }}>
                    <p style={{ color: "#A7F3D0", fontSize: 12, lineHeight: 1.6 }}>{selectedItem.mitigation}</p>
                  </div>
                </div>
                <div className="p-4">
                  <div className="grid grid-cols-2 gap-2">
                    <div className="rounded-xl p-3" style={{ background: "rgba(255,255,255,0.04)" }}>
                      <div style={{ color: "#6B7280", fontSize: 10 }}>Linked CVEs</div>
                      <div style={{ color: "#E5E7EB", fontSize: 18, fontWeight: 700 }}>{selectedItem.linkedCVEs.toLocaleString()}</div>
                    </div>
                    <div className="rounded-xl p-3" style={{ background: "rgba(167,139,250,0.07)" }}>
                      <div style={{ color: "#6B7280", fontSize: 10 }}>CAPEC Patterns</div>
                      <div style={{ color: "#A78BFA", fontSize: 18, fontWeight: 700 }}>{selectedItem.capecCount}</div>
                    </div>
                  </div>
                </div>
              </div>
            )}
          </div>
        );
      }}
    </QueryBoundary>
  );
}
