import { useQuery } from '@tanstack/react-query';
import { findingKeys } from '@/shared/api/queryClient';
import { findingApi, type FindingsListParams } from '../api/findingApi';

export function useFindings(params: FindingsListParams) {
  return useQuery({
    queryKey: findingKeys.list(params),
    queryFn: () => findingApi.list(params),
    staleTime: 30_000,
    placeholderData: (prev) => prev,
  });
}

export function useFindingDetail(id: string | null) {
  return useQuery({
    queryKey: findingKeys.detail(id ?? ''),
    queryFn: () => findingApi.getById(id!),
    enabled: !!id,
    staleTime: 30_000,
  });
}
