import { Component, createSignal, For, Show, onCleanup, onMount } from 'solid-js';
import { Portal } from 'solid-js/web';
import { POLLING_INTERVALS } from '@/constants';
import { getSemanticTonePresentation } from '@/utils/semanticTonePresentation';

export type ToastType = 'success' | 'error' | 'warning' | 'info';

export interface ToastMessage {
  id: string;
  type: ToastType;
  title: string;
  message?: string;
  detail?: string;
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
      <svg class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
        <path
          stroke-linecap="round"
          stroke-linejoin="round"
          stroke-width="2.5"
          d="M9 12l2 2 4-4m6 2a9 9 0 11-18 0 9 9 0 0118 0z"
        />
      </svg>
    ),
    error: (
      <svg class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
        <path
          stroke-linecap="round"
          stroke-linejoin="round"
          stroke-width="2.5"
          d="M12 8v4m0 4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z"
        />
      </svg>
    ),
    warning: (
      <svg class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
        <path
          stroke-linecap="round"
          stroke-linejoin="round"
          stroke-width="2.5"
          d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z"
        />
      </svg>
    ),
    info: (
      <svg class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
        <path
          stroke-linecap="round"
          stroke-linejoin="round"
          stroke-width="2.5"
          d="M13 16h-1v-4h-1m1-4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z"
        />
      </svg>
    ),
  };

  const iconTone = () => getSemanticTonePresentation(props.toast.type);

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
      } animate-slide-in-card`}
    >
      <div
        class={`
           bg-surface
          border border-border
          px-4 py-3 sm:px-5 sm:py-4 rounded-md shadow-sm 
          flex items-center gap-3 sm:gap-4 
          min-w-[300px] max-w-[400px] sm:min-w-[320px] sm:max-w-[500px]
        `}
      >
        <div
          class={`flex-shrink-0 flex items-center justify-center p-1.5 sm:p-2 rounded-md border bg-surface ${iconTone().iconClass} ${iconTone().panelClass}`}
        >
          {icons[props.toast.type]}
        </div>
        <div class="flex-1">
          <h3 class="text-sm font-medium text-base-content">{props.toast.title}</h3>
          <Show when={props.toast.message}>
            <p class="mt-1 text-xs text-base-content opacity-90">{props.toast.message}</p>
          </Show>
          <Show when={props.toast.detail}>
            <details class="mt-1">
              <summary class="text-xs text-muted cursor-pointer select-none hover:text-base-content">
                Details
              </summary>
              <p class="mt-1 text-xs text-base-content/70 break-all">{props.toast.detail}</p>
            </details>
          </Show>
        </div>
        <button
          type="button"
          onClick={handleClose}
          class="flex-shrink-0 text-muted hover:text-base-content hover:bg-surface rounded-md p-1.5 transition-all duration-200"
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
    showToast: (
      type: ToastType,
      title: string,
      message?: string,
      duration?: number,
      detail?: string,
    ) => string;
  }
}

export const ToastContainer: Component = () => {
  const [toasts, setToasts] = createSignal<ToastMessage[]>([]);

  const removeToast = (id: string) => {
    setToasts((prev) => prev.filter((t) => t.id !== id));
  };

  // Expose global toast function
  window.showToast = (
    type: ToastType,
    title: string,
    message?: string,
    duration?: number,
    detail?: string,
  ) => {
    const id =
      typeof crypto !== 'undefined' && crypto.randomUUID
        ? crypto.randomUUID()
        : Date.now().toString();
    setToasts((prev) => [...prev, { id, type, title, message, detail, duration }]);
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
