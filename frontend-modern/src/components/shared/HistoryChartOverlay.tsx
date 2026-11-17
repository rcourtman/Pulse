import { Component, Show } from 'solid-js';
import { LoadingSpinner } from './LoadingSpinner';
import type { HistoryChartState } from './useHistoryChartState';

interface HistoryChartOverlayProps {
  chart: HistoryChartState;
}

export const HistoryChartOverlay: Component<HistoryChartOverlayProps> = (props) => {
  return (
    <>
      <Show
        when={!props.chart.loading() && props.chart.data().length === 0 && !props.chart.error()}
      >
        <div class="absolute inset-0 flex items-center justify-center bg-surface">
          <div class="text-center">
            <div class="text-slate-400 mb-2">
              <svg
                xmlns="http://www.w3.org/2000/svg"
                width="32"
                height="32"
                viewBox="0 0 24 24"
                fill="none"
                stroke="currentColor"
                stroke-width="2"
                stroke-linecap="round"
                stroke-linejoin="round"
                class="mx-auto"
              >
                <path d="M21 12a9 9 0 0 0-9-9 9.75 9.75 0 0 0-6.74 2.74L3 8" />
                <path d="M3 3v5h5" />
                <path d="M3 12a9 9 0 0 0 9 9 9.75 9.75 0 0 0 6.74-2.74L21 16" />
                <path d="M16 16l5 5" />
                <path d="M21 21v-5h-5" />
              </svg>
            </div>
            <p class="text-sm text-slate-500">No history samples in this time range.</p>
          </div>
        </div>
      </Show>

      <Show when={props.chart.loading()}>
        <div class="absolute inset-0 flex items-center justify-center bg-surface -[1px]">
          <LoadingSpinner size="xl" tone="info" label="Loading history" />
        </div>
      </Show>

      <Show when={props.chart.error()}>
        <div class="absolute inset-0 flex items-center justify-center">
          <p role="alert" class="px-3 text-center text-sm text-red-500">
            {props.chart.error()}
          </p>
        </div>
      </Show>
    </>
  );
};
