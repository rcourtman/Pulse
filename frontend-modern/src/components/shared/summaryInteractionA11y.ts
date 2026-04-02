import type { JSX } from 'solid-js';

export const SUMMARY_INTERACTIVE_ROW_FOCUS_CLASS =
  'focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-sky-400/70 focus-visible:ring-inset';

const isFinePointerEvent = (event: PointerEvent): boolean => {
  const pointerType = typeof event.pointerType === 'string' ? event.pointerType : '';
  if (pointerType === 'mouse' || pointerType === 'pen') {
    return true;
  }
  if (pointerType === 'touch') {
    return false;
  }
  if (typeof window !== 'undefined' && typeof window.matchMedia === 'function') {
    return window.matchMedia('(pointer: fine)').matches;
  }
  return true;
};

const isCurrentTargetKeyboardEvent = (event: KeyboardEvent): boolean =>
  event.target === event.currentTarget;

interface SummaryInteractiveRowHandlersOptions {
  onPreview?: () => void;
  onPreviewClear?: () => void;
  onToggle?: () => void;
}

export const createSummaryInteractiveRowHandlers = (
  options: SummaryInteractiveRowHandlersOptions,
): Pick<
  JSX.HTMLAttributes<HTMLElement>,
  'onFocusIn' | 'onFocusOut' | 'onKeyDown' | 'onPointerEnter' | 'onPointerLeave' | 'tabIndex'
> => ({
  tabIndex: 0,
  onPointerEnter: (event) => {
    if (isFinePointerEvent(event)) {
      options.onPreview?.();
    }
  },
  onPointerLeave: (event) => {
    if (isFinePointerEvent(event)) {
      options.onPreviewClear?.();
    }
  },
  onFocusIn: () => {
    options.onPreview?.();
  },
  onFocusOut: (event) => {
    const nextTarget = event.relatedTarget;
    if (nextTarget instanceof Node && event.currentTarget.contains(nextTarget)) {
      return;
    }
    options.onPreviewClear?.();
  },
  onKeyDown: (event) => {
    if (!isCurrentTargetKeyboardEvent(event)) {
      return;
    }

    if (event.key === 'Enter' || event.key === ' ') {
      event.preventDefault();
      options.onToggle?.();
      return;
    }

    if (event.key === 'Escape') {
      event.preventDefault();
      options.onPreviewClear?.();
      event.currentTarget.blur();
    }
  },
});
