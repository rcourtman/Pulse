import { resolveResourcePlatformType } from '@/utils/sourcePlatforms';
import type { Resource, ResourceType } from '@/types/resource';
import {
  getInfrastructureSystemIdentityBadges,
  type ResourceBadge,
} from '@/utils/resourceBadgePresentation';

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

const DOCKER_CONTAINER_BASE_DEFAULT_HIDDEN_COLUMNS = ['disk', 'tags'] as const;

const hasFiniteMetric = (value: unknown): value is number =>
  typeof value === 'number' && Number.isFinite(value);

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

export function buildDockerContainerDefaultHiddenColumnIds(
  containers: readonly Resource[],
): string[] {
  const hasNetworkIOTelemetry = containers.some(
    (container) =>
      hasFiniteMetric(container.network?.rxBytes) || hasFiniteMetric(container.network?.txBytes),
  );
  const hasDiskIOTelemetry = containers.some(
    (container) =>
      hasFiniteMetric(container.diskIO?.readRate) || hasFiniteMetric(container.diskIO?.writeRate),
  );

  return [
    ...DOCKER_CONTAINER_BASE_DEFAULT_HIDDEN_COLUMNS,
    ...(hasNetworkIOTelemetry ? [] : ['netIo']),
    ...(hasDiskIOTelemetry ? [] : ['diskIo']),
  ];
}

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
