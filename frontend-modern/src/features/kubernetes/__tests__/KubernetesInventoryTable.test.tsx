import { cleanup, render, screen } from '@solidjs/testing-library';
import { afterEach, describe, expect, it } from 'vitest';

import type { Resource } from '@/types/resource';
import { KubernetesInventoryTable } from '../KubernetesInventoryTable';

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

describe('KubernetesInventoryTable', () => {
  it('renders autoscaling inventory fields in the remaining generic table', () => {
    render(() => (
      <KubernetesInventoryTable
        resources={[
          makeResource({
            id: 'checkout-api-hpa',
            type: 'k8s-horizontal-pod-autoscaler',
            kubernetes: {
              namespace: 'apps',
              resourceKind: 'HorizontalPodAutoscaler',
              targetKind: 'Deployment',
              targetName: 'checkout-api',
              minReplicas: 2,
              maxReplicas: 10,
              currentReplicas: 4,
              desiredReplicas: 5,
              metricTypes: ['Resource', 'Pods'],
            },
          }),
        ]}
        variant="autoscaling"
        emptyIcon={<span />}
        emptyTitle="No autoscaling"
        emptyDescription="No autoscaling"
        showToolbar={false}
      />
    ));

    expect(screen.getByText('Target')).toBeInTheDocument();
    expect(screen.getByText('Min')).toBeInTheDocument();
    expect(screen.getByText('Max')).toBeInTheDocument();
    expect(screen.getByText('Current')).toBeInTheDocument();
    expect(screen.getByText('Desired')).toBeInTheDocument();
    expect(screen.getByText('Metrics')).toBeInTheDocument();
    expect(screen.getByText('Deployment/checkout-api')).toBeInTheDocument();
    expect(screen.getByText('Resource, Pods')).toBeInTheDocument();
  });
});
