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

export type VmwareTabSpec = {
  id: VmwarePageTabId;
  label: string;
  path: string;
};

// The Overview tab mirrors Proxmox: hosts on top, embedded WorkloadsSurface
// (VMs) underneath grouped by host. A dedicated `vms` tab would just remount
// the same WorkloadsSurface, so it's intentionally absent — the Workloads
// filter inside Overview owns search/grouping for VMs.
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
  const vms = vmwareResources.filter((resource) => resource.type === 'vm');
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

const vmwareDatastoreDisplayName = (resource: Resource): string =>
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
