import type { Resource } from '@/types/resource';
import { getCpuPercent, getDiskPercent, getDisplayName, getMemoryPercent } from '@/types/resource';

export interface IODistributionStats {
  median: number;
  mad: number;
  max: number;
  p97: number;
  p99: number;
  count: number;
}

export interface OutlierEmphasis {
  fontWeight: string;
  color: string;
  showOutlierHint: boolean;
}

export interface ResourceGroup {
  cluster: string;
  resources: Resource[];
}

const statusLabels: Record<string, string> = {
  online: 'Online',
  offline: 'Offline',
  degraded: 'Degraded',
  paused: 'Paused',
  unknown: 'Unknown',
  running: 'Running',
  stopped: 'Stopped',
};

const statusOrder = ['online', 'degraded', 'paused', 'offline', 'stopped', 'unknown', 'running'];

const isServiceInfrastructureResource = (resource: Resource) =>
  resource.type === 'pbs' || resource.type === 'pmg';

const isResourceOnline = (resource: Resource) => {
  const status = resource.status?.toLowerCase();
  return status !== 'offline' && status !== 'stopped';
};

const computeMedian = (values: number[]): number => {
  if (values.length === 0) return 0;
  const sorted = [...values].sort((a, b) => a - b);
  const mid = Math.floor(sorted.length / 2);
  if (sorted.length % 2 === 0) {
    return (sorted[mid - 1] + sorted[mid]) / 2;
  }
  return sorted[mid];
};

const computePercentile = (values: number[], percentile: number): number => {
  if (values.length === 0) return 0;
  const sorted = [...values].sort((a, b) => a - b);
  const clamped = Math.max(0, Math.min(1, percentile));
  const index = Math.max(0, Math.min(sorted.length - 1, Math.ceil(clamped * sorted.length) - 1));
  return sorted[index];
};

const buildIODistribution = (values: number[]): IODistributionStats => {
  const valid = values.filter((value) => Number.isFinite(value) && value >= 0);
  if (valid.length === 0) {
    return { median: 0, mad: 0, max: 0, p97: 0, p99: 0, count: 0 };
  }

  const median = computeMedian(valid);
  const deviations = valid.map((value) => Math.abs(value - median));
  const mad = computeMedian(deviations);
  const max = Math.max(...valid, 0);
  const p97 = computePercentile(valid, 0.97);
  const p99 = computePercentile(valid, 0.99);

  return { median, mad, max, p97, p99, count: valid.length };
};

function normalizeSource(value: string): string | null {
  const normalized = value.toLowerCase();
  switch (normalized) {
    case 'pve':
    case 'proxmox':
    case 'proxmox-pve':
      return 'proxmox';
    case 'agent':
    case 'host-agent':
      return 'agent';
    case 'docker':
      return 'docker';
    case 'pbs':
    case 'proxmox-pbs':
      return 'pbs';
    case 'pmg':
    case 'proxmox-pmg':
      return 'pmg';
    case 'k8s':
    case 'kubernetes':
      return 'kubernetes';
    case 'truenas':
      return 'truenas';
    default:
      return null;
  }
}

const getSortValue = (resource: Resource, key: string): number | string | null => {
  switch (key) {
    case 'name':
      return getDisplayName(resource);
    case 'uptime':
      return resource.uptime ?? 0;
    case 'cpu':
      return resource.cpu ? getCpuPercent(resource) : null;
    case 'memory':
      return resource.memory ? getMemoryPercent(resource) : null;
    case 'disk':
      return resource.disk ? getDiskPercent(resource) : null;
    case 'network':
      return resource.network ? (resource.network.rxBytes + resource.network.txBytes) : null;
    case 'diskio':
      return resource.diskIO ? (resource.diskIO.readRate + resource.diskIO.writeRate) : null;
    case 'source':
      return `${resource.platformType ?? ''}-${resource.sourceType ?? ''}`;
    case 'temp':
      return resource.temperature ?? null;
    default:
      return null;
  }
};

const defaultComparison = (a: Resource, b: Resource) => {
  const aOnline = isResourceOnline(a);
  const bOnline = isResourceOnline(b);
  if (aOnline !== bOnline) return aOnline ? -1 : 1;
  return getDisplayName(a).localeCompare(getDisplayName(b));
};

const compareValues = (valueA: number | string | null, valueB: number | string | null) => {
  const aEmpty = valueA === null || valueA === undefined || (typeof valueA === 'number' && Number.isNaN(valueA));
  const bEmpty = valueB === null || valueB === undefined || (typeof valueB === 'number' && Number.isNaN(valueB));

  if (aEmpty && bEmpty) return 0;
  if (aEmpty) return 1;
  if (bEmpty) return -1;

  if (typeof valueA === 'number' && typeof valueB === 'number') {
    if (valueA === valueB) return 0;
    return valueA < valueB ? -1 : 1;
  }

  const aStr = String(valueA).toLowerCase();
  const bStr = String(valueB).toLowerCase();

  if (aStr === bStr) return 0;
  return aStr < bStr ? -1 : 1;
};

export const tokenizeSearch = (query: string): string[] =>
  query
    .trim()
    .toLowerCase()
    .split(/\s+/)
    .filter((term) => term.length > 0);

export const matchesSearch = (resource: Resource, term: string): boolean => {
  if (!term) return true;
  const normalizedTerm = term.toLowerCase();
  const candidates: string[] = [
    resource.name,
    resource.displayName,
    resource.id,
    resource.identity?.hostname ?? '',
    ...(resource.identity?.ips ?? []),
    ...(resource.tags ?? []),
  ];
  const haystack = candidates
    .filter((value): value is string => typeof value === 'string' && value.length > 0)
    .join(' ')
    .toLowerCase();
  return haystack.includes(normalizedTerm);
};

export const getResourceSources = (resource: Resource): string[] => {
  const platformData = resource.platformData as { sources?: string[] } | undefined;
  const normalized = (platformData?.sources ?? [])
    .map((source) => normalizeSource(source))
    .filter((source): source is string => Boolean(source));
  return Array.from(new Set(normalized));
};

export const collectAvailableSources = (resources: Resource[]): Set<string> => {
  const set = new Set<string>();
  resources.forEach((resource) => {
    getResourceSources(resource).forEach((source) => set.add(source));
  });
  return set;
};

export const collectAvailableStatuses = (resources: Resource[]): Set<string> => {
  const set = new Set<string>();
  resources.forEach((resource) => {
    const status = (resource.status || 'unknown').toLowerCase();
    if (status) set.add(status);
  });
  return set;
};

export const buildStatusOptions = (statuses: Set<string>): Array<{ key: string; label: string }> => {
  const items = Array.from(statuses);
  items.sort((a, b) => {
    const indexA = statusOrder.indexOf(a);
    const indexB = statusOrder.indexOf(b);
    if (indexA === -1 && indexB === -1) return a.localeCompare(b);
    if (indexA === -1) return 1;
    if (indexB === -1) return -1;
    return indexA - indexB;
  });
  return items.map((status) => ({
    key: status,
    label: statusLabels[status] ?? status,
  }));
};

export const filterResources = (
  resources: Resource[],
  sources: Set<string>,
  statuses: Set<string>,
  searchTerms: string[],
): Resource[] => {
  let filtered = resources;

  if (sources.size > 0) {
    filtered = filtered.filter((resource) => {
      const resourceSources = getResourceSources(resource);
      if (resourceSources.length === 0) return false;
      return resourceSources.some((source) => sources.has(source));
    });
  }

  if (statuses.size > 0) {
    filtered = filtered.filter((resource) => {
      const status = (resource.status || 'unknown').toLowerCase();
      return statuses.has(status);
    });
  }

  if (searchTerms.length > 0) {
    filtered = filtered.filter((resource) =>
      searchTerms.every((term) => matchesSearch(resource, term)),
    );
  }

  return filtered;
};

export const splitHostAndServiceResources = (
  resources: Resource[],
): { hosts: Resource[]; services: Resource[] } => ({
  hosts: resources.filter((resource) => !isServiceInfrastructureResource(resource)),
  services: resources.filter((resource) => isServiceInfrastructureResource(resource)),
});

export const sortResources = (
  resources: Resource[],
  sortKey: string,
  sortDirection: 'asc' | 'desc',
): Resource[] => {
  const list = [...resources];
  return list.sort((a, b) => {
    if (sortKey === 'default') {
      return defaultComparison(a, b);
    }

    const valueA = getSortValue(a, sortKey);
    const valueB = getSortValue(b, sortKey);
    const comparison = compareValues(valueA, valueB);

    if (comparison !== 0) {
      return sortDirection === 'asc' ? comparison : -comparison;
    }

    return defaultComparison(a, b);
  });
};

export const groupResources = (
  sortedResources: Resource[],
  mode: 'grouped' | 'flat',
): ResourceGroup[] => {
  if (mode !== 'grouped') {
    return [{ cluster: '', resources: sortedResources }];
  }

  const groups = new Map<string, Resource[]>();
  for (const resource of sortedResources) {
    const cluster = resource.clusterId || '';
    const list = groups.get(cluster);
    if (list) {
      list.push(resource);
    } else {
      groups.set(cluster, [resource]);
    }
  }

  const entries = Array.from(groups.entries()).map(([cluster, resources]) => ({ cluster, resources }));
  entries.sort((a, b) => {
    if (!a.cluster && b.cluster) return 1;
    if (a.cluster && !b.cluster) return -1;
    return a.cluster.localeCompare(b.cluster);
  });
  return entries;
};

export const computeIOScale = (
  resources: Resource[],
): { network: IODistributionStats; diskIO: IODistributionStats } => {
  const networkValues: number[] = [];
  const diskIOValues: number[] = [];

  for (const resource of resources) {
    const networkTotal = (resource.network?.rxBytes ?? 0) + (resource.network?.txBytes ?? 0);
    if (resource.network) {
      networkValues.push(networkTotal);
    }

    const diskIOTotal = (resource.diskIO?.readRate ?? 0) + (resource.diskIO?.writeRate ?? 0);
    if (resource.diskIO) {
      diskIOValues.push(diskIOTotal);
    }
  }

  return {
    network: buildIODistribution(networkValues),
    diskIO: buildIODistribution(diskIOValues),
  };
};

export const getOutlierEmphasis = (value: number, stats: IODistributionStats): OutlierEmphasis => {
  if (!Number.isFinite(value) || value <= 0 || stats.max <= 0) {
    return { fontWeight: 'normal', color: 'text-muted', showOutlierHint: false };
  }

  if (stats.count < 4) {
    const ratio = value / stats.max;
    if (ratio >= 0.995) {
      return { fontWeight: '500', color: 'text-base-content', showOutlierHint: true };
    }
    return { fontWeight: 'normal', color: 'text-muted', showOutlierHint: false };
  }

  if (stats.mad > 0) {
    const modifiedZ = (0.6745 * (value - stats.median)) / stats.mad;
    if (modifiedZ >= 6.5 && value >= stats.p99) {
      return { fontWeight: '600', color: 'text-base-content', showOutlierHint: true };
    }
    if (modifiedZ >= 5.5 && value >= stats.p97) {
      return { fontWeight: '500', color: 'text-base-content', showOutlierHint: true };
    }
    return { fontWeight: 'normal', color: 'text-muted', showOutlierHint: false };
  }

  if (value >= stats.p99) return { fontWeight: '600', color: 'text-base-content', showOutlierHint: true };
  if (value >= stats.p97) return { fontWeight: '500', color: 'text-base-content', showOutlierHint: true };
  if (value > 0) return { fontWeight: 'normal', color: 'text-muted', showOutlierHint: false };
  return { fontWeight: 'normal', color: 'text-muted', showOutlierHint: false };
};
