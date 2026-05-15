import { For, Show, createSignal, type Component } from 'solid-js';

import type { HistoryTimeRange } from '@/api/charts';
import { HistoryChart } from '@/components/shared/HistoryChart';
import { HISTORY_CHART_RANGES } from '@/components/shared/historyChartModel';
import { filterSelectClass } from '@/components/shared/FilterToolbar';

import {
  GUEST_DRAWER_HISTORY_CHARTS,
  GUEST_DRAWER_HISTORY_DEFAULT_RANGE,
  type GuestDrawerHistoryTarget,
} from './guestDrawerModel';

interface GuestDrawerHistoryProps {
  target: GuestDrawerHistoryTarget | null;
}

const formatRangeLabel = (range: HistoryTimeRange): string => {
  switch (range) {
    case '24h':
      return '24 hours';
    case '7d':
      return '7 days';
    case '14d':
      return '14 days';
    case '30d':
      return '30 days';
    case '90d':
      return '90 days';
    default:
      return range;
  }
};

export const GuestDrawerHistory: Component<GuestDrawerHistoryProps> = (props) => {
  const [range, setRange] = createSignal<HistoryTimeRange>(GUEST_DRAWER_HISTORY_DEFAULT_RANGE);

  return (
    <Show
      when={props.target}
      fallback={<div class="py-6 text-center text-sm text-muted">History unavailable</div>}
    >
      {(target) => (
        <div class="space-y-3">
          <div class="flex items-center justify-end">
            <label class="sr-only" for="guest-history-range">
              History range
            </label>
            <select
              id="guest-history-range"
              class={filterSelectClass}
              value={range()}
              onChange={(event) => setRange(event.currentTarget.value as HistoryTimeRange)}
            >
              <For each={HISTORY_CHART_RANGES}>
                {(option) => <option value={option}>{formatRangeLabel(option)}</option>}
              </For>
            </select>
          </div>
          <div class="grid gap-3 xl:grid-cols-2">
            <For each={GUEST_DRAWER_HISTORY_CHARTS}>
              {(chart) => (
                <div class="min-h-[180px] rounded-sm border border-border bg-surface p-3">
                  <HistoryChart
                    resourceType={target().resourceType}
                    resourceId={target().resourceId}
                    metric={chart.metric}
                    label={chart.label}
                    unit={chart.unit}
                    color={chart.color}
                    range={range()}
                    onRangeChange={setRange}
                    compact
                    hideSelector
                  />
                </div>
              )}
            </For>
          </div>
        </div>
      )}
    </Show>
  );
};
