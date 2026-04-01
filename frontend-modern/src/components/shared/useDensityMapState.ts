import { createEffect, createMemo, createSignal, onCleanup } from 'solid-js';
import {
  buildDensityMapChartData,
  buildDensityMapFocusDetail,
  getDensityMapExternalSeriesIndex,
  buildDensityMapHoveredState,
  buildDensityMapSynchronizedHoveredState,
  formatDensityMapValue,
  getDensityMapCellOpacity,
  DENSITY_MAP_COLUMNS,
  DENSITY_MAP_PADDING_X,
  DENSITY_MAP_PADDING_Y,
  type DensityMapHoveredState,
  type DensityMapProps,
} from './densityMapModel';

export function useDensityMapState(props: DensityMapProps) {
  const [hoveredState, setHoveredState] = createSignal<DensityMapHoveredState | null>(null);
  const [canvasRef, setCanvasRef] = createSignal<HTMLCanvasElement>();

  const chartData = createMemo(() =>
    buildDensityMapChartData({
      series: props.series,
      timeRange: props.timeRange,
      highlightSeriesId: props.highlightSeriesId,
    }),
  );
  const externalSeriesIndex = createMemo(() =>
    getDensityMapExternalSeriesIndex(chartData(), props.highlightSeriesId),
  );
  const synchronizedHoveredState = createMemo(() => {
    const hoverSync = props.hoverSync;
    if (!hoverSync) {
      return null;
    }
    if (props.hoverSourceKey && hoverSync.sourceKey === props.hoverSourceKey) {
      return null;
    }
    return buildDensityMapSynchronizedHoveredState({
      data: chartData(),
      hoverSync,
    });
  });
  const activeHoveredState = createMemo<DensityMapHoveredState | null>(() => {
    return hoveredState() ?? synchronizedHoveredState();
  });
  const focusDetail = createMemo(() =>
    buildDensityMapFocusDetail({
      activeHoveredState: activeHoveredState(),
      data: chartData(),
      highlightSeriesId: props.highlightSeriesId,
    }),
  );

  const formatValue = (value: number) => formatDensityMapValue(value, props.formatValue);

  const drawCanvas = () => {
    const canvas = canvasRef();
    if (!canvas) return;

    const context = canvas.getContext('2d');
    if (!context) return;

    const rect = canvas.getBoundingClientRect();
    const width = rect.width;
    const height = rect.height;
    if (width <= 0 || height <= 0) return;

    const dpr = typeof window !== 'undefined' ? window.devicePixelRatio || 1 : 1;
    canvas.width = width * dpr;
    canvas.height = height * dpr;
    context.setTransform(1, 0, 0, 1, 0, 0);
    context.scale(dpr, dpr);
    context.clearRect(0, 0, width, height);

    const data = chartData();
    if (data.series.length === 0 || data.globalMax <= 0) return;

    const rows = data.series.length;
    const cellWidth = width / DENSITY_MAP_COLUMNS;
    const cellHeight = height / rows;
    const interactionOpacity = props.interactionState === 'inactive' ? 0.35 : 1;
    const hover = activeHoveredState();
    const activeSeriesIndex = hover?.seriesIndex ?? externalSeriesIndex();
    const activeSeries =
      activeSeriesIndex !== null && activeSeriesIndex >= 0 && activeSeriesIndex < data.series.length
        ? data.series[activeSeriesIndex]
        : null;
    const hasActiveSeries = activeSeriesIndex !== null;

    for (let row = 0; row < rows; row += 1) {
      const cellY = row * cellHeight;
      const isActiveRow = activeSeriesIndex === row;
      const rowOpacity = !hasActiveSeries ? 1 : isActiveRow ? 1 : 0.42;
      for (let column = 0; column < DENSITY_MAP_COLUMNS; column += 1) {
        const cellX = column * cellWidth;
        const value = data.grid[row][column];

        if (value <= 0) {
          context.globalAlpha = rowOpacity * interactionOpacity;
          context.fillStyle = isActiveRow
            ? 'rgba(148, 163, 184, 0.08)'
            : 'rgba(148, 163, 184, 0.04)';
          context.fillRect(
            cellX + DENSITY_MAP_PADDING_X / 2,
            cellY + DENSITY_MAP_PADDING_Y / 2,
            cellWidth - DENSITY_MAP_PADDING_X,
            cellHeight - DENSITY_MAP_PADDING_Y,
          );
          continue;
        }

        context.globalAlpha =
          getDensityMapCellOpacity(value, data.globalMax) * rowOpacity * interactionOpacity;
        context.fillStyle = data.series[row].color;
        if (context.roundRect) {
          context.beginPath();
          context.roundRect(
            cellX + DENSITY_MAP_PADDING_X / 2,
            cellY + DENSITY_MAP_PADDING_Y / 2,
            cellWidth - DENSITY_MAP_PADDING_X,
            cellHeight - DENSITY_MAP_PADDING_Y,
            2,
          );
          context.fill();
        } else {
          context.fillRect(
            cellX + DENSITY_MAP_PADDING_X / 2,
            cellY + DENSITY_MAP_PADDING_Y / 2,
            cellWidth - DENSITY_MAP_PADDING_X,
            cellHeight - DENSITY_MAP_PADDING_Y,
          );
        }
      }
    }

    if (activeSeries !== null && activeSeriesIndex !== null) {
      const highlightY = activeSeriesIndex * cellHeight;
      context.save();
      if (hover) {
        const highlightX = hover.columnIndex * cellWidth;
        context.globalAlpha = 0.12 * interactionOpacity;
        context.fillStyle = activeSeries.color;
        context.fillRect(highlightX, 0, Math.max(cellWidth, 1), height);
      }
      context.globalAlpha = 0.12 * interactionOpacity;
      context.fillStyle = activeSeries.color;
      context.fillRect(0, highlightY, width, cellHeight);
      context.globalAlpha = 0.7 * interactionOpacity;
      context.strokeStyle = activeSeries.color;
      context.lineWidth = 1.25;
      if (context.roundRect) {
        context.beginPath();
        context.roundRect(0.5, highlightY + 0.5, width - 1, Math.max(cellHeight - 1, 1), 4);
        context.stroke();
      } else {
        context.strokeRect(0.5, highlightY + 0.5, width - 1, Math.max(cellHeight - 1, 1));
      }
      context.restore();
    }

    context.globalAlpha = 1;
  };

  createEffect(() => {
    drawCanvas();
  });

  const handleMouseMove = (event: MouseEvent) => {
    const canvas = canvasRef();
    if (!canvas) return;

    const computed = buildDensityMapHoveredState({
      clientX: event.clientX,
      clientY: event.clientY,
      rect: canvas.getBoundingClientRect(),
      data: chartData(),
    });
    setHoveredState(computed);
    if (!props.onHoverSyncChange || !props.hoverSourceKey || !computed) {
      props.onHoverSyncChange?.(null);
      return;
    }
    const hoveredSeries = chartData().series[computed.seriesIndex];
    const hoveredSeriesId = hoveredSeries?.id?.trim() || '';
    if (!hoveredSeriesId) {
      props.onHoverSyncChange(null);
      return;
    }
    props.onHoverSyncChange({
      sourceKey: props.hoverSourceKey,
      seriesId: hoveredSeriesId,
      timestamp: computed.timestamp,
    });
  };

  const handleMouseLeave = () => {
    setHoveredState(null);
    props.onHoverSyncChange?.(null);
  };

  createEffect(() => {
    if (typeof window === 'undefined') return;

    const handleResize = () => drawCanvas();
    window.addEventListener('resize', handleResize);
    onCleanup(() => window.removeEventListener('resize', handleResize));
  });

  return {
    activeHoveredState,
    chartData,
    externalSeriesIndex,
    focusDetail,
    formatValue,
    handleMouseLeave,
    handleMouseMove,
    hoveredState,
    setCanvasRef,
  };
}

export type DensityMapState = ReturnType<typeof useDensityMapState>;
