import { create } from 'zustand';

export interface NotificationEvent {
  type: string;
  title: string;
  severity?: string;
  entity_id?: string;
  timestamp: string;
}

interface NotificationState {
  notifications: NotificationEvent[];
  unreadCount: number;
  addNotification: (n: NotificationEvent) => void;
  markAllRead:     () => void;
  clearAll:        () => void;
}

export const useNotificationStore = create<NotificationState>((set) => ({
  notifications: [],
  unreadCount:   0,

  addNotification: (notification) =>
    set((state) => ({
      notifications: [notification, ...state.notifications].slice(0, 50),
      unreadCount:   state.unreadCount + 1,
    })),

  markAllRead: () => set({ unreadCount: 0 }),
  clearAll:    () => set({ notifications: [], unreadCount: 0 }),
}));
