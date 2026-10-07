import { markRouteStateDeliberateScroll } from '@/utils/routeStateNavigation';

const INLINE_DETAIL_TARGET_TOP_RATIO = 0.28;
const INLINE_DETAIL_MIN_TOP_MARGIN = 72;
const INLINE_DETAIL_MIN_DETAIL_PEEK = 160;

export interface SummaryChartHoverSync {
  sourceKey: string;
  seriesId: string;
  timestamp: number;
}

const resolveScrollableAncestor = (element: Element | null | undefined): HTMLElement | null => {
  let scroller = element instanceof HTMLElement ? element : null;
  while (scroller) {
    const { overflowY } = getComputedStyle(scroller);
    if (
      (overflowY === 'auto' || overflowY === 'scroll') &&
      scroller.scrollHeight > scroller.clientHeight
    ) {
      return scroller;
    }
    scroller = scroller.parentElement;
  }
  return null;
};

const isWindowScroller = (scroller: Window | HTMLElement): scroller is Window => {
  return typeof window !== 'undefined' && scroller === window;
};

const resolveVerticalScroller = (element: Element | null | undefined): Window | HTMLElement => {
  if (typeof window === 'undefined') {
    throw new Error('resolveVerticalScroller requires a browser environment');
  }
  return resolveScrollableAncestor(element) ?? window;
};

const getScrollerMetrics = (scroller: Window | HTMLElement) => {
  if (
    isWindowScroller(scroller) ||
    typeof (scroller as HTMLElement).getBoundingClientRect !== 'function'
  ) {
    return {
      top: 0,
      bottom: window.innerHeight,
      height: window.innerHeight,
      scrollTop: window.scrollY,
    };
  }

  const rect = scroller.getBoundingClientRect();
  return {
    top: rect.top,
    bottom: rect.bottom,
    height: scroller.clientHeight,
    scrollTop: scroller.scrollTop,
  };
};

const scrollVerticalScroller = (
  scroller: Window | HTMLElement,
  top: number,
  behavior: ScrollBehavior,
) => {
  const clampedTop = Math.max(0, top);
  markRouteStateDeliberateScroll();
  const canUseScrollTo = (() => {
    if (typeof scroller.scrollTo !== 'function') {
      return false;
    }
    if (
      isWindowScroller(scroller) &&
      typeof window !== 'undefined' &&
      /jsdom/i.test(window.navigator.userAgent) &&
      !('mock' in scroller.scrollTo)
    ) {
      return false;
    }
    try {
      return !Function.prototype.toString.call(scroller.scrollTo).includes('notImplemented');
    } catch {
      return true;
    }
  })();
  try {
    if (isWindowScroller(scroller)) {
      if (canUseScrollTo) {
        scroller.scrollTo({ top: clampedTop, behavior });
        return;
      }
      document.documentElement.scrollTop = clampedTop;
      document.body.scrollTop = clampedTop;
      return;
    }
    if (!canUseScrollTo) {
      scroller.scrollTop = clampedTop;
      return;
    }
    scroller.scrollTo({ top: clampedTop, behavior });
  } catch {
    // JSDOM can throw for unimplemented scroll APIs. Ignore that path in tests.
  }
};

// Leave a visible target anchored. Only off-screen navigation needs a reveal.
export const revealElementInViewport = (options: {
  element: HTMLElement;
  bottomPadding?: number;
  behavior?: ScrollBehavior;
}): boolean => {
  if (typeof window === 'undefined') return false;
  const scroller = resolveVerticalScroller(options.element);
  const metrics = getScrollerMetrics(scroller);
  const rect = options.element.getBoundingClientRect();
  const top = Math.max(0, metrics.top);
  const bottom = Math.min(window.innerHeight, metrics.bottom);
  if (rect.top >= top && rect.bottom <= bottom) return false;
  const offset =
    rect.top < top ? rect.top - top : rect.bottom - bottom + (options.bottomPadding ?? 0);
  scrollVerticalScroller(scroller, metrics.scrollTop + offset, options.behavior ?? 'instant');
  return true;
};

export const findInlineDetailElement = (
  root: ParentNode | null | undefined,
  seriesId: string | null | undefined,
): HTMLElement | null => {
  const normalizedId = seriesId?.trim() || '';
  if (!root || !normalizedId) {
    return null;
  }
  return root.querySelector<HTMLElement>(`[data-inline-detail-for="${normalizedId}"]`);
};

export const revealInlineDetailInViewport = (options: {
  row: HTMLElement;
  detail?: HTMLElement | null;
  behavior?: ScrollBehavior;
}): boolean => {
  if (typeof window === 'undefined') {
    return false;
  }

  const scroller = resolveVerticalScroller(options.row);
  const metrics = getScrollerMetrics(scroller);
  const rowRect = options.row.getBoundingClientRect();
  const detailRect = options.detail?.getBoundingClientRect() ?? null;
  const minTopMargin = Math.max(INLINE_DETAIL_MIN_TOP_MARGIN, Math.round(metrics.height * 0.12));
  const preferredTop = Math.max(
    minTopMargin,
    Math.round(metrics.height * INLINE_DETAIL_TARGET_TOP_RATIO),
  );
  const detailPeek = Math.max(INLINE_DETAIL_MIN_DETAIL_PEEK, Math.round(metrics.height * 0.24));
  const currentRowOffset = rowRect.top - metrics.top;
  const rowHasBreathingRoom = currentRowOffset >= minTopMargin;
  const detailHasPeek = !detailRect || detailRect.top - metrics.top <= metrics.height - detailPeek;

  if (rowHasBreathingRoom && detailHasPeek) {
    return false;
  }

  const desiredRowOffset =
    currentRowOffset > preferredTop ? preferredTop : Math.max(minTopMargin, currentRowOffset);
  const nextScrollTop = metrics.scrollTop + currentRowOffset - desiredRowOffset;
  scrollVerticalScroller(scroller, nextScrollTop, options.behavior ?? 'smooth');
  return true;
};

export const preserveScrollableAncestorVerticalOffset = (
  anchor: Element | null | undefined,
  apply: () => void,
): void => {
  const scroller = resolveScrollableAncestor(anchor);
  const scrollTop = scroller?.scrollTop ?? null;

  apply();

  if (!scroller || scrollTop === null) {
    return;
  }

  scroller.scrollTop = scrollTop;
  if (typeof window === 'undefined' || typeof window.requestAnimationFrame !== 'function') {
    return;
  }
  window.requestAnimationFrame(() => {
    if (scroller.isConnected) {
      scroller.scrollTop = scrollTop;
    }
  });
};
