import { useState } from "react";
import { Search, Package } from "lucide-react";
import { useVendorCatalog } from "@/features/cve-intel/hooks/useTaxonomy";
import { QueryBoundary } from "@/shared/components/feedback/QueryBoundary";

function VendorSkeleton() {
  return (
    <div className="flex-1 overflow-y-auto px-6 py-5 animate-pulse" style={{ background: "#0B1020" }}>
      <div className="grid grid-cols-2 gap-4 mb-5">
        {Array.from({ length: 2 }).map((_, i) => (
          <div key={i} className="rounded-xl h-16" style={{ background: "#151B2F" }} />
        ))}
      </div>
      <div className="grid grid-cols-3 gap-4">
        {Array.from({ length: 9 }).map((_, i) => (
          <div key={i} className="rounded-2xl h-20" style={{ background: "#151B2F" }} />
        ))}
      </div>
    </div>
  );
}

export function VendorCatalog({ onSelectVendor }: { onSelectVendor?: () => void }) {
  const [search, setSearch] = useState("");
  const [selected, setSelected] = useState<string | null>(null);

  const vendorsQuery = useVendorCatalog();

  return (
    <QueryBoundary query={vendorsQuery} skeleton={<VendorSkeleton />}>
      {(data) => {
        // vendors is string[] from /api/v2/vendors
        const vendors: string[] = data.vendors ?? [];

        const filtered = search
          ? vendors.filter((name) => name.toLowerCase().includes(search.toLowerCase()))
          : vendors;

        return (
          <div className="flex-1 overflow-y-auto px-6 py-5" style={{ background: "#0B1020" }}>
            <div className="flex items-center justify-between mb-5">
              <div>
                <h2 style={{ color: "#E5E7EB", fontSize: 18, fontWeight: 700 }}>Vendor Vulnerability Catalog</h2>
                <p style={{ color: "#6B7280", fontSize: 12 }}>
                  {data.total.toLocaleString()} vendors tracked · Updated daily from NVD
                </p>
              </div>
            </div>

            {/* Global stats */}
            <div className="grid grid-cols-2 gap-4 mb-5">
              {[
                { label: "Total Vendors", value: data.total.toLocaleString(), color: "#4F8CFF" },
                { label: "Filtered Results", value: filtered.length.toLocaleString(), color: "#F97316" },
              ].map((s) => (
                <div key={s.label} className="rounded-xl px-4 py-3" style={{ background: "#151B2F", border: "1px solid rgba(255,255,255,0.07)" }}>
                  <div style={{ color: s.color, fontSize: 22, fontWeight: 700 }}>{s.value}</div>
                  <div style={{ color: "#6B7280", fontSize: 12 }}>{s.label}</div>
                </div>
              ))}
            </div>

            {/* Search toolbar */}
            <div className="flex items-center gap-3 mb-5">
              <div className="relative">
                <Search size={13} color="#4B5563" style={{ position: "absolute", left: 10, top: "50%", transform: "translateY(-50%)" }} />
                <input
                  value={search}
                  onChange={(e) => setSearch(e.target.value)}
                  placeholder="Search vendors..."
                  className="rounded-xl pl-8 pr-4 py-2 outline-none"
                  style={{ background: "#151B2F", border: "1px solid rgba(255,255,255,0.08)", color: "#E5E7EB", fontSize: 12, width: 220 }}
                />
              </div>
              {search && (
                <span style={{ color: "#6B7280", fontSize: 12 }}>
                  {filtered.length} of {vendors.length} vendors
                </span>
              )}
            </div>

            {/* Vendor cards grid */}
            {filtered.length === 0 ? (
              <div className="flex flex-col items-center justify-center py-16" style={{ color: "#6B7280" }}>
                <Package size={40} style={{ marginBottom: 12, opacity: 0.4 }} />
                <p style={{ fontSize: 14 }}>No vendors match your search</p>
              </div>
            ) : (
              <div className="grid grid-cols-3 gap-3">
                {filtered.map((name) => (
                  <div
                    key={name}
                    onClick={() => { setSelected(name); onSelectVendor?.(); }}
                    className="rounded-2xl p-4 cursor-pointer transition-all"
                    style={{
                      background: "#151B2F",
                      border: selected === name
                        ? "1px solid rgba(79,140,255,0.5)"
                        : "1px solid rgba(255,255,255,0.07)",
                    }}
                    onMouseEnter={(e) => { e.currentTarget.style.borderColor = "rgba(79,140,255,0.3)"; }}
                    onMouseLeave={(e) => { e.currentTarget.style.borderColor = selected === name ? "rgba(79,140,255,0.5)" : "rgba(255,255,255,0.07)"; }}
                  >
                    <div className="flex items-center gap-3">
                      <div
                        className="w-9 h-9 rounded-xl flex items-center justify-center"
                        style={{ background: "rgba(79,140,255,0.12)", color: "#4F8CFF", fontSize: 12, fontWeight: 800, flexShrink: 0 }}
                      >
                        {name.slice(0, 2).toUpperCase()}
                      </div>
                      <div className="min-w-0">
                        <div
                          style={{ color: "#E5E7EB", fontSize: 13, fontWeight: 600 }}
                          className="truncate"
                          title={name}
                        >
                          {name}
                        </div>
                        <div style={{ color: "#6B7280", fontSize: 11 }}>View CVEs →</div>
                      </div>
                    </div>
                  </div>
                ))}
              </div>
            )}
          </div>
        );
      }}
    </QueryBoundary>
  );
}
