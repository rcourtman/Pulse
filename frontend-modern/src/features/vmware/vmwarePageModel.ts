import { resolveResourcePlatformType } from '@/utils/sourcePlatforms';
import type { Resource, ResourceType } from '@/types/resource';

export type VmwarePageTabId = 'overview' | 'storage';
export type VmwareDatastoreStatusFilter =
  | 'all'
  | 'accessible'
  | 'attention'
  | 'inaccessible'
  | 'maintenance'
  | 'unknown';
export type VmwareVirtualMachineStatusFilter =
  | 'all'
  | 'powered-on'
  | 'attention'
  | 'powered-off'
  | 'suspended'
  | 'unknown';

export type VmwareTabSpec = {
  id: VmwarePageTabId;
  label: string;
  path: string;
};

// The Overview tab mirrors the vCenter inventory shape: ESXi hosts on top,
// vSphere VMs underneath grouped by runtime host. A dedicated `vms` tab would
// repeat the same inventory slice, so VM search and power filtering live inside
// the Overview table.
export const VMWARE_TAB_SPECS: readonly VmwareTabSpec[] = [
  { id: 'overview', label: 'Overview', path: '/vmware/overview' },
  { id: 'storage', label: 'Datastores', path: '/vmware/storage' },
] as const;

const VMWARE_RESOURCE_TYPES = new Set<ResourceType>(['agent', 'vm', 'storage']);

const isVmwarePlatform = (resource: Resource): boolean =>
  resolveResourcePlatformType(resource) === 'vmware-vsphere';

export type VmwarePageModel = {
  resources: Resource[];
  hosts: Resource[];
  vms: Resource[];
  datastores: Resource[];
};

export function buildVmwarePageModel(resources: Resource[]): VmwarePageModel {
  const vmwareResources = resources.filter(
    (resource) => isVmwarePlatform(resource) && VMWARE_RESOURCE_TYPES.has(resource.type),
  );
  const hosts = vmwareResources.filter((resource) => resource.type === 'agent');
  const vms = vmwareResources
    .filter((resource) => resource.type === 'vm')
    .sort(compareVmwareVirtualMachines);
  const datastores = vmwareResources
    .filter(
      (resource) =>
        resource.type === 'storage' &&
        (resource.storage?.topology === 'datastore' || resource.vmware?.entityType === 'datastore'),
    )
    .sort(compareVmwareDatastores);

  return {
    resources: vmwareResources,
    hosts,
    vms,
    datastores,
  };
}

const normalize = (value: unknown): string =>
  typeof value === 'string' ? value.trim().toLowerCase() : '';

const normalizeToken = (value: unknown): string => normalize(value).replace(/[\s_-]/g, '');

const vmwareDatastoreDisplayName = (resource: Resource): string =>
  resource.displayName?.trim() || resource.name?.trim() || resource.id;

const vmwareVirtualMachineDisplayName = (resource: Resource): string =>
  resource.displayName?.trim() || resource.name?.trim() || resource.id;

const vmwareDatastoreStatusRank = (resource: Resource): number => {
  switch (mapVmwareDatastoreStatus(resource)) {
    case 'inaccessible':
      return 0;
    case 'maintenance':
      return 1;
    case 'attention':
      return 2;
    case 'unknown':
      return 3;
    case 'accessible':
      return 4;
  }
};

const compareVmwareDatastores = (left: Resource, right: Resource): number => {
  const rankDelta = vmwareDatastoreStatusRank(left) - vmwareDatastoreStatusRank(right);
  if (rankDelta !== 0) return rankDelta;
  return vmwareDatastoreDisplayName(left).localeCompare(vmwareDatastoreDisplayName(right));
};

const vmwareVirtualMachineHostKey = (resource: Resource): string =>
  normalize(resource.vmware?.runtimeHostName || resource.parentName || 'unknown');

const vmwareVirtualMachineStatusRank = (resource: Resource): number => {
  switch (mapVmwareVirtualMachineStatus(resource)) {
    case 'attention':
      return 0;
    case 'suspended':
      return 1;
    case 'powered-off':
      return 2;
    case 'unknown':
      return 3;
    case 'powered-on':
      return 4;
  }
};

const compareVmwareVirtualMachines = (left: Resource, right: Resource): number => {
  const hostDelta = vmwareVirtualMachineHostKey(left).localeCompare(
    vmwareVirtualMachineHostKey(right),
  );
  if (hostDelta !== 0) return hostDelta;
  const rankDelta = vmwareVirtualMachineStatusRank(left) - vmwareVirtualMachineStatusRank(right);
  if (rankDelta !== 0) return rankDelta;
  return vmwareVirtualMachineDisplayName(left).localeCompare(
    vmwareVirtualMachineDisplayName(right),
  );
};

export function mapVmwareVirtualMachineStatus(
  resource: Resource,
): Exclude<VmwareVirtualMachineStatusFilter, 'all'> {
  const powerState = normalizeToken(resource.vmware?.powerState);
  const status = normalize(resource.status);
  const overall = normalize(resource.vmware?.overallStatus);
  const activeAlarms = resource.vmware?.activeAlarmCount ?? 0;

  if (
    activeAlarms > 0 ||
    ['red', 'yellow', 'degraded', 'warning', 'critical', 'paused'].includes(overall) ||
    ['degraded', 'warning', 'critical', 'paused'].includes(status)
  ) {
    return 'attention';
  }
  if (['poweredoff', 'off'].includes(powerState) || status === 'offline' || status === 'stopped') {
    return 'powered-off';
  }
  if (['suspended', 'suspend'].includes(powerState) || status === 'paused') return 'suspended';
  if (
    ['poweredon', 'on', 'running'].includes(powerState) ||
    status === 'online' ||
    status === 'running'
  ) {
    return 'powered-on';
  }
  return 'unknown';
}

export function mapVmwareDatastoreStatus(
  resource: Resource,
): Exclude<VmwareDatastoreStatusFilter, 'all'> {
  const maintenanceMode = normalize(resource.vmware?.maintenanceMode);
  const status = normalize(resource.status);
  const overall = normalize(resource.vmware?.overallStatus);

  if (
    maintenanceMode &&
    maintenanceMode !== 'normal' &&
    maintenanceMode !== 'none' &&
    maintenanceMode !== 'not_in_maintenance'
  ) {
    return 'maintenance';
  }
  if (resource.vmware?.datastoreAccessible === false || status === 'offline') {
    return 'inaccessible';
  }
  if (['red', 'yellow', 'degraded', 'warning', 'critical', 'paused'].includes(overall)) {
    return 'attention';
  }
  if (['degraded', 'warning', 'critical', 'paused'].includes(status)) return 'attention';
  if (resource.vmware?.datastoreAccessible === true || ['online', 'running'].includes(status)) {
    return 'accessible';
  }
  return 'unknown';
}

const vmwareDatastoreSearchHaystack = (resource: Resource): string =>
  [
    resource.id,
    resource.name,
    resource.displayName,
    resource.parentName,
    resource.status,
    resource.vmware?.connectionName,
    resource.vmware?.vcenterHost,
    resource.vmware?.managedObjectId,
    resource.vmware?.datacenterName,
    resource.vmware?.folderName,
    resource.vmware?.datastoreType,
    resource.vmware?.datastoreUrl,
    resource.vmware?.maintenanceMode,
    resource.vmware?.overallStatus,
    resource.storage?.type,
    resource.storage?.topology,
    resource.storage?.platform,
    resource.storage?.nodes?.join(' '),
    resource.storage?.consumerTypes?.join(' '),
    resource.storage?.topConsumers?.map((consumer) => consumer.name).join(' '),
    ...(resource.tags ?? []),
  ]
    .filter((value): value is string => typeof value === 'string' && value.trim().length > 0)
    .join(' ')
    .toLowerCase();

export function filterVmwareDatastores(
  datastores: Resource[],
  search: string,
  status: VmwareDatastoreStatusFilter,
): Resource[] {
  const needle = normalize(search);
  return datastores.filter((datastore) => {
    if (status !== 'all' && mapVmwareDatastoreStatus(datastore) !== status) return false;
    if (!needle) return true;
    return vmwareDatastoreSearchHaystack(datastore).includes(needle);
  });
}

const vmwareVirtualMachineSearchHaystack = (resource: Resource): string =>
  [
    resource.id,
    resource.name,
    resource.displayName,
    resource.parentName,
    resource.status,
    resource.vmware?.connectionName,
    resource.vmware?.vcenterHost,
    resource.vmware?.managedObjectId,
    resource.vmware?.datacenterName,
    resource.vmware?.clusterName,
    resource.vmware?.computeResourceName,
    resource.vmware?.folderName,
    resource.vmware?.resourcePoolName,
    resource.vmware?.runtimeHostName,
    resource.vmware?.powerState,
    resource.vmware?.overallStatus,
    resource.vmware?.instanceUuid,
    resource.vmware?.biosUuid,
    resource.vmware?.guestOsFamily,
    resource.vmware?.guestHostname,
    resource.vmware?.guestIpAddresses?.join(' '),
    resource.vmware?.datastoreNames?.join(' '),
    resource.vmware?.activeAlarmSummary,
    resource.vmware?.recentTaskSummary,
    ...(resource.tags ?? []),
  ]
    .filter((value): value is string => typeof value === 'string' && value.trim().length > 0)
    .join(' ')
    .toLowerCase();

export function filterVmwareVirtualMachines(
  vms: Resource[],
  search: string,
  status: VmwareVirtualMachineStatusFilter,
): Resource[] {
  const needle = normalize(search);
  return vms.filter((vm) => {
    if (status !== 'all' && mapVmwareVirtualMachineStatus(vm) !== status) return false;
    if (!needle) return true;
    return vmwareVirtualMachineSearchHaystack(vm).includes(needle);
  });
}
