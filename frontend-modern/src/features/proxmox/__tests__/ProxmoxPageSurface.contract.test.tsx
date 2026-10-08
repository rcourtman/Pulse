import { cleanup, fireEvent, render, screen, waitFor } from '@solidjs/testing-library';
import { Route, Router } from '@solidjs/router';
import { createSignal } from 'solid-js';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import type { UnifiedResourceFacets } from '@/hooks/useUnifiedResources';
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
const mockBackupsTableProps = vi.hoisted(() => vi.fn());
const mockWorkloadSearch = vi.hoisted(() => vi.fn(() => ''));
const mockSelectedNode = vi.hoisted(() => vi.fn<() => string | null>(() => null));
const mockHandleNodeSelect = vi.hoisted(() => vi.fn());
const mockWorkloadsOptions = vi.hoisted(() => vi.fn());
const mockFetchReplicationJobs = vi.hoisted(() =>
  vi.fn((_signal?: AbortSignal) => Promise.resolve([] as unknown[])),
);

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
  useWorkloadsState: (options: unknown) => {
    mockWorkloadsOptions(options);
    return {
      surfaceConnected: () => false,
      surfaceInitialDataReceived: () => false,
      allGuests: () => [],
      selectedNode: mockSelectedNode,
      handleNodeSelect: mockHandleNodeSelect,
      selectedHostHint: () => null,
      totalStats: mockTotalStats,
      search: mockWorkloadSearch,
      setSearch: vi.fn(),
    };
  },
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
  ProxmoxBackupsTable: (props: { workloads: Resource[]; servers: Resource[] }) => {
    mockBackupsTableProps(props);
    return <div data-testid="backups-table" />;
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
  ProxmoxReplicationTable: (props: { jobs?: unknown[]; error?: unknown; onRetry: () => void }) => (
    <div
      data-testid="replication-table"
      data-jobs={props.jobs === undefined ? 'loading' : props.jobs.length}
      data-error={props.error ? 'yes' : 'no'}
    >
      <button type="button" onClick={() => props.onRetry()}>
        Retry replication
      </button>
    </div>
  ),
  fetchReplicationJobs: (signal?: AbortSignal) => mockFetchReplicationJobs(signal),
}));

const renderSurface = () =>
  render(() => (
    <Router>
      <Route path="/" component={ProxmoxPageSurface} />
    </Router>
  ));

describe('ProxmoxPageSurface contract', () => {
  beforeEach(() => {
    mockWorkloadsOptions.mockClear();
    mockSelectedNode.mockReturnValue(null);
    mockHandleNodeSelect.mockClear();
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

  it('passes the committed resource change metadata into the Workloads owner', () => {
    const change = { version: 3, changedIds: new Set(['vm-1']) };
    const resourceSnapshotChange = () => change;
    mockUseUnifiedResources.mockReturnValue({
      resources: () => [makeResource({ id: 'vm-1', type: 'vm' })],
      resourceSnapshotChange,
      loading: () => false,
      error: () => null,
      refetch: vi.fn(),
    });

    renderSurface();

    expect(mockWorkloadsOptions).toHaveBeenCalledWith(
      expect.objectContaining({ resourceSnapshotChange }),
    );
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
    expect(notice).toHaveTextContent('delly runs an older Pulse agent (v5.1.34).');
    expect(notice).toHaveTextContent('latest fixes and node details');
    expect(screen.getByRole('link', { name: 'Open agent upgrade commands' })).toHaveAttribute(
      'href',
      '/settings/infrastructure/agent-doctor?agents=agent%3Aagent-delly',
    );
  });

  it('toggles only the node scope when the selected node is activated again', () => {
    const node = makeResource({
      id: 'agent:pve1',
      name: 'pve1',
      type: 'agent',
      proxmox: { nodeName: 'pve1', instance: 'lab' },
    });
    setResources([node]);
    renderSurface();
    const { onShowGuests } = mockNodesTableProps.mock.lastCall![0];
    const revealFrame = vi.spyOn(window, 'requestAnimationFrame');

    onShowGuests(node);
    expect(mockHandleNodeSelect).toHaveBeenLastCalledWith('lab-pve1', 'pve');
    expect(revealFrame).toHaveBeenCalledTimes(1);
    revealFrame.mockClear();
    mockSelectedNode.mockReturnValue('lab-pve1');
    onShowGuests(node);
    expect(mockHandleNodeSelect).toHaveBeenLastCalledWith(null, null);
    expect(revealFrame).not.toHaveBeenCalled();
    mockSelectedNode.mockReturnValue('lab-other');
    onShowGuests(node);
    expect(mockHandleNodeSelect).toHaveBeenLastCalledWith('lab-pve1', 'pve');
    expect(revealFrame).toHaveBeenCalledTimes(1);
    revealFrame.mockRestore();
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

  it('shows the outdated sensor setup notice on Overview without hydrating disks', () => {
    const node = makeResource({
      id: 'agent:pve3',
      name: 'pve3',
      displayName: 'pve3',
      type: 'agent',
      proxmox: {
        nodeName: 'pve3',
        temperatureDetails: { available: true, legacySensorsFormat: true },
        sensorSetupOutdated: true,
      },
    });
    // Like production, the Overview query returns nodes and guests only: the
    // verdict has to arrive on the node, because no physical_disk rows do.
    mockUseUnifiedResources.mockImplementation((options: { cacheKey?: string }) => ({
      resources: () => (options.cacheKey === 'proxmox-overview' ? [node] : []),
      loading: () => false,
      error: () => null,
      refetch: vi.fn(),
    }));

    renderSurface();

    const overviewOptions = mockUseUnifiedResources.mock.calls
      .map(([options]) => options as { cacheKey?: string; query?: string })
      .find((options) => options.cacheKey === 'proxmox-overview');
    expect(overviewOptions?.query).toBeDefined();
    expect(overviewOptions?.query).not.toContain('physical_disk');
    expect(screen.getByTestId('platform-section-tabs')).toHaveAttribute('data-active', 'overview');
    expect(screen.getByTestId('platform-outdated-sensor-setup-notice')).toHaveTextContent(
      'pve3 is using an older temperature monitoring setup that cannot read SATA/SAS disk temperatures.',
    );
    expect(screen.getByRole('link', { name: 'Open Infrastructure settings' })).toHaveAttribute(
      'href',
      '/settings/infrastructure',
    );
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

  it('hydrates only the active source-scoped resource family', () => {
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
          enabled?: () => boolean;
        },
    );
    expect(options.map((value) => value.cacheKey)).toEqual([
      'proxmox-tab-evidence',
      'proxmox-overview',
      'proxmox-storage-shell',
      'proxmox-replication-shell',
      'proxmox-backups-shell',
      'proxmox-ceph',
      'proxmox-mail',
    ]);
    expect(options.map((value) => value.query)).toEqual([
      'type=pmg&source=proxmox,pbs,pmg,agent',
      'type=agent,vm,system-container,oci-container&source=proxmox',
      'type=agent,pbs,storage,physical_disk,ceph&source=proxmox,pbs,agent',
      'type=agent&source=proxmox',
      'type=pbs,agent&source=pbs',
      'type=ceph&source=proxmox',
      'type=pmg&source=pmg',
    ]);
    expect(options[0].enabled).toBeUndefined();
    expect(options[1].enabled?.()).toBe(true);
    expect(options.slice(2).every((value) => value.enabled?.() === false)).toBe(true);
    expect(proxmoxPageSurfaceSource).not.toContain('backgroundHydrationTabs');
    expect(proxmoxPageSurfaceSource).not.toContain('requestIdleCallback');
    expect(proxmoxPageSurfaceSource).toContain('resourceSource={storageResources}');
  });

  it('reuses guests and hydrates standalone PBS telemetry without duplicate candidates', () => {
    mockPathname.mockReturnValue('/proxmox/backups');
    const guest = makeResource({
      id: 'vm-100',
      type: 'vm',
      proxmox: { vmid: 100 },
      agent: { agentId: 'guest-agent', hostname: 'pbs-vm' },
    });
    const agent = makeResource({
      id: 'pbs-agent',
      type: 'agent',
      platformType: 'proxmox-pbs',
      sources: ['pbs', 'agent'],
      metricsTarget: { resourceType: 'agent', resourceId: 'pbs-agent' },
    });
    const server = makeResource({
      id: 'pbs-1',
      type: 'pbs',
      platformType: 'proxmox-pbs',
      sources: ['pbs'],
    });
    mockUseUnifiedResources.mockImplementation((options: { cacheKey: string }) => ({
      resources: () => {
        if (options.cacheKey === 'proxmox-overview') return [guest, agent];
        if (options.cacheKey === 'proxmox-backups-shell') return [server, agent];
        return [];
      },
      loading: () => false,
      error: () => null,
      refetch: vi.fn(),
    }));

    renderSurface();

    const options = mockUseUnifiedResources.mock.calls.map(
      ([value]) => value as { cacheKey: string; query: string; enabled?: () => boolean },
    );
    expect(options.map((value) => value.enabled?.() ?? true)).toEqual([
      true,
      true,
      false,
      false,
      true,
      false,
      false,
    ]);
    expect(options[4]).toMatchObject({
      cacheKey: 'proxmox-backups-shell',
      query: 'type=pbs,agent&source=pbs',
    });
    expect(mockBackupsTableProps).toHaveBeenCalledWith(
      expect.objectContaining({ workloads: [guest], servers: [guest, agent, server] }),
    );
  });

  it('places workload controls beside the workload table they affect', () => {
    const nodesTableIndex = proxmoxPageSurfaceSource.indexOf('<ProxmoxNodesTable');
    const workloadFilterIndex = proxmoxPageSurfaceSource.indexOf('<WorkloadsFilter');
    const workloadsSurfaceIndex = proxmoxPageSurfaceSource.indexOf('<WorkloadsSurface');

    expect(nodesTableIndex).toBeGreaterThan(-1);
    expect(workloadFilterIndex).toBeGreaterThan(nodesTableIndex);
    expect(workloadsSurfaceIndex).toBeGreaterThan(workloadFilterIndex);
    expect(proxmoxPageSurfaceSource).not.toContain('<ProxmoxBackupServersTable');
    expect(proxmoxPageSurfaceSource).toContain(
      '{...getWorkloadsMetricFilterProps(workloadsState)}',
    );
    expect(proxmoxPageSurfaceSource).not.toContain(
      'metricHoverMode={workloadsState.workloadMetricHoverMode}',
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

  it('does not advertise optional sections before resource counts arrive', () => {
    setResourcesSnapshot(undefined, true);

    renderSurface();

    expect(screen.getByTestId('platform-section-tabs')).toHaveAttribute('data-tabs', 'overview');
    expect(screen.getByTestId('platform-table-loading-state')).toBeInTheDocument();
  });

  it('hydrates a direct link while counts are unknown, then gates it on actual capabilities', async () => {
    mockPathname.mockReturnValue('/proxmox/mail');
    const [facets, setFacets] = createSignal<UnifiedResourceFacets | null>(null);
    const refetch = vi.fn(async () => []);
    mockUseUnifiedResources.mockReturnValue({
      resources: () => [],
      // Estate-wide counts include unrelated providers and must not unlock tabs.
      aggregations: () => ({ total: 5, byType: { storage: 1, vm: 1, ceph: 1, pmg: 1 } }),
      facets,
      loading: () => false,
      error: () => null,
      refetch,
    });

    renderSurface();

    const tabs = screen.getByTestId('platform-section-tabs');
    expect(tabs).toHaveAttribute('data-tabs', 'overview');
    expect(tabs).toHaveAttribute('data-active', 'mail');
    const options = mockUseUnifiedResources.mock.calls.map(
      ([value]) => value as { cacheKey: string; enabled: () => boolean },
    );
    expect(options.find((option) => option.cacheKey === 'proxmox-mail')?.enabled()).toBe(true);
    await waitFor(() => expect(refetch).toHaveBeenCalledTimes(1));

    setFacets({ incidentCount: 0, byType: { pbs: 1, pmg: 1 } });
    await waitFor(() => {
      expect(tabs).toHaveAttribute('data-tabs', 'overview,backups,mail');
      expect(tabs).toHaveAttribute('data-active', 'mail');
    });

    setFacets({ incidentCount: 0, byType: { pbs: 1 } });
    await waitFor(() => {
      expect(tabs).toHaveAttribute('data-tabs', 'overview,backups');
      expect(tabs).toHaveAttribute('data-active', 'overview');
    });
  });

  it('re-reads replication jobs in the background and keeps the tab through a failed read', async () => {
    vi.useFakeTimers({ toFake: ['setInterval', 'clearInterval'] });
    mockPathname.mockReturnValue('/proxmox/replication');
    const node = makeResource({ id: 'node:pve1', type: 'agent', name: 'pve1' });
    mockUseUnifiedResources.mockReturnValue({
      resources: () => [node],
      facets: () => ({ incidentCount: 0, byType: {} }),
      loading: () => false,
      error: () => null,
      refetch: vi.fn(async () => []),
    });
    let resolveFirstRead: (jobs: unknown[]) => void = () => undefined;
    mockFetchReplicationJobs.mockReset();
    mockFetchReplicationJobs.mockReturnValueOnce(
      new Promise<unknown[]>((resolve) => {
        resolveFirstRead = resolve;
      }),
    );

    try {
      renderSurface();
      const tabs = screen.getByTestId('platform-section-tabs');
      // Counts are known but the first read is pending: a direct link stays put.
      expect(tabs).toHaveAttribute('data-active', 'replication');
      expect(screen.getByTestId('replication-table')).toHaveAttribute('data-jobs', 'loading');

      resolveFirstRead([{ id: 'job-1' }]);
      await waitFor(() =>
        expect(screen.getByTestId('replication-table')).toHaveAttribute('data-jobs', '1'),
      );
      expect(tabs).toHaveAttribute('data-tabs', 'overview,replication');

      // The table's ages move on the shared clock, so its jobs are re-read in
      // the background rather than aging a snapshot taken at mount.
      mockFetchReplicationJobs.mockResolvedValueOnce([{ id: 'job-1' }, { id: 'job-2' }]);
      vi.advanceTimersByTime(30_000);
      await waitFor(() =>
        expect(screen.getByTestId('replication-table')).toHaveAttribute('data-jobs', '2'),
      );

      // A failed read reports the error but keeps the tab and the user on it.
      mockFetchReplicationJobs.mockRejectedValueOnce(new Error('replication read failed'));
      vi.advanceTimersByTime(30_000);
      await waitFor(() =>
        expect(screen.getByTestId('replication-table')).toHaveAttribute('data-error', 'yes'),
      );
      expect(tabs).toHaveAttribute('data-tabs', 'overview,replication');
      expect(tabs).toHaveAttribute('data-active', 'replication');

      mockFetchReplicationJobs.mockResolvedValueOnce([{ id: 'job-1' }]);
      vi.advanceTimersByTime(30_000);
      await waitFor(() =>
        expect(screen.getByTestId('replication-table')).toHaveAttribute('data-error', 'no'),
      );
      expect(screen.getByTestId('replication-table')).toHaveAttribute('data-jobs', '1');
      expect(mockFetchReplicationJobs).toHaveBeenCalledTimes(4);
    } finally {
      vi.useRealTimers();
      mockFetchReplicationJobs.mockReset();
      mockFetchReplicationJobs.mockImplementation(() => Promise.resolve([]));
    }
  });

  it('holds a direct replication link on a failed first read and recovers on retry', async () => {
    mockPathname.mockReturnValue('/proxmox/replication');
    const node = makeResource({ id: 'node:pve1', type: 'agent', name: 'pve1' });
    mockUseUnifiedResources.mockReturnValue({
      resources: () => [node],
      facets: () => ({ incidentCount: 0, byType: {} }),
      loading: () => false,
      error: () => null,
      refetch: vi.fn(async () => []),
    });
    mockFetchReplicationJobs.mockReset();
    mockFetchReplicationJobs.mockRejectedValueOnce(new Error('replication read failed'));
    mockFetchReplicationJobs.mockResolvedValueOnce([{ id: 'job-1' }]);

    try {
      renderSurface();
      const tabs = screen.getByTestId('platform-section-tabs');
      // A failed read says nothing about whether replication exists, so the
      // link keeps its error and Retry instead of falling back to Overview.
      await waitFor(() =>
        expect(screen.getByTestId('replication-table')).toHaveAttribute('data-error', 'yes'),
      );
      expect(tabs).toHaveAttribute('data-active', 'replication');
      expect(tabs).toHaveAttribute('data-tabs', 'overview');

      fireEvent.click(screen.getByRole('button', { name: 'Retry replication' }));
      await waitFor(() =>
        expect(screen.getByTestId('replication-table')).toHaveAttribute('data-jobs', '1'),
      );
      expect(screen.getByTestId('replication-table')).toHaveAttribute('data-error', 'no');
      expect(tabs).toHaveAttribute('data-tabs', 'overview,replication');
      expect(tabs).toHaveAttribute('data-active', 'replication');
    } finally {
      mockFetchReplicationJobs.mockReset();
      mockFetchReplicationJobs.mockImplementation(() => Promise.resolve([]));
    }
  });

  it('keeps an empty replication answer through a failed poll instead of reopening the route', async () => {
    vi.useFakeTimers({ toFake: ['setInterval', 'clearInterval'] });
    mockPathname.mockReturnValue('/proxmox/replication');
    const node = makeResource({ id: 'node:pve1', type: 'agent', name: 'pve1' });
    mockUseUnifiedResources.mockReturnValue({
      resources: () => [node],
      facets: () => ({ incidentCount: 0, byType: {} }),
      loading: () => false,
      error: () => null,
      refetch: vi.fn(async () => []),
    });
    mockFetchReplicationJobs.mockReset();
    mockFetchReplicationJobs.mockResolvedValueOnce([]);
    mockFetchReplicationJobs.mockRejectedValueOnce(new Error('replication read failed'));
    mockFetchReplicationJobs.mockResolvedValueOnce([]);

    try {
      renderSurface();
      const tabs = screen.getByTestId('platform-section-tabs');
      // An empty answer means no replication: the link falls back to Overview.
      await waitFor(() => expect(tabs).toHaveAttribute('data-active', 'overview'));

      vi.advanceTimersByTime(30_000);
      await waitFor(() => expect(mockFetchReplicationJobs).toHaveBeenCalledTimes(2));
      await Promise.resolve();
      expect(tabs).toHaveAttribute('data-active', 'overview');
      expect(screen.queryByTestId('replication-table')).not.toBeInTheDocument();

      vi.advanceTimersByTime(30_000);
      await waitFor(() => expect(mockFetchReplicationJobs).toHaveBeenCalledTimes(3));
      expect(tabs).toHaveAttribute('data-active', 'overview');
    } finally {
      vi.useRealTimers();
      mockFetchReplicationJobs.mockReset();
      mockFetchReplicationJobs.mockImplementation(() => Promise.resolve([]));
    }
  });

  it('holds a direct replication link on an access denial that withdraws loaded jobs', async () => {
    vi.useFakeTimers({ toFake: ['setInterval', 'clearInterval'] });
    mockPathname.mockReturnValue('/proxmox/replication');
    const node = makeResource({ id: 'node:pve1', type: 'agent', name: 'pve1' });
    mockUseUnifiedResources.mockReturnValue({
      resources: () => [node],
      facets: () => ({ incidentCount: 0, byType: {} }),
      loading: () => false,
      error: () => null,
      refetch: vi.fn(async () => []),
    });
    mockFetchReplicationJobs.mockReset();
    mockFetchReplicationJobs.mockResolvedValueOnce([{ id: 'job-1' }]);
    mockFetchReplicationJobs.mockRejectedValueOnce(
      Object.assign(new Error('Insufficient permissions'), { status: 403 }),
    );

    try {
      renderSurface();
      const tabs = screen.getByTestId('platform-section-tabs');
      await waitFor(() =>
        expect(screen.getByTestId('replication-table')).toHaveAttribute('data-jobs', '1'),
      );

      vi.advanceTimersByTime(30_000);
      await waitFor(() =>
        expect(screen.getByTestId('replication-table')).toHaveAttribute('data-error', 'yes'),
      );
      // The denied jobs are withdrawn, so the tab goes; the link shows why.
      expect(tabs).toHaveAttribute('data-tabs', 'overview');
      expect(tabs).toHaveAttribute('data-active', 'replication');
    } finally {
      vi.useRealTimers();
      mockFetchReplicationJobs.mockReset();
      mockFetchReplicationJobs.mockImplementation(() => Promise.resolve([]));
    }
  });

  it('cancels an in-flight replication read when the page unmounts', async () => {
    mockPathname.mockReturnValue('/proxmox/replication');
    setResources([makeResource({ id: 'node:pve1', type: 'agent', name: 'pve1' })]);
    mockFetchReplicationJobs.mockReset();
    mockFetchReplicationJobs.mockReturnValueOnce(new Promise<unknown[]>(() => undefined));

    try {
      const view = renderSurface();
      expect(mockFetchReplicationJobs).toHaveBeenCalledTimes(1);
      const signal = mockFetchReplicationJobs.mock.calls[0][0];
      expect(signal).toBeInstanceOf(AbortSignal);
      expect(signal?.aborted).toBe(false);

      view.unmount();
      expect(signal?.aborted).toBe(true);
    } finally {
      mockFetchReplicationJobs.mockReset();
      mockFetchReplicationJobs.mockImplementation(() => Promise.resolve([]));
    }
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
