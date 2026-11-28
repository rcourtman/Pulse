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
    expect(screen.getByText('Target')).toBeInTheDocument();
    expect(screen.getByText('Bounds')).toBeInTheDocument();
    expect(screen.getByText('Metrics')).toBeInTheDocument();
    expect(screen.getByText('checkout-api-hpa')).toBeInTheDocument();
    expect(screen.getByText('apps').closest('[data-kubernetes-scope]')).toHaveAttribute(
      'title',
      'prod/apps',
    );
    expect(screen.getByText('Deployment/checkout-api')).toBeInTheDocument();
    expect(screen.getByText('2-10')).toBeInTheDocument();
    expect(screen.getByText('4')).toBeInTheDocument();
    expect(screen.getByText('5')).toBeInTheDocument();
    expect(screen.getByText('Resource, Pods')).toBeInTheDocument();
  });

  it('surfaces stale readings on autoscalers while preserving worse failure states', () => {
    const staleSource = {
      kubernetes: {
        status: 'stale',
        lastSeen: '2026-11-26T10:00:00Z',
        error: 'k8s collector timed out',
      },
    };
    const { container } = render(() => (
      <KubernetesAutoscalingTable
        resources={[
          makeResource({
            id: 'fresh-hpa',
            type: 'k8s-horizontal-pod-autoscaler',
            status: 'online',
            kubernetes: {
              clusterName: 'prod',
              namespace: 'apps',
              targetKind: 'Deployment',
              targetName: 'web',
              minReplicas: 2,
              maxReplicas: 6,
              currentReplicas: 2,
              desiredReplicas: 2,
            },
          }),
          makeResource({
            id: 'stale-hpa',
            type: 'k8s-horizontal-pod-autoscaler',
            status: 'online',
            sourceStatus: staleSource,
            kubernetes: {
              clusterName: 'edge',
              namespace: 'apps',
              targetKind: 'Deployment',
              targetName: 'api',
              minReplicas: 1,
              maxReplicas: 8,
              currentReplicas: 3,
              desiredReplicas: 4,
            },
          }),
          makeResource({
            id: 'offline-stale-hpa',
            type: 'k8s-horizontal-pod-autoscaler',
            status: 'offline',
            sourceStatus: staleSource,
            kubernetes: {
              clusterName: 'edge',
              namespace: 'apps',
              targetKind: 'Deployment',
              targetName: 'worker',
              minReplicas: 1,
              maxReplicas: 4,
              currentReplicas: 0,
              desiredReplicas: 1,
            },
          }),
        ]}
        emptyIcon={<span />}
        emptyTitle="No autoscaling"
        emptyDescription="No autoscaling"
        showToolbar={false}
      />
    ));

    const freshRow = container.querySelector('[data-kubernetes-autoscaling-row="fresh-hpa"]');
    expect(freshRow?.querySelector('[title="Online"]')).toHaveClass('bg-emerald-500');
    expect(freshRow?.querySelector('[data-kubernetes-stale="autoscaling"]')).toBeNull();

    const staleRow = container.querySelector('[data-kubernetes-autoscaling-row="stale-hpa"]');
    expect(staleRow?.querySelector('[title="Stale data"]')).toHaveClass('bg-amber-500');
    expect(staleRow?.querySelector('[data-kubernetes-stale="autoscaling"]')).toHaveTextContent(
      'Stale data',
    );
    expect(staleRow).toHaveTextContent('Deployment/api');

    const offlineRow = container.querySelector(
      '[data-kubernetes-autoscaling-row="offline-stale-hpa"]',
    );
    expect(offlineRow?.querySelector('[title="Offline"]')).toHaveClass('bg-red-500');
    expect(offlineRow?.querySelector('[data-kubernetes-stale="autoscaling"]')).toHaveTextContent(
      'Stale data',
    );
  });
});
