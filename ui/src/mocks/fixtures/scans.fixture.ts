// ⚠️ Chỉ dùng trong src/mocks/
import type { Scan } from '@/shared/types/scan';

export const scansFixture: Scan[] = [
  { id: 'SC-0047', name: 'Production Network Q2', type: 'nmap_full', status: 'completed', targets: ['10.0.0.0/24'], progress: 100, findingCount: 47, createdBy: 'admin@osv.local', startedAt: '2026-06-14T08:00:00Z', completedAt: '2026-06-14T10:30:00Z', durationMs: 9000000 },
  { id: 'SC-0048', name: 'API Security Scan', type: 'zap', status: 'running', targets: ['https://api.internal'], progress: 45, findingCount: 12, createdBy: 'admin@osv.local', startedAt: '2026-06-14T11:00:00Z' },
  { id: 'SC-0046', name: 'Dev Environment', type: 'nmap_discovery', status: 'completed', targets: ['192.168.1.0/24'], progress: 100, findingCount: 8, createdBy: 'dev@osv.local', startedAt: '2026-06-14T06:00:00Z', completedAt: '2026-06-14T08:00:00Z', durationMs: 7200000 },
  { id: 'SC-0045', name: 'Staging Network', type: 'nmap_full', status: 'failed', targets: ['10.1.0.0/24'], progress: 67, findingCount: 0, createdBy: 'admin@osv.local', startedAt: '2026-06-13T14:00:00Z', error: 'Connection timeout to 10.1.0.1' },
  { id: 'SC-0044', name: 'Weekly Discovery', type: 'nmap_discovery', status: 'completed', targets: ['10.0.0.0/16'], progress: 100, findingCount: 23, createdBy: 'admin@osv.local', startedAt: '2026-06-08T02:00:00Z', completedAt: '2026-06-08T04:30:00Z', durationMs: 9000000 },
];
