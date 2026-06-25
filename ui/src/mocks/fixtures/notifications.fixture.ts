/**
 * Fixture dữ liệu cho Notifications.
 * Dùng trong notification.handlers.ts và các unit tests.
 *
 * ⚠️ Chỉ dùng trong src/mocks/
 */

export interface NotificationFixture {
  id: string;
  type: 'critical' | 'sla' | 'kev' | 'scan';
  title: string;
  description: string;
  product?: string;
  read: boolean;
  created_at: string;
  time_ago: string;
  metadata?: Record<string, unknown>;
}

export const notificationsFixture: NotificationFixture[] = [
  {
    id: 'n-1',
    type: 'critical',
    title: 'Critical Finding Detected',
    description: 'CVE-2025-44228 found on webserver01.prod — CVSS 10.0, KEV active',
    product: 'Banking App',
    read: false,
    created_at: new Date(Date.now() - 10 * 60_000).toISOString(),
    time_ago: '10 min ago',
    metadata: { finding_id: 'F-2847', cve_id: 'CVE-2025-44228', cvss: 10.0 },
  },
  {
    id: 'n-2',
    type: 'sla',
    title: 'SLA Breach Imminent',
    description: 'F-2842 (Cisco IOS XE) SLA expires in 24h — escalation required',
    product: 'Network Infra',
    read: false,
    created_at: new Date(Date.now() - 25 * 60_000).toISOString(),
    time_ago: '25 min ago',
    metadata: { finding_id: 'F-2842', hours_remaining: 24 },
  },
  {
    id: 'n-3',
    type: 'kev',
    title: 'New KEV Added',
    description: 'CISA added CVE-2025-77001 (Microsoft Exchange) to KEV catalog',
    product: 'Global',
    read: false,
    created_at: new Date(Date.now() - 3_600_000).toISOString(),
    time_ago: '1h ago',
    metadata: { cve_id: 'CVE-2025-77001' },
  },
  {
    id: 'n-4',
    type: 'scan',
    title: 'Scan Completed',
    description: 'Production Network Sweep (SC-0047) completed — 47 findings discovered',
    product: 'Production',
    read: true,
    created_at: new Date(Date.now() - 7_200_000).toISOString(),
    time_ago: '2h ago',
    metadata: { scan_id: 'SC-0047', finding_count: 47 },
  },
  {
    id: 'n-5',
    type: 'critical',
    title: 'SLA Overdue',
    description: 'F-2846 (Spring Framework RCE) is 2 days overdue — immediate action required',
    product: 'API Gateway',
    read: true,
    created_at: new Date(Date.now() - 10_800_000).toISOString(),
    time_ago: '3h ago',
    metadata: { finding_id: 'F-2846', days_overdue: 2 },
  },
];
