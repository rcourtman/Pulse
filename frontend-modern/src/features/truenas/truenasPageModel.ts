import { resolveResourcePlatformType } from '@/utils/sourcePlatforms';
import type { Resource, ResourceType } from '@/types/resource';

export type TrueNASPageTabId = 'overview' | 'storage';

export type TrueNASTabSpec = {
  id: TrueNASPageTabId;
  label: string;
  path: string;
};

// The Overview tab mirrors Proxmox / vSphere: systems table on top with the
// pools-only StorageSurface, physical disks, and apps stacked beneath. The
// standalone Disks and Apps tabs that used to live here were pure
// duplicates — the Workloads filter inside Overview owns app
// search/grouping, and the Storage tab provides the full StorageSurface
// (pools + disks toggle, with toolbar) for richer storage exploration.
export const TRUENAS_TAB_SPECS: readonly TrueNASTabSpec[] = [
  { id: 'overview', label: 'Overview', path: '/truenas/overview' },
  { id: 'storage', label: 'Storage', path: '/truenas/storage' },
] as const;

const TRUENAS_RESOURCE_TYPES = new Set<ResourceType>([
  'agent',
  'app-container',
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
  disks: Resource[];
  apps: Resource[];
};

export function buildTrueNASPageModel(resources: Resource[]): TrueNASPageModel {
  const trueNasResources = resources.filter(
    (resource) => isTrueNASPlatform(resource) && TRUENAS_RESOURCE_TYPES.has(resource.type),
  );

  const systems = trueNasResources.filter((resource) => resource.type === 'agent');
  const apps = trueNasResources.filter((resource) => resource.type === 'app-container');
  const disks = trueNasResources.filter((resource) => resource.type === 'physical_disk');

  return {
    resources: trueNasResources,
    systems,
    disks,
    apps,
  };
}
