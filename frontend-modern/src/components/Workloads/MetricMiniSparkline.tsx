import { For, Show, createMemo, type Component } from 'solid-js';

import {
  buildMetricMiniSparklinePath,
  getMetricMiniSparklineScale,
  hasRenderableMetricSeries,
  type WorkloadMetricSparklineSeries,
} from './workloadMetricHistoryModel';

interface MetricMiniSparklineProps {
  series: WorkloadMetricSparklineSeries[];
  valueLabel?: string;
  title?: string;
  unit?: string;
  emptyLabel?: string;
}

export const MetricMiniSparkline: Component<MetricMiniSparklineProps> = (props) => {
  const scale = createMemo(() => getMetricMiniSparklineScale(props.series, props.unit));
  const paths = createMemo(() =>
    props.series
      .map((series) => ({
        ...series,
        path: buildMetricMiniSparklinePath(series.points, scale()),
      }))
      .filter((series) => series.path.length > 0),
  );
  const hasLine = createMemo(() => hasRenderableMetricSeries(props.series));
  const displayLabel = createMemo(() => props.valueLabel || props.emptyLabel || '—');

  return (
    <div
      class="grid h-4 w-full min-w-0 grid-cols-[minmax(2.5rem,1fr)_auto] items-center gap-1 overflow-hidden"
      title={props.title}
      data-testid="metric-mini-sparkline"
      data-rendered-series-count={paths().length}
    >
      <svg
        class="block h-4 w-full min-w-0"
        viewBox="0 0 96 18"
        role="img"
        aria-label={props.title || 'Metric history'}
        preserveAspectRatio="none"
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
        </Show>
      </svg>
      <span class="block max-w-[5.5rem] overflow-hidden text-ellipsis whitespace-nowrap text-right text-[10px] font-medium tabular-nums text-base-content">
        {displayLabel()}
      </span>
    </div>
  );
};
