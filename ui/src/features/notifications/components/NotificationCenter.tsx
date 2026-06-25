import { useState } from "react";
import { AlertTriangle, Clock, Shield, Activity, Bell, Loader2, RefreshCw } from "lucide-react";
import { useNotificationList, useMarkAllNotificationsRead, type AppNotification, type NotificationType } from "../hooks/useNotificationList";

// ─── Icon & color map per notification type ────────────────────────────────────

const TYPE_CONFIG: Record<NotificationType, { icon: React.ElementType; color: string; bg: string }> = {
  critical: { icon: AlertTriangle, color: "var(--color-status-error, #EF4444)",    bg: "var(--color-status-error-bg, rgba(239,68,68,0.1))" },
  sla:      { icon: Clock,         color: "var(--color-status-warning, #F59E0B)",   bg: "var(--color-status-warning-bg, rgba(245,158,11,0.1))" },
  kev:      { icon: Shield,        color: "var(--color-status-error, #EF4444)",     bg: "var(--color-status-error-bg, rgba(239,68,68,0.08))" },
  scan:     { icon: Activity,      color: "var(--color-primary, #4F8CFF)",          bg: "var(--color-primary-bg, rgba(79,140,255,0.1))" },
  info:     { icon: Bell,          color: "var(--color-status-success, #10B981)",   bg: "var(--color-status-success-bg, rgba(16,185,129,0.1))" },
};

const CATEGORIES = [
  { id: "all",      label: "All" },
  { id: "critical", label: "Critical" },
  { id: "sla",      label: "SLA Alerts" },
  { id: "kev",      label: "KEV Updates" },
  { id: "scan",     label: "Scan Events" },
];

function formatTimeAgo(iso: string): string {
  const diff = Date.now() - new Date(iso).getTime();
  const mins = Math.floor(diff / 60_000);
  if (mins < 60) return `${mins}m ago`;
  const hrs = Math.floor(mins / 60);
  if (hrs < 24) return `${hrs}h ago`;
  return `${Math.floor(hrs / 24)}d ago`;
}

// ─── Main Component ───────────────────────────────────────────────────────────

export function NotificationCenter() {
  const [filter, setFilter] = useState<NotificationType | "all">("all");
  const [showUnreadOnly, setShowUnreadOnly] = useState(false);

  const { data, isLoading, isError, refetch } = useNotificationList({
    type: filter !== "all" ? filter : undefined,
    unread_only: showUnreadOnly || undefined,
  });
  const markAllRead = useMarkAllNotificationsRead();

  const notifications: AppNotification[] = data?.items ?? [];
  const unreadCount = data?.unread_count ?? 0;

  if (isLoading) {
    return (
      <div className="flex-1 flex items-center justify-center" style={{ background: "var(--color-bg-page, #0B1020)" }}>
        <div className="flex flex-col items-center gap-3">
          <Loader2 size={28} className="animate-spin" style={{ color: "var(--color-status-error, #EF4444)" }} />
          <p style={{ color: "var(--color-text-muted, #6B7280)", fontSize: 13 }}>Loading notifications...</p>
        </div>
      </div>
    );
  }

  if (isError) {
    return (
      <div className="flex-1 flex items-center justify-center" style={{ background: "var(--color-bg-page, #0B1020)" }}>
        <div className="text-center">
          <AlertTriangle size={32} style={{ color: "var(--color-status-error, #EF4444)", margin: "0 auto 12px" }} />
          <p style={{ color: "var(--color-text-secondary, #9CA3AF)", fontSize: 13 }}>Failed to load notifications</p>
          <button
            onClick={() => refetch()}
            className="mt-3 px-4 py-2 rounded-xl flex items-center gap-2 mx-auto"
            style={{ background: "var(--color-primary-bg, rgba(79,140,255,0.1))", color: "var(--color-primary, #4F8CFF)", border: "none", cursor: "pointer", fontSize: 13 }}
          >
            <RefreshCw size={13} /> Retry
          </button>
        </div>
      </div>
    );
  }

  return (
    <div className="flex flex-col flex-1 overflow-hidden" style={{ background: "var(--color-bg-page, #0B1020)" }}>
      {/* Header */}
      <div className="px-6 py-4" style={{ borderBottom: "1px solid var(--color-border-section, rgba(255,255,255,0.06))" }}>
        <div className="flex items-center justify-between mb-4">
          <div className="flex items-center gap-3">
            <div
              className="w-9 h-9 rounded-xl flex items-center justify-center relative"
              style={{ background: "var(--color-status-error-bg, rgba(239,68,68,0.1))" }}
            >
              <Bell size={18} color="var(--color-status-error, #EF4444)" />
              {unreadCount > 0 && (
                <span
                  className="absolute -top-1 -right-1 w-4 h-4 rounded-full flex items-center justify-center text-white"
                  style={{ background: "var(--color-status-error, #EF4444)", fontSize: 9, fontWeight: 700 }}
                >
                  {unreadCount}
                </span>
              )}
            </div>
            <div>
              <h2 style={{ color: "var(--color-text-primary, #E5E7EB)", fontSize: 18, fontWeight: 700 }}>
                Notification Center
              </h2>
              <p style={{ color: "var(--color-text-muted, #6B7280)", fontSize: 12 }}>
                {unreadCount} unread · {data?.total ?? 0} total
              </p>
            </div>
          </div>
          <div className="flex items-center gap-3">
            <button
              id="notif-unread-toggle"
              onClick={() => setShowUnreadOnly((v) => !v)}
              className="px-3 py-1.5 rounded-lg"
              style={{
                background: showUnreadOnly
                  ? "var(--color-primary-bg, rgba(79,140,255,0.12))"
                  : "var(--color-bg-input, rgba(255,255,255,0.05))",
                color: showUnreadOnly ? "var(--color-primary, #4F8CFF)" : "var(--color-text-muted, #6B7280)",
                border: "none",
                fontSize: 12,
                cursor: "pointer",
              }}
            >
              Unread only
            </button>
            <button
              id="notif-mark-all-read"
              onClick={() => markAllRead.mutate()}
              disabled={markAllRead.isPending || unreadCount === 0}
              className="px-3 py-1.5 rounded-lg flex items-center gap-2"
              style={{
                background: "var(--color-bg-input, rgba(255,255,255,0.05))",
                color: "var(--color-text-secondary, #9CA3AF)",
                border: "none",
                fontSize: 12,
                cursor: markAllRead.isPending || unreadCount === 0 ? "not-allowed" : "pointer",
                opacity: unreadCount === 0 ? 0.5 : 1,
              }}
            >
              {markAllRead.isPending && <Loader2 size={11} className="animate-spin" />}
              Mark all read
            </button>
          </div>
        </div>
        <div className="flex gap-2">
          {CATEGORIES.map((cat) => (
            <button
              key={cat.id}
              onClick={() => setFilter(cat.id as NotificationType | "all")}
              className="px-3 py-1.5 rounded-lg"
              style={{
                background: filter === cat.id
                  ? "var(--color-primary-bg, rgba(79,140,255,0.12))"
                  : "var(--color-bg-input, rgba(255,255,255,0.05))",
                color: filter === cat.id ? "var(--color-primary, #4F8CFF)" : "var(--color-text-muted, #6B7280)",
                fontSize: 12,
                border: "none",
                cursor: "pointer",
              }}
            >
              {cat.label}
            </button>
          ))}
        </div>
      </div>

      {/* Notifications list */}
      <div className="flex-1 overflow-y-auto">
        {notifications.map((n) => {
          const cfg = TYPE_CONFIG[n.type] ?? TYPE_CONFIG.info;
          const Icon = cfg.icon;
          return (
            <div
              key={n.id}
              className="flex items-start gap-4 px-6 py-4 cursor-pointer transition-all"
              style={{
                borderBottom: "1px solid var(--color-border-section, rgba(255,255,255,0.04))",
                background: !n.read ? "var(--color-primary-bg-faint, rgba(79,140,255,0.03))" : "transparent",
                borderLeft: !n.read ? "3px solid var(--color-primary, #4F8CFF)" : "3px solid transparent",
              }}
              onMouseEnter={(e) => (e.currentTarget.style.background = "var(--color-bg-hover, rgba(255,255,255,0.02))")}
              onMouseLeave={(e) => (e.currentTarget.style.background = !n.read ? "var(--color-primary-bg-faint, rgba(79,140,255,0.03))" : "transparent")}
            >
              <div
                className="w-10 h-10 rounded-xl flex items-center justify-center flex-shrink-0 mt-0.5"
                style={{ background: cfg.bg }}
              >
                <Icon size={18} color={cfg.color} />
              </div>
              <div className="flex-1 min-w-0">
                <div className="flex items-center justify-between mb-1">
                  <div className="flex items-center gap-2">
                    <span style={{ color: "var(--color-text-primary, #E5E7EB)", fontSize: 13, fontWeight: n.read ? 400 : 600 }}>
                      {n.title}
                    </span>
                    {!n.read && (
                      <div
                        className="w-2 h-2 rounded-full"
                        style={{ background: "var(--color-primary, #4F8CFF)" }}
                      />
                    )}
                  </div>
                  <span style={{ color: "var(--color-text-faint, #4B5563)", fontSize: 11 }}>
                    {n.time_ago ?? formatTimeAgo(n.created_at)}
                  </span>
                </div>
                <p style={{ color: "var(--color-text-secondary, #9CA3AF)", fontSize: 12, lineHeight: 1.5 }}>
                  {n.description}
                </p>
                <div className="flex items-center gap-3 mt-2">
                  <span
                    className="px-2 py-0.5 rounded"
                    style={{ background: "var(--color-bg-input, rgba(255,255,255,0.06))", color: "var(--color-text-muted, #6B7280)", fontSize: 10 }}
                  >
                    {n.product}
                  </span>
                  <button
                    style={{ color: "var(--color-primary, #4F8CFF)", fontSize: 11, background: "none", border: "none", cursor: "pointer" }}
                  >
                    View details
                  </button>
                </div>
              </div>
            </div>
          );
        })}

        {notifications.length === 0 && (
          <div className="text-center py-16">
            <Bell size={32} style={{ color: "var(--color-text-disabled, #374151)", margin: "0 auto 12px" }} />
            <p style={{ color: "var(--color-text-muted, #6B7280)", fontSize: 13 }}>No notifications</p>
          </div>
        )}
      </div>
    </div>
  );
}
