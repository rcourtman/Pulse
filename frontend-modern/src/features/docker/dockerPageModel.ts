import { resolveResourcePlatformType } from '@/utils/sourcePlatforms';
import type { Resource, ResourceType } from '@/types/resource';
import type { WorkloadsStatusMode } from '@/components/Workloads/workloadsFilterModel';
import {
  getInfrastructureSystemIdentityBadges,
  type ResourceBadge,
} from '@/utils/resourceBadgePresentation';
import { DEGRADED_HEALTH_STATUSES, OFFLINE_HEALTH_STATUSES } from '@/utils/status';
import { buildAppContainerDefaultHiddenColumnIds } from '@/features/platformPage/appContainerColumns';

const DOCKER_HOST_TYPES = new Set<ResourceType>(['agent', 'docker-host']);
const DOCKER_CONTAINER_TYPES = new Set<ResourceType>(['app-container']);
const DOCKER_SERVICE_TYPES = new Set<ResourceType>(['docker-service']);

const asTrimmedString = (value: unknown): string => (typeof value === 'string' ? value.trim() : '');

const isDockerPlatform = (resource: Resource): boolean =>
  resolveResourcePlatformType(resource) === 'docker';

export function hasDockerSwarmEvidence(resource: Resource): boolean {
  const swarm = resource.docker?.swarm;
  if (!swarm) return false;

  const localState = asTrimmedString(swarm.localState).toLowerCase();
  if (localState === 'inactive') {
    return (
      swarm.controlAvailable === true ||
      asTrimmedString(swarm.clusterId) !== '' ||
      asTrimmedString(swarm.clusterName) !== '' ||
      asTrimmedString(swarm.error) !== ''
    );
  }

  return (
    asTrimmedString(swarm.nodeId) !== '' ||
    localState !== '' ||
    swarm.controlAvailable === true ||
    asTrimmedString(swarm.clusterId) !== '' ||
    asTrimmedString(swarm.clusterName) !== '' ||
    asTrimmedString(swarm.error) !== ''
  );
}

export type DockerPageModel = {
  resources: Resource[];
  hosts: Resource[];
  containers: Resource[];
  services: Resource[];
};

export interface DockerPageFilters {
  containerRuntime?: string | null;
  searchTerm?: string | null;
  selectedHostScope?: string | null;
  statusMode?: WorkloadsStatusMode;
}

const RUNTIME_ONLY_SYSTEM_LABELS = new Set(['docker', 'docker / podman', 'podman']);

const addDockerGroupBadgeAlias = (
  badges: Record<string, ResourceBadge>,
  alias: string | undefined,
  badge: ResourceBadge,
): void => {
  const normalized = (alias || '').trim();
  if (!normalized) return;
  const key = `app-container:${normalized}`;
  if (!badges[key]) {
    badges[key] = badge;
  }
  const lowerKey = key.toLowerCase();
  if (!badges[lowerKey]) {
    badges[lowerKey] = badge;
  }
};

export const getDockerHostSystemBadge = (host: Resource): ResourceBadge | undefined =>
  getInfrastructureSystemIdentityBadges(host).find(
    (badge) => !RUNTIME_ONLY_SYSTEM_LABELS.has(badge.label.trim().toLowerCase()),
  );

export function buildDockerWorkloadGroupLabelBadges(
  hosts: readonly Resource[],
): Record<string, ResourceBadge> {
  const badges: Record<string, ResourceBadge> = {};

  for (const host of hosts) {
    const badge = getDockerHostSystemBadge(host);
    if (!badge) continue;

    addDockerGroupBadgeAlias(badges, host.name, badge);
    addDockerGroupBadgeAlias(badges, host.displayName, badge);
    addDockerGroupBadgeAlias(badges, host.id, badge);
    addDockerGroupBadgeAlias(badges, host.agent?.hostname, badge);
    addDockerGroupBadgeAlias(badges, host.docker?.hostname, badge);
    addDockerGroupBadgeAlias(badges, host.identity?.hostname, badge);
    addDockerGroupBadgeAlias(badges, host.canonicalIdentity?.displayName, badge);
    addDockerGroupBadgeAlias(badges, host.canonicalIdentity?.hostname, badge);
    addDockerGroupBadgeAlias(badges, host.canonicalIdentity?.primaryId, badge);
    host.canonicalIdentity?.aliases?.forEach((alias) =>
      addDockerGroupBadgeAlias(badges, alias, badge),
    );
  }

  return badges;
}

export const buildDockerContainerDefaultHiddenColumnIds = buildAppContainerDefaultHiddenColumnIds;

export function buildDockerPageModel(resources: Resource[]): DockerPageModel {
  const dockerResources = resources.filter(
    (resource) =>
      isDockerPlatform(resource) ||
      DOCKER_CONTAINER_TYPES.has(resource.type) ||
      DOCKER_SERVICE_TYPES.has(resource.type),
  );

  const hosts = dockerResources.filter(
    (resource) => DOCKER_HOST_TYPES.has(resource.type) && isDockerPlatform(resource),
  );
  const containers = dockerResources.filter(
    (resource) => DOCKER_CONTAINER_TYPES.has(resource.type) && isDockerPlatform(resource),
  );
  const services = dockerResources.filter((resource) => DOCKER_SERVICE_TYPES.has(resource.type));

  return {
    resources: dockerResources,
    hosts,
    containers,
    services,
  };
}

const resourceSearchCandidates = (resource: Resource): Array<string | undefined> => [
  resource.name,
  resource.displayName,
  resource.id,
  resource.parentName,
  resource.agent?.hostname,
  resource.docker?.hostname,
  resource.docker?.runtime,
  resource.docker?.runtimeVersion,
  resource.docker?.dockerVersion,
  resource.docker?.os,
  resource.docker?.kernelVersion,
  resource.docker?.architecture,
  resource.docker?.swarm?.clusterName,
  resource.docker?.swarm?.nodeRole,
  resource.identity?.hostname,
  resource.canonicalIdentity?.displayName,
  resource.canonicalIdentity?.hostname,
  resource.canonicalIdentity?.primaryId,
  ...(resource.canonicalIdentity?.aliases ?? []),
  ...(resource.tags ?? []),
];

const matchesSearch = (resource: Resource, searchTerm: string): boolean => {
  const needle = searchTerm.trim().toLowerCase();
  if (!needle) return true;
  return resourceSearchCandidates(resource)
    .filter((value): value is string => typeof value === 'string')
    .some((value) => value.toLowerCase().includes(needle));
};

const matchesStatusMode = (
  resource: Resource,
  statusMode: WorkloadsStatusMode | undefined,
): boolean => {
  if (!statusMode || statusMode === 'all') return true;
  const normalizedStatus = (resource.status || '').trim().toLowerCase();
  if (statusMode === 'running') {
    return normalizedStatus === 'running' || normalizedStatus === 'online';
  }
  if (statusMode === 'degraded') {
    return (
      DEGRADED_HEALTH_STATUSES.has(normalizedStatus) ||
      (normalizedStatus !== 'running' &&
        normalizedStatus !== 'online' &&
        !OFFLINE_HEALTH_STATUSES.has(normalizedStatus))
    );
  }
  return OFFLINE_HEALTH_STATUSES.has(normalizedStatus);
};

const matchesContainerRuntime = (
  resource: Resource,
  containerRuntime: string | null | undefined,
): boolean => {
  const normalizedRuntime = (containerRuntime || '').trim().toLowerCase();
  if (!normalizedRuntime) return true;
  return (resource.docker?.runtime || '').trim().toLowerCase() === normalizedRuntime;
};

const matchesHostScope = (
  resource: Resource,
  selectedHostScope: string | null | undefined,
): boolean => {
  const normalizedScope = (selectedHostScope || '').trim().toLowerCase();
  if (!normalizedScope) return true;

  const candidates = [
    resource.id,
    resource.name,
    resource.displayName,
    resource.docker?.hostSourceId,
    resource.docker?.hostname,
    resource.agent?.hostname,
    resource.identity?.hostname,
    resource.canonicalIdentity?.displayName,
    resource.canonicalIdentity?.hostname,
    resource.canonicalIdentity?.primaryId,
    ...(resource.canonicalIdentity?.aliases ?? []),
  ]
    .filter((value): value is string => typeof value === 'string')
    .map((value) => value.trim().toLowerCase())
    .filter((value) => value.length > 0);

  return candidates.includes(normalizedScope);
};

export function filterDockerHosts(
  resources: readonly Resource[],
  filters: DockerPageFilters,
): Resource[] {
  return resources.filter(
    (resource) =>
      matchesSearch(resource, filters.searchTerm || '') &&
      matchesStatusMode(resource, filters.statusMode) &&
      matchesContainerRuntime(resource, filters.containerRuntime) &&
      matchesHostScope(resource, filters.selectedHostScope),
  );
}

const serviceSearchCandidates = (resource: Resource): Array<string | undefined> => [
  ...resourceSearchCandidates(resource),
  resource.docker?.image,
  resource.docker?.mode,
];

export function filterDockerServices(
  resources: readonly Resource[],
  filters: DockerPageFilters,
): Resource[] {
  const normalizedRuntime = (filters.containerRuntime || '').trim().toLowerCase();
  if (normalizedRuntime === 'podman') {
    return [];
  }

  return resources.filter(
    (resource) =>
      serviceSearchCandidates(resource)
        .filter((value): value is string => typeof value === 'string')
        .some((value) => {
          const needle = (filters.searchTerm || '').trim().toLowerCase();
          return needle.length === 0 ? true : value.toLowerCase().includes(needle);
        }) &&
      matchesStatusMode(resource, filters.statusMode) &&
      matchesHostScope(resource, filters.selectedHostScope),
  );
}
