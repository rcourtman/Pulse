import type { Resource, ResourceType } from '@/types/resource';
import { resolveResourcePlatformType } from '@/utils/sourcePlatforms';

export type KubernetesPageTabId =
  | 'overview'
  | 'workloads'
  | 'services'
  | 'storage'
  | 'networking'
  | 'config'
  | 'events';

export type KubernetesTabSpec = {
  id: KubernetesPageTabId;
  label: string;
  path: string;
};

export const KUBERNETES_TAB_SPECS: readonly KubernetesTabSpec[] = [
  { id: 'overview', label: 'Overview', path: '/kubernetes/overview' },
  { id: 'workloads', label: 'Workloads', path: '/kubernetes/workloads' },
  { id: 'services', label: 'Services', path: '/kubernetes/services' },
  { id: 'storage', label: 'Storage', path: '/kubernetes/storage' },
  { id: 'networking', label: 'Networking', path: '/kubernetes/networking' },
  { id: 'config', label: 'Config', path: '/kubernetes/config' },
  { id: 'events', label: 'Events', path: '/kubernetes/events' },
] as const;

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
  'k8s-serviceaccount',
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
  serviceAccounts: Resource[];
  events: Resource[];
  workloads: Resource[];
  storage: Resource[];
  networking: Resource[];
  config: Resource[];
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
  const serviceAccounts = k8sResources.filter(
    (resource) => resource.type === 'k8s-serviceaccount',
  );
  const events = k8sResources.filter((resource) => resource.type === 'k8s-event');
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
  const networking = [...services, ...ingresses, ...endpointSlices, ...networkPolicies];
  const config = [...namespaces, ...configMaps, ...serviceAccounts];

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
    serviceAccounts,
    events,
    workloads,
    storage,
    networking,
    config,
  };
}
