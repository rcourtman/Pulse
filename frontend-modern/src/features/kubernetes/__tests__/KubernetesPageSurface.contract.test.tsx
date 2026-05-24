import { describe, expect, it, vi } from 'vitest';
import { render, screen } from '@solidjs/testing-library';
import { Route, Router } from '@solidjs/router';

import type { WorkloadsSurfaceProps } from '@/components/Workloads/WorkloadsSurface';
import { KubernetesPageSurface } from '../KubernetesPageSurface';

const mockUseUnifiedResources = vi.fn();
const mockUseWorkloadsState = vi.fn();
const mockWorkloadsSurface = vi.fn((props: WorkloadsSurfaceProps) => (
  <div
    data-testid="kubernetes-workloads-surface"
    data-forced-platform={props.forcedPlatform}
    data-forced-view-mode={props.forcedViewMode}
    data-column-scope={props.columnVisibilityStorageScope}
    data-allow-scope-filters={String(props.allowEmbeddedScopeFilters)}
    data-filter-placeholder={props.filterSearchPlaceholder}
  />
));

vi.mock('@/hooks/useUnifiedResources', () => ({
  useUnifiedResources: (...args: unknown[]) => mockUseUnifiedResources(...args),
}));

vi.mock('@/components/Workloads/useWorkloadsState', () => ({
  useWorkloadsState: (...args: unknown[]) => mockUseWorkloadsState(...args),
}));

vi.mock('@/components/Workloads/WorkloadsSurface', () => ({
  WorkloadsSurface: (props: WorkloadsSurfaceProps) => mockWorkloadsSurface(props),
}));

vi.mock('@/features/platformPage/sharedPlatformPage', () => ({
  PlatformErrorState: () => <div data-testid="platform-error-state" />,
  PlatformSectionTabs: () => <div data-testid="platform-section-tabs" />,
  PlatformTableEmptyState: () => <div data-testid="platform-table-empty-state" />,
  PlatformTableLoadingState: () => <div data-testid="platform-table-loading-state" />,
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
    mockUseWorkloadsState.mockReturnValue({
      allGuests: () => [],
      clearPinnedSummaryScope: vi.fn(),
      containerRuntimeFilterConfig: () => undefined,
      focusedSummaryWorkloadGroupId: () => null,
      groupingMode: () => 'grouped',
      handleBeforeAutoFocus: vi.fn(),
      search: () => '',
      selectedGuestId: () => null,
      setGroupingMode: vi.fn(),
      setMetricDisplayMode: vi.fn(),
      setSearch: vi.fn(),
      setSortDirection: vi.fn(),
      setSortKey: vi.fn(),
      setStatusMode: vi.fn(),
      setViewMode: vi.fn(),
      setWorkloadMetricHistoryRange: vi.fn(),
      statusMode: () => 'all',
      surfaceConnected: () => false,
      surfaceInitialDataReceived: () => false,
      viewMode: () => 'pod',
      workloadMetricDisplayMode: () => 'bars',
      workloadMetricHistoryRange: () => '1h',
      workloadsFilterColumnVisibility: () => undefined,
    });

    render(() => (
      <Router>
        <Route path="/" component={KubernetesPageSurface} />
      </Router>
    ));

    expect(mockUseUnifiedResources).toHaveBeenCalledWith(
      expect.objectContaining({
        query: expect.stringContaining('k8s-secret'),
      }),
    );
    expect(mockUseUnifiedResources).toHaveBeenCalledWith(
      expect.objectContaining({
        query: expect.stringContaining('k8s-horizontal-pod-autoscaler'),
      }),
    );
    expect(screen.getByTestId('kubernetes-workloads-surface')).toHaveAttribute(
      'data-forced-platform',
      'kubernetes',
    );
    expect(screen.getByTestId('kubernetes-workloads-surface')).toHaveAttribute(
      'data-forced-view-mode',
      'pod',
    );
    expect(mockUseWorkloadsState).toHaveBeenCalledWith(
      expect.objectContaining({
        allowEmbeddedScopeFilters: true,
        columnVisibilityStorageScope: 'kubernetes-pods',
        forcedPlatform: 'kubernetes',
        forcedViewMode: 'pod',
      }),
    );
  });
});
