import { createMemo, type Accessor } from 'solid-js';

import type { WorkloadChartsResponse } from '@/api/charts';
import type { Node } from '@/types/api';
import type { WorkloadGuest } from '@/types/workloads';
import { createNonSuspendingQuery } from '@/hooks/createNonSuspendingQuery';
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
  enabled: Accessor<boolean>;
  range: Accessor<WorkloadTableMetricHistoryRange>;
  selectedNode: Accessor<string | null | undefined>;
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
    fetcher: (scope) => {
      const parsed = parseHistoryScope(scope);
      return fetchWorkloadsSummaryAndCache(parsed.range, {
        caller: 'WorkloadTableMetricHistory',
        maxPoints: WORKLOAD_TABLE_HISTORY_MAX_POINTS,
        nodeId: parsed.nodeScope === '__all__' ? null : parsed.nodeScope,
      });
    },
    initialValue: EMPTY_WORKLOAD_CHARTS_RESPONSE,
    cacheKey: (scope) => `workload-table-history:${scope}`,
    pollMs: WORKLOAD_TABLE_HISTORY_POLL_MS,
  });

  const infrastructureHistory = createNonSuspendingQuery<InfrastructureSummaryFetchResult, string>({
    source: infrastructureHistoryScope,
    fetcher: (scope) => {
      const parsed = parseHistoryScope(scope);
      return fetchInfrastructureSummaryAndCache(parsed.range, {
        caller: 'WorkloadTableMetricHistory',
        metrics: WORKLOAD_TABLE_HISTORY_INFRA_METRICS,
      });
    },
    initialValue: EMPTY_INFRASTRUCTURE_CHARTS_RESPONSE,
    cacheKey: (scope) => `workload-table-infra-history:${scope}`,
    pollMs: WORKLOAD_TABLE_HISTORY_POLL_MS,
  });

  const getGuestMetricSeries = (guest: WorkloadGuest, metric: WorkloadTableMetric) => {
    const response = workloadHistory.value();
    const chartData = findChartDataForCandidates(getWorkloadChartKeyCandidates(guest), [
      response.data,
      response.dockerData,
    ]);
    return getMetricSparklineSeriesFromChartData(chartData, metric);
  };

  const getNodeMetricSeries = (node: Node, metric: WorkloadTableMetric) => {
    const chartData = findChartDataForCandidates(getNodeChartKeyCandidates(node), [
      infrastructureHistory.value().map,
    ]);
    return getMetricSparklineSeriesFromChartData(chartData, metric);
  };

  return {
    getGuestMetricSeries,
    getNodeMetricSeries,
  };
}
