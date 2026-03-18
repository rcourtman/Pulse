import { describe, expect, it } from 'vitest';
import {
  getK8sNamespacesDrawerPresentation,
  getK8sNamespacesEmptyState,
  getK8sNamespacesFailureState,
  getK8sNamespacesLoadingState,
} from '../k8sNamespacePresentation';

describe('k8sNamespacePresentation', () => {
  it('returns canonical drawer presentation', () => {
    expect(getK8sNamespacesDrawerPresentation()).toEqual({
      title: 'Namespaces',
      description: 'Scope Pods and Deployments by namespace',
      searchPlaceholder: 'Search namespaces...',
      openAllPodsLabel: 'Open All Pods',
      openPodsLabel: 'Open Pods',
      viewDeploymentsLabel: 'View Deployments',
      namespaceColumnLabel: 'Namespace',
      podsColumnLabel: 'Pods',
      deploymentsColumnLabel: 'Deployments',
      actionsColumnLabel: 'Actions',
    });
  });

  it('returns canonical namespaces empty-state copy', () => {
    expect(getK8sNamespacesLoadingState()).toEqual({
      title: 'Loading namespaces...',
      description: 'Aggregating Kubernetes namespaces.',
    });
    expect(getK8sNamespacesFailureState('boom')).toEqual({
      title: 'Failed to load namespaces',
      description: 'boom',
    });
    expect(getK8sNamespacesFailureState()).toEqual({
      title: 'Failed to load namespaces',
      description: 'Unknown error',
    });
    expect(getK8sNamespacesEmptyState(true)).toEqual({
      title: 'No namespaces match your filters',
      description: 'Try clearing your search.',
    });
    expect(getK8sNamespacesEmptyState(false)).toEqual({
      title: 'No namespaces found',
      description: 'Enable Kubernetes collection and wait for the next report.',
    });
  });
});
