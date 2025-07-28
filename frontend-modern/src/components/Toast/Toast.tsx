import { Component, createSignal, For, Show } from 'solid-js';
import { Portal } from 'solid-js/web';
import { POLLING_INTERVALS, ANIMATIONS } from '@/constants';

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

const Toast: Component<ToastProps> = (props) => {
  const [show, setShow] = createSignal(true);

  const icons = {
    success: (
      <svg class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M5 13l4 4L19 7" />
      </svg>
    ),
    error: (
      <svg class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" />
      </svg>
    ),
    warning: (
      <svg class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z" />
      </svg>
    ),
    info: (
      <svg class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M13 16h-1v-4h-1m1-4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
      </svg>
    )
  };

  const colors = {
    success: 'bg-green-50 dark:bg-green-900/20 text-green-800 dark:text-green-200 border-green-200 dark:border-green-800',
    error: 'bg-red-50 dark:bg-red-900/20 text-red-800 dark:text-red-200 border-red-200 dark:border-red-800',
    warning: 'bg-yellow-50 dark:bg-yellow-900/20 text-yellow-800 dark:text-yellow-200 border-yellow-200 dark:border-yellow-800',
    info: 'bg-blue-50 dark:bg-blue-900/20 text-blue-800 dark:text-blue-200 border-blue-200 dark:border-blue-800'
  };

  const iconColors = {
    success: 'text-green-500',
    error: 'text-red-500',
    warning: 'text-yellow-500',
    info: 'text-blue-500'
  };

  // Auto-remove after duration
  window.setTimeout(() => {
    setShow(false);
    window.setTimeout(() => props.onRemove(props.toast.id), ANIMATIONS.TOAST_SLIDE);
  }, props.toast.duration || POLLING_INTERVALS.TOAST_DURATION);

  return (
    <div
      class={`transform transition-[transform,opacity] duration-300 ${
        show() ? 'translate-x-0 opacity-100' : 'translate-x-full opacity-0'
      }`}
    >
      <div class={`flex items-start gap-3 p-4 border rounded-lg shadow-lg ${colors[props.toast.type]}`}>
        <div class={`flex-shrink-0 ${iconColors[props.toast.type]}`}>
          {icons[props.toast.type]}
        </div>
        <div class="flex-1">
          <h3 class="text-sm font-medium">{props.toast.title}</h3>
          <Show when={props.toast.message}>
            <p class="mt-1 text-xs opacity-90">{props.toast.message}</p>
          </Show>
        </div>
        <button
          onClick={() => {
            setShow(false);
            window.setTimeout(() => props.onRemove(props.toast.id), ANIMATIONS.TOAST_SLIDE);
          }}
          class="flex-shrink-0 ml-2 hover:opacity-70 transition-opacity"
        >
          <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" />
          </svg>
        </button>
      </div>
    </div>
  );
};

// Toast Container Component
export const ToastContainer: Component = () => {
  const [toasts, setToasts] = createSignal<ToastMessage[]>([]);

  const removeToast = (id: string) => {
    setToasts(toasts().filter(t => t.id !== id));
  };

  // Expose global toast function
  (window as any).showToast = (type: ToastType, title: string, message?: string, duration?: number) => {
    const id = Date.now().toString();
    setToasts([...toasts(), { id, type, title, message, duration }]);
  };

  return (
    <Portal>
      <div class="fixed top-4 right-4 z-50 space-y-2 max-w-sm">
        <For each={toasts()}>
          {(toast) => <Toast toast={toast} onRemove={removeToast} />}
        </For>
      </div>
    </Portal>
  );
};