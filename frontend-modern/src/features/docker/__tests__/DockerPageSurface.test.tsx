import { cleanup, render, screen } from '@solidjs/testing-library';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import type { Resource } from '@/types/resource';
import { DockerPageSurface } from '../DockerPageSurface';

const mocks = vi.hoisted(() => ({
  pathname: '/docker/overview',
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
  DockerStorageUsageTable: vi.fn((props: { hosts: Resource[] }) => (
    <div data-testid="docker-storage-usage-table" data-host-count={props.hosts.length} />
  )),
  DockerVolumesTable: vi.fn((props: { resources: Resource[] }) => (
    <div data-testid="docker-volumes-table" data-resource-count={props.resources.length} />
  )),
}));

vi.mock('@/hooks/useUnifiedResources', () => ({
  useUnifiedResources: mocks.useUnifiedResources,
}));

vi.mock('@solidjs/router', () => ({
  useLocation: () => ({
    get pathname() {
      return mocks.pathname;
    },
  }),
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

beforeEach(() => {
  mocks.pathname = '/docker/overview';
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
  it('keeps overview focused on runtime hosts while detailed inventory owns the object tabs', () => {
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
    expect(screen.getByTestId('docker-hosts-table')).toHaveAttribute(
      'data-resource-count',
      '1',
    );
    expect(screen.getByTestId('docker-hosts-table')).toHaveAttribute(
      'data-show-toolbar',
      'false',
    );
    expect(screen.getByTestId('docker-section-tabs')).toHaveAttribute(
      'data-tabs',
      'overview,containers,images,storage,networks',
    );
    expect(screen.queryByTestId('docker-containers-table')).toBeNull();
  });

  it('renders the dedicated containers route through the Docker containers table', () => {
    mocks.pathname = '/docker/containers';

    render(() => <DockerPageSurface />);

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
      'overview,containers,images,storage,networks,swarm',
    );
  });

  it('falls back to Overview when the Swarm route is requested without Swarm evidence', () => {
    mocks.pathname = '/docker/swarm';

    render(() => <DockerPageSurface />);

    expect(screen.getByTestId('docker-section-tabs')).toHaveAttribute('data-active', 'overview');
    expect(screen.getByTestId('docker-hosts-table')).toHaveAttribute(
      'data-resource-count',
      '1',
    );
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

  it('uses one Storage tab empty state when no storage inventory exists', () => {
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
    expect(screen.getAllByTestId('platform-table-empty-state')).toHaveLength(1);
    expect(screen.getByTestId('platform-table-empty-state')).toHaveAttribute(
      'data-title',
      'No Docker or Podman storage inventory',
    );
  });
});
