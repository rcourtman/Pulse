import { showToast } from '@/utils/toast';

const DEFAULT_DURATIONS: Record<'success' | 'error' | 'info', number> = {
  success: 5000,
  error: 10000,
  info: 5000,
};

export const notificationStore = {
  success: (message: string, duration: number = DEFAULT_DURATIONS.success) => {
    return showToast('success', message, undefined, duration);
  },

  error: (message: string, duration: number = DEFAULT_DURATIONS.error) => {
    return showToast('error', message, undefined, duration);
  },

  info: (message: string, duration: number = DEFAULT_DURATIONS.info) => {
    return showToast('info', message, undefined, duration);
  },

  warning: (message: string, duration: number = DEFAULT_DURATIONS.info) => {
    return showToast('warning', message, undefined, duration);
  },
};
