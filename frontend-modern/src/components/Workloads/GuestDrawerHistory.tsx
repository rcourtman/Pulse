import { For, Show, createMemo, createSignal, onMount, type Component } from 'solid-js';

import {
  ChartsAPI,
  type AggregatedMetricPoint,
  type AllMetricsHistoryResponse,
  type HistoryTimeRange,
  type ResourceType,
  type SingleMetricHistoryResponse,
} from '@/api/charts';
import { filterSelectClass } from '@/components/shared/FilterToolbar';
import { HISTORY_CHART_RANGES } from '@/components/shared/historyChartModel';
import { createNonSuspendingQuery } from '@/hooks/createNonSuspendingQuery';
import { isRangeLocked, loadRuntimeCapabilities, maxHistoryDays } from '@/stores/license';

import {
  GUEST_DRAWER_HISTORY_DEFAULT_RANGE,
  GUEST_DRAWER_HISTORY_GROUPS,
  buildGuestDrawerHistoryPath,
  getGuestDrawerHistoryRangeBounds,
  getGuestDrawerHistoryScale,
  getGuestDrawerHistoryValueLabel,
  normalizeGuestDrawerHistoryPoints,
  type GuestDrawerHistoryGroupConfig,
  type GuestDrawerHistoryTarget,
} from './guestDrawerModel';

interface GuestDrawerHistoryProps {
  target: GuestDrawerHistoryTarget | null;
}

interface GuestDrawerHistoryQueryKey {
  resourceType: ResourceType;
  resourceId: string;
  range: HistoryTimeRange;
}

interface GuestDrawerHistoryGroupChartProps {
  group: GuestDrawerHistoryGroupConfig;
  loading: boolean;
  metrics: Record<string, AggregatedMetricPoint[] | undefined>;
}

const GUEST_DRAWER_HISTORY_MAX_POINTS = 240;
const GUEST_DRAWER_HISTORY_POLL_MS = 30_000;
const GUEST_DRAWER_HISTORY_CHART_WIDTH = 360;
const GUEST_DRAWER_HISTORY_CHART_HEIGHT = 92;

const EMPTY_HISTORY_RESPONSE: AllMetricsHistoryResponse = {
  resourceType: '',
  resourceId: '',
  range: GUEST_DRAWER_HISTORY_DEFAULT_RANGE,
  start: 0,
  end: 0,
  metrics: {},
  source: 'store',
};

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

const normalizeHistoryResponse = (
  result: SingleMetricHistoryResponse | AllMetricsHistoryResponse,
): AllMetricsHistoryResponse => {
  if ('metrics' in result) return result;
  return {
    resourceType: result.resourceType,
    resourceId: result.resourceId,
    range: result.range,
    start: result.start,
    end: result.end,
    metrics: { [result.metric]: result.points ?? [] },
    source: result.source,
  };
};

const getAxisLabel = (unit: string, pct: number): string => {
  if (unit === '%') {
    if (pct === 0) return '100%';
    if (pct === 0.5) return '50%';
    return '0%';
  }
  if (pct === 0) return 'Max';
  if (pct === 0.5) return 'Avg';
  return '0';
};

const GuestDrawerHistoryGroupChart: Component<GuestDrawerHistoryGroupChartProps> = (props) => {
  const series = createMemo(() =>
    props.group.series.map((config) => ({
      ...config,
      points: normalizeGuestDrawerHistoryPoints(props.metrics[config.metric], config.unit),
    })),
  );
  const drawableSeries = createMemo(() => series().filter((item) => item.points.length >= 2));
  const scale = createMemo(() => getGuestDrawerHistoryScale(series(), props.group.unit));
  const bounds = createMemo(() => getGuestDrawerHistoryRangeBounds(series()));
  const hasDrawableData = createMemo(() => drawableSeries().length > 0 && bounds() !== null);

  return (
    <section
      class="min-h-[154px] rounded-sm border border-border bg-surface p-2.5"
      data-testid="guest-history-group-chart"
      data-history-group={props.group.id}
    >
      <div class="mb-2 flex flex-wrap items-start justify-between gap-x-4 gap-y-1">
        <h4 class="text-xs font-semibold uppercase tracking-wide text-muted">
          {props.group.label}
        </h4>
        <div class="flex flex-wrap justify-end gap-x-3 gap-y-1 text-[11px] text-muted">
          <For each={series()}>
            {(item) => (
              <span class="inline-flex items-center gap-1">
                <svg aria-hidden="true" class="h-2.5 w-2.5 shrink-0" viewBox="0 0 10 10">
                  <circle cx="5" cy="5" r="4" fill={item.color} />
                </svg>
                <span class="font-medium text-base-content">{item.label}</span>
                <span>{getGuestDrawerHistoryValueLabel(item.points, item.unit)}</span>
              </span>
            )}
          </For>
        </div>
      </div>

      <div class="relative h-24">
        <For each={[0, 0.5, 1]}>
          {(tick) => (
            <span
              class={`pointer-events-none absolute left-0 w-8 text-right text-[10px] text-muted ${
                tick === 0 ? 'top-1' : tick === 0.5 ? 'top-1/2 -translate-y-1/2' : 'bottom-3'
              }`}
            >
              {getAxisLabel(props.group.unit, tick)}
            </span>
          )}
        </For>
        <svg
          aria-label={`${props.group.label} history`}
          class="absolute inset-0 h-full w-full"
          preserveAspectRatio="none"
          role="img"
          viewBox={`0 0 ${GUEST_DRAWER_HISTORY_CHART_WIDTH} ${GUEST_DRAWER_HISTORY_CHART_HEIGHT}`}
        >
          <For each={[8, 41, 74]}>
            {(y) => (
              <line
                x1="34"
                x2="352"
                y1={y}
                y2={y}
                stroke="currentColor"
                stroke-width="1"
                class="text-border-subtle"
                vector-effect="non-scaling-stroke"
              />
            )}
          </For>
          <Show when={bounds()}>
            {(rangeBounds) => (
              <For each={drawableSeries()}>
                {(item) => (
                  <path
                    d={buildGuestDrawerHistoryPath(
                      item.points,
                      scale(),
                      rangeBounds().startTime,
                      rangeBounds().endTime,
                      GUEST_DRAWER_HISTORY_CHART_WIDTH,
                      GUEST_DRAWER_HISTORY_CHART_HEIGHT,
                    )}
                    fill="none"
                    stroke={item.color}
                    stroke-linecap="round"
                    stroke-linejoin="round"
                    stroke-width="2"
                    vector-effect="non-scaling-stroke"
                  />
                )}
              </For>
            )}
          </Show>
        </svg>
        <Show when={!hasDrawableData() && !props.loading}>
          <div class="absolute inset-x-8 inset-y-2 flex items-center justify-center rounded-sm bg-surface/80 text-xs text-muted">
            Collecting history
          </div>
        </Show>
        <Show when={props.loading}>
          <div class="absolute inset-x-8 inset-y-2 flex items-center justify-center rounded-sm bg-surface/80 text-xs text-muted">
            Loading history
          </div>
        </Show>
      </div>
    </section>
  );
};

export const GuestDrawerHistory: Component<GuestDrawerHistoryProps> = (props) => {
  const [range, setRange] = createSignal<HistoryTimeRange>(GUEST_DRAWER_HISTORY_DEFAULT_RANGE);

  onMount(() => {
    void loadRuntimeCapabilities();
  });

  const locked = createMemo(() => isRangeLocked(range()));
  const historyQuery = createNonSuspendingQuery<
    AllMetricsHistoryResponse,
    GuestDrawerHistoryQueryKey
  >({
    source: () => {
      const target = props.target;
      if (!target || locked()) return null;
      return {
        resourceType: target.resourceType,
        resourceId: target.resourceId,
        range: range(),
      };
    },
    fetcher: async (key) =>
      normalizeHistoryResponse(
        await ChartsAPI.getMetricsHistory({
          resourceType: key.resourceType,
          resourceId: key.resourceId,
          range: key.range,
          maxPoints: GUEST_DRAWER_HISTORY_MAX_POINTS,
        }),
      ),
    initialValue: EMPTY_HISTORY_RESPONSE,
    cacheKey: (key) => `guest-drawer-history:${key.resourceType}:${key.resourceId}:${key.range}`,
    pollMs: GUEST_DRAWER_HISTORY_POLL_MS,
  });

  const metrics = createMemo(() => historyQuery.value().metrics ?? {});
  const errorText = createMemo(() => (historyQuery.error() ? 'Failed to load history data' : ''));

  return (
    <Show
      when={props.target}
      fallback={<div class="py-6 text-center text-sm text-muted">History unavailable</div>}
    >
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

        <Show
          when={!locked()}
          fallback={
            <div class="rounded-sm border border-border bg-surface p-5 text-sm text-muted">
              {formatRangeLabel(range())} history is not enabled on this instance. Current runtime
              allows {maxHistoryDays()} days.
            </div>
          }
        >
          <Show
            when={!errorText()}
            fallback={
              <div class="rounded-sm border border-red-500/30 bg-red-500/10 p-5 text-sm text-red-300">
                {errorText()}
              </div>
            }
          >
            <div class="grid gap-3 xl:grid-cols-3">
              <For each={GUEST_DRAWER_HISTORY_GROUPS}>
                {(group) => (
                  <GuestDrawerHistoryGroupChart
                    group={group}
                    loading={historyQuery.loading() && !historyQuery.resolvedOnce()}
                    metrics={metrics()}
                  />
                )}
              </For>
            </div>
          </Show>
        </Show>
      </div>
    </Show>
  );
};
