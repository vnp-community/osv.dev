import { apiClient } from '@/shared/api/client';
import { ENDPOINTS } from '@/shared/api/endpoints';

export const notificationApi = {
  list: async (params?: any) => {
    const { data } = await apiClient.get(ENDPOINTS.notifications.list, { params });
    return data;
  },
  markRead: async (id: string) => {
    const { data } = await apiClient.patch(ENDPOINTS.notifications.markRead(id));
    return data;
  },
  markAllRead: async () => {
    const { data } = await apiClient.post(ENDPOINTS.notifications.markAllRead);
    return data;
  },
  getUnreadCount: async () => {
    const { data } = await apiClient.get(ENDPOINTS.notifications.unreadCount);
    return data;
  },
};
