import { cleanup, render, screen } from '@solidjs/testing-library';
import { Route, Router } from '@solidjs/router';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import type { Resource } from '@/types/resource';
import { ProxmoxPageSurface } from '../ProxmoxPageSurface';
import proxmoxPageSurfaceSource from '../ProxmoxPageSurface.tsx?raw';
import proxmoxBackupServersTableSource from '../ProxmoxBackupServersTable.tsx?raw';
import proxmoxBackupsTableSource from '../ProxmoxBackupsTable.tsx?raw';
import proxmoxRecoverableTableSource from '../ProxmoxRecoverableTable.tsx?raw';

const mockUseUnifiedResources = vi.fn();
const mockPathname = vi.hoisted(() => vi.fn(() => '/proxmox/overview'));
const mockVersionInfo = vi.hoisted(() => vi.fn());
const mockStorageProps = vi.hoisted(() => vi.fn());
const mockTotalStats = vi.hoisted(() => vi.fn());
const mockNodesTableProps = vi.hoisted(() => vi.fn());
const mockBackupServersTableProps = vi.hoisted(() => vi.fn());
const mockWorkloadSearch = vi.hoisted(() => vi.fn(() => ''));

const makeResource = (resource: Partial<Resource> & Pick<Resource, 'id' | 'type'>): Resource =>
  ({
    name: resource.id,
    displayName: resource.id,
    platformId: 'proxmox-1',
    platformType: 'proxmox-pve',
    sourceType: 'api',
    sources: ['agent', 'proxmox'],
    status: 'online',
    lastSeen: 1_700_000_000_000,
    ...resource,
  }) as Resource;

const setResources = (resources: Resource[]) => {
  mockUseUnifiedResources.mockReturnValue({
    resources: () => resources,
    loading: () => false,
    error: () => null,
    refetch: vi.fn(),
  });
};

const setResourcesSnapshot = (resources: Resource[] | undefined, loading = false) => {
  mockUseUnifiedResources.mockReturnValue({
    resources: () => resources as Resource[],
    loading: () => loading,
    error: () => null,
    refetch: vi.fn(),
  });
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

vi.mock('@solidjs/router', async () => {
  const actual = await vi.importActual<typeof import('@solidjs/router')>('@solidjs/router');
  return {
    ...actual,
    useLocation: () => ({ pathname: mockPathname() }),
  };
});

vi.mock('@/components/Storage/Storage', () => ({
  default: (props: { forcedSourceFilter?: string }) => {
    mockStorageProps(props);
    return <div data-testid="storage-surface" />;
  },
}));

vi.mock('@/components/Workloads/WorkloadsFilter', () => ({
  WorkloadsFilter: () => <div data-testid="workloads-filter" />,
}));

vi.mock('@/components/Workloads/WorkloadsSurface', () => ({
  WorkloadsSurface: () => <div data-testid="workloads-surface" />,
}));

vi.mock('@/components/Workloads/useWorkloadsState', () => ({
  useWorkloadsState: () => ({
    surfaceConnected: () => false,
    surfaceInitialDataReceived: () => false,
    allGuests: () => [],
    totalStats: mockTotalStats,
    search: mockWorkloadSearch,
    setSearch: vi.fn(),
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

vi.mock('../ProxmoxBackupsTable', () => ({
  ProxmoxBackupsTable: () => <div data-testid="backups-table" />,
}));

vi.mock('../ProxmoxBackupServersTable', () => ({
  ProxmoxBackupServersTable: (props: { servers: Resource[] }) => {
    mockBackupServersTableProps(props);
    return <div data-testid="backup-servers-table" data-rows={props.servers.length} />;
  },
}));

vi.mock('../ProxmoxCephTable', () => ({
  ProxmoxCephTable: () => <div data-testid="ceph-table" />,
}));

vi.mock('../ProxmoxMailGatewayTable', () => ({
  ProxmoxMailGatewayTable: () => <div data-testid="mail-table" />,
}));

vi.mock('../ProxmoxNodesTable', () => ({
  ProxmoxNodesTable: (props: { nodes: Resource[]; search?: () => string; topology?: unknown }) => {
    mockNodesTableProps(props);
    return (
      <div
        data-testid="nodes-table"
        data-rows={props.nodes.length}
        data-search={props.search?.() ?? ''}
      />
    );
  },
}));

vi.mock('../ProxmoxReplicationTable', () => ({
  ProxmoxReplicationTable: () => <div data-testid="replication-table" />,
  fetchReplicationJobs: () => Promise.resolve([]),
}));

const renderSurface = () =>
  render(() => (
    <Router>
      <Route path="/" component={ProxmoxPageSurface} />
    </Router>
  ));

describe('ProxmoxPageSurface contract', () => {
  beforeEach(() => {
    mockPathname.mockReturnValue('/proxmox/overview');
    mockVersionInfo.mockReturnValue(null);
    mockTotalStats.mockReturnValue({
      total: 3,
      running: 1,
      degraded: 1,
      stopped: 1,
      vms: 3,
      containers: 0,
      appContainers: 0,
      pods: 0,
    });
    mockWorkloadSearch.mockReturnValue('');
  });

  afterEach(() => {
    cleanup();
    vi.clearAllMocks();
  });

  it('scopes the storage tab to both PVE and PBS resources', () => {
    mockPathname.mockReturnValue('/proxmox/storage');
    setResources([
      makeResource({
        id: 'pbs-datastore-1',
        type: 'storage',
        platformType: 'proxmox-pbs',
        sources: ['pbs'],
      }),
    ]);

    renderSurface();

    expect(screen.getByTestId('storage-surface')).toBeInTheDocument();
    expect(mockStorageProps).toHaveBeenCalledWith(
      expect.objectContaining({ forcedSourceFilter: 'proxmox-all' }),
    );
  });

  it('surfaces stale agent-backed Proxmox nodes', () => {
    mockVersionInfo.mockReturnValue({
      version: 'v6.0.0-rc.6',
      agentUpdateTargetVersion: 'v6.0.0-rc.6',
    });
    setResources([
      makeResource({
        id: 'agent:delly',
        name: 'delly',
        displayName: 'delly',
        type: 'agent',
        proxmox: { nodeName: 'delly', clusterName: 'homelab' },
        agent: { agentId: 'agent-delly', agentVersion: 'v5.1.34' },
      }),
    ]);

    renderSurface();

    expect(screen.getByTestId('platform-section-tabs')).toHaveAttribute('data-active', 'overview');
    expect(screen.getByTestId('nodes-table')).toHaveAttribute('data-rows', '1');
    const notice = screen.getByTestId('platform-outdated-agent-notice');
    expect(notice).toHaveTextContent('delly is running an older Pulse agent (v5.1.34).');
    expect(notice).toHaveTextContent(
      'latest agent-contributed Proxmox node detail and command support',
    );
    expect(screen.getByRole('link', { name: 'Open agent upgrade commands' })).toHaveAttribute(
      'href',
      '/settings/infrastructure/agent-doctor?agents=agent%3Aagent-delly',
    );
  });

  it('passes committed workload search terms to the node table', () => {
    mockWorkloadSearch.mockReturnValue('reporting-api-01, wireguard-edge-01');
    setResources([
      makeResource({
        id: 'agent:pve1',
        type: 'agent',
        proxmox: { nodeName: 'pve1', clusterName: 'production' },
      }),
      makeResource({
        id: 'agent:pve2',
        type: 'agent',
        proxmox: { nodeName: 'pve2', clusterName: 'production' },
      }),
      makeResource({
        id: 'lxc:108',
        type: 'system-container',
        name: 'reporting-api-01',
        proxmox: { nodeName: 'pve1', vmid: 108 },
      }),
      makeResource({
        id: 'lxc:111',
        type: 'system-container',
        name: 'wireguard-edge-01',
        proxmox: { nodeName: 'pve2', vmid: 111 },
      }),
    ]);

    renderSurface();

    expect(screen.getByTestId('nodes-table')).toHaveAttribute('data-rows', '2');
    expect(screen.getByTestId('nodes-table')).toHaveAttribute(
      'data-search',
      'reporting-api-01, wireguard-edge-01',
    );
    expect(mockNodesTableProps).toHaveBeenCalledWith(
      expect.objectContaining({
        nodes: expect.arrayContaining([expect.any(Object)]),
        search: mockWorkloadSearch,
      }),
    );
    expect(proxmoxPageSurfaceSource).toContain('search={workloadsState.search}');
  });

  it('does not call a retained version from a stopped Proxmox agent currently running', () => {
    mockVersionInfo.mockReturnValue({
      version: 'v6.0.0-rc.6',
      agentUpdateTargetVersion: 'v6.0.0-rc.6',
    });
    setResources([
      makeResource({
        id: 'agent:old-pi-agent',
        name: 'pi',
        displayName: 'pi',
        type: 'agent',
        status: 'online',
        proxmox: { nodeName: 'pi', clusterName: 'homelab' },
        agent: {
          agentId: 'old-pi-agent',
          agentVersion: 'v6.0.0-rc.5',
          stale: true,
          lastReportAt: '2026-08-08T23:05:41Z',
        },
      }),
    ]);

    renderSurface();

    expect(screen.getByTestId('nodes-table')).toHaveAttribute('data-rows', '1');
    expect(screen.queryByTestId('platform-outdated-agent-notice')).not.toBeInTheDocument();
  });

  it('folds estate topology into the existing nodes table header contract', () => {
    setResources([
      makeResource({
        id: 'agent:pve-1',
        type: 'agent',
        proxmox: { nodeName: 'pve-1', clusterName: 'homelab' },
      }),
      makeResource({
        id: 'vm-100',
        type: 'vm',
        status: 'running',
        proxmox: { nodeName: 'pve-1', vmid: 100 },
      }),
      makeResource({
        id: 'vm-101',
        type: 'vm',
        status: 'degraded',
        proxmox: { nodeName: 'pve-1', vmid: 101 },
      }),
      makeResource({
        id: 'vm-102',
        type: 'vm',
        status: 'stopped',
        proxmox: { nodeName: 'pve-1', vmid: 102 },
      }),
    ]);
    renderSurface();

    expect(mockNodesTableProps).toHaveBeenLastCalledWith(
      expect.objectContaining({
        topology: { clusters: 1, nodes: 1, standalone: 0 },
      }),
    );
  });

  it('keeps standalone PBS host health on the Proxmox overview', () => {
    const pbsServer = makeResource({
      id: 'pbs:standalone',
      type: 'pbs',
      name: 'pbs-standalone',
      platformType: 'proxmox-pbs',
      sources: ['pbs'],
      pbs: { instanceId: 'pbs-standalone', connectionHealth: 'healthy' },
    });
    const pbsAgent = makeResource({
      id: 'agent:pbs-standalone',
      type: 'agent',
      name: 'pbs-standalone',
      platformType: 'proxmox-pbs',
      sources: ['agent', 'pbs'],
      agent: { agentId: 'agent-pbs-standalone', hostname: 'pbs-standalone' },
    });
    setResources([pbsServer, pbsAgent]);

    renderSurface();

    expect(screen.getByTestId('backup-servers-table')).toHaveAttribute('data-rows', '2');
    expect(mockBackupServersTableProps).toHaveBeenCalledWith(
      expect.objectContaining({ servers: [pbsServer, pbsAgent] }),
    );
  });

  it('shares workload search with the node inventory', () => {
    mockWorkloadSearch.mockReturnValue('pve-1');
    setResources([
      makeResource({
        id: 'agent:pve-1',
        type: 'agent',
        proxmox: { nodeName: 'pve-1', clusterName: 'homelab' },
      }),
    ]);

    renderSurface();

    expect(mockNodesTableProps).toHaveBeenLastCalledWith(
      expect.objectContaining({ search: mockWorkloadSearch }),
    );
  });

  it('keeps Proxmox workload and backup filters free of a saved-views affordance', () => {
    // Saved views persisted the page's URL query string to localStorage. The
    // browser's own bookmarks already do that and survive a cleared cache, so
    // the control was removed; PROXMOX_BACKUPS_QUERY_PARAMS keeps the backup
    // toolbar URL-owned and therefore still shareable.
    expect(proxmoxPageSurfaceSource).not.toContain('savedViewsKey');
    expect(proxmoxBackupsTableSource).not.toContain('savedViewsKey');
    expect(proxmoxBackupsTableSource).not.toContain('SavedViews');
  });

  it('keeps backup summary identity on one canonical compact row', () => {
    expect(proxmoxBackupServersTableSource).not.toContain(
      'font-mono text-[10px] font-normal text-muted',
    );
    expect(proxmoxBackupServersTableSource).toContain(
      "title={[row.serverName, row.datastore?.name].filter(Boolean).join(' · ')}",
    );
    expect(proxmoxRecoverableTableSource).toContain('class="flex min-w-0 items-center gap-1"');
    expect(proxmoxRecoverableTableSource).not.toContain('class="truncate text-[10px] text-muted"');
  });

  it('hydrates route-scoped resource families instead of one broad estate request', () => {
    setResources([
      makeResource({
        id: 'agent:pve-1',
        type: 'agent',
        proxmox: { nodeName: 'pve-1', clusterName: 'homelab' },
      }),
    ]);

    renderSurface();

    const options = mockUseUnifiedResources.mock.calls.map(
      ([value]) =>
        value as {
          query: string;
          cacheKey: string;
          enabled: () => boolean;
        },
    );
    expect(options.map((value) => value.cacheKey)).toEqual([
      'proxmox-overview',
      'proxmox-storage-shell',
      'proxmox-replication-shell',
      'proxmox-backups-shell',
      'proxmox-ceph',
      'proxmox-mail',
    ]);
    expect(options.map((value) => value.query)).toEqual([
      'type=agent,vm,system-container,oci-container,pbs',
      'type=agent,pbs,storage,physical_disk,ceph',
      'type=agent',
      'type=agent,vm,system-container,pbs',
      'type=ceph',
      'type=pmg',
    ]);
    expect(options[0].enabled()).toBe(true);
    expect(options.slice(1).every((value) => value.enabled() === false)).toBe(true);
    expect(proxmoxPageSurfaceSource).toContain('requestIdleCallback');
    expect(proxmoxPageSurfaceSource).toContain("phoneViewport\n      ? ['storage']");
    expect(proxmoxPageSurfaceSource).toContain('resourceSource={storageResources}');
    expect(proxmoxPageSurfaceSource).toContain('servers={currentModel().pbs}');
  });

  it('places workload controls beside the workload table they affect', () => {
    const nodesTableIndex = proxmoxPageSurfaceSource.indexOf('<ProxmoxNodesTable');
    const workloadFilterIndex = proxmoxPageSurfaceSource.indexOf('<WorkloadsFilter');
    const workloadsSurfaceIndex = proxmoxPageSurfaceSource.indexOf('<WorkloadsSurface');

    expect(nodesTableIndex).toBeGreaterThan(-1);
    expect(workloadFilterIndex).toBeGreaterThan(nodesTableIndex);
    expect(workloadsSurfaceIndex).toBeGreaterThan(workloadFilterIndex);
    expect(proxmoxPageSurfaceSource.indexOf('<ProxmoxBackupServersTable')).toBeGreaterThan(
      nodesTableIndex,
    );
    expect(proxmoxPageSurfaceSource.indexOf('<ProxmoxBackupServersTable')).toBeLessThan(
      workloadFilterIndex,
    );
  });

  it('keeps the bounded node preview before guests at every viewport', () => {
    expect(proxmoxPageSurfaceSource).toContain('<section>\n        <ProxmoxNodesTable');
    expect(proxmoxPageSurfaceSource).toContain('class="space-y-3 scroll-mt-4"');
    expect(proxmoxPageSurfaceSource).not.toContain('order-2 lg:order-1');
    expect(proxmoxPageSurfaceSource).not.toContain('order-1 space-y-3');
    expect(proxmoxPageSurfaceSource).toContain('id="proxmox-guests-section"');
    expect(proxmoxPageSurfaceSource).toContain('tableTitle={');
    expect(proxmoxPageSurfaceSource).toContain('id="proxmox-guests-heading"');
    expect(proxmoxPageSurfaceSource).not.toContain('class="flex items-center gap-2 px-1"');
  });

  it('keeps Patrol coverage off the Proxmox overview', () => {
    setResources([
      makeResource({
        id: 'agent:pve-1',
        type: 'agent',
        proxmox: { nodeName: 'pve-1', clusterName: 'homelab' },
      }),
    ]);

    renderSurface();

    expect(screen.getByTestId('nodes-table')).toHaveAttribute('data-rows', '1');
    expect(screen.queryByRole('list', { name: 'Proxmox Patrol coverage' })).not.toBeInTheDocument();
    expect(screen.queryByText('Protection current')).not.toBeInTheDocument();
    expect(
      screen.queryByRole('list', { name: 'Patrol protection posture' }),
    ).not.toBeInTheDocument();
  });

  it('does not crash while Proxmox resources hydrate', () => {
    setResourcesSnapshot(undefined, true);

    renderSurface();

    expect(screen.getByTestId('platform-table-loading-state')).toBeInTheDocument();
    expect(screen.queryByRole('list', { name: 'Proxmox Patrol coverage' })).not.toBeInTheDocument();
  });

  it('does not surface stale-agent notices for development builds without an agent target', () => {
    mockVersionInfo.mockReturnValue({
      version: '6.0.0-rc.6+git.172.g2c360f779.dirty',
      isDevelopment: true,
    });
    setResources([
      makeResource({
        id: 'agent:delly',
        name: 'delly',
        displayName: 'delly',
        type: 'agent',
        proxmox: { nodeName: 'delly', clusterName: 'homelab' },
        agent: { agentId: 'agent-delly', agentVersion: 'v5.1.34' },
      }),
    ]);

    renderSurface();

    expect(screen.queryByTestId('platform-outdated-agent-notice')).not.toBeInTheDocument();
  });
});
