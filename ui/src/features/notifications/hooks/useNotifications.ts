import { useState, useCallback } from 'react';
import { useSSE } from '@/shared/hooks/useSSE';
import { useAuthStore } from '@/features/auth/store/authStore';
import { ENDPOINTS } from '@/shared/api/endpoints';

interface Notification {
  id: string;
  type: string;
  title: string;
  message: string;
  severity?: string;
  timestamp: string;
  read: boolean;
}

export function useNotifications() {
  const { isAuthenticated } = useAuthStore();
  const [notifications, setNotifications] = useState<Notification[]>([]);

  const handleMessage = useCallback((notification: Notification) => {
    setNotifications((prev) => [notification, ...prev.slice(0, 99)]); // Max 100
  }, []);

  const { status } = useSSE<Notification>(
    ENDPOINTS.notifications.stream,
    isAuthenticated,
    { onMessage: handleMessage }
  );

  const unreadCount = notifications.filter((n) => !n.read).length;

  const markRead = useCallback((id: string) => {
    setNotifications((prev) =>
      prev.map((n) => (n.id === id ? { ...n, read: true } : n))
    );
  }, []);

  return { notifications, unreadCount, markRead, sseStatus: status };
}
