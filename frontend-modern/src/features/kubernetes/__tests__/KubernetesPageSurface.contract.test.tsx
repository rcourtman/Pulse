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

  it('declares Pods, Deployments, and Controllers as API-native workload tabs', () => {
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
        query: expect.stringContaining('k8s-statefulset'),
      }),
    );
    expect(screen.getByTestId('platform-section-tabs')).toHaveAttribute(
      'data-tabs',
      'overview,nodes,pods,deployments,controllers,services,storage,networking,config,policy,autoscaling,events',
    );
    expect(screen.getByTestId('platform-section-tabs').getAttribute('data-tabs')).not.toContain(
      'workloads',
    );
    expect(screen.getByTestId('pods-table')).toHaveAttribute('data-rows', '1');
    expect(screen.getByTestId('deployments-table')).toHaveAttribute('data-rows', '1');
    expect(screen.getByTestId('controllers-table')).toHaveAttribute('data-rows', '1');
  });

  it('renders Kubernetes pods through the pod-native table', () => {
    mockPathname.mockReturnValue('/kubernetes/pods');
    setResources([makeResource({ id: 'checkout-api', type: 'pod' })]);

    renderSurface();

    expect(screen.getByTestId('platform-section-tabs')).toHaveAttribute('data-active', 'pods');
    expect(screen.getByTestId('pods-table')).toHaveAttribute('data-rows', '1');
    expect(screen.queryByTestId('deployments-table')).not.toBeInTheDocument();
  });

  it('maps the legacy Kubernetes workloads route to the pod-native tab', () => {
    mockPathname.mockReturnValue('/kubernetes/workloads');
    setResources([makeResource({ id: 'checkout-api', type: 'pod' })]);

    renderSurface();

    expect(screen.getByTestId('platform-section-tabs')).toHaveAttribute('data-active', 'pods');
    expect(screen.getByTestId('pods-table')).toHaveAttribute('data-rows', '1');
  });

  it('renders Kubernetes deployments through the deployment-native table', () => {
    mockPathname.mockReturnValue('/kubernetes/deployments');
    setResources([makeResource({ id: 'checkout-api', type: 'k8s-deployment' })]);

    renderSurface();

    expect(screen.getByTestId('deployments-table')).toHaveAttribute('data-rows', '1');
    expect(screen.queryByTestId('pods-table')).not.toBeInTheDocument();
  });

  it('renders Kubernetes workload controllers through the controller-native table', () => {
    mockPathname.mockReturnValue('/kubernetes/controllers');
    setResources([
      makeResource({ id: 'checkout-stateful', type: 'k8s-statefulset' }),
      makeResource({ id: 'node-exporter', type: 'k8s-daemonset' }),
      makeResource({ id: 'nightly-import', type: 'k8s-job' }),
      makeResource({ id: 'billing-rollup', type: 'k8s-cronjob' }),
      makeResource({ id: 'deployment-1', type: 'k8s-deployment' }),
    ]);

    renderSurface();

    expect(screen.getByTestId('controllers-table')).toHaveAttribute('data-rows', '4');
    expect(screen.queryByTestId('deployments-table')).not.toBeInTheDocument();
  });

  it.each([
    ['/kubernetes/nodes', 'k8s-node', 'nodes-table'],
    ['/kubernetes/services', 'k8s-service', 'services-table'],
    ['/kubernetes/storage', 'k8s-storage-class', 'storage-table'],
    ['/kubernetes/networking', 'k8s-ingress', 'networking-table'],
    ['/kubernetes/config', 'k8s-configmap', 'config-table'],
    ['/kubernetes/policy', 'k8s-network-policy', 'policy-table'],
    ['/kubernetes/autoscaling', 'k8s-horizontal-pod-autoscaler', 'autoscaling-table'],
    ['/kubernetes/events', 'k8s-event', 'events-table'],
  ] as const)('renders %s through its API-native table', (path, type, testId) => {
    mockPathname.mockReturnValue(path);
    setResources([makeResource({ id: `${type}-1`, type })]);

    renderSurface();

    expect(screen.getByTestId(testId)).toHaveAttribute('data-rows', '1');
  });
});
