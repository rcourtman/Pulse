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
interface DialogStackEntry {
  id: symbol;
  layer?: HTMLElement;
}

const dialogStack: DialogStackEntry[] = [];
// Solid portals are body-level siblings of the app and of one another. Preserve
// each sibling's existing state so nested dialogs can isolate the active layer
// without permanently changing surfaces owned by another feature.
const backgroundInertState = new Map<HTMLElement, boolean>();
let bodyObserver: MutationObserver | undefined;

export function dialogStackHasBlockingDialog() {
  return dialogStackDepth() > 0;
}

function getBodyChildContaining(element: HTMLElement | undefined): HTMLElement | undefined {
  if (!element || typeof document === 'undefined' || !document.body.contains(element)) {
    return undefined;
  }

  let bodyChild = element;
  while (bodyChild.parentElement && bodyChild.parentElement !== document.body) {
    bodyChild = bodyChild.parentElement;
  }
  return bodyChild.parentElement === document.body ? bodyChild : undefined;
}

function syncBackgroundInertness() {
  if (typeof document === 'undefined') return;

  const topmostLayer = dialogStack[dialogStack.length - 1]?.layer;
  const activeBodyChild = getBodyChildContaining(topmostLayer);

  for (const child of Array.from(document.body.children)) {
    if (!(child instanceof HTMLElement)) continue;
    if (!backgroundInertState.has(child)) {
      backgroundInertState.set(child, child.hasAttribute('inert'));
    }
    child.toggleAttribute('inert', child !== activeBodyChild);
  }
}

function restoreBackgroundInertness() {
  bodyObserver?.disconnect();
  bodyObserver = undefined;
  for (const [element, wasInert] of backgroundInertState) {
    element.toggleAttribute('inert', wasInert);
  }
  backgroundInertState.clear();
}

function lockBodyScroll(dialogId: symbol, layer: HTMLElement | undefined) {
  if (typeof document === 'undefined') return;
  if (openDialogCount === 0) {
    previousBodyOverflow = document.body.style.overflow;
    document.body.style.overflow = 'hidden';
    bodyObserver = new MutationObserver(syncBackgroundInertness);
    bodyObserver.observe(document.body, { childList: true });
  }
  dialogStack.push({ id: dialogId, layer });
  openDialogCount = dialogStack.length;
  setDialogStackDepth(openDialogCount);
  syncBackgroundInertness();
}

function unlockBodyScroll(dialogId: symbol) {
  if (typeof document === 'undefined') return;
  const stackIndex = dialogStack.findIndex((entry) => entry.id === dialogId);
  if (stackIndex !== -1) dialogStack.splice(stackIndex, 1);
  openDialogCount = dialogStack.length;
  setDialogStackDepth(openDialogCount);
  if (openDialogCount === 0) {
    document.body.style.overflow = previousBodyOverflow;
    restoreBackgroundInertness();
  } else {
    syncBackgroundInertness();
  }
}

function isTopmostDialog(dialogId: symbol) {
  return dialogStack[dialogStack.length - 1]?.id === dialogId;
}

export function useDialogState(options: DialogStateOptions): {
  handleBackdropClick: () => void;
  layout: Accessor<DialogLayout>;
  setLayerRef: (el: HTMLDivElement) => void;
  setPanelRef: (el: HTMLDivElement) => void;
} {
  let layerRef: HTMLDivElement | undefined;
  let panelRef: HTMLDivElement | undefined;
  const dialogId = Symbol('dialog');

  createEffect(() => {
    if (!options.isOpen || typeof document === 'undefined') return;

    const previousFocus =
      document.activeElement instanceof HTMLElement ? document.activeElement : null;
    lockBodyScroll(dialogId, layerRef);

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
    setLayerRef: (el) => {
      layerRef = el;
      const entry = dialogStack.find((candidate) => candidate.id === dialogId);
      if (entry) {
        entry.layer = el;
        syncBackgroundInertness();
      }
    },
    setPanelRef: (el) => {
      panelRef = el;
    },
  };
}
