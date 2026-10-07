import { For, Show, createMemo, createSignal, type Component, type JSX } from 'solid-js';
import { TooltipPortal } from '@/components/shared/TooltipPortal';
import { formatCompactSpeed } from '@/utils/format';

import {
  buildMetricMiniSparklinePath,
  computeMetricMiniSparklineHoverState,
  formatMetricMiniSparklineHoverTime,
  getMetricMiniSparklineScale,
  getMetricMiniSparklineTimeRange,
  hasRenderableMetricSeries,
  WORKLOAD_RATE_METRIC_GLYPHS,
  type MetricMiniSparklineHoverState,
  type WorkloadMetricSparklineSeries,
  type WorkloadRateMetric,
} from './workloadMetricHistoryModel';

type MetricMiniSparklineValueLabelMode = 'inline' | 'tooltip' | 'hidden';
export type MetricMiniSparklineValueLabelContext = 'current' | 'last known';

interface MetricMiniSparklineProps {
  series: WorkloadMetricSparklineSeries[];
  valueLabel?: string;
  /** Visible inline value when it should read shorter than the accessible valueLabel. */
  valueContent?: JSX.Element;
  valueLabelMode?: MetricMiniSparklineValueLabelMode;
  valueLabelContext?: MetricMiniSparklineValueLabelContext;
  title?: string;
  unit?: string;
  emptyLabel?: string;
  formatValue?: (value: number) => string;
  cursorRatio?: number | null;
  onCursorRatioChange?: (ratio: number | null) => void;
  showTooltip?: boolean;
}

interface MetricMiniSparklinePointer {
  cursorRatio: number;
  tooltipX: number;
  tooltipY: number;
}

type MetricMiniSparklineTooltipState = MetricMiniSparklineHoverState & MetricMiniSparklinePointer;

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
  // Keep only where the pointer is. The tooltip's point and values derive from
  // the current series, so a live update under an open tooltip re-reads it
  // instead of showing the snapshot taken at the last mouse move.
  const [pointer, setPointer] = createSignal<MetricMiniSparklinePointer | null>(null);
  const hoveredState = createMemo<MetricMiniSparklineTooltipState | null>(() => {
    const current = pointer();
    if (!current) return null;
    const state = computeMetricMiniSparklineHoverState(props.series, current.cursorRatio, 1);
    return state ? { ...state, tooltipX: current.tooltipX, tooltipY: current.tooltipY } : null;
  });
  const synchronizedState = createMemo(() => {
    const ratio = props.cursorRatio;
    if (ratio === null || ratio === undefined) return null;
    return computeMetricMiniSparklineHoverState(props.series, ratio, 1);
  });
  const activeHoverState = createMemo(() => synchronizedState() ?? hoveredState());
  const rootColumns = createMemo(() =>
    // Narrow workload columns cannot spare a 40px plot plus its value.
    // Keep the number readable and let the trend line use the remaining width.
    showInlineValue() ? 'grid-cols-[minmax(0,1fr)_auto]' : 'grid-cols-[minmax(0,1fr)]',
  );
  const ariaLabel = createMemo(() => {
    const title = props.title || 'Metric history';
    const value = props.valueLabel
      ? `, ${props.valueLabelContext ?? 'current'} ${props.valueLabel}`
      : '';
    return `${title}${value}`;
  });
  const formatHoverValue = (value: number) =>
    props.formatValue?.(value) ?? formatDefaultHoverValue(value, props.unit);
  const cursorX = createMemo(() =>
    activeHoverState()
      ? SPARKLINE_PLOT_X_PADDING + activeHoverState()!.cursorRatio * SPARKLINE_PLOT_WIDTH
      : 0,
  );
  const handleMouseMove: JSX.EventHandler<SVGSVGElement, MouseEvent> = (event) => {
    if (!hasLine()) {
      setPointer(null);
      props.onCursorRatioChange?.(null);
      return;
    }

    const rect = event.currentTarget.getBoundingClientRect();
    const next = computeMetricMiniSparklineHoverState(
      props.series,
      event.clientX - rect.left,
      rect.width,
    );
    setPointer(
      next
        ? {
            cursorRatio: next.cursorRatio,
            tooltipX: event.clientX,
            tooltipY: rect.top,
          }
        : null,
    );
    props.onCursorRatioChange?.(next?.cursorRatio ?? null);
  };
  const handleMouseLeave = () => {
    setPointer(null);
    props.onCursorRatioChange?.(null);
  };
  const showTooltip = createMemo(() => (props.showTooltip ?? true) && hoveredState());

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
          <Show when={activeHoverState()}>
            <line
              data-metric-history-cursor="true"
              x1={cursorX()}
              y1="2"
              x2={cursorX()}
              y2="16"
              stroke="currentColor"
              stroke-opacity="0.46"
              stroke-width="1"
              vector-effect="non-scaling-stroke"
            />
          </Show>
        </Show>
      </svg>
      <Show when={showInlineValue()}>
        <span class="block max-w-22 overflow-hidden text-ellipsis whitespace-nowrap text-right text-[10px] font-medium tabular-nums text-base-content">
          {props.valueContent ?? displayLabel()}
        </span>
      </Show>
      <Show when={showTooltip()}>
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

interface MetricMiniSparklineRatePairProps {
  metric: WorkloadRateMetric;
  values: readonly [number, number];
}

/**
 * Current in/out (or read/write) rates compact enough to sit beside a rate
 * sparkline. The full rates belong in the chart's valueLabel, which carries
 * them to assistive technology, so this abbreviation stays out of that tree.
 */
export const MetricMiniSparklineRatePair: Component<MetricMiniSparklineRatePairProps> = (props) => (
  <span aria-hidden="true" data-metric-rate-pair="true" class="inline-flex items-center gap-1">
    <For each={WORKLOAD_RATE_METRIC_GLYPHS[props.metric]}>
      {(entry, index) => (
        <span class="inline-flex items-center gap-px">
          <span class={entry.mono ? 'font-mono' : undefined} style={{ color: entry.color }}>
            {entry.glyph}
          </span>
          {formatCompactSpeed(props.values[index()])}
        </span>
      )}
    </For>
  </span>
);
