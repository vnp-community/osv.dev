import { useState } from "react";
import { ChevronRight, ChevronDown, Package } from "lucide-react";
import { AreaChart, Area, XAxis, YAxis, Tooltip, ResponsiveContainer, PieChart, Pie, Cell } from "recharts";
import { useQuery } from "@tanstack/react-query";
import { apiClient } from "@/shared/api/client";
import { ENDPOINTS } from "@/shared/api/endpoints";
import { productKeys } from "@/shared/api/queryClient";
import { QueryBoundary } from "@/shared/components/feedback/QueryBoundary";

// ── Types ────────────────────────────────────────────────────────────────────

interface Product {
  id: string; name: string; grade: string; score: number;
  criticalCount: number; highCount: number; mediumCount: number; lowCount: number;
  slaCompliance: string; engagementCount: number;
}
interface ProductType { id: string; name: string; type: string; products: Product[] }
interface ProductsResponse {
  productTypes: ProductType[];
  riskTrend: Array<{ month: string; critical: number; high: number }>;
}
interface Engagement {
  id: string; name: string; status: string; testCount: number; findingCount: number;
  startDate: string; endDate: string;
}

// ── Hooks ────────────────────────────────────────────────────────────────────

function useProducts() {
  return useQuery<ProductsResponse>({
    queryKey: productKeys.list(),
    queryFn: async () => {
      const { data } = await apiClient.get<ProductsResponse>(ENDPOINTS.products.list);
      return data;
    },
    staleTime: 5 * 60_000,
  });
}

function useEngagements(productId: string | null) {
  return useQuery<{ engagements: Engagement[] }>({
    queryKey: productId ? productKeys.detail(productId) : productKeys.list(),
    queryFn: async () => {
      const { data } = await apiClient.get(ENDPOINTS.products.engagements(productId!));
      return data;
    },
    enabled: !!productId,
    staleTime: 5 * 60_000,
  });
}

// ── UI helpers ───────────────────────────────────────────────────────────────

const GRADE_COLOR = (g: string) => {
  if (g.startsWith("A")) return "#10B981";
  if (g.startsWith("B")) return "#4F8CFF";
  if (g.startsWith("C")) return "#F59E0B";
  return "#EF4444";
};

const SEVERITY_CHART_COLORS = { Critical: "#EF4444", High: "#F97316", Medium: "#EAB308", Low: "#3B82F6" };

function ProductSkeleton() {
  return (
    <div className="flex flex-1 overflow-hidden animate-pulse" style={{ background: "#0B1020" }}>
      <div className="w-60 flex-shrink-0" style={{ background: "#0F1629" }} />
      <div className="flex-1 p-6">
        <div className="h-8 rounded mb-4" style={{ background: "#151B2F" }} />
        <div className="grid grid-cols-4 gap-4 mb-6">
          {Array.from({ length: 4 }).map((_, i) => <div key={i} className="rounded-2xl h-20" style={{ background: "#151B2F" }} />)}
        </div>
      </div>
    </div>
  );
}

// ── Main component ───────────────────────────────────────────────────────────

export function ProductSecurity() {
  const productsQuery = useProducts();
  const [expandedTypes, setExpandedTypes] = useState<string[]>(["pt-1"]);
  const [selectedProductId, setSelectedProductId] = useState<string>("p-1");
  const [activeTab, setActiveTab] = useState("Engagements");

  const engagementsQuery = useEngagements(selectedProductId);

  const toggleType = (id: string) => {
    setExpandedTypes((prev) => prev.includes(id) ? prev.filter((x) => x !== id) : [...prev, id]);
  };

  return (
    <QueryBoundary query={productsQuery} skeleton={<ProductSkeleton />}>
      {({ productTypes, riskTrend }) => {
        const allProducts = productTypes.flatMap(pt => pt.products);
        const selectedProduct = allProducts.find(p => p.id === selectedProductId) ?? allProducts[0];

        const severityChartItems = selectedProduct ? [
          { name: "Critical", value: selectedProduct.criticalCount, color: SEVERITY_CHART_COLORS.Critical },
          { name: "High", value: selectedProduct.highCount, color: SEVERITY_CHART_COLORS.High },
          { name: "Medium", value: selectedProduct.mediumCount, color: SEVERITY_CHART_COLORS.Medium },
          { name: "Low", value: selectedProduct.lowCount, color: SEVERITY_CHART_COLORS.Low },
        ] : [];

        return (
          <div className="flex flex-1 overflow-hidden" style={{ background: "#0B1020" }}>
            {/* Left tree */}
            <div className="w-60 flex-shrink-0 overflow-y-auto py-4 px-3" style={{ background: "#0F1629", borderRight: "1px solid rgba(255,255,255,0.06)" }}>
              <div style={{ color: "#6B7280", fontSize: 10, fontWeight: 600, letterSpacing: 1, marginBottom: 12, paddingLeft: 8 }}>PRODUCT HIERARCHY</div>
              {productTypes.map((pt) => (
                <div key={pt.id} className="mb-1">
                  <button
                    onClick={() => toggleType(pt.id)}
                    className="w-full flex items-center gap-2 px-3 py-2 rounded-lg text-left"
                    style={{ color: "#9CA3AF", fontSize: 13 }}
                  >
                    {expandedTypes.includes(pt.id) ? <ChevronDown size={12} /> : <ChevronRight size={12} />}
                    <Package size={13} />
                    <span>{pt.name}</span>
                    <span style={{ color: "#4B5563", fontSize: 10, marginLeft: "auto" }}>{pt.type}</span>
                  </button>
                  {expandedTypes.includes(pt.id) && (
                    <div className="ml-4 mt-0.5 pl-3" style={{ borderLeft: "1px solid rgba(255,255,255,0.06)" }}>
                      {pt.products.map((p) => (
                        <button
                          key={p.id}
                          onClick={() => setSelectedProductId(p.id)}
                          className="w-full flex items-center gap-2 px-3 py-2 rounded-lg text-left mb-0.5"
                          style={{
                            background: selectedProductId === p.id ? "rgba(79,140,255,0.12)" : "transparent",
                            color: selectedProductId === p.id ? "#4F8CFF" : "#9CA3AF",
                            fontSize: 13,
                          }}
                        >
                          <span style={{ color: GRADE_COLOR(p.grade), fontSize: 11, fontWeight: 700, width: 22 }}>{p.grade}</span>
                          <span style={{ flex: 1 }}>{p.name}</span>
                          {p.criticalCount > 0 && <span style={{ color: "#EF4444", fontSize: 10 }}>{p.criticalCount}</span>}
                        </button>
                      ))}
                    </div>
                  )}
                </div>
              ))}
            </div>

            {/* Main content */}
            {selectedProduct && (
              <div className="flex-1 overflow-y-auto p-6">
                {/* Product header */}
                <div className="flex items-center justify-between mb-6">
                  <div>
                    <h2 style={{ color: "#E5E7EB", fontSize: 20, fontWeight: 700 }}>{selectedProduct.name}</h2>
                    <p style={{ color: "#6B7280", fontSize: 12, marginTop: 2 }}>{selectedProduct.engagementCount} engagements · Score {selectedProduct.score}/100</p>
                  </div>
                  <div
                    className="w-16 h-16 rounded-2xl flex items-center justify-center"
                    style={{ background: `${GRADE_COLOR(selectedProduct.grade)}15`, border: `2px solid ${GRADE_COLOR(selectedProduct.grade)}40` }}
                  >
                    <span style={{ color: GRADE_COLOR(selectedProduct.grade), fontSize: 24, fontWeight: 800 }}>{selectedProduct.grade}</span>
                  </div>
                </div>

                {/* Stats */}
                <div className="grid grid-cols-4 gap-4 mb-6">
                  {[
                    { label: "Critical", value: selectedProduct.criticalCount, color: "#EF4444" },
                    { label: "High", value: selectedProduct.highCount, color: "#F97316" },
                    { label: "Medium", value: selectedProduct.mediumCount, color: "#EAB308" },
                    { label: "SLA Compliance", value: selectedProduct.slaCompliance, color: "#10B981" },
                  ].map((s) => (
                    <div key={s.label} className="rounded-2xl p-4" style={{ background: "#151B2F", border: "1px solid rgba(255,255,255,0.07)" }}>
                      <div style={{ color: "#6B7280", fontSize: 11, marginBottom: 6 }}>{s.label}</div>
                      <div style={{ color: s.color, fontSize: 24, fontWeight: 700 }}>{s.value}</div>
                    </div>
                  ))}
                </div>

                {/* Charts */}
                <div className="grid grid-cols-2 gap-4 mb-6">
                  <div className="rounded-2xl p-4" style={{ background: "#151B2F", border: "1px solid rgba(255,255,255,0.07)" }}>
                    <div style={{ color: "#E5E7EB", fontSize: 13, fontWeight: 600, marginBottom: 12 }}>Findings Trend</div>
                    <ResponsiveContainer width="100%" height={140}>
                      <AreaChart data={riskTrend}>
                        <defs>
                          <linearGradient id="cGrad" x1="0" y1="0" x2="0" y2="1">
                            <stop offset="0%" stopColor="#EF4444" stopOpacity={0.3} />
                            <stop offset="100%" stopColor="#EF4444" stopOpacity={0} />
                          </linearGradient>
                        </defs>
                        <XAxis dataKey="month" tick={{ fill: "#6B7280", fontSize: 10 }} axisLine={false} tickLine={false} />
                        <YAxis tick={{ fill: "#6B7280", fontSize: 10 }} axisLine={false} tickLine={false} />
                        <Tooltip contentStyle={{ background: "#1E2A45", border: "1px solid rgba(255,255,255,0.1)", borderRadius: 8, fontSize: 11 }} />
                        <Area type="monotone" dataKey="critical" stroke="#EF4444" strokeWidth={2} fill="url(#cGrad)" name="Critical" />
                        <Area type="monotone" dataKey="high" stroke="#F97316" strokeWidth={2} fill="none" name="High" />
                      </AreaChart>
                    </ResponsiveContainer>
                  </div>
                  <div className="rounded-2xl p-4 flex flex-col" style={{ background: "#151B2F", border: "1px solid rgba(255,255,255,0.07)" }}>
                    <div style={{ color: "#E5E7EB", fontSize: 13, fontWeight: 600, marginBottom: 8 }}>Severity Distribution</div>
                    <div className="flex items-center gap-4">
                      <ResponsiveContainer width={120} height={120}>
                        <PieChart>
                          <Pie data={severityChartItems} cx="50%" cy="50%" innerRadius={35} outerRadius={55} paddingAngle={3} dataKey="value">
                            {severityChartItems.map((entry, index) => <Cell key={index} fill={entry.color} />)}
                          </Pie>
                        </PieChart>
                      </ResponsiveContainer>
                      <div className="flex flex-col gap-2">
                        {severityChartItems.map((d) => (
                          <div key={d.name} className="flex items-center gap-2">
                            <div className="w-2 h-2 rounded-sm" style={{ background: d.color }} />
                            <span style={{ color: "#9CA3AF", fontSize: 11 }}>{d.name}:</span>
                            <span style={{ color: d.color, fontSize: 11, fontWeight: 600 }}>{d.value}</span>
                          </div>
                        ))}
                      </div>
                    </div>
                  </div>
                </div>

                {/* Tabs */}
                <div className="rounded-2xl overflow-hidden" style={{ background: "#151B2F", border: "1px solid rgba(255,255,255,0.07)" }}>
                  <div className="flex gap-1 px-4 py-3" style={{ borderBottom: "1px solid rgba(255,255,255,0.06)" }}>
                    {["Engagements", "Findings", "Risk Acceptance"].map((tab) => (
                      <button
                        key={tab}
                        onClick={() => setActiveTab(tab)}
                        className="px-4 py-1.5 rounded-lg"
                        style={{ background: activeTab === tab ? "rgba(79,140,255,0.12)" : "transparent", color: activeTab === tab ? "#4F8CFF" : "#6B7280", fontSize: 13, border: "none", cursor: "pointer" }}
                      >
                        {tab}
                      </button>
                    ))}
                  </div>
                  <div className="p-4">
                    {activeTab === "Engagements" && (
                      <div className="flex flex-col gap-3">
                        {(engagementsQuery.data?.engagements ?? []).map((e) => (
                          <div key={e.id} className="flex items-center gap-4 rounded-xl p-4" style={{ background: "rgba(255,255,255,0.03)", border: "1px solid rgba(255,255,255,0.06)" }}>
                            <div className="flex-1">
                              <div style={{ color: "#E5E7EB", fontSize: 13, fontWeight: 500 }}>{e.name}</div>
                              <div style={{ color: "#6B7280", fontSize: 11, marginTop: 2 }}>{e.startDate} — {e.endDate} · {e.testCount} tests</div>
                            </div>
                            <span className="px-2 py-0.5 rounded" style={{ background: e.status === "Active" ? "rgba(79,140,255,0.12)" : "rgba(16,185,129,0.1)", color: e.status === "Active" ? "#4F8CFF" : "#10B981", fontSize: 11 }}>{e.status}</span>
                            <span style={{ color: "#F59E0B", fontSize: 12, fontWeight: 600 }}>{e.findingCount} findings</span>
                          </div>
                        ))}
                        {engagementsQuery.isLoading && <div style={{ color: "#6B7280", fontSize: 12 }}>Loading engagements...</div>}
                      </div>
                    )}
                    {activeTab === "Findings" && (
                      <div style={{ color: "#6B7280", fontSize: 13, textAlign: "center", padding: "20px 0" }}>
                        Showing {selectedProduct.criticalCount + selectedProduct.highCount + selectedProduct.mediumCount} open findings for {selectedProduct.name}
                      </div>
                    )}
                    {activeTab === "Risk Acceptance" && (
                      <div style={{ color: "#6B7280", fontSize: 13, textAlign: "center", padding: "20px 0" }}>
                        No risk acceptances pending review
                      </div>
                    )}
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
