import { Show, createEffect, onCleanup } from 'solid-js';
import type { Component, JSX } from 'solid-js';
import { Portal } from 'solid-js/web';

interface DialogProps {
  isOpen: boolean;
  onClose: () => void;
  children: JSX.Element;
  panelClass?: string;
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
    (element) => !element.hasAttribute('disabled') && element.getAttribute('aria-hidden') !== 'true'
  );

export const Dialog: Component<DialogProps> = (props) => {
  let panelRef: HTMLDivElement | undefined;

  createEffect(() => {
    if (!props.isOpen || typeof document === 'undefined') return;

    const previousFocus = document.activeElement instanceof HTMLElement ? document.activeElement : null;
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

  return (
    <Show when={props.isOpen}>
      <Portal mount={document.body}>
        <div class="fixed inset-0 z-[1000]">
          <div
            class="absolute inset-0 bg-slate-900/40 backdrop-blur-sm transition-opacity duration-300"
            data-dialog-backdrop
            onClick={handleBackdropClick}
          />
          <div class="relative h-full overflow-y-auto p-4 sm:p-6 pointer-events-none">
            <div class="flex min-h-full items-start justify-center sm:items-center">
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
                data-dialog-panel
                class={`relative flex w-full max-h-[calc(100dvh-2rem)] flex-col overflow-hidden rounded-xl bg-white/95 dark:bg-slate-900/90 backdrop-blur-xl border border-white/20 dark:border-white/10 shadow-2xl outline-none pointer-events-auto animate-slide-up ${props.panelClass ?? 'max-w-lg'
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
