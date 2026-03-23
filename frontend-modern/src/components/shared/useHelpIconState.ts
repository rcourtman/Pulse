import { createEffect, createMemo, createSignal, onCleanup } from 'solid-js';
import {
  calculateHelpPopoverPosition,
  getHelpPopoverMaxWidth,
  getHelpPopoverPreferredPosition,
  getHelpIconSize,
  getMissingHelpContentWarning,
  resolveHelpContent,
  type HelpIconProps,
} from './helpIconModel';

export function useHelpIconState(props: HelpIconProps) {
  const [isOpen, setIsOpen] = createSignal(false);
  const [popoverPosition, setPopoverPosition] = createSignal({ top: 0, left: 0 });
  const [buttonRef, setButtonRef] = createSignal<HTMLButtonElement>();
  const [popoverRef, setPopoverRef] = createSignal<HTMLDivElement>();

  const helpContent = createMemo(() => resolveHelpContent(props));
  const size = createMemo(() => getHelpIconSize(props.size));
  const maxWidth = createMemo(() => getHelpPopoverMaxWidth(props.maxWidth));
  const preferredPosition = createMemo(() =>
    getHelpPopoverPreferredPosition(props.position),
  );

  createEffect(() => {
    if (!isOpen() || !buttonRef()) return;

    requestAnimationFrame(() => {
      const button = buttonRef();
      const popover = popoverRef();
      if (!button || !popover) return;

      setPopoverPosition(
        calculateHelpPopoverPosition({
          buttonRect: button.getBoundingClientRect(),
          popoverRect: popover.getBoundingClientRect(),
          preferredPosition: preferredPosition(),
          viewportWidth: window.innerWidth,
          viewportHeight: window.innerHeight,
        }),
      );
    });
  });

  createEffect(() => {
    if (!isOpen()) return;

    const button = buttonRef();
    const popover = popoverRef();

    const handleClickOutside = (event: MouseEvent) => {
      const target = event.target as Node;
      if (button?.contains(target) || popover?.contains(target)) return;
      setIsOpen(false);
    };

    const handleEscape = (event: KeyboardEvent) => {
      if (event.key === 'Escape') {
        setIsOpen(false);
        button?.focus();
      }
    };

    const timeoutId = setTimeout(() => {
      document.addEventListener('click', handleClickOutside);
      document.addEventListener('keydown', handleEscape);
    }, 0);

    onCleanup(() => {
      clearTimeout(timeoutId);
      document.removeEventListener('click', handleClickOutside);
      document.removeEventListener('keydown', handleEscape);
    });
  });

  const missingContentWarning = getMissingHelpContentWarning(props.contentId);
  if (!helpContent() && missingContentWarning) {
    console.warn(missingContentWarning);
  }

  const toggleOpen = (event: MouseEvent) => {
    event.stopPropagation();
    setIsOpen(!isOpen());
  };

  return {
    buttonRef,
    helpContent,
    isOpen,
    maxWidth,
    popoverPosition,
    popoverRef,
    preferredPosition,
    setButtonRef,
    setIsOpen,
    setPopoverRef,
    size,
    toggleOpen,
  };
}

export type HelpIconState = ReturnType<typeof useHelpIconState>;
