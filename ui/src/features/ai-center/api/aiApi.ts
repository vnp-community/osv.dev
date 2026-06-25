import { apiClient } from '@/shared/api/client';
import { ENDPOINTS } from '@/shared/api/endpoints';

export const aiApi = {
  triggerTriage: async (findingId: string) => {
    const { data } = await apiClient.post(ENDPOINTS.ai.triage(findingId));
    return data;
  },
  reviewTriage: async (findingId: string, payload: any) => {
    const { data } = await apiClient.post(ENDPOINTS.ai.triageReview(findingId), payload);
    return data;
  },
  getTriageQueue: async (params?: any) => {
    const { data } = await apiClient.get(ENDPOINTS.ai.triageQueue, { params });
    return data;
  },
  getEnrichmentStatus: async () => {
    const { data } = await apiClient.get(ENDPOINTS.ai.enrichment);
    return data;
  },
  triggerEnrichment: async (payload: any) => {
    const { data } = await apiClient.post(ENDPOINTS.ai.enrichTrigger, payload);
    return data;
  },
  getEnrichmentDetail: async (cveId: string) => {
    const { data } = await apiClient.get(ENDPOINTS.ai.enrichByCve(cveId));
    return data;
  },
};
