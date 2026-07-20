import { cleanup, render, screen } from '@solidjs/testing-library';
import { afterEach, describe, expect, it, vi } from 'vitest';

import type { Resource } from '@/types/resource';
import { KubernetesDeploymentsTable } from '../KubernetesDeploymentsTable';

const makeResource = ({
  id,
  type = 'k8s-deployment',
  ...overrides
}: Partial<Resource> & Pick<Resource, 'id'>): Resource => ({
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
  vi.useRealTimers();
});

describe('KubernetesDeploymentsTable', () => {
  it('renders Deployment status fields from the Kubernetes apps API contract', () => {
    vi.useFakeTimers();
    vi.setSystemTime(new Date('2026-05-24T17:00:00Z'));

    render(() => (
      <KubernetesDeploymentsTable
        resources={[
          makeResource({
            id: 'checkout-api',
            kubernetes: {
              clusterId: 'prod-euw1',
              clusterName: 'Production EUW1',
              namespace: 'services',
              resourceKind: 'Deployment',
              resourceUid: 'deploy-uid-1',
              deploymentUid: 'deploy-uid-1',
              desiredReplicas: 4,
              updatedReplicas: 3,
              readyReplicas: 2,
              availableReplicas: 2,
              observedGeneration: 12,
              createdAt: '2026-05-24T15:00:00Z',
            },
          }),
        ]}
        emptyIcon={<span />}
        emptyTitle="No deployments"
        emptyDescription="No deployments"
        showToolbar={false}
      />
    ));

    expect(screen.getByText('Deployment')).toBeInTheDocument();
    expect(screen.getByText('Namespace')).toBeInTheDocument();
    expect(screen.getByText('Desired')).toBeInTheDocument();
    expect(screen.getByText('Updated')).toBeInTheDocument();
    expect(screen.getByText('Ready')).toBeInTheDocument();
    expect(screen.getByText('Available')).toBeInTheDocument();
    // observedGeneration is deliberately not a column: the raw number is
    // unactionable without the spec generation beside it.
    expect(screen.queryByText('Observed')).toBeNull();
    expect(screen.getByText('Age')).toBeInTheDocument();

    expect(screen.getByText('checkout-api')).toBeInTheDocument();
    expect(screen.getByText('services')).toBeInTheDocument();
    expect(screen.getByText('Production EUW1')).toBeInTheDocument();
    expect(screen.getByText('4')).toBeInTheDocument();
    expect(screen.getByText('3')).toBeInTheDocument();
    expect(screen.getAllByText('2')).toHaveLength(2);
    expect(screen.queryByText('12')).toBeNull();
    expect(screen.getByText('2h ago')).toBeInTheDocument();
    expect(
      document.querySelector('[data-kubernetes-deployment-row="checkout-api"]'),
    ).not.toBeNull();
  });

  it('maps Deployment status from replica progress and orders attention rows first', () => {
    render(() => (
      <KubernetesDeploymentsTable
        resources={[
          makeResource({
            id: 'healthy',
            kubernetes: { desiredReplicas: 3, readyReplicas: 3 },
          }),
          makeResource({
            id: 'partial',
            kubernetes: { desiredReplicas: 3, readyReplicas: 1 },
          }),
          makeResource({
            id: 'broken',
            kubernetes: { desiredReplicas: 3, readyReplicas: 0 },
          }),
        ]}
        emptyIcon={<span />}
        emptyTitle="No deployments"
        emptyDescription="No deployments"
        showToolbar={false}
      />
    ));

    const rows = Array.from(document.querySelectorAll('[data-kubernetes-deployment-row]')).map(
      (row) => row.getAttribute('data-kubernetes-deployment-row'),
    );
    expect(rows).toEqual(['broken', 'partial', 'healthy']);
    expect(screen.getByTitle('0 / 3 ready')).toHaveClass('bg-red-500');
    expect(screen.getByTitle('1 / 3 ready')).toHaveClass('bg-amber-500');
    expect(screen.getByTitle('Ready')).toHaveClass('bg-emerald-500');
  });
});
