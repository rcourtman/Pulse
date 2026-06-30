import { cleanup, render, screen, waitFor } from '@solidjs/testing-library';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import type { Resource } from '@/types/resource';
import { ProxmoxPageSurface } from '../ProxmoxPageSurface';

const mockUseUnifiedResources = vi.fn();
const mockPathname = vi.hoisted(() => vi.fn(() => '/proxmox/overview'));
const mockVersionInfo = vi.hoisted(() => vi.fn());
const mockGetPatrolStatus = vi.hoisted(() => vi.fn());
const mockGetPatrolRunHistory = vi.hoisted(() => vi.fn());
const mockLoadPatrolFindings = vi.hoisted(() => vi.fn());
const mockLoadPendingApprovals = vi.hoisted(() => vi.fn());
const mockPatrolState = vi.hoisted(() => ({
  findings: [] as Array<{
    investigationOutcome?: string;
    investigationStatus?: string;
    regressionCount?: number;
    status: string;
    timesRaised?: number;
  }>,
  pendingApprovalCount: 0,
}));

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

vi.mock('@/stores/aiIntelligence', () => ({
  aiIntelligenceStore: {
    get patrolFindings() {
      return mockPatrolState.findings;
    },
    get patrolPendingApprovalCount() {
      return mockPatrolState.pendingApprovalCount;
    },
    loadPatrolFindings: mockLoadPatrolFindings,
    loadPendingApprovals: mockLoadPendingApprovals,
  },
}));

vi.mock('@/api/patrol', () => ({
  getPatrolStatus: mockGetPatrolStatus,
  getPatrolRunHistory: mockGetPatrolRunHistory,
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
    mockPatrolState.findings = [];
    mockPatrolState.pendingApprovalCount = 0;
    mockLoadPatrolFindings.mockResolvedValue(undefined);
    mockLoadPendingApprovals.mockResolvedValue(undefined);
    mockGetPatrolStatus.mockResolvedValue({
      enabled: true,
      error_count: 0,
      findings_count: 0,
      healthy: true,
      next_patrol_at: '2099-06-30T14:05:00Z',
      resources_checked: 4,
      running: false,
      runtime_state: 'active',
    });
    mockGetPatrolRunHistory.mockResolvedValue([
      {
        error_count: 0,
        resources_checked: 4,
        status: 'healthy',
      },
    ]);
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

  it('renders monitor-context Patrol coverage without using the Patrol empty-work strip', async () => {
    setResources([
      makeResource({
        id: 'agent:pve-1',
        type: 'agent',
        proxmox: { nodeName: 'pve-1', clusterName: 'homelab' },
      }),
    ]);

    render(() => <ProxmoxPageSurface />);

    await waitFor(() => expect(mockGetPatrolStatus).toHaveBeenCalled());
    const posture = await screen.findByRole('list', { name: 'Proxmox Patrol coverage' });
    expect(mockLoadPatrolFindings).toHaveBeenCalled();
    expect(mockLoadPendingApprovals).toHaveBeenCalled();
    expect(posture).toHaveTextContent('Patrol checked 4 resources');
    expect(posture).toHaveTextContent('No Patrol work waiting');
    expect(posture).toHaveTextContent('Next check scheduled');
    expect(screen.queryByText('Protection current')).not.toBeInTheDocument();
    expect(
      screen.queryByRole('list', { name: 'Patrol protection posture' }),
    ).not.toBeInTheDocument();
  });

  it('does not crash when Patrol state refreshes before Proxmox resources hydrate', async () => {
    setResourcesSnapshot(undefined, true);

    render(() => <ProxmoxPageSurface />);

    expect(screen.getByTestId('platform-table-loading-state')).toBeInTheDocument();
    await waitFor(() => expect(mockLoadPatrolFindings).toHaveBeenCalled());
    expect(screen.queryByRole('list', { name: 'Proxmox Patrol coverage' })).not.toBeInTheDocument();
  });

  it('does not render monitor-context Patrol coverage when active Patrol work exists', () => {
    mockPatrolState.findings = [{ status: 'active' }];
    setResources([
      makeResource({
        id: 'agent:pve-1',
        type: 'agent',
        proxmox: { nodeName: 'pve-1', clusterName: 'homelab' },
      }),
    ]);

    render(() => <ProxmoxPageSurface />);

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
