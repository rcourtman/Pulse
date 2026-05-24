import { resolveResourcePlatformType } from '@/utils/sourcePlatforms';
import type { DockerStorageUsageMeta, Resource, ResourceType } from '@/types/resource';
import type { StatusIndicator, StatusIndicatorVariant } from '@/utils/status';
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

// `containerState` reasons that mean the container will not stay up on its
// own. Distinct from `exited` with exit code 0, which is just a stopped
// container, and from `restarting`, which is in-flight.
const FATAL_CONTAINER_STATES = new Set([
  'dead',
  'removing',
  'oomkilled',
]);

// Container states that read as deliberately stopped rather than as a
// problem. Pulse should surface these as muted, not as success or danger.
const STOPPED_CONTAINER_STATES = new Set([
  'created',
  'paused',
  'stopped',
  'exited',
]);

const normalizeDockerToken = (value: unknown): string =>
  typeof value === 'string' ? value.trim().toLowerCase().replace(/[\s_-]/g, '') : '';

const dockerDisplayName = (resource: Resource): string =>
  asTrimmedString(resource.displayName) ||
  asTrimmedString(resource.name) ||
  asTrimmedString(resource.docker?.serviceName) ||
  asTrimmedString(resource.docker?.nodeName) ||
  resource.id;

export function mapDockerContainerStatus(resource: Resource): StatusIndicator {
  const state = normalizeDockerToken(resource.docker?.containerState || resource.status);
  const health = normalizeDockerToken(resource.docker?.health);
  const exitCode = resource.docker?.exitCode;

  if (FATAL_CONTAINER_STATES.has(state)) {
    return { variant: 'danger', label: state === 'oomkilled' ? 'OOMKilled' : titleCase(state) };
  }
  if (health === 'unhealthy') return { variant: 'danger', label: 'Unhealthy' };
  if (state === 'exited' && typeof exitCode === 'number' && exitCode !== 0) {
    return { variant: 'danger', label: `Exited (${exitCode})` };
  }
  if (state === 'restarting') return { variant: 'warning', label: 'Restarting' };
  if (state === 'running' && health === 'starting') {
    return { variant: 'warning', label: 'Starting' };
  }
  if (state === 'running') {
    return { variant: 'success', label: health === 'healthy' ? 'Healthy' : 'Running' };
  }
  if (STOPPED_CONTAINER_STATES.has(state)) {
    return { variant: 'muted', label: titleCase(state) };
  }
  if (!state) return { variant: 'muted', label: 'Unknown' };
  return { variant: 'muted', label: titleCase(state) };
}

export function mapDockerServiceStatus(resource: Resource): StatusIndicator {
  const desired = resource.docker?.desiredTasks ?? 0;
  const running = resource.docker?.runningTasks ?? 0;
  const updateState = normalizeDockerToken(resource.docker?.serviceUpdate?.state);
  if (updateState === 'paused' || updateState === 'rollbackstarted') {
    return { variant: 'warning', label: 'Rollback paused' };
  }
  if (desired <= 0) return { variant: 'muted', label: 'Scaled to 0' };
  if (running >= desired) return { variant: 'success', label: 'Healthy' };
  if (running <= 0) return { variant: 'danger', label: `0 / ${desired} running` };
  return { variant: 'warning', label: `${running} / ${desired} running` };
}

export function mapDockerTaskStatus(resource: Resource): StatusIndicator {
  const current = normalizeDockerToken(resource.docker?.currentState);
  const desired = normalizeDockerToken(resource.docker?.desiredState);
  if (current === 'failed' || current === 'rejected' || current === 'orphaned') {
    return { variant: 'danger', label: titleCase(current) };
  }
  if (current === 'running') return { variant: 'success', label: 'Running' };
  if (current === 'complete') return { variant: 'success', label: 'Complete' };
  if (current === 'shutdown' && desired === 'shutdown') {
    return { variant: 'muted', label: 'Shutdown' };
  }
  if (
    current === 'preparing' ||
    current === 'starting' ||
    current === 'pending' ||
    current === 'assigned' ||
    current === 'accepted' ||
    current === 'ready'
  ) {
    return { variant: 'warning', label: titleCase(current) };
  }
  if (!current) return { variant: 'muted', label: 'Unknown' };
  return { variant: 'muted', label: titleCase(current) };
}

export function mapDockerSwarmNodeStatus(resource: Resource): StatusIndicator {
  const availability = normalizeDockerToken(resource.docker?.availability);
  const reachability = normalizeDockerToken(resource.docker?.managerReachability);
  const role = normalizeDockerToken(resource.docker?.nodeRole);
  const status = normalizeDockerToken(resource.status);

  if (role === 'manager' && reachability === 'unreachable') {
    return { variant: 'danger', label: 'Manager unreachable' };
  }
  if (status === 'offline' || status === 'stopped' || status === 'failed') {
    return { variant: 'danger', label: 'Down' };
  }
  if (availability === 'drain') return { variant: 'warning', label: 'Drained' };
  if (availability === 'pause') return { variant: 'muted', label: 'Paused' };
  if (status === 'degraded' || status === 'warning') {
    return { variant: 'warning', label: 'Degraded' };
  }
  if (availability === 'active' || status === 'online' || status === 'running') {
    return { variant: 'success', label: resource.docker?.leader ? 'Leader' : 'Ready' };
  }
  return { variant: 'muted', label: 'Unknown' };
}

const titleCase = (value: string): string =>
  value ? value.charAt(0).toUpperCase() + value.slice(1) : '';

// Attention-first ordering: rows that need an operator's eye float to the
// top of the table. Tie-broken by display name for stable rendering.
const STATUS_VARIANT_RANK: Record<StatusIndicatorVariant, number> = {
  danger: 0,
  warning: 1,
  muted: 2,
  success: 3,
};

const compareByDockerStatus = (
  mapper: (resource: Resource) => StatusIndicator,
): ((left: Resource, right: Resource) => number) => {
  return (left, right) => {
    const rankDelta =
      STATUS_VARIANT_RANK[mapper(left).variant] - STATUS_VARIANT_RANK[mapper(right).variant];
    if (rankDelta !== 0) return rankDelta;
    return dockerDisplayName(left).localeCompare(dockerDisplayName(right));
  };
};

export const compareDockerContainers = compareByDockerStatus(mapDockerContainerStatus);
export const compareDockerServices = compareByDockerStatus(mapDockerServiceStatus);
export const compareDockerTasks = compareByDockerStatus(mapDockerTaskStatus);
export const compareDockerSwarmNodes = compareByDockerStatus(mapDockerSwarmNodeStatus);

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
  const containers = dockerResources
    .filter((resource) => DOCKER_CONTAINER_TYPES.has(resource.type) && isDockerPlatform(resource))
    .sort(compareDockerContainers);
  const services = dockerResources
    .filter((resource) => DOCKER_SERVICE_TYPES.has(resource.type))
    .sort(compareDockerServices);
  const images = dockerResources.filter((resource) => DOCKER_IMAGE_TYPES.has(resource.type));
  const volumes = dockerResources.filter((resource) => DOCKER_VOLUME_TYPES.has(resource.type));
  const networks = dockerResources.filter((resource) => DOCKER_NETWORK_TYPES.has(resource.type));
  const nodes = dockerResources
    .filter((resource) => DOCKER_SWARM_NODE_TYPES.has(resource.type))
    .sort(compareDockerSwarmNodes);
  const tasks = dockerResources
    .filter((resource) => DOCKER_TASK_TYPES.has(resource.type))
    .sort(compareDockerTasks);
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
