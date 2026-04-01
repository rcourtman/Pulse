import type { TimeRange } from '@/api/charts';
import { timeRangeToMs } from '@/utils/timeRange';
import type { InteractiveSparklineSeries } from './InteractiveSparkline';
import type { SummaryChartHoverSync } from './contextualFocus';
import type { SummaryCardInteractionState } from './summaryCardInteraction';

export interface DensityMapProps {
  series: InteractiveSparklineSeries[];
  rangeLabel?: string;
  timeRange?: TimeRange;
  formatValue?: (value: number) => string;
  focusEmptyStateLabel?: string;
  hoverSourceKey?: string;
  hoverSync?: SummaryChartHoverSync | null;
  onHoverSyncChange?: (value: SummaryChartHoverSync | null) => void;
  highlightSeriesId?: string | null;
  interactionState?: SummaryCardInteractionState;
}

export interface DensityMapHoveredState {
  columnIndex: number;
  tooltipX: number;
  tooltipY: number;
  timestamp: number;
  value: number;
  seriesName: string;
  seriesColor: string;
  seriesIndex: number;
}

export interface DensityMapFocusDetail {
  peakValue: number | null;
  seriesColor: string;
  seriesId: string;
  seriesName: string;
}

export interface DensityMapChartData {
  series: InteractiveSparklineSeries[];
  grid: number[][];
  globalMax: number;
  windowStart: number;
  rangeMs: number;
  bucketDuration: number;
}

export const DENSITY_MAP_COLUMNS = 45;
export const DENSITY_MAP_PADDING_Y = 2;
export const DENSITY_MAP_PADDING_X = 2;

export function clampDensityMapValue(value: number, min: number, max: number): number {
  return Math.max(min, Math.min(value, max));
}

export function formatDensityMapHoverTime(timestamp: number): string {
  return new Date(timestamp).toLocaleString([], {
    month: 'short',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
    hour12: false,
  });
}

export function formatDensityMapValue(
  value: number,
  formatter?: (value: number) => string,
): string {
  return formatter ? formatter(value) : `${value.toFixed(1)}`;
}

export function buildDensityMapChartData(options: {
  series: InteractiveSparklineSeries[];
  timeRange?: TimeRange;
  now?: number;
  highlightSeriesId?: string | null;
}): DensityMapChartData {
  const range = options.timeRange || '1h';
  const rangeMs = timeRangeToMs(range);
  const windowEnd = options.now ?? Date.now();
  const windowStart = windowEnd - rangeMs;
  const bucketDuration = rangeMs / DENSITY_MAP_COLUMNS;

  const activeSeries = options.series.filter(
    (series) => series.data.length > 0 || series.id === options.highlightSeriesId,
  );
  const seriesWithVolume = activeSeries.map((series) => {
    let total = 0;
    for (const point of series.data) {
      if (point.timestamp >= windowStart) total += point.value;
    }
    return { series, total };
  });
  seriesWithVolume.sort((left, right) => right.total - left.total);

  const topSeriesEntries = seriesWithVolume.slice(0, 20);
  const highlightedEntry = options.highlightSeriesId
    ? seriesWithVolume.find((entry) => entry.series.id === options.highlightSeriesId)
    : undefined;

  if (
    highlightedEntry &&
    !topSeriesEntries.some((entry) => entry.series.id === highlightedEntry.series.id)
  ) {
    if (topSeriesEntries.length >= 20) {
      topSeriesEntries[topSeriesEntries.length - 1] = highlightedEntry;
    } else {
      topSeriesEntries.push(highlightedEntry);
    }
  }

  const topSeries = topSeriesEntries.map((entry) => entry.series);
  let globalMax = 0;
  const grid: { sum: number; count: number; max: number }[][] = topSeries.map(() =>
    Array(DENSITY_MAP_COLUMNS)
      .fill(null)
      .map(() => ({ sum: 0, count: 0, max: 0 })),
  );

  for (let row = 0; row < topSeries.length; row += 1) {
    for (const point of topSeries[row].data) {
      if (point.timestamp < windowStart || point.timestamp > windowEnd) continue;
      const column = Math.floor(((point.timestamp - windowStart) / rangeMs) * DENSITY_MAP_COLUMNS);
      const clampedColumn = clampDensityMapValue(column, 0, DENSITY_MAP_COLUMNS - 1);

      grid[row][clampedColumn].sum += point.value;
      grid[row][clampedColumn].count += 1;
      if (point.value > grid[row][clampedColumn].max) {
        grid[row][clampedColumn].max = point.value;
      }
      if (point.value > globalMax) {
        globalMax = point.value;
      }
    }
  }

  const cellData = topSeries.map((_, row) =>
    grid[row].map((cell) => (cell.count > 0 ? cell.max : 0)),
  );

  return {
    series: topSeries,
    grid: cellData,
    globalMax,
    windowStart,
    rangeMs,
    bucketDuration,
  };
}

export function getDensityMapCellOpacity(value: number, globalMax: number): number {
  if (value <= 0 || globalMax <= 0) return 0;
  const normalized = Math.log(1 + (value / globalMax) * 99) / Math.log(100);
  return clampDensityMapValue(normalized, 0.15, 1.0);
}

export function getDensityMapExternalSeriesIndex(
  data: DensityMapChartData,
  highlightSeriesId?: string | null,
): number | null {
  if (!highlightSeriesId) return null;
  const index = data.series.findIndex((series) => series.id === highlightSeriesId);
  return index >= 0 ? index : null;
}

export function getDensityMapColumnIndexForTimestamp(
  data: DensityMapChartData,
  timestamp: number | null | undefined,
): number | null {
  if (timestamp === null || timestamp === undefined || data.rangeMs <= 0) {
    return null;
  }
  const clampedTimestamp = clampDensityMapValue(
    timestamp,
    data.windowStart,
    data.windowStart + data.rangeMs,
  );
  return clampDensityMapValue(
    Math.floor(((clampedTimestamp - data.windowStart) / data.rangeMs) * DENSITY_MAP_COLUMNS),
    0,
    DENSITY_MAP_COLUMNS - 1,
  );
}

export function buildDensityMapHoveredState(options: {
  clientX: number;
  clientY: number;
  rect: DOMRect;
  data: DensityMapChartData;
}): DensityMapHoveredState | null {
  const { rect, data } = options;
  if (data.series.length === 0) return null;

  const mouseX = clampDensityMapValue(options.clientX - rect.left, 0, rect.width - 1);
  const mouseY = clampDensityMapValue(options.clientY - rect.top, 0, rect.height - 1);
  const column = Math.floor((mouseX / rect.width) * DENSITY_MAP_COLUMNS);
  const row = Math.floor((mouseY / rect.height) * data.series.length);

  if (row < 0 || row >= data.series.length || column < 0 || column >= DENSITY_MAP_COLUMNS) {
    return null;
  }

  const cellWidth = rect.width / DENSITY_MAP_COLUMNS;
  const cellHeight = rect.height / data.series.length;

  return {
    columnIndex: column,
    tooltipX: rect.left + column * cellWidth + cellWidth / 2,
    tooltipY: rect.top + row * cellHeight,
    timestamp: data.windowStart + column * data.bucketDuration,
    value: data.grid[row][column],
    seriesName: data.series[row].name || 'Unknown',
    seriesColor: data.series[row].color,
    seriesIndex: row,
  };
}

export function buildDensityMapSynchronizedHoveredState(options: {
  data: DensityMapChartData;
  hoverSync?: SummaryChartHoverSync | null;
}): DensityMapHoveredState | null {
  const { data, hoverSync } = options;
  if (!hoverSync || data.series.length === 0 || data.rangeMs <= 0) {
    return null;
  }

  const seriesIndex = data.series.findIndex((series) => series.id === hoverSync.seriesId);
  if (seriesIndex < 0) {
    return null;
  }

  const clampedTimestamp = clampDensityMapValue(
    hoverSync.timestamp,
    data.windowStart,
    data.windowStart + data.rangeMs,
  );
  const column = clampDensityMapValue(
    Math.floor(((clampedTimestamp - data.windowStart) / data.rangeMs) * DENSITY_MAP_COLUMNS),
    0,
    DENSITY_MAP_COLUMNS - 1,
  );

  return {
    columnIndex: column,
    tooltipX: 0,
    tooltipY: 0,
    timestamp: clampedTimestamp,
    value: data.grid[seriesIndex][column],
    seriesName: data.series[seriesIndex].name || 'Unknown',
    seriesColor: data.series[seriesIndex].color,
    seriesIndex,
  };
}

export function buildDensityMapFocusDetail(options: {
  activeHoveredState?: DensityMapHoveredState | null;
  data: DensityMapChartData;
  highlightSeriesId?: string | null;
}): DensityMapFocusDetail | null {
  const activeHoveredState = options.activeHoveredState ?? null;
  const seriesIndex =
    activeHoveredState?.seriesIndex ??
    getDensityMapExternalSeriesIndex(options.data, options.highlightSeriesId);
  if (seriesIndex === null || seriesIndex < 0 || seriesIndex >= options.data.series.length) {
    return null;
  }

  const series = options.data.series[seriesIndex];
  const seriesId = series.id?.trim() || '';
  if (!seriesId) {
    return null;
  }

  const windowEnd = options.data.windowStart + options.data.rangeMs;
  const points = series.data.filter(
    (point) => point.timestamp >= options.data.windowStart && point.timestamp <= windowEnd,
  );
  let peakValue: number | null = null;
  for (const point of points) {
    peakValue = peakValue === null ? point.value : Math.max(peakValue, point.value);
  }

  return {
    peakValue,
    seriesColor: series.color,
    seriesId,
    seriesName: series.name || 'Unknown',
  };
}

export function hasDensityMapFocusActivity(detail: DensityMapFocusDetail): boolean {
  return detail.peakValue !== null;
}
