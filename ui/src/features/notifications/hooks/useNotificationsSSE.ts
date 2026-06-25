import { useEffect, useRef } from 'react';
import { useAuthStore } from '@/features/auth/store/authStore';
import { useNotificationStore } from '../store/notificationStore';
import { toast } from 'sonner';
import type { NotificationEvent } from '../store/notificationStore';
import { ENDPOINTS } from '@/shared/api/endpoints';

export type SSEStatus = 'connecting' | 'open' | 'closed' | 'error';

export function useNotificationsSSE() {
  const { accessToken, isAuthenticated } = useAuthStore();
  const { addNotification } = useNotificationStore();
  const sourceRef = useRef<EventSource | null>(null);
  const statusRef = useRef<SSEStatus>('closed');

  useEffect(() => {
    if (!isAuthenticated || !accessToken) return;

    // SSE không hỗ trợ Authorization header → dùng ?token= query param
    const url = `${ENDPOINTS.notifications.stream}?token=${encodeURIComponent(accessToken)}`;
    const source = new EventSource(url, { withCredentials: true });
    sourceRef.current = source;
    statusRef.current = 'connecting';

    source.onopen = () => { statusRef.current = 'open'; };

    source.addEventListener('notification', (e: MessageEvent) => {
      const event = JSON.parse(e.data) as NotificationEvent;
      addNotification(event);

      // Toast cho events quan trọng
      if (event.type === 'finding.sla.breached' || event.type === 'kev.new') {
        toast.error(event.title, { duration: 8000 });
      } else if (event.type === 'scan.completed') {
        toast.success(event.title, { duration: 5000 });
      }
    });

    source.addEventListener('ping', () => { /* keep-alive — no-op */ });

    source.onerror = () => {
      statusRef.current = 'error';
      source.close();
    };

    return () => {
      source.close();
      statusRef.current = 'closed';
    };
  }, [isAuthenticated, accessToken, addNotification]);

  return {
    isConnected: statusRef.current === 'open',
  };
}
