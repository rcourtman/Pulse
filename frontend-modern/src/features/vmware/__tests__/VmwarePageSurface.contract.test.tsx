import { createSignal, type JSX } from 'solid-js';
import { cleanup, fireEvent, render, screen, within } from '@solidjs/testing-library';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import type { Resource } from '@/types/resource';
import { workloadNodeScopeId } from '@/components/Workloads/workloadTopology';
import { VmwarePageSurface } from '../VmwarePageSurface';
import vmwarePageSurfaceSource from '../VmwarePageSurface.tsx?raw';

const mockUseUnifiedResources = vi.fn();
const mockPathname = vi.hoisted(() => vi.fn(() => '/vmware/overview'));
const mockVersionInfo = vi.hoisted(() => vi.fn());
const mockGetGlobalTimeline = vi.hoisted(() => vi.fn());
const mockListInventorySources = vi.hoisted(() => vi.fn());
const mockUseWorkloadsState = vi.hoisted(() => vi.fn());

const makeResource = (resource: Partial<Resource> & Pick<Resource, 'id' | 'type'>): Resource =>
  ({
    name: resource.id,
    displayName: resource.id,
    platformId: 'vmware-1',
    platformType: 'vmware-vsphere',
    sourceType: 'api',
    status: 'online',
    lastSeen: 1_700_000_000_000,
    ...resource,
  }) as Resource;

const setResources = (
  resources: Resource[],
  options: { loading?: boolean; refetch?: ReturnType<typeof vi.fn> } = {},
) => {
  const refetch = options.refetch ?? vi.fn().mockResolvedValue(resources);
  mockUseUnifiedResources.mockReturnValue({
    resources: () => resources,
    loading: () => options.loading ?? false,
    error: () => null,
    refetch,
  });
  return refetch;
};

vi.mock('@/hooks/useUnifiedResources', () => ({
  useUnifiedResources: (...args: unknown[]) => mockUseUnifiedResources(...args),
}));

vi.mock('@/hooks/usePersistentSignal', () => ({
  usePersistentSignal: (_key: string, initial: unknown) => [() => initial, vi.fn()],
}));

vi.mock('@/stores/updates', () => ({
  updateStore: {
    versionInfo: mockVersionInfo,
  },
}));

vi.mock('@/api/resources', () => ({
  ResourceAPI: {
    getGlobalTimeline: (...args: unknown[]) => mockGetGlobalTimeline(...args),
  },
}));

vi.mock('@/api/runtimeInventorySources', () => ({
  RuntimeInventorySourcesAPI: {
    list: (...args: unknown[]) => mockListInventorySources(...args),
  },
}));

vi.mock('@solidjs/router', async () => {
  const actual = await vi.importActual<typeof import('@solidjs/router')>('@solidjs/router');
  return {
    ...actual,
    useLocation: () => ({ pathname: mockPathname() }),
  };
});

vi.mock('@/components/Workloads/WorkloadsFilter', () => ({
  WorkloadsFilter: () => <div data-testid="workloads-filter" />,
}));

vi.mock('@/components/Workloads/WorkloadsSurface', () => ({
  WorkloadsSurface: (props: { tableTitle?: JSX.Element }) => (
    <div data-testid="workloads-surface">{props.tableTitle}</div>
  ),
}));

vi.mock('@/components/Workloads/useWorkloadsState', () => ({
  // No host is picked unless a test says so.
  useWorkloadsState: (...args: unknown[]) => ({
    selectedNode: () => null,
    selectedHostHint: () => null,
    handleNodeSelect: vi.fn(),
    hostFilterConfig: () => undefined,
    ...mockUseWorkloadsState(...args),
  }),
}));

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

vi.mock('../VsphereActivityTable', () => ({
  VsphereActivityTable: () => <div data-testid="activity-table" />,
}));

vi.mock('../VsphereAlertsTable', () => ({
  VsphereAlertsTable: () => <div data-testid="alerts-table" />,
}));

vi.mock('../VsphereDatastoresTable', () => ({
  VsphereDatastoresTable: () => <div data-testid="datastores-table" />,
}));

vi.mock('../VsphereHostsTable', () => ({
  VsphereHostsTable: (props: {
    hosts: Resource[];
    vmScope?: {
      sectionId: string;
      selectedHostId: () => string | null;
      onShow: (host: Resource) => void;
    };
  }) => (
    <div data-testid="hosts-table" data-rows={props.hosts.length}>
      {props.hosts.map((host) => (
        <button
          type="button"
          data-scope-section={props.vmScope?.sectionId}
          aria-pressed={props.vmScope?.selectedHostId() === host.id}
          onClick={() => props.vmScope?.onShow(host)}
        >
          {`host ${host.id}`}
        </button>
      ))}
    </div>
  ),
}));

vi.mock('../VsphereNetworksTable', () => ({
  VsphereNetworksTable: () => <div data-testid="networks-table" />,
}));

describe('VmwarePageSurface contract', () => {
  beforeEach(() => {
    mockPathname.mockReturnValue('/vmware/overview');
    mockVersionInfo.mockReturnValue(null);
    mockGetGlobalTimeline.mockResolvedValue({ recentChanges: [] });
    mockListInventorySources.mockResolvedValue({ sources: [] });
    mockUseWorkloadsState.mockReturnValue({
      surfaceConnected: () => false,
      surfaceInitialDataReceived: () => false,
      allGuests: () => [],
      filteredGuests: () => [],
      search: () => '',
      setSearch: vi.fn(),
    });
    setResources([]);
  });

  afterEach(() => {
    cleanup();
    vi.clearAllMocks();
  });

  it('uses the complete canonical workload metric-control binding', () => {
    expect(vmwarePageSurfaceSource).toContain('{...getWorkloadsMetricFilterProps(workloadsState)}');
    expect(vmwarePageSurfaceSource).not.toContain(
      'metricDisplayMode={workloadsState.workloadMetricDisplayMode}',
    );
  });

  it('renders the one workload toolbar above the VM table once workload state is ready', () => {
    setResources([makeResource({ id: 'vm-app-01', type: 'vm' })]);

    render(() => <VmwarePageSurface />);
    expect(screen.queryByTestId('workloads-filter')).not.toBeInTheDocument();
    cleanup();

    mockUseWorkloadsState.mockReturnValue({
      surfaceConnected: () => true,
      surfaceInitialDataReceived: () => true,
      allGuests: () => [{ id: 'vm-app-01' }],
      filteredGuests: () => [{ id: 'vm-app-01' }],
      search: () => '',
      setSearch: vi.fn(),
    });
    render(() => <VmwarePageSurface />);
    expect(screen.getAllByTestId('workloads-filter')).toHaveLength(1);
    expect(
      screen
        .getByTestId('workloads-filter')
        .compareDocumentPosition(screen.getByTestId('workloads-surface')) &
        Node.DOCUMENT_POSITION_FOLLOWING,
    ).toBeTruthy();
  });

  it('narrows the VM table to a host picked in the hosts table, as Proxmox does', () => {
    setResources([
      makeResource({
        id: 'esxi-01',
        type: 'agent',
        vmware: {
          entityType: 'host',
          managedObjectId: 'host-1',
          connectionId: 'vc-1',
          clusterName: 'Production',
        },
      }),
      makeResource({
        id: 'vm-app-01',
        type: 'vm',
        vmware: {
          runtimeHostId: 'host-1',
          runtimeHostName: 'esxi-01',
          connectionId: 'vc-1',
          clusterName: 'Production',
        },
      }),
    ]);
    const [selected, setSelected] = createSignal<string | null>(null);
    const handleNodeSelect = vi.fn((scope: string | null) => setSelected(scope));
    mockUseWorkloadsState.mockReturnValue({
      surfaceConnected: () => true,
      surfaceInitialDataReceived: () => true,
      // The workload table's own placement for the host's VM.
      allGuests: () => [
        { id: 'vm-app-01', node: 'esxi-01.lab', instance: 'Production Cluster' },
        { id: 'vm-app-02', node: 'esxi-02.lab', instance: 'Production Cluster' },
      ],
      filteredGuests: () => [{ id: 'vm-app-01' }],
      search: () => '',
      setSearch: vi.fn(),
      selectedNode: selected,
      handleNodeSelect,
    });

    render(() => <VmwarePageSurface />);
    const hostButton = () => screen.getByRole('button', { name: 'host esxi-01' });
    const heading = () =>
      within(screen.getByTestId('workloads-surface')).getByRole('heading', { level: 2 });
    expect(hostButton()).toHaveAttribute('data-scope-section', 'vsphere-vms-section');
    expect(screen.getByTestId('vsphere-vms-section')).toContainElement(
      screen.getByTestId('workloads-surface'),
    );

    // The scope is the one the table gave the host's own VM.
    fireEvent.click(hostButton());
    expect(handleNodeSelect).toHaveBeenLastCalledWith(
      workloadNodeScopeId({ instance: 'Production Cluster', node: 'esxi-01.lab' }),
      'pve',
    );
    expect(hostButton()).toHaveAttribute('aria-pressed', 'true');
    expect(heading()).toHaveTextContent(/^VMs on esxi-01\s*1$/);

    // A second click on the host, or Show all hosts, clears it.
    fireEvent.click(hostButton());
    expect(handleNodeSelect).toHaveBeenLastCalledWith(null, null);
    fireEvent.click(hostButton());
    fireEvent.click(screen.getByRole('button', { name: 'Show all hosts' }));
    expect(handleNodeSelect).toHaveBeenLastCalledWith(null, null);
    expect(heading()).toHaveTextContent(/^VMs\s*1$/);
  });

  it('titles the VM table through the tableTitle slot and counts the filtered rows', () => {
    setResources([makeResource({ id: 'vm-app-01', type: 'vm' })]);
    const [ready, setReady] = createSignal(false);
    const [filtered, setFiltered] = createSignal([{ id: 'vm-app-01' }, { id: 'vm-app-02' }]);
    mockUseWorkloadsState.mockReturnValue({
      surfaceConnected: ready,
      surfaceInitialDataReceived: ready,
      allGuests: () => [{ id: 'vm-app-01' }, { id: 'vm-app-02' }, { id: 'vm-app-03' }],
      filteredGuests: filtered,
      search: () => 'app',
      setSearch: vi.fn(),
    });

    // The mocked surface renders the slot unconditionally; the real one renders
    // it with the table or the empty card. Kiosk unmounts the filter bar that
    // otherwise introduces the table, so the title has to carry the name.
    render(() => <VmwarePageSurface />);
    const heading = () =>
      within(screen.getByTestId('workloads-surface')).getByRole('heading', { level: 2 });
    expect(heading()).toHaveTextContent(/^VMs$/);

    // Once the toolbar is ready the count is the filtered rows (2), not the
    // inventory (3), and it keeps following them.
    setReady(true);
    expect(heading()).toHaveTextContent(/^VMs\s*2$/);
    setFiltered([{ id: 'vm-app-01' }]);
    expect(heading()).toHaveTextContent(/^VMs\s*1$/);
    setFiltered([]);
    expect(heading()).toHaveTextContent(/^VMs\s*0$/);
    expect(screen.getAllByRole('heading', { level: 2 })).toHaveLength(1);
  });

  it('surfaces stale in-guest agents on correlated vSphere VMs', () => {
    mockVersionInfo.mockReturnValue({
      version: 'v6.0.0-rc.6',
      agentUpdateTargetVersion: 'v6.0.0-rc.6',
    });
    setResources([
      makeResource({
        id: 'esxi-host-1',
        name: 'esxi-host-1',
        displayName: 'esxi-host-1',
        type: 'agent',
        agent: { osVersion: 'VMware ESXi 8.0.3' },
        vmware: { entityType: 'host', managedObjectId: 'host-1' },
      }),
      makeResource({
        id: 'vm-app-01',
        name: 'app-01',
        displayName: 'app-01',
        type: 'vm',
        agent: { agentId: 'agent-app-01', agentVersion: 'v5.1.34' },
        vmware: { entityType: 'vm', managedObjectId: 'vm-1', runtimeHostName: 'esxi-host-1' },
      }),
    ]);

    render(() => <VmwarePageSurface />);

    expect(screen.getByTestId('platform-section-tabs')).toHaveAttribute('data-active', 'overview');
    expect(screen.getByTestId('hosts-table')).toHaveAttribute('data-rows', '1');
    const notice = screen.getByTestId('platform-outdated-agent-notice');
    expect(notice).toHaveTextContent('app-01 runs an older Pulse agent (v5.1.34).');
    expect(notice).toHaveTextContent('latest fixes and VM details.');
    expect(screen.getByRole('link', { name: 'Open agent upgrade commands' })).toHaveAttribute(
      'href',
      '/settings/infrastructure/agent-doctor?agents=agent%3Aagent-app-01',
    );
  });

  it('leads the overview with vCenter health signals only when vCenter reports any', () => {
    const host = makeResource({
      id: 'esxi-host-1',
      type: 'agent',
      vmware: { entityType: 'host', managedObjectId: 'host-1' },
    });
    const alarmedDatastore = makeResource({
      id: 'datastore-1',
      type: 'storage',
      vmware: { entityType: 'datastore', managedObjectId: 'datastore-1' },
      incidents: [{ code: 'vmware_alarm', severity: 'warning', summary: 'Datastore latency' }],
    } as Partial<Resource> & Pick<Resource, 'id' | 'type'>);
    // Each query gets its own fixture so the test proves the signals come
    // from the Health query while the overview model keeps its own resources.
    const byCacheKey = (healthResources: Resource[]) =>
      mockUseUnifiedResources.mockImplementation((options: { cacheKey?: string }) => {
        const resources = options.cacheKey === 'vmware-health' ? healthResources : [host];
        return {
          resources: () => resources,
          loading: () => false,
          error: () => null,
          refetch: vi.fn().mockResolvedValue(resources),
        };
      });

    byCacheKey([host]);
    const { unmount } = render(() => <VmwarePageSurface />);
    expect(screen.queryByTestId('alerts-table')).not.toBeInTheDocument();
    unmount();

    byCacheKey([host, alarmedDatastore]);
    render(() => <VmwarePageSurface />);
    expect(screen.getByTestId('platform-section-tabs')).toHaveAttribute('data-active', 'overview');
    expect(screen.getByTestId('alerts-table')).toBeInTheDocument();
    expect(screen.getByTestId('hosts-table')).toHaveAttribute('data-rows', '1');
    expect(
      mockUseUnifiedResources.mock.calls.some(
        ([options]) =>
          (options as { cacheKey?: string }).cacheKey === 'vmware-health' &&
          (options as { enabled: () => boolean }).enabled(),
      ),
    ).toBe(true);
  });

  it('reuses the source-scoped page snapshot for the vSphere workload surface', async () => {
    const refetch = vi.fn().mockResolvedValue([]);
    const resources = [
      makeResource({
        id: 'esxi-host-1',
        type: 'agent',
        vmware: { entityType: 'host', managedObjectId: 'host-1' },
      }),
      makeResource({
        id: 'vm-app-01',
        type: 'vm',
        vmware: { entityType: 'vm', managedObjectId: 'vm-1' },
      }),
      makeResource({
        id: 'other-platform-vm',
        type: 'vm',
        platformType: 'proxmox-pve',
        vmware: undefined,
      }),
    ];
    setResources(resources, { refetch });

    render(() => <VmwarePageSurface />);

    expect(mockUseUnifiedResources).toHaveBeenCalledWith(
      expect.objectContaining({
        query: 'type=agent,vm&source=vmware-vsphere',
        cacheKey: 'vmware-overview',
      }),
    );
    const stateOptions = mockUseWorkloadsState.mock.calls[0]?.[0] as {
      resourceSnapshot: () => Resource[] | undefined;
      resourceSnapshotRefetch: () => Promise<unknown>;
    };
    expect(stateOptions.resourceSnapshot()?.map((resource) => resource.id)).toEqual([
      'esxi-host-1',
      'vm-app-01',
    ]);

    await stateOptions.resourceSnapshotRefetch();
    expect(refetch).toHaveBeenCalledTimes(1);
  });

  it('hydrates only the active vSphere workflow resource family', () => {
    mockPathname.mockReturnValue('/vmware/networks');
    setResources([
      makeResource({
        id: 'esxi-host-1',
        type: 'agent',
        vmware: { entityType: 'host', managedObjectId: 'host-1' },
      }),
      makeResource({
        id: 'network-1',
        type: 'network',
        vmware: { entityType: 'network', managedObjectId: 'network-1' },
      }),
    ]);

    render(() => <VmwarePageSurface />);

    const activeCall = mockUseUnifiedResources.mock.calls.find(([options]) =>
      (options as { enabled: () => boolean }).enabled(),
    );
    expect(activeCall?.[0]).toEqual(
      expect.objectContaining({
        cacheKey: 'vmware-networks',
        query: 'type=agent,network&source=vmware-vsphere',
      }),
    );
  });

  it('does not treat vSphere ESXi API host resources as Pulse agent update targets', () => {
    mockVersionInfo.mockReturnValue({
      version: 'v6.0.0-rc.6',
      agentUpdateTargetVersion: 'v6.0.0-rc.6',
    });
    setResources([
      makeResource({
        id: 'esxi-host-1',
        name: 'esxi-host-1',
        displayName: 'esxi-host-1',
        type: 'agent',
        agent: { agentId: 'esxi-host-1', agentVersion: 'v5.1.34', osVersion: 'VMware ESXi 8.0.3' },
        vmware: { entityType: 'host', managedObjectId: 'host-1' },
      }),
      makeResource({
        id: 'vm-app-01',
        name: 'app-01',
        displayName: 'app-01',
        type: 'vm',
        vmware: { entityType: 'vm', managedObjectId: 'vm-1', runtimeHostName: 'esxi-host-1' },
      }),
    ]);

    render(() => <VmwarePageSurface />);

    expect(screen.getByTestId('hosts-table')).toHaveAttribute('data-rows', '1');
    expect(screen.queryByTestId('platform-outdated-agent-notice')).not.toBeInTheDocument();
  });

  it('does not surface stale-agent notices for development builds without an agent target', () => {
    mockVersionInfo.mockReturnValue({
      version: '6.0.0-rc.6+git.172.g2c360f779.dirty',
      isDevelopment: true,
    });
    setResources([
      makeResource({
        id: 'vm-app-01',
        name: 'app-01',
        displayName: 'app-01',
        type: 'vm',
        agent: { agentId: 'agent-app-01', agentVersion: 'v5.1.34' },
        vmware: { entityType: 'vm', managedObjectId: 'vm-1', runtimeHostName: 'esxi-host-1' },
      }),
    ]);

    render(() => <VmwarePageSurface />);

    expect(screen.queryByTestId('platform-outdated-agent-notice')).not.toBeInTheDocument();
  });

  it('surfaces viewer-safe vCenter collection completeness diagnostics', async () => {
    mockListInventorySources.mockResolvedValue({
      sources: [
        {
          type: 'vmware',
          name: 'Production vCenter',
          state: 'active',
          surfaces: ['vms'],
          completeness: {
            state: 'degraded',
            issueCount: 3,
            issues: [{ stage: 'tags', category: 'permission', occurrences: 3 }],
          },
        },
      ],
    });
    setResources([
      makeResource({
        id: 'esxi-host-1',
        type: 'agent',
        vmware: { entityType: 'host', managedObjectId: 'host-1' },
      }),
    ]);

    render(() => <VmwarePageSurface />);

    const notice = await screen.findByTestId('vmware-inventory-completeness-notice');
    expect(notice).toHaveTextContent('Some vSphere inventory details are incomplete');
    expect(notice).toHaveTextContent(
      'Production vCenter: the last successful collection reported 3 optional read issues.',
    );
  });
});
