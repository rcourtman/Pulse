import { Component, Show, createEffect, createMemo, createSignal, onCleanup } from 'solid-js';
import { InteractiveSparkline } from '@/components/shared/InteractiveSparkline';
import type { InteractiveSparklineSeries } from '@/components/shared/InteractiveSparkline';
import {
  useSummaryContextualFocusState,
  type SummaryChartHoverSync,
} from '@/components/shared/contextualFocus';
import type { SummarySeriesGroupScope } from '@/components/shared/summaryCardInteraction';
import { SummaryJumpToRowButton } from '@/components/shared/SummaryJumpToRowButton';
import { SummaryPanel } from '@/components/shared/SummaryPanel';
import { SummaryMetricCard } from '@/components/shared/SummaryMetricCard';
import { SummarySynchronizedReadout } from '@/components/shared/SummarySynchronizedReadout';
import { buildInteractiveSparklineSynchronizedReadout } from '@/components/shared/interactiveSparklineModel';
import {
  ChartsAPI,
  type MetricPoint,
  type TimeRange,
  type StorageSummaryChartsResponse,
} from '@/api/charts';
import {
  SUMMARY_TIME_RANGE_LABEL,
  type SummaryTimeRange,
} from '@/components/shared/summaryTimeRange';
import { formatBytes } from '@/utils/format';
import { getOrgID } from '@/utils/apiClient';
import { normalizeOrgScope } from '@/utils/orgScope';
import { eventBus } from '@/stores/events';
import { getChartSeriesColor } from '@/utils/chartSeriesPresentation';

// ---------------------------------------------------------------------------
// Cache (org-scoped to prevent cross-tenant data leakage)
// ---------------------------------------------------------------------------

const POLL_INTERVAL_MS = 30_000;
const STORAGE_SUMMARY_IN_MEMORY_CACHE_VERSION = 1;
const inMemoryCache = new Map<string, StorageSummaryChartsResponse>();

function inMemoryCacheKey(range: TimeRange, nodeId?: string): string {
  return `${STORAGE_SUMMARY_IN_MEMORY_CACHE_VERSION}::${normalizeOrgScope(getOrgID())}::${range}::${nodeId || '__all__'}`;
}

// Clear in-memory cache on org switch to prevent cross-org data leakage.
const unsubscribeStorageOrgSwitch = eventBus.on('org_switched', () => {
  inMemoryCache.clear();
});

if (import.meta.hot) {
  import.meta.hot.dispose(() => {
    unsubscribeStorageOrgSwitch();
  });
}

// ---------------------------------------------------------------------------
// Props
// ---------------------------------------------------------------------------

interface StorageSummaryProps {
  poolCount: number;
  diskCount: number;
  timeRange: SummaryTimeRange;
  onTimeRangeChange?: (range: SummaryTimeRange) => void;
  nodeId?: string;
  hoveredResourceId?: string | null;
  hoveredGroupScope?: SummarySeriesGroupScope | null;
  focusedResourceId?: string | null;
  focusedGroupScope?: SummarySeriesGroupScope | null;
  chartHoverSync?: SummaryChartHoverSync | null;
  onChartHoverSyncChange?: (value: SummaryChartHoverSync | null) => void;
  showJumpToActiveRow?: boolean;
  onJumpToActiveRow?: () => void;
}

// ---------------------------------------------------------------------------
// Component
// ---------------------------------------------------------------------------

const StorageSummary: Component<StorageSummaryProps> = (props) => {
  const [data, setData] = createSignal<StorageSummaryChartsResponse | null>(null);
  const [loaded, setLoaded] = createSignal(false);
  const [fetchFailed, setFetchFailed] = createSignal(false);
  const [localChartHoverSync, setLocalChartHoverSync] = createSignal<SummaryChartHoverSync | null>(
    null,
  );
  const chartHoverSync = () => props.chartHoverSync ?? localChartHoverSync();
  const setChartHoverSync = (value: SummaryChartHoverSync | null) => {
    if (props.chartHoverSync === undefined) {
      setLocalChartHoverSync(value);
    }
    props.onChartHoverSyncChange?.(value);
  };

  // Track org switches so the effect re-runs when the org changes.
  const [orgVersion, setOrgVersion] = createSignal(0);
  const unsubscribeOrgSwitch = eventBus.on('org_switched', () => {
    setOrgVersion((v) => v + 1);
  });

  let activeFetchController: AbortController | null = null;
  let activeFetchRequest = 0;
  let refreshTimer: ReturnType<typeof setInterval> | undefined;

  const selectedRange = createMemo<TimeRange>(() => (props.timeRange as TimeRange) || '1h');
  const selectedNodeId = createMemo(() => {
    const id = props.nodeId?.trim();
    return id && id !== 'all' ? id : undefined;
  });

  // Fetch data with race-condition prevention via request ID
  const fetchData = async (options?: { prioritize?: boolean }) => {
    const prioritize = options?.prioritize === true;
    if (activeFetchController && !prioritize) return;
    if (activeFetchController && prioritize) {
      activeFetchController.abort();
    }

    const requestedRange = selectedRange();
    const requestedNodeId = selectedNodeId();
    // Capture scope before async gap to prevent cross-org/scope cache writes
    const capturedCacheKey = inMemoryCacheKey(requestedRange, requestedNodeId);
    const controller = new AbortController();
    const requestId = ++activeFetchRequest;
    activeFetchController = controller;

    try {
      const response = await ChartsAPI.getStorageSummaryCharts(requestedRange, controller.signal, {
        nodeId: requestedNodeId,
      });
      if (requestId !== activeFetchRequest) return; // stale response
      inMemoryCache.set(capturedCacheKey, response);
      setData(response);
      setFetchFailed(false);
    } catch (err: unknown) {
      if (err instanceof DOMException && err.name === 'AbortError') return;
      if (requestId !== activeFetchRequest) return; // stale error
      setFetchFailed(true);
      // Fall back to cache
      const cached = inMemoryCache.get(capturedCacheKey);
      if (cached) setData(cached);
    } finally {
      if (activeFetchController === controller) {
        activeFetchController = null;
      }
      if (requestId === activeFetchRequest) {
        setLoaded(true);
      }
    }
  };

  // Initial load + range/org/node change
  createEffect(() => {
    const range = selectedRange();
    const nodeId = selectedNodeId();
    const _org = orgVersion(); // subscribe to org switches
    void _org;

    // Clear stale timer on scope change
    if (refreshTimer) {
      clearInterval(refreshTimer);
      refreshTimer = undefined;
    }

    // Try cache first for instant display
    const cached = inMemoryCache.get(inMemoryCacheKey(range, nodeId));
    if (cached) {
      setData(cached);
      setLoaded(true);
    } else {
      setData(null);
      setLoaded(false);
    }
    setFetchFailed(false);

    // Start polling
    refreshTimer = setInterval(() => void fetchData(), POLL_INTERVAL_MS);

    void fetchData({ prioritize: true });

    onCleanup(() => {
      if (refreshTimer) {
        clearInterval(refreshTimer);
        refreshTimer = undefined;
      }
    });
  });

  onCleanup(() => {
    activeFetchController?.abort();
    unsubscribeOrgSwitch();
  });

  // ---------------------------------------------------------------------------
  // Series builders
  // ---------------------------------------------------------------------------

  const allPoolUsageSeries = createMemo((): InteractiveSparklineSeries[] => {
    const d = data();
    if (!d?.pools) return [];
    const entries = Object.entries(d.pools);
    return entries
      .filter(([, pool]) => pool.usage && pool.usage.length >= 2)
      .map(([id, pool], i) => ({
        id,
        name: pool.name || id,
        color: getChartSeriesColor(i),
        data: pool.usage as MetricPoint[],
      }));
  });

  const allPoolUsedSeries = createMemo((): InteractiveSparklineSeries[] => {
    const d = data();
    if (!d?.pools) return [];
    const entries = Object.entries(d.pools);
    return entries
      .filter(([, pool]) => pool.used && pool.used.length >= 2)
      .map(([id, pool], i) => ({
        id,
        name: pool.name || id,
        color: getChartSeriesColor(i),
        data: pool.used as MetricPoint[],
      }));
  });

  const allPoolAvailSeries = createMemo((): InteractiveSparklineSeries[] => {
    const d = data();
    if (!d?.pools) return [];
    const entries = Object.entries(d.pools);
    return entries
      .filter(([, pool]) => pool.avail && pool.avail.length >= 2)
      .map(([id, pool], i) => ({
        id,
        name: pool.name || id,
        color: getChartSeriesColor(i),
        data: pool.avail as MetricPoint[],
      }));
  });

  const allDiskTempSeries = createMemo((): InteractiveSparklineSeries[] => {
    const d = data();
    if (!d?.disks) return [];
    const entries = Object.entries(d.disks);
    return entries
      .filter(([, disk]) => disk.temperature && disk.temperature.length >= 2)
      .map(([id, disk], i) => ({
        id,
        name: disk.name || id,
        color: getChartSeriesColor(i),
        data: disk.temperature as MetricPoint[],
      }));
  });
  const interactiveSummarySeries = createMemo<InteractiveSparklineSeries[]>(() => [
      ...allPoolUsageSeries(),
      ...allPoolUsedSeries(),
      ...allPoolAvailSeries(),
      ...allDiskTempSeries(),
  ]);
  const summaryFocus = useSummaryContextualFocusState({
    chartHoveredSeriesId: () => chartHoverSync()?.seriesId ?? null,
    interactiveSeries: interactiveSummarySeries,
    focusedGroupScope: () => props.focusedGroupScope,
    hoveredGroupScope: () => props.hoveredGroupScope,
    hoveredSeriesId: () => props.hoveredResourceId,
    focusedSeriesId: () => props.focusedResourceId,
  });

  createEffect(() => {
    const hovered = chartHoverSync();
    if (!hovered) return;
    if (!summaryFocus.isSeriesIdVisibleInActiveScope(hovered.seriesId)) {
      setChartHoverSync(null);
    }
  });

  const poolUsageSeries = createMemo(() =>
    summaryFocus.filterSeriesForActiveScope(allPoolUsageSeries()),
  );
  const poolUsedSeries = createMemo(() =>
    summaryFocus.filterSeriesForActiveScope(allPoolUsedSeries()),
  );
  const poolAvailSeries = createMemo(() =>
    summaryFocus.filterSeriesForActiveScope(allPoolAvailSeries()),
  );
  const diskTempSeries = createMemo(() =>
    summaryFocus.filterSeriesForActiveScope(allDiskTempSeries()),
  );

  const hasPoolUsage = () => poolUsageSeries().length > 0;
  const hasDiskTemp = () => diskTempSeries().length > 0;
  const hasPoolUsed = () => poolUsedSeries().length > 0;
  const hasPoolAvail = () => poolAvailSeries().length > 0;

  const emptyLabel = () => {
    if (fetchFailed()) return 'Trend data unavailable';
    if (summaryFocus.activeGroupScope()) return 'No group history yet';
    return 'No history yet';
  };

  const rangeLabel = () => SUMMARY_TIME_RANGE_LABEL[props.timeRange] ?? props.timeRange;

  const formatTemp = (value: number) => `${value.toFixed(0)}°C`;

  const showComponent = () => props.poolCount > 0 || props.diskCount > 0;
  const getFocusedSeriesName = (series: InteractiveSparklineSeries[]): string | null =>
    summaryFocus.getActiveSeriesName(series);
  const focusedLabel = (series: InteractiveSparklineSeries[]) => {
    const name = getFocusedSeriesName(series);
    if (!name) return undefined;
    return <span class="text-xs text-muted ml-1.5 truncate">&mdash; {name}</span>;
  };
  const renderSyncedReadout = (
    readout: { empty?: boolean; timestamp: number; value: string } | null,
  ) =>
    readout ? (
      <SummarySynchronizedReadout
        empty={readout.empty}
        timestamp={readout.timestamp}
        value={readout.value}
      />
    ) : undefined;
  const poolUsageSyncedReadout = () =>
    buildInteractiveSparklineSynchronizedReadout({
      hoverSourceKey: 'pool-usage',
      hoverSync: chartHoverSync(),
      series: poolUsageSeries(),
      timeRange: props.timeRange as TimeRange,
    });
  const diskTempSyncedReadout = () =>
    buildInteractiveSparklineSynchronizedReadout({
      formatValue: formatTemp,
      hoverSourceKey: 'disk-temperature',
      hoverSync: chartHoverSync(),
      series: diskTempSeries(),
      timeRange: props.timeRange as TimeRange,
    });
  const poolUsedSyncedReadout = () =>
    buildInteractiveSparklineSynchronizedReadout({
      formatValue: (value) => formatBytes(value),
      hoverSourceKey: 'used-capacity',
      hoverSync: chartHoverSync(),
      series: poolUsedSeries(),
      timeRange: props.timeRange as TimeRange,
    });
  const poolAvailSyncedReadout = () =>
    buildInteractiveSparklineSynchronizedReadout({
      formatValue: (value) => formatBytes(value),
      hoverSourceKey: 'available-space',
      hoverSync: chartHoverSync(),
      series: poolAvailSeries(),
      timeRange: props.timeRange as TimeRange,
    });

  return (
    <Show when={showComponent()}>
      <div class="space-y-2">
        <SummaryPanel
          testId="storage-summary"
          headerLeft={
            <>
              <span class="font-medium text-base-content">
                {props.poolCount} {props.poolCount === 1 ? 'pool' : 'pools'}
              </span>
              <Show when={props.diskCount > 0}>
                <span class="text-muted">
                  {props.diskCount} {props.diskCount === 1 ? 'disk' : 'disks'}
                </span>
              </Show>
              <Show when={props.showJumpToActiveRow && props.onJumpToActiveRow}>
                <SummaryJumpToRowButton onClick={() => props.onJumpToActiveRow?.()} />
              </Show>
            </>
          }
          timeRange={props.timeRange}
          onTimeRangeChange={props.onTimeRangeChange}
        >
          <SummaryMetricCard
            label="Pool Usage"
            secondaryLabel={focusedLabel(poolUsageSeries())}
            headerValue={renderSyncedReadout(poolUsageSyncedReadout())}
            loaded={loaded()}
            hasData={hasPoolUsage()}
            emptyMessage={emptyLabel()}
            interactionState={summaryFocus.interactionStateFor(poolUsageSeries())}
          >
            <InteractiveSparkline
              series={poolUsageSeries()}
              rangeLabel={rangeLabel()}
              timeRange={props.timeRange as TimeRange}
              activeSeriesDisplay="isolate"
              yMode="percent"
              highlightNearestSeriesOnHover
              hoverSourceKey="pool-usage"
              hoverSync={chartHoverSync()}
              highlightSeriesId={summaryFocus.activeSeriesId()}
              interactionState={summaryFocus.interactionStateFor(poolUsageSeries())}
              onHoverSyncChange={setChartHoverSync}
            />
          </SummaryMetricCard>

          <SummaryMetricCard
            label="Disk Temperature"
            secondaryLabel={focusedLabel(diskTempSeries())}
            headerValue={renderSyncedReadout(diskTempSyncedReadout())}
            loaded={loaded()}
            hasData={hasDiskTemp()}
            emptyMessage={emptyLabel()}
            interactionState={summaryFocus.interactionStateFor(diskTempSeries())}
          >
            <InteractiveSparkline
              series={diskTempSeries()}
              rangeLabel={rangeLabel()}
              timeRange={props.timeRange as TimeRange}
              activeSeriesDisplay="isolate"
              yMode="auto"
              formatValue={formatTemp}
              formatTopLabel={(max) => `${max.toFixed(0)}°C`}
              highlightNearestSeriesOnHover
              hoverSourceKey="disk-temperature"
              hoverSync={chartHoverSync()}
              highlightSeriesId={summaryFocus.activeSeriesId()}
              interactionState={summaryFocus.interactionStateFor(diskTempSeries())}
              onHoverSyncChange={setChartHoverSync}
            />
          </SummaryMetricCard>

          <SummaryMetricCard
            label="Used Capacity"
            secondaryLabel={focusedLabel(poolUsedSeries())}
            headerValue={renderSyncedReadout(poolUsedSyncedReadout())}
            loaded={loaded()}
            hasData={hasPoolUsed()}
            emptyMessage={emptyLabel()}
            interactionState={summaryFocus.interactionStateFor(poolUsedSeries())}
          >
            <InteractiveSparkline
              series={poolUsedSeries()}
              rangeLabel={rangeLabel()}
              timeRange={props.timeRange as TimeRange}
              activeSeriesDisplay="isolate"
              yMode="auto"
              formatValue={(v) => formatBytes(v)}
              formatTopLabel={(max) => formatBytes(max)}
              highlightNearestSeriesOnHover
              hoverSourceKey="used-capacity"
              hoverSync={chartHoverSync()}
              highlightSeriesId={summaryFocus.activeSeriesId()}
              interactionState={summaryFocus.interactionStateFor(poolUsedSeries())}
              onHoverSyncChange={setChartHoverSync}
            />
          </SummaryMetricCard>

          <SummaryMetricCard
            label="Available Space"
            secondaryLabel={focusedLabel(poolAvailSeries())}
            headerValue={renderSyncedReadout(poolAvailSyncedReadout())}
            loaded={loaded()}
            hasData={hasPoolAvail()}
            emptyMessage={emptyLabel()}
            interactionState={summaryFocus.interactionStateFor(poolAvailSeries())}
          >
            <InteractiveSparkline
              series={poolAvailSeries()}
              rangeLabel={rangeLabel()}
              timeRange={props.timeRange as TimeRange}
              activeSeriesDisplay="isolate"
              yMode="auto"
              formatValue={(v) => formatBytes(v)}
              formatTopLabel={(max) => formatBytes(max)}
              highlightNearestSeriesOnHover
              hoverSourceKey="available-space"
              hoverSync={chartHoverSync()}
              highlightSeriesId={summaryFocus.activeSeriesId()}
              interactionState={summaryFocus.interactionStateFor(poolAvailSeries())}
              onHoverSyncChange={setChartHoverSync}
            />
          </SummaryMetricCard>
        </SummaryPanel>
      </div>
    </Show>
  );
};

export default StorageSummary;
