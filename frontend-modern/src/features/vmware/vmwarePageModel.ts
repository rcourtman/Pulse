import { resolveResourcePlatformType } from '@/utils/sourcePlatforms';
import type { Resource, ResourceType } from '@/types/resource';

export type VmwarePageTabId = 'overview' | 'vms' | 'storage';

export type VmwareTabSpec = {
  id: VmwarePageTabId;
  label: string;
  path: string;
};

export const VMWARE_TAB_SPECS: readonly VmwareTabSpec[] = [
  { id: 'overview', label: 'Hosts', path: '/vmware/overview' },
  { id: 'vms', label: 'Virtual machines', path: '/vmware/vms' },
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
