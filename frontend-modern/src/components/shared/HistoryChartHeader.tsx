import { Component, Show } from 'solid-js';
import { formatHistoryChartTooltipValue } from './historyChartModel';
import type { HistoryChartState } from './useHistoryChartState';

interface HistoryChartHeaderProps {
  chart: HistoryChartState;
  compact?: boolean;
  hideSelector?: boolean;
  label?: string;
  unit?: string;
}

export const HistoryChartHeader: Component<HistoryChartHeaderProps> = (props) => {
  return (
    <div class={`flex items-center justify-between ${props.compact ? 'mb-2' : 'mb-4'}`}>
      <div class="flex items-center gap-2">
        <span class="text-sm font-medium text-base-content">{props.label || 'History'}</span>
        <Show when={props.unit}>
          <span class="text-xs text-slate-400">({props.unit})</span>
        </Show>
        <Show when={props.chart.source() && props.chart.source() !== 'store'}>
          <span
            class={`text-[10px] font-semibold px-2 py-0.5 rounded-full uppercase tracking-wide ${
              props.chart.source() === 'live'
                ? 'bg-red-100 text-red-700 dark:bg-red-900 dark:text-red-300'
                : 'bg-amber-100 text-amber-700 dark:bg-amber-900 dark:text-amber-300'
            }`}
            title={
              props.chart.source() === 'live'
                ? 'Live sample shown because history is not available yet.'
                : 'In-memory buffer shown while history is warming up.'
            }
          >
            {props.chart.source() === 'live' ? 'Live' : 'Buffer'}
          </span>
        </Show>
      </div>

      <div class="flex items-center gap-3">
        <Show when={props.chart.dataMin() !== null && props.chart.dataMax() !== null}>
          <div class="flex items-center gap-2 text-[10px]">
            <span>
              <span class="text-muted">Min </span>
              <span class="text-blue-400">
                {formatHistoryChartTooltipValue(props.chart.dataMin()!, props.unit)}
              </span>
            </span>
            <span>
              <span class="text-muted">Max </span>
              <span class="text-red-400">
                {formatHistoryChartTooltipValue(props.chart.dataMax()!, props.unit)}
              </span>
            </span>
          </div>
        </Show>
        <Show when={!props.hideSelector}>
          <div class="flex bg-surface-hover rounded-md p-0.5">
            {props.chart.ranges.map((range) => (
              <button
                onClick={() => props.chart.updateRange(range)}
                class={`px-3 py-1 text-xs font-medium rounded-md transition-colors ${
                  props.chart.range() === range
                    ? 'bg-surface text-base-content shadow-sm'
                    : 'text-muted hover:text-base-content'
                }`}
              >
                {range}
              </button>
            ))}
          </div>
        </Show>
      </div>
    </div>
  );
};
