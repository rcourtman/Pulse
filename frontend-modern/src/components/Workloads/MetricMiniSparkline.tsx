import { For, Show, createMemo, createSignal, type Component, type JSX } from 'solid-js';
import { TooltipPortal } from '@/components/shared/TooltipPortal';

import {
  buildMetricMiniSparklinePath,
  computeMetricMiniSparklineHoverState,
  formatMetricMiniSparklineHoverTime,
  getMetricMiniSparklineScale,
  getMetricMiniSparklineTimeRange,
  hasRenderableMetricSeries,
  type MetricMiniSparklineHoverState,
  type WorkloadMetricSparklineSeries,
} from './workloadMetricHistoryModel';

type MetricMiniSparklineValueLabelMode = 'inline' | 'tooltip' | 'hidden';

interface MetricMiniSparklineProps {
  series: WorkloadMetricSparklineSeries[];
  valueLabel?: string;
  valueLabelMode?: MetricMiniSparklineValueLabelMode;
  title?: string;
  unit?: string;
  emptyLabel?: string;
  formatValue?: (value: number) => string;
}

type MetricMiniSparklineTooltipState = MetricMiniSparklineHoverState & {
  tooltipX: number;
  tooltipY: number;
};

const SPARKLINE_VIEWBOX_WIDTH = 96;
const SPARKLINE_PLOT_X_PADDING = 1;
const SPARKLINE_PLOT_WIDTH = SPARKLINE_VIEWBOX_WIDTH - SPARKLINE_PLOT_X_PADDING * 2;

const formatDefaultHoverValue = (value: number, unit?: string): string => {
  const absValue = Math.abs(value);
  const formatted = value.toLocaleString(undefined, {
    maximumFractionDigits: absValue >= 10 ? 1 : 2,
  });

  if (unit === '%') return `${Math.round(value)}%`;
  if (unit) return `${formatted} ${unit}`;
  return formatted;
};

export const MetricMiniSparkline: Component<MetricMiniSparklineProps> = (props) => {
  const scale = createMemo(() => getMetricMiniSparklineScale(props.series, props.unit));
  const timeRange = createMemo(() => getMetricMiniSparklineTimeRange(props.series));
  const paths = createMemo(() =>
    props.series
      .map((series) => ({
        ...series,
        path: buildMetricMiniSparklinePath(
          series.points,
          scale(),
          undefined,
          undefined,
          timeRange(),
        ),
      }))
      .filter((series) => series.path.length > 0),
  );
  const hasLine = createMemo(() => hasRenderableMetricSeries(props.series));
  const displayLabel = createMemo(() => props.valueLabel || props.emptyLabel || '—');
  const valueLabelMode = createMemo(() => props.valueLabelMode ?? 'inline');
  const showInlineValue = createMemo(
    () => valueLabelMode() === 'inline' || (valueLabelMode() === 'tooltip' && !hasLine()),
  );
  const [hoveredState, setHoveredState] = createSignal<MetricMiniSparklineTooltipState | null>(
    null,
  );
  const rootColumns = createMemo(() =>
    showInlineValue() ? 'grid-cols-[minmax(2.5rem,1fr)_auto]' : 'grid-cols-[minmax(2.5rem,1fr)]',
  );
  const ariaLabel = createMemo(() => {
    const title = props.title || 'Metric history';
    const value = props.valueLabel ? `, current ${props.valueLabel}` : '';
    return `${title}${value}`;
  });
  const formatHoverValue = (value: number) =>
    props.formatValue?.(value) ?? formatDefaultHoverValue(value, props.unit);
  const cursorX = createMemo(() =>
    hoveredState()
      ? SPARKLINE_PLOT_X_PADDING + hoveredState()!.cursorRatio * SPARKLINE_PLOT_WIDTH
      : 0,
  );
  const handleMouseMove: JSX.EventHandler<SVGSVGElement, MouseEvent> = (event) => {
    if (!hasLine()) {
      setHoveredState(null);
      return;
    }

    const rect = event.currentTarget.getBoundingClientRect();
    const next = computeMetricMiniSparklineHoverState(
      props.series,
      event.clientX - rect.left,
      rect.width,
    );
    setHoveredState(
      next
        ? {
            ...next,
            tooltipX: event.clientX,
            tooltipY: rect.top,
          }
        : null,
    );
  };
  const handleMouseLeave = () => setHoveredState(null);

  return (
    <div
      class={`grid h-4 w-full min-w-0 ${rootColumns()} items-center gap-1 overflow-hidden`}
      title={props.title}
      data-testid="metric-mini-sparkline"
      data-value-label-mode={valueLabelMode()}
      data-rendered-series-count={paths().length}
    >
      <svg
        class="block h-4 w-full min-w-0 cursor-crosshair"
        viewBox="0 0 96 18"
        role="img"
        aria-label={ariaLabel()}
        preserveAspectRatio="none"
        onMouseMove={handleMouseMove}
        onMouseLeave={handleMouseLeave}
      >
        <line x1="1" y1="16" x2="95" y2="16" stroke="currentColor" stroke-opacity="0.16" />
        <Show when={hasLine()}>
          <For each={paths()}>
            {(series) => (
              <path
                d={series.path}
                fill="none"
                stroke={series.color}
                stroke-width="1.7"
                stroke-linecap="round"
                stroke-linejoin="round"
                vector-effect="non-scaling-stroke"
              />
            )}
          </For>
          <Show when={hoveredState()}>
            <line
              x1={cursorX()}
              y1="2"
              x2={cursorX()}
              y2="16"
              stroke="currentColor"
              stroke-opacity="0.36"
              stroke-width="1"
              vector-effect="non-scaling-stroke"
            />
          </Show>
        </Show>
      </svg>
      <Show when={showInlineValue()}>
        <span class="block max-w-[5.5rem] overflow-hidden text-ellipsis whitespace-nowrap text-right text-[10px] font-medium tabular-nums text-base-content">
          {displayLabel()}
        </span>
      </Show>
      <Show when={hoveredState()}>
        {(hover) => (
          <TooltipPortal
            when={true}
            x={hover().tooltipX}
            y={hover().tooltipY}
            maxWidth={220}
            align="center"
          >
            <div data-metric-mini-sparkline-tooltip="true" class="min-w-[126px] text-[10px]">
              <div class="mb-1 text-center font-medium text-base-content">
                {formatMetricMiniSparklineHoverTime(hover().timestamp)}
              </div>
              <For each={hover().entries}>
                {(entry) => (
                  <div class="flex items-center gap-1.5 leading-tight">
                    <svg class="h-2 w-2 shrink-0" viewBox="0 0 8 8" aria-hidden="true">
                      <circle cx="4" cy="4" r="4" fill={entry.color} />
                    </svg>
                    <span class="text-muted">{entry.label}</span>
                    <span class="ml-auto font-medium tabular-nums text-base-content">
                      {formatHoverValue(entry.value)}
                    </span>
                  </div>
                )}
              </For>
            </div>
          </TooltipPortal>
        )}
      </Show>
    </div>
  );
};
