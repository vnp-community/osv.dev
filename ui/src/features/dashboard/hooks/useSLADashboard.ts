import { useQuery } from '@tanstack/react-query';
import { dashboardApi } from '../api/dashboardApi';
import { dashboardKeys } from './useDashboardMetrics';

export function useSLADashboard(params: {
  productId?: string;
  page?: number;
  pageSize?: number;
} = {}) {
  const queryParams = {
    product_id: params.productId,
    page:       params.page ?? 1,
    page_size:  params.pageSize ?? 20,
  };

  return useQuery({
    queryKey: dashboardKeys.sla(queryParams),
    queryFn:  () => dashboardApi.getSLADetail(queryParams),
    staleTime: 30_000,
  });
}
