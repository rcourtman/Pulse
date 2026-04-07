import { Component, Show } from 'solid-js';
import { presentationPolicyHidesUpgradePrompts } from '@/stores/sessionPresentationPolicy';
import { UpgradeLink } from './UpgradeLink';
import type { HistoryChartState } from './useHistoryChartState';

interface HistoryChartOverlayProps {
  chart: HistoryChartState;
  hideLock?: boolean;
}

export const HistoryChartOverlay: Component<HistoryChartOverlayProps> = (props) => {
  return (
    <>
      <Show when={!props.chart.loading() && props.chart.data().length === 0 && !props.chart.error()}>
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

      <Show when={props.chart.loading()}>
        <div class="absolute inset-0 flex items-center justify-center bg-surface -[1px]">
          <div class="w-6 h-6 border-2 border-blue-500 border-t-transparent rounded-full animate-spin" />
        </div>
      </Show>

      <Show when={props.chart.error()}>
        <div class="absolute inset-0 flex items-center justify-center">
          <p class="text-sm text-red-500">{props.chart.error()}</p>
        </div>
      </Show>

      <Show when={props.chart.isLocked() && !props.hideLock}>
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
          <h3 class="text-lg font-bold text-base-content mb-1">{props.chart.lockDays()}-Day History</h3>
          <Show
            when={!presentationPolicyHidesUpgradePrompts()}
            fallback={
              <p class="text-sm text-muted text-center max-w-[220px] mb-4">
                Historical data beyond {props.chart.lockDays()} days is hidden in this demo.
              </p>
            }
          >
            <p class="text-sm text-muted text-center max-w-[200px] mb-4">
              Upgrade to {props.chart.lockTierLabel()} to unlock {props.chart.lockDays()} days of
              historical data retention.
            </p>
            <div class="flex flex-col items-center gap-2">
              <UpgradeLink
                destination={props.chart.getUpgradeActionDestination('long_term_metrics')}
                onClick={() => props.chart.trackUpgradeClicked('history_chart', 'long_term_metrics')}
                class="px-4 py-2 bg-indigo-600 hover:bg-indigo-700 text-white text-sm font-medium rounded-md shadow-sm transition-colors"
              >
                Unlock {props.chart.lockTierLabel()} Features
              </UpgradeLink>
              <Show when={props.chart.canStartTrial()}>
                <button
                  type="button"
                  class="text-xs font-semibold text-indigo-700 dark:text-indigo-300 hover:underline disabled:opacity-60"
                  disabled={props.chart.startingTrial()}
                  onClick={props.chart.handleStartTrial}
                >
                  Or start a free 14-day trial
                </button>
              </Show>
            </div>
          </Show>
        </div>
      </Show>
    </>
  );
};
