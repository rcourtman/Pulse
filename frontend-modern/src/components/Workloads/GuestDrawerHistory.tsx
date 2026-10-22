import {
  For,
  Show,
  createEffect,
  createMemo,
  createSignal,
  createUniqueId,
  on,
  onCleanup,
  onMount,
  type Component,
} from 'solid-js';

import { type AggregatedMetricPoint, type HistoryTimeRange } from '@/api/charts';
import { FormSelect } from '@/components/shared/FormSelect';
import { filterSelectClass } from '@/components/shared/FilterToolbar';
import { Button } from '@/components/shared/Button';
import { InlineNotice } from '@/components/shared/InlineNotice';
import { LoadingSpinner } from '@/components/shared/LoadingSpinner';
import { PhoneChartsToggle } from '@/components/shared/PhoneChartsToggle';
import {
  HISTORY_CHART_RANGES,
  formatHistoryChartTooltipValue,
  formatHistoryRangeOptionLabel,
  formatHistoryChartTimeLabel,
  getHistoryReadoutLabel,
  getHistorySourceNotice,
  type HistorySource,
  withoutRepeatedHistoryAxisLabels,
} from '@/components/shared/historyChartModel';
import { isRangeLocked, loadRuntimeCapabilities, maxHistoryDays } from '@/stores/license';
import { getAPIReadAccessErrorMessage } from '@/utils/apiAccessError';
import { formatCompactSpeed } from '@/utils/format';

import {
  GUEST_DRAWER_HISTORY_GROUPS,
  buildGuestDrawerHistoryBandPath,
  buildGuestDrawerHistoryPath,
  getGuestDrawerHistoryRangeBounds,
  getGuestDrawerHistoryScale,
  getGuestDrawerHistorySegments,
  getGuestDrawerHistoryValueLabel,
  normalizeGuestDrawerHistoryPoints,
  type GuestDrawerHistoryGroupConfig,
  type GuestDrawerHistoryDeferredMetric,
  type GuestDrawerHistoryTarget,
  type GuestDrawerHistoryTimeBounds,
} from './guestDrawerModel';
import {
  createGuestDrawerHistoryQuery,
  createGuestDrawerHistorySource,
} from './guestDrawerHistoryQuery';

/** An active alert on one charted metric; a numeric trigger draws a dashed line. */
export interface GuestDrawerHistoryAlert {
  severity: 'warning' | 'critical';
  value?: number;
}

interface GuestDrawerHistoryProps {
  currentMetrics?: Record<string, number | null | undefined>;
  deferredMetrics?: Record<string, GuestDrawerHistoryDeferredMetric | undefined>;
  groups?: GuestDrawerHistoryGroupConfig[];
  range: HistoryTimeRange;
  target: GuestDrawerHistoryTarget | null;
  /** Shows the range picker beside the charts, for hosts without a tab strip slot. */
  onRangeChange?: (range: HistoryTimeRange) => void;
  /** False while the charts are hidden (another drawer tab), so they do not poll. */
  enabled?: boolean;
  alerts?: Record<string, GuestDrawerHistoryAlert | undefined>;
}

interface GuestDrawerHistoryRangeSelectProps {
  onRangeChange: (range: HistoryTimeRange) => void;
  range: HistoryTimeRange;
}

interface GuestDrawerHistoryGroupChartProps {
  alerts?: Record<string, GuestDrawerHistoryAlert | undefined>;
  /** Collapsed below the sm breakpoint until the reader asks for more charts. */
  hiddenOnPhone?: boolean;
  currentMetrics?: Record<string, number | null | undefined>;
  deferredMetrics?: Record<string, GuestDrawerHistoryDeferredMetric | undefined>;
  group: GuestDrawerHistoryGroupConfig;
  loading: boolean;
  metrics: Record<string, AggregatedMetricPoint[] | undefined>;
  range: HistoryTimeRange;
  /** Where the reply's readings came from; names the readout's time. */
  source?: HistorySource;
  sourceKey: string;
  timeBounds: GuestDrawerHistoryTimeBounds | null;
  touchInspection: boolean;
}

const GUEST_DRAWER_HISTORY_CHART_WIDTH = 360;
const GUEST_DRAWER_HISTORY_CHART_HEIGHT = 92;
const GUEST_DRAWER_HISTORY_PLOT_LEFT = 34;
const GUEST_DRAWER_HISTORY_PLOT_RIGHT = 8;
const GUEST_DRAWER_HISTORY_PLOT_TOP = 8;
const GUEST_DRAWER_HISTORY_PLOT_BOTTOM = 18;

// A guest and a node drawer can be open together, so each picker owns its id.
export const GuestDrawerHistoryRangeSelect: Component<GuestDrawerHistoryRangeSelectProps> = (
  props,
) => {
  // Created once: a getter prop would mint a new id on every read, splitting
  // the label from its select.
  const id = `guest-history-range-${createUniqueId()}`;
  return (
    <FormSelect
      id={id}
      label="History range"
      labelClass="sr-only"
      fieldBaseClass="contents"
      selectBaseClass={`${filterSelectClass} h-7 min-h-11 py-0 text-[11px] sm:min-h-0`}
      data-testid="guest-history-range-control"
      value={props.range}
      onChange={(event) => props.onRangeChange(event.currentTarget.value as HistoryTimeRange)}
    >
      <For each={HISTORY_CHART_RANGES}>
        {(option) => <option value={option}>{formatHistoryRangeOptionLabel(option)}</option>}
      </For>
    </FormSelect>
  );
};

// Gridline labels state the scale's actual value at each line, so a reader
// can tell a KB/s peak from a MB/s one. ("Avg" used to mark the midpoint of
// the scale, which is not the measured average.)
const getAxisLabel = (
  unit: string,
  pct: number,
  scale: { minValue: number; maxValue: number },
): string => {
  const value = scale.maxValue - pct * (scale.maxValue - scale.minValue);
  if (unit === '%') return `${Math.round(value)}%`;
  if (unit === 'C') return `${Math.round(value)}°C`;
  if (unit === 'B/s') return value <= 0 ? '0' : `${formatCompactSpeed(value)}/s`;
  return `${Math.round(value)}`;
};

const alertToneClass = (severity: GuestDrawerHistoryAlert['severity'] | undefined) =>
  severity === 'critical'
    ? 'font-semibold text-red-700 dark:text-red-300'
    : severity === 'warning'
      ? 'font-semibold text-amber-700 dark:text-amber-300'
      : '';

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

interface HistoryPointMarkerProps {
  x: number;
  y: number;
  diameter: number;
  color: string;
  'data-history-observation'?: string;
  'data-history-hover-marker'?: string;
}

// The plot stretches its viewBox to the panel (preserveAspectRatio="none"), so
// a <circle> renders as an oval. A zero-length, round-capped stroke with a
// non-scaling stroke is drawn in screen pixels and stays round at any width.
// The surface-coloured ring underneath keeps the dot legible over a line.
const HistoryPointMarker: Component<HistoryPointMarkerProps> = (props) => {
  const d = () => `M${props.x} ${props.y}h0`;
  return (
    <g
      aria-hidden="true"
      data-history-observation={props['data-history-observation']}
      data-history-hover-marker={props['data-history-hover-marker']}
      data-x={props.x}
      data-y={props.y}
    >
      <path
        d={d()}
        stroke="currentColor"
        class="text-surface"
        stroke-width={props.diameter + 2}
        stroke-linecap="round"
        vector-effect="non-scaling-stroke"
      />
      <path
        d={d()}
        stroke={props.color}
        stroke-width={props.diameter}
        stroke-linecap="round"
        vector-effect="non-scaling-stroke"
      />
    </g>
  );
};

const GuestDrawerHistoryGroupChart: Component<GuestDrawerHistoryGroupChartProps> = (props) => {
  const [hoverTimestamp, setHoverTimestamp] = createSignal<number | null>(null);
  const [selectedTimestamp, setSelectedTimestamp] = createSignal<number | null>(null);
  const inspectionId = `history-inspection-${createUniqueId()}`;
  createEffect(
    on(
      () => props.touchInspection,
      (touch, previousTouch) => {
        // Native touch scrubbing need not focus the input. Give inspection
        // back to the mouse when no focused control still owns the selection.
        if (previousTouch && !touch && document.activeElement?.id !== inspectionId) {
          setSelectedTimestamp(null);
        }
      },
    ),
  );
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
  // The default legend is a single dated observation too. Independent latest
  // values would combine different times (or a live fallback) into a tuple
  // which never existed, disagreeing with the chart's accessible description.
  const readoutTimestamp = createMemo(() => activeTimestamp() ?? observationTimes().at(-1) ?? null);
  const observationValueText = (timestamp: number | undefined): string => {
    if (timestamp === undefined) return 'No stored history observations.';
    const values = series().map((item) => {
      const point = item.points.find((point) => point.timestamp === timestamp);
      if (!point) return `${item.label} no observation`;
      const value = formatHistoryChartTooltipValue(point.value, item.unit);
      const low = formatHistoryChartTooltipValue(point.min ?? point.value, item.unit);
      const high = formatHistoryChartTooltipValue(point.max ?? point.value, item.unit);
      return low === high
        ? `${item.label} ${value}`
        : `${item.label} ${value} average, ${low} to ${high}`;
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
  const segmentedSeries = createMemo(() =>
    series().map((item) => ({
      ...item,
      segments: getGuestDrawerHistorySegments(item.points, observationTimes()),
    })),
  );
  const gapMessage = createMemo(() => {
    const labels = segmentedSeries()
      .filter((item) => item.segments.length > 1)
      .map((item) => item.label);
    return labels.length > 0
      ? `Missing observations: ${labels.join(', ')}. Lines break where a reading is missing.`
      : null;
  });
  const drawableSeries = createMemo(() =>
    segmentedSeries().flatMap((item) =>
      item.segments.filter((points) => points.length >= 2).map((points) => ({ ...item, points })),
    ),
  );
  const singlePointSeries = createMemo(() =>
    segmentedSeries().flatMap((item) =>
      item.segments.filter((points) => points.length === 1).map((points) => ({ ...item, points })),
    ),
  );
  const singleObservation = createMemo(() =>
    observationTimes().length === 1 ? { timestamp: observationTimes()[0] } : null,
  );
  const thresholds = createMemo(() =>
    series().flatMap((item) => {
      const alert = props.alerts?.[item.metric];
      return alert && typeof alert.value === 'number' && Number.isFinite(alert.value)
        ? [{ metric: item.metric, label: item.label, severity: alert.severity, value: alert.value }]
        : [];
    }),
  );
  const alertDescription = createMemo(() =>
    series()
      .flatMap((item) => {
        const alert = props.alerts?.[item.metric];
        if (!alert) return [];
        const trigger =
          typeof alert.value === 'number' && Number.isFinite(alert.value)
            ? ` at ${formatHistoryChartTooltipValue(alert.value, item.unit)}`
            : '';
        return [`${item.label} ${alert.severity} alert${trigger}.`];
      })
      .join(' '),
  );
  // An alert trigger outside the observed range widens the scale, so the
  // dashed line sits at its real value, not clamped to an edge.
  const scale = createMemo(() =>
    getGuestDrawerHistoryScale(
      [
        ...series(),
        ...(observationTimes().length > 0
          ? thresholds().map((threshold) => ({
              points: [
                {
                  timestamp: observationTimes()[0],
                  value: threshold.value,
                  min: threshold.value,
                  max: threshold.value,
                },
              ],
            }))
          : []),
      ],
      props.group.unit,
    ),
  );
  // Top, middle and bottom gridline labels; a repeated middle one is blank.
  const axisLabels = createMemo(() =>
    withoutRepeatedHistoryAxisLabels(
      [0, 0.5, 1].map((tick) => getAxisLabel(props.group.unit, tick, scale())),
    ),
  );
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
  const displaySeries = createMemo(() =>
    series().map((item, index) => {
      const timestamp = readoutTimestamp();
      const point = item.points.find((point) => point.timestamp === timestamp);
      const deferred = props.deferredMetrics?.[item.metric];
      const fallbackValue = deferred
        ? deferred.lastKnownValue
        : props.currentMetrics?.[item.metric];
      const isFallback =
        timestamp === null && typeof fallbackValue === 'number' && Number.isFinite(fallbackValue);
      // While a time is inspected, name the bucket's peak when it rises a
      // visible distance (a tenth of the plot) above the average the line
      // shows. The default readout stays one number per series.
      const peakVisible =
        point !== undefined &&
        activeTimestamp() !== null &&
        (point.max ?? point.value) - point.value >= 0.1 * (scale().maxValue - scale().minValue);
      const alert = props.alerts?.[item.metric];
      return {
        ...item,
        // Names the dashed line: its metric's reading carries the trigger.
        alertLabel:
          alert && typeof alert.value === 'number' && Number.isFinite(alert.value)
            ? `alert at ${formatHistoryChartTooltipValue(alert.value, item.unit)}`
            : null,
        peakLabel:
          point && peakVisible
            ? `peak ${formatHistoryChartTooltipValue(point.max ?? point.value, item.unit)}`
            : null,
        deferred,
        descriptionId: `${inspectionId}-deferred-${index}`,
        isCurrent: isFallback && !deferred,
        isLastKnown: isFallback && Boolean(deferred) && !deferred?.valueLabel,
        isUnknown: isFallback && deferred?.valueLabel === 'freshness unknown',
        valueLabel: point
          ? getGuestDrawerHistoryValueLabel([point], item.unit)
          : // A stored time where this series has no reading says so in place,
            // rather than through a note that appears and disappears (and moves
            // the chart) as the inspected time changes.
            timestamp !== null
            ? 'no reading'
            : typeof fallbackValue === 'number' && Number.isFinite(fallbackValue)
              ? formatHistoryChartTooltipValue(fallbackValue, item.unit)
              : '-',
      };
    }),
  );
  const readoutTime = createMemo(() => {
    const timestamp = readoutTimestamp();
    if (timestamp === null) return null;
    const date = new Date(timestamp);
    return {
      label: formatHistoryChartTimeLabel(timestamp, props.range),
      dateTime: date.toISOString(),
      fullLabel: date.toLocaleString(),
    };
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
      class={`flex min-w-0 min-h-[154px] flex-col rounded-xs border border-border bg-surface p-2.5 ${props.hiddenOnPhone ? 'max-sm:hidden' : ''}`}
      data-testid="guest-history-group-chart"
      data-phone-collapsed={props.hiddenOnPhone ? 'true' : undefined}
      data-history-group={props.group.id}
    >
      <div class="mb-2 flex flex-wrap items-start justify-between gap-x-4 gap-y-1">
        <div class="flex min-w-0 flex-wrap items-baseline gap-x-2 gap-y-1">
          <h4 class="text-xs font-semibold uppercase tracking-wide text-muted">
            {props.group.label}
          </h4>
          <Show when={readoutTime()}>
            {(time) => (
              <time
                class="text-[10px] tabular-nums text-muted"
                data-testid="guest-history-hover-time"
                dateTime={time().dateTime}
                title={time().fullLabel}
                aria-label={`${getHistoryReadoutLabel(props.source, true)} observation: ${time().fullLabel}`}
              >
                {getHistoryReadoutLabel(props.source, activeTimestamp() !== null)}
                {' · '}
                {time().label}
              </time>
            )}
          </Show>
        </div>
        <div class="flex flex-wrap justify-end gap-x-3 gap-y-1 text-xs text-muted">
          <For each={displaySeries()}>
            {(item) => (
              <span
                class="inline-flex items-center gap-1"
                data-history-current={item.isCurrent ? item.metric : undefined}
                data-history-last-known={item.isLastKnown ? item.metric : undefined}
                data-history-unknown={item.isUnknown ? item.metric : undefined}
                aria-describedby={item.deferred ? item.descriptionId : undefined}
              >
                <svg aria-hidden="true" class="h-2.5 w-2.5 shrink-0" viewBox="0 0 10 10">
                  <circle cx="5" cy="5" r="4" fill={item.color} />
                </svg>
                <span class="font-medium text-base-content">{item.label}</span>
                <span class={alertToneClass(props.alerts?.[item.metric]?.severity)}>
                  {item.valueLabel}
                </span>
                <Show when={item.peakLabel}>
                  {(label) => <span data-history-peak={item.metric}>{label()}</span>}
                </Show>
                <Show when={item.alertLabel}>
                  {(label) => (
                    <span
                      data-history-alert-label={item.metric}
                      class={alertToneClass(props.alerts?.[item.metric]?.severity)}
                    >
                      {label()}
                    </span>
                  )}
                </Show>
                <Show when={item.isCurrent}>
                  <span>current</span>
                </Show>
                <Show when={item.isUnknown}>
                  <span>freshness unknown</span>
                </Show>
                <Show when={item.isLastKnown}>
                  <span>last known</span>
                </Show>
              </span>
            )}
          </For>
        </div>
      </div>

      <For each={displaySeries().filter((item) => item.deferred)}>
        {(item) => (
          <p
            id={item.descriptionId}
            class="mb-2 whitespace-normal wrap-anywhere text-xs leading-relaxed text-muted"
            data-history-deferred={item.metric}
          >
            {item.label} reading: {item.deferred?.message}
          </p>
        )}
      </For>

      <div class="relative min-h-24 flex-1">
        <For each={[0, 0.5, 1]}>
          {(tick) => (
            <span
              data-history-axis-label
              // Centred on the SVG gridlines at y = 8, 41 and 74 of the 92-unit
              // viewBox (8.7%, 44.6%, 80.4%), so each label sits on its line.
              class={`pointer-events-none absolute left-0 w-8 -translate-y-1/2 text-right text-[10px] tabular-nums text-muted ${
                tick === 0 ? 'top-[8.7%]' : tick === 0.5 ? 'top-[44.6%]' : 'top-[80.4%]'
              }`}
            >
              {axisLabels()[tick === 0 ? 0 : tick === 0.5 ? 1 : 2]}
            </span>
          )}
        </For>
        <svg
          aria-label={`${props.group.label} history`}
          aria-describedby={`${inspectionId}-description${gapMessage() ? ` ${inspectionId}-gaps` : ''}`}
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
          {/* Before any history the scale is a placeholder, so a trigger line
              would sit at an arbitrary height (an I/O trigger clamps to the top). */}
          <For each={hasStoredData() ? thresholds() : []}>
            {(threshold) => (
              <line
                data-history-threshold={threshold.metric}
                x1={GUEST_DRAWER_HISTORY_PLOT_LEFT}
                x2={GUEST_DRAWER_HISTORY_CHART_WIDTH - GUEST_DRAWER_HISTORY_PLOT_RIGHT}
                y1={getGuestDrawerHistoryY(threshold.value, scale())}
                y2={getGuestDrawerHistoryY(threshold.value, scale())}
                stroke="currentColor"
                class={threshold.severity === 'critical' ? 'text-red-500' : 'text-amber-500'}
                stroke-width="1"
                stroke-dasharray="4 4"
                vector-effect="non-scaling-stroke"
              >
                <title>{`${threshold.label} alert at ${formatHistoryChartTooltipValue(threshold.value, props.group.unit)}`}</title>
              </line>
            )}
          </For>
          <Show when={bounds()}>
            {(rangeBounds) => (
              <>
                <For each={drawableSeries()}>
                  {(item) => (
                    <Show
                      when={buildGuestDrawerHistoryBandPath(
                        item.points,
                        scale(),
                        rangeBounds().startTime,
                        rangeBounds().endTime,
                        GUEST_DRAWER_HISTORY_CHART_WIDTH,
                        GUEST_DRAWER_HISTORY_CHART_HEIGHT,
                      )}
                    >
                      {(band) => (
                        <path
                          data-history-band={item.metric}
                          d={band()}
                          fill={item.color}
                          fill-opacity="0.2"
                          stroke={item.color}
                          stroke-opacity="0.45"
                          stroke-width="1"
                          stroke-linejoin="round"
                          vector-effect="non-scaling-stroke"
                        />
                      )}
                    </Show>
                  )}
                </For>
                <For each={drawableSeries()}>
                  {(item) => (
                    <path
                      data-history-series={item.metric}
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
                    <HistoryPointMarker
                      data-history-observation={item.metric}
                      x={getGuestDrawerHistoryX(
                        item.points[0].timestamp,
                        rangeBounds().startTime,
                        rangeBounds().endTime,
                      )}
                      y={getGuestDrawerHistoryY(item.points[0].value, scale())}
                      diameter={7}
                      color={item.color}
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
                <HistoryPointMarker
                  data-history-hover-marker={item.metric}
                  x={item.x}
                  y={item.y}
                  diameter={6}
                  color={item.color}
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
        {alertDescription() ? ` ${alertDescription()}` : ''}
      </p>
      <Show when={gapMessage()}>
        {(message) => (
          <p id={`${inspectionId}-gaps`} class="mt-2 text-xs text-muted" data-history-gaps>
            {message()}
          </p>
        )}
      </Show>
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
        {/* Keep native keyboard and assistive-technology access while hiding
            mouse-redundant controls. Touch also reveals them on hybrid devices. */}
        <div
          class={`mt-2 ${props.touchInspection ? '' : '[@media(hover:hover)_and_(pointer:fine)]:not-focus-within:sr-only'}`}
          data-testid="guest-history-inspection-control"
        >
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
            class={`h-11 w-full cursor-pointer rounded-xs focus-visible:outline-solid focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-blue-500 ${props.touchInspection ? '' : '[@media(hover:hover)_and_(pointer:fine)]:h-6'}`}
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
  const [touchInspection, setTouchInspection] = createSignal(false);
  // On a phone every panel stacks, so a drawer's Overview ran four charts
  // deep before its disks and details. Phones show the first panel, plus any
  // panel with an active alert, and the rest on request. Wider layouts show
  // every panel; the class only applies below the sm breakpoint.
  const [showAllOnPhone, setShowAllOnPhone] = createSignal(false);
  const gridId = `history-groups-${createUniqueId()}`;
  onMount(() => {
    void loadRuntimeCapabilities();
  });

  const locked = createMemo(() => isRangeLocked(props.range));
  const historySource = createGuestDrawerHistorySource(
    () => props.target,
    () => props.range,
    () => !locked() && props.enabled !== false,
  );
  const historyQuery = createGuestDrawerHistoryQuery(historySource);
  // A successful retry removes the focused Retry button with the error. Keep
  // keyboard focus in the history region instead of dropping it to the page.
  // Focus is read when the button's branch is disposed, before its node leaves
  // the DOM: Chromium blurs a removed element, so a blur-tracked flag is
  // already cleared by the time the error effect runs.
  let regionRef: HTMLDivElement | undefined;
  let retryRef: HTMLButtonElement | undefined;
  let retryHadFocus = false;
  createEffect(
    on(
      () => historyQuery.error(),
      (error, previous) => {
        if (previous && !error && retryHadFocus) regionRef?.focus();
        retryHadFocus = false;
      },
    ),
  );

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
  // A panel shown on a phone because of an alert stays shown after the alert
  // clears, so a reader mid-inspection does not lose the chart under them.
  const alertedGroupIds = new Set<string>();
  const alertedGroups = createMemo(() => {
    for (const group of groups()) {
      if (group.series.some((series) => Boolean(props.alerts?.[series.metric]))) {
        alertedGroupIds.add(group.id);
      }
    }
    return new Set(alertedGroupIds);
  });
  const groupHasAlert = (group: GuestDrawerHistoryGroupConfig) => alertedGroups().has(group.id);
  const isPhoneCollapsed = (group: GuestDrawerHistoryGroupConfig, index: number) =>
    !showAllOnPhone() && index > 0 && !groupHasAlert(group);
  const phoneCollapsedGroups = createMemo(() =>
    groups().filter((group, index) => index > 0 && !groupHasAlert(group)),
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
      <div
        ref={regionRef}
        tabIndex={-1}
        data-testid="guest-history-region"
        class="space-y-3 outline-none"
        onPointerDown={(event) => setTouchInspection(event.pointerType === 'touch')}
        onPointerMove={(event) => {
          if (event.pointerType === 'mouse') setTouchInspection(false);
        }}
      >
        <div class="flex flex-wrap items-start justify-end gap-2">
          <div
            class="min-w-0 flex-1"
            role="status"
            aria-label="History refresh status"
            aria-live="polite"
            aria-atomic="true"
          >
            <Show when={!locked() && historyQuery.error()}>
              <InlineNotice tone={hasHistoryPoints() ? 'warning' : 'danger'}>
                {getAPIReadAccessErrorMessage(historyQuery.error()) ||
                  (hasHistoryPoints()
                    ? 'History refresh failed. Showing previously loaded history.'
                    : 'Failed to load history data')}
              </InlineNotice>
            </Show>
          </div>
          <Show when={props.onRangeChange}>
            {(onRangeChange) => (
              <GuestDrawerHistoryRangeSelect range={props.range} onRangeChange={onRangeChange()} />
            )}
          </Show>
          {/* The charts poll every 30 seconds, so a manual refresh is offered
              only to recover from a failed read. */}
          <Show when={!locked() && historyQuery.error()}>
            {(_error) => {
              onCleanup(() => {
                retryHadFocus = retryRef !== undefined && document.activeElement === retryRef;
              });
              return (
                <Button
                  ref={retryRef}
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
                  Retry history
                </Button>
              );
            }}
          </Show>
        </div>
        <Show
          when={!locked()}
          fallback={
            <div class="rounded-xs border border-border bg-surface p-5 text-sm text-muted">
              {formatHistoryRangeOptionLabel(props.range)} history requires a higher license plan.
              This instance's plan retains {maxHistoryDays()} days.
            </div>
          }
        >
          <Show when={!historyQuery.error() || hasHistoryPoints()}>
            {/* A reply that is not stored history says so once, above its charts. */}
            <Show when={hasHistoryPoints() && getHistorySourceNotice(historyQuery.value().source)}>
              {(notice) => (
                <p class="text-xs text-muted" data-testid="history-source-notice">
                  {notice().detail}
                </p>
              )}
            </Show>
            <div
              id={gridId}
              class={`grid gap-3 md:grid-cols-2 ${groups().length > 3 ? 'xl:grid-cols-4' : 'xl:grid-cols-3'}`}
            >
              <For each={groups()}>
                {(group, index) => (
                  <GuestDrawerHistoryGroupChart
                    alerts={props.alerts}
                    hiddenOnPhone={isPhoneCollapsed(group, index())}
                    group={group}
                    loading={historyQuery.loading() && !historyQuery.resolvedOnce()}
                    metrics={metrics()}
                    currentMetrics={props.currentMetrics}
                    deferredMetrics={props.deferredMetrics}
                    range={props.range}
                    source={historyQuery.value().source}
                    sourceKey={`${props.target?.resourceType}:${props.target?.resourceId}:${props.range}`}
                    timeBounds={timeBounds()}
                    touchInspection={touchInspection()}
                  />
                )}
              </For>
            </div>
            <PhoneChartsToggle
              hiddenLabels={phoneCollapsedGroups().map((group) => group.label)}
              expanded={showAllOnPhone()}
              onToggle={() => setShowAllOnPhone((open) => !open)}
              controls={gridId}
            />
          </Show>
        </Show>
      </div>
    </Show>
  );
};
