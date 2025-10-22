import type { ToastType } from '@/components/Toast/Toast';

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
  console.log(`[${type.toUpperCase()}] ${title}${message ? `: ${message}` : ''}`);
  return undefined;
};

// Convenience functions - only export what's used
export const showSuccess = (title: string, message?: string, duration?: number) =>
  showToast('success', title, message, duration);
export const showError = (title: string, message?: string, duration?: number) =>
  showToast('error', title, message, duration);
export const showInfo = (title: string, message?: string, duration?: number) =>
  showToast('info', title, message, duration);
