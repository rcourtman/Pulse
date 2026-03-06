export type CanonicalFrontendResourceType =
  | 'agent'
  | 'vm'
  | 'system-container'
  | 'app-container'
  | 'oci-container'
  | 'pod'
  | 'storage'
  | 'disk'
  | 'docker-host'
  | 'pbs'
  | 'pmg'
  | 'k8s-node'
  | 'k8s-cluster'
  | 'k8s-deployment'
  | 'k8s-service';

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
      return 'agent';
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
    case 'pbs':
    case 'pmg':
    case 'k8s-node':
    case 'k8s-cluster':
    case 'k8s-deployment':
    case 'k8s-service':
      return normalized;
    default:
      return undefined;
  }
};
