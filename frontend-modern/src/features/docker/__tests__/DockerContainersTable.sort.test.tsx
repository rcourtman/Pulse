import { cleanup, fireEvent, render, screen } from '@solidjs/testing-library';
import { Route, Router } from '@solidjs/router';
import type { JSX } from 'solid-js';
import { afterEach, describe, expect, it, vi } from 'vitest';

import type { Resource } from '@/types/resource';
import { DockerContainersTable } from '../DockerContainersTable';

vi.mock('@/api/monitoring', () => ({
  MonitoringAPI: {
    updateDockerContainer: vi.fn().mockResolvedValue({ success: true }),
  },
}));

vi.mock('@/stores/notifications', () => ({
  notificationStore: {
    success: vi.fn(),
    error: vi.fn(),
  },
}));

vi.mock('@/stores/containerUpdates', () => ({
  clearContainerUpdateState: vi.fn(),
  getContainerUpdateState: vi.fn(() => undefined),
  markContainerQueued: vi.fn(),
  markContainerUpdateError: vi.fn(),
  markContainerUpdateSuccess: vi.fn(),
  updateStates: vi.fn(() => ({})),
}));

vi.mock('@/stores/systemSettings', () => ({
  areSystemSettingsLoaded: () => true,
  shouldHideDockerUpdateActions: () => false,
}));

vi.mock('@/components/shared/responsive', () => ({
  ResponsiveMetricCell: (props: { type: string; resourceId?: string }) => (
    <div data-testid={`responsive-${props.type}-metric`} data-resource-id={props.resourceId ?? ''} />
  ),
}));

vi.mock('@/components/Workloads/StackedMemoryBar', () => ({
  StackedMemoryBar: () => <div data-testid="stacked-memory-bar" />,
}));

const makeContainer = ({
  id,
  host,
  ...overrides
}: Partial<Resource> & { id: string; host: string }): Resource => ({
  id,
  name: id,
  displayName: id,
  platformId: 'docker-1',
  platformType: 'docker',
  sourceType: 'agent',
  sources: ['docker'],
  status: 'running',
  type: 'app-container',
  lastSeen: 1_700_000_000_000,
  ...overrides,
  docker: {
    agentId: `agent-${host}`,
    hostname: host,
    containerState: 'running',
    image: 'nginx:latest',
    ...(overrides.docker ?? {}),
  },
});

// Two hosts with mixed statuses so both the attention-first default order and
// the within-group user sort are observable.
const FIXTURE: Resource[] = [
  makeContainer({ id: 'alpha', host: 'host-a', cpu: { current: 20 } }),
  makeContainer({
    id: 'zulu',
    host: 'host-a',
    status: 'stopped',
    cpu: undefined,
    docker: { agentId: 'agent-host-a', hostname: 'host-a', containerState: 'exited', exitCode: 1 },
  }),
  makeContainer({ id: 'mike', host: 'host-a', cpu: { current: 80 } }),
  makeContainer({ id: 'bravo', host: 'host-b', cpu: { current: 10 } }),
  makeContainer({ id: 'yankee', host: 'host-b', cpu: { current: 55 } }),
];

const renderTable = () =>
  render(() => (
    <Router>
      <Route
        path="/"
        component={(() => (
          <DockerContainersTable
            resources={FIXTURE}
            emptyIcon={<span />}
            emptyTitle="No containers"
            emptyDescription="No containers"
            showToolbar={false}
          />
        )) as () => JSX.Element}
      />
    </Router>
  ));

// Row order including group boundaries: group header rows render as
// `group:<host>`, container rows as their id.
const visibleRowOrder = (container: HTMLElement): string[] =>
  Array.from(
    container.querySelectorAll('tr[data-docker-container-row], tr[data-docker-host-group]'),
  ).map((row) =>
    row.hasAttribute('data-docker-host-group')
      ? `group:${row.getAttribute('data-docker-host-group')}`
      : (row.getAttribute('data-docker-container-row') ?? ''),
  );

const headerFor = (label: string): HTMLElement => {
  const header = screen
    .getAllByRole('columnheader')
    .find((th) => th.textContent?.trim().startsWith(label));
  if (!header) throw new Error(`No column header labelled ${label}`);
  return header;
};

afterEach(() => {
  window.history.pushState({}, '', '/');
  window.localStorage.clear();
  cleanup();
  vi.clearAllMocks();
});

describe('DockerContainersTable user sorting', () => {
  it('sorts within host groups without changing the group order', () => {
    const { container } = renderTable();

    // Built-in order: attention first (zulu exited non-zero), then names.
    expect(visibleRowOrder(container)).toEqual([
      'group:host-a',
      'zulu',
      'alpha',
      'mike',
      'group:host-b',
      'bravo',
      'yankee',
    ]);

    fireEvent.click(headerFor('Container'));
    expect(headerFor('Container')).toHaveAttribute('aria-sort', 'ascending');
    expect(visibleRowOrder(container)).toEqual([
      'group:host-a',
      'alpha',
      'mike',
      'zulu',
      'group:host-b',
      'bravo',
      'yankee',
    ]);

    fireEvent.click(headerFor('Container'));
    expect(headerFor('Container')).toHaveAttribute('aria-sort', 'descending');
    expect(visibleRowOrder(container)).toEqual([
      'group:host-a',
      'zulu',
      'mike',
      'alpha',
      'group:host-b',
      'yankee',
      'bravo',
    ]);

    // Third click clears back to the built-in attention-first order.
    fireEvent.click(headerFor('Container'));
    expect(headerFor('Container')).not.toHaveAttribute('aria-sort');
    expect(visibleRowOrder(container)).toEqual([
      'group:host-a',
      'zulu',
      'alpha',
      'mike',
      'group:host-b',
      'bravo',
      'yankee',
    ]);
  });

  it('sorts metric columns descending first with missing values last', () => {
    const { container } = renderTable();

    fireEvent.click(headerFor('CPU'));
    expect(headerFor('CPU')).toHaveAttribute('aria-sort', 'descending');
    // zulu reports no CPU metric, so it sinks to the bottom of its group.
    expect(visibleRowOrder(container)).toEqual([
      'group:host-a',
      'mike',
      'alpha',
      'zulu',
      'group:host-b',
      'yankee',
      'bravo',
    ]);
  });

  it('persists the chosen sort across a remount', () => {
    const first = renderTable();
    fireEvent.click(headerFor('Container'));
    expect(visibleRowOrder(first.container)).toEqual([
      'group:host-a',
      'alpha',
      'mike',
      'zulu',
      'group:host-b',
      'bravo',
      'yankee',
    ]);
    expect(window.localStorage.getItem('dockerContainersSortKey')).toBe('container');
    expect(window.localStorage.getItem('dockerContainersSortDirection')).toBe('asc');

    cleanup();

    const second = renderTable();
    expect(headerFor('Container')).toHaveAttribute('aria-sort', 'ascending');
    expect(visibleRowOrder(second.container)).toEqual([
      'group:host-a',
      'alpha',
      'mike',
      'zulu',
      'group:host-b',
      'bravo',
      'yankee',
    ]);
  });
});
