import { cleanup, fireEvent, render, screen } from '@solidjs/testing-library';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import type { Resource } from '@/types/resource';
import { DockerPageSurface } from '../DockerPageSurface';

const mocks = vi.hoisted(() => ({
  pathname: '/docker/overview',
  searchParams: {} as Record<string, string>,
  useUnifiedResources: vi.fn(),
  DockerHostsTable: vi.fn(
    (props: { resources: Resource[]; showToolbar?: boolean; emptyTitle: string }) => (
      <div
        data-testid="docker-hosts-table"
        data-resource-count={props.resources.length}
        data-show-toolbar={String(props.showToolbar)}
      >
        {props.emptyTitle}
      </div>
    ),
  ),
  DockerContainersTable: vi.fn(
    (props: { resources: Resource[]; showToolbar?: boolean; emptyTitle: string }) => (
      <div
        data-testid="docker-containers-table"
        data-resource-count={props.resources.length}
        data-show-toolbar={String(props.showToolbar)}
      >
        {props.emptyTitle}
      </div>
    ),
  ),
  DockerStorageUsageTable: vi.fn(
    (props: {
      hosts: Resource[];
      showToolbar?: boolean;
      externalSearch?: () => string;
      externalStatus?: () => string;
    }) => (
      <div
        data-testid="docker-storage-usage-table"
        data-host-count={props.hosts.length}
        data-show-toolbar={String(props.showToolbar)}
        data-external-search={props.externalSearch?.() ?? ''}
        data-external-status={props.externalStatus?.() ?? ''}
      />
    ),
  ),
  DockerVolumesTable: vi.fn(
    (props: {
      resources: Resource[];
      showToolbar?: boolean;
      externalSearch?: () => string;
      externalStatus?: () => string;
    }) => (
      <div
        data-testid="docker-volumes-table"
        data-resource-count={props.resources.length}
        data-show-toolbar={String(props.showToolbar)}
        data-external-search={props.externalSearch?.() ?? ''}
        data-external-status={props.externalStatus?.() ?? ''}
      />
    ),
  ),
}));

vi.mock('@/hooks/useUnifiedResources', () => ({
  useUnifiedResources: mocks.useUnifiedResources,
}));

vi.mock('@/stores/updates', () => ({
  updateStore: {
    versionInfo: () => ({
      version: '6.0.0-rc.6',
      agentUpdateTargetVersion: '6.0.0-rc.6',
    }),
  },
}));

vi.mock('@solidjs/router', () => ({
  useLocation: () => ({
    get pathname() {
      return mocks.pathname;
    },
  }),
  useSearchParams: () => [mocks.searchParams, vi.fn()],
}));

vi.mock('../DockerContainersTable', () => ({
  DockerContainersTable: mocks.DockerContainersTable,
}));

vi.mock('../DockerHostsTable', () => ({
  DockerHostsTable: mocks.DockerHostsTable,
}));

vi.mock('../DockerStorageUsageTable', () => ({
  DockerStorageUsageTable: mocks.DockerStorageUsageTable,
}));

vi.mock('../DockerVolumesTable', () => ({
  DockerVolumesTable: mocks.DockerVolumesTable,
}));

vi.mock('@/features/platformPage/sharedPlatformPage', async () => {
  const actual = await vi.importActual<typeof import('@/features/platformPage/sharedPlatformPage')>(
    '@/features/platformPage/sharedPlatformPage',
  );
  return {
    ...actual,
    PlatformSectionTabs: (props: {
      active: string;
      tabs: Array<{ id: string; label: string; path: string }>;
    }) => (
      <div
        data-testid="docker-section-tabs"
        data-active={props.active}
        data-tabs={props.tabs.map((tab) => tab.id).join(',')}
      />
    ),
    PlatformTableEmptyState: (props: { title: string; description: string }) => (
      <div data-testid="platform-table-empty-state" data-title={props.title}>
        {props.description}
      </div>
    ),
  };
});

const makeDockerHost = (overrides: Partial<Resource> = {}): Resource => ({
  id: 'agent:docker-01',
  name: 'docker-01',
  displayName: 'docker-01',
  platformId: 'lab',
  platformType: 'docker',
  sourceType: 'agent',
  status: 'online',
  type: 'agent',
  lastSeen: 1_700_000_000_000,
  docker: {
    runtime: 'docker',
    runtimeVersion: '27.5.1',
    containerCount: 4,
  } as NonNullable<Resource['docker']>,
  ...overrides,
});

const makeDockerContainer = (overrides: Partial<Resource> = {}): Resource => ({
  id: 'app-container:docker-01:web',
  name: 'web',
  displayName: 'web',
  platformId: 'lab',
  platformType: 'docker',
  sourceType: 'agent',
  status: 'running',
  type: 'app-container',
  lastSeen: 1_700_000_000_000,
  docker: {
    runtime: 'docker',
    hostname: 'docker-01',
    image: 'nginx:latest',
  } as NonNullable<Resource['docker']>,
  ...overrides,
});

const makeDockerVolume = (overrides: Partial<Resource> = {}): Resource => ({
  id: 'docker-volume:docker-01:checkout-data',
  name: 'checkout-data',
  displayName: 'checkout-data',
  platformId: 'lab',
  platformType: 'docker',
  sourceType: 'agent',
  status: 'online',
  type: 'docker-volume',
  lastSeen: 1_700_000_000_000,
  docker: {
    runtime: 'docker',
    driver: 'local',
  } as NonNullable<Resource['docker']>,
  ...overrides,
});

const makeDockerImage = (overrides: Partial<Resource> = {}): Resource => ({
  id: 'docker-image:docker-01:nginx',
  name: 'nginx:latest',
  displayName: 'nginx:latest',
  platformId: 'lab',
  platformType: 'docker',
  sourceType: 'agent',
  status: 'online',
  type: 'docker-image',
  lastSeen: 1_700_000_000_000,
  docker: {
    runtime: 'docker',
    image: 'nginx:latest',
  } as NonNullable<Resource['docker']>,
  ...overrides,
});

const makeDockerNetwork = (overrides: Partial<Resource> = {}): Resource => ({
  id: 'docker-network:docker-01:frontend',
  name: 'frontend',
  displayName: 'frontend',
  platformId: 'lab',
  platformType: 'docker',
  sourceType: 'agent',
  status: 'online',
  type: 'docker-network',
  lastSeen: 1_700_000_000_000,
  docker: {
    runtime: 'docker',
    driver: 'bridge',
  } as NonNullable<Resource['docker']>,
  ...overrides,
});

beforeEach(() => {
  mocks.pathname = '/docker/overview';
  mocks.searchParams = {};
  mocks.useUnifiedResources.mockReturnValue({
    error: () => null,
    loading: () => false,
    refetch: vi.fn(),
    resources: () => [makeDockerHost(), makeDockerContainer()],
  });
});

afterEach(() => {
  cleanup();
  vi.clearAllMocks();
});

describe('DockerPageSurface', () => {
  it('explains the direct host and Proxmox LXC Docker install paths when empty', () => {
    mocks.useUnifiedResources.mockReturnValue({
      error: () => null,
      loading: () => false,
      refetch: vi.fn(),
      resources: () => [],
    });

    render(() => <DockerPageSurface />);

    const emptyState = screen.getByTestId('platform-table-empty-state');
    expect(emptyState).toHaveAttribute('data-title', 'No Docker or Podman hosts');
    expect(emptyState).toHaveTextContent('Install the Pulse agent on a Docker or Podman host');
    expect(emptyState).toHaveTextContent('Docker inside Proxmox LXCs');
    expect(emptyState).toHaveTextContent('command execution enabled');
    expect(emptyState).toHaveTextContent('Proxmox guest Docker inventory');
  });

  it('keeps overview focused on runtime hosts plus primary container workloads', () => {
    render(() => <DockerPageSurface />);

    expect(mocks.useUnifiedResources).toHaveBeenCalledWith(
      expect.objectContaining({
        query: expect.stringContaining('docker-swarm-node'),
      }),
    );
    expect(mocks.useUnifiedResources).toHaveBeenCalledWith(
      expect.objectContaining({
        query: expect.stringContaining('docker-secret'),
      }),
    );
    expect(mocks.useUnifiedResources).toHaveBeenCalledWith(
      expect.objectContaining({
        query: expect.stringContaining('docker-config'),
      }),
    );
    expect(screen.getByTestId('docker-hosts-table')).toHaveAttribute('data-resource-count', '1');
    expect(screen.getByTestId('docker-hosts-table')).toHaveAttribute('data-show-toolbar', 'false');
    expect(screen.getByTestId('docker-section-tabs')).toHaveAttribute('data-tabs', 'overview');
    expect(screen.getByTestId('docker-containers-table')).toHaveAttribute(
      'data-resource-count',
      '1',
    );
    expect(screen.getByTestId('docker-containers-table')).toHaveAttribute(
      'data-show-toolbar',
      'undefined',
    );
  });

  it('applies the URL host scope to the overview host table', () => {
    mocks.searchParams = { host: 'frigate.mist-stork.ts.net' };
    mocks.useUnifiedResources.mockReturnValue({
      error: () => null,
      loading: () => false,
      refetch: vi.fn(),
      resources: () => [
        makeDockerHost({
          id: 'agent:frigate',
          name: 'frigate',
          displayName: 'frigate',
          docker: {
            runtime: 'docker',
            runtimeVersion: '29.5.1',
            hostname: 'frigate.mist-stork.ts.net',
            containerCount: 2,
          } as NonNullable<Resource['docker']>,
        }),
        makeDockerHost({
          id: 'agent:tower',
          name: 'tower',
          displayName: 'tower',
          docker: {
            runtime: 'docker',
            runtimeVersion: '29.5.2',
            hostname: 'tower',
            containerCount: 1,
          } as NonNullable<Resource['docker']>,
        }),
        makeDockerContainer({
          id: 'app-container:frigate',
          docker: {
            runtime: 'docker',
            hostname: 'frigate.mist-stork.ts.net',
            image: 'ghcr.io/blakeblackshear/frigate:stable',
          } as NonNullable<Resource['docker']>,
        }),
      ],
    });

    render(() => <DockerPageSurface />);

    expect(screen.getByTestId('docker-hosts-table')).toHaveAttribute('data-resource-count', '1');
    expect(screen.getByTestId('docker-containers-table')).toHaveAttribute(
      'data-resource-count',
      '1',
    );
  });

  it('routes stale agent notices to the agent install commands', () => {
    mocks.useUnifiedResources.mockReturnValue({
      error: () => null,
      loading: () => false,
      refetch: vi.fn(),
      resources: () => [
        makeDockerHost({
          name: 'docker-old-agent',
          agent: { agentId: 'agent-docker-old', agentVersion: 'v5.1.34' },
        }),
      ],
    });

    render(() => <DockerPageSurface />);

    expect(screen.getByRole('link', { name: 'Open agent upgrade commands' })).toHaveAttribute(
      'href',
      '/settings/infrastructure?agentUpdates=1&agents=agent%3Aagent-docker-old',
    );
  });

  it('shows Docker object tabs only when the matching inventory exists', () => {
    mocks.useUnifiedResources.mockReturnValue({
      error: () => null,
      loading: () => false,
      refetch: vi.fn(),
      resources: () => [
        makeDockerHost(),
        makeDockerContainer(),
        makeDockerImage(),
        makeDockerVolume(),
        makeDockerNetwork(),
      ],
    });

    render(() => <DockerPageSurface />);

    expect(screen.getByTestId('docker-section-tabs')).toHaveAttribute(
      'data-tabs',
      'overview,images,storage,networks',
    );
  });

  it('keeps the legacy containers route on the Overview landing surface', () => {
    mocks.pathname = '/docker/containers';

    render(() => <DockerPageSurface />);

    expect(screen.getByTestId('docker-section-tabs')).toHaveAttribute('data-active', 'overview');
    expect(screen.getByTestId('docker-hosts-table')).toHaveAttribute('data-resource-count', '1');
    expect(screen.getByTestId('docker-containers-table')).toHaveAttribute(
      'data-resource-count',
      '1',
    );
    expect(screen.getByTestId('docker-containers-table')).toHaveAttribute(
      'data-show-toolbar',
      'undefined',
    );
  });

  it('shows the Swarm tab only when Docker hosts report Swarm evidence', () => {
    mocks.useUnifiedResources.mockReturnValue({
      error: () => null,
      loading: () => false,
      refetch: vi.fn(),
      resources: () => [
        makeDockerHost({
          docker: {
            runtime: 'docker',
            swarm: {
              nodeId: 'node-1',
              nodeRole: 'manager',
              localState: 'active',
            },
          } as NonNullable<Resource['docker']>,
        }),
      ],
    });

    render(() => <DockerPageSurface />);

    expect(screen.getByTestId('docker-section-tabs')).toHaveAttribute(
      'data-tabs',
      'overview,swarm',
    );
  });

  it('falls back to Overview when the Swarm route is requested without Swarm evidence', () => {
    mocks.pathname = '/docker/swarm';

    render(() => <DockerPageSurface />);

    expect(screen.getByTestId('docker-section-tabs')).toHaveAttribute('data-active', 'overview');
    expect(screen.getByTestId('docker-hosts-table')).toHaveAttribute('data-resource-count', '1');
  });

  it('renders only volume storage inventory when engine storage usage is absent', () => {
    mocks.pathname = '/docker/storage';
    mocks.useUnifiedResources.mockReturnValue({
      error: () => null,
      loading: () => false,
      refetch: vi.fn(),
      resources: () => [makeDockerHost(), makeDockerVolume()],
    });

    render(() => <DockerPageSurface />);

    expect(screen.queryByTestId('docker-storage-usage-table')).toBeNull();
    expect(screen.getByTestId('docker-volumes-table')).toHaveAttribute('data-resource-count', '1');
    expect(screen.queryByTestId('platform-table-empty-state')).toBeNull();
  });

  it('renders only engine storage usage when volume inventory is absent', () => {
    mocks.pathname = '/docker/storage';
    mocks.useUnifiedResources.mockReturnValue({
      error: () => null,
      loading: () => false,
      refetch: vi.fn(),
      resources: () => [
        makeDockerHost({
          docker: {
            runtime: 'docker',
            imagesUsage: {
              totalCount: 1,
              totalSizeBytes: 1024,
            },
          } as NonNullable<Resource['docker']>,
        }),
      ],
    });

    render(() => <DockerPageSurface />);

    expect(screen.getByTestId('docker-storage-usage-table')).toHaveAttribute(
      'data-host-count',
      '1',
    );
    expect(screen.queryByTestId('docker-volumes-table')).toBeNull();
    expect(screen.queryByTestId('platform-table-empty-state')).toBeNull();
  });

  it('uses one Storage toolbar to filter engine storage usage and volumes together', () => {
    mocks.pathname = '/docker/storage';
    mocks.useUnifiedResources.mockReturnValue({
      error: () => null,
      loading: () => false,
      refetch: vi.fn(),
      resources: () => [
        makeDockerHost({
          id: 'agent:docker-storage',
          name: 'engine-alpha',
          docker: {
            runtime: 'docker',
            imagesUsage: {
              totalCount: 2,
              totalSizeBytes: 2048,
            },
          } as NonNullable<Resource['docker']>,
        }),
        makeDockerVolume({
          id: 'docker-volume:docker-01:checkout-data',
          name: 'checkout-data',
          displayName: 'checkout-data',
          status: 'offline',
          docker: {
            runtime: 'docker',
            driver: 'local',
            mountpoint: '/var/lib/docker/volumes/checkout-data/_data',
          } as NonNullable<Resource['docker']>,
        }),
      ],
    });

    render(() => <DockerPageSurface />);

    const search = screen.getByPlaceholderText('Search storage usage and volumes');
    expect(screen.queryByPlaceholderText('Search storage usage')).toBeNull();
    expect(screen.queryByPlaceholderText('Search volumes')).toBeNull();
    expect(screen.getByText('2 rows')).toBeInTheDocument();

    expect(screen.getByTestId('docker-storage-usage-table')).toHaveAttribute(
      'data-show-toolbar',
      'false',
    );
    expect(screen.getByTestId('docker-volumes-table')).toHaveAttribute(
      'data-show-toolbar',
      'false',
    );
    expect(screen.getByTestId('docker-storage-usage-table')).toHaveAttribute(
      'data-external-status',
      'all',
    );
    expect(screen.getByTestId('docker-volumes-table')).toHaveAttribute(
      'data-external-status',
      'all',
    );

    fireEvent.input(search, { target: { value: 'checkout' } });

    expect(search).toHaveValue('checkout');
    expect(screen.getByText('1 of 2 rows')).toBeInTheDocument();
    expect(screen.getByTestId('docker-storage-usage-table')).toHaveAttribute(
      'data-external-search',
      'checkout',
    );
    expect(screen.getByTestId('docker-volumes-table')).toHaveAttribute(
      'data-external-search',
      'checkout',
    );

    fireEvent.click(screen.getByRole('button', { name: 'Offline' }));

    expect(screen.getByText('1 of 2 rows')).toBeInTheDocument();
    expect(screen.getByTestId('docker-storage-usage-table')).toHaveAttribute(
      'data-external-status',
      'offline',
    );
    expect(screen.getByTestId('docker-volumes-table')).toHaveAttribute(
      'data-external-status',
      'offline',
    );

    fireEvent.click(screen.getByRole('button', { name: 'Clear all' }));

    expect(search).toHaveValue('');
    expect(screen.getByText('2 rows')).toBeInTheDocument();
    expect(screen.getByTestId('docker-storage-usage-table')).toHaveAttribute(
      'data-external-status',
      'all',
    );
    expect(screen.getByTestId('docker-volumes-table')).toHaveAttribute(
      'data-external-status',
      'all',
    );
  });

  it('falls back to Overview when the Storage route is requested without storage evidence', () => {
    mocks.pathname = '/docker/storage';
    mocks.useUnifiedResources.mockReturnValue({
      error: () => null,
      loading: () => false,
      refetch: vi.fn(),
      resources: () => [makeDockerHost()],
    });

    render(() => <DockerPageSurface />);

    expect(screen.queryByTestId('docker-storage-usage-table')).toBeNull();
    expect(screen.queryByTestId('docker-volumes-table')).toBeNull();
    expect(screen.getByTestId('docker-section-tabs')).toHaveAttribute('data-active', 'overview');
    expect(screen.getByTestId('docker-hosts-table')).toHaveAttribute('data-resource-count', '1');
    expect(screen.getByTestId('docker-containers-table')).toHaveAttribute(
      'data-resource-count',
      '0',
    );
  });
});
