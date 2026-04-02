export type SummaryCardInteractionState = 'default' | 'active' | 'inactive';
export type SummaryGroupMemberInteractionState = 'default' | 'preview' | 'pinned';
export type SummaryScopeKind = 'page' | 'group' | 'entity';
export type SummaryScopeSource = 'page' | 'preview' | 'pinned';

type SummarySeriesIdentity = {
  id?: string | null;
};

export interface SummarySeriesGroupScope {
  id: string;
  label?: string | null;
  seriesIds: readonly string[];
}

export interface SummaryScopeState {
  groupScope: SummarySeriesGroupScope | null;
  kind: SummaryScopeKind;
  seriesId: string | null;
  source: SummaryScopeSource;
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

export const resolveSummaryGroupMemberInteractionState = (options: {
  seriesId?: string | null;
  hoveredGroupScope?: SummarySeriesGroupScope | null;
  focusedGroupScope?: SummarySeriesGroupScope | null;
}): SummaryGroupMemberInteractionState => {
  const normalizedSeriesId = normalizeSeriesId(options.seriesId);
  if (!normalizedSeriesId) {
    return 'default';
  }

  const hoveredGroupScope = normalizeSummarySeriesGroupScope(options.hoveredGroupScope);
  if (isSummarySeriesInGroupScope(hoveredGroupScope, normalizedSeriesId)) {
    return 'preview';
  }

  const focusedGroupScope = normalizeSummarySeriesGroupScope(options.focusedGroupScope);
  if (isSummarySeriesInGroupScope(focusedGroupScope, normalizedSeriesId)) {
    return 'pinned';
  }

  return 'default';
};

export const resolveSummaryScopeState = (options: {
  chartHoveredSeriesId?: string | null;
  hoveredSeriesId?: string | null;
  focusedSeriesId?: string | null;
  hoveredGroupScope?: SummarySeriesGroupScope | null;
  focusedGroupScope?: SummarySeriesGroupScope | null;
  groupScope?: SummarySeriesGroupScope | null;
}): SummaryScopeState => {
  const groupScope =
    normalizeSummarySeriesGroupScope(options.groupScope) ??
    resolveSummaryGroupScope({
      hoveredGroupScope: options.hoveredGroupScope,
      focusedGroupScope: options.focusedGroupScope,
    });

  const chartHoveredSeriesId = normalizeSeriesId(options.chartHoveredSeriesId);
  if (chartHoveredSeriesId && (!groupScope || isSummarySeriesInGroupScope(groupScope, chartHoveredSeriesId))) {
    return {
      groupScope,
      kind: 'entity',
      seriesId: chartHoveredSeriesId,
      source: 'preview',
    };
  }

  const hoveredSeriesId = normalizeSeriesId(options.hoveredSeriesId);
  if (hoveredSeriesId && (!groupScope || isSummarySeriesInGroupScope(groupScope, hoveredSeriesId))) {
    return {
      groupScope,
      kind: 'entity',
      seriesId: hoveredSeriesId,
      source: 'preview',
    };
  }

  const focusedSeriesId = normalizeSeriesId(options.focusedSeriesId);
  if (focusedSeriesId && (!groupScope || isSummarySeriesInGroupScope(groupScope, focusedSeriesId))) {
    return {
      groupScope,
      kind: 'entity',
      seriesId: focusedSeriesId,
      source: 'pinned',
    };
  }

  const hoveredGroupScope = normalizeSummarySeriesGroupScope(options.hoveredGroupScope);
  if (hoveredGroupScope) {
    return {
      groupScope: hoveredGroupScope,
      kind: 'group',
      seriesId: null,
      source: 'preview',
    };
  }

  const focusedGroupScope = normalizeSummarySeriesGroupScope(options.focusedGroupScope);
  if (focusedGroupScope) {
    return {
      groupScope: focusedGroupScope,
      kind: 'group',
      seriesId: null,
      source: 'pinned',
    };
  }

  return {
    groupScope: null,
    kind: 'page',
    seriesId: null,
    source: 'page',
  };
};

export function resolveSummaryActiveSeriesId(options: {
  chartHoveredSeriesId?: string | null;
  hoveredSeriesId?: string | null;
  focusedSeriesId?: string | null;
  groupScope?: SummarySeriesGroupScope | null;
}): string | null {
  return resolveSummaryScopeState(options).seriesId;
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
