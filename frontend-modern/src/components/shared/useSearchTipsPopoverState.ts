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
  let popoverRef: HTMLDivElement | undefined;
  let triggerRef: HTMLButtonElement | undefined;
  let pointerInside = false;

  const close = () => setOpen(false);

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

    onCleanup(() => {
      window.removeEventListener('pointerdown', handlePointerDown);
      window.removeEventListener('keydown', handleKeyDown);
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
    setPopoverRef: (el) => {
      popoverRef = el;
    },
    setTriggerRef: (el) => {
      triggerRef = el;
    },
  };
}
