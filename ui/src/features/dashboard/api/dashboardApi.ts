import { apiClient } from '@/shared/api/client';
import { ENDPOINTS } from '@/shared/api/endpoints';
import type { DashboardData, SLADashboardData } from '../types';

export const dashboardApi = {
  // GET /api/v1/dashboard?period=30d|90d|1y
  getMetrics: async (period: '30d' | '90d' | '1y' = '30d'): Promise<DashboardData> => {
    const { data } = await apiClient.get<DashboardData>(ENDPOINTS.dashboard.metrics, {
      params: { period },
    });
    return data;
  },

  // GET /api/v1/dashboard/sla
  getSLADetail: async (params: {
    product_id?: string;
    page?: number;
    page_size?: number;
  } = {}): Promise<SLADashboardData> => {
    const { data } = await apiClient.get<SLADashboardData>(ENDPOINTS.dashboard.sla, { params });
    return data;
  },

  /**
   * Connect to SSE notification stream.
   * Returns native EventSource for real-time notifications.
   * Use `?token=` query param since SSE doesn't support Authorization header.
   */
  streamNotifications: (token: string): EventSource => {
    const url = `${ENDPOINTS.notifications.stream}?token=${encodeURIComponent(token)}`;
    return new EventSource(url);
  },
};
