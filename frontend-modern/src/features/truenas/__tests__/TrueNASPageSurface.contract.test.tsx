import { cleanup, render, screen } from '@solidjs/testing-library';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import type { Resource } from '@/types/resource';
import { TrueNASPageSurface } from '../TrueNASPageSurface';

const mockUseUnifiedResources = vi.fn();
const mockUseRecoveryPoints = vi.fn();
const mockPathname = vi.hoisted(() => vi.fn(() => '/truenas/overview'));
const mockVersionInfo = vi.hoisted(() => vi.fn());

const makeResource = (resource: Partial<Resource> & Pick<Resource, 'id' | 'type'>): Resource =>
  ({
    name: resource.id,
    displayName: resource.id,
    platformId: 'truenas-1',
    platformType: 'truenas',
    sourceType: 'agent',
    sources: ['agent', 'truenas'],
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

vi.mock('@/hooks/useRecoveryPoints', () => ({
  useRecoveryPoints: (...args: unknown[]) => mockUseRecoveryPoints(...args),
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

vi.mock('../TrueNASAlertsTable', () => ({
  TrueNASAlertsTable: (props: { incidents: unknown[] }) => (
    <div data-testid="alerts-table" data-rows={props.incidents.length} />
  ),
}));

vi.mock('../TrueNASAppsTable', () => ({
  TrueNASAppsTable: (props: { apps: Resource[] }) => (
    <div data-testid="apps-table" data-rows={props.apps.length} />
  ),
}));

vi.mock('../TrueNASNetworkSharesTable', () => ({
  TrueNASNetworkSharesTable: (props: { shares: Resource[] }) => (
    <div data-testid="shares-table" data-rows={props.shares.length} />
  ),
}));

vi.mock('../TrueNASProtectionTable', () => ({
  TrueNASProtectionTable: (props: { points: unknown[] }) => (
    <div data-testid="protection-table" data-rows={props.points.length} />
  ),
}));

vi.mock('../TrueNASServicesTable', () => ({
  TrueNASServicesTable: (props: { services: unknown[] }) => (
    <div data-testid="services-table" data-rows={props.services.length} />
  ),
}));

vi.mock('../TrueNASStorageTopologyTable', () => ({
  TrueNASStorageTopologyTable: (props: { resources: Resource[] }) => (
    <div data-testid="storage-table" data-rows={props.resources.length} />
  ),
}));

vi.mock('../TrueNASSystemsTable', () => ({
  TrueNASSystemsTable: (props: { systems: Resource[] }) => (
    <div data-testid="systems-table" data-rows={props.systems.length} />
  ),
}));

vi.mock('../TrueNASVirtualMachinesTable', () => ({
  TrueNASVirtualMachinesTable: (props: { vms: Resource[] }) => (
    <div data-testid="vms-table" data-rows={props.vms.length} />
  ),
}));

describe('TrueNASPageSurface contract', () => {
  beforeEach(() => {
    mockPathname.mockReturnValue('/truenas/overview');
    mockVersionInfo.mockReturnValue(null);
    mockUseRecoveryPoints.mockReturnValue({
      meta: () => ({ total: 0 }),
      points: () => [],
      response: { loading: false, error: null },
      refetch: vi.fn(),
    });
  });

  afterEach(() => {
    cleanup();
    vi.clearAllMocks();
  });

  it('surfaces stale agent-backed TrueNAS systems', () => {
    mockVersionInfo.mockReturnValue({
      version: 'v6.0.0-rc.6',
      agentUpdateTargetVersion: 'v6.0.0-rc.6',
    });
    setResources([
      makeResource({
        id: 'agent:truenas-scale',
        name: 'truenas-scale',
        type: 'agent',
        agent: { agentId: 'agent-truenas-scale', agentVersion: 'v5.1.34' },
      }),
    ]);

    render(() => <TrueNASPageSurface />);

    expect(screen.getByTestId('platform-section-tabs')).toHaveAttribute('data-active', 'overview');
    expect(screen.getByTestId('systems-table')).toHaveAttribute('data-rows', '1');
    const notice = screen.getByTestId('platform-outdated-agent-notice');
    expect(notice).toHaveTextContent('truenas-scale is running an older Pulse agent (v5.1.34).');
    expect(notice).toHaveTextContent(
      'latest agent-contributed TrueNAS system detail and command support',
    );
    expect(screen.getByRole('link', { name: 'Open agent upgrade commands' })).toHaveAttribute(
      'href',
      '/settings/infrastructure/agent-doctor?agents=agent%3Aagent-truenas-scale',
    );
  });
});
