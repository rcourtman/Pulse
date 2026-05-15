import { normalizeSourcePlatformQueryValue } from '@/utils/sourcePlatforms';
import type { Resource, ResourceType } from '@/types/resource';

export type KubernetesPageTabId =
  | 'overview'
  | 'nodes'
  | 'pods'
  | 'deployments'
  | 'services';

export type KubernetesTabSpec = {
  id: KubernetesPageTabId;
  label: string;
  path: string;
};

export const KUBERNETES_TAB_SPECS: readonly KubernetesTabSpec[] = [
  { id: 'overview', label: 'Clusters', path: '/kubernetes/overview' },
  { id: 'nodes', label: 'Nodes', path: '/kubernetes/nodes' },
  { id: 'pods', label: 'Pods', path: '/kubernetes/pods' },
  { id: 'deployments', label: 'Deployments', path: '/kubernetes/deployments' },
  { id: 'services', label: 'Services', path: '/kubernetes/services' },
] as const;

const KUBERNETES_RESOURCE_TYPES = new Set<ResourceType>([
  'k8s-cluster',
  'k8s-node',
  'pod',
  'k8s-deployment',
  'k8s-service',
]);

const isKubernetesPlatform = (resource: Resource): boolean => {
  if (normalizeSourcePlatformQueryValue(resource.platformType || '') === 'kubernetes') return true;
  return KUBERNETES_RESOURCE_TYPES.has(resource.type);
};

export type KubernetesPageModel = {
  resources: Resource[];
  clusters: Resource[];
  nodes: Resource[];
  pods: Resource[];
  deployments: Resource[];
  services: Resource[];
};

export function buildKubernetesPageModel(resources: Resource[]): KubernetesPageModel {
  const k8sResources = resources.filter(isKubernetesPlatform);
  const clusters = k8sResources.filter((resource) => resource.type === 'k8s-cluster');
  const nodes = k8sResources.filter((resource) => resource.type === 'k8s-node');
  const pods = k8sResources.filter((resource) => resource.type === 'pod');
  const deployments = k8sResources.filter((resource) => resource.type === 'k8s-deployment');
  const services = k8sResources.filter((resource) => resource.type === 'k8s-service');

  return {
    resources: k8sResources,
    clusters,
    nodes,
    pods,
    deployments,
    services,
  };
}
