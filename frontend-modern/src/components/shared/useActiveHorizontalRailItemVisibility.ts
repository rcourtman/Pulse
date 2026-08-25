import { createEffect, onCleanup, onMount } from 'solid-js';
import { getHorizontalRailScrollLeft } from './horizontalRailVisibilityModel';

interface ActiveHorizontalRailItemVisibilityOptions {
  active: () => unknown;
  rail: () => HTMLElement | undefined;
  activeSelector?: string;
  edgePadding?: number;
}

interface ActiveHorizontalRailItemVisibilityController {
  markManualScrollIntent: () => void;
}

export function useActiveHorizontalRailItemVisibility(
  options: ActiveHorizontalRailItemVisibilityOptions,
): ActiveHorizontalRailItemVisibilityController {
  let preserveManualPosition = false;

  const keepActiveItemVisible = () => {
    const rail = options.rail();
    const activeItem = rail?.querySelector<HTMLElement>(
      options.activeSelector ?? '[aria-current="page"]',
    );
    if (!rail || !activeItem) return;

    rail.scrollLeft = getHorizontalRailScrollLeft({
      scrollLeft: rail.scrollLeft,
      scrollWidth: rail.scrollWidth,
      clientWidth: rail.clientWidth,
      itemOffsetLeft: activeItem.offsetLeft,
      itemOffsetWidth: activeItem.offsetWidth,
      edgePadding: options.edgePadding,
    });
  };

  const keepActiveItemVisibleAfterResize = () => {
    if (preserveManualPosition) return;
    keepActiveItemVisible();
  };

  const markManualScrollIntent = () => {
    // Mobile browser chrome and responsive layout changes can emit resize
    // events during a native horizontal swipe. Once the operator starts
    // exploring the rail, keep their chosen position until the active item
    // actually changes instead of pulling the rail back to the active tab.
    preserveManualPosition = true;
  };

  createEffect(() => {
    const active = options.active();
    preserveManualPosition = false;
    const timeoutId = window.setTimeout(() => {
      if (options.active() !== active) return;
      keepActiveItemVisible();
    });
    onCleanup(() => window.clearTimeout(timeoutId));
  });

  onMount(() => {
    const rail = options.rail();
    window.addEventListener('resize', keepActiveItemVisibleAfterResize);
    rail?.addEventListener('pointerdown', markManualScrollIntent, { passive: true });
    rail?.addEventListener('touchstart', markManualScrollIntent, { passive: true });
    rail?.addEventListener('wheel', markManualScrollIntent, { passive: true });
    const resizeObserver =
      typeof ResizeObserver === 'function'
        ? new ResizeObserver(keepActiveItemVisibleAfterResize)
        : undefined;
    if (rail) resizeObserver?.observe(rail);

    onCleanup(() => {
      window.removeEventListener('resize', keepActiveItemVisibleAfterResize);
      rail?.removeEventListener('pointerdown', markManualScrollIntent);
      rail?.removeEventListener('touchstart', markManualScrollIntent);
      rail?.removeEventListener('wheel', markManualScrollIntent);
      resizeObserver?.disconnect();
    });
  });

  return { markManualScrollIntent };
}
