import { createEffect, createSignal, onCleanup } from 'solid-js';
import type { Accessor } from 'solid-js';

interface SearchTipsPopoverState {
  buttonLabel: Accessor<string>;
  close: () => void;
  closeAndRestoreFocus: () => void;
  handleBlur: (event: FocusEvent) => void;
  handleClick: () => void;
  handleMouseEnter: () => void;
  handleMouseLeave: () => void;
  handlePopoverBlur: (event: FocusEvent) => void;
  isOpen: Accessor<boolean>;
  popoverStyle: Accessor<string | undefined>;
  setCloseButtonRef: (el: HTMLButtonElement) => void;
  setPopoverRef: (el: HTMLDivElement) => void;
  setTriggerRef: (el: HTMLButtonElement) => void;
}

interface SearchTipsPopoverStateOptions {
  buttonLabel: Accessor<string>;
  openOnHover: Accessor<boolean>;
}

export function useSearchTipsPopoverState(
  options: SearchTipsPopoverStateOptions,
): SearchTipsPopoverState {
  const [open, setOpen] = createSignal(false);
  const [popoverStyle, setPopoverStyle] = createSignal<string>();
  let popoverRef: HTMLDivElement | undefined;
  let triggerRef: HTMLButtonElement | undefined;
  let closeButtonRef: HTMLButtonElement | undefined;
  let pointerInside = false;

  const close = () => setOpen(false);
  const closeAndRestoreFocus = () => {
    close();
    triggerRef?.focus({ preventScroll: true });
  };
  const containsFocus = (target: EventTarget | null = document.activeElement) =>
    target instanceof Node &&
    ((popoverRef?.contains(target) ?? false) || (triggerRef?.contains(target) ?? false));
  const focusCloseButton = () => {
    queueMicrotask(() => closeButtonRef?.focus({ preventScroll: true }));
  };
  const updatePopoverPosition = () => {
    if (!triggerRef || !popoverRef || (window.innerWidth >= 1280 && window.innerHeight >= 768)) {
      setPopoverStyle(undefined);
      return;
    }

    const viewportMargin = 16;
    const triggerGap = 8;
    const width = Math.min(288, window.innerWidth - viewportMargin * 2);
    const triggerRect = triggerRef.getBoundingClientRect();
    const mobileNavRect = document
      .querySelector<HTMLElement>('nav[aria-label="Mobile navigation"]')
      ?.getBoundingClientRect();
    const mobileNavTop = mobileNavRect && mobileNavRect.height > 0 ? mobileNavRect.top : undefined;
    const availableBottom = Math.min(
      window.innerHeight - viewportMargin,
      mobileNavTop == null ? Number.POSITIVE_INFINITY : mobileNavTop - viewportMargin,
    );
    const availableHeight = Math.max(96, availableBottom - viewportMargin);
    const popoverHeight = Math.min(popoverRef.getBoundingClientRect().height, availableHeight);
    const left = Math.min(
      Math.max(triggerRect.right - width, viewportMargin),
      window.innerWidth - viewportMargin - width,
    );
    const belowTop = triggerRect.bottom + triggerGap;
    const top =
      belowTop + popoverHeight <= availableBottom
        ? belowTop
        : Math.max(viewportMargin, triggerRect.top - triggerGap - popoverHeight);

    setPopoverStyle(
      `position:fixed !important;left:${left}px;top:${top}px;width:${width}px;max-height:${availableHeight}px`,
    );
  };

  createEffect(() => {
    if (!open()) {
      return;
    }

    const handlePointerDown = (event: PointerEvent) => {
      const target = event.target as Node;
      const insidePopover = popoverRef?.contains(target) ?? false;
      const onTrigger = triggerRef?.contains(target) ?? false;

      if (!insidePopover && !onTrigger) {
        close();
      }
    };

    const handleKeyDown = (event: KeyboardEvent) => {
      if (event.key === 'Escape') {
        if (containsFocus()) {
          closeAndRestoreFocus();
        } else {
          close();
        }
      }
    };

    window.addEventListener('pointerdown', handlePointerDown);
    window.addEventListener('keydown', handleKeyDown);
    window.addEventListener('resize', updatePopoverPosition);
    window.addEventListener('scroll', updatePopoverPosition, true);
    queueMicrotask(updatePopoverPosition);

    onCleanup(() => {
      window.removeEventListener('pointerdown', handlePointerDown);
      window.removeEventListener('keydown', handleKeyDown);
      window.removeEventListener('resize', updatePopoverPosition);
      window.removeEventListener('scroll', updatePopoverPosition, true);
    });
  });

  return {
    buttonLabel: options.buttonLabel,
    close,
    closeAndRestoreFocus,
    handleBlur: (event) => {
      if (!pointerInside && !containsFocus(event.relatedTarget)) {
        setOpen(false);
      }
    },
    handleClick: () => {
      if (options.openOnHover()) {
        setOpen(true);
        focusCloseButton();
        return;
      }
      setOpen((value) => {
        if (!value) focusCloseButton();
        return !value;
      });
    },
    handleMouseEnter: () => {
      pointerInside = true;
      setOpen(true);
    },
    handleMouseLeave: () => {
      pointerInside = false;
      if (!containsFocus()) {
        setOpen(false);
      }
    },
    handlePopoverBlur: (event) => {
      if (!pointerInside && !containsFocus(event.relatedTarget)) {
        setOpen(false);
      }
    },
    isOpen: open,
    popoverStyle,
    setCloseButtonRef: (el) => {
      closeButtonRef = el;
    },
    setPopoverRef: (el) => {
      popoverRef = el;
      queueMicrotask(updatePopoverPosition);
    },
    setTriggerRef: (el) => {
      triggerRef = el;
    },
  };
}
