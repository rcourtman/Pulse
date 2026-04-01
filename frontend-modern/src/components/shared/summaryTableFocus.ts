import { createMemo, createSignal, type Accessor } from 'solid-js';
import type { SummaryChartHoverSync } from './contextualFocus';
import {
  resolveSummaryActiveSeriesId,
  resolveSummaryGroupScope,
  type SummarySeriesGroupScope,
} from './summaryCardInteraction';

const normalizeSeriesId = (value: string | null | undefined): string => value?.trim() || '';

const escapeAttributeSelectorValue = (value: string): string =>
  value.split('\\').join('\\\\').split('"').join('\\"');

const isElementVisibleWithinViewport = (element: HTMLElement): boolean => {
  const rect = element.getBoundingClientRect();
  if (rect.height <= 0 || rect.width <= 0) {
    return false;
  }
  if (typeof window === 'undefined') {
    return true;
  }

  if (rect.bottom <= 0 || rect.top >= window.innerHeight) {
    return false;
  }

  let current: HTMLElement | null = element.parentElement;
  while (current) {
    const style = getComputedStyle(current);
    const clipsVertically =
      (style.overflowY === 'auto' || style.overflowY === 'scroll' || style.overflowY === 'hidden') &&
      current.scrollHeight > current.clientHeight;
    if (clipsVertically) {
      const containerRect = current.getBoundingClientRect();
      if (rect.bottom <= containerRect.top || rect.top >= containerRect.bottom) {
        return false;
      }
    }
    current = current.parentElement;
  }

  return true;
};

export interface UseSummaryTableFocusBridgeOptions {
  activeSeriesId: Accessor<string | null | undefined>;
  revealActiveSeries?: (seriesId: string) => void;
}

export function useSummaryTableFocusBridge(options: UseSummaryTableFocusBridgeOptions) {
  const [tableRoot, setTableRoot] = createSignal<HTMLElement | null>(null);

  const normalizedActiveSeriesId = createMemo<string | null>(() => {
    const normalized = normalizeSeriesId(options.activeSeriesId());
    return normalized || null;
  });

  const activeRow = (): HTMLElement | null => {
    const root = tableRoot();
    const activeSeriesId = normalizedActiveSeriesId();
    if (!root || !activeSeriesId) {
      return null;
    }
    return root.querySelector<HTMLElement>(
      `[data-summary-series-id="${escapeAttributeSelectorValue(activeSeriesId)}"]`,
    );
  };

  const isActiveRowVisible = createMemo<boolean>(() => {
    const row = activeRow();
    if (!row) {
      return false;
    }
    return isElementVisibleWithinViewport(row);
  });

  const shouldShowJumpToActiveRow = createMemo<boolean>(() => {
    return Boolean(normalizedActiveSeriesId()) && !isActiveRowVisible();
  });

  const jumpToActiveRow = () => {
    const activeSeriesId = normalizedActiveSeriesId();
    if (!activeSeriesId) {
      return;
    }

    options.revealActiveSeries?.(activeSeriesId);

    const attemptScroll = (remainingFrames: number) => {
      const row = activeRow();
      if (row) {
        row.scrollIntoView({ behavior: 'smooth', block: 'center' });
        return;
      }
      if (remainingFrames <= 0 || typeof window === 'undefined') {
        return;
      }
      window.requestAnimationFrame(() => attemptScroll(remainingFrames - 1));
    };

    attemptScroll(6);
  };

  return {
    activeRow,
    isActiveRowVisible,
    jumpToActiveRow,
    setTableRootRef: (element: HTMLElement | undefined) => setTableRoot(element ?? null),
    shouldShowJumpToActiveRow,
  } as const;
}

export interface UseSummaryPageInteractionStateOptions {
  hoveredSeriesId?: Accessor<string | null | undefined>;
  focusedSeriesId?: Accessor<string | null | undefined>;
  hoveredGroupScope?: Accessor<SummarySeriesGroupScope | null | undefined>;
  focusedGroupScope?: Accessor<SummarySeriesGroupScope | null | undefined>;
  revealActiveSeries?: (seriesId: string) => void;
}

export function useSummaryPageInteractionState(options: UseSummaryPageInteractionStateOptions) {
  const [chartHoverSync, setChartHoverSync] = createSignal<SummaryChartHoverSync | null>(null);
  const hoveredSeriesId = options.hoveredSeriesId ?? (() => null);
  const focusedSeriesId = options.focusedSeriesId ?? (() => null);
  const hoveredGroupScope = options.hoveredGroupScope ?? (() => null);
  const focusedGroupScope = options.focusedGroupScope ?? (() => null);

  const activeGroupScope = createMemo<SummarySeriesGroupScope | null>(() =>
    resolveSummaryGroupScope({
      hoveredGroupScope: hoveredGroupScope(),
      focusedGroupScope: focusedGroupScope(),
    }),
  );

  const activeSeriesId = createMemo<string | null>(() =>
    resolveSummaryActiveSeriesId({
      chartHoveredSeriesId: chartHoverSync()?.seriesId ?? null,
      hoveredSeriesId: hoveredSeriesId(),
      focusedSeriesId: focusedSeriesId(),
      groupScope: activeGroupScope(),
    }),
  );

  const tableFocus = useSummaryTableFocusBridge({
    activeSeriesId,
    revealActiveSeries: options.revealActiveSeries,
  });

  return {
    activeGroupScope,
    activeSeriesId,
    chartHoverSync,
    jumpToActiveRow: tableFocus.jumpToActiveRow,
    setChartHoverSync,
    setTableRootRef: tableFocus.setTableRootRef,
    shouldShowJumpToActiveRow: tableFocus.shouldShowJumpToActiveRow,
  } as const;
}
