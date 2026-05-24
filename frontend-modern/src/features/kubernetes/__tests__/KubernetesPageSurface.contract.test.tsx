import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { render, screen } from '@solidjs/testing-library';
import { Route, Router } from '@solidjs/router';

import type { WorkloadsSurfaceProps } from '@/components/Workloads/WorkloadsSurface';
import { KubernetesPageSurface } from '../KubernetesPageSurface';

const mockUseUnifiedResources = vi.fn();
const mockUseWorkloadsState = vi.fn();
const mockPathname = vi.hoisted(() => vi.fn(() => '/'));
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

vi.mock('@solidjs/router', async () => {
  const actual = await vi.importActual<typeof import('@solidjs/router')>('@solidjs/router');
  return {
    ...actual,
    useLocation: () => ({ pathname: mockPathname() }),
  };
});

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

vi.mock('../KubernetesAutoscalingTable', () => ({
  KubernetesAutoscalingTable: () => <div data-testid="autoscaling-table" />,
}));

vi.mock('../KubernetesClustersTable', () => ({
  KubernetesClustersTable: () => <div data-testid="clusters-table" />,
}));

vi.mock('../KubernetesConfigTable', () => ({
  KubernetesConfigTable: () => <div data-testid="config-table" />,
}));

vi.mock('../KubernetesControllersTable', () => ({
  KubernetesControllersTable: () => <div data-testid="controllers-table" />,
}));

vi.mock('../KubernetesNodesTable', () => ({
  KubernetesNodesTable: () => <div data-testid="nodes-table" />,
}));

vi.mock('../KubernetesServicesTable', () => ({
  KubernetesServicesTable: () => <div data-testid="services-table" />,
}));

vi.mock('../KubernetesDeploymentsTable', () => ({
  KubernetesDeploymentsTable: () => <div data-testid="deployments-table" />,
}));

vi.mock('../KubernetesEventsTable', () => ({
  KubernetesEventsTable: () => <div data-testid="events-table" />,
}));

vi.mock('../KubernetesNetworkingTable', () => ({
  KubernetesNetworkingTable: () => <div data-testid="networking-table" />,
}));

vi.mock('../KubernetesPolicyTable', () => ({
  KubernetesPolicyTable: () => <div data-testid="policy-table" />,
}));

vi.mock('../KubernetesStorageTable', () => ({
  KubernetesStorageTable: () => <div data-testid="storage-table" />,
}));

describe('KubernetesPageSurface contract', () => {
  beforeEach(() => {
    mockPathname.mockReturnValue('/');
  });

  afterEach(() => {
    vi.clearAllMocks();
  });

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

  it('promotes Kubernetes node inventory to a dedicated API-native tab', () => {
    mockPathname.mockReturnValue('/kubernetes/nodes');
    mockUseUnifiedResources.mockReturnValue({
      resources: () => [
        {
          id: 'node-1',
          type: 'k8s-node',
          name: 'worker-1',
          displayName: 'worker-1',
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

    expect(screen.getByTestId('nodes-table')).toBeInTheDocument();
    expect(screen.queryByTestId('kubernetes-workloads-surface')).not.toBeInTheDocument();
  });

  it('renders Kubernetes storage inventory through the storage-native table', () => {
    mockPathname.mockReturnValue('/kubernetes/storage');
    mockUseUnifiedResources.mockReturnValue({
      resources: () => [
        {
          id: 'fast-csi',
          type: 'k8s-storage-class',
          name: 'fast-csi',
          displayName: 'fast-csi',
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

    expect(screen.getByTestId('storage-table')).toBeInTheDocument();
    expect(screen.queryByTestId('kubernetes-workloads-surface')).not.toBeInTheDocument();
  });

  it('renders Kubernetes services through the service-native table', () => {
    mockPathname.mockReturnValue('/kubernetes/services');
    mockUseUnifiedResources.mockReturnValue({
      resources: () => [
        {
          id: 'checkout-api',
          type: 'k8s-service',
          name: 'checkout-api',
          displayName: 'checkout-api',
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

    expect(screen.getByTestId('services-table')).toBeInTheDocument();
    expect(screen.queryByTestId('kubernetes-workloads-surface')).not.toBeInTheDocument();
  });

  it('renders Kubernetes networking inventory through the networking-native table', () => {
    mockPathname.mockReturnValue('/kubernetes/networking');
    mockUseUnifiedResources.mockReturnValue({
      resources: () => [
        {
          id: 'checkout-api',
          type: 'k8s-service',
          name: 'checkout-api',
          displayName: 'checkout-api',
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

    expect(screen.getByTestId('networking-table')).toBeInTheDocument();
    expect(screen.queryByTestId('kubernetes-workloads-surface')).not.toBeInTheDocument();
  });

  it('renders Kubernetes config inventory through the config-native table', () => {
    mockPathname.mockReturnValue('/kubernetes/config');
    mockUseUnifiedResources.mockReturnValue({
      resources: () => [
        {
          id: 'checkout-api-config',
          type: 'k8s-configmap',
          name: 'checkout-api-config',
          displayName: 'checkout-api-config',
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

    expect(screen.getByTestId('config-table')).toBeInTheDocument();
    expect(screen.queryByTestId('kubernetes-workloads-surface')).not.toBeInTheDocument();
  });

  it('renders Kubernetes policy inventory through the policy-native table', () => {
    mockPathname.mockReturnValue('/kubernetes/policy');
    mockUseUnifiedResources.mockReturnValue({
      resources: () => [
        {
          id: 'default-deny',
          type: 'k8s-network-policy',
          name: 'default-deny',
          displayName: 'default-deny',
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

    expect(screen.getByTestId('policy-table')).toBeInTheDocument();
    expect(screen.queryByTestId('kubernetes-workloads-surface')).not.toBeInTheDocument();
  });

  it('renders Kubernetes autoscaling inventory through the autoscaling-native table', () => {
    mockPathname.mockReturnValue('/kubernetes/autoscaling');
    mockUseUnifiedResources.mockReturnValue({
      resources: () => [
        {
          id: 'checkout-api-hpa',
          type: 'k8s-horizontal-pod-autoscaler',
          name: 'checkout-api-hpa',
          displayName: 'checkout-api-hpa',
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

    expect(screen.getByTestId('autoscaling-table')).toBeInTheDocument();
    expect(screen.queryByTestId('kubernetes-workloads-surface')).not.toBeInTheDocument();
  });

  it('renders Kubernetes events through the events-native table', () => {
    mockPathname.mockReturnValue('/kubernetes/events');
    mockUseUnifiedResources.mockReturnValue({
      resources: () => [
        {
          id: 'event-1',
          type: 'k8s-event',
          name: 'event-1',
          displayName: 'event-1',
          platformId: 'cluster-1',
          platformType: 'kubernetes',
          sourceType: 'agent',
          sources: ['kubernetes'],
          status: 'degraded',
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

    expect(screen.getByTestId('events-table')).toBeInTheDocument();
    expect(screen.queryByTestId('kubernetes-workloads-surface')).not.toBeInTheDocument();
  });
});
