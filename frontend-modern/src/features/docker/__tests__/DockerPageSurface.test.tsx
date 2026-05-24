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

vi.mock('@/features/platformPage/sharedPlatformPage', async () => {
  const actual = await vi.importActual<typeof import('@/features/platformPage/sharedPlatformPage')>(
    '@/features/platformPage/sharedPlatformPage',
  );
  return {
    ...actual,
    PlatformSectionTabs: () => <div data-testid="docker-section-tabs" />,
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
});
