import { cleanup, render, screen } from '@solidjs/testing-library';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import type { Resource } from '@/types/resource';
import { DockerPageSurface } from '../DockerPageSurface';

const mocks = vi.hoisted(() => ({
  useUnifiedResources: vi.fn(),
  useWorkloadsState: vi.fn(),
}));

vi.mock('@/hooks/useUnifiedResources', () => ({
  useUnifiedResources: mocks.useUnifiedResources,
}));

vi.mock('@/components/Workloads/useWorkloadsState', () => ({
  useWorkloadsState: mocks.useWorkloadsState,
}));

vi.mock('@/components/Workloads/WorkloadsSurface', () => ({
  WorkloadsSurface: (props: {
    groupNodeDrawerMode?: string;
    state?: { groupNodeDrawerMode?: () => string };
  }) => (
    <div
      data-testid="docker-workloads-surface"
      data-group-node-drawer-mode={props.groupNodeDrawerMode ?? ''}
      data-state-group-node-drawer-mode={props.state?.groupNodeDrawerMode?.() ?? ''}
    />
  ),
}));

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
  } as NonNullable<Resource['docker']>,
  ...overrides,
});

const makeWorkloadsState = (props: { groupNodeDrawerMode?: 'inline' | 'disabled' }) =>
  ({
    allGuests: () => [],
    clearPinnedSummaryScope: vi.fn(),
    containerRuntime: () => '',
    containerRuntimeFilterConfig: () => undefined,
    focusedSummaryWorkloadGroupId: () => null,
    groupNodeDrawerMode: () => props.groupNodeDrawerMode ?? 'inline',
    groupingMode: () => 'grouped',
    handleBeforeAutoFocus: vi.fn(),
    hostFilterConfig: () => undefined,
    search: () => '',
    selectedGuestId: () => null,
    selectedNode: () => null,
    setGroupingMode: vi.fn(),
    setMetricDisplayMode: vi.fn(),
    setSearch: vi.fn(),
    setSortDirection: vi.fn(),
    setSortKey: vi.fn(),
    setStatusMode: vi.fn(),
    setViewMode: vi.fn(),
    setWorkloadMetricHistoryRange: vi.fn(),
    statusMode: () => 'all',
    surfaceConnected: () => false,
    surfaceInitialDataReceived: () => false,
    viewMode: () => 'app-container',
    workloadMetricDisplayMode: () => 'bars',
    workloadMetricHistoryRange: () => '1h',
    workloadsFilterColumnVisibility: () => undefined,
  }) as const;

beforeEach(() => {
  mocks.useUnifiedResources.mockReturnValue({
    error: () => null,
    loading: () => false,
    refetch: vi.fn(),
    resources: () => [makeDockerHost(), makeDockerContainer()],
  });
  mocks.useWorkloadsState.mockImplementation(makeWorkloadsState);
});

afterEach(() => {
  cleanup();
  vi.clearAllMocks();
});

describe('DockerPageSurface', () => {
  it('keeps host drawer ownership on the Docker hosts table instead of workload groups', () => {
    render(() => <DockerPageSurface />);

    expect(mocks.useWorkloadsState).toHaveBeenCalledWith(
      expect.objectContaining({
        compactGroupHeaders: true,
        forcedPlatform: 'docker',
        forcedViewMode: 'app-container',
        groupNodeDrawerMode: 'disabled',
        tableOnly: true,
      }),
    );
    expect(screen.getByTestId('docker-workloads-surface')).toHaveAttribute(
      'data-group-node-drawer-mode',
      'disabled',
    );
    expect(screen.getByTestId('docker-workloads-surface')).toHaveAttribute(
      'data-state-group-node-drawer-mode',
      'disabled',
    );
  });
});
