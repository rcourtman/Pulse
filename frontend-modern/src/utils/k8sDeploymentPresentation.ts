export const K8S_DEPLOYMENTS_DRAWER_TITLE = 'Deployments';
export const K8S_DEPLOYMENTS_DRAWER_DESCRIPTION = 'Desired state controllers (not Pods)';
export const K8S_DEPLOYMENTS_SEARCH_PLACEHOLDER = 'Search deployments...';
export const K8S_DEPLOYMENTS_NAMESPACE_FILTER_LABEL = 'Namespace';
export const K8S_DEPLOYMENTS_ALL_NAMESPACES_LABEL = 'All namespaces';
export const K8S_DEPLOYMENTS_OPEN_PODS_LABEL = 'Open Pods';
export const K8S_DEPLOYMENTS_VIEW_PODS_LABEL = 'View Pods';
export const K8S_DEPLOYMENTS_COLUMN_DEPLOYMENT_LABEL = 'Deployment';
export const K8S_DEPLOYMENTS_COLUMN_NAMESPACE_LABEL = 'Namespace';
export const K8S_DEPLOYMENTS_COLUMN_DESIRED_LABEL = 'Desired';
export const K8S_DEPLOYMENTS_COLUMN_UPDATED_LABEL = 'Updated';
export const K8S_DEPLOYMENTS_COLUMN_READY_LABEL = 'Ready';
export const K8S_DEPLOYMENTS_COLUMN_AVAILABLE_LABEL = 'Available';
export const K8S_DEPLOYMENTS_COLUMN_ACTIONS_LABEL = 'Actions';

export function getK8sDeploymentsDrawerPresentation() {
  return {
    title: K8S_DEPLOYMENTS_DRAWER_TITLE,
    description: K8S_DEPLOYMENTS_DRAWER_DESCRIPTION,
    searchPlaceholder: K8S_DEPLOYMENTS_SEARCH_PLACEHOLDER,
    namespaceFilterLabel: K8S_DEPLOYMENTS_NAMESPACE_FILTER_LABEL,
    allNamespacesLabel: K8S_DEPLOYMENTS_ALL_NAMESPACES_LABEL,
    openPodsLabel: K8S_DEPLOYMENTS_OPEN_PODS_LABEL,
    viewPodsLabel: K8S_DEPLOYMENTS_VIEW_PODS_LABEL,
    deploymentColumnLabel: K8S_DEPLOYMENTS_COLUMN_DEPLOYMENT_LABEL,
    namespaceColumnLabel: K8S_DEPLOYMENTS_COLUMN_NAMESPACE_LABEL,
    desiredColumnLabel: K8S_DEPLOYMENTS_COLUMN_DESIRED_LABEL,
    updatedColumnLabel: K8S_DEPLOYMENTS_COLUMN_UPDATED_LABEL,
    readyColumnLabel: K8S_DEPLOYMENTS_COLUMN_READY_LABEL,
    availableColumnLabel: K8S_DEPLOYMENTS_COLUMN_AVAILABLE_LABEL,
    actionsColumnLabel: K8S_DEPLOYMENTS_COLUMN_ACTIONS_LABEL,
  } as const;
}

export function getK8sDeploymentsLoadingState() {
  return {
    title: 'Loading deployments...',
    description: 'Fetching unified resources.',
  } as const;
}

export function getK8sDeploymentsEmptyState(hasDeployments: boolean) {
  return hasDeployments
    ? {
        title: 'No deployments match your filters',
        description: 'Try clearing the search or namespace filter.',
      }
    : {
        title: 'No deployments found',
        description:
          'Enable the Kubernetes agent deployment collection, then wait for the next report.',
      };
}
