import { useQuery, useMutation } from '@tanstack/react-query';
import { scanKeys, queryClient } from '@/shared/api/queryClient';
import { scanApi, type CreateScanPayload } from '../api/scanApi';
import { useNavigate } from 'react-router';
import { toast } from 'sonner';

export function useScans(params?: { status?: string }) {
  return useQuery({
    queryKey: scanKeys.list(params),
    queryFn: () => scanApi.list(params),
    staleTime: 15_000,
    refetchInterval: 15_000,  // Poll tích cực khi có scan đang chạy
  });
}

export function useScanDetail(id: string | null) {
  return useQuery({
    queryKey: scanKeys.detail(id ?? ''),
    queryFn: () => scanApi.getById(id!),
    enabled: !!id,
    staleTime: 10_000,
  });
}

export function useCreateScan() {
  const navigate = useNavigate();

  return useMutation({
    mutationFn: (payload: CreateScanPayload) => scanApi.create(payload),
    onSuccess: (scan) => {
      queryClient.invalidateQueries({ queryKey: scanKeys.all });
      toast.success(`Scan "${scan.name}" queued successfully`);
      navigate(`/scans/${scan.id}`);
    },
  });
}

export function useCancelScan() {
  return useMutation({
    mutationFn: (id: string) => scanApi.cancel(id),
    onSuccess: (_, id) => {
      queryClient.invalidateQueries({ queryKey: scanKeys.detail(id) });
      queryClient.invalidateQueries({ queryKey: scanKeys.list() });
      toast.success('Scan cancelled');
    },
  });
}
