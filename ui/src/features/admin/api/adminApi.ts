import { apiClient } from '@/shared/api/client';
import { ENDPOINTS } from '@/shared/api/endpoints';

export const adminApi = {
  getUsers: async () => {
    const { data } = await apiClient.get(ENDPOINTS.admin.users);
    return data;
  },
  getHealth: async () => {
    const { data } = await apiClient.get(ENDPOINTS.admin.health);
    return data;
  },
  getAuditLogs: async (params?: { page?: number; pageSize?: number }) => {
    const { data } = await apiClient.get(ENDPOINTS.audit.log, { params });
    return data;
  },
  getSettings: async () => {
    const { data } = await apiClient.get(ENDPOINTS.admin.settings);
    return data;
  },
};
