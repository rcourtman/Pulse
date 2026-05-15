import { normalizeSourcePlatformQueryValue } from '@/utils/sourcePlatforms';
import type { Resource, ResourceType } from '@/types/resource';

export type DockerPageTabId = 'overview' | 'containers' | 'services';

export type DockerTabSpec = {
  id: DockerPageTabId;
  label: string;
  path: string;
};

export const DOCKER_TAB_SPECS: readonly DockerTabSpec[] = [
  { id: 'overview', label: 'Hosts', path: '/docker/overview' },
  { id: 'containers', label: 'Containers', path: '/docker/containers' },
  { id: 'services', label: 'Swarm services', path: '/docker/services' },
] as const;

const DOCKER_HOST_TYPES = new Set<ResourceType>(['agent', 'docker-host']);
const DOCKER_CONTAINER_TYPES = new Set<ResourceType>(['app-container']);
const DOCKER_SERVICE_TYPES = new Set<ResourceType>(['docker-service']);

const isDockerPlatform = (resource: Resource): boolean =>
  normalizeSourcePlatformQueryValue(resource.platformType || '') === 'docker';

export type DockerPageModel = {
  resources: Resource[];
  hosts: Resource[];
  containers: Resource[];
  services: Resource[];
};

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
  const services = dockerResources.filter(
    (resource) => DOCKER_SERVICE_TYPES.has(resource.type) && isDockerPlatform(resource),
  );

  return {
    resources: dockerResources,
    hosts,
    containers,
    services,
  };
}
