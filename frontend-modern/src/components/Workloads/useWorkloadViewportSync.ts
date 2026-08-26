import { createEffect, createSignal, onCleanup, untrack, type Accessor } from 'solid-js';

import { findInlineDetailElement } from '@/components/shared/contextualFocus';
import {
  bindWindowedPageScrollEvents,
  findWindowedPageScrollContainer,
  isWindowedSurfaceHidden,
  wheelDeltaInPixels,
} from '@/components/shared/windowedPageScroll';

import type { UseGroupedTableWindowingResult } from './useGroupedTableWindowing';

const SCROLL_TO_TOP_VISIBILITY_THRESHOLD_PX = 640;

interface WorkloadsWorkloadViewportSyncOptions {
  expandedDetailActive?: Accessor<boolean>;
  filteredGuestCount: Accessor<number>;
  groupedWindowing: UseGroupedTableWindowingResult;
  onExpandedDetailHeightChange?: (height: number) => void;
  onRowGeometryChange?: (geometry: { groupHeaderHeight?: number; rowHeight?: number }) => void;
  rowHeight: Accessor<number>;
  selectedGuestId?: Accessor<string | null>;
  tableBodyRef: Accessor<HTMLTableSectionElement | null>;
}

export function useWorkloadViewportSync(options: WorkloadsWorkloadViewportSyncOptions) {
  const [isScrollToTopVisible, setIsScrollToTopVisible] = createSignal(false);

  const reportExpandedDetailHeight = (body: HTMLTableSectionElement, selectedGuestId: string) => {
    const detail = findInlineDetailElement(body, selectedGuestId);
    const detailHeight = detail?.getBoundingClientRect().height ?? 0;
    if (detailHeight > 0) options.onExpandedDetailHeightChange?.(detailHeight);
    return detail;
  };

  const syncGuestWindowToViewport = (measureRows = false, projectedScrollDelta = 0) => {
    if (typeof window === 'undefined') return;
    const body = options.tableBodyRef();
    if (!body || isWindowedSurfaceHidden(body)) return;
    const selectedGuestId = options.selectedGuestId?.();
    if (selectedGuestId) {
      reportExpandedDetailHeight(body, selectedGuestId);
    }
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
    const scrollContainer = findWindowedPageScrollContainer(body);
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
    const selectedGuestId = options.selectedGuestId?.();
    const body = options.tableBodyRef();
    if (!selectedGuestId || !body) return;

    let detailObserver: ResizeObserver | undefined;
    const measurementFrame = window.requestAnimationFrame(() => {
      const detail = reportExpandedDetailHeight(body, selectedGuestId);
      if (!detail || typeof ResizeObserver === 'undefined') return;
      detailObserver = new ResizeObserver(() => {
        reportExpandedDetailHeight(body, selectedGuestId);
      });
      detailObserver.observe(detail);
    });

    onCleanup(() => {
      window.cancelAnimationFrame(measurementFrame);
      detailObserver?.disconnect();
    });
  });

  createEffect(() => {
    if (typeof window === 'undefined') return;
    options.filteredGuestCount();
    if (!options.tableBodyRef()) return;

    const handleViewportScroll = () => {
      syncGuestWindowToViewport();
    };
    const handleViewportWheel = (event: Event) => {
      const wheelEvent = event as WheelEvent;
      // Let native input move first while a variable-height drawer is open.
      // Prewarming can replace that keyed detail row before the browser applies
      // its delta, causing scroll anchoring to consume the drawer height.
      if (
        !options.groupedWindowing.isWindowed() ||
        options.expandedDetailActive?.() ||
        wheelEvent.deltaY === 0
      ) {
        return;
      }
      const viewportHeight =
        scrollTarget instanceof HTMLElement
          ? scrollTarget.clientHeight || window.innerHeight
          : window.innerHeight;
      syncGuestWindowToViewport(false, wheelDeltaInPixels(wheelEvent, viewportHeight));
    };
    const handleViewportResize = () => {
      syncGuestWindowToViewport(true);
    };

    // The initial measurement pass reads windowing signals (startIndex via
    // groupedWindowing.onScroll) that the pass itself moves. Tracking them
    // would re-run this effect — remeasuring rows with forced reflows and
    // rebinding listeners — on every runway top-up during scrolling.
    untrack(() => handleViewportResize());
    const scrollContainer = findWindowedPageScrollContainer(options.tableBodyRef()!);
    const scrollTarget = scrollContainer ?? window;
    onCleanup(
      bindWindowedPageScrollEvents({
        scrollTarget,
        onScroll: handleViewportScroll,
        onWheel: handleViewportWheel,
        onResize: handleViewportResize,
      }),
    );
  });

  const scrollToTop = () => {
    if (typeof window === 'undefined') return;
    const body = options.tableBodyRef();
    const scrollContainer = body ? findWindowedPageScrollContainer(body) : null;
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
