import { cleanup, render, screen } from '@solidjs/testing-library';
import { afterEach, describe, expect, it, vi } from 'vitest';

import type { Resource } from '@/types/resource';
import { KubernetesClustersTable } from '../KubernetesClustersTable';

vi.mock('@/components/shared/responsive', () => ({
  ResponsiveMetricCell: () => <div data-testid="responsive-metric-cell" />,
}));

vi.mock('@/components/Workloads/StackedMemoryBar', () => ({
  StackedMemoryBar: () => <div data-testid="stacked-memory-bar" />,
}));

const makeClusterResource = (overrides: Partial<Resource> = {}): Resource => ({
  id: 'cluster:prod-west',
  name: 'prod-west',
  displayName: 'prod-west',
  platformId: 'homelab',
  platformType: 'kubernetes',
  sourceType: 'hybrid',
  status: 'online',
  type: 'k8s-cluster',
  lastSeen: 1_700_000_000_000,
  cpu: { current: 42 },
  memory: { total: 32_000, used: 20_000, free: 12_000, current: 62.5 },
  kubernetes: {
    clusterId: 'prod-west',
    clusterName: 'prod-west',
    context: 'prod-west-admin',
    version: 'v1.31.3',
    server: 'https://prod-west.example.invalid',
  },
  ...overrides,
});

afterEach(() => {
  cleanup();
  vi.clearAllMocks();
});

describe('KubernetesClustersTable', () => {
  it('renders canonical CPU and memory metric primitives for cluster rows', () => {
    const cluster = makeClusterResource();

    render(() => (
      <KubernetesClustersTable
        clusters={[cluster]}
        scope={[
          cluster,
          {
            ...makeClusterResource({
              id: 'node-1',
              type: 'k8s-node',
              name: 'node-1',
              kubernetes: {
                clusterId: 'prod-west',
                clusterName: 'prod-west',
              },
            }),
          },
        ]}
        emptyIcon={<span />}
        emptyTitle="No clusters"
        emptyDescription="No clusters"
        showToolbar={false}
      />
    ));

    expect(screen.getByText('prod-west')).toBeInTheDocument();
    expect(screen.getByTestId('responsive-metric-cell')).toBeInTheDocument();
    expect(screen.getByTestId('stacked-memory-bar')).toBeInTheDocument();
    expect(screen.queryByText('Pending uninstall')).toBeNull();
  });

  it('renders child counts through the shared count-ratio primitive', () => {
    const cluster = makeClusterResource();

    const { container } = render(() => (
      <KubernetesClustersTable
        clusters={[cluster]}
        scope={[
          cluster,
          {
            ...makeClusterResource({
              id: 'node-ready',
              type: 'k8s-node',
              name: 'node-ready',
              status: 'online',
              kubernetes: {
                clusterId: 'prod-west',
                clusterName: 'prod-west',
                ready: true,
              },
            }),
          },
          {
            ...makeClusterResource({
              id: 'node-not-ready',
              type: 'k8s-node',
              name: 'node-not-ready',
              status: 'offline',
              kubernetes: {
                clusterId: 'prod-west',
                clusterName: 'prod-west',
                ready: false,
              },
            }),
          },
        ]}
        emptyIcon={<span />}
        emptyTitle="No clusters"
        emptyDescription="No clusters"
        showToolbar={false}
      />
    ));

    const row = container.querySelector('[data-kubernetes-cluster-row="cluster:prod-west"]');
    expect(row?.textContent).toContain('1/2');
    expect(row?.querySelector('.text-amber-700')?.textContent).toBe('1');
    expect(row?.querySelectorAll('.tabular-nums').length).toBeGreaterThanOrEqual(2);
  });

  it('badges clusters whose agent is pending uninstall', () => {
    const cluster = makeClusterResource({
      kubernetes: {
        clusterId: 'prod-west',
        clusterName: 'prod-west',
        pendingUninstall: true,
      },
    });

    render(() => (
      <KubernetesClustersTable
        clusters={[cluster]}
        scope={[cluster]}
        emptyIcon={<span />}
        emptyTitle="No clusters"
        emptyDescription="No clusters"
        showToolbar={false}
      />
    ));

    expect(screen.getByText('Pending uninstall')).toBeInTheDocument();
  });
});
