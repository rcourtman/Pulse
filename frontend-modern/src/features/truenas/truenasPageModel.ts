import { resolveResourcePlatformType } from '@/utils/sourcePlatforms';
import type { Resource, ResourceType } from '@/types/resource';

export type TrueNASPageTabId = 'overview' | 'storage';

export type TrueNASTabSpec = {
  id: TrueNASPageTabId;
  label: string;
  path: string;
};

// The Overview tab is intentionally narrow: appliance systems first, then apps
// when present. Storage inventory, pool topology, and physical disks all live
// on the Storage tab so operators have one canonical storage surface instead
// of a duplicated overview snapshot plus a richer storage page.
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
  apps: Resource[];
};

export function buildTrueNASPageModel(resources: Resource[]): TrueNASPageModel {
  const trueNasResources = resources.filter(
    (resource) => isTrueNASPlatform(resource) && TRUENAS_RESOURCE_TYPES.has(resource.type),
  );

  const systems = trueNasResources.filter((resource) => resource.type === 'agent');
  const apps = trueNasResources.filter((resource) => resource.type === 'app-container');
  return {
    resources: trueNasResources,
    systems,
    apps,
  };
}
