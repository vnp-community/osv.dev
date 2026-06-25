import { useQuery } from '@tanstack/react-query';
import { assetKeys } from '@/shared/api/queryClient';
import { assetApi, type AssetsListParams } from '../api/assetApi';

export function useAssets(params?: AssetsListParams) {
  return useQuery({
    queryKey: assetKeys.list(params),
    queryFn: () => assetApi.list(params),
    staleTime: 60_000,
    placeholderData: (prev) => prev,
  });
}

export function useAssetDetail(id: string | null) {
  return useQuery({
    queryKey: assetKeys.detail(id ?? ''),
    queryFn: () => assetApi.getById(id!),
    enabled: !!id,
    staleTime: 60_000,
  });
}
