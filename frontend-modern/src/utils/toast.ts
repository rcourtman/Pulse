import type { ToastType } from '@/components/Toast/Toast';
import { logger } from '@/utils/logger';

// Global declaration is in Toast.tsx

export const showToast = (
  type: ToastType,
  title: string,
  message?: string,
  duration?: number,
): string | undefined => {
  if (typeof window !== 'undefined' && window.showToast) {
    return window.showToast(type, title, message, duration);
  }

  // Fallback to console if toast system not ready
  logger.info(`[toast:${type}] ${title}${message ? `: ${message}` : ''}`);
  return undefined;
};

// Convenience functions - only export what's used
export const showSuccess = (title: string, message?: string, duration?: number) =>
  showToast('success', title, message, duration);
export const showError = (title: string, message?: string, duration?: number) =>
  showToast('error', title, message, duration);
