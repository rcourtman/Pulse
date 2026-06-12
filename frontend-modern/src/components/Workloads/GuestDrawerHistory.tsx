import { For, Show, createMemo, createSignal, onMount, type Component } from 'solid-js';

import {
  ChartsAPI,
  type AggregatedMetricPoint,
  type AllMetricsHistoryResponse,
  type HistoryTimeRange,
  type ResourceType,
  type SingleMetricHistoryResponse,
} from '@/api/charts';
import { FormSelect } from '@/components/shared/FormSelect';
import { filterSelectClass } from '@/components/shared/FilterToolbar';
import {
  HISTORY_CHART_RANGES,
  formatHistoryChartTimeLabel,
} from '@/components/shared/historyChartModel';
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
  fallbackMetrics?: Record<string, number | null | undefined>;
  groups?: GuestDrawerHistoryGroupConfig[];
  range: HistoryTimeRange;
  target: GuestDrawerHistoryTarget | null;
}

interface GuestDrawerHistoryRangeSelectProps {
  onRangeChange: (range: HistoryTimeRange) => void;
  range: HistoryTimeRange;
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
  range: HistoryTimeRange;
}

const GUEST_DRAWER_HISTORY_MAX_POINTS = 240;
const GUEST_DRAWER_HISTORY_POLL_MS = 30_000;
const GUEST_DRAWER_HISTORY_CHART_WIDTH = 360;
const GUEST_DRAWER_HISTORY_CHART_HEIGHT = 92;
const GUEST_DRAWER_HISTORY_PLOT_LEFT = 34;
const GUEST_DRAWER_HISTORY_PLOT_RIGHT = 8;
const GUEST_DRAWER_HISTORY_PLOT_TOP = 8;
const GUEST_DRAWER_HISTORY_PLOT_BOTTOM = 18;

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
    case '1h':
      return '1 hour';
    case '6h':
      return '6 hours';
    case '12h':
      return '12 hours';
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

export const GuestDrawerHistoryRangeSelect: Component<GuestDrawerHistoryRangeSelectProps> = (
  props,
) => (
  <FormSelect
    id="guest-history-range"
    label="History range"
    labelClass="sr-only"
    fieldBaseClass="contents"
    selectBaseClass={`${filterSelectClass} h-7 py-0 text-[11px]`}
    data-testid="guest-history-range-control"
    value={props.range}
    onChange={(event) => props.onRangeChange(event.currentTarget.value as HistoryTimeRange)}
  >
    <For each={HISTORY_CHART_RANGES}>
      {(option) => <option value={option}>{formatRangeLabel(option)}</option>}
    </For>
  </FormSelect>
);

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
  if (unit === 'C') {
    if (pct === 0) return 'Max';
    if (pct === 0.5) return 'Mid';
    return 'Min';
  }
  if (pct === 0) return 'Max';
  if (pct === 0.5) return 'Avg';
  return '0';
};

const clampNumber = (value: number, min: number, max: number): number =>
  Math.min(Math.max(value, min), max);

const getGuestDrawerHistoryX = (timestamp: number, startTime: number, endTime: number): number => {
  const plotWidth =
    GUEST_DRAWER_HISTORY_CHART_WIDTH -
    GUEST_DRAWER_HISTORY_PLOT_LEFT -
    GUEST_DRAWER_HISTORY_PLOT_RIGHT;
  const timeSpan = Math.max(1, endTime - startTime);
  return GUEST_DRAWER_HISTORY_PLOT_LEFT + ((timestamp - startTime) / timeSpan) * plotWidth;
};

const getGuestDrawerHistoryY = (
  value: number,
  scale: { minValue: number; maxValue: number },
): number => {
  const plotHeight =
    GUEST_DRAWER_HISTORY_CHART_HEIGHT -
    GUEST_DRAWER_HISTORY_PLOT_TOP -
    GUEST_DRAWER_HISTORY_PLOT_BOTTOM;
  const valueSpan = Math.max(1, scale.maxValue - scale.minValue);
  const bounded = clampNumber(value, scale.minValue, scale.maxValue);
  return GUEST_DRAWER_HISTORY_PLOT_TOP + (1 - (bounded - scale.minValue) / valueSpan) * plotHeight;
};

const findClosestGuestDrawerHistoryPoint = (
  points: readonly AggregatedMetricPoint[],
  timestamp: number,
): AggregatedMetricPoint | null => {
  if (points.length === 0) return null;

  let closest = points[0];
  let closestDistance = Math.abs(points[0].timestamp - timestamp);
  for (const point of points.slice(1)) {
    const distance = Math.abs(point.timestamp - timestamp);
    if (distance < closestDistance) {
      closest = point;
      closestDistance = distance;
    }
  }
  return closest;
};

const buildFallbackHistoryPoints = (value: number): AggregatedMetricPoint[] => {
  if (!Number.isFinite(value)) return [];
  const end = Date.now();
  return [
    { timestamp: end - 60_000, value, min: value, max: value },
    { timestamp: end, value, min: value, max: value },
  ];
};

const mergeFallbackHistoryMetrics = (
  metrics: Record<string, AggregatedMetricPoint[] | undefined>,
  fallbackMetrics: Record<string, number | null | undefined> | undefined,
): Record<string, AggregatedMetricPoint[] | undefined> => {
  if (!fallbackMetrics) return metrics;

  let next: Record<string, AggregatedMetricPoint[] | undefined> | null = null;
  for (const [metric, value] of Object.entries(fallbackMetrics)) {
    if (typeof value !== 'number' || !Number.isFinite(value)) continue;
    const existing = metrics[metric] ?? [];
    if (existing.length >= 2) continue;
    next ??= { ...metrics };
    next[metric] = buildFallbackHistoryPoints(value);
  }
  return next ?? metrics;
};

const GuestDrawerHistoryGroupChart: Component<GuestDrawerHistoryGroupChartProps> = (props) => {
  const [hoverTimestamp, setHoverTimestamp] = createSignal<number | null>(null);
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
  const hoveredSeries = createMemo(() => {
    const timestamp = hoverTimestamp();
    const rangeBounds = bounds();
    if (timestamp === null || !rangeBounds) return [];

    return drawableSeries()
      .map((item) => {
        const point = findClosestGuestDrawerHistoryPoint(item.points, timestamp);
        if (!point) return null;
        return {
          ...item,
          point,
          x: getGuestDrawerHistoryX(point.timestamp, rangeBounds.startTime, rangeBounds.endTime),
          y: getGuestDrawerHistoryY(point.value, scale()),
        };
      })
      .filter((item): item is NonNullable<typeof item> => item !== null);
  });
  const hoverX = createMemo(() => hoveredSeries()[0]?.x ?? null);
  const hoveredByMetric = createMemo(() => {
    const byMetric = new Map<string, ReturnType<typeof hoveredSeries>[number]>();
    for (const item of hoveredSeries()) {
      byMetric.set(item.metric, item);
    }
    return byMetric;
  });
  const displaySeries = createMemo(() =>
    series().map((item) => {
      const hovered = hoveredByMetric().get(item.metric);
      return {
        ...item,
        displayPoints: hovered ? [hovered.point] : item.points,
      };
    }),
  );
  const hoverTimeLabel = createMemo(() => {
    const point = hoveredSeries()[0]?.point;
    return point ? formatHistoryChartTimeLabel(point.timestamp, props.range) : '';
  });

  const handleHoverMove = (event: MouseEvent & { currentTarget: SVGSVGElement }) => {
    const rangeBounds = bounds();
    if (!rangeBounds) return;

    const rect = event.currentTarget.getBoundingClientRect();
    if (rect.width <= 0) return;

    const pointerX = ((event.clientX - rect.left) / rect.width) * GUEST_DRAWER_HISTORY_CHART_WIDTH;
    const plotRight = GUEST_DRAWER_HISTORY_CHART_WIDTH - GUEST_DRAWER_HISTORY_PLOT_RIGHT;
    const clampedX = clampNumber(pointerX, GUEST_DRAWER_HISTORY_PLOT_LEFT, plotRight);
    const plotWidth = plotRight - GUEST_DRAWER_HISTORY_PLOT_LEFT;
    const ratio = (clampedX - GUEST_DRAWER_HISTORY_PLOT_LEFT) / Math.max(1, plotWidth);
    setHoverTimestamp(
      rangeBounds.startTime + ratio * (rangeBounds.endTime - rangeBounds.startTime),
    );
  };

  return (
    <section
      class="flex min-h-[154px] flex-col rounded-sm border border-border bg-surface p-2.5"
      data-testid="guest-history-group-chart"
      data-history-group={props.group.id}
    >
      <div class="mb-2 flex flex-wrap items-start justify-between gap-x-4 gap-y-1">
        <div class="inline-flex items-baseline gap-2">
          <h4 class="text-xs font-semibold uppercase tracking-wide text-muted">
            {props.group.label}
          </h4>
          <Show when={hoverTimeLabel()}>
            {(label) => (
              <span
                class="text-[10px] font-semibold tabular-nums text-base-content"
                data-testid="guest-history-hover-time"
              >
                {label()}
              </span>
            )}
          </Show>
        </div>
        <div class="flex flex-wrap justify-end gap-x-3 gap-y-1 text-[11px] text-muted">
          <For each={displaySeries()}>
            {(item) => (
              <span class="inline-flex items-center gap-1">
                <svg aria-hidden="true" class="h-2.5 w-2.5 shrink-0" viewBox="0 0 10 10">
                  <circle cx="5" cy="5" r="4" fill={item.color} />
                </svg>
                <span class="font-medium text-base-content">{item.label}</span>
                <span>{getGuestDrawerHistoryValueLabel(item.displayPoints, item.unit)}</span>
              </span>
            )}
          </For>
        </div>
      </div>

      <div class="relative min-h-24 flex-1">
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
          class="absolute inset-0 h-full w-full cursor-crosshair"
          data-testid="guest-history-plot"
          onMouseMove={handleHoverMove}
          onPointerLeave={() => setHoverTimestamp(null)}
          onPointerMove={handleHoverMove}
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
          <Show when={hoverX() !== null && hoveredSeries().length > 0}>
            <line
              x1={hoverX() ?? 0}
              x2={hoverX() ?? 0}
              y1={GUEST_DRAWER_HISTORY_PLOT_TOP}
              y2={GUEST_DRAWER_HISTORY_CHART_HEIGHT - GUEST_DRAWER_HISTORY_PLOT_BOTTOM}
              stroke="currentColor"
              stroke-dasharray="3 3"
              stroke-width="1"
              class="text-base-content/50"
              vector-effect="non-scaling-stroke"
            />
            <For each={hoveredSeries()}>
              {(item) => (
                <circle
                  cx={item.x}
                  cy={item.y}
                  fill={item.color}
                  r="3"
                  stroke="currentColor"
                  stroke-width="1"
                  class="text-surface"
                  vector-effect="non-scaling-stroke"
                />
              )}
            </For>
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
  onMount(() => {
    void loadRuntimeCapabilities();
  });

  const locked = createMemo(() => isRangeLocked(props.range));
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
        range: props.range,
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
  const displayMetrics = createMemo(() =>
    mergeFallbackHistoryMetrics(metrics(), props.fallbackMetrics),
  );
  const groups = createMemo(() => props.groups ?? GUEST_DRAWER_HISTORY_GROUPS);
  const errorText = createMemo(() => (historyQuery.error() ? 'Failed to load history data' : ''));

  return (
    <Show
      when={props.target}
      fallback={<div class="py-6 text-center text-sm text-muted">History unavailable</div>}
    >
      <div class="space-y-3">
        <Show
          when={!locked()}
          fallback={
            <div class="rounded-sm border border-border bg-surface p-5 text-sm text-muted">
              {formatRangeLabel(props.range)} history is not enabled on this instance. Current
              runtime allows {maxHistoryDays()} days.
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
            <div class={`grid gap-3 ${groups().length > 3 ? 'xl:grid-cols-4' : 'xl:grid-cols-3'}`}>
              <For each={groups()}>
                {(group) => (
                  <GuestDrawerHistoryGroupChart
                    group={group}
                    loading={historyQuery.loading() && !historyQuery.resolvedOnce()}
                    metrics={displayMetrics()}
                    range={props.range}
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
