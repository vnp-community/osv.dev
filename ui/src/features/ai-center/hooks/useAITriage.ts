import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { apiClient } from '@/shared/api/client';
import { ENDPOINTS } from '@/shared/api/endpoints';

// ─── Types ────────────────────────────────────────────────────────────────────

export type TriageStatus = 'pending' | 'accepted' | 'rejected';
export type TriageVerdict =
  | 'Patch Immediately'
  | 'Schedule Patch'
  | 'Configure Auth'
  | 'False Positive'
  | 'Accept Risk';

export interface AITriageItem {
  id: string;
  finding_id: string;
  title: string;
  verdict: TriageVerdict;
  confidence: number;       // 0–100
  severity: 'Critical' | 'High' | 'Medium' | 'Low';
  created_at: string;
  status: TriageStatus;
  reasoning: string;
  suggested_fixes: string[];
}

export interface AITriageQueueResponse {
  items: AITriageItem[];
  total: number;
  pending_count: number;
  accepted_today: number;
  avg_confidence: number;
}

// ─── Query Keys ──────────────────────────────────────────────────────────────

const triageKeys = {
  all: ['ai', 'triage'] as const,
  queue: (params?: Record<string, unknown>) =>
    [...triageKeys.all, 'queue', params] as const,
};

// ─── GET /api/v1/ai/triage/queue ─────────────────────────────────────────────

export function useAITriageQueue(params?: { status?: TriageStatus }) {
  return useQuery<AITriageQueueResponse>({
    queryKey: triageKeys.queue(params),
    queryFn: async () => {
      const { data } = await apiClient.get<AITriageQueueResponse>(
        ENDPOINTS.ai.triageQueue,
        { params }
      );
      return data;
    },
    staleTime: 30_000,
  });
}

// ─── POST /api/v1/ai/triage/:findingId/review ────────────────────────────────

export function useReviewTriage() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({
      findingId,
      decision,
    }: {
      findingId: string;
      decision: 'accepted' | 'rejected';
    }) =>
      apiClient.post(ENDPOINTS.ai.triageReview(findingId), { decision }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: triageKeys.all });
    },
  });
}
