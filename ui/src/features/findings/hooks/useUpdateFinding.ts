import { useMutation } from '@tanstack/react-query';
import { findingKeys, queryClient } from '@/shared/api/queryClient';
import { findingApi } from '../api/findingApi';
import type { FindingStatus } from '@/shared/types/finding';
import { toast } from 'sonner';

export function useUpdateFinding() {
  return useMutation({
    mutationFn: ({
      id,
      status,
      comment,
    }: {
      id: string;
      status: FindingStatus;
      comment?: string;
    }) => findingApi.update(id, { status, comment }),

    onSuccess: (updated) => {
      // Update specific finding in cache
      queryClient.setQueryData(findingKeys.detail(updated.id), updated);
      // Invalidate list queries
      queryClient.invalidateQueries({ queryKey: findingKeys.all });
      toast.success('Finding status updated');
    },
  });
}

export function useBulkCloseFinding() {
  return useMutation({
    mutationFn: ({
      findingIds,
      comment,
    }: {
      findingIds: string[];
      comment?: string;
    }) => findingApi.bulkClose(findingIds, comment),

    onSuccess: (result) => {
      queryClient.invalidateQueries({ queryKey: findingKeys.all });
      toast.success(`${result.success_count} findings closed`);
    },
  });
}
