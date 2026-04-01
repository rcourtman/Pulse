import { createMemo, type Accessor } from 'solid-js';
import {
  resolveSummaryActiveSeriesId,
  resolveSummaryCardInteractionState,
  type SummaryCardInteractionState,
} from './summaryCardInteraction';

type ContextualFocusSeries = {
  id?: string | null;
  name?: string | null;
};

export interface UseSummaryContextualFocusStateOptions<T extends ContextualFocusSeries> {
  focusedSeriesId?: Accessor<string | null | undefined>;
  getSeriesName?: (series: T) => string | null | undefined;
  hoveredSeriesId?: Accessor<string | null | undefined>;
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
  const hoveredSeriesId = options.hoveredSeriesId ?? (() => null);
  const focusedSeriesId = options.focusedSeriesId ?? (() => null);
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

  const effectiveFocusedSeriesId = createMemo<string | null>(() => {
    return hasInteractiveSeriesId(focusedSeriesId()) ? normalizeSeriesId(focusedSeriesId()) : null;
  });

  const activeSeriesId = createMemo<string | null>(() =>
    resolveSummaryActiveSeriesId({
      hoveredSeriesId: effectiveHoveredSeriesId(),
      focusedSeriesId: effectiveFocusedSeriesId(),
    }),
  );

  const getFocusedSeriesName = (series: readonly T[]): string | null => {
    const focusedId = effectiveFocusedSeriesId();
    if (!focusedId) {
      return null;
    }
    const match = series.find((entry) => normalizeSeriesId(entry.id) === focusedId);
    return match ? getSeriesName(match) || null : null;
  };

  const interactionStateFor = (
    series: ReadonlyArray<{ id?: string | null }>,
  ): SummaryCardInteractionState =>
    resolveSummaryCardInteractionState({
      series,
      hoveredSeriesId: effectiveHoveredSeriesId(),
      focusedSeriesId: effectiveFocusedSeriesId(),
    });

  return {
    activeSeriesId,
    effectiveFocusedSeriesId,
    effectiveHoveredSeriesId,
    getFocusedSeriesName,
    hasInteractiveSeriesId,
    interactionStateFor,
  } as const;
}
