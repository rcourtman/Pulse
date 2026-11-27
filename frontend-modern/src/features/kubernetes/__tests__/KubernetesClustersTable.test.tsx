import { cleanup, fireEvent, render, screen } from '@solidjs/testing-library';
import { afterEach, describe, expect, it, vi } from 'vitest';

import type { Resource } from '@/types/resource';
import { KubernetesClustersTable } from '../KubernetesClustersTable';

vi.mock('@/contexts/appRuntime', async (importOriginal) => ({
  ...(await importOriginal<typeof import('@/contexts/appRuntime')>()),
  useWebSocket: () => ({ activeAlerts: {} }),
}));

vi.mock('@/api/resources', () => ({
  ResourceAPI: {
    getFacetBundle: vi.fn().mockResolvedValue({
      capabilities: [],
      relationships: [],
      recentChanges: [],
    }),
  },
}));

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
    const cluster = makeClusterResource({ customUrl: 'https://kubernetes.internal' });

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
    expect(screen.getByRole('link', { name: 'Open web interface for prod-west' })).toHaveAttribute(
      'href',
      'https://kubernetes.internal',
    );
    expect(screen.getByText('prod-west').closest('a')).toBeNull();
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

  it('lets the overview scope workload inventory from the cluster name', () => {
    const cluster = makeClusterResource();
    const onSelectCluster = vi.fn();

    render(() => (
      <KubernetesClustersTable
        clusters={[cluster]}
        scope={[cluster]}
        emptyIcon={<span />}
        emptyTitle="No clusters"
        emptyDescription="No clusters"
        showToolbar={false}
        selectedClusterId="prod-west"
        onSelectCluster={onSelectCluster}
      />
    ));

    const selector = screen.getByRole('button', { name: 'Show workloads for prod-west' });
    expect(selector).toHaveAttribute('aria-pressed', 'true');
    fireEvent.click(selector);
    expect(onSelectCluster).toHaveBeenCalledWith(cluster);
  });

  it('marks stale Kubernetes cluster readings without hiding retained metrics or offline severity', () => {
    const staleCluster = makeClusterResource({
      id: 'cluster:stale-eu',
      name: 'stale-eu',
      status: 'online',
      sourceStatus: {
        kubernetes: {
          status: 'stale',
          lastSeen: '2026-11-26T10:00:00Z',
          error: 'agent heartbeat timed out',
        },
        availability: { status: 'online', lastSeen: '2026-11-26T12:00:00Z' },
      },
      kubernetes: {
        clusterId: 'stale-eu',
        clusterName: 'stale-eu',
        context: 'stale-eu-admin',
        version: 'v1.31.3',
      },
    });
    const offlineStaleCluster = makeClusterResource({
      id: 'cluster:offline-us',
      name: 'offline-us',
      status: 'offline',
      sourceStatus: {
        kubernetes: { status: 'stale', lastSeen: '2026-11-26T09:00:00Z' },
      },
      kubernetes: {
        clusterId: 'offline-us',
        clusterName: 'offline-us',
      },
    });

    const { container } = render(() => (
      <KubernetesClustersTable
        clusters={[staleCluster, offlineStaleCluster]}
        scope={[staleCluster, offlineStaleCluster]}
        emptyIcon={<span />}
        emptyTitle="No clusters"
        emptyDescription="No clusters"
        showToolbar={true}
      />
    ));

    const staleRow = container.querySelector('[data-kubernetes-cluster-row="cluster:stale-eu"]');
    expect(staleRow?.querySelector('[data-kubernetes-stale="cluster"]')).toHaveTextContent(
      'Stale data',
    );
    expect(staleRow?.querySelector('[title="Stale data"]')).toHaveClass('bg-amber-500');
    expect(staleRow?.querySelector('[data-testid="responsive-metric-cell"]')).not.toBeNull();

    // Offline clusters preserve red severity while still noting stale readings.
    const offlineRow = container.querySelector(
      '[data-kubernetes-cluster-row="cluster:offline-us"]',
    );
    expect(offlineRow?.querySelector('[title="Offline"]')).toHaveClass('bg-red-500');
    expect(offlineRow?.querySelector('[data-kubernetes-stale="cluster"]')).toHaveTextContent(
      'Stale data',
    );

    // Expanding the stale cluster explains why its readings are retained.
    fireEvent.click(staleRow as HTMLElement);
    const notice = screen.getByTestId('kubernetes-stale-readings-notice');
    expect(notice).toHaveTextContent('Stale data.');
    expect(notice).toHaveTextContent('agent heartbeat timed out');
  });
});
