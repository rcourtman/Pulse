import { createEffect, createSignal, onCleanup } from 'solid-js';
import type { Accessor } from 'solid-js';
import { getDialogFocusableElements, getDialogLayout, type DialogLayout } from './dialogModel';

interface DialogStateOptions {
  closeOnBackdrop?: boolean;
  isOpen: boolean;
  layout?: DialogLayout;
  onClose: () => void;
}

let openDialogCount = 0;
let previousBodyOverflow = '';
const [dialogStackDepth, setDialogStackDepth] = createSignal(0);
const dialogStack: symbol[] = [];

export function dialogStackHasBlockingDialog() {
  return dialogStackDepth() > 0;
}

function lockBodyScroll(dialogId: symbol) {
  if (typeof document === 'undefined') return;
  if (openDialogCount === 0) {
    previousBodyOverflow = document.body.style.overflow;
    document.body.style.overflow = 'hidden';
  }
  dialogStack.push(dialogId);
  openDialogCount = dialogStack.length;
  setDialogStackDepth(openDialogCount);
}

function unlockBodyScroll(dialogId: symbol) {
  if (typeof document === 'undefined') return;
  const stackIndex = dialogStack.lastIndexOf(dialogId);
  if (stackIndex !== -1) dialogStack.splice(stackIndex, 1);
  openDialogCount = dialogStack.length;
  setDialogStackDepth(openDialogCount);
  if (openDialogCount === 0) {
    document.body.style.overflow = previousBodyOverflow;
  }
}

function isTopmostDialog(dialogId: symbol) {
  return dialogStack[dialogStack.length - 1] === dialogId;
}

export function useDialogState(options: DialogStateOptions): {
  handleBackdropClick: () => void;
  layout: Accessor<DialogLayout>;
  setPanelRef: (el: HTMLDivElement) => void;
} {
  let panelRef: HTMLDivElement | undefined;
  const dialogId = Symbol('dialog');

  createEffect(() => {
    if (!options.isOpen || typeof document === 'undefined') return;

    const previousFocus =
      document.activeElement instanceof HTMLElement ? document.activeElement : null;
    lockBodyScroll(dialogId);

    queueMicrotask(() => {
      if (!panelRef || !document.contains(panelRef) || !isTopmostDialog(dialogId)) return;
      const focusable = getDialogFocusableElements(panelRef);
      const requestedInitialFocus = focusable.find((element) => element.hasAttribute('autofocus'));
      if (requestedInitialFocus) {
        requestedInitialFocus.focus();
        return;
      }
      if (focusable.length > 0) {
        focusable[0].focus();
        return;
      }
      panelRef.focus();
    });

    const onKeyDown = (event: KeyboardEvent) => {
      if (!panelRef || !isTopmostDialog(dialogId)) return;
      if (event.key === 'Escape') {
        event.preventDefault();
        event.stopImmediatePropagation();
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
      const shouldRestoreFocus = isTopmostDialog(dialogId);
      document.removeEventListener('keydown', onKeyDown);
      unlockBodyScroll(dialogId);
      if (shouldRestoreFocus && previousFocus && document.contains(previousFocus)) {
        previousFocus.focus({ preventScroll: true });
      }
    });
  });

  return {
    handleBackdropClick: () => {
      if (isTopmostDialog(dialogId) && (options.closeOnBackdrop ?? true)) {
        options.onClose();
      }
    },
    layout: () => getDialogLayout(options.layout),
    setPanelRef: (el) => {
      panelRef = el;
    },
  };
}
