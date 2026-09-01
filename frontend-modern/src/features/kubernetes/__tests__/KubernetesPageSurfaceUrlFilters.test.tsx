import { Route, Router } from '@solidjs/router';
import { cleanup, fireEvent, render, screen, waitFor } from '@solidjs/testing-library';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import type { Resource } from '@/types/resource';
import { KubernetesPageSurface } from '../KubernetesPageSurface';

// URL-backed shared-toolbar filters: the workloads / services / configuration
// tabs read search (q) and status from the URL so bookmarks capture -term
// exclusions, mirroring the Docker containers table. These tests render the
// real surface with a real router; only the data hooks are mocked.

const mockUseUnifiedResources = vi.fn();
const mockVersionInfo = vi.hoisted(() => vi.fn());

vi.mock('@/hooks/useUnifiedResources', () => ({
  useUnifiedResources: (...args: unknown[]) => mockUseUnifiedResources(...args),
}));

vi.mock('@/stores/updates', () => ({
  updateStore: {
    versionInfo: mockVersionInfo,
  },
}));

const makeResource = (resource: Partial<Resource> & Pick<Resource, 'id' | 'type'>): Resource => ({
  name: resource.id,
  displayName: resource.id,
  platformId: 'cluster-1',
  platformType: 'kubernetes',
  sourceType: 'agent',
  sources: ['kubernetes'],
  status: 'online',
  lastSeen: 1_700_000_000_000,
  kubernetes: {
    clusterId: 'cluster-1',
    clusterName: 'Cluster 1',
    namespace: 'default',
  },
  ...resource,
});

const setResources = (resources: Resource[]) => {
  mockUseUnifiedResources.mockReturnValue({
    resources: () => resources,
    loading: () => false,
    error: () => null,
    refetch: vi.fn(),
  });
};

const renderSurfaceAt = (url: string) => {
  window.history.pushState({}, '', url);
  return render(() => (
    <Router>
      <Route path="/kubernetes/:section?" component={KubernetesPageSurface} />
    </Router>
  ));
};

afterEach(() => {
  cleanup();
  vi.clearAllMocks();
});

beforeEach(() => {
  mockVersionInfo.mockReturnValue(undefined);
  window.history.pushState({}, '', '/kubernetes');
});

describe('Kubernetes URL-backed shared toolbar filters', () => {
  it('keeps routine workload attention compact and local to the workload toolbar', () => {
    setResources([
      makeResource({ id: 'cluster-1', type: 'k8s-cluster' }),
      makeResource({
        id: 'pending-pod',
        type: 'pod',
        kubernetes: {
          clusterId: 'cluster-1',
          clusterName: 'Cluster 1',
          namespace: 'default',
          podPhase: 'Pending',
        },
      }),
    ]);

    renderSurfaceAt('/kubernetes/overview');

    expect(screen.queryByRole('region', { name: 'Kubernetes attention' })).not.toBeInTheDocument();
    expect(
      screen
        .getAllByRole('status')
        .find((status) => status.textContent?.includes('1 workload needs attention')),
    ).toBeInTheDocument();
    expect(screen.getByRole('link', { name: 'Review' })).toHaveAttribute(
      'href',
      '/kubernetes/workloads',
    );
  });

  it('surfaces node availability in the clusters table when a node is not ready', () => {
    setResources([
      makeResource({ id: 'cluster-1', type: 'k8s-cluster' }),
      makeResource({
        id: 'unavailable-node',
        type: 'k8s-node',
        kubernetes: {
          clusterId: 'cluster-1',
          clusterName: 'Cluster 1',
          nodeName: 'unavailable-node',
          ready: false,
        },
      }),
    ]);

    renderSurfaceAt('/kubernetes/overview');

    // The dedicated attention banner was folded into the canonical tables:
    // node availability now reads as a warning-toned ready/total ratio on the
    // cluster row instead of a standalone summary region.
    expect(screen.queryByRole('region', { name: 'Kubernetes attention' })).not.toBeInTheDocument();
    const clusterRow = screen.getAllByText('Cluster 1')[0].closest('tr');
    expect(clusterRow).not.toBeNull();
    expect(clusterRow).toHaveTextContent('0/1');
    const degradedCount = clusterRow!.querySelector('.text-amber-700');
    expect(degradedCount).not.toBeNull();
    expect(degradedCount).toHaveTextContent('0');
  });

  it('applies the URL search filter, including -term exclusions, on the workloads tab', () => {
    setResources([
      makeResource({ id: 'checkout-api', type: 'k8s-deployment' }),
      makeResource({ id: 'cache-worker', type: 'k8s-deployment' }),
    ]);

    renderSurfaceAt('/kubernetes/workloads?q=-cache');

    expect(screen.getByText('checkout-api')).toBeInTheDocument();
    expect(screen.queryByText('cache-worker')).not.toBeInTheDocument();
  });

  it('applies the URL status filter on the workloads tab', () => {
    setResources([
      makeResource({ id: 'checkout-api', type: 'k8s-deployment', status: 'online' }),
      makeResource({ id: 'batch-runner', type: 'k8s-deployment', status: 'offline' }),
    ]);

    renderSurfaceAt('/kubernetes/workloads?status=offline');

    expect(screen.getByText('batch-runner')).toBeInTheDocument();
    expect(screen.queryByText('checkout-api')).not.toBeInTheDocument();
  });

  it('applies the URL cluster filter across workload sections', () => {
    setResources([
      makeResource({
        id: 'west-api',
        type: 'k8s-deployment',
        kubernetes: { clusterId: 'cluster-west', clusterName: 'West', namespace: 'prod' },
      }),
      makeResource({
        id: 'east-api',
        type: 'k8s-deployment',
        kubernetes: { clusterId: 'cluster-east', clusterName: 'East', namespace: 'prod' },
      }),
      makeResource({
        id: 'west-pod',
        type: 'pod',
        kubernetes: { clusterId: 'cluster-west', clusterName: 'West', namespace: 'prod' },
      }),
      makeResource({
        id: 'east-pod',
        type: 'pod',
        kubernetes: { clusterId: 'cluster-east', clusterName: 'East', namespace: 'prod' },
      }),
    ]);

    renderSurfaceAt('/kubernetes/workloads?cluster=cluster-west');

    expect(screen.getByText('west-api')).toBeInTheDocument();
    expect(screen.getByText('west-pod')).toBeInTheDocument();
    expect(screen.queryByText('east-api')).not.toBeInTheDocument();
    expect(screen.queryByText('east-pod')).not.toBeInTheDocument();
  });

  it('scopes the Overview workload inventory when a cluster name is selected', async () => {
    setResources([
      makeResource({
        id: 'cluster-west',
        type: 'k8s-cluster',
        name: 'West',
        kubernetes: { clusterId: 'cluster-west', clusterName: 'West' },
      }),
      makeResource({
        id: 'cluster-east',
        type: 'k8s-cluster',
        name: 'East',
        kubernetes: { clusterId: 'cluster-east', clusterName: 'East' },
      }),
      makeResource({
        id: 'west-api',
        type: 'k8s-deployment',
        kubernetes: { clusterId: 'cluster-west', clusterName: 'West', namespace: 'prod' },
      }),
      makeResource({
        id: 'east-api',
        type: 'k8s-deployment',
        kubernetes: { clusterId: 'cluster-east', clusterName: 'East', namespace: 'prod' },
      }),
    ]);

    renderSurfaceAt('/kubernetes/overview');
    fireEvent.click(screen.getByRole('button', { name: 'Show workloads for West' }));

    await waitFor(() => expect(window.location.search).toBe('?cluster=cluster-west'));
    expect(screen.getByText('west-api')).toBeInTheDocument();
    expect(screen.queryByText('east-api')).not.toBeInTheDocument();
  });

  it('applies the URL search filter on the services tab', () => {
    setResources([
      makeResource({ id: 'checkout-svc', type: 'k8s-service' }),
      makeResource({ id: 'cache-svc', type: 'k8s-service' }),
    ]);

    renderSurfaceAt('/kubernetes/services?q=-cache');

    expect(screen.getByText('checkout-svc')).toBeInTheDocument();
    expect(screen.queryByText('cache-svc')).not.toBeInTheDocument();
  });

  it('applies the URL search filter on the configuration tab', () => {
    setResources([
      makeResource({ id: 'app-settings', type: 'k8s-configmap' }),
      makeResource({ id: 'cache-settings', type: 'k8s-configmap' }),
    ]);

    renderSurfaceAt('/kubernetes/config?q=-cache');

    expect(screen.getByText('app-settings')).toBeInTheDocument();
    expect(screen.queryByText('cache-settings')).not.toBeInTheDocument();
  });

  it('clears search, status, cluster, and namespace in one navigation on reset', async () => {
    setResources([
      makeResource({
        id: 'checkout-api',
        type: 'k8s-deployment',
        kubernetes: { clusterId: 'cluster-1', clusterName: 'Cluster 1', namespace: 'prod' },
      }),
      makeResource({
        id: 'cache-worker',
        type: 'k8s-deployment',
        status: 'offline',
        kubernetes: { clusterId: 'cluster-1', clusterName: 'Cluster 1', namespace: 'staging' },
      }),
    ]);

    renderSurfaceAt(
      '/kubernetes/workloads?q=-cache&status=offline&cluster=cluster-1&namespace=prod',
    );

    fireEvent.click(screen.getByLabelText('Clear filters'));

    // A multi-write reset would leave earlier-cleared params resurrected by
    // later writes (each merges against the pre-navigation URL); the settled
    // URL must lose all four params.
    await waitFor(() => expect(window.location.search).toBe(''));
    expect(screen.getByText('checkout-api')).toBeInTheDocument();
    expect(screen.getByText('cache-worker')).toBeInTheDocument();
  });
});
