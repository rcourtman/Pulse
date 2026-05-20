import { resolveResourcePlatformType } from '@/utils/sourcePlatforms';
import type { Resource, ResourceType } from '@/types/resource';

export type TrueNASPageTabId = 'overview' | 'storage';
export type TrueNASAppStatusFilter = 'all' | 'running' | 'attention' | 'stopped';
export type TrueNASVMStatusFilter = 'all' | 'running' | 'attention' | 'stopped';
export type TrueNASShareStatusFilter = 'all' | 'active' | 'attention' | 'disabled';

export type TrueNASTabSpec = {
  id: TrueNASPageTabId;
  label: string;
  path: string;
};

// The Overview tab is intentionally narrow: appliance systems first, then
// native workload inventory when present. Storage inventory, pool topology, and
// physical disks all live on the Storage tab so operators have one canonical
// storage surface instead of a duplicated overview snapshot plus a richer
// storage page.
export const TRUENAS_TAB_SPECS: readonly TrueNASTabSpec[] = [
  { id: 'overview', label: 'Overview', path: '/truenas/overview' },
  { id: 'storage', label: 'Storage', path: '/truenas/storage' },
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
  };
}

const normalize = (value: unknown): string =>
  typeof value === 'string' ? value.trim().toLowerCase() : '';

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
