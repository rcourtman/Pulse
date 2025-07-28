import type { ToastType } from '@/components/Toast/Toast';

export const showToast = (type: ToastType, title: string, message?: string, duration?: number) => {
  // Use the global toast function exposed by ToastContainer
  if (typeof window !== 'undefined' && (window as any).showToast) {
    (window as any).showToast(type, title, message, duration);
  } else {
    // Fallback to console if toast system not ready
    console.log(`[${type.toUpperCase()}] ${title}${message ? ': ' + message : ''}`);
  }
};

// Convenience functions - only export what's used
export const showSuccess = (title: string, message?: string) => showToast('success', title, message);
export const showError = (title: string, message?: string) => showToast('error', title, message);