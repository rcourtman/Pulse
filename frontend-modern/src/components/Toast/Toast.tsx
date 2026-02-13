import { Component, createSignal, For, Show, onCleanup, onMount } from 'solid-js';
import { Portal } from 'solid-js/web';
import { POLLING_INTERVALS } from '@/constants';

export type ToastType = 'success' | 'error' | 'warning' | 'info';

export interface ToastMessage {
  id: string;
  type: ToastType;
  title: string;
  message?: string;
  duration?: number;
}

interface ToastProps {
  toast: ToastMessage;
  onRemove: (id: string) => void;
}

export const Toast: Component<ToastProps> = (props) => {
  const [show, setShow] = createSignal(true);
  let autoRemoveTimer: number | undefined;
  let closeAnimationTimer: number | undefined;

  const icons = {
    success: (
      <div class="relative">
        <div class="absolute inset-0 bg-green-400 rounded-full blur-xl opacity-50 animate-pulse"></div>
        <svg class="w-6 h-6 relative" fill="none" stroke="currentColor" viewBox="0 0 24 24">
          <path
            stroke-linecap="round"
            stroke-linejoin="round"
            stroke-width="2.5"
            d="M9 12l2 2 4-4m6 2a9 9 0 11-18 0 9 9 0 0118 0z"
          />
        </svg>
      </div>
    ),
    error: (
      <div class="relative">
        <div class="absolute inset-0 bg-red-400 rounded-full blur-xl opacity-50 animate-pulse"></div>
        <svg class="w-6 h-6 relative" fill="none" stroke="currentColor" viewBox="0 0 24 24">
          <path
            stroke-linecap="round"
            stroke-linejoin="round"
            stroke-width="2.5"
            d="M12 8v4m0 4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z"
          />
        </svg>
      </div>
    ),
    warning: (
      <div class="relative">
        <div class="absolute inset-0 bg-yellow-400 rounded-full blur-xl opacity-50 animate-pulse"></div>
        <svg class="w-6 h-6 relative" fill="none" stroke="currentColor" viewBox="0 0 24 24">
          <path
            stroke-linecap="round"
            stroke-linejoin="round"
            stroke-width="2.5"
            d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z"
          />
        </svg>
      </div>
    ),
    info: (
      <div class="relative">
        <div class="absolute inset-0 bg-blue-400 rounded-full blur-xl opacity-50 animate-pulse"></div>
        <svg class="w-6 h-6 relative" fill="none" stroke="currentColor" viewBox="0 0 24 24">
          <path
            stroke-linecap="round"
            stroke-linejoin="round"
            stroke-width="2.5"
            d="M13 16h-1v-4h-1m1-4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z"
          />
        </svg>
      </div>
    ),
  };

  const iconColors = {
    success: 'text-green-400',
    error: 'text-red-400',
    warning: 'text-yellow-400',
    info: 'text-blue-400',
  };

  const handleClose = () => {
    if (!show()) {
      return;
    }
    setShow(false);
    if (autoRemoveTimer !== undefined) {
      window.clearTimeout(autoRemoveTimer);
      autoRemoveTimer = undefined;
    }
    if (closeAnimationTimer !== undefined) {
      window.clearTimeout(closeAnimationTimer);
    }
    closeAnimationTimer = window.setTimeout(() => props.onRemove(props.toast.id), 300);
  };

  onMount(() => {
    // Auto-remove after duration.
    autoRemoveTimer = window.setTimeout(
      () => handleClose(),
      props.toast.duration || POLLING_INTERVALS.TOAST_DURATION,
    );
  });

  onCleanup(() => {
    if (autoRemoveTimer !== undefined) {
      window.clearTimeout(autoRemoveTimer);
    }
    if (closeAnimationTimer !== undefined) {
      window.clearTimeout(closeAnimationTimer);
    }
  });

  return (
    <div
      class={`transform transition-all duration-500 ease-out ${
        show() ? 'translate-x-0 opacity-100 scale-100' : 'translate-x-full opacity-0 scale-95'
      } animate-slide-in-glass`}
    >
      <div
        class={`
          backdrop-blur-xl bg-white/10 dark:bg-gray-900/30
          border border-white/20 dark:border-gray-700/30
          px-5 py-4 rounded-2xl shadow-2xl 
          flex items-center gap-4 
          min-w-[320px] max-w-[500px]
        `}
        style={{
          background: 'rgba(255,255,255,0.1)',
          'box-shadow':
            '0 8px 32px 0 rgba(31, 38, 135, 0.37), inset 0 0 0 1px rgba(255,255,255,0.1)',
        }}
      >
        <div class={`flex-shrink-0 ${iconColors[props.toast.type]}`}>{icons[props.toast.type]}</div>
        <div class="flex-1">
          <h3 class="text-sm font-medium text-gray-800 dark:text-gray-100">{props.toast.title}</h3>
          <Show when={props.toast.message}>
            <p class="mt-1 text-xs text-gray-700 dark:text-gray-300 opacity-90">
              {props.toast.message}
            </p>
          </Show>
        </div>
        <button
          type="button"
          onClick={handleClose}
          class="flex-shrink-0 text-gray-500 dark:text-gray-400 hover:text-gray-700 dark:hover:text-gray-200 hover:bg-white/10 rounded-lg p-1.5 transition-all duration-200"
        >
          <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path
              stroke-linecap="round"
              stroke-linejoin="round"
              stroke-width="2"
              d="M6 18L18 6M6 6l12 12"
            />
          </svg>
        </button>
      </div>
    </div>
  );
};

// Toast Container Component
// Declare global interface extension
declare global {
  interface Window {
    showToast: (type: ToastType, title: string, message?: string, duration?: number) => string;
  }
}

export const ToastContainer: Component = () => {
  const [toasts, setToasts] = createSignal<ToastMessage[]>([]);

  const removeToast = (id: string) => {
    setToasts((prev) => prev.filter((t) => t.id !== id));
  };

  // Expose global toast function
  window.showToast = (type: ToastType, title: string, message?: string, duration?: number) => {
    const id = (typeof crypto !== 'undefined' && crypto.randomUUID ? crypto.randomUUID() : Date.now().toString());
    setToasts((prev) => [...prev, { id, type, title, message, duration }]);
    return id;
  };

  return (
    <Portal>
      <div class="fixed bottom-4 right-4 z-[9999] space-y-2 max-w-sm">
        <For each={toasts()}>{(toast) => <Toast toast={toast} onRemove={removeToast} />}</For>
      </div>
    </Portal>
  );
};
