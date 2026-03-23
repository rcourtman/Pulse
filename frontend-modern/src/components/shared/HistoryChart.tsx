import { Component, Show } from 'solid-js';
import { Portal } from 'solid-js/web';
import { formatHistoryChartTooltipValue, type HistoryChartProps } from './historyChartModel';
import { useHistoryChartState } from './useHistoryChartState';

export type { HistoryChartProps } from './historyChartModel';

export const HistoryChart: Component<HistoryChartProps> = (props) => {
  let canvasRef: HTMLCanvasElement | undefined;
  let containerRef: HTMLDivElement | undefined;

  const chart = useHistoryChartState(props, {
    getCanvas: () => canvasRef,
    getContainer: () => containerRef,
  });

  return (
    <div
      class={`flex flex-col h-full ${props.compact ? '' : 'bg-surface rounded-md shadow-sm border border-border p-4'}`}
    >
      <div class={`flex items-center justify-between ${props.compact ? 'mb-2' : 'mb-4'}`}>
        <div class="flex items-center gap-2">
          <span class="text-sm font-medium text-base-content">{props.label || 'History'}</span>
          <Show when={props.unit}>
            <span class="text-xs text-slate-400">({props.unit})</span>
          </Show>
          <Show when={chart.source() && chart.source() !== 'store'}>
            <span
              class={`text-[10px] font-semibold px-2 py-0.5 rounded-full uppercase tracking-wide ${
                chart.source() === 'live'
                  ? 'bg-red-100 text-red-700 dark:bg-red-900 dark:text-red-300'
                  : 'bg-amber-100 text-amber-700 dark:bg-amber-900 dark:text-amber-300'
              }`}
              title={
                chart.source() === 'live'
                  ? 'Live sample shown because history is not available yet.'
                  : 'In-memory buffer shown while history is warming up.'
              }
            >
              {chart.source() === 'live' ? 'Live' : 'Buffer'}
            </span>
          </Show>
        </div>

        <div class="flex items-center gap-3">
          <Show when={chart.dataMin() !== null && chart.dataMax() !== null}>
            <div class="flex items-center gap-2 text-[10px]">
              <span>
                <span class="text-muted">Min </span>
                <span class="text-blue-400">
                  {formatHistoryChartTooltipValue(chart.dataMin()!, props.unit)}
                </span>
              </span>
              <span>
                <span class="text-muted">Max </span>
                <span class="text-red-400">
                  {formatHistoryChartTooltipValue(chart.dataMax()!, props.unit)}
                </span>
              </span>
            </div>
          </Show>
          <Show when={!props.hideSelector}>
            <div class="flex bg-surface-hover rounded-md p-0.5">
              {chart.ranges.map((range) => (
                <button
                  onClick={() => chart.updateRange(range)}
                  class={`px-3 py-1 text-xs font-medium rounded-md transition-colors ${
                    chart.range() === range
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

      <div
        class={`relative flex-1 w-full ${props.compact ? 'min-h-[120px]' : 'min-h-[200px]'}`}
        ref={containerRef}
      >
        <canvas
          ref={canvasRef}
          class="block w-full h-full cursor-crosshair"
          onMouseMove={chart.handleMouseMove}
          onMouseLeave={chart.handleMouseLeave}
        />

        <Show when={!chart.loading() && chart.data().length === 0 && !chart.error()}>
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
              <p class="text-sm text-slate-500">Collecting data... History will appear here.</p>
            </div>
          </div>
        </Show>

        <Show when={chart.loading()}>
          <div class="absolute inset-0 flex items-center justify-center bg-surface -[1px]">
            <div class="w-6 h-6 border-2 border-blue-500 border-t-transparent rounded-full animate-spin" />
          </div>
        </Show>

        <Show when={chart.error()}>
          <div class="absolute inset-0 flex items-center justify-center">
            <p class="text-sm text-red-500">{chart.error()}</p>
          </div>
        </Show>

        <Show when={chart.isLocked() && !props.hideLock}>
          <div class="absolute inset-0 z-10 flex flex-col items-center justify-center bg-surface rounded-md">
            <div class="bg-indigo-500 rounded-full p-3 shadow-sm mb-3">
              <svg
                xmlns="http://www.w3.org/2000/svg"
                width="24"
                height="24"
                viewBox="0 0 24 24"
                fill="none"
                stroke="white"
                stroke-width="2"
                stroke-linecap="round"
                stroke-linejoin="round"
              >
                <rect x="3" y="11" width="18" height="11" rx="2" ry="2"></rect>
                <path d="M7 11V7a5 5 0 0 1 10 0v4"></path>
              </svg>
            </div>
            <h3 class="text-lg font-bold text-base-content mb-1">{chart.lockDays()}-Day History</h3>
            <p class="text-sm text-muted text-center max-w-[200px] mb-4">
              Upgrade to {chart.lockTierLabel()} to unlock {chart.lockDays()} days of historical
              data retention.
            </p>
            <div class="flex flex-col items-center gap-2">
              <a
                href={chart.getUpgradeActionUrlOrFallback('long_term_metrics')}
                target="_blank"
                rel="noopener noreferrer"
                onClick={() => chart.trackUpgradeClicked('history_chart', 'long_term_metrics')}
                class="px-4 py-2 bg-indigo-600 hover:bg-indigo-700 text-white text-sm font-medium rounded-md shadow-sm transition-colors"
              >
                Unlock {chart.lockTierLabel()} Features
              </a>
              <Show when={chart.canStartTrial()}>
                <button
                  type="button"
                  class="text-xs font-semibold text-indigo-700 dark:text-indigo-300 hover:underline disabled:opacity-60"
                  disabled={chart.startingTrial()}
                  onClick={chart.handleStartTrial}
                >
                  Or start a free 14-day trial
                </button>
              </Show>
            </div>
          </div>
        </Show>
      </div>

      <Portal>
        <Show when={chart.hoveredPoint()}>
          {(point) => (
            <div
              class="fixed pointer-events-none text-xs rounded px-2 py-1 shadow-lg border border-slate-600 z-[9999]"
              style={{
                left: `${point().x}px`,
                top: `${point().y}px`,
                transform: 'translateX(-50%)',
                'background-color': 'rgb(15, 23, 42)',
                color: 'rgb(248, 250, 252)',
              }}
            >
              <div class="font-medium text-center mb-0.5">
                {new Date(point().timestamp).toLocaleString()}
              </div>
              <div style={{ color: 'rgb(203, 213, 225)' }}>
                {formatHistoryChartTooltipValue(point().value, props.unit)}
              </div>
            </div>
          )}
        </Show>
      </Portal>
    </div>
  );
};
