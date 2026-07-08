import { Route, Router } from '@solidjs/router';
import { cleanup, fireEvent, render, screen, waitFor } from '@solidjs/testing-library';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import type { Resource } from '@/types/resource';
import { KubernetesPageSurface } from '../KubernetesPageSurface';

// URL-backed shared-toolbar filters: the workloads / services / configuration
// tabs read search (q) and status from the URL so saved views capture -term
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

  it('clears search, status, and namespace in one navigation on reset', async () => {
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

    renderSurfaceAt('/kubernetes/workloads?q=-cache&status=offline&namespace=prod');

    fireEvent.click(screen.getByLabelText('Clear all'));

    // A multi-write reset would leave earlier-cleared params resurrected by
    // later writes (each merges against the pre-navigation URL); the settled
    // URL must lose all three params.
    await waitFor(() => expect(window.location.search).toBe(''));
    expect(screen.getByText('checkout-api')).toBeInTheDocument();
    expect(screen.getByText('cache-worker')).toBeInTheDocument();
  });
});
