import type { Resource, ResourceType } from '@/types/resource';
import type { StatusIndicator } from '@/utils/status';
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

export function buildKubernetesPageModel(resources: Resource[]): KubernetesPageModel {
  const k8sResources = resources.filter(isKubernetesPlatform);
  const clusters = k8sResources.filter((resource) => resource.type === 'k8s-cluster');
  const nodes = k8sResources.filter(isKubernetesNodeRow);
  const pods = k8sResources.filter((resource) => resource.type === 'pod');
  const deployments = k8sResources.filter((resource) => resource.type === 'k8s-deployment');
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
  const workloads = [
    ...deployments,
    ...replicaSets,
    ...statefulSets,
    ...daemonSets,
    ...jobs,
    ...cronJobs,
    ...pods,
  ];
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
