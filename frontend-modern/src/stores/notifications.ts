import { createStore } from 'solid-js/store';

export interface Notification {
  id: string;
  message: string;
  type: 'success' | 'error' | 'info';
  duration?: number;
}

const [notifications, setNotifications] = createStore<Notification[]>([]);

export const notificationStore = {
  notifications,

  add: (notification: Omit<Notification, 'id'>) => {
    const id = Date.now().toString();
    setNotifications((prev) => [...prev, { ...notification, id }]);

    // Auto-remove after duration
    if (notification.duration && notification.duration > 0) {
      setTimeout(() => {
        notificationStore.remove(id);
      }, notification.duration);
    }

    return id;
  },

  remove: (id: string) => {
    setNotifications((prev) => prev.filter((n) => n.id !== id));
  },

  success: (message: string, duration = 5000) => {
    return notificationStore.add({ message, type: 'success', duration });
  },

  error: (message: string, duration = 10000) => {
    return notificationStore.add({ message, type: 'error', duration });
  },

  info: (message: string, duration = 5000) => {
    return notificationStore.add({ message, type: 'info', duration });
  },
};
