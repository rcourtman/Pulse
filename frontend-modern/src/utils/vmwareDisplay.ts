import type { ResourceVMwareMeta } from '@/types/resource';

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
