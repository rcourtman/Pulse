import { Component, createSignal, For, Show, onCleanup, onMount } from 'solid-js';
import { Portal } from 'solid-js/web';
import AlertTriangleIcon from 'lucide-solid/icons/alert-triangle';
import CheckCircleIcon from 'lucide-solid/icons/check-circle';
import CircleAlertIcon from 'lucide-solid/icons/circle-alert';
import InfoIcon from 'lucide-solid/icons/info';
import XIcon from 'lucide-solid/icons/x';
import { ActionIconButton } from '@/components/shared/Button';
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
  let autoRemoveStartedAt = 0;
  let autoRemoveRemaining = props.toast.duration || POLLING_INTERVALS.TOAST_DURATION;
  let pointerIsInside = false;
  let focusIsInside = false;

  const icon = () => {
    const iconClass = 'h-5 w-5';
    switch (props.toast.type) {
      case 'success':
        return <CheckCircleIcon class={iconClass} aria-hidden="true" />;
      case 'error':
        return <CircleAlertIcon class={iconClass} aria-hidden="true" />;
      case 'warning':
        return <AlertTriangleIcon class={iconClass} aria-hidden="true" />;
      case 'info':
      default:
        return <InfoIcon class={iconClass} aria-hidden="true" />;
    }
  };

  const iconTone = () => getSemanticTonePresentation(props.toast.type);

  const clearAutoRemoveTimer = (preserveRemaining: boolean) => {
    if (autoRemoveTimer === undefined) return;
    window.clearTimeout(autoRemoveTimer);
    autoRemoveTimer = undefined;
    if (preserveRemaining) {
      autoRemoveRemaining = Math.max(0, autoRemoveRemaining - (Date.now() - autoRemoveStartedAt));
    }
  };

  const startAutoRemoveTimer = () => {
    if (!show() || autoRemoveTimer !== undefined || autoRemoveRemaining <= 0) return;
    autoRemoveStartedAt = Date.now();
    autoRemoveTimer = window.setTimeout(() => handleClose(), autoRemoveRemaining);
  };

  // Keep timed feedback available while a user is reading or operating it,
  // then continue from the remaining duration instead of restarting the clock.
  const pauseAutoRemove = () => clearAutoRemoveTimer(true);

  const resumeAutoRemove = () => {
    if (!pointerIsInside && !focusIsInside) startAutoRemoveTimer();
  };

  const handleClose = () => {
    if (!show()) {
      return;
    }
    setShow(false);
    clearAutoRemoveTimer(false);
    if (closeAnimationTimer !== undefined) {
      window.clearTimeout(closeAnimationTimer);
    }
    closeAnimationTimer = window.setTimeout(() => props.onRemove(props.toast.id), 300);
  };

  onMount(() => {
    startAutoRemoveTimer();
  });

  onCleanup(() => {
    if (autoRemoveTimer !== undefined) {
      window.clearTimeout(autoRemoveTimer);
    }
    if (closeAnimationTimer !== undefined) {
      window.clearTimeout(closeAnimationTimer);
    }
  });

  const isAlert = () => props.toast.type === 'error' || props.toast.type === 'warning';

  return (
    <div
      role={isAlert() ? 'alert' : 'status'}
      aria-live={isAlert() ? 'assertive' : 'polite'}
      aria-atomic="true"
      onPointerEnter={() => {
        pointerIsInside = true;
        pauseAutoRemove();
      }}
      onPointerLeave={() => {
        pointerIsInside = false;
        resumeAutoRemove();
      }}
      onFocusIn={() => {
        focusIsInside = true;
        pauseAutoRemove();
      }}
      onFocusOut={(event) => {
        if (
          event.relatedTarget instanceof Node &&
          event.currentTarget.contains(event.relatedTarget)
        ) {
          return;
        }
        focusIsInside = false;
        resumeAutoRemove();
      }}
      class={`transform transition-all duration-500 ease-out ${
        show() ? 'translate-x-0 opacity-100 scale-100' : 'translate-x-full opacity-0 scale-95'
      } animate-slide-in-card`}
    >
      <div
        class={`
           bg-surface
          border border-border
          px-4 py-3 sm:px-5 sm:py-4 rounded-md shadow-sm
          flex w-full min-w-0 items-start gap-3 sm:gap-4
        `}
      >
        <div
          class={`flex-shrink-0 flex items-center justify-center p-1.5 sm:p-2 rounded-md border bg-surface ${iconTone().iconClass} ${iconTone().panelClass}`}
        >
          {icon()}
        </div>
        <div class="min-w-0 flex-1">
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
        <ActionIconButton
          type="button"
          onClick={handleClose}
          label="Dismiss notification"
          title="Dismiss"
          tone="muted"
          size="sm"
          class="flex-shrink-0"
        >
          <XIcon class="h-4 w-4" aria-hidden="true" />
        </ActionIconButton>
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
      <div
        role="region"
        aria-label="Notifications"
        class="fixed inset-x-3 bottom-3 z-[9999] space-y-2 sm:left-auto sm:right-4 sm:bottom-4 sm:w-full sm:max-w-[500px]"
      >
        <For each={toasts()}>{(toast) => <Toast toast={toast} onRemove={removeToast} />}</For>
      </div>
    </Portal>
  );
};
