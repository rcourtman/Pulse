export const K8S_NAMESPACES_DRAWER_TITLE = 'Namespaces';
export const K8S_NAMESPACES_DRAWER_DESCRIPTION = 'Scope Pods and Deployments by namespace';
export const K8S_NAMESPACES_SEARCH_PLACEHOLDER = 'Search namespaces...';
// Pods and deployments share the Workloads tab, so one action opens both,
// scoped to the namespace.
export const K8S_NAMESPACES_OPEN_ALL_WORKLOADS_LABEL = 'Open all workloads';
export const K8S_NAMESPACES_OPEN_WORKLOADS_LABEL = 'Open workloads';
export const K8S_NAMESPACES_COLUMN_NAMESPACE_LABEL = 'Namespace';
export const K8S_NAMESPACES_COLUMN_PODS_LABEL = 'Pods';
export const K8S_NAMESPACES_COLUMN_DEPLOYMENTS_LABEL = 'Deployments';
export const K8S_NAMESPACES_COLUMN_ACTIONS_LABEL = 'Actions';

export function getK8sNamespacesDrawerPresentation() {
  return {
    title: K8S_NAMESPACES_DRAWER_TITLE,
    description: K8S_NAMESPACES_DRAWER_DESCRIPTION,
    searchPlaceholder: K8S_NAMESPACES_SEARCH_PLACEHOLDER,
    openAllWorkloadsLabel: K8S_NAMESPACES_OPEN_ALL_WORKLOADS_LABEL,
    openWorkloadsLabel: K8S_NAMESPACES_OPEN_WORKLOADS_LABEL,
    namespaceColumnLabel: K8S_NAMESPACES_COLUMN_NAMESPACE_LABEL,
    podsColumnLabel: K8S_NAMESPACES_COLUMN_PODS_LABEL,
    deploymentsColumnLabel: K8S_NAMESPACES_COLUMN_DEPLOYMENTS_LABEL,
    actionsColumnLabel: K8S_NAMESPACES_COLUMN_ACTIONS_LABEL,
  } as const;
}

export function getK8sNamespacesLoadingState() {
  return {
    title: 'Loading namespaces...',
    description: 'Aggregating Kubernetes namespaces.',
  } as const;
}

export function getK8sNamespacesFailureState(message?: string | null) {
  return {
    title: 'Failed to load namespaces',
    description: message || 'Unknown error',
  } as const;
}

export function getK8sNamespacesEmptyState(hasNamespaces: boolean) {
  return hasNamespaces
    ? {
        title: 'No namespaces match your filters',
        description: 'Try clearing your search.',
      }
    : {
        title: 'No namespaces found',
        description: 'Enable Kubernetes collection and wait for the next report.',
      };
}
