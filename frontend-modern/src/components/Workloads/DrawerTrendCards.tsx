import { For, Show, createMemo, type Component } from 'solid-js';

import type { AggregatedMetricPoint } from '@/api/charts';
import { formatHistoryChartTooltipValue } from '@/components/shared/historyChartModel';

import {
  DRAWER_TREND_RANGE,
  getGuestDrawerHistoryScale,
  normalizeGuestDrawerHistoryPoints,
  type GuestDrawerHistoryDeferredMetric,
  type GuestDrawerHistoryTarget,
} from './guestDrawerModel';
import {
  createGuestDrawerHistoryQuery,
  createGuestDrawerHistorySource,
} from './guestDrawerHistoryQuery';

export interface DrawerTrendCardConfig {
  /** Metrics-store key, shared with the History tab's series. */
  metric: string;
  label: string;
  unit: '%' | 'C';
  color: string;
  /** Live reading; a current value is never drawn as a trend point. */
  current?: number;
  /** Set when the live reading is not current, e.g. during a backup. */
  deferred?: GuestDrawerHistoryDeferredMetric;
  /** Short secondary fact, e.g. "3.42 of 4.00 GB". */
  detail?: string;
  /** Severity of an active alert on this metric. */
  alertSeverity?: 'warning' | 'critical';
  /** The active alert's trigger, drawn as a dashed line. */
  alertThreshold?: number;
}

interface DrawerTrendCardsProps {
  target: GuestDrawerHistoryTarget | null;
  /** False while another drawer tab is showing, so hidden cards do not poll. */
  enabled: boolean;
  cards: DrawerTrendCardConfig[];
}

const PLOT_WIDTH = 240;
const PLOT_HEIGHT = 48;

const formatValue = (value: number, unit: string): string =>
  formatHistoryChartTooltipValue(value, unit);

const buildPath = (
  points: readonly AggregatedMetricPoint[],
  scale: { minValue: number; maxValue: number },
  startTime: number,
  endTime: number,
): string => {
  if (points.length < 2) return '';
  const timeSpan = Math.max(1, endTime - startTime);
  const valueSpan = Math.max(1, scale.maxValue - scale.minValue);
  return points
    .map((point, index) => {
      const x = ((point.timestamp - startTime) / timeSpan) * PLOT_WIDTH;
      const bounded = Math.min(Math.max(point.value, scale.minValue), scale.maxValue);
      const y = (1 - (bounded - scale.minValue) / valueSpan) * PLOT_HEIGHT;
      return `${index === 0 ? 'M' : 'L'}${x.toFixed(1)},${y.toFixed(1)}`;
    })
    .join(' ');
};

const thresholdY = (value: number, scale: { minValue: number; maxValue: number }): number => {
  const valueSpan = Math.max(1, scale.maxValue - scale.minValue);
  const bounded = Math.min(Math.max(value, scale.minValue), scale.maxValue);
  return (1 - (bounded - scale.minValue) / valueSpan) * PLOT_HEIGHT;
};

const cardBorderClass = (severity: DrawerTrendCardConfig['alertSeverity']): string => {
  if (severity === 'critical') return 'border-red-400 dark:border-red-700';
  if (severity === 'warning') return 'border-amber-400 dark:border-amber-600';
  return 'border-border';
};

const valueToneClass = (severity: DrawerTrendCardConfig['alertSeverity']): string => {
  if (severity === 'critical') return 'text-red-700 dark:text-red-300';
  if (severity === 'warning') return 'text-amber-700 dark:text-amber-300';
  return 'text-base-content';
};

// The drawer Overview's 24h trend for the metrics its table row shows. It reads
// the same query as the History tab, so a trend here and the History tab's 24h
// chart are one observation set.
const DrawerTrendCards: Component<DrawerTrendCardsProps> = (props) => {
  const source = createGuestDrawerHistorySource(
    () => props.target,
    () => DRAWER_TREND_RANGE,
    () => props.enabled,
  );
  const query = createGuestDrawerHistoryQuery(source);
  const metrics = createMemo(() => query.value().metrics ?? {});
  const historyWindow = createMemo(() => {
    const value = query.value();
    return value.end > value.start ? { start: value.start, end: value.end } : null;
  });

  return (
    <Show when={props.target && props.cards.length > 0}>
      <ul
        data-testid="drawer-trend-cards"
        aria-label="Last 24 hours"
        class={`grid gap-2 sm:grid-cols-2 ${props.cards.length > 3 ? 'xl:grid-cols-4' : 'xl:grid-cols-3'}`}
      >
        <For each={props.cards}>
          {(card) => {
            const points = createMemo(() =>
              normalizeGuestDrawerHistoryPoints(metrics()[card.metric], card.unit),
            );
            // An alert trigger outside the observed range widens the scale,
            // so the dashed line sits at its real value, not clamped to an edge.
            const scale = createMemo(() =>
              getGuestDrawerHistoryScale(
                [
                  { points: points() },
                  ...(typeof card.alertThreshold === 'number' && points().length > 0
                    ? [
                        {
                          points: [
                            {
                              timestamp: points()[0].timestamp,
                              value: card.alertThreshold,
                              min: card.alertThreshold,
                              max: card.alertThreshold,
                            },
                          ],
                        },
                      ]
                    : []),
                ],
                card.unit,
              ),
            );
            const bounds = createMemo(() => {
              const list = points();
              const w = historyWindow();
              const first = list[0]?.timestamp;
              const last = list[list.length - 1]?.timestamp;
              if (first === undefined || last === undefined) return null;
              return {
                start: Math.min(w?.start ?? first, first),
                end: Math.max(w?.end ?? last, last),
              };
            });
            const path = createMemo(() => {
              const b = bounds();
              return b ? buildPath(points(), scale(), b.start, b.end) : '';
            });
            const range = createMemo(() => {
              const list = points();
              if (list.length === 0) return null;
              let low = Infinity;
              let high = -Infinity;
              // Buckets carry their own extremes; a spike inside an averaged
              // bucket must still widen the stated range.
              for (const point of list) {
                low = Math.min(low, point.min ?? point.value);
                high = Math.max(high, point.max ?? point.value);
              }
              return { low, high };
            });
            const displayValue = createMemo(() => {
              if (typeof card.current === 'number') return formatValue(card.current, card.unit);
              if (card.deferred && typeof card.deferred.lastKnownValue === 'number') {
                return formatValue(card.deferred.lastKnownValue, card.unit);
              }
              return '—';
            });
            const valueQualifier = () => {
              if (typeof card.current === 'number') return '';
              if (card.deferred?.valueLabel) return card.deferred.valueLabel;
              if (card.deferred && typeof card.deferred.lastKnownValue === 'number') {
                return 'last known';
              }
              return card.deferred ? 'not current' : '';
            };
            const rangeLabel = () => {
              const r = range();
              if (!r) return null;
              if (points().length === 1) return '24h: one reading so far';
              const low = formatValue(r.low, card.unit);
              const high = formatValue(r.high, card.unit);
              return low === high ? `24h: steady at ${low}` : `24h: ${low}–${high}`;
            };
            const ariaLabel = () => {
              const r = range();
              const trend = r
                ? `, ${formatValue(r.low, card.unit)} to ${formatValue(r.high, card.unit)} over 24 hours`
                : '';
              const qualifier = valueQualifier() ? `${valueQualifier()} ` : '';
              return `${card.label} ${qualifier}${displayValue()}${trend}`;
            };
            return (
              <li
                data-testid={`drawer-trend-card-${card.metric}`}
                class={`flex min-w-0 flex-col gap-1.5 rounded-md border bg-surface px-3 py-2.5 ${cardBorderClass(card.alertSeverity)}`}
              >
                <div class="flex items-baseline justify-between gap-2">
                  <span class="text-xs text-muted">{card.label}</span>
                  <span class="flex items-baseline gap-1.5">
                    <Show when={valueQualifier()}>
                      <span class="text-[10px] text-muted">{valueQualifier()}</span>
                    </Show>
                    <span
                      class={`text-lg font-semibold tabular-nums leading-none ${valueToneClass(card.alertSeverity)}`}
                    >
                      {displayValue()}
                    </span>
                  </span>
                </div>
                <Show
                  when={path()}
                  fallback={
                    <div class="flex h-12 items-center text-[11px] text-muted">
                      {query.loading() && !query.resolvedOnce()
                        ? 'Loading trend…'
                        : query.error()
                          ? 'Trend unavailable'
                          : 'Collecting history'}
                    </div>
                  }
                >
                  <svg
                    class="block h-12 w-full"
                    viewBox={`0 0 ${PLOT_WIDTH} ${PLOT_HEIGHT}`}
                    preserveAspectRatio="none"
                    role="img"
                    aria-label={ariaLabel()}
                  >
                    <Show when={typeof card.alertThreshold === 'number'}>
                      <line
                        data-testid="drawer-trend-threshold"
                        x1="0"
                        x2={PLOT_WIDTH}
                        y1={thresholdY(card.alertThreshold!, scale())}
                        y2={thresholdY(card.alertThreshold!, scale())}
                        stroke="currentColor"
                        class={
                          card.alertSeverity === 'critical' ? 'text-red-500' : 'text-amber-500'
                        }
                        stroke-width="1"
                        stroke-dasharray="4 4"
                        vector-effect="non-scaling-stroke"
                      />
                    </Show>
                    <path
                      d={path()}
                      fill="none"
                      stroke={card.color}
                      stroke-width="1.5"
                      stroke-linejoin="round"
                      vector-effect="non-scaling-stroke"
                    />
                  </svg>
                </Show>
                <Show when={path() && query.error()}>
                  <p class="text-[11px] leading-snug text-amber-700 dark:text-amber-300">
                    Refresh failed. Showing the last loaded trend.
                  </p>
                </Show>
                <div class="flex flex-wrap justify-between gap-x-2 text-[11px] text-muted">
                  <span>{rangeLabel() ?? ''}</span>
                  <Show when={card.detail}>
                    <span class="tabular-nums">{card.detail}</span>
                  </Show>
                </div>
                <Show when={card.deferred?.message}>
                  <p class="text-[11px] leading-snug text-muted">{card.deferred!.message}</p>
                </Show>
              </li>
            );
          }}
        </For>
      </ul>
    </Show>
  );
};

export default DrawerTrendCards;
