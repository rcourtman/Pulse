import { resolveResourcePlatformType } from '@/utils/sourcePlatforms';
import type { Resource, ResourceType } from '@/types/resource';

// Docker Swarm services are not yet emitted as `docker-service` resources by
// the canonical unified resource adapter at the /api/resources boundary, so
// the Swarm services sub-tab is intentionally absent from the page until
// that gap is closed.
export type DockerPageTabId = 'overview' | 'containers';

export type DockerTabSpec = {
  id: DockerPageTabId;
  label: string;
  path: string;
};

export const DOCKER_TAB_SPECS: readonly DockerTabSpec[] = [
  { id: 'overview', label: 'Hosts', path: '/docker/overview' },
  { id: 'containers', label: 'Containers', path: '/docker/containers' },
] as const;

const DOCKER_HOST_TYPES = new Set<ResourceType>(['agent', 'docker-host']);
const DOCKER_CONTAINER_TYPES = new Set<ResourceType>(['app-container']);

const isDockerPlatform = (resource: Resource): boolean =>
  resolveResourcePlatformType(resource) === 'docker';

export type DockerPageModel = {
  resources: Resource[];
  hosts: Resource[];
  containers: Resource[];
};

export function buildDockerPageModel(resources: Resource[]): DockerPageModel {
  const dockerResources = resources.filter(
    (resource) => isDockerPlatform(resource) || DOCKER_CONTAINER_TYPES.has(resource.type),
  );

  const hosts = dockerResources.filter(
    (resource) => DOCKER_HOST_TYPES.has(resource.type) && isDockerPlatform(resource),
  );
  const containers = dockerResources.filter(
    (resource) => DOCKER_CONTAINER_TYPES.has(resource.type) && isDockerPlatform(resource),
  );

  return {
    resources: dockerResources,
    hosts,
    containers,
  };
}
