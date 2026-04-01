export type SummaryCardInteractionState = 'default' | 'active' | 'inactive';

type SummarySeriesIdentity = {
  id?: string | null;
};

const normalizeSeriesId = (value: string | null | undefined): string => value?.trim() || '';

export function resolveSummaryActiveSeriesId(options: {
  chartHoveredSeriesId?: string | null;
  hoveredSeriesId?: string | null;
  focusedSeriesId?: string | null;
}): string | null {
  const chartHoveredSeriesId = normalizeSeriesId(options.chartHoveredSeriesId);
  if (chartHoveredSeriesId) {
    return chartHoveredSeriesId;
  }

  const hoveredSeriesId = normalizeSeriesId(options.hoveredSeriesId);
  if (hoveredSeriesId) {
    return hoveredSeriesId;
  }

  const focusedSeriesId = normalizeSeriesId(options.focusedSeriesId);
  return focusedSeriesId || null;
}

export function resolveSummaryCardInteractionState(options: {
  series: readonly SummarySeriesIdentity[];
  chartHoveredSeriesId?: string | null;
  hoveredSeriesId?: string | null;
  focusedSeriesId?: string | null;
}): SummaryCardInteractionState {
  const activeSeriesId = resolveSummaryActiveSeriesId({
    chartHoveredSeriesId: options.chartHoveredSeriesId,
    hoveredSeriesId: options.hoveredSeriesId,
    focusedSeriesId: options.focusedSeriesId,
  });
  if (!activeSeriesId) {
    return 'default';
  }

  for (const series of options.series) {
    if (normalizeSeriesId(series.id) === activeSeriesId) {
      return 'active';
    }
  }

  return 'inactive';
}
