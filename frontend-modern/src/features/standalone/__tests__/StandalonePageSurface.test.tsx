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
      externalStatus?: () => string;
      onExternalStatusChange?: (value: string) => void;
    }) => <div data-testid="agents-machines-table" data-resource-count={props.resources.length} />,
  ),
  AvailabilityChecksTable: vi.fn((props: { resources: Resource[] }) => (
    <div data-testid="availability-checks-table" data-resource-count={props.resources.length} />
  )),
}));

vi.mock('@/hooks/useUnifiedResources', () => ({
  useUnifiedResources: mocks.useUnifiedResources,
}));

vi.mock('@/stores/updates', () => ({
  updateStore: {
    versionInfo: mocks.versionInfo,
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

vi.mock('../AvailabilityChecksTable', () => ({
  AvailabilityChecksTable: mocks.AvailabilityChecksTable,
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
});

describe('StandalonePageSurface', () => {
  it('normalizes legacy machine status links into route-owned filter state', () => {
    mocks.searchParams = { status: 'running' };
    render(() => <StandalonePageSurface />);

    const props = mocks.AgentsMachinesTable.mock.calls.at(-1)?.[0];
    expect(props?.externalStatus?.()).toBe('online');
    props?.onExternalStatusChange?.('offline');
    expect(mocks.setSearchParams).toHaveBeenCalledWith({ status: 'offline' }, { replace: true });
  });

  it('keeps overview focused on Pulse Agent machines only', () => {
    render(() => <StandalonePageSurface />);

    expect(mocks.useUnifiedResources).toHaveBeenCalledWith(
      expect.objectContaining({
        query: 'type=agent,network-endpoint',
      }),
    );
    expect(screen.getByTestId('standalone-section-tabs')).toHaveAttribute(
      'data-tabs',
      'machines,availability',
    );
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
    expect(notice).toHaveTextContent('tower is running an older Pulse agent (v5.1.34).');
    expect(notice).toHaveTextContent(
      'latest agent command support and agent-managed platform detail',
    );
    expect(screen.getByRole('link', { name: 'Open agent upgrade commands' })).toHaveAttribute(
      'href',
      '/settings/infrastructure/agent-doctor?agents=agent%3Aagent-tower',
    );
  });

  it('redirects retired Standalone overview links to the machines tab', () => {
    mocks.pathname = '/standalone/overview';

    render(() => <StandalonePageSurface />);

    expect(mocks.navigate).toHaveBeenCalledWith('/standalone/machines', { replace: true });
    expect(screen.getByTestId('standalone-section-tabs')).toHaveAttribute(
      'data-active',
      'machines',
    );
  });

  it('uses the availability tab as a focused check monitor', () => {
    mocks.pathname = '/standalone/availability';

    render(() => <StandalonePageSurface />);

    expect(screen.getByTestId('standalone-section-tabs')).toHaveAttribute(
      'data-active',
      'availability',
    );
    expect(screen.queryByTestId('agents-machines-table')).not.toBeInTheDocument();
    expect(screen.getByTestId('availability-checks-table')).toHaveAttribute(
      'data-resource-count',
      '2',
    );
    expect(screen.getByTestId('standalone-posture-summary')).toHaveTextContent(
      'All 2 checks reporting normally',
    );
    expect(screen.getByRole('link', { name: 'Manage checks' })).toHaveAttribute(
      'href',
      '/settings/monitoring/availability',
    );
  });

  it('counts every configured check when two attached services share one monitored host', () => {
    mocks.pathname = '/standalone/availability';
    mocks.useUnifiedResources.mockReturnValue({
      resources: () => [
        resource({
          id: 'agent-core2026',
          type: 'agent',
          platformType: 'agent',
          sources: ['agent', 'availability'],
          availability: freshAvailability({
            targetId: 'stats-pv',
            correlationState: 'attached',
          }),
        }),
        resource({
          id: 'availability:stats-pv',
          type: 'network-endpoint',
          platformType: 'availability',
          sources: ['availability'],
          availability: freshAvailability({
            targetId: 'stats-pv',
            correlationState: 'attached',
          }),
        }),
        resource({
          id: 'availability:grafana',
          type: 'network-endpoint',
          platformType: 'availability',
          sources: ['availability'],
          availability: freshAvailability({
            targetId: 'grafana',
            correlationState: 'attached',
          }),
        }),
        resource({
          id: 'availability:public-api',
          type: 'network-endpoint',
          platformType: 'availability',
          sources: ['availability'],
          availability: freshAvailability({ targetId: 'public-api' }),
        }),
        resource({
          id: 'availability:router',
          type: 'network-endpoint',
          platformType: 'availability',
          sources: ['availability'],
          availability: freshAvailability({ targetId: 'router' }),
        }),
      ],
      loading: () => false,
      error: () => null,
      refetch: vi.fn(),
    });

    render(() => <StandalonePageSurface />);

    expect(screen.getByTestId('availability-checks-table')).toHaveAttribute(
      'data-resource-count',
      '4',
    );
    expect(screen.getByTestId('standalone-posture-summary')).toHaveTextContent(
      'All 4 checks reporting normally',
    );
  });

  it('makes failed availability posture visible before the table', () => {
    mocks.pathname = '/standalone/availability';
    mocks.useUnifiedResources.mockReturnValue({
      resources: () => [
        resource({
          id: 'failed-check',
          type: 'network-endpoint',
          platformType: 'availability',
          sources: ['availability'],
          status: 'offline',
          availability: freshAvailability({ targetKind: 'service', available: false }),
        }),
        resource({
          id: 'healthy-check',
          type: 'network-endpoint',
          platformType: 'availability',
          sources: ['availability'],
          status: 'online',
          availability: freshAvailability({ targetKind: 'service' }),
        }),
      ],
      loading: () => false,
      error: () => null,
      refetch: vi.fn(),
    });

    render(() => <StandalonePageSurface />);

    const summary = screen.getByTestId('standalone-posture-summary');
    expect(summary).toHaveTextContent('1 check offline');
    expect(summary).toHaveTextContent('1 need attention');
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
      '/standalone/availability',
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
      '/standalone/availability',
    );
  });
});
