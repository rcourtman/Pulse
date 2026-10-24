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
export interface PlatformWindowedItemsOptions<Item> {
  items: Accessor<readonly Item[]>;
  estimatedItemHeight?: number;
  enableThreshold?: number;
  windowSize?: number;
  /** Native table items may include a variable-height inline detail row. */
  measureTableRows?: boolean;
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
  // Measurements belong to keyed items, so sorting and leaving the mounted
  // runway preserve their scroll extent. Offscreen items retain their last
  // measured size until they are mounted and measured again.
  const [measuredHeights, setMeasuredHeights] = createSignal<ReadonlyMap<Item, number>>(new Map());
  const measuredLayout = createMemo(() => {
    if (!options.measureTableRows) return undefined;
    const heights = measuredHeights();
    const estimate = estimatedItemHeight();
    const offsets = [0];
    for (const item of options.items()) {
      offsets.push(offsets[offsets.length - 1]! + (heights.get(item) ?? estimate));
    }
    const indexAt = (offset: number) => {
      let low = 0;
      let high = Math.max(0, offsets.length - 2);
      while (low < high) {
        const middle = Math.ceil((low + high) / 2);
        if (offsets[middle]! <= offset) low = middle;
        else high = middle - 1;
      }
      return low;
    };
    return { offsets, indexAt };
  });

  const visibleItems = createMemo<readonly Item[]>(() => {
    const items = options.items();
    if (!windowing.isWindowed()) return items;
    return items.slice(windowing.startIndex(), windowing.endIndex());
  });
  const topSpacerHeight = createMemo(() =>
    windowing.isWindowed()
      ? (measuredLayout()?.offsets[windowing.startIndex()] ??
        windowing.startIndex() * estimatedItemHeight())
      : 0,
  );
  const bottomSpacerHeight = createMemo(() =>
    windowing.isWindowed()
      ? measuredLayout()
        ? measuredLayout()!.offsets[options.items().length]! -
          measuredLayout()!.offsets[windowing.endIndex()]!
        : Math.max(0, options.items().length - windowing.endIndex()) * estimatedItemHeight()
      : 0,
  );

  const measureTableRows = () => {
    if (!options.measureTableRows) return;
    const anchor = anchorRef();
    if (!anchor || isWindowedSurfaceHidden(anchor)) return;
    const items = visibleItems();
    const measurements = new Map<Item, number>();
    let index = -1;
    let sibling = anchor.nextElementSibling;
    while (sibling && !sibling.hasAttribute('data-platform-window-spacer')) {
      const height = sibling.getBoundingClientRect().height;
      if (!sibling.hasAttribute('data-inline-detail-row')) index += 1;
      const item = items[index];
      if (item !== undefined && height > 0) {
        measurements.set(item, (measurements.get(item) ?? 0) + height);
      }
      sibling = sibling.nextElementSibling;
    }
    const previous = measuredHeights();
    const activeItems = new Set(options.items());
    const next = new Map([...previous].filter(([item]) => activeItems.has(item)));
    for (const [item, height] of measurements) next.set(item, height);
    if (
      next.size !== previous.size ||
      [...next].some(([item, height]) => previous.get(item) !== height)
    ) {
      setMeasuredHeights(next);
    }
  };

  const syncWindowToViewport = (projectedScrollDelta = 0, measureItems = false) => {
    if (typeof window === 'undefined' || !windowing.isWindowed()) return;
    const anchor = anchorRef();
    if (!anchor || isWindowedSurfaceHidden(anchor)) return;

    if (measureItems && !options.measureTableRows) {
      let measuredHeight = 0;
      let sibling = anchor.nextElementSibling;
      for (let sampled = 0; sampled < MEASUREMENT_SAMPLE_SIZE && sibling; sampled += 1) {
        const height = sibling.getBoundingClientRect().height;
        if (height > measuredHeight) measuredHeight = height;
        sibling = sibling.nextElementSibling;
      }
      if (measuredHeight > 0) setEstimatedItemHeight(measuredHeight);
    }

    const segmentRect = anchor.getBoundingClientRect();
    const scrollContainer = findWindowedPageScrollContainer(anchor);
    const updateWindow = (scrollTop: number, viewportHeight: number) => {
      const layout = measuredLayout();
      if (!layout) {
        windowing.onScroll(scrollTop, viewportHeight, estimatedItemHeight());
        return;
      }
      // Map native pixels into the controller's logical row coordinates. A
      // viewport within a tall detail still belongs to its owning item.
      const first = layout.indexAt(scrollTop);
      const last = layout.indexAt(scrollTop + viewportHeight);
      const estimate = estimatedItemHeight();
      windowing.onScroll(first * estimate, (last - first + 1) * estimate, estimate);
    };
    if (scrollContainer) {
      const containerRect = scrollContainer.getBoundingClientRect();
      updateWindow(
        Math.max(0, containerRect.top - segmentRect.top + projectedScrollDelta),
        scrollContainer.clientHeight || window.innerHeight,
      );
      return;
    }
    updateWindow(Math.max(0, -segmentRect.top + projectedScrollDelta), window.innerHeight);
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
    const handleResize = () => {
      measureTableRows();
      syncWindowToViewport(0, true);
    };

    // Native disclosure, history and live-content height changes are measured
    // outside the scroll/wheel hot path. Observe the owning tbody so newly
    // mounted detail rows participate without consumer-specific callbacks.
    let resizeObserver: ResizeObserver | undefined;
    let mutationObserver: MutationObserver | undefined;
    if (options.measureTableRows && anchor.parentElement) {
      if (typeof ResizeObserver !== 'undefined') {
        resizeObserver = new ResizeObserver(handleResize);
        resizeObserver.observe(anchor.parentElement);
      }
      mutationObserver = new MutationObserver(handleResize);
      mutationObserver.observe(anchor.parentElement, { childList: true });
    }

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
    onCleanup(() => {
      resizeObserver?.disconnect();
      mutationObserver?.disconnect();
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
