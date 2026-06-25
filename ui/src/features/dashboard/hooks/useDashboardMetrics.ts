import { useQuery } from '@tanstack/react-query';
import { dashboardApi } from '../api/dashboardApi';
import type { DashboardData } from '../types';

export const dashboardKeys = {
  all:     ['dashboard'] as const,
  metrics: (period: string) => ['dashboard', 'metrics', period] as const,
  sla:     (params: object) => ['dashboard', 'sla', params] as const,
};

export function useDashboardMetrics(period: '30d' | '90d' | '1y' = '30d') {
  return useQuery<DashboardData>({
    queryKey:        dashboardKeys.metrics(period),
    queryFn:         () => dashboardApi.getMetrics(period),
    staleTime:       60_000,
    refetchInterval: 60_000,   // Auto-refresh mỗi 60s
    retry: 1,
  });
}
