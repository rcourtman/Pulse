import { createEffect, createSignal, onCleanup } from 'solid-js';
import type { Accessor } from 'solid-js';

interface SearchTipsPopoverState {
  buttonLabel: Accessor<string>;
  close: () => void;
  handleBlur: () => void;
  handleClick: () => void;
  handleMouseEnter: () => void;
  handleMouseLeave: () => void;
  isOpen: Accessor<boolean>;
  popoverStyle: Accessor<string | undefined>;
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
  let pointerInside = false;

  const close = () => setOpen(false);
  const updatePopoverPosition = () => {
    if (!triggerRef || !popoverRef || window.innerWidth >= 640) {
      setPopoverStyle(undefined);
      return;
    }

    const viewportMargin = 16;
    const triggerGap = 8;
    const width = Math.min(288, window.innerWidth - viewportMargin * 2);
    const triggerRect = triggerRef.getBoundingClientRect();
    const popoverHeight = popoverRef.getBoundingClientRect().height;
    const left = Math.min(
      Math.max(triggerRect.right - width, viewportMargin),
      window.innerWidth - viewportMargin - width,
    );
    const belowTop = triggerRect.bottom + triggerGap;
    const top =
      belowTop + popoverHeight <= window.innerHeight - viewportMargin
        ? belowTop
        : Math.max(viewportMargin, triggerRect.top - triggerGap - popoverHeight);

    setPopoverStyle(`left:${left}px;top:${top}px;width:${width}px`);
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
        close();
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
    handleBlur: () => {
      if (!pointerInside) {
        setOpen(false);
      }
    },
    handleClick: () => {
      if (options.openOnHover()) {
        setOpen(true);
        return;
      }
      setOpen((value) => !value);
    },
    handleMouseEnter: () => {
      pointerInside = true;
      setOpen(true);
    },
    handleMouseLeave: () => {
      pointerInside = false;
      setOpen(false);
    },
    isOpen: open,
    popoverStyle,
    setPopoverRef: (el) => {
      popoverRef = el;
      queueMicrotask(updatePopoverPosition);
    },
    setTriggerRef: (el) => {
      triggerRef = el;
    },
  };
}
