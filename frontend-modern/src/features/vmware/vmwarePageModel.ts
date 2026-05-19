import { resolveResourcePlatformType } from '@/utils/sourcePlatforms';
import type { Resource, ResourceType } from '@/types/resource';

export type VmwarePageTabId = 'overview' | 'storage';

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
  { id: 'storage', label: 'Storage', path: '/vmware/storage' },
] as const;

const VMWARE_RESOURCE_TYPES = new Set<ResourceType>([
  'agent',
  'vm',
  'storage',
  'datastore',
]);

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
  const datastores = vmwareResources.filter(
    (resource) =>
      resource.type === 'storage' &&
      (resource.storage?.topology === 'datastore' || resource.vmware?.entityType === 'datastore'),
  );

  return {
    resources: vmwareResources,
    hosts,
    vms,
    datastores,
  };
}
