import {
  For,
  Show,
  createEffect,
  createMemo,
  createSignal,
  createUniqueId,
  on,
  onCleanup,
  type Component,
} from 'solid-js';

import { type AggregatedMetricPoint, type HistoryTimeRange } from '@/api/charts';
import { Button } from '@/components/shared/Button';
import { FormSelect } from '@/components/shared/FormSelect';
import { InlineNotice } from '@/components/shared/InlineNotice';
import { LoadingSpinner } from '@/components/shared/LoadingSpinner';
import { PhoneChartsToggle } from '@/components/shared/PhoneChartsToggle';
import {
  formatHistoryChartTooltipValue,
  formatHistoryChartTimeLabel,
  formatHistoryChartTimeLabelForSpan,
  formatHistoryWindowEndpointLabel,
  formatHistoryWindowSummary,
  getHistoryReadoutLabel,
  getHistorySourceNotice,
  type HistoryReadoutSource,
  withoutRepeatedHistoryAxisLabels,
} from '@/components/shared/historyChartModel';
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
import { GuestDrawerHistoryRangeSelect } from './GuestDrawerHistoryRangeSelect';

export { GuestDrawerHistoryRangeSelect };

/** An active alert on one charted metric; a numeric trigger draws a dashed line. */
export interface GuestDrawerHistoryAlert {
  severity: 'warning' | 'critical';
  value?: number;
}

interface GuestDrawerHistoryProps {
  currentMetrics?: Record<string, number | null | undefined>;
  /** False when the owning resource no longer supplies current readings. */
  liveReadingsAvailable?: boolean;
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

interface HistoryBrushPreview {
  originGroupId: string;
  startTime: number;
  endTime: number;
  tooShort: boolean;
}

interface GuestDrawerHistoryGroupChartProps {
  alerts?: Record<string, GuestDrawerHistoryAlert | undefined>;
  brushPreview: HistoryBrushPreview | null;
  clampWindow: GuestDrawerHistoryTimeBounds | null;
  enabled: boolean;
  /** Collapsed below the sm breakpoint until the reader asks for more charts. */
  hiddenOnPhone?: boolean;
  isZoomed: boolean;
  currentMetrics?: Record<string, number | null | undefined>;
  deferredMetrics?: Record<string, GuestDrawerHistoryDeferredMetric | undefined>;
  group: GuestDrawerHistoryGroupConfig;
  loading: boolean;
  metrics: Record<string, AggregatedMetricPoint[] | undefined>;
  onBrushPreviewChange: (preview: HistoryBrushPreview | null) => void;
  onCommitZoom: (window: GuestDrawerHistoryTimeBounds) => void;
  onResetZoom: () => void;
  range: HistoryTimeRange;
  /** Where the reply's readings came from; names the readout's time. */
  source?: HistoryReadoutSource;
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
const HISTORY_BRUSH_THRESHOLD_PX = 6;
const HISTORY_MIN_ZOOM_SPAN_MS = 60_000;
const HISTORY_MAX_ZOOM_SPAN_MS = 365 * 24 * 60 * 60 * 1000;
const HISTORY_SHORT_SPAN_MS = 15 * 60_000;

const pad2 = (value: number): string => String(value).padStart(2, '0');

const formatUtcOffsetLabel = (timestamp: number): string => {
  const offsetMinutes = -new Date(timestamp).getTimezoneOffset();
  const sign = offsetMinutes >= 0 ? '+' : '-';
  const absMinutes = Math.abs(offsetMinutes);
  const hours = Math.floor(absMinutes / 60);
  const mins = absMinutes % 60;
  return `UTC${sign}${pad2(hours)}:${pad2(mins)}`;
};

const toDatetimeLocalInputValue = (timestamp: number): string => {
  const date = new Date(timestamp);
  if (!Number.isFinite(date.getTime())) return '';
  return `${date.getFullYear()}-${pad2(date.getMonth() + 1)}-${pad2(date.getDate())}T${pad2(date.getHours())}:${pad2(date.getMinutes())}:${pad2(date.getSeconds())}`;
};

interface LocalDatetimeCandidate {
  timestamp: number;
  offsetLabel: string;
}

const LOCAL_DATETIME_PATTERN =
  /^(\d{4})-(\d{2})-(\d{2})T(\d{2}):(\d{2})(?::(\d{2})(?:\.(\d{1,3}))?)?$/;
const DST_PROBE_OFFSETS_MS = [
  -7_200_000, -3_600_000, -1_800_000, 0, 1_800_000, 3_600_000, 7_200_000,
];

const resolveLocalDatetimeCandidates = (raw: string): LocalDatetimeCandidate[] => {
  const match = LOCAL_DATETIME_PATTERN.exec(raw.trim());
  if (!match) return [];
  const year = Number(match[1]);
  const month = Number(match[2]);
  const day = Number(match[3]);
  const hour = Number(match[4]);
  const minute = Number(match[5]);
  const second = match[6] ? Number(match[6]) : 0;
  const millisecond = match[7] ? Number(match[7].padEnd(3, '0')) : 0;
  const base = new Date(year, month - 1, day, hour, minute, second, millisecond).getTime();
  if (!Number.isFinite(base)) return [];

  const matchesWallClock = (ms: number): boolean => {
    const d = new Date(ms);
    return (
      d.getFullYear() === year &&
      d.getMonth() === month - 1 &&
      d.getDate() === day &&
      d.getHours() === hour &&
      d.getMinutes() === minute &&
      d.getSeconds() === second &&
      d.getMilliseconds() === millisecond
    );
  };

  const unique = new Map<number, LocalDatetimeCandidate>();
  for (const delta of DST_PROBE_OFFSETS_MS) {
    const candidate = base + delta;
    if (candidate > 0 && matchesWallClock(candidate) && !unique.has(candidate)) {
      unique.set(candidate, {
        timestamp: candidate,
        offsetLabel: formatUtcOffsetLabel(candidate),
      });
    }
  }
  return [...unique.values()].sort((a, b) => a.timestamp - b.timestamp);
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

// Inspecting a time swaps the readout's text, and text that changes width
// re-wraps the legend, moving the chart up and down under the pointer. Each
// readout slot therefore reserves the widest text it can show for the loaded
// range in two invisible pseudo-elements. Under tabular-nums every digit has
// one width, so texts compare by shape and the two widest by estimated advance
// are reserved.
const RESERVED_TEXT_CLASS =
  'inline-grid whitespace-nowrap tabular-nums before:invisible before:[grid-area:1/1] before:content-[attr(data-reserve-a)] after:invisible after:[grid-area:1/1] after:content-[attr(data-reserve-b)]';
const NARROW_GLYPHS = new Set('.,:/- ilIjtfr');
const WIDE_GLYPHS = new Set('%@MWmw');

const estimateTextAdvance = (text: string): number =>
  [...text].reduce(
    (sum, glyph) => sum + (NARROW_GLYPHS.has(glyph) ? 0.3 : WIDE_GLYPHS.has(glyph) ? 0.85 : 0.6),
    0,
  );

const getReservedTextShapes = (texts: readonly string[]): [string, string] => {
  const shapes = [...new Set(texts.map((text) => text.replace(/\d/g, '0')))].sort(
    (a, b) => estimateTextAdvance(b) - estimateTextAdvance(a),
  );
  return [shapes[0] ?? '', shapes[1] ?? shapes[0] ?? ''];
};

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

type LiveReadingQualifier = 'current' | 'last known' | 'freshness unknown';

interface LiveReading {
  value: number | undefined;
  deferred: GuestDrawerHistoryDeferredMetric | undefined;
  qualifier: LiveReadingQualifier | null;
}

// The resource's own reading for one series: current, or a deferred
// last-known or freshness-unknown value with its explanation. The chart
// readouts and the strip shown when no chart can be drawn both use it, so a
// retained reading is qualified the same way in either place.
const getLiveReading = (
  metric: string,
  currentMetrics: Record<string, number | null | undefined> | undefined,
  deferredMetrics: Record<string, GuestDrawerHistoryDeferredMetric | undefined> | undefined,
): LiveReading => {
  const deferred = deferredMetrics?.[metric];
  const raw = deferred ? deferred.lastKnownValue : currentMetrics?.[metric];
  const value = typeof raw === 'number' && Number.isFinite(raw) ? raw : undefined;
  return {
    value,
    deferred,
    qualifier:
      value === undefined
        ? null
        : !deferred
          ? 'current'
          : deferred.valueLabel === 'freshness unknown'
            ? 'freshness unknown'
            : 'last known',
  };
};

interface GuestDrawerLiveReadingsProps {
  currentMetrics?: Record<string, number | null | undefined>;
  deferredMetrics?: Record<string, GuestDrawerHistoryDeferredMetric | undefined>;
  groups: GuestDrawerHistoryGroupConfig[];
}

// Shown in place of the charts when none can be drawn (the first read failed
// or the range is beyond the plan), so CPU and memory stay visible: they
// belong to the resource, not to its stored history.
const GuestDrawerLiveReadings: Component<GuestDrawerLiveReadingsProps> = (props) => {
  const descriptionId = `live-readings-${createUniqueId()}`;
  const items = createMemo(() => {
    const seen = new Set<string>();
    return props.groups.flatMap((group) =>
      group.series.flatMap((series) => {
        if (seen.has(series.metric)) return [];
        seen.add(series.metric);
        const reading = getLiveReading(series.metric, props.currentMetrics, props.deferredMetrics);
        return reading.value !== undefined || reading.deferred ? [{ ...series, ...reading }] : [];
      }),
    );
  });
  return (
    <Show when={items().length > 0}>
      <section
        class="rounded-xs border border-border bg-surface p-2.5"
        data-testid="guest-history-live-readings"
      >
        <h4 class="mb-2 text-xs font-semibold uppercase tracking-wide text-muted">
          Latest readings
        </h4>
        <div class="flex flex-wrap gap-x-4 gap-y-1 text-xs text-muted">
          <For each={items()}>
            {(item, index) => (
              <span
                class="inline-flex items-center gap-1"
                data-live-reading={item.metric}
                data-live-reading-qualifier={item.qualifier ?? undefined}
                aria-describedby={item.deferred ? `${descriptionId}-${index()}` : undefined}
              >
                <svg aria-hidden="true" class="h-2.5 w-2.5 shrink-0" viewBox="0 0 10 10">
                  <circle cx="5" cy="5" r="4" fill={item.color} />
                </svg>
                <span class="font-medium text-base-content">{item.label}</span>
                <span class="tabular-nums text-base-content">
                  {item.value !== undefined
                    ? formatHistoryChartTooltipValue(item.value, item.unit)
                    : '-'}
                </span>
                <Show when={item.qualifier}>{(qualifier) => <span>{qualifier()}</span>}</Show>
              </span>
            )}
          </For>
        </div>
        <For each={items()}>
          {(item, index) => (
            <Show when={item.deferred}>
              {(deferred) => (
                <p
                  id={`${descriptionId}-${index()}`}
                  class="mt-2 text-xs leading-relaxed text-muted"
                  data-history-deferred={item.metric}
                >
                  {item.label} reading: {deferred().message}
                </p>
              )}
            </Show>
          )}
        </For>
      </section>
    </Show>
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
    const origin =
      props.source === 'native'
        ? 'device'
        : props.source === 'mixed' || props.source === 'memory'
          ? 'recorded'
          : 'stored';
    return `${count} ${origin} observation ${count === 1 ? 'time' : 'times'}. ${inspectionValueText()}`;
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
  // While a time is inspected, name a bucket's peak when it rises a visible
  // distance (a tenth of the plot) above the average the line shows. The label
  // sits on the plot at the spike, beside the cursor, so it never reflows the
  // readout. Labels for several series are spaced apart and kept in the plot.
  const peakLabels = createMemo(() => {
    const currentScale = scale();
    const labels = hoveredSeries()
      .filter(
        (item) =>
          (item.point.max ?? item.point.value) - item.point.value >=
          0.1 * (currentScale.maxValue - currentScale.minValue),
      )
      .map((item) => ({
        metric: item.metric,
        color: item.color,
        label: `peak ${formatHistoryChartTooltipValue(item.point.max ?? item.point.value, item.unit)}`,
        left: (item.x / GUEST_DRAWER_HISTORY_CHART_WIDTH) * 100,
        top:
          (getGuestDrawerHistoryY(item.point.max ?? item.point.value, currentScale) /
            GUEST_DRAWER_HISTORY_CHART_HEIGHT) *
          100,
      }))
      .sort((a, b) => a.top - b.top);
    const gap = 14;
    const highest = (GUEST_DRAWER_HISTORY_PLOT_TOP / GUEST_DRAWER_HISTORY_CHART_HEIGHT) * 100;
    const lowest =
      ((GUEST_DRAWER_HISTORY_CHART_HEIGHT - GUEST_DRAWER_HISTORY_PLOT_BOTTOM) /
        GUEST_DRAWER_HISTORY_CHART_HEIGHT) *
      100;
    labels.forEach((label, index) => {
      label.top = Math.max(label.top, highest, index > 0 ? labels[index - 1].top + gap : highest);
    });
    const overflow = (labels.at(-1)?.top ?? lowest) - lowest;
    return labels.map((label) => ({
      ...label,
      top: overflow > 0 ? Math.max(highest, label.top - overflow) : label.top,
      // Past the middle the label reads leftward so it stays inside the panel.
      flip: label.left > 55,
    }));
  });
  // Every text each series' value slot can show across the loaded range.
  const valueReserves = createMemo(
    () =>
      new Map(
        series().map((item) => {
          const labels = new Map(
            item.points.map((point) => [
              point.timestamp,
              getGuestDrawerHistoryValueLabel([point], item.unit),
            ]),
          );
          return [
            item.metric,
            getReservedTextShapes(
              observationTimes().map((timestamp) => labels.get(timestamp) ?? 'no reading'),
            ),
          ] as const;
        }),
      ),
  );
  const crossesMidnight = createMemo(() => {
    const rangeBounds = bounds();
    if (!rangeBounds) return false;
    const startDate = new Date(rangeBounds.startTime);
    const endDate = new Date(rangeBounds.endTime);
    return (
      startDate.getFullYear() !== endDate.getFullYear() ||
      startDate.getMonth() !== endDate.getMonth() ||
      startDate.getDate() !== endDate.getDate()
    );
  });
  const formatObservationTime = (timestamp: number): string => {
    const rangeBounds = bounds();
    if (props.isZoomed && rangeBounds) {
      return formatHistoryChartTimeLabelForSpan(
        timestamp,
        rangeBounds.endTime - rangeBounds.startTime,
        crossesMidnight(),
      );
    }
    return formatHistoryChartTimeLabel(timestamp, props.range);
  };
  const timeReserve = createMemo(() =>
    getReservedTextShapes(
      observationTimes().flatMap((timestamp) => {
        const time = formatObservationTime(timestamp);
        return [false, true].map(
          (inspected) => `${getHistoryReadoutLabel(props.source, inspected)} · ${time}`,
        );
      }),
    ),
  );
  const effectiveCurrentMetrics = () => (props.isZoomed ? undefined : props.currentMetrics);
  const effectiveDeferredMetrics = () => (props.isZoomed ? undefined : props.deferredMetrics);
  const displaySeries = createMemo(() =>
    series().map((item, index) => {
      const timestamp = readoutTimestamp();
      const point = item.points.find((point) => point.timestamp === timestamp);
      const reading = getLiveReading(
        item.metric,
        effectiveCurrentMetrics(),
        effectiveDeferredMetrics(),
      );
      const isFallback = !props.isZoomed && timestamp === null && reading.value !== undefined;
      const alert = props.alerts?.[item.metric];
      return {
        ...item,
        // Names the dashed line: its metric's reading carries the trigger.
        alertLabel:
          alert && typeof alert.value === 'number' && Number.isFinite(alert.value)
            ? `alert at ${formatHistoryChartTooltipValue(alert.value, item.unit)}`
            : null,
        reserve: valueReserves().get(item.metric) ?? ['', ''],
        deferred: reading.deferred,
        descriptionId: `${inspectionId}-deferred-${index}`,
        isCurrent: isFallback && reading.qualifier === 'current',
        isLastKnown: isFallback && reading.qualifier === 'last known',
        isUnknown: isFallback && reading.qualifier === 'freshness unknown',
        valueLabel: point
          ? getGuestDrawerHistoryValueLabel([point], item.unit)
          : // A stored time where this series has no reading says so in place,
            // rather than through a note that appears and disappears (and moves
            // the chart) as the inspected time changes.
            timestamp !== null
            ? 'no reading'
            : reading.value !== undefined
              ? formatHistoryChartTooltipValue(reading.value, item.unit)
              : '-',
      };
    }),
  );
  const readoutTime = createMemo(() => {
    const timestamp = readoutTimestamp();
    if (timestamp === null) return null;
    const date = new Date(timestamp);
    return {
      label: formatObservationTime(timestamp),
      dateTime: date.toISOString(),
      fullLabel: date.toLocaleString(),
    };
  });

  const plotRight = GUEST_DRAWER_HISTORY_CHART_WIDTH - GUEST_DRAWER_HISTORY_PLOT_RIGHT;
  const plotWidth = plotRight - GUEST_DRAWER_HISTORY_PLOT_LEFT;
  const plotHeight =
    GUEST_DRAWER_HISTORY_CHART_HEIGHT -
    GUEST_DRAWER_HISTORY_PLOT_TOP -
    GUEST_DRAWER_HISTORY_PLOT_BOTTOM;

  interface ActiveBrushDrag {
    pointerId: number;
    startClientX: number;
    startSvgX: number;
    bounds: GuestDrawerHistoryTimeBounds;
    clampBounds: GuestDrawerHistoryTimeBounds;
    active: boolean;
  }

  let plotSvgRef: SVGSVGElement | undefined;
  let activeDrag: ActiveBrushDrag | null = null;

  const clientXToSvgX = (clientX: number, rect: DOMRect): number =>
    ((clientX - rect.left) / rect.width) * GUEST_DRAWER_HISTORY_CHART_WIDTH;

  const svgXToTimestamp = (svgX: number, rangeBounds: GuestDrawerHistoryTimeBounds): number => {
    const clampedX = clampNumber(svgX, GUEST_DRAWER_HISTORY_PLOT_LEFT, plotRight);
    const ratio = (clampedX - GUEST_DRAWER_HISTORY_PLOT_LEFT) / Math.max(1, plotWidth);
    return rangeBounds.startTime + ratio * (rangeBounds.endTime - rangeBounds.startTime);
  };

  const computeDragSelection = (currentSvgX: number, drag: ActiveBrushDrag) => {
    const rawA = svgXToTimestamp(drag.startSvgX, drag.bounds);
    const rawB = svgXToTimestamp(currentSvgX, drag.bounds);
    const sortedStart = Math.min(rawA, rawB);
    const sortedEnd = Math.max(rawA, rawB);
    const startTime = Math.round(
      clampNumber(sortedStart, drag.clampBounds.startTime, drag.clampBounds.endTime),
    );
    const endTime = Math.round(
      clampNumber(sortedEnd, drag.clampBounds.startTime, drag.clampBounds.endTime),
    );
    return {
      startTime,
      endTime,
      tooShort: endTime - startTime < HISTORY_MIN_ZOOM_SPAN_MS,
    };
  };

  const handleBrushKeyDown = (event: KeyboardEvent) => {
    if (event.key !== 'Escape' || !activeDrag) return;
    event.preventDefault();
    event.stopPropagation();
    event.stopImmediatePropagation();
    cancelDrag();
  };

  const cancelDrag = () => {
    if (!activeDrag) return;
    const pointerId = activeDrag.pointerId;
    activeDrag = null;
    window.removeEventListener('keydown', handleBrushKeyDown, true);
    try {
      if (plotSvgRef?.hasPointerCapture?.(pointerId)) {
        plotSvgRef.releasePointerCapture(pointerId);
      }
    } catch {
      // Synthetic pointer events in unit tests do not register a real capture.
    }
    if (props.brushPreview?.originGroupId === props.group.id) {
      props.onBrushPreviewChange(null);
    }
  };

  createEffect(
    on(
      () => [props.sourceKey, props.enabled, bounds()?.startTime, bounds()?.endTime] as const,
      () => {
        // Cancel an in-flight drag if the resource, preset, visibility, or
        // underlying coordinate bounds shift under the pointer.
        cancelDrag();
      },
      { defer: true },
    ),
  );
  onCleanup(() => cancelDrag());

  const handleHoverMove = (event: MouseEvent & { currentTarget: SVGSVGElement }) => {
    if (activeDrag || props.brushPreview !== null) return;
    if (inspectionTimestamp() !== null) return;
    const rangeBounds = bounds();
    if (!rangeBounds) return;

    const rect = event.currentTarget.getBoundingClientRect();
    if (rect.width <= 0) return;

    const pointerX = clientXToSvgX(event.clientX, rect);
    setHoverTimestamp(svgXToTimestamp(pointerX, rangeBounds));
  };

  const handlePointerDown = (event: PointerEvent & { currentTarget: SVGSVGElement }) => {
    if (
      event.pointerType === 'touch' ||
      event.button !== 0 ||
      event.isPrimary === false ||
      props.loading ||
      !hasStoredData()
    ) {
      return;
    }
    const rangeBounds = bounds();
    if (!rangeBounds) return;
    const rect = event.currentTarget.getBoundingClientRect();
    if (rect.width <= 0) return;

    const startSvgX = clientXToSvgX(event.clientX, rect);
    if (startSvgX < GUEST_DRAWER_HISTORY_PLOT_LEFT || startSvgX > plotRight) return;

    event.preventDefault();
    cancelDrag();
    const clampBounds = props.clampWindow
      ? { startTime: props.clampWindow.startTime, endTime: props.clampWindow.endTime }
      : { startTime: rangeBounds.startTime, endTime: rangeBounds.endTime };
    activeDrag = {
      pointerId: event.pointerId,
      startClientX: event.clientX,
      startSvgX,
      bounds: { startTime: rangeBounds.startTime, endTime: rangeBounds.endTime },
      clampBounds,
      active: false,
    };
    try {
      event.currentTarget.setPointerCapture?.(event.pointerId);
    } catch {
      // Synthetic pointer events in unit tests do not register a real capture.
    }
    window.addEventListener('keydown', handleBrushKeyDown, true);
  };

  const handlePointerMove = (event: PointerEvent & { currentTarget: SVGSVGElement }) => {
    if (activeDrag) {
      if (event.pointerId !== activeDrag.pointerId) return;
      setHoverTimestamp(null);
      const rect = event.currentTarget.getBoundingClientRect();
      if (rect.width <= 0) return;
      if (
        !activeDrag.active &&
        Math.abs(event.clientX - activeDrag.startClientX) >= HISTORY_BRUSH_THRESHOLD_PX
      ) {
        activeDrag.active = true;
      }
      if (activeDrag.active) {
        const currentSvgX = clampNumber(
          clientXToSvgX(event.clientX, rect),
          GUEST_DRAWER_HISTORY_PLOT_LEFT,
          plotRight,
        );
        const selection = computeDragSelection(currentSvgX, activeDrag);
        props.onBrushPreviewChange({
          originGroupId: props.group.id,
          ...selection,
        });
      }
      return;
    }
    handleHoverMove(event);
  };

  const handlePointerUp = (event: PointerEvent & { currentTarget: SVGSVGElement }) => {
    if (!activeDrag || event.pointerId !== activeDrag.pointerId) return;
    const drag = activeDrag;
    const rect = event.currentTarget.getBoundingClientRect();
    const selection =
      drag.active && rect.width > 0
        ? computeDragSelection(
            clampNumber(
              clientXToSvgX(event.clientX, rect),
              GUEST_DRAWER_HISTORY_PLOT_LEFT,
              plotRight,
            ),
            drag,
          )
        : null;
    cancelDrag();
    if (
      selection &&
      !selection.tooShort &&
      (selection.startTime !== drag.bounds.startTime || selection.endTime !== drag.bounds.endTime)
    ) {
      props.onCommitZoom({
        startTime: selection.startTime,
        endTime: selection.endTime,
      });
    }
  };

  const brushBox = createMemo(() => {
    const preview = props.brushPreview;
    const rangeBounds = bounds();
    if (!preview || !rangeBounds) return null;
    const x1 = clampNumber(
      getGuestDrawerHistoryX(preview.startTime, rangeBounds.startTime, rangeBounds.endTime),
      GUEST_DRAWER_HISTORY_PLOT_LEFT,
      plotRight,
    );
    const x2 = clampNumber(
      getGuestDrawerHistoryX(preview.endTime, rangeBounds.startTime, rangeBounds.endTime),
      GUEST_DRAWER_HISTORY_PLOT_LEFT,
      plotRight,
    );
    const left = Math.min(x1, x2);
    const right = Math.max(x1, x2);
    const centerPct = clampNumber(
      ((left + right) / 2 / GUEST_DRAWER_HISTORY_CHART_WIDTH) * 100,
      24,
      76,
    );
    return {
      left,
      right,
      width: Math.max(1, right - left),
      centerPct,
      preview,
    };
  });

  return (
    <section
      class={`flex min-h-[154px] flex-col rounded-xs border border-border bg-surface p-2.5 ${props.hiddenOnPhone ? 'max-sm:hidden' : ''}`}
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
              <span
                class={`${RESERVED_TEXT_CLASS} text-[10px] text-muted`}
                data-reserve-a={timeReserve()[0]}
                data-reserve-b={timeReserve()[1]}
              >
                <time
                  class="[grid-area:1/1]"
                  data-testid="guest-history-hover-time"
                  dateTime={time().dateTime}
                  title={time().fullLabel}
                  aria-label={`${getHistoryReadoutLabel(props.source, true)} observation: ${time().fullLabel}`}
                >
                  {getHistoryReadoutLabel(props.source, activeTimestamp() !== null)}
                  {' · '}
                  {time().label}
                </time>
              </span>
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
                <span
                  class={`${RESERVED_TEXT_CLASS} ${alertToneClass(props.alerts?.[item.metric]?.severity)}`}
                  data-reserve-a={item.reserve[0]}
                  data-reserve-b={item.reserve[1]}
                >
                  <span class="[grid-area:1/1]">{item.valueLabel}</span>
                </span>
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
            class="mb-2 text-xs leading-relaxed text-muted"
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
          ref={plotSvgRef}
          aria-label={`${props.group.label} history`}
          aria-describedby={`${inspectionId}-description${gapMessage() ? ` ${inspectionId}-gaps` : ''}`}
          class="absolute inset-0 h-full w-full cursor-crosshair select-none"
          data-testid="guest-history-plot"
          onDblClick={() => {
            if (props.isZoomed) props.onResetZoom();
          }}
          onLostPointerCapture={(event) => {
            if (activeDrag && event.pointerId === activeDrag.pointerId) cancelDrag();
          }}
          onMouseMove={handleHoverMove}
          onPointerCancel={(event) => {
            if (activeDrag && event.pointerId === activeDrag.pointerId) cancelDrag();
          }}
          onPointerDown={handlePointerDown}
          onPointerLeave={() => {
            if (!activeDrag) setHoverTimestamp(null);
          }}
          onPointerMove={handlePointerMove}
          onPointerUp={handlePointerUp}
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
          <Show when={brushBox()}>
            {(box) => (
              <g aria-hidden="true" data-history-brush-selection={props.group.id}>
                <rect
                  x={box().left}
                  y={GUEST_DRAWER_HISTORY_PLOT_TOP}
                  width={box().width}
                  height={plotHeight}
                  class={box().preview.tooShort ? 'fill-amber-500/20' : 'fill-blue-500/20'}
                />
                <line
                  x1={box().left}
                  x2={box().left}
                  y1={GUEST_DRAWER_HISTORY_PLOT_TOP}
                  y2={GUEST_DRAWER_HISTORY_PLOT_TOP + plotHeight}
                  stroke="currentColor"
                  stroke-width="1.5"
                  class={box().preview.tooShort ? 'text-amber-500' : 'text-blue-500'}
                  vector-effect="non-scaling-stroke"
                />
                <line
                  x1={box().right}
                  x2={box().right}
                  y1={GUEST_DRAWER_HISTORY_PLOT_TOP}
                  y2={GUEST_DRAWER_HISTORY_PLOT_TOP + plotHeight}
                  stroke="currentColor"
                  stroke-width="1.5"
                  class={box().preview.tooShort ? 'text-amber-500' : 'text-blue-500'}
                  vector-effect="non-scaling-stroke"
                />
              </g>
            )}
          </Show>
          <Show
            when={props.brushPreview === null && hoverX() !== null && hoveredSeries().length > 0}
          >
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
        <Show
          when={
            brushBox() && brushBox()!.preview.originGroupId === props.group.id ? brushBox() : null
          }
        >
          {(box) => (
            <span
              aria-hidden="true"
              data-testid="guest-history-brush-preview"
              class="pointer-events-none absolute top-1 z-10 -translate-x-1/2 whitespace-nowrap rounded-xs border border-border bg-surface/95 px-1.5 py-0.5 text-[10px] leading-none tabular-nums text-base-content shadow-xs"
              style={{ left: `${box().centerPct}%` }}
            >
              {formatHistoryWindowSummary(box().preview.startTime, box().preview.endTime)}
              {box().preview.tooShort ? ' (min 1m)' : ''}
            </span>
          )}
        </Show>
        {/* The slider's value text states each reading's range, so these
            labels are for sighted pointer readers only. */}
        <For each={props.brushPreview === null ? peakLabels() : []}>
          {(peak) => (
            <span
              aria-hidden="true"
              data-history-peak={peak.metric}
              class={`pointer-events-none absolute inline-flex -translate-y-1/2 items-center gap-1 whitespace-nowrap rounded-xs bg-surface/95 px-1 py-0.5 text-[10px] leading-none tabular-nums text-base-content shadow-xs ${
                peak.flip ? '-translate-x-[calc(100%+6px)]' : 'translate-x-1.5'
              }`}
              style={{ left: `${peak.left}%`, top: `${peak.top}%` }}
            >
              <svg class="h-2 w-2 shrink-0" viewBox="0 0 10 10">
                <circle cx="5" cy="5" r="4" fill={peak.color} />
              </svg>
              {peak.label}
            </span>
          )}
        </For>
        <Show when={!hasStoredData() && !props.loading}>
          <div class="pointer-events-none absolute inset-x-8 inset-y-2 flex items-center justify-center rounded-xs text-xs text-muted">
            No stored history in this range
          </div>
        </Show>
        <Show when={props.loading}>
          <div class="pointer-events-none absolute inset-x-8 inset-y-2 flex items-center justify-center rounded-xs text-xs text-muted">
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
                  {formatHistoryWindowEndpointLabel(
                    endpoint.timestamp,
                    props.isZoomed &&
                      window().endTime - window().startTime <= HISTORY_SHORT_SPAN_MS,
                  )}
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
            Use arrow keys, Home or End to inspect{' '}
            {props.source === 'native' || props.source === 'mixed' || props.source === 'memory'
              ? 'readings'
              : 'stored observations'}
            .
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
  const presetScopeKey = createMemo(
    () => `${props.target?.resourceType ?? ''}:${props.target?.resourceId ?? ''}:${props.range}`,
  );
  const [scopedZoom, setScopedZoom] = createSignal<{
    scopeKey: string;
    window: GuestDrawerHistoryTimeBounds;
  } | null>(null);
  const zoomWindow = createMemo<GuestDrawerHistoryTimeBounds | null>(() => {
    const current = scopedZoom();
    return current && current.scopeKey === presetScopeKey() ? current.window : null;
  });
  const [brushPreview, setBrushPreview] = createSignal<HistoryBrushPreview | null>(null);
  const [windowAnnouncement, setWindowAnnouncement] = createSignal('');
  const [windowEditorOpen, setWindowEditorOpen] = createSignal(false);
  const [windowDraftStart, setWindowDraftStart] = createSignal('');
  const [windowDraftEnd, setWindowDraftEnd] = createSignal('');
  const [windowStartOffset, setWindowStartOffset] = createSignal('');
  const [windowEndOffset, setWindowEndOffset] = createSignal('');
  const [windowInitialBounds, setWindowInitialBounds] = createSignal<{
    startMs: number;
    endMs: number;
    startText: string;
    endText: string;
    startOffset: string;
    endOffset: string;
  } | null>(null);
  const [windowDraftError, setWindowDraftError] = createSignal<string | null>(null);
  const gridId = `history-groups-${createUniqueId()}`;
  const windowEditorId = `history-window-editor-${createUniqueId()}`;
  const windowStartInputId = `history-window-start-${createUniqueId()}`;
  const windowEndInputId = `history-window-end-${createUniqueId()}`;
  const historySource = createGuestDrawerHistorySource(
    () => props.target,
    () => props.range,
    () => props.enabled !== false,
    () => props.liveReadingsAvailable !== false,
    zoomWindow,
  );
  const historyQuery = createGuestDrawerHistoryQuery(historySource);
  // A successful retry removes the focused Retry button with the error. Keep
  // keyboard focus in the history region instead of dropping it to the page.
  // Focus is read when the button's branch is disposed, before its node leaves
  // the DOM: Chromium blurs a removed element, so a blur-tracked flag is
  // already cleared by the time the error effect runs.
  let regionRef: HTMLDivElement | undefined;
  let retryRef: HTMLButtonElement | undefined;
  let resetZoomRef: HTMLButtonElement | undefined;
  let selectWindowRef: HTMLButtonElement | undefined;
  let retryHadFocus = false;
  let resetZoomHadFocus = false;
  createEffect(
    on(
      () => historyQuery.error(),
      (error, previous) => {
        if (previous && !error && retryHadFocus) regionRef?.focus();
        retryHadFocus = false;
      },
    ),
  );
  createEffect(
    on(zoomWindow, (current, previous) => {
      if (previous && !current && resetZoomHadFocus) regionRef?.focus();
      resetZoomHadFocus = false;
    }),
  );
  createEffect(
    on(
      presetScopeKey,
      () => {
        setScopedZoom(null);
        setBrushPreview(null);
        setWindowEditorOpen(false);
        setWindowDraftError(null);
        setWindowAnnouncement('');
      },
      { defer: true },
    ),
  );

  const commitZoom = (next: GuestDrawerHistoryTimeBounds) => {
    const startTime = Math.round(next.startTime);
    const endTime = Math.round(next.endTime);
    if (endTime - startTime < HISTORY_MIN_ZOOM_SPAN_MS) return;
    setScopedZoom({
      scopeKey: presetScopeKey(),
      window: { startTime, endTime },
    });
    setBrushPreview(null);
    setWindowEditorOpen(false);
    setWindowDraftError(null);
    setWindowAnnouncement(`Zoomed to ${formatHistoryWindowSummary(startTime, endTime)}.`);
  };

  const resetZoom = () => {
    if (!zoomWindow()) return;
    setScopedZoom(null);
    setBrushPreview(null);
    setWindowEditorOpen(false);
    setWindowDraftError(null);
    setWindowAnnouncement('Zoom reset.');
  };

  const metrics = createMemo(() => historyQuery.value().metrics ?? {});
  const groups = createMemo(() => props.groups ?? GUEST_DRAWER_HISTORY_GROUPS);
  const sourceForGroup = (
    group: GuestDrawerHistoryGroupConfig,
  ): HistoryReadoutSource | undefined => {
    const { source, metricSources } = historyQuery.value();
    if (!metricSources) return source;
    const origins = new Set(
      group.series.map(({ metric }) => metricSources[metric]).filter(Boolean),
    );
    // The time readout belongs to the whole chart, including all of its
    // directions. Don't call it stored if the plotted series disagree.
    return origins.size === 1 ? origins.values().next().value : 'mixed';
  };
  const sourceNotice = () => {
    const { source, metricSources } = historyQuery.value();
    if (!metricSources) return getHistorySourceNotice(source)?.detail;
    // This describes the series we drew, not where each sample might also
    // have been saved.
    const hasMemory = Object.values(metricSources).includes('memory');
    if (metricSources.smart_temp === 'native') {
      return hasMemory
        ? 'Temperature from the device. Some readings from memory.'
        : 'Temperature history comes from the device.';
    }
    return hasMemory ? 'This view includes readings from memory.' : undefined;
  };
  const sourceKey = () => {
    const zoom = zoomWindow();
    return zoom
      ? `${props.target?.resourceType}:${props.target?.resourceId}:${props.range}:custom:${zoom.startTime}:${zoom.endTime}`
      : `${props.target?.resourceType}:${props.target?.resourceId}:${props.range}`;
  };
  const normalizedGroups = createMemo(() =>
    groups().map((group) => ({
      id: group.id,
      series: group.series.map(({ metric, unit }) => ({
        points: normalizeGuestDrawerHistoryPoints(metrics()[metric], unit),
      })),
    })),
  );
  const replyWindow = createMemo<GuestDrawerHistoryTimeBounds | null>(() => {
    const reply = historyQuery.value();
    if (
      Number.isFinite(reply.start) &&
      Number.isFinite(reply.end) &&
      reply.end > reply.start &&
      reply.start > 0
    ) {
      return { startTime: reply.start, endTime: reply.end };
    }
    return null;
  });
  const timeBounds = createMemo(() => {
    const zoom = zoomWindow();
    if (zoom) {
      return getGuestDrawerHistoryRangeBounds(
        normalizedGroups().flatMap(({ series }) => series),
        { start: zoom.startTime, end: zoom.endTime },
        true,
      );
    }
    return getGuestDrawerHistoryRangeBounds(
      normalizedGroups().flatMap(({ series }) => series),
      historyQuery.value(),
    );
  });
  const groupsWithPoints = createMemo(
    () =>
      new Set(
        normalizedGroups()
          .filter(({ series }) => series.some(({ points }) => points.length > 0))
          .map(({ id }) => id),
      ),
  );
  const hasHistoryPoints = () => groupsWithPoints().size > 0;
  const liveCurrentMetrics = () =>
    props.liveReadingsAvailable !== false ? props.currentMetrics : undefined;

  const windowStartCandidates = createMemo(() =>
    resolveLocalDatetimeCandidates(windowDraftStart()),
  );
  const windowEndCandidates = createMemo(() => resolveLocalDatetimeCandidates(windowDraftEnd()));

  const resolveEndpointTimestamp = (
    draftText: string,
    selectedOffset: string,
    candidates: readonly LocalDatetimeCandidate[],
    initialText: string | undefined,
    initialOffset: string | undefined,
    initialMs: number | undefined,
  ): number | null => {
    if (
      initialMs !== undefined &&
      draftText === initialText &&
      (!selectedOffset || selectedOffset === initialOffset)
    ) {
      return initialMs;
    }
    if (candidates.length === 0) return null;
    if (candidates.length === 1) return candidates[0].timestamp;
    const matched = candidates.find((candidate) => candidate.offsetLabel === selectedOffset);
    return (matched ?? candidates[0]).timestamp;
  };

  const openWindowEditor = () => {
    const activeBounds = zoomWindow() ?? replyWindow() ?? timeBounds();
    const startMs = activeBounds ? activeBounds.startTime : Date.now() - 3_600_000;
    const endMs = activeBounds ? activeBounds.endTime : Date.now();
    const startText = toDatetimeLocalInputValue(startMs);
    const endText = toDatetimeLocalInputValue(endMs);
    const startOffset = formatUtcOffsetLabel(startMs);
    const endOffset = formatUtcOffsetLabel(endMs);
    setWindowInitialBounds({ startMs, endMs, startText, endText, startOffset, endOffset });
    setWindowDraftStart(startText);
    setWindowDraftEnd(endText);
    setWindowStartOffset(startOffset);
    setWindowEndOffset(endOffset);
    setWindowDraftError(null);
    setWindowEditorOpen(true);
  };

  const applyWindowEditor = (event: SubmitEvent) => {
    event.preventDefault();
    const initial = windowInitialBounds();
    const startMs = resolveEndpointTimestamp(
      windowDraftStart(),
      windowStartOffset(),
      windowStartCandidates(),
      initial?.startText,
      initial?.startOffset,
      initial?.startMs,
    );
    const endMs = resolveEndpointTimestamp(
      windowDraftEnd(),
      windowEndOffset(),
      windowEndCandidates(),
      initial?.endText,
      initial?.endOffset,
      initial?.endMs,
    );
    if (startMs === null || endMs === null) {
      setWindowDraftError('Enter valid start and end times.');
      return;
    }
    if (endMs <= startMs) {
      setWindowDraftError('End time must be after start time.');
      return;
    }
    const spanMs = endMs - startMs;
    if (spanMs < HISTORY_MIN_ZOOM_SPAN_MS) {
      setWindowDraftError('Select a window of at least 1 minute.');
      return;
    }
    if (spanMs > HISTORY_MAX_ZOOM_SPAN_MS) {
      setWindowDraftError('Window cannot exceed 365 days.');
      return;
    }
    if (endMs > Date.now()) {
      setWindowDraftError('End time cannot be in the future.');
      return;
    }
    commitZoom({ startTime: startMs, endTime: endMs });
    // Closing the form removes the focused Apply button. Move keyboard focus
    // onto Reset zoom (or back to Select time window) rather than document.body.
    queueMicrotask(() => {
      if (resetZoomRef?.isConnected) {
        resetZoomRef.focus();
      } else if (selectWindowRef?.isConnected) {
        selectWindowRef.focus();
      } else {
        regionRef?.focus();
      }
    });
  };

  const inlineDiskActivity = () => {
    const first = groups()[0];
    return props.target?.resourceType === 'disk' && first?.id === 'disk-activity'
      ? first
      : undefined;
  };
  const diskActivityHasReading = createMemo(() => {
    const activity = inlineDiskActivity();
    return Boolean(
      activity &&
      (groupsWithPoints().has(activity.id) ||
        (!zoomWindow() &&
          activity.series.some(({ metric }) => {
            const reading = getLiveReading(metric, liveCurrentMetrics(), props.deferredMetrics);
            return reading.value !== undefined || reading.deferred !== undefined;
          }))),
    );
  });

  // The inline disk drawer usually leads with Activity. If that chart has
  // nothing to report, lead with recorded Temperature or I/O instead. Keep
  // that chart open for this disk and range while it still has observations:
  // a poll must not fold away the chart someone is inspecting.
  const diskPhoneFallback = createMemo<{ key: string; id: string } | undefined>((previous) => {
    if (!inlineDiskActivity()) return;
    const reply = historyQuery.value();
    const zoom = zoomWindow();
    const expectedRange = zoom ? 'custom' : props.range;
    if (
      reply.resourceId !== props.target?.resourceId ||
      reply.resourceType !== props.target?.resourceType ||
      reply.range !== expectedRange ||
      (zoom !== null && (reply.start !== zoom.startTime || reply.end !== zoom.endTime))
    ) {
      return;
    }
    const key = sourceKey();
    const withPoints = groupsWithPoints();
    if (previous?.key === key && withPoints.has(previous.id)) return previous;
    if (diskActivityHasReading()) return;
    const id = ['disk-thermal', 'disk-io'].find((id) => withPoints.has(id));
    return id ? { key, id } : undefined;
  });

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
  const phoneCollapsedGroups = createMemo(() => {
    const fallback = diskPhoneFallback();
    const foldActivity = fallback !== undefined && !diskActivityHasReading();
    const alerted = alertedGroups();
    return groups().filter(
      (group, index) =>
        (index > 0 || foldActivity) && group.id !== fallback?.id && !alerted.has(group.id),
    );
  });
  // A final 401/403 withdraws the whole History panel, readings included; an
  // outage or a locked range leaves the resource's own readings standing.
  const liveReadings = () => (
    <Show when={!zoomWindow() && !getAPIReadAccessErrorMessage(historyQuery.error())}>
      <GuestDrawerLiveReadings
        groups={groups()}
        currentMetrics={liveCurrentMetrics()}
        deferredMetrics={props.deferredMetrics}
      />
    </Show>
  );

  // Keep Select time window available for any authorized target, including when
  // the current preset has no stored readings, so users can jump to older windows.
  const canSelectWindow = createMemo(
    () => Boolean(props.target) && !getAPIReadAccessErrorMessage(historyQuery.error()),
  );
  const isInitialLoading = createMemo(() => historyQuery.loading() && !historyQuery.resolvedOnce());
  const isEnabled = createMemo(() => props.enabled !== false);
  const isZoomed = createMemo(() => zoomWindow() !== null);
  const showToolbar = () =>
    Boolean(props.onRangeChange || historyQuery.error() || isZoomed() || canSelectWindow());

  return (
    <Show
      when={props.target}
      fallback={<div class="py-6 text-center text-sm text-muted">History unavailable</div>}
    >
      <div
        ref={regionRef}
        tabIndex={-1}
        data-testid="guest-history-region"
        class="outline-none"
        onPointerDown={(event) => setTouchInspection(event.pointerType === 'touch')}
        onPointerMove={(event) => {
          if (event.pointerType === 'mouse') setTouchInspection(false);
        }}
      >
        <div
          class="sr-only"
          role="status"
          aria-label="History window status"
          aria-live="polite"
          aria-atomic="true"
        >
          {windowAnnouncement()}
        </div>
        <div
          class={showToolbar() ? 'mb-3 flex flex-wrap items-center justify-end gap-2' : 'sr-only'}
        >
          <div
            class="min-w-0 flex-1"
            role="status"
            aria-label="History refresh status"
            aria-live="polite"
            aria-atomic="true"
          >
            <Show when={historyQuery.error()}>
              <InlineNotice tone={hasHistoryPoints() ? 'warning' : 'danger'}>
                {getAPIReadAccessErrorMessage(historyQuery.error()) ||
                  (hasHistoryPoints()
                    ? 'History refresh failed. Showing previously loaded history.'
                    : 'Failed to load history data')}
              </InlineNotice>
            </Show>
          </div>
          <Show when={zoomWindow()}>
            {(zoom) => {
              onCleanup(() => {
                resetZoomHadFocus =
                  resetZoomRef !== undefined && document.activeElement === resetZoomRef;
              });
              return (
                <div
                  class="flex flex-wrap items-center gap-1.5"
                  data-testid="guest-history-zoom-bar"
                >
                  <span
                    class="rounded-xs border border-border bg-surface-subtle px-2 py-1 text-[11px] tabular-nums text-base-content"
                    data-testid="guest-history-zoom-label"
                  >
                    {formatHistoryWindowSummary(zoom().startTime, zoom().endTime)}
                  </span>
                  <Button
                    ref={resetZoomRef}
                    size="sm"
                    variant="secondary"
                    class="min-h-11 shrink-0 text-[11px] sm:min-h-7 sm:py-0"
                    data-testid="guest-history-reset-zoom"
                    onClick={resetZoom}
                  >
                    Reset zoom
                  </Button>
                </div>
              );
            }}
          </Show>
          <Show when={canSelectWindow()}>
            <Button
              ref={selectWindowRef}
              size="sm"
              variant="ghost"
              class="min-h-11 shrink-0 text-[11px] text-muted hover:text-base-content sm:min-h-7 sm:py-0"
              aria-expanded={windowEditorOpen()}
              aria-controls={windowEditorId}
              data-testid="guest-history-select-window"
              onClick={() => {
                if (windowEditorOpen()) {
                  setWindowEditorOpen(false);
                  setWindowDraftError(null);
                } else {
                  openWindowEditor();
                }
              }}
            >
              Select time window
            </Button>
          </Show>
          <Show when={props.onRangeChange}>
            {(onRangeChange) => (
              <GuestDrawerHistoryRangeSelect range={props.range} onRangeChange={onRangeChange()} />
            )}
          </Show>
          {/* The charts poll every 30 seconds, so a manual refresh is offered
              only to recover from a failed read. */}
          <Show when={historyQuery.error()}>
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
        <Show when={windowEditorOpen()}>
          <form
            id={windowEditorId}
            class="mb-3 rounded-xs border border-border bg-surface p-2.5"
            data-testid="guest-history-window-editor"
            onSubmit={applyWindowEditor}
            onKeyDown={(event) => {
              if (event.key === 'Escape') {
                event.preventDefault();
                event.stopPropagation();
                setWindowEditorOpen(false);
                setWindowDraftError(null);
                selectWindowRef?.focus();
              }
            }}
          >
            <div class="flex flex-wrap items-end gap-2">
              <div class="flex min-w-[180px] flex-1 flex-col gap-1">
                <label for={windowStartInputId} class="text-[11px] text-muted">
                  Start
                </label>
                <input
                  id={windowStartInputId}
                  type="datetime-local"
                  step="any"
                  required
                  value={windowDraftStart()}
                  onInput={(event) => {
                    const nextValue = event.currentTarget.value;
                    setWindowDraftStart(nextValue);
                    const candidates = resolveLocalDatetimeCandidates(nextValue);
                    if (
                      candidates.length > 0 &&
                      !candidates.some((c) => c.offsetLabel === windowStartOffset())
                    ) {
                      setWindowStartOffset(candidates[0].offsetLabel);
                    }
                    setWindowDraftError(null);
                  }}
                  class="min-h-11 rounded-xs border border-border bg-surface px-2 py-1 text-xs tabular-nums text-base-content sm:min-h-7"
                />
                <Show when={windowStartCandidates().length > 1}>
                  <FormSelect
                    density="compact"
                    label="Start timezone offset"
                    labelClass="sr-only"
                    fieldClass="mt-1"
                    selectClass="text-[11px] tabular-nums"
                    value={windowStartOffset()}
                    onChange={(event) => {
                      setWindowStartOffset(event.currentTarget.value);
                      setWindowDraftError(null);
                    }}
                  >
                    <For each={windowStartCandidates()}>
                      {(candidate) => (
                        <option value={candidate.offsetLabel}>{candidate.offsetLabel}</option>
                      )}
                    </For>
                  </FormSelect>
                </Show>
              </div>
              <div class="flex min-w-[180px] flex-1 flex-col gap-1">
                <label for={windowEndInputId} class="text-[11px] text-muted">
                  End
                </label>
                <input
                  id={windowEndInputId}
                  type="datetime-local"
                  step="any"
                  required
                  value={windowDraftEnd()}
                  onInput={(event) => {
                    const nextValue = event.currentTarget.value;
                    setWindowDraftEnd(nextValue);
                    const candidates = resolveLocalDatetimeCandidates(nextValue);
                    if (
                      candidates.length > 0 &&
                      !candidates.some((c) => c.offsetLabel === windowEndOffset())
                    ) {
                      setWindowEndOffset(candidates[0].offsetLabel);
                    }
                    setWindowDraftError(null);
                  }}
                  class="min-h-11 rounded-xs border border-border bg-surface px-2 py-1 text-xs tabular-nums text-base-content sm:min-h-7"
                />
                <Show when={windowEndCandidates().length > 1}>
                  <FormSelect
                    density="compact"
                    label="End timezone offset"
                    labelClass="sr-only"
                    fieldClass="mt-1"
                    selectClass="text-[11px] tabular-nums"
                    value={windowEndOffset()}
                    onChange={(event) => {
                      setWindowEndOffset(event.currentTarget.value);
                      setWindowDraftError(null);
                    }}
                  >
                    <For each={windowEndCandidates()}>
                      {(candidate) => (
                        <option value={candidate.offsetLabel}>{candidate.offsetLabel}</option>
                      )}
                    </For>
                  </FormSelect>
                </Show>
              </div>
              <div class="flex items-center gap-1.5">
                <Button
                  type="submit"
                  size="sm"
                  class="min-h-11 text-[11px] sm:min-h-7 sm:py-0"
                  data-testid="guest-history-window-apply"
                >
                  Apply
                </Button>
                <Button
                  type="button"
                  size="sm"
                  variant="ghost"
                  class="min-h-11 text-[11px] sm:min-h-7 sm:py-0"
                  onClick={() => {
                    setWindowEditorOpen(false);
                    setWindowDraftError(null);
                    selectWindowRef?.focus();
                  }}
                >
                  Cancel
                </Button>
              </div>
            </div>
            <Show when={windowDraftError()}>
              {(message) => (
                <p
                  role="alert"
                  class="mt-2 text-xs text-red-600 dark:text-red-400"
                  data-testid="guest-history-window-error"
                >
                  {message()}
                </p>
              )}
            </Show>
          </form>
        </Show>
        <div class="space-y-3">
          <Show when={!historyQuery.error() || hasHistoryPoints()} fallback={liveReadings()}>
            <Show when={hasHistoryPoints() && sourceNotice()}>
              {(notice) => (
                <p class="text-xs text-muted" data-testid="history-source-notice">
                  {notice()}
                </p>
              )}
            </Show>
            <div
              id={gridId}
              class={`grid gap-3 md:grid-cols-2 ${groups().length > 3 ? 'xl:grid-cols-4' : 'xl:grid-cols-3'}`}
            >
              <For each={groups()}>
                {(group) => (
                  <GuestDrawerHistoryGroupChart
                    alerts={props.alerts}
                    brushPreview={brushPreview()}
                    clampWindow={replyWindow()}
                    enabled={isEnabled()}
                    hiddenOnPhone={!showAllOnPhone() && phoneCollapsedGroups().includes(group)}
                    isZoomed={isZoomed()}
                    group={group}
                    loading={isInitialLoading()}
                    metrics={metrics()}
                    onBrushPreviewChange={setBrushPreview}
                    onCommitZoom={commitZoom}
                    onResetZoom={resetZoom}
                    currentMetrics={liveCurrentMetrics()}
                    deferredMetrics={props.deferredMetrics}
                    range={props.range}
                    source={sourceForGroup(group)}
                    sourceKey={sourceKey()}
                    timeBounds={timeBounds()}
                    touchInspection={touchInspection()}
                  />
                )}
              </For>
            </div>
            {/* Keep collapse after the chart stack, in reach from the final
                chart's inspector and in the same place in empty ranges. */}
            <PhoneChartsToggle
              hiddenLabels={phoneCollapsedGroups().map((group) => group.label)}
              expanded={showAllOnPhone()}
              onToggle={() => setShowAllOnPhone((open) => !open)}
              controls={gridId}
            />
          </Show>
        </div>
      </div>
    </Show>
  );
};
