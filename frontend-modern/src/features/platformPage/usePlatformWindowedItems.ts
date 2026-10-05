import {
  createEffect,
  createMemo,
  createSignal,
  onCleanup,
  untrack,
  type Accessor,
} from 'solid-js';

import { useTableWindowing } from '@/components/Infrastructure/useTableWindowing';
import {
  bindWindowedPageScrollEvents,
  findWindowedPageScrollContainer,
  isWindowedSurfaceHidden,
  wheelDeltaInPixels,
} from '@/components/shared/windowedPageScroll';

const DEFAULT_ESTIMATED_ITEM_HEIGHT = 40;
const DESKTOP_WINDOW_SIZE = 140;
const PHONE_WINDOW_SIZE = 36;
// Grouped surfaces render a short group header before their first content row.
// Sampling a few siblings and keeping the tallest keeps that header from
// collapsing the estimate and desynchronising the window from the scroll
// position, while uniform tables still measure their real row height.
const MEASUREMENT_SAMPLE_SIZE = 3;

const measureTallestSampledSibling = (anchor: HTMLElement): number => {
  let measuredHeight = 0;
  let sibling = anchor.nextElementSibling;
  for (let sampled = 0; sampled < MEASUREMENT_SAMPLE_SIZE && sibling; sampled += 1) {
    const height = sibling.getBoundingClientRect().height;
    if (height > measuredHeight) measuredHeight = height;
    sibling = sibling.nextElementSibling;
  }
  return measuredHeight;
};

// A table runway sits between a top and a bottom spacer row, so the mounted
// rows can be measured as a whole. Averaging them per item lets a few taller
// rows (a reason line under the name, an open detail row) count once instead
// of setting the height of every row off screen, which made the spacers
// overshoot and the window jump as rows scrolled out. Returns 0 when the
// anchor is not such a runway, so other surfaces keep the sampled estimate.
const measureRenderedRunwayItemHeight = (anchor: HTMLElement, renderedItems: number): number => {
  if (anchor.tagName !== 'TR' || anchor.dataset.platformWindowSpacer !== 'top') return 0;
  if (renderedItems <= 0) return 0;
  let total = 0;
  let sibling = anchor.nextElementSibling;
  while (sibling && (sibling as HTMLElement).dataset.platformWindowSpacer !== 'bottom') {
    total += sibling.getBoundingClientRect().height;
    sibling = sibling.nextElementSibling;
  }
  if (!sibling || total <= 0) return 0;
  return total / renderedItems;
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
 * Wheel input projects the next scroll position before the browser paints it.
 * Touch scrolling stays compositor-native and updates through the scroll event
 * so changing the keyed runway cannot consume a phone swipe through anchoring.
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
    if (!anchor || isWindowedSurfaceHidden(anchor)) return;

    if (measureItems) {
      const measuredHeight =
        measureRenderedRunwayItemHeight(anchor, visibleItems().length) ||
        measureTallestSampledSibling(anchor);
      if (measuredHeight > 0) setEstimatedItemHeight(measuredHeight);
    }

    const segmentRect = anchor.getBoundingClientRect();
    const scrollContainer = findWindowedPageScrollContainer(anchor);
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

    const scrollContainer = findWindowedPageScrollContainer(anchor);
    const scrollTarget: HTMLElement | Window = scrollContainer ?? window;
    const viewportHeight = () =>
      scrollContainer?.clientHeight || window.innerHeight || estimatedItemHeight();
    const handleScroll = () => syncWindowToViewport();
    const handleWheel = (event: Event) => {
      const wheelEvent = event as WheelEvent;
      if (wheelEvent.deltaY === 0) return;
      syncWindowToViewport(wheelDeltaInPixels(wheelEvent, viewportHeight()));
    };
    const handleResize = () => syncWindowToViewport(0, true);

    // The initial measurement reads windowing signals the pass itself moves.
    // Keep those reads outside this setup effect's dependency graph so runway
    // top-ups during scrolling cannot re-run measurement and listener binding.
    untrack(handleResize);
    onCleanup(
      bindWindowedPageScrollEvents({
        scrollTarget,
        onScroll: handleScroll,
        onWheel: handleWheel,
        onResize: handleResize,
      }),
    );
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
