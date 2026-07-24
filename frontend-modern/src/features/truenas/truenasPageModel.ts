import { resolveResourcePlatformType } from '@/utils/sourcePlatforms';
import { asTrimmedString } from '@/utils/stringUtils';
import { hasImpairedResourceSource } from '@/utils/resourceSourceHealth';
import type {
  Resource,
  ResourceIncident,
  ResourceTrueNASServiceMeta,
  ResourceType,
} from '@/types/resource';
import type { RecoveryPoint } from '@/types/recovery';

export type TrueNASPageTabId =
  'overview' | 'storage' | 'services' | 'apps' | 'vms' | 'shares' | 'protection';
export type TrueNASAppStatusFilter = 'all' | 'running' | 'attention' | 'stopped';
export type TrueNASServiceStatusFilter = 'all' | 'running' | 'attention' | 'stopped' | 'disabled';
export type TrueNASVMStatusFilter = 'all' | 'running' | 'attention' | 'stopped';
export type TrueNASShareStatusFilter = 'all' | 'active' | 'attention' | 'disabled';
export type TrueNASIncidentSeverityFilter = 'all' | 'critical' | 'warning' | 'info';
export type TrueNASStorageStatusFilter = 'all' | 'healthy' | 'attention' | 'offline';
export type TrueNASProtectionStatusFilter =
  'all' | 'attention' | 'success' | 'warning' | 'failed' | 'running' | 'unknown';
export type TrueNASProtectionStatusBucket = Exclude<
  TrueNASProtectionStatusFilter,
  'all' | 'attention'
>;
export type TrueNASProtectionKind = 'snapshot' | 'replication' | 'other';

export type TrueNASTabSpec = {
  id: TrueNASPageTabId;
  label: string;
  path: string;
};

// The Overview tab is intentionally narrow: appliance systems and active
// health signals. Native API inventory has first-class homes: Storage for
// pools, datasets, and disks; Services, Apps, VMs, and Shares for their TrueNAS
// API facets; and Protection for snapshots and replication over the canonical
// recovery points contract.
export const TRUENAS_TAB_SPECS: readonly TrueNASTabSpec[] = [
  { id: 'overview', label: 'Overview', path: '/truenas/overview' },
  { id: 'storage', label: 'Storage', path: '/truenas/storage' },
  { id: 'services', label: 'Services', path: '/truenas/services' },
  { id: 'apps', label: 'Apps', path: '/truenas/apps' },
  { id: 'vms', label: 'VMs', path: '/truenas/vms' },
  { id: 'shares', label: 'Shares', path: '/truenas/shares' },
  { id: 'protection', label: 'Protection', path: '/truenas/protection' },
] as const;

export type TrueNASTabInventoryOptions = {
  hasProtectionInventory?: boolean;
};

const TRUENAS_RESOURCE_TYPES = new Set<ResourceType>([
  'agent',
  'vm',
  'app-container',
  'network-share',
  'storage',
  'pool',
  'dataset',
  'physical_disk',
]);

const isTrueNASPlatform = (resource: Resource): boolean =>
  resolveResourcePlatformType(resource) === 'truenas';

export type TrueNASPageModel = {
  resources: Resource[];
  systems: Resource[];
  pools: Resource[];
  datasets: Resource[];
  disks: Resource[];
  shares: Resource[];
  vms: Resource[];
  apps: Resource[];
  services: TrueNASServiceRow[];
  incidents: TrueNASIncidentRow[];
};

export type TrueNASServiceRow = {
  id: string;
  system: Resource;
  systemId: string;
  systemName: string;
  service: ResourceTrueNASServiceMeta;
};

export type TrueNASIncidentRow = {
  id: string;
  resource: Resource;
  resourceId: string;
  resourceName: string;
  resourceType: ResourceType;
  severity: string;
  severityBucket: Exclude<TrueNASIncidentSeverityFilter, 'all'>;
  code: string;
  source: string;
  summary: string;
  label: string;
  category: string;
  startedAt?: string;
  action: string;
  priority: number;
};

export type TrueNASSystemChildCounts = {
  pools: number;
  datasets: number;
  shares: number;
  vms: number;
  apps: number;
  disks: number;
  services: number;
};

export type TrueNASStorageChildCounts = {
  datasets: number;
  shares: number;
  disks: number;
};

export type TrueNASStorageTopologyKind = 'pool' | 'dataset' | 'disk';

export type TrueNASStorageTopologyRow = {
  id: string;
  resource: Resource;
  kind: TrueNASStorageTopologyKind;
  depth: number;
  parentRowId?: string;
  counts: TrueNASStorageChildCounts;
};

const emptyTrueNASSystemChildCounts = (): TrueNASSystemChildCounts => ({
  pools: 0,
  datasets: 0,
  shares: 0,
  vms: 0,
  apps: 0,
  disks: 0,
  services: 0,
});

const emptyTrueNASStorageChildCounts = (): TrueNASStorageChildCounts => ({
  datasets: 0,
  shares: 0,
  disks: 0,
});

export function buildTrueNASPageModel(resources: Resource[]): TrueNASPageModel {
  const trueNasResources = resources.filter(
    (resource) => isTrueNASPlatform(resource) && TRUENAS_RESOURCE_TYPES.has(resource.type),
  );

  const systems = trueNasResources.filter((resource) => resource.type === 'agent');
  const pools = trueNasResources.filter(isTrueNASPoolResource);
  const datasets = trueNasResources.filter(isTrueNASDatasetResource);
  const disks = trueNasResources.filter((resource) => resource.type === 'physical_disk');
  const shares = trueNasResources.filter((resource) => resource.type === 'network-share');
  const vms = trueNasResources.filter((resource) => resource.type === 'vm');
  const apps = trueNasResources.filter((resource) => resource.type === 'app-container');
  const services = buildTrueNASServiceRows(systems);
  const incidents = buildTrueNASIncidentRows(trueNasResources);
  return {
    resources: trueNasResources,
    systems,
    pools,
    datasets,
    disks,
    shares,
    vms,
    apps,
    services,
    incidents,
  };
}

const hasTrueNASStorageInventory = (model: TrueNASPageModel): boolean =>
  model.pools.length > 0 || model.datasets.length > 0 || model.disks.length > 0;

const hasTrueNASTabInventory = (
  model: TrueNASPageModel,
  tab: TrueNASPageTabId,
  options: TrueNASTabInventoryOptions = {},
): boolean => {
  switch (tab) {
    case 'overview':
      return true;
    case 'storage':
      return hasTrueNASStorageInventory(model);
    case 'services':
      return model.services.length > 0;
    case 'apps':
      return model.apps.length > 0;
    case 'vms':
      return model.vms.length > 0;
    case 'shares':
      return model.shares.length > 0;
    case 'protection':
      return Boolean(options.hasProtectionInventory);
  }
};

export const getTrueNASPageTabSpecs = (
  model: TrueNASPageModel,
  options: TrueNASTabInventoryOptions = {},
): readonly TrueNASTabSpec[] =>
  TRUENAS_TAB_SPECS.filter((tab) => hasTrueNASTabInventory(model, tab.id, options));

const isTrueNASPoolResource = (resource: Resource): boolean =>
  resource.type === 'pool' || resource.storage?.topology === 'pool';

const isTrueNASDatasetResource = (resource: Resource): boolean =>
  resource.type === 'dataset' || resource.storage?.topology === 'dataset';

const normalizeTrueNASStoragePath = (value?: string | null): string => {
  const trimmed = asTrimmedString(value);
  if (!trimmed) return '';
  return trimmed
    .replace(/^\/mnt\//i, '')
    .replace(/^\/+/, '')
    .replace(/\/+$/, '')
    .toLowerCase();
};

const firstTrueNASStoragePathSegment = (value?: string | null): string => {
  const normalized = normalizeTrueNASStoragePath(value);
  return normalized.split('/').filter(Boolean)[0] ?? '';
};

const trueNASPoolAliases = (resource: Resource): Set<string> =>
  new Set(
    [
      normalizeTrueNASStoragePath(resource.storage?.pool),
      normalizeTrueNASStoragePath(resource.storage?.path),
      normalizeTrueNASStoragePath(resource.name),
      normalizeTrueNASStoragePath(resource.displayName),
    ].filter(Boolean),
  );

const trueNASDatasetAliases = (resource: Resource): Set<string> =>
  new Set(
    [
      normalizeTrueNASStoragePath(resource.storage?.path),
      normalizeTrueNASStoragePath(resource.name),
      normalizeTrueNASStoragePath(resource.displayName),
    ].filter(Boolean),
  );

const trueNASResourceDatasetAliases = (resource: Resource): Set<string> =>
  new Set(
    [
      normalizeTrueNASStoragePath(resource.truenas?.share?.dataset),
      normalizeTrueNASStoragePath(resource.truenas?.share?.path),
      normalizeTrueNASStoragePath(resource.storage?.path),
      normalizeTrueNASStoragePath(resource.name),
      normalizeTrueNASStoragePath(resource.displayName),
    ].filter(Boolean),
  );

const inferTrueNASPoolName = (resource: Resource): string =>
  normalizeTrueNASStoragePath(resource.physicalDisk?.storageGroup) ||
  firstTrueNASStoragePathSegment(resource.truenas?.share?.dataset) ||
  firstTrueNASStoragePathSegment(resource.truenas?.share?.path) ||
  normalizeTrueNASStoragePath(resource.storage?.pool) ||
  firstTrueNASStoragePathSegment(resource.storage?.path) ||
  firstTrueNASStoragePathSegment(resource.name) ||
  firstTrueNASStoragePathSegment(resource.displayName);

const pathContainsOrDescendsFrom = (candidate: string, ancestor: string): boolean =>
  Boolean(
    candidate && ancestor && (candidate === ancestor || candidate.startsWith(`${ancestor}/`)),
  );

export function buildTrueNASSystemChildCounts(
  resources: Resource[],
  systems: Resource[],
): Map<string, TrueNASSystemChildCounts> {
  const countsBySystem = new Map<string, TrueNASSystemChildCounts>();
  for (const system of systems) {
    countsBySystem.set(system.id, emptyTrueNASSystemChildCounts());
  }
  if (systems.length === 0) return countsBySystem;

  const systemIds = new Set(systems.map((system) => system.id));
  const resourceById = new Map(resources.map((resource) => [resource.id, resource]));
  const fallbackSystemId = systems.length === 1 ? asTrimmedString(systems[0]?.id) : '';

  const owningSystemId = (resource: Resource): string => {
    let parentId = asTrimmedString(resource.parentId);
    const visited = new Set<string>();
    while (parentId && !visited.has(parentId)) {
      if (systemIds.has(parentId)) return parentId;
      visited.add(parentId);
      parentId = asTrimmedString(resourceById.get(parentId)?.parentId);
    }
    return fallbackSystemId && resource.id !== fallbackSystemId ? fallbackSystemId : '';
  };

  for (const resource of resources) {
    if (resource.type === 'agent') continue;
    const systemId = owningSystemId(resource);
    if (!systemId) continue;
    const counts = countsBySystem.get(systemId);
    if (!counts) continue;

    if (resource.type === 'pool' || resource.storage?.topology === 'pool') {
      counts.pools += 1;
    } else if (resource.type === 'dataset' || resource.storage?.topology === 'dataset') {
      counts.datasets += 1;
    } else if (resource.type === 'network-share') {
      counts.shares += 1;
    } else if (resource.type === 'vm') {
      counts.vms += 1;
    } else if (resource.type === 'app-container') {
      counts.apps += 1;
    } else if (resource.type === 'physical_disk') {
      counts.disks += 1;
    }
  }

  for (const system of systems) {
    const counts = countsBySystem.get(system.id);
    if (!counts) continue;
    counts.services = system.truenas?.services?.length ?? 0;
  }

  return countsBySystem;
}

export function buildTrueNASStorageChildCounts(
  resources: Resource[],
): Map<string, TrueNASStorageChildCounts> {
  const countsByStorage = new Map<string, TrueNASStorageChildCounts>();
  const trueNasResources = resources.filter(isTrueNASPlatform);
  const resourceById = new Map(trueNasResources.map((resource) => [resource.id, resource]));
  const storageResources = trueNasResources.filter(
    (resource) => isTrueNASPoolResource(resource) || isTrueNASDatasetResource(resource),
  );

  for (const storage of storageResources) {
    countsByStorage.set(storage.id, emptyTrueNASStorageChildCounts());
  }

  const hasAncestor = (resource: Resource, ancestorId: string): boolean => {
    let parentId = asTrimmedString(resource.parentId);
    const visited = new Set<string>();
    while (parentId && !visited.has(parentId)) {
      if (parentId === ancestorId) return true;
      visited.add(parentId);
      parentId = asTrimmedString(resourceById.get(parentId)?.parentId);
    }
    return false;
  };

  const hasInferredStorageRelationship = (resource: Resource, storage: Resource): boolean => {
    if (isTrueNASPoolResource(storage)) {
      const poolName = inferTrueNASPoolName(resource);
      return poolName ? trueNASPoolAliases(storage).has(poolName) : false;
    }
    if (!isTrueNASDatasetResource(storage)) return false;
    const datasetAliases = trueNASDatasetAliases(storage);
    const resourceAliases = trueNASResourceDatasetAliases(resource);
    for (const datasetAlias of datasetAliases) {
      for (const resourceAlias of resourceAliases) {
        if (pathContainsOrDescendsFrom(resourceAlias, datasetAlias)) return true;
      }
    }
    return false;
  };

  for (const storage of storageResources) {
    const counts = countsByStorage.get(storage.id);
    if (!counts) continue;
    for (const resource of trueNasResources) {
      if (
        resource.id === storage.id ||
        (!hasAncestor(resource, storage.id) && !hasInferredStorageRelationship(resource, storage))
      ) {
        continue;
      }
      if (isTrueNASDatasetResource(resource)) {
        counts.datasets += 1;
      } else if (resource.type === 'network-share') {
        counts.shares += 1;
      } else if (resource.type === 'physical_disk') {
        counts.disks += 1;
      }
    }
  }

  return countsByStorage;
}

export function buildTrueNASStorageTopologyRows(
  resources: Resource[],
): TrueNASStorageTopologyRow[] {
  const storageResources = resources.filter(
    (resource) =>
      isTrueNASPlatform(resource) &&
      (isTrueNASPoolResource(resource) ||
        isTrueNASDatasetResource(resource) ||
        resource.type === 'physical_disk'),
  );
  const resourceById = new Map(storageResources.map((resource) => [resource.id, resource]));
  const countsByStorage = buildTrueNASStorageChildCounts(resources);
  const pools = storageResources.filter(isTrueNASPoolResource).sort(compareStorageResources);
  const datasets = storageResources.filter(isTrueNASDatasetResource).sort(compareStorageResources);
  const disks = storageResources
    .filter((resource) => resource.type === 'physical_disk')
    .sort(compareStorageResources);

  const owningPoolId = (resource: Resource): string => {
    let parentId = asTrimmedString(resource.parentId);
    const visited = new Set<string>();
    while (parentId && !visited.has(parentId)) {
      const parent = resourceById.get(parentId);
      if (parent && isTrueNASPoolResource(parent)) return parent.id;
      visited.add(parentId);
      parentId = asTrimmedString(parent?.parentId);
    }
    const poolName = inferTrueNASPoolName(resource);
    if (poolName) {
      const inferredPool = pools.find((pool) => trueNASPoolAliases(pool).has(poolName));
      if (inferredPool) return inferredPool.id;
    }
    if (pools.length === 1 && !isTrueNASPoolResource(resource)) return pools[0]?.id ?? '';
    return '';
  };

  const owningDatasetId = (resource: Resource): string => {
    let parentId = asTrimmedString(resource.parentId);
    const visited = new Set<string>();
    while (parentId && !visited.has(parentId)) {
      const parent = resourceById.get(parentId);
      if (parent && isTrueNASDatasetResource(parent)) return parent.id;
      if (parent && isTrueNASPoolResource(parent)) return '';
      visited.add(parentId);
      parentId = asTrimmedString(parent?.parentId);
    }

    const resourceAliases = trueNASDatasetAliases(resource);
    const resourcePoolId = owningPoolId(resource);
    let bestMatch: { id: string; length: number } | undefined;
    for (const candidate of datasets) {
      if (candidate.id === resource.id) continue;
      const candidatePoolId = owningPoolId(candidate);
      if (resourcePoolId && candidatePoolId && resourcePoolId !== candidatePoolId) continue;

      for (const candidateAlias of trueNASDatasetAliases(candidate)) {
        for (const resourceAlias of resourceAliases) {
          if (
            resourceAlias !== candidateAlias &&
            pathContainsOrDescendsFrom(resourceAlias, candidateAlias) &&
            candidateAlias.length > (bestMatch?.length ?? 0)
          ) {
            bestMatch = { id: candidate.id, length: candidateAlias.length };
          }
        }
      }
    }
    return bestMatch?.id ?? '';
  };

  const rootDatasetsByPool = new Map<string, Resource[]>();
  const childDatasetsByDataset = new Map<string, Resource[]>();
  const rootDatasets: Resource[] = [];
  for (const dataset of datasets) {
    const datasetParentId = owningDatasetId(dataset);
    if (datasetParentId) {
      const children = childDatasetsByDataset.get(datasetParentId) ?? [];
      children.push(dataset);
      childDatasetsByDataset.set(datasetParentId, children);
      continue;
    }

    const poolId = owningPoolId(dataset);
    if (poolId) {
      const roots = rootDatasetsByPool.get(poolId) ?? [];
      roots.push(dataset);
      rootDatasetsByPool.set(poolId, roots);
      continue;
    }

    rootDatasets.push(dataset);
  }

  const rows: TrueNASStorageTopologyRow[] = [];
  const emitted = new Set<string>();
  const appendRow = (
    resource: Resource,
    kind: TrueNASStorageTopologyKind,
    depth: number,
    parentRowId?: string,
  ) => {
    emitted.add(resource.id);
    rows.push({
      id: `${kind}:${resource.id}`,
      resource,
      kind,
      depth,
      parentRowId,
      counts: countsByStorage.get(resource.id) ?? emptyTrueNASStorageChildCounts(),
    });
  };

  const appendDatasetTree = (
    dataset: Resource,
    depth: number,
    parentRowId: string | undefined,
    visiting = new Set<string>(),
  ) => {
    if (emitted.has(dataset.id) || visiting.has(dataset.id)) return;
    visiting.add(dataset.id);
    const datasetRowId = `dataset:${dataset.id}`;
    appendRow(dataset, 'dataset', depth, parentRowId);
    for (const child of childDatasetsByDataset.get(dataset.id) ?? []) {
      appendDatasetTree(child, depth + 1, datasetRowId, visiting);
    }
    visiting.delete(dataset.id);
  };

  for (const pool of pools) {
    const poolRowId = `pool:${pool.id}`;
    appendRow(pool, 'pool', 0);
    for (const dataset of rootDatasetsByPool.get(pool.id) ?? []) {
      appendDatasetTree(dataset, 1, poolRowId);
    }
    for (const disk of disks.filter((candidate) => owningPoolId(candidate) === pool.id)) {
      appendRow(disk, 'disk', 1, poolRowId);
    }
  }

  for (const dataset of rootDatasets) {
    appendDatasetTree(dataset, 0, undefined);
  }
  for (const dataset of datasets.filter((resource) => !emitted.has(resource.id))) {
    appendDatasetTree(dataset, 0, undefined);
  }
  for (const disk of disks.filter((resource) => !emitted.has(resource.id))) {
    appendRow(disk, 'disk', 0);
  }

  return rows;
}

function filterTrueNASStorageTopologyRow(
  row: TrueNASStorageTopologyRow,
  search: string,
  status: TrueNASStorageStatusFilter,
): boolean {
  if (status !== 'all' && mapTrueNASStorageStatus(row.resource) !== status) return false;
  if (!search.trim()) return true;
  if (row.kind.includes(search.trim().toLowerCase())) return true;
  return matchesTrueNASStorageSearch(row.resource, search);
}

export function filterTrueNASStorageTopologyRows(
  rows: TrueNASStorageTopologyRow[],
  search: string,
  status: TrueNASStorageStatusFilter,
): TrueNASStorageTopologyRow[] {
  const directMatches = new Set(
    rows.filter((row) => filterTrueNASStorageTopologyRow(row, search, status)).map((row) => row.id),
  );
  if (directMatches.size === rows.length) return rows;

  const rowById = new Map(rows.map((row) => [row.id, row]));
  for (const row of rows) {
    if (!directMatches.has(row.id)) continue;
    let parentRowId = row.parentRowId;
    const visited = new Set<string>();
    while (parentRowId && !visited.has(parentRowId)) {
      directMatches.add(parentRowId);
      visited.add(parentRowId);
      parentRowId = rowById.get(parentRowId)?.parentRowId;
    }
  }

  return rows.filter((row) => directMatches.has(row.id));
}

const normalize = (value: unknown): string =>
  typeof value === 'string' ? value.trim().toLowerCase() : '';

export const getTrueNASResourceDisplayStatus = (resource: Resource): string =>
  hasImpairedResourceSource(resource, 'truenas') ? 'degraded' : resource.status;

const normalizeProtectionOutcome = (value: unknown): TrueNASProtectionStatusBucket => {
  const normalized = normalize(value);
  if (normalized === 'success' || normalized === 'ok') return 'success';
  if (normalized === 'warning' || normalized === 'warn') return 'warning';
  if (normalized === 'failed' || normalized === 'failure' || normalized === 'error') {
    return 'failed';
  }
  if (normalized === 'running') return 'running';
  return 'unknown';
};

const getRecoveryPointTimestampMs = (point: RecoveryPoint): number => {
  const parsed = Date.parse(
    asTrimmedString(point.completedAt) || asTrimmedString(point.startedAt) || '',
  );
  return Number.isFinite(parsed) ? parsed : 0;
};

const titleize = (value: string): string =>
  value
    .split(/[\s_-]+/)
    .filter(Boolean)
    .map((part) => part.charAt(0).toUpperCase() + part.slice(1).toLowerCase())
    .join(' ');

const storageStatusRank = (resource: Resource): number => {
  switch (mapTrueNASStorageStatus(resource)) {
    case 'attention':
      return 0;
    case 'offline':
      return 1;
    case 'unknown':
      return 2;
    case 'healthy':
      return 3;
  }
};

const compareStorageResources = (left: Resource, right: Resource): number => {
  const rankDelta = storageStatusRank(left) - storageStatusRank(right);
  if (rankDelta !== 0) return rankDelta;
  const nameDelta = resourceDisplayName(left).localeCompare(resourceDisplayName(right));
  if (nameDelta !== 0) return nameDelta;
  // Two systems can expose identically-named pools (a DR pair both named
  // "tank"); without a deterministic tie-break their order tracked map
  // iteration order and flipped between refreshes (#1573).
  const parentDelta = (asTrimmedString(left.parentName) ?? '').localeCompare(
    asTrimmedString(right.parentName) ?? '',
  );
  if (parentDelta !== 0) return parentDelta;
  return left.id.localeCompare(right.id);
};

const serviceStatusRank = (row: TrueNASServiceRow): number => {
  switch (mapTrueNASServiceStatus(row)) {
    case 'attention':
      return 0;
    case 'stopped':
      return 1;
    case 'disabled':
      return 2;
    case 'running':
      return 3;
  }
};

const compareServiceRows = (left: TrueNASServiceRow, right: TrueNASServiceRow): number => {
  const rankDelta = serviceStatusRank(left) - serviceStatusRank(right);
  if (rankDelta !== 0) return rankDelta;
  const systemDelta = left.systemName.localeCompare(right.systemName);
  if (systemDelta !== 0) return systemDelta;
  return serviceDisplayName(left.service).localeCompare(serviceDisplayName(right.service));
};

const incidentSeverityRank = (severity: string): number => {
  switch (mapTrueNASIncidentSeverity(severity)) {
    case 'critical':
      return 3;
    case 'warning':
      return 2;
    case 'info':
      return 1;
  }
};

export function mapTrueNASIncidentSeverity(
  severity: string | undefined,
): Exclude<TrueNASIncidentSeverityFilter, 'all'> {
  const normalized = normalize(severity);
  if (['critical', 'crit', 'fatal', 'error', 'failed', 'failure'].includes(normalized)) {
    return 'critical';
  }
  if (['warning', 'warn', 'alert', 'degraded'].includes(normalized)) return 'warning';
  return 'info';
}

export function mapTrueNASStorageStatus(
  resource: Resource,
): Exclude<TrueNASStorageStatusFilter, 'all'> | 'unknown' {
  const status = normalize(getTrueNASResourceDisplayStatus(resource));
  const zfsState = normalize(resource.storage?.zfsPoolState);
  const diskHealth = normalize(resource.physicalDisk?.health);

  if (
    ['warning', 'degraded', 'faulted', 'critical', 'failed', 'failure', 'paused'].includes(status)
  ) {
    return 'attention';
  }
  if (['offline', 'stopped'].includes(status)) return 'offline';
  if (zfsState && zfsState !== 'online' && zfsState !== 'healthy') return 'attention';
  // The TrueNAS disk API carries no SMART/health field, so "unknown" is the
  // normal state of a healthy disk, not a warning signal (#1573). Only an
  // affirmatively bad health value earns the attention bucket.
  if (diskHealth && !['passed', 'healthy', 'ok', 'unknown', 'unavailable'].includes(diskHealth)) {
    return 'attention';
  }
  if (['online', 'running', 'healthy'].includes(status)) return 'healthy';
  return 'unknown';
}

export function mapTrueNASProtectionStatus(point: RecoveryPoint): TrueNASProtectionStatusBucket {
  return normalizeProtectionOutcome(point.outcome);
}

export type TrueNASProtectionPosture = {
  healthy: number;
  warning: number;
  failed: number;
  running: number;
  unknown: number;
  attention: number;
};

export function buildTrueNASProtectionPosture(
  points: readonly RecoveryPoint[],
): TrueNASProtectionPosture {
  const posture: TrueNASProtectionPosture = {
    healthy: 0,
    warning: 0,
    failed: 0,
    running: 0,
    unknown: 0,
    attention: 0,
  };
  for (const point of points) {
    const status = mapTrueNASProtectionStatus(point);
    if (status === 'success') posture.healthy += 1;
    else posture[status] += 1;
  }
  posture.attention = posture.warning + posture.failed;
  return posture;
}

export function mapTrueNASProtectionKind(point: RecoveryPoint): TrueNASProtectionKind {
  const kind = normalize(point.kind);
  const mode = normalize(point.mode);
  if (kind === 'snapshot' || mode === 'snapshot') return 'snapshot';
  if (kind === 'backup' && mode === 'remote') return 'replication';
  const details = point.details ?? {};
  if (
    asTrimmedString(details.taskName) ||
    asTrimmedString(details.taskId) ||
    asTrimmedString(details.targetDataset)
  ) {
    return 'replication';
  }
  return 'other';
}

const trueNASProtectionSearchTokens = (point: RecoveryPoint): string[] => {
  const details = point.details ?? {};
  const sourceDatasets = Array.isArray(details.sourceDatasets)
    ? details.sourceDatasets.filter((value): value is string => typeof value === 'string')
    : [];

  return [
    point.id,
    point.platform,
    point.kind,
    point.mode,
    point.outcome,
    point.entityId,
    point.cluster,
    point.node,
    point.namespace,
    point.display?.itemLabel,
    point.display?.subjectLabel,
    point.display?.itemType,
    point.display?.subjectType,
    point.display?.clusterLabel,
    point.display?.nodeHostLabel,
    point.display?.nodeAgentLabel,
    point.display?.namespaceLabel,
    point.display?.entityIdLabel,
    point.display?.repositoryLabel,
    point.display?.detailsSummary,
    point.itemRef?.type,
    point.itemRef?.namespace,
    point.itemRef?.name,
    point.itemRef?.uid,
    point.itemRef?.id,
    point.subjectRef?.type,
    point.subjectRef?.namespace,
    point.subjectRef?.name,
    point.subjectRef?.uid,
    point.subjectRef?.id,
    point.repositoryRef?.type,
    point.repositoryRef?.namespace,
    point.repositoryRef?.name,
    point.repositoryRef?.uid,
    point.repositoryRef?.id,
    details.connectionId,
    details.dataset,
    details.direction,
    details.fullName,
    details.hostname,
    details.lastError,
    details.lastSnapshot,
    details.lastState,
    details.snapshot,
    details.targetDataset,
    details.taskId,
    details.taskName,
    ...sourceDatasets,
  ].filter((value): value is string => typeof value === 'string' && value.trim().length > 0);
};

export function sortTrueNASProtectionPoints(points: readonly RecoveryPoint[]): RecoveryPoint[] {
  return [...points].sort((left, right) => {
    const timeDelta = getRecoveryPointTimestampMs(right) - getRecoveryPointTimestampMs(left);
    if (timeDelta !== 0) return timeDelta;
    const leftLabel =
      asTrimmedString(left.display?.itemLabel) ||
      asTrimmedString(left.display?.subjectLabel) ||
      asTrimmedString(left.itemRef?.name) ||
      asTrimmedString(left.subjectRef?.name) ||
      left.id;
    const rightLabel =
      asTrimmedString(right.display?.itemLabel) ||
      asTrimmedString(right.display?.subjectLabel) ||
      asTrimmedString(right.itemRef?.name) ||
      asTrimmedString(right.subjectRef?.name) ||
      right.id;
    return leftLabel.localeCompare(rightLabel);
  });
}

export function filterTrueNASProtectionPoints(
  points: RecoveryPoint[],
  search: string,
  status: TrueNASProtectionStatusFilter,
): RecoveryPoint[] {
  const needle = search.trim().toLowerCase();
  return points.filter((point) => {
    const pointStatus = mapTrueNASProtectionStatus(point);
    if (status === 'attention' && pointStatus !== 'warning' && pointStatus !== 'failed') {
      return false;
    }
    if (status !== 'all' && status !== 'attention' && pointStatus !== status) return false;
    if (!needle) return true;
    if (mapTrueNASProtectionKind(point).includes(needle)) return true;
    return trueNASProtectionSearchTokens(point).join(' ').toLowerCase().includes(needle);
  });
}

const matchesTrueNASStorageSearch = (resource: Resource, search: string): boolean => {
  const needle = search.trim().toLowerCase();
  if (!needle) return true;
  const storage = resource.storage;
  const disk = resource.physicalDisk;
  const incidents = resource.incidents ?? [];
  const haystack = [
    resource.name,
    resource.displayName,
    resource.id,
    resource.parentName,
    resource.status,
    resource.metricsTarget?.resourceId,
    storage?.type,
    storage?.topology,
    storage?.platform,
    storage?.protection,
    storage?.pool,
    storage?.path,
    storage?.zfsPoolState,
    storage?.risk?.level,
    storage?.riskSummary,
    storage?.protectionSummary,
    disk?.devPath,
    disk?.model,
    disk?.serial,
    disk?.wwn,
    disk?.diskType,
    disk?.health,
    disk?.storageGroup,
    disk?.storageRole,
    disk?.storageState,
    ...(resource.tags ?? []),
    ...incidents.map((incident) => incident.summary),
    ...incidents.map((incident) => incident.code),
  ]
    .filter((value): value is string => typeof value === 'string')
    .join(' ')
    .toLowerCase();
  return haystack.includes(needle);
};

const resourceIncidentLabel = (resource: Resource, incident: ResourceIncident): string => {
  const label = asTrimmedString(resource.incidentLabel);
  if (label) return label;
  const code = asTrimmedString(incident.code);
  return code ? titleize(code.replace(/^truenas_/, '')) : 'TrueNAS Alert';
};

const resourceDisplayName = (resource: Resource): string =>
  asTrimmedString(resource.displayName) ||
  asTrimmedString(resource.name) ||
  asTrimmedString(resource.truenas?.hostname) ||
  resource.id;

const serviceDisplayName = (service: ResourceTrueNASServiceMeta): string =>
  asTrimmedString(service.service) || asTrimmedString(service.id) || 'unknown';

export function buildTrueNASServiceRows(systems: Resource[]): TrueNASServiceRow[] {
  const rows: TrueNASServiceRow[] = [];
  for (const system of systems) {
    const systemName = resourceDisplayName(system);
    for (const service of system.truenas?.services ?? []) {
      const serviceKey = serviceDisplayName(service);
      rows.push({
        id: `${system.id}:service:${asTrimmedString(service.id) || serviceKey}`,
        system,
        systemId: system.id,
        systemName,
        service,
      });
    }
  }
  return rows.sort(compareServiceRows);
}

const hasIncidentSignal = (incident: ResourceIncident): boolean =>
  Boolean(asTrimmedString(incident.code) || asTrimmedString(incident.summary));

const hasIncidentRollup = (resource: Resource): boolean =>
  (resource.incidentCount ?? 0) > 0 ||
  Boolean(
    asTrimmedString(resource.incidentCode) ||
    asTrimmedString(resource.incidentSummary) ||
    asTrimmedString(resource.incidentLabel),
  );

const buildIncidentRow = (
  resource: Resource,
  incident: ResourceIncident,
  index: number,
): TrueNASIncidentRow => {
  const severity =
    asTrimmedString(incident.severity) || asTrimmedString(resource.incidentSeverity) || 'info';
  const code =
    asTrimmedString(incident.code) || asTrimmedString(resource.incidentCode) || 'truenas_alert';
  const summary =
    asTrimmedString(incident.summary) ||
    asTrimmedString(resource.incidentSummary) ||
    resourceIncidentLabel(resource, incident);
  const nativeId = asTrimmedString(incident.nativeId);
  const rowKey = nativeId || code || String(index);

  return {
    id: `${resource.id}:incident:${rowKey}:${index}`,
    resource,
    resourceId: resource.id,
    resourceName: resourceDisplayName(resource),
    resourceType: resource.type,
    severity,
    severityBucket: mapTrueNASIncidentSeverity(severity),
    code,
    source: asTrimmedString(incident.source) || asTrimmedString(incident.provider) || 'truenas',
    summary,
    label: resourceIncidentLabel(resource, incident),
    category: asTrimmedString(resource.incidentCategory) || 'health',
    startedAt: incident.startedAt,
    action: asTrimmedString(resource.incidentAction) || 'Investigate in TrueNAS',
    priority: resource.incidentPriority ?? incidentSeverityRank(severity) * 1000,
  };
};

const buildRollupIncidentRow = (resource: Resource): TrueNASIncidentRow => {
  const severity = asTrimmedString(resource.incidentSeverity) || 'info';
  const code = asTrimmedString(resource.incidentCode) || 'truenas_alert';
  const count = resource.incidentCount ?? 0;
  const summary =
    asTrimmedString(resource.incidentSummary) ||
    asTrimmedString(resource.incidentLabel) ||
    `${count || 1} active TrueNAS alert${count === 1 ? '' : 's'}`;
  const incident: ResourceIncident = {
    code,
    severity,
    summary,
    source: 'truenas',
  };

  return {
    ...buildIncidentRow(resource, incident, 0),
    id: `${resource.id}:incident:rollup`,
  };
};

export function buildTrueNASIncidentRows(resources: Resource[]): TrueNASIncidentRow[] {
  const rows: TrueNASIncidentRow[] = [];
  const rowByNativeSignal = new Map<string, number>();
  const specificity = (resource: Resource): number => {
    if (resource.type === 'physical_disk') return 4;
    if (resource.type === 'app-container') return 3;
    if (resource.type === 'storage') return 2;
    return 1;
  };
  for (const resource of resources) {
    const incidents = (resource.incidents ?? []).filter(hasIncidentSignal);
    if (incidents.length > 0) {
      incidents.forEach((incident, index) => {
        const row = buildIncidentRow(resource, incident, index);
        const nativeId = asTrimmedString(incident.nativeId);
        const provider = asTrimmedString(incident.provider);
        const signalKey =
          nativeId && provider
            ? `${provider.toLowerCase()}|${nativeId}|${row.code.toLowerCase()}`
            : '';
        const existingIndex = signalKey ? rowByNativeSignal.get(signalKey) : undefined;
        if (existingIndex === undefined) {
          if (signalKey) rowByNativeSignal.set(signalKey, rows.length);
          rows.push(row);
          return;
        }
        if (specificity(resource) > specificity(rows[existingIndex]!.resource)) {
          rows[existingIndex] = row;
        }
      });
      continue;
    }
    if (hasIncidentRollup(resource)) {
      rows.push(buildRollupIncidentRow(resource));
    }
  }

  return rows.sort((a, b) => {
    const severityDelta = incidentSeverityRank(b.severity) - incidentSeverityRank(a.severity);
    if (severityDelta !== 0) return severityDelta;
    const priorityDelta = b.priority - a.priority;
    if (priorityDelta !== 0) return priorityDelta;
    return a.resourceName.localeCompare(b.resourceName);
  });
}

export function mapTrueNASAppStatus(resource: Resource): Exclude<TrueNASAppStatusFilter, 'all'> {
  if (hasImpairedResourceSource(resource, 'truenas')) return 'attention';
  const state = normalize(resource.truenas?.app?.state);
  if (state === 'running') return 'running';
  if (state === 'stopped') return 'stopped';
  if (state === 'crashed' || state === 'deploying' || state === 'stopping') return 'attention';

  if (resource.status === 'online' || resource.status === 'running') return 'running';
  if (resource.status === 'offline' || resource.status === 'stopped') return 'stopped';
  if (resource.status === 'degraded' || resource.status === 'paused') return 'attention';
  return 'attention';
}

export function mapTrueNASServiceStatus(
  row: TrueNASServiceRow,
): Exclude<TrueNASServiceStatusFilter, 'all'> {
  if (hasImpairedResourceSource(row.system, 'truenas')) return 'attention';
  const state = normalize(row.service.state);
  if (['running', 'started', 'active'].includes(state)) return 'running';
  if (['failed', 'error', 'crashed', 'degraded', 'unknown'].includes(state)) return 'attention';
  if (['stopped', 'stop', 'inactive'].includes(state)) {
    return row.service.enabled === false ? 'disabled' : 'stopped';
  }
  if (row.service.enabled === false) return 'disabled';
  return 'attention';
}

export function mapTrueNASVMStatus(resource: Resource): Exclude<TrueNASVMStatusFilter, 'all'> {
  if (hasImpairedResourceSource(resource, 'truenas')) return 'attention';
  const state = normalize(resource.truenas?.vm?.state || resource.truenas?.vm?.domainState);
  if (state === 'running' || state === 'active') return 'running';
  if (state === 'stopped' || state === 'shutoff' || state === 'shutdown' || state === 'poweroff') {
    return 'stopped';
  }
  if (state === 'paused' || state === 'suspended' || state === 'error' || state === 'crashed') {
    return 'attention';
  }

  if (resource.status === 'online' || resource.status === 'running') return 'running';
  if (resource.status === 'offline' || resource.status === 'stopped') return 'stopped';
  return 'attention';
}

export function mapTrueNASShareStatus(
  resource: Resource,
): Exclude<TrueNASShareStatusFilter, 'all'> {
  if (hasImpairedResourceSource(resource, 'truenas')) return 'attention';
  const share = resource.truenas?.share;
  if (share?.enabled === false || resource.status === 'offline' || resource.status === 'stopped') {
    return 'disabled';
  }
  if (share?.locked || resource.status === 'degraded' || resource.status === 'paused') {
    return 'attention';
  }
  if (share?.enabled === true || resource.status === 'online' || resource.status === 'running') {
    return 'active';
  }
  return 'attention';
}

const portSearchTokens = (resource: Resource): string[] => {
  const app = resource.truenas?.app;
  const tokens: string[] = [];
  for (const port of app?.usedPorts ?? []) {
    if (typeof port.containerPort === 'number') tokens.push(String(port.containerPort));
    if (port.protocol) tokens.push(port.protocol);
    for (const hostPort of port.hostPorts ?? []) {
      if (typeof hostPort.hostPort === 'number') tokens.push(String(hostPort.hostPort));
      if (hostPort.hostIp) tokens.push(hostPort.hostIp);
    }
  }
  for (const port of resource.docker?.ports ?? []) {
    if (typeof port.publicPort === 'number') tokens.push(String(port.publicPort));
    if (typeof port.privatePort === 'number') tokens.push(String(port.privatePort));
    if (port.protocol) tokens.push(port.protocol);
    if (port.ip) tokens.push(port.ip);
  }
  return tokens;
};

const appSearchHaystack = (resource: Resource): string => {
  const app = resource.truenas?.app;
  return [
    resource.name,
    resource.displayName,
    resource.id,
    resource.parentName,
    resource.platformId,
    resource.platformType,
    resource.docker?.runtime,
    resource.docker?.image,
    resource.truenas?.hostname,
    app?.id,
    app?.name,
    app?.state,
    app?.version,
    app?.humanVersion,
    app?.notes,
    ...(app?.usedHostIps ?? []),
    ...(app?.images ?? []),
    ...(app?.containers?.flatMap((container) => [
      container.id,
      container.serviceName,
      container.image,
      container.state,
    ]) ?? []),
    ...(app?.volumes?.flatMap((volume) => [
      volume.source,
      volume.destination,
      volume.mode,
      volume.type,
    ]) ?? []),
    ...(app?.networks?.flatMap((network) => [network.id, network.name]) ?? []),
    ...portSearchTokens(resource),
    ...(resource.tags ?? []),
  ]
    .filter((value): value is string => typeof value === 'string' && value.trim().length > 0)
    .join(' ')
    .toLowerCase();
};

export function filterTrueNASApps(
  apps: Resource[],
  search: string,
  status: TrueNASAppStatusFilter,
): Resource[] {
  const needle = normalize(search);
  return apps.filter((app) => {
    if (status !== 'all' && mapTrueNASAppStatus(app) !== status) return false;
    if (!needle) return true;
    return appSearchHaystack(app).includes(needle);
  });
}

const serviceSearchHaystack = (row: TrueNASServiceRow): string =>
  [
    row.id,
    row.systemId,
    row.systemName,
    row.system.truenas?.hostname,
    row.service.id,
    row.service.service,
    row.service.state,
    row.service.enabled === true ? 'enabled boot start' : 'disabled no boot',
    ...(row.service.pids ?? []).map(String),
  ]
    .filter((value): value is string => typeof value === 'string' && value.trim().length > 0)
    .join(' ')
    .toLowerCase();

export function filterTrueNASServices(
  services: TrueNASServiceRow[],
  search: string,
  status: TrueNASServiceStatusFilter,
): TrueNASServiceRow[] {
  const needle = normalize(search);
  return services.filter((service) => {
    if (status !== 'all' && mapTrueNASServiceStatus(service) !== status) return false;
    if (!needle) return true;
    return serviceSearchHaystack(service).includes(needle);
  });
}

const vmSearchHaystack = (resource: Resource): string => {
  const vm = resource.truenas?.vm;
  return [
    resource.name,
    resource.displayName,
    resource.id,
    resource.parentName,
    resource.platformId,
    resource.platformType,
    resource.truenas?.hostname,
    vm?.id,
    vm?.name,
    vm?.description,
    vm?.state,
    vm?.domainState,
    vm?.cpuMode,
    vm?.cpuModel,
    vm?.bootloader,
    vm?.time,
    vm?.archType,
    vm?.machineType,
    vm?.uuid,
    ...(resource.tags ?? []),
  ]
    .filter((value): value is string => typeof value === 'string' && value.trim().length > 0)
    .join(' ')
    .toLowerCase();
};

export function filterTrueNASVMs(
  vms: Resource[],
  search: string,
  status: TrueNASVMStatusFilter,
): Resource[] {
  const needle = normalize(search);
  return vms.filter((vm) => {
    if (status !== 'all' && mapTrueNASVMStatus(vm) !== status) return false;
    if (!needle) return true;
    return vmSearchHaystack(vm).includes(needle);
  });
}

const shareSearchHaystack = (resource: Resource): string => {
  const share = resource.truenas?.share;
  return [
    resource.name,
    resource.displayName,
    resource.id,
    resource.parentName,
    resource.platformId,
    resource.platformType,
    resource.truenas?.hostname,
    share?.id,
    share?.name,
    share?.protocol,
    share?.path,
    share?.dataset,
    share?.relativePath,
    share?.comment,
    share?.enabled === false ? 'disabled' : share?.enabled === true ? 'enabled active' : undefined,
    share?.readOnly === true
      ? 'read-only readonly'
      : share?.readOnly === false
        ? 'read-write'
        : undefined,
    share?.browsable ? 'browsable' : undefined,
    share?.locked ? 'locked' : undefined,
    share?.accessBasedEnumeration ? 'access based enumeration abe' : undefined,
    share?.auditEnabled ? 'audit audited' : undefined,
    share?.exposeSnapshots ? 'snapshots' : undefined,
    ...(share?.aliases ?? []),
    ...(share?.hosts ?? []),
    ...(share?.networks ?? []),
    ...(share?.security ?? []),
    ...(resource.tags ?? []),
  ]
    .filter((value): value is string => typeof value === 'string' && value.trim().length > 0)
    .join(' ')
    .toLowerCase();
};

export function filterTrueNASShares(
  shares: Resource[],
  search: string,
  status: TrueNASShareStatusFilter,
): Resource[] {
  const needle = normalize(search);
  return shares.filter((share) => {
    if (status !== 'all' && mapTrueNASShareStatus(share) !== status) return false;
    if (!needle) return true;
    return shareSearchHaystack(share).includes(needle);
  });
}

const incidentSearchHaystack = (row: TrueNASIncidentRow): string =>
  [
    row.resourceName,
    row.resourceId,
    row.resourceType,
    row.resource.parentName,
    row.resource.platformId,
    row.resource.truenas?.hostname,
    row.severity,
    row.code,
    row.source,
    row.summary,
    row.label,
    row.category,
    row.action,
    ...(row.resource.tags ?? []),
  ]
    .filter((value): value is string => typeof value === 'string' && value.trim().length > 0)
    .join(' ')
    .toLowerCase();

export function filterTrueNASIncidents(
  incidents: TrueNASIncidentRow[],
  search: string,
  severity: TrueNASIncidentSeverityFilter,
): TrueNASIncidentRow[] {
  const needle = normalize(search);
  return incidents.filter((incident) => {
    if (severity !== 'all' && incident.severityBucket !== severity) return false;
    if (!needle) return true;
    return incidentSearchHaystack(incident).includes(needle);
  });
}
