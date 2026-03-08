import { Component, Show, createEffect, createMemo, createSignal, onCleanup } from 'solid-js';
import { InteractiveSparkline } from '@/components/shared/InteractiveSparkline';
import type { InteractiveSparklineSeries } from '@/components/shared/InteractiveSparkline';
import { SummaryPanel } from '@/components/shared/SummaryPanel';
import { SummaryMetricCard } from '@/components/shared/SummaryMetricCard';
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
import { RESOURCE_COLORS } from '@/pages/DashboardPanels/resourceColors';
import { formatBytes } from '@/utils/format';
import { getOrgID } from '@/utils/apiClient';
import { eventBus } from '@/stores/events';

// ---------------------------------------------------------------------------
// Cache (org-scoped to prevent cross-tenant data leakage)
// ---------------------------------------------------------------------------

const POLL_INTERVAL_MS = 30_000;
const inMemoryCache = new Map<string, StorageSummaryChartsResponse>();

function inMemoryCacheKey(range: TimeRange, nodeId?: string): string {
  return `${getOrgID() || 'default'}::${range}::${nodeId || '__all__'}`;
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
}

// ---------------------------------------------------------------------------
// Component
// ---------------------------------------------------------------------------

const StorageSummary: Component<StorageSummaryProps> = (props) => {
  const [data, setData] = createSignal<StorageSummaryChartsResponse | null>(null);
  const [loaded, setLoaded] = createSignal(false);
  const [fetchFailed, setFetchFailed] = createSignal(false);

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
            </>
          }
          timeRange={props.timeRange}
          onTimeRangeChange={props.onTimeRangeChange}
        >
          <SummaryMetricCard
            label="Pool Usage"
            loaded={loaded()}
            hasData={hasPoolUsage()}
            emptyMessage={emptyLabel()}
          >
            <InteractiveSparkline
              series={poolUsageSeries()}
              rangeLabel={rangeLabel()}
              timeRange={props.timeRange as TimeRange}
              yMode="percent"
              highlightNearestSeriesOnHover
            />
          </SummaryMetricCard>

          <SummaryMetricCard
            label="Disk Temperature"
            loaded={loaded()}
            hasData={hasDiskTemp()}
            emptyMessage={emptyLabel()}
          >
            <InteractiveSparkline
              series={diskTempSeries()}
              rangeLabel={rangeLabel()}
              timeRange={props.timeRange as TimeRange}
              yMode="auto"
              formatValue={formatTemp}
              formatTopLabel={(max) => `${max.toFixed(0)}°C`}
              highlightNearestSeriesOnHover
            />
          </SummaryMetricCard>

          <SummaryMetricCard
            label="Used Capacity"
            loaded={loaded()}
            hasData={hasPoolUsed()}
            emptyMessage={emptyLabel()}
          >
            <InteractiveSparkline
              series={poolUsedSeries()}
              rangeLabel={rangeLabel()}
              timeRange={props.timeRange as TimeRange}
              yMode="auto"
              formatValue={(v) => formatBytes(v)}
              formatTopLabel={(max) => formatBytes(max)}
              highlightNearestSeriesOnHover
            />
          </SummaryMetricCard>

          <SummaryMetricCard
            label="Available Space"
            loaded={loaded()}
            hasData={hasPoolAvail()}
            emptyMessage={emptyLabel()}
          >
            <InteractiveSparkline
              series={poolAvailSeries()}
              rangeLabel={rangeLabel()}
              timeRange={props.timeRange as TimeRange}
              yMode="auto"
              formatValue={(v) => formatBytes(v)}
              formatTopLabel={(max) => formatBytes(max)}
              highlightNearestSeriesOnHover
            />
          </SummaryMetricCard>
        </SummaryPanel>
      </div>
    </Show>
  );
};

export default StorageSummary;
