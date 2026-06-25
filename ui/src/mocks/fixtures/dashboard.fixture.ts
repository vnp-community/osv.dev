// ⚠️ Chỉ dùng trong src/mocks/ — KHÔNG import vào component files
import type { Scan } from '@/shared/types/scan';

export interface DashboardFixtureData {
  kpis: {
    criticalFindings: number;
    highFindings: number;
    totalAssets: number;
    highRiskAssets: number;
    activeScans: number;
    queuedScans: number;
    securityGrade: string;
    securityScore: number;
    slaCompliance: number;
    slaAtRisk: number;
    slaBreached: number;
  };
  riskTrend: Array<{ month: string; critical: number; high: number; medium: number; low: number }>;
  severityDistribution: { critical: number; high: number; medium: number; low: number; total: number };
  productGrades: Array<{ id: string; name: string; grade: string; score: number; criticalCount: number; highCount: number }>;
  kevAlerts: Array<{ cveId: string; vendor: string; product: string; dateAdded: string; isRansomware: boolean }>;
  recentScans: Scan[];
  slaBreaches: Array<{ findingId: string; title: string; dueIn: string; severity: string; isOverdue: boolean }>;
}

const base: DashboardFixtureData = {
  kpis: {
    criticalFindings: 245,
    highFindings: 395,
    totalAssets: 1247,
    highRiskAssets: 98,
    activeScans: 3,
    queuedScans: 2,
    securityGrade: 'B-',
    securityScore: 61,
    slaCompliance: 94.2,
    slaAtRisk: 22,
    slaBreached: 8,
  },
  riskTrend: [
    { month: 'Jan', critical: 320, high: 480, medium: 820, low: 1200 },
    { month: 'Feb', critical: 290, high: 450, medium: 780, low: 1150 },
    { month: 'Mar', critical: 310, high: 510, medium: 850, low: 1280 },
    { month: 'Apr', critical: 280, high: 420, medium: 740, low: 1100 },
    { month: 'May', critical: 245, high: 380, medium: 690, low: 980 },
    { month: 'Jun', critical: 245, high: 395, medium: 710, low: 1020 },
  ],
  severityDistribution: { critical: 245, high: 395, medium: 710, low: 1020, total: 2370 },
  productGrades: [
    { id: 'p1', name: 'Banking App', grade: 'B', score: 62, criticalCount: 8, highCount: 24 },
    { id: 'p2', name: 'Mobile App', grade: 'A-', score: 78, criticalCount: 2, highCount: 11 },
    { id: 'p3', name: 'API Gateway', grade: 'C+', score: 45, criticalCount: 14, highCount: 38 },
    { id: 'p4', name: 'Admin Portal', grade: 'B+', score: 71, criticalCount: 4, highCount: 16 },
    { id: 'p5', name: 'Data Pipeline', grade: 'C+', score: 55, criticalCount: 9, highCount: 22 },
  ],
  kevAlerts: [
    { cveId: 'CVE-2025-44228', vendor: 'Apache', product: 'Log4j2', dateAdded: '2026-06-12', isRansomware: false },
    { cveId: 'CVE-2025-22965', vendor: 'VMware', product: 'Spring', dateAdded: '2026-06-09', isRansomware: true },
    { cveId: 'CVE-2025-09876', vendor: 'Cisco', product: 'IOS XE', dateAdded: '2026-06-07', isRansomware: false },
  ],
  recentScans: [
    { id: 'SC-0047', name: 'Production Network', type: 'nmap_full', status: 'completed', targets: ['10.0.0.0/24'], progress: 100, findingCount: 47, createdBy: 'admin@osv.local', completedAt: '2026-06-14T10:30:00Z' },
    { id: 'SC-0048', name: 'API Security Scan', type: 'zap', status: 'running', targets: ['https://api.internal'], progress: 45, findingCount: 12, createdBy: 'admin@osv.local', startedAt: '2026-06-14T11:00:00Z' },
    { id: 'SC-0046', name: 'Dev Environment', type: 'nmap_discovery', status: 'completed', targets: ['192.168.1.0/24'], progress: 100, findingCount: 8, createdBy: 'dev@osv.local', completedAt: '2026-06-14T08:00:00Z' },
  ],
  slaBreaches: [
    { findingId: 'F-2846', title: 'Spring Framework RCE', dueIn: 'Overdue -2d', severity: 'Critical', isOverdue: true },
    { findingId: 'F-2841', title: 'Kubernetes API Exposure', dueIn: 'Overdue -1d', severity: 'Critical', isOverdue: true },
    { findingId: 'F-2838', title: 'Redis Unauthorized Access', dueIn: '2d left', severity: 'High', isOverdue: false },
  ],
};

export const dashboardFixture: Record<string, DashboardFixtureData> = {
  '30d': base,
  '90d': {
    ...base,
    kpis: { ...base.kpis, criticalFindings: 890, highFindings: 1240 },
  },
  '1y': {
    ...base,
    kpis: { ...base.kpis, criticalFindings: 3200, highFindings: 4800 },
  },
};
