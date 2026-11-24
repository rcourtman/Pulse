import {
  For,
  Show,
  createEffect,
  createMemo,
  createSignal,
  createUniqueId,
  type Component,
} from 'solid-js';
import ActivityIcon from 'lucide-solid/icons/activity';
import ClockIcon from 'lucide-solid/icons/clock';
import {
  AvailabilityHistoryAPI,
  type AvailabilityHistoryBucket,
  type AvailabilityHistoryRange,
  type AvailabilityHistoryTarget,
} from '@/api/availabilityHistory';
import { Button } from '@/components/shared/Button';
import { FilterSegmentedControl } from '@/components/shared/FilterToolbar';
import { InfoCardFrame } from '@/components/shared/InfoCardFrame';
import {
  AVAILABILITY_HISTORY_RANGE_OPTIONS,
  AVAILABILITY_STATE_PRESENTATION,
  classifyAvailabilityBucket,
  formatAvailabilityDurationSeconds,
  formatAvailabilityObservedDuration,
  formatAvailabilityPercent,
  getAvailabilityHistoryRangeMeta,
  getAvailabilityUptimeVerdict,
  type AvailabilityHistoryBucketState,
} from '@/utils/availabilityHistoryPresentation';

export interface AvailabilityDrawerHistoryProps {
  targetId: string;
  protocol?: string;
  enabled?: boolean;
}

const CHART_WIDTH = 400;
const CHART_HEIGHT = 108;
const PAD_LEFT = 38;
const PAD_RIGHT = 8;
const PAD_TOP = 10;
const PAD_BOTTOM = 10;
const PLOT_WIDTH = CHART_WIDTH - PAD_LEFT - PAD_RIGHT;
const PLOT_HEIGHT = CHART_HEIGHT - PAD_TOP - PAD_BOTTOM;

const formatBucketTimestamp = (iso: string, range: AvailabilityHistoryRange): string => {
  const parsed = Date.parse(iso);
  if (!Number.isFinite(parsed)) return '';
  const date = new Date(parsed);
  if (range === '24h') {
    return date.toLocaleTimeString(undefined, { hour: '2-digit', minute: '2-digit' });
  }
  if (range === '365d') {
    return date.toLocaleDateString(undefined, { month: 'short', day: 'numeric' });
  }
  return date.toLocaleString(undefined, {
    month: 'short',
    day: 'numeric',
    hour: '2-digit',
    minute: '2-digit',
  });
};

const formatAxisTick = (iso: string | undefined, range: AvailabilityHistoryRange): string => {
  if (!iso) return '';
  const parsed = Date.parse(iso);
  if (!Number.isFinite(parsed)) return '';
  const date = new Date(parsed);
  if (range === '24h') {
    return date.toLocaleTimeString(undefined, { hour: '2-digit', minute: '2-digit' });
  }
  return date.toLocaleDateString(undefined, { month: 'short', day: 'numeric' });
};

const niceLatencyCeiling = (maxValue: number): number => {
  if (!Number.isFinite(maxValue) || maxValue <= 0) return 10;
  const padded = maxValue * 1.15;
  if (padded <= 10) return 10;
  if (padded <= 25) return 25;
  if (padded <= 50) return 50;
  if (padded <= 100) return Math.ceil(padded / 10) * 10;
  if (padded <= 500) return Math.ceil(padded / 50) * 50;
  return Math.ceil(padded / 100) * 100;
};

const formatLatencyTick = (ms: number): string =>
  ms >= 1000 ? `${(ms / 1000).toFixed(ms % 1000 === 0 ? 0 : 1)}s` : `${Math.round(ms)} ms`;

interface LatencyGeometry {
  ceiling: number;
  linePaths: string[];
  areaPaths: string[];
  bandPaths: string[];
  points: Array<{
    index: number;
    x: number;
    y: number;
    avg: number;
    min: number;
    max: number;
  } | null>;
}

const buildLatencyGeometry = (buckets: readonly AvailabilityHistoryBucket[]): LatencyGeometry => {
  const count = buckets.length;
  const values: Array<{ avg: number; min: number; max: number } | null> = buckets.map((bucket) => {
    const lat = bucket.latencyMillis;
    if (!lat || !Number.isFinite(lat.average) || lat.average < 0) return null;
    const min = Number.isFinite(lat.min) && lat.min >= 0 ? lat.min : lat.average;
    const max = Number.isFinite(lat.max) && lat.max >= min ? lat.max : lat.average;
    return { avg: lat.average, min, max };
  });

  const maxObserved = values.reduce((acc, item) => (item ? Math.max(acc, item.max) : acc), 0);
  const ceiling = niceLatencyCeiling(maxObserved);
  const xFor = (index: number) =>
    count <= 1 ? PAD_LEFT + PLOT_WIDTH / 2 : PAD_LEFT + (index / (count - 1)) * PLOT_WIDTH;
  const yFor = (ms: number) => PAD_TOP + (1 - Math.min(ms, ceiling) / ceiling) * PLOT_HEIGHT;
  const baseY = PAD_TOP + PLOT_HEIGHT;

  const points: LatencyGeometry['points'] = values.map((item, index) =>
    item
      ? {
          index,
          x: xFor(index),
          y: yFor(item.avg),
          avg: item.avg,
          min: item.min,
          max: item.max,
        }
      : null,
  );

  const linePaths: string[] = [];
  const areaPaths: string[] = [];
  const bandPaths: string[] = [];
  let segment: Array<{ x: number; y: number; yMin: number; yMax: number; hasSpread: boolean }> = [];

  const flush = () => {
    if (segment.length === 0) return;
    if (segment.length === 1) {
      const pt = segment[0];
      const left = Math.max(PAD_LEFT, pt.x - 1.5);
      const right = Math.min(PAD_LEFT + PLOT_WIDTH, pt.x + 1.5);
      linePaths.push(
        `M ${left.toFixed(1)} ${pt.y.toFixed(1)} L ${right.toFixed(1)} ${pt.y.toFixed(1)}`,
      );
      areaPaths.push(
        `M ${left.toFixed(1)} ${baseY.toFixed(1)} L ${left.toFixed(1)} ${pt.y.toFixed(1)} L ${right.toFixed(1)} ${pt.y.toFixed(1)} L ${right.toFixed(1)} ${baseY.toFixed(1)} Z`,
      );
    } else {
      const lineCmds = segment
        .map((pt, i) => `${i === 0 ? 'M' : 'L'} ${pt.x.toFixed(1)} ${pt.y.toFixed(1)}`)
        .join(' ');
      linePaths.push(lineCmds);
      const first = segment[0];
      const last = segment[segment.length - 1];
      areaPaths.push(
        `${lineCmds} L ${last.x.toFixed(1)} ${baseY.toFixed(1)} L ${first.x.toFixed(1)} ${baseY.toFixed(1)} Z`,
      );
      if (segment.some((pt) => pt.hasSpread)) {
        const upper = segment
          .map((pt, i) => `${i === 0 ? 'M' : 'L'} ${pt.x.toFixed(1)} ${pt.yMax.toFixed(1)}`)
          .join(' ');
        const lower = [...segment]
          .reverse()
          .map((pt) => `L ${pt.x.toFixed(1)} ${pt.yMin.toFixed(1)}`)
          .join(' ');
        bandPaths.push(`${upper} ${lower} Z`);
      }
    }
    segment = [];
  };

  for (let index = 0; index < values.length; index++) {
    const item = values[index];
    if (!item) {
      flush();
      continue;
    }
    segment.push({
      x: xFor(index),
      y: yFor(item.avg),
      yMin: yFor(item.min),
      yMax: yFor(item.max),
      hasSpread: item.max - item.min >= 1,
    });
  }
  flush();

  return { ceiling, linePaths, areaPaths, bandPaths, points };
};

const AvailabilityDrawerHistory: Component<AvailabilityDrawerHistoryProps> = (props) => {
  const gradientId = `avail-latency-fill-${createUniqueId()}`;
  const [range, setRange] = createSignal<AvailabilityHistoryRange>('24h');
  const [history, setHistory] = createSignal<AvailabilityHistoryTarget | null>(null);
  const [loading, setLoading] = createSignal(false);
  const [error, setError] = createSignal<string | null>(null);
  const [activeIndex, setActiveIndex] = createSignal<number | null>(null);
  const [reloadTick, setReloadTick] = createSignal(0);

  let requestSeq = 0;

  createEffect(() => {
    const targetId = props.targetId.trim();
    const activeRange = range();
    const enabled = props.enabled !== false;
    void reloadTick();

    if (!enabled || !targetId) return;

    const seq = ++requestSeq;
    setLoading(true);
    setError(null);
    setActiveIndex(null);

    void AvailabilityHistoryAPI.batch([targetId], activeRange)
      .then((response) => {
        if (seq !== requestSeq) return;
        const entry =
          response.targets.find((item) => item.targetId === targetId) ??
          response.targets[0] ??
          null;
        if (entry?.error) {
          setHistory(null);
          setError(entry.error.message || 'Could not load availability history.');
          return;
        }
        setHistory(entry);
      })
      .catch((err) => {
        if (seq !== requestSeq) return;
        setHistory(null);
        setError(err instanceof Error ? err.message : 'Could not load availability history.');
      })
      .finally(() => {
        if (seq !== requestSeq) return;
        setLoading(false);
      });
  });

  const buckets = createMemo(() => history()?.buckets ?? []);
  const summary = createMemo(() => history()?.summary);
  const rangeMeta = createMemo(() => getAvailabilityHistoryRangeMeta(range()));

  const hasObservedHistory = createMemo(() => {
    const s = summary();
    if (!s) return false;
    return s.reachableSeconds + s.unreachableSeconds + s.indeterminateSeconds > 0;
  });

  const bucketStates = createMemo<AvailabilityHistoryBucketState[]>(() =>
    buckets().map((bucket) => classifyAvailabilityBucket(bucket)),
  );

  const revisionBucketIndices = createMemo<Set<number>>(() => {
    const boundaries = history()?.revisionBoundaries ?? [];
    const list = buckets();
    const indices = new Set<number>();
    if (boundaries.length === 0 || list.length === 0) return indices;
    for (const boundary of boundaries) {
      const ts = Date.parse(boundary.at);
      if (!Number.isFinite(ts)) continue;
      for (let i = 0; i < list.length; i++) {
        const start = Date.parse(list[i].start);
        const end = Date.parse(list[i].end);
        if (Number.isFinite(start) && Number.isFinite(end) && ts >= start && ts <= end) {
          indices.add(i);
          break;
        }
      }
    }
    return indices;
  });

  const latencyGeometry = createMemo(() => buildLatencyGeometry(buckets()));
  const hasLatencyPoints = createMemo(() =>
    latencyGeometry().points.some((point) => point !== null),
  );

  const uptimeHeadline = createMemo(() => {
    const s = summary();
    if (!s) return null;
    const verdict = getAvailabilityUptimeVerdict(s);
    if (verdict.kind === 'thin') {
      return {
        label: `Insufficient coverage (${formatAvailabilityObservedDuration(verdict.observedMinutes, range())} observed)`,
        toneClass: 'text-muted',
      };
    }
    if (verdict.kind === 'inconclusive') {
      return {
        label: 'Mostly inconclusive results',
        toneClass: 'text-amber-700 dark:text-amber-300',
      };
    }
    const toneClass =
      verdict.percent < 95
        ? 'text-rose-700 dark:text-rose-300'
        : verdict.percent < 99
          ? 'text-amber-700 dark:text-amber-300'
          : 'text-emerald-700 dark:text-emerald-300';
    return {
      label: `${formatAvailabilityPercent(verdict.percent)} uptime`,
      toneClass,
    };
  });

  const downtimeLabel = createMemo(() => {
    const s = summary();
    if (!s || !hasObservedHistory()) return null;
    if (s.unreachableSeconds > 0) {
      return {
        text: `${formatAvailabilityDurationSeconds(s.unreachableSeconds)} downtime`,
        isDown: true,
      };
    }
    return { text: 'No outages', isDown: false };
  });

  const activeBucket = createMemo(() => {
    const idx = activeIndex();
    const list = buckets();
    if (idx === null || idx < 0 || idx >= list.length) return null;
    return {
      index: idx,
      bucket: list[idx],
      state: bucketStates()[idx] ?? 'unknown',
      hasRevision: revisionBucketIndices().has(idx),
    };
  });

  const axisLabels = createMemo(() => {
    const list = buckets();
    if (list.length === 0) return { start: '', mid: '', end: 'Now' };
    const midIndex = Math.floor(list.length / 2);
    return {
      start: formatAxisTick(list[0]?.start, range()),
      mid: formatAxisTick(list[midIndex]?.start, range()),
      end: 'Now',
    };
  });

  const handleSvgPointerMove = (event: PointerEvent & { currentTarget: SVGSVGElement }) => {
    const list = buckets();
    if (list.length === 0) return;
    const rect = event.currentTarget.getBoundingClientRect();
    if (rect.width <= 0) return;
    const relX = ((event.clientX - rect.left) / rect.width) * CHART_WIDTH;
    const clamped = Math.max(PAD_LEFT, Math.min(PAD_LEFT + PLOT_WIDTH, relX));
    const ratio = (clamped - PAD_LEFT) / PLOT_WIDTH;
    const idx = Math.round(ratio * (list.length - 1));
    setActiveIndex(Math.max(0, Math.min(list.length - 1, idx)));
  };

  const handleTimelineKeyDown = (event: KeyboardEvent) => {
    const count = buckets().length;
    if (count === 0) return;
    if (event.key === 'ArrowRight') {
      event.preventDefault();
      const current = activeIndex();
      setActiveIndex(current === null ? 0 : Math.min(count - 1, current + 1));
    } else if (event.key === 'ArrowLeft') {
      event.preventDefault();
      const current = activeIndex();
      setActiveIndex(current === null ? count - 1 : Math.max(0, current - 1));
    } else if (event.key === 'Escape') {
      setActiveIndex(null);
    }
  };

  return (
    <section
      data-testid="availability-drawer-history"
      aria-label="Availability history"
      class="space-y-2.5"
    >
      <div class="flex flex-wrap items-center justify-between gap-2">
        <div class="flex min-w-0 flex-wrap items-center gap-x-3 gap-y-1 text-xs">
          <Show when={uptimeHeadline()}>
            {(headline) => (
              <span class={`font-semibold ${headline().toneClass}`}>{headline().label}</span>
            )}
          </Show>
          <Show when={downtimeLabel()}>
            {(downtime) => (
              <span
                class={
                  downtime().isDown ? 'font-medium text-rose-700 dark:text-rose-300' : 'text-muted'
                }
              >
                {downtime().text}
              </span>
            )}
          </Show>
          <Show when={summary()?.reachableLatencyMillis}>
            {(lat) => (
              <span class="tabular-nums text-muted">
                Avg {Math.round(lat().average)} ms ({Math.round(lat().min)}-{Math.round(lat().max)}{' '}
                ms)
              </span>
            )}
          </Show>
        </div>

        <FilterSegmentedControl
          value={range()}
          onChange={(next) => setRange(next as AvailabilityHistoryRange)}
          options={AVAILABILITY_HISTORY_RANGE_OPTIONS.map((opt) => ({
            value: opt.value,
            label: opt.label,
          }))}
          aria-label="Availability history range"
        />
      </div>

      <Show
        when={!loading() || history() !== null}
        fallback={
          <div data-testid="availability-drawer-history-loading" class="grid gap-3 md:grid-cols-2">
            <InfoCardFrame>
              <div class="h-24 animate-pulse rounded bg-base-200/70" />
            </InfoCardFrame>
            <InfoCardFrame>
              <div class="h-24 animate-pulse rounded bg-base-200/70" />
            </InfoCardFrame>
          </div>
        }
      >
        <Show
          when={!error()}
          fallback={
            <InfoCardFrame data-testid="availability-drawer-history-error">
              <div class="flex flex-wrap items-center justify-between gap-2 text-xs">
                <span class="text-muted">{error()}</span>
                <Button
                  type="button"
                  variant="outline"
                  size="xs"
                  onClick={() => setReloadTick((n) => n + 1)}
                >
                  Retry
                </Button>
              </div>
            </InfoCardFrame>
          }
        >
          <Show
            when={hasObservedHistory() && buckets().length > 0}
            fallback={
              <InfoCardFrame data-testid="availability-drawer-history-empty">
                <p class="text-xs text-muted">
                  History will appear here once checks begin reporting ({rangeMeta().evidenceLabel}{' '}
                  window).
                </p>
              </InfoCardFrame>
            }
          >
            <div class={`grid gap-3 ${hasLatencyPoints() ? 'md:grid-cols-2' : ''}`}>
              <InfoCardFrame data-testid="availability-drawer-timeline" class="flex flex-col">
                <div class="mb-2 flex flex-wrap items-center justify-between gap-2">
                  <div class="flex items-center gap-1.5">
                    <ActivityIcon class="h-3.5 w-3.5 text-muted" aria-hidden="true" />
                    <h3 class="text-[11px] font-medium uppercase tracking-wide text-base-content">
                      Status timeline
                    </h3>
                  </div>
                  <div
                    class="min-h-4 text-[11px] tabular-nums"
                    data-testid="availability-timeline-readout"
                  >
                    <Show
                      when={activeBucket()}
                      fallback={
                        <span class="text-muted">
                          {formatAvailabilityPercent(summary()?.coveragePercent ?? 0)} observed
                        </span>
                      }
                    >
                      {(item) => {
                        const pres = AVAILABILITY_STATE_PRESENTATION[item().state];
                        const lat = item().bucket.latencyMillis;
                        return (
                          <span class="flex flex-wrap items-center gap-x-1.5">
                            <span class="text-muted">
                              {formatBucketTimestamp(item().bucket.start, range())}
                            </span>
                            <span class={`font-medium ${pres.textClassName}`}>{pres.label}</span>
                            <Show when={item().bucket.unreachableSeconds > 0}>
                              <span class="text-rose-700 dark:text-rose-300">
                                (
                                {formatAvailabilityDurationSeconds(
                                  item().bucket.unreachableSeconds,
                                )}{' '}
                                down)
                              </span>
                            </Show>
                            <Show when={lat && Number.isFinite(lat.average)}>
                              <span class="text-muted">{Math.round(lat!.average)} ms</span>
                            </Show>
                            <Show when={item().hasRevision}>
                              <span class="text-sky-700 dark:text-sky-300">Config changed</span>
                            </Show>
                          </span>
                        );
                      }}
                    </Show>
                  </div>
                </div>

                <div class="flex flex-1 flex-col justify-center">
                  <div
                    role="group"
                    tabIndex={0}
                    aria-label={`Availability status timeline for the last ${rangeMeta().windowPhrase}. Use left and right arrow keys to inspect intervals.`}
                    onKeyDown={handleTimelineKeyDown}
                    onPointerLeave={() => setActiveIndex(null)}
                    onBlur={() => setActiveIndex(null)}
                    class="flex h-12 w-full items-stretch gap-px rounded-sm bg-base-200/70 p-0.5 focus:outline-hidden focus:ring-2 focus:ring-blue-500/50"
                  >
                    <For each={buckets()}>
                      {(bucket, index) => {
                        const state = () => bucketStates()[index()] ?? 'unknown';
                        const pres = () => AVAILABILITY_STATE_PRESENTATION[state()];
                        const isSelected = () => activeIndex() === index();
                        const hasRevision = () => revisionBucketIndices().has(index());
                        return (
                          <div
                            onPointerEnter={() => setActiveIndex(index())}
                            class={`relative min-w-px flex-1 rounded-[1px] transition-opacity ${pres().className} ${
                              activeIndex() !== null && !isSelected() ? 'opacity-55' : 'opacity-100'
                            } ${isSelected() ? 'ring-1 ring-base-content/70' : ''}`}
                            title={`${formatBucketTimestamp(bucket.start, range())} · ${pres().label}`}
                          >
                            <Show when={hasRevision()}>
                              <span
                                class="pointer-events-none absolute inset-x-0 -top-0.5 h-1 rounded-full bg-sky-500 dark:bg-sky-300"
                                aria-hidden="true"
                              />
                            </Show>
                          </div>
                        );
                      }}
                    </For>
                  </div>

                  <div class="mt-1.5 flex items-center justify-between text-[10px] tabular-nums text-muted">
                    <span>{axisLabels().start}</span>
                    <span>{axisLabels().mid}</span>
                    <span>{axisLabels().end}</span>
                  </div>
                </div>

                <div class="mt-2 flex flex-wrap items-center gap-x-3 gap-y-1 border-t border-border/60 pt-2 text-[10px] text-muted">
                  <span class="inline-flex items-center gap-1">
                    <span class="h-2 w-2 rounded-[1px] bg-emerald-500 dark:bg-emerald-400" />
                    Reachable
                  </span>
                  <span class="inline-flex items-center gap-1">
                    <span class="h-2 w-2 rounded-[1px] bg-rose-500 dark:bg-rose-400" />
                    Unreachable
                  </span>
                  <span class="inline-flex items-center gap-1">
                    <span class="h-2 w-2 rounded-[1px] bg-amber-400 dark:bg-amber-300" />
                    Indeterminate
                  </span>
                  <span class="inline-flex items-center gap-1">
                    <span class="h-2 w-2 rounded-[1px] bg-slate-300 dark:bg-slate-600" />
                    Unknown
                  </span>
                  <Show when={revisionBucketIndices().size > 0}>
                    <span class="inline-flex items-center gap-1">
                      <span class="h-1.5 w-2 rounded-full bg-sky-500 dark:bg-sky-300" />
                      Config changed
                    </span>
                  </Show>
                </div>
              </InfoCardFrame>

              <Show when={hasLatencyPoints()}>
                <InfoCardFrame
                  data-testid="availability-drawer-latency-chart"
                  class="flex flex-col"
                >
                  <div class="mb-2 flex flex-wrap items-center justify-between gap-2">
                    <div class="flex items-center gap-1.5">
                      <ClockIcon class="h-3.5 w-3.5 text-muted" aria-hidden="true" />
                      <h3 class="text-[11px] font-medium uppercase tracking-wide text-base-content">
                        Response time
                      </h3>
                    </div>
                    <div
                      class="min-h-4 text-[11px] tabular-nums text-muted"
                      data-testid="availability-latency-readout"
                    >
                      <Show
                        when={activeBucket()?.bucket.latencyMillis}
                        fallback={
                          <Show when={summary()?.reachableLatencyMillis}>
                            {(lat) => (
                              <span>
                                Avg {Math.round(lat().average)} ms · Max {Math.round(lat().max)} ms
                              </span>
                            )}
                          </Show>
                        }
                      >
                        {(lat) => (
                          <span>
                            {formatBucketTimestamp(activeBucket()!.bucket.start, range())} ·{' '}
                            <span class="font-medium text-sky-700 dark:text-sky-300">
                              {Math.round(lat().average)} ms
                            </span>
                            <Show when={lat().max - lat().min >= 1}>
                              <span>
                                {' '}
                                ({Math.round(lat().min)}-{Math.round(lat().max)} ms)
                              </span>
                            </Show>
                          </span>
                        )}
                      </Show>
                    </div>
                  </div>

                  <svg
                    viewBox={`0 0 ${CHART_WIDTH} ${CHART_HEIGHT}`}
                    class="h-28 w-full select-none overflow-visible"
                    role="img"
                    aria-label={`Response time trend over the last ${rangeMeta().windowPhrase}`}
                    onPointerMove={handleSvgPointerMove}
                    onPointerLeave={() => setActiveIndex(null)}
                  >
                    <defs>
                      <linearGradient
                        id={gradientId}
                        class="text-sky-500 dark:text-sky-400"
                        x1="0"
                        y1="0"
                        x2="0"
                        y2="1"
                      >
                        <stop offset="0%" stop-color="currentColor" stop-opacity="0.24" />
                        <stop offset="100%" stop-color="currentColor" stop-opacity="0.02" />
                      </linearGradient>
                    </defs>

                    {[0, 0.5, 1].map((fraction) => {
                      const y = PAD_TOP + fraction * PLOT_HEIGHT;
                      const value = latencyGeometry().ceiling * (1 - fraction);
                      return (
                        <g>
                          <line
                            x1={PAD_LEFT}
                            x2={PAD_LEFT + PLOT_WIDTH}
                            y1={y}
                            y2={y}
                            stroke="currentColor"
                            stroke-opacity="0.14"
                            stroke-dasharray={fraction === 1 ? undefined : '2 3'}
                            class="text-base-content"
                          />
                          <text
                            x={PAD_LEFT - 5}
                            y={y + 3}
                            text-anchor="end"
                            class="fill-current text-[9px] tabular-nums text-muted"
                          >
                            {formatLatencyTick(value)}
                          </text>
                        </g>
                      );
                    })}

                    <g class="text-sky-500 dark:text-sky-400">
                      <For each={latencyGeometry().areaPaths}>
                        {(d) => <path d={d} fill={`url(#${gradientId})`} />}
                      </For>
                      <For each={latencyGeometry().bandPaths}>
                        {(d) => <path d={d} fill="currentColor" fill-opacity="0.14" />}
                      </For>
                      <For each={latencyGeometry().linePaths}>
                        {(d) => (
                          <path
                            d={d}
                            fill="none"
                            stroke="currentColor"
                            stroke-width="1.75"
                            stroke-linecap="round"
                            stroke-linejoin="round"
                          />
                        )}
                      </For>
                    </g>

                    <Show when={activeIndex() !== null}>
                      {(() => {
                        const idx = activeIndex()!;
                        const count = buckets().length;
                        const x =
                          count <= 1
                            ? PAD_LEFT + PLOT_WIDTH / 2
                            : PAD_LEFT + (idx / (count - 1)) * PLOT_WIDTH;
                        const pt = latencyGeometry().points[idx] ?? null;
                        return (
                          <g pointer-events="none">
                            <line
                              x1={x}
                              x2={x}
                              y1={PAD_TOP}
                              y2={PAD_TOP + PLOT_HEIGHT}
                              stroke="currentColor"
                              stroke-opacity="0.45"
                              stroke-dasharray="2 2"
                              class="text-base-content"
                            />
                            <Show when={pt}>
                              {(activePoint) => (
                                <circle
                                  cx={activePoint().x}
                                  cy={activePoint().y}
                                  r="3.5"
                                  class="fill-sky-500 stroke-surface stroke-2 dark:fill-sky-300"
                                />
                              )}
                            </Show>
                          </g>
                        );
                      })()}
                    </Show>
                  </svg>

                  <div class="mt-1 flex items-center justify-between pl-9 text-[10px] tabular-nums text-muted">
                    <span>{axisLabels().start}</span>
                    <span>{axisLabels().mid}</span>
                    <span>{axisLabels().end}</span>
                  </div>
                </InfoCardFrame>
              </Show>
            </div>
          </Show>
        </Show>
      </Show>
    </section>
  );
};

export default AvailabilityDrawerHistory;
