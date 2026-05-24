import type {
  Resource,
  ResourceKubernetesPodContainerStatus,
  ResourceType,
} from '@/types/resource';
import type { StatusIndicator, StatusIndicatorVariant } from '@/utils/status';
import { resolveResourcePlatformType } from '@/utils/sourcePlatforms';

export type KubernetesPageTabId =
  | 'overview'
  | 'nodes'
  | 'workloads'
  | 'services'
  | 'storage'
  | 'configuration'
  | 'events';

export type KubernetesTabSpec = {
  id: KubernetesPageTabId;
  label: string;
  path: string;
};

// Keep Kubernetes tabs at the operator-workflow level. The API exposes many
// object kinds, but the page should not become one tab per kind or repeat a
// detailed table in Overview when that table already has a dedicated workflow
// home.
export const KUBERNETES_TAB_SPECS: readonly KubernetesTabSpec[] = [
  { id: 'overview', label: 'Overview', path: '/kubernetes/overview' },
  { id: 'nodes', label: 'Nodes', path: '/kubernetes/nodes' },
  { id: 'workloads', label: 'Workloads', path: '/kubernetes/workloads' },
  { id: 'services', label: 'Services', path: '/kubernetes/services' },
  { id: 'storage', label: 'Storage', path: '/kubernetes/storage' },
  { id: 'configuration', label: 'Configuration', path: '/kubernetes/configuration' },
  { id: 'events', label: 'Events', path: '/kubernetes/events' },
] as const;

const asTrimmedString = (value: unknown): string => (typeof value === 'string' ? value.trim() : '');

const eventTypeLabel = (value: string, fallback: string): string => {
  const trimmed = asTrimmedString(value);
  if (!trimmed) return fallback;
  return trimmed.charAt(0).toUpperCase() + trimmed.slice(1);
};

export function mapKubernetesEventSeverity(eventType: string | undefined): StatusIndicator {
  const normalized = asTrimmedString(eventType).toLowerCase();
  if (normalized === 'warning') return { variant: 'warning', label: 'Warning' };
  if (normalized === 'normal') return { variant: 'muted', label: 'Normal' };
  return {
    variant: 'muted',
    label: eventTypeLabel(eventType ?? '', 'Unknown'),
  };
}

const parseKubernetesEventObservedTime = (resource: Resource): number => {
  const observed =
    asTrimmedString(resource.kubernetes?.eventTime) ||
    asTrimmedString(resource.kubernetes?.firstSeen) ||
    asTrimmedString(resource.kubernetes?.createdAt);
  if (!observed) return 0;
  const timestamp = Date.parse(observed);
  return Number.isFinite(timestamp) ? timestamp : 0;
};

export const compareKubernetesEvents = (left: Resource, right: Resource): number => {
  const timeDelta = parseKubernetesEventObservedTime(right) - parseKubernetesEventObservedTime(left);
  if (timeDelta !== 0) return timeDelta;
  return left.id.localeCompare(right.id);
};

// Container-state reasons that mean "the kubelet can't get this container
// to a running state". Distinct from a transient `Pending` phase: these
// are the reasons that should escalate the pod row to a danger dot regardless
// of the surrounding phase.
const POD_CONTAINER_FATAL_REASONS = new Set([
  'crashloopbackoff',
  'imagepullbackoff',
  'errimagepull',
  'createcontainerconfigerror',
  'createcontainererror',
  'invalidimagename',
  'runcontainererror',
  'oomkilled',
]);

const normalizeKubernetesToken = (value: unknown): string =>
  typeof value === 'string' ? value.trim().toLowerCase().replace(/[\s_-]/g, '') : '';

const displayName = (resource: Resource): string =>
  asTrimmedString(resource.displayName) ||
  asTrimmedString(resource.name) ||
  asTrimmedString(resource.kubernetes?.podName) ||
  resource.id;

const containerHasFatalReason = (container: ResourceKubernetesPodContainerStatus): boolean => {
  const reason = normalizeKubernetesToken(container.reason);
  if (reason && POD_CONTAINER_FATAL_REASONS.has(reason)) return true;
  const state = normalizeKubernetesToken(container.state);
  return state === 'terminated' && !container.ready;
};

const podHasFatalContainer = (containers: ResourceKubernetesPodContainerStatus[]): boolean =>
  containers.some(containerHasFatalReason);

const podAllContainersReady = (containers: ResourceKubernetesPodContainerStatus[]): boolean =>
  containers.length > 0 && containers.every((container) => container.ready === true);

export function mapKubernetesPodStatus(resource: Resource): StatusIndicator {
  const phase = normalizeKubernetesToken(resource.kubernetes?.podPhase || resource.kubernetes?.phase);
  const containers = resource.kubernetes?.podContainers ?? [];

  if (phase === 'failed') return { variant: 'danger', label: 'Failed' };
  if (podHasFatalContainer(containers)) {
    const reason =
      containers.find(containerHasFatalReason)?.reason?.trim() || 'Container error';
    return { variant: 'danger', label: reason };
  }
  if (phase === 'pending') return { variant: 'warning', label: 'Pending' };
  if (phase === 'running') {
    if (containers.length === 0) return { variant: 'success', label: 'Running' };
    if (podAllContainersReady(containers)) return { variant: 'success', label: 'Running' };
    return { variant: 'warning', label: 'Not ready' };
  }
  if (phase === 'succeeded') return { variant: 'success', label: 'Succeeded' };
  if (phase === 'unknown') return { variant: 'muted', label: 'Unknown' };
  if (!phase) return { variant: 'muted', label: 'Unknown' };
  return { variant: 'muted', label: eventTypeLabel(resource.kubernetes?.podPhase ?? '', 'Unknown') };
}

export function mapKubernetesNodeStatus(resource: Resource): StatusIndicator {
  const ready = resource.kubernetes?.ready;
  if (ready === false) return { variant: 'danger', label: 'NotReady' };
  if (ready === true) return { variant: 'success', label: 'Ready' };

  const status = normalizeKubernetesToken(resource.status);
  if (status === 'online' || status === 'running' || status === 'healthy') {
    return { variant: 'success', label: 'Ready' };
  }
  if (status === 'offline' || status === 'stopped' || status === 'failed') {
    return { variant: 'danger', label: 'NotReady' };
  }
  if (status === 'degraded' || status === 'warning' || status === 'pending') {
    return { variant: 'warning', label: 'Degraded' };
  }
  return { variant: 'muted', label: 'Unknown' };
}

const replicaIndicator = (
  desired: number | undefined,
  ready: number | undefined,
  readyLabel = 'Ready',
): StatusIndicator => {
  const desiredCount = typeof desired === 'number' ? desired : 0;
  const readyCount = typeof ready === 'number' ? ready : 0;
  if (desiredCount <= 0) return { variant: 'muted', label: 'Scaled to 0' };
  if (readyCount >= desiredCount) return { variant: 'success', label: readyLabel };
  if (readyCount <= 0) return { variant: 'danger', label: `0 / ${desiredCount} ready` };
  return { variant: 'warning', label: `${readyCount} / ${desiredCount} ready` };
};

export function mapKubernetesDeploymentStatus(resource: Resource): StatusIndicator {
  return replicaIndicator(
    resource.kubernetes?.desiredReplicas,
    resource.kubernetes?.readyReplicas,
  );
}

export function mapKubernetesReplicaSetStatus(resource: Resource): StatusIndicator {
  return replicaIndicator(
    resource.kubernetes?.desiredReplicas,
    resource.kubernetes?.readyReplicas,
  );
}

export function mapKubernetesStatefulSetStatus(resource: Resource): StatusIndicator {
  return replicaIndicator(
    resource.kubernetes?.desiredReplicas,
    resource.kubernetes?.readyReplicas,
  );
}

export function mapKubernetesDaemonSetStatus(resource: Resource): StatusIndicator {
  const desired = resource.kubernetes?.desiredNumberScheduled;
  const ready = resource.kubernetes?.numberReady;
  const misscheduled = resource.kubernetes?.numberMisscheduled ?? 0;
  const base = replicaIndicator(desired, ready, 'Scheduled');
  if (base.variant === 'success' && misscheduled > 0) {
    return { variant: 'warning', label: `${misscheduled} misscheduled` };
  }
  return base;
}

export function mapKubernetesJobStatus(resource: Resource): StatusIndicator {
  const failed = resource.kubernetes?.failed ?? 0;
  const succeeded = resource.kubernetes?.succeeded ?? 0;
  const active = resource.kubernetes?.active ?? 0;
  if (failed > 0) return { variant: 'danger', label: `${failed} failed` };
  if (active > 0) return { variant: 'warning', label: `${active} active` };
  if (succeeded > 0) return { variant: 'success', label: 'Succeeded' };
  return { variant: 'muted', label: 'Idle' };
}

export function mapKubernetesCronJobStatus(resource: Resource): StatusIndicator {
  if (resource.kubernetes?.suspend === true) return { variant: 'muted', label: 'Suspended' };
  return { variant: 'success', label: 'Scheduled' };
}

export function mapKubernetesControllerStatus(resource: Resource): StatusIndicator {
  switch (resource.type) {
    case 'k8s-replicaset':
      return mapKubernetesReplicaSetStatus(resource);
    case 'k8s-statefulset':
      return mapKubernetesStatefulSetStatus(resource);
    case 'k8s-daemonset':
      return mapKubernetesDaemonSetStatus(resource);
    case 'k8s-job':
      return mapKubernetesJobStatus(resource);
    case 'k8s-cronjob':
      return mapKubernetesCronJobStatus(resource);
    default:
      return { variant: 'muted', label: 'Unknown' };
  }
}

// Attention-first ordering: rows that need an operator's eye float to the
// top of the table. Tie-broken by display name for stable rendering.
const STATUS_VARIANT_RANK: Record<StatusIndicatorVariant, number> = {
  danger: 0,
  warning: 1,
  muted: 2,
  success: 3,
};

const compareByStatus = (
  mapper: (resource: Resource) => StatusIndicator,
): ((left: Resource, right: Resource) => number) => {
  return (left, right) => {
    const rankDelta = STATUS_VARIANT_RANK[mapper(left).variant] - STATUS_VARIANT_RANK[mapper(right).variant];
    if (rankDelta !== 0) return rankDelta;
    return displayName(left).localeCompare(displayName(right));
  };
};

export const compareKubernetesPods = compareByStatus(mapKubernetesPodStatus);
export const compareKubernetesNodes = compareByStatus(mapKubernetesNodeStatus);
export const compareKubernetesDeployments = compareByStatus(mapKubernetesDeploymentStatus);
export const compareKubernetesControllers = compareByStatus(mapKubernetesControllerStatus);

const KUBERNETES_ROUTE_TAB_ALIASES: Record<string, KubernetesPageTabId> = {
  autoscaling: 'workloads',
  config: 'configuration',
  controllers: 'workloads',
  deployments: 'workloads',
  networking: 'services',
  pods: 'workloads',
  policy: 'configuration',
};

export const resolveKubernetesPageTabId = (segment: string | undefined): KubernetesPageTabId => {
  const normalized = asTrimmedString(segment).toLowerCase();
  if (!normalized) return 'overview';
  const direct = KUBERNETES_TAB_SPECS.find((tab) => tab.id === normalized);
  if (direct) return direct.id;
  return KUBERNETES_ROUTE_TAB_ALIASES[normalized] ?? 'overview';
};

const KUBERNETES_RESOURCE_TYPES = new Set<ResourceType>([
  'k8s-cluster',
  'k8s-node',
  'pod',
  'k8s-deployment',
  'k8s-replicaset',
  'k8s-namespace',
  'k8s-service',
  'k8s-statefulset',
  'k8s-daemonset',
  'k8s-job',
  'k8s-cronjob',
  'k8s-ingress',
  'k8s-endpoint-slice',
  'k8s-network-policy',
  'k8s-persistent-volume',
  'k8s-persistent-volume-claim',
  'k8s-storage-class',
  'k8s-configmap',
  'k8s-secret',
  'k8s-serviceaccount',
  'k8s-resource-quota',
  'k8s-limit-range',
  'k8s-pod-disruption-budget',
  'k8s-horizontal-pod-autoscaler',
  'k8s-event',
]);

const isKubernetesPlatform = (resource: Resource): boolean => {
  if (resolveResourcePlatformType(resource) === 'kubernetes') return true;
  return KUBERNETES_RESOURCE_TYPES.has(resource.type);
};

// Kubernetes nodes registered as Pulse Agents are merged onto the linked
// agent row by the backend registry, so the "nodes" bucket must include
// both the standalone k8s-node projections and the agent rows that report a
// kubernetes source.
const isKubernetesNodeRow = (resource: Resource): boolean => {
  if (resource.type === 'k8s-node') return true;
  if (resource.type !== 'agent') return false;
  return resolveResourcePlatformType(resource) === 'kubernetes';
};

export type KubernetesPageModel = {
  resources: Resource[];
  clusters: Resource[];
  nodes: Resource[];
  pods: Resource[];
  deployments: Resource[];
  replicaSets: Resource[];
  namespaces: Resource[];
  services: Resource[];
  statefulSets: Resource[];
  daemonSets: Resource[];
  jobs: Resource[];
  cronJobs: Resource[];
  ingresses: Resource[];
  endpointSlices: Resource[];
  networkPolicies: Resource[];
  persistentVolumes: Resource[];
  persistentVolumeClaims: Resource[];
  storageClasses: Resource[];
  configMaps: Resource[];
  secrets: Resource[];
  serviceAccounts: Resource[];
  resourceQuotas: Resource[];
  limitRanges: Resource[];
  podDisruptionBudgets: Resource[];
  horizontalPodAutoscalers: Resource[];
  events: Resource[];
  workloads: Resource[];
  storage: Resource[];
  serviceNetworking: Resource[];
  config: Resource[];
  policy: Resource[];
  autoscaling: Resource[];
};

export type KubernetesClusterChildCounts = {
  nodes: number;
  pods: number;
  deployments: number;
};

const emptyKubernetesClusterChildCounts = (): KubernetesClusterChildCounts => ({
  nodes: 0,
  pods: 0,
  deployments: 0,
});

const matchClusterFor = (
  resource: Resource,
  clusters: readonly Resource[],
): Resource | undefined => {
  const clusterId =
    asTrimmedString(resource.kubernetes?.clusterId) ||
    asTrimmedString(resource.kubernetes?.clusterName);
  if (!clusterId) return undefined;
  for (const cluster of clusters) {
    const k = cluster.kubernetes;
    if (!k) continue;
    if (
      asTrimmedString(k.clusterId) === clusterId ||
      asTrimmedString(k.clusterName) === clusterId
    ) {
      return cluster;
    }
  }
  return undefined;
};

// Walks the resource snapshot once and rolls per-cluster Nodes / Pods /
// Deployments into a Map keyed by cluster row id. The Overview clusters
// table calls this instead of computing counts inline, so the same
// rollup is testable and reusable by other consumers (incident rollups,
// future Overview "active issues" cards).
export function buildKubernetesClusterChildCounts(
  resources: readonly Resource[],
  clusters: readonly Resource[],
): Map<string, KubernetesClusterChildCounts> {
  const counts = new Map<string, KubernetesClusterChildCounts>();
  for (const cluster of clusters) {
    counts.set(cluster.id, emptyKubernetesClusterChildCounts());
  }
  if (clusters.length === 0) return counts;

  for (const resource of resources) {
    const cluster = matchClusterFor(resource, clusters);
    if (!cluster) continue;
    const bucket = counts.get(cluster.id);
    if (!bucket) continue;
    if (resource.type === 'k8s-node') {
      bucket.nodes += 1;
    } else if (resource.type === 'agent' && resource.sources?.includes('kubernetes')) {
      bucket.nodes += 1;
    } else if (resource.type === 'pod') {
      bucket.pods += 1;
    } else if (resource.type === 'k8s-deployment') {
      bucket.deployments += 1;
    }
  }
  return counts;
}

export function buildKubernetesPageModel(resources: Resource[]): KubernetesPageModel {
  const k8sResources = resources.filter(isKubernetesPlatform);
  const clusters = k8sResources.filter((resource) => resource.type === 'k8s-cluster');
  const nodes = k8sResources.filter(isKubernetesNodeRow).sort(compareKubernetesNodes);
  const pods = k8sResources
    .filter((resource) => resource.type === 'pod')
    .sort(compareKubernetesPods);
  const deployments = k8sResources
    .filter((resource) => resource.type === 'k8s-deployment')
    .sort(compareKubernetesDeployments);
  const replicaSets = k8sResources.filter((resource) => resource.type === 'k8s-replicaset');
  const namespaces = k8sResources.filter((resource) => resource.type === 'k8s-namespace');
  const services = k8sResources.filter((resource) => resource.type === 'k8s-service');
  const statefulSets = k8sResources.filter((resource) => resource.type === 'k8s-statefulset');
  const daemonSets = k8sResources.filter((resource) => resource.type === 'k8s-daemonset');
  const jobs = k8sResources.filter((resource) => resource.type === 'k8s-job');
  const cronJobs = k8sResources.filter((resource) => resource.type === 'k8s-cronjob');
  const ingresses = k8sResources.filter((resource) => resource.type === 'k8s-ingress');
  const endpointSlices = k8sResources.filter((resource) => resource.type === 'k8s-endpoint-slice');
  const networkPolicies = k8sResources.filter((resource) => resource.type === 'k8s-network-policy');
  const persistentVolumes = k8sResources.filter(
    (resource) => resource.type === 'k8s-persistent-volume',
  );
  const persistentVolumeClaims = k8sResources.filter(
    (resource) => resource.type === 'k8s-persistent-volume-claim',
  );
  const storageClasses = k8sResources.filter((resource) => resource.type === 'k8s-storage-class');
  const configMaps = k8sResources.filter((resource) => resource.type === 'k8s-configmap');
  const secrets = k8sResources.filter((resource) => resource.type === 'k8s-secret');
  const serviceAccounts = k8sResources.filter((resource) => resource.type === 'k8s-serviceaccount');
  const resourceQuotas = k8sResources.filter((resource) => resource.type === 'k8s-resource-quota');
  const limitRanges = k8sResources.filter((resource) => resource.type === 'k8s-limit-range');
  const podDisruptionBudgets = k8sResources.filter(
    (resource) => resource.type === 'k8s-pod-disruption-budget',
  );
  const horizontalPodAutoscalers = k8sResources.filter(
    (resource) => resource.type === 'k8s-horizontal-pod-autoscaler',
  );
  const events = k8sResources
    .filter((resource) => resource.type === 'k8s-event')
    .sort(compareKubernetesEvents);
  const sortedControllers = [
    ...replicaSets,
    ...statefulSets,
    ...daemonSets,
    ...jobs,
    ...cronJobs,
  ].sort(compareKubernetesControllers);
  const workloads = [...deployments, ...sortedControllers, ...pods];
  const storage = [...storageClasses, ...persistentVolumes, ...persistentVolumeClaims];
  const serviceNetworking = [...ingresses, ...endpointSlices];
  const config = [...namespaces, ...configMaps, ...secrets, ...serviceAccounts];
  const policy = [...networkPolicies, ...podDisruptionBudgets, ...resourceQuotas, ...limitRanges];
  const autoscaling = [...horizontalPodAutoscalers];

  return {
    resources: k8sResources,
    clusters,
    nodes,
    pods,
    deployments,
    replicaSets,
    namespaces,
    services,
    statefulSets,
    daemonSets,
    jobs,
    cronJobs,
    ingresses,
    endpointSlices,
    networkPolicies,
    persistentVolumes,
    persistentVolumeClaims,
    storageClasses,
    configMaps,
    secrets,
    serviceAccounts,
    resourceQuotas,
    limitRanges,
    podDisruptionBudgets,
    horizontalPodAutoscalers,
    events,
    workloads,
    storage,
    serviceNetworking,
    config,
    policy,
    autoscaling,
  };
}
