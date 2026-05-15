import { resolveResourcePlatformType } from '@/utils/sourcePlatforms';
import type { Resource, ResourceType } from '@/types/resource';

// The canonical unified resource adapter does not yet project a top-level
// TrueNAS system (`agent` row tagged with the `truenas` platform) at the
// /api/resources boundary, so the Hosts overview tab is intentionally absent
// from the page until that gap is closed. Storage and Apps remain the
// canonical operator entry points.
export type TrueNASPageTabId = 'storage' | 'apps';

export type TrueNASTabSpec = {
  id: TrueNASPageTabId;
  label: string;
  path: string;
};

export const TRUENAS_TAB_SPECS: readonly TrueNASTabSpec[] = [
  { id: 'storage', label: 'Storage', path: '/truenas/storage' },
  { id: 'apps', label: 'Apps', path: '/truenas/apps' },
] as const;

const TRUENAS_RESOURCE_TYPES = new Set<ResourceType>([
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
  apps: Resource[];
};

export function buildTrueNASPageModel(resources: Resource[]): TrueNASPageModel {
  const trueNasResources = resources.filter(
    (resource) => isTrueNASPlatform(resource) && TRUENAS_RESOURCE_TYPES.has(resource.type),
  );

  const apps = trueNasResources.filter((resource) => resource.type === 'app-container');

  return {
    resources: trueNasResources,
    apps,
  };
}
