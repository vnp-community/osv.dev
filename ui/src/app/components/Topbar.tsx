import { useState } from "react";
import { useNavigate } from "react-router";
import { Search, Bell, ChevronDown, Plus, Shield, AlertCircle } from "lucide-react";
import { useQuery } from "@tanstack/react-query";
import { useNotificationStore } from "@/features/notifications/store/notificationStore";
import { useAuthStore } from "@/features/auth/store/authStore";
import { kevApi } from "@/features/cve-intel/api/kevApi";
import { useSearchStore } from "@/shared/stores/useSearchStore";

interface TopbarProps {
  breadcrumbs?: string[];
  actions?: React.ReactNode;
}

export function Topbar({ breadcrumbs = [], actions }: TopbarProps) {
  const [searchFocused, setSearchFocused] = useState(false);
  const navigate = useNavigate();
  const unreadCount = useNotificationStore((state) => state.unreadCount);
  const user = useAuthStore((state) => state.user);

  const getInitials = (name?: string, email?: string) => {
    if (name) {
      return name.split(" ").map(n => n[0]).join("").toUpperCase().substring(0, 2);
    }
    if (email) {
      return email.substring(0, 2).toUpperCase();
    }
    return "U";
  };
  
  const displayName = user?.name || user?.email?.split('@')[0] || "User";
  const initials = getInitials(user?.name, user?.email);

  const kevStatsQuery = useQuery({
    queryKey: ['cve', 'kev-stats'],
    queryFn: () => kevApi.getStats(),
    staleTime: 30 * 60_000,
  });

  const kevCount = kevStatsQuery.data?.stats?.unmitigated_in_platform ?? kevStatsQuery.data?.stats?.added_last_30_days ?? 0;

  return (
    <div
      className="flex items-center justify-between px-6 py-3 flex-shrink-0"
      style={{
        background: "#0F1629",
        borderBottom: "1px solid rgba(255,255,255,0.06)",
        height: 56,
      }}
    >
      {/* Breadcrumbs */}
      <div className="flex items-center gap-2">
        {breadcrumbs.map((crumb, i) => (
          <div key={crumb} className="flex items-center gap-2">
            {i > 0 && <span style={{ color: "#374151" }}>/</span>}
            <span
              style={{
                color: i === breadcrumbs.length - 1 ? "#E5E7EB" : "#6B7280",
                fontSize: 13,
                fontWeight: i === breadcrumbs.length - 1 ? 500 : 400,
              }}
            >
              {crumb}
            </span>
          </div>
        ))}
      </div>

      {/* Center search */}
      <div className="relative flex-1 max-w-md mx-8" onClick={() => useSearchStore.getState().openSearch()}>
        <Search
          size={15}
          color="#4B5563"
          style={{ position: "absolute", left: 12, top: "50%", transform: "translateY(-50%)" }}
        />
        <input
          readOnly
          placeholder="Search CVEs, findings, assets..."
          className="w-full rounded-xl pl-9 pr-4 py-2 outline-none transition-all cursor-pointer"
          style={{
            background: "rgba(255,255,255,0.05)",
            border: `1px solid ${searchFocused ? "rgba(79,140,255,0.5)" : "rgba(255,255,255,0.07)"}`,
            color: "#E5E7EB",
            fontSize: 13,
          }}
          onFocus={() => setSearchFocused(true)}
          onBlur={() => setSearchFocused(false)}
        />
        <kbd
          style={{
            position: "absolute",
            right: 10,
            top: "50%",
            transform: "translateY(-50%)",
            background: "rgba(255,255,255,0.07)",
            border: "1px solid rgba(255,255,255,0.1)",
            borderRadius: 4,
            padding: "1px 5px",
            color: "#6B7280",
            fontSize: 10,
          }}
        >
          ⌘K
        </kbd>
      </div>

      {/* Right actions */}
      <div className="flex items-center gap-3">
        {actions}

        {/* KEV Alert */}
        <button
          onClick={() => navigate("/cve/kev")}
          className="flex items-center gap-1.5 px-3 py-1.5 rounded-lg transition-all hover:bg-amber-500/10"
          style={{
            background: "rgba(245,158,11,0.1)",
            border: "1px solid rgba(245,158,11,0.2)",
            color: "#F59E0B",
            fontSize: 12,
            cursor: "pointer",
          }}
        >
          <AlertCircle size={12} />
          <span>{kevCount} KEV</span>
        </button>

        {/* Notifications */}
        <button
          onClick={() => navigate("/notifications")}
          className="relative w-9 h-9 rounded-xl flex items-center justify-center transition-all"
          style={{
            background: "rgba(255,255,255,0.05)",
            border: "1px solid rgba(255,255,255,0.07)",
            color: "#9CA3AF",
            cursor: "pointer",
          }}
        >
          <Bell size={16} />
          {unreadCount > 0 && (
            <span
              className="absolute -top-1 -right-1 min-w-[16px] h-4 px-1 rounded-full flex items-center justify-center text-white"
              style={{ background: "#EF4444", fontSize: 9, fontWeight: 700 }}
            >
              {unreadCount > 99 ? "99+" : unreadCount}
            </span>
          )}
        </button>

        {/* User menu */}
        <button
          onClick={() => navigate("/profile")}
          className="flex items-center gap-2.5 px-3 py-1.5 rounded-xl transition-all hover:bg-white/5"
          style={{
            background: "rgba(255,255,255,0.05)",
            border: "1px solid rgba(255,255,255,0.07)",
            cursor: "pointer",
          }}
        >
          <div
            className="w-6 h-6 rounded-full flex items-center justify-center text-white"
            style={{ background: "linear-gradient(135deg, #4F8CFF, #7C3AED)", fontSize: 10, fontWeight: 700 }}
          >
            {initials}
          </div>
          <span className="truncate max-w-[100px]" style={{ color: "#E5E7EB", fontSize: 13 }}>{displayName}</span>
          <ChevronDown size={12} color="#6B7280" />
        </button>
      </div>
    </div>
  );
}
