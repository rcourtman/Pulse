import { createEffect, createMemo, createSignal, onCleanup } from 'solid-js';
import { scheduleSparkline, setupCanvasDPR } from '@/utils/canvasRenderQueue';
import {
  buildInteractiveSparklineAxisTicks,
  buildInteractiveSparklineChartData,
  getInteractiveSparklineCursorXForTimestamp,
  buildInteractiveSparklineGridLineX,
  buildInteractiveSparklineGridLineY,
  buildInteractiveSparklineSynchronizedHoverState,
  buildInteractiveSparklineTopLabel,
  buildInteractiveSparklineXAxisTicks,
  clampInteractiveSparklineValue,
  computeInteractiveSparklineHoverState,
  createInteractiveSparklineValueToY,
  getInteractiveSparklineActiveEmphasisSeriesIndex,
  getInteractiveSparklineExternalSeriesIndex,
  getInteractiveSparklineShouldUseCanvas,
  type InteractiveSparklineHoverState,
  type InteractiveSparklineProps,
} from './interactiveSparklineModel';

interface InteractiveSparklineRefs {
  getCanvas: () => HTMLCanvasElement | undefined;
  getCanvasHost: () => HTMLDivElement | undefined;
  getChartSurface: () => Element | undefined;
}

export function useInteractiveSparklineState(
  props: InteractiveSparklineProps,
  refs: InteractiveSparklineRefs,
) {
  const vbH = 100;
  const vbW = 200;
  const xAxisBandPx = 16;
  const tooltipPadding = 8;
  const tooltipEstimatedWidth = 190;
  const maxRows = () => props.maxTooltipRows ?? 6;
  const yMode = () => props.yMode ?? 'percent';
  const activeSeriesDisplay = () => props.activeSeriesDisplay ?? 'emphasize';
  const shouldUseCanvas = createMemo(() =>
    getInteractiveSparklineShouldUseCanvas(props.series, props.renderMode),
  );
  const formatValue = (value: number) =>
    props.formatValue ? props.formatValue(value) : `${value.toFixed(1)}%`;
  const interactionOpacityMultiplier = () => (props.interactionState === 'inactive' ? 0.35 : 1);

  const [hoveredState, setHoveredState] = createSignal<InteractiveSparklineHoverState | null>(null);
  const [lockedSeriesIndex, setLockedSeriesIndex] = createSignal<number | null>(null);

  const chartData = createMemo(() =>
    buildInteractiveSparklineChartData({
      series: props.series,
      timeRange: props.timeRange || '1h',
      yMode: yMode(),
      vbW,
      vbH,
      shouldUseCanvas: shouldUseCanvas(),
      bridgeLeadingGap: props.bridgeLeadingGap === true,
    }),
  );

  let pendingHoverPosition: { clientX: number; clientY: number } | null = null;
  let hoverRafId: number | null = null;
  let unregisterCanvasDraw: (() => void) | null = null;

  const computeHoverState = (clientX: number, clientY: number) => {
    const chartSurface = refs.getChartSurface();
    if (!chartSurface) return;
    const computed = computeInteractiveSparklineHoverState({
      chartData: chartData(),
      chartRect: chartSurface.getBoundingClientRect(),
      clientX,
      clientY,
      vbW,
      vbH,
      yMode: yMode(),
      maxRows: maxRows(),
      sortTooltipByValue: props.sortTooltipByValue,
      highlightNearestSeriesOnHover: props.highlightNearestSeriesOnHover,
      lockedSeriesIndex: props.highlightNearestSeriesOnHover ? lockedSeriesIndex() : null,
      tooltipPadding,
      tooltipEstimatedWidth,
    });
    setHoveredState(computed);
    if (!props.onHoverSyncChange || !props.hoverSourceKey) {
      return;
    }
    if (!computed) {
      props.onHoverSyncChange(null);
      return;
    }
    const highlightedIndex = computed?.highlightedSeriesIndex;
    if (highlightedIndex === null || highlightedIndex === undefined) {
      props.onHoverSyncChange(null);
      return;
    }
    const highlightedSeries = chartData().validSeries[highlightedIndex];
    const highlightedSeriesId = highlightedSeries?.id?.trim() || '';
    if (!highlightedSeriesId) {
      props.onHoverSyncChange(null);
      return;
    }
    props.onHoverSyncChange({
      sourceKey: props.hoverSourceKey,
      seriesId: highlightedSeriesId,
      timestamp: computed.timestamp,
    });
  };

  const flushHoverState = () => {
    hoverRafId = null;
    if (!pendingHoverPosition) return;
    const position = pendingHoverPosition;
    pendingHoverPosition = null;
    computeHoverState(position.clientX, position.clientY);
  };

  const handleMouseMove = (event: MouseEvent) => {
    const shouldThrottle = chartData().validSeries.length > 80;
    if (typeof window === 'undefined' || !shouldThrottle) {
      computeHoverState(event.clientX, event.clientY);
      return;
    }
    pendingHoverPosition = { clientX: event.clientX, clientY: event.clientY };
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
    props.onHoverSyncChange?.(null);
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

  const topLabel = createMemo(() =>
    buildInteractiveSparklineTopLabel({
      yMode: yMode(),
      scaleMax: chartData().scaleMax,
      formatTopLabel: props.formatTopLabel,
    }),
  );

  const axisTicks = createMemo(() => buildInteractiveSparklineAxisTicks(yMode(), topLabel()));
  const gridLineY = createMemo(() => buildInteractiveSparklineGridLineY(yMode(), vbH));
  const gridLineX = createMemo(() => buildInteractiveSparklineGridLineX(vbW));
  const xAxisTicks = createMemo(() =>
    buildInteractiveSparklineXAxisTicks({
      rangeMs: chartData().rangeMs,
      rangeLabel: props.rangeLabel,
      timeRange: props.timeRange || '1h',
    }),
  );

  const externalSeriesIndex = createMemo(() =>
    getInteractiveSparklineExternalSeriesIndex(chartData(), props.highlightSeriesId),
  );
  const synchronizedHoverState = createMemo(() => {
    const hoverSync = props.hoverSync;
    if (!hoverSync) {
      return null;
    }
    if (props.hoverSourceKey && hoverSync.sourceKey === props.hoverSourceKey) {
      return null;
    }
    return buildInteractiveSparklineSynchronizedHoverState({
      chartData: chartData(),
      hoverSync,
      vbW,
    });
  });
  const synchronizedHoverTimestamp = createMemo<number | null>(() => {
    const hoverSync = props.hoverSync;
    if (!hoverSync) {
      return null;
    }
    if (props.hoverSourceKey && hoverSync.sourceKey === props.hoverSourceKey) {
      return null;
    }
    return hoverSync.timestamp;
  });
  const activeHoverState = createMemo<InteractiveSparklineHoverState | null>(() => {
    return hoveredState() ?? synchronizedHoverState();
  });
  const activeHoverTimestamp = createMemo<number | null>(() => {
    return hoveredState()?.timestamp ?? synchronizedHoverTimestamp();
  });
  const activeHoverCursorX = createMemo<number | null>(() => {
    const localHover = hoveredState();
    if (localHover) {
      return localHover.x;
    }
    return getInteractiveSparklineCursorXForTimestamp({
      chartData: chartData(),
      timestamp: synchronizedHoverTimestamp(),
      vbW,
    });
  });
  const activeEmphasisSeriesIndex = createMemo(() =>
    getInteractiveSparklineActiveEmphasisSeriesIndex({
      highlightNearestSeriesOnHover: props.highlightNearestSeriesOnHover === true,
      lockedSeriesIndex: lockedSeriesIndex(),
      hoveredState: activeHoverState(),
      externalSeriesIndex: externalSeriesIndex(),
    }),
  );
  const shouldIsolateActiveSeries = createMemo(
    () => activeSeriesDisplay() === 'isolate' && activeEmphasisSeriesIndex() !== null,
  );
  const shouldRenderSeries = (seriesIndex: number) => {
    const active = activeEmphasisSeriesIndex();
    if (active === null) {
      return true;
    }
    if (shouldIsolateActiveSeries()) {
      return active === seriesIndex;
    }
    return true;
  };
  const renderedSeriesCount = createMemo(() => {
    const active = activeEmphasisSeriesIndex();
    if (active === null) {
      return chartData().validSeries.length;
    }
    if (shouldIsolateActiveSeries()) {
      return 1;
    }
    return chartData().validSeries.length;
  });
  const lineWidthForSeries = (seriesIndex: number) => {
    const active = activeEmphasisSeriesIndex();
    const isLg = props.size === 'lg';
    if (active === null) {
      return isLg ? 2 : 1.5;
    }
    if (active === seriesIndex) {
      return isLg ? 4 : 3.2;
    }
    if (shouldIsolateActiveSeries()) {
      return 0;
    }
    return isLg ? 0.7 : 0.6;
  };
  const opacityForSeries = (seriesIndex: number) => {
    const active = activeEmphasisSeriesIndex();
    if (active === null) {
      return 0.75 * interactionOpacityMultiplier();
    }
    if (active === seriesIndex) {
      return interactionOpacityMultiplier();
    }
    if (shouldIsolateActiveSeries()) {
      return 0;
    }
    return 0.05 * interactionOpacityMultiplier();
  };

  const drawCanvas = () => {
    const canvas = refs.getCanvas();
    if (!canvas) return;
    const computed = chartData();
    const ctx = canvas.getContext('2d');
    if (!ctx) return;

    const rect = canvas.getBoundingClientRect();
    const width = rect.width;
    const height = rect.height;
    if (width <= 0 || height <= 0) return;

    setupCanvasDPR(canvas, ctx, width, height);

    const isDark =
      typeof document !== 'undefined' && document.documentElement.classList.contains('dark');
    const gridColor = isDark ? 'rgba(255, 255, 255, 0.06)' : 'rgba(17, 24, 39, 0.06)';
    const gridColorStrong = isDark ? 'rgba(255, 255, 255, 0.10)' : 'rgba(17, 24, 39, 0.10)';
    const hoverLineColor = isDark ? 'rgba(255, 255, 255, 0.45)' : 'rgba(17, 24, 39, 0.45)';
    const inactiveSeriesColor = isDark ? 'rgb(148, 163, 184)' : 'rgb(100, 116, 139)';

    const yLines = yMode() === 'percent' ? [0.2, 0.4, 0.6, 0.8] : [0.25, 0.5, 0.75];
    ctx.save();
    ctx.clearRect(0, 0, width, height);
    ctx.lineWidth = 0.5;
    for (let index = 0; index < yLines.length; index++) {
      const y = yLines[index] * height;
      ctx.strokeStyle = index === 1 ? gridColorStrong : gridColor;
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
    const valueToY = createInteractiveSparklineValueToY(yMode(), computed.scaleMax, height);

    for (let seriesIndex = 0; seriesIndex < computed.validSeries.length; seriesIndex++) {
      if (!shouldRenderSeries(seriesIndex)) {
        continue;
      }
      const series = computed.validSeries[seriesIndex];
      const lineWidth = lineWidthForSeries(seriesIndex);
      const opacity = opacityForSeries(seriesIndex);

      ctx.save();
      ctx.globalAlpha = opacity;
      ctx.strokeStyle =
        active !== null && active !== seriesIndex ? inactiveSeriesColor : series.color;
      ctx.lineWidth = lineWidth;
      ctx.lineCap = 'round';
      ctx.lineJoin = 'round';

      for (const segment of series.segments) {
        if (segment.length === 0) continue;

        if (computed.validSeries.length === 1) {
          ctx.beginPath();
          for (let index = 0; index < segment.length; index++) {
            const point = segment[index];
            const x = clampInteractiveSparklineValue(
              ((point.timestamp - computed.windowStart) / computed.rangeMs) * width,
              0,
              width,
            );
            const y = valueToY(point.value);
            if (index === 0) ctx.moveTo(x, y);
            else ctx.lineTo(x, y);
          }
          const lastPoint = segment[segment.length - 1];
          const firstPoint = segment[0];
          const lastX = clampInteractiveSparklineValue(
            ((lastPoint.timestamp - computed.windowStart) / computed.rangeMs) * width,
            0,
            width,
          );
          const firstX = clampInteractiveSparklineValue(
            ((firstPoint.timestamp - computed.windowStart) / computed.rangeMs) * width,
            0,
            width,
          );

          ctx.lineTo(lastX, height);
          ctx.lineTo(firstX, height);
          ctx.closePath();

          const areaGradient = ctx.createLinearGradient(0, 0, 0, height);
          if (
            series.color.startsWith('#') &&
            (series.color.length === 7 || series.color.length === 4)
          ) {
            areaGradient.addColorStop(0, `${series.color}40`);
            areaGradient.addColorStop(1, `${series.color}00`);
          } else {
            areaGradient.addColorStop(0, 'rgba(255, 255, 255, 0.15)');
            areaGradient.addColorStop(1, 'rgba(255, 255, 255, 0)');
          }
          ctx.fillStyle = areaGradient;
          ctx.fill();
        }

        ctx.beginPath();
        for (let index = 0; index < segment.length; index++) {
          const point = segment[index];
          const x = clampInteractiveSparklineValue(
            ((point.timestamp - computed.windowStart) / computed.rangeMs) * width,
            0,
            width,
          );
          const y = valueToY(point.value);
          if (index === 0) ctx.moveTo(x, y);
          else ctx.lineTo(x, y);
        }
        ctx.stroke();
      }
      ctx.restore();
    }

    const cursorX = activeHoverCursorX();
    if (cursorX === null) return;
    ctx.save();
    const x = (cursorX / vbW) * width;
    const lineStartY = 0;
    const hoverLineGradient = ctx.createLinearGradient(0, lineStartY, 0, height);
    hoverLineGradient.addColorStop(0, 'transparent');
    hoverLineGradient.addColorStop(0.1, hoverLineColor);
    hoverLineGradient.addColorStop(1, hoverLineColor);
    ctx.strokeStyle = hoverLineGradient;
    ctx.lineWidth = 1;
    ctx.setLineDash([3, 3]);
    ctx.beginPath();
    ctx.moveTo(x, lineStartY);
    ctx.lineTo(x, height);
    ctx.stroke();
    ctx.restore();
  };

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
    void activeHoverState();
    void activeHoverCursorX();
    queueCanvasDraw();
  });

  createEffect(() => {
    const canvasHost = refs.getCanvasHost();
    if (!shouldUseCanvas() || !canvasHost) return;
    const observer = new ResizeObserver(() => queueCanvasDraw());
    observer.observe(canvasHost);
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

  return {
    activeEmphasisSeriesIndex,
    activeHoverState,
    activeHoverCursorX,
    activeHoverTimestamp,
    activeSeriesDisplay,
    axisTicks,
    chartData,
    externalSeriesIndex,
    formatValue,
    gridLineX,
    gridLineY,
    handleClick,
    handleMouseLeave,
    handleMouseMove,
    hoveredState,
    lineWidthForSeries,
    opacityForSeries,
    renderedSeriesCount,
    shouldUseCanvas,
    shouldRenderSeries,
    vbH,
    vbW,
    xAxisBandPx,
    xAxisTicks,
  };
}

export type InteractiveSparklineState = ReturnType<typeof useInteractiveSparklineState>;
