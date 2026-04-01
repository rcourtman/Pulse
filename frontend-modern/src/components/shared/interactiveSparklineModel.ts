import type { MetricPoint, TimeRange } from '@/api/charts';
import { downsampleLTTB, calculateOptimalPoints } from '@/utils/downsample';
import { timeRangeToMs } from '@/utils/timeRange';
import type { SummaryChartHoverSync } from './contextualFocus';
import type { SummaryCardInteractionState } from './summaryCardInteraction';

export interface InteractiveSparklineSeries {
  id?: string;
  data: MetricPoint[];
  color: string;
  name?: string;
}

export interface InteractiveSparklineProps {
  series: InteractiveSparklineSeries[];
  rangeLabel?: string;
  timeRange?: TimeRange;
  renderMode?: 'auto' | 'svg' | 'canvas';
  activeSeriesDisplay?: 'emphasize' | 'isolate';
  yMode?: 'percent' | 'auto';
  size?: 'sm' | 'md' | 'lg';
  bridgeLeadingGap?: boolean;
  formatValue?: (value: number) => string;
  formatTopLabel?: (maxValue: number) => string;
  sortTooltipByValue?: boolean;
  maxTooltipRows?: number;
  highlightNearestSeriesOnHover?: boolean;
  highlightSeriesId?: string | null;
  hoverSourceKey?: string;
  hoverSync?: SummaryChartHoverSync | null;
  onHoverSyncChange?: (value: SummaryChartHoverSync | null) => void;
  interactionState?: SummaryCardInteractionState;
}

export interface InteractiveSparklineChartSeries extends InteractiveSparklineSeries {
  hoverData: MetricPoint[];
  drawData: MetricPoint[];
  segments: MetricPoint[][];
}

export interface InteractiveSparklinePathData {
  path: string;
  areaPath?: string;
  color: string;
  seriesIndex: number;
}

export interface InteractiveSparklineChartData {
  validSeries: InteractiveSparklineChartSeries[];
  paths: InteractiveSparklinePathData[];
  windowStart: number;
  rangeMs: number;
  scaleMax: number;
}

export interface InteractiveSparklineDisplayValue {
  name: string;
  color: string;
  value: number;
  seriesIndex: number;
}

export interface InteractiveSparklineHoverState {
  x: number;
  tooltipX: number;
  tooltipY: number;
  timestamp: number;
  totalValues: number;
  nearestSeriesIndex: number | null;
  highlightedSeriesIndex: number | null;
  focusedTooltip: boolean;
  values: InteractiveSparklineDisplayValue[];
}

interface HoverSeriesValue extends InteractiveSparklineDisplayValue {
  timestamp: number;
}

export const formatInteractiveSparklineHoverTime = (timestamp: number): string =>
  new Date(timestamp).toLocaleString([], {
    year: 'numeric',
    month: 'short',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
    second: '2-digit',
    hour12: false,
  });

export const clampInteractiveSparklineValue = (value: number, min: number, max: number): number =>
  Math.max(min, Math.min(value, max));

export const formatInteractiveSparklineRelativeOffset = (offsetMs: number): string => {
  if (offsetMs <= 0) return 'now';
  const dayMs = 24 * 60 * 60_000;
  const hourMs = 60 * 60_000;
  const minuteMs = 60_000;

  if (offsetMs >= dayMs) {
    return `-${Math.round(offsetMs / dayMs)}d`;
  }
  if (offsetMs >= hourMs) {
    return `-${Math.round(offsetMs / hourMs)}h`;
  }
  return `-${Math.max(1, Math.round(offsetMs / minuteMs))}m`;
};

const findNearestMetricPoint = (
  points: MetricPoint[],
  targetTimestamp: number,
): { point: MetricPoint; index: number } | null => {
  if (points.length === 0) return null;

  let low = 0;
  let high = points.length - 1;
  while (low < high) {
    const mid = Math.floor((low + high) / 2);
    if (points[mid].timestamp < targetTimestamp) {
      low = mid + 1;
    } else {
      high = mid;
    }
  }

  const candidate = points[low];
  const previous = low > 0 ? points[low - 1] : candidate;
  if (
    Math.abs(previous.timestamp - targetTimestamp) <=
    Math.abs(candidate.timestamp - targetTimestamp)
  ) {
    return { point: previous, index: low > 0 ? low - 1 : low };
  }
  return { point: candidate, index: low };
};

const ensureRenderablePoints = (points: MetricPoint[], windowStart: number): MetricPoint[] => {
  if (points.length === 0) return [];
  if (points.length >= 2) return points;
  const point = points[0];
  return [
    {
      timestamp: Math.max(windowStart, point.timestamp - 1_000),
      value: point.value,
    },
    point,
  ];
};

const splitPointsOnGaps = (
  points: MetricPoint[],
  gapThresholdMs: number,
  windowStart: number,
  bridgeLeadingGap: boolean,
): MetricPoint[][] => {
  if (points.length === 0) return [];
  const segments: MetricPoint[][] = [];
  let currentSegment: MetricPoint[] = [points[0]];

  for (let index = 1; index < points.length; index++) {
    const previous = points[index - 1];
    const current = points[index];
    const hasGap = current.timestamp - previous.timestamp > gapThresholdMs;
    const isLeadingBridgeGap =
      bridgeLeadingGap &&
      segments.length === 0 &&
      currentSegment.length === 1 &&
      currentSegment[0].timestamp <= windowStart + 1_000;

    if (hasGap && !isLeadingBridgeGap) {
      segments.push(ensureRenderablePoints(currentSegment, windowStart));
      currentSegment = [current];
      continue;
    }
    currentSegment.push(current);
  }

  segments.push(ensureRenderablePoints(currentSegment, windowStart));
  return segments;
};

const inferGapThresholdMs = (points: MetricPoint[], rangeMs: number): number => {
  if (points.length < 3) return rangeMs + 1;

  const deltas: number[] = [];
  for (let index = 1; index < points.length; index++) {
    const delta = points[index].timestamp - points[index - 1].timestamp;
    if (delta > 0) {
      deltas.push(delta);
    }
  }
  if (deltas.length < 2) return rangeMs + 1;

  deltas.sort((left, right) => left - right);
  const p90Delta = deltas[Math.floor(deltas.length * 0.9)];
  const minThreshold = Math.max(15_000, Math.floor(rangeMs / 120));
  const maxThreshold = Math.max(minThreshold, Math.floor(rangeMs / 2));
  return clampInteractiveSparklineValue(p90Delta * 3, minThreshold, maxThreshold);
};

const selectTopValuesByValue = (values: HoverSeriesValue[], limit: number): HoverSeriesValue[] => {
  if (limit <= 0 || values.length === 0) return [];
  if (values.length <= limit) {
    return [...values].sort((left, right) => right.value - left.value);
  }

  const top: HoverSeriesValue[] = [];
  for (const value of values) {
    let insertAt = top.length;
    for (let index = 0; index < top.length; index++) {
      if (value.value > top[index].value) {
        insertAt = index;
        break;
      }
    }
    if (insertAt >= limit && top.length >= limit) continue;
    top.splice(insertAt, 0, value);
    if (top.length > limit) {
      top.pop();
    }
  }
  return top;
};

export const getInteractiveSparklineShouldUseCanvas = (
  series: InteractiveSparklineSeries[],
  renderMode: InteractiveSparklineProps['renderMode'],
  canvasSeriesThreshold = 120,
  canvasPointThreshold = 15_000,
) => {
  if (renderMode === 'svg') return false;
  if (renderMode === 'canvas') return true;
  const totalPoints = series.reduce((sum, item) => sum + item.data.length, 0);
  return series.length >= canvasSeriesThreshold || totalPoints >= canvasPointThreshold;
};

export const buildInteractiveSparklineChartData = ({
  series,
  timeRange,
  yMode,
  vbW,
  vbH,
  shouldUseCanvas,
  bridgeLeadingGap,
}: {
  series: InteractiveSparklineSeries[];
  timeRange: TimeRange;
  yMode: 'percent' | 'auto';
  vbW: number;
  vbH: number;
  shouldUseCanvas: boolean;
  bridgeLeadingGap: boolean;
}): InteractiveSparklineChartData => {
  const rangeMs = timeRangeToMs(timeRange);
  const windowEnd = Date.now();
  const windowStart = windowEnd - rangeMs;
  const targetPoints = calculateOptimalPoints(vbW, 'sparkline');
  const validSeries = series
    .map((item) => {
      const inWindow = item.data.filter(
        (point) => point.timestamp >= windowStart && point.timestamp <= windowEnd,
      );
      if (inWindow.length < 2) return null;
      const renderable = ensureRenderablePoints(inWindow, windowStart);
      if (renderable.length === 0) return null;
      const drawData =
        renderable.length > targetPoints * 1.5
          ? downsampleLTTB(renderable, targetPoints)
          : renderable;
      const segments = splitPointsOnGaps(
        drawData,
        inferGapThresholdMs(drawData, rangeMs),
        windowStart,
        bridgeLeadingGap,
      );
      return {
        ...item,
        hoverData: renderable,
        drawData,
        segments,
      };
    })
    .filter((item): item is InteractiveSparklineChartSeries => item !== null);

  if (validSeries.length === 0) {
    return {
      validSeries: [],
      paths: [],
      windowStart,
      rangeMs,
      scaleMax: 0,
    };
  }

  let scaleMax = 100;
  if (yMode === 'auto') {
    scaleMax = 0;
    for (const item of validSeries) {
      for (const point of item.hoverData) {
        if (point.value > scaleMax) scaleMax = point.value;
      }
    }
    if (scaleMax <= 0) {
      return {
        validSeries,
        paths: [],
        windowStart,
        rangeMs,
        scaleMax: 0,
      };
    }
  }

  const paths = shouldUseCanvas
    ? []
    : validSeries.flatMap((item, seriesIndex) =>
        item.segments.map((segment) => {
          const coords = segment.map((point) => {
            const x = clampInteractiveSparklineValue(
              ((point.timestamp - windowStart) / rangeMs) * vbW,
              0,
              vbW,
            );
            const normalized =
              yMode === 'auto'
                ? Math.max(0, point.value) / scaleMax
                : Math.min(Math.max(point.value, 0), 100) / 100;
            const y = vbH - normalized * vbH;
            return { x, y };
          });
          const pathStrings = coords.map((coord) => `${coord.x.toFixed(1)},${coord.y.toFixed(1)}`);
          const path = `M${pathStrings.join('L')}`;
          let areaPath = '';
          if (validSeries.length === 1 && coords.length > 1) {
            areaPath = `${path} L${coords[coords.length - 1].x.toFixed(1)},${vbH} L${coords[0].x.toFixed(1)},${vbH} Z`;
          }
          return { path, areaPath, color: item.color, seriesIndex };
        }),
      );

  return {
    validSeries,
    paths,
    windowStart,
    rangeMs,
    scaleMax,
  };
};

export const createInteractiveSparklineValueToY = (
  yMode: 'percent' | 'auto',
  scaleMax: number,
  vbH: number,
) => {
  return (value: number): number => {
    if (yMode === 'auto') {
      if (scaleMax <= 0) return vbH;
      return vbH - (Math.max(0, value) / scaleMax) * vbH;
    }
    return vbH - (Math.min(Math.max(value, 0), 100) / 100) * vbH;
  };
};

export const buildInteractiveSparklineTopLabel = ({
  yMode,
  scaleMax,
  formatTopLabel,
}: {
  yMode: 'percent' | 'auto';
  scaleMax: number;
  formatTopLabel?: (maxValue: number) => string;
}) => {
  if (yMode === 'percent') return '100%';
  if (scaleMax <= 0) return '0';
  return formatTopLabel ? formatTopLabel(scaleMax) : scaleMax.toFixed(1);
};

export const buildInteractiveSparklineAxisTicks = (yMode: 'percent' | 'auto', topLabel: string) => {
  if (yMode === 'percent') {
    return [
      { label: '100%', top: '0%', anchor: 'top' as const },
      { label: '80%', top: '20%', anchor: 'middle' as const },
      { label: '60%', top: '40%', anchor: 'middle' as const },
      { label: '40%', top: '60%', anchor: 'middle' as const },
      { label: '20%', top: '80%', anchor: 'middle' as const },
      { label: '0%', top: '100%', anchor: 'bottom' as const },
    ];
  }

  return [
    { label: topLabel, top: '0%', anchor: 'top' as const },
    { label: '0', top: '100%', anchor: 'bottom' as const },
  ];
};

export const buildInteractiveSparklineGridLineY = (yMode: 'percent' | 'auto', vbH: number) => {
  if (yMode === 'percent') {
    return [vbH * 0.2, vbH * 0.4, vbH * 0.6, vbH * 0.8];
  }
  return [vbH * 0.25, vbH * 0.5, vbH * 0.75];
};

export const buildInteractiveSparklineGridLineX = (vbW: number) => [vbW * 0.5];

export const buildInteractiveSparklineXAxisTicks = ({
  rangeMs,
  rangeLabel,
  timeRange,
}: {
  rangeMs: number;
  rangeLabel?: string;
  timeRange: TimeRange;
}) => {
  const rangeToken = rangeLabel || timeRange;
  return [
    { left: 0, label: `-${rangeToken}`, anchor: 'start' as const },
    {
      left: 50,
      label: formatInteractiveSparklineRelativeOffset(rangeMs / 2),
      anchor: 'middle' as const,
    },
    { left: 100, label: 'now', anchor: 'end' as const },
  ];
};

export const getInteractiveSparklineExternalSeriesIndex = (
  chartData: InteractiveSparklineChartData,
  highlightSeriesId?: string | null,
) => {
  if (!highlightSeriesId) return null;
  const index = chartData.validSeries.findIndex((series) => series.id === highlightSeriesId);
  return index >= 0 ? index : null;
};

export const getInteractiveSparklineCursorXForTimestamp = ({
  chartData,
  timestamp,
  vbW,
}: {
  chartData: InteractiveSparklineChartData;
  timestamp: number | null | undefined;
  vbW: number;
}): number | null => {
  if (timestamp === null || timestamp === undefined || chartData.rangeMs <= 0) {
    return null;
  }
  const clampedTimestamp = clampInteractiveSparklineValue(
    timestamp,
    chartData.windowStart,
    chartData.windowStart + chartData.rangeMs,
  );
  return ((clampedTimestamp - chartData.windowStart) / chartData.rangeMs) * vbW;
};

export const getInteractiveSparklineActiveEmphasisSeriesIndex = ({
  highlightNearestSeriesOnHover,
  lockedSeriesIndex,
  hoveredState,
  externalSeriesIndex,
}: {
  highlightNearestSeriesOnHover: boolean;
  lockedSeriesIndex: number | null;
  hoveredState: InteractiveSparklineHoverState | null;
  externalSeriesIndex: number | null;
}) => {
  if (highlightNearestSeriesOnHover && lockedSeriesIndex !== null) {
    return lockedSeriesIndex;
  }
  if (
    highlightNearestSeriesOnHover &&
    hoveredState?.highlightedSeriesIndex !== null &&
    hoveredState
  ) {
    return hoveredState.highlightedSeriesIndex;
  }
  return externalSeriesIndex;
};

export const buildInteractiveSparklineSynchronizedHoverState = ({
  chartData,
  hoverSync,
  vbW,
}: {
  chartData: InteractiveSparklineChartData;
  hoverSync?: SummaryChartHoverSync | null;
  vbW: number;
}): InteractiveSparklineHoverState | null => {
  if (!hoverSync || chartData.validSeries.length === 0 || chartData.rangeMs <= 0) {
    return null;
  }

  const seriesIndex = chartData.validSeries.findIndex((series) => series.id === hoverSync.seriesId);
  if (seriesIndex < 0) {
    return null;
  }

  const clampedTimestamp = clampInteractiveSparklineValue(
    hoverSync.timestamp,
    chartData.windowStart,
    chartData.windowStart + chartData.rangeMs,
  );
  const series = chartData.validSeries[seriesIndex];
  const nearest = findNearestMetricPoint(series.hoverData, clampedTimestamp);
  if (!nearest) {
    return null;
  }

  const chartX = ((clampedTimestamp - chartData.windowStart) / chartData.rangeMs) * vbW;

  return {
    x: chartX,
    tooltipX: 0,
    tooltipY: 0,
    timestamp: clampedTimestamp,
    totalValues: 1,
    nearestSeriesIndex: seriesIndex,
    highlightedSeriesIndex: seriesIndex,
    focusedTooltip: true,
    values: [
      {
        name: series.name || 'Series',
        color: series.color,
        value: nearest.point.value,
        seriesIndex,
      },
    ],
  };
};

export const computeInteractiveSparklineHoverState = ({
  chartData,
  chartRect,
  clientX,
  clientY,
  vbW,
  vbH,
  yMode,
  maxRows,
  sortTooltipByValue,
  highlightNearestSeriesOnHover,
  lockedSeriesIndex,
  tooltipPadding,
  tooltipEstimatedWidth,
}: {
  chartData: InteractiveSparklineChartData;
  chartRect: DOMRect;
  clientX: number;
  clientY: number;
  vbW: number;
  vbH: number;
  yMode: 'percent' | 'auto';
  maxRows: number;
  sortTooltipByValue?: boolean;
  highlightNearestSeriesOnHover?: boolean;
  lockedSeriesIndex: number | null;
  tooltipPadding: number;
  tooltipEstimatedWidth: number;
}): InteractiveSparklineHoverState | null => {
  if (chartData.validSeries.length === 0 || chartData.rangeMs <= 0 || chartRect.width <= 0) {
    return null;
  }

  const mouseX = Math.max(0, Math.min(clientX - chartRect.left, chartRect.width));
  const chartX = (mouseX / chartRect.width) * vbW;
  const targetTimestamp = chartData.windowStart + (chartX / vbW) * chartData.rangeMs;
  const shouldTrackNearest = highlightNearestSeriesOnHover === true;
  const chartY = shouldTrackNearest
    ? (Math.max(0, Math.min(clientY - chartRect.top, chartRect.height)) / chartRect.height) * vbH
    : 0;
  const valueToChartY = createInteractiveSparklineValueToY(yMode, chartData.scaleMax, vbH);

  let nearestSeriesIndex: number | null = null;
  let nearestDistance = Number.POSITIVE_INFINITY;
  const values: HoverSeriesValue[] = [];

  for (let seriesIndex = 0; seriesIndex < chartData.validSeries.length; seriesIndex++) {
    const series = chartData.validSeries[seriesIndex];
    const nearest = findNearestMetricPoint(series.hoverData, targetTimestamp);
    if (!nearest) continue;
    const point = nearest.point;
    const pointY = valueToChartY(point.value);

    if (shouldTrackNearest) {
      const distance = Math.abs(pointY - chartY);
      if (distance < nearestDistance) {
        nearestDistance = distance;
        nearestSeriesIndex = seriesIndex;
      }
    }

    values.push({
      name: series.name || 'Series',
      color: series.color,
      value: point.value,
      timestamp: point.timestamp,
      seriesIndex,
    });
  }

  if (values.length === 0) {
    return null;
  }

  const nearLineThresholdPx = 6;
  const nearLineThresholdChartUnits = (nearLineThresholdPx / Math.max(1, chartRect.height)) * vbH;
  const effectiveSeriesIndex =
    lockedSeriesIndex !== null
      ? lockedSeriesIndex
      : shouldTrackNearest &&
          nearestSeriesIndex !== null &&
          nearestDistance <= nearLineThresholdChartUnits
        ? nearestSeriesIndex
        : null;
  const focusedTooltip = effectiveSeriesIndex !== null;

  let groupedValues = values;
  if (!focusedTooltip) {
    const byValue = new Map<number, HoverSeriesValue[]>();
    for (const value of values) {
      const key = Math.round(value.value * 1000) / 1000;
      if (!byValue.has(key)) byValue.set(key, []);
      byValue.get(key)!.push(value);
    }
    groupedValues = [];
    for (const group of byValue.values()) {
      if (group.length > 1) {
        groupedValues.push({
          name: `${group.length} Series`,
          color: 'currentColor',
          value: group[0].value,
          timestamp: group[0].timestamp,
          seriesIndex: -1,
        });
      } else {
        groupedValues.push(group[0]);
      }
    }
  }

  const tooltipValues =
    focusedTooltip && effectiveSeriesIndex !== null
      ? values.filter((value) => value.seriesIndex === effectiveSeriesIndex)
      : sortTooltipByValue
        ? selectTopValuesByValue(groupedValues, maxRows)
        : groupedValues.slice(0, maxRows);
  if (tooltipValues.length === 0) {
    return null;
  }

  const totalValues = focusedTooltip ? tooltipValues.length : values.length;
  let tooltipX = clientX;
  let tooltipY = clientY - 10;

  if (typeof window !== 'undefined') {
    const activeTooltipWidth = focusedTooltip ? 150 : tooltipEstimatedWidth;
    const minTooltipX = tooltipPadding + activeTooltipWidth / 2;
    const maxTooltipX = Math.max(
      minTooltipX,
      window.innerWidth - tooltipPadding - activeTooltipWidth / 2,
    );
    tooltipX = clampInteractiveSparklineValue(tooltipX, minTooltipX, maxTooltipX);

    const shownRows = tooltipValues.length;
    const tooltipEstimatedHeight = 22 + shownRows * 16 + (totalValues > shownRows ? 14 : 0);
    const minTooltipY = tooltipEstimatedHeight + tooltipPadding;
    tooltipY = Math.max(tooltipY, minTooltipY);
  }

  return {
    x: chartX,
    tooltipX,
    tooltipY,
    timestamp: tooltipValues[0].timestamp,
    totalValues,
    nearestSeriesIndex,
    highlightedSeriesIndex:
      highlightNearestSeriesOnHover && focusedTooltip ? effectiveSeriesIndex : null,
    focusedTooltip,
    values: tooltipValues.map((value) => ({
      name: value.name,
      color: value.color,
      value: value.value,
      seriesIndex: value.seriesIndex,
    })),
  };
};
