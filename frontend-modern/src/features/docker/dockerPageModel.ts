import { resolveResourcePlatformType } from '@/utils/sourcePlatforms';
import type { DockerStorageUsageMeta, Resource, ResourceType } from '@/types/resource';
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
  | 'storage'
  | 'networks'
  | 'swarm';

export type DockerTabSpec = {
  id: DockerPageTabId;
  label: string;
  path: string;
};

export const DOCKER_TAB_SPECS: readonly DockerTabSpec[] = [
  // Keep the runtime lens at operator-workflow granularity. Overview owns
  // runtime hosts; detailed object inventory belongs in the Containers,
  // Images, Storage, Networks, and Swarm workflows so the page does not repeat
  // the same tables in multiple places.
  { id: 'overview', label: 'Overview', path: '/docker/overview' },
  { id: 'containers', label: 'Containers', path: '/docker/containers' },
  { id: 'images', label: 'Images', path: '/docker/images' },
  { id: 'storage', label: 'Storage', path: '/docker/storage' },
  { id: 'networks', label: 'Networks', path: '/docker/networks' },
  { id: 'swarm', label: 'Swarm', path: '/docker/swarm' },
] as const;

const asTrimmedString = (value: unknown): string => (typeof value === 'string' ? value.trim() : '');

const DOCKER_ROUTE_TAB_ALIASES: Record<string, DockerPageTabId> = {
  configs: 'swarm',
  secrets: 'swarm',
  services: 'swarm',
  'swarm-nodes': 'swarm',
  tasks: 'swarm',
  volumes: 'storage',
};

export const resolveDockerPageTabId = (segment: string | undefined): DockerPageTabId => {
  const normalized = asTrimmedString(segment).toLowerCase();
  if (!normalized) return 'overview';
  const direct = DOCKER_TAB_SPECS.find((tab) => tab.id === normalized);
  if (direct) return direct.id;
  return DOCKER_ROUTE_TAB_ALIASES[normalized] ?? 'overview';
};

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

export const hasDockerStorageUsageBucket = (bucket?: DockerStorageUsageMeta): boolean =>
  Boolean(
    bucket &&
      ((bucket.totalCount ?? 0) > 0 ||
        (bucket.activeCount ?? 0) > 0 ||
        (bucket.totalSizeBytes ?? 0) > 0 ||
        (bucket.reclaimableBytes ?? 0) > 0),
  );

export const hasDockerEngineStorageUsage = (host: Resource): boolean =>
  hasDockerStorageUsageBucket(host.docker?.imagesUsage) ||
  hasDockerStorageUsageBucket(host.docker?.containersUsage) ||
  hasDockerStorageUsageBucket(host.docker?.volumesUsage) ||
  hasDockerStorageUsageBucket(host.docker?.buildCacheUsage);

export const hasDockerSwarmInventory = (model: DockerPageModel): boolean =>
  model.hosts.some(hasDockerSwarmEvidence) ||
  model.services.length > 0 ||
  model.tasks.length > 0 ||
  model.nodes.length > 0 ||
  model.secrets.length > 0 ||
  model.configs.length > 0;

export const getDockerPageTabSpecs = (model: DockerPageModel): readonly DockerTabSpec[] =>
  DOCKER_TAB_SPECS.filter((tab) => tab.id !== 'swarm' || hasDockerSwarmInventory(model));

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
