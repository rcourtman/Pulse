export type SummaryCardInteractionState = 'default' | 'active' | 'inactive';

type SummarySeriesIdentity = {
  id?: string | null;
};

export interface SummarySeriesGroupScope {
  id: string;
  label?: string | null;
  seriesIds: readonly string[];
}

const normalizeSeriesId = (value: string | null | undefined): string => value?.trim() || '';

export const normalizeSummarySeriesGroupScope = (
  scope: SummarySeriesGroupScope | null | undefined,
): SummarySeriesGroupScope | null => {
  if (!scope) {
    return null;
  }

  const id = normalizeSeriesId(scope.id);
  const seriesIds = Array.from(
    new Set((scope.seriesIds ?? []).map((value) => normalizeSeriesId(value)).filter(Boolean)),
  );

  if (!id || seriesIds.length === 0) {
    return null;
  }

  const label = typeof scope.label === 'string' ? scope.label.trim() : '';
  return {
    id,
    label: label || null,
    seriesIds,
  };
};

export const isSummarySeriesInGroupScope = (
  scope: SummarySeriesGroupScope | null | undefined,
  seriesId: string | null | undefined,
): boolean => {
  const normalizedScope = normalizeSummarySeriesGroupScope(scope);
  const normalizedSeriesId = normalizeSeriesId(seriesId);
  if (!normalizedScope || !normalizedSeriesId) {
    return false;
  }
  return normalizedScope.seriesIds.includes(normalizedSeriesId);
};

export const resolveSummaryGroupScope = (options: {
  hoveredGroupScope?: SummarySeriesGroupScope | null;
  focusedGroupScope?: SummarySeriesGroupScope | null;
}): SummarySeriesGroupScope | null => {
  const hoveredGroupScope = normalizeSummarySeriesGroupScope(options.hoveredGroupScope);
  if (hoveredGroupScope) {
    return hoveredGroupScope;
  }
  return normalizeSummarySeriesGroupScope(options.focusedGroupScope);
};

export function resolveSummaryActiveSeriesId(options: {
  chartHoveredSeriesId?: string | null;
  hoveredSeriesId?: string | null;
  focusedSeriesId?: string | null;
  groupScope?: SummarySeriesGroupScope | null;
}): string | null {
  const groupScope = normalizeSummarySeriesGroupScope(options.groupScope);
  const chartHoveredSeriesId = normalizeSeriesId(options.chartHoveredSeriesId);
  if (chartHoveredSeriesId && (!groupScope || isSummarySeriesInGroupScope(groupScope, chartHoveredSeriesId))) {
    return chartHoveredSeriesId;
  }

  const hoveredSeriesId = normalizeSeriesId(options.hoveredSeriesId);
  if (hoveredSeriesId && (!groupScope || isSummarySeriesInGroupScope(groupScope, hoveredSeriesId))) {
    return hoveredSeriesId;
  }

  const focusedSeriesId = normalizeSeriesId(options.focusedSeriesId);
  if (focusedSeriesId && (!groupScope || isSummarySeriesInGroupScope(groupScope, focusedSeriesId))) {
    return focusedSeriesId;
  }

  return null;
}

export function filterSummarySeriesByGroupScope<T extends SummarySeriesIdentity>(
  series: readonly T[],
  groupScope?: SummarySeriesGroupScope | null,
): T[] {
  const normalizedScope = normalizeSummarySeriesGroupScope(groupScope);
  if (!normalizedScope) {
    return [...series];
  }
  return series.filter((entry) => isSummarySeriesInGroupScope(normalizedScope, entry.id));
}

export function resolveSummaryCardInteractionState(options: {
  series: readonly SummarySeriesIdentity[];
  chartHoveredSeriesId?: string | null;
  hoveredSeriesId?: string | null;
  focusedSeriesId?: string | null;
  hoveredGroupScope?: SummarySeriesGroupScope | null;
  focusedGroupScope?: SummarySeriesGroupScope | null;
  groupScope?: SummarySeriesGroupScope | null;
}): SummaryCardInteractionState {
  const groupScope =
    normalizeSummarySeriesGroupScope(options.groupScope) ??
    resolveSummaryGroupScope({
      hoveredGroupScope: options.hoveredGroupScope,
      focusedGroupScope: options.focusedGroupScope,
    });
  const activeSeriesId = resolveSummaryActiveSeriesId({
    chartHoveredSeriesId: options.chartHoveredSeriesId,
    hoveredSeriesId: options.hoveredSeriesId,
    focusedSeriesId: options.focusedSeriesId,
    groupScope,
  });
  if (!activeSeriesId) {
    if (groupScope) {
      for (const series of options.series) {
        if (isSummarySeriesInGroupScope(groupScope, series.id)) {
          return 'active';
        }
      }
      return 'inactive';
    }
    return 'default';
  }

  for (const series of options.series) {
    if (normalizeSeriesId(series.id) === activeSeriesId) {
      return 'active';
    }
  }

  return 'inactive';
}
