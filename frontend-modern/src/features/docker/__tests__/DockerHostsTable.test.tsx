import { cleanup, fireEvent, render, screen, within } from '@solidjs/testing-library';
import { afterEach, describe, expect, it, vi } from 'vitest';

import type { Disk } from '@/types/api';
import type { Resource } from '@/types/resource';
import { DockerHostsTable } from '../DockerHostsTable';

vi.mock('@/components/shared/responsive', () => ({
  ResponsiveMetricCell: (props: { type: string; isRunning?: boolean; resourceId?: string }) => (
    <div
      data-testid={`responsive-${props.type}-metric`}
      data-resource-id={props.resourceId ?? ''}
      data-running={String(props.isRunning)}
    />
  ),
}));

vi.mock('@/contexts/appRuntime', () => ({
  useWebSocket: () => ({ activeAlerts: {} as Record<string, never> }),
}));
vi.mock('@/stores/alertsActivation', () => ({
  useAlertsActivation: () => ({
    detectionEnabled: () => true,
    getMetricThresholds: () => ({ warning: 80, critical: 85 }),
  }),
}));

vi.mock('@/components/Workloads/StackedMemoryBar', () => ({
  StackedMemoryBar: (props: { used: number; total: number; percentOnly?: number }) => (
    <div
      data-testid="stacked-memory-bar"
      data-used={String(props.used)}
      data-total={String(props.total)}
      data-percent-only={String(props.percentOnly ?? '')}
    />
  ),
}));

vi.mock('@/components/Workloads/StackedDiskBar', () => ({
  StackedDiskBar: (props: { disks?: Disk[]; aggregateDisk?: Disk; mode?: string }) => (
    <div
      data-testid="stacked-disk-bar"
      data-mode={props.mode ?? ''}
      data-disks={String(props.disks?.length ?? 0)}
      data-aggregate-usage={String(props.aggregateDisk?.usage ?? '')}
    />
  ),
}));

const makeDockerHost = (overrides: Partial<Resource> = {}): Resource => ({
  id: 'agent:docker-01',
  name: 'docker-01',
  displayName: 'docker-01',
  platformId: 'homelab',
  platformType: 'docker',
  sourceType: 'agent',
  status: 'degraded',
  type: 'agent',
  lastSeen: 1_700_000_000_000,
  cpu: { current: 42 },
  memory: { total: 8_000, used: 3_200, free: 4_800, current: 40 },
  disk: { total: 20_000, used: 12_500, free: 7_500, current: 62.5 },
  agent: {
    disks: [
      { device: '/dev/sda1', mountpoint: '/', total: 10_000, used: 6_000, free: 4_000 },
      {
        device: '/dev/sdb1',
        mountpoint: '/var/lib/docker',
        total: 10_000,
        used: 6_500,
        free: 3_500,
      },
    ],
  },
  docker: {
    runtimeVersion: '27.5.1',
    containerCount: 12,
  } as NonNullable<Resource['docker']> & { runtimeVersion?: string; containerCount?: number },
  ...overrides,
});

afterEach(() => {
  cleanup();
  vi.clearAllMocks();
});

describe('DockerHostsTable', () => {
  it('renders Docker hosts with a single-line Version column and shared metric bars', () => {
    render(() => (
      <DockerHostsTable
        resources={[makeDockerHost()]}
        emptyIcon={<span />}
        emptyTitle="No Docker hosts"
        emptyDescription="No hosts"
        showToolbar={false}
      />
    ));

    expect(screen.getByRole('columnheader', { name: 'Version' })).toBeInTheDocument();
    expect(screen.getByRole('columnheader', { name: 'System' })).toBeInTheDocument();
    expect(screen.queryByRole('columnheader', { name: 'Swarm role' })).not.toBeInTheDocument();
    expect(screen.queryByRole('columnheader', { name: 'Runtime' })).not.toBeInTheDocument();
    expect(screen.getByText('27.5.1')).toBeInTheDocument();
    expect(screen.queryByText('Docker')).not.toBeInTheDocument();
    expect(screen.getByTestId('responsive-cpu-metric')).toHaveAttribute('data-running', 'true');
    expect(screen.getByTestId('stacked-memory-bar')).toHaveAttribute('data-used', '3200');
    expect(screen.getByTestId('stacked-memory-bar')).toHaveAttribute('data-total', '8000');
    expect(screen.getByTestId('stacked-disk-bar')).toHaveAttribute('data-mode', 'vertical-bars');
    expect(screen.getByTestId('stacked-disk-bar')).toHaveAttribute('data-disks', '2');
  });

  it('opens host details inline without route navigation or submit-style drawer controls', () => {
    window.history.pushState({}, '', '/docker/overview');

    render(() => (
      <DockerHostsTable
        resources={[makeDockerHost()]}
        emptyIcon={<span />}
        emptyTitle="No Docker hosts"
        emptyDescription="No hosts"
        showToolbar={false}
      />
    ));

    const hostRow = screen.getByText('docker-01').closest('tr');
    expect(hostRow).not.toBeNull();

    fireEvent.click(hostRow!);

    expect(hostRow).toHaveAttribute('aria-expanded', 'true');
    expect(screen.getByTestId('docker-host-drawer')).toBeInTheDocument();
    expect(window.location.pathname).toBe('/docker/overview');
    expect(window.location.search).toBe('');
    expect(screen.getByRole('tab', { name: 'Overview' })).toHaveAttribute('type', 'button');
    expect(screen.getByRole('tab', { name: 'History' })).toHaveAttribute('type', 'button');
  });

  it('colors drawer host temperatures from configured thresholds', () => {
    render(() => (
      <DockerHostsTable
        resources={[makeDockerHost({ temperature: 76 })]}
        emptyIcon={<span />}
        emptyTitle="No Docker hosts"
        emptyDescription="No hosts"
        showToolbar={false}
      />
    ));

    fireEvent.click(screen.getByText('docker-01').closest('tr')!);

    const drawer = screen.getByTestId('docker-host-drawer');
    expect(within(drawer).getByText('76°C')).toHaveClass('text-green-600');
  });

  it('surfaces container update actions in the host drawer', () => {
    render(() => (
      <DockerHostsTable
        resources={[
          makeDockerHost({
            docker: {
              runtimeVersion: '27.5.1',
              containerCount: 12,
              hostSourceId: 'docker-01',
              updatesAvailableCount: 3,
            } as NonNullable<Resource['docker']>,
          }),
        ]}
        emptyIcon={<span />}
        emptyTitle="No Docker hosts"
        emptyDescription="No hosts"
        showToolbar={false}
      />
    ));

    fireEvent.click(screen.getByText('docker-01').closest('tr')!);

    expect(screen.getByRole('button', { name: 'Check updates' })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Update all (3)' })).toBeInTheDocument();
  });

  it('identifies the host system separately from the container runtime', () => {
    render(() => (
      <DockerHostsTable
        resources={[
          makeDockerHost({
            name: 'tower',
            agent: {
              hostProfile: 'unraid',
              osName: 'Unraid OS',
              osVersion: '6.12.10',
            },
          }),
          makeDockerHost({
            id: 'agent:qnap-01',
            name: 'qnap-01',
            agent: {
              platform: 'linux',
              osName: 'QuTS hero',
              osVersion: '5.2',
            },
          }),
        ]}
        emptyIcon={<span />}
        emptyTitle="No Docker hosts"
        emptyDescription="No hosts"
        showToolbar={false}
      />
    ));

    expect(screen.getAllByText('Unraid 6.12.10').length).toBeGreaterThan(0);
    expect(screen.getAllByText('QNAP 5.2').length).toBeGreaterThan(0);
    expect(screen.queryByText('Docker / Podman')).not.toBeInTheDocument();
  });

  it('shows Swarm role only for hosts with active Swarm evidence', () => {
    render(() => (
      <DockerHostsTable
        resources={[
          makeDockerHost({
            docker: {
              runtimeVersion: '27.5.1',
              containerCount: 12,
              swarm: {
                nodeId: 'node-1',
                nodeRole: 'manager',
                localState: 'active',
              },
            } as NonNullable<Resource['docker']> & {
              runtimeVersion?: string;
              containerCount?: number;
            },
          }),
        ]}
        emptyIcon={<span />}
        emptyTitle="No Docker hosts"
        emptyDescription="No hosts"
        showToolbar={false}
      />
    ));

    expect(screen.getByRole('columnheader', { name: 'Swarm role' })).toBeInTheDocument();
    expect(screen.getByText('Manager')).toBeInTheDocument();
  });

  it('does not show inactive standalone Swarm metadata as a host role', () => {
    render(() => (
      <DockerHostsTable
        resources={[
          makeDockerHost({
            docker: {
              runtimeVersion: '27.5.1',
              containerCount: 12,
              swarm: {
                nodeRole: 'worker',
                localState: 'inactive',
                scope: 'node',
              },
            } as NonNullable<Resource['docker']> & {
              runtimeVersion?: string;
              containerCount?: number;
            },
          }),
        ]}
        emptyIcon={<span />}
        emptyTitle="No Docker hosts"
        emptyDescription="No hosts"
        showToolbar={false}
      />
    ));

    expect(screen.queryByRole('columnheader', { name: 'Swarm role' })).not.toBeInTheDocument();
    expect(screen.queryByText('Worker')).not.toBeInTheDocument();
  });

  it('uses percent-only memory and aggregate disk bars when capacity details are missing', () => {
    render(() => (
      <DockerHostsTable
        resources={[
          makeDockerHost({
            status: 'online',
            memory: { current: 55 },
            disk: { current: 71 },
            agent: undefined,
          }),
        ]}
        emptyIcon={<span />}
        emptyTitle="No Docker hosts"
        emptyDescription="No hosts"
        showToolbar={false}
      />
    ));

    expect(screen.getByTestId('stacked-memory-bar')).toHaveAttribute('data-total', '0');
    expect(screen.getByTestId('stacked-memory-bar')).toHaveAttribute('data-percent-only', '55');
    expect(screen.getByTestId('stacked-disk-bar')).toHaveAttribute('data-disks', '0');
    expect(screen.getByTestId('stacked-disk-bar')).toHaveAttribute('data-aggregate-usage', '71');
  });
});
