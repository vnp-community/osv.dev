import { useState } from "react";
import { AlertTriangle, Clock, Shield, Activity, Bell, CheckCircle, Filter } from "lucide-react";

const notifications = [
  { id: 1, type: "critical", icon: AlertTriangle, color: "#EF4444", bg: "rgba(239,68,68,0.1)", title: "Critical Finding Detected", desc: "CVE-2025-44228 found on webserver01.prod — CVSS 10.0, KEV active", time: "10 min ago", read: false, product: "Banking App" },
  { id: 2, type: "sla", icon: Clock, color: "#F59E0B", bg: "rgba(245,158,11,0.1)", title: "SLA Breach Imminent", desc: "F-2842 (Cisco IOS XE) SLA expires in 24h — escalation required", time: "25 min ago", read: false, product: "Network Infra" },
  { id: 3, type: "kev", icon: Shield, color: "#EF4444", bg: "rgba(239,68,68,0.08)", title: "New KEV Added", desc: "CISA added CVE-2025-77001 (Microsoft Exchange) to KEV catalog", time: "1h ago", read: false, product: "Global" },
  { id: 4, type: "scan", icon: Activity, color: "#4F8CFF", bg: "rgba(79,140,255,0.1)", title: "Scan Completed", desc: "Production Network Sweep (SC-0047) completed — 47 findings discovered", time: "2h ago", read: true, product: "Production" },
  { id: 5, type: "critical", icon: AlertTriangle, color: "#EF4444", bg: "rgba(239,68,68,0.1)", title: "SLA Overdue", desc: "F-2846 (Spring Framework RCE) is 2 days overdue — immediate action required", time: "3h ago", read: true, product: "API Gateway" },
  { id: 6, type: "kev", icon: Shield, color: "#A78BFA", bg: "rgba(167,139,250,0.1)", title: "KEV Ransomware Update", desc: "CVE-2025-44228 confirmed used in LockBit 3.0 ransomware campaign", time: "5h ago", read: true, product: "Global" },
  { id: 7, type: "scan", icon: Activity, color: "#10B981", bg: "rgba(16,185,129,0.1)", title: "Scheduled Scan Started", desc: "Daily Production Network Scan started on schedule", time: "6h ago", read: true, product: "Production" },
  { id: 8, type: "sla", icon: Clock, color: "#F59E0B", bg: "rgba(245,158,11,0.1)", title: "SLA Warning", desc: "3 findings will breach SLA in the next 48h — review required", time: "8h ago", read: true, product: "Multiple" },
  { id: 9, type: "critical", icon: AlertTriangle, color: "#F97316", bg: "rgba(249,115,22,0.1)", title: "High Severity Cluster", desc: "5 new High findings in API Gateway — potential attack vector chain", time: "12h ago", read: true, product: "API Gateway" },
  { id: 10, type: "scan", icon: Activity, color: "#EF4444", bg: "rgba(239,68,68,0.08)", title: "Scan Failed", desc: "Database server scan (SC-0042) failed — connection timeout", time: "1d ago", read: true, product: "Data Pipeline" },
];

const CATEGORIES = [
  { id: "all", label: "All" },
  { id: "critical", label: "Critical" },
  { id: "sla", label: "SLA Alerts" },
  { id: "kev", label: "KEV Updates" },
  { id: "scan", label: "Scan Events" },
];

export function NotificationCenter() {
  const [filter, setFilter] = useState("all");
  const [showUnreadOnly, setShowUnreadOnly] = useState(false);

  const filtered = notifications.filter((n) => {
    if (filter !== "all" && n.type !== filter) return false;
    if (showUnreadOnly && n.read) return false;
    return true;
  });

  const unreadCount = notifications.filter((n) => !n.read).length;

  return (
    <div className="flex flex-col flex-1 overflow-hidden" style={{ background: "#0B1020" }}>
      {/* Header */}
      <div className="px-6 py-4" style={{ borderBottom: "1px solid rgba(255,255,255,0.06)" }}>
        <div className="flex items-center justify-between mb-4">
          <div className="flex items-center gap-3">
            <div className="w-9 h-9 rounded-xl flex items-center justify-center relative" style={{ background: "rgba(239,68,68,0.1)" }}>
              <Bell size={18} color="#EF4444" />
              {unreadCount > 0 && (
                <span className="absolute -top-1 -right-1 w-4 h-4 rounded-full flex items-center justify-center text-white" style={{ background: "#EF4444", fontSize: 9, fontWeight: 700 }}>{unreadCount}</span>
              )}
            </div>
            <div>
              <h2 style={{ color: "#E5E7EB", fontSize: 18, fontWeight: 700 }}>Notification Center</h2>
              <p style={{ color: "#6B7280", fontSize: 12 }}>{unreadCount} unread · {notifications.length} total</p>
            </div>
          </div>
          <div className="flex items-center gap-3">
            <button
              onClick={() => setShowUnreadOnly(!showUnreadOnly)}
              className="px-3 py-1.5 rounded-lg"
              style={{ background: showUnreadOnly ? "rgba(79,140,255,0.12)" : "rgba(255,255,255,0.05)", color: showUnreadOnly ? "#4F8CFF" : "#6B7280", border: "none", fontSize: 12, cursor: "pointer" }}
            >
              Unread only
            </button>
            <button
              className="px-3 py-1.5 rounded-lg"
              style={{ background: "rgba(255,255,255,0.05)", color: "#9CA3AF", border: "none", fontSize: 12, cursor: "pointer" }}
            >
              Mark all read
            </button>
          </div>
        </div>
        <div className="flex gap-2">
          {CATEGORIES.map((cat) => (
            <button
              key={cat.id}
              onClick={() => setFilter(cat.id)}
              className="px-3 py-1.5 rounded-lg"
              style={{ background: filter === cat.id ? "rgba(79,140,255,0.12)" : "rgba(255,255,255,0.05)", color: filter === cat.id ? "#4F8CFF" : "#6B7280", fontSize: 12, border: "none", cursor: "pointer" }}
            >
              {cat.label}
            </button>
          ))}
        </div>
      </div>

      {/* Notifications */}
      <div className="flex-1 overflow-y-auto">
        {filtered.map((n) => {
          const Icon = n.icon;
          return (
            <div
              key={n.id}
              className="flex items-start gap-4 px-6 py-4 cursor-pointer transition-all"
              style={{
                borderBottom: "1px solid rgba(255,255,255,0.04)",
                background: !n.read ? "rgba(79,140,255,0.03)" : "transparent",
                borderLeft: !n.read ? "3px solid #4F8CFF" : "3px solid transparent",
              }}
              onMouseEnter={(e) => (e.currentTarget.style.background = "rgba(255,255,255,0.02)")}
              onMouseLeave={(e) => (e.currentTarget.style.background = !n.read ? "rgba(79,140,255,0.03)" : "transparent")}
            >
              <div className="w-10 h-10 rounded-xl flex items-center justify-center flex-shrink-0 mt-0.5" style={{ background: n.bg }}>
                <Icon size={18} color={n.color} />
              </div>
              <div className="flex-1 min-w-0">
                <div className="flex items-center justify-between mb-1">
                  <div className="flex items-center gap-2">
                    <span style={{ color: "#E5E7EB", fontSize: 13, fontWeight: n.read ? 400 : 600 }}>{n.title}</span>
                    {!n.read && <div className="w-2 h-2 rounded-full" style={{ background: "#4F8CFF" }} />}
                  </div>
                  <span style={{ color: "#4B5563", fontSize: 11 }}>{n.time}</span>
                </div>
                <p style={{ color: "#9CA3AF", fontSize: 12, lineHeight: 1.5 }}>{n.desc}</p>
                <div className="flex items-center gap-3 mt-2">
                  <span className="px-2 py-0.5 rounded" style={{ background: "rgba(255,255,255,0.06)", color: "#6B7280", fontSize: 10 }}>{n.product}</span>
                  <button style={{ color: "#4F8CFF", fontSize: 11, background: "none", border: "none", cursor: "pointer" }}>View details</button>
                </div>
              </div>
            </div>
          );
        })}
      </div>
    </div>
  );
}
