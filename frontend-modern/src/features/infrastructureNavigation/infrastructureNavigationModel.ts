import type { Accessor } from 'solid-js';
import type { Resource, ResourceType } from '@/types/resource';
import {
  ADMITTED_PLATFORM_IDS,
  SUPPORTED_PLATFORM_IDS,
  getSourcePlatformManifestEntry,
  type SourcePlatformManifestEntry,
} from '@/utils/platformSupportManifest';
import { normalizeSourcePlatformKey, resolveResourcePlatformType } from '@/utils/sourcePlatforms';

export type PrimaryInfrastructureNavId = 'proxmox' | 'docker' | 'kubernetes' | 'truenas' | 'vmware';

export type InfrastructureNavigationVisibility = Record<PrimaryInfrastructureNavId, boolean>;

export type InfrastructureNavigationShortcut = {
  key: string;
  route: string;
};

export const PRIMARY_INFRASTRUCTURE_NAV_IDS: readonly PrimaryInfrastructureNavId[] = [
  'proxmox',
  'docker',
  'kubernetes',
  'truenas',
  'vmware',
] as const;

export const PRIMARY_INFRASTRUCTURE_NAV_SCOPE_IDS: Record<
  PrimaryInfrastructureNavId,
  readonly string[]
> = {
  proxmox: ['proxmox-pve', 'proxmox-pbs', 'proxmox-pmg'],
  docker: ['docker'],
  kubernetes: ['kubernetes'],
  truenas: ['truenas'],
  vmware: ['vmware-vsphere'],
};

const NAVIGABLE_PLATFORM_ID_SET = new Set<string>([
  ...SUPPORTED_PLATFORM_IDS,
  ...ADMITTED_PLATFORM_IDS,
]);

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

const addPlatformScopeEvidence = (ids: Set<string>, values: unknown): void => {
  if (!Array.isArray(values)) return;
  values.forEach((value) => {
    if (typeof value === 'string') addManifestPlatformId(ids, value);
  });
};

const collectExplicitPlatformScopeEvidence = (
  resource: Resource,
  platformData: Record<string, unknown> | null,
): string[] => {
  const ids = new Set<string>();
  addPlatformScopeEvidence(ids, resource.platformScopes);
  addPlatformScopeEvidence(ids, platformData?.platformScopes);
  return [...ids];
};

export function collectResourcePlatformEvidence(resource: Resource): string[] {
  const platformData = asRecord(resource.platformData);
  const platformScopeIds = collectExplicitPlatformScopeEvidence(resource, platformData);
  if (platformScopeIds.length > 0) {
    return platformScopeIds;
  }

  const ids = new Set<string>();
  addManifestPlatformId(ids, resource.platformType);
  addManifestPlatformId(ids, resolveResourcePlatformType(resource));
  resource.sources?.forEach((source) => addManifestPlatformId(ids, source));

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

export function buildNavigableResourceInfrastructureScopeSet(
  resources: readonly Resource[],
): Set<string> {
  const present = new Set<string>();
  for (const resource of resources) {
    for (const platformId of collectResourcePlatformEvidence(resource)) {
      const manifestEntry = getSourcePlatformManifestEntry(
        platformId,
      ) as SourcePlatformManifestEntry | null;
      if (!manifestEntry || !NAVIGABLE_PLATFORM_ID_SET.has(manifestEntry.id)) continue;
      present.add(manifestEntry.id);
    }
  }
  return present;
}

export function buildPrimaryInfrastructureNavigationVisibility(
  resources: readonly Resource[],
): InfrastructureNavigationVisibility {
  const presentNavigableScopes = buildNavigableResourceInfrastructureScopeSet(resources);
  return {
    proxmox: PRIMARY_INFRASTRUCTURE_NAV_SCOPE_IDS.proxmox.some((id) =>
      presentNavigableScopes.has(id),
    ),
    docker: PRIMARY_INFRASTRUCTURE_NAV_SCOPE_IDS.docker.some((id) =>
      presentNavigableScopes.has(id),
    ),
    kubernetes: PRIMARY_INFRASTRUCTURE_NAV_SCOPE_IDS.kubernetes.some((id) =>
      presentNavigableScopes.has(id),
    ),
    truenas: PRIMARY_INFRASTRUCTURE_NAV_SCOPE_IDS.truenas.some((id) =>
      presentNavigableScopes.has(id),
    ),
    vmware: PRIMARY_INFRASTRUCTURE_NAV_SCOPE_IDS.vmware.some((id) =>
      presentNavigableScopes.has(id),
    ),
  };
}

export function primaryInfrastructureNavigationIsVisible(
  visibility: InfrastructureNavigationVisibility,
  navId: PrimaryInfrastructureNavId,
): boolean {
  return visibility[navId] === true;
}

export function selectFirstVisiblePrimaryInfrastructureNavigationId(
  visibility: InfrastructureNavigationVisibility,
): PrimaryInfrastructureNavId | null {
  return (
    PRIMARY_INFRASTRUCTURE_NAV_IDS.find((navId) =>
      primaryInfrastructureNavigationIsVisible(visibility, navId),
    ) ?? null
  );
}

export function filterInfrastructureNavigationShortcuts(
  shortcuts: Record<string, InfrastructureNavigationShortcut>,
  visibility: InfrastructureNavigationVisibility,
): Record<string, string> {
  const routes: Record<string, string> = {};
  for (const [navId, shortcut] of Object.entries(shortcuts)) {
    if (
      !primaryInfrastructureNavigationIsVisible(visibility, navId as PrimaryInfrastructureNavId)
    ) {
      continue;
    }
    routes[shortcut.key] = shortcut.route;
  }
  return routes;
}

export function createEmptyInfrastructureNavigationVisibility(): InfrastructureNavigationVisibility {
  return {
    proxmox: false,
    docker: false,
    kubernetes: false,
    truenas: false,
    vmware: false,
  };
}

export function infrastructureNavigationVisibilityFromResources(
  resources: Accessor<readonly Resource[]>,
): InfrastructureNavigationVisibility {
  return buildPrimaryInfrastructureNavigationVisibility(resources());
}
