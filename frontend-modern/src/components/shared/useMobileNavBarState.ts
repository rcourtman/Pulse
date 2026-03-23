import { createEffect, createMemo, createSignal, onCleanup } from 'solid-js';
import {
  buildOrderedMobileNavPlatformTabs,
  buildOrderedMobileNavUtilityTabs,
  getMobileNavFadeState,
  type MobileNavBarPlatformTab,
  type MobileNavBarProps,
  type MobileNavBarUtilityTab,
} from './mobileNavBarModel';

export type {
  MobileNavBarPlatformTab,
  MobileNavBarProps,
  MobileNavBarUtilityTab,
} from './mobileNavBarModel';

export function useMobileNavBarState(props: MobileNavBarProps) {
  const [showFade, setShowFade] = createSignal(false);
  const [showLeftFade, setShowLeftFade] = createSignal(false);
  const [navRef, setNavRef] = createSignal<HTMLDivElement>();

  const orderedPlatformTabs = createMemo(() =>
    buildOrderedMobileNavPlatformTabs(props.platformTabs()),
  );
  const orderedUtilityTabs = createMemo(() =>
    buildOrderedMobileNavUtilityTabs(props.utilityTabs()),
  );

  const updateFadeIndicator = () => {
    const fadeState = getMobileNavFadeState(navRef());
    setShowFade(fadeState.showRightFade);
    setShowLeftFade(fadeState.showLeftFade);
  };

  createEffect(() => {
    const nav = navRef();
    if (!nav) return;

    updateFadeIndicator();
    const handleScroll = () => updateFadeIndicator();
    nav.addEventListener('scroll', handleScroll, { passive: true });
    window.addEventListener('resize', handleScroll);
    onCleanup(() => {
      nav.removeEventListener('scroll', handleScroll);
      window.removeEventListener('resize', handleScroll);
    });
  });

  createEffect(() => {
    const nav = navRef();
    if (!nav) return;

    const activeId = props.activeTab();
    if (!activeId) return;

    const activeEl = nav.querySelector<HTMLElement>(`[data-tab-id="${activeId}"]`);
    if (!activeEl) return;

    requestAnimationFrame(() => {
      activeEl.scrollIntoView({ behavior: 'smooth', block: 'nearest', inline: 'center' });
      updateFadeIndicator();
    });
  });

  const handlePlatformClick = (platform: MobileNavBarPlatformTab) => {
    props.onPlatformClick(platform);
  };

  const handleUtilityClick = (tab: MobileNavBarUtilityTab) => {
    props.onUtilityClick(tab);
  };

  return {
    handlePlatformClick,
    handleUtilityClick,
    orderedPlatformTabs,
    orderedUtilityTabs,
    setNavRef,
    showFade,
    showLeftFade,
  };
}

export type MobileNavBarState = ReturnType<typeof useMobileNavBarState>;
