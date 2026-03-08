import { Component, For, Show, createEffect, createMemo, createSignal, onCleanup } from 'solid-js';
import { Card } from '@/components/shared/Card';
import { InteractiveSparkline } from '@/components/shared/InteractiveSparkline';
import type { InteractiveSparklineSeries } from '@/components/shared/InteractiveSparkline';
import { SparklineSkeleton } from '@/components/shared/SparklineSkeleton';
import {
  ChartsAPI,
  type MetricPoint,
  type TimeRange,
  type StorageSummaryChartsResponse,
} from '@/api/charts';
import {
  SUMMARY_TIME_RANGES,
  SUMMARY_TIME_RANGE_LABEL,
  type SummaryTimeRange,
} from '@/components/shared/summaryTimeRange';
import { RESOURCE_COLORS } from '@/pages/DashboardPanels/resourceColors';
import { formatBytes } from '@/utils/format';

// ---------------------------------------------------------------------------
// Cache
// ---------------------------------------------------------------------------

const POLL_INTERVAL_MS = 30_000;
const inMemoryCache = new Map<string, StorageSummaryChartsResponse>();

// ---------------------------------------------------------------------------
// Props
// ---------------------------------------------------------------------------

interface StorageSummaryProps {
  poolCount: number;
  diskCount: number;
  timeRange: SummaryTimeRange;
  onTimeRangeChange?: (range: SummaryTimeRange) => void;
}

// ---------------------------------------------------------------------------
// Component
// ---------------------------------------------------------------------------

const StorageSummary: Component<StorageSummaryProps> = (props) => {
  const [data, setData] = createSignal<StorageSummaryChartsResponse | null>(null);
  const [loaded, setLoaded] = createSignal(false);
  const [fetchFailed, setFetchFailed] = createSignal(false);

  let abortController: AbortController | undefined;

  const cacheKey = () => props.timeRange;

  // Fetch data
  const fetchData = async () => {
    abortController?.abort();
    abortController = new AbortController();

    try {
      const response = await ChartsAPI.getStorageSummaryCharts(
        props.timeRange as TimeRange,
        abortController.signal,
      );
      inMemoryCache.set(cacheKey(), response);
      setData(response);
      setFetchFailed(false);
    } catch (err: unknown) {
      if (err instanceof DOMException && err.name === 'AbortError') return;
      setFetchFailed(true);
      // Fall back to cache
      const cached = inMemoryCache.get(cacheKey());
      if (cached) setData(cached);
    } finally {
      setLoaded(true);
    }
  };

  // Initial load + range change
  createEffect(() => {
    const _range = props.timeRange; // track dependency
    void _range;
    // Try cache first for instant display
    const cached = inMemoryCache.get(cacheKey());
    if (cached) {
      setData(cached);
      setLoaded(true);
    } else {
      setLoaded(false);
    }
    fetchData();
  });

  // Polling
  createEffect(() => {
    const _range = props.timeRange; // track dependency
    void _range;
    const timer = setInterval(fetchData, POLL_INTERVAL_MS);
    onCleanup(() => clearInterval(timer));
  });

  onCleanup(() => abortController?.abort());

  // ---------------------------------------------------------------------------
  // Series builders
  // ---------------------------------------------------------------------------

  const poolUsageSeries = createMemo((): InteractiveSparklineSeries[] => {
    const d = data();
    if (!d?.pools) return [];
    const entries = Object.entries(d.pools);
    return entries
      .filter(([, pool]) => pool.usage && pool.usage.length >= 2)
      .map(([id, pool], i) => ({
        id,
        name: pool.name || id,
        color: RESOURCE_COLORS[i % RESOURCE_COLORS.length],
        data: pool.usage as MetricPoint[],
      }));
  });

  const poolUsedSeries = createMemo((): InteractiveSparklineSeries[] => {
    const d = data();
    if (!d?.pools) return [];
    const entries = Object.entries(d.pools);
    return entries
      .filter(([, pool]) => pool.used && pool.used.length >= 2)
      .map(([id, pool], i) => ({
        id,
        name: pool.name || id,
        color: RESOURCE_COLORS[i % RESOURCE_COLORS.length],
        data: pool.used as MetricPoint[],
      }));
  });

  const poolAvailSeries = createMemo((): InteractiveSparklineSeries[] => {
    const d = data();
    if (!d?.pools) return [];
    const entries = Object.entries(d.pools);
    return entries
      .filter(([, pool]) => pool.avail && pool.avail.length >= 2)
      .map(([id, pool], i) => ({
        id,
        name: pool.name || id,
        color: RESOURCE_COLORS[i % RESOURCE_COLORS.length],
        data: pool.avail as MetricPoint[],
      }));
  });

  const diskTempSeries = createMemo((): InteractiveSparklineSeries[] => {
    const d = data();
    if (!d?.disks) return [];
    const entries = Object.entries(d.disks);
    return entries
      .filter(([, disk]) => disk.temperature && disk.temperature.length >= 2)
      .map(([id, disk], i) => ({
        id,
        name: disk.name || id,
        color: RESOURCE_COLORS[i % RESOURCE_COLORS.length],
        data: disk.temperature as MetricPoint[],
      }));
  });

  const hasPoolUsage = () => poolUsageSeries().length > 0;
  const hasDiskTemp = () => diskTempSeries().length > 0;
  const hasPoolUsed = () => poolUsedSeries().length > 0;
  const hasPoolAvail = () => poolAvailSeries().length > 0;

  const emptyLabel = () => (fetchFailed() ? 'Trend data unavailable' : 'No history yet');

  const rangeLabel = () => SUMMARY_TIME_RANGE_LABEL[props.timeRange] ?? props.timeRange;

  const formatTemp = (value: number) => `${value.toFixed(0)}°C`;

  const showComponent = () => props.poolCount > 0 || props.diskCount > 0;

  return (
    <Show when={showComponent()}>
      <div data-testid="storage-summary" class="space-y-2">
        <div class="rounded-md border border-border bg-surface p-2 shadow-sm sm:p-3">
          {/* Header */}
          <div class="mb-2 flex flex-wrap items-center justify-between gap-2 border-b border-border-subtle px-1 pb-2 text-[11px] text-slate-500">
            <div class="flex items-center gap-3">
              <span class="font-medium text-base-content">
                {props.poolCount} {props.poolCount === 1 ? 'pool' : 'pools'}
              </span>
              <Show when={props.diskCount > 0}>
                <span class="text-muted">
                  {props.diskCount} {props.diskCount === 1 ? 'disk' : 'disks'}
                </span>
              </Show>
            </div>
            <Show when={props.onTimeRangeChange}>
              <div class="inline-flex shrink-0 rounded border border-border bg-surface p-0.5 text-xs">
                <For each={SUMMARY_TIME_RANGES as unknown as SummaryTimeRange[]}>
                  {(range) => (
                    <button
                      type="button"
                      onClick={() => props.onTimeRangeChange?.(range)}
                      class={`rounded px-2 py-1 ${
                        props.timeRange === range
                          ? 'bg-blue-100 text-blue-700 dark:bg-blue-900 dark:text-blue-200'
                          : 'text-muted hover:bg-surface-hover'
                      }`}
                    >
                      {SUMMARY_TIME_RANGE_LABEL[range]}
                    </button>
                  )}
                </For>
              </div>
            </Show>
          </div>

          {/* Chart cards */}
          <div class="grid gap-2 sm:gap-3 grid-cols-2 lg:grid-cols-4">
            {/* Pool Usage % */}
            <Card padding="sm" class="h-full">
              <div class="flex flex-col h-full">
                <div class="flex items-center mb-1.5 min-w-0">
                  <span class="text-xs font-medium text-muted uppercase tracking-wide shrink-0">
                    Pool Usage
                  </span>
                </div>
                <Show
                  when={hasPoolUsage()}
                  fallback={
                    loaded() ? (
                      <div class="text-sm text-muted py-2">{emptyLabel()}</div>
                    ) : (
                      <SparklineSkeleton />
                    )
                  }
                >
                  <div class="flex-1 min-h-0">
                    <InteractiveSparkline
                      series={poolUsageSeries()}
                      rangeLabel={rangeLabel()}
                      timeRange={props.timeRange as TimeRange}
                      yMode="percent"
                      highlightNearestSeriesOnHover
                    />
                  </div>
                </Show>
              </div>
            </Card>

            {/* Disk Temperature */}
            <Card padding="sm" class="h-full">
              <div class="flex flex-col h-full">
                <div class="flex items-center mb-1.5 min-w-0">
                  <span class="text-xs font-medium text-muted uppercase tracking-wide shrink-0">
                    Disk Temperature
                  </span>
                </div>
                <Show
                  when={hasDiskTemp()}
                  fallback={
                    loaded() ? (
                      <div class="text-sm text-muted py-2">{emptyLabel()}</div>
                    ) : (
                      <SparklineSkeleton />
                    )
                  }
                >
                  <div class="flex-1 min-h-0">
                    <InteractiveSparkline
                      series={diskTempSeries()}
                      rangeLabel={rangeLabel()}
                      timeRange={props.timeRange as TimeRange}
                      yMode="auto"
                      formatValue={formatTemp}
                      formatTopLabel={(max) => `${max.toFixed(0)}°C`}
                      highlightNearestSeriesOnHover
                    />
                  </div>
                </Show>
              </div>
            </Card>

            {/* Used Capacity */}
            <Card padding="sm" class="h-full">
              <div class="flex flex-col h-full">
                <div class="flex items-center mb-1.5 min-w-0">
                  <span class="text-xs font-medium text-muted uppercase tracking-wide shrink-0">
                    Used Capacity
                  </span>
                </div>
                <Show
                  when={hasPoolUsed()}
                  fallback={
                    loaded() ? (
                      <div class="text-sm text-muted py-2">{emptyLabel()}</div>
                    ) : (
                      <SparklineSkeleton />
                    )
                  }
                >
                  <div class="flex-1 min-h-0">
                    <InteractiveSparkline
                      series={poolUsedSeries()}
                      rangeLabel={rangeLabel()}
                      timeRange={props.timeRange as TimeRange}
                      yMode="auto"
                      formatValue={(v) => formatBytes(v)}
                      formatTopLabel={(max) => formatBytes(max)}
                      highlightNearestSeriesOnHover
                    />
                  </div>
                </Show>
              </div>
            </Card>

            {/* Available Space */}
            <Card padding="sm" class="h-full">
              <div class="flex flex-col h-full">
                <div class="flex items-center mb-1.5 min-w-0">
                  <span class="text-xs font-medium text-muted uppercase tracking-wide shrink-0">
                    Available Space
                  </span>
                </div>
                <Show
                  when={hasPoolAvail()}
                  fallback={
                    loaded() ? (
                      <div class="text-sm text-muted py-2">{emptyLabel()}</div>
                    ) : (
                      <SparklineSkeleton />
                    )
                  }
                >
                  <div class="flex-1 min-h-0">
                    <InteractiveSparkline
                      series={poolAvailSeries()}
                      rangeLabel={rangeLabel()}
                      timeRange={props.timeRange as TimeRange}
                      yMode="auto"
                      formatValue={(v) => formatBytes(v)}
                      formatTopLabel={(max) => formatBytes(max)}
                      highlightNearestSeriesOnHover
                    />
                  </div>
                </Show>
              </div>
            </Card>
          </div>
        </div>
      </div>
    </Show>
  );
};

export default StorageSummary;
