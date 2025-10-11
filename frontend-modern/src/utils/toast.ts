import type { ToastType } from '@/components/Toast/Toast';

// Global declaration is in Toast.tsx

export const showToast = (type: ToastType, title: string, message?: string, duration?: number) => {
  // Use the global toast function exposed by ToastContainer
  if (typeof window !== 'undefined' && window.showToast) {
    window.showToast(type, title, message, duration);
  } else {
    // Fallback to console if toast system not ready
    console.log(`[${type.toUpperCase()}] ${title}${message ? ': ' + message : ''}`);
  }
};

// Convenience functions - only export what's used
export const showSuccess = (title: string, message?: string) =>
  showToast('success', title, message);
export const showError = (title: string, message?: string) => showToast('error', title, message);
export const showInfo = (title: string, message?: string) => showToast('info', title, message);
