import { createMemo, type Accessor } from 'solid-js';
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
