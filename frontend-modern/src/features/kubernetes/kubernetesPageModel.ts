import { resolveResourcePlatformType } from '@/utils/sourcePlatforms';
import type { Resource, ResourceType } from '@/types/resource';

// The Overview tab stacks the K8s object graph from logical aggregate down
// to runtime detail: clusters → nodes → deployments → pods. Deployments
// (desired state) sit above pods (the actual runtime result) so a reader
// scanning for "did my rollout converge?" hits Ready/Available before
// scrolling into the per-pod list. The deployments table is gated on the
// cluster reporting any; when it's empty the pods section follows nodes
// directly. The standalone Nodes / Pods / Deployments tabs that used to
// live here were pure duplicates of the Overview stack, so they're
// intentionally absent — the Workloads filter inside Overview owns
// search/grouping for pods. Services are not surfaced by the canonical
// unified resource model today (no ResourceTypeK8sService projection on
// the backend), so a Services tab is similarly absent until that gap is
// closed in the canonical adapter.
export type KubernetesPageTabId = 'overview';

export type KubernetesTabSpec = {
  id: KubernetesPageTabId;
  label: string;
  path: string;
};

export const KUBERNETES_TAB_SPECS: readonly KubernetesTabSpec[] = [
  { id: 'overview', label: 'Overview', path: '/kubernetes/overview' },
] as const;

const KUBERNETES_RESOURCE_TYPES = new Set<ResourceType>([
  'k8s-cluster',
  'k8s-node',
  'pod',
  'k8s-deployment',
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
};

export function buildKubernetesPageModel(resources: Resource[]): KubernetesPageModel {
  const k8sResources = resources.filter(isKubernetesPlatform);
  const clusters = k8sResources.filter((resource) => resource.type === 'k8s-cluster');
  const nodes = k8sResources.filter(isKubernetesNodeRow);
  const pods = k8sResources.filter((resource) => resource.type === 'pod');
  const deployments = k8sResources.filter((resource) => resource.type === 'k8s-deployment');

  return {
    resources: k8sResources,
    clusters,
    nodes,
    pods,
    deployments,
  };
}
