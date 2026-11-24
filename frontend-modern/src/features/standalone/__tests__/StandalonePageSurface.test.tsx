import { cleanup, render, screen } from '@solidjs/testing-library';
import type { JSX } from 'solid-js';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import type { Resource } from '@/types/resource';
import { StandalonePageSurface } from '../StandalonePageSurface';

const mocks = vi.hoisted(() => ({
  pathname: '/standalone/machines',
  searchParams: {} as Record<string, string>,
  setSearchParams: vi.fn(),
  navigate: vi.fn(),
  useUnifiedResources: vi.fn(),
  versionInfo: vi.fn(),
  AgentsMachinesTable: vi.fn(
    (props: {
      resources: Resource[];
      vmCountByMachineId?: () => ReadonlyMap<string, number>;
      onOpenHostVms?: (machine: Resource) => void;
      externalSearch?: () => string;
      onExternalSearchChange?: (value: string) => void;
      externalStatus?: () => string;
      onExternalStatusChange?: (value: string) => void;
      onResetFilters?: () => void;
    }) => <div data-testid="agents-machines-table" data-resource-count={props.resources.length} />,
  ),
  StandaloneVirtualMachinesTable: vi.fn(
    (props: {
      vms: Resource[];
      machines: Resource[];
      selectedHostId?: () => string | undefined;
      onClearHostScope?: () => void;
      externalSearch?: () => string;
      onExternalSearchChange?: (value: string) => void;
      externalStatus?: () => string;
      onExternalStatusChange?: (value: string) => void;
      onResetFilters?: () => void;
    }) => (
      <div
        data-testid="standalone-vms-table-mock"
        data-vm-count={props.vms.length}
        data-host-scope={props.selectedHostId?.() ?? ''}
      />
    ),
  ),
  AvailabilityChecksTable: vi.fn(
    (props: {
      resources: Resource[];
      view?: string;
      onViewChange?: (value: 'table' | 'fleet') => void;
    }) => (
      <div
        data-testid="availability-checks-table"
        data-resource-count={props.resources.length}
        data-view={props.view}
      />
    ),
  ),
}));

vi.mock('@/hooks/useUnifiedResources', () => ({
  useUnifiedResources: mocks.useUnifiedResources,
}));

vi.mock('@/stores/updates', () => ({
  updateStore: {
    versionInfo: mocks.versionInfo,
    lastError: () => null,
  },
}));

vi.mock('@solidjs/router', async () => {
  const actual = await vi.importActual<typeof import('@solidjs/router')>('@solidjs/router');
  return {
    ...actual,
    useLocation: () => ({
      get pathname() {
        return mocks.pathname;
      },
    }),
    useNavigate: () => mocks.navigate,
    useSearchParams: () => [mocks.searchParams, mocks.setSearchParams],
    A: (props: { href: string; children: JSX.Element }) => (
      <a href={props.href}>{props.children}</a>
    ),
  };
});

vi.mock('../AgentsMachinesTable', () => ({
  AgentsMachinesTable: mocks.AgentsMachinesTable,
}));

vi.mock('../StandaloneVirtualMachinesTable', () => ({
  StandaloneVirtualMachinesTable: mocks.StandaloneVirtualMachinesTable,
}));

vi.mock('../AvailabilityChecksTable', () => ({
  AvailabilityChecksTable: mocks.AvailabilityChecksTable,
  resolveAvailabilityChecksView: (
    routeValue: string | string[] | undefined,
    checkCount: number,
  ) => {
    const selectedValue = Array.isArray(routeValue) ? routeValue[0] : routeValue;
    if (selectedValue === 'table' || selectedValue === 'fleet') return selectedValue;
    return checkCount >= 20 ? 'fleet' : 'table';
  },
}));

vi.mock('@/features/platformPage/sharedPlatformPage', async () => {
  const actual = await vi.importActual<typeof import('@/features/platformPage/sharedPlatformPage')>(
    '@/features/platformPage/sharedPlatformPage',
  );
  return {
    ...actual,
    PlatformErrorState: () => <div data-testid="platform-error-state" />,
    PlatformSectionTabs: (props: {
      active: string;
      tabs: Array<{ id: string; label: string; path: string }>;
    }) => (
      <div
        data-testid="standalone-section-tabs"
        data-active={props.active}
        data-tabs={props.tabs.map((tab) => tab.id).join(',')}
      />
    ),
    PlatformTableEmptyState: (props: { title: string; actions?: JSX.Element }) => (
      <div data-testid="platform-table-empty-state">
        <span>{props.title}</span>
        {props.actions}
      </div>
    ),
    PlatformTableLoadingState: () => <div data-testid="platform-table-loading-state" />,
  };
});

const resource = (overrides: Partial<Resource>): Resource =>
  ({
    id: overrides.id ?? 'resource-1',
    name: overrides.name ?? overrides.id ?? 'resource-1',
    displayName: overrides.displayName ?? overrides.name ?? overrides.id ?? 'resource-1',
    type: overrides.type ?? 'agent',
    platformId: overrides.platformId ?? 'platform-1',
    platformType: overrides.platformType ?? 'agent',
    sourceType: overrides.sourceType ?? 'agent',
    status: overrides.status ?? 'online',
    lastSeen: overrides.lastSeen ?? Date.now(),
    ...overrides,
  }) as Resource;

const freshAvailability = (
  overrides: NonNullable<Resource['availability']> = {},
): NonNullable<Resource['availability']> => ({
  available: true,
  lastChecked: new Date().toISOString(),
  pollIntervalSeconds: 60,
  correlationState: 'standalone',
  ...overrides,
});

beforeEach(() => {
  mocks.pathname = '/standalone/machines';
  mocks.searchParams = {};
  mocks.setSearchParams.mockClear();
  mocks.navigate.mockClear();
  mocks.versionInfo.mockReturnValue(null);
  mocks.useUnifiedResources.mockReturnValue({
    resources: () => [
      resource({ id: 'linux-server', type: 'agent', platformType: 'agent', sources: ['agent'] }),
      resource({
        id: 'mac-mini',
        type: 'network-endpoint',
        platformType: 'availability',
        sources: ['availability'],
        availability: freshAvailability({ targetKind: 'machine' }),
      }),
      resource({
        id: 'mqtt-meter',
        type: 'network-endpoint',
        platformType: 'availability',
        sources: ['availability'],
        availability: freshAvailability({ targetKind: 'service' }),
      }),
    ],
    loading: () => false,
    error: () => null,
    refetch: vi.fn(),
  });
});

afterEach(() => {
  cleanup();
  vi.clearAllMocks();
  vi.unstubAllEnvs();
});

describe('StandalonePageSurface', () => {
  it('redirects legacy /standalone/availability links to /availability', () => {
    mocks.pathname = '/standalone/availability';
    render(() => <StandalonePageSurface />);
    expect(mocks.navigate).toHaveBeenCalledWith('/availability', { replace: true });
  });

  it('keeps machine search and normalized status in route-owned saved-view state', () => {
    mocks.searchParams = { q: 'ubuntu', status: 'running' };
    render(() => <StandalonePageSurface />);

    const props = mocks.AgentsMachinesTable.mock.calls.at(-1)?.[0];
    expect(props?.externalSearch?.()).toBe('ubuntu');
    expect(props?.externalStatus?.()).toBe('online');

    props?.onExternalSearchChange?.('macos');
    expect(mocks.setSearchParams).toHaveBeenCalledWith({ q: 'macos' }, { replace: true });

    props?.onExternalStatusChange?.('offline');
    expect(mocks.setSearchParams).toHaveBeenCalledWith({ status: 'offline' }, { replace: true });

    props?.onResetFilters?.();
    expect(mocks.setSearchParams).toHaveBeenCalledWith(
      { q: null, status: null },
      { replace: true },
    );
  });

  it('keeps overview focused on Pulse Agent machines only', () => {
    render(() => <StandalonePageSurface />);

    expect(mocks.useUnifiedResources).toHaveBeenCalledWith(
      expect.objectContaining({
        query:
          'source=agent,availability&excludeSource=proxmox,kubernetes,truenas,vmware&type=agent,vm,network-endpoint',
      }),
    );
    expect(screen.queryByTestId('standalone-section-tabs')).not.toBeInTheDocument();
    expect(screen.getByTestId('agents-machines-table')).toHaveAttribute('data-resource-count', '1');
    expect(screen.queryByTestId('standalone-posture-summary')).not.toBeInTheDocument();
    expect(screen.queryByTestId('availability-checks-table')).not.toBeInTheDocument();
  });

  it('keeps machine attention in the table instead of adding a page-wide posture banner', () => {
    mocks.useUnifiedResources.mockReturnValue({
      resources: () => [
        resource({
          id: 'tower',
          name: 'tower',
          type: 'agent',
          platformType: 'agent',
          sources: ['agent'],
          status: 'warning',
        }),
      ],
      loading: () => false,
      error: () => null,
      refetch: vi.fn(),
    });

    render(() => <StandalonePageSurface />);

    expect(screen.getByTestId('agents-machines-table')).toHaveAttribute('data-resource-count', '1');
    expect(screen.queryByTestId('standalone-posture-summary')).not.toBeInTheDocument();
  });

  it('surfaces stale Pulse Agent binaries on the Machines page', () => {
    mocks.versionInfo.mockReturnValue({
      version: 'v6.0.0-rc.6',
      agentUpdateTargetVersion: 'v6.0.0-rc.6',
    });
    mocks.useUnifiedResources.mockReturnValue({
      resources: () => [
        resource({
          id: 'tower',
          name: 'tower',
          type: 'agent',
          platformType: 'agent',
          sources: ['agent'],
          agent: { agentId: 'agent-tower', agentVersion: 'v5.1.34' },
        }),
      ],
      loading: () => false,
      error: () => null,
      refetch: vi.fn(),
    });

    render(() => <StandalonePageSurface />);

    const notice = screen.getByTestId('platform-outdated-agent-notice');
    expect(notice).toHaveTextContent('tower runs an older Pulse agent (v5.1.34).');
    expect(notice).toHaveTextContent('latest fixes and machine details');
    expect(screen.getByRole('link', { name: 'Open agent upgrade commands' })).toHaveAttribute(
      'href',
      '/settings/infrastructure/agent-doctor?agents=agent%3Aagent-tower',
    );
  });

  it('redirects retired Standalone overview links to the machines tab', () => {
    mocks.pathname = '/standalone/overview';

    render(() => <StandalonePageSurface />);

    expect(mocks.navigate).toHaveBeenCalledWith('/standalone/machines', { replace: true });
  });

  it('uses an overview handoff when only agentless availability checks are present', () => {
    mocks.useUnifiedResources.mockReturnValue({
      resources: () => [
        resource({
          id: 'mqtt-meter',
          type: 'network-endpoint',
          platformType: 'availability',
          sources: ['availability'],
          availability: { targetKind: 'service' },
        }),
      ],
      loading: () => false,
      error: () => null,
      refetch: vi.fn(),
    });

    render(() => <StandalonePageSurface />);

    expect(screen.queryByTestId('agents-machines-table')).not.toBeInTheDocument();
    expect(screen.queryByTestId('availability-checks-table')).not.toBeInTheDocument();
    expect(screen.getByText('No Pulse Agent machines')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Add agent' })).toBeInTheDocument();
    expect(screen.getByRole('link', { name: 'View checks' })).toHaveAttribute(
      'href',
      '/availability',
    );
    expect(screen.queryByRole('link', { name: 'Add machine check' })).not.toBeInTheDocument();
  });

  it('keeps agentless machine checks in the availability handoff', () => {
    mocks.useUnifiedResources.mockReturnValue({
      resources: () => [
        resource({
          id: 'mac-mini',
          type: 'network-endpoint',
          platformType: 'availability',
          sources: ['availability'],
          availability: { targetKind: 'machine' },
        }),
      ],
      loading: () => false,
      error: () => null,
      refetch: vi.fn(),
    });

    render(() => <StandalonePageSurface />);

    expect(screen.queryByTestId('agents-machines-table')).not.toBeInTheDocument();
    expect(screen.getByText('No Pulse Agent machines')).toBeInTheDocument();
    expect(screen.getByRole('link', { name: 'View checks' })).toHaveAttribute(
      'href',
      '/availability',
    );
  });

  it('reveals the evidence-gated VMs tab and wires host VM navigation when standalone VMs exist', async () => {
    const tower = resource({
      id: 'agent:tower',
      name: 'Tower',
      type: 'agent',
      platformType: 'agent',
      sources: ['agent'],
    });
    const pulseDev = resource({
      id: 'agent:tower:libvirt:domain-1',
      name: 'pulse-dev',
      type: 'vm',
      parentId: 'agent:tower',
      parentName: 'Tower',
      platformType: 'agent',
      sources: ['agent'],
      technology: 'libvirt',
      virtualMachine: { runtimeState: 'running', hypervisor: 'libvirt', vcpus: 8 },
    });
    mocks.useUnifiedResources.mockReturnValue({
      resources: () => [tower, pulseDev],
      loading: () => false,
      error: () => null,
      refetch: vi.fn(),
    });

    render(() => <StandalonePageSurface />);

    expect(screen.getByTestId('standalone-section-tabs')).toHaveAttribute(
      'data-tabs',
      'machines,vms',
    );
    const machineProps = mocks.AgentsMachinesTable.mock.calls.at(-1)?.[0];
    expect(machineProps?.vmCountByMachineId?.().get('agent:tower')).toBe(1);
    machineProps?.onOpenHostVms?.(tower);
    expect(mocks.navigate).toHaveBeenCalledWith('/standalone/vms?host=agent%3Atower');

    cleanup();
    mocks.pathname = '/standalone/vms';
    mocks.searchParams = { host: 'agent:tower', q: 'pulse', status: 'running' };
    render(() => <StandalonePageSurface />);

    expect(screen.getByTestId('standalone-section-tabs')).toHaveAttribute('data-active', 'vms');
    const vmTable = await screen.findByTestId('standalone-vms-table-mock');
    expect(vmTable).toHaveAttribute('data-vm-count', '1');
    expect(vmTable).toHaveAttribute('data-host-scope', 'agent:tower');

    const vmProps = mocks.StandaloneVirtualMachinesTable.mock.calls.at(-1)?.[0];
    expect(vmProps?.externalSearch?.()).toBe('pulse');
    expect(vmProps?.externalStatus?.()).toBe('running');
    vmProps?.onClearHostScope?.();
    expect(mocks.setSearchParams).toHaveBeenCalledWith({ host: null }, { replace: true });
    vmProps?.onResetFilters?.();
    expect(mocks.setSearchParams).toHaveBeenCalledWith(
      { q: null, status: null, host: null },
      { replace: true },
    );
  });

  it('keeps Machines usable with VM inventory in a release build', () => {
    vi.stubEnv('DEV', false);
    mocks.versionInfo.mockReturnValue({ channel: 'stable' });
    mocks.useUnifiedResources.mockReturnValue({
      resources: () => [
        resource({ id: 'tower', type: 'agent', platformType: 'agent', sources: ['agent'] }),
        resource({ type: 'vm', platformType: 'agent', sources: ['agent'], parentId: 'tower' }),
      ],
      loading: () => false,
      error: () => null,
      refetch: vi.fn(),
    });

    render(() => <StandalonePageSurface />);

    expect(screen.queryByTestId('standalone-section-tabs')).not.toBeInTheDocument();
    expect(screen.getByTestId('agents-machines-table')).toHaveAttribute('data-resource-count', '1');
    expect(screen.queryByTestId('standalone-vms-table-mock')).not.toBeInTheDocument();
    const machineProps = mocks.AgentsMachinesTable.mock.calls.at(-1)?.[0];
    expect(machineProps?.vmCountByMachineId?.().has('tower')).toBe(false);
    expect(machineProps?.onOpenHostVms).toBeUndefined();
  });

  it('does not treat a held VM as a reason to hide ordinary first-run actions', () => {
    vi.stubEnv('DEV', false);
    mocks.versionInfo.mockReturnValue({ channel: 'rc' });
    mocks.useUnifiedResources.mockReturnValue({
      resources: () => [resource({ type: 'vm', platformType: 'agent', sources: ['agent'] })],
      loading: () => false,
      error: () => null,
      refetch: vi.fn(),
    });

    render(() => <StandalonePageSurface />);

    expect(screen.getByText('No Pulse Agent machines')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Add agent' })).toBeInTheDocument();
    expect(screen.getByRole('link', { name: 'View checks' })).toHaveAttribute(
      'href',
      '/availability',
    );
  });
});
