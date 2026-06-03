import { cleanup, render, screen } from '@solidjs/testing-library';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import type { Resource } from '@/types/resource';
import { VmwarePageSurface } from '../VmwarePageSurface';

const mockUseUnifiedResources = vi.fn();
const mockPathname = vi.hoisted(() => vi.fn(() => '/vmware/overview'));
const mockVersionInfo = vi.hoisted(() => vi.fn());
const mockGetGlobalTimeline = vi.hoisted(() => vi.fn());

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

const setResources = (resources: Resource[]) => {
  mockUseUnifiedResources.mockReturnValue({
    resources: () => resources,
    loading: () => false,
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

vi.mock('@/api/resources', () => ({
  ResourceAPI: {
    getGlobalTimeline: (...args: unknown[]) => mockGetGlobalTimeline(...args),
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
  VsphereHostsTable: (props: { hosts: Resource[] }) => (
    <div data-testid="hosts-table" data-rows={props.hosts.length} />
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
    setResources([]);
  });

  afterEach(() => {
    cleanup();
    vi.clearAllMocks();
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
    expect(notice).toHaveTextContent('app-01 is running an older Pulse agent (v5.1.34).');
    expect(notice).toHaveTextContent(
      'latest in-guest telemetry and command support on this VM',
    );
    expect(screen.getByRole('link', { name: 'Open agent upgrade commands' })).toHaveAttribute(
      'href',
      '/settings/infrastructure?agentUpdates=1&agents=agent%3Aagent-app-01',
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
});
