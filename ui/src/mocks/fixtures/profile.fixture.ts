// Fixtures for Profile sessions and notification settings

export interface UserSession {
  id: string;
  device: string;
  ip: string;
  location: string;
  lastActive: string;
  current: boolean;
}

export interface NotifSetting {
  id: string;
  label: string;
  desc: string;
  enabled: boolean;
}

export const sessionsFixture: UserSession[] = [
  { id: 's-1', device: 'Chrome on macOS',     ip: '192.168.1.100',  location: 'Ho Chi Minh City, VN', lastActive: new Date().toISOString(), current: true },
  { id: 's-2', device: 'Firefox on Windows',  ip: '10.10.5.22',     location: 'Hanoi, VN',             lastActive: new Date(Date.now() - 2 * 3600_000).toISOString(), current: false },
  { id: 's-3', device: 'Mobile — Safari iOS', ip: '203.113.131.45', location: 'Singapore, SG',         lastActive: new Date(Date.now() - 86400_000).toISOString(), current: false },
];

export const notifSettingsFixture: NotifSetting[] = [
  { id: 'ns-1', label: 'Critical Finding Alerts', desc: 'Notify when Critical severity findings are created',        enabled: true },
  { id: 'ns-2', label: 'SLA Breach Warnings',      desc: 'Alert 48h before SLA expiration',                         enabled: true },
  { id: 'ns-3', label: 'KEV Updates',              desc: 'New CISA KEV additions affecting your assets',             enabled: true },
  { id: 'ns-4', label: 'Scan Completion',          desc: 'Notify when scans complete or fail',                       enabled: false },
  { id: 'ns-5', label: 'Weekly Digest',            desc: 'Weekly summary of platform activity',                      enabled: true },
];
