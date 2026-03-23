import type { TimeRange } from '@/api/charts';
import { timeRangeToMs } from '@/utils/timeRange';
import type { InteractiveSparklineSeries } from './InteractiveSparkline';

export interface DensityMapProps {
  series: InteractiveSparklineSeries[];
  rangeLabel?: string;
  timeRange?: TimeRange;
  formatValue?: (value: number) => string;
}

export interface DensityMapHoveredState {
  tooltipX: number;
  tooltipY: number;
  timestamp: number;
  value: number;
  seriesName: string;
  seriesColor: string;
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
}): DensityMapChartData {
  const range = options.timeRange || '1h';
  const rangeMs = timeRangeToMs(range);
  const windowEnd = options.now ?? Date.now();
  const windowStart = windowEnd - rangeMs;
  const bucketDuration = rangeMs / DENSITY_MAP_COLUMNS;

  const activeSeries = options.series.filter((series) => series.data.length > 0);
  const seriesWithVolume = activeSeries.map((series) => {
    let total = 0;
    for (const point of series.data) {
      if (point.timestamp >= windowStart) total += point.value;
    }
    return { series, total };
  });
  seriesWithVolume.sort((left, right) => right.total - left.total);

  const topSeries = seriesWithVolume.slice(0, 20).map((entry) => entry.series);
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
    tooltipX: rect.left + column * cellWidth + cellWidth / 2,
    tooltipY: rect.top + row * cellHeight,
    timestamp: data.windowStart + column * data.bucketDuration,
    value: data.grid[row][column],
    seriesName: data.series[row].name || 'Unknown',
    seriesColor: data.series[row].color,
  };
}
