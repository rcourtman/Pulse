import { describe, expect, it } from 'vitest';
import {
  getK8sDeploymentsDrawerPresentation,
  getK8sDeploymentsEmptyState,
  getK8sDeploymentsLoadingState,
} from '../k8sDeploymentPresentation';

describe('k8sDeploymentPresentation', () => {
  it('returns canonical drawer presentation', () => {
    expect(getK8sDeploymentsDrawerPresentation()).toEqual({
      title: 'Deployments',
      description: 'Desired state controllers (not Pods)',
      searchPlaceholder: 'Search deployments...',
      namespaceFilterLabel: 'Namespace',
      allNamespacesLabel: 'All namespaces',
      openPodsLabel: 'Open Pods',
      viewPodsLabel: 'View Pods',
      deploymentColumnLabel: 'Deployment',
      namespaceColumnLabel: 'Namespace',
      desiredColumnLabel: 'Desired',
      updatedColumnLabel: 'Updated',
      readyColumnLabel: 'Ready',
      availableColumnLabel: 'Available',
      actionsColumnLabel: 'Actions',
    });
  });

  it('returns canonical deployments empty-state copy', () => {
    expect(getK8sDeploymentsLoadingState()).toEqual({
      title: 'Loading deployments...',
      description: 'Fetching unified resources.',
    });
    expect(getK8sDeploymentsEmptyState(true)).toEqual({
      title: 'No deployments match your filters',
      description: 'Try clearing the search or namespace filter.',
    });
    expect(getK8sDeploymentsEmptyState(false)).toEqual({
      title: 'No deployments found',
      description:
        'Enable the Kubernetes agent deployment collection, then wait for the next report.',
    });
  });
});
