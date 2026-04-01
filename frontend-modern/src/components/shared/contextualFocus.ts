import { createMemo, type Accessor } from 'solid-js';
import { markRouteStateDeliberateScroll } from '@/utils/routeStateNavigation';
import {
  filterSummarySeriesByGroupScope,
  resolveSummaryActiveSeriesId,
  resolveSummaryCardInteractionState,
  resolveSummaryGroupScope,
  type SummarySeriesGroupScope,
  type SummaryCardInteractionState,
} from './summaryCardInteraction';

type ContextualFocusSeries = {
  id?: string | null;
  name?: string | null;
};

const INLINE_DETAIL_TARGET_TOP_RATIO = 0.28;
const INLINE_DETAIL_MIN_TOP_MARGIN = 72;
const INLINE_DETAIL_MIN_DETAIL_PEEK = 160;

export interface SummaryChartHoverSync {
  sourceKey: string;
  seriesId: string;
  timestamp: number;
}

export interface UseSummaryContextualFocusStateOptions<T extends ContextualFocusSeries> {
  chartHoveredSeriesId?: Accessor<string | null | undefined>;
  focusedSeriesId?: Accessor<string | null | undefined>;
  focusedGroupScope?: Accessor<SummarySeriesGroupScope | null | undefined>;
  getSeriesName?: (series: T) => string | null | undefined;
  hoveredSeriesId?: Accessor<string | null | undefined>;
  hoveredGroupScope?: Accessor<SummarySeriesGroupScope | null | undefined>;
  interactiveSeries: Accessor<readonly T[]>;
  isSeriesInteractive?: (series: T) => boolean;
}

const normalizeSeriesId = (value: string | null | undefined): string => value?.trim() || '';

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
  if (isWindowScroller(scroller) || typeof (scroller as HTMLElement).getBoundingClientRect !== 'function') {
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
  const minTopMargin = Math.max(
    INLINE_DETAIL_MIN_TOP_MARGIN,
    Math.round(metrics.height * 0.12),
  );
  const preferredTop = Math.max(
    minTopMargin,
    Math.round(metrics.height * INLINE_DETAIL_TARGET_TOP_RATIO),
  );
  const detailPeek = Math.max(
    INLINE_DETAIL_MIN_DETAIL_PEEK,
    Math.round(metrics.height * 0.24),
  );
  const currentRowOffset = rowRect.top - metrics.top;
  const rowHasBreathingRoom = currentRowOffset >= minTopMargin;
  const detailHasPeek =
    !detailRect || detailRect.top - metrics.top <= metrics.height - detailPeek;

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

export function useSummaryContextualFocusState<T extends ContextualFocusSeries>(
  options: UseSummaryContextualFocusStateOptions<T>,
) {
  const chartHoveredSeriesId = options.chartHoveredSeriesId ?? (() => null);
  const hoveredSeriesId = options.hoveredSeriesId ?? (() => null);
  const focusedSeriesId = options.focusedSeriesId ?? (() => null);
  const hoveredGroupScope = options.hoveredGroupScope ?? (() => null);
  const focusedGroupScope = options.focusedGroupScope ?? (() => null);
  const getSeriesName =
    options.getSeriesName ??
    ((series: T) => (typeof series.name === 'string' ? series.name : null));
  const isSeriesInteractive =
    options.isSeriesInteractive ?? ((series: T) => normalizeSeriesId(series.id) !== '');

  const interactiveSeriesIds = createMemo(() => {
    const ids = new Set<string>();
    for (const series of options.interactiveSeries()) {
      if (!isSeriesInteractive(series)) {
        continue;
      }
      const id = normalizeSeriesId(series.id);
      if (id) {
        ids.add(id);
      }
    }
    return ids;
  });

  const hasInteractiveSeriesId = (value: string | null | undefined): value is string => {
    const normalized = normalizeSeriesId(value);
    return normalized !== '' && interactiveSeriesIds().has(normalized);
  };

  const effectiveHoveredSeriesId = createMemo<string | null>(() => {
    return hasInteractiveSeriesId(hoveredSeriesId()) ? normalizeSeriesId(hoveredSeriesId()) : null;
  });

  const effectiveChartHoveredSeriesId = createMemo<string | null>(() => {
    return hasInteractiveSeriesId(chartHoveredSeriesId())
      ? normalizeSeriesId(chartHoveredSeriesId())
      : null;
  });

  const effectiveFocusedSeriesId = createMemo<string | null>(() => {
    return hasInteractiveSeriesId(focusedSeriesId()) ? normalizeSeriesId(focusedSeriesId()) : null;
  });

  const normalizeGroupScope = (
    scope: SummarySeriesGroupScope | null | undefined,
  ): SummarySeriesGroupScope | null => {
    const resolvedScope = resolveSummaryGroupScope({
      hoveredGroupScope: scope,
    });
    if (!resolvedScope) {
      return null;
    }
    const scopedSeriesIds = resolvedScope.seriesIds.filter((id) => interactiveSeriesIds().has(id));
    if (scopedSeriesIds.length === 0) {
      return null;
    }
    return {
      ...resolvedScope,
      seriesIds: scopedSeriesIds,
    };
  };

  const effectiveHoveredGroupScope = createMemo<SummarySeriesGroupScope | null>(() =>
    normalizeGroupScope(hoveredGroupScope()),
  );

  const effectiveFocusedGroupScope = createMemo<SummarySeriesGroupScope | null>(() =>
    normalizeGroupScope(focusedGroupScope()),
  );

  const activeGroupScope = createMemo<SummarySeriesGroupScope | null>(() =>
    resolveSummaryGroupScope({
      hoveredGroupScope: effectiveHoveredGroupScope(),
      focusedGroupScope: effectiveFocusedGroupScope(),
    }),
  );

  const activeSeriesId = createMemo<string | null>(() => {
    return resolveSummaryActiveSeriesId({
      chartHoveredSeriesId: effectiveChartHoveredSeriesId(),
      hoveredSeriesId: effectiveHoveredSeriesId(),
      focusedSeriesId: effectiveFocusedSeriesId(),
      groupScope: activeGroupScope(),
    });
  });

  const getFocusedSeriesName = (series: readonly T[]): string | null => {
    const focusedId = effectiveFocusedSeriesId();
    if (!focusedId) {
      return null;
    }
    const match = series.find((entry) => normalizeSeriesId(entry.id) === focusedId);
    return match ? getSeriesName(match) || null : null;
  };

  const getActiveSeriesName = (series: readonly T[]): string | null => {
    const currentActiveId = activeSeriesId();
    if (currentActiveId) {
      const match = series.find((entry) => normalizeSeriesId(entry.id) === currentActiveId);
      return match ? getSeriesName(match) || null : null;
    }
    return activeGroupScope()?.label || null;
  };

  const interactionStateFor = (
    series: ReadonlyArray<{ id?: string | null }>,
  ): SummaryCardInteractionState =>
    resolveSummaryCardInteractionState({
      series,
      chartHoveredSeriesId: effectiveChartHoveredSeriesId(),
      hoveredSeriesId: effectiveHoveredSeriesId(),
      focusedSeriesId: effectiveFocusedSeriesId(),
      groupScope: activeGroupScope(),
    });

  const filterSeriesForActiveScope = <S extends { id?: string | null }>(
    series: readonly S[],
  ): S[] => {
    return filterSummarySeriesByGroupScope(series, activeGroupScope());
  };

  const isSeriesIdVisibleInActiveScope = (value: string | null | undefined): value is string => {
    const normalized = normalizeSeriesId(value);
    if (!normalized || !interactiveSeriesIds().has(normalized)) {
      return false;
    }
    const groupScope = activeGroupScope();
    if (!groupScope) {
      return true;
    }
    return groupScope.seriesIds.includes(normalized);
  };

  return {
    activeGroupScope,
    activeSeriesId,
    effectiveChartHoveredSeriesId,
    effectiveFocusedSeriesId,
    effectiveFocusedGroupScope,
    effectiveHoveredSeriesId,
    effectiveHoveredGroupScope,
    filterSeriesForActiveScope,
    getActiveSeriesName,
    getFocusedSeriesName,
    hasInteractiveSeriesId,
    isSeriesIdVisibleInActiveScope,
    interactionStateFor,
  } as const;
}
