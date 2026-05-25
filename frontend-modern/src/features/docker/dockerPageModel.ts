import { resolveResourcePlatformType } from '@/utils/sourcePlatforms';
import type {
  DockerStorageUsageMeta,
  Resource,
  ResourceIncident,
  ResourceType,
} from '@/types/resource';
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

export type DockerResourceStatusFilter = 'all' | 'online' | 'degraded' | 'offline';

const ONLINE_STATUSES = new Set<string>(['online', 'running']);
const DEGRADED_STATUSES = new Set<string>(['degraded', 'paused']);
const OFFLINE_STATUSES = new Set<string>(['offline', 'stopped']);

const mapResourceStatusToTriad = (
  status: string | undefined,
): Exclude<DockerResourceStatusFilter, 'all'> | 'unknown' => {
  if (!status) return 'unknown';
  if (ONLINE_STATUSES.has(status)) return 'online';
  if (DEGRADED_STATUSES.has(status)) return 'degraded';
  if (OFFLINE_STATUSES.has(status)) return 'offline';
  return 'unknown';
};

const numberToken = (value: number | undefined): string | undefined =>
  typeof value === 'number' ? String(value) : undefined;

const dockerPortToken = (
  port: NonNullable<NonNullable<Resource['docker']>['ports']>[number],
): string => {
  const protocol = asTrimmedString(port.protocol)?.toLowerCase() || 'tcp';
  const privatePort = numberToken(port.privatePort);
  const publicPort = numberToken(port.publicPort);
  const ip = asTrimmedString(port.ip);
  if (privatePort && publicPort) {
    return `${ip ? `${ip}:` : ''}${publicPort}->${privatePort}/${protocol}`;
  }
  if (privatePort) return `${privatePort}/${protocol}`;
  if (publicPort) return `${ip ? `${ip}:` : ''}${publicPort}/${protocol}`;
  return protocol;
};

// Builds the lowercase search haystack a Docker page table consults when
// filtering rows. The shared platformPage helper carries only generic Resource
// fields; docker.* lookups live here so the cross-platform helper does not
// have to know which platforms exist.
export function dockerResourceSearchHaystack(resource: Resource): string {
  const docker = resource.docker;
  return [
    resource.id,
    resource.name,
    resource.displayName,
    resource.parentName,
    resource.platformId,
    resource.platformType,
    resource.agent?.hostname,
    resource.identity?.hostname,
    resource.canonicalIdentity?.displayName,
    resource.canonicalIdentity?.hostname,
    resource.canonicalIdentity?.primaryId,
    ...(resource.canonicalIdentity?.aliases ?? []),
    docker?.hostname,
    docker?.displayName,
    docker?.runtime,
    docker?.runtimeVersion,
    docker?.dockerVersion,
    docker?.containerId,
    docker?.image,
    docker?.imageId,
    docker?.containerState,
    docker?.health,
    numberToken(docker?.restartCount),
    numberToken(docker?.exitCode),
    docker?.updateStatus?.error,
    docker?.updateStatus?.currentDigest,
    docker?.updateStatus?.latestDigest,
    docker?.volumeName,
    docker?.networkId,
    docker?.driver,
    docker?.mountpoint,
    docker?.serviceName,
    docker?.taskId,
    docker?.nodeId,
    docker?.nodeName,
    docker?.nodeRole,
    docker?.availability,
    docker?.address,
    docker?.managerReachability,
    docker?.managerAddress,
    docker?.engineVersion,
    docker?.secretName,
    docker?.configName,
    docker?.mode,
    docker?.currentState,
    docker?.desiredState,
    docker?.message,
    docker?.error,
    docker?.swarm?.clusterId,
    docker?.swarm?.clusterName,
    docker?.swarm?.nodeRole,
    docker?.swarm?.localState,
    ...(docker?.ports?.flatMap((port) => [
      dockerPortToken(port),
      port.ip,
      port.protocol,
      numberToken(port.privatePort),
      numberToken(port.publicPort),
    ]) ?? []),
    ...(docker?.networks?.flatMap((network) => [network.name, network.ipv4, network.ipv6]) ?? []),
    ...(docker?.mounts?.flatMap((mount) => [
      mount.type,
      mount.source,
      mount.destination,
      mount.mode,
      mount.rw === false ? 'read-only' : mount.rw === true ? 'read-write' : undefined,
    ]) ?? []),
    ...(docker?.repoTags ?? []),
    ...(docker?.repoDigests ?? []),
    ...(resource.tags ?? []),
  ]
    .filter((value): value is string => typeof value === 'string' && value.trim().length > 0)
    .join(' ')
    .toLowerCase();
}

export function filterDockerResources(
  resources: Resource[],
  search: string,
  status: DockerResourceStatusFilter,
): Resource[] {
  const needle = search.trim().toLowerCase();
  const result: Resource[] = [];
  for (const resource of resources) {
    if (status !== 'all') {
      const triad = mapResourceStatusToTriad(resource.status);
      if (triad !== status) continue;
    }
    if (!needle) {
      result.push(resource);
      continue;
    }
    if (dockerResourceSearchHaystack(resource).includes(needle)) {
      result.push(resource);
    }
  }
  return result;
}

export type DockerIncidentSeverityFilter = 'all' | 'critical' | 'warning' | 'info';

export type DockerIncidentRow = {
  id: string;
  resource: Resource;
  resourceId: string;
  resourceName: string;
  resourceType: ResourceType;
  severity: string;
  severityBucket: Exclude<DockerIncidentSeverityFilter, 'all'>;
  code: string;
  source: string;
  summary: string;
  label: string;
  category: string;
  startedAt?: string;
  action: string;
  priority: number;
};

export function mapDockerIncidentSeverity(
  severity: string | undefined,
): Exclude<DockerIncidentSeverityFilter, 'all'> {
  const normalized = asTrimmedString(severity).toLowerCase();
  if (['critical', 'crit', 'fatal', 'error', 'failed', 'failure'].includes(normalized)) {
    return 'critical';
  }
  if (['warning', 'warn', 'alert', 'degraded'].includes(normalized)) return 'warning';
  return 'info';
}

const incidentSeverityRank = (severity: string): number => {
  switch (mapDockerIncidentSeverity(severity)) {
    case 'critical':
      return 3;
    case 'warning':
      return 2;
    case 'info':
      return 1;
  }
};

const titleCaseIncidentCode = (value: string): string =>
  value
    .split(/[\s_-]+/)
    .filter(Boolean)
    .map((part) => part.charAt(0).toUpperCase() + part.slice(1).toLowerCase())
    .join(' ');

const dockerIncidentLabel = (resource: Resource, incident: ResourceIncident): string => {
  const label = asTrimmedString(resource.incidentLabel);
  if (label) return label;
  const code = asTrimmedString(incident.code);
  return code ? titleCaseIncidentCode(code.replace(/^docker_/, '')) : 'Docker Alert';
};

const dockerIncidentResourceDisplayName = (resource: Resource): string =>
  asTrimmedString(resource.displayName) ||
  asTrimmedString(resource.name) ||
  asTrimmedString(resource.docker?.serviceName) ||
  asTrimmedString(resource.docker?.hostname) ||
  resource.id;

const hasIncidentSignal = (incident: ResourceIncident): boolean =>
  Boolean(asTrimmedString(incident.code) || asTrimmedString(incident.summary));

const hasIncidentRollup = (resource: Resource): boolean =>
  (resource.incidentCount ?? 0) > 0 ||
  Boolean(
    asTrimmedString(resource.incidentCode) ||
      asTrimmedString(resource.incidentSummary) ||
      asTrimmedString(resource.incidentLabel),
  );

const buildDockerIncidentRow = (
  resource: Resource,
  incident: ResourceIncident,
  index: number,
): DockerIncidentRow => {
  const severity =
    asTrimmedString(incident.severity) || asTrimmedString(resource.incidentSeverity) || 'info';
  const code =
    asTrimmedString(incident.code) || asTrimmedString(resource.incidentCode) || 'docker_alert';
  const summary =
    asTrimmedString(incident.summary) ||
    asTrimmedString(resource.incidentSummary) ||
    dockerIncidentLabel(resource, incident);
  const nativeId = asTrimmedString(incident.nativeId);
  const rowKey = nativeId || code || String(index);
  return {
    id: `${resource.id}:incident:${rowKey}:${index}`,
    resource,
    resourceId: resource.id,
    resourceName: dockerIncidentResourceDisplayName(resource),
    resourceType: resource.type,
    severity,
    severityBucket: mapDockerIncidentSeverity(severity),
    code,
    source: asTrimmedString(incident.source) || asTrimmedString(incident.provider) || 'docker',
    summary,
    label: dockerIncidentLabel(resource, incident),
    category: asTrimmedString(resource.incidentCategory) || 'docker-health',
    startedAt: incident.startedAt,
    action: asTrimmedString(resource.incidentAction) || 'Investigate in Pulse alerts',
    priority: resource.incidentPriority ?? incidentSeverityRank(severity) * 1000,
  };
};

const buildDockerRollupIncidentRow = (resource: Resource): DockerIncidentRow => {
  const severity = asTrimmedString(resource.incidentSeverity) || 'info';
  const code = asTrimmedString(resource.incidentCode) || 'docker_alert';
  const count = resource.incidentCount ?? 0;
  const summary =
    asTrimmedString(resource.incidentSummary) ||
    asTrimmedString(resource.incidentLabel) ||
    `${count || 1} active Docker alert${count === 1 ? '' : 's'}`;
  const incident: ResourceIncident = { code, severity, summary, source: 'docker' };
  return {
    ...buildDockerIncidentRow(resource, incident, 0),
    id: `${resource.id}:incident:rollup`,
  };
};

// Walks resource.incidents[] for each row; when a resource carries only
// rollup-level incident fields (incidentCount / incidentSeverity / etc.) but
// no per-incident list, emits a single synthesized row so the operator still
// sees the alert on the Overview. Mirrors buildTrueNASIncidentRows /
// buildVmwareIncidentRows.
export function buildDockerIncidentRows(resources: Resource[]): DockerIncidentRow[] {
  const rows: DockerIncidentRow[] = [];
  for (const resource of resources) {
    const incidents = (resource.incidents ?? []).filter(hasIncidentSignal);
    if (incidents.length > 0) {
      incidents.forEach((incident, index) =>
        rows.push(buildDockerIncidentRow(resource, incident, index)),
      );
      continue;
    }
    if (hasIncidentRollup(resource)) {
      rows.push(buildDockerRollupIncidentRow(resource));
    }
  }
  return rows.sort((a, b) => {
    const severityDelta = incidentSeverityRank(b.severity) - incidentSeverityRank(a.severity);
    if (severityDelta !== 0) return severityDelta;
    const priorityDelta = b.priority - a.priority;
    if (priorityDelta !== 0) return priorityDelta;
    return a.resourceName.localeCompare(b.resourceName);
  });
}

const dockerIncidentSearchHaystack = (row: DockerIncidentRow): string =>
  [
    row.resourceName,
    row.resourceId,
    row.resourceType,
    row.resource.parentName,
    row.resource.platformId,
    row.resource.docker?.hostname,
    row.resource.docker?.serviceName,
    row.resource.docker?.swarm?.clusterName,
    row.severity,
    row.code,
    row.source,
    row.summary,
    row.label,
    row.category,
    row.action,
    ...(row.resource.tags ?? []),
  ]
    .filter((value): value is string => typeof value === 'string' && value.trim().length > 0)
    .join(' ')
    .toLowerCase();

export function filterDockerIncidents(
  incidents: DockerIncidentRow[],
  search: string,
  severity: DockerIncidentSeverityFilter,
): DockerIncidentRow[] {
  const needle = search.trim().toLowerCase();
  return incidents.filter((incident) => {
    if (severity !== 'all' && incident.severityBucket !== severity) return false;
    if (!needle) return true;
    return dockerIncidentSearchHaystack(incident).includes(needle);
  });
}

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
  incidents: DockerIncidentRow[];
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
  const incidents = buildDockerIncidentRows(dockerResources);

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
    incidents,
  };
}
