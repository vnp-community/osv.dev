import { apiClient } from '@/shared/api/client';
import { ENDPOINTS } from '@/shared/api/endpoints';
import type { Asset } from '@/shared/types/scan';

export interface AssetsListParams {
  riskLevel?: 'critical' | 'high' | 'medium' | 'low';
  tags?: string[];
  query?: string;
  page?: number;
  pageSize?: number;
}

export interface AssetsListResponse {
  assets: Asset[];
  total: number;
}

export const assetApi = {
  list: async (params?: AssetsListParams): Promise<AssetsListResponse> => {
    const { data } = await apiClient.get<AssetsListResponse>(
      ENDPOINTS.assets.list,
      { params }
    );
    // Normalize — đảm bảo assets luôn là array dù backend trả null/undefined
    return {
      assets: Array.isArray(data?.assets) ? data.assets : [],
      total:  typeof data?.total === 'number' ? data.total : 0,
    };
  },

  getById: async (id: string): Promise<Asset> => {
    const { data } = await apiClient.get<Asset>(ENDPOINTS.assets.detail(id));
    return data;
  },
};
