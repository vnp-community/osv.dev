/**
 * useScanStats — React Query hooks cho scan stats & weekly activity chart.
 * TASK-P4-02: thay thế Math.random() trong ScanDashboard.tsx
 */
import { useQuery } from '@tanstack/react-query';
import { apiClient } from '@/shared/api/client';

// ─── Types ────────────────────────────────────────────────────────────────────

export interface ScanStats {
  active_scans: number;
  completed_today: number;
  total_findings: number;
  scheduled_scans: number;
  failed_today: number;
}

export interface WeeklyActivity {
  day: string;  // 'Mon', 'Tue', 'Wed', 'Thu', 'Fri', 'Sat', 'Sun'
  scans: number;
  findings: number;
}

// ─── Query Keys ──────────────────────────────────────────────────────────────

export const scanStatsKeys = {
  stats: ['scans', 'stats'] as const,
  weekly: ['scans', 'stats', 'weekly'] as const,
};

// ─── GET /api/v1/scans/stats ─────────────────────────────────────────────────

export function useScanStats() {
  return useQuery<ScanStats>({
    queryKey: scanStatsKeys.stats,
    queryFn: async () => {
      const { data } = await apiClient.get<ScanStats>('/api/v1/scans/stats');
      return data;
    },
    staleTime: 30_000,
    refetchInterval: 30_000,
  });
}

// ─── GET /api/v1/scans/stats/weekly ──────────────────────────────────────────

export function useWeeklyScanActivity() {
  return useQuery<WeeklyActivity[]>({
    queryKey: scanStatsKeys.weekly,
    queryFn: async () => {
      const { data } = await apiClient.get<WeeklyActivity[]>('/api/v1/scans/stats/weekly');
      return data;
    },
    staleTime: 5 * 60_000,
  });
}
