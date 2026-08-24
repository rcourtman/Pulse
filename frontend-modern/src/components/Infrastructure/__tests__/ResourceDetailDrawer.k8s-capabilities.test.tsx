import { describe, expect, it, vi } from 'vitest';
import { fireEvent, render, screen, waitFor } from '@solidjs/testing-library';

import type { Resource } from '@/types/resource';
import { ResourceDetailDrawer } from '@/components/Infrastructure/ResourceDetailDrawer';

const navigateSpy = vi.hoisted(() => vi.fn());

vi.mock('@solidjs/router', async () => {
  const actual = await vi.importActual<typeof import('@solidjs/router')>('@solidjs/router');
  return {
    ...actual,
    useLocation: () => ({ pathname: '/', search: '' }),
    useNavigate: () => navigateSpy,
  };
});

vi.mock('@/components/Discovery/DiscoveryTab', () => ({
  DiscoveryTab: () => <div data-testid="discovery-tab" />,
}));

vi.mock('@/components/Kubernetes/K8sNamespacesDrawer', () => ({
  K8sNamespacesDrawer: (props: { cluster: string }) => (
    <div data-testid="k8s-namespaces-drawer" data-cluster={props.cluster} />
  ),
}));

vi.mock('@/components/Kubernetes/K8sDeploymentsDrawer', () => ({
  K8sDeploymentsDrawer: (props: { cluster: string }) => (
    <div data-testid="k8s-deployments-drawer" data-cluster={props.cluster} />
  ),
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

vi.mock('@/api/ai', () => ({
  AIAPI: {
    getResourceIntelligence: vi.fn().mockResolvedValue({
      health: {
        grade: 'A',
        score: 95,
        trend: 'stable',
      },
      dependencies: [],
      dependents: [],
      correlations: [],
      note_count: 0,
      recent_changes: [],
    }),
  },
}));

const buildKubernetesResource = (): Resource => ({
  id: 'k8s-cluster-1',
  type: 'k8s-cluster',
  name: 'secret-cluster',
  displayName: 'Governed Cluster',
  platformId: 'cluster-a',
  platformType: 'kubernetes',
  sourceType: 'api',
  status: 'online',
  lastSeen: Date.now(),
  platformData: {
    sources: ['kubernetes'],
    kubernetes: {
      clusterName: 'cluster-a',
      metricCapabilities: {
        nodeCpuMemory: true,
        nodeTelemetry: true,
        podCpuMemory: true,
        podNetwork: true,
        podEphemeralDisk: true,
        podDiskIo: false,
      },
    },
  },
});

describe('ResourceDetailDrawer kubernetes capabilities', () => {
  it('keeps raw Kubernetes collection capabilities out of the operator overview', async () => {
    const { queryByText } = render(() => (
      <ResourceDetailDrawer resource={buildKubernetesResource()} />
    ));

    const details = (await screen.findByTestId('resource-platform-details')) as HTMLDetailsElement;
    details.open = true;
    fireEvent(details, new Event('toggle'));
    expect(queryByText('Platform signals')).toBeNull();
    expect(queryByText('K8s Node CPU/Memory')).toBeNull();
    expect(queryByText('Node Telemetry (Agent)')).toBeNull();
    expect(screen.getByRole('tab', { name: 'Namespaces' })).toBeInTheDocument();
    expect(screen.getByRole('tab', { name: 'Deployments' })).toBeInTheDocument();
  });

  it('uses the canonical Kubernetes cluster name for drawer fetch keys', async () => {
    render(() => <ResourceDetailDrawer resource={buildKubernetesResource()} />);

    await fireEvent.click(screen.getByRole('tab', { name: 'Namespaces' }));
    await waitFor(() => {
      expect(screen.getByTestId('k8s-namespaces-drawer')).toHaveAttribute(
        'data-cluster',
        'cluster-a',
      );
    });

    await fireEvent.click(screen.getByRole('tab', { name: 'Deployments' }));
    await waitFor(() => {
      expect(screen.getByTestId('k8s-deployments-drawer')).toHaveAttribute(
        'data-cluster',
        'cluster-a',
      );
    });
  });
});
