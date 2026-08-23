import { createEffect, createSignal, onCleanup, type Accessor } from 'solid-js';

import type { UseGroupedTableWindowingResult } from './useGroupedTableWindowing';

const SCROLLABLE_OVERFLOW_PATTERN = /(?:auto|scroll|overlay)/;
const MIN_VERTICAL_SCROLL_RANGE_PX = 1;
const SCROLL_TO_TOP_VISIBILITY_THRESHOLD_PX = 640;
const WHEEL_LINE_HEIGHT_PX = 16;

interface TouchPosition {
  x: number;
  y: number;
}

const wheelDeltaInPixels = (event: WheelEvent, viewportHeight: number) => {
  if (event.deltaMode === WheelEvent.DOM_DELTA_LINE) return event.deltaY * WHEEL_LINE_HEIGHT_PX;
  if (event.deltaMode === WheelEvent.DOM_DELTA_PAGE) return event.deltaY * viewportHeight;
  return event.deltaY;
};

const findScrollContainer = (element: HTMLElement): HTMLElement | null => {
  let parent = element.parentElement;
  while (parent && parent !== document.body && parent !== document.documentElement) {
    const styles = getComputedStyle(parent);
    const hasVerticalScrollRange =
      parent.scrollHeight - parent.clientHeight > MIN_VERTICAL_SCROLL_RANGE_PX;
    const ownsVerticalScrollBeforeOverflow = styles.overflowY === 'scroll';
    if (
      SCROLLABLE_OVERFLOW_PATTERN.test(styles.overflowY) &&
      (ownsVerticalScrollBeforeOverflow || hasVerticalScrollRange)
    ) {
      return parent;
    }
    parent = parent.parentElement;
  }
  return null;
};

interface WorkloadsWorkloadViewportSyncOptions {
  filteredGuestCount: Accessor<number>;
  groupedWindowing: UseGroupedTableWindowingResult;
  onRowGeometryChange?: (geometry: { groupHeaderHeight?: number; rowHeight?: number }) => void;
  rowHeight: Accessor<number>;
  tableBodyRef: Accessor<HTMLTableSectionElement | null>;
}

export function useWorkloadViewportSync(options: WorkloadsWorkloadViewportSyncOptions) {
  const [isScrollToTopVisible, setIsScrollToTopVisible] = createSignal(false);

  const syncGuestWindowToViewport = (measureRows = false, projectedScrollDelta = 0) => {
    if (typeof window === 'undefined') return;
    const body = options.tableBodyRef();
    if (!body) return;
    if (measureRows) {
      const guestRowHeight = body
        .querySelector<HTMLTableRowElement>(':scope > tr.workload-row')
        ?.getBoundingClientRect().height;
      const groupHeaderHeight = body
        .querySelector<HTMLTableRowElement>(':scope > tr[data-summary-group-id]')
        ?.getBoundingClientRect().height;
      options.onRowGeometryChange?.({
        groupHeaderHeight:
          groupHeaderHeight && groupHeaderHeight > 0 ? groupHeaderHeight : undefined,
        rowHeight: guestRowHeight && guestRowHeight > 0 ? guestRowHeight : undefined,
      });
    }
    const rect = body.getBoundingClientRect();
    const scrollContainer = findScrollContainer(body);
    if (scrollContainer) {
      setIsScrollToTopVisible(scrollContainer.scrollTop > SCROLL_TO_TOP_VISIBILITY_THRESHOLD_PX);
      if (!options.groupedWindowing.isWindowed()) return;
      const containerRect = scrollContainer.getBoundingClientRect();
      const scrollTop = Math.max(0, containerRect.top - rect.top);
      options.groupedWindowing.onScroll(
        Math.max(0, scrollTop + projectedScrollDelta),
        scrollContainer.clientHeight || window.innerHeight,
        options.rowHeight(),
      );
      return;
    }

    setIsScrollToTopVisible(window.scrollY > SCROLL_TO_TOP_VISIBILITY_THRESHOLD_PX);
    if (!options.groupedWindowing.isWindowed()) return;
    options.groupedWindowing.onScroll(
      Math.max(0, -rect.top + projectedScrollDelta),
      window.innerHeight,
      options.rowHeight(),
    );
  };

  createEffect(() => {
    if (typeof window === 'undefined') return;
    options.filteredGuestCount();
    if (!options.tableBodyRef()) return;

    const handleViewportScroll = () => {
      syncGuestWindowToViewport();
    };
    let lastTouchPosition: TouchPosition | null = null;
    const handleViewportWheel = (event: Event) => {
      const wheelEvent = event as WheelEvent;
      if (!options.groupedWindowing.isWindowed() || wheelEvent.deltaY === 0) return;
      const viewportHeight =
        scrollTarget instanceof HTMLElement
          ? scrollTarget.clientHeight || window.innerHeight
          : window.innerHeight;
      syncGuestWindowToViewport(false, wheelDeltaInPixels(wheelEvent, viewportHeight));
    };
    const handleViewportTouchStart = (event: Event) => {
      const touch = (event as TouchEvent).touches.item(0);
      lastTouchPosition = touch ? { x: touch.clientX, y: touch.clientY } : null;
    };
    const handleViewportTouchMove = (event: Event) => {
      const touch = (event as TouchEvent).touches.item(0);
      if (!touch) {
        lastTouchPosition = null;
        return;
      }

      const nextPosition = { x: touch.clientX, y: touch.clientY };
      const previousPosition = lastTouchPosition;
      lastTouchPosition = nextPosition;
      if (!previousPosition || !options.groupedWindowing.isWindowed()) return;

      const deltaX = previousPosition.x - nextPosition.x;
      const deltaY = previousPosition.y - nextPosition.y;
      if (deltaY === 0 || Math.abs(deltaY) <= Math.abs(deltaX)) return;
      syncGuestWindowToViewport(false, deltaY);
    };
    const handleViewportTouchEnd = (event: Event) => {
      const touch = (event as TouchEvent).touches.item(0);
      lastTouchPosition = touch ? { x: touch.clientX, y: touch.clientY } : null;
    };
    const handleViewportResize = () => {
      syncGuestWindowToViewport(true);
    };

    handleViewportResize();
    const scrollContainer = findScrollContainer(options.tableBodyRef()!);
    const scrollTarget = scrollContainer ?? window;
    scrollTarget.addEventListener('scroll', handleViewportScroll, { passive: true });
    // These two pre-scroll listeners intentionally remain non-passive. That
    // makes the browser wait for the bounded row window to move before its
    // compositor advances the viewport; neither handler cancels native input.
    scrollTarget.addEventListener('wheel', handleViewportWheel, { passive: false });
    scrollTarget.addEventListener('touchstart', handleViewportTouchStart, { passive: true });
    scrollTarget.addEventListener('touchmove', handleViewportTouchMove, { passive: false });
    scrollTarget.addEventListener('touchend', handleViewportTouchEnd, { passive: true });
    scrollTarget.addEventListener('touchcancel', handleViewportTouchEnd, { passive: true });
    window.addEventListener('resize', handleViewportResize);
    onCleanup(() => {
      scrollTarget.removeEventListener('scroll', handleViewportScroll);
      scrollTarget.removeEventListener('wheel', handleViewportWheel);
      scrollTarget.removeEventListener('touchstart', handleViewportTouchStart);
      scrollTarget.removeEventListener('touchmove', handleViewportTouchMove);
      scrollTarget.removeEventListener('touchend', handleViewportTouchEnd);
      scrollTarget.removeEventListener('touchcancel', handleViewportTouchEnd);
      window.removeEventListener('resize', handleViewportResize);
    });
  });

  const scrollToTop = () => {
    if (typeof window === 'undefined') return;
    const body = options.tableBodyRef();
    const scrollContainer = body ? findScrollContainer(body) : null;
    if (scrollContainer) {
      scrollContainer.scrollTo({ top: 0, behavior: 'smooth' });
      return;
    }
    window.scrollTo({ top: 0, behavior: 'smooth' });
  };

  return {
    isScrollToTopVisible,
    scrollToTop,
  } as const;
}
