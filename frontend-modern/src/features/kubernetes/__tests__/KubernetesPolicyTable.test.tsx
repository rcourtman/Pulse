import { cleanup, fireEvent, render, screen, within } from '@solidjs/testing-library';
import { afterEach, describe, expect, it } from 'vitest';

import type { Resource } from '@/types/resource';
import { KubernetesPolicyTable } from '../KubernetesPolicyTable';

const makeResource = ({
  id,
  type,
  ...overrides
}: Partial<Resource> & Pick<Resource, 'id' | 'type'>): Resource => ({
  id,
  name: id,
  displayName: id,
  platformId: 'cluster-1',
  platformType: 'kubernetes',
  sourceType: 'agent',
  sources: ['kubernetes'],
  status: 'online',
  type,
  lastSeen: 1_700_000_000_000,
  ...overrides,
});

afterEach(() => {
  cleanup();
});

describe('KubernetesPolicyTable', () => {
  it('renders Kubernetes policy resources with API-native policy fields', () => {
    render(() => (
      <KubernetesPolicyTable
        resources={[
          makeResource({
            id: 'default-deny',
            type: 'k8s-network-policy',
            kubernetes: {
              clusterName: 'prod',
              namespace: 'apps',
              resourceKind: 'NetworkPolicy',
              policyTypes: ['Ingress', 'Egress'],
              ingressRuleCount: 1,
              egressRuleCount: 2,
            },
            tags: ['tier:backend'],
          }),
          makeResource({
            id: 'checkout-budget',
            type: 'k8s-pod-disruption-budget',
            kubernetes: {
              clusterName: 'prod',
              namespace: 'apps',
              resourceKind: 'PodDisruptionBudget',
              minAvailable: '1',
              desiredHealthy: 2,
              currentHealthy: 1,
              disruptionsAllowed: 0,
              expectedPods: 3,
            },
          }),
          makeResource({
            id: 'apps-quota',
            type: 'k8s-resource-quota',
            kubernetes: {
              clusterName: 'prod',
              namespace: 'apps',
              resourceKind: 'ResourceQuota',
              hard: {
                pods: '80',
                'requests.cpu': '24',
                'requests.memory': '96Gi',
              },
              used: {
                pods: '34',
                'requests.cpu': '11',
                'requests.memory': '42Gi',
              },
            },
          }),
          makeResource({
            id: 'apps-limits',
            type: 'k8s-limit-range',
            kubernetes: {
              clusterName: 'prod',
              namespace: 'apps',
              resourceKind: 'LimitRange',
              limitTypes: ['Container', 'Pod'],
            },
          }),
        ]}
        emptyIcon={<span />}
        emptyTitle="No policy"
        emptyDescription="No policy"
        showToolbar={false}
      />
    ));

    expect(screen.getByText('Policy shape')).toBeInTheDocument();
    expect(screen.getByText('Spec / limits')).toBeInTheDocument();
    expect(screen.getByText('Observed state')).toBeInTheDocument();
    expect(screen.getByText('NetworkPolicy')).toBeInTheDocument();
    expect(screen.getByText('Ingress, Egress policy')).toBeInTheDocument();
    expect(screen.getByText('1 ingress, 2 egress')).toBeInTheDocument();
    expect(screen.getByText('PodDisruptionBudget')).toBeInTheDocument();
    expect(screen.getByText('min 1')).toBeInTheDocument();
    expect(screen.getByText('1/2 healthy, 0 disruptions, 3 pods')).toBeInTheDocument();
    expect(screen.getByText('ResourceQuota')).toBeInTheDocument();
    expect(screen.getByText('3 hard limits')).toBeInTheDocument();
    expect(screen.getByText(/pods 34\/80/)).toBeInTheDocument();
    expect(screen.getByText('LimitRange')).toBeInTheDocument();
    expect(screen.getByText('Container, Pod')).toBeInTheDocument();
  });
  it('opens a ResourceQuota as a record with every limit in full', () => {
    render(() => (
      <KubernetesPolicyTable
        resources={[
          makeResource({
            id: 'apps-quota',
            type: 'k8s-resource-quota',
            kubernetes: {
              clusterName: 'prod',
              namespace: 'apps',
              resourceKind: 'ResourceQuota',
              hard: { pods: '80', 'requests.cpu': '24', 'requests.memory': '96Gi' },
              used: { pods: '34', 'requests.cpu': '11', 'requests.memory': '42Gi' },
            },
          }),
        ]}
        emptyIcon={<span />}
        emptyTitle="No policy"
        emptyDescription="No policy"
        showToolbar={false}
      />
    ));

    const disclosure = screen.getByRole('button', { name: 'Expand details for apps-quota' });
    fireEvent.click(disclosure);
    const panel = document.querySelector<HTMLElement>('[data-testid="kubernetes-policy-detail"]')!;
    const detail = within(panel);
    expect(disclosure).toHaveAttribute('aria-controls', 'kubernetes-policy-drawer-apps-quota');
    expect(detail.getByRole('heading', { name: 'apps-quota' })).toBeInTheDocument();
    expect(detail.getByText('ResourceQuota')).toBeInTheDocument();
    expect(detail.getByText('34 of 80')).toBeInTheDocument();
    expect(detail.getByText('requests.memory')).toBeInTheDocument();
    expect(detail.getByText('42Gi of 96Gi')).toBeInTheDocument();
    expect(detail.queryByRole('tab', { name: 'Manage' })).toBeNull();

    fireEvent.click(detail.getByRole('button', { name: 'Collapse apps-quota details' }));
    expect(document.querySelector('[data-testid="kubernetes-policy-detail"]')).toBeNull();
  });
});
