import type { Accessor } from 'solid-js';
import type { Resource, ResourceType } from '@/types/resource';
import {
  SUPPORTED_PLATFORM_IDS,
  getSourcePlatformManifestEntry,
  type SourcePlatformManifestEntry,
} from '@/utils/platformSupportManifest';
import { normalizeSourcePlatformKey, resolveResourcePlatformType } from '@/utils/sourcePlatforms';

export type PrimaryPlatformNavId = 'proxmox' | 'docker' | 'kubernetes' | 'truenas' | 'vmware';

export type PlatformNavigationVisibility = Record<PrimaryPlatformNavId, boolean>;

export type PlatformNavigationShortcut = {
  key: string;
  route: string;
};

export const PRIMARY_PLATFORM_NAV_IDS: readonly PrimaryPlatformNavId[] = [
  'proxmox',
  'docker',
  'kubernetes',
  'truenas',
  'vmware',
] as const;

export const PRIMARY_PLATFORM_NAV_PLATFORM_IDS: Record<PrimaryPlatformNavId, readonly string[]> = {
  proxmox: ['proxmox-pve', 'proxmox-pbs', 'proxmox-pmg'],
  docker: ['docker'],
  kubernetes: ['kubernetes'],
  truenas: ['truenas'],
  vmware: ['vmware-vsphere'],
};

const SUPPORTED_PLATFORM_ID_SET = new Set<string>(SUPPORTED_PLATFORM_IDS);

const KUBERNETES_RESOURCE_TYPES = new Set<ResourceType>([
  'k8s-cluster',
  'k8s-node',
  'pod',
  'k8s-deployment',
  'k8s-service',
]);
const DOCKER_RESOURCE_TYPES = new Set<ResourceType>(['docker-host', 'docker-service']);

const asRecord = (value: unknown): Record<string, unknown> | null =>
  typeof value === 'object' && value !== null ? (value as Record<string, unknown>) : null;

const addManifestPlatformId = (ids: Set<string>, value: string | null | undefined): void => {
  const normalized = normalizeSourcePlatformKey(value);
  if (!normalized || normalized === 'generic') return;
  if (!getSourcePlatformManifestEntry(normalized)) return;
  ids.add(normalized);
};

const addPlatformDataSources = (ids: Set<string>, platformData: Record<string, unknown> | null) => {
  const sources = platformData?.sources;
  if (!Array.isArray(sources)) return;
  sources.forEach((source) => {
    if (typeof source === 'string') addManifestPlatformId(ids, source);
  });
};

export function collectResourcePlatformEvidence(resource: Resource): string[] {
  const ids = new Set<string>();
  addManifestPlatformId(ids, resource.platformType);
  addManifestPlatformId(ids, resolveResourcePlatformType(resource));
  resource.sources?.forEach((source) => addManifestPlatformId(ids, source));

  const platformData = asRecord(resource.platformData);
  addPlatformDataSources(ids, platformData);

  if (resource.type === 'pbs' || resource.pbs || asRecord(platformData?.pbs)) {
    ids.add('proxmox-pbs');
  }
  if (resource.type === 'pmg' || resource.pmg || asRecord(platformData?.pmg)) {
    ids.add('proxmox-pmg');
  }
  if (resource.proxmox || asRecord(platformData?.proxmox)) {
    ids.add('proxmox-pve');
  }
  if (resource.type === 'ceph' || resource.ceph || asRecord(platformData?.ceph)) {
    ids.add('proxmox-pve');
  }
  if (resource.vmware || asRecord(platformData?.vmware)) {
    ids.add('vmware-vsphere');
  }
  if (resource.kubernetes || asRecord(platformData?.kubernetes)) {
    ids.add('kubernetes');
  }
  if (
    resource.docker ||
    asRecord(platformData?.docker) ||
    DOCKER_RESOURCE_TYPES.has(resource.type)
  ) {
    ids.add('docker');
  }
  if (KUBERNETES_RESOURCE_TYPES.has(resource.type)) {
    ids.add('kubernetes');
  }

  return [...ids];
}

export function buildSupportedResourcePlatformSet(resources: readonly Resource[]): Set<string> {
  const present = new Set<string>();
  for (const resource of resources) {
    for (const platformId of collectResourcePlatformEvidence(resource)) {
      const manifestEntry = getSourcePlatformManifestEntry(
        platformId,
      ) as SourcePlatformManifestEntry | null;
      if (!manifestEntry || !SUPPORTED_PLATFORM_ID_SET.has(manifestEntry.id)) continue;
      present.add(manifestEntry.id);
    }
  }
  return present;
}

export function buildPrimaryPlatformNavigationVisibility(
  resources: readonly Resource[],
): PlatformNavigationVisibility {
  const presentSupportedPlatforms = buildSupportedResourcePlatformSet(resources);
  return {
    proxmox: PRIMARY_PLATFORM_NAV_PLATFORM_IDS.proxmox.some((id) =>
      presentSupportedPlatforms.has(id),
    ),
    docker: PRIMARY_PLATFORM_NAV_PLATFORM_IDS.docker.some((id) =>
      presentSupportedPlatforms.has(id),
    ),
    kubernetes: PRIMARY_PLATFORM_NAV_PLATFORM_IDS.kubernetes.some((id) =>
      presentSupportedPlatforms.has(id),
    ),
    truenas: PRIMARY_PLATFORM_NAV_PLATFORM_IDS.truenas.some((id) =>
      presentSupportedPlatforms.has(id),
    ),
    vmware: PRIMARY_PLATFORM_NAV_PLATFORM_IDS.vmware.some((id) =>
      presentSupportedPlatforms.has(id),
    ),
  };
}

export function primaryPlatformNavigationIsVisible(
  visibility: PlatformNavigationVisibility,
  platformId: PrimaryPlatformNavId,
): boolean {
  return visibility[platformId] === true;
}

export function selectFirstVisiblePrimaryPlatformNavigationId(
  visibility: PlatformNavigationVisibility,
): PrimaryPlatformNavId | null {
  return (
    PRIMARY_PLATFORM_NAV_IDS.find((platformId) =>
      primaryPlatformNavigationIsVisible(visibility, platformId),
    ) ?? null
  );
}

export function filterPlatformNavigationShortcuts(
  shortcuts: Record<string, PlatformNavigationShortcut>,
  visibility: PlatformNavigationVisibility,
): Record<string, string> {
  const routes: Record<string, string> = {};
  for (const [platformId, shortcut] of Object.entries(shortcuts)) {
    if (!primaryPlatformNavigationIsVisible(visibility, platformId as PrimaryPlatformNavId)) {
      continue;
    }
    routes[shortcut.key] = shortcut.route;
  }
  return routes;
}

export function createEmptyPlatformNavigationVisibility(): PlatformNavigationVisibility {
  return {
    proxmox: false,
    docker: false,
    kubernetes: false,
    truenas: false,
    vmware: false,
  };
}

export function platformNavigationVisibilityFromResources(
  resources: Accessor<readonly Resource[]>,
): PlatformNavigationVisibility {
  return buildPrimaryPlatformNavigationVisibility(resources());
}
