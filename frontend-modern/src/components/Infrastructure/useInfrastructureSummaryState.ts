import { createEffect, createMemo, createSignal, onCleanup } from 'solid-js';
import type { ChartData, TimeRange } from '@/api/charts';
import { useResources } from '@/hooks/useResources';
import {
  fetchInfrastructureSummaryAndCache,
  readInfrastructureSummaryCache,
} from '@/utils/infrastructureSummaryCache';
import { isAgentFacetInfrastructureResource } from '@/utils/agentResources';
import { getOrgID } from '@/utils/apiClient';
import { normalizeOrgScope } from '@/utils/orgScope';
import { eventBus } from '@/stores/events';
import {
  type InfrastructureSummaryProps,
  type InfrastructureSummarySparkMetric,
  buildInfrastructureDisplaySeries,
  buildInfrastructureEmptyHistoryLabel,
  buildInfrastructureEmptyMessage,
  buildInfrastructureMetricSeries,
  buildInfrastructureResourceCounts,
  buildInfrastructureSummarySeries,
  buildInfrastructureWorkloadStats,
  getFocusedInfrastructureResourceName,
  getAverageDiskCapacity,
  getSingleDisplayedOnlineInfrastructureResource,
  hasInfrastructureSeriesData,
  isInfrastructureAwaitingFirstSample,
  shouldShowInfrastructureNetworkCard,
} from './infrastructureSummaryModel';

// In-memory full-resolution cache keyed by "org::range".
// Survives component unmount/remount (page navigation) without the
// downsampling artifacts that localStorage cache introduces.
// Capped at MAX_IN_MEMORY_INFRA_ENTRIES to prevent unbounded growth.
const MAX_IN_MEMORY_INFRA_ENTRIES = 20;
const inMemoryChartCache = new Map<string, Map<string, ChartData>>();

function inMemoryCacheKey(range: TimeRange): string {
  return `${normalizeOrgScope(getOrgID())}::${range}`;
}

/** @internal Test-only reset for in-memory chart cache. */
export function __resetInMemoryChartCacheForTests(): void {
  inMemoryChartCache.clear();
}

const unsubscribeInfraOrgSwitch = eventBus.on('org_switched', () => {
  inMemoryChartCache.clear();
});

if (import.meta.hot) {
  import.meta.hot.dispose(() => {
    unsubscribeInfraOrgSwitch();
  });
}

export function useInfrastructureSummaryState(props: InfrastructureSummaryProps) {
  const [chartMap, setChartMap] = createSignal<Map<string, ChartData>>(new Map());
  const [chartRange, setChartRange] = createSignal<TimeRange | null>(null);
  const [loadedRange, setLoadedRange] = createSignal<TimeRange | null>(null);
  const [oldestDataTimestamp, setOldestDataTimestamp] = createSignal<number | null>(null);
  const [fetchFailed, setFetchFailed] = createSignal(false);
  const selectedRange = createMemo<TimeRange>(() => props.timeRange || '1h');
  const hasCurrentRangeCharts = createMemo(() => chartRange() === selectedRange());
  const isCurrentRangeLoaded = createMemo(() => loadedRange() === selectedRange());

  const { workloads, resources } = useResources();
  const agentResources = createMemo(() =>
    resources().filter((resource) => isAgentFacetInfrastructureResource(resource)),
  );

  const [orgVersion, setOrgVersion] = createSignal(0);
  const unsubscribeOrgSwitch = eventBus.on('org_switched', () => {
    setOrgVersion((value) => value + 1);
  });

  let refreshTimer: ReturnType<typeof setInterval> | undefined;
  let activeFetchController: AbortController | null = null;
  let activeFetchRequest = 0;
  let activeScopeKey: string | null = null;

  const awaitAbortable = <T,>(promise: Promise<T>, signal: AbortSignal): Promise<T> => {
    if (signal.aborted) {
      return Promise.reject(new DOMException('Aborted', 'AbortError'));
    }
    return new Promise<T>((resolve, reject) => {
      const onAbort = () => {
        reject(new DOMException('Aborted', 'AbortError'));
      };
      signal.addEventListener('abort', onAbort, { once: true });
      promise.then(
        (value) => {
          signal.removeEventListener('abort', onAbort);
          resolve(value);
        },
        (error) => {
          signal.removeEventListener('abort', onAbort);
          reject(error);
        },
      );
    });
  };

  const fetchCharts = async (options?: { prioritize?: boolean }) => {
    if (props.resources.length === 0) {
      return;
    }

    const prioritize = options?.prioritize === true;
    if (activeFetchController && !prioritize) {
      return;
    }
    if (activeFetchController && prioritize) {
      activeFetchController.abort();
    }

    const requestedRange = selectedRange();
    const controller = new AbortController();
    const requestId = ++activeFetchRequest;
    activeFetchController = controller;

    try {
      const fetched = await awaitAbortable(
        fetchInfrastructureSummaryAndCache(requestedRange, { caller: 'InfrastructureSummary' }),
        controller.signal,
      );
      if (requestId !== activeFetchRequest) {
        return;
      }
      const fetchedMap = fetched.map;
      const currentMapMatchesRequestedRange = chartRange() === requestedRange;
      if (fetchedMap.size > 0 || chartMap().size === 0 || !currentMapMatchesRequestedRange) {
        const cacheKey = inMemoryCacheKey(requestedRange);
        if (
          !inMemoryChartCache.has(cacheKey) &&
          inMemoryChartCache.size >= MAX_IN_MEMORY_INFRA_ENTRIES
        ) {
          const oldest = inMemoryChartCache.keys().next().value;
          if (oldest !== undefined) inMemoryChartCache.delete(oldest);
        }
        inMemoryChartCache.set(cacheKey, fetchedMap);
        setChartMap(fetchedMap);
        setChartRange(requestedRange);
      }
      setOldestDataTimestamp(fetched.oldestDataTimestamp);
      setFetchFailed(false);
    } catch (error) {
      if (error instanceof Error && error.name === 'AbortError') {
        return;
      }
      if (requestId === activeFetchRequest) {
        setFetchFailed(true);
        if (chartRange() !== requestedRange) {
          const cached = readInfrastructureSummaryCache(requestedRange);
          if (cached) {
            setChartMap(cached.map);
            setChartRange(requestedRange);
            setOldestDataTimestamp(cached.oldestDataTimestamp);
          }
        }
      }
    } finally {
      if (activeFetchController === controller) {
        activeFetchController = null;
      }
      if (requestId === activeFetchRequest) {
        setLoadedRange(requestedRange);
      }
    }
  };

  createEffect(() => {
    const hasResources = props.resources.length > 0;
    const currentOrg = orgVersion();
    if (!hasResources) {
      if (refreshTimer) {
        clearInterval(refreshTimer);
        refreshTimer = undefined;
      }
      if (activeFetchController) {
        activeFetchController.abort();
        activeFetchController = null;
      }
      activeScopeKey = null;
      setChartMap(new Map());
      setChartRange(null);
      setLoadedRange(null);
      setOldestDataTimestamp(null);
      return;
    }

    if (!refreshTimer) {
      refreshTimer = setInterval(() => void fetchCharts(), 30_000);
    }

    const nextRange = selectedRange();
    const nextScopeKey = `${currentOrg}::${nextRange}`;
    if (activeScopeKey !== nextScopeKey) {
      activeScopeKey = nextScopeKey;
      const inMemoryCached = inMemoryChartCache.get(inMemoryCacheKey(nextRange));
      if (inMemoryCached && inMemoryCached.size > 0) {
        setChartMap(inMemoryCached);
        setChartRange(nextRange);
        setLoadedRange(nextRange);
        const cached = readInfrastructureSummaryCache(nextRange);
        setOldestDataTimestamp(cached?.oldestDataTimestamp ?? null);
      } else {
        setChartMap(new Map());
        setChartRange(null);
        setLoadedRange(null);
        setOldestDataTimestamp(null);
      }
      setFetchFailed(false);
      void fetchCharts({ prioritize: true });
    }
  });

  onCleanup(() => {
    if (refreshTimer) clearInterval(refreshTimer);
    if (activeFetchController) {
      activeFetchController.abort();
      activeFetchController = null;
    }
    unsubscribeOrgSwitch();
  });

  const resourceSeries = createMemo(() =>
    hasCurrentRangeCharts()
      ? buildInfrastructureSummarySeries(props.resources, chartMap(), agentResources())
      : buildInfrastructureSummarySeries(props.resources, new Map(), agentResources()),
  );

  const displaySeries = createMemo(() => {
    return buildInfrastructureDisplaySeries(resourceSeries(), props.focusedResourceId);
  });

  const focusedResourceName = createMemo(() => {
    return getFocusedInfrastructureResourceName(resourceSeries(), props.focusedResourceId);
  });

  const singleDisplayedOnlineResource = createMemo(() => {
    return getSingleDisplayedOnlineInfrastructureResource(props.resources, displaySeries());
  });

  const isAwaitingFirstSample = createMemo(() => {
    return isInfrastructureAwaitingFirstSample({
      resource: singleDisplayedOnlineResource(),
      isCurrentRangeLoaded: isCurrentRangeLoaded(),
      fetchFailed: fetchFailed(),
      oldestDataTimestamp: oldestDataTimestamp(),
    });
  });

  const emptyHistoryLabel = createMemo(() =>
    buildInfrastructureEmptyHistoryLabel(isAwaitingFirstSample()),
  );

  const hasData = (metric: InfrastructureSummarySparkMetric) =>
    hasInfrastructureSeriesData(displaySeries(), metric);

  const networkSeries = createMemo(() =>
    buildInfrastructureMetricSeries(displaySeries(), 'network'),
  );

  const hasNetData = createMemo(() => hasInfrastructureSeriesData(displaySeries(), 'network'));

  const diskioSeries = createMemo(() =>
    buildInfrastructureMetricSeries(displaySeries(), 'diskio'),
  );

  const hasDiskIOData = createMemo(() => hasInfrastructureSeriesData(displaySeries(), 'diskio'));

  const avgDiskCapacity = createMemo(() => getAverageDiskCapacity(props.resources));
  const shouldShowNetworkCard = createMemo(
    () => shouldShowInfrastructureNetworkCard(hasNetData(), props.resources),
  );
  const workloadStats = createMemo(() => buildInfrastructureWorkloadStats(workloads()));
  const resourceCounts = createMemo(() => buildInfrastructureResourceCounts(props.resources));
  const emptyMessage = createMemo(() =>
    buildInfrastructureEmptyMessage(fetchFailed(), emptyHistoryLabel()),
  );
  const seriesFor = (metric: InfrastructureSummarySparkMetric) =>
    buildInfrastructureMetricSeries(displaySeries(), metric);

  return {
    selectedRange,
    isCurrentRangeLoaded,
    emptyHistoryLabel,
    emptyMessage,
    focusedResourceName,
    resourceCounts,
    workloadStats,
    avgDiskCapacity,
    shouldShowNetworkCard,
    networkSeries,
    hasNetData,
    diskioSeries,
    hasDiskIOData,
    hasData,
    seriesFor,
  };
}

export type InfrastructureSummaryState = ReturnType<typeof useInfrastructureSummaryState>;
