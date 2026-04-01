import type { ChartData, MetricPoint, TimeRange } from '@/api/charts';
import {
  getActionableAgentIdFromResource,
  getMetricsChartKeyCandidatesFromResource,
  getPlatformAgentRecord,
  getPlatformDataRecord,
  hasAgentFacet,
} from '@/utils/agentResources';
import {
  getPreferredInfrastructureDisplayName,
  getPreferredResourceHostname,
  getResourceIdentityAliases,
  getNormalizedIdentityLookupVariants,
} from '@/utils/resourceIdentity';
import { asTrimmedString } from '@/utils/stringUtils';
import { getChartSeriesColor } from '@/utils/chartSeriesPresentation';
import type { Resource } from '@/types/resource';
import { getDiskPercent } from '@/types/resource';
import type { SummaryChartHoverSync } from '@/components/shared/contextualFocus';

export interface InfrastructureSummaryProps {
  resources: Resource[];
  timeRange?: TimeRange;
  hoveredResourceId?: string | null;
  focusedResourceId?: string | null;
  chartHoverSync?: SummaryChartHoverSync | null;
  onTimeRangeChange?: (range: TimeRange) => void;
  onChartHoverSyncChange?: (value: SummaryChartHoverSync | null) => void;
  showJumpToActiveRow?: boolean;
  onJumpToActiveRow?: () => void;
}

export interface InfrastructureSummarySeries {
  key: string;
  id: string;
  cpu: MetricPoint[];
  memory: MetricPoint[];
  disk: MetricPoint[];
  netin: MetricPoint[];
  netout: MetricPoint[];
  network: MetricPoint[];
  diskio: MetricPoint[];
  color: string;
  name: string;
}

export type InfrastructureSummarySparkMetric = 'cpu' | 'memory' | 'disk';

export interface InfrastructureWorkloadStats {
  total: number;
  running: number;
  stopped: number;
  vms: number;
  containers: number;
}

export interface InfrastructureResourceCounts {
  total: number;
  online: number;
  offline: number;
}

export interface InfrastructureSummaryMetricSeriesEntry {
  id: string;
  data: MetricPoint[];
  color: string;
  name: string;
}

const getNormalizedResourceIdentifiers = (resource: Resource): Set<string> =>
  new Set<string>([
    ...getNormalizedIdentityLookupVariants(resource.id),
    ...getNormalizedIdentityLookupVariants(resource.platformId),
    ...getNormalizedIdentityLookupVariants(getPreferredInfrastructureDisplayName(resource)),
    ...getNormalizedIdentityLookupVariants(getPreferredResourceHostname(resource)),
    ...getResourceIdentityAliases(resource).flatMap((value) =>
      getNormalizedIdentityLookupVariants(value),
    ),
  ]);

const getLinkedNodeIdFromResource = (resource: Resource): string | null =>
  asTrimmedString(getPlatformDataRecord(resource)?.linkedNodeId) ||
  asTrimmedString(getPlatformAgentRecord(resource)?.linkedNodeId) ||
  null;

// Combine a resource's net in/out into a single throughput series.
// Buckets points into 30-second windows and sums rates from both directions.
export function combineResourceThroughputSeries(
  inSeries: MetricPoint[],
  outSeries: MetricPoint[],
): MetricPoint[] {
  const bucketSize = 30_000; // 30 seconds
  const buckets = new Map<number, number>();
  for (const point of inSeries) {
    const bucket = Math.round(point.timestamp / bucketSize) * bucketSize;
    buckets.set(bucket, (buckets.get(bucket) || 0) + point.value);
  }
  for (const point of outSeries) {
    const bucket = Math.round(point.timestamp / bucketSize) * bucketSize;
    buckets.set(bucket, (buckets.get(bucket) || 0) + point.value);
  }

  return Array.from(buckets.entries())
    .sort((left, right) => left[0] - right[0])
    .map(([timestamp, value]) => ({ timestamp, value }));
}

export function findInfrastructureChartData(
  resource: Resource,
  chartMap: Map<string, ChartData>,
): ChartData | undefined {
  if (chartMap.size === 0) return undefined;

  for (const key of getMetricsChartKeyCandidatesFromResource(resource)) {
    const match = chartMap.get(key);
    if (match) return match;
  }

  if (resource.clusterId && resource.platformId) {
    const clusterKey = `${resource.clusterId}-${resource.platformId}`;
    const clusterMatch = chartMap.get(clusterKey);
    if (clusterMatch) return clusterMatch;
  }

  const nameToMatch =
    getPreferredResourceHostname(resource) || resource.platformId || resource.name;
  if (nameToMatch) {
    const suffix = `-${nameToMatch}`;
    for (const [key, data] of chartMap) {
      if (key.endsWith(suffix)) return data;
    }
  }

  return undefined;
}

export function findInfrastructureAgentChartData(
  resource: Resource,
  chartMap: Map<string, ChartData>,
  agentFacetResources: Resource[],
): ChartData | undefined {
  if (chartMap.size === 0) return undefined;

  const directAgentCandidates: string[] = [];
  const actionableAgentId = getActionableAgentIdFromResource(resource);
  if (actionableAgentId) {
    directAgentCandidates.push(actionableAgentId);
  }
  if (resource.platformType === 'agent') {
    const discoveryResourceId = asTrimmedString(resource.discoveryTarget?.resourceId);
    const discoveryHostId = asTrimmedString(resource.discoveryTarget?.agentId);
    if (discoveryResourceId) directAgentCandidates.push(discoveryResourceId);
    if (discoveryHostId) directAgentCandidates.push(discoveryHostId);
  }

  for (const key of Array.from(new Set(directAgentCandidates))) {
    const direct = chartMap.get(key);
    if (direct) return direct;
  }

  if (agentFacetResources.length === 0) return undefined;

  const nodeRefCandidates = new Set<string>(
    [resource.id, resource.platformId, getPreferredResourceHostname(resource)]
      .map((value) => value?.trim().toLowerCase())
      .filter((value): value is string => Boolean(value)),
  );
  const resourceNameCandidates = getNormalizedResourceIdentifiers(resource);

  for (const agentResource of agentFacetResources) {
    const linkedNodeId = getLinkedNodeIdFromResource(agentResource);
    const normalizedLinkedNodeId = linkedNodeId?.toLowerCase();

    const linkedMatch = normalizedLinkedNodeId
      ? nodeRefCandidates.has(normalizedLinkedNodeId)
      : false;
    const agentResourceNames = Array.from(getNormalizedResourceIdentifiers(agentResource));
    const hostnameMatch = agentResourceNames.some((candidate) =>
      resourceNameCandidates.has(candidate),
    );

    if (linkedMatch || hostnameMatch) {
      for (const key of getMetricsChartKeyCandidatesFromResource(agentResource)) {
        const agentData = chartMap.get(key);
        if (agentData) return agentData;
      }
    }
  }
  return undefined;
}

export function buildInfrastructureSummarySeries(
  resources: Resource[],
  chartMap: Map<string, ChartData>,
  agentFacetResources: Resource[],
): InfrastructureSummarySeries[] {
  return resources.map((resource, index) => {
    const primaryData = findInfrastructureChartData(resource, chartMap);
    const agentData = findInfrastructureAgentChartData(resource, chartMap, agentFacetResources);
    const seriesId = resource.id || resource.platformId || resource.name || `resource-${index}`;

    const metricSeries = (metric: keyof ChartData): MetricPoint[] => {
      const primary = primaryData?.[metric];
      if (primary && primary.length > 0) return primary;
      const fallback = agentData?.[metric];
      if (fallback && fallback.length > 0) return fallback;
      return [];
    };

    return {
      key: seriesId,
      id: seriesId,
      cpu: metricSeries('cpu'),
      memory: metricSeries('memory'),
      disk: metricSeries('disk'),
      netin: metricSeries('netin'),
      netout: metricSeries('netout'),
      network: combineResourceThroughputSeries(metricSeries('netin'), metricSeries('netout')),
      diskio: combineResourceThroughputSeries(
        metricSeries('diskread'),
        metricSeries('diskwrite'),
      ),
      color: getChartSeriesColor(index),
      name: getPreferredInfrastructureDisplayName(resource),
    };
  });
}

export function buildInfrastructureDisplaySeries(
  allSeries: InfrastructureSummarySeries[],
  focusedResourceId?: string | null,
): InfrastructureSummarySeries[] {
  if (!focusedResourceId) return allSeries;
  const match = allSeries.find((series) => series.id === focusedResourceId);
  return match ? [match] : allSeries;
}

export function getFocusedInfrastructureResourceName(
  allSeries: InfrastructureSummarySeries[],
  focusedResourceId?: string | null,
): string | null {
  if (!focusedResourceId) return null;
  const match = allSeries.find((series) => series.id === focusedResourceId);
  return match?.name ?? null;
}

export function getSingleDisplayedOnlineInfrastructureResource(
  resources: Resource[],
  displaySeries: InfrastructureSummarySeries[],
): Resource | null {
  if (displaySeries.length !== 1 || resources.length !== 1) return null;
  const [resource] = resources;
  if (!resource) return null;
  return resource.status?.toLowerCase() === 'online' ? resource : null;
}

export function isInfrastructureAwaitingFirstSample(options: {
  resource: Resource | null;
  isCurrentRangeLoaded: boolean;
  fetchFailed: boolean;
  oldestDataTimestamp: number | null;
}): boolean {
  if (
    !options.resource ||
    !options.isCurrentRangeLoaded ||
    options.fetchFailed
  ) {
    return false;
  }

  if (options.oldestDataTimestamp === null) {
    return true;
  }

  return options.resource.lastSeen >= options.oldestDataTimestamp;
}

export function buildInfrastructureEmptyHistoryLabel(isAwaitingFirstSample: boolean): string {
  return isAwaitingFirstSample ? 'Waiting for first sample' : 'No history yet';
}

export function buildInfrastructureEmptyMessage(
  fetchFailed: boolean,
  emptyHistoryLabel: string,
): string {
  return fetchFailed ? 'Trend data unavailable' : emptyHistoryLabel;
}

export function buildInfrastructureMetricSeries(
  displaySeries: InfrastructureSummarySeries[],
  metric: InfrastructureSummarySparkMetric | 'network' | 'diskio',
): InfrastructureSummaryMetricSeriesEntry[] {
  return displaySeries.map((series) => ({
    id: series.id,
    data: series[metric],
    color: series.color,
    name: series.name,
  }));
}

export function hasInfrastructureSeriesData(
  displaySeries: InfrastructureSummarySeries[],
  metric: InfrastructureSummarySparkMetric | 'network' | 'diskio',
): boolean {
  return displaySeries.some((series) => series[metric].length >= 1);
}

export function buildInfrastructureWorkloadStats(
  workloads: Resource[],
): InfrastructureWorkloadStats {
  let running = 0;
  let stopped = 0;
  let vms = 0;
  let containers = 0;
  for (const workload of workloads) {
    if (workload.status === 'running' || workload.status === 'online') {
      running++;
    } else {
      stopped++;
    }
    if (workload.type === 'vm') {
      vms++;
    } else {
      containers++;
    }
  }
  return { total: workloads.length, running, stopped, vms, containers };
}

export function buildInfrastructureResourceCounts(
  resources: Resource[],
): InfrastructureResourceCounts {
  const total = resources.length;
  const online = resources.filter((resource) => resource.status === 'online').length;
  return {
    total,
    online,
    offline: total - online,
  };
}

export function getAverageDiskCapacity(resources: Resource[]): number | null {
  const diskResources = resources.filter((resource) => resource.disk && resource.disk.total);
  if (diskResources.length === 0) return null;
  const average =
    diskResources.reduce((sum, resource) => sum + getDiskPercent(resource), 0) /
    diskResources.length;
  return Math.round(average);
}

export function hasInfrastructureNetworkCapability(resources: Resource[]): boolean {
  return resources.some((resource) => {
    if (resource.type === 'docker-host' || hasAgentFacet(resource)) return true;
    const rx = resource.network?.rxBytes ?? 0;
    const tx = resource.network?.txBytes ?? 0;
    return rx > 0 || tx > 0;
  });
}

export function shouldShowInfrastructureNetworkCard(
  hasNetworkData: boolean,
  resources: Resource[],
): boolean {
  return hasNetworkData || hasInfrastructureNetworkCapability(resources);
}
