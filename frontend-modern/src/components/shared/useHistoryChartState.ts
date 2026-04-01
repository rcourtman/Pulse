import {
  createEffect,
  createMemo,
  createSignal,
  onCleanup,
  onMount,
} from 'solid-js';
import { ChartsAPI, type HistoryTimeRange } from '@/api/charts';
import {
  getUpgradeActionUrlOrFallback,
  isRangeLocked,
  licenseStatus,
  loadLicenseStatus,
  maxHistoryDays,
} from '@/stores/license';
import { calculateOptimalPoints } from '@/utils/downsample';
import { setupCanvasDPR } from '@/utils/canvasRenderQueue';
import { trackPaywallViewed, trackUpgradeClicked } from '@/utils/upgradeMetrics';
import { notificationStore } from '@/stores/notifications';
import { runStartProTrialAction } from '@/utils/trialStartAction';
import {
  HISTORY_CHART_RANGES,
  createHistoryChartGeometry,
  findHistoryChartClosestPoint,
  formatHistoryChartTimeLabel,
  getHistoryChartDataMax,
  getHistoryChartDataMin,
  getHistoryChartDefaultColor,
  getHistoryChartRefreshIntervalMs,
  getHistoryChartScale,
  getHistoryChartYAxisLabels,
  type HistoryChartProps,
  type HistoryChartHoverPoint,
} from './historyChartModel';

interface HistoryChartRefs {
  getCanvas: () => HTMLCanvasElement | undefined;
  getContainer: () => HTMLDivElement | undefined;
}

export function useHistoryChartState(props: HistoryChartProps, refs: HistoryChartRefs) {
  const [range, setRange] = createSignal<HistoryTimeRange>(props.range || '24h');
  const [data, setData] = createSignal(props.data ?? []);
  const [loading, setLoading] = createSignal(false);
  const [error, setError] = createSignal<string | null>(null);
  const [source, setSource] = createSignal<'store' | 'memory' | 'live' | 'mock_synthetic' | null>(null);
  const [maxPoints, setMaxPoints] = createSignal<number | null>(null);
  const [refreshTick, setRefreshTick] = createSignal(0);
  const [hasLoadedOnce, setHasLoadedOnce] = createSignal(false);
  const [cursorX, setCursorX] = createSignal<number | null>(null);
  const [startingTrial, setStartingTrial] = createSignal(false);
  const [hoveredPoint, setHoveredPoint] = createSignal<HistoryChartHoverPoint | null>(null);

  const canStartTrial = createMemo(() => {
    const ent = licenseStatus();
    if (!ent) return false;
    if (ent.subscription_state === 'active' || ent.subscription_state === 'trial') return false;
    return ent.trial_eligible !== false;
  });

  const handleStartTrial = async () => {
    if (startingTrial()) return;
    setStartingTrial(true);
    try {
      await runStartProTrialAction({
        branded: true,
        showSuccess: notificationStore.success,
        showError: notificationStore.error,
      });
    } finally {
      setStartingTrial(false);
    }
  };

  const refreshIntervalMs = createMemo(() => getHistoryChartRefreshIntervalMs(range()));

  onMount(() => {
    loadLicenseStatus();
  });

  createEffect(() => {
    if (props.range) {
      setRange(props.range);
    }
  });

  createEffect(() => {
    if (props.data) {
      setData(props.data);
      if (!hasLoadedOnce()) setHasLoadedOnce(true);
      setSource('live');
    }
  });

  const updateRange = (nextRange: HistoryTimeRange) => {
    setRange(nextRange);
    props.onRangeChange?.(nextRange);
  };

  const isLocked = createMemo(() => isRangeLocked(range()));
  const lockDays = createMemo(() => (range() === '30d' ? '30' : '90'));
  const lockTierLabel = createMemo(() => {
    const max = maxHistoryDays();
    const targetDays = range() === '30d' ? 30 : range() === '90d' ? 90 : 14;
    if (max <= 7 && targetDays <= 14) return 'Relay';
    return 'Pro';
  });

  createEffect((wasVisible) => {
    const visible = isLocked() && !props.hideLock;
    if (visible && !wasVisible) {
      trackPaywallViewed('long_term_metrics', 'history_chart');
    }
    return visible;
  }, false);

  const dataMin = createMemo(() => getHistoryChartDataMin(data()));
  const dataMax = createMemo(() => getHistoryChartDataMax(data()));

  const loadData = async (
    chartRange: HistoryTimeRange,
    pointsCap: number | null,
    isBackgroundRefresh: boolean,
  ) => {
    if (!isBackgroundRefresh && !hasLoadedOnce()) {
      setLoading(true);
    }
    setError(null);
    if (!isBackgroundRefresh) {
      setSource(null);
    }

    try {
      const result = await ChartsAPI.getMetricsHistory({
        resourceType: props.resourceType,
        resourceId: props.resourceId,
        metric: props.metric,
        range: chartRange,
        maxPoints: pointsCap ?? undefined,
      });

      if ('points' in result) {
        setData(result.points || []);
        setSource(result.source ?? 'store');
      } else {
        setData([]);
        setSource(result.source ?? 'store');
      }
      if (!hasLoadedOnce()) {
        setHasLoadedOnce(true);
      }
    } catch (err) {
      console.error('Failed to fetch metrics history:', err);
      if (!hasLoadedOnce()) {
        setError('Failed to load history data');
      }
      setSource(null);
    } finally {
      setLoading(false);
    }
  };

  createEffect(async () => {
    if (props.data) return;
    if (!props.resourceId || !props.resourceType) return;

    const chartRange = range();
    const locked = isLocked();
    const pointsCap = maxPoints();

    if (locked) {
      setLoading(false);
      setError(null);
      setSource(null);
      return;
    }

    void loadData(chartRange, pointsCap, false);
  });

  createEffect(() => {
    const tick = refreshTick();
    if (tick === 0) return;
    if (!props.resourceId || !props.resourceType || isLocked()) return;

    void loadData(range(), maxPoints(), true);
  });

  createEffect(() => {
    const interval = refreshIntervalMs();
    if (!interval || interval <= 0) return;
    const timer = window.setInterval(() => {
      setRefreshTick((value) => value + 1);
    }, interval);
    onCleanup(() => window.clearInterval(timer));
  });

  const drawChart = () => {
    const canvas = refs.getCanvas();
    if (!canvas) return;
    const ctx = canvas.getContext('2d');
    if (!ctx) return;

    const points = data();
    const width = canvas.parentElement?.clientWidth || 300;
    const height = props.height || 200;

    setupCanvasDPR(canvas, ctx, width, height);
    ctx.clearRect(0, 0, width, height);

    const isDark = document.documentElement.classList.contains('dark');
    const gridColor = isDark ? 'rgba(255, 255, 255, 0.1)' : 'rgba(0, 0, 0, 0.05)';
    const textColor = isDark ? '#9ca3af' : '#6b7280';
    const axisTextColor = isDark ? '#9ca3af' : '#6b7280';
    const mainColor = getHistoryChartDefaultColor(props.metric, props.color);
    const scale = getHistoryChartScale(points, props.unit);

    ctx.strokeStyle = gridColor;
    ctx.lineWidth = 1;
    for (const tick of getHistoryChartYAxisLabels(scale)) {
      const y = height - 20 - tick.pct * (height - 40);
      ctx.beginPath();
      ctx.moveTo(40, y);
      ctx.lineTo(width, y);
      ctx.stroke();

      ctx.fillStyle = textColor;
      ctx.font = '10px sans-serif';
      ctx.textAlign = 'right';
      ctx.textBaseline = 'middle';
      ctx.fillText(tick.label, 35, y);
    }

    if (points.length === 0) {
      return;
    }

    const geometry = createHistoryChartGeometry({
      width,
      height,
      startTime: points[0].timestamp,
      endTime: points[points.length - 1].timestamp,
      minValue: scale.minValue,
      maxValue: scale.maxValue,
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

    const labelCount = 4;
    for (let index = 0; index < labelCount; index++) {
      const timestamp =
        points[0].timestamp + (geometry.timeSpan * index) / (labelCount - 1);
      const x = geometry.getX(timestamp);
      ctx.fillText(formatHistoryChartTimeLabel(timestamp, range()), x, height - 2);
    }

    const cursor = cursorX();
    if (cursor === null || cursor < 40 || points.length === 0) return;

    ctx.save();
    ctx.strokeStyle = isDark ? 'rgba(255, 255, 255, 0.4)' : 'rgba(0, 0, 0, 0.3)';
    ctx.lineWidth = 1;
    ctx.setLineDash([4, 4]);
    ctx.beginPath();
    ctx.moveTo(cursor, 0);
    ctx.lineTo(cursor, height - 20);
    ctx.stroke();
    ctx.restore();

    const ratio = (cursor - 40) / (width - 40);
    const hoverTimestamp = points[0].timestamp + ratio * geometry.timeSpan;
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
  };

  createEffect(() => {
    cursorX();
    drawChart();
  });

  createEffect(() => {
    const container = refs.getContainer();
    if (!container) return;

    const updateMaxPoints = () => {
      const width = container.clientWidth || 0;
      if (width <= 0) return;
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

  const handleMouseMove = (event: MouseEvent) => {
    const canvas = refs.getCanvas();
    const points = data();
    if (!canvas || points.length === 0) return;

    const rect = canvas.getBoundingClientRect();
    const x = event.clientX - rect.left;
    const width = rect.width;
    const geometry = createHistoryChartGeometry({
      width,
      height: props.height || 200,
      startTime: points[0].timestamp,
      endTime: points[points.length - 1].timestamp,
      minValue: getHistoryChartScale(points, props.unit).minValue,
      maxValue: getHistoryChartScale(points, props.unit).maxValue,
    });

    if (x < 40) {
      setCursorX(null);
      setHoveredPoint(null);
      return;
    }

    setCursorX(x);
    const ratio = (x - 40) / (width - 40);
    const hoverTimestamp = points[0].timestamp + ratio * geometry.timeSpan;
    const closest = findHistoryChartClosestPoint(points, hoverTimestamp);

    setHoveredPoint({
      value: closest.value,
      timestamp: closest.timestamp,
      x: rect.left + x,
      y: rect.top + 20,
    });
  };

  const handleMouseLeave = () => {
    setHoveredPoint(null);
    setCursorX(null);
  };

  return {
    canStartTrial,
    data,
    dataMax,
    dataMin,
    error,
    getUpgradeActionUrlOrFallback,
    handleMouseLeave,
    handleMouseMove,
    handleStartTrial,
    hoveredPoint,
    isLocked,
    loading,
    lockDays,
    lockTierLabel,
    range,
    ranges: HISTORY_CHART_RANGES,
    source,
    startingTrial,
    trackUpgradeClicked,
    updateRange,
  };
}

export type HistoryChartState = ReturnType<typeof useHistoryChartState>;
