import { createEffect, onCleanup, onMount } from 'solid-js';
import { getHorizontalRailScrollLeft } from './horizontalRailVisibilityModel';

interface ActiveHorizontalRailItemVisibilityOptions {
  active: () => unknown;
  rail: () => HTMLElement | undefined;
  activeSelector?: string;
}

export function useActiveHorizontalRailItemVisibility(
  options: ActiveHorizontalRailItemVisibilityOptions,
): void {
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
    });
  };

  createEffect(() => {
    const active = options.active();
    const timeoutId = window.setTimeout(() => {
      if (options.active() !== active) return;
      keepActiveItemVisible();
    });
    onCleanup(() => window.clearTimeout(timeoutId));
  });

  onMount(() => {
    window.addEventListener('resize', keepActiveItemVisible);
    const rail = options.rail();
    const resizeObserver =
      typeof ResizeObserver === 'function' ? new ResizeObserver(keepActiveItemVisible) : undefined;
    if (rail) resizeObserver?.observe(rail);

    onCleanup(() => {
      window.removeEventListener('resize', keepActiveItemVisible);
      resizeObserver?.disconnect();
    });
  });
}
