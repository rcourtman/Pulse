import { Route, Router } from '@solidjs/router';
import { cleanup, render, screen } from '@solidjs/testing-library';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import type { Resource } from '@/types/resource';
import { KubernetesPageSurface } from '../KubernetesPageSurface';

const mockUseUnifiedResources = vi.fn();
const mockPathname = vi.hoisted(() => vi.fn(() => '/'));

const makeResource = (resource: Partial<Resource> & Pick<Resource, 'id' | 'type'>): Resource => ({
  name: resource.id,
  displayName: resource.id,
  platformId: 'cluster-1',
  platformType: 'kubernetes',
  sourceType: 'agent',
  sources: ['kubernetes'],
  status: 'online',
  lastSeen: 1_700_000_000_000,
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

const renderSurface = () =>
  render(() => (
    <Router>
      <Route path="/" component={KubernetesPageSurface} />
    </Router>
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

vi.mock('@/features/platformPage/sharedPlatformPage', () => ({
  PLATFORM_HEALTH_FILTER_OPTIONS: [],
  PlatformErrorState: () => <div data-testid="platform-error-state" />,
  PlatformSectionTabs: (props: {
    active: string;
    tabs: Array<{ id: string; label: string; path: string }>;
  }) => (
    <div
      data-testid="platform-section-tabs"
      data-active={props.active}
      data-tabs={props.tabs.map((tab) => tab.id).join(',')}
    />
  ),
  PlatformTableEmptyState: () => <div data-testid="platform-table-empty-state" />,
  PlatformTableLoadingState: () => <div data-testid="platform-table-loading-state" />,
  PlatformTableToolbar: () => <div data-testid="platform-table-toolbar" />,
}));

vi.mock('../KubernetesAlertsTable', () => ({
  KubernetesAlertsTable: (props: { incidents: unknown[] }) => (
    <div data-testid="alerts-table" data-rows={props.incidents.length} />
  ),
}));

vi.mock('../KubernetesAutoscalingTable', () => ({
  KubernetesAutoscalingTable: (props: { resources: Resource[] }) => (
    <div data-testid="autoscaling-table" data-rows={props.resources.length} />
  ),
}));

vi.mock('../KubernetesClustersTable', () => ({
  KubernetesClustersTable: (props: { clusters: Resource[] }) => (
    <div data-testid="clusters-table" data-rows={props.clusters.length} />
  ),
}));

vi.mock('../KubernetesConfigTable', () => ({
  KubernetesConfigTable: (props: { resources: Resource[] }) => (
    <div data-testid="config-table" data-rows={props.resources.length} />
  ),
}));

vi.mock('../KubernetesControllersTable', () => ({
  KubernetesControllersTable: (props: { resources: Resource[] }) => (
    <div data-testid="controllers-table" data-rows={props.resources.length} />
  ),
}));

vi.mock('../KubernetesDeploymentsTable', () => ({
  KubernetesDeploymentsTable: (props: { resources: Resource[] }) => (
    <div data-testid="deployments-table" data-rows={props.resources.length} />
  ),
}));

vi.mock('../KubernetesEventsTable', () => ({
  KubernetesEventsTable: (props: { resources: Resource[] }) => (
    <div data-testid="events-table" data-rows={props.resources.length} />
  ),
}));

vi.mock('../KubernetesNetworkingTable', () => ({
  KubernetesNetworkingTable: (props: { resources: Resource[] }) => (
    <div data-testid="networking-table" data-rows={props.resources.length} />
  ),
}));

vi.mock('../KubernetesNodesTable', () => ({
  KubernetesNodesTable: (props: { resources: Resource[] }) => (
    <div data-testid="nodes-table" data-rows={props.resources.length} />
  ),
}));

vi.mock('../KubernetesPodsTable', () => ({
  KubernetesPodsTable: (props: { resources: Resource[] }) => (
    <div data-testid="pods-table" data-rows={props.resources.length} />
  ),
}));

vi.mock('../KubernetesPolicyTable', () => ({
  KubernetesPolicyTable: (props: { resources: Resource[] }) => (
    <div data-testid="policy-table" data-rows={props.resources.length} />
  ),
}));

vi.mock('../KubernetesServicesTable', () => ({
  KubernetesServicesTable: (props: { resources: Resource[] }) => (
    <div data-testid="services-table" data-rows={props.resources.length} />
  ),
}));

vi.mock('../KubernetesStorageTable', () => ({
  KubernetesStorageTable: (props: { resources: Resource[] }) => (
    <div data-testid="storage-table" data-rows={props.resources.length} />
  ),
}));

describe('KubernetesPageSurface contract', () => {
  beforeEach(() => {
    mockPathname.mockReturnValue('/');
  });

  afterEach(() => {
    cleanup();
    vi.clearAllMocks();
  });

  it('declares workflow tabs while querying API-native Kubernetes resources', () => {
    setResources([
      makeResource({ id: 'cluster-1', type: 'k8s-cluster' }),
      makeResource({ id: 'pod-1', type: 'pod' }),
      makeResource({ id: 'deployment-1', type: 'k8s-deployment' }),
      makeResource({ id: 'statefulset-1', type: 'k8s-statefulset' }),
    ]);

    renderSurface();

    expect(mockUseUnifiedResources).toHaveBeenCalledWith(
      expect.objectContaining({
        query: expect.stringContaining('pod'),
      }),
    );
    expect(mockUseUnifiedResources).toHaveBeenCalledWith(
      expect.objectContaining({
        query: expect.stringContaining('k8s-deployment'),
      }),
    );
    expect(mockUseUnifiedResources).toHaveBeenCalledWith(
      expect.objectContaining({
        query: expect.stringContaining('k8s-replicaset'),
      }),
    );
    expect(mockUseUnifiedResources).toHaveBeenCalledWith(
      expect.objectContaining({
        query: expect.stringContaining('k8s-statefulset'),
      }),
    );
    expect(screen.getByTestId('platform-section-tabs')).toHaveAttribute(
      'data-tabs',
      'overview,nodes,workloads,services,storage,configuration,events',
    );
    expect(screen.getByTestId('platform-section-tabs')).toHaveAttribute('data-active', 'overview');
    expect(screen.getByTestId('clusters-table')).toHaveAttribute('data-rows', '1');
    expect(screen.queryByTestId('pods-table')).toBeNull();
    expect(screen.queryByTestId('deployments-table')).toBeNull();
    expect(screen.queryByTestId('controllers-table')).toBeNull();
  });

  it('groups workload API tables under the Workloads tab', () => {
    mockPathname.mockReturnValue('/kubernetes/workloads');
    setResources([
      makeResource({ id: 'checkout-api', type: 'pod' }),
      makeResource({ id: 'checkout-deployment', type: 'k8s-deployment' }),
      makeResource({ id: 'checkout-replicaset', type: 'k8s-replicaset' }),
      makeResource({ id: 'checkout-stateful', type: 'k8s-statefulset' }),
      makeResource({ id: 'checkout-hpa', type: 'k8s-horizontal-pod-autoscaler' }),
    ]);

    renderSurface();

    expect(screen.getByTestId('platform-section-tabs')).toHaveAttribute('data-active', 'workloads');
    expect(screen.getByTestId('pods-table')).toHaveAttribute('data-rows', '1');
    expect(screen.getByTestId('deployments-table')).toHaveAttribute('data-rows', '1');
    expect(screen.getByTestId('controllers-table')).toHaveAttribute('data-rows', '2');
    expect(screen.getByTestId('autoscaling-table')).toHaveAttribute('data-rows', '1');
  });

  it('routes ReplicaSets to the rendered workload controllers table', () => {
    mockPathname.mockReturnValue('/kubernetes/workloads');
    setResources([makeResource({ id: 'checkout-replicaset', type: 'k8s-replicaset' })]);

    renderSurface();

    expect(screen.getByTestId('controllers-table')).toHaveAttribute('data-rows', '1');
    expect(screen.queryByTestId('pods-table')).toBeNull();
    expect(screen.queryByTestId('deployments-table')).toBeNull();
  });

  it('groups Services, ingress, and endpoints under the Services tab without duplicating services in networking', () => {
    mockPathname.mockReturnValue('/kubernetes/services');
    setResources([
      makeResource({ id: 'checkout-service', type: 'k8s-service' }),
      makeResource({ id: 'checkout-ingress', type: 'k8s-ingress' }),
      makeResource({ id: 'checkout-endpoints', type: 'k8s-endpoint-slice' }),
    ]);

    renderSurface();

    expect(screen.getByTestId('platform-section-tabs')).toHaveAttribute('data-active', 'services');
    expect(screen.getByTestId('services-table')).toHaveAttribute('data-rows', '1');
    expect(screen.getByTestId('networking-table')).toHaveAttribute('data-rows', '2');
  });

  it('groups config and policy API tables under the Configuration tab', () => {
    mockPathname.mockReturnValue('/kubernetes/configuration');
    setResources([
      makeResource({ id: 'checkout-config', type: 'k8s-configmap' }),
      makeResource({ id: 'checkout-policy', type: 'k8s-network-policy' }),
    ]);

    renderSurface();

    expect(screen.getByTestId('platform-section-tabs')).toHaveAttribute(
      'data-active',
      'configuration',
    );
    expect(screen.getByTestId('config-table')).toHaveAttribute('data-rows', '1');
    expect(screen.getByTestId('policy-table')).toHaveAttribute('data-rows', '1');
  });

  it.each([
    ['/kubernetes/nodes', 'k8s-node', 'nodes', 'nodes-table'],
    ['/kubernetes/pods', 'pod', 'workloads', 'pods-table'],
    ['/kubernetes/deployments', 'k8s-deployment', 'workloads', 'deployments-table'],
    ['/kubernetes/controllers', 'k8s-statefulset', 'workloads', 'controllers-table'],
    ['/kubernetes/autoscaling', 'k8s-horizontal-pod-autoscaler', 'workloads', 'autoscaling-table'],
    ['/kubernetes/services', 'k8s-service', 'services', 'services-table'],
    ['/kubernetes/networking', 'k8s-ingress', 'services', 'networking-table'],
    ['/kubernetes/storage', 'k8s-storage-class', 'storage', 'storage-table'],
    ['/kubernetes/config', 'k8s-configmap', 'configuration', 'config-table'],
    ['/kubernetes/policy', 'k8s-network-policy', 'configuration', 'policy-table'],
    ['/kubernetes/events', 'k8s-event', 'events', 'events-table'],
  ] as const)(
    'maps %s to the %s workflow and renders its API-native table',
    (path, type, expectedActive, testId) => {
      mockPathname.mockReturnValue(path);
      setResources([makeResource({ id: `${type}-1`, type })]);

      renderSurface();

      expect(screen.getByTestId('platform-section-tabs')).toHaveAttribute(
        'data-active',
        expectedActive,
      );
      expect(screen.getByTestId(testId)).toHaveAttribute('data-rows', '1');
    },
  );
});
