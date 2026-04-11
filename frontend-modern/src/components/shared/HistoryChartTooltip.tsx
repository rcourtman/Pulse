import { Component, Show } from 'solid-js';
import {
  formatHistoryChartTooltipValue,
  getHistoryChartTooltipLayout,
  type HistoryChartHoverPoint,
} from './historyChartModel';

interface HistoryChartTooltipProps {
  hoveredPoint: HistoryChartHoverPoint | null;
  chartWidth: number;
  chartHeight: number;
  unit?: string;
}

export const HistoryChartTooltip: Component<HistoryChartTooltipProps> = (props) => {
  return (
    <svg
      class="pointer-events-none absolute inset-0 h-full w-full overflow-visible"
      viewBox={`0 0 ${props.chartWidth} ${props.chartHeight}`}
      preserveAspectRatio="none"
      aria-hidden="true"
    >
      <Show when={props.hoveredPoint}>
        {(point) => {
          const layout = () =>
            getHistoryChartTooltipLayout({
              hoveredPoint: point(),
              chartWidth: props.chartWidth,
              chartHeight: props.chartHeight,
            });
          return (
            <foreignObject
              x={layout().x}
              y={layout().y}
              width={layout().width}
              height={layout().height}
              overflow="visible"
            >
              <div
                data-history-chart-tooltip="true"
                class="h-full w-full rounded border border-slate-600 bg-slate-900 px-2 py-1 text-xs text-slate-50 shadow-lg"
              >
                <div class="mb-0.5 text-center font-medium">
                  {new Date(point().timestamp).toLocaleString()}
                </div>
                <div class="text-slate-300">
                  {formatHistoryChartTooltipValue(point().value, props.unit)}
                </div>
              </div>
            </foreignObject>
          );
        }}
      </Show>
    </svg>
  );
};
