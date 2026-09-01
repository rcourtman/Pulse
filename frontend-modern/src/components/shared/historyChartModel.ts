import type { AggregatedMetricPoint, HistoryTimeRange, ResourceType } from '@/api/charts';
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

export interface HistoryChartTooltipLayout {
  x: number;
  y: number;
  width: number;
  height: number;
}

export const HISTORY_CHART_RANGES: HistoryTimeRange[] = [
  '1h',
  '6h',
  '12h',
  '24h',
  '7d',
  '14d',
  '30d',
  '90d',
];

export const HISTORY_CHART_MIN_LEFT_INSET = 40;

const HISTORY_CHART_RANGE_LABELS: Record<HistoryTimeRange, string> = {
  '30m': '30-minute',
  '1h': '1-hour',
  '6h': '6-hour',
  '12h': '12-hour',
  '24h': '24-hour',
  '7d': '7-day',
  '14d': '14-day',
  '30d': '30-day',
  '90d': '90-day',
};

export interface HistoryChartAccessibleDescriptionInput {
  data: AggregatedMetricPoint[];
  error: string | null;
  isLocked: boolean;
  loading: boolean;
  range: HistoryTimeRange;
  unit?: string;
}

export function formatHistoryChartTooltipValue(value: number, unit?: string): string {
  if (unit === '%') return `${value.toFixed(1)}%`;
  if (unit === 'B/s') return `${formatBytes(value)}/s`;
  if (unit === 'C') return `${Math.round(value)}°C`;
  if (!unit) return formatBytes(value);
  return `${Number.isInteger(value) ? value : value.toFixed(1)} ${unit}`;
}

export function getHistoryChartAccessibleLabel(label?: string): string {
  return `${label?.trim() || 'History'} chart`;
}

export function getHistoryChartAccessibleDescription({
  data,
  error,
  isLocked,
  loading,
  range,
  unit,
}: HistoryChartAccessibleDescriptionInput): string {
  const rangeLabel = HISTORY_CHART_RANGE_LABELS[range] ?? `${range} history`;
  if (loading) return `Loading ${rangeLabel} history data.`;
  if (error) return `${rangeLabel} history data could not be loaded.`;
  if (isLocked) return `${rangeLabel} history data is unavailable on the current plan.`;
  if (data.length === 0) return `No ${rangeLabel} history data is available.`;

  const first = data[0];
  const latest = data[data.length - 1];
  const minimum = getHistoryChartDataMin(data)!;
  const maximum = getHistoryChartDataMax(data)!;
  const formatTimestamp = (timestamp: number) => new Date(timestamp).toLocaleString();
  const formatValue = (value: number) => formatHistoryChartTooltipValue(value, unit);

  if (data.length === 1) {
    return `${rangeLabel} history contains 1 data point at ${formatTimestamp(latest.timestamp)}: ${formatValue(latest.value)}.`;
  }

  const direction =
    latest.value > first.value
      ? 'increased'
      : latest.value < first.value
        ? 'decreased'
        : 'remained unchanged';
  const changeSummary =
    direction === 'remained unchanged'
      ? `Values remained unchanged at ${formatValue(latest.value)}.`
      : `Values ${direction} from ${formatValue(first.value)} to ${formatValue(latest.value)}.`;

  return `${rangeLabel} history contains ${data.length} data points from ${formatTimestamp(first.timestamp)} to ${formatTimestamp(latest.timestamp)}. ${changeSummary} Minimum ${formatValue(minimum)}. Maximum ${formatValue(maximum)}.`;
}

export function getHistoryChartRefreshIntervalMs(range: HistoryTimeRange) {
  switch (range) {
    case '7d':
    case '14d':
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

export function getHistoryChartYAxisLabels(
  {
    minValue,
    maxValue,
    isPercentLike,
    isByteLike,
  }: {
    minValue: number;
    maxValue: number;
    isPercentLike: boolean;
    isByteLike: boolean;
  },
  unit?: string,
) {
  return [0, 0.5, 1].map((pct) => {
    const scaleValue = minValue + pct * (maxValue - minValue);
    let label: string;
    if (isPercentLike) {
      label = `${Math.round(scaleValue)}%`;
    } else if (isByteLike) {
      label = formatHistoryChartTooltipValue(scaleValue, unit);
    } else {
      label = `${Math.round(scaleValue)}`;
    }
    return { pct, label };
  });
}

export function getHistoryChartLeftInset(labelWidths: number[]) {
  const widestLabel = Math.max(0, ...labelWidths);
  return Math.max(HISTORY_CHART_MIN_LEFT_INSET, Math.ceil(widestLabel) + 8);
}

export function getHistoryChartRightInset(lastTimeLabelWidth: number) {
  return Math.max(0, Math.ceil(lastTimeLabelWidth / 2) + 2);
}

export function formatHistoryChartTimeLabel(timestamp: number, range: HistoryTimeRange) {
  const date = new Date(timestamp);
  if (range === '30d' || range === '90d' || range === '14d' || range === '7d') {
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
  leftInset = HISTORY_CHART_MIN_LEFT_INSET,
  rightInset = 0,
}: {
  width: number;
  height: number;
  startTime: number;
  endTime: number;
  minValue: number;
  maxValue: number;
  leftInset?: number;
  rightInset?: number;
}) {
  const timeSpan = Math.max(1, endTime - startTime);
  const getX = (timestamp: number) =>
    leftInset + ((timestamp - startTime) / timeSpan) * (width - leftInset - rightInset);
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

export function getHistoryChartTooltipLayout({
  hoveredPoint,
  chartWidth,
  chartHeight,
}: {
  hoveredPoint: HistoryChartHoverPoint;
  chartWidth: number;
  chartHeight: number;
}): HistoryChartTooltipLayout {
  const width = 156;
  const height = 46;
  const margin = 8;
  const pointGap = 12;
  const minX = margin;
  const maxX = Math.max(minX, chartWidth - width - margin);
  const minY = margin;
  const maxY = Math.max(minY, chartHeight - height - margin);
  const clampX = (value: number) => Math.min(Math.max(value, minX), maxX);
  const clampY = (value: number) => Math.min(Math.max(value, minY), maxY);

  const rightX = hoveredPoint.x + pointGap;
  const leftX = hoveredPoint.x - width - pointGap;
  const canPlaceRight = rightX <= maxX;
  const canPlaceLeft = leftX >= minX;
  const rightRoom = chartWidth - margin - hoveredPoint.x;
  const leftRoom = hoveredPoint.x - margin;
  const x =
    canPlaceRight && (!canPlaceLeft || rightRoom >= leftRoom)
      ? rightX
      : canPlaceLeft
        ? leftX
        : clampX(hoveredPoint.x - width / 2);

  let y = clampY(hoveredPoint.y - height / 2);
  const overlapsHoveredPoint =
    hoveredPoint.x >= x &&
    hoveredPoint.x <= x + width &&
    hoveredPoint.y >= y &&
    hoveredPoint.y <= y + height;
  if (overlapsHoveredPoint) {
    const showBelow = hoveredPoint.y < height + margin;
    y = showBelow ? clampY(hoveredPoint.y + pointGap) : clampY(hoveredPoint.y - height - pointGap);
  }
  return { x, y, width, height };
}
