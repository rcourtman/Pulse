import { createEffect, createSignal, onCleanup, untrack, type Accessor } from 'solid-js';
import { useTableWindowing } from './useTableWindowing';

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

interface UseUnifiedResourceTableViewportSyncOptions {
  totalCount: Accessor<number>;
  estimatedRowHeight: number;
  hostWindowing: ReturnType<typeof useTableWindowing>;
}

export function useUnifiedResourceTableViewportSync(
  options: UseUnifiedResourceTableViewportSyncOptions,
) {
  const { totalCount, estimatedRowHeight, hostWindowing } = options;
  const [hostBodyRef, setHostBodyRef] = createSignal<HTMLTableSectionElement | null>(null);

  const syncHostWindowToViewport = (projectedScrollDelta = 0) => {
    if (!hostWindowing.isWindowed() || typeof window === 'undefined') return;
    const body = hostBodyRef();
    if (!body) return;
    const rect = body.getBoundingClientRect();
    const scrollContainer = findScrollContainer(body);
    if (scrollContainer) {
      const containerRect = scrollContainer.getBoundingClientRect();
      hostWindowing.onScroll(
        Math.max(0, containerRect.top - rect.top + projectedScrollDelta),
        scrollContainer.clientHeight || window.innerHeight,
        estimatedRowHeight,
      );
      return;
    }
    hostWindowing.onScroll(
      Math.max(0, -rect.top + projectedScrollDelta),
      window.innerHeight,
      estimatedRowHeight,
    );
  };

  createEffect(() => {
    if (typeof window === 'undefined') return;
    totalCount();
    if (!hostWindowing.isWindowed()) return;
    if (!hostBodyRef()) return;

    let lastTouchY: number | null = null;
    const body = hostBodyRef()!;
    const scrollContainer = findScrollContainer(body);
    const scrollTarget: HTMLElement | Window = scrollContainer ?? window;
    const viewportHeight = () => scrollContainer?.clientHeight || window.innerHeight;
    const handleViewportChange = () => syncHostWindowToViewport();
    const handleWheel = (event: Event) => {
      const wheelEvent = event as WheelEvent;
      if (wheelEvent.deltaY === 0) return;
      syncHostWindowToViewport(wheelDeltaInPixels(wheelEvent, viewportHeight()));
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
      if (deltaY !== 0) syncHostWindowToViewport(deltaY);
    };
    const handleTouchEnd = () => {
      lastTouchY = null;
    };

    // The initial measurement reads the current window bounds and may move the
    // window. Keep those reads outside this setup effect's dependency graph;
    // otherwise a far-away reveal target and the viewport can repeatedly move
    // the window back and forth until Solid exhausts the call stack.
    untrack(handleViewportChange);
    scrollTarget.addEventListener('scroll', handleViewportChange, { passive: true });
    scrollTarget.addEventListener('wheel', handleWheel, { passive: false });
    scrollTarget.addEventListener('touchstart', handleTouchStart, { passive: true });
    scrollTarget.addEventListener('touchmove', handleTouchMove, { passive: false });
    scrollTarget.addEventListener('touchend', handleTouchEnd, { passive: true });
    scrollTarget.addEventListener('touchcancel', handleTouchEnd, { passive: true });
    window.addEventListener('resize', handleViewportChange);
    onCleanup(() => {
      scrollTarget.removeEventListener('scroll', handleViewportChange);
      scrollTarget.removeEventListener('wheel', handleWheel);
      scrollTarget.removeEventListener('touchstart', handleTouchStart);
      scrollTarget.removeEventListener('touchmove', handleTouchMove);
      scrollTarget.removeEventListener('touchend', handleTouchEnd);
      scrollTarget.removeEventListener('touchcancel', handleTouchEnd);
      window.removeEventListener('resize', handleViewportChange);
    });
  });

  return {
    setHostBodyRef,
  };
}
