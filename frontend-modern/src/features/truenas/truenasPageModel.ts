import { resolveResourcePlatformType } from '@/utils/sourcePlatforms';
import { asTrimmedString } from '@/utils/stringUtils';
import type { Resource, ResourceIncident, ResourceType } from '@/types/resource';
import type { RecoveryPoint } from '@/types/recovery';

export type TrueNASPageTabId = 'overview' | 'storage' | 'apps' | 'vms' | 'shares' | 'protection';
export type TrueNASAppStatusFilter = 'all' | 'running' | 'attention' | 'stopped';
export type TrueNASVMStatusFilter = 'all' | 'running' | 'attention' | 'stopped';
export type TrueNASShareStatusFilter = 'all' | 'active' | 'attention' | 'disabled';
export type TrueNASIncidentSeverityFilter = 'all' | 'critical' | 'warning' | 'info';
export type TrueNASStorageStatusFilter = 'all' | 'healthy' | 'attention' | 'offline';
export type TrueNASProtectionStatusFilter =
  | 'all'
  | 'success'
  | 'warning'
  | 'failed'
  | 'running'
  | 'unknown';
export type TrueNASProtectionKind = 'snapshot' | 'replication' | 'other';

export type TrueNASTabSpec = {
  id: TrueNASPageTabId;
  label: string;
  path: string;
};

// The Overview tab is intentionally narrow: appliance systems and active
// health signals. Native API inventory has first-class homes: Storage for
// pools, datasets, and disks; Apps, VMs, and Shares for their TrueNAS API
// facets; and Protection for snapshots and replication over the canonical
// recovery points contract.
export const TRUENAS_TAB_SPECS: readonly TrueNASTabSpec[] = [
  { id: 'overview', label: 'Overview', path: '/truenas/overview' },
  { id: 'storage', label: 'Storage', path: '/truenas/storage' },
  { id: 'apps', label: 'Apps', path: '/truenas/apps' },
  { id: 'vms', label: 'VMs', path: '/truenas/vms' },
  { id: 'shares', label: 'Shares', path: '/truenas/shares' },
  { id: 'protection', label: 'Protection', path: '/truenas/protection' },
] as const;

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
  incidents: TrueNASIncidentRow[];
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
  apps: number;
  disks: number;
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
  apps: 0,
  disks: 0,
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
  return {
    resources: trueNasResources,
    systems,
    pools,
    datasets,
    disks,
    shares,
    vms,
    apps,
    incidents: buildTrueNASIncidentRows(trueNasResources),
  };
}

const isTrueNASPoolResource = (resource: Resource): boolean =>
  resource.type === 'pool' || resource.storage?.topology === 'pool';

const isTrueNASDatasetResource = (resource: Resource): boolean =>
  resource.type === 'dataset' || resource.storage?.topology === 'dataset';

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
  const hasParentSignals = resources.some((resource) =>
    Boolean(asTrimmedString(resource.parentId)),
  );

  const fallbackSystemId =
    systems.length === 1 && !hasParentSignals ? asTrimmedString(systems[0]?.id) : '';

  const owningSystemId = (resource: Resource): string => {
    if (fallbackSystemId && resource.id !== fallbackSystemId) return fallbackSystemId;

    let parentId = asTrimmedString(resource.parentId);
    const visited = new Set<string>();
    while (parentId && !visited.has(parentId)) {
      if (systemIds.has(parentId)) return parentId;
      visited.add(parentId);
      parentId = asTrimmedString(resourceById.get(parentId)?.parentId);
    }
    return '';
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
    } else if (resource.type === 'app-container') {
      counts.apps += 1;
    } else if (resource.type === 'physical_disk') {
      counts.disks += 1;
    }
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

  for (const storage of storageResources) {
    const counts = countsByStorage.get(storage.id);
    if (!counts) continue;
    for (const resource of trueNasResources) {
      if (resource.id === storage.id || !hasAncestor(resource, storage.id)) continue;
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
    return '';
  };

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

  for (const pool of pools) {
    const poolRowId = `pool:${pool.id}`;
    appendRow(pool, 'pool', 0);
    for (const dataset of datasets.filter((candidate) => owningPoolId(candidate) === pool.id)) {
      appendRow(dataset, 'dataset', 1, poolRowId);
    }
    for (const disk of disks.filter((candidate) => owningPoolId(candidate) === pool.id)) {
      appendRow(disk, 'disk', 1, poolRowId);
    }
  }

  for (const dataset of datasets.filter((resource) => !emitted.has(resource.id))) {
    appendRow(dataset, 'dataset', 0);
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

const normalizeProtectionOutcome = (
  value: unknown,
): Exclude<TrueNASProtectionStatusFilter, 'all'> => {
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
  return resourceDisplayName(left).localeCompare(resourceDisplayName(right));
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
  const status = normalize(resource.status);
  const zfsState = normalize(resource.storage?.zfsPoolState);
  const diskHealth = normalize(resource.physicalDisk?.health);

  if (
    ['warning', 'degraded', 'faulted', 'critical', 'failed', 'failure', 'paused'].includes(status)
  ) {
    return 'attention';
  }
  if (['offline', 'stopped'].includes(status)) return 'offline';
  if (zfsState && zfsState !== 'online' && zfsState !== 'healthy') return 'attention';
  if (diskHealth && diskHealth !== 'passed' && diskHealth !== 'healthy' && diskHealth !== 'ok') {
    return 'attention';
  }
  if (['online', 'running', 'healthy'].includes(status)) return 'healthy';
  return 'unknown';
}

export function mapTrueNASProtectionStatus(
  point: RecoveryPoint,
): Exclude<TrueNASProtectionStatusFilter, 'all'> {
  return normalizeProtectionOutcome(point.outcome);
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
    if (status !== 'all' && mapTrueNASProtectionStatus(point) !== status) return false;
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

export function filterTrueNASStorageResources(
  resources: Resource[],
  search: string,
  status: TrueNASStorageStatusFilter,
): Resource[] {
  return resources.filter((resource) => {
    if (!matchesTrueNASStorageSearch(resource, search)) return false;
    if (status === 'all') return true;
    return mapTrueNASStorageStatus(resource) === status;
  });
}

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
  for (const resource of resources) {
    const incidents = (resource.incidents ?? []).filter(hasIncidentSignal);
    if (incidents.length > 0) {
      incidents.forEach((incident, index) =>
        rows.push(buildIncidentRow(resource, incident, index)),
      );
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
  const state = normalize(resource.truenas?.app?.state);
  if (state === 'running') return 'running';
  if (state === 'stopped') return 'stopped';
  if (state === 'crashed' || state === 'deploying' || state === 'stopping') return 'attention';

  if (resource.status === 'online' || resource.status === 'running') return 'running';
  if (resource.status === 'offline' || resource.status === 'stopped') return 'stopped';
  if (resource.status === 'degraded' || resource.status === 'paused') return 'attention';
  return 'attention';
}

export function mapTrueNASVMStatus(resource: Resource): Exclude<TrueNASVMStatusFilter, 'all'> {
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
