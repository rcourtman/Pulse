import { cleanup, fireEvent, render, screen } from '@solidjs/testing-library';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import type { Resource } from '@/types/resource';
import { AgentsMachinesTable } from '../AgentsMachinesTable';

vi.mock('@/components/Infrastructure/ResourceDetailDrawer', () => ({
  ResourceDetailDrawer: () => <div data-testid="resource-detail-drawer" />,
}));

vi.mock('@/components/Workloads/EnhancedCPUBar', () => ({
  EnhancedCPUBar: (props: {
    usage: number;
    loadAverage?: number;
    cores?: number;
    resourceId?: string;
  }) => (
    <div
      data-testid="agent-machine-cpu-bar"
      data-usage={props.usage}
      data-load-average={props.loadAverage}
      data-cores={props.cores}
      data-resource-id={props.resourceId}
    />
  ),
}));

vi.mock('@/components/Workloads/StackedMemoryBar', () => ({
  StackedMemoryBar: (props: {
    used: number;
    total: number;
    percentOnly?: number;
    balloon?: number;
    swapUsed?: number;
    swapTotal?: number;
    resourceId?: string;
  }) => (
    <div
      data-testid="agent-machine-memory-bar"
      data-used={props.used}
      data-total={props.total}
      data-percent-only={props.percentOnly}
      data-balloon={props.balloon}
      data-swap-used={props.swapUsed}
      data-swap-total={props.swapTotal}
      data-resource-id={props.resourceId}
    />
  ),
}));

const resource = (overrides: Partial<Resource>): Resource =>
  ({
    id: overrides.id ?? 'machine-1',
    name: overrides.name ?? overrides.id ?? 'machine-1',
    displayName: overrides.displayName ?? overrides.name ?? overrides.id ?? 'machine-1',
    type: 'agent',
    platformId: 'agent',
    platformType: 'agent',
    sourceType: 'agent',
    status: 'online',
    lastSeen: 1_700_000_000_000,
    ...overrides,
  }) as Resource;

const emptyIcon = <span data-testid="empty-icon" />;

beforeEach(() => {
  vi.stubGlobal(
    'ResizeObserver',
    class {
      observe = vi.fn();
      unobserve = vi.fn();
      disconnect = vi.fn();
    },
  );
});

afterEach(() => {
  cleanup();
  window.localStorage.clear();
  vi.unstubAllGlobals();
  vi.clearAllMocks();
});

describe('AgentsMachinesTable', () => {
  it('surfaces machine-native monitoring columns for agent machines', async () => {
    render(() => (
      <AgentsMachinesTable
        resources={[
          resource({
            id: 'tower',
            name: 'Tower',
            cpu: { current: 12 },
            memory: { current: 32, total: 100, used: 32, free: 68 },
            disk: { current: 45, total: 100, used: 45, free: 55 },
            network: { rxBytes: 1024, txBytes: 2048 },
            diskIO: { readRate: 4096, writeRate: 8192 },
            temperature: 52,
            uptime: 86_400,
            identity: { ips: ['192.168.0.10'] },
            agent: {
              agentVersion: '6.0.0',
              osName: 'Unraid',
              osVersion: '7.2.2',
              architecture: 'x86_64',
              kernelVersion: '6.12.24',
              raid: [
                {
                  device: '/dev/md0',
                  level: 'raid1',
                  state: 'clean',
                  totalDevices: 2,
                  activeDevices: 2,
                  workingDevices: 2,
                  failedDevices: 0,
                  spareDevices: 0,
                  devices: [],
                  rebuildPercent: 0,
                },
              ],
            },
          }),
        ]}
        emptyIcon={emptyIcon}
        emptyTitle="No machines"
        emptyDescription="Install Pulse Agent."
      />
    ));

    expect(screen.getByRole('button', { name: 'Sort by Net I/O' })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Sort by Disk I/O' })).toBeInTheDocument();
    expect(screen.getByText('1.00 KB/s')).toBeInTheDocument();
    expect(screen.getByText('8.00 KB/s')).toBeInTheDocument();

    expect(screen.queryByText('192.168.0.10')).not.toBeInTheDocument();
    await fireEvent.click(screen.getByTitle('Choose which columns to display'));
    await fireEvent.click(screen.getByLabelText('IP'));
    await fireEvent.click(screen.getByLabelText('RAID'));

    expect(screen.getByText('192.168.0.10')).toBeInTheDocument();
    expect(screen.getByText('1 clean')).toBeInTheDocument();
  });

  it('sorts machines by operational metrics from the column headers', async () => {
    const { container } = render(() => (
      <AgentsMachinesTable
        resources={[
          resource({ id: 'alpha', name: 'Alpha', cpu: { current: 3 } }),
          resource({ id: 'zulu', name: 'Zulu', cpu: { current: 80 } }),
        ]}
        emptyIcon={emptyIcon}
        emptyTitle="No machines"
        emptyDescription="Install Pulse Agent."
      />
    ));

    expect(
      Array.from(container.querySelectorAll('[data-agents-machine-row]')).map((row) =>
        row.getAttribute('data-agents-machine-row'),
      ),
    ).toEqual(['alpha', 'zulu']);

    await fireEvent.click(screen.getByRole('button', { name: 'Sort by CPU' }));

    expect(
      Array.from(container.querySelectorAll('[data-agents-machine-row]')).map((row) =>
        row.getAttribute('data-agents-machine-row'),
      ),
    ).toEqual(['zulu', 'alpha']);
  });

  it('keeps deep identity columns available through the column picker', async () => {
    render(() => (
      <AgentsMachinesTable
        resources={[
          resource({
            id: 'mac-mini',
            name: 'Mac Mini',
            agent: {
              architecture: 'arm64',
              kernelVersion: 'Darwin 25.5.0',
            },
          }),
        ]}
        emptyIcon={emptyIcon}
        emptyTitle="No machines"
        emptyDescription="Install Pulse Agent."
      />
    ));

    expect(screen.queryByText('arm64')).not.toBeInTheDocument();

    await fireEvent.click(screen.getByTitle('Choose which columns to display'));
    await fireEvent.click(screen.getByLabelText('Arch'));

    expect(screen.getByText('arm64')).toBeInTheDocument();
  });

  it('preserves agent CPU load and memory swap details in row metrics', () => {
    render(() => (
      <AgentsMachinesTable
        resources={[
          resource({
            id: 'workstation',
            name: 'Workstation',
            cpu: { current: 48 },
            memory: { current: 61 },
            agent: {
              cpuCount: 12,
              loadAverage: [2.45, 2.1, 1.9],
              memory: {
                used: 6_000,
                total: 12_000,
                free: 6_000,
                usage: 50,
                balloon: 8_000,
                swapUsed: 1_024,
                swapTotal: 2_048,
              },
            },
          }),
        ]}
        emptyIcon={emptyIcon}
        emptyTitle="No machines"
        emptyDescription="Install Pulse Agent."
      />
    ));

    expect(screen.getByTestId('agent-machine-cpu-bar')).toHaveAttribute(
      'data-load-average',
      '2.45',
    );
    expect(screen.getByTestId('agent-machine-cpu-bar')).toHaveAttribute('data-cores', '12');
    expect(screen.getByTestId('agent-machine-memory-bar')).toHaveAttribute(
      'data-swap-used',
      '1024',
    );
    expect(screen.getByTestId('agent-machine-memory-bar')).toHaveAttribute(
      'data-swap-total',
      '2048',
    );
    expect(screen.getByTestId('agent-machine-memory-bar')).toHaveAttribute('data-balloon', '8000');
  });
});
