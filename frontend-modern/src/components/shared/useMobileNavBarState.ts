import { createEffect, createMemo, createSignal, onCleanup } from 'solid-js';
import {
  buildMobileNavBarLayout,
  isMobileNavDestinationActive,
  type MobileNavBarDestination,
  type MobileNavBarPrimaryTab,
  type MobileNavBarProps,
  type MobileNavBarUtilityTab,
} from './mobileNavBarModel';

export type {
  MobileNavBarPrimaryTab,
  MobileNavBarProps,
  MobileNavBarUtilityTab,
} from './mobileNavBarModel';

export function useMobileNavBarState(props: MobileNavBarProps) {
  const [isOverflowOpen, setIsOverflowOpen] = createSignal(false);
  const [overflowTriggerRef, setOverflowTriggerRef] = createSignal<HTMLButtonElement>();
  const [overflowMenuRef, setOverflowMenuRef] = createSignal<HTMLDivElement>();

  const layout = createMemo(() =>
    buildMobileNavBarLayout(props.primaryTabs(), props.utilityTabs()),
  );
  const fixedDestinations = createMemo(() => layout().fixedDestinations);
  const overflowDestinations = createMemo(() => layout().overflowDestinations);
  const overflowPrimaryDestinations = createMemo(() =>
    overflowDestinations().filter(
      (destination): destination is Extract<MobileNavBarDestination, { kind: 'primary' }> =>
        destination.kind === 'primary',
    ),
  );
  const overflowUtilityDestinations = createMemo(() =>
    overflowDestinations().filter(
      (destination): destination is Extract<MobileNavBarDestination, { kind: 'utility' }> =>
        destination.kind === 'utility',
    ),
  );
  const overflowIsActive = createMemo(() =>
    overflowDestinations().some((destination) =>
      isMobileNavDestinationActive(destination, props.activeTab()),
    ),
  );

  const overflowMenuItems = () =>
    Array.from(overflowMenuRef()?.querySelectorAll<HTMLButtonElement>('[role="menuitem"]') ?? []);

  const focusOverflowItem = (target: 'active' | 'first' | 'last') => {
    queueMicrotask(() => {
      const items = overflowMenuItems();
      if (items.length === 0) return;
      const item =
        target === 'last'
          ? items.at(-1)
          : target === 'active'
            ? items.find((candidate) => candidate.getAttribute('aria-current') === 'page')
            : undefined;
      (item ?? items[0])?.focus();
    });
  };

  const openOverflow = (focusTarget: 'active' | 'first' | 'last' = 'active') => {
    setIsOverflowOpen(true);
    focusOverflowItem(focusTarget);
  };

  const closeOverflow = (restoreFocus = false) => {
    setIsOverflowOpen(false);
    if (restoreFocus) {
      queueMicrotask(() => overflowTriggerRef()?.focus());
    }
  };

  createEffect(() => {
    if (!isOverflowOpen()) return;

    const handlePointerDown = (event: PointerEvent) => {
      const target = event.target as Node;
      if (overflowMenuRef()?.contains(target) || overflowTriggerRef()?.contains(target)) return;
      closeOverflow();
    };
    document.addEventListener('pointerdown', handlePointerDown);
    onCleanup(() => document.removeEventListener('pointerdown', handlePointerDown));
  });

  let previousActiveTab = props.activeTab();
  createEffect(() => {
    const activeTab = props.activeTab();
    if (activeTab === previousActiveTab) return;
    previousActiveTab = activeTab;
    closeOverflow();
  });

  const handlePrimaryClick = (tab: MobileNavBarPrimaryTab) => {
    props.onPrimaryClick(tab);
  };

  const handleUtilityClick = (tab: MobileNavBarUtilityTab) => {
    props.onUtilityClick(tab);
  };

  const handleDestinationClick = (destination: MobileNavBarDestination) => {
    closeOverflow();
    if (destination.kind === 'primary') {
      handlePrimaryClick(destination.tab);
      return;
    }
    handleUtilityClick(destination.tab);
  };

  const handleOverflowTriggerClick = () => {
    if (isOverflowOpen()) {
      closeOverflow();
      return;
    }
    openOverflow();
  };

  const handleOverflowTriggerKeyDown = (event: KeyboardEvent) => {
    if (event.key !== 'ArrowDown' && event.key !== 'ArrowUp') return;
    event.preventDefault();
    openOverflow(event.key === 'ArrowUp' ? 'last' : 'first');
  };

  const handleOverflowMenuKeyDown = (event: KeyboardEvent) => {
    if (event.key === 'Escape') {
      event.preventDefault();
      closeOverflow(true);
      return;
    }
    if (event.key === 'Tab') {
      closeOverflow();
      return;
    }
    if (!['ArrowDown', 'ArrowUp', 'Home', 'End'].includes(event.key)) return;

    const items = overflowMenuItems();
    if (items.length === 0) return;
    event.preventDefault();
    const currentIndex = items.indexOf(document.activeElement as HTMLButtonElement);
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
    closeOverflow,
    fixedDestinations,
    handleDestinationClick,
    handleOverflowMenuKeyDown,
    handleOverflowTriggerClick,
    handleOverflowTriggerKeyDown,
    isOverflowOpen,
    overflowDestinations,
    overflowIsActive,
    overflowPrimaryDestinations,
    overflowUtilityDestinations,
    setOverflowMenuRef,
    setOverflowTriggerRef,
  };
}
