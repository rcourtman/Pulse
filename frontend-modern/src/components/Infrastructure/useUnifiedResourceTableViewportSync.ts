import { createEffect, createSignal, onCleanup, untrack, type Accessor } from 'solid-js';
import {
  bindWindowedPageScrollEvents,
  findWindowedPageScrollContainer,
  wheelDeltaInPixels,
} from '@/components/shared/windowedPageScroll';
import { useTableWindowing } from './useTableWindowing';

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
    const scrollContainer = findWindowedPageScrollContainer(body);
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

    const body = hostBodyRef()!;
    const scrollContainer = findWindowedPageScrollContainer(body);
    const scrollTarget: HTMLElement | Window = scrollContainer ?? window;
    const viewportHeight = () => scrollContainer?.clientHeight || window.innerHeight;
    const handleViewportChange = () => syncHostWindowToViewport();
    const handleWheel = (event: Event) => {
      const wheelEvent = event as WheelEvent;
      if (wheelEvent.deltaY === 0) return;
      syncHostWindowToViewport(wheelDeltaInPixels(wheelEvent, viewportHeight()));
    };
    // The initial measurement reads the current window bounds and may move the
    // window. Keep those reads outside this setup effect's dependency graph;
    // otherwise a far-away reveal target and the viewport can repeatedly move
    // the window back and forth until Solid exhausts the call stack.
    untrack(handleViewportChange);
    onCleanup(
      bindWindowedPageScrollEvents({
        scrollTarget,
        onScroll: handleViewportChange,
        onWheel: handleWheel,
        onResize: handleViewportChange,
      }),
    );
  });

  return {
    setHostBodyRef,
  };
}
