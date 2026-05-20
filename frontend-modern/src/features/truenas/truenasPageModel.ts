import { resolveResourcePlatformType } from '@/utils/sourcePlatforms';
import { asTrimmedString } from '@/utils/stringUtils';
import type { Resource, ResourceIncident, ResourceType } from '@/types/resource';

export type TrueNASPageTabId = 'overview' | 'storage' | 'protection';
export type TrueNASAppStatusFilter = 'all' | 'running' | 'attention' | 'stopped';
export type TrueNASVMStatusFilter = 'all' | 'running' | 'attention' | 'stopped';
export type TrueNASShareStatusFilter = 'all' | 'active' | 'attention' | 'disabled';
export type TrueNASIncidentSeverityFilter = 'all' | 'critical' | 'warning' | 'info';

export type TrueNASTabSpec = {
  id: TrueNASPageTabId;
  label: string;
  path: string;
};

// The Overview tab is intentionally narrow: appliance systems first, then
// native workload inventory when present. Storage inventory, pool topology, and
// physical disks all live on the Storage tab, while snapshots and replication
// live on the Protection tab through the canonical Recovery surface.
export const TRUENAS_TAB_SPECS: readonly TrueNASTabSpec[] = [
  { id: 'overview', label: 'Overview', path: '/truenas/overview' },
  { id: 'storage', label: 'Storage', path: '/truenas/storage' },
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

export function buildTrueNASPageModel(resources: Resource[]): TrueNASPageModel {
  const trueNasResources = resources.filter(
    (resource) => isTrueNASPlatform(resource) && TRUENAS_RESOURCE_TYPES.has(resource.type),
  );

  const systems = trueNasResources.filter((resource) => resource.type === 'agent');
  const shares = trueNasResources.filter((resource) => resource.type === 'network-share');
  const vms = trueNasResources.filter((resource) => resource.type === 'vm');
  const apps = trueNasResources.filter((resource) => resource.type === 'app-container');
  return {
    resources: trueNasResources,
    systems,
    shares,
    vms,
    apps,
    incidents: buildTrueNASIncidentRows(trueNasResources),
  };
}

const normalize = (value: unknown): string =>
  typeof value === 'string' ? value.trim().toLowerCase() : '';

const titleize = (value: string): string =>
  value
    .split(/[\s_-]+/)
    .filter(Boolean)
    .map((part) => part.charAt(0).toUpperCase() + part.slice(1).toLowerCase())
    .join(' ');

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
