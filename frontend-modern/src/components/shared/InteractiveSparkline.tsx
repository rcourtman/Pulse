import { Component, Show, For, createEffect, createMemo, createSignal, onCleanup } from 'solid-js';
import { Portal } from 'solid-js/web';
import type { MetricPoint, TimeRange } from '@/api/charts';
import { downsampleLTTB, calculateOptimalPoints } from '@/utils/downsample';
import { timeRangeToMs } from '@/utils/timeRange';
import { scheduleSparkline, setupCanvasDPR } from '@/utils/canvasRenderQueue';

export interface InteractiveSparklineSeries {
  id?: string;
  data: MetricPoint[];
  color: string;
  name?: string;
}

interface InteractiveSparklineProps {
  series: InteractiveSparklineSeries[];
  rangeLabel?: string;
  timeRange?: TimeRange;
  renderMode?: 'auto' | 'svg' | 'canvas';
  yMode?: 'percent' | 'auto';
  /** When true, keep a synthetic window-start anchor connected to first real point. */
  bridgeLeadingGap?: boolean;
  formatValue?: (value: number) => string;
  formatTopLabel?: (maxValue: number) => string;
  sortTooltipByValue?: boolean;
  maxTooltipRows?: number;
  highlightNearestSeriesOnHover?: boolean;
  highlightSeriesId?: string | null;
}

const formatHoverTime = (timestamp: number): string =>
  new Date(timestamp).toLocaleString([], {
    year: 'numeric',
    month: 'short',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
    second: '2-digit',
    hour12: false,
  });

const clamp = (value: number, min: number, max: number): number =>
  Math.max(min, Math.min(value, max));

const formatRelativeOffset = (offsetMs: number): string => {
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

const findNearestMetricPoint = (points: MetricPoint[], targetTimestamp: number): MetricPoint | null => {
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
  return Math.abs(previous.timestamp - targetTimestamp) <= Math.abs(candidate.timestamp - targetTimestamp)
    ? previous
    : candidate;
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

  for (let i = 1; i < points.length; i++) {
    const previous = points[i - 1];
    const current = points[i];
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
  for (let i = 1; i < points.length; i++) {
    const delta = points[i].timestamp - points[i - 1].timestamp;
    if (delta > 0) {
      deltas.push(delta);
    }
  }
  if (deltas.length < 2) return rangeMs + 1;

  deltas.sort((a, b) => a - b);
  // P90 captures upper normal variation; 3x multiplier only breaks on genuine outages.
  const p90Delta = deltas[Math.floor(deltas.length * 0.9)];
  const minThreshold = Math.max(15_000, Math.floor(rangeMs / 120));
  const maxThreshold = Math.max(minThreshold, Math.floor(rangeMs / 2));
  return clamp(p90Delta * 3, minThreshold, maxThreshold);
};

interface HoverSeriesValue {
  name: string;
  color: string;
  value: number;
  timestamp: number;
  seriesIndex: number;
}

const selectTopValuesByValue = (
  values: HoverSeriesValue[],
  limit: number,
): HoverSeriesValue[] => {
  if (limit <= 0 || values.length === 0) return [];
  if (values.length <= limit) {
    return [...values].sort((a, b) => b.value - a.value);
  }

  const top: HoverSeriesValue[] = [];
  for (const value of values) {
    let insertAt = top.length;
    for (let i = 0; i < top.length; i++) {
      if (value.value > top[i].value) {
        insertAt = i;
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

export const InteractiveSparkline: Component<InteractiveSparklineProps> = (props) => {
  let chartSurfaceRef: Element | undefined;
  let canvasRef: HTMLCanvasElement | undefined;
  let canvasHostRef: HTMLDivElement | undefined;
  const vbH = 100;
  const vbW = 200;
  const canvasSeriesThreshold = 120;
  const canvasPointThreshold = 15_000;
  const tooltipPadding = 8;
  const tooltipEstimatedWidth = 190;
  const maxRows = () => props.maxTooltipRows ?? 6;
  const yMode = () => props.yMode ?? 'percent';
  const shouldUseCanvas = createMemo(() => {
    const renderMode = props.renderMode ?? 'auto';
    if (renderMode === 'svg') return false;
    if (renderMode === 'canvas') return true;
    const totalPoints = props.series.reduce((sum, series) => sum + series.data.length, 0);
    return props.series.length >= canvasSeriesThreshold || totalPoints >= canvasPointThreshold;
  });
  const formatValue = (value: number) => props.formatValue ? props.formatValue(value) : `${value.toFixed(1)}%`;

  const [hoveredState, setHoveredState] = createSignal<{
    x: number;
    tooltipX: number;
    tooltipY: number;
    timestamp: number;
    totalValues: number;
    nearestSeriesIndex: number | null;
    highlightedSeriesIndex: number | null;
    focusedTooltip: boolean;
    values: { name: string; color: string; value: number; seriesIndex: number }[];
  } | null>(null);
  const [lockedSeriesIndex, setLockedSeriesIndex] = createSignal<number | null>(null);

  const chartData = createMemo(() => {
    const range = props.timeRange || '1h';
    const rangeMs = timeRangeToMs(range);
    const windowEnd = Date.now();
    const windowStart = windowEnd - rangeMs;
    const targetPoints = calculateOptimalPoints(vbW, 'sparkline');
    const bridgeLeadingGap = props.bridgeLeadingGap === true;
    const validSeries = props.series
      .map((series) => {
        const inWindow = series.data.filter((point) =>
          point.timestamp >= windowStart && point.timestamp <= windowEnd
        );
        // Need at least 2 real points to render a meaningful line.
        // Single-point series create tiny artifacts at the chart edge.
        if (inWindow.length < 2) return null;
        const renderable = ensureRenderablePoints(inWindow, windowStart);
        if (renderable.length === 0) return null;
        // Downsample to target resolution — LTTB preserves visual shape (peaks, valleys, trends).
        const drawData = renderable.length > targetPoints * 1.5
          ? downsampleLTTB(renderable, targetPoints)
          : renderable;
        const segments = splitPointsOnGaps(
          drawData,
          inferGapThresholdMs(drawData, rangeMs),
          windowStart,
          bridgeLeadingGap,
        );
        return {
          ...series,
          hoverData: renderable,
          drawData,
          segments,
        };
      })
      .filter((series): series is {
        id?: string;
        data: MetricPoint[];
        color: string;
        name?: string;
        hoverData: MetricPoint[];
        drawData: MetricPoint[];
        segments: MetricPoint[][];
      } => series !== null);

    if (validSeries.length === 0) {
      return {
        validSeries: [] as Array<{
          id?: string;
          data: MetricPoint[];
          color: string;
          name?: string;
          hoverData: MetricPoint[];
          drawData: MetricPoint[];
          segments: MetricPoint[][];
        }>,
        paths: [] as { path: string; color: string; seriesIndex: number }[],
        windowStart,
        rangeMs,
        scaleMax: 0,
      };
    }

    let scaleMax = 100;
    if (yMode() === 'auto') {
      scaleMax = 0;
      for (const series of validSeries) {
        for (const point of series.hoverData) {
          if (point.value > scaleMax) scaleMax = point.value;
        }
      }
      if (scaleMax <= 0) {
        return {
          validSeries,
          paths: [] as { path: string; color: string; seriesIndex: number }[],
          windowStart,
          rangeMs,
          scaleMax: 0,
        };
      }
    }

    // Break lines across large gaps so we don't imply continuity through missing data.
    // Skip SVG path generation in canvas mode to avoid expensive string allocation.
    const paths = shouldUseCanvas()
      ? []
      : (() => {
          return validSeries.flatMap((series, index) => {
            return series.segments.map((segment) => {
              const points = segment.map((point) => {
                const x = clamp(((point.timestamp - windowStart) / rangeMs) * vbW, 0, vbW);
                const normalized = yMode() === 'auto'
                  ? Math.max(0, point.value) / scaleMax
                  : Math.min(Math.max(point.value, 0), 100) / 100;
                const y = vbH - normalized * vbH;
                return `${x.toFixed(1)},${y.toFixed(1)}`;
              });
              return { path: `M${points.join('L')}`, color: series.color, seriesIndex: index };
            });
          });
        })();

    return {
      validSeries,
      paths,
      windowStart,
      rangeMs,
      scaleMax,
    };
  });

  let pendingHoverPosition: { clientX: number; clientY: number } | null = null;
  let hoverRafId: number | null = null;

  const computeHoverState = (clientX: number, clientY: number) => {
    const computed = chartData();
    if (!chartSurfaceRef || computed.validSeries.length === 0 || computed.rangeMs <= 0) return;
    const rect = chartSurfaceRef.getBoundingClientRect();
    if (rect.width <= 0) return;

    const mouseX = Math.max(0, Math.min(clientX - rect.left, rect.width));
    const chartX = (mouseX / rect.width) * vbW;
    const targetTimestamp = computed.windowStart + (chartX / vbW) * computed.rangeMs;
    const shouldTrackNearest = props.highlightNearestSeriesOnHover === true;
    const chartY = shouldTrackNearest
      ? (Math.max(0, Math.min(clientY - rect.top, rect.height)) / rect.height) * vbH
      : 0;
    const valueToChartY = (value: number): number => {
      if (yMode() === 'auto') {
        if (computed.scaleMax <= 0) return vbH;
        return vbH - (Math.max(0, value) / computed.scaleMax) * vbH;
      }
      return vbH - (Math.min(Math.max(value, 0), 100) / 100) * vbH;
    };

    let nearestSeriesIndex: number | null = null;
    let nearestDistance = Number.POSITIVE_INFINITY;
    const values: HoverSeriesValue[] = [];
    for (let seriesIndex = 0; seriesIndex < computed.validSeries.length; seriesIndex++) {
      const series = computed.validSeries[seriesIndex];
      const point = findNearestMetricPoint(series.hoverData, targetTimestamp);
      if (!point) continue;

      if (shouldTrackNearest) {
        const pointY = valueToChartY(point.value);
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
      setHoveredState(null);
      return;
    }

    const nearLineThresholdPx = 6;
    const nearLineThresholdChartUnits = (nearLineThresholdPx / Math.max(1, rect.height)) * vbH;
    const lockedIndex = shouldTrackNearest ? lockedSeriesIndex() : null;
    const effectiveSeriesIndex =
      lockedIndex !== null
        ? lockedIndex
        : (
          shouldTrackNearest &&
          nearestSeriesIndex !== null &&
          nearestDistance <= nearLineThresholdChartUnits
            ? nearestSeriesIndex
            : null
        );
    const focusedTooltip = effectiveSeriesIndex !== null;

    const tooltipValues: HoverSeriesValue[] = focusedTooltip && effectiveSeriesIndex !== null
      ? values.filter((value) => value.seriesIndex === effectiveSeriesIndex)
      : (
        props.sortTooltipByValue
          ? selectTopValuesByValue(values, maxRows())
          : values.slice(0, maxRows())
      );
    if (tooltipValues.length === 0) {
      setHoveredState(null);
      return;
    }
    const totalValues = focusedTooltip ? tooltipValues.length : values.length;

    let tooltipX = clientX;
    let tooltipY = rect.top - 6;
    if (typeof window !== 'undefined') {
      const activeTooltipWidth = focusedTooltip ? 150 : tooltipEstimatedWidth;
      const minTooltipX = tooltipPadding + activeTooltipWidth / 2;
      const maxTooltipX = Math.max(minTooltipX, window.innerWidth - tooltipPadding - activeTooltipWidth / 2);
      tooltipX = clamp(tooltipX, minTooltipX, maxTooltipX);

      const shownRows = tooltipValues.length;
      const tooltipEstimatedHeight = 22 + shownRows * 16 + (totalValues > shownRows ? 14 : 0);
      const minTooltipY = tooltipEstimatedHeight + tooltipPadding;
      tooltipY = Math.max(tooltipY, minTooltipY);
    }

    setHoveredState({
      x: chartX,
      tooltipX,
      tooltipY,
      timestamp: tooltipValues[0].timestamp,
      totalValues,
      nearestSeriesIndex,
      highlightedSeriesIndex:
        shouldTrackNearest && focusedTooltip ? effectiveSeriesIndex : null,
      focusedTooltip,
      values: tooltipValues.map((value) => ({
        name: value.name,
        color: value.color,
        value: value.value,
        seriesIndex: value.seriesIndex,
      })),
    });
  };

  const flushHoverState = () => {
    hoverRafId = null;
    if (!pendingHoverPosition) return;
    const position = pendingHoverPosition;
    pendingHoverPosition = null;
    computeHoverState(position.clientX, position.clientY);
  };

  const handleMouseMove = (e: MouseEvent) => {
    const shouldThrottle = chartData().validSeries.length > 80;
    if (typeof window === 'undefined' || !shouldThrottle) {
      computeHoverState(e.clientX, e.clientY);
      return;
    }
    pendingHoverPosition = { clientX: e.clientX, clientY: e.clientY };
    if (hoverRafId !== null) return;
    hoverRafId = window.requestAnimationFrame(flushHoverState);
  };

  const handleMouseLeave = () => {
    pendingHoverPosition = null;
    if (typeof window !== 'undefined' && hoverRafId !== null) {
      window.cancelAnimationFrame(hoverRafId);
      hoverRafId = null;
    }
    setHoveredState(null);
  };

  const handleClick = () => {
    if (!props.highlightNearestSeriesOnHover) return;
    const locked = lockedSeriesIndex();
    if (locked !== null) {
      setLockedSeriesIndex(null);
      return;
    }
    const hovered = hoveredState();
    if (!hovered) return;
    const candidateSeriesIndex = hovered.highlightedSeriesIndex ?? hovered.nearestSeriesIndex;
    if (candidateSeriesIndex === null) return;
    setLockedSeriesIndex(candidateSeriesIndex);
  };

  createEffect(() => {
    const locked = lockedSeriesIndex();
    if (locked === null) return;
    if (locked < 0 || locked >= chartData().validSeries.length) {
      setLockedSeriesIndex(null);
    }
  });

  const topLabel = createMemo(() => {
    if (yMode() === 'percent') return '100%';
    const maxValue = chartData().scaleMax;
    if (maxValue <= 0) return '0';
    return props.formatTopLabel ? props.formatTopLabel(maxValue) : maxValue.toFixed(1);
  });

  const axisTicks = createMemo(() => {
    if (yMode() === 'percent') {
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
      { label: topLabel(), top: '0%', anchor: 'top' as const },
      { label: '0', top: '100%', anchor: 'bottom' as const },
    ];
  });

  const gridLineY = createMemo(() => {
    if (yMode() === 'percent') {
      return [vbH * 0.2, vbH * 0.4, vbH * 0.6, vbH * 0.8];
    }
    return [vbH * 0.25, vbH * 0.5, vbH * 0.75];
  });

  const gridLineX = createMemo(() => [vbW * 0.5]);

  const xAxisTicks = createMemo(() => {
    const computed = chartData();
    const range = props.timeRange || '1h';
    const rangeToken = props.rangeLabel || range;
    return [
      { left: 0, label: `-${rangeToken}`, anchor: 'start' as const },
      { left: 50, label: formatRelativeOffset(computed.rangeMs / 2), anchor: 'middle' as const },
      { left: 100, label: 'now', anchor: 'end' as const },
    ];
  });
  const xAxisBandPx = 16;

  const externallyHighlightedSeriesIndex = createMemo(() => {
    const highlightId = props.highlightSeriesId;
    if (!highlightId) return null;
    const index = chartData().validSeries.findIndex((series) => series.id === highlightId);
    return index >= 0 ? index : null;
  });

  const activeEmphasisSeriesIndex = createMemo(() => {
    const locked = lockedSeriesIndex();
    if (props.highlightNearestSeriesOnHover && locked !== null) {
      return locked;
    }
    const hovered = hoveredState();
    if (props.highlightNearestSeriesOnHover && hovered?.highlightedSeriesIndex !== null && hovered) {
      return hovered.highlightedSeriesIndex;
    }
    return externallyHighlightedSeriesIndex();
  });

  const drawCanvas = () => {
    if (!canvasRef) return;
    const computed = chartData();
    const ctx = canvasRef.getContext('2d');
    if (!ctx) return;

    const rect = canvasRef.getBoundingClientRect();
    const width = rect.width;
    const height = rect.height;
    if (width <= 0 || height <= 0) return;

    setupCanvasDPR(canvasRef, ctx, width, height);

    const isDark = typeof document !== 'undefined' && document.documentElement.classList.contains('dark');
    const gridColor = isDark ? 'rgba(255, 255, 255, 0.06)' : 'rgba(17, 24, 39, 0.06)';
    const gridColorStrong = isDark ? 'rgba(255, 255, 255, 0.10)' : 'rgba(17, 24, 39, 0.10)';
    const hoverLineColor = isDark ? 'rgba(255, 255, 255, 0.45)' : 'rgba(17, 24, 39, 0.45)';

    const yLines = yMode() === 'percent' ? [0.2, 0.4, 0.6, 0.8] : [0.25, 0.5, 0.75];
    ctx.save();
    ctx.lineWidth = 0.5;
    for (let i = 0; i < yLines.length; i++) {
      const y = yLines[i] * height;
      ctx.strokeStyle = i === 1 ? gridColorStrong : gridColor;
      ctx.beginPath();
      ctx.moveTo(0, y);
      ctx.lineTo(width, y);
      ctx.stroke();
    }
    ctx.strokeStyle = isDark ? 'rgba(255, 255, 255, 0.04)' : 'rgba(17, 24, 39, 0.04)';
    ctx.beginPath();
    ctx.moveTo(width * 0.5, 0);
    ctx.lineTo(width * 0.5, height);
    ctx.stroke();
    ctx.restore();

    if (computed.validSeries.length === 0 || computed.rangeMs <= 0) {
      return;
    }

    const active = activeEmphasisSeriesIndex();
    const valueToY = (value: number): number => {
      if (yMode() === 'auto') {
        if (computed.scaleMax <= 0) return height;
        return height - (Math.max(0, value) / computed.scaleMax) * height;
      }
      return height - (Math.min(Math.max(value, 0), 100) / 100) * height;
    };

    for (let seriesIndex = 0; seriesIndex < computed.validSeries.length; seriesIndex++) {
      const series = computed.validSeries[seriesIndex];
      const lineWidth = active === null ? 1.5 : active === seriesIndex ? 2.8 : 0.9;
      const opacity = active === null ? 0.75 : active === seriesIndex ? 1 : 0.1;

      ctx.save();
      ctx.globalAlpha = opacity;
      ctx.strokeStyle = series.color;
      ctx.lineWidth = lineWidth;
      ctx.lineCap = 'round';
      ctx.lineJoin = 'round';

      for (const segment of series.segments) {
        if (segment.length === 0) continue;
        ctx.beginPath();
        for (let i = 0; i < segment.length; i++) {
          const point = segment[i];
          const x = clamp(((point.timestamp - computed.windowStart) / computed.rangeMs) * width, 0, width);
          const y = valueToY(point.value);
          if (i === 0) {
            ctx.moveTo(x, y);
          } else {
            ctx.lineTo(x, y);
          }
        }
        ctx.stroke();
      }
      ctx.restore();
    }

    const hover = hoveredState();
    if (hover) {
      ctx.save();
      ctx.strokeStyle = hoverLineColor;
      ctx.lineWidth = 1;
      ctx.setLineDash([3, 3]);
      const x = (hover.x / vbW) * width;
      ctx.beginPath();
      ctx.moveTo(x, 0);
      ctx.lineTo(x, height);
      ctx.stroke();
      ctx.restore();
    }
  };

  let unregisterCanvasDraw: (() => void) | null = null;
  const queueCanvasDraw = () => {
    if (!shouldUseCanvas()) return;
    if (unregisterCanvasDraw) {
      unregisterCanvasDraw();
    }
    unregisterCanvasDraw = scheduleSparkline(drawCanvas);
  };

  createEffect(() => {
    if (!shouldUseCanvas()) {
      if (unregisterCanvasDraw) {
        unregisterCanvasDraw();
        unregisterCanvasDraw = null;
      }
      return;
    }

    void chartData();
    void activeEmphasisSeriesIndex();
    void hoveredState();
    queueCanvasDraw();
  });

  createEffect(() => {
    if (!shouldUseCanvas() || !canvasHostRef) return;
    const observer = new ResizeObserver(() => queueCanvasDraw());
    observer.observe(canvasHostRef);
    onCleanup(() => observer.disconnect());
  });

  onCleanup(() => {
    pendingHoverPosition = null;
    if (typeof window !== 'undefined' && hoverRafId !== null) {
      window.cancelAnimationFrame(hoverRafId);
      hoverRafId = null;
    }
    if (unregisterCanvasDraw) {
      unregisterCanvasDraw();
      unregisterCanvasDraw = null;
    }
  });

  return (
    <div class="w-full h-full min-h-[88px] flex flex-col">
      <div class="relative flex-1 min-h-0">
        <div class="absolute inset-y-0 left-0 w-7 pointer-events-none">
          <For each={axisTicks()}>
            {(tick) => (
              <span
                class="absolute left-0 text-[8px] leading-none text-slate-400 dark:text-slate-500"
                style={{
                  top: tick.top,
                  transform:
                    tick.anchor === 'top'
                      ? 'translateY(0)'
                      : tick.anchor === 'bottom'
                        ? 'translateY(-100%)'
                        : 'translateY(-50%)',
                }}
              >
                {tick.label}
              </span>
            )}
          </For>
        </div>
        <div class="h-full ml-7 mr-3" ref={canvasHostRef}>
          <Show
            when={shouldUseCanvas()}
            fallback={
              <svg
                ref={(el) => {
                  chartSurfaceRef = el;
                }}
                class="w-full h-full cursor-crosshair"
                viewBox={`0 0 ${vbW} ${vbH}`}
                preserveAspectRatio="none"
                onMouseMove={handleMouseMove}
                onMouseLeave={handleMouseLeave}
                onClick={handleClick}
              >
                <For each={gridLineY()}>
                  {(y, index) => (
                    <line
                      x1="0"
                      y1={y}
                      x2={vbW}
                      y2={y}
                      stroke="currentColor"
                      stroke-opacity={index() === 1 ? '0.1' : '0.06'}
                      stroke-width="0.5"
                    />
                  )}
                </For>

                <For each={gridLineX()}>
                  {(x) => (
                    <line
                      x1={x}
                      y1="0"
                      x2={x}
                      y2={vbH}
                      stroke="currentColor"
                      stroke-opacity="0.04"
                      stroke-width="0.5"
                    />
                  )}
                </For>

                <Show when={hoveredState()}>
                  {(hover) => (
                    <line
                      x1={hover().x}
                      y1="0"
                      x2={hover().x}
                      y2={vbH}
                      stroke="currentColor"
                      stroke-opacity="0.45"
                      stroke-width="1"
                      stroke-dasharray="3 3"
                      vector-effect="non-scaling-stroke"
                    />
                  )}
                </Show>

                <For each={chartData().paths}>
                  {(pathData) => (
                    <path
                      d={pathData.path}
                      fill="none"
                      stroke={pathData.color}
                      stroke-width={(() => {
                        const active = activeEmphasisSeriesIndex();
                        if (active === null) {
                          return '1.5';
                        }
                        return active === pathData.seriesIndex ? '2.8' : '0.9';
                      })()}
                      stroke-linecap="round"
                      stroke-linejoin="round"
                      opacity={(() => {
                        const active = activeEmphasisSeriesIndex();
                        if (active === null) {
                          return '0.75';
                        }
                        return active === pathData.seriesIndex ? '1' : '0.1';
                      })()}
                      style={{ transition: 'opacity 90ms linear, stroke-width 90ms linear' }}
                      vector-effect="non-scaling-stroke"
                    />
                  )}
                </For>
              </svg>
            }
          >
            <canvas
              ref={(el) => {
                canvasRef = el;
                chartSurfaceRef = el;
              }}
              class="w-full h-full cursor-crosshair block"
              onMouseMove={handleMouseMove}
              onMouseLeave={handleMouseLeave}
              onClick={handleClick}
            />
          </Show>
        </div>
      </div>
      <div
        class="relative pointer-events-none ml-7 mr-3"
        style={{ height: `${xAxisBandPx}px` }}
      >
        <For each={xAxisTicks()}>
          {(tick) => (
            <span
              class="absolute top-[2px] text-[9px] font-medium leading-none text-slate-500 dark:text-slate-400"
              style={{
                left: `${tick.left}%`,
                transform:
                  tick.anchor === 'start'
                    ? 'translateX(0)'
                    : tick.anchor === 'end'
                      ? 'translateX(-100%)'
                      : 'translateX(-50%)',
              }}
            >
              {tick.label}
            </span>
          )}
        </For>
      </div>

      <Portal>
        <Show when={hoveredState()}>
          {(hover) => (
            <div
              class="fixed pointer-events-none bg-slate-900 dark:bg-slate-800 text-white text-xs rounded px-2 py-1.5 shadow-sm border border-slate-700"
              style={{
                left: `${hover().tooltipX}px`,
                top: `${hover().tooltipY}px`,
                transform: 'translate(-50%, -100%)',
                'z-index': '9999',
              }}
            >
              <div class="font-medium text-center mb-1">{formatHoverTime(hover().timestamp)}</div>
              <For each={hover().values}>
                {(entry) => (
                  <div class={`flex items-center gap-1.5 leading-tight ${
                    props.highlightNearestSeriesOnHover &&
                    hover().focusedTooltip &&
                    hover().highlightedSeriesIndex !== null
                      ? hover().highlightedSeriesIndex === entry.seriesIndex
                        ? 'text-white bg-white rounded px-1'
                        : 'opacity-40'
                      : ''
                  }`}>
                    <span class="w-1.5 h-1.5 rounded-full" style={{ background: entry.color }} />
                    <span class="text-slate-300">{entry.name}</span>
                    <span class="ml-auto font-medium text-white">{formatValue(entry.value)}</span>
                  </div>
                )}
              </For>
              <Show when={hover().totalValues > hover().values.length}>
                <div class="text-[10px] text-slate-400 mt-0.5">+{hover().totalValues - hover().values.length} more series</div>
              </Show>
            </div>
          )}
        </Show>
      </Portal>
    </div>
  );
};

export default InteractiveSparkline;
