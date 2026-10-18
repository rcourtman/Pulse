export type SummaryScopeKind = 'page' | 'entity';
export type SummaryScopeSource = 'page' | 'preview' | 'pinned';

export interface SummaryScopeState {
  kind: SummaryScopeKind;
  seriesId: string | null;
  source: SummaryScopeSource;
}

const normalizeSeriesId = (value: string | null | undefined): string => value?.trim() || '';

export const resolveSummaryScopeState = (options: {
  hoveredSeriesId?: string | null;
  focusedSeriesId?: string | null;
}): SummaryScopeState => {
  const hoveredSeriesId = normalizeSeriesId(options.hoveredSeriesId);
  if (hoveredSeriesId) {
    return {
      kind: 'entity',
      seriesId: hoveredSeriesId,
      source: 'preview',
    };
  }

  const focusedSeriesId = normalizeSeriesId(options.focusedSeriesId);
  if (focusedSeriesId) {
    return {
      kind: 'entity',
      seriesId: focusedSeriesId,
      source: 'pinned',
    };
  }

  return {
    kind: 'page',
    seriesId: null,
    source: 'page',
  };
};

export function resolveSummaryActiveSeriesId(options: {
  hoveredSeriesId?: string | null;
  focusedSeriesId?: string | null;
}): string | null {
  return resolveSummaryScopeState(options).seriesId;
}
