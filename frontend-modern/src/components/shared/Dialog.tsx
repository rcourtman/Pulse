import { Show, createEffect, onCleanup } from 'solid-js';
import type { Component, JSX } from 'solid-js';
import { Portal } from 'solid-js/web';

interface DialogProps {
  isOpen: boolean;
  onClose: () => void;
  children: JSX.Element;
  panelClass?: string;
  layout?: 'modal' | 'drawer-right';
  closeOnBackdrop?: boolean;
  ariaLabel?: string;
  ariaLabelledBy?: string;
  ariaDescribedBy?: string;
}

const FOCUSABLE_SELECTOR =
  'a[href],area[href],button:not([disabled]),input:not([disabled]):not([type="hidden"]),select:not([disabled]),textarea:not([disabled]),[tabindex]:not([tabindex="-1"])';

let openDialogCount = 0;
let previousBodyOverflow = '';

const lockBodyScroll = () => {
  if (typeof document === 'undefined') return;
  if (openDialogCount === 0) {
    previousBodyOverflow = document.body.style.overflow;
    document.body.style.overflow = 'hidden';
  }
  openDialogCount += 1;
};

const unlockBodyScroll = () => {
  if (typeof document === 'undefined') return;
  openDialogCount = Math.max(0, openDialogCount - 1);
  if (openDialogCount === 0) {
    document.body.style.overflow = previousBodyOverflow;
  }
};

const getFocusableElements = (container: HTMLElement): HTMLElement[] =>
  Array.from(container.querySelectorAll<HTMLElement>(FOCUSABLE_SELECTOR)).filter(
    (element) =>
      !element.hasAttribute('disabled') && element.getAttribute('aria-hidden') !== 'true',
  );

export const Dialog: Component<DialogProps> = (props) => {
  let panelRef: HTMLDivElement | undefined;

  createEffect(() => {
    if (!props.isOpen || typeof document === 'undefined') return;

    const previousFocus =
      document.activeElement instanceof HTMLElement ? document.activeElement : null;
    lockBodyScroll();

    queueMicrotask(() => {
      if (!panelRef) return;
      const focusable = getFocusableElements(panelRef);
      if (focusable.length > 0) {
        focusable[0].focus();
        return;
      }
      panelRef.focus();
    });

    const onKeyDown = (event: KeyboardEvent) => {
      if (!panelRef) return;
      if (event.key === 'Escape') {
        event.preventDefault();
        props.onClose();
        return;
      }
      if (event.key !== 'Tab') return;

      const focusable = getFocusableElements(panelRef);
      if (focusable.length === 0) {
        event.preventDefault();
        panelRef.focus();
        return;
      }

      const first = focusable[0];
      const last = focusable[focusable.length - 1];
      const active = document.activeElement as HTMLElement | null;
      const isOutside = !active || !panelRef.contains(active);

      if (event.shiftKey) {
        if (active === first || isOutside) {
          event.preventDefault();
          last.focus();
        }
        return;
      }

      if (active === last || isOutside) {
        event.preventDefault();
        first.focus();
      }
    };

    document.addEventListener('keydown', onKeyDown);
    onCleanup(() => {
      document.removeEventListener('keydown', onKeyDown);
      unlockBodyScroll();
      if (previousFocus && document.contains(previousFocus)) {
        previousFocus.focus();
      }
    });
  });

  const handleBackdropClick = () => {
    if (props.closeOnBackdrop ?? true) {
      props.onClose();
    }
  };

  const layout = () => props.layout ?? 'modal';

  return (
    <Show when={props.isOpen}>
      <Portal mount={document.body}>
        <div class="fixed inset-0 z-[1000]">
          <div
            class="absolute inset-0 bg-black/60 backdrop-blur-sm transition-opacity duration-300"
            data-dialog-backdrop
            onClick={handleBackdropClick}
          />
          <div
            class={`relative h-full overflow-y-auto pointer-events-none ${
              layout() === 'drawer-right' ? 'p-0' : 'p-4 sm:p-6'
            }`}
          >
            <div
              class={`flex min-h-full ${
                layout() === 'drawer-right'
                  ? 'items-stretch justify-end'
                  : 'items-start justify-center sm:items-center'
              }`}
            >
              <div
                ref={(el) => {
                  panelRef = el;
                }}
                role="dialog"
                aria-modal="true"
                aria-label={props.ariaLabel}
                aria-labelledby={props.ariaLabelledBy}
                aria-describedby={props.ariaDescribedBy}
                tabindex="-1"
                class={`relative flex w-full flex-col overflow-hidden bg-surface border border-border outline-none pointer-events-auto ${
                  layout() === 'drawer-right'
                    ? 'h-dvh max-w-[720px] rounded-none border-y-0 border-r-0 animate-slide-up sm:h-full sm:max-h-dvh sm:rounded-l-xl sm:border-y sm:border-r-0'
                    : 'max-h-[calc(100dvh-2rem)] rounded-md animate-slide-up'
                } ${
                  props.panelClass ?? (layout() === 'drawer-right' ? '' : 'max-w-lg')
                }`.trim()}
                onClick={(event) => event.stopPropagation()}
              >
                {props.children}
              </div>
            </div>
          </div>
        </div>
      </Portal>
    </Show>
  );
};

export default Dialog;
