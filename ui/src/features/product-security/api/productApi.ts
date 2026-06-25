import { apiClient } from '@/shared/api/client';
import { ENDPOINTS } from '@/shared/api/endpoints';

export const productApi = {
  list: async (params?: any) => {
    const { data } = await apiClient.get(ENDPOINTS.products.list, { params });
    return data;
  },
  create: async (payload: any) => {
    const { data } = await apiClient.post(ENDPOINTS.products.create, payload);
    return data;
  },
  getById: async (id: string) => {
    const { data } = await apiClient.get(ENDPOINTS.products.detail(id));
    return data;
  },
  patch: async (id: string, payload: any) => {
    const { data } = await apiClient.patch(ENDPOINTS.products.patch(id), payload);
    return data;
  },
  getEngagements: async (productId: string) => {
    const { data } = await apiClient.get(ENDPOINTS.products.engagements(productId));
    return data;
  },
  createEngagement: async (productId: string, payload: any) => {
    const { data } = await apiClient.post(ENDPOINTS.products.engagements(productId), payload);
    return data;
  },
  getTests: async (engagementId: string) => {
    const { data } = await apiClient.get(ENDPOINTS.engagements.tests(engagementId));
    return data;
  },
  getTypes: async () => {
    const { data } = await apiClient.get(ENDPOINTS.products.types);
    return data;
  },
  getGrades: async () => {
    const { data } = await apiClient.get(ENDPOINTS.products.grades);
    return data;
  },
};
