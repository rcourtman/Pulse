import { describe, expect, it, vi } from 'vitest';
import { render, screen } from '@solidjs/testing-library';

import type { WorkloadsSurfaceProps } from '@/components/Workloads/WorkloadsSurface';
import { KubernetesPageSurface } from '../KubernetesPageSurface';

const mockUseUnifiedResources = vi.fn();
const mockWorkloadsSurface = vi.fn((props: WorkloadsSurfaceProps) => (
  <div
    data-testid="kubernetes-workloads-surface"
    data-forced-platform={props.forcedPlatform}
    data-forced-view-mode={props.forcedViewMode}
    data-column-scope={props.columnVisibilityStorageScope}
  />
));

vi.mock('@/hooks/useUnifiedResources', () => ({
  useUnifiedResources: (...args: unknown[]) => mockUseUnifiedResources(...args),
}));

vi.mock('@/components/Workloads/WorkloadsSurface', () => ({
  WorkloadsSurface: (props: WorkloadsSurfaceProps) => mockWorkloadsSurface(props),
}));

vi.mock('@/features/platformPage/sharedPlatformPage', () => ({
  PlatformErrorState: () => <div data-testid="platform-error-state" />,
  PlatformSectionTabs: () => <div data-testid="platform-section-tabs" />,
  PlatformTableEmptyState: () => <div data-testid="platform-table-empty-state" />,
}));

vi.mock('../KubernetesClustersTable', () => ({
  KubernetesClustersTable: () => <div data-testid="clusters-table" />,
}));

vi.mock('../KubernetesNodesTable', () => ({
  KubernetesNodesTable: () => <div data-testid="nodes-table" />,
}));

vi.mock('../KubernetesDeploymentsTable', () => ({
  KubernetesDeploymentsTable: () => <div data-testid="deployments-table" />,
}));

describe('KubernetesPageSurface contract', () => {
  it('pins the embedded workloads surface to the pod-native Kubernetes columns', () => {
    mockUseUnifiedResources.mockReturnValue({
      resources: () => [
        {
          id: 'cluster-1',
          type: 'k8s-cluster',
          name: 'cluster-1',
          displayName: 'cluster-1',
          platformId: 'cluster-1',
          platformType: 'kubernetes',
          sourceType: 'agent',
          sources: ['kubernetes'],
          status: 'online',
          lastSeen: 1_700_000_000_000,
        },
      ],
      loading: () => false,
      error: () => null,
      refetch: vi.fn(),
    });

    render(() => <KubernetesPageSurface />);

    expect(screen.getByTestId('kubernetes-workloads-surface')).toHaveAttribute(
      'data-forced-platform',
      'kubernetes',
    );
    expect(screen.getByTestId('kubernetes-workloads-surface')).toHaveAttribute(
      'data-forced-view-mode',
      'pod',
    );
    expect(screen.getByTestId('kubernetes-workloads-surface')).toHaveAttribute(
      'data-column-scope',
      'kubernetes-pods',
    );
  });
});
