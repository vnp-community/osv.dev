export type ScanType = 'nmap_full' | 'nmap_discovery' | 'zap' | 'agent' | 'import';
export type ScanStatus = 'pending' | 'queued' | 'running' | 'completed' | 'failed' | 'cancelled';
export type ScanFrequency = 'once' | 'daily' | 'weekly' | 'custom';

export interface Scan {
  id: string;
  name: string;
  type: ScanType;
  status: ScanStatus;
  targets: string[];             // IPs, CIDRs, URLs
  progress: number;              // 0-100
  findingCount: number;
  startedAt?: string;
  completedAt?: string;
  durationMs?: number;
  createdBy: string;
  engagementId?: string;
  error?: string;
}

export interface ScanProgress {
  scanId: string;
  status: ScanStatus;
  progress: number;              // 0-100
  currentTarget?: string;
  message?: string;
  findingsFound: number;
}

export interface ScanSummary {
  id: string;
  name: string;
  type: ScanType;
  status: ScanStatus;
  findingCount: number;
  startedAt?: string;
  completedAt?: string;
}

export interface NmapPort {
  port: number;
  protocol: 'tcp' | 'udp';
  state: 'open' | 'filtered';
  service: string;
  version?: string;
  cveIds: string[];
}

export interface NmapHost {
  ip: string;
  hostname?: string;
  os?: string;
  state: 'up' | 'down';
  ports: NmapPort[];
  cveIds: string[];
  riskScore: number;            // Max CVSS across CVEs
}

export interface ZAPAlert {
  id: string;
  name: string;
  risk: 'High' | 'Medium' | 'Low' | 'Informational';
  confidence: 'High' | 'Medium' | 'Low';
  url: string;
  description: string;
  solution: string;
  evidence?: string;
  cweId?: string;
  references: string[];
}

export interface ScheduledScan {
  id: string;
  name: string;
  type: ScanType;
  targets: string[];
  frequency: ScanFrequency;
  cronExpr?: string;
  nextRunAt: string;
  lastRunAt?: string;
  enabled: boolean;
}

// Asset type (related to scans)
export interface AssetService {
  port: number;
  protocol: string;
  service: string;
  version?: string;
  cveIds: string[];
}

export interface Asset {
  id: string;
  ip: string;
  hostname?: string;
  os?: string;
  services: AssetService[];
  webTechnologies: string[];
  tags: string[];
  riskScore: number;
  activeFindingCount: number;
  firstSeenAt: string;
  lastSeenAt: string;
  lastScanId?: string;
}
