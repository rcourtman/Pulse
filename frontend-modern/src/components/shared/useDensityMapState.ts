import { createEffect, createMemo, createSignal, onCleanup } from 'solid-js';
import {
  buildDensityMapChartData,
  getDensityMapExternalSeriesIndex,
  buildDensityMapHoveredState,
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
    const activeSeriesIndex = hoveredState()?.seriesIndex ?? externalSeriesIndex();

    for (let row = 0; row < rows; row += 1) {
      const cellY = row * cellHeight;
      const isDimmed = activeSeriesIndex !== null && row !== activeSeriesIndex;
      for (let column = 0; column < DENSITY_MAP_COLUMNS; column += 1) {
        const cellX = column * cellWidth;
        const value = data.grid[row][column];

        if (value <= 0) {
          context.globalAlpha = isDimmed ? 0.45 : 1;
          context.fillStyle = 'rgba(128, 128, 128, 0.05)';
          context.fillRect(
            cellX + DENSITY_MAP_PADDING_X / 2,
            cellY + DENSITY_MAP_PADDING_Y / 2,
            cellWidth - DENSITY_MAP_PADDING_X,
            cellHeight - DENSITY_MAP_PADDING_Y,
          );
          continue;
        }

        context.globalAlpha =
          getDensityMapCellOpacity(value, data.globalMax) * (isDimmed ? 0.18 : 1);
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

    context.globalAlpha = 1;
  };

  createEffect(() => {
    drawCanvas();
  });

  const handleMouseMove = (event: MouseEvent) => {
    const canvas = canvasRef();
    if (!canvas) return;

    setHoveredState(
      buildDensityMapHoveredState({
        clientX: event.clientX,
        clientY: event.clientY,
        rect: canvas.getBoundingClientRect(),
        data: chartData(),
      }),
    );
  };

  const handleMouseLeave = () => {
    setHoveredState(null);
  };

  createEffect(() => {
    if (typeof window === 'undefined') return;

    const handleResize = () => drawCanvas();
    window.addEventListener('resize', handleResize);
    onCleanup(() => window.removeEventListener('resize', handleResize));
  });

  return {
    chartData,
    externalSeriesIndex,
    formatValue,
    handleMouseLeave,
    handleMouseMove,
    hoveredState,
    setCanvasRef,
  };
}

export type DensityMapState = ReturnType<typeof useDensityMapState>;
