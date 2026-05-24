export type CanonicalFrontendResourceType =
  | 'agent'
  | 'ceph'
  | 'vm'
  | 'system-container'
  | 'app-container'
  | 'oci-container'
  | 'pod'
  | 'storage'
  | 'disk'
  | 'docker-host'
  | 'docker-image'
  | 'docker-volume'
  | 'docker-network'
  | 'docker-task'
  | 'network'
  | 'network-share'
  | 'pbs'
  | 'pmg'
  | 'k8s-node'
  | 'k8s-cluster'
  | 'k8s-deployment'
  | 'k8s-service'
  | 'k8s-namespace'
  | 'k8s-statefulset'
  | 'k8s-daemonset'
  | 'k8s-job'
  | 'k8s-cronjob'
  | 'k8s-ingress'
  | 'k8s-persistent-volume'
  | 'k8s-persistent-volume-claim'
  | 'k8s-event'
  | 'network-endpoint';

const asNormalizedString = (value: unknown): string | undefined => {
  if (typeof value !== 'string') return undefined;
  const trimmed = value.trim().toLowerCase();
  return trimmed.length > 0 ? trimmed : undefined;
};

export const canonicalizeFrontendResourceType = (
  value: unknown,
): CanonicalFrontendResourceType | undefined => {
  const normalized = asNormalizedString(value);
  if (!normalized) return undefined;

  switch (normalized) {
    case 'host':
    case 'hosts':
    case 'truenas':
      return 'agent';
    case 'availability':
    case 'endpoint':
    case 'network_endpoint':
      return 'network-endpoint';
    case 'docker':
      return 'app-container';
    case 'dockerhost':
    case 'docker_host':
      return 'docker-host';
    case 'k8s':
    case 'kubernetes':
    case 'k8s-pod':
      return 'pod';
    case 'kubernetes-cluster':
    case 'kubernetes_cluster':
      return 'k8s-cluster';
    case 'kubernetes-node':
    case 'kubernetes_node':
      return 'k8s-node';
    case 'agent':
    case 'ceph':
    case 'vm':
    case 'system-container':
    case 'app-container':
    case 'oci-container':
    case 'pod':
      return normalized;
    case 'node':
      return 'agent';
    case 'storage':
    case 'disk':
    case 'docker-host':
    case 'docker-image':
    case 'docker-volume':
    case 'docker-network':
    case 'docker-task':
    case 'network':
    case 'network-share':
    case 'pbs':
    case 'pmg':
    case 'k8s-node':
    case 'k8s-cluster':
    case 'k8s-deployment':
    case 'k8s-service':
    case 'k8s-namespace':
    case 'k8s-statefulset':
    case 'k8s-daemonset':
    case 'k8s-job':
    case 'k8s-cronjob':
    case 'k8s-ingress':
    case 'k8s-persistent-volume':
    case 'k8s-persistent-volume-claim':
    case 'k8s-event':
    case 'network-endpoint':
      return normalized;
    default:
      return undefined;
  }
};
