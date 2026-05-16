import { resolveResourcePlatformType } from '@/utils/sourcePlatforms';
import type { Resource, ResourceType } from '@/types/resource';

export type TrueNASPageTabId = 'overview' | 'storage' | 'apps';

export type TrueNASTabSpec = {
  id: TrueNASPageTabId;
  label: string;
  path: string;
};

export const TRUENAS_TAB_SPECS: readonly TrueNASTabSpec[] = [
  { id: 'overview', label: 'Systems', path: '/truenas/overview' },
  { id: 'storage', label: 'Storage', path: '/truenas/storage' },
  { id: 'apps', label: 'Apps', path: '/truenas/apps' },
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
