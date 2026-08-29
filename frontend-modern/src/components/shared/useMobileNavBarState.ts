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
  const [openMenu, setOpenMenu] = createSignal<'platform' | 'overflow' | null>(null);
  const [lastPrimaryTabId, setLastPrimaryTabId] = createSignal<string | null>(null);
  const [platformTriggerRef, setPlatformTriggerRef] = createSignal<HTMLButtonElement>();
  const [platformMenuRef, setPlatformMenuRef] = createSignal<HTMLDivElement>();
  const [overflowTriggerRef, setOverflowTriggerRef] = createSignal<HTMLButtonElement>();
  const [overflowMenuRef, setOverflowMenuRef] = createSignal<HTMLDivElement>();

  const layout = createMemo(() =>
    buildMobileNavBarLayout(props.primaryTabs(), props.utilityTabs()),
  );
  const platformDestinations = createMemo(() => layout().platformDestinations);
  const fixedDestinations = createMemo(() => layout().fixedDestinations);
  const overflowDestinations = createMemo(() => layout().overflowDestinations);
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
  const platformIsActive = createMemo(() =>
    platformDestinations().some((destination) =>
      isMobileNavDestinationActive(destination, props.activeTab()),
    ),
  );
  const activePlatformDestination = createMemo(() => {
    const destinations = platformDestinations();
    return (
      destinations.find((destination) => destination.tab.id === props.activeTab()) ??
      destinations.find((destination) => destination.tab.id === lastPrimaryTabId()) ??
      destinations[0]
    );
  });

  const activeMenuRef = () => (openMenu() === 'platform' ? platformMenuRef() : overflowMenuRef());
  const activeTriggerRef = () =>
    openMenu() === 'platform' ? platformTriggerRef() : overflowTriggerRef();
  const activeMenuItems = () =>
    Array.from(activeMenuRef()?.querySelectorAll<HTMLElement>('[role="menuitem"]') ?? []);

  const focusMenuItem = (target: 'active' | 'first' | 'last') => {
    queueMicrotask(() => {
      const items = activeMenuItems();
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

  const openNavigationMenu = (
    menu: 'platform' | 'overflow',
    focusTarget: 'active' | 'first' | 'last' = 'active',
  ) => {
    setOpenMenu(menu);
    focusMenuItem(focusTarget);
  };

  const closeNavigationMenus = (restoreFocus = false) => {
    const trigger = activeTriggerRef();
    setOpenMenu(null);
    if (restoreFocus) {
      queueMicrotask(() => trigger?.focus());
    }
  };

  createEffect(() => {
    if (!openMenu()) return;

    const handlePointerDown = (event: PointerEvent) => {
      const target = event.target as Node;
      if (activeMenuRef()?.contains(target) || activeTriggerRef()?.contains(target)) return;
      closeNavigationMenus();
    };
    document.addEventListener('pointerdown', handlePointerDown);
    onCleanup(() => document.removeEventListener('pointerdown', handlePointerDown));
  });

  let previousActiveTab = props.activeTab();
  createEffect(() => {
    const activeTab = props.activeTab();
    if (platformDestinations().some((destination) => destination.tab.id === activeTab)) {
      setLastPrimaryTabId(activeTab);
    }
    if (activeTab === previousActiveTab) return;
    previousActiveTab = activeTab;
    closeNavigationMenus();
  });

  const handlePrimaryClick = (tab: MobileNavBarPrimaryTab) => {
    props.onPrimaryClick(tab);
  };

  const handleUtilityClick = (tab: MobileNavBarUtilityTab) => {
    props.onUtilityClick(tab);
  };

  const handleDestinationClick = (destination: MobileNavBarDestination) => {
    closeNavigationMenus();
    if (destination.kind === 'primary') {
      handlePrimaryClick(destination.tab);
      return;
    }
    handleUtilityClick(destination.tab);
  };

  const handlePlatformTriggerClick = () => {
    if (platformDestinations().length === 1) {
      const destination = platformDestinations()[0];
      if (destination) handleDestinationClick(destination);
      return;
    }
    if (openMenu() === 'platform') {
      closeNavigationMenus();
      return;
    }
    openNavigationMenu('platform');
  };

  const handlePlatformTriggerKeyDown = (event: KeyboardEvent) => {
    if (event.key !== 'ArrowDown' && event.key !== 'ArrowUp') return;
    event.preventDefault();
    openNavigationMenu('platform', event.key === 'ArrowUp' ? 'last' : 'first');
  };

  const handleOverflowTriggerClick = () => {
    if (openMenu() === 'overflow') {
      closeNavigationMenus();
      return;
    }
    openNavigationMenu('overflow');
  };

  const handleOverflowTriggerKeyDown = (event: KeyboardEvent) => {
    if (event.key !== 'ArrowDown' && event.key !== 'ArrowUp') return;
    event.preventDefault();
    openNavigationMenu('overflow', event.key === 'ArrowUp' ? 'last' : 'first');
  };

  const handleNavigationMenuKeyDown = (event: KeyboardEvent) => {
    if (event.key === 'Escape') {
      event.preventDefault();
      closeNavigationMenus(true);
      return;
    }
    if (event.key === 'Tab') {
      closeNavigationMenus();
      return;
    }
    if (!['ArrowDown', 'ArrowUp', 'Home', 'End'].includes(event.key)) return;

    const items = activeMenuItems();
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
    activePlatformDestination,
    closeOverflow: closeNavigationMenus,
    fixedDestinations,
    handleDestinationClick,
    handleOverflowMenuKeyDown: handleNavigationMenuKeyDown,
    handleOverflowTriggerClick,
    handleOverflowTriggerKeyDown,
    handlePlatformMenuKeyDown: handleNavigationMenuKeyDown,
    handlePlatformTriggerClick,
    handlePlatformTriggerKeyDown,
    isOverflowOpen: () => openMenu() === 'overflow',
    isPlatformMenuOpen: () => openMenu() === 'platform',
    overflowDestinations,
    overflowIsActive,
    overflowUtilityDestinations,
    platformDestinations,
    platformIsActive,
    setPlatformMenuRef,
    setPlatformTriggerRef,
    setOverflowMenuRef,
    setOverflowTriggerRef,
  };
}
