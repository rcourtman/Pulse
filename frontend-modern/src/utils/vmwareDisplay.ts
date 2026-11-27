import type { Resource, ResourceVMwareMeta } from '@/types/resource';
import { getResourceSourceStatus } from '@/utils/resourceSourceHealth';
import { buildProviderStaleNotice, type ProviderStaleNotice } from '@/utils/truenasDatasetState';

export const formatVmwareClusterServices = (meta: ResourceVMwareMeta | undefined): string => {
  const parts: string[] = [];
  if (meta?.clusterHaEnabled !== undefined) {
    parts.push(`HA ${meta.clusterHaEnabled ? 'enabled' : 'disabled'}`);
  }
  if (meta?.clusterDrsEnabled !== undefined) {
    parts.push(`DRS ${meta.clusterDrsEnabled ? 'enabled' : 'disabled'}`);
  }
  return parts.join(' · ');
};

export type VmwareStaleNotice = ProviderStaleNotice;

// Prefer top-level sourceStatus written by live WebSocket and REST adaptation,
// falling back to platformData.sourceStatus for older fixtures so live recovery
// clears a stale REST copy without waiting for a reload.
export const getVmwareStaleNotice = (resource: Resource): VmwareStaleNotice | null =>
  buildProviderStaleNotice(
    resource.sourceStatus?.vmware ??
      resource.sourceStatus?.['vmware-vsphere'] ??
      getResourceSourceStatus(resource, 'vmware-vsphere'),
    'vCenter',
  );

export const isVmwareSourceImpaired = (resource: Resource): boolean =>
  getVmwareStaleNotice(resource) !== null;
