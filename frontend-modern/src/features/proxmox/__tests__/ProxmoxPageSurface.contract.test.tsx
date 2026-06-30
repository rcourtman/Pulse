import { cleanup, render, screen } from '@solidjs/testing-library';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import type { Resource } from '@/types/resource';
import { ProxmoxPageSurface } from '../ProxmoxPageSurface';

const mockUseUnifiedResources = vi.fn();
const mockPathname = vi.hoisted(() => vi.fn(() => '/proxmox/overview'));
const mockVersionInfo = vi.hoisted(() => vi.fn());

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
  default: () => <div data-testid="storage-surface" />,
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
    search: () => '',
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

vi.mock('../ProxmoxCephTable', () => ({
  ProxmoxCephTable: () => <div data-testid="ceph-table" />,
}));

vi.mock('../ProxmoxMailGatewayTable', () => ({
  ProxmoxMailGatewayTable: () => <div data-testid="mail-table" />,
}));

vi.mock('../ProxmoxNodesTable', () => ({
  ProxmoxNodesTable: (props: { nodes: Resource[] }) => (
    <div data-testid="nodes-table" data-rows={props.nodes.length} />
  ),
}));

vi.mock('../ProxmoxReplicationTable', () => ({
  ProxmoxReplicationTable: () => <div data-testid="replication-table" />,
  fetchReplicationJobs: () => Promise.resolve([]),
}));

describe('ProxmoxPageSurface contract', () => {
  beforeEach(() => {
    mockPathname.mockReturnValue('/proxmox/overview');
    mockVersionInfo.mockReturnValue(null);
  });

  afterEach(() => {
    cleanup();
    vi.clearAllMocks();
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

    render(() => <ProxmoxPageSurface />);

    expect(screen.getByTestId('platform-section-tabs')).toHaveAttribute('data-active', 'overview');
    expect(screen.getByTestId('nodes-table')).toHaveAttribute('data-rows', '1');
    const notice = screen.getByTestId('platform-outdated-agent-notice');
    expect(notice).toHaveTextContent('delly is running an older Pulse agent (v5.1.34).');
    expect(notice).toHaveTextContent(
      'latest agent-contributed Proxmox node detail and command support',
    );
    expect(screen.getByRole('link', { name: 'Open agent upgrade commands' })).toHaveAttribute(
      'href',
      '/settings/infrastructure?agentUpdates=1&agents=agent%3Aagent-delly',
    );
  });

  it('renders the v5-style guest totals strip from the page summary', () => {
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

    render(() => <ProxmoxPageSurface />);

    const totals = screen.getByTestId('proxmox-guest-totals');
    expect(totals).toHaveTextContent('1 running');
    expect(totals).toHaveTextContent('1 attention');
    expect(totals).toHaveTextContent('1 stopped');
  });

  it('keeps Patrol coverage off the Proxmox overview', () => {
    setResources([
      makeResource({
        id: 'agent:pve-1',
        type: 'agent',
        proxmox: { nodeName: 'pve-1', clusterName: 'homelab' },
      }),
    ]);

    render(() => <ProxmoxPageSurface />);

    expect(screen.getByTestId('nodes-table')).toHaveAttribute('data-rows', '1');
    expect(screen.queryByRole('list', { name: 'Proxmox Patrol coverage' })).not.toBeInTheDocument();
    expect(screen.queryByText('Protection current')).not.toBeInTheDocument();
    expect(
      screen.queryByRole('list', { name: 'Patrol protection posture' }),
    ).not.toBeInTheDocument();
  });

  it('does not crash while Proxmox resources hydrate', () => {
    setResourcesSnapshot(undefined, true);

    render(() => <ProxmoxPageSurface />);

    expect(screen.getByTestId('platform-table-loading-state')).toBeInTheDocument();
    expect(
      screen.queryByRole('list', { name: 'Proxmox Patrol coverage' }),
    ).not.toBeInTheDocument();
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

    render(() => <ProxmoxPageSurface />);

    expect(screen.queryByTestId('platform-outdated-agent-notice')).not.toBeInTheDocument();
  });
});
