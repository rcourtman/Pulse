import { resolveResourcePlatformType } from '@/utils/sourcePlatforms';
import type { Resource, ResourceType } from '@/types/resource';

const DOCKER_HOST_TYPES = new Set<ResourceType>(['agent', 'docker-host']);
const DOCKER_CONTAINER_TYPES = new Set<ResourceType>(['app-container']);
const DOCKER_SERVICE_TYPES = new Set<ResourceType>(['docker-service']);

const asTrimmedString = (value: unknown): string =>
  typeof value === 'string' ? value.trim() : '';

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

export function buildDockerContainerDefaultHiddenColumnIds(
  containers: readonly Resource[],
): string[] {
  const hasNetworkIOTelemetry = containers.some(
    (container) =>
      hasFiniteMetric(container.network?.rxBytes) ||
      hasFiniteMetric(container.network?.txBytes),
  );
  const hasDiskIOTelemetry = containers.some(
    (container) =>
      hasFiniteMetric(container.diskIO?.readRate) ||
      hasFiniteMetric(container.diskIO?.writeRate),
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
  const services = dockerResources.filter((resource) =>
    DOCKER_SERVICE_TYPES.has(resource.type),
  );

  return {
    resources: dockerResources,
    hosts,
    containers,
    services,
  };
}

