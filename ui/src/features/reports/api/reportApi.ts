import { apiClient } from '@/shared/api/client';
import { ENDPOINTS } from '@/shared/api/endpoints';

export const reportApi = {
  list: async () => {
    const { data } = await apiClient.get(ENDPOINTS.reports.list);
    return data;
  },
  create: async (payload: { name: string; type: string; config: object }) => {
    const { data } = await apiClient.post(ENDPOINTS.reports.create, payload);
    return data;
  },
  download: async (id: string, format: 'pdf' | 'html' | 'csv' | 'xlsx'): Promise<Blob> => {
    const response = await apiClient.get(ENDPOINTS.reports.download(id), {
      params: { format },
      responseType: 'blob',
    });
    return response.data as Blob;
  },
};
