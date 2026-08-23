import { createEffect, createMemo, createSignal, onCleanup, type Accessor } from 'solid-js';

import { useTableWindowing } from '@/components/Infrastructure/useTableWindowing';

const DEFAULT_ESTIMATED_ITEM_HEIGHT = 40;
const DESKTOP_WINDOW_SIZE = 140;
const PHONE_WINDOW_SIZE = 36;
const SCROLLABLE_OVERFLOW_PATTERN = /(?:auto|scroll|overlay)/;
const WHEEL_LINE_HEIGHT_PX = 16;

const findScrollContainer = (element: HTMLElement): HTMLElement | null => {
  let parent = element.parentElement;
  while (parent && parent !== document.body && parent !== document.documentElement) {
    const styles = getComputedStyle(parent);
    const hasVerticalScrollRange = parent.scrollHeight - parent.clientHeight > 1;
    if (
      SCROLLABLE_OVERFLOW_PATTERN.test(styles.overflowY) &&
      (styles.overflowY === 'scroll' || hasVerticalScrollRange)
    ) {
      return parent;
    }
    parent = parent.parentElement;
  }
  return null;
};

const wheelDeltaInPixels = (event: WheelEvent, viewportHeight: number): number => {
  if (event.deltaMode === WheelEvent.DOM_DELTA_LINE) return event.deltaY * WHEEL_LINE_HEIGHT_PX;
  if (event.deltaMode === WheelEvent.DOM_DELTA_PAGE) return event.deltaY * viewportHeight;
  return event.deltaY;
};

export interface PlatformWindowedItemsOptions<Item> {
  items: Accessor<readonly Item[]>;
  estimatedItemHeight?: number;
  enableThreshold?: number;
  windowSize?: number;
}

/**
 * Shared native-scroll window controller for platform tables and card lists.
 *
 * The wheel/touch listeners project the next scroll position before the browser
 * paints it, keeping a directional runway mounted during rapid input.
 */
export function usePlatformWindowedItems<Item>(options: PlatformWindowedItemsOptions<Item>) {
  const defaultWindowSize =
    typeof window !== 'undefined' && window.innerWidth < 640
      ? PHONE_WINDOW_SIZE
      : DESKTOP_WINDOW_SIZE;
  const resolvedWindowSize = options.windowSize ?? defaultWindowSize;
  const [anchorRef, setAnchorRef] = createSignal<HTMLElement | null>(null);
  const [estimatedItemHeight, setEstimatedItemHeight] = createSignal(
    Math.max(1, options.estimatedItemHeight ?? DEFAULT_ESTIMATED_ITEM_HEIGHT),
  );
  const windowing = useTableWindowing({
    totalCount: () => options.items().length,
    windowSize: resolvedWindowSize,
    enableThreshold: options.enableThreshold ?? resolvedWindowSize,
  });

  const visibleItems = createMemo<readonly Item[]>(() => {
    const items = options.items();
    if (!windowing.isWindowed()) return items;
    return items.slice(windowing.startIndex(), windowing.endIndex());
  });
  const topSpacerHeight = createMemo(() =>
    windowing.isWindowed() ? windowing.startIndex() * estimatedItemHeight() : 0,
  );
  const bottomSpacerHeight = createMemo(() =>
    windowing.isWindowed()
      ? Math.max(0, options.items().length - windowing.endIndex()) * estimatedItemHeight()
      : 0,
  );

  const syncWindowToViewport = (projectedScrollDelta = 0, measureItems = false) => {
    if (typeof window === 'undefined' || !windowing.isWindowed()) return;
    const anchor = anchorRef();
    if (!anchor) return;

    if (measureItems) {
      const measuredHeight = anchor.nextElementSibling?.getBoundingClientRect().height;
      if (measuredHeight && measuredHeight > 0) setEstimatedItemHeight(measuredHeight);
    }

    const segmentRect = anchor.getBoundingClientRect();
    const scrollContainer = findScrollContainer(anchor);
    if (scrollContainer) {
      const containerRect = scrollContainer.getBoundingClientRect();
      windowing.onScroll(
        Math.max(0, containerRect.top - segmentRect.top + projectedScrollDelta),
        scrollContainer.clientHeight || window.innerHeight,
        estimatedItemHeight(),
      );
      return;
    }
    windowing.onScroll(
      Math.max(0, -segmentRect.top + projectedScrollDelta),
      window.innerHeight,
      estimatedItemHeight(),
    );
  };

  createEffect(() => {
    if (typeof window === 'undefined') return;
    options.items().length;
    const anchor = anchorRef();
    if (!anchor || !windowing.isWindowed()) return;

    let lastTouchY: number | null = null;
    const scrollContainer = findScrollContainer(anchor);
    const scrollTarget: HTMLElement | Window = scrollContainer ?? window;
    const viewportHeight = () =>
      scrollContainer?.clientHeight || window.innerHeight || estimatedItemHeight();
    const handleScroll = () => syncWindowToViewport();
    const handleWheel = (event: Event) => {
      const wheelEvent = event as WheelEvent;
      if (wheelEvent.deltaY === 0) return;
      syncWindowToViewport(wheelDeltaInPixels(wheelEvent, viewportHeight()));
    };
    const handleTouchStart = (event: Event) => {
      lastTouchY = (event as TouchEvent).touches.item(0)?.clientY ?? null;
    };
    const handleTouchMove = (event: Event) => {
      const nextTouchY = (event as TouchEvent).touches.item(0)?.clientY ?? null;
      if (nextTouchY == null || lastTouchY == null) {
        lastTouchY = nextTouchY;
        return;
      }
      const deltaY = lastTouchY - nextTouchY;
      lastTouchY = nextTouchY;
      if (deltaY !== 0) syncWindowToViewport(deltaY);
    };
    const handleTouchEnd = () => {
      lastTouchY = null;
    };
    const handleResize = () => syncWindowToViewport(0, true);

    handleResize();
    scrollTarget.addEventListener('scroll', handleScroll, { passive: true });
    // Let the keyed runway move before the compositor advances native scroll.
    // The handlers never cancel input; non-passive ordering only prevents a
    // fast wheel or fling from exposing an as-yet-unmounted region.
    scrollTarget.addEventListener('wheel', handleWheel, { passive: false });
    scrollTarget.addEventListener('touchstart', handleTouchStart, { passive: true });
    scrollTarget.addEventListener('touchmove', handleTouchMove, { passive: false });
    scrollTarget.addEventListener('touchend', handleTouchEnd, { passive: true });
    scrollTarget.addEventListener('touchcancel', handleTouchEnd, { passive: true });
    window.addEventListener('resize', handleResize);
    onCleanup(() => {
      scrollTarget.removeEventListener('scroll', handleScroll);
      scrollTarget.removeEventListener('wheel', handleWheel);
      scrollTarget.removeEventListener('touchstart', handleTouchStart);
      scrollTarget.removeEventListener('touchmove', handleTouchMove);
      scrollTarget.removeEventListener('touchend', handleTouchEnd);
      scrollTarget.removeEventListener('touchcancel', handleTouchEnd);
      window.removeEventListener('resize', handleResize);
    });
  });

  return {
    isWindowed: windowing.isWindowed,
    startIndex: windowing.startIndex,
    visibleItems,
    topSpacerHeight,
    bottomSpacerHeight,
    setAnchorRef,
  };
}
