import { Component, Show, For, createMemo, createSignal } from 'solid-js';
import { Portal } from 'solid-js/web';
import type { MetricPoint, TimeRange } from '@/api/charts';
import { downsampleLTTB, calculateOptimalPoints } from '@/utils/downsample';
import { timeRangeToMs } from '@/utils/timeRange';

export interface InteractiveSparklineSeries {
  data: MetricPoint[];
  color: string;
  name?: string;
}

interface InteractiveSparklineProps {
  series: InteractiveSparklineSeries[];
  rangeLabel?: string;
  timeRange?: TimeRange;
  yMode?: 'percent' | 'auto';
  formatValue?: (value: number) => string;
  formatTopLabel?: (maxValue: number) => string;
  sortTooltipByValue?: boolean;
  maxTooltipRows?: number;
}

const formatHoverTime = (timestamp: number): string =>
  new Date(timestamp).toLocaleTimeString([], {
    hour: '2-digit',
    minute: '2-digit',
    second: '2-digit',
    hour12: false,
  });

const clamp = (value: number, min: number, max: number): number =>
  Math.max(min, Math.min(value, max));

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

export const InteractiveSparkline: Component<InteractiveSparklineProps> = (props) => {
  let svgRef: SVGSVGElement | undefined;
  const vbH = 100;
  const vbW = 200;
  const tooltipPadding = 8;
  const tooltipEstimatedWidth = 190;
  const maxRows = () => props.maxTooltipRows ?? 6;
  const yMode = () => props.yMode ?? 'percent';
  const formatValue = (value: number) => props.formatValue ? props.formatValue(value) : `${value.toFixed(1)}%`;

  const [hoveredState, setHoveredState] = createSignal<{
    x: number;
    tooltipX: number;
    tooltipY: number;
    timestamp: number;
    values: { name: string; color: string; value: number }[];
  } | null>(null);

  const chartData = createMemo(() => {
    const range = props.timeRange || '1h';
    const rangeMs = timeRangeToMs(range);
    const windowEnd = Date.now();
    const windowStart = windowEnd - rangeMs;
    const targetPoints = calculateOptimalPoints(vbW, 'sparkline');
    const validSeries = props.series
      .map((series) => {
        const inWindow = series.data.filter((point) =>
          point.timestamp >= windowStart && point.timestamp <= windowEnd
        );
        if (inWindow.length < 2) return null;
        const drawData = inWindow.length > targetPoints * 1.5
          ? downsampleLTTB(inWindow, targetPoints)
          : inWindow;
        return {
          ...series,
          hoverData: inWindow,
          drawData,
        };
      })
      .filter((series): series is {
        data: MetricPoint[];
        color: string;
        name?: string;
        hoverData: MetricPoint[];
        drawData: MetricPoint[];
      } => series !== null);

    if (validSeries.length === 0) {
      return {
        validSeries: [] as Array<{
          data: MetricPoint[];
          color: string;
          name?: string;
          hoverData: MetricPoint[];
          drawData: MetricPoint[];
        }>,
        paths: [] as { path: string; color: string }[],
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
          paths: [] as { path: string; color: string }[],
          windowStart,
          rangeMs,
          scaleMax: 0,
        };
      }
    }

    const paths = validSeries.map((series) => {
      const points = series.drawData.map((point) => {
        const x = clamp(((point.timestamp - windowStart) / rangeMs) * vbW, 0, vbW);
        const normalized = yMode() === 'auto'
          ? Math.max(0, point.value) / scaleMax
          : Math.min(Math.max(point.value, 0), 100) / 100;
        const y = vbH - normalized * vbH;
        return `${x.toFixed(1)},${y.toFixed(1)}`;
      });
      return { path: `M${points.join('L')}`, color: series.color };
    });

    return {
      validSeries,
      paths,
      windowStart,
      rangeMs,
      scaleMax,
    };
  });

  const handleMouseMove = (e: MouseEvent) => {
    const computed = chartData();
    if (!svgRef || computed.paths.length === 0 || computed.rangeMs <= 0) return;
    const rect = svgRef.getBoundingClientRect();
    if (rect.width <= 0) return;

    const mouseX = Math.max(0, Math.min(e.clientX - rect.left, rect.width));
    const chartX = (mouseX / rect.width) * vbW;
    const targetTimestamp = computed.windowStart + (chartX / vbW) * computed.rangeMs;

    const values = computed.validSeries
      .map((series) => {
        const point = findNearestMetricPoint(series.hoverData, targetTimestamp);
        if (!point) return null;
        return {
          name: series.name || 'Series',
          color: series.color,
          value: point.value,
          timestamp: point.timestamp,
        };
      })
      .filter((value): value is { name: string; color: string; value: number; timestamp: number } => value !== null);

    if (values.length === 0) {
      setHoveredState(null);
      return;
    }

    if (props.sortTooltipByValue) {
      values.sort((a, b) => b.value - a.value);
    }

    let tooltipX = e.clientX;
    let tooltipY = rect.top - 6;
    if (typeof window !== 'undefined') {
      const minTooltipX = tooltipPadding + tooltipEstimatedWidth / 2;
      const maxTooltipX = Math.max(minTooltipX, window.innerWidth - tooltipPadding - tooltipEstimatedWidth / 2);
      tooltipX = clamp(tooltipX, minTooltipX, maxTooltipX);

      const shownRows = Math.min(values.length, maxRows());
      const tooltipEstimatedHeight = 22 + shownRows * 16 + (values.length > maxRows() ? 14 : 0);
      const minTooltipY = tooltipEstimatedHeight + tooltipPadding;
      tooltipY = Math.max(tooltipY, minTooltipY);
    }

    setHoveredState({
      x: chartX,
      tooltipX,
      tooltipY,
      timestamp: values[0].timestamp,
      values: values.map((value) => ({
        name: value.name,
        color: value.color,
        value: value.value,
      })),
    });
  };

  const handleMouseLeave = () => {
    setHoveredState(null);
  };

  const topLabel = createMemo(() => {
    if (yMode() === 'percent') return '100%';
    const maxValue = chartData().scaleMax;
    if (maxValue <= 0) return '0';
    return props.formatTopLabel ? props.formatTopLabel(maxValue) : maxValue.toFixed(1);
  });

  return (
    <div class="relative w-full h-full">
      <span class="absolute top-0 left-0 text-[8px] leading-none text-gray-400 dark:text-gray-500">{topLabel()}</span>
      <span class="absolute bottom-0 left-0 text-[8px] leading-none text-gray-400 dark:text-gray-500">0</span>
      <span class="absolute bottom-0 right-0 text-[8px] leading-none text-gray-400 dark:text-gray-500">{props.rangeLabel || '1h'}</span>
      <div class="h-full ml-7 mr-3">
        <svg
          ref={svgRef}
          class="w-full h-full cursor-crosshair"
          viewBox={`0 0 ${vbW} ${vbH}`}
          preserveAspectRatio="none"
          onMouseMove={handleMouseMove}
          onMouseLeave={handleMouseLeave}
        >
          <line x1="0" y1={vbH * 0.25} x2={vbW} y2={vbH * 0.25} stroke="currentColor" stroke-opacity="0.06" stroke-width="0.5" />
          <line x1="0" y1={vbH * 0.5} x2={vbW} y2={vbH * 0.5} stroke="currentColor" stroke-opacity="0.1" stroke-width="0.5" />
          <line x1="0" y1={vbH * 0.75} x2={vbW} y2={vbH * 0.75} stroke="currentColor" stroke-opacity="0.06" stroke-width="0.5" />

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
                stroke-width="1.5"
                stroke-linecap="round"
                stroke-linejoin="round"
                opacity="0.75"
                vector-effect="non-scaling-stroke"
              />
            )}
          </For>
        </svg>
      </div>

      <Portal>
        <Show when={hoveredState()}>
          {(hover) => (
            <div
              class="fixed pointer-events-none bg-gray-900 dark:bg-gray-800 text-white text-xs rounded px-2 py-1.5 shadow-lg border border-gray-700"
              style={{
                left: `${hover().tooltipX}px`,
                top: `${hover().tooltipY}px`,
                transform: 'translate(-50%, -100%)',
                'z-index': '9999',
              }}
            >
              <div class="font-medium text-center mb-1">{formatHoverTime(hover().timestamp)}</div>
              <For each={hover().values.slice(0, maxRows())}>
                {(entry) => (
                  <div class="flex items-center gap-1.5 leading-tight">
                    <span class="w-1.5 h-1.5 rounded-full" style={{ background: entry.color }} />
                    <span class="text-gray-300">{entry.name}</span>
                    <span class="ml-auto font-medium text-white">{formatValue(entry.value)}</span>
                  </div>
                )}
              </For>
              <Show when={hover().values.length > maxRows()}>
                <div class="text-[10px] text-gray-400 mt-0.5">+{hover().values.length - maxRows()} more series</div>
              </Show>
            </div>
          )}
        </Show>
      </Portal>
    </div>
  );
};

export default InteractiveSparkline;
