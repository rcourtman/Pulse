import type { JSX } from 'solid-js';

export const SUMMARY_ROW_ACTION_BUTTON_FOCUS_CLASS =
  'focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-sky-400/70';

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

interface SummaryInteractiveRowPreviewOptions {
  onPreview?: () => void;
  onPreviewClear?: () => void;
}

export const createSummaryInteractiveRowPreviewHandlers = (
  options: SummaryInteractiveRowPreviewOptions,
): Pick<
  JSX.HTMLAttributes<HTMLElement>,
  'onFocusIn' | 'onFocusOut' | 'onPointerEnter' | 'onPointerLeave'
> => ({
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
});

interface SummaryInteractiveActionKeydownOptions {
  onAction?: () => void;
  onPreviewClear?: () => void;
}

export const createSummaryInteractiveActionKeydownHandler = (
  options: SummaryInteractiveActionKeydownOptions,
): JSX.EventHandlerUnion<HTMLButtonElement, KeyboardEvent> => (event) => {
  if (
    event.key === 'Enter' ||
    event.key === ' ' ||
    event.key === 'Space' ||
    event.code === 'Space'
  ) {
    event.preventDefault();
    options.onAction?.();
    return;
  }

  if (event.key !== 'Escape') {
    return;
  }
  event.preventDefault();
  options.onPreviewClear?.();
  event.currentTarget.blur();
};

export const buildSummaryDisclosureControlsId = (seriesId: string): string =>
  `summary-row-detail-${seriesId.replace(/[^a-zA-Z0-9_-]+/g, '-')}`;
