import { resolveResourcePlatformType } from '@/utils/sourcePlatforms';
import type { Resource, ResourceType } from '@/types/resource';
import {
  getInfrastructureSystemIdentityBadges,
  type ResourceBadge,
} from '@/utils/resourceBadgePresentation';

const DOCKER_HOST_TYPES = new Set<ResourceType>(['agent', 'docker-host']);
const DOCKER_CONTAINER_TYPES = new Set<ResourceType>(['app-container']);
const DOCKER_SERVICE_TYPES = new Set<ResourceType>(['docker-service']);
const DOCKER_IMAGE_TYPES = new Set<ResourceType>(['docker-image']);
const DOCKER_VOLUME_TYPES = new Set<ResourceType>(['docker-volume']);
const DOCKER_NETWORK_TYPES = new Set<ResourceType>(['docker-network']);
const DOCKER_TASK_TYPES = new Set<ResourceType>(['docker-task']);
const DOCKER_SWARM_NODE_TYPES = new Set<ResourceType>(['docker-swarm-node']);
const DOCKER_SECRET_TYPES = new Set<ResourceType>(['docker-secret']);
const DOCKER_CONFIG_TYPES = new Set<ResourceType>(['docker-config']);

export type DockerPageTabId =
  | 'overview'
  | 'containers'
  | 'images'
  | 'volumes'
  | 'networks'
  | 'storage'
  | 'swarm-nodes'
  | 'services'
  | 'tasks'
  | 'secrets'
  | 'configs';

export const DOCKER_TAB_SPECS: readonly {
  id: DockerPageTabId;
  label: string;
  path: string;
}[] = [
  { id: 'overview', label: 'Overview', path: '/docker/overview' },
  { id: 'containers', label: 'Containers', path: '/docker/containers' },
  { id: 'images', label: 'Images', path: '/docker/images' },
  { id: 'volumes', label: 'Volumes', path: '/docker/volumes' },
  { id: 'networks', label: 'Networks', path: '/docker/networks' },
  { id: 'storage', label: 'Storage', path: '/docker/storage' },
  { id: 'swarm-nodes', label: 'Swarm Nodes', path: '/docker/swarm-nodes' },
  { id: 'services', label: 'Services', path: '/docker/services' },
  { id: 'tasks', label: 'Tasks', path: '/docker/tasks' },
  { id: 'secrets', label: 'Secrets', path: '/docker/secrets' },
  { id: 'configs', label: 'Configs', path: '/docker/configs' },
] as const;

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
  images: Resource[];
  volumes: Resource[];
  networks: Resource[];
  nodes: Resource[];
  tasks: Resource[];
  secrets: Resource[];
  configs: Resource[];
};

const RUNTIME_ONLY_SYSTEM_LABELS = new Set(['docker', 'docker / podman', 'podman']);

export const getDockerHostSystemBadge = (host: Resource): ResourceBadge | undefined =>
  getInfrastructureSystemIdentityBadges(host).find(
    (badge) => !RUNTIME_ONLY_SYSTEM_LABELS.has(badge.label.trim().toLowerCase()),
  );

export function buildDockerPageModel(resources: Resource[]): DockerPageModel {
  const dockerResources = resources.filter(
    (resource) =>
      isDockerPlatform(resource) ||
      DOCKER_CONTAINER_TYPES.has(resource.type) ||
      DOCKER_SERVICE_TYPES.has(resource.type) ||
      DOCKER_IMAGE_TYPES.has(resource.type) ||
      DOCKER_VOLUME_TYPES.has(resource.type) ||
      DOCKER_NETWORK_TYPES.has(resource.type) ||
      DOCKER_SWARM_NODE_TYPES.has(resource.type) ||
      DOCKER_SECRET_TYPES.has(resource.type) ||
      DOCKER_CONFIG_TYPES.has(resource.type) ||
      DOCKER_TASK_TYPES.has(resource.type),
  );

  const hosts = dockerResources.filter(
    (resource) => DOCKER_HOST_TYPES.has(resource.type) && isDockerPlatform(resource),
  );
  const containers = dockerResources.filter(
    (resource) => DOCKER_CONTAINER_TYPES.has(resource.type) && isDockerPlatform(resource),
  );
  const services = dockerResources.filter((resource) => DOCKER_SERVICE_TYPES.has(resource.type));
  const images = dockerResources.filter((resource) => DOCKER_IMAGE_TYPES.has(resource.type));
  const volumes = dockerResources.filter((resource) => DOCKER_VOLUME_TYPES.has(resource.type));
  const networks = dockerResources.filter((resource) => DOCKER_NETWORK_TYPES.has(resource.type));
  const nodes = dockerResources.filter((resource) => DOCKER_SWARM_NODE_TYPES.has(resource.type));
  const tasks = dockerResources.filter((resource) => DOCKER_TASK_TYPES.has(resource.type));
  const secrets = dockerResources.filter((resource) => DOCKER_SECRET_TYPES.has(resource.type));
  const configs = dockerResources.filter((resource) => DOCKER_CONFIG_TYPES.has(resource.type));

  return {
    resources: dockerResources,
    hosts,
    containers,
    services,
    images,
    volumes,
    networks,
    nodes,
    tasks,
    secrets,
    configs,
  };
}
