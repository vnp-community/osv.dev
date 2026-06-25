import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { apiClient } from '@/shared/api/client';
import { ENDPOINTS } from '@/shared/api/endpoints';

// ─── Types ────────────────────────────────────────────────────────────────────

export type RAStatus = 'pending' | 'approved' | 'rejected' | 'expired';

export interface RiskAcceptance {
  id: string;
  finding_id: string;
  finding_title: string;
  product_id: string;
  product_name: string;
  reason: string;
  expiration_date: string;
  retest_date?: string;
  approved_by?: string;
  owner: string;
  status: RAStatus;
  severity: 'Critical' | 'High' | 'Medium' | 'Low';
  days_left: number;
  created_at: string;
}

export interface RiskAcceptancesResponse {
  items: RiskAcceptance[];
  total: number;
}

export interface CreateRARequest {
  finding_id: string;
  reason: string;
  expiration_date: string;
  retest_date?: string;
}

// ─── Query Keys ──────────────────────────────────────────────────────────────

const raKeys = {
  all: ['risk-acceptances'] as const,
  list: (params?: Record<string, unknown>) =>
    [...raKeys.all, 'list', params] as const,
};

// ─── GET /api/v1/risk-acceptances ────────────────────────────────────────────

export function useRiskAcceptances(params?: { status?: string }) {
  return useQuery<RiskAcceptancesResponse>({
    queryKey: raKeys.list(params),
    queryFn: async () => {
      const { data } = await apiClient.get<RiskAcceptancesResponse>(
        ENDPOINTS.riskAcceptances.list,
        { params }
      );
      return data;
    },
    staleTime: 60_000,
  });
}

// ─── POST /api/v1/risk-acceptances ───────────────────────────────────────────

export function useCreateRiskAcceptance() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (req: CreateRARequest) =>
      apiClient.post(ENDPOINTS.riskAcceptances.create, req),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: raKeys.all });
    },
  });
}

// ─── PATCH /api/v1/risk-acceptances/:id (approve/reject) ─────────────────────

export function useUpdateRiskAcceptance() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({ id, status }: { id: string; status: RAStatus }) =>
      apiClient.patch(`${ENDPOINTS.riskAcceptances.list}/${id}`, { status }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: raKeys.all });
    },
  });
}
