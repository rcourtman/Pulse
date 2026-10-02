import { createEffect, createMemo, createSignal, onCleanup, onMount } from 'solid-js';
import { ChartsAPI, type HistoryTimeRange } from '@/api/charts';
import { isRangeLocked, loadRuntimeCapabilities, maxHistoryDays } from '@/stores/license';
import { calculateOptimalPoints } from '@/utils/downsample';
import { setupCanvasDPR } from '@/utils/canvasRenderQueue';
import {
  HISTORY_CHART_RANGES,
  HISTORY_CHART_MIN_LEFT_INSET,
  createHistoryChartGeometry,
  findHistoryChartClosestPoint,
  formatHistoryChartTimeLabel,
  getHistoryChartDataMax,
  getHistoryChartDataMin,
  getHistoryChartDefaultColor,
  getHistoryChartLeftInset,
  getHistoryChartRefreshIntervalMs,
  getHistoryChartRightInset,
  getHistoryChartScale,
  getHistoryChartYAxisLabels,
  type HistoryChartProps,
  type HistoryChartHoverPoint,
} from './historyChartModel';
import type { HistoryChartHoverGroupState } from './HistoryChartHoverGroup';

interface HistoryChartRefs {
  getCanvas: () => HTMLCanvasElement | undefined;
  getContainer: () => HTMLDivElement | undefined;
}

export function useHistoryChartState(
  props: HistoryChartProps,
  refs: HistoryChartRefs,
  hoverGroup?: HistoryChartHoverGroupState,
) {
  const [range, setRange] = createSignal<HistoryTimeRange>(props.range || '24h');
  const [data, setData] = createSignal(props.data ?? []);
  const [keyboardInspecting, setKeyboardInspecting] = createSignal(false);
  const [loading, setLoading] = createSignal(false);
  const [refreshFailed, setRefreshFailed] = createSignal(false);
  const [error, setError] = createSignal<string | null>(null);
  const [source, setSource] = createSignal<'store' | 'memory' | 'live' | 'mock_synthetic' | null>(
    null,
  );
  const [maxPoints, setMaxPoints] = createSignal<number | null>(null);
  const [localHoveredTimestamp, setLocalHoveredTimestamp] = createSignal<number | null>(null);
  const [hoveredPoint, setHoveredPoint] = createSignal<HistoryChartHoverPoint | null>(null);
  const [chartWidth, setChartWidth] = createSignal(300);
  const chartHeight = createMemo(() => props.height || 200);
  let chartLeftInset = HISTORY_CHART_MIN_LEFT_INSET;
  let chartRightInset = 0;
  const hoveredTimestamp = hoverGroup?.hoveredTimestamp ?? localHoveredTimestamp;
  const setHoveredTimestamp = hoverGroup?.setHoveredTimestamp ?? setLocalHoveredTimestamp;

  const refreshIntervalMs = createMemo(() => getHistoryChartRefreshIntervalMs(range()));

  onMount(() => {
    loadRuntimeCapabilities();
  });

  createEffect(() => {
    if (props.range) {
      setRange(props.range);
    }
  });

  const updateRange = (nextRange: HistoryTimeRange) => {
    setRange(nextRange);
    props.onRangeChange?.(nextRange);
  };

  const isLocked = createMemo(() => isRangeLocked(range()));
  const lockDays = createMemo(() => {
    switch (range()) {
      case '14d':
        return '14';
      case '30d':
        return '30';
      case '90d':
        return '90';
      default:
        return '14';
    }
  });
  const lockTierLabel = createMemo(() => {
    const max = maxHistoryDays();
    const targetDays =
      range() === '14d' ? 14 : range() === '30d' ? 30 : range() === '90d' ? 90 : 14;
    if (max <= 7 && targetDays <= 14) return 'Relay';
    return 'Pro';
  });

  const dataMin = createMemo(() => getHistoryChartDataMin(data()));
  const dataMax = createMemo(() => getHistoryChartDataMax(data()));

  let previousSelection: string | undefined;

  // One effect owns a selection, its request and its polling timer. Cleanup
  // invalidates completions even when a transport ignores cancellation.
  createEffect(() => {
    const suppliedData = props.data;
    const resourceId = props.resourceId;
    const resourceType = props.resourceType;
    const metric = props.metric;
    const chartRange = range();
    const pointsCap = maxPoints();
    const locked = isLocked();
    const interval = refreshIntervalMs();
    let active = true;
    let pending = false;
    let hasLoaded = false;
    let controller: AbortController | undefined;
    let timer: number | undefined;

    onCleanup(() => {
      active = false;
      controller?.abort();
      if (timer !== undefined) window.clearInterval(timer);
    });

    setData(suppliedData ?? []);
    setSource(suppliedData !== undefined ? 'live' : null);
    setError(null);
    setRefreshFailed(false);
    setLoading(false);
    const selection = JSON.stringify([
      resourceType,
      resourceId,
      metric,
      chartRange,
      pointsCap,
      locked,
      suppliedData !== undefined,
    ]);
    if (selection !== previousSelection) {
      setHoveredPoint(null);
      setHoveredTimestamp(null);
    }
    previousSelection = selection;
    if (suppliedData !== undefined || locked || !resourceId || !resourceType) return;

    const loadData = async () => {
      if (!active || pending) return;
      pending = true;
      controller = new AbortController();
      if (!hasLoaded) setLoading(true);
      setError(null);
      try {
        const result = await ChartsAPI.getMetricsHistory({
          resourceType,
          resourceId,
          metric,
          range: chartRange,
          maxPoints: pointsCap ?? undefined,
          signal: controller.signal,
        });
        if (!active) return;
        setData('points' in result ? (result.points ?? []) : []);
        setSource(result.source ?? 'store');
        setRefreshFailed(false);
        hasLoaded = true;
      } catch (err) {
        if (!active) return;
        console.error('Failed to fetch metrics history:', err);
        if (hasLoaded) setRefreshFailed(true);
        else setError('Failed to load history data');
      } finally {
        if (active) {
          pending = false;
          setLoading(false);
        }
      }
    };

    void loadData();
    if (interval > 0) timer = window.setInterval(() => void loadData(), interval);
  });

  const drawChart = () => {
    const canvas = refs.getCanvas();
    if (!canvas) return;
    const ctx = canvas.getContext('2d');
    if (!ctx) return;

    const points = data();
    const width = canvas.parentElement?.clientWidth || 300;
    const height = chartHeight();

    setupCanvasDPR(canvas, ctx, width, height);
    ctx.clearRect(0, 0, width, height);

    const isDark = document.documentElement.classList.contains('dark');
    const gridColor = isDark ? 'rgba(255, 255, 255, 0.1)' : 'rgba(0, 0, 0, 0.05)';
    const textColor = isDark ? '#9ca3af' : '#6b7280';
    const axisTextColor = isDark ? '#9ca3af' : '#6b7280';
    const mainColor = getHistoryChartDefaultColor(props.metric, props.color);
    const scale = getHistoryChartScale(points, props.unit);
    const yAxisTicks = getHistoryChartYAxisLabels(scale, props.unit);
    const labelCount = 4;
    const timeAxisTicks =
      points.length > 0
        ? Array.from({ length: labelCount }, (_, index) => {
            const timestamp =
              points[0].timestamp +
              ((points[points.length - 1].timestamp - points[0].timestamp) * index) /
                (labelCount - 1);
            return { timestamp, label: formatHistoryChartTimeLabel(timestamp, range()) };
          })
        : [];

    ctx.font = '10px sans-serif';
    chartLeftInset = getHistoryChartLeftInset(
      yAxisTicks.map((tick) => ctx.measureText(tick.label).width),
    );
    chartRightInset = getHistoryChartRightInset(
      timeAxisTicks.length > 0
        ? ctx.measureText(timeAxisTicks[timeAxisTicks.length - 1].label).width
        : 0,
    );

    ctx.strokeStyle = gridColor;
    ctx.lineWidth = 1;
    for (const tick of yAxisTicks) {
      const y = height - 20 - tick.pct * (height - 40);
      ctx.beginPath();
      ctx.moveTo(chartLeftInset, y);
      ctx.lineTo(width - chartRightInset, y);
      ctx.stroke();

      ctx.fillStyle = textColor;
      ctx.textAlign = 'right';
      ctx.textBaseline = 'middle';
      ctx.fillText(tick.label, chartLeftInset - 5, y);
    }

    if (points.length === 0) {
      setHoveredPoint(null);
      return;
    }

    const geometry = createHistoryChartGeometry({
      width,
      height,
      startTime: points[0].timestamp,
      endTime: points[points.length - 1].timestamp,
      minValue: scale.minValue,
      maxValue: scale.maxValue,
      leftInset: chartLeftInset,
      rightInset: chartRightInset,
    });

    ctx.beginPath();
    points.forEach((point, index) => {
      if (index === 0) ctx.moveTo(geometry.getX(point.timestamp), height - 20);
      ctx.lineTo(geometry.getX(point.timestamp), geometry.getY(point.value));
    });
    if (points.length > 0) {
      ctx.lineTo(geometry.getX(points[points.length - 1].timestamp), height - 20);
    }
    ctx.closePath();
    ctx.fillStyle = `${mainColor}66`;
    ctx.fill();

    ctx.beginPath();
    ctx.strokeStyle = mainColor;
    ctx.lineWidth = 2;
    points.forEach((point, index) => {
      if (index === 0) ctx.moveTo(geometry.getX(point.timestamp), geometry.getY(point.value));
      else ctx.lineTo(geometry.getX(point.timestamp), geometry.getY(point.value));
    });
    ctx.stroke();

    ctx.fillStyle = axisTextColor;
    ctx.font = '10px sans-serif';
    ctx.textAlign = 'center';
    ctx.textBaseline = 'bottom';

    for (const tick of timeAxisTicks) {
      ctx.fillText(tick.label, geometry.getX(tick.timestamp), height - 2);
    }

    const hoverTimestamp = hoveredTimestamp();
    if (
      hoverTimestamp === null ||
      hoverTimestamp < points[0].timestamp ||
      hoverTimestamp > points[points.length - 1].timestamp
    ) {
      setHoveredPoint(null);
      return;
    }

    const cursor = geometry.getX(hoverTimestamp);

    ctx.save();
    ctx.strokeStyle = isDark ? 'rgba(255, 255, 255, 0.4)' : 'rgba(0, 0, 0, 0.3)';
    ctx.lineWidth = 1;
    ctx.setLineDash([4, 4]);
    ctx.beginPath();
    ctx.moveTo(cursor, 0);
    ctx.lineTo(cursor, height - 20);
    ctx.stroke();
    ctx.restore();

    const closest = findHistoryChartClosestPoint(points, hoverTimestamp);
    const pointX = geometry.getX(closest.timestamp);
    const pointY = geometry.getY(closest.value);

    ctx.beginPath();
    ctx.arc(pointX, pointY, 5, 0, Math.PI * 2);
    ctx.fillStyle = isDark ? '#1f2937' : '#ffffff';
    ctx.fill();

    ctx.beginPath();
    ctx.arc(pointX, pointY, 4, 0, Math.PI * 2);
    ctx.fillStyle = mainColor;
    ctx.fill();

    ctx.beginPath();
    ctx.arc(pointX, pointY, 2, 0, Math.PI * 2);
    ctx.fillStyle = isDark ? 'rgba(255, 255, 255, 0.6)' : 'rgba(255, 255, 255, 0.8)';
    ctx.fill();

    setHoveredPoint({
      value: closest.value,
      timestamp: closest.timestamp,
      x: pointX,
      y: pointY,
    });
  };

  createEffect(() => {
    hoveredTimestamp();
    drawChart();
  });

  createEffect(() => {
    const container = refs.getContainer();
    if (!container) return;

    const updateMaxPoints = () => {
      const width = container.clientWidth || 0;
      if (width <= 0) return;
      setChartWidth(width);
      const next = calculateOptimalPoints(width, 'history');
      if (next !== maxPoints()) {
        setMaxPoints(next);
      }
    };

    const resizeObserver = new ResizeObserver(() => {
      updateMaxPoints();
      drawChart();
    });
    resizeObserver.observe(container);
    updateMaxPoints();
    onCleanup(() => resizeObserver.disconnect());
  });

  const handleFocus = () => {
    setKeyboardInspecting(true);
    const points = data();
    setHoveredTimestamp(points.length ? points[points.length - 1].timestamp : null);
  };

  const handleBlur = () => {
    setKeyboardInspecting(false);
    setHoveredTimestamp(null);
  };

  const handleKeyDown = (event: KeyboardEvent) => {
    if (event.altKey || event.ctrlKey || event.metaKey) return;
    if (event.key === 'Escape') {
      setHoveredTimestamp(null);
      return;
    }
    const points = data();
    if (!points.length || !['ArrowLeft', 'ArrowRight', 'Home', 'End'].includes(event.key)) return;
    event.preventDefault();
    setKeyboardInspecting(true);
    const timestamp = hoveredTimestamp();
    const index =
      timestamp === null
        ? points.length - 1
        : points.indexOf(findHistoryChartClosestPoint(points, timestamp));
    const next =
      event.key === 'Home'
        ? 0
        : event.key === 'End'
          ? points.length - 1
          : Math.max(0, Math.min(points.length - 1, index + (event.key === 'ArrowLeft' ? -1 : 1)));
    setHoveredTimestamp(points[next].timestamp);
  };

  const handleMouseMove = (event: MouseEvent) => {
    const canvas = refs.getCanvas();
    const points = data();
    if (!canvas || points.length === 0) return;

    setKeyboardInspecting(false);
    const rect = canvas.getBoundingClientRect();
    const x = event.clientX - rect.left;
    const width = rect.width;
    if (x < chartLeftInset || x > width - chartRightInset) {
      setHoveredTimestamp(null);
      return;
    }

    const ratio = (x - chartLeftInset) / (width - chartLeftInset - chartRightInset);
    const timeSpan = Math.max(1, points[points.length - 1].timestamp - points[0].timestamp);
    setHoveredTimestamp(points[0].timestamp + ratio * timeSpan);
  };

  const handleMouseLeave = () => {
    if (keyboardInspecting()) return;
    setHoveredTimestamp(null);
  };

  return {
    data,
    dataMax,
    dataMin,
    error,
    refreshFailed,
    handleFocus,
    handleBlur,
    handleKeyDown,
    keyboardInspecting,
    handleMouseLeave,
    handleMouseMove,
    chartHeight,
    chartWidth,
    hoveredPoint,
    isLocked,
    loading,
    lockDays,
    lockTierLabel,
    range,
    ranges: HISTORY_CHART_RANGES,
    source,
    updateRange,
  };
}

export type HistoryChartState = ReturnType<typeof useHistoryChartState>;
