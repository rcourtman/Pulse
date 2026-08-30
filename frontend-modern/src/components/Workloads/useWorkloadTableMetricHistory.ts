import { createEffect, createMemo, createSignal, onCleanup, type Accessor } from 'solid-js';

import {
  ChartsAPI,
  type AllMetricsHistoryResponse,
  type ResourceType,
  type SingleMetricHistoryResponse,
  type WorkloadChartsResponse,
} from '@/api/charts';
import type { Node } from '@/types/api';
import type { WorkloadGuest } from '@/types/workloads';
import { createNonSuspendingQuery } from '@/hooks/createNonSuspendingQuery';
import { getCanonicalWorkloadId } from '@/utils/workloads';
import { getWorkloadMetricHistoryTarget } from '@/utils/workloadMetricHistoryTarget';
import {
  fetchInfrastructureSummaryAndCache,
  type InfrastructureSummaryFetchResult,
} from '@/utils/infrastructureSummaryCache';
import { fetchWorkloadsSummaryAndCache } from '@/utils/workloadsSummaryCache';

import {
  findChartDataForCandidates,
  getMetricSparklineSeriesFromChartData,
  getNodeChartKeyCandidates,
  getWorkloadChartKeyCandidates,
  WORKLOAD_TABLE_HISTORY_INFRA_METRICS,
  WORKLOAD_TABLE_HISTORY_MAX_POINTS,
  WORKLOAD_TABLE_HISTORY_POLL_MS,
  type WorkloadMetricHistoryReader,
  type WorkloadTableMetricHistoryRange,
  type WorkloadTableMetric,
} from './workloadMetricHistoryModel';

interface WorkloadTableMetricHistoryOptions {
  activeGuest?: Accessor<WorkloadGuest | null>;
  enabled: Accessor<boolean>;
  onDemand?: Accessor<boolean>;
  prefetchGuests?: Accessor<readonly WorkloadGuest[]>;
  range: Accessor<WorkloadTableMetricHistoryRange>;
  selectedNode: Accessor<string | null | undefined>;
}

interface ActiveGuestHistoryQueryKey {
  guestId: string;
  range: WorkloadTableMetricHistoryRange;
  resourceId: string;
  resourceType: ResourceType;
}

const EMPTY_WORKLOAD_CHARTS_RESPONSE: WorkloadChartsResponse = {
  data: {},
  dockerData: {},
  guestTypes: {},
  timestamp: 0,
  stats: { oldestDataTimestamp: 0 },
};

const EMPTY_INFRASTRUCTURE_CHARTS_RESPONSE: InfrastructureSummaryFetchResult = {
  map: new Map(),
  oldestDataTimestamp: null,
};

const ROW_HISTORY_PREFETCH_CONCURRENCY = 4;
const ROW_HISTORY_INITIAL_PREFETCH_COUNT = 6;
const ROW_HISTORY_ADJACENT_PREFETCH_COUNT = 4;
const ROW_HISTORY_CACHE_MAX_ENTRIES = 18;

const normalizeActiveGuestHistoryResponse = (
  response: AllMetricsHistoryResponse | SingleMetricHistoryResponse,
): AllMetricsHistoryResponse =>
  'metrics' in response
    ? response
    : {
        resourceType: response.resourceType,
        resourceId: response.resourceId,
        range: response.range,
        start: response.start,
        end: response.end,
        metrics: { [response.metric]: response.points },
        source: response.source,
      };

const normalizeNodeScope = (value: string | null | undefined): string => value?.trim() ?? '';
const buildHistoryScope = (range: WorkloadTableMetricHistoryRange, nodeScope = ''): string =>
  `${range}::${nodeScope || '__all__'}`;
const parseHistoryScope = (scope: string) => {
  const [range, nodeScope = '__all__'] = scope.split('::');
  return {
    range: range as WorkloadTableMetricHistoryRange,
    nodeScope,
  };
};

const buildActiveGuestHistoryQueryKey = (
  guest: WorkloadGuest,
  range: WorkloadTableMetricHistoryRange,
): ActiveGuestHistoryQueryKey | null => {
  const target = getWorkloadMetricHistoryTarget(guest);
  if (!target) return null;
  return {
    guestId: getCanonicalWorkloadId(guest),
    range,
    resourceId: target.resourceId,
    resourceType: target.resourceType,
  };
};

const buildActiveGuestHistoryCacheKey = (key: ActiveGuestHistoryQueryKey): string =>
  `${key.resourceType}:${key.resourceId}:${key.range}:${WORKLOAD_TABLE_HISTORY_MAX_POINTS}`;

export function useWorkloadTableMetricHistory(
  options: WorkloadTableMetricHistoryOptions,
): WorkloadMetricHistoryReader {
  const selectedNodeScope = createMemo(() => normalizeNodeScope(options.selectedNode()));
  const workloadHistoryScope = createMemo(() => {
    if (!options.enabled()) return null;
    return buildHistoryScope(options.range(), selectedNodeScope());
  });
  const infrastructureHistoryScope = createMemo(() =>
    options.enabled() ? buildHistoryScope(options.range()) : null,
  );

  const workloadHistory = createNonSuspendingQuery<WorkloadChartsResponse, string>({
    source: workloadHistoryScope,
    fetcher: (scope, signal) => {
      const parsed = parseHistoryScope(scope);
      return fetchWorkloadsSummaryAndCache(parsed.range, {
        caller: 'WorkloadTableMetricHistory',
        maxPoints: WORKLOAD_TABLE_HISTORY_MAX_POINTS,
        nodeId: parsed.nodeScope === '__all__' ? null : parsed.nodeScope,
        signal,
      });
    },
    initialValue: EMPTY_WORKLOAD_CHARTS_RESPONSE,
    cacheKey: (scope) => `workload-table-history:${scope}`,
    pollMs: WORKLOAD_TABLE_HISTORY_POLL_MS,
    retainPreviousValueOnSourceChange: false,
  });

  const infrastructureHistory = createNonSuspendingQuery<InfrastructureSummaryFetchResult, string>({
    source: infrastructureHistoryScope,
    fetcher: (scope, signal) => {
      const parsed = parseHistoryScope(scope);
      return fetchInfrastructureSummaryAndCache(parsed.range, {
        caller: 'WorkloadTableMetricHistory',
        metrics: WORKLOAD_TABLE_HISTORY_INFRA_METRICS,
        signal,
      });
    },
    initialValue: EMPTY_INFRASTRUCTURE_CHARTS_RESPONSE,
    cacheKey: (scope) => `workload-table-infra-history:${scope}`,
    pollMs: WORKLOAD_TABLE_HISTORY_POLL_MS,
    retainPreviousValueOnSourceChange: false,
  });

  const [rowHistoryRevision, setRowHistoryRevision] = createSignal(0);
  const rowHistoryCache = new Map<string, AllMetricsHistoryResponse>();
  const pendingRowHistory: ActiveGuestHistoryQueryKey[] = [];
  const inFlightRowHistory = new Map<string, { controller: AbortController; generation: number }>();
  let rowHistoryGeneration = 0;
  let scheduledRange: WorkloadTableMetricHistoryRange | null = null;
  let rowHistoryWarmingActive = false;
  let disposed = false;

  const cancelRowHistoryQueue = () => {
    rowHistoryGeneration += 1;
    pendingRowHistory.splice(0, pendingRowHistory.length);
    inFlightRowHistory.forEach(({ controller }) => controller.abort());
    inFlightRowHistory.clear();
  };

  const trimRowHistoryCache = () => {
    while (rowHistoryCache.size > ROW_HISTORY_CACHE_MAX_ENTRIES) {
      const oldestKey = rowHistoryCache.keys().next().value as string | undefined;
      if (!oldestKey) return;
      rowHistoryCache.delete(oldestKey);
    }
  };

  const pumpRowHistoryQueue = () => {
    while (
      !disposed &&
      inFlightRowHistory.size < ROW_HISTORY_PREFETCH_CONCURRENCY &&
      pendingRowHistory.length > 0
    ) {
      const key = pendingRowHistory.shift();
      if (!key) break;
      const cacheKey = buildActiveGuestHistoryCacheKey(key);
      if (rowHistoryCache.has(cacheKey) || inFlightRowHistory.has(cacheKey)) continue;

      const controller = new AbortController();
      const generation = rowHistoryGeneration;
      inFlightRowHistory.set(cacheKey, { controller, generation });
      void ChartsAPI.getMetricsHistory({
        resourceType: key.resourceType,
        resourceId: key.resourceId,
        range: key.range,
        maxPoints: WORKLOAD_TABLE_HISTORY_MAX_POINTS,
        signal: controller.signal,
      })
        .then((response) => {
          if (disposed || generation !== rowHistoryGeneration || controller.signal.aborted) return;
          rowHistoryCache.set(cacheKey, normalizeActiveGuestHistoryResponse(response));
          trimRowHistoryCache();
          setRowHistoryRevision((revision) => revision + 1);
        })
        .catch(() => {
          // Hover warming is opportunistic. The live bars remain available if
          // history is missing or the request is cancelled by a range change.
        })
        .finally(() => {
          const current = inFlightRowHistory.get(cacheKey);
          if (current?.generation === generation) {
            inFlightRowHistory.delete(cacheKey);
          }
          pumpRowHistoryQueue();
        });
    }
  };

  const enqueueRowHistory = (keys: ActiveGuestHistoryQueryKey[], prioritize: boolean) => {
    const uniqueKeys = keys.filter((key, index) => {
      const cacheKey = buildActiveGuestHistoryCacheKey(key);
      return (
        keys.findIndex((candidate) => buildActiveGuestHistoryCacheKey(candidate) === cacheKey) ===
          index &&
        !rowHistoryCache.has(cacheKey) &&
        !inFlightRowHistory.has(cacheKey)
      );
    });
    if (uniqueKeys.length === 0) return;

    const uniqueCacheKeys = new Set(uniqueKeys.map(buildActiveGuestHistoryCacheKey));
    for (let index = pendingRowHistory.length - 1; index >= 0; index -= 1) {
      if (uniqueCacheKeys.has(buildActiveGuestHistoryCacheKey(pendingRowHistory[index]))) {
        pendingRowHistory.splice(index, 1);
      }
    }
    if (prioritize) {
      pendingRowHistory.unshift(...uniqueKeys);
    } else {
      pendingRowHistory.push(...uniqueKeys);
    }
    pumpRowHistoryQueue();
  };

  createEffect(() => {
    const range = options.range();
    const onDemand = options.onDemand?.() ?? false;
    const candidates = options.prefetchGuests?.() ?? [];
    const activeGuest = options.activeGuest?.() ?? null;

    if (scheduledRange !== range) {
      cancelRowHistoryQueue();
      rowHistoryCache.clear();
      setRowHistoryRevision((revision) => revision + 1);
      scheduledRange = range;
    }

    if (!onDemand) {
      if (rowHistoryWarmingActive) cancelRowHistoryQueue();
      rowHistoryWarmingActive = false;
      return;
    }
    rowHistoryWarmingActive = true;

    if (!activeGuest) {
      enqueueRowHistory(
        candidates
          .slice(0, ROW_HISTORY_INITIAL_PREFETCH_COUNT)
          .map((guest) => buildActiveGuestHistoryQueryKey(guest, range))
          .filter((key): key is ActiveGuestHistoryQueryKey => key !== null),
        false,
      );
      return;
    }

    const activeId = getCanonicalWorkloadId(activeGuest);
    const activeIndex = candidates.findIndex((guest) => getCanonicalWorkloadId(guest) === activeId);
    const adjacentGuests =
      activeIndex >= 0
        ? [
            activeGuest,
            ...candidates.slice(
              activeIndex + 1,
              activeIndex + 1 + ROW_HISTORY_ADJACENT_PREFETCH_COUNT,
            ),
            ...(activeIndex > 0 ? [candidates[activeIndex - 1]] : []),
          ]
        : [activeGuest];
    enqueueRowHistory(
      adjacentGuests
        .map((guest) => buildActiveGuestHistoryQueryKey(guest, range))
        .filter((key): key is ActiveGuestHistoryQueryKey => key !== null),
      true,
    );
  });

  onCleanup(() => {
    disposed = true;
    cancelRowHistoryQueue();
  });

  const getCachedGuestHistory = (guest: WorkloadGuest): AllMetricsHistoryResponse | null => {
    rowHistoryRevision();
    const key = buildActiveGuestHistoryQueryKey(guest, options.range());
    return key ? (rowHistoryCache.get(buildActiveGuestHistoryCacheKey(key)) ?? null) : null;
  };

  const getGuestMetricSeries: WorkloadMetricHistoryReader['getGuestMetricSeries'] = (
    guest,
    metric,
    seriesOptions,
  ) => {
    const activeGuest = options.onDemand?.() ? options.activeGuest?.() : null;
    if (activeGuest) {
      if (getCanonicalWorkloadId(activeGuest) !== getCanonicalWorkloadId(guest)) {
        return [];
      }
      return getMetricSparklineSeriesFromChartData(
        getCachedGuestHistory(guest)?.metrics,
        metric,
        seriesOptions,
      );
    }

    const response = workloadHistory.value();
    const chartData = findChartDataForCandidates(getWorkloadChartKeyCandidates(guest), [
      response.data,
      response.dockerData,
    ]);
    return getMetricSparklineSeriesFromChartData(chartData, metric, seriesOptions);
  };

  const getNodeMetricSeries = (node: Node, metric: WorkloadTableMetric) => {
    const chartData = findChartDataForCandidates(getNodeChartKeyCandidates(node), [
      infrastructureHistory.value().map,
    ]);
    return getMetricSparklineSeriesFromChartData(chartData, metric);
  };

  return {
    hasGuestHistory: (guest) => {
      const response = getCachedGuestHistory(guest);
      return (
        response !== null && Object.values(response.metrics).some((points) => points.length > 0)
      );
    },
    getGuestMetricSeries,
    getNodeMetricSeries,
  };
}
