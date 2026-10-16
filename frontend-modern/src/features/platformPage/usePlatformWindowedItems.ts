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
  /** Logical items may span several DOM rows; measure the consumer's block. */
  measureItemHeight?: (anchor: HTMLElement) => number;
  /** Re-measure after a container layout or accepted inventory changes. */
  measurementKey?: Accessor<unknown>;
  /** Opt-in cumulative logical-item geometry for expandable, nonuniform rows. */
  itemGeometry?: {
    offsets: Accessor<readonly number[]>;
    measure: (anchor: HTMLElement) => void;
  };
}

// The last offset is the end of the list, not another item. Keep a scroll
// position inside a tall detail attached to its own summary until its end.
export function findWindowedItemIndex(offsets: readonly number[], position: number): number {
  let low = 0;
  let high = Math.max(0, offsets.length - 2);
  while (low < high) {
    const middle = Math.ceil((low + high) / 2);
    if (offsets[middle]! <= position) low = middle;
    else high = middle - 1;
  }
  return low;
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
  const topSpacerHeight = createMemo(() => {
    if (!windowing.isWindowed()) return 0;
    return options.itemGeometry
      ? options.itemGeometry.offsets()[windowing.startIndex()]!
      : windowing.startIndex() * estimatedItemHeight();
  });
  const bottomSpacerHeight = createMemo(() => {
    if (!windowing.isWindowed()) return 0;
    const offsets = options.itemGeometry?.offsets();
    return offsets
      ? offsets[offsets.length - 1]! - offsets[windowing.endIndex()]!
      : Math.max(0, options.items().length - windowing.endIndex()) * estimatedItemHeight();
  });

  const syncWindowToViewport = (projectedScrollDelta = 0, measureItems = false) => {
    if (typeof window === 'undefined' || !windowing.isWindowed()) return;
    const anchor = anchorRef();
    if (!anchor || isWindowedSurfaceHidden(anchor)) return;

    if (measureItems) {
      options.itemGeometry?.measure(anchor);
      let measuredHeight = options.measureItemHeight?.(anchor) ?? 0;
      if (!options.measureItemHeight) {
        let sibling = anchor.nextElementSibling;
        for (let sampled = 0; sampled < MEASUREMENT_SAMPLE_SIZE && sibling; sampled += 1) {
          const height = sibling.getBoundingClientRect().height;
          if (height > measuredHeight) measuredHeight = height;
          sibling = sibling.nextElementSibling;
        }
      }
      if (Number.isFinite(measuredHeight) && measuredHeight > 0)
        setEstimatedItemHeight(measuredHeight);
    }

    const segmentRect = anchor.getBoundingClientRect();
    const scrollContainer = findWindowedPageScrollContainer(anchor);
    const sync = (scrollTop: number, height: number) => {
      const offsets = options.itemGeometry?.offsets();
      if (offsets) {
        const first = findWindowedItemIndex(offsets, scrollTop);
        const last = findWindowedItemIndex(offsets, scrollTop + height);
        // Reuse the bounded directional runway in logical row units, not a
        // fleet-wide average that would evict a tall row while it is read.
        windowing.onScroll(first, Math.max(1, last - first + 1), 1);
      } else {
        windowing.onScroll(scrollTop, height, estimatedItemHeight());
      }
    };
    if (scrollContainer) {
      const containerRect = scrollContainer.getBoundingClientRect();
      sync(
        Math.max(0, containerRect.top - segmentRect.top + projectedScrollDelta),
        scrollContainer.clientHeight || window.innerHeight,
      );
      return;
    }
    sync(
      Math.max(0, -segmentRect.top + projectedScrollDelta),
      window.innerHeight,
    );
  };

  createEffect(() => {
    if (typeof window === 'undefined') return;
    options.items().length;
    options.measurementKey?.();
    const anchor = anchorRef();
    if (!anchor || !windowing.isWindowed()) return;

    const scrollContainer = findWindowedPageScrollContainer(anchor);
    const scrollTarget: HTMLElement | Window = scrollContainer ?? window;
    const viewportHeight = () =>
      scrollContainer?.clientHeight || window.innerHeight || estimatedItemHeight();
    const measureOnScroll = Boolean(options.measureItemHeight || options.itemGeometry);
    const handleScroll = () => syncWindowToViewport(0, measureOnScroll);
    const handleWheel = (event: Event) => {
      const wheelEvent = event as WheelEvent;
      if (wheelEvent.deltaY === 0) return;
      syncWindowToViewport(
        wheelDeltaInPixels(wheelEvent, viewportHeight()),
        measureOnScroll,
      );
    };
    const handleResize = () => syncWindowToViewport(0, true);

    // The initial measurement reads windowing signals the pass itself moves.
    // Keep those reads outside this setup effect's dependency graph so runway
    // top-ups during scrolling cannot re-run measurement and listener binding.
    untrack(handleResize);
    // An expansion, refreshed detail or container-width change can resize
    // the logical block without a page scroll or a window resize.
    const observer =
      options.itemGeometry && typeof ResizeObserver === 'function'
        ? new ResizeObserver(handleResize)
        : undefined;
    observer?.observe(anchor);
    onCleanup(() => observer?.disconnect());
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
