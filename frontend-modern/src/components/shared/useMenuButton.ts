import { createEffect, createUniqueId } from 'solid-js';
import type { Accessor } from 'solid-js';

type MenuFocusTarget = 'first' | 'last';

interface MenuButtonOptions {
  isOpen: Accessor<boolean>;
  setOpen: (open: boolean) => void;
  trigger: () => HTMLElement | undefined;
  menu: () => HTMLElement | undefined;
}

const MENU_ITEM_SELECTOR = '[role="menuitem"], [role="menuitemcheckbox"], [role="menuitemradio"]';

function getEnabledMenuItems(menu: HTMLElement | undefined): HTMLElement[] {
  return Array.from(menu?.querySelectorAll<HTMLElement>(MENU_ITEM_SELECTOR) ?? []).filter(
    (item) => !item.hasAttribute('disabled') && item.getAttribute('aria-disabled') !== 'true',
  );
}

/**
 * Focus and keyboard behavior shared by action-menu buttons.
 *
 * Menu content may be portaled away from its trigger, so relying on document
 * tab order would make operators traverse the rest of the page before reaching
 * it. This helper explicitly moves focus into the menu and implements the APG
 * menu-button arrow, Home, End, Escape, and Tab behaviors.
 */
export function useMenuButton(options: MenuButtonOptions) {
  const uniqueId = createUniqueId();
  const triggerId = `pulse-menu-trigger-${uniqueId}`;
  const menuId = `pulse-menu-${uniqueId}`;
  let requestedFocus: MenuFocusTarget = 'first';

  const menuItems = () => getEnabledMenuItems(options.menu());

  const focusItem = (target: MenuFocusTarget) => {
    queueMicrotask(() => {
      const items = menuItems();
      (target === 'last' ? items.at(-1) : items[0])?.focus();
    });
  };

  const openMenu = (target: MenuFocusTarget = 'first') => {
    requestedFocus = target;
    options.setOpen(true);
    focusItem(target);
  };

  const closeMenu = (restoreFocus = false) => {
    options.setOpen(false);
    if (restoreFocus) queueMicrotask(() => options.trigger()?.focus());
  };

  const toggleMenu = () => {
    if (options.isOpen()) {
      closeMenu();
      return;
    }
    openMenu();
  };

  createEffect(() => {
    if (!options.isOpen()) return;
    const target = requestedFocus;
    requestedFocus = 'first';
    focusItem(target);
  });

  const handleTriggerKeyDown = (event: KeyboardEvent) => {
    if (event.key === 'Escape' && options.isOpen()) {
      event.preventDefault();
      closeMenu(true);
      return;
    }
    if (event.key !== 'ArrowDown' && event.key !== 'ArrowUp') return;
    event.preventDefault();
    openMenu(event.key === 'ArrowUp' ? 'last' : 'first');
  };

  const handleMenuKeyDown = (event: KeyboardEvent) => {
    if (event.key === 'Escape') {
      event.preventDefault();
      closeMenu(true);
      return;
    }
    if (event.key === 'Tab') {
      // Return focus to the trigger synchronously, then let the browser's
      // native Tab/Shift+Tab action move to the adjacent page control.
      // If the focused portaled item were removed first, browsers could
      // restart sequential navigation at the beginning of the document.
      options.trigger()?.focus({ preventScroll: true });
      options.setOpen(false);
      return;
    }
    if (!['ArrowDown', 'ArrowUp', 'Home', 'End'].includes(event.key)) return;

    const items = menuItems();
    if (items.length === 0) return;
    event.preventDefault();
    const currentIndex = items.indexOf(document.activeElement as HTMLElement);
    let nextIndex = 0;
    if (event.key === 'End') {
      nextIndex = items.length - 1;
    } else if (event.key === 'ArrowDown') {
      nextIndex = currentIndex < 0 ? 0 : (currentIndex + 1) % items.length;
    } else if (event.key === 'ArrowUp') {
      nextIndex = currentIndex <= 0 ? items.length - 1 : currentIndex - 1;
    }
    items[nextIndex]?.focus();
  };

  return {
    closeMenu,
    handleMenuKeyDown,
    handleTriggerKeyDown,
    menuId,
    openMenu,
    toggleMenu,
    triggerId,
  };
}
