import {
  For,
  Show,
  createEffect,
  createMemo,
  createSignal,
  createUniqueId,
  on,
  onMount,
  type Component,
} from 'solid-js';

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
import { Button } from '@/components/shared/Button';
import { InlineNotice } from '@/components/shared/InlineNotice';
import { LoadingSpinner } from '@/components/shared/LoadingSpinner';
import {
  HISTORY_CHART_RANGES,
  formatHistoryChartTooltipValue,
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
  type GuestDrawerHistoryTimeBounds,
} from './guestDrawerModel';

interface GuestDrawerHistoryProps {
  currentMetrics?: Record<string, number | null | undefined>;
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
  currentMetrics?: Record<string, number | null | undefined>;
  group: GuestDrawerHistoryGroupConfig;
  loading: boolean;
  metrics: Record<string, AggregatedMetricPoint[] | undefined>;
  range: HistoryTimeRange;
  sourceKey: string;
  timeBounds: GuestDrawerHistoryTimeBounds | null;
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
    selectBaseClass={`${filterSelectClass} h-7 min-h-11 py-0 text-[11px] sm:min-h-0`}
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

const findClosestGuestDrawerHistoryTimestamp = (
  timestamps: readonly number[],
  timestamp: number,
): number | null => {
  if (timestamps.length === 0) return null;

  let closest = timestamps[0];
  let closestDistance = Math.abs(closest - timestamp);
  for (const candidate of timestamps.slice(1)) {
    const distance = Math.abs(candidate - timestamp);
    if (distance < closestDistance) {
      closest = candidate;
      closestDistance = distance;
    }
  }
  return closest;
};

const GuestDrawerHistoryGroupChart: Component<GuestDrawerHistoryGroupChartProps> = (props) => {
  const [hoverTimestamp, setHoverTimestamp] = createSignal<number | null>(null);
  const [selectedTimestamp, setSelectedTimestamp] = createSignal<number | null>(null);
  const inspectionId = `history-inspection-${createUniqueId()}`;
  createEffect(
    on(
      () => props.sourceKey,
      () => {
        setHoverTimestamp(null);
        setSelectedTimestamp(null);
      },
    ),
  );
  const series = createMemo(() =>
    props.group.series.map((config) => ({
      ...config,
      points: normalizeGuestDrawerHistoryPoints(props.metrics[config.metric], config.unit),
    })),
  );
  // Step through actual stored times, not synthetic points between samples.
  // Keep the selected time across polls even if its ordinal index changes.
  const observationTimes = createMemo(() =>
    [...new Set(series().flatMap((item) => item.points.map((point) => point.timestamp)))].sort(
      (a, b) => a - b,
    ),
  );
  const selectedIndex = createMemo(() => {
    const times = observationTimes();
    const selected = selectedTimestamp();
    if (selected === null) return Math.max(0, times.length - 1);
    const closest = findClosestGuestDrawerHistoryTimestamp(times, selected);
    return closest === null ? 0 : times.indexOf(closest);
  });
  const inspectionTimestamp = createMemo(() =>
    selectedTimestamp() === null ? null : (observationTimes()[selectedIndex()] ?? null),
  );
  // Resolve the pointer to one real time across the whole group. Independent
  // nearest-series reads would attribute values from different times to the
  // first metric's timestamp, and can even borrow a live fallback reading.
  const activeTimestamp = createMemo(() => {
    const inspected = inspectionTimestamp();
    if (inspected !== null) return inspected;
    const hovered = hoverTimestamp();
    return hovered === null
      ? null
      : findClosestGuestDrawerHistoryTimestamp(observationTimes(), hovered);
  });
  const observationValueText = (timestamp: number | undefined): string => {
    if (timestamp === undefined) return 'No stored history observations.';
    const values = series().map((item) => {
      const point = item.points.find((point) => point.timestamp === timestamp);
      return `${item.label} ${point ? formatHistoryChartTooltipValue(point.value, item.unit) : 'no observation'}`;
    });
    return `${new Date(timestamp).toLocaleString()}. ${values.join('. ')}.`;
  };
  const inspectionValueText = createMemo(() =>
    observationValueText(observationTimes()[selectedIndex()]),
  );
  const chartDescription = createMemo(() => {
    if (props.loading) return 'Loading history.';
    const timestamp = activeTimestamp();
    if (timestamp !== null) return observationValueText(timestamp);
    const count = observationTimes().length;
    if (count === 0) return inspectionValueText();
    return `${count} stored observation ${count === 1 ? 'time' : 'times'}. ${inspectionValueText()}`;
  });
  const drawableSeries = createMemo(() => series().filter((item) => item.points.length >= 2));
  const singlePointSeries = createMemo(() => series().filter((item) => item.points.length === 1));
  const singleObservation = createMemo(() =>
    observationTimes().length === 1 ? { timestamp: observationTimes()[0] } : null,
  );
  const scale = createMemo(() => getGuestDrawerHistoryScale(series(), props.group.unit));
  const bounds = () => props.timeBounds;
  const hasStoredData = createMemo(() => observationTimes().length > 0 && bounds() !== null);
  const hoveredSeries = createMemo(() => {
    const timestamp = activeTimestamp();
    const rangeBounds = bounds();
    if (timestamp === null || !rangeBounds) return [];

    return series()
      .map((item) => {
        const point = item.points.find((point) => point.timestamp === timestamp);
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
      const currentValue = props.currentMetrics?.[item.metric];
      return {
        ...item,
        isCurrent:
          activeTimestamp() === null &&
          item.points.length === 0 &&
          typeof currentValue === 'number' &&
          Number.isFinite(currentValue),
        valueLabel: hovered
          ? getGuestDrawerHistoryValueLabel([hovered.point], item.unit)
          : activeTimestamp() !== null
            ? '-'
            : item.points.length > 0
              ? getGuestDrawerHistoryValueLabel(item.points, item.unit)
              : typeof currentValue === 'number' && Number.isFinite(currentValue)
                ? formatHistoryChartTooltipValue(currentValue, item.unit)
                : '-',
      };
    }),
  );
  const hoverTimeLabel = createMemo(() => {
    const timestamp = activeTimestamp();
    return timestamp === null ? '' : formatHistoryChartTimeLabel(timestamp, props.range);
  });

  const handleHoverMove = (event: MouseEvent & { currentTarget: SVGSVGElement }) => {
    if (inspectionTimestamp() !== null) return;
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
      class="flex min-h-[154px] flex-col rounded-xs border border-border bg-surface p-2.5"
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
              <span
                class="inline-flex items-center gap-1"
                data-history-current={item.isCurrent ? item.metric : undefined}
              >
                <svg aria-hidden="true" class="h-2.5 w-2.5 shrink-0" viewBox="0 0 10 10">
                  <circle cx="5" cy="5" r="4" fill={item.color} />
                </svg>
                <span class="font-medium text-base-content">{item.label}</span>
                <span>{item.valueLabel}</span>
                <Show when={item.isCurrent}>
                  <span>current</span>
                </Show>
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
          aria-describedby={`${inspectionId}-description`}
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
              <>
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
                <For each={singlePointSeries()}>
                  {(item) => (
                    <circle
                      aria-hidden="true"
                      data-history-observation={item.metric}
                      cx={getGuestDrawerHistoryX(
                        item.points[0].timestamp,
                        rangeBounds().startTime,
                        rangeBounds().endTime,
                      )}
                      cy={getGuestDrawerHistoryY(item.points[0].value, scale())}
                      r="3.5"
                      fill={item.color}
                      stroke="currentColor"
                      stroke-width="1"
                      class="text-surface"
                      vector-effect="non-scaling-stroke"
                    />
                  )}
                </For>
              </>
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
              class=""
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
        <Show when={!hasStoredData() && !props.loading}>
          <div class="absolute inset-x-8 inset-y-2 flex items-center justify-center rounded-xs text-xs text-muted">
            No stored history in this range
          </div>
        </Show>
        <Show when={props.loading}>
          <div class="absolute inset-x-8 inset-y-2 flex items-center justify-center rounded-xs text-xs text-muted">
            Loading history
          </div>
        </Show>
      </div>
      <Show when={bounds()}>
        {(window) => (
          <div
            class="ml-[34px] mr-2 mt-1 flex justify-between gap-2 text-[10px] tabular-nums text-muted"
            data-testid="guest-history-time-window"
          >
            <For
              each={[
                { label: 'Window start', timestamp: window().startTime },
                { label: 'Window end', timestamp: window().endTime },
              ]}
            >
              {(endpoint) => (
                <time
                  dateTime={new Date(endpoint.timestamp).toISOString()}
                  aria-label={`${endpoint.label}: ${new Date(endpoint.timestamp).toLocaleString()}`}
                  title={new Date(endpoint.timestamp).toLocaleString()}
                >
                  {new Date(endpoint.timestamp).toLocaleString([], {
                    month: 'short',
                    day: 'numeric',
                    hour: '2-digit',
                    minute: '2-digit',
                  })}
                </time>
              )}
            </For>
          </div>
        )}
      </Show>
      <p id={`${inspectionId}-description`} class="sr-only">
        {chartDescription()}
      </p>
      <Show when={!props.loading && singleObservation()}>
        {(observation) => (
          <p class="mt-2 text-[10px] text-muted">
            <span>Single observation. No trend yet.</span>
            <time
              class="block tabular-nums"
              dateTime={new Date(observation().timestamp).toISOString()}
            >
              {new Date(observation().timestamp).toLocaleString()}
            </time>
          </p>
        )}
      </Show>
      <Show when={observationTimes().length > 1}>
        <div class="mt-2">
          <label for={inspectionId} class="flex justify-between gap-2 text-[10px] text-muted">
            <span>Inspect history</span>
            <span aria-hidden="true">
              {selectedIndex() + 1} / {observationTimes().length}
            </span>
          </label>
          <input
            id={inspectionId}
            type="range"
            aria-label={`Inspect ${props.group.label} history`}
            aria-valuetext={inspectionValueText()}
            aria-describedby={`${inspectionId}-help`}
            min="0"
            max={observationTimes().length - 1}
            step="1"
            value={selectedIndex()}
            class="h-11 w-full cursor-pointer rounded-xs focus-visible:outline-solid focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-blue-500 sm:h-6"
            onFocus={() => {
              setHoverTimestamp(null);
              setSelectedTimestamp(observationTimes()[selectedIndex()] ?? null);
            }}
            onBlur={() => setSelectedTimestamp(null)}
            onInput={(event) => {
              const index = event.currentTarget.valueAsNumber;
              if (Number.isFinite(index)) setSelectedTimestamp(observationTimes()[index] ?? null);
            }}
          />
          <p id={`${inspectionId}-help`} class="sr-only">
            Use arrow keys, Home or End to inspect stored observations.
          </p>
        </div>
      </Show>
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
    fetcher: async (key, signal) =>
      normalizeHistoryResponse(
        await ChartsAPI.getMetricsHistory({
          resourceType: key.resourceType,
          resourceId: key.resourceId,
          range: key.range,
          maxPoints: GUEST_DRAWER_HISTORY_MAX_POINTS,
          signal,
        }),
      ),
    initialValue: EMPTY_HISTORY_RESPONSE,
    cacheKey: (key) => `guest-drawer-history:${key.resourceType}:${key.resourceId}:${key.range}`,
    // Former-host or former-range observations are not evidence for this
    // target. Matching cached reads and same-source polling remain retained.
    retainPreviousValueOnSourceChange: false,
    pollMs: GUEST_DRAWER_HISTORY_POLL_MS,
  });

  const metrics = createMemo(() => historyQuery.value().metrics ?? {});
  const groups = createMemo(() => props.groups ?? GUEST_DRAWER_HISTORY_GROUPS);
  const timeBounds = createMemo(() =>
    getGuestDrawerHistoryRangeBounds(
      groups().flatMap((group) =>
        group.series.map((series) => ({
          points: normalizeGuestDrawerHistoryPoints(metrics()[series.metric], series.unit),
        })),
      ),
      historyQuery.value(),
    ),
  );
  const hasHistoryPoints = createMemo(() =>
    groups().some((group) =>
      group.series.some(
        (series) =>
          normalizeGuestDrawerHistoryPoints(metrics()[series.metric], series.unit).length > 0,
      ),
    ),
  );

  return (
    <Show
      when={props.target}
      fallback={<div class="py-6 text-center text-sm text-muted">History unavailable</div>}
    >
      <div class="space-y-3">
        <Show
          when={!locked()}
          fallback={
            <div class="rounded-xs border border-border bg-surface p-5 text-sm text-muted">
              {formatRangeLabel(props.range)} history requires a higher license plan. This
              instance's plan retains {maxHistoryDays()} days.
            </div>
          }
        >
          <div class="flex flex-wrap items-start justify-end gap-3">
            <div
              class="min-w-0 flex-1"
              role="status"
              aria-label="History refresh status"
              aria-live="polite"
              aria-atomic="true"
            >
              <Show when={historyQuery.error()}>
                <InlineNotice tone={hasHistoryPoints() ? 'warning' : 'danger'}>
                  {hasHistoryPoints()
                    ? 'History refresh failed. Showing previously loaded history.'
                    : 'Failed to load history data'}
                </InlineNotice>
              </Show>
            </div>
            <Button
              size="sm"
              class="min-h-11 shrink-0 aria-disabled:opacity-50 sm:min-h-8"
              aria-busy={historyQuery.loading()}
              aria-disabled={historyQuery.loading()}
              onClick={() => {
                // Keep the control focusable through a retry, but reject repeat
                // activation until the current read settles.
                if (!historyQuery.loading()) void historyQuery.refetch();
              }}
            >
              <Show when={historyQuery.loading()}>
                <LoadingSpinner size="sm" tone="current" class="mr-2" />
              </Show>
              {historyQuery.error() ? 'Retry history' : 'Refresh history'}
            </Button>
          </div>
          <Show when={!historyQuery.error() || hasHistoryPoints()}>
            <div class={`grid gap-3 ${groups().length > 3 ? 'xl:grid-cols-4' : 'xl:grid-cols-3'}`}>
              <For each={groups()}>
                {(group) => (
                  <GuestDrawerHistoryGroupChart
                    group={group}
                    loading={historyQuery.loading() && !historyQuery.resolvedOnce()}
                    metrics={metrics()}
                    currentMetrics={props.currentMetrics}
                    range={props.range}
                    sourceKey={`${props.target?.resourceType}:${props.target?.resourceId}:${props.range}`}
                    timeBounds={timeBounds()}
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
