import { createEffect, onCleanup } from 'solid-js';
import type { Accessor } from 'solid-js';
import {
  getDialogFocusableElements,
  getDialogLayout,
  type DialogLayout,
} from './dialogModel';

interface DialogStateOptions {
  closeOnBackdrop?: boolean;
  isOpen: boolean;
  layout?: DialogLayout;
  onClose: () => void;
}

let openDialogCount = 0;
let previousBodyOverflow = '';

function lockBodyScroll() {
  if (typeof document === 'undefined') return;
  if (openDialogCount === 0) {
    previousBodyOverflow = document.body.style.overflow;
    document.body.style.overflow = 'hidden';
  }
  openDialogCount += 1;
}

function unlockBodyScroll() {
  if (typeof document === 'undefined') return;
  openDialogCount = Math.max(0, openDialogCount - 1);
  if (openDialogCount === 0) {
    document.body.style.overflow = previousBodyOverflow;
  }
}

export function useDialogState(options: DialogStateOptions): {
  handleBackdropClick: () => void;
  layout: Accessor<DialogLayout>;
  setPanelRef: (el: HTMLDivElement) => void;
} {
  let panelRef: HTMLDivElement | undefined;

  createEffect(() => {
    if (!options.isOpen || typeof document === 'undefined') return;

    const previousFocus =
      document.activeElement instanceof HTMLElement ? document.activeElement : null;
    lockBodyScroll();

    queueMicrotask(() => {
      if (!panelRef) return;
      const focusable = getDialogFocusableElements(panelRef);
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
        options.onClose();
        return;
      }
      if (event.key !== 'Tab') return;

      const focusable = getDialogFocusableElements(panelRef);
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

  return {
    handleBackdropClick: () => {
      if (options.closeOnBackdrop ?? true) {
        options.onClose();
      }
    },
    layout: () => getDialogLayout(options.layout),
    setPanelRef: (el) => {
      panelRef = el;
    },
  };
}
