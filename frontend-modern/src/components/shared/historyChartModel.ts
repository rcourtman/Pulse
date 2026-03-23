import type {
  AggregatedMetricPoint,
  HistoryTimeRange,
  ResourceType,
} from '@/api/charts';
import { formatBytes } from '@/utils/format';

export interface HistoryChartProps {
  resourceType: ResourceType;
  resourceId: string;
  metric: string;
  height?: number;
  color?: string;
  label?: string;
  unit?: string;
  range?: HistoryTimeRange;
  onRangeChange?: (range: HistoryTimeRange) => void;
  hideSelector?: boolean;
  compact?: boolean;
  hideLock?: boolean;
  data?: AggregatedMetricPoint[];
}

export interface HistoryChartHoverPoint {
  value: number;
  timestamp: number;
  x: number;
  y: number;
}

export const HISTORY_CHART_RANGES: HistoryTimeRange[] = ['24h', '7d', '30d', '90d'];

export function formatHistoryChartTooltipValue(value: number, unit?: string): string {
  if (unit === '%') return `${value.toFixed(1)}%`;
  if (unit === 'B/s') return `${formatBytes(value)}/s`;
  if (unit === 'C') return `${Math.round(value)}°C`;
  if (!unit) return formatBytes(value);
  return `${Number.isInteger(value) ? value : value.toFixed(1)} ${unit}`;
}

export function getHistoryChartRefreshIntervalMs(range: HistoryTimeRange) {
  switch (range) {
    case '7d':
      return 30000;
    case '30d':
      return 60000;
    case '90d':
      return 120000;
    default:
      return 10000;
  }
}

export function getHistoryChartDefaultColor(metric: string, color?: string) {
  if (color) return color;
  if (metric === 'cpu') return '#8b5cf6';
  if (metric === 'memory') return '#f59e0b';
  if (metric === 'disk') return '#10b981';
  return '#3b82f6';
}

export function getHistoryChartDataMin(points: AggregatedMetricPoint[]) {
  if (points.length === 0) return null;
  let min = Infinity;
  for (const point of points) {
    const value = point.min != null ? point.min : point.value;
    if (value < min) min = value;
  }
  return min;
}

export function getHistoryChartDataMax(points: AggregatedMetricPoint[]) {
  if (points.length === 0) return null;
  let max = -Infinity;
  for (const point of points) {
    const value = point.max != null ? point.max : point.value;
    if (value > max) max = value;
  }
  return max;
}

export function getHistoryChartScale(points: AggregatedMetricPoint[], unit?: string) {
  const minValue = 0;
  const isPercentLike = unit === '%';
  const isByteLike = !unit || unit === 'B/s';
  let maxValue = 100;
  if (points.length > 0) {
    const rawMax = Math.max(...points.map((point) => point.max || point.value));
    maxValue = isPercentLike ? Math.max(100, rawMax) : Math.max(1, rawMax * 1.15);
  }

  return {
    isPercentLike,
    isByteLike,
    minValue,
    maxValue,
  };
}

export function getHistoryChartYAxisLabels({
  minValue,
  maxValue,
  isPercentLike,
  isByteLike,
}: {
  minValue: number;
  maxValue: number;
  isPercentLike: boolean;
  isByteLike: boolean;
}) {
  return [0, 0.5, 1].map((pct) => {
    let label = '';
    if (isPercentLike) {
      label = pct === 0 ? '0%' : pct === 1 ? '100%' : '50%';
    } else if (isByteLike) {
      label = pct === 0 ? '0' : pct === 1 ? 'Max' : 'Avg';
    } else {
      const scaleValue = Math.round(minValue + pct * (maxValue - minValue));
      label = pct === 0 ? '0' : `${scaleValue}`;
    }
    return { pct, label };
  });
}

export function formatHistoryChartTimeLabel(timestamp: number, range: HistoryTimeRange) {
  const date = new Date(timestamp);
  if (range === '30d' || range === '90d' || range === '7d') {
    return date.toLocaleDateString([], { month: 'short', day: 'numeric' });
  }
  return date.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' });
}

export function createHistoryChartGeometry({
  width,
  height,
  startTime,
  endTime,
  minValue,
  maxValue,
}: {
  width: number;
  height: number;
  startTime: number;
  endTime: number;
  minValue: number;
  maxValue: number;
}) {
  const timeSpan = Math.max(1, endTime - startTime);
  const getX = (timestamp: number) => 40 + ((timestamp - startTime) / timeSpan) * (width - 40);
  const getY = (value: number) =>
    height - 20 - ((value - minValue) / (maxValue - minValue)) * (height - 40);

  return {
    timeSpan,
    getX,
    getY,
  };
}

export function findHistoryChartClosestPoint(
  points: AggregatedMetricPoint[],
  hoverTimestamp: number,
) {
  let closest = points[0];
  let minDiff = Math.abs(points[0].timestamp - hoverTimestamp);
  for (const point of points) {
    const diff = Math.abs(point.timestamp - hoverTimestamp);
    if (diff < minDiff) {
      minDiff = diff;
      closest = point;
    }
  }
  return closest;
}
