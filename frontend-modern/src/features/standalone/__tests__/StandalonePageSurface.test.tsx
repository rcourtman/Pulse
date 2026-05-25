import { cleanup, render, screen } from '@solidjs/testing-library';
import type { JSX } from 'solid-js';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import type { Resource } from '@/types/resource';
import { StandalonePageSurface } from '../StandalonePageSurface';

const mocks = vi.hoisted(() => ({
  pathname: '/standalone/machines',
  navigate: vi.fn(),
  useUnifiedResources: vi.fn(),
  AgentsMachinesTable: vi.fn((props: { resources: Resource[] }) => (
    <div data-testid="agents-machines-table" data-resource-count={props.resources.length} />
  )),
  AvailabilityChecksTable: vi.fn((props: { resources: Resource[] }) => (
    <div data-testid="availability-checks-table" data-resource-count={props.resources.length} />
  )),
}));

vi.mock('@/hooks/useUnifiedResources', () => ({
  useUnifiedResources: mocks.useUnifiedResources,
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

vi.mock('@/features/platformPage/sharedPlatformPage', () => ({
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
}));

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
    lastSeen: overrides.lastSeen ?? 1_700_000_000_000,
    ...overrides,
  }) as Resource;

beforeEach(() => {
  mocks.pathname = '/standalone/machines';
  mocks.navigate.mockClear();
  mocks.useUnifiedResources.mockReturnValue({
    resources: () => [
      resource({ id: 'linux-server', type: 'agent', platformType: 'agent', sources: ['agent'] }),
      resource({
        id: 'mac-mini',
        type: 'network-endpoint',
        platformType: 'availability',
        sources: ['availability'],
        availability: { targetKind: 'machine' },
      }),
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
});

afterEach(() => {
  cleanup();
  vi.clearAllMocks();
});

describe('StandalonePageSurface', () => {
  it('keeps overview focused on standalone machines, including agentless machines only', () => {
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
    expect(screen.getByTestId('agents-machines-table')).toHaveAttribute('data-resource-count', '2');
    expect(screen.queryByTestId('availability-checks-table')).not.toBeInTheDocument();
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
    expect(screen.getByText('No machines')).toBeInTheDocument();
    expect(screen.getByRole('link', { name: 'View checks' })).toHaveAttribute(
      'href',
      '/standalone/availability',
    );
  });

  it('renders the machines table when only an agentless machine is present', () => {
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

    expect(screen.getByTestId('agents-machines-table')).toHaveAttribute('data-resource-count', '1');
    expect(screen.queryByText('No machines')).not.toBeInTheDocument();
  });
});
