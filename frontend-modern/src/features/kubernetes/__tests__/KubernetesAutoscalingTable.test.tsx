import { cleanup, render, screen } from '@solidjs/testing-library';
import { afterEach, describe, expect, it } from 'vitest';

import type { Resource } from '@/types/resource';
import { KubernetesAutoscalingTable } from '../KubernetesAutoscalingTable';

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

describe('KubernetesAutoscalingTable', () => {
  it('renders HorizontalPodAutoscaler fields from the autoscaling API shape', () => {
    render(() => (
      <KubernetesAutoscalingTable
        resources={[
          makeResource({
            id: 'checkout-api-hpa',
            type: 'k8s-horizontal-pod-autoscaler',
            kubernetes: {
              clusterName: 'prod',
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
            tags: ['app:checkout', 'tier:backend'],
          }),
        ]}
        emptyIcon={<span />}
        emptyTitle="No autoscaling"
        emptyDescription="No autoscaling"
        showToolbar={false}
      />
    ));

    expect(screen.getByText('Autoscaler')).toBeInTheDocument();
    expect(screen.getByText('Scale target')).toBeInTheDocument();
    expect(screen.getByText('Bounds')).toBeInTheDocument();
    expect(screen.getByText('Metrics')).toBeInTheDocument();
    expect(screen.getByText('checkout-api-hpa')).toBeInTheDocument();
    expect(screen.getByText('prod/apps')).toBeInTheDocument();
    expect(screen.getByText('Deployment/checkout-api')).toBeInTheDocument();
    expect(screen.getByText('2-10')).toBeInTheDocument();
    expect(screen.getByText('4')).toBeInTheDocument();
    expect(screen.getByText('5')).toBeInTheDocument();
    expect(screen.getByText('Resource, Pods')).toBeInTheDocument();
  });
});
